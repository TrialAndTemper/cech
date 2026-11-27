// Package evalrun makes the model runs of cech eval, each in a fresh
// container from the eval image (ADR 26; eval-runs spec criteria 11,
// 12, 14 to 21 and 29): it builds the run's workspace and home, installs its
// arm's plugins, hands it one credential, runs the harness behind an
// allowlist proxy, and writes what the run left in a recording folder
// that internal/evals grades.
package evalrun

import "time"

// Settings are the values the CLI holds (criterion 33) that the runner
// reads; the bars' values are in internal/evalstats.
type Settings struct {
	// Image is the eval image, named by digest.
	Image string
	// Models is the model each harness runs, its cheapest (ADR 27 item 1).
	Models map[string]string
	// Hosts are the hosts each harness may reach through the proxy.
	Hosts map[string][]string
	// Timeout is the longest a run may take before it counts as an error
	// (criterion 9).
	Timeout time.Duration
	// Parallel is how many runs share a harness at once by default.
	Parallel map[string]int
	// MedianRun is the median time of a run on each harness, for the
	// estimates of --dry-run.
	MedianRun map[string]time.Duration
}

// The harnesses, as internal/evals names them.
const (
	claudeCode = "claude-code"
	codex      = "codex"
)

// Defaults are the values cech eval uses (criterion 33), set from the
// timing runs of research.md on 2026-11-26. The image is the one the
// eval image workflow published; until it has run on main, a
// maintainer sets CECH_EVAL_IMAGE to an image built from images/eval/.
var Defaults = Settings{
	Image: "",
	Models: map[string]string{
		claudeCode: "claude-haiku-5-5",
		codex:      "gpt-6-luna",
	},
	Hosts: map[string][]string{
		claudeCode: {"api.anthropic.com", "statsig.anthropic.com", "console.anthropic.com"},
		codex:      {"chatgpt.com", "api.openai.com", "auth.openai.com"},
	},
	Timeout:   10 * time.Minute,
	Parallel:  map[string]int{claudeCode: 6, codex: 4},
	MedianRun: map[string]time.Duration{claudeCode: 10 * time.Second, codex: 20 * time.Second},
}
