package releasecheck

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/TrialAndTemper/cech/cli/internal/evalstats"
)

// Outcome is what one run gave: whether it passed (or, for a trigger
// run, loaded the skill), or that it errored, and why.
type Outcome struct {
	Pass bool
	Err  bool
	// Cause is why the run errored, which the report gives.
	Cause error
	// Abort, when set, stops the harness's check, since the runner
	// itself cannot work, such as a results folder it cannot write.
	Abort error
	// Loaded is what the run's harness loaded, and Installed the plugins
	// its arm installed, which the baseline of criterion 12 judges.
	Loaded    Loaded
	Installed []string
	// Failed are the graders the run failed, for a content or control
	// case (criterion 27).
	Failed []string
}

// Loaded is what a run's harness loaded (criterion 12).
type Loaded struct {
	Plugins, Skills, MCPServers []string
}

// Executor makes the n-th run of arm, counting from 1, or reuses one that
// an earlier cech eval finished (criterion 31). attempt is 1, or 2 for
// the rerun of a run that errored (criterion 9).
type Executor func(ctx context.Context, arm Arm, n, attempt int) Outcome

// Config is what one harness's check reads.
type Config struct {
	Stats evalstats.Settings
	// Budget is the most runs this harness may make, reruns and reused
	// runs included (criteria 2, 4 and 31).
	Budget int
	// Parallel is how many runs share the harness at once.
	Parallel int
}

// The runs each arm starts with, and adds while a bar that reads it is
// unclear (criteria 1 and 2).
const step = 5

// BarResult is a bar as the report gives it.
type BarResult struct {
	BarSpec
	Verdict  evalstats.Verdict
	Interval evalstats.Interval
	// Runs are the runs with a result in each arm it read.
	Runs []int
}

// ArmResult is an arm as the report gives it.
type ArmResult struct {
	Arm
	Runs, Passes, Errors int
	Interval             evalstats.Interval
	// Graders are the pass counts of the case's graders over Runs.
	Graders []GraderResult
}

// GraderResult is how many of an arm's runs one grader passed.
type GraderResult struct {
	Name   string
	Passes int
}

// RunError is one reason runs errored, and how many it holds.
type RunError struct {
	Message string
	Runs    int
}

// Result is the release check on one harness.
type Result struct {
	Harness string
	Bars    []BarResult
	Arms    []ArmResult
	// NoResult are the cases with no result under criterion 10.
	NoResult []string
	// Runs is how many runs it made or reused, reruns included.
	Runs int
	// BudgetSpent is set when the budget ran out.
	BudgetSpent bool
	// Errors are why runs errored, most runs first.
	Errors []RunError
	// Aborted is why the check stopped early, or nil.
	Aborted error
}

// Passed reports whether every bar passed.
func (r Result) Passed() bool {
	return r.Aborted == nil && !slices.ContainsFunc(r.Bars, func(b BarResult) bool { return b.Verdict != evalstats.Pass })
}

// record is one run with a result.
type record struct {
	pass      bool
	loaded    Loaded
	installed []string
	failed    []string
}

type armState struct {
	records []record
	made    int // runs started, so the next run's number is made+1
	errors  int
	target  int
}

// counted are an arm's runs as a check reads them: the records within
// the baseline, and how many records fell outside it.
type counted struct {
	records []record
	outside int
}

func (c counted) outcomes() []bool {
	out := make([]bool, len(c.records))
	for i, r := range c.records {
		out[i] = r.pass
	}
	return out
}

// baseline is everything the runs without the plugin loaded (criterion
// 12).
func baseline(arms []Arm, states map[Arm]*armState) Loaded {
	var b Loaded
	for _, a := range arms {
		if a.Name != ArmWithout {
			continue
		}
		for _, r := range states[a].records {
			b.Plugins = append(b.Plugins, r.loaded.Plugins...)
			b.Skills = append(b.Skills, r.loaded.Skills...)
			b.MCPServers = append(b.MCPServers, r.loaded.MCPServers...)
		}
	}
	return b
}

// outside reports whether r loaded anything that is neither in the
// baseline nor its own arm's plugins, theirs being named after them.
func (r record) outside(base Loaded) bool {
	own := func(name string) bool {
		return slices.ContainsFunc(r.installed, func(p string) bool {
			return name == p || strings.HasPrefix(name, p+":") || strings.HasPrefix(name, "plugin:"+p+":")
		})
	}
	for _, l := range []struct{ got, base []string }{
		{r.loaded.Plugins, base.Plugins}, {r.loaded.Skills, base.Skills}, {r.loaded.MCPServers, base.MCPServers},
	} {
		for _, n := range l.got {
			if !slices.Contains(l.base, n) && !own(n) {
				return true
			}
		}
	}
	return false
}

// count reads each arm against the baseline as it stands, so a verdict
// can change as the baseline grows (criterion 12).
func count(arms []Arm, states map[Arm]*armState) map[Arm]counted {
	base := baseline(arms, states)
	out := map[Arm]counted{}
	for _, a := range arms {
		var c counted
		for _, r := range states[a].records {
			if a.Name != ArmWithout && r.outside(base) {
				c.outside++
				continue
			}
			c.records = append(c.records, r)
		}
		out[a] = c
	}
	return out
}

// errOutside is the cause of a run that loaded more than the baseline.
const errOutside = "loaded a plugin, skill or MCP server that no run without the plugin loaded"

// Check runs the release check of cases on one harness: every arm starts
// at five runs, and while a bar is unclear five more runs go to each arm
// it reads, up to the cap and the budget (criteria 1 to 10).
func Check(ctx context.Context, harness string, cases []Case, cfg Config, run Executor) Result {
	res := Result{Harness: harness}
	states := map[Arm]*armState{}
	graders := map[Arm][]string{}
	var arms []Arm
	var bars []BarSpec
	var plugins []string
	for _, c := range cases {
		for _, a := range c.Arms(harness) {
			arms = append(arms, a)
			states[a] = &armState{target: step}
			graders[a] = c.Graders
		}
		if !slices.Contains(plugins, c.Plugin) {
			plugins = append(plugins, c.Plugin)
		}
	}
	for _, p := range plugins {
		bars = append(bars, Bars(harness, p, cases)...)
	}
	verdicts := make([]evalstats.Verdict, len(bars))
	intervals := make([]evalstats.Interval, len(bars))
	noResult := map[string]bool{} // plugin/case
	caseKey := func(a Arm) string { return a.Plugin + "/" + a.Case }
	// A bar has no result once any case it reads has none, so a pooled
	// trigger bar stops growing a case whose runs error.
	barNoResult := func(b BarSpec) bool {
		return slices.ContainsFunc(b.Reads, func(a Arm) bool { return noResult[caseKey(a)] })
	}
	causes := map[string]int{}
	caseErrors := func(c Case, counts map[Arm]counted) (errs, runs int) {
		for _, a := range c.Arms(harness) {
			errs += states[a].errors + counts[a].outside
			runs += states[a].made
		}
		return errs, runs
	}

	var counts map[Arm]counted
	for ctx.Err() == nil && res.Aborted == nil {
		jobs := plan(arms, states, cfg.Budget-res.Runs)
		if len(jobs) == 0 {
			break
		}
		res.Runs += len(jobs)
		outcomes := runAll(ctx, jobs, cfg.Parallel, run)
		for i, j := range jobs {
			o := outcomes[i]
			st := states[j.arm]
			if o.Abort == nil && o.Err && res.Runs < cfg.Budget && ctx.Err() == nil {
				res.Runs++
				o = run(ctx, j.arm, j.n, 2)
			}
			switch {
			case o.Abort != nil:
				if res.Aborted == nil {
					res.Aborted = o.Abort
				}
			case o.Err:
				st.errors++
				msg := "unknown"
				if o.Cause != nil {
					msg = o.Cause.Error()
				}
				causes[msg]++
			default:
				st.records = append(st.records, record{pass: o.Pass, loaded: o.Loaded, installed: o.Installed, failed: o.Failed})
			}
		}
		if res.Aborted != nil {
			break
		}
		counts = count(arms, states)
		// Criterion 10, read from the ErrorFrom-th run of a case on.
		for _, c := range cases {
			if errs, runs := caseErrors(c, counts); cfg.Stats.TooManyErrors(errs, runs, false) {
				noResult[c.Plugin+"/"+c.Name] = true
			}
		}
		budgetLeft := cfg.Budget - res.Runs
		var unclear []evalstats.Bar
		var unclearIdx []int
		// Each round reads every bar again, since the baseline of
		// criterion 12 can move runs in or out of the counts; a bar that
		// turns unclear grows again.
		for i, b := range bars {
			if verdicts[i] == evalstats.NoResult {
				continue
			}
			if barNoResult(b) {
				verdicts[i] = evalstats.NoResult
				continue
			}
			o := Outcomes{}
			for _, a := range b.Reads {
				o[a] = counts[a].outcomes()
			}
			verdicts[i], intervals[i] = Decide(b, o, cfg.Stats)
			if verdicts[i] != evalstats.Unclear {
				continue
			}
			atCap := slices.ContainsFunc(b.Reads, func(a Arm) bool { return states[a].made >= cfg.Stats.Cap })
			verdicts[i] = evalstats.Settle(verdicts[i], atCap, budgetLeft <= 0)
			if verdicts[i] == evalstats.Unclear {
				var runs []int
				for _, a := range b.Reads {
					runs = append(runs, states[a].made)
				}
				unclear = append(unclear, evalstats.Bar{Name: b.Kind, Case: b.Plugin + "/" + b.Case, Verdict: evalstats.Unclear, Runs: runs})
				unclearIdx = append(unclearIdx, i)
			}
		}
		// While the budget lasts, the next runs go to the unclear bars in
		// the order of criterion 2.
		for len(unclear) > 0 && budgetLeft > 0 {
			k := evalstats.Next(unclear)
			for _, a := range bars[unclearIdx[k]].Reads {
				// An arm two bars read grows once a round.
				st := states[a]
				if st.target == st.made && st.target < cfg.Stats.Cap {
					add := min(step, cfg.Stats.Cap-st.target)
					st.target += add
					budgetLeft -= add
				}
			}
			unclear = slices.Delete(unclear, k, k+1)
			unclearIdx = slices.Delete(unclearIdx, k, k+1)
		}
	}
	if cfg.Budget-res.Runs <= 0 {
		res.BudgetSpent = true
	}
	counts = count(arms, states)
	for _, c := range cases {
		if errs, runs := caseErrors(c, counts); cfg.Stats.TooManyErrors(errs, runs, true) {
			noResult[c.Plugin+"/"+c.Name] = true
		}
	}
	for i, b := range bars {
		v := evalstats.Settle(verdicts[i], true, true)
		if barNoResult(b) || res.Aborted != nil {
			v = evalstats.NoResult
		}
		var runs []int
		for _, a := range b.Reads {
			runs = append(runs, len(counts[a].records))
		}
		res.Bars = append(res.Bars, BarResult{BarSpec: b, Verdict: v, Interval: intervals[i], Runs: runs})
	}
	for _, a := range arms {
		st, c := states[a], counts[a]
		if c.outside > 0 {
			causes[errOutside] += c.outside
		}
		ar := ArmResult{
			Arm: a, Runs: len(c.records), Errors: st.errors + c.outside,
			Interval: evalstats.RateInterval(c.outcomes(), cfg.Stats.FalseAlarm),
		}
		for _, r := range c.records {
			if r.pass {
				ar.Passes++
			}
		}
		for _, g := range graders[a] {
			gr := GraderResult{Name: g}
			for _, r := range c.records {
				if !slices.Contains(r.failed, g) {
					gr.Passes++
				}
			}
			ar.Graders = append(ar.Graders, gr)
		}
		res.Arms = append(res.Arms, ar)
		if noResult[caseKey(a)] && !slices.Contains(res.NoResult, caseKey(a)) {
			res.NoResult = append(res.NoResult, caseKey(a))
		}
	}
	for msg, n := range causes {
		res.Errors = append(res.Errors, RunError{msg, n})
	}
	slices.SortFunc(res.Errors, func(a, b RunError) int {
		if a.Runs != b.Runs {
			return b.Runs - a.Runs
		}
		return strings.Compare(a.Message, b.Message)
	})
	return res
}

type job struct {
	arm Arm
	n   int
}

// plan lists the runs this round makes: each arm up to its target, as
// far as budget allows, arms in the order of the cases.
func plan(arms []Arm, states map[Arm]*armState, budget int) []job {
	var jobs []job
	for _, a := range arms {
		st := states[a]
		for st.made < st.target && len(jobs) < budget {
			st.made++
			jobs = append(jobs, job{a, st.made})
		}
	}
	return jobs
}

// runAll makes the jobs, parallel at a time, and returns their outcomes
// in the jobs' order, so each arm's outcomes keep the order of its runs.
func runAll(ctx context.Context, jobs []job, parallel int, run Executor) []Outcome {
	out := make([]Outcome, len(jobs))
	sem := make(chan struct{}, max(parallel, 1))
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			out[i] = run(ctx, j.arm, j.n, 1)
		}()
	}
	wg.Wait()
	return out
}
