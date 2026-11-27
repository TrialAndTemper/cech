package releasecheck

import (
	"context"
	"errors"
	"hash/fnv"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"

	"github.com/TrialAndTemper/cech/cli/internal/evalstats"
)

var stats = evalstats.Settings{
	FalseAlarm: 0.05, Threshold: 0.15, Cap: 500,
	ContentBar: 0.5, LoadBar: 0.7, SkipBar: 0.2,
	ErrorShare: 0.1, ErrorFrom: 20,
}

var cases = []Case{
	{Plugin: "cech-example", Name: "trailer", Family: "content", Expect: "better", Compared: true},
	{Plugin: "cech-example", Name: "control", Family: "control", Expect: "same"},
	{Plugin: "cech-example", Name: "load-1", Family: "trigger", Expect: "load"},
	{Plugin: "cech-example", Name: "skip-1", Family: "trigger", Expect: "skip"},
}

// sim is an executor whose runs pass with a chance set per arm, drawn
// from a seed of the arm and run number, so the outcome of each run is
// the same however the runs are spread over goroutines.
type sim struct {
	mu    sync.Mutex
	rates map[string]float64 // case/arm to the chance a run passes
	errs  map[string]bool    // case/arm whose runs all error
	seen  map[Arm][]int      // run numbers each arm was asked for, first attempts only
	tries int                // second attempts
}

func (s *sim) run(_ context.Context, a Arm, n, attempt int) Outcome {
	s.mu.Lock()
	defer s.mu.Unlock()
	if attempt == 1 {
		if s.seen == nil {
			s.seen = map[Arm][]int{}
		}
		s.seen[a] = append(s.seen[a], n)
	} else {
		s.tries++
	}
	key := a.Case + "/" + a.Name
	if s.errs[key] {
		return Outcome{Err: true}
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(a.String()))
	rng := rand.New(rand.NewPCG(h.Sum64(), uint64(n)))
	return Outcome{Pass: rng.Float64() < s.rates[key]}
}

func verdicts(r Result) map[string]evalstats.Verdict {
	out := map[string]evalstats.Verdict{}
	for _, b := range r.Bars {
		out[b.Kind+" "+b.Case] = b.Verdict
	}
	return out
}

func armRuns(r Result, c, arm string) int {
	for _, a := range r.Arms {
		if a.Case == c && a.Name == arm {
			return a.Runs
		}
	}
	return -1
}

// TestCheckGoodSkill checks a skill that works and did not get worse:
// every bar passes, and the arms no bar reads stay at five runs
// (criteria 1, 2 and 5 to 8).
func TestCheckGoodSkill(t *testing.T) {
	s := &sim{rates: map[string]float64{
		"trailer/with": 0.97, "trailer/latest-release": 0.97, "trailer/without": 0.1,
		"control/with": 0.95, "control/without": 0.95,
		"load-1/routed": 0.97, "load-1/unrouted": 0.6,
		"skip-1/routed": 0.01, "skip-1/unrouted": 0.0,
	}}
	r := Check(context.Background(), "claude-code", cases, Config{Stats: stats, Budget: 10000, Parallel: 4}, s.run)
	for k, v := range verdicts(r) {
		if v != evalstats.Pass {
			t.Errorf("%s: %v; want pass", k, v)
		}
	}
	if !r.Passed() {
		t.Error("Passed() = false")
	}
	for _, a := range [][2]string{{"trailer", "without"}, {"load-1", "unrouted"}, {"skip-1", "unrouted"}} {
		if n := armRuns(r, a[0], a[1]); n != 5 {
			t.Errorf("%s/%s made %d runs; an arm no bar reads stays at 5", a[0], a[1], n)
		}
	}
	// Each arm is asked for its runs in order, with no gap.
	for a, ns := range s.seen {
		sorted := slices.Sorted(slices.Values(ns))
		for i, n := range sorted {
			if n != i+1 {
				t.Errorf("%v: runs %v; want 1 to %d", a, sorted, len(ns))
				break
			}
		}
	}
}

// TestCheckBrokenSkill checks that a content case that seldom passes
// fails, and that a large drop since the latest final release fails.
func TestCheckBrokenSkill(t *testing.T) {
	s := &sim{rates: map[string]float64{
		"trailer/with": 0.2, "trailer/latest-release": 0.95, "trailer/without": 0.1,
		"control/with": 0.9, "control/without": 0.9,
		"load-1/routed": 0.97, "skip-1/routed": 0.01,
	}}
	r := Check(context.Background(), "codex", cases, Config{Stats: stats, Budget: 10000, Parallel: 2}, s.run)
	v := verdicts(r)
	if v[BarContent+" trailer"] != evalstats.Fail || v[BarNoDrop+" trailer"] != evalstats.Fail {
		t.Errorf("verdicts %v; want the content and no-drop bars of trailer to fail", v)
	}
	if r.Passed() {
		t.Error("Passed() = true")
	}
}

// TestCheckBudget checks that bars still unclear when the budget runs
// out have no result (criterion 4).
func TestCheckBudget(t *testing.T) {
	s := &sim{rates: map[string]float64{
		"trailer/with": 0.85, "trailer/latest-release": 0.85,
		"control/with": 0.85, "control/without": 0.85,
		"load-1/routed": 0.8, "skip-1/routed": 0.1,
	}}
	r := Check(context.Background(), "claude-code", cases, Config{Stats: stats, Budget: 60, Parallel: 4}, s.run)
	if !r.BudgetSpent || r.Runs > 60 {
		t.Errorf("BudgetSpent %v, runs %d; want the budget of 60 spent and kept", r.BudgetSpent, r.Runs)
	}
	if v := verdicts(r)[BarControl+" control"]; v != evalstats.NoResult {
		t.Errorf("control bar: %v; want no result", v)
	}
}

// TestCheckErrors checks criteria 9 and 10: a run that errors is run once
// more, and a case whose runs keep erroring has no result.
func TestCheckErrors(t *testing.T) {
	s := &sim{
		rates: map[string]float64{"trailer/with": 0.97, "trailer/latest-release": 0.97, "load-1/routed": 0.97, "skip-1/routed": 0.0},
		errs:  map[string]bool{"control/with": true, "control/without": true},
	}
	r := Check(context.Background(), "claude-code", cases, Config{Stats: stats, Budget: 10000, Parallel: 4}, s.run)
	if s.tries == 0 {
		t.Error("no run was made again")
	}
	if v := verdicts(r)[BarControl+" control"]; v != evalstats.NoResult {
		t.Errorf("control bar: %v; want no result", v)
	}
	if !slices.Contains(r.NoResult, "cech-example/control") {
		t.Errorf("NoResult = %v; want the control case", r.NoResult)
	}
}

// TestCheckTriggerErrors checks that a trigger case whose runs all error
// gives the pooled bars that read it no result, and stops its arms once
// that is clear rather than growing them to the cap (criterion 10).
func TestCheckTriggerErrors(t *testing.T) {
	s := &sim{
		rates: map[string]float64{"trailer/with": 0.97, "trailer/latest-release": 0.97, "skip-1/routed": 0.0},
		errs:  map[string]bool{"load-1/routed": true, "load-1/unrouted": true},
	}
	r := Check(context.Background(), "claude-code", cases, Config{Stats: stats, Budget: 10000, Parallel: 4}, s.run)
	if v := verdicts(r)[BarLoad+" "]; v != evalstats.NoResult {
		t.Errorf("load bar: %v; want no result", v)
	}
	if n := len(s.seen[Arm{"claude-code", "cech-example", "load-1", ArmRouted}]); n > stats.ErrorFrom {
		t.Errorf("the routed arm of load-1 made %d runs; want at most %d", n, stats.ErrorFrom)
	}
}

// TestArmsAndBars checks which arms each family runs and which bars
// read them (criteria 1 and 5 to 8).
func TestArmsAndBars(t *testing.T) {
	names := func(as []Arm) []string {
		var out []string
		for _, a := range as {
			out = append(out, a.Name)
		}
		return out
	}
	if got := names(cases[0].Arms("codex")); !slices.Equal(got, []string{ArmWith, ArmWithout, ArmLatestRelease}) {
		t.Errorf("compared content arms = %v", got)
	}
	if got := names(Case{Family: "content"}.Arms("codex")); !slices.Equal(got, []string{ArmWith, ArmWithout}) {
		t.Errorf("new content arms = %v", got)
	}
	if got := names(cases[2].Arms("codex")); !slices.Equal(got, []string{ArmRouted, ArmUnrouted}) {
		t.Errorf("trigger arms = %v", got)
	}
	var kinds []string
	for _, b := range Bars("codex", "cech-example", cases) {
		kinds = append(kinds, b.Kind)
	}
	if want := []string{BarContent, BarNoDrop, BarControl, BarLoad, BarSkip}; !slices.Equal(kinds, want) {
		t.Errorf("bars = %v; want %v", kinds, want)
	}
}

// TestCheckBaseline checks criterion 12: a run with the plugin that
// loaded what no run without it loaded counts as a run that errors, its
// own plugin's skills aside, and the error says why.
func TestCheckBaseline(t *testing.T) {
	extra := func(_ context.Context, a Arm, _, _ int) Outcome {
		o := Outcome{Pass: true, Loaded: Loaded{Skills: []string{"builtin"}}}
		if a.Name != ArmWithout {
			o.Installed = []string{a.Plugin}
			o.Loaded.Skills = append(o.Loaded.Skills, a.Plugin+":"+a.Plugin)
			if a.Case == "trailer" && a.Name == ArmWith {
				o.Loaded.MCPServers = []string{"surprise"}
			}
		}
		return o
	}
	r := Check(context.Background(), "claude-code", cases, Config{Stats: stats, Budget: 10000, Parallel: 4}, extra)
	for _, a := range r.Arms {
		switch {
		case a.Case == "trailer" && a.Name == ArmWith:
			if a.Runs != 0 || a.Errors == 0 {
				t.Errorf("trailer/with: %d runs, %d errors; want every run outside the baseline", a.Runs, a.Errors)
			}
		case a.Errors != 0:
			t.Errorf("%s/%s: %d errors; want none, since its plugin's own skill is allowed", a.Case, a.Name, a.Errors)
		}
	}
	if !slices.ContainsFunc(r.Errors, func(e RunError) bool { return e.Message == errOutside }) {
		t.Errorf("Errors = %v; want the baseline named", r.Errors)
	}
}

// TestCheckBaselineGrows checks that a load the baseline gains later
// stops counting as an error, since each check reads the baseline as it
// stands (criterion 12).
func TestCheckBaselineGrows(t *testing.T) {
	states := map[Arm]*armState{}
	with := Arm{"claude-code", "p", "c", ArmWith}
	without := Arm{"claude-code", "p", "c", ArmWithout}
	states[with] = &armState{records: []record{{pass: true, installed: []string{"p"}, loaded: Loaded{MCPServers: []string{"m"}}}}}
	states[without] = &armState{}
	arms := []Arm{with, without}
	if c := count(arms, states); c[with].outside != 1 {
		t.Fatalf("before: %+v; want the run outside the baseline", c[with])
	}
	states[without].records = []record{{pass: true, loaded: Loaded{MCPServers: []string{"m"}}}}
	if c := count(arms, states); c[with].outside != 0 || len(c[with].records) != 1 {
		t.Errorf("after: %+v; want the run counted", c[with])
	}
}

// TestCheckGraders checks that each arm of a content case gives each
// grader's passes (criterion 27), and that a run's errors keep their
// cause.
func TestCheckGraders(t *testing.T) {
	cs := []Case{{Plugin: "cech-example", Name: "trailer", Family: "content", Expect: "better", Graders: []string{"a", "b"}}}
	exec := func(_ context.Context, a Arm, n, attempt int) Outcome {
		if a.Name == ArmWithout {
			if n == 1 && attempt == 1 {
				return Outcome{Err: true, Cause: errors.New("boom")}
			}
			return Outcome{Failed: []string{"b"}}
		}
		return Outcome{Pass: true}
	}
	r := Check(context.Background(), "codex", cs, Config{Stats: stats, Budget: 10000, Parallel: 2}, exec)
	for _, a := range r.Arms {
		want := map[string]int{"a": a.Runs, "b": a.Runs}
		if a.Name == ArmWithout {
			want["b"] = 0
		}
		for _, g := range a.Graders {
			if g.Passes != want[g.Name] {
				t.Errorf("%s grader %s: %d passes; want %d", a.Name, g.Name, g.Passes, want[g.Name])
			}
		}
		if len(a.Graders) != 2 {
			t.Errorf("%s graders = %v; want a and b", a.Name, a.Graders)
		}
	}
	if len(r.Errors) != 0 {
		t.Errorf("Errors = %v; want none, since the rerun took the error's place", r.Errors)
	}
}

// TestCheckAbort checks that a runner that cannot work stops the check,
// whose bars then have no result.
func TestCheckAbort(t *testing.T) {
	calls := 0
	exec := func(context.Context, Arm, int, int) Outcome {
		calls++
		return Outcome{Abort: errors.New("cannot write")}
	}
	r := Check(context.Background(), "codex", cases, Config{Stats: stats, Budget: 10000, Parallel: 1}, exec)
	if r.Aborted == nil || r.Passed() {
		t.Errorf("Aborted %v, Passed %v; want aborted and not passed", r.Aborted, r.Passed())
	}
	arms := 0
	for _, c := range cases {
		arms += len(c.Arms("codex"))
	}
	if calls > step*arms {
		t.Errorf("%d runs; want the check to stop after its first round", calls)
	}
	for _, b := range r.Bars {
		if b.Verdict != evalstats.NoResult {
			t.Errorf("%s: %v; want no result", b.Kind, b.Verdict)
		}
	}
}

// TestCheckRerunCountsAgainstTheBudget checks criterion 9: a rerun
// counts against the budget, not against the arm's cap.
func TestCheckRerunCountsAgainstTheBudget(t *testing.T) {
	cs := []Case{{Plugin: "p", Name: "c", Family: "control", Expect: "same"}}
	exec := func(_ context.Context, _ Arm, _, attempt int) Outcome {
		if attempt == 1 {
			return Outcome{Err: true}
		}
		return Outcome{Pass: true}
	}
	r := Check(context.Background(), "codex", cs, Config{Stats: stats, Budget: 20, Parallel: 1}, exec)
	if r.Runs != 20 || !r.BudgetSpent {
		t.Errorf("Runs %d, BudgetSpent %v; want the 20 of the budget spent", r.Runs, r.BudgetSpent)
	}
	for _, a := range r.Arms {
		if a.Runs != 5 {
			t.Errorf("%s: %d runs; want 5, each rerun in its first run's place", a.Name, a.Runs)
		}
	}
}
