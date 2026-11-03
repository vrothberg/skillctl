package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/vrothberg/skillctl/internal/cli"
)

func main() {
	// Cancel the command context on the first signal so deferred cleanup of
	// temporary trust, staging, and publishing directories runs. Stop
	// notifying right after, so a second signal terminates the process at
	// once instead of waiting for the cleanup to finish.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	interrupted := make(chan os.Signal, 1)
	go func() {
		select {
		case sig := <-signals:
			interrupted <- sig
			signal.Stop(signals)
			cancel()
		case <-ctx.Done():
		}
	}()

	err := cli.Root().ExecuteContext(ctx)
	if err == nil {
		return
	}

	// Report a signal like a shell does: 128 plus the signal number, without
	// the cancellation error the interrupted operation returned. The
	// goroutine queues the signal before canceling the context.
	select {
	case sig := <-interrupted:
		fmt.Fprintln(os.Stderr, "Error: interrupted")
		if number, ok := sig.(syscall.Signal); ok {
			os.Exit(128 + int(number))
		}
		os.Exit(1)
	default:
	}

	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
