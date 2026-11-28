// Package evalstats decides the bars of the release check (ADR 27) from
// the outcomes of model runs, with confidence sequences that hold however
// often they are read (eval-runs spec criteria 2 to 8 and 10). Each case is
// read at the false-alarm rate, with no correction for the number of
// cases (criterion 4).
package evalstats

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
)

// Verdict is the state of a bar: unclear while more runs could settle it,
// then passed, failed, or without a result.
type Verdict int

// The verdicts a bar can reach.
const (
	Unclear Verdict = iota
	Pass
	Fail
	NoResult
)

// String names the verdict as the report and --json write it.
func (v Verdict) String() string {
	switch v {
	case Unclear:
		return "unclear"
	case Pass:
		return "pass"
	case Fail:
		return "fail"
	case NoResult:
		return "no result"
	}
	return fmt.Sprintf("Verdict(%d)", int(v))
}

// Settings are the values of criterion 33 that the bars read.
type Settings struct {
	// FalseAlarm is the chance that a bar fails, or passes, against the
	// truth, over every read of its sequence: α.
	FalseAlarm float64
	// Threshold is the difference in pass rate, from 0 to 1, that a
	// comparison must rule out (criteria 7 and 8).
	Threshold float64
	// Cap is the most runs per arm per case (criterion 2).
	Cap int
	// ContentBar, LoadBar and SkipBar are the rates of criteria 5 and 6.
	ContentBar, LoadBar, SkipBar float64
	// ErrorShare and ErrorFrom are the rule of criterion 10: a case in
	// which more than ErrorShare of its runs error, read once it has
	// ErrorFrom runs or at the end, has no result.
	ErrorShare float64
	ErrorFrom  int
}

// Interval is a range [Low, High] for a pass rate or for a difference of
// two, which holds at the false-alarm rate however often it is read.
type Interval struct {
	Low, High float64
}

// Contains reports whether x lies in the interval.
func (iv Interval) Contains(x float64) bool { return iv.Low <= x && x <= iv.High }

// sequence is the predictable plug-in empirical-Bernstein confidence
// sequence for the mean of outcomes in [0, 1] (Waudby-Smith and Ramdas,
// "Estimating means of bounded random variables by betting", JRSS B,
// 2024, section 3.2, with c = 1/2). Each outcome gets a weight λ chosen
// from the outcomes before it, so the range narrows with the spread it
// sees, and holds two-sided at the false-alarm rate alpha for every
// number of outcomes at once.
func sequence(xs []float64, alpha float64) Interval {
	if len(xs) == 0 {
		return Interval{0, 1}
	}
	logTerm := math.Log(2 / alpha)
	// The running estimates start from a mean of 1/2 and a variance of
	// 1/4, the widest the range allows.
	var sumLam, sumLamX, sumV, sum, sumSq float64
	mean, variance := 0.5, 0.25
	for i, x := range xs {
		t := float64(i + 1)
		lam := min(math.Sqrt(2*logTerm/(variance*t*math.Log(1+t))), 0.5)
		sumV += 4 * (x - mean) * (x - mean) * psi(lam)
		sumLam += lam
		sumLamX += lam * x
		sum += x
		sumSq += x * x
		mean = (0.5 + sum) / (t + 1)
		variance = max((0.25+sumSq-2*mean*sum+t*mean*mean)/(t+1), 1e-6)
	}
	centre := sumLamX / sumLam
	r := (logTerm + sumV) / sumLam
	return Interval{max(0, centre-r), min(1, centre+r)}
}

// psi is ψ_E(λ) = (-log(1-λ) - λ)/4, the empirical-Bernstein penalty.
func psi(lam float64) float64 {
	return (-math.Log1p(-lam) - lam) / 4
}

// RateInterval is the confidence sequence for a pass rate from the
// outcomes of runs, in the order they were made, read at the
// false-alarm rate alpha. With no runs it is [0, 1].
func RateInterval(runs []bool, alpha float64) Interval {
	xs := make([]float64, len(runs))
	for i, r := range runs {
		xs[i] = outcome(r)
	}
	return sequence(xs, alpha)
}

// DiffInterval is the confidence sequence for the pass rate of a minus
// that of b, from their outcomes paired in order: the k-th of a with the
// k-th of b, up to the shorter list. A pair's difference, from -1 to 1,
// is scaled into [0, 1] for the sequence and the range scaled back. With
// no pairs it is [-1, 1].
func DiffInterval(a, b []bool, alpha float64) Interval {
	n := min(len(a), len(b))
	a, b = a[:n], b[:n]
	xs := make([]float64, n)
	for i := range a {
		xs[i] = (outcome(a[i]) - outcome(b[i]) + 1) / 2
	}
	iv := sequence(xs, alpha)
	return Interval{2*iv.Low - 1, 2*iv.High - 1}
}

func outcome(pass bool) float64 {
	if pass {
		return 1
	}
	return 0
}

// Above decides a bar that a rate must clear: it passes when the whole
// interval lies above bar, and fails when the whole interval lies at or
// below it (criteria 4 to 6).
func Above(iv Interval, bar float64) Verdict {
	switch {
	case iv.Low > bar:
		return Pass
	case iv.High <= bar:
		return Fail
	}
	return Unclear
}

// Below decides a bar that a rate must stay under: it passes when the
// whole interval lies below bar, and fails when the whole interval lies
// at or above it (criterion 6).
func Below(iv Interval, bar float64) Verdict {
	switch {
	case iv.High < bar:
		return Pass
	case iv.Low >= bar:
		return Fail
	}
	return Unclear
}

// Same decides a comparison that must show no difference of the
// threshold either way: it passes when the whole interval lies within
// (-t, t), and fails when it lies wholly at or beyond t or -t
// (criterion 7).
func Same(iv Interval, t float64) Verdict {
	switch {
	case iv.Low > -t && iv.High < t:
		return Pass
	case iv.Low >= t || iv.High <= -t:
		return Fail
	}
	return Unclear
}

// NotWorse decides a comparison of the version being released, minus the
// latest final release: it passes when the whole interval lies above -t,
// and fails when it lies wholly at or below -t (criterion 8).
func NotWorse(iv Interval, t float64) Verdict {
	switch {
	case iv.Low > -t:
		return Pass
	case iv.High <= -t:
		return Fail
	}
	return Unclear
}

// Settle turns a verdict that is still unclear into no result once the
// runs can grow no more: at the cap, or when the budget is spent
// (criterion 4).
func Settle(v Verdict, atCap, budgetSpent bool) Verdict {
	if v == Unclear && (atCap || budgetSpent) {
		return NoResult
	}
	return v
}

// TooManyErrors reports whether a case that has made runs runs, of which
// errors still errored after their rerun, has no result under criterion
// 10 of the eval-runs spec: read from the ErrorFrom-th run on, or at the end.
func (s Settings) TooManyErrors(errors, runs int, end bool) bool {
	if runs == 0 || (!end && runs < s.ErrorFrom) {
		return false
	}
	return float64(errors) > s.ErrorShare*float64(runs)
}

// Bar is one bar of one case, or of a plugin's trigger cases, as the
// report gives it.
type Bar struct {
	Name     string
	Case     string
	Verdict  Verdict
	Interval Interval
	Runs     []int
}

// Next picks the bar that gets the next runs while the budget lasts: of
// the unclear bars, the one whose arms have the fewest runs, and among
// equals the case whose name sorts first (criterion 2). It returns -1
// when no bar is unclear.
func Next(bars []Bar) int {
	best := -1
	var bestBar Bar
	for i, b := range bars {
		if b.Verdict != Unclear {
			continue
		}
		if best < 0 || less(b, bestBar) {
			best, bestBar = i, b
		}
	}
	return best
}

func less(a, b Bar) bool {
	fewest := func(b Bar) int { return slices.Min(append([]int{math.MaxInt}, b.Runs...)) }
	if c := cmp.Compare(fewest(a), fewest(b)); c != 0 {
		return c < 0
	}
	// Case is plugin/case, or plugin/ for a bar over a plugin's trigger
	// cases; plugins compare first, so plugin a sorts before plugin a-b.
	pa, ca, _ := strings.Cut(a.Case, "/")
	pb, cb, _ := strings.Cut(b.Case, "/")
	if pa != pb {
		return pa < pb
	}
	return ca < cb
}
