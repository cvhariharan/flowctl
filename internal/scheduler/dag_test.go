package scheduler

import (
	"reflect"
	"testing"
)

func acts(pairs ...[]string) []Action {
	out := make([]Action, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, Action{ID: p[0], Needs: p[1:]})
	}
	return out
}

func mustGraph(t *testing.T, actions []Action) *Graph {
	t.Helper()
	g, err := BuildGraph(actions)
	if err != nil {
		t.Fatalf("BuildGraph() error = %v", err)
	}
	return g
}

func TestBuildGraph_Roots(t *testing.T) {
	g := mustGraph(t, acts(
		[]string{"build"},
		[]string{"lint"},
		[]string{"test", "build"},
		[]string{"deploy", "test", "lint"},
	))

	if got, want := g.Roots(), []string{"build", "lint"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Roots() = %v, want %v", got, want)
	}
}

func TestBuildGraph_Levels(t *testing.T) {
	tests := []struct {
		name    string
		actions []Action
		want    [][]string
	}{
		{
			name:    "chain",
			actions: acts([]string{"a"}, []string{"b", "a"}, []string{"c", "b"}),
			want:    [][]string{{"a"}, {"b"}, {"c"}},
		},
		{
			name:    "diamond",
			actions: acts([]string{"a"}, []string{"b", "a"}, []string{"c", "a"}, []string{"d", "b", "c"}),
			want:    [][]string{{"a"}, {"b", "c"}, {"d"}},
		},
		{
			name:    "all roots",
			actions: acts([]string{"a"}, []string{"b"}, []string{"c"}),
			want:    [][]string{{"a", "b", "c"}},
		},
		{
			name: "longest path wins",
			actions: acts(
				[]string{"a"},
				[]string{"b", "a"},
				[]string{"c", "b"},
				[]string{"d", "a", "c"},
			),
			want: [][]string{{"a"}, {"b"}, {"c"}, {"d"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustGraph(t, tt.actions).Levels()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Levels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGraph_Descendants(t *testing.T) {
	g := mustGraph(t, acts(
		[]string{"a"},
		[]string{"b", "a"},
		[]string{"c", "a"},
		[]string{"d", "b"},
		[]string{"isolated"},
	))

	got := g.Descendants("a")
	want := map[string]bool{"b": true, "c": true, "d": true}
	if len(got) != len(want) {
		t.Fatalf("Descendants(a) = %v, want keys %v", got, want)
	}
	for _, id := range got {
		if !want[id] {
			t.Errorf("Descendants(a) returned unexpected %q", id)
		}
	}

	if got := g.Descendants("isolated"); len(got) != 0 {
		t.Errorf("Descendants(isolated) = %v, want empty", got)
	}
}

func TestBuildGraph_Errors(t *testing.T) {
	tests := []struct {
		name    string
		actions []Action
	}{
		{"unknown dependency", acts([]string{"a", "missing"})},
		{"self reference", acts([]string{"a", "a"})},
		{"two node cycle", acts([]string{"a", "b"}, []string{"b", "a"})},
		{"three node cycle", acts([]string{"a", "c"}, []string{"b", "a"}, []string{"c", "b"})},
		{"cycle behind a root", acts([]string{"root"}, []string{"a", "b", "root"}, []string{"b", "a"})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildGraph(tt.actions); err == nil {
				t.Error("BuildGraph() = nil error, want error")
			}
		})
	}
}

func TestBuildGraph_Empty(t *testing.T) {
	g := mustGraph(t, nil)
	if got := g.Roots(); len(got) != 0 {
		t.Errorf("Roots() = %v, want empty", got)
	}
	if got := g.Levels(); len(got) != 0 {
		t.Errorf("Levels() = %v, want empty", got)
	}
}
