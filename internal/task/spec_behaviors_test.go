package task_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	task "github.com/clintmod/rite/internal/task"
)

// TestPrecedenceShellVsCLI asserts SPEC §Variable Precedence tier 1 beats
// tier 2: a shell env var must win over the same name passed as a CLI var,
// and over entrypoint and task-scope defaults. See SPEC.md worked example.
func TestPrecedenceShellVsCLI(t *testing.T) {
	t.Setenv("VAR", "shell")
	NewExecutorTest(t,
		WithExecutorOptions(
			task.WithDir("testdata/precedence_shell_vs_cli"),
			task.WithSilent(true),
			task.WithForce(true),
		),
		WithVar("VAR", "cli"),
	)
}

// TestPrecedenceDotenvVsVars asserts SPEC §Variable Precedence tier-3
// (entrypoint `env:` / dotenv) slots correctly between tier-2 (CLI) and
// tier-4 (entrypoint `vars:`): a dotenv entry beats an entrypoint-vars
// default of the same name, but loses to a CLI override.
func TestPrecedenceDotenvVsVars(t *testing.T) {
	t.Parallel()
	NewExecutorTest(t,
		WithName("dotenv-beats-entrypoint-vars"),
		WithExecutorOptions(
			task.WithDir("testdata/precedence_dotenv_vs_vars"),
			task.WithSilent(true),
			task.WithForce(true),
		),
	)
	NewExecutorTest(t,
		WithName("cli-beats-dotenv"),
		WithExecutorOptions(
			task.WithDir("testdata/precedence_dotenv_vs_vars"),
			task.WithSilent(true),
			task.WithForce(true),
		),
		WithVar("VAR", "cli"),
	)
}

// TestExportFalse asserts SPEC §Non-exported variables: a var declared with
// `export: false` is visible to Go-template rendering but is not exported to
// the cmd shell environ.
func TestExportFalse(t *testing.T) {
	// SECRET must not leak in from the caller's shell for this test to mean
	// what it claims. Set it to a sentinel that would obviously corrupt the
	// golden output if `export: false` were silently ignored, then unset —
	// t.Setenv tracks the var so teardown restores the prior state even
	// after our Unsetenv below. SHOWN is also cleared for the same reason.
	t.Setenv("SECRET", "caller-should-not-see-this")
	require.NoError(t, os.Unsetenv("SECRET"))
	t.Setenv("SHOWN", "caller-visible")
	require.NoError(t, os.Unsetenv("SHOWN"))
	NewExecutorTest(t,
		WithExecutorOptions(
			task.WithDir("testdata/export_false"),
			task.WithSilent(true),
			task.WithForce(true),
		),
	)
}

// TestLazyDynamicVars asserts SPEC §Dynamic Variables: a `sh:` declared on a
// task that is never invoked must never execute. We use a sentinel file in a
// per-test TempDir — if the `sh:` fires eagerly, the file will exist.
func TestLazyDynamicVars(t *testing.T) {
	tmp := t.TempDir()
	sentinel := filepath.Join(tmp, "sentinel")
	t.Setenv("LAZY_SENTINEL_PATH", sentinel)

	NewExecutorTest(t,
		WithExecutorOptions(
			task.WithDir("testdata/lazy_dynamic"),
			task.WithSilent(true),
			task.WithForce(true),
		),
	)

	_, err := os.Stat(sentinel)
	require.True(t, os.IsNotExist(err),
		"sh: on the uninvoked `expensive` task fired eagerly — SPEC §Dynamic Variables requires lazy evaluation. stat err: %v", err)
}
