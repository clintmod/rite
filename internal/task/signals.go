package task

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/clintmod/rite/internal/logger"
)

const maxInterruptSignals = 3

// InterceptInterruptSignals installs a handler for SIGINT, SIGTERM, and SIGHUP
// and returns a child context that cancels on the first signal, plus a stop
// function that tears the handler down.
//
// On the first signal the returned context is cancelled, which propagates
// through [errgroup.WithContext] and the [mvdan.cc/sh] interpreter inside
// [execext.RunCommand] so running subprocesses receive cooperative shutdown
// instead of being orphaned. Subsequent signals re-log; the third signal
// escalates to `os.Exit(1)` as a safety net in case a task hangs during
// cleanup.
//
// Without this wiring, `signal.Notify` would hijack the default terminate
// behavior but never cancel anything — a programmatic `kill -TERM <pid>`
// (which, unlike interactive Ctrl-C, does not fan out to the process group)
// would leave child processes running after rite exited.
//
// Callers should `defer stop()` to unregister the handler cleanly; stop also
// cancels the returned context.
func (e *Executor) InterceptInterruptSignals(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan os.Signal, maxInterruptSignals)
	stopCh := make(chan struct{})
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		defer signal.Stop(ch)
		for i := range maxInterruptSignals {
			var sig os.Signal
			select {
			case sig = <-ch:
			case <-stopCh:
				return
			}

			if i+1 >= maxInterruptSignals {
				e.Logger.Errf(logger.Red, "rite: Signal received for the third time: %q. Forcing shutdown\n", sig)
				os.Exit(1)
			}

			e.Logger.Outf(logger.Yellow, "rite: Signal received: %q\n", sig)
			cancel()
		}
	}()

	stop := func() {
		select {
		case <-stopCh:
		default:
			close(stopCh)
		}
		cancel()
	}
	return ctx, stop
}
