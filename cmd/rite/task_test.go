package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/clintmod/rite/internal/logger"
	task "github.com/clintmod/rite/internal/task"
)

// TestBareInvocationFallback covers the #102 fallback: when `rite` is invoked
// with no positional task, a missing `default:` entry should surface the task
// list (silent) instead of erroring. An entirely-empty Ritefile should print
// a friendly hint.
func TestBareInvocationFallback(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name               string
		dir                string
		wantHandled        bool
		wantStdoutLines    []string
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
			name:            "no default task — list silent",
			dir:             "testdata/bare_nodefault",
			wantHandled:     true,
			wantStdoutLines: []string{"build", "test"},
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
			handled, err := bareInvocationFallback(e, log, false)
			require.NoError(t, err)
			assert.Equal(t, tc.wantHandled, handled)

			if tc.wantStdoutEmpty {
				assert.Empty(t, stdout.String())
			}
			for _, line := range tc.wantStdoutLines {
				assert.Contains(t, stdout.String(), line)
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

// TestBareInvocationFallbackDoesNotListInternal guards that a Ritefile with
// no default and only non-desc tasks behaves consistently with `rite -l -s`
// — i.e. prints nothing (not an error) when `allTasks=false`. #102's test
// plan calls this out implicitly: silent-list semantics are preserved.
func TestBareInvocationFallbackHidesDesclessWhenNotAll(t *testing.T) {
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
	handled, err := bareInvocationFallback(e, log, false)
	require.NoError(t, err)
	assert.True(t, handled)
	// "internal" has no desc and allTasks=false — must not appear.
	assert.NotContains(t, stdout.String(), "internal")
	// build and test both have descs — must appear.
	names := strings.Fields(stdout.String())
	assert.Contains(t, names, "build")
	assert.Contains(t, names, "test")
}
