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
	// Cancel the command context on the first interrupt so deferred cleanup
	// of temporary trust, staging, and publishing directories runs. Restore
	// default signal handling afterwards so a second interrupt exits at once.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()

	err := cli.Root().ExecuteContext(ctx)
	stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
