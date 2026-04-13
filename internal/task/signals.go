package task

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/clintmod/rite/internal/logger"
)

const maxInterruptSignals = 3

// NOTE(@andreynering): This function intercepts SIGINT and SIGTERM signals
// so the Task process is not killed immediately and processes running have
// time to do cleanup work.
func (e *Executor) InterceptInterruptSignals() {
	ch := make(chan os.Signal, maxInterruptSignals)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for i := range maxInterruptSignals {
			sig := <-ch

			if i+1 >= maxInterruptSignals {
				e.Logger.Errf(logger.Red, "rite: Signal received for the third time: %q. Forcing shutdown\n", sig)
				os.Exit(1)
			}

			e.Logger.Outf(logger.Yellow, "rite: Signal received: %q\n", sig)
		}
	}()
}
