// Command cech is the CECH command-line tool (ADR 14); so far it prints
// its version.
package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/TrialAndTemper/cech/cli/internal/app"
)

func main() {
	// A write to a closed pipe on stdout would otherwise kill cech with
	// SIGPIPE. With SIGPIPE ignored, the write returns an error instead,
	// and cech exits 1, ADR 14's code for a failure.
	signal.Ignore(syscall.SIGPIPE)
	os.Exit(app.Run(os.Args[1:], os.Stdout, os.Stderr))
}
