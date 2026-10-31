package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TrialAndTemper/cech/cli/internal/build"
	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

const buildLong = `Write the files generated from the hand-written sources of a
marketplace with CECH's layout: the catalogue
.claude-plugin/marketplace.json, Codex TOML for each reviewer agent, a
copy of the root LICENSE in each skill folder, .github/CODEOWNERS and
docs/catalogue.md. In each plugin.json it sets the description, the
version and, in an extension, each CECH dependency's marketplace. It
prints each file it creates, updates or removes.`

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

// buildOptions are the flags of cech build.
type buildOptions struct {
	check, dryRun, asJSON bool
	release, dir          string
}

// newBuildCommand returns cech build, which prints its results on stdout
// and its messages on stderr, and sets *code to its exit code.
func newBuildCommand(stdout, stderr io.Writer, code *int) *cobra.Command {
	var opts buildOptions
	c := &cobra.Command{
		Use:   "build",
		Short: "write the files generated from a marketplace's sources",
		Long:  buildLong,
		Args:  noArgs,
		// No argument, so no file names to offer.
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(c *cobra.Command, _ []string) error {
			// Changed sees only the flags given, so --release "" is refused too.
			if c.Flags().Changed("release") && !marketplace.IsRelease(opts.release) {
				return fmt.Errorf("build --release takes X.Y.Z or X.Y.Z-rc.N, got %q", opts.release)
			}
			if opts.dir == "" {
				return errors.New("build -C takes a directory, got \"\"")
			}
			if opts.check && opts.dryRun {
				return errors.New("build takes --check or --dry-run, not both")
			}
			*code = runBuild(opts, stdout, stderr)
			return nil
		},
	}
	f := c.Flags()
	f.BoolVar(&opts.check, "check", false,
		"write nothing; list each file that differs from what the build would write, and fail if there is one")
	f.BoolVar(&opts.dryRun, "dry-run", false, "write nothing, and list the files the build would change")
	f.StringVar(&opts.release, "release", "",
		"build this release, as X.Y.Z or X.Y.Z-rc.N: the catalogue is named after the namespace and the release, "+
			"and every plugin's version is the release; without it, the version is 0.0.0-dev")
	f.BoolVar(&opts.asJSON, "json", false,
		"print one JSON object with the mode, whether it passed, the files, any changes a failed build did not make, and any errors")
	f.StringVarP(&opts.dir, "directory", "C", ".", "run in this directory, the marketplace's root, instead of the current one")
	_ = c.MarkFlagDirname("directory") // fails only for a flag that does not exist
	return c
}

// runBuild runs cech build with the flags in opts, its results on stdout
// and its messages on stderr, and returns the exit code.
func runBuild(opts buildOptions, stdout, stderr io.Writer) int {
	mode := modeBuild
	switch {
	case opts.check:
		mode = modeCheck
	case opts.dryRun:
		mode = modeDryRun
	}

	res := buildResult{Mode: mode, Files: []build.Diff{}, Skipped: []build.Diff{}, Errors: []string{}}
	files, skipped, err := buildIn(opts.dir, mode, opts.release)
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
	if opts.asJSON {
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

// buildIn runs the build in mode on the marketplace rooted at dir, for
// release or for none when it is "", and returns the files it changed
// or, without writing, would change. When writing fails partway, it
// also returns the changes it did not make.
func buildIn(dir, mode, release string) (files, skipped []build.Diff, err error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, nil, marketplace.Unreadable(err)
	}
	defer func() { _ = root.Close() }()
	m, err := marketplace.Load(root.FS())
	if err != nil {
		return nil, nil, err
	}
	outs, err := build.Outputs(m, release)
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
