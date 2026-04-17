package task_test

import (
	"bytes"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	task "github.com/clintmod/rite/internal/task"
)

// TestColorForwardToCmds is the regression test for #153: when the outer
// rite has color resolved as on (terminal TTY, or --color / FORCE_COLOR /
// CLICOLOR_FORCE), child processes spawned by the cmd layer must see
// CLICOLOR_FORCE=1 and FORCE_COLOR=1 in their environ so color-aware CLIs
// (another `rite -l`, `ls`, `rg`, `fd`, …) don't strip ANSI just because
// their stdout is a pipe into us.
//
// Not t.Parallel and not in the executor-test harness: we need to clear the
// package-init NO_COLOR=1 (see task_test.go) and un-stick fatih/color's
// NoColor global, which races with other tests that assume the default
// no-color state.
func TestColorForwardToCmds(t *testing.T) {
	// Ensure the injection path is active: no ambient NO_COLOR, no
	// explicit off-signal, color.NoColor flipped on.
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("FORCE_COLOR", "")
	prevNoColor := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = prevNoColor })

	var stdout bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/color-forward"),
		task.WithStdout(&stdout),
		task.WithStderr(&bytes.Buffer{}),
		task.WithColor(true),
		task.WithSilent(true),
	)
	require.NoError(t, e.Setup())
	require.NoError(t, e.Run(t.Context(), &task.Call{Task: "default"}))

	out := stdout.String()
	assert.Contains(t, out, "CLICOLOR_FORCE=1", "expected CLICOLOR_FORCE=1 in child env, got %q", out)
	assert.Contains(t, out, "FORCE_COLOR=1", "expected FORCE_COLOR=1 in child env, got %q", out)
}

// TestColorForwardToCmds_NoColorSuppresses verifies that NO_COLOR in the
// parent environ suppresses the injection — color-aware children see the
// un-augmented env and decide on their own (NO_COLOR reaches them too).
func TestColorForwardToCmds_NoColorSuppresses(t *testing.T) {
	// NO_COLOR=1 is the ambient test-harness state; just be explicit.
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("FORCE_COLOR", "")
	prevNoColor := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = prevNoColor })

	var stdout bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/color-forward"),
		task.WithStdout(&stdout),
		task.WithStderr(&bytes.Buffer{}),
		task.WithColor(true),
		task.WithSilent(true),
	)
	require.NoError(t, e.Setup())
	require.NoError(t, e.Run(t.Context(), &task.Call{Task: "default"}))

	out := stdout.String()
	assert.Contains(t, out, "CLICOLOR_FORCE=unset", "NO_COLOR=1 should suppress CLICOLOR_FORCE injection, got %q", out)
	assert.Contains(t, out, "FORCE_COLOR=unset", "NO_COLOR=1 should suppress FORCE_COLOR injection, got %q", out)
}

// TestColorForwardToCmds_ColorOffDoesNotInject verifies the symmetric
// case: when rite's --color is explicitly false, we must not force color
// on children even if other ambient state would allow it.
func TestColorForwardToCmds_ColorOffDoesNotInject(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("FORCE_COLOR", "")
	prevNoColor := color.NoColor
	color.NoColor = true // what flags.go sets when --color=false
	t.Cleanup(func() { color.NoColor = prevNoColor })

	var stdout bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/color-forward"),
		task.WithStdout(&stdout),
		task.WithStderr(&bytes.Buffer{}),
		task.WithColor(false),
		task.WithSilent(true),
	)
	require.NoError(t, e.Setup())
	require.NoError(t, e.Run(t.Context(), &task.Call{Task: "default"}))

	out := stdout.String()
	assert.Contains(t, out, "CLICOLOR_FORCE=unset", "--color=false must not inject CLICOLOR_FORCE, got %q", out)
	assert.Contains(t, out, "FORCE_COLOR=unset", "--color=false must not inject FORCE_COLOR, got %q", out)
}
