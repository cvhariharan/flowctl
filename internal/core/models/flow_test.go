package models

import (
	"strings"
	"testing"
)

const sequentialFlow = `metadata:
    id: greet_user
    name: Greet User
    description: A simple greeting workflow
    namespace: default
    allow_overlap: true
    max_retries: 0
inputs: []
actions:
    - id: greet
      name: Greet
      executor: docker
      with:
        image: docker.io/alpine
        script: echo hi
      approval: false
      variables: []
      "on": []
outputs: []
`

// A flow without dependencies must round-trip without gaining execution_mode or needs keys,
// otherwise saving an existing flow from the UI would silently change how it runs.
func TestMarshalFlow_SequentialAddsNoDAGKeys(t *testing.T) {
	f, err := UnmarshalFlow([]byte(sequentialFlow), FlowFormatYAML)
	if err != nil {
		t.Fatalf("UnmarshalFlow() error = %v", err)
	}

	out, err := MarshalFlow(f, FlowFormatYAML)
	if err != nil {
		t.Fatalf("MarshalFlow() error = %v", err)
	}

	for _, key := range []string{"execution_mode", "max_parallel", "needs"} {
		if strings.Contains(string(out), key) {
			t.Errorf("marshalled sequential flow contains %q:\n%s", key, out)
		}
	}

	back, err := UnmarshalFlow(out, FlowFormatYAML)
	if err != nil {
		t.Fatalf("UnmarshalFlow(round trip) error = %v", err)
	}
	if back.Meta.IsDAG() {
		t.Error("round-tripped sequential flow reports dag mode")
	}
}

func dagFlow(t *testing.T, mode ExecutionMode, actions []Action) Flow {
	t.Helper()
	return Flow{
		Meta:    Metadata{ID: "f", Name: "F", ExecutionMode: mode},
		Inputs:  []Input{},
		Actions: actions,
	}
}

func action(id string, needs ...string) Action {
	return Action{
		ID:    id,
		Name:  id,
		With:  map[string]any{"script": "true"},
		Needs: needs,
	}
}

func TestValidate_Dependencies(t *testing.T) {
	tests := []struct {
		name    string
		flow    Flow
		wantErr string
	}{
		{
			name: "diamond is valid",
			flow: dagFlow(t, ExecutionModeDAG, []Action{
				action("build"),
				action("test_unit", "build"),
				action("test_integration", "build"),
				action("deploy", "test_unit", "test_integration"),
			}),
		},
		{
			name: "duplicate needs entries are tolerated",
			flow: dagFlow(t, ExecutionModeDAG, []Action{
				action("build"),
				action("deploy", "build", "build"),
			}),
		},
		{
			name: "forward reference is valid",
			flow: dagFlow(t, ExecutionModeDAG, []Action{
				action("deploy", "build"),
				action("build"),
			}),
		},
		{
			name: "needs without dag mode",
			flow: dagFlow(t, "", []Action{
				action("build"),
				action("deploy", "build"),
			}),
			wantErr: "requires metadata.execution_mode",
		},
		{
			name: "needs with explicit sequential mode",
			flow: dagFlow(t, ExecutionModeSequential, []Action{
				action("build"),
				action("deploy", "build"),
			}),
			wantErr: "requires metadata.execution_mode",
		},
		{
			name: "unknown dependency",
			flow: dagFlow(t, ExecutionModeDAG, []Action{
				action("deploy", "nope"),
			}),
			wantErr: `needs unknown action "nope"`,
		},
		{
			name: "self dependency",
			flow: dagFlow(t, ExecutionModeDAG, []Action{
				action("deploy", "deploy"),
			}),
			wantErr: "cannot depend on itself",
		},
		{
			name: "two node cycle",
			flow: dagFlow(t, ExecutionModeDAG, []Action{
				action("a", "b"),
				action("b", "a"),
			}),
			wantErr: "dependency cycle",
		},
		{
			name: "three node cycle",
			flow: dagFlow(t, ExecutionModeDAG, []Action{
				action("a", "c"),
				action("b", "a"),
				action("c", "b"),
			}),
			wantErr: "dependency cycle",
		},
		{
			name: "dag mode without any needs is valid",
			flow: dagFlow(t, ExecutionModeDAG, []Action{
				action("a"),
				action("b"),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.flow.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
