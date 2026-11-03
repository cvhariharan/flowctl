package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/google/uuid"
)

const versionedFlowSource = `metadata:
  id: test_flow
  name: Test Flow
inputs: []
actions:
  - id: first
    name: First
    executor: shell
    with:
      script: echo original
`

func TestParseFlowSource(t *testing.T) {
	for _, format := range []models.FlowFormat{models.FlowFormatYAML, models.FlowFormatHUML} {
		t.Run(string(format), func(t *testing.T) {
			f, err := models.UnmarshalFlow([]byte(versionedFlowSource), models.FlowFormatYAML)
			if err != nil {
				t.Fatal(err)
			}
			data, err := models.MarshalFlow(f, format)
			if err != nil {
				t.Fatal(err)
			}
			got, err := parseFlowSource(data, format, "default", "test_dir")
			if err != nil {
				t.Fatal(err)
			}
			if got.Meta.Namespace != "default" || got.Meta.SrcDir != "test_dir" || got.Actions[0].ID != "first" {
				t.Fatalf("unexpected flow: %+v", got)
			}
		})
	}
	for _, tt := range []struct{ name, source string }{
		{"namespace mismatch", strings.Replace(versionedFlowSource, "  id: test_flow", "  namespace: other\n  id: test_flow", 1)},
		{"invalid flow", "metadata: {}"},
		{"invalid YAML", "metadata: ["},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseFlowSource([]byte(tt.source), models.FlowFormatYAML, "default", "test_dir"); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestNullVersionDBID(t *testing.T) {
	for _, id := range []int32{0, 1, 3} {
		got := nullVersionDBID(id)
		if got.Int32 != id || got.Valid != (id != 0) {
			t.Fatalf("nullVersionDBID(%d) = %+v", id, got)
		}
	}
}

func TestDiffLines(t *testing.T) {
	for _, tt := range []struct {
		name, a, b string
		want       []DiffLine
	}{
		{"identical", "a\nb\n", "a\nb\n", []DiffLine{{"equal", 1, 1, "a"}, {"equal", 2, 2, "b"}}},
		{"insert", "a\n", "a\nb\n", []DiffLine{{"equal", 1, 1, "a"}, {"insert", 0, 2, "b"}}},
		{"delete", "a\nb\n", "a\n", []DiffLine{{"equal", 1, 1, "a"}, {"delete", 2, 0, "b"}}},
		{"middle change", "a\nb\nc\n", "a\nx\nc\n", []DiffLine{{"equal", 1, 1, "a"}, {"delete", 2, 0, "b"}, {"insert", 0, 2, "x"}, {"equal", 3, 3, "c"}}},
		{"no trailing newline", "a", "b", []DiffLine{{"delete", 1, 0, "a"}, {"insert", 0, 1, "b"}}},
		{"trailing newline changed", "a", "a\n", []DiffLine{{"delete", 1, 0, "a"}, {"insert", 0, 1, "a"}}},
		{"empty old", "", "a\n", []DiffLine{{"insert", 0, 1, "a"}}},
		{"empty new", "a\n", "", []DiffLine{{"delete", 1, 0, "a"}}},
		{"both empty", "", "", []DiffLine{}},
		{"blank line", "a\n\n", "a\n\n", []DiffLine{{"equal", 1, 1, "a"}, {"equal", 2, 2, ""}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := diffLines(tt.a, tt.b); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDiffLinesLargeFile(t *testing.T) {
	var source strings.Builder
	for i := 0; i < 3000; i++ {
		fmt.Fprintf(&source, "line %d\n", i)
	}
	a := source.String()
	b := strings.Replace(a, "line 2900\n", "changed\n", 1)
	var changes []DiffLine
	for _, line := range diffLines(a, b) {
		if line.Op != "equal" {
			changes = append(changes, line)
		}
	}
	want := []DiffLine{{"delete", 2901, 0, "line 2900"}, {"insert", 0, 2901, "changed"}}
	if !reflect.DeepEqual(changes, want) {
		t.Fatalf("changes = %+v, want %+v", changes, want)
	}
}

type flowVersionStore struct {
	repo.Store
	source  repo.GetExecutionFlowSourceRow
	version repo.GetFlowVersionRow
}

func (s flowVersionStore) GetExecutionFlowSource(context.Context, repo.GetExecutionFlowSourceParams) (repo.GetExecutionFlowSourceRow, error) {
	return s.source, nil
}
func (s flowVersionStore) GetFlowVersion(context.Context, repo.GetFlowVersionParams) (repo.GetFlowVersionRow, error) {
	return s.version, nil
}

func TestGetFlowFromLogIDPinnedAndLegacy(t *testing.T) {
	ns := uuid.NewString()
	current := models.Flow{Meta: models.Metadata{ID: "test_flow", VersionDBID: 12, Version: 1}, Actions: []models.Action{{ID: "current_only"}}}
	row := repo.GetExecutionFlowSourceRow{FlowID: 1, Slug: "test_flow", NamespaceName: "default", FilePath: "/flows/default/test_flow/flow.yaml", VersionID: nullVersionDBID(13), Version: sql.NullInt32{Int32: 2, Valid: true}, Format: sql.NullString{String: "yaml", Valid: true}, Source: sql.NullString{String: versionedFlowSource, Valid: true}}
	for _, tt := range []struct {
		name       string
		id         int32
		source     string
		wantAction string
		wantErr    bool
	}{
		{"legacy", 0, "", "current_only", false},
		{"active", 12, "", "current_only", false},
		{"pinned newer than active", 13, versionedFlowSource, "first", false},
		{"invalid pinned never falls back", 13, "metadata: {}", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := row
			r.VersionID, r.Source.String = nullVersionDBID(tt.id), tt.source
			c := &Core{store: flowVersionStore{source: r}, flows: map[string]models.Flow{"test_flow:" + ns: current}}
			f, err := c.GetFlowFromLogID("exec", ns)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected pinned parse error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if f.Actions[0].ID != tt.wantAction {
				t.Fatalf("got action %s, want %s", f.Actions[0].ID, tt.wantAction)
			}
			if tt.id == 13 && (f.Meta.VersionDBID != 13 || f.Meta.Version != 2) {
				t.Fatalf("lost version: %+v", f.Meta)
			}
		})
	}
}

func TestActivateFlowVersionRejectsBeforeWriting(t *testing.T) {
	ns := uuid.NewString()
	path := filepath.Join(t.TempDir(), "flow.yaml")
	if err := os.WriteFile(path, []byte("original file"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, source, format string
		want                 error
	}{
		{"invalid", "metadata: {}", "yaml", ErrFlowVersionInvalid},
		{"wrong format", "", "huml", ErrFlowVersionFormatMismatch},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := tt.source
			if tt.format == "huml" {
				f, err := models.UnmarshalFlow([]byte(versionedFlowSource), models.FlowFormatYAML)
				if err != nil {
					t.Fatal(err)
				}
				data, err := models.MarshalFlow(f, models.FlowFormatHUML)
				if err != nil {
					t.Fatal(err)
				}
				source = string(data)
			}
			c := &Core{store: flowVersionStore{version: repo.GetFlowVersionRow{Source: source, Format: tt.format, NamespaceName: "default", FilePath: path}}, flows: map[string]models.Flow{"test_flow:" + ns: {Meta: models.Metadata{Version: 1}}}}
			if _, err := c.ActivateFlowVersion(context.Background(), "test_flow", ns, 2); !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "original file" {
				t.Fatal("rejected switch modified file")
			}
		})
	}
}
