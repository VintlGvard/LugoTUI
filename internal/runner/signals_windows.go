//go:build windows

package runner

import (
	"os"
	"os/signal"
	"syscall"
)

func notifySignals(ch chan<- os.Signal) {
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
}

func isResizeSignal(_ os.Signal) bool {
	return false
}
