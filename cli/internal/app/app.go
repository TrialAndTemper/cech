// Package app parses the cech command line and runs its commands, under
// the output contract of ADR 14: results on stdout, messages on stderr,
// one JSON object on stdout with --json, and fixed exit codes.
package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/TrialAndTemper/cech/cli/internal/version"
)

// Exit codes (ADR 14). Other codes are not used.
const (
	ExitOK          = 0 // the command did what it was asked
	ExitFailure     = 1 // it ran and met a failure
	ExitUsage       = 2 // the command line is wrong
	ExitUnreachable = 4 // it could not reach what it needs
)

const usage = `Usage: cech <command> [flags]

Commands:
  version   print the version of cech

Run cech help <command>, or cech <command> -h, for the flags of a command.
`

const versionHelp = `Usage: cech version [--json]

Print the version of cech.

Flags:
  --json    print one JSON object with the version, the Go release
            that built cech, and the OS and architecture it was built for
`

// helps holds the help text of each command, for cech help <command>.
var helps = map[string]string{
	"version": versionHelp,
}

// Run runs the command line args, without the program name, and returns
// the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return run(args, stdout, stderr, version.Current())
}

func run(args []string, stdout, stderr io.Writer, info version.Info) int {
	if len(args) == 0 {
		return usageError(stderr, "no command given")
	}
	switch args[0] {
	case "help", "-h", "-help", "--help":
		return runHelp(args[1:], stdout, stderr)
	case "version":
		return runVersion(args[1:], stdout, stderr, info)
	default:
		return usageError(stderr, fmt.Sprintf("unknown command %q", args[0]))
	}
}

// runHelp prints the usage, or with a command name that command's help.
func runHelp(args []string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 0, len(args) == 1 && (isHelpFlag(args[0]) || args[0] == "help"):
		return write(stdout, stderr, usage)
	case len(args) > 1:
		return usageError(stderr, "help takes at most one command")
	}
	text, ok := helps[args[0]]
	if !ok {
		return usageError(stderr, fmt.Sprintf("no help for %q: unknown command", args[0]))
	}
	return write(stdout, stderr, text)
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
