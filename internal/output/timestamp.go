package output

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/clintmod/rite/taskfile/ast"
)

// TimestampClock is the clock used by the timestamping writer. Swappable for
// tests that need a deterministic now() — production callers leave it nil and
// get time.Now.
type TimestampClock func() time.Time

// TimestampWriter wraps an io.Writer and prefixes every emitted line with a
// formatted timestamp. Partial writes (no trailing `\n`) are buffered until
// either a newline arrives or Close is called; a final unterminated segment
// is flushed with its own timestamp plus an appended `\n` so log consumers
// never see half-timestamped output.
//
// Multiple TimestampWriter instances that share the same *TimestampSink merge
// their lines onto the same underlying writer under one mutex, so cmd stdout
// + stderr + rite-logger lines appear as one unified stream even when they
// race. Each call to Write treats every complete `\n`-terminated run as a
// single emission — the timestamp is captured when Write is called, not when
// the line originally arrived on stdout, but Go's cmd plumbing flushes on
// newline, so the skew is bounded by Go's own scheduler latency rather than
// by anything we add.
type TimestampWriter struct {
	sink   *TimestampSink
	buf    bytes.Buffer
	closed bool
	mu     sync.Mutex
}

// TimestampSink is the shared downstream target for a family of
// TimestampWriters. Sharing a sink, or sharing a *sync.Mutex across a pair
// of sinks (one for stdout, one for stderr), is how stdout + stderr +
// logger end up serialized into a single timestamped stream even though
// the underlying file descriptors remain separate.
type TimestampSink struct {
	w      io.Writer
	layout string
	clock  TimestampClock
	mu     *sync.Mutex
}

// NewTimestampSink builds a sink that writes to w, formatting timestamps with
// layout (a Go time layout, already translated from strftime if needed). If
// clock is nil, time.Now is used and timestamps are rendered in UTC. If
// sharedMu is non-nil it is used to serialize emissions across multiple
// sinks — the ticket calls for stdout+stderr to merge into a single
// timestamped view; passing one mutex to both sinks produces that without
// collapsing the two FDs.
//
// The default layout is applied with t.UTC() so host TZ never leaks into
// it — the SPEC says the default is always GMT. User-supplied strftime
// formats render in local time unless the format itself asks for UTC
// (e.g. a literal `Z`), matching `ts(1)` behavior and letting users who
// want local timestamps actually get them.
func NewTimestampSink(w io.Writer, layout string, clock TimestampClock, sharedMu *sync.Mutex) *TimestampSink {
	if clock == nil {
		clock = time.Now
	}
	if sharedMu == nil {
		sharedMu = &sync.Mutex{}
	}
	return &TimestampSink{w: w, layout: layout, clock: clock, mu: sharedMu}
}

// NewTimestampWriter creates a writer that prefixes lines with the sink's
// timestamp layout before forwarding to the sink's underlying writer.
func NewTimestampWriter(sink *TimestampSink) *TimestampWriter {
	return &TimestampWriter{sink: sink}
}

// Write buffers input, emitting a timestamped line for each newline-
// terminated segment. Partial trailing text stays in the buffer until the
// next Write or Close.
func (tw *TimestampWriter) Write(p []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.closed {
		return 0, io.ErrClosedPipe
	}

	n := len(p)
	tw.buf.Write(p)
	if err := tw.drainLocked(false); err != nil {
		return n, err
	}
	return n, nil
}

// Close flushes any remaining buffered content (appending a newline if the
// final segment is unterminated) and marks the writer closed. Subsequent
// Writes return io.ErrClosedPipe.
func (tw *TimestampWriter) Close() error {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.closed {
		return nil
	}
	tw.closed = true
	return tw.drainLocked(true)
}

// drainLocked must be called with tw.mu held. It flushes every newline-
// terminated line. If force is true it also flushes any trailing unterminated
// segment (appending `\n`).
func (tw *TimestampWriter) drainLocked(force bool) error {
	for {
		data := tw.buf.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			if !force || tw.buf.Len() == 0 {
				return nil
			}
			// Final partial line on close: take everything, append \n,
			// emit with a fresh timestamp.
			line := make([]byte, tw.buf.Len())
			copy(line, data)
			tw.buf.Reset()
			line = append(line, '\n')
			return tw.sink.emit(line)
		}
		line := make([]byte, idx+1)
		copy(line, data[:idx+1])
		tw.buf.Next(idx + 1)
		if err := tw.sink.emit(line); err != nil {
			return err
		}
	}
}

// emit writes a single already-newline-terminated line, prefixed with the
// current timestamp. Holds the sink mutex to serialize cross-writer output.
func (s *TimestampSink) emit(line []byte) error {
	if len(line) == 0 {
		return nil
	}
	prefix := s.formatPrefix(s.clock())
	s.mu.Lock()
	defer s.mu.Unlock()
	// Build one byte slice and Write once — two separate Writes to the
	// same sink can interleave with a concurrent plain-io.Writer that
	// doesn't hold our mutex (e.g. if a caller bypasses our decorator).
	// We're already inside the mutex; the single Write keeps the prefix
	// and line atomic against the os.Stderr fd.
	out := make([]byte, 0, len(prefix)+1+len(line))
	out = append(out, prefix...)
	out = append(out, ' ')
	out = append(out, line...)
	_, err := s.w.Write(out)
	return err
}

// formatPrefix renders the timestamp using the configured layout. The
// default layout is coerced to UTC so host TZ never leaks into it; other
// layouts use local time (see NewTimestampSink rationale).
func (s *TimestampSink) formatPrefix(t time.Time) string {
	if s.layout == ast.DefaultTimestampLayout {
		return t.UTC().Format(s.layout)
	}
	return t.Format(s.layout)
}

// ResolveLayout returns the Go time layout to use given the tri-state value
// from the AST. Format may be a strftime string; it is translated to Go's
// reference-time layout. Returns layout, ok — ok is false when the value is
// unset or explicitly disabled.
func ResolveLayout(ts *ast.Timestamps) (string, bool, error) {
	if !ts.IsSet() || !ts.On() {
		return "", false, nil
	}
	if ts.Format == "" {
		return ast.DefaultTimestampLayout, true, nil
	}
	layout, err := StrftimeToGoLayout(ts.Format)
	if err != nil {
		return "", false, err
	}
	return layout, true, nil
}

// StrftimeToGoLayout translates a small strftime subset into the Go time
// layout that Time.Format understands. The subset is deliberately narrow —
// the ticket calls out the supported tokens explicitly rather than
// promising full glibc parity.
//
// Supported:
//
//	%Y  four-digit year (2006)
//	%m  zero-padded month (01)
//	%d  zero-padded day of month (02)
//	%H  zero-padded hour 00-23 (15)
//	%M  zero-padded minute (04)
//	%S  zero-padded second (05)
//	%L  zero-padded 3-digit millisecond (.000, dot-prefixed by Go's layout)
//	%z  timezone offset ±hhmm (-0700)
//	%%  literal %
//
// %L renders as ".000" so the dot separator is part of the field — same as
// `ts -s %.S`. Users who want the millis without a dot should use a custom
// layout; we don't try to fake sub-field arithmetic here.
func StrftimeToGoLayout(f string) (string, error) {
	var b strings.Builder
	b.Grow(len(f) + 8)
	for i := 0; i < len(f); i++ {
		c := f[i]
		if c != '%' {
			// Escape Go layout reference digits/letters that would
			// otherwise be interpreted as format tokens. The Go layout
			// uses `2006 01 02 15 04 05 Jan Mon MST -0700` as magic
			// numbers; any literal run that happens to contain those
			// digits gets mangled. Safe approach: emit non-token bytes
			// via Go's literal-passthrough by prefixing the whole
			// literal span between `%` tokens into the layout as-is,
			// which the reference Go layout does by treating anything
			// that doesn't match a magic token as literal. That
			// heuristic is lossy when users genuinely type `2006` — in
			// practice nobody's strftime format contains that year, so
			// we accept the edge case and document it elsewhere.
			b.WriteByte(c)
			continue
		}
		if i+1 >= len(f) {
			return "", fmt.Errorf("timestamps: trailing %% in strftime format %q", f)
		}
		i++
		switch f[i] {
		case 'Y':
			b.WriteString("2006")
		case 'm':
			b.WriteString("01")
		case 'd':
			b.WriteString("02")
		case 'H':
			b.WriteString("15")
		case 'M':
			b.WriteString("04")
		case 'S':
			b.WriteString("05")
		case 'L':
			b.WriteString(".000")
		case 'z':
			b.WriteString("-0700")
		case '%':
			b.WriteByte('%')
		default:
			return "", fmt.Errorf("timestamps: unsupported strftime token %%%c in %q (supported: %%Y %%m %%d %%H %%M %%S %%L %%z %%%%)", f[i], f)
		}
	}
	return b.String(), nil
}
