// Package app parses the cech command line and runs its commands, under
// the output contract of ADR 15: results on stdout, messages on stderr,
// one JSON object on stdout with --json, and fixed exit codes.
package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime/debug"
	"strings"

	"github.com/TrialAndTemper/cech/cli/internal/version"
)

// Exit codes (ADR 15). Other codes are not used.
const (
	ExitOK          = 0 // the command did what it was asked
	ExitFailure     = 1 // it ran and met a failure, or only partly succeeded; wins over ExitUnreachable
	ExitUsage       = 2 // the command line is wrong
	ExitUnreachable = 3 // it could not reach what it needs
)

// command is one cech command. The table of them drives dispatch, the
// usage list and cech help <command>.
type command struct {
	name    string
	summary string // one line for the usage list
	help    string // the text of cech help <command> and cech <command> -h
	run     func(args []string, stdout, stderr io.Writer, info version.Info) int
}

// commands lists every command but help, sorted by name.
var commands = []command{
	{"version", "print the version of cech", versionHelp, runVersion},
}

// lookup returns the command named name.
func lookup(name string) (command, bool) {
	for _, c := range commands {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}

// usage returns the usage text, with a line for each command.
func usage() string {
	var b strings.Builder
	b.WriteString("Usage: cech <command> [flags]\n\nCommands:\n")
	for _, c := range commands {
		fmt.Fprintf(&b, "  %-9s %s\n", c.name, c.summary)
	}
	b.WriteString("\nRun cech help <command>, or cech <command> -h, for the flags of a command.\n")
	return b.String()
}

const versionHelp = `Usage: cech version [--json]

Print the version of cech.

Flags:
  --json    print one JSON object with the version, the Go release
            that built cech, and the OS and architecture it was built for
`

// Run runs the command line args, without the program name, and returns
// the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return run(args, stdout, stderr, version.Current())
}

func run(args []string, stdout, stderr io.Writer, info version.Info) (code int) {
	// Go exits 2 after a panic, ADR 15's code for a wrong command line, so
	// a panic is reported as the failure it is. This catches panics in
	// this goroutine only: a command that starts goroutines recovers in
	// each of them.
	defer func() {
		if r := recover(); r != nil {
			_, _ = fmt.Fprintf(stderr, "cech: internal error: %v\n%s", r, debug.Stack())
			code = ExitFailure
		}
	}()
	if len(args) == 0 {
		return usageError(stderr, "no command given")
	}
	if args[0] == "help" || isHelpFlag(args[0]) {
		return runHelp(args[1:], stdout, stderr)
	}
	c, ok := lookup(args[0])
	if !ok {
		return usageError(stderr, fmt.Sprintf("unknown command %q", args[0]))
	}
	return c.run(args[1:], stdout, stderr, info)
}

// runHelp prints the usage, or with a command name that command's help.
func runHelp(args []string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 0, len(args) == 1 && (isHelpFlag(args[0]) || args[0] == "help"):
		return write(stdout, stderr, usage())
	case len(args) > 1:
		return usageError(stderr, "help takes at most one command")
	}
	c, ok := lookup(args[0])
	if !ok {
		return usageError(stderr, fmt.Sprintf("no help for %q: unknown command", args[0]))
	}
	return write(stdout, stderr, c.help)
}

func isHelpFlag(arg string) bool {
	return arg == "-h" || arg == "-help" || arg == "--help"
}

func runVersion(args []string, stdout, stderr io.Writer, info version.Info) int {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	asJSON := flags.Bool("json", false, "")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return write(stdout, stderr, versionHelp)
		}
		return usageError(stderr, err.Error())
	}
	if flags.NArg() > 0 {
		return usageError(stderr, fmt.Sprintf("version takes no arguments, got %q", flags.Arg(0)))
	}
	if !*asJSON {
		return write(stdout, stderr, "cech "+info.Version+"\n")
	}
	out, err := json.Marshal(info)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "cech: %v\n", err)
		return ExitFailure
	}
	return write(stdout, stderr, string(out)+"\n")
}

// usageError reports a wrong command line on stderr.
func usageError(stderr io.Writer, msg string) int {
	_, _ = fmt.Fprintf(stderr, "cech: %s\nRun cech help for usage.\n", msg)
	return ExitUsage
}

// write prints text to stdout. A failed write, as to a closed pipe, is
// a failure: the caller did not get the result.
func write(stdout, stderr io.Writer, text string) int {
	if _, err := io.WriteString(stdout, text); err != nil {
		_, _ = fmt.Fprintf(stderr, "cech: cannot write the output: %v\n", err)
		return ExitFailure
	}
	return ExitOK
}
