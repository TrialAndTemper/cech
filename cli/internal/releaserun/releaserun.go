// Package releaserun makes the runs of the release check on one harness
// (eval-runs spec criteria 9 to 12, 23 to 25 and 29 to 31): for each
// run the scheduler of internal/releasecheck asks for, it reuses a run an
// earlier cech eval finished, or builds the run from the tree being
// checked, has internal/evalrun make it, keeps its telemetry, and grades
// it as cech eval --check grades a recording.
package releaserun

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/TrialAndTemper/cech/cli/internal/evalrun"
	"github.com/TrialAndTemper/cech/cli/internal/evals"
	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
	"github.com/TrialAndTemper/cech/cli/internal/otlpsink"
	"github.com/TrialAndTemper/cech/cli/internal/releasecheck"
)

// Runner makes one run; *evalrun.Batch is one.
type Runner interface {
	Execute(ctx context.Context, r evalrun.Run, scratch string) (evalrun.Result, error)
}

// Sink gives what a run exported as telemetry; *otlpsink.Sink is one.
type Sink interface {
	Run(id string) (otlpsink.Telemetry, error)
}

// Key is what a finished run must match to be reused (criterion 31).
type Key struct {
	Image, Model, Cech, Credential, Version string
}

// Harness makes the runs of one harness for one cech eval.
type Harness struct {
	// Name is the harness, as internal/evals names it.
	Name string
	// Root is the checkout, and FS the same folder, from which grading
	// reads the cases, the skills and the run results.
	Root string
	FS   fs.FS
	// Results is the slash path of this commit's results under Root,
	// evals/results/<commit>.
	Results string
	// Fixtures is a folder holding the eval folders of the covered
	// plugins as the commit holds them, so nothing uncommitted or
	// ignored reaches a run.
	Fixtures string
	// Marketplace and Latest are the staged marketplaces the arms
	// install from: the commit's, and the latest final release's, or "".
	Marketplace, Latest string
	// Catalogue is the catalogue name both marketplaces use.
	Catalogue string
	// Plugins are the marketplace's plugins by name, and Needs gives the
	// plugins each depends on, which its arms install too (criterion 23).
	Plugins map[string]marketplace.Plugin
	Needs   func(plugin string) []string
	// LatestVersion is the version of the latest final release, as
	// X.Y.Z, or "".
	LatestVersion string
	Credential    evalrun.Credential
	Key           Key
	Runner        Runner
	Sink          Sink
	// Scratch is a folder for the runs' workspaces and homes.
	Scratch string
}

// runFile is a run folder's run.json, as internal/evalrun writes it.
type runFile struct {
	Error   string  `json:"error,omitempty"`
	Seconds float64 `json:"seconds"`
	Harness string  `json:"harness"`
	Image   string  `json:"image"`
	Model   string  `json:"model"`
	Kind    string  `json:"credential"`
}

// telemetryFile is what a run keeps of its telemetry (criterion 24).
type telemetryFile struct {
	InputTokens         int64             `json:"input_tokens"`
	OutputTokens        int64             `json:"output_tokens"`
	CacheReadTokens     int64             `json:"cache_read_tokens"`
	CacheCreationTokens int64             `json:"cache_creation_tokens"`
	CostUSD             *float64          `json:"cost_usd,omitempty"`
	Requests            int               `json:"requests"`
	MaxAttempt          int64             `json:"max_attempt"`
	Skills              []string          `json:"skills"`
	Plugins             []otlpsink.Plugin `json:"plugins"`
	FirstInput          *int64            `json:"first_input,omitempty"`
}

// The files of a run folder this package writes; GradeRun takes them,
// as evals.ResultFiles lists.
const (
	keyFile       = "key.json"
	telemetryName = "telemetry.json"
	runName       = "run.json"
	loadedName    = "loaded.json"
)

// runDir is the slash path, under Root, of the n-th run of arm, or of its
// rerun.
func (h *Harness) runDir(arm releasecheck.Arm, n int, rerun bool) string {
	d := path.Join(h.Results, h.Name, arm.Case, arm.Name, fmt.Sprint(n))
	if rerun {
		d += "-rerun"
	}
	return d
}

func (h *Harness) abs(rel string) string { return filepath.Join(h.Root, filepath.FromSlash(rel)) }

// installs are the plugins an arm installs: the plugin and those it
// depends on, dependencies first; the latest-release arm installs those
// the release holds.
func (h *Harness) installs(arm releasecheck.Arm) []string {
	if arm.Name == releasecheck.ArmWithout {
		return nil
	}
	var out []string
	if h.Needs != nil {
		out = h.Needs(arm.Plugin)
	}
	out = append(out, arm.Plugin)
	if arm.Name == releasecheck.ArmLatestRelease {
		out = slices.DeleteFunc(out, func(name string) bool {
			p, ok := h.Plugins[name]
			if !ok {
				return true
			}
			_, err := os.Stat(filepath.Join(h.Latest, filepath.FromSlash(p.ManifestPath())))
			return err != nil
		})
	}
	return out
}

// Run makes the n-th run of arm, or its rerun when attempt is 2; it is
// a releasecheck.Executor.
func (h *Harness) Run(ctx context.Context, arm releasecheck.Arm, n, attempt int) releasecheck.Outcome {
	rel := h.runDir(arm, n, attempt > 1)
	if o, ok := h.reuse(arm, rel); ok {
		return o
	}
	if attempt == 1 {
		// An earlier cech eval may have made this run, seen it error and
		// made its rerun, which takes its place for good (criteria 9 and 31).
		if msg, errored := h.erroredBefore(rel); errored {
			if o, ok := h.reuse(arm, h.runDir(arm, n, true)); ok {
				return o
			}
			return releasecheck.Outcome{Err: true, Cause: errors.New(msg)}
		}
	}
	if attempt > 1 {
		// A rerun that errored with this key keeps its place for good
		// (criterion 9).
		if msg, errored := h.erroredBefore(rel); errored {
			return releasecheck.Outcome{Err: true, Cause: errors.New(msg)}
		}
	}
	abs := h.abs(rel)
	if err := os.RemoveAll(abs); err != nil {
		return releasecheck.Outcome{Abort: err}
	}
	r, err := h.build(arm, n, attempt, abs)
	if err != nil {
		return releasecheck.Outcome{Abort: err}
	}
	scratch := filepath.Join(h.Scratch, r.ID)
	rs, err := h.Runner.Execute(ctx, r, scratch)
	_ = os.RemoveAll(scratch)
	if ctx.Err() != nil {
		// An interrupted run is no run: nothing of it is kept, and the
		// next cech eval makes it (criterion 30).
		_ = os.RemoveAll(abs)
		return releasecheck.Outcome{Err: true, Cause: ctx.Err()}
	}
	if err != nil {
		return releasecheck.Outcome{Abort: fmt.Errorf("%s: %w", rel, err)}
	}
	tel, terr := h.Sink.Run(r.ID)
	if werr := writeTelemetry(abs, tel); werr != nil {
		return releasecheck.Outcome{Abort: werr}
	}
	if rs.Err == nil && terr != nil {
		// A run whose telemetry cannot be read counts as a run that
		// errors (criterion 24); run.json says so, for a later cech
		// eval.
		rs.Err = fmt.Errorf("telemetry: %w", terr)
		if merr := markError(abs, rs.Err); merr != nil {
			return releasecheck.Outcome{Abort: merr}
		}
	}
	// The key goes with a run that errored too, so only a later check
	// with the same key keeps its error (criterion 31).
	if werr := writeKey(abs, h.Key); werr != nil {
		return releasecheck.Outcome{Abort: werr}
	}
	if rs.Err != nil {
		return releasecheck.Outcome{Err: true, Cause: scrubbed(rs.Err)}
	}
	return h.grade(arm, rel, rs.Loaded)
}

// build makes the run's description from the tree being checked.
func (h *Harness) build(arm releasecheck.Arm, n, attempt int, out string) (evalrun.Run, error) {
	p, ok := h.Plugins[arm.Plugin]
	if !ok {
		return evalrun.Run{}, fmt.Errorf("no plugin %s", arm.Plugin)
	}
	evalDir := path.Join(evals.Dir, p.Category, p.Name)
	caseDir := path.Join(evalDir, arm.Case)
	fixture := func(rel string) string { return filepath.Join(h.Fixtures, filepath.FromSlash(rel)) }
	prompt, err := os.ReadFile(fixture(path.Join(caseDir, "prompt.md"))) // #nosec G304 -- the commit's eval folder
	if err != nil {
		return evalrun.Run{}, err
	}
	r := evalrun.Run{
		Harness: h.Name, ID: fmt.Sprintf("%s-%s-%s-%d-%d", h.Name, arm.Case, arm.Name, n, attempt),
		Prompt: string(prompt), Project: fixture(path.Join(evalDir, "project")),
		Credential: h.Credential, Out: out,
	}
	for _, staged := range []string{path.Join(caseDir, "project.staged"), path.Join(evalDir, "project.staged")} {
		if info, serr := os.Stat(fixture(staged)); serr == nil && info.IsDir() {
			r.Staged = fixture(staged)
			break
		}
	}
	if arm.Name != releasecheck.ArmUnrouted {
		rows, rerr := os.ReadFile(fixture(path.Join(evalDir, "routing.md"))) // #nosec G304 -- the commit's eval folder
		if rerr != nil {
			return evalrun.Run{}, rerr
		}
		r.Routing = strings.Split(strings.TrimRight(string(rows), "\n"), "\n")
	}
	switch arm.Name {
	case releasecheck.ArmWith, releasecheck.ArmRouted, releasecheck.ArmUnrouted:
		r.Marketplace = h.Marketplace
	case releasecheck.ArmLatestRelease:
		r.Marketplace = h.Latest
	}
	if r.Marketplace != "" {
		r.Catalogue, r.Install = h.Catalogue, h.installs(arm)
	}
	return r, nil
}

// sameKey reports whether an earlier cech eval finished the run at rel
// with this check's key.
func (h *Harness) sameKey(rel string) bool {
	src, err := os.ReadFile(filepath.Join(h.abs(rel), keyFile)) // #nosec G304 -- a run folder of this commit's results
	var got Key
	return err == nil && json.Unmarshal(src, &got) == nil && got == h.Key
}

// reuse grades the run at rel when an earlier cech eval finished it with
// the same key and without an error (criterion 31).
func (h *Harness) reuse(arm releasecheck.Arm, rel string) (releasecheck.Outcome, bool) {
	if !h.sameKey(rel) {
		return releasecheck.Outcome{}, false
	}
	if _, errored := h.erroredBefore(rel); errored {
		return releasecheck.Outcome{}, false
	}
	loaded, ok := readLoaded(h.abs(rel))
	if !ok {
		return releasecheck.Outcome{}, false
	}
	return h.grade(arm, rel, loaded), true
}

// erroredBefore reports whether the run at rel finished as a run that
// errors, with this check's key, and why.
func (h *Harness) erroredBefore(rel string) (string, bool) {
	if !h.sameKey(rel) {
		return "", false
	}
	src, err := os.ReadFile(filepath.Join(h.abs(rel), runName)) // #nosec G304 -- a run folder of this commit's results
	if err != nil {
		return "", false
	}
	var f runFile
	if json.Unmarshal(src, &f) != nil || f.Error == "" {
		return "", false
	}
	return f.Error, true
}

// grade grades the finished run at rel as a recording.
func (h *Harness) grade(arm releasecheck.Arm, rel string, loaded evalrun.Loaded) releasecheck.Outcome {
	p := h.Plugins[arm.Plugin]
	caseDir := path.Join(evals.Dir, p.Category, p.Name, arm.Case)
	v, err := evals.GradeRun(h.FS, p.SkillPath(), p.Name, caseDir, rel, h.Name)
	if err != nil {
		cause := scrubbed(fmt.Errorf("grading: %w", err))
		// run.json says so, so no figure reads the run as one that gave a
		// result.
		if merr := markError(h.abs(rel), cause); merr != nil {
			return releasecheck.Outcome{Abort: merr}
		}
		return releasecheck.Outcome{Err: true, Cause: cause}
	}
	return releasecheck.Outcome{
		Pass:      v.Pass || v.Loaded,
		Loaded:    releasecheck.Loaded{Plugins: loaded.Plugins, Skills: loaded.Skills, MCPServers: loaded.MCPServers},
		Installed: h.installs(arm),
		Failed:    v.Failed,
	}
}

// scrubbed is err with its message rid of local paths, since the report
// gives it.
func scrubbed(err error) error {
	msg, _ := evals.Scrub(err.Error()).(string)
	return errors.New(msg)
}

func readLoaded(dir string) (evalrun.Loaded, bool) {
	var l evalrun.Loaded
	src, err := os.ReadFile(filepath.Join(dir, loadedName)) // #nosec G304 -- a run folder of this commit's results
	return l, err == nil && json.Unmarshal(src, &l) == nil
}

func writeKey(dir string, k Key) error {
	out, err := json.Marshal(k, json.Deterministic(true))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, keyFile), append(out, '\n'), 0o600)
}

// markError records err in the run folder's run.json.
func markError(dir string, err error) error {
	p := filepath.Join(dir, runName)
	src, rerr := os.ReadFile(p) // #nosec G304 -- a run folder of this commit's results
	if rerr != nil {
		return rerr
	}
	var f runFile
	if uerr := json.Unmarshal(src, &f); uerr != nil {
		return uerr
	}
	f.Error, _ = evals.Scrub(err.Error()).(string)
	out, merr := json.Marshal(f, json.Deterministic(true))
	if merr != nil {
		return merr
	}
	return os.WriteFile(p, append(out, '\n'), 0o600)
}

func writeTelemetry(dir string, t otlpsink.Telemetry) error {
	f := telemetryFile{
		InputTokens: t.InputTokens, OutputTokens: t.OutputTokens, CacheReadTokens: t.CacheReadTokens,
		CacheCreationTokens: t.CacheCreationTokens, Requests: t.Requests, MaxAttempt: t.MaxAttempt,
		Skills: append([]string{}, t.Skills...), Plugins: append([]otlpsink.Plugin{}, t.Plugins...),
	}
	if t.CostReported {
		f.CostUSD = &t.CostUSD
	}
	if t.FirstInputSeen {
		f.FirstInput = &t.FirstInput
	}
	out, err := json.Marshal(f, json.Deterministic(true))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, telemetryName), append(out, '\n'), 0o600)
}

func readTelemetry(dir string) (telemetryFile, bool) {
	var t telemetryFile
	src, err := os.ReadFile(filepath.Join(dir, telemetryName)) // #nosec G304 -- a run folder of this commit's results
	return t, err == nil && json.Unmarshal(src, &t) == nil
}

// kept returns the folder of the run that counts as the n-th of arm: the
// run, or its rerun, whichever finished with this check's key; "" when
// neither did.
func (h *Harness) kept(arm releasecheck.Arm, n int) string {
	for _, rerun := range []bool{false, true} {
		rel := h.runDir(arm, n, rerun)
		if _, errored := h.erroredBefore(rel); h.sameKey(rel) && !errored {
			return h.abs(rel)
		}
	}
	return ""
}

// made reports whether the n-th run of arm was started, whether or not it
// finished.
func (h *Harness) made(arm releasecheck.Arm, n int) bool {
	_, err := os.Stat(h.abs(h.runDir(arm, n, false)))
	return err == nil
}

// Usage sums the telemetry of each kept run of the cases' arms (ADR 27
// item 6).
func (h *Harness) Usage(cases []releasecheck.Case) releasecheck.Usage {
	var u releasecheck.Usage
	for _, c := range cases {
		for _, a := range c.Arms(h.Name) {
			for n := 1; h.made(a, n); n++ {
				dir := h.kept(a, n)
				if dir == "" {
					continue
				}
				t, ok := readTelemetry(dir)
				if !ok {
					continue
				}
				u.Runs++
				u.InputTokens += t.InputTokens
				u.CacheReadTokens += t.CacheReadTokens
				u.CacheCreationTokens += t.CacheCreationTokens
				u.OutputTokens += t.OutputTokens
				if t.CostUSD != nil {
					u.CostUSD += *t.CostUSD
					u.CostReported = true
				}
			}
		}
	}
	return u
}

// Costs gives criterion 25's figures for each covered plugin's skill
// versions: this commit's, from the arm with the plugin, and the latest
// final release's, from the latest-release arm, each paired with the arm
// without the plugin. The figure is the median, over the k-th kept runs
// of the two arms in each content and control case, of the difference in
// the first request's whole input. A pair whose runs differ in what they
// loaded by more than the plugin's own plugins, skills and MCP servers is
// left out and named.
func (h *Harness) Costs(cases []releasecheck.Case) []releasecheck.SkillCost {
	var plugins []string
	for _, c := range cases {
		if !slices.Contains(plugins, c.Plugin) {
			plugins = append(plugins, c.Plugin)
		}
	}
	var out []releasecheck.SkillCost
	for _, name := range plugins {
		p := h.Plugins[name]
		versions := []struct{ label, arm string }{{"this commit", releasecheck.ArmWith}}
		if h.LatestVersion != "" {
			versions = append(versions, struct{ label, arm string }{h.LatestVersion, releasecheck.ArmLatestRelease})
		}
		for _, v := range versions {
			sc := releasecheck.SkillCost{Plugin: name, Version: v.label}
			if v.arm == releasecheck.ArmWith {
				sc.DescriptionSize = utf8.RuneCountInString(p.Description)
			} else if d, ok := h.latestDescription(p); ok {
				sc.DescriptionSize = d
			}
			var diffs []int64
			for _, c := range cases {
				if c.Plugin != name || c.Family == "trigger" || (v.arm == releasecheck.ArmLatestRelease && !c.Compared) {
					continue
				}
				with := releasecheck.Arm{Harness: h.Name, Plugin: name, Case: c.Name, Name: v.arm}
				without := releasecheck.Arm{Harness: h.Name, Plugin: name, Case: c.Name, Name: releasecheck.ArmWithout}
				for k := 1; h.made(with, k) && h.made(without, k); k++ {
					dw, do := h.kept(with, k), h.kept(without, k)
					if dw == "" || do == "" {
						continue
					}
					tw, okw := readTelemetry(dw)
					to, oko := readTelemetry(do)
					if !okw || !oko || tw.FirstInput == nil || to.FirstInput == nil {
						continue
					}
					if !onlyPluginDiffers(dw, do, h.installs(with)) {
						rel, _ := filepath.Rel(h.Root, dw)
						sc.Skipped = append(sc.Skipped, filepath.ToSlash(rel))
						continue
					}
					diffs = append(diffs, *tw.FirstInput-*to.FirstInput)
				}
			}
			sc.Pairs = len(diffs)
			sc.AddedTokens = median(diffs)
			out = append(out, sc)
		}
	}
	return out
}

// latestDescription is the size of the plugin's description in the
// latest final release, as the staged release holds its manifest.
func (h *Harness) latestDescription(p marketplace.Plugin) (int, bool) {
	if h.Latest == "" {
		return 0, false
	}
	src, err := os.ReadFile(filepath.Join(h.Latest, filepath.FromSlash(p.ManifestPath()))) // #nosec G304 -- the staged release
	if err != nil {
		return 0, false
	}
	var m struct {
		Description string `json:"description"`
	}
	if json.Unmarshal(src, &m, json.RejectUnknownMembers(false)) != nil {
		return 0, false
	}
	return utf8.RuneCountInString(m.Description), true
}

// median is the median of xs, the mean of the middle two for an even
// count, and 0 for none.
func median(xs []int64) int {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	if len(s)%2 == 1 {
		return int(s[len(s)/2])
	}
	return int((s[len(s)/2-1] + s[len(s)/2]) / 2)
}

// onlyPluginDiffers reports whether the two runs loaded the same
// plugins, skills and MCP servers, apart from those of installed.
func onlyPluginDiffers(with, without string, installed []string) bool {
	a, okA := readLoaded(with)
	b, okB := readLoaded(without)
	if !okA || !okB {
		return false
	}
	strip := func(xs []string) []string {
		return slices.DeleteFunc(slices.Clone(xs), func(x string) bool {
			return slices.ContainsFunc(installed, func(p string) bool { return x == p || strings.HasPrefix(x, p+":") })
		})
	}
	return slices.Equal(strip(a.Plugins), strip(b.Plugins)) && slices.Equal(strip(a.Skills), strip(b.Skills)) && slices.Equal(strip(a.MCPServers), strip(b.MCPServers))
}
