package output_test

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/clintmod/rite/internal/output"
	"github.com/clintmod/rite/taskfile/ast"
)

// safeBuffer is a mutex-wrapped bytes.Buffer so concurrent sink writes don't
// race on the backing storage itself — we're testing the sink's serialization
// of timestamp-order, not bytes.Buffer's (lack of) concurrency safety.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// fixedClock returns the same instant every call. Useful so golden-style
// tests can assert exact prefix strings.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestTimestampWriterDefaultLayoutIsUTC(t *testing.T) {
	t.Parallel()

	// Non-UTC input time — if the writer leaks host TZ the rendered hour
	// will not be 14.
	in := time.Date(2026, 4, 15, 21, 23, 1, 123_000_000, time.FixedZone("PDT", -7*3600))
	var b bytes.Buffer
	sink := output.NewTimestampSink(&b, ast.DefaultTimestampLayout, fixedClock(in), nil)
	w := output.NewTimestampWriter(sink)

	_, err := fmt.Fprintln(w, "hello")
	require.NoError(t, err)

	require.NoError(t, w.Close())
	assert.Equal(t, "[2026-04-16T04:23:01.123Z] hello\n", b.String())
}

func TestTimestampWriterMillisecondsAreZeroPadded(t *testing.T) {
	t.Parallel()

	// 5ms after the second — the ticket demands `.005`, not `.5`.
	in := time.Date(2026, 4, 15, 14, 23, 1, 5_000_000, time.UTC)
	var b bytes.Buffer
	sink := output.NewTimestampSink(&b, ast.DefaultTimestampLayout, fixedClock(in), nil)
	w := output.NewTimestampWriter(sink)

	_, _ = fmt.Fprintln(w, "x")
	require.NoError(t, w.Close())
	assert.Contains(t, b.String(), "[2026-04-15T14:23:01.005Z]")
}

func TestTimestampWriterPartialLineFlushOnClose(t *testing.T) {
	t.Parallel()

	in := time.Date(2026, 4, 15, 14, 23, 1, 0, time.UTC)
	var b bytes.Buffer
	sink := output.NewTimestampSink(&b, ast.DefaultTimestampLayout, fixedClock(in), nil)
	w := output.NewTimestampWriter(sink)

	// Feed characters one at a time without a trailing newline.
	for _, c := range "partial" {
		_, _ = fmt.Fprintf(w, "%c", c)
	}
	// Nothing should have been emitted yet (the writer must buffer until
	// newline).
	assert.Empty(t, b.String())

	require.NoError(t, w.Close())
	// Close must flush the partial with a final timestamp + synthesized newline.
	assert.Equal(t, "[2026-04-15T14:23:01.000Z] partial\n", b.String())
}

func TestTimestampWriterMultiLineWrite(t *testing.T) {
	t.Parallel()

	in := time.Date(2026, 4, 15, 14, 23, 1, 0, time.UTC)
	var b bytes.Buffer
	sink := output.NewTimestampSink(&b, ast.DefaultTimestampLayout, fixedClock(in), nil)
	w := output.NewTimestampWriter(sink)

	_, _ = io.WriteString(w, "one\ntwo\nthree\n")
	require.NoError(t, w.Close())

	got := b.String()
	lines := strings.SplitN(got, "\n", 4)
	require.Len(t, lines, 4)
	// Every non-empty line starts with the expected prefix.
	for _, line := range lines[:3] {
		assert.True(t, strings.HasPrefix(line, "[2026-04-15T14:23:01.000Z] "), line)
	}
}

func TestTimestampWriterPreservesANSIColor(t *testing.T) {
	t.Parallel()

	in := time.Date(2026, 4, 15, 14, 23, 1, 0, time.UTC)
	var b bytes.Buffer
	sink := output.NewTimestampSink(&b, ast.DefaultTimestampLayout, fixedClock(in), nil)
	w := output.NewTimestampWriter(sink)

	// ESC[31m red foo ESC[0m reset, followed by newline.
	colored := "\x1b[31mfoo\x1b[0m\n"
	_, _ = io.WriteString(w, colored)
	require.NoError(t, w.Close())

	// The prefix goes before the first byte; the escape sequence must
	// survive intact in its original order.
	assert.Equal(t, "[2026-04-15T14:23:01.000Z] \x1b[31mfoo\x1b[0m\n", b.String())
}

func TestTimestampSinkSerializesAcrossWriters(t *testing.T) {
	t.Parallel()

	in := time.Date(2026, 4, 15, 14, 23, 1, 0, time.UTC)
	var b bytes.Buffer
	mu := &sync.Mutex{}
	outSink := output.NewTimestampSink(&b, ast.DefaultTimestampLayout, fixedClock(in), mu)
	errSink := output.NewTimestampSink(&b, ast.DefaultTimestampLayout, fixedClock(in), mu)
	stdout := output.NewTimestampWriter(outSink)
	stderr := output.NewTimestampWriter(errSink)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			_, _ = fmt.Fprintf(stdout, "out-%d\n", i)
		}(i)
		go func(i int) {
			defer wg.Done()
			_, _ = fmt.Fprintf(stderr, "err-%d\n", i)
		}(i)
	}
	wg.Wait()
	require.NoError(t, stdout.Close())
	require.NoError(t, stderr.Close())

	// Every line should be atomic: "[prefix] <token>\n". No interleavings.
	re := regexp.MustCompile(`^\[2026-04-15T14:23:01\.000Z\] (out|err)-\d+$`)
	for _, line := range strings.Split(strings.TrimRight(b.String(), "\n"), "\n") {
		assert.Regexp(t, re, line)
	}
}

// TestTimestampSinkConcurrentWritesProduceMonotonicTimestamps locks in the
// invariant that sample-clock order matches emission order. If clock() is
// sampled outside the sink mutex, two goroutines can pick timestamps T1 < T2
// then race for the lock in the opposite order, and the log ends up with
// timestamps going backwards. The fake clock yields mid-call to maximize the
// interleaving window so a regression is caught reliably rather than
// probabilistically.
func TestTimestampSinkConcurrentWritesProduceMonotonicTimestamps(t *testing.T) {
	t.Parallel()

	var counter atomic.Int64
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		n := counter.Add(1)
		// Give the scheduler a chance to pick a racing goroutine between
		// clock-sample and mutex-acquire — this is the window where the
		// bug manifests.
		runtime.Gosched()
		return base.Add(time.Duration(n) * time.Microsecond)
	}

	// Microsecond resolution layout so counter.Add(1) increments show up
	// in the rendered prefix. The default layout is millisecond, which
	// would produce ties and mask the bug.
	const layout = "20060102T150405.000000Z"

	var buf safeBuffer
	sink := output.NewTimestampSink(&buf, layout, clock, nil)

	const N = 200
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := output.NewTimestampWriter(sink)
			_, _ = fmt.Fprintf(w, "line-%03d\n", i)
			_ = w.Close()
		}(i)
	}
	wg.Wait()

	// Assert rendered timestamps are non-decreasing across every emitted line.
	var prev time.Time
	for _, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		sp := strings.Index(line, " ")
		require.GreaterOrEqual(t, sp, 0, "line missing space separator: %q", line)
		ts, err := time.Parse(layout, line[:sp])
		require.NoError(t, err, "parse %q", line[:sp])
		if ts.Before(prev) {
			t.Fatalf("non-monotonic: %v after %v\nfull log:\n%s", ts, prev, buf.String())
		}
		prev = ts
	}
}

func TestStrftimeToGoLayoutCoverage(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"%Y-%m-%dT%H:%M:%S": "2006-01-02T15:04:05",
		"[%H:%M:%S%L]":      "[15:04:05.000]",
		"%Y %z":             "2006 -0700",
		"%%literal%%":       "%literal%",
		"plain-prefix-%Y":   "plain-prefix-2006",
	}
	for in, want := range cases {
		got, err := output.StrftimeToGoLayout(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, "input=%q", in)
	}
}

func TestStrftimeToGoLayoutRejectsBadTokens(t *testing.T) {
	t.Parallel()
	_, err := output.StrftimeToGoLayout("%Q")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported strftime token")

	_, err = output.StrftimeToGoLayout("trailing-%")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trailing")
}

func TestResolveLayoutTristate(t *testing.T) {
	t.Parallel()

	// Unset → off.
	layout, ok, err := output.ResolveLayout(nil)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, layout)

	// Explicit false → off.
	off := false
	layout, ok, err = output.ResolveLayout(&ast.Timestamps{Enabled: &off})
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, layout)

	// Explicit true with no format → default.
	on := true
	layout, ok, err = output.ResolveLayout(&ast.Timestamps{Enabled: &on})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, ast.DefaultTimestampLayout, layout)

	// Custom strftime → translated.
	layout, ok, err = output.ResolveLayout(&ast.Timestamps{Enabled: &on, Format: "[%H:%M:%S]"})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "[15:04:05]", layout)
}
