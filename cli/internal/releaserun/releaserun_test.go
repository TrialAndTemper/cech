package releaserun

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/TrialAndTemper/cech/cli/internal/evalrun"
	"github.com/TrialAndTemper/cech/cli/internal/evals"
	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
	"github.com/TrialAndTemper/cech/cli/internal/otlpsink"
	"github.com/TrialAndTemper/cech/cli/internal/releasecheck"
)

// fixture is the eval fixture marketplace the app's tests use.
const fixture = "../app/testdata/eval/complete"

const evalDir = "evals/writing/cech-example"

// fakeRunner makes a run by copying a recording of the fixture into the
// run's folder, with loaded.json and run.json as internal/evalrun writes
// them, and records each run it was asked for.
type fakeRunner struct {
	mu   sync.Mutex
	runs []evalrun.Run
	// recording is the recording each run copies, by case and arm.
	recording func(r evalrun.Run) string
	// loaded is what a run loaded.
	loaded evalrun.Loaded
	// runErr is the error a run ends with; execErr the runner's own.
	runErr, execErr error
}

func (f *fakeRunner) Execute(_ context.Context, r evalrun.Run, _ string) (evalrun.Result, error) {
	f.mu.Lock()
	f.runs = append(f.runs, r)
	f.mu.Unlock()
	if f.execErr != nil {
		return evalrun.Result{}, f.execErr
	}
	if err := os.CopyFS(r.Out, os.DirFS(f.recording(r))); err != nil {
		return evalrun.Result{}, err
	}
	loaded, _ := json.Marshal(f.loaded)
	meta := runFile{Seconds: 1, Harness: r.Harness, Model: "m", Kind: r.Credential.Kind}
	if f.runErr != nil {
		meta.Error = f.runErr.Error()
	}
	runJSON, _ := json.Marshal(meta)
	for name, data := range map[string][]byte{loadedName: loaded, runName: runJSON} {
		if err := os.WriteFile(filepath.Join(r.Out, name), append(data, '\n'), 0o600); err != nil {
			return evalrun.Result{}, err
		}
	}
	return evalrun.Result{Err: f.runErr, Loaded: f.loaded}, nil
}

// fakeSink gives each run the same telemetry, or err.
type fakeSink struct {
	t   otlpsink.Telemetry
	err error
}

func (s fakeSink) Run(string) (otlpsink.Telemetry, error) { return s.t, s.err }

// newHarness copies the fixture into a new folder and returns a Harness
// on it for Claude Code, whose runs copy the trailer recording.
func newHarness(t *testing.T) (*Harness, *fakeRunner) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(fixture)); err != nil {
		t.Fatal(err)
	}
	m, err := marketplace.Load(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	plugins := map[string]marketplace.Plugin{}
	for _, p := range m.Plugins {
		plugins[p.Name] = p
	}
	f := &fakeRunner{recording: func(evalrun.Run) string {
		return filepath.Join(root, evalDir, "cech-example-trailer", "recordings", "claude-code", "trailer")
	}}
	// The latest final release holds the plugin, with a shorter
	// description, but not the plugin it now depends on.
	latest := t.TempDir()
	manifest := filepath.Join(latest, filepath.FromSlash(plugins["cech-example"].ManifestPath()))
	if err := os.MkdirAll(filepath.Dir(manifest), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(`{"name": "cech-example", "description": "Ends commits ünd more"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	plugins["cech-base"] = marketplace.Plugin{Category: "writing", Name: "cech-base"}
	h := &Harness{
		Name: evals.HarnessClaude, Root: root, FS: os.DirFS(root), Results: "evals/results/c0ffee",
		Fixtures: root, Marketplace: "/staged/mkt", Latest: latest, Catalogue: "cech",
		Plugins: plugins, Needs: func(string) []string { return []string{"cech-base"} },
		Credential: evalrun.Credential{Kind: "a subscription", Env: "CLAUDE_CODE_OAUTH_TOKEN"},
		Key:        Key{Image: "img", Model: "m", Cech: "0.1.0", Credential: "a subscription", Version: "2.1.330"},
		Runner:     f, Sink: fakeSink{t: otlpsink.Telemetry{InputTokens: 10, FirstInput: 100, FirstInputSeen: true}},
		Scratch: t.TempDir(),
	}
	return h, f
}

func arm(c, name string) releasecheck.Arm {
	return releasecheck.Arm{Harness: evals.HarnessClaude, Plugin: "cech-example", Case: c, Name: name}
}

// TestRunBuildsEachArm checks what each arm's run gets: its folder, the
// marketplace and plugins it installs, dependencies first, and the
// routing rows on every arm but the unrouted one (criteria 1, 11 and
// 23).
func TestRunBuildsEachArm(t *testing.T) {
	h, f := newHarness(t)
	for _, a := range []releasecheck.Arm{
		arm("cech-example-trailer", releasecheck.ArmWith), arm("cech-example-trailer", releasecheck.ArmWithout),
		arm("cech-example-trailer", releasecheck.ArmLatestRelease),
		arm("cech-example-load-1", releasecheck.ArmRouted), arm("cech-example-load-1", releasecheck.ArmUnrouted),
	} {
		h.Run(t.Context(), a, 1, 1)
	}
	if len(f.runs) != 5 {
		t.Fatalf("%d runs; want 5", len(f.runs))
	}
	want := []struct {
		mkt     string
		install []string
		routed  bool
	}{
		{"/staged/mkt", []string{"cech-base", "cech-example"}, true},
		{"", nil, true},
		{h.Latest, []string{"cech-example"}, true},
		{"/staged/mkt", []string{"cech-base", "cech-example"}, true},
		{"/staged/mkt", []string{"cech-base", "cech-example"}, false},
	}
	for i, r := range f.runs {
		w := want[i]
		if r.Marketplace != w.mkt || !slices.Equal(r.Install, w.install) || (len(r.Routing) > 0) != w.routed {
			t.Errorf("run %s: marketplace %q, install %v, routing %v; want %q, %v, routed %v", r.ID, r.Marketplace, r.Install, r.Routing, w.mkt, w.install, w.routed)
		}
		if !strings.HasPrefix(r.Project, filepath.Join(h.Fixtures, evalDir)) || r.Staged == "" {
			t.Errorf("run %s: project %q, staged %q; want both from the fixtures", r.ID, r.Project, r.Staged)
		}
	}
	if got := f.runs[0].Out; got != filepath.Join(h.Root, "evals", "results", "c0ffee", "claude-code", "cech-example-trailer", "with", "1") {
		t.Errorf("run folder %s", got)
	}
}

// TestRunGradesAndKeeps checks that a finished run is graded, keeps its
// key and telemetry, and is reused, not run again, by a later check with
// the same key, and run again with another (criteria 11, 24 and 31).
func TestRunGradesAndKeeps(t *testing.T) {
	h, f := newHarness(t)
	a := arm("cech-example-trailer", releasecheck.ArmWith)
	o := h.Run(t.Context(), a, 1, 1)
	if !o.Pass || o.Err || o.Abort != nil {
		t.Fatalf("outcome %+v; want a pass", o)
	}
	dir := f.runs[0].Out
	for _, name := range evals.ResultFiles {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("the run folder lacks %s: %v", name, err)
		}
	}
	if o := h.Run(t.Context(), a, 1, 1); !o.Pass || len(f.runs) != 1 {
		t.Errorf("outcome %+v after %d runs; want the run reused", o, len(f.runs))
	}
	h.Key.Version = "2.1.331"
	if h.Run(t.Context(), a, 1, 1); len(f.runs) != 2 {
		t.Errorf("%d runs; want a new run for a new harness version", len(f.runs))
	}
}

// TestRunFailedGraders checks that a run that fails graders names them
// (criterion 27).
func TestRunFailedGraders(t *testing.T) {
	h, f := newHarness(t)
	f.recording = func(evalrun.Run) string {
		return filepath.Join(h.Root, evalDir, "cech-example-trailer", "recordings", "claude-code", "no-trailer")
	}
	o := h.Run(t.Context(), arm("cech-example-trailer", releasecheck.ArmWithout), 1, 1)
	if o.Pass || o.Err || len(o.Failed) == 0 {
		t.Errorf("outcome %+v; want a fail naming graders", o)
	}
}

// TestRunErrors checks a run that errors: its cause is kept without
// local paths, it is not reused, and a run whose telemetry cannot be read
// errors too and says so in run.json (criteria 9, 24 and 31).
func TestRunErrors(t *testing.T) {
	h, f := newHarness(t)
	a := arm("cech-example-trailer", releasecheck.ArmWith)
	f.runErr = errors.New("the run took longer than 10m0s in /home/someone/x")
	o := h.Run(t.Context(), a, 1, 1)
	if !o.Err || o.Cause == nil || !strings.HasSuffix(o.Cause.Error(), " in /cech/home/someone/x") {
		t.Errorf("outcome %+v; want an error with its cause scrubbed", o)
	}
	f.runErr = nil
	h.Sink = fakeSink{err: otlpsink.ErrNoTelemetry}
	o = h.Run(t.Context(), a, 2, 1)
	if !o.Err || !strings.Contains(o.Cause.Error(), "telemetry") {
		t.Errorf("outcome %+v; want a telemetry error", o)
	}
	if msg, errored := h.erroredBefore(h.runDir(a, 2, false)); !errored || !strings.Contains(msg, "telemetry") {
		t.Errorf("run.json error %q; want the telemetry error kept", msg)
	}
}

// TestRunResumesARerun checks that a later check keeps a rerun in its
// first run's place, and does not make the first run again: criterion
// 9's rerun takes its place for good.
func TestRunResumesARerun(t *testing.T) {
	h, f := newHarness(t)
	a := arm("cech-example-trailer", releasecheck.ArmWith)
	f.runErr = errors.New("boom")
	h.Run(t.Context(), a, 1, 1)
	f.runErr = nil
	h.Run(t.Context(), a, 1, 2)
	runs := len(f.runs)

	o := h.Run(t.Context(), a, 1, 1)
	if !o.Pass || len(f.runs) != runs {
		t.Errorf("outcome %+v after %d new runs; want the kept rerun", o, len(f.runs)-runs)
	}
	// A first run that errored and has no rerun yet errors again without
	// a new run, so the scheduler makes its rerun.
	f.runErr = errors.New("boom")
	h.Run(t.Context(), a, 2, 1)
	runs = len(f.runs)
	if o := h.Run(t.Context(), a, 2, 1); !o.Err || len(f.runs) != runs {
		t.Errorf("outcome %+v after %d new runs; want the earlier error, and no run", o, len(f.runs)-runs)
	}
	// A rerun that errored keeps its place too.
	h.Run(t.Context(), a, 2, 2)
	runs = len(f.runs)
	if o := h.Run(t.Context(), a, 2, 2); !o.Err || len(f.runs) != runs {
		t.Errorf("outcome %+v after %d new runs; want the rerun's error, and no run", o, len(f.runs)-runs)
	}
	// Under another key, the errored run is made again.
	f.runErr = nil
	h.Key.Version = "2.1.331"
	if o := h.Run(t.Context(), a, 2, 1); !o.Pass || len(f.runs) != runs+1 {
		t.Errorf("outcome %+v after %d new runs; want a new run", o, len(f.runs)-runs)
	}
}

// TestRunAborts checks that a runner that cannot work stops the check
// instead of counting as a run that errors.
func TestRunAborts(t *testing.T) {
	h, f := newHarness(t)
	f.execErr = errors.New("cannot write the run folder")
	if o := h.Run(t.Context(), arm("cech-example-trailer", releasecheck.ArmWith), 1, 1); o.Abort == nil {
		t.Errorf("outcome %+v; want an abort", o)
	}
}

// writeRun writes a kept run of arm: its key, its first request's input
// and what it loaded.
func writeRun(t *testing.T, h *Harness, a releasecheck.Arm, n int, rerun bool, first int64, skills ...string) {
	t.Helper()
	dir := h.abs(h.runDir(a, n, rerun))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := writeKey(dir, h.Key); err != nil {
		t.Fatal(err)
	}
	if err := writeTelemetry(dir, otlpsink.Telemetry{InputTokens: first, FirstInput: first, FirstInputSeen: true, CostUSD: 0.01, CostReported: true}); err != nil {
		t.Fatal(err)
	}
	loaded, _ := json.Marshal(evalrun.Loaded{Plugins: []string{}, Skills: append([]string{"builtin"}, skills...), MCPServers: []string{}})
	if err := os.WriteFile(filepath.Join(dir, loadedName), loaded, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestCosts checks criterion 25's figure: the median over pairs of kept
// runs, a rerun taking its run's place, a pair with a run missing left
// out without ending the count, and a pair whose loads differ by more
// than the plugin's named and left out.
func TestCosts(t *testing.T) {
	h, _ := newHarness(t)
	h.Latest = ""
	c := releasecheck.Case{Plugin: "cech-example", Name: "cech-example-control", Family: "control", Expect: "same"}
	with, without := arm(c.Name, releasecheck.ArmWith), arm(c.Name, releasecheck.ArmWithout)
	own := "cech-example:cech-example"
	writeRun(t, h, with, 1, false, 1100, own)
	writeRun(t, h, without, 1, false, 1000)
	// Run 2 of the arm with the plugin errored; its rerun counts.
	if err := os.MkdirAll(h.abs(h.runDir(with, 2, false)), 0o750); err != nil {
		t.Fatal(err)
	}
	writeRun(t, h, with, 2, true, 1130, own)
	writeRun(t, h, without, 2, false, 1000)
	// Run 3 of the arm without it never finished.
	writeRun(t, h, with, 3, false, 9999, own)
	if err := os.MkdirAll(h.abs(h.runDir(without, 3, false)), 0o750); err != nil {
		t.Fatal(err)
	}
	// Run 4 loaded a skill that run 4 without the plugin did not.
	writeRun(t, h, with, 4, false, 5000, own, "surprise")
	writeRun(t, h, without, 4, false, 1000)
	writeRun(t, h, with, 5, false, 1140, own)
	writeRun(t, h, without, 5, false, 1000)
	writeRun(t, h, with, 6, false, 1150, own)
	writeRun(t, h, without, 6, false, 1000)

	costs := h.Costs([]releasecheck.Case{c})
	if len(costs) != 1 {
		t.Fatalf("costs %+v; want this commit's alone", costs)
	}
	got := costs[0]
	// Pairs 1, 2, 5 and 6: 100, 130, 140 and 150, whose median is 135.
	if got.Pairs != 4 || got.AddedTokens != 135 || len(got.Skipped) != 1 || !strings.HasSuffix(got.Skipped[0], path.Join(c.Name, "with", "4")) {
		t.Errorf("cost %+v; want 4 pairs, 135 tokens, and run 4 left out", got)
	}
	if u := h.Usage([]releasecheck.Case{c}); u.Runs != 11 || !u.CostReported {
		t.Errorf("usage %+v; want the 11 kept runs, with a cost", u)
	}
}

// TestCostsOfTheLatestRelease checks criterion 25's second version: the
// latest final release's, from the latest-release arm of compared cases
// only, with the release's description size.
func TestCostsOfTheLatestRelease(t *testing.T) {
	h, _ := newHarness(t)
	h.LatestVersion = "0.1.0"
	compared := releasecheck.Case{Plugin: "cech-example", Name: "cech-example-trailer", Family: "content", Expect: "better", Compared: true}
	added := releasecheck.Case{Plugin: "cech-example", Name: "cech-example-control", Family: "control", Expect: "same"}
	own := "cech-example:cech-example"
	for k, first := range []int64{1090, 1110} {
		writeRun(t, h, arm(compared.Name, releasecheck.ArmLatestRelease), k+1, false, first, own)
		writeRun(t, h, arm(compared.Name, releasecheck.ArmWithout), k+1, false, 1000)
		// A case the release did not have gives no pair, even with runs.
		writeRun(t, h, arm(added.Name, releasecheck.ArmLatestRelease), k+1, false, 9000, own)
		writeRun(t, h, arm(added.Name, releasecheck.ArmWithout), k+1, false, 1000)
	}
	costs := h.Costs([]releasecheck.Case{compared, added})
	if len(costs) != 2 {
		t.Fatalf("costs %+v; want this commit's and the release's", costs)
	}
	if got := costs[1]; got.Version != "0.1.0" || got.Pairs != 2 || got.AddedTokens != 100 || got.DescriptionSize != 21 {
		t.Errorf("release cost %+v; want 0.1.0, 2 pairs, 100 tokens and 21 characters", got)
	}
}

// TestRunInterrupted checks that a run an interrupt stopped leaves
// nothing, so the next cech eval makes it again (criterion 30).
func TestRunInterrupted(t *testing.T) {
	h, f := newHarness(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	f.runErr = context.Canceled
	a := arm("cech-example-trailer", releasecheck.ArmWith)
	if o := h.Run(ctx, a, 1, 1); !o.Err {
		t.Errorf("outcome %+v; want an error", o)
	}
	if _, err := os.Stat(h.abs(h.runDir(a, 1, false))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the interrupted run's folder is kept: %v", err)
	}
}
