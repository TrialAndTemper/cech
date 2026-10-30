package evals

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

// TestCheckResultsCategory checks that a category named results is a
// finding (spec rule 2): its eval folder would be evals/results/, which
// the check skips as claude plugin eval's results. Only an extension can
// add the category, so the test builds the marketplace itself.
func TestCheckResultsCategory(t *testing.T) {
	m := &marketplace.Marketplace{Plugins: []marketplace.Plugin{{Category: "results", Name: "acme-tally"}}}
	res, err := Check(fstest.MapFS{}, m)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Findings {
		if f.Path == "plugins/results/acme-tally" && strings.Contains(f.Message, "claude plugin eval writes its results") {
			return
		}
	}
	t.Errorf("findings %+v hold none for the category results", res.Findings)
}

// TestDescriptionLine checks the line the Codex load detector looks
// for, as SKILL.md writes it (spec rule 24).
func TestDescriptionLine(t *testing.T) {
	for _, tt := range []struct{ src, want string }{
		{"---\nname: x\ndescription: Use when writing.\n---\n", "Use when writing."},
		{"---\nname: x\ndescription: \"Use when: writing.\"\n---\n", `"Use when: writing."`},
		{"---\nname: x\ndescription: >-\n  Use when\n  writing.\n---\n", "Use when"},
		{"---\nname: x\ndescription: |\n\n  Use when writing.\n---\n", "Use when writing."},
		{"---\nname: x\n---\ndescription: in the body\n", ""},
		{"name: x\ndescription: no frontmatter\n", ""},
	} {
		if got := descriptionLine(tt.src); got != tt.want {
			t.Errorf("descriptionLine(%q) = %q, want %q", tt.src, got, tt.want)
		}
	}
}

// TestLocalPath checks the local-path rule around the placeholders
// (spec rule 22): a placeholder is no local path, a real path around one
// is, and a folder inside the workspace named like a local root is not.
func TestLocalPath(t *testing.T) {
	for _, tt := range []struct {
		s     string
		local bool
	}{
		{"/cech/home/.codex/plugins", false},
		{"cat /cech/workspace/tmp/out.log", false},
		{"/cech/plugin/home/x", false},
		{"/tmp/x/cech/workspace/a", true},
		{"/home/someone/cech/plugin", true},
		{`C:\work`, true},
		{"PATH=/cech/home/bin:/home/me/bin", true},
		{"a=/cech/workspace,/Users/me/x", true},
		{"(/cech/plugin)/tmp/x", true},
		{"/cech/home/../../Users/me", true},
		{"/private/x/plugin", true},
		{"/var/folders/x/plugin", true},
	} {
		if got := localPath(tt.s); got != tt.local {
			t.Errorf("localPath(%q) = %v, want %v", tt.s, got, tt.local)
		}
	}
}

// TestGlobRegexp checks file_exists globs: * and ? stay inside a
// folder, and ** crosses folders, matching none at the start.
func TestGlobRegexp(t *testing.T) {
	for _, tt := range []struct {
		glob, path string
		match      bool
	}{
		{"*_MSG", "COMMIT_MSG", true},
		{"*_MSG", "a/COMMIT_MSG", false},
		{"?.txt", "a.txt", true},
		{"?.txt", "/.txt", false},
		{"**/x", "x", true},
		{"**/x", "a/b/x", true},
		{"a/**/b", "a/b", true},
		{"a/**/b", "a/c/d/b", true},
		{"a/**/b", "ab", false},
	} {
		if got := globRegexp(tt.glob).MatchString(tt.path); got != tt.match {
			t.Errorf("glob %q on %q = %v, want %v", tt.glob, tt.path, got, tt.match)
		}
	}
}
