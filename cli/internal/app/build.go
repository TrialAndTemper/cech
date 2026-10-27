package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/TrialAndTemper/cech/cli/internal/build"
	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
	"github.com/TrialAndTemper/cech/cli/internal/version"
)

const buildHelp = `Usage: cech build [--check | --dry-run] [--json] [-C dir]

Write the files generated from the hand-written sources of a
marketplace with CECH's layout: the catalogue
.claude-plugin/marketplace.json, Codex TOML for each reviewer agent,
.github/CODEOWNERS and docs/catalogue.md. In each plugin.json it sets
the description and, in an extension, each CECH dependency's
marketplace. It prints each file it creates, updates or removes.

Flags:
  --check     write nothing; list each file that differs from what
              the build would write, and fail if there is one
  --dry-run   write nothing, and list the files the build would change
  --json      print one JSON object with the mode, whether it passed,
              the files, any changes a failed build did not make, and
              any errors
  -C dir      run in dir, the marketplace's root, instead of the
              current directory
`

// The modes of cech build.
const (
	modeBuild  = "build"
	modeCheck  = "check"
	modeDryRun = "dry-run"
)

// buildResult is the JSON output of cech build. Files are the changes
// made, or with --check and --dry-run the changes the build would make.
// Skipped are the changes a build that failed partway did not make.
type buildResult struct {
	Mode    string       `json:"mode"`
	OK      bool         `json:"ok"`
	Files   []build.Diff `json:"files"`
	Skipped []build.Diff `json:"skipped"`
	Errors  []string     `json:"errors"`
}

func runBuild(args []string, stdout, stderr io.Writer, _ version.Info) int {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	check := flags.Bool("check", false, "")
	dryRun := flags.Bool("dry-run", false, "")
	asJSON := flags.Bool("json", false, "")
	dir := flags.String("C", ".", "")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return write(stdout, stderr, buildHelp)
		}
		return usageError(stderr, err.Error())
	}
	if flags.NArg() > 0 {
		return usageError(stderr, fmt.Sprintf("build takes no arguments, got %q", flags.Arg(0)))
	}
	mode := modeBuild
	switch {
	case *check && *dryRun:
		return usageError(stderr, "build takes --check or --dry-run, not both")
	case *check:
		mode = modeCheck
	case *dryRun:
		mode = modeDryRun
	}

	res := buildResult{Mode: mode, Files: []build.Diff{}, Skipped: []build.Diff{}, Errors: []string{}}
	files, skipped, err := buildIn(*dir, mode)
	if files != nil {
		res.Files = files
	}
	if skipped != nil {
		res.Skipped = skipped
	}
	errs := leaves(err)
	for _, e := range errs {
		res.Errors = append(res.Errors, e.Error())
	}
	res.OK = err == nil && (mode != modeCheck || len(files) == 0)

	// A source it could not read leaves it unable to decide; any other
	// error is a failure, which wins (ADR 15 item 5).
	code := ExitOK
	switch {
	case res.OK:
	case len(errs) > 0 && !slices.ContainsFunc(errs, func(e error) bool {
		return !errors.Is(e, marketplace.ErrUnreadable)
	}):
		code = ExitUnreachable
	default:
		code = ExitFailure
	}
	if *asJSON {
		out, err := json.Marshal(res)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "cech: %v\n", err)
			return ExitFailure
		}
		if write(stdout, stderr, string(out)+"\n") != ExitOK {
			return ExitFailure
		}
		return code
	}

	var b strings.Builder
	for _, f := range res.Files {
		verb := string(f.Change)
		if mode == modeBuild {
			verb += "d"
		}
		b.WriteString(verb + " " + f.Path + "\n")
	}
	if write(stdout, stderr, b.String()) != ExitOK {
		return ExitFailure
	}
	for _, msg := range res.Errors {
		_, _ = fmt.Fprintf(stderr, "cech: %s\n", msg)
	}
	for _, f := range res.Skipped {
		_, _ = fmt.Fprintf(stderr, "cech: skipped: %s %s\n", f.Change, f.Path)
	}
	if mode == modeCheck && len(res.Files) > 0 {
		what := fmt.Sprintf("%d generated files differ from their sources", len(res.Files))
		if len(res.Files) == 1 {
			what = "1 generated file differs from its sources"
		}
		_, _ = fmt.Fprintf(stderr, "cech: %s; run cech build\n", what)
	}
	return code
}

// buildIn runs the build in mode on the marketplace rooted at dir, and
// returns the files it changed or, without writing, would change. When
// writing fails partway, it also returns the changes it did not make.
func buildIn(dir, mode string) (files, skipped []build.Diff, err error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, nil, marketplace.Unreadable(err)
	}
	defer func() { _ = root.Close() }()
	m, err := marketplace.Load(root.FS())
	if err != nil {
		return nil, nil, err
	}
	outs, err := build.Outputs(m)
	if err != nil {
		return nil, nil, err
	}
	diffs, err := build.Compare(root, outs)
	if err != nil || mode != modeBuild {
		return diffs, nil, err
	}
	done, err := build.Apply(root, outs, diffs)
	if err != nil {
		return done, diffs[len(done):], err
	}
	return done, nil, nil
}

// leaves returns err, or each error joined in it; none for nil.
func leaves(err error) []error {
	if err == nil {
		return nil
	}
	// Only a join at the top: one inside a wrap keeps the wrap's prefix.
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return []error{err}
	}
	var errs []error
	for _, e := range joined.Unwrap() {
		errs = append(errs, leaves(e)...)
	}
	return errs
}
