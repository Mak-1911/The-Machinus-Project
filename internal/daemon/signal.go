package daemon

import (
	"os"
	"os/signal"
	"syscall"
)

// SetupSignals: Creates a channel that receives shutdown signals.
// 	-> Returns a channel that will receive SIGINT (Ctrl+C) and SIGTERM (kill command)
func SetupSignals() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	
	// Notifying on these signals,
	//  - SIGINT: Ctrl + C
	// 	- SIGTERM: kill Command
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	return sigChan
}

// WaitForShutdown: Blocks until a shutdown signal is received
//   -> When a signal is received, calls cleanup function and closes shutdown channel to notify other goroutines
func WaitForShutdown(sigChan chan os.Signal, shutdown chan struct{}, cleanup func()) {
	// Block until signal received
	<-sigChan

	// Run cleanup (close files, save state, etc)
	if cleanup != nil {
		cleanup()
	}

	// Signal that shutdown is complete
	close(shutdown)
}