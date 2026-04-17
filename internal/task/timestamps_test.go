package task_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/fatih/color"
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

// TestTimestampsMarkerInjectedWhenWrapping verifies that when rite wraps a
// cmd's output with a TimestampWriter, it also sets
// RITE_TIMESTAMPS_HANDLED=1 in that cmd's environ. A nested rite process
// that sees this marker suppresses its own wrapping (issue #136).
func TestTimestampsMarkerInjectedWhenWrapping(t *testing.T) {
	t.Parallel()

	on := true
	var stdout bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/timestamps-marker"),
		task.WithStdout(&stdout),
		task.WithStderr(&bytes.Buffer{}),
		task.WithTimestamps(&ast.Timestamps{Enabled: &on}),
	)
	require.NoError(t, e.Setup())
	require.NoError(t, e.Run(t.Context(), &task.Call{Task: "probe"}))

	// Cmd saw the marker set to 1. Strip the timestamp prefix first; the
	// payload itself must match.
	line := firstNonEmptyLine(stdout.String())
	stripped := timestampRe.ReplaceAllString(line, "")
	assert.Equal(t, "marker=1", stripped)
}

// TestTimestampsMarkerAbsentWhenUnwrapped confirms the inverse: with
// timestamps off at every scope, no marker leaks into the cmd environ.
// Non-rite children would never read it, but we don't want stray rite
// config bleeding into processes whose parent didn't opt into stamping.
func TestTimestampsMarkerAbsentWhenUnwrapped(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/timestamps-marker"),
		task.WithStdout(&stdout),
		task.WithStderr(&bytes.Buffer{}),
	)
	require.NoError(t, e.Setup())
	require.NoError(t, e.Run(t.Context(), &task.Call{Task: "probe"}))

	// No wrapping, so no timestamp prefix and no marker.
	assert.Equal(t, "marker=\n", stdout.String())
}

// TestTimestampsMarkerSuppressesInnerWrapping is the load-bearing
// regression for #136. With the marker already set in rite's own environ
// (simulating an outer rite that wrapped us), a CLI `--timestamps` on the
// inner invocation is deliberately suppressed — otherwise the outer
// already-prefixed line would get a second prefix, giving `[ts] [ts] foo`.
// The outer rite's single wrap becomes the only source of timestamps.
// This is the intended "marker beats explicit user request at the inner
// level" semantics from the issue; it is not a bug.
func TestTimestampsMarkerSuppressesInnerWrapping(t *testing.T) {
	t.Setenv(ast.TimestampMarkerEnvVar, "1")

	on := true
	var stdout bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/timestamps"),
		task.WithStdout(&stdout),
		task.WithStderr(&bytes.Buffer{}),
		task.WithTimestamps(&ast.Timestamps{Enabled: &on}),
	)
	require.NoError(t, e.Setup())
	require.NoError(t, e.Run(t.Context(), &task.Call{Task: "echo"}))

	// Output must NOT carry a timestamp prefix — the inner wrap was
	// suppressed by the marker.
	line := firstNonEmptyLine(stdout.String())
	assert.Equal(t, "hello", line)
	assert.NotRegexp(t, timestampRe, line)
}

// TestTimestampsInternalSubcallStillStampsOnce pins the invariant that the
// marker-based fix doesn't regress the existing in-process `cmds: [task:
// foo]` path. These subcalls never fork a rite process and so never route
// through the marker; each line still gets exactly one timestamp.
func TestTimestampsInternalSubcallStillStampsOnce(t *testing.T) {
	t.Parallel()

	on := true
	var stdout bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/timestamps-nested"),
		task.WithStdout(&stdout),
		task.WithStderr(&bytes.Buffer{}),
		task.WithTimestamps(&ast.Timestamps{Enabled: &on}),
	)
	require.NoError(t, e.Setup())
	require.NoError(t, e.Run(t.Context(), &task.Call{Task: "outer"}))

	lines := nonEmptyLines(stdout.String())
	require.Len(t, lines, 2)
	// Exactly one timestamp prefix per line — never two. Easiest check:
	// the regex `[ts] [ts]` must not appear.
	doublePrefix := regexp.MustCompile(`^\[\d{4}-[^]]+\]\s+\[\d{4}-`)
	for _, line := range lines {
		assert.Regexp(t, timestampRe, line)
		assert.NotRegexp(t, doublePrefix, line, "double-prefixed line: %q", line)
	}
	assert.Contains(t, lines[0], "alpha")
	assert.Contains(t, lines[1], "beta")
}

// TestTimestampsListTasksIsNeverStamped captures the post-#148 rule: the
// `rite -l` listing is CLI metadata — on the same tier as `--help` /
// `--version` — and must NOT carry the global run-time timestamp even under
// top-level `timestamps: true`.
//
// History:
//
//  1. #145 reported that with `timestamps: true`, only the header line of
//     `rite -l` was stamped; every task row bypassed the TimestampWriter
//     because the tabwriter was pointed at `e.Stdout` (unwrapped) while the
//     header went through `e.Logger.Outf` (wrapped). The v1.0.7 fix routed
//     both through `e.Logger.Stdout` so every line came out stamped.
//
//  2. #148 flagged that fix: `rite -l` output is metadata, not task
//     output. Stamping *any* of it is wrong. The real fix (this test)
//     routes both header AND rows through the Logger's pre-wrap
//     (un-stamped) writer, so the listing is clean under any global
//     timestamp setting.
//
// This test asserts BOTH halves:
//   - No line in the listing carries a timestamp prefix (the #148 invariant).
//   - Colored ANSI bytes still round-trip into the output buffer (the
//     load-bearing part of the #145 invariant that we MUST not lose: the
//     header and rows share one writer, so fatih/color's buffered reset
//     never lands on a side channel).
func TestTimestampsListTasksIsNeverStamped(t *testing.T) {
	// Not t.Parallel: fatih/color reads BOTH the package-level `NoColor`
	// flag AND the `NO_COLOR` env var at every `color.New()` call (via its
	// `noColorIsSet()` helper), and the task-test harness sets
	// `NO_COLOR=1` at package init (see `task_test.go`) to stabilize
	// golden fixtures. We have to clear `NO_COLOR` and flip `NoColor` back
	// on for the duration of the test, then restore. Parallel tests that
	// rely on the default (no-color) state would race with us.
	t.Setenv("NO_COLOR", "")
	prevNoColor := color.NoColor
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = prevNoColor })

	var stdout bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/timestamps-list"),
		task.WithStdout(&stdout),
		task.WithStderr(&bytes.Buffer{}),
		task.WithColor(true),
	)
	require.NoError(t, e.Setup())

	found, err := e.ListTasks(task.ListOptions{ListOnlyTasksWithDescriptions: true})
	require.NoError(t, err)
	require.True(t, found)

	// #148 invariant: no line in the listing carries a timestamp prefix,
	// even though the fixture sets `timestamps: true` at the top level.
	lines := nonEmptyLines(stdout.String())
	require.GreaterOrEqual(t, len(lines), 3, "expected header + 2 task rows, got %d lines: %q", len(lines), stdout.String())
	for i, line := range lines {
		assert.NotRegexp(t, timestampRe, line, "line %d stamped under timestamps:true — `rite -l` is metadata, not task output: %q", i, line)
	}
	// Header content sanity — still the same text, just un-stamped.
	assert.Contains(t, lines[0], "rite: Available tasks for this project:")
	// Task rows still present.
	joined := strings.Join(lines[1:], "\n")
	assert.Contains(t, joined, "hello")
	assert.Contains(t, joined, "world")

	// #145 ANSI round-trip invariant we cannot lose: colored bytes must
	// appear in the output. The bullet is yellow (`\x1b[33m`), the task
	// name is green (`\x1b[32m`), and fatih/color's reset (`\x1b[0m`)
	// must land in-order alongside them. If the header and rows drifted
	// across two writers, fatih/color's buffered reset at end-of-previous-
	// write would get orphaned; asserting all three bytes on the task
	// rows catches the regression without re-requiring a stamp.
	for _, line := range lines[1:] {
		assert.Contains(t, line, "\x1b[33m", "task-row line missing yellow bullet ANSI — split-writer regression? %q", line)
		assert.Contains(t, line, "\x1b[32m", "task-row line missing green task-name ANSI: %q", line)
		assert.Contains(t, line, "\x1b[0m", "task-row line missing reset ANSI: %q", line)
	}
}

// TestTimestampsListTasksNoStampingWhenTimestampsOff is the
// timestamps-disabled control case: `rite -l` must also come out un-stamped
// when the Ritefile doesn't set `timestamps:` at all. This is the trivial
// path (there's no wrapper to bypass), but pinning it down guards against
// a future refactor routing the list output through some always-stamping
// sink.
func TestTimestampsListTasksNoStampingWhenTimestampsOff(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	e := task.NewExecutor(
		task.WithDir("testdata/timestamps-list-off"),
		task.WithStdout(&stdout),
		task.WithStderr(&bytes.Buffer{}),
	)
	require.NoError(t, e.Setup())

	found, err := e.ListTasks(task.ListOptions{ListOnlyTasksWithDescriptions: true})
	require.NoError(t, err)
	require.True(t, found)

	lines := nonEmptyLines(stdout.String())
	require.GreaterOrEqual(t, len(lines), 2)
	for i, line := range lines {
		assert.NotRegexp(t, timestampRe, line, "line %d unexpectedly stamped (timestamps off): %q", i, line)
	}
	assert.Contains(t, lines[0], "rite: Available tasks for this project:")
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
