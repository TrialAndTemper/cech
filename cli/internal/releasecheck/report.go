package releasecheck

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/TrialAndTemper/cech/cli/internal/evalstats"
)

// Report is what the report file gives (criteria 26 and 27).
type Report struct {
	// Image is the eval image as named, and ImageID its ID, which the runs
	// went by.
	Commit, Image, ImageID, Cech string
	// Harnesses are in the order the check ran them.
	Harnesses []HarnessReport
}

// HarnessReport is one harness's part of the report.
type HarnessReport struct {
	Name, Version, Model, Credential string
	Result                           Result
	// Skills are the description size and added tokens of each skill
	// version the check ran (criterion 25).
	Skills []SkillCost
	// Usage is what the harness's runs used, summed over the runs whose
	// telemetry was read (ADR 27 item 6).
	Usage Usage
}

// Usage is the tokens and cost of a set of runs.
type Usage struct {
	Runs                                                            int
	InputTokens, CacheReadTokens, CacheCreationTokens, OutputTokens int64
	// CostUSD is valid when CostReported is set; only Claude Code
	// reports a cost.
	CostUSD      float64
	CostReported bool
}

// SkillCost is criterion 25's figures for one skill version.
type SkillCost struct {
	Plugin, Version string
	// DescriptionSize is in Unicode code points.
	DescriptionSize int
	// AddedTokens is the median, over the pairs of runs, of the input
	// tokens the plugin adds to the first request of the main agent
	// loop; Pairs is how many pairs it read, and Skipped names those
	// left out because their loaded tools differed by more than the
	// plugin's.
	AddedTokens int
	Pairs       int
	Skipped     []string
}

//go:embed report.md.tmpl
var reportTemplate string

var tmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"pct": func(f float64) string { return fmt.Sprintf("%.0f%%", 100*f) },
	"pts": func(f float64) string { return fmt.Sprintf("%+.0f", 100*f) },
	"interval": func(b BarResult) string {
		if len(b.Reads) == 2 {
			return fmt.Sprintf("%+.0f to %+.0f points", 100*b.Interval.Low, 100*b.Interval.High)
		}
		return fmt.Sprintf("%.0f%% to %.0f%%", 100*b.Interval.Low, 100*b.Interval.High)
	},
	"runs": func(rs []int) string {
		s := make([]string, len(rs))
		for i, r := range rs {
			s[i] = fmt.Sprint(r)
		}
		return strings.Join(s, " and ")
	},
	"caseOf": func(b BarResult) string {
		if b.Case == "" {
			return "trigger cases"
		}
		return b.Case
	},
	"belowThreshold": func(b BarResult) bool {
		return b.Verdict == evalstats.Pass && len(b.Reads) == 2 && b.Interval.Low < 0
	},
	"join": strings.Join,
	"usd":  func(f float64) string { return fmt.Sprintf("$%.2f", f) },
	// cell keeps a message to one table cell or one line.
	"cell": func(v any) string {
		return strings.NewReplacer("|", `\|`, "\r", " ", "\n", " ").Replace(fmt.Sprint(v))
	},
	"hasGraders": func(arms []ArmResult) bool {
		for _, a := range arms {
			if len(a.Graders) > 0 {
				return true
			}
		}
		return false
	},
}).Parse(reportTemplate))

// Render writes the report file from its one template, so its form
// stays the same between releases and changes only with a CLI release
// (ADR 27 item 6, criterion 27).
func Render(r Report) ([]byte, error) {
	var b bytes.Buffer
	if err := tmpl.Execute(&b, r); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Passed reports whether every bar on every harness passed.
func (r Report) Passed() bool {
	for _, h := range r.Harnesses {
		if !h.Result.Passed() {
			return false
		}
	}
	return true
}
