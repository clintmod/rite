package task_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/clintmod/rite/errors"
	task "github.com/clintmod/rite/internal/task"
)

// Issue #129: same-name keys in `vars:` and `env:` at the same scope must
// fail at load time. The two blocks share a single variable table under
// SPEC §vars / env Unification — picking a winner silently would hide the
// precedence the SPEC is built to make obvious.
//
// We exercise three cases: entrypoint-scope collision, task-scope collision,
// and a cross-scope control where the same key appears in entrypoint vars
// and task env (different scopes — independent, not a collision).

func TestVarsEnvCollision_Entrypoint(t *testing.T) {
	t.Parallel()
	e := task.NewExecutor(
		task.WithDir("testdata/vars_env_collision/entrypoint"),
		task.WithVersionCheck(true),
	)
	err := e.Setup()
	require.Error(t, err, "entrypoint vars/env collision should fail at load time")

	// The collision error itself is a TaskError (CodeRitefileInvalid) but it
	// gets wrapped by RitefileInvalidError in the reader, so assert on the
	// substring rather than the exact type — the user-visible message is
	// what we care about.
	assert.Contains(t, err.Error(), `variable "FOO" declared in both vars: and env: at top-level scope`)
	assert.Contains(t, err.Error(), "vars: and env: are the same variable table")
}

func TestVarsEnvCollision_Task(t *testing.T) {
	t.Parallel()
	e := task.NewExecutor(
		task.WithDir("testdata/vars_env_collision/task"),
		task.WithVersionCheck(true),
	)
	err := e.Setup()
	require.Error(t, err, "task-scope vars/env collision should fail at load time")
	assert.Contains(t, err.Error(), `variable "FOO" declared in both tasks.deploy.vars and tasks.deploy.env`)
	assert.Contains(t, err.Error(), "vars: and env: are the same variable table")
}

// A key declared in entrypoint vars and a different scope's env (here, a
// task's env) is NOT a collision — the scopes are independent. The loader
// must accept this without error.
func TestVarsEnvCollision_CrossScopeOK(t *testing.T) {
	t.Parallel()
	e := task.NewExecutor(
		task.WithDir("testdata/vars_env_collision/cross_scope_ok"),
		task.WithVersionCheck(true),
	)
	require.NoError(t, e.Setup(), "cross-scope same-name should load cleanly")
}

// Validate() runs the full read+merge path, so the same collision should
// surface there for editor/lint stages — not just at execution.
func TestVarsEnvCollision_SurfacesViaValidate(t *testing.T) {
	t.Parallel()
	e := task.NewExecutor(
		task.WithDir("testdata/vars_env_collision/entrypoint"),
		task.WithVersionCheck(true),
	)
	err := e.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `variable "FOO" declared in both vars: and env: at top-level scope`)

	// Confirm the underlying typed error is reachable for callers that want
	// to branch on it (e.g. an LSP that wants a structured diagnostic).
	var collision *errors.VarEnvCollisionError
	if errors.As(err, &collision) {
		assert.Equal(t, "FOO", collision.Name)
		assert.Empty(t, collision.TaskName, "entrypoint scope should have empty TaskName")
	}
}
