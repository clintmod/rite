//go:build !windows

package task_test

import (
	"bytes"
	"context"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/clintmod/rite/internal/logger"
	task "github.com/clintmod/rite/internal/task"
)

// sendSelf delivers sig to the current test process. SIGTERM sent this way
// reaches only the current PID (not the process group) — exactly the
// programmatic `kill -TERM <pid>` case issue #50 is about.
func sendSelf(t *testing.T, sig syscall.Signal) {
	t.Helper()
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("FindProcess(self): %v", err)
	}
	if err := p.Signal(sig); err != nil {
		t.Fatalf("Signal(%v): %v", sig, err)
	}
}

// lockedBuffer is a bytes.Buffer safe for concurrent Write from the
// signal-handler goroutine and Read/String from the test.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newTestExecutor(buf *lockedBuffer) *task.Executor {
	return &task.Executor{Logger: &logger.Logger{Stdout: buf, Stderr: buf}}
}

// TestInterceptInterruptSignalsSIGTERMCancelsCtx is the regression test
// for issue #50: programmatic SIGTERM must cancel the returned context so
// `errgroup.WithContext` ctx and the `mvdan.cc/sh` interpreter can drain
// child processes cooperatively, rather than being orphaned to init/launchd.
//
//nolint:paralleltest // installs a process-global signal handler
func TestInterceptInterruptSignalsSIGTERMCancelsCtx(t *testing.T) {
	var buf lockedBuffer
	e := newTestExecutor(&buf)

	ctx, stop := e.InterceptInterruptSignals(context.Background())
	t.Cleanup(stop)

	select {
	case <-ctx.Done():
		t.Fatal("ctx cancelled before any signal was sent")
	default:
	}

	sendSelf(t, syscall.SIGTERM)

	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatalf("ctx was not cancelled within 3s after SIGTERM — handler is not plumbing cancellation. Log so far: %q", buf.String())
	}
}

//nolint:paralleltest // installs a process-global signal handler
func TestInterceptInterruptSignalsSIGINTCancelsCtx(t *testing.T) {
	var buf lockedBuffer
	e := newTestExecutor(&buf)

	ctx, stop := e.InterceptInterruptSignals(context.Background())
	t.Cleanup(stop)

	sendSelf(t, syscall.SIGINT)

	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatalf("ctx was not cancelled within 3s after SIGINT. Log so far: %q", buf.String())
	}
}

// TestInterceptInterruptSignalsStopCancelsAndIsIdempotent verifies that
// `defer stop()` cleanly cancels the ctx and that a second call is a no-op.
//
//nolint:paralleltest // installs a process-global signal handler
func TestInterceptInterruptSignalsStopCancelsAndIsIdempotent(t *testing.T) {
	var buf lockedBuffer
	e := newTestExecutor(&buf)

	ctx, stop := e.InterceptInterruptSignals(context.Background())
	stop()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("stop() did not cancel ctx")
	}

	stop()
}

// TestInterceptInterruptSignalsParentCancelPropagates pins the standard
// context.WithCancel contract: cancelling the parent cancels the child.
//
//nolint:paralleltest // installs a process-global signal handler
func TestInterceptInterruptSignalsParentCancelPropagates(t *testing.T) {
	var buf lockedBuffer
	e := newTestExecutor(&buf)

	parent, parentCancel := context.WithCancel(context.Background())
	ctx, stop := e.InterceptInterruptSignals(parent)
	t.Cleanup(stop)

	parentCancel()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("cancelling parent did not cancel returned ctx")
	}
}
