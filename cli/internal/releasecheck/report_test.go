package releasecheck

import (
	"flag"
	"os"
	"testing"

	"github.com/TrialAndTemper/cech/cli/internal/evalstats"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// TestRender checks the report file against its golden copy, which
// lint:markdown also checks, so the template writes Markdown in the
// house form (criterion 27).
func TestRender(t *testing.T) {
	arm := func(c, name string) Arm { return Arm{"claude-code", "cech-example", c, name} }
	r := Report{
		Commit:  "0123456789abcdef0123456789abcdef01234567",
		Image:   "ghcr.io/trialandtemper/cech-eval@sha256:0000000000000000000000000000000000000000000000000000000000000000",
		ImageID: "1111111111111111111111111111111111111111111111111111111111111111",
		Cech:    "0.1.0",
		Harnesses: []HarnessReport{{
			Name: "claude-code", Version: "2.1.332", Model: "claude-haiku-5-5", Credential: "a subscription",
			Result: Result{
				Harness: "claude-code", Runs: 681,
				Bars: []BarResult{
					{BarSpec: BarSpec{BarContent, "claude-code", "cech-example", "trailer", []Arm{arm("trailer", ArmWith)}}, Verdict: evalstats.Pass, Interval: evalstats.Interval{Low: 0.82, High: 1}, Runs: []int{60}},
					{BarSpec: BarSpec{BarNoDrop, "claude-code", "cech-example", "trailer", []Arm{arm("trailer", ArmWith), arm("trailer", ArmLatestRelease)}}, Verdict: evalstats.Pass, Interval: evalstats.Interval{Low: -0.12, High: 0.08}, Runs: []int{60, 60}},
					{BarSpec: BarSpec{BarLoad, "claude-code", "cech-example", "", []Arm{arm("load-1", ArmRouted)}}, Verdict: evalstats.NoResult, Interval: evalstats.Interval{Low: 0.61, High: 0.95}, Runs: []int{500}},
				},
				Arms: []ArmResult{
					{Arm: arm("trailer", ArmWith), Runs: 60, Passes: 58, Interval: evalstats.Interval{Low: 0.82, High: 1}, Graders: []GraderResult{{"trailer-line", 58}, {"file-written", 60}}},
					{Arm: arm("trailer", ArmWithout), Runs: 60, Passes: 3, Errors: 1, Interval: evalstats.Interval{Low: 0, High: 0.18}, Graders: []GraderResult{{"trailer-line", 3}, {"file-written", 60}}},
					{Arm: arm("trailer", ArmLatestRelease), Runs: 60, Passes: 59, Interval: evalstats.Interval{Low: 0.85, High: 1}, Graders: []GraderResult{{"trailer-line", 59}, {"file-written", 60}}},
				},
				NoResult: []string{"cech-example/control"},
				Errors:   []RunError{{"the run took longer than 10 minutes", 1}, {"git add: a | b\nfailed", 1}},
			},
			Skills: []SkillCost{
				{Plugin: "cech-example", Version: "this commit", DescriptionSize: 85, AddedTokens: 112, Pairs: 60},
				{Plugin: "cech-example", Version: "0.1.0", DescriptionSize: 80, AddedTokens: 104, Pairs: 60},
			},
			Usage: Usage{Runs: 680, InputTokens: 12000, CacheReadTokens: 3400000, CacheCreationTokens: 90000, OutputTokens: 21000, CostUSD: 0.84, CostReported: true},
		}},
	}
	got, err := Render(r)
	if err != nil {
		t.Fatal(err)
	}
	const golden = "testdata/report.md"
	if *update {
		if werr := os.WriteFile(golden, got, 0o600); werr != nil {
			t.Fatal(werr)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("report differs from %s; run with -update after an intended change:\n%s", golden, got)
	}
}
