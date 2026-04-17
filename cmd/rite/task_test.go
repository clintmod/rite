package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/clintmod/rite/internal/logger"
	task "github.com/clintmod/rite/internal/task"
)

// TestBareInvocationFallback covers the #102 / #154 fallback: when `rite` is
// invoked with no positional task, a missing `default:` entry should surface
// the task list (identical to `rite -l`) instead of erroring. An entirely-
// empty Ritefile should print a friendly hint.
func TestBareInvocationFallback(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name               string
		dir                string
		wantHandled        bool
		wantStdoutContains []string
		wantStdoutEmpty    bool
		wantStderrMentions string
	}{
		{
			name:        "default task present — no fallback",
			dir:         "testdata/bare_default",
			wantHandled: false,
			// When handled is false, no output is produced by the fallback
			// itself; the caller proceeds with the default task.
			wantStdoutEmpty: true,
		},
		{
			name:        "no default task — list like `rite -l`",
			dir:         "testdata/bare_nodefault",
			wantHandled: true,
			wantStdoutContains: []string{
				"rite: Available tasks for this project:",
				"* build:",
				"Build the binary",
				"* test:",
				"Run tests",
			},
		},
		{
			name:               "empty Ritefile — friendly hint",
			dir:                "testdata/bare_empty",
			wantHandled:        true,
			wantStderrMentions: "no tasks defined",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			e := task.NewExecutor(
				task.WithDir(tc.dir),
				task.WithStdout(&stdout),
				task.WithStderr(&stderr),
				task.WithVersionCheck(true),
			)
			require.NoError(t, e.Setup())

			log := &logger.Logger{Stdout: &stdout, Stderr: &stderr}
			handled, err := bareInvocationFallback(e, log)
			require.NoError(t, err)
			assert.Equal(t, tc.wantHandled, handled)

			if tc.wantStdoutEmpty {
				assert.Empty(t, stdout.String())
			}
			for _, want := range tc.wantStdoutContains {
				assert.Contains(t, stdout.String(), want)
			}
			if tc.wantStderrMentions != "" {
				// The "no tasks defined" hint is informational, written via
				// the Logger's Outf — which routes to Stderr by default.
				combined := stdout.String() + stderr.String()
				assert.Contains(t, combined, tc.wantStderrMentions)
			}
		})
	}
}

// TestBareInvocationFallbackMatchesListFlag is the #154 contract: `rite` with
// no args on a no-default Ritefile must produce byte-for-byte the same output
// as `rite -l` on the same Ritefile. The two paths must share a formatter
// (header + tabwriter) — not drift into parallel implementations.
func TestBareInvocationFallbackMatchesListFlag(t *testing.T) {
	t.Parallel()

	const dir = "testdata/bare_nodefault"

	// Bare-invocation output
	var bareOut, bareErr bytes.Buffer
	bareExec := task.NewExecutor(
		task.WithDir(dir),
		task.WithStdout(&bareOut),
		task.WithStderr(&bareErr),
		task.WithVersionCheck(true),
	)
	require.NoError(t, bareExec.Setup())
	bareLog := &logger.Logger{Stdout: &bareOut, Stderr: &bareErr}
	handled, err := bareInvocationFallback(bareExec, bareLog)
	require.NoError(t, err)
	require.True(t, handled)

	// `rite -l` equivalent output through the same Executor.ListTasks path
	var listOut, listErr bytes.Buffer
	listExec := task.NewExecutor(
		task.WithDir(dir),
		task.WithStdout(&listOut),
		task.WithStderr(&listErr),
		task.WithVersionCheck(true),
	)
	require.NoError(t, listExec.Setup())
	found, err := listExec.ListTasks(task.NewListOptions(true, false, false, false, false))
	require.NoError(t, err)
	require.True(t, found)

	assert.Equal(t, listOut.String(), bareOut.String(),
		"bare `rite` invocation must produce identical stdout to `rite -l`")
}

// TestBareInvocationFallbackHidesDesclessTasks guards that a Ritefile with no
// default and a mix of desc/no-desc tasks hides the no-desc ones (consistent
// with `rite -l`, which is what we're mimicking).
func TestBareInvocationFallbackHidesDesclessTasks(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/bare_nodefault"),
		task.WithStdout(&stdout),
		task.WithStderr(&stderr),
		task.WithVersionCheck(true),
	)
	require.NoError(t, e.Setup())

	log := &logger.Logger{Stdout: &stdout, Stderr: &stderr}
	handled, err := bareInvocationFallback(e, log)
	require.NoError(t, err)
	assert.True(t, handled)
	// "internal" has no desc — must not appear (matches `rite -l` behavior).
	assert.NotContains(t, stdout.String(), "internal")
	// build and test both have descs — must appear.
	assert.Contains(t, stdout.String(), "build")
	assert.Contains(t, stdout.String(), "test")
}
