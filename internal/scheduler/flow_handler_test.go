package scheduler

import (
	"fmt"
	"sync"
	"testing"
)

// A shallow copy would leave the nested node and global buckets shared, so a reader would still
// race against the scheduler merging later results.
func TestCloneOutputs_IsDeep(t *testing.T) {
	src := map[string]any{
		"FLAT": "1",
		"web1": map[string]any{"IP": "10.0.0.1"},
		globalOutputKey: map[string]any{
			"build": map[string]any{"BUILD_ID": "42"},
		},
	}

	clone := cloneOutputs(src)

	src["FLAT"] = "changed"
	src["web1"].(map[string]any)["IP"] = "changed"
	src[globalOutputKey].(map[string]any)["build"].(map[string]any)["BUILD_ID"] = "changed"
	src["NEW"] = "added"

	if got := clone["FLAT"]; got != "1" {
		t.Errorf("clone FLAT = %v, want 1", got)
	}
	if got := clone["web1"].(map[string]any)["IP"]; got != "10.0.0.1" {
		t.Errorf("clone web1.IP = %v, want 10.0.0.1", got)
	}
	if got := clone[globalOutputKey].(map[string]any)["build"].(map[string]any)["BUILD_ID"]; got != "42" {
		t.Errorf("clone global.build.BUILD_ID = %v, want 42", got)
	}
	if _, ok := clone["NEW"]; ok {
		t.Error("clone picked up a key added to the source afterwards")
	}
}

// Mirrors what the dag runner does: readers get a snapshot at dispatch while the scheduler keeps
// merging results into the live map. Run with -race.
func TestCloneOutputs_SnapshotWhileMerging(t *testing.T) {
	outputs := map[string]any{}
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		snapshot := cloneOutputs(outputs)

		wg.Add(1)
		go func(snap map[string]any) {
			defer wg.Done()
			for k, v := range snap {
				if inner, ok := v.(map[string]any); ok {
					for range inner {
					}
					continue
				}
				_ = fmt.Sprint(k, v)
			}
		}(snapshot)

		actionID := fmt.Sprintf("action_%d", i)
		processActionResults(map[string]string{
			fmt.Sprintf("KEY_%d", i):           "value",
			fmt.Sprintf("NODE_KEY_%d@web1", i): "value",
		}, outputs)
		mergeGlobals(map[string]string{"BUILD_ID": actionID}, actionID, outputs)
	}

	wg.Wait()
}
