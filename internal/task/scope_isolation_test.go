package task_test

import (
	"fmt"
	"testing"

	task "github.com/clintmod/rite/internal/task"
)

// TestScopeIsolation exercises SPEC §Scoping — vars flow downward only,
// siblings and included Ritefiles are sandboxed from each other, and
// child task-scope vars cannot leak upward into their caller.
func TestScopeIsolation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		call string
	}{
		{name: "parent_child"},
		{name: "siblings"},
		{name: "cross_include"},
		{name: "nested_includes"},
	}

	for _, test := range tests {
		call := test.call
		if call == "" {
			call = "default"
		}
		NewExecutorTest(t,
			WithName(test.name),
			WithExecutorOptions(
				task.WithDir(fmt.Sprintf("testdata/scope_isolation/%s", test.name)),
				task.WithSilent(true),
				task.WithForce(true),
			),
			WithTask(call),
		)
	}
}
