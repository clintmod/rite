package task_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	task "github.com/clintmod/rite/internal/task"
	"github.com/clintmod/rite/taskfile/ast"
)

// timestampRe matches the default ISO-8601 UTC ms prefix: `[YYYY-MM-DDTHH:MM:SS.mmmZ]`.
var timestampRe = regexp.MustCompile(`^\[\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z\] `)

// TestTimestampsCLIGlobalStampsCmdAndLogger verifies that the CLI-level
// WithTimestamps(true) stamps both cmd output AND rite's own `rite: …`
// logger lines. This is the load-bearing assertion from #130 — if the
// logger goes untimestamped we shipped half the feature.
func TestTimestampsCLIGlobalStampsCmdAndLogger(t *testing.T) {
	t.Parallel()

	on := true
	var stdout, stderr bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/timestamps"),
		task.WithStdout(&stdout),
		task.WithStderr(&stderr),
		task.WithVerbose(true), // so `rite: [echo] echo hello` actually prints
		task.WithTimestamps(&ast.Timestamps{Enabled: &on}),
	)
	require.NoError(t, e.Setup())
	require.NoError(t, e.Run(t.Context(), &task.Call{Task: "echo"}))

	// Cmd output lands on stdout; rite logger lines land on stderr.
	// Every non-empty line on *both* streams must carry the default
	// ISO-8601 prefix.
	for name, got := range map[string]string{"stdout": stdout.String(), "stderr": stderr.String()} {
		for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
			if line == "" {
				continue
			}
			assert.Regexp(t, timestampRe, line, "%s line not stamped: %q", name, line)
		}
	}
	assert.Contains(t, stdout.String(), "hello")
	// The `rite: [echo] echo hello` preamble has to be on stderr and
	// stamped.
	assert.Regexp(t, timestampRe, firstNonEmptyLine(stderr.String()))
	assert.Contains(t, stderr.String(), "rite: [echo] echo hello")
}

// TestTimestampsCustomStrftime validates that `--timestamps="[%H:%M:%S]"`
// yields a 10-char bracketed HH:MM:SS prefix.
func TestTimestampsCustomStrftime(t *testing.T) {
	t.Parallel()

	on := true
	var stdout, stderr bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/timestamps"),
		task.WithStdout(&stdout),
		task.WithStderr(&stderr),
		task.WithTimestamps(&ast.Timestamps{Enabled: &on, Format: "[%H:%M:%S]"}),
	)
	require.NoError(t, e.Setup())
	require.NoError(t, e.Run(t.Context(), &task.Call{Task: "echo"}))

	re := regexp.MustCompile(`^\[\d{2}:\d{2}:\d{2}\] hello$`)
	line := firstNonEmptyLine(stdout.String())
	assert.Regexp(t, re, line)
}

// TestTimestampsTopLevelPlusTaskOptOut asserts precedence: top-level
// `timestamps: true` stamps by default, but a task with
// `timestamps: false` is exempt from stamping on its cmd output.
func TestTimestampsTopLevelPlusTaskOptOut(t *testing.T) {
	t.Parallel()

	var stdoutOn, stdoutOff bytes.Buffer

	eOn := task.NewExecutor(
		task.WithDir("testdata/timestamps-task-off"),
		task.WithStdout(&stdoutOn),
		task.WithStderr(&bytes.Buffer{}),
	)
	require.NoError(t, eOn.Setup())
	require.NoError(t, eOn.Run(t.Context(), &task.Call{Task: "stamped"}))
	assert.Regexp(t, timestampRe, firstNonEmptyLine(stdoutOn.String()))
	assert.Contains(t, stdoutOn.String(), "stamped-line")

	eOff := task.NewExecutor(
		task.WithDir("testdata/timestamps-task-off"),
		task.WithStdout(&stdoutOff),
		task.WithStderr(&bytes.Buffer{}),
	)
	require.NoError(t, eOff.Setup())
	require.NoError(t, eOff.Run(t.Context(), &task.Call{Task: "plain"}))
	// `plain-line` comes through untouched — no timestamp prefix.
	line := firstNonEmptyLine(stdoutOff.String())
	assert.Equal(t, "plain-line", line)
}

// TestTimestampsCLIOverridesTaskLevel confirms CLI > task precedence: even
// when the task declares `timestamps: false`, a CLI `--timestamps` forces
// stamping onto that task's cmd output.
func TestTimestampsCLIOverridesTaskLevel(t *testing.T) {
	t.Parallel()

	on := true
	var stdout bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/timestamps-task-off"),
		task.WithStdout(&stdout),
		task.WithStderr(&bytes.Buffer{}),
		task.WithTimestamps(&ast.Timestamps{Enabled: &on}),
	)
	require.NoError(t, e.Setup())
	require.NoError(t, e.Run(t.Context(), &task.Call{Task: "plain"}))
	line := firstNonEmptyLine(stdout.String())
	assert.Regexp(t, timestampRe, line)
	assert.Contains(t, line, "plain-line")
}

// TestTimestampsCLIOffDisablesTopLevel pins CLI=false preempting a
// top-level `timestamps: true`.
func TestTimestampsCLIOffDisablesTopLevel(t *testing.T) {
	t.Parallel()

	off := false
	var stdout bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/timestamps-task-off"),
		task.WithStdout(&stdout),
		task.WithStderr(&bytes.Buffer{}),
		task.WithTimestamps(&ast.Timestamps{Enabled: &off}),
	)
	require.NoError(t, e.Setup())
	require.NoError(t, e.Run(t.Context(), &task.Call{Task: "stamped"}))
	line := firstNonEmptyLine(stdout.String())
	assert.Equal(t, "stamped-line", line)
}

// TestTimestampsWithGroupBanner ensures group begin/end banners are
// themselves timestamped — the ticket explicitly calls this out because
// they're rite's own output inside the output pipeline.
func TestTimestampsWithGroupBanner(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/timestamps-group"),
		task.WithStdout(&stdout),
		task.WithStderr(&bytes.Buffer{}),
	)
	require.NoError(t, e.Setup())
	require.NoError(t, e.Run(t.Context(), &task.Call{Task: "grouped"}))

	// All three emitted lines (begin, cmd, end) carry the prefix.
	lines := nonEmptyLines(stdout.String())
	require.Len(t, lines, 3)
	for _, line := range lines {
		assert.Regexp(t, timestampRe, line)
	}
	assert.Contains(t, lines[0], "::group::grouped")
	assert.Contains(t, lines[1], "inner-line")
	assert.Contains(t, lines[2], "::endgroup::")
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			return line
		}
	}
	return ""
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
