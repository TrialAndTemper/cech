package evalstats

import (
	"math/rand/v2"
	"testing"
)

// runsAt makes n outcomes that pass with chance p, from a fixed seed, so
// the tests are the same on every run.
func runsAt(rng *rand.Rand, p float64, n int) []bool {
	out := make([]bool, n)
	for i := range out {
		out[i] = rng.Float64() < p
	}
	return out
}

// TestRateIntervalHolds checks the sequence's promise on the rates the
// release check meets: over many simulated cases, read after every five
// runs up to the cap, the range leaves out the true rate in at most the
// false-alarm share of cases (criterion 3).
func TestRateIntervalHolds(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	const alpha, most, cases = 0.05, 200, 300
	for _, p := range []float64{0.5, 0.85, 0.97} {
		missed := 0
		for range cases {
			runs := runsAt(rng, p, most)
			for n := 5; n <= most; n += 5 {
				if !RateInterval(runs[:n], alpha).Contains(p) {
					missed++
					break
				}
			}
		}
		if share := float64(missed) / cases; share > alpha {
			t.Errorf("p = %v: the range left out the true rate in %.3f of cases; want at most %v", p, share, alpha)
		}
	}
}

// TestDiffIntervalHolds checks the same promise for the difference of two
// arms, paired run by run (criteria 3, 7 and 8).
func TestDiffIntervalHolds(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	const alpha, most, cases = 0.05, 200, 300
	for _, pq := range [][2]float64{{0.85, 0.85}, {0.70, 0.85}, {0.95, 0.60}} {
		missed := 0
		for range cases {
			a, b := runsAt(rng, pq[0], most), runsAt(rng, pq[1], most)
			for n := 5; n <= most; n += 5 {
				if !DiffInterval(a[:n], b[:n], alpha).Contains(pq[0] - pq[1]) {
					missed++
					break
				}
			}
		}
		if share := float64(missed) / cases; share > alpha {
			t.Errorf("rates %v: the range left out the true difference in %.3f of cases; want at most %v", pq, share, alpha)
		}
	}
}

// TestIntervalsNarrow checks that more runs give a narrower range, and
// that a range at a high pass rate is narrower than one at one half,
// which is what the empirical-Bernstein sequence buys over a bound for
// the widest spread (research.md, Simulated release checks).
func TestIntervalsNarrow(t *testing.T) {
	width := func(iv Interval) float64 { return iv.High - iv.Low }
	half := make([]bool, 200)
	high := make([]bool, 200)
	for i := range half {
		half[i] = i%2 == 0
		high[i] = i%20 != 0
	}
	if w50, w200 := width(RateInterval(half[:50], 0.05)), width(RateInterval(half, 0.05)); w200 >= w50 {
		t.Errorf("width after 200 runs %v, after 50 %v; want it narrower", w200, w50)
	}
	if wh, w5 := width(RateInterval(high, 0.05)), width(RateInterval(half, 0.05)); wh >= w5 {
		t.Errorf("width at a 95%% rate %v, at 50%% %v; want it narrower", wh, w5)
	}
}

// TestNoRuns checks the ranges before any run: everything is possible.
func TestNoRuns(t *testing.T) {
	if iv := RateInterval(nil, 0.05); iv != (Interval{0, 1}) {
		t.Errorf("rate with no runs = %v; want [0, 1]", iv)
	}
	if iv := DiffInterval([]bool{true}, nil, 0.05); iv != (Interval{-1, 1}) {
		t.Errorf("difference with no pairs = %v; want [-1, 1]", iv)
	}
}

// TestDiffPairsInOrder checks that the difference pairs the k-th runs
// and leaves out the extra runs of the longer arm.
func TestDiffPairsInOrder(t *testing.T) {
	a := make([]bool, 100)
	b := make([]bool, 120)
	for i := range a {
		a[i] = true
	}
	for i := 100; i < 120; i++ {
		b[i] = true
	}
	// Every pair is a pass against a fail, so the difference is near +1;
	// the 20 extra passes of b would pull it down if they were read.
	if iv := DiffInterval(a, b, 0.05); iv.Low < 0.5 {
		t.Errorf("difference = %v; want it near 1", iv)
	}
}

// TestVerdicts checks each bar's rule at and around its edges
// (criteria 4 to 8).
func TestVerdicts(t *testing.T) {
	tests := []struct {
		name string
		got  Verdict
		want Verdict
	}{
		{"above, wholly over the bar", Above(Interval{0.6, 0.9}, 0.5), Pass},
		{"above, touching the bar", Above(Interval{0.5, 0.9}, 0.5), Unclear},
		{"above, wholly at or under", Above(Interval{0.2, 0.5}, 0.5), Fail},
		{"below, wholly under the bar", Below(Interval{0.0, 0.19}, 0.2), Pass},
		{"below, across the bar", Below(Interval{0.1, 0.3}, 0.2), Unclear},
		{"below, wholly at or over", Below(Interval{0.2, 0.4}, 0.2), Fail},
		{"same, inside the threshold", Same(Interval{-0.1, 0.1}, 0.15), Pass},
		{"same, across the threshold", Same(Interval{-0.1, 0.2}, 0.15), Unclear},
		{"same, wholly beyond +t", Same(Interval{0.15, 0.4}, 0.15), Fail},
		{"same, wholly beyond -t", Same(Interval{-0.4, -0.15}, 0.15), Fail},
		{"not worse, wholly above -t", NotWorse(Interval{-0.1, 0.3}, 0.15), Pass},
		{"not worse, across -t", NotWorse(Interval{-0.2, 0.0}, 0.15), Unclear},
		{"not worse, wholly at or below -t", NotWorse(Interval{-0.5, -0.15}, 0.15), Fail},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s: %v; want %v", tt.name, tt.got, tt.want)
		}
	}
}

// TestSettle checks that only an unclear bar turns into no result, and
// only at the cap or once the budget is spent (criterion 4).
func TestSettle(t *testing.T) {
	for _, tt := range []struct {
		v              Verdict
		atCap, noMoney bool
		want           Verdict
	}{
		{Unclear, false, false, Unclear},
		{Unclear, true, false, NoResult},
		{Unclear, false, true, NoResult},
		{Pass, true, true, Pass},
		{Fail, true, false, Fail},
	} {
		if got := Settle(tt.v, tt.atCap, tt.noMoney); got != tt.want {
			t.Errorf("Settle(%v, %v, %v) = %v; want %v", tt.v, tt.atCap, tt.noMoney, got, tt.want)
		}
	}
}

// TestTooManyErrors checks criterion 10's rule: more than the error share
// of a case's runs, read from the ErrorFrom-th run on or at the end.
func TestTooManyErrors(t *testing.T) {
	s := Settings{ErrorShare: 0.10, ErrorFrom: 20}
	for _, tt := range []struct {
		errors, runs int
		end, want    bool
	}{
		{1, 5, false, false},  // too early to read
		{1, 5, true, true},    // at the end, 1 of 5 is over 10%
		{2, 20, false, false}, // exactly 10% is not more
		{3, 20, false, true},
		{0, 0, true, false},
	} {
		if got := s.TooManyErrors(tt.errors, tt.runs, tt.end); got != tt.want {
			t.Errorf("TooManyErrors(%d, %d, %v) = %v; want %v", tt.errors, tt.runs, tt.end, got, tt.want)
		}
	}
}

// TestNext checks which unclear bar gets the next runs: the one whose
// arms have the fewest runs, and among equals the case that sorts first
// (criterion 2).
func TestNext(t *testing.T) {
	bars := []Bar{
		{Case: "b", Verdict: Unclear, Runs: []int{10, 10}},
		{Case: "a", Verdict: Pass, Runs: []int{5}},
		{Case: "c", Verdict: Unclear, Runs: []int{10, 5}},
		{Case: "a", Verdict: Unclear, Runs: []int{5, 20}},
	}
	if got := Next(bars); got != 3 {
		t.Errorf("Next = %d; want 3, case a with an arm of 5 runs", got)
	}
	if got := Next(bars[:2]); got != 0 {
		t.Errorf("Next of the first two = %d; want 0", got)
	}
	if got := Next([]Bar{{Verdict: Fail}}); got != -1 {
		t.Errorf("Next with no unclear bar = %d; want -1", got)
	}
	// Plugins compare before cases: plugin a's trigger bar, then its
	// cases, then plugin a-b's, whatever "-" sorts against "/".
	tied := []Bar{
		{Case: "a-b/x", Verdict: Unclear, Runs: []int{5}},
		{Case: "a/z", Verdict: Unclear, Runs: []int{5}},
		{Case: "a/", Verdict: Unclear, Runs: []int{5}},
	}
	if got := Next(tied); got != 2 {
		t.Errorf("Next of tied bars = %d; want 2, plugin a's trigger bar", got)
	}
	if got := Next(tied[:2]); got != 1 {
		t.Errorf("Next of a-b/x and a/z = %d; want 1, plugin a first", got)
	}
}
