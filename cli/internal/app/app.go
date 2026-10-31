// Package app parses the cech command line and runs its commands, under
// the output contract of ADR 15: results on stdout, messages on stderr,
// one JSON object on stdout with --json, and fixed exit codes.
package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TrialAndTemper/cech/cli/internal/version"
)

// Exit codes (ADR 15). Other codes are not used.
const (
	ExitOK          = 0 // the command did what it was asked
	ExitFailure     = 1 // it ran and met a failure, or only partly succeeded; wins over ExitUnreachable
	ExitUsage       = 2 // the command line is wrong
	ExitUnreachable = 3 // it could not reach what it needs
)

// Run runs the command line args, without the program name, and returns
// the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return run(args, stdout, stderr, version.Current())
}

// run runs the command line args, with output on stdout and stderr and
// info as the version that cech version prints, and returns the exit
// code.
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
	out := &outWriter{w: stdout}
	root := newRoot(out, stdout, stderr, info, &code)
	root.SetArgs(args)
	// Commands report results through code and return an error only for a
	// wrong command line, so every error here is one.
	err := root.Execute()
	switch {
	case out.err != nil:
		_, _ = fmt.Fprintf(stderr, "cech: cannot write the output: %v\n", out.err)
		return ExitFailure
	case err != nil:
		return usageError(stderr, strings.TrimRight(err.Error(), "\n"))
	}
	return code
}

// newRoot returns the command tree of cech. Cobra prints help and
// completion scripts on out; the commands print their results on stdout
// and their messages on stderr, and set *code to their exit code. info
// is the version that cech version prints.
func newRoot(out, stdout, stderr io.Writer, info version.Info, code *int) *cobra.Command {
	root := &cobra.Command{
		Use:   "cech",
		Short: "cech is the command-line tool of CECH, a marketplace of agent skills",
		// Cobra checks for an unknown command only when an argument
		// starts the command line; this catches the rest, as cech -- or
		// cech --help=false, which would otherwise print help and exit 0.
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("no command given")
			}
			return fmt.Errorf("unknown command %q for \"cech\"", args[0])
		},
		// run prints errors on stderr and exits 2 (ADR 15 item 5).
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetOut(out)
	root.SetErr(stderr)
	// cech and cech completion run only to refuse a missing command, so
	// their usage shows the form with a command alone.
	root.SetUsageTemplate(strings.Replace(root.UsageTemplate(),
		"{{if .Runnable}}", "{{if and .Runnable (not .HasAvailableSubCommands)}}", 1))
	root.AddCommand(
		newBuildCommand(stdout, stderr, code),
		newVersionCommand(stdout, stderr, info, code),
	)
	// Cobra adds the help flag, help and completion when the command line
	// runs. Adding the flag here lets cech --help build find build; adding
	// help and completion here lets them refuse a wrong command line and
	// use cech's help text in place of Cobra's; adding all of them lets
	// the tests walk the whole tree.
	root.InitDefaultHelpFlag()
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	for _, c := range root.Commands() {
		switch c.Name() {
		case "help":
			// Keep Cobra's help, which completes a command's path, but give
			// it cech's wording and refuse a command it does not know,
			// where Cobra's prints the usage and exits 0.
			c.Short = "print the help of cech or of a command"
			c.Long = "Print the help of cech, or of the command named, as cech help completion bash."
			c.Run = nil
			c.RunE = func(c *cobra.Command, args []string) error {
				target, rest, err := c.Root().Find(args)
				if err != nil || len(rest) > 0 {
					return fmt.Errorf("no help for %q: unknown command", strings.Join(args, " "))
				}
				target.InitDefaultHelpFlag()
				return target.Help()
			}
		case "completion":
			c.Short = "print the completion script of a shell"
			c.Long = "Print the completion script of a shell, for the shell to source.\n" +
				"Run cech completion <shell> --help for how to load it."
			c.RunE = func(*cobra.Command, []string) error {
				return errors.New("completion takes a shell: bash, zsh, fish or powershell")
			}
			for _, shell := range c.Commands() {
				shell.Short = "print the completion script of " + shell.Name()
			}
		}
	}
	return root
}

// noArgs refuses any argument to the command c, naming the first of args.
func noArgs(c *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("%s takes no arguments, got %q", c.Name(), args[0])
	}
	return nil
}

// newVersionCommand returns cech version, which prints info on stdout,
// any error on stderr, and sets *code to its exit code.
func newVersionCommand(stdout, stderr io.Writer, info version.Info, code *int) *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "version",
		Short: "print the version of cech",
		Long:  "Print the version of cech.",
		Args:  noArgs,
		// No argument, so no file names to offer.
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(*cobra.Command, []string) error {
			*code = runVersion(asJSON, stdout, stderr, info)
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false,
		"print one JSON object with the version, the Go release that built cech, and the OS and architecture it was built for")
	return c
}

// runVersion prints info on stdout, as JSON when asJSON is set, and any
// error on stderr, and returns the exit code.
func runVersion(asJSON bool, stdout, stderr io.Writer, info version.Info) int {
	if !asJSON {
		return write(stdout, stderr, "cech "+info.Version+"\n")
	}
	out, err := json.Marshal(info)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "cech: %v\n", err)
		return ExitFailure
	}
	return write(stdout, stderr, string(out)+"\n")
}

// outWriter is Cobra's stdout. Cobra ignores a failed write of help, so
// outWriter keeps the first error in err for run to report.
type outWriter struct {
	w   io.Writer
	err error
}

// Write writes p to the wrapped writer and keeps the first error.
func (o *outWriter) Write(p []byte) (int, error) {
	n, err := o.w.Write(p)
	if err != nil && o.err == nil {
		o.err = err
	}
	return n, err
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
