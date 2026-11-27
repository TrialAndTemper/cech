package evals

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

// TestCheckResultsCategory checks that a category named results is a
// finding (spec rule 2): its eval folder would be evals/results/, which
// the check skips as cech eval's run results. Only an extension can
// add the category, so the test builds the marketplace itself.
func TestCheckResultsCategory(t *testing.T) {
	m := &marketplace.Marketplace{Plugins: []marketplace.Plugin{{Category: "results", Name: "acme-tally"}}}
	res, err := Check(fstest.MapFS{}, m)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Findings {
		if f.Path == "plugins/results/acme-tally" && strings.Contains(f.Message, "cech eval writes its run results") {
			return
		}
	}
	t.Errorf("findings %+v hold none for the category results", res.Findings)
}

// TestCheckReportsCategory checks that a category named reports is a
// finding (eval-runs spec criterion 13): its eval folder would be
// evals/reports/, where cech eval writes its report files.
func TestCheckReportsCategory(t *testing.T) {
	m := &marketplace.Marketplace{Plugins: []marketplace.Plugin{{Category: "reports", Name: "acme-tally"}}}
	res, err := Check(fstest.MapFS{}, m)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Findings {
		if f.Path == "plugins/reports/acme-tally" && strings.Contains(f.Message, "report files") {
			return
		}
	}
	t.Errorf("findings %+v hold none for the category reports", res.Findings)
}

// TestSkillLines checks the name line and description line the Codex
// load detector looks for, as SKILL.md writes them (spec rule 24).
func TestSkillLines(t *testing.T) {
	for _, tt := range []struct{ src, name, desc string }{
		{"---\nname: x\ndescription: Use when writing.\n---\n", "name: x", "Use when writing."},
		{"---\r\nname: x\r\ndescription: d\r\n---\r\n", "name: x", "d"},
		{"---\nname: \"x\" \ndescription: \"Use when: writing.\"\n---\n", `name: "x"`, `"Use when: writing."`},
		{"---\n\"name\": x\n'description': d\n---\n", `"name": x`, "d"},
		{"---\nname : x\ndescription:   d\n---\n", "name : x", "d"},
		// Each line runs from its node to the end of the line, as the
		// file writes it; the column counts characters.
		{"---\n{name: x, description: d}\n---\n", "name: x, description: d}", "d}"},
		{"---\n{ññ: y, name: x, description: d}\n---\n", "name: x, description: d}", "d}"},
		// Lines break where YAML breaks them, not only at LF.
		{"---\n# a\r\r\r\r\rname: x\ndescription: d\n---\n", "name: x", "d"},
		{"---\n# a\u2028\u2029\u0085\u2028name: x\ndescription: d\n---\n", "name: x", "d"},
		{"---\n# a\r\r\nname: x\ndescription: d\n---\n", "name: x", "d"},
		{"---\nname: x\ndescription:\n  Use when writing.\n---\n", "name: x", "Use when writing."},
		{"---\nname: x\ndescription: >-\n  Use when\n  writing.\n---\n", "name: x", "Use when"},
		{"---\nname: x\ndescription: |\n\n  Use when writing.\n---\n", "name: x", "Use when writing."},
		{"---\nname:\ndescription:\n---\nname: x\ndescription: in the body\n", "", ""},
		{"name: x\ndescription: no frontmatter\n", "", ""},
	} {
		if name, desc := skillLines(tt.src); name != tt.name || desc != tt.desc {
			t.Errorf("skillLines(%q) = %q, %q; want %q, %q", tt.src, name, desc, tt.name, tt.desc)
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
		{`cd C:\work`, true},
		{`grep 'name:\s'`, false},
		{`key_x:\d`, false},
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
