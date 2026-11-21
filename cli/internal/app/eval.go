package app

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TrialAndTemper/cech/cli/internal/evals"
	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

const evalLong = `Check a marketplace's eval folders in evals/ without running a model
(ADR 25): each plugin's fixture project and routing rows, its cases'
prompt.md, graders and cech.toml, and the cases each plugin needs. Each
recorded run of Claude Code and Codex must pass or fail as its
cech.toml declares, by the case's deterministic graders or, for a
trigger case, by the harness's load detector. It prints each finding
as <path>: <problem>.`

// evalResult is the JSON output of cech eval --check. Errors are the
// reads and sources that kept it from checking something: a denied read
// skips the rest of the folder or case it names, and a note says when
// every case was skipped.
type evalResult struct {
	OK         bool            `json:"ok"`
	Cases      int             `json:"cases"`
	Recordings int             `json:"recordings"`
	Findings   []evals.Finding `json:"findings"`
	Errors     []string        `json:"errors"`
	// empty is a tree with no plugins/ and no evals/, which passes with
	// nothing checked (spec rule 27).
	empty bool
}

// evalOptions are the flags of cech eval.
type evalOptions struct {
	check, asJSON bool
	dir           string
}

// newEvalCommand returns cech eval, which prints its results on stdout
// and its messages on stderr, and sets *code to its exit code.
func newEvalCommand(stdout, stderr io.Writer, code *int) *cobra.Command {
	var opts evalOptions
	c := &cobra.Command{
		Use:   "eval",
		Short: "check a marketplace's eval cases without a model",
		Long:  evalLong,
		Args:  noArgs,
		// No argument, so no file names to offer.
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(*cobra.Command, []string) error {
			if !opts.check {
				return errors.New("eval runs only with --check, which checks the cases without a model")
			}
			if opts.dir == "" {
				return errors.New("eval -C takes a directory, got \"\"")
			}
			*code = runEval(opts, stdout, stderr)
			return nil
		},
	}
	f := c.Flags()
	f.BoolVar(&opts.check, "check", false, "check the cases; cech eval runs only with --check")
	f.BoolVar(&opts.asJSON, "json", false,
		"print one JSON object with whether it passed, the numbers of cases and recordings found, the findings, "+
			"and the errors that kept it from checking something")
	f.StringVarP(&opts.dir, "directory", "C", ".", "run in this directory, the marketplace's root, instead of the current one")
	_ = c.MarkFlagDirname("directory") // fails only for a flag that does not exist
	return c
}

// runEval runs cech eval --check with the flags in opts, its results on
// stdout and its messages on stderr, and returns the exit code.
func runEval(opts evalOptions, stdout, stderr io.Writer) int {
	res, errs := checkEvals(opts.dir)
	for _, e := range errs {
		res.Errors = append(res.Errors, e.Error())
	}
	// A read it was denied leaves it unable to decide, unless a finding
	// or another error already fails it (ADR 15 item 5).
	code := ExitOK
	switch {
	case len(res.Findings) > 0, slices.ContainsFunc(errs, func(e error) bool {
		return !errors.Is(e, marketplace.ErrUnreadable)
	}):
		code = ExitFailure
	case len(errs) > 0:
		code = ExitUnreachable
	}
	res.OK = code == ExitOK
	if opts.asJSON {
		out, err := json.Marshal(res, jsonOptions)
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
	for _, f := range res.Findings {
		b.WriteString(f.Path + ": " + f.Message + "\n")
	}
	if write(stdout, stderr, b.String()) != ExitOK {
		return ExitFailure
	}
	for _, msg := range res.Errors {
		_, _ = fmt.Fprintf(stderr, "cech: %s\n", msg)
	}
	// A pass with nothing checked says so, or a -C to the wrong folder
	// would pass without a word; with --json, cases and recordings are 0.
	if res.empty {
		_, _ = fmt.Fprintln(stderr, "cech: nothing checked: no plugins/ or evals/ here")
	}
	switch n := len(res.Findings); {
	case n == 1:
		_, _ = fmt.Fprintln(stderr, "cech: 1 finding")
	case n > 1:
		_, _ = fmt.Fprintf(stderr, "cech: %d findings\n", n)
	}
	return code
}

// The notes that no eval case was checked, after an error in the
// marketplace's sources (spec rule 27): one it was denied, or one
// cech build rejects.
var (
	errSkippedUnread = errors.New("skipped the eval cases: the marketplace's sources could not be read")
	errSkippedBad    = errors.New("skipped the eval cases: the marketplace's sources have errors")
)

// checkEvals checks the eval cases of the marketplace rooted at dir, and
// returns the errors that kept it from checking something. A source
// cech build rejects is such an error, and skips every case, since
// their plugins are then unknown (spec rule 27).
func checkEvals(dir string) (evalResult, []error) {
	res := evalResult{Findings: []evals.Finding{}, Errors: []string{}}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return res, []error{marketplace.Unreadable(err)}
	}
	defer func() { _ = root.Close() }()
	fsys := root.FS()
	if !exists(fsys, "plugins") && !exists(fsys, evals.Dir) {
		res.empty = true
		return res, nil
	}
	m, err := marketplace.Load(fsys)
	if err != nil {
		errs := leaves(err)
		// The note shares the errors' kind, so a denied read alone still
		// leaves the check unable to decide.
		skipped := errSkippedBad
		if !slices.ContainsFunc(errs, func(e error) bool { return !errors.Is(e, marketplace.ErrUnreadable) }) {
			skipped = marketplace.Unreadable(errSkippedUnread)
		}
		return res, append(errs, skipped)
	}
	r, err := evals.Check(fsys, m)
	res.Cases, res.Recordings = r.Cases, r.Recordings
	res.Findings = append(res.Findings, r.Findings...)
	return res, leaves(err)
}

// exists reports whether name exists in fsys; a denied look counts as
// existing, so the read that follows reports it.
func exists(fsys fs.FS, name string) bool {
	_, err := fs.Stat(fsys, name)
	return !errors.Is(err, fs.ErrNotExist)
}
