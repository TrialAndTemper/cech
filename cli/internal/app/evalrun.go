package app

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/TrialAndTemper/cech/cli/internal/evalchange"
	"github.com/TrialAndTemper/cech/cli/internal/evalrun"
	"github.com/TrialAndTemper/cech/cli/internal/evals"
	"github.com/TrialAndTemper/cech/cli/internal/evalstats"
	"github.com/TrialAndTemper/cech/cli/internal/gitenv"
	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
	"github.com/TrialAndTemper/cech/cli/internal/otlpsink"
	"github.com/TrialAndTemper/cech/cli/internal/releasecheck"
	"github.com/TrialAndTemper/cech/cli/internal/releaserun"
	"github.com/TrialAndTemper/cech/cli/internal/version"
)

// releaseStats are the bars' values the CLI holds (eval-runs spec
// criterion 33), which docs/reference/evals.md gives.
var releaseStats = evalstats.Settings{
	FalseAlarm: 0.05, Threshold: 0.10, Cap: 500,
	ContentBar: 0.8, LoadBar: 0.7, SkipBar: 0.1,
	ErrorShare: 0.1, ErrorFrom: 20,
}

// releaseBudget is the most runs per harness per release (criterion 33).
const releaseBudget = 10000

// runOptions are the flags of cech eval without --check.
type runOptions struct {
	plugins  []string
	harness  string
	parallel int
	dryRun   bool
	asJSON   bool
	dir      string
}

// evalRunResult is the JSON output of cech eval without --check.
type evalRunResult struct {
	OK      bool        `json:"ok"`
	Commit  string      `json:"commit"`
	Plugins []string    `json:"plugins"`
	DryRun  bool        `json:"dry_run"`
	Plan    []planEntry `json:"plan"`
	Report  string      `json:"report,omitempty"`
	Bars    []barEntry  `json:"bars"`
	Errors  []string    `json:"errors"`
}

type planEntry struct {
	Harness      string  `json:"harness"`
	FirstRuns    int     `json:"first_runs"`
	MostRuns     int     `json:"most_runs"`
	FirstMinutes float64 `json:"first_minutes"`
	MostMinutes  float64 `json:"most_minutes"`
}

type barEntry struct {
	Harness string `json:"harness"`
	Plugin  string `json:"plugin"`
	Case    string `json:"case"`
	Bar     string `json:"bar"`
	Result  string `json:"result"`
	Runs    []int  `json:"runs"`
}

// errRefused marks a refusal to run (criterion 29), which exits 1.
var errRefused = errors.New("refused")

// runRelease runs cech eval without --check: the release check on the
// marketplace at opts.dir (eval-runs spec criteria 26 to 31).
func runRelease(ctx context.Context, opts runOptions, stdout, stderr io.Writer) int {
	res := evalRunResult{Plugins: []string{}, Plan: []planEntry{}, Bars: []barEntry{}, Errors: []string{}, DryRun: opts.dryRun}
	code, err := releaseCheck(ctx, opts, &res, stderr)
	if code == ExitUsage {
		// A wrong command line gets no JSON, as with any other command.
		_, _ = fmt.Fprintf(stderr, "cech: %v\n", err)
		return code
	}
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
	}
	res.OK = code == ExitOK
	if opts.asJSON {
		out, jerr := json.Marshal(res, jsonOptions)
		if jerr != nil {
			_, _ = fmt.Fprintf(stderr, "cech: %v\n", jerr)
			return ExitFailure
		}
		if write(stdout, stderr, string(out)+"\n") != ExitOK {
			return ExitFailure
		}
		return code
	}
	var b strings.Builder
	for _, p := range res.Plan {
		fmt.Fprintf(&b, "%s: %d runs to start, at most %d; about %.0f to %.0f minutes\n", p.Harness, p.FirstRuns, p.MostRuns, p.FirstMinutes, p.MostMinutes)
	}
	for _, bar := range res.Bars {
		c := bar.Case
		if c == "" {
			c = "trigger cases"
		}
		fmt.Fprintf(&b, "%s: %s %s: %s: %s\n", bar.Harness, bar.Plugin, c, bar.Bar, bar.Result)
	}
	if res.Report != "" {
		fmt.Fprintf(&b, "report: %s\n", res.Report)
	}
	if write(stdout, stderr, b.String()) != ExitOK {
		return ExitFailure
	}
	for _, msg := range res.Errors {
		_, _ = fmt.Fprintf(stderr, "cech: %s\n", msg)
	}
	return code
}

func releaseCheck(ctx context.Context, opts runOptions, res *evalRunResult, stderr io.Writer) (int, error) {
	dir, err := filepath.Abs(opts.dir)
	if err != nil {
		return ExitUnreachable, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return ExitUnreachable, marketplace.Unreadable(err)
	}
	defer func() { _ = root.Close() }()
	fsys := root.FS()
	m, err := marketplace.Load(fsys)
	if err != nil {
		return ExitFailure, err
	}
	byName := map[string]marketplace.Plugin{}
	for _, p := range m.Plugins {
		byName[p.Name] = p
	}
	// A --plugin that names no plugin is a wrong command line, whatever
	// state the checkout is in (criterion 28).
	var named []string
	for _, name := range opts.plugins {
		if _, ok := byName[name]; !ok {
			return ExitUsage, fmt.Errorf("eval --plugin %s names no plugin of this marketplace", name)
		}
		if !slices.Contains(named, name) {
			named = append(named, name)
		}
	}
	commit, err := gitOut(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return ExitUnreachable, fmt.Errorf("%s is not a git checkout: %w", opts.dir, err)
	}
	res.Commit = commit
	if dirty, derr := gitOut(ctx, dir, "status", "--porcelain", "--untracked-files=all", "--",
		"plugins", "evals", ":(exclude)evals/reports", ":(exclude)evals/results", ".claude-plugin", "data/marketplace.toml", "LICENSE"); derr != nil {
		return ExitUnreachable, derr
	} else if dirty != "" {
		return ExitFailure, fmt.Errorf("%w: plugins/, evals/, .claude-plugin/, data/marketplace.toml or LICENSE differ from HEAD; commit them, since the report names the commit it ran on", errRefused)
	}
	// The cases run only as cech eval --check would pass them.
	checked, err := evals.Check(fsys, m)
	if len(checked.Findings) > 0 {
		return ExitFailure, fmt.Errorf("%w: cech eval --check finds problems in the eval cases (%d); run it to see them, and fix them first", errRefused, len(checked.Findings))
	}
	if err != nil {
		// A denied read is 3 only when no other error applies (ADR 15
		// item 5).
		if !slices.ContainsFunc(leaves(err), func(e error) bool { return !errors.Is(e, marketplace.ErrUnreadable) }) {
			return ExitUnreachable, err
		}
		return ExitFailure, err
	}
	changes, err := evalchange.Changed(ctx, dir, m.Plugins)
	if errors.Is(err, evalchange.ErrShallow) {
		return ExitUnreachable, err
	} else if err != nil {
		return ExitFailure, err
	}
	covered := changes.Covered()
	if len(named) > 0 {
		covered = named
	}
	res.Plugins = covered
	if len(covered) == 0 {
		_, _ = fmt.Fprintln(stderr, "cech: no plugin changed since the latest final release, so nothing to check")
		return ExitOK, nil
	}
	// An arm installs the plugin and those it depends on, all from this
	// marketplace (criterion 23).
	install := map[string]bool{}
	for _, name := range covered {
		install[name] = true
		for _, d := range changes.Needs(name) {
			if _, ok := byName[d]; !ok {
				return ExitFailure, fmt.Errorf("%w: %s depends on %s, which this marketplace does not hold; cech eval installs only plugins of the marketplace it checks", errRefused, name, d)
			}
			install[d] = true
		}
	}
	harnesses := evals.Harnesses()
	if opts.harness != "" {
		harnesses = []string{opts.harness}
	}
	cases := readCases(ctx, dir, checked.Found, covered, changes)
	settings := evalrun.Defaults
	if img := os.Getenv("CECH_EVAL_IMAGE"); img != "" {
		settings.Image = img
	}
	for _, h := range harnesses {
		res.Plan = append(res.Plan, planFor(h, cases, settings, opts.parallel))
	}
	if opts.dryRun {
		return ExitOK, nil
	}
	if settings.Image == "" {
		return ExitUnreachable, errors.New("no eval image: none is published yet, so set CECH_EVAL_IMAGE to an image built from images/eval/")
	}
	engine, err := evalrun.Detect(ctx)
	if err != nil {
		return ExitUnreachable, err
	}
	// Runs, their reuse key and the Claude Code install go by the image's
	// ID, so an image rebuilt under the same name never passes for the
	// one before it; the report gives the name too.
	imageName := settings.Image
	if settings.Image, err = engine.ImageID(ctx, imageName); err != nil {
		return ExitUnreachable, fmt.Errorf("the eval image %s: %w", imageName, err)
	}
	results := path.Join(evals.Dir, "results", commit)
	if merr := root.MkdirAll(results, 0o750); merr != nil {
		return ExitUnreachable, merr
	}
	// Another cech eval on this commit's results may end soon, so a
	// retry may work.
	unlock, err := evalrun.Lock(filepath.Join(dir, filepath.FromSlash(results), ".lock"))
	if err != nil {
		return ExitUnreachable, err
	}
	defer unlock()
	scratch, err := os.MkdirTemp("", "cech-eval-")
	if err != nil {
		return ExitUnreachable, err
	}
	defer func() { _ = os.RemoveAll(scratch) }()

	sink := otlpsink.New()
	otlpDir, stopSink, err := listen(ctx, sink)
	if err != nil {
		return ExitUnreachable, err
	}
	defer stopSink()

	st, err := stage(ctx, dir, commit, scratch, m, covered, install, changes.Latest)
	if err != nil {
		return ExitFailure, err
	}
	report := releasecheck.Report{Commit: commit, Image: imageName, ImageID: settings.Image, Cech: version.Current().Version}
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := map[string]error{}
	for _, h := range harnesses {
		wg.Go(func() {
			hc := harnessCheck{
				engine: engine, settings: settings, harness: h, otlpDir: otlpDir, sink: sink, parallel: opts.parallel,
				dir: dir, fsys: fsys, m: m, byName: byName, changes: changes, cases: cases, stage: st,
				results: results, scratch: filepath.Join(scratch, h),
			}
			hr, herr := hc.run(ctx)
			mu.Lock()
			defer mu.Unlock()
			report.Harnesses = append(report.Harnesses, hr)
			if herr != nil {
				errs[h] = herr
			}
		})
	}
	wg.Wait()
	if ctx.Err() != nil {
		// An interrupted check keeps its finished runs for the next
		// cech eval, but writes no report (criterion 30).
		return ExitFailure, errors.New("interrupted; the finished runs are kept, and the next cech eval reuses them")
	}
	slices.SortFunc(report.Harnesses, func(a, b releasecheck.HarnessReport) int { return strings.Compare(a.Name, b.Name) })
	for _, hr := range report.Harnesses {
		for _, b := range hr.Result.Bars {
			res.Bars = append(res.Bars, barEntry{hr.Name, b.Plugin, b.Case, b.Kind, b.Verdict.String(), b.Runs})
		}
	}
	out, err := releasecheck.Render(report)
	if err != nil {
		return ExitFailure, err
	}
	rel := path.Join(evals.ReportsDir, commit+".md")
	if err := root.MkdirAll(evals.ReportsDir, 0o750); err != nil {
		return ExitUnreachable, err
	}
	if err := root.WriteFile(rel, out, 0o644); err != nil { // #nosec G306 -- a report to commit
		return ExitUnreachable, err
	}
	res.Report = rel
	return exitFor(report, errs)
}

// exitFor is the exit status of a finished check, given each harness's
// error: 3 when every harness error is one a retry may fix and every
// other harness passed, else 1 when a bar fails or has no result
// (criterion 29).
func exitFor(report releasecheck.Report, errs map[string]error) (int, error) {
	var all []error
	for _, hr := range report.Harnesses {
		if e := errs[hr.Name]; e != nil {
			all = append(all, e)
		}
	}
	err := errors.Join(all...)
	retry := len(all) > 0 && !slices.ContainsFunc(all, func(e error) bool { return !errors.Is(e, errUnreachable) })
	othersPassed := !slices.ContainsFunc(report.Harnesses, func(hr releasecheck.HarnessReport) bool {
		return errs[hr.Name] == nil && !hr.Result.Passed()
	})
	switch {
	case retry && othersPassed:
		return ExitUnreachable, err
	case err != nil || !report.Passed():
		return ExitFailure, err
	}
	return ExitOK, nil
}

// errUnreachable marks a harness error that a retry may fix, such as a
// host or the container engine that could not be reached; retryable
// wraps an error with it.
var errUnreachable = errors.New("a retry may work")

type retryableError struct{ err error }

func (e retryableError) Error() string        { return e.err.Error() + "; a retry may work" }
func (e retryableError) Unwrap() error        { return e.err }
func (e retryableError) Is(target error) bool { return target == errUnreachable }

func retryable(err error) error { return retryableError{err} }

// listen starts the OTLP receiver on a Unix socket in a new folder of the
// user's cache, which the forwarder of each batch mounts, so no network
// port of this machine is opened; a socket's path must be short, so it
// is not in a temporary folder that may be deep. It returns the folder
// and a function that stops the receiver and removes the folder.
func listen(ctx context.Context, sink *otlpsink.Sink) (string, func(), error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", nil, err
	}
	if merr := os.MkdirAll(filepath.Join(cache, "cech"), 0o700); merr != nil {
		return "", nil, merr
	}
	dir, err := os.MkdirTemp(filepath.Join(cache, "cech"), "otlp-")
	if err != nil {
		return "", nil, err
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "unix", filepath.Join(dir, evalrun.OTLPSocket))
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	srv := &http.Server{Handler: sink, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return dir, func() { _ = srv.Close(); _ = os.RemoveAll(dir) }, nil
}

// staged are the trees the arms read, each as a commit holds it, so
// nothing uncommitted or ignored reaches a run.
type staged struct {
	// marketplace and latest are the marketplaces the arms install from:
	// the commit's catalogue and plugins, and the latest final release's,
	// or "".
	marketplace, latest string
	// latestVersion is the latest final release's version, or "".
	latestVersion string
	// fixtures holds the covered plugins' eval folders.
	fixtures string
}

// stage exports the trees of the commit and of the latest final release
// into scratch (criteria 1 and 23).
func stage(ctx context.Context, dir, commit, scratch string, m *marketplace.Marketplace, covered []string, install map[string]bool, latest *evalchange.Release) (staged, error) {
	var st staged
	var all, installed, latestPlugins []marketplace.Plugin
	for _, p := range m.Plugins {
		all = append(all, p)
		if install[p.Name] {
			installed = append(installed, p)
		}
	}
	paths := []string{".claude-plugin/marketplace.json"}
	for _, p := range all {
		paths = append(paths, p.Dir()+"/")
	}
	st.marketplace = filepath.Join(scratch, "marketplace")
	if _, err := evalchange.ExportTree(ctx, dir, commit, paths, st.marketplace); err != nil {
		return st, err
	}
	var evalPaths []string
	for _, p := range m.Plugins {
		if slices.Contains(covered, p.Name) {
			evalPaths = append(evalPaths, path.Join(evals.Dir, p.Category, p.Name)+"/")
		}
	}
	st.fixtures = filepath.Join(scratch, "fixtures")
	if _, err := evalchange.ExportTree(ctx, dir, commit, evalPaths, st.fixtures); err != nil {
		return st, err
	}
	if latest == nil {
		return st, nil
	}
	for _, p := range installed {
		if evalchange.InTree(ctx, dir, "refs/tags/"+latest.Tag, p.ManifestPath()) {
			latestPlugins = append(latestPlugins, p)
		}
	}
	st.latest = filepath.Join(scratch, "latest-release")
	st.latestVersion = strings.TrimPrefix(latest.Tag, "v")
	return st, evalchange.ExportRelease(ctx, dir, latest.Tag, latestPlugins, st.latest)
}

// readCases are the cases of the covered plugins, as cech eval --check
// read them; a content case of a changed plugin is compared when the
// latest final release also had it (criterion 1).
func readCases(ctx context.Context, dir string, found []evals.Case, covered []string, changes evalchange.Changes) []releasecheck.Case {
	var cases []releasecheck.Case
	for _, f := range found {
		if !slices.Contains(covered, f.Plugin) {
			continue
		}
		c := releasecheck.Case{Plugin: f.Plugin, Name: f.Name, Family: f.Family, Expect: f.Expect, Graders: f.Graders}
		if f.Family == "content" && changes.Status[f.Plugin] == evalchange.Modified && changes.Latest != nil {
			c.Compared = evalchange.InTree(ctx, dir, "refs/tags/"+changes.Latest.Tag, path.Join(f.Dir, "cech.toml"))
		}
		cases = append(cases, c)
	}
	return cases
}

// planFor estimates a harness's runs for --dry-run: five per arm to
// start, and at most the cap for each arm a bar reads (criterion 28).
func planFor(harness string, cases []releasecheck.Case, s evalrun.Settings, parallel int) planEntry {
	first, most := 0, 0
	read := map[releasecheck.Arm]bool{}
	var plugins []string
	for _, c := range cases {
		if !slices.Contains(plugins, c.Plugin) {
			plugins = append(plugins, c.Plugin)
		}
	}
	for _, p := range plugins {
		for _, b := range releasecheck.Bars(harness, p, cases) {
			for _, a := range b.Reads {
				read[a] = true
			}
		}
	}
	for _, c := range cases {
		for _, a := range c.Arms(harness) {
			first += 5
			if read[a] {
				most += releaseStats.Cap
			} else {
				most += 5
			}
		}
	}
	most = min(most, releaseBudget)
	if parallel <= 0 {
		parallel = s.Parallel[harness]
	}
	minutes := func(runs int) float64 {
		return s.MedianRun[harness].Minutes() * float64(runs) / float64(max(parallel, 1))
	}
	return planEntry{harness, first, most, minutes(first), minutes(most)}
}

// credential finds the one credential of a harness in the environment
// (criterion 20): an API key before a subscription.
func credential(harness string) (evalrun.Credential, error) {
	try := func(name, kind string) (evalrun.Credential, bool) {
		return evalrun.Credential{Kind: kind, Env: name}, os.Getenv(name) != ""
	}
	if harness == evals.HarnessClaude {
		if c, ok := try("ANTHROPIC_API_KEY", "an API key"); ok {
			return c, nil
		}
		if c, ok := try("CLAUDE_CODE_OAUTH_TOKEN", "a subscription"); ok {
			return c, nil
		}
		return evalrun.Credential{}, errors.New("claude-code: no credential: set ANTHROPIC_API_KEY, or CLAUDE_CODE_OAUTH_TOKEN from claude setup-token")
	}
	if c, ok := try("CODEX_API_KEY", "an API key"); ok {
		return c, nil
	}
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return evalrun.Credential{}, err
		}
		home = filepath.Join(h, ".codex")
	}
	login := filepath.Join(home, "auth.json")
	if _, err := os.Stat(login); err != nil { // #nosec G703 -- the user's own Codex login
		return evalrun.Credential{}, errors.New("codex: no credential: set CODEX_API_KEY, or log in with codex login")
	}
	return evalrun.Credential{Kind: "a subscription", LoginFile: login}, nil
}

// harnessCheck is the release check on one harness.
type harnessCheck struct {
	engine   *evalrun.Engine
	settings evalrun.Settings
	harness  string
	otlpDir  string
	sink     *otlpsink.Sink
	parallel int
	dir      string
	fsys     fs.FS
	m        *marketplace.Marketplace
	byName   map[string]marketplace.Plugin
	changes  evalchange.Changes
	cases    []releasecheck.Case
	stage    staged
	results  string
	scratch  string
}

// noResult is a harness report whose every bar has no result, for a
// harness that could not run (criterion 29).
func (c harnessCheck) noResult(hr releasecheck.HarnessReport) releasecheck.HarnessReport {
	hr.Result = releasecheck.Result{Harness: c.harness}
	for _, p := range uniquePlugins(c.cases) {
		for _, b := range releasecheck.Bars(c.harness, p, c.cases) {
			hr.Result.Bars = append(hr.Result.Bars, releasecheck.BarResult{BarSpec: b, Verdict: evalstats.NoResult})
		}
	}
	return hr
}

func (c harnessCheck) run(ctx context.Context) (releasecheck.HarnessReport, error) {
	s := c.settings
	hr := releasecheck.HarnessReport{Name: c.harness, Model: s.Models[c.harness]}
	cred, err := credential(c.harness)
	if err != nil {
		hr.Credential = "none"
		return c.noResult(hr), err
	}
	hr.Credential = cred.Kind
	if merr := os.MkdirAll(c.scratch, 0o700); merr != nil {
		return c.noResult(hr), merr
	}
	if cred.LoginFile != "" {
		if rerr := evalrun.RefreshCodexLogin(ctx, c.engine, s, cred.LoginFile, c.scratch); rerr != nil {
			return c.noResult(hr), retryable(fmt.Errorf("codex: checking the ChatGPT login with codex login status failed, so codex has no result: %w", rerr))
		}
	}
	tools := ""
	if c.harness == evals.HarnessClaude {
		tools, err = evalrun.InstallClaudeCode(ctx, c.engine, s)
		if err != nil {
			return c.noResult(hr), retryable(fmt.Errorf("claude-code: %w", err))
		}
	}
	batch, err := evalrun.StartBatch(ctx, c.engine, s, c.harness, c.otlpDir, c.scratch)
	if err != nil {
		return c.noResult(hr), retryable(fmt.Errorf("%s: starting the proxy and its networks: %w", c.harness, err))
	}
	defer batch.Close(context.WithoutCancel(ctx))
	batch.Tools = tools
	hr.Version, err = batch.Version(ctx)
	if err == nil && hr.Version == "" {
		err = errors.New("it printed none")
	}
	if err != nil {
		return c.noResult(hr), retryable(fmt.Errorf("%s: reading its version: %w", c.harness, err))
	}
	h := &releaserun.Harness{
		Name: c.harness, Root: c.dir, FS: c.fsys, Results: c.results, Fixtures: c.stage.fixtures,
		Marketplace: c.stage.marketplace, Latest: c.stage.latest, LatestVersion: c.stage.latestVersion,
		Catalogue: c.m.Data.Namespace, Plugins: c.byName, Needs: c.changes.Needs,
		Credential: cred, Runner: batch, Sink: c.sink, Scratch: c.scratch,
		Key: releaserun.Key{Image: s.Image, Model: s.Models[c.harness], Cech: version.Current().Version, Credential: cred.Kind, Version: hr.Version},
	}
	parallel := c.parallel
	if parallel <= 0 {
		parallel = s.Parallel[c.harness]
	}
	hr.Result = releasecheck.Check(ctx, c.harness, c.cases, releasecheck.Config{Stats: releaseStats, Budget: releaseBudget, Parallel: parallel}, h.Run)
	hr.Skills = h.Costs(c.cases)
	hr.Usage = h.Usage(c.cases)
	if hr.Result.Aborted != nil {
		return hr, fmt.Errorf("%s: stopped: %w", c.harness, hr.Result.Aborted)
	}
	if noRunGaveResult(hr.Result) {
		// Not one run gave a result: the likeliest cause is a host the
		// proxy could not reach, so a retry may work.
		return hr, retryable(fmt.Errorf("%s: every run errored; check that its hosts are reachable", c.harness))
	}
	return hr, nil
}

// noRunGaveResult reports whether a harness made runs and not one of
// them gave a result.
func noRunGaveResult(r releasecheck.Result) bool {
	return r.Runs > 0 && !slices.ContainsFunc(r.Arms, func(a releasecheck.ArmResult) bool { return a.Runs > 0 })
}

func uniquePlugins(cases []releasecheck.Case) []string {
	var out []string
	for _, c := range cases {
		if !slices.Contains(out, c.Plugin) {
			out = append(out, c.Plugin)
		}
	}
	return out
}

// gitOut runs git in dir and returns its output without the last line
// break.
func gitOut(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- fixed git arguments
	cmd.Dir = dir
	cmd.Env = gitenv.Env()
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}
