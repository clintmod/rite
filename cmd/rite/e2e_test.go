package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// End-to-end tests: spawn the real `rite` binary as a child process against
// fixture Ritefiles on disk, then assert on stdout/stderr bytes.
//
// Why this harness exists: a string of 1.0.x bugs — #145, #148, #151, #153,
// #154 — all shipped despite green in-process unit tests. Each one only
// reproduced under the real binary-as-child-process boundary (pipe vs TTY,
// env inheritance, color detection, timestamp stream routing). This harness
// closes that gap by building the `rite` binary once in TestMain and running
// `exec.Command` against it per scenario.
//
// Invariants the harness enforces that the in-process tests cannot:
//   - Color state is forced via FORCE_COLOR=1 / CLICOLOR_FORCE=1 in the
//     child env (never via TTY detection — that varies across CI / OS / shell
//     and is where #153 hid).
//   - Nested rite invocations resolve through PATH, exactly like a user's
//     shell would. The test's build dir is prepended to PATH so the outer
//     rite finds the same binary.
//   - Assertions check literal ANSI SGR sequences and the timestamp regex,
//     not parsed / color-stripped output.

// riteBin is the absolute path to the freshly-built `rite` binary shared by
// every e2e test. Populated by TestMain before any test runs; the temp
// directory and its contents are cleaned up on exit.
var riteBin string

// timestampRx matches the default rite timestamp prefix
// (ast.DefaultTimestampLayout = "[2006-01-02T15:04:05.000Z]"). The trailing
// space is part of the on-wire prefix; we include it so we don't false-match
// a Ritefile that happens to contain "[2026-...]" in echoed text.
var timestampRx = regexp.MustCompile(`\[\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z\] `)

// ANSI SGR sequences the logger emits. Checked as literal substrings.
const (
	ansiYellow = "\x1b[33m" // logger.Yellow → bullet "* "
	ansiGreen  = "\x1b[32m" // logger.Green → task name, verbose "rite: [task] cmd"
	ansiReset  = "\x1b[0m"  // logger.Default / color.Reset
)

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "rite-e2e-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: mktemp: %v\n", err)
		os.Exit(1)
	}
	binary := "rite"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	riteBin = filepath.Join(tmp, binary)

	// Build the binary from the cmd/rite package. `go test` sets the cwd to
	// the package dir, so `.` resolves to cmd/rite. 5-minute cap is generous
	// — cold builds on CI have been observed at ~30s; the ceiling exists so
	// a stuck build doesn't wedge the suite.
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	build := exec.CommandContext(buildCtx, "go", "build", "-o", riteBin, ".")
	build.Stderr = os.Stderr
	build.Stdout = os.Stderr
	if err := build.Run(); err != nil {
		buildCancel()
		fmt.Fprintf(os.Stderr, "e2e: go build: %v\n", err)
		_ = os.RemoveAll(tmp)
		os.Exit(1)
	}
	buildCancel()

	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

// runRite runs the pre-built `rite` binary with the given args and working
// dir. The child env is the current process env plus whatever the caller
// supplied in `extraEnv` (later entries win over earlier ones), and with the
// build directory prepended to PATH so nested rite invocations resolve the
// same binary the harness built.
//
// Line-ending normalization: Windows processes may emit "\r\n"; we fold it
// to "\n" so substring assertions stay portable. The caller-visible
// contract is "text as the user would read it"; if a test actually cares
// about a literal CR it can re-introduce one locally.
func runRite(t *testing.T, dir string, extraEnv []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	absDir, err := filepath.Abs(dir)
	require.NoError(t, err, "resolve fixture dir")

	// 60s is a paranoid ceiling: every scenario runs under a second locally;
	// the cap just prevents one accidentally-hung child from freezing CI for
	// the full test-pkg timeout. Plumbed through t.Context() so `go test
	// -timeout` still wins cleanly.
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, riteBin, args...)
	cmd.Dir = absDir

	// Start from the test's own environ so HOME/TEMP/etc. survive, then
	// prepend the build dir to PATH and append caller overrides. exec.Command
	// inherits os.Environ() by default, but we construct the slice explicitly
	// so a nested invocation can find the same rite binary.
	env := append([]string{}, os.Environ()...)
	env = append(env, "PATH="+filepath.Dir(riteBin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	env = append(env, extraEnv...)
	cmd.Env = env

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			t.Fatalf("rite %v: spawn failed: %v", args, err)
		}
	}

	stdout = strings.ReplaceAll(outBuf.String(), "\r\n", "\n")
	stderr = strings.ReplaceAll(errBuf.String(), "\r\n", "\n")
	return stdout, stderr, exitCode
}

// forceColorEnv returns the env overrides that make rite (and its children)
// treat the child-stdout pipe as color-capable. We always set both — FORCE_COLOR
// is the npm-ish convention (also honored by fatih/color); CLICOLOR_FORCE is
// the BSD convention rite itself forwards to grandchildren after #153.
func forceColorEnv() []string {
	return []string{
		"FORCE_COLOR=1",
		"CLICOLOR_FORCE=1",
		// Defensive: some CI runners set NO_COLOR for their own convenience;
		// unset it in the child so our forcing actually takes effect.
		"NO_COLOR=",
	}
}

// TestE2E_ListTasks_NotStamped_ColorsIntact covers #145 (split-writer ANSI
// reset loss on the `rite -l` header) and #148 (the regression that followed
// #145: metadata lines shouldn't carry run-time timestamps at all). Under
// `timestamps: true` + FORCE_COLOR=1, the task list must:
//   - contain no timestamp prefix on any line, AND
//   - still carry the yellow bullet (\x1b[33m), green task-name (\x1b[32m),
//     and reset (\x1b[0m) SGRs.
func TestE2E_ListTasks_NotStamped_ColorsIntact(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := runRite(t, "testdata/e2e/list_timestamps_on", forceColorEnv(), "-l")
	require.Equalf(t, 0, code, "rite -l exited non-zero\nstdout: %q\nstderr: %q", stdout, stderr)

	for i, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		if m := timestampRx.FindString(line); m != "" {
			t.Errorf("line %d has timestamp prefix %q (should be un-stamped meta-output):\n%s", i, m, stdout)
		}
	}

	// Color sequences: bullet + name + reset. Assert each substring survives.
	assert.Containsf(t, stdout, ansiYellow, "missing yellow SGR (\\x1b[33m) on bullet; stdout=%q", stdout)
	assert.Containsf(t, stdout, ansiGreen, "missing green SGR (\\x1b[32m) on task name; stdout=%q", stdout)
	assert.Containsf(t, stdout, ansiReset, "missing reset SGR (\\x1b[0m); stdout=%q", stdout)
}

// TestE2E_TaskRun_ColorResetNotLost covers #151: when a task runs under
// `timestamps: true` with a colored logger header ("rite: [hello] echo hi"
// in verbose mode), the trailing reset (\x1b[0m) after the green header must
// not be swallowed by the TimestampSink's SGR-coalescing path. We assert the
// reset appears AFTER the green that opened it — ordering matters because a
// reset that migrated to the start of the stream would defeat the point.
func TestE2E_TaskRun_ColorResetNotLost(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := runRite(t, "testdata/e2e/task_run_timestamps_on", forceColorEnv(), "-v", "hello")
	require.Equalf(t, 0, code, "rite -v hello exited non-zero\nstdout: %q\nstderr: %q", stdout, stderr)

	// The `rite: [hello] echo hi` verbose header is written to stderr; the
	// green-and-reset wrapping is what we're guarding. Check the combined
	// stream so we don't couple the test to which handle the header went to
	// (a fix could move it and the assertion should still hold).
	combined := stdout + stderr
	greenIdx := strings.Index(combined, ansiGreen)
	require.NotEqualf(t, -1, greenIdx, "expected green SGR in output; combined=%q", combined)

	resetAfter := strings.Index(combined[greenIdx:], ansiReset)
	require.NotEqualf(t, -1, resetAfter, "expected reset SGR after green; combined=%q", combined)
}

// TestE2E_NestedRite_ColorsSurvivePipe covers #153: when a cmd spawns a
// color-aware child (here, `rite -l`), the child's stdout is a pipe to the
// outer rite — modern CLIs would strip color by default. #153 fixed this by
// forwarding CLICOLOR_FORCE=1 / FORCE_COLOR=1 to cmd children whenever the
// outer rite has color on. End-to-end proof: run the outer rite with
// FORCE_COLOR=1 and assert ANSI bytes show up in the captured stdout, which
// means they survived child → pipe → outer's TimestampWriter → test.
func TestE2E_NestedRite_ColorsSurvivePipe(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := runRite(t, "testdata/e2e/nested_rite", forceColorEnv())
	require.Equalf(t, 0, code, "nested rite exited non-zero\nstdout: %q\nstderr: %q", stdout, stderr)

	// Any SGR sequence is enough — the #153 regression is "all color gets
	// stripped." We still sanity-check the specific ones the list-formatter
	// emits so a future change that silently degrades one color flavor is
	// still noticed.
	assert.Containsf(t, stdout, "\x1b[", "no ANSI SGR bytes at all in nested rite output; stdout=%q", stdout)
	assert.Containsf(t, stdout, ansiYellow, "missing yellow SGR in nested rite output; stdout=%q", stdout)
	assert.Containsf(t, stdout, ansiGreen, "missing green SGR in nested rite output; stdout=%q", stdout)
}

// TestE2E_BareRite_NoDefault_MatchesListFlag covers #154: `rite` (bare) on a
// Ritefile with no `default:` task must produce output byte-for-byte
// equivalent to `rite -l` on the same Ritefile. Any drift = parallel
// formatter implementations, which is exactly what caused the bug. This is
// the strict equality test; no substring / regex slack.
func TestE2E_BareRite_NoDefault_MatchesListFlag(t *testing.T) {
	t.Parallel()

	dir := "testdata/e2e/bare_nodefault"

	// Both invocations use the same env so rite's TTY-dependent output
	// decisions (color, terminal width) cannot diverge.
	env := forceColorEnv()

	listOut, listErr, listCode := runRite(t, dir, env, "-l")
	require.Equalf(t, 0, listCode, "rite -l exited non-zero\nstdout: %q\nstderr: %q", listOut, listErr)

	bareOut, bareErr, bareCode := runRite(t, dir, env)
	require.Equalf(t, 0, bareCode, "bare rite exited non-zero\nstdout: %q\nstderr: %q", bareOut, bareErr)

	assert.Equalf(t, listOut, bareOut,
		"bare rite stdout must be byte-for-byte equal to `rite -l` stdout\nlist=%q\nbare=%q",
		listOut, bareOut)
}

// TestE2E_ListTasks_TimestampsOff_Baseline is the control for
// TestE2E_ListTasks_NotStamped_ColorsIntact: timestamps explicitly off,
// FORCE_COLOR on. Everything the "timestamps on" case asserts should still
// hold — this pins the baseline so a regression that silently disables
// color on the baseline case (or silently starts stamping meta-output) is
// still caught.
func TestE2E_ListTasks_TimestampsOff_Baseline(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := runRite(t, "testdata/e2e/list_timestamps_off", forceColorEnv(), "-l")
	require.Equalf(t, 0, code, "rite -l exited non-zero\nstdout: %q\nstderr: %q", stdout, stderr)

	assert.NotRegexpf(t, timestampRx, stdout, "timestamps off: no prefix expected; stdout=%q", stdout)
	assert.Containsf(t, stdout, ansiYellow, "missing yellow SGR; stdout=%q", stdout)
	assert.Containsf(t, stdout, ansiGreen, "missing green SGR; stdout=%q", stdout)
	assert.Containsf(t, stdout, ansiReset, "missing reset SGR; stdout=%q", stdout)
}

// TestE2E_BareRite_DefaultTaskWins is the symmetric control for #154: when
// the Ritefile DOES define `default:`, a bare `rite` invocation must run
// that task and MUST NOT fall through to the list formatter. Guards against
// an over-eager fix that swaps the whole bare-invocation path for `-l`
// unconditionally.
func TestE2E_BareRite_DefaultTaskWins(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := runRite(t, "testdata/e2e/bare_default", forceColorEnv())
	require.Equalf(t, 0, code, "bare rite exited non-zero\nstdout: %q\nstderr: %q", stdout, stderr)

	assert.Containsf(t, stdout, "default-task-ran",
		"default task output missing; stdout=%q stderr=%q", stdout, stderr)
	assert.NotContainsf(t, stdout, "Available tasks for this project:",
		"list header leaked into bare-with-default output; stdout=%q", stdout)
}
