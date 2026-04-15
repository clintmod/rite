package task

import (
	"io"
	"sync"

	"github.com/clintmod/rite/internal/output"
	"github.com/clintmod/rite/taskfile/ast"
)

// timestampContext carries the timestamp wiring for one Executor run. A nil
// *timestampContext means timestamps are off globally; per-task decoration
// is still possible via wrapCmdWriters() falling through to the original
// writers.
type timestampContext struct {
	// globalLayout is the resolved Go time layout when the CLI or top-level
	// scope turned timestamps on. Empty when neither did.
	globalLayout string
	// cliLayout is the resolved layout when --timestamps was passed (or
	// RITE_TIMESTAMPS was set). Empty when not set. Precedence rule: if
	// cliLayout is non-empty it overrides task-level Timestamps entirely.
	cliLayout string
	// cliSet is true when the CLI gave any value (including explicit false)
	// so per-task values are preempted.
	cliSet bool
	// cliOff is true when the CLI said "--timestamps=false" / "RITE_TIMESTAMPS=0".
	cliOff bool
	// topLayout is the resolved layout for the top-level Ritefile
	// `timestamps:` key. Empty when unset or disabled.
	topLayout string
	// sharedMu serializes writes across the stdout/stderr sinks so stdout
	// and stderr lines interleave cleanly but keep their own file
	// descriptors — the ticket's "merge the timestamped view, keep FDs
	// separate" rule.
	sharedMu *sync.Mutex
}

// buildTimestampContext resolves the Executor's CLI/top-level timestamp
// configuration into a single struct. It returns an error if any scope
// carries an invalid strftime format, so the error surfaces at setup time
// instead of mid-run.
func (e *Executor) buildTimestampContext() (*timestampContext, error) {
	tc := &timestampContext{sharedMu: &sync.Mutex{}}
	if e.Timestamps.IsSet() {
		tc.cliSet = true
		if e.Timestamps.On() {
			layout, _, err := output.ResolveLayout(e.Timestamps)
			if err != nil {
				return nil, err
			}
			tc.cliLayout = layout
		} else {
			tc.cliOff = true
		}
	}
	if e.Ritefile != nil && e.Ritefile.Timestamps.IsSet() && e.Ritefile.Timestamps.On() {
		layout, _, err := output.ResolveLayout(e.Ritefile.Timestamps)
		if err != nil {
			return nil, err
		}
		tc.topLayout = layout
	}
	// Effective global layout for rite's own logger: CLI wins if set;
	// otherwise top-level.
	switch {
	case tc.cliOff:
		tc.globalLayout = ""
	case tc.cliLayout != "":
		tc.globalLayout = tc.cliLayout
	default:
		tc.globalLayout = tc.topLayout
	}
	return tc, nil
}

// effectiveLayoutForTask resolves the per-task Go time layout given the
// SPEC's precedence: CLI > task > top-level. An empty return means
// timestamps are off for this task's cmd output.
func (tc *timestampContext) effectiveLayoutForTask(taskTS *ast.Timestamps) (string, error) {
	if tc == nil {
		return "", nil
	}
	if tc.cliSet {
		if tc.cliOff {
			return "", nil
		}
		return tc.cliLayout, nil
	}
	if taskTS.IsSet() {
		if !taskTS.On() {
			return "", nil
		}
		layout, _, err := output.ResolveLayout(taskTS)
		if err != nil {
			return "", err
		}
		return layout, nil
	}
	return tc.topLayout, nil
}

// wrapLoggerWriters wraps the Executor's Stdout/Stderr with
// TimestampWriters when the global layout is non-empty. Returns cleanup
// closers so the final partial-line flush can run at end-of-run. Safe to
// call with a nil context; returns no-op closers in that case.
func (tc *timestampContext) wrapLoggerWriters(stdout, stderr io.Writer) (io.Writer, io.Writer, func()) {
	if tc == nil || tc.globalLayout == "" {
		return stdout, stderr, func() {}
	}
	outSink := output.NewTimestampSink(stdout, tc.globalLayout, nil, tc.sharedMu)
	errSink := output.NewTimestampSink(stderr, tc.globalLayout, nil, tc.sharedMu)
	outW := output.NewTimestampWriter(outSink)
	errW := output.NewTimestampWriter(errSink)
	return outW, errW, func() {
		_ = outW.Close()
		_ = errW.Close()
	}
}

// wrapCmdWriters wraps a pair of cmd writers with TimestampWriters if the
// effective layout is non-empty. The returned closer must be called after
// the cmd finishes so any partial trailing line flushes.
func (tc *timestampContext) wrapCmdWriters(stdout, stderr io.Writer, layout string) (io.Writer, io.Writer, func() error) {
	if tc == nil || layout == "" {
		return stdout, stderr, func() error { return nil }
	}
	outSink := output.NewTimestampSink(stdout, layout, nil, tc.sharedMu)
	errSink := output.NewTimestampSink(stderr, layout, nil, tc.sharedMu)
	outW := output.NewTimestampWriter(outSink)
	errW := output.NewTimestampWriter(errSink)
	return outW, errW, func() error {
		if err := outW.Close(); err != nil {
			return err
		}
		return errW.Close()
	}
}
