package core

import (
	"errors"
	"reflect"
	"testing"

	"github.com/cvhariharan/flowctl/internal/core/models"
)

func TestResetActionIDsDAG(t *testing.T) {
	flow := models.Flow{
		Meta: models.Metadata{ExecutionMode: models.ExecutionModeDAG},
		Actions: []models.Action{
			{ID: "build"},
			{ID: "test_unit", Needs: []string{"build"}},
			{ID: "test_integration", Needs: []string{"build"}},
			{ID: "deploy", Needs: []string{"test_unit", "test_integration"}},
		},
	}

	tests := []struct {
		from string
		want []string
	}{
		{from: "test_unit", want: []string{"test_unit", "deploy"}},
		{from: "build", want: []string{"build", "test_unit", "test_integration", "deploy"}},
	}
	for _, tt := range tests {
		t.Run(tt.from, func(t *testing.T) {
			got, err := resetActionIDs(flow, tt.from)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("resetActionIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResetActionIDsSequential(t *testing.T) {
	flow := models.Flow{Actions: []models.Action{{ID: "first"}, {ID: "middle"}, {ID: "last"}}}
	got, err := resetActionIDs(flow, "middle")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"middle", "last"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("resetActionIDs() = %v, want %v", got, want)
	}
}

func TestResetActionIDsUnknown(t *testing.T) {
	flow := models.Flow{Actions: []models.Action{{ID: "known"}}}
	_, err := resetActionIDs(flow, "missing")
	if !errors.Is(err, ErrActionNotFound) {
		t.Fatalf("error = %v, want ErrActionNotFound", err)
	}
}
