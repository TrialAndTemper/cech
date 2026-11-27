package app

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/TrialAndTemper/cech/cli/internal/evals"
	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

const evalLong = `Run the release check, or with --check check the eval cases in
evals/ without running an agent.

Without --check, it runs the eval cases of each plugin changed since the
latest final release, or of each plugin --plugin names, with a model on
each harness, each run in a fresh container from the eval image, until
each bar of the release check is clear or has no result. It writes each
run's results under evals/results/<commit>/ and the report file
evals/reports/<commit>.md. With --dry-run it prints the runs it would
make and writes nothing.

With --check, for each plugin, it checks the fixture project, the routing rows, and
that the plugin has every case it needs. For each case, it checks
prompt.md and cech.toml with its graders.

Then it checks each recorded run of Claude Code and Codex. A run must
pass or fail as the case's cech.toml says. A run of a trigger case
passes if the skill loads as the case expects; a run of any other case
passes if all the case's graders pass.

It prints each problem as <path>: <problem>.`

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
	// empty is a marketplace with no plugins and no evals/, which passes
	// with nothing checked (spec rule 27).
	empty bool
}

// evalOptions are the flags of cech eval.
type evalOptions struct {
	check, asJSON bool
	dir           string
	run           runOptions
}

// newEvalCommand returns cech eval, which prints its results on stdout
// and its messages on stderr, and sets *code to its exit code.
func newEvalCommand(stdout, stderr io.Writer, code *int) *cobra.Command {
	var opts evalOptions
	c := &cobra.Command{
		Use:   "eval",
		Short: "run the release check, or check the eval cases without running an agent",
		Long:  evalLong,
		Args:  noArgs,
		// No argument, so no file names to offer.
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.dir == "" {
				return errors.New("eval -C takes a directory, got \"\"")
			}
			if opts.check {
				for _, f := range []string{"plugin", "harness", "parallel", "dry-run"} {
					if cmd.Flags().Changed(f) {
						return fmt.Errorf("eval --check takes no --%s, which is for the release check", f)
					}
				}
				*code = runEval(opts, stdout, stderr)
				return nil
			}
			if h := opts.run.harness; h != "" && !slices.Contains(evals.Harnesses(), h) {
				return fmt.Errorf("eval --harness takes %s, got %q", strings.Join(evals.Harnesses(), " or "), h)
			}
			if opts.run.parallel < 0 {
				return fmt.Errorf("eval --parallel takes a number of runs, or 0 for the default, got %d", opts.run.parallel)
			}
			opts.run.dir, opts.run.asJSON = opts.dir, opts.asJSON
			// An interrupt cancels the runs, so their containers are removed
			// and the finished runs kept (criterion 30).
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			// After the first signal, a second ends cech eval at once,
			// as when the clean-up hangs.
			// The deferred unregister runs before the deferred stop, which
			// cancels ctx too, so the message comes only after a signal.
			unregister := context.AfterFunc(ctx, func() {
				stop()
				_, _ = fmt.Fprintln(stderr, "cech: stopping the runs and removing their containers; interrupt again to stop at once")
			})
			defer unregister()
			*code = runRelease(ctx, opts.run, stdout, stderr)
			return nil
		},
	}
	f := c.Flags()
	f.BoolVar(&opts.check, "check", false, "check the cases and their recordings without running an agent")
	f.StringSliceVar(&opts.run.plugins, "plugin", nil, "check these plugins, not those changed since the latest final release")
	f.StringVar(&opts.run.harness, "harness", "", "run on this harness only, claude-code or codex")
	f.IntVar(&opts.run.parallel, "parallel", 0, "how many runs share a harness at once; by default 6 on claude-code and 4 on codex")
	f.BoolVar(&opts.run.dryRun, "dry-run", false, "print the runs it would make and their estimated time, and write nothing")
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
		_, _ = fmt.Fprintln(stderr, "cech: nothing checked: the marketplace has no plugins and no evals/")
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
	if len(m.Plugins) == 0 && !exists(fsys, evals.Dir) {
		res.empty = true
		return res, nil
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
