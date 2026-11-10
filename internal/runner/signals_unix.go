//go:build !windows

package runner

import (
	"os"
	"os/signal"
	"syscall"
)

func notifySignals(ch chan<- os.Signal) {
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGWINCH)
}

func isResizeSignal(sig os.Signal) bool {
	return sig == syscall.SIGWINCH
}
