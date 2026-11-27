// Package releasecheck plans and runs the release check of ADR 27: which
// arms each case runs, which bars read them, how runs are added in rounds
// of five until each bar is clear, and what the report file says
// (eval-runs spec criteria 1 to 10, 12, 25 and 27).
package releasecheck

import (
	"fmt"

	"github.com/TrialAndTemper/cech/cli/internal/evalstats"
)

// The arms, as run folders name them (criterion 26).
const (
	ArmWith          = "with"
	ArmWithout       = "without"
	ArmLatestRelease = "latest-release"
	ArmRouted        = "routed"
	ArmUnrouted      = "unrouted"
)

// Case is one eval case of a plugin the release check covers.
type Case struct {
	Plugin, Name string
	// Family and Expect are as the case's cech.toml has them.
	Family, Expect string
	// Compared is set for a content case of a changed plugin that the
	// latest final release also had (criterion 1).
	Compared bool
	// Graders are the names of its graders, for a content or control
	// case.
	Graders []string
}

// Arm is one arm of one case on one harness.
type Arm struct {
	Harness, Plugin, Case, Name string
}

func (a Arm) String() string { return fmt.Sprintf("%s/%s/%s/%s", a.Harness, a.Plugin, a.Case, a.Name) }

// Arms are the arms a case runs (criterion 1, ADR 27 item 2).
func (c Case) Arms(harness string) []Arm {
	arm := func(name string) Arm { return Arm{harness, c.Plugin, c.Name, name} }
	switch c.Family {
	case "trigger":
		return []Arm{arm(ArmRouted), arm(ArmUnrouted)}
	case "content":
		if c.Compared {
			return []Arm{arm(ArmWith), arm(ArmWithout), arm(ArmLatestRelease)}
		}
	}
	return []Arm{arm(ArmWith), arm(ArmWithout)}
}

// Bar kinds, as the report names them.
const (
	BarContent = "content passes with the plugin"
	BarLoad    = "routed skill loads when it should"
	BarSkip    = "routed skill seldom loads otherwise"
	BarControl = "control does not differ"
	BarNoDrop  = "no drop since the latest final release"
)

// BarSpec is one bar: what it reads and how it decides.
type BarSpec struct {
	Kind, Harness, Plugin string
	// Case is the case a per-case bar reads, or "" for a bar over a
	// plugin's trigger cases.
	Case string
	// Reads are the arms whose outcomes the bar reads: one arm for a
	// rate, two for a comparison (the first minus the second), or several
	// pooled for criterion 6.
	Reads []Arm
}

// Bars are the bars of the cases of one plugin on one harness
// (criteria 5 to 8).
func Bars(harness, plugin string, cases []Case) []BarSpec {
	var bars []BarSpec
	var load, skip []Arm
	for _, c := range cases {
		if c.Plugin != plugin {
			continue
		}
		arm := func(name string) Arm { return Arm{harness, plugin, c.Name, name} }
		switch c.Family {
		case "content":
			bars = append(bars, BarSpec{BarContent, harness, plugin, c.Name, []Arm{arm(ArmWith)}})
			if c.Compared {
				bars = append(bars, BarSpec{BarNoDrop, harness, plugin, c.Name, []Arm{arm(ArmWith), arm(ArmLatestRelease)}})
			}
		case "control":
			bars = append(bars, BarSpec{BarControl, harness, plugin, c.Name, []Arm{arm(ArmWith), arm(ArmWithout)}})
		case "trigger":
			if c.Expect == "load" {
				load = append(load, arm(ArmRouted))
			} else {
				skip = append(skip, arm(ArmRouted))
			}
		}
	}
	if len(load) > 0 {
		bars = append(bars, BarSpec{BarLoad, harness, plugin, "", load})
	}
	if len(skip) > 0 {
		bars = append(bars, BarSpec{BarSkip, harness, plugin, "", skip})
	}
	return bars
}

// Outcomes are the results of the runs of each arm, in the order they
// were made; a run without a result is left out.
type Outcomes map[Arm][]bool

// Decide reads a bar from the outcomes so far (criteria 3 to 8),
// each case at the false-alarm rate (criterion 4).
func Decide(b BarSpec, o Outcomes, s evalstats.Settings) (evalstats.Verdict, evalstats.Interval) {
	alpha := s.FalseAlarm
	switch b.Kind {
	case BarContent:
		iv := evalstats.RateInterval(o[b.Reads[0]], alpha)
		return evalstats.Above(iv, s.ContentBar), iv
	case BarLoad, BarSkip:
		var pooled []bool
		for _, a := range b.Reads {
			pooled = append(pooled, o[a]...)
		}
		iv := evalstats.RateInterval(pooled, alpha)
		if b.Kind == BarLoad {
			return evalstats.Above(iv, s.LoadBar), iv
		}
		return evalstats.Below(iv, s.SkipBar), iv
	case BarControl:
		iv := evalstats.DiffInterval(o[b.Reads[0]], o[b.Reads[1]], alpha)
		return evalstats.Same(iv, s.Threshold), iv
	case BarNoDrop:
		iv := evalstats.DiffInterval(o[b.Reads[0]], o[b.Reads[1]], alpha)
		return evalstats.NotWorse(iv, s.Threshold), iv
	}
	return evalstats.NoResult, evalstats.Interval{}
}
