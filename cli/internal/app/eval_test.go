package app

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The fixture for cech eval --check: a marketplace with one plugin, its
// fixture project and routing row, and every case ADR 25 asks of it,
// each recorded on both harnesses. Two recordings are real runs, trimmed
// as spec rule 21 says: the content case on Claude Code, on which every
// grader passed, and a trigger case on Codex, which loaded the skill.
var evalFixture = filepath.Join("testdata", "eval", "complete")

const (
	evalPlugin  = "evals/writing/cech-example/"
	evalProject = evalPlugin + "project/"
	evalRouting = evalPlugin + "routing.md"
	evalTrailer = evalPlugin + "cech-example-trailer/"
	evalControl = evalPlugin + "cech-example-control/"
	evalLoad    = evalPlugin + "cech-example-load-1/"
	evalOblique = evalPlugin + "cech-example-load-3/"
	evalSkip    = evalPlugin + "cech-example-skip-1/"
	evalNear    = evalPlugin + "cech-example-skip-3/"

	// The recordings of each harness, under a case folder.
	claudeRecs = "recordings/claude-code/"
	codexRecs  = "recordings/codex/"
)

func TestEvalCheck(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, evalFixture, dir, nil)
	code, stdout, stderr := runIn(t, "eval", "--check", "-C", dir)
	if code != ExitOK || stdout != "" || stderr != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q; want 0 and no output", code, stdout, stderr)
	}
	code, stdout, _ = runIn(t, "eval", "--check", "--json", "-C", dir)
	if code != ExitOK {
		t.Fatalf("exit %d with --json", code)
	}
	checkGolden(t, "eval-check-json.golden", []byte(stdout))
}

// TestEvalCheckEmpty checks that a marketplace with no plugins and no
// evals passes, as CECH's own does before its first plugin, and says on
// stderr that it checked nothing, or with --json shows no cases; and that
// a folder that is no marketplace fails, as for cech build (spec rules 27
// and 29).
func TestEvalCheckEmpty(t *testing.T) {
	dir := t.TempDir()
	writeTo("data/marketplace.toml", "namespace = \"cech\"\ndescription = \"Skills\"\n\n[owner]\nname = \"CECH maintainers\"\n")(t, dir)
	const note = "cech: nothing checked: the marketplace has no plugins and no evals/\n"
	code, stdout, stderr := runIn(t, "eval", "--check", "-C", dir)
	if code != ExitOK || stdout != "" || stderr != note {
		t.Fatalf("exit %d, stdout %q, stderr %q; want 0, no stdout and %q", code, stdout, stderr, note)
	}
	code, stdout, stderr = runIn(t, "eval", "--check", "--json", "-C", dir)
	var res evalResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("--json: %v in %q", err, stdout)
	}
	if code != ExitOK || !res.OK || res.Cases != 0 || res.Recordings != 0 || len(res.Findings) != 0 || len(res.Errors) != 0 || stderr != "" {
		t.Fatalf("--json: exit %d, %+v, stderr %q; want 0, ok with no cases, and no stderr", code, res, stderr)
	}
	code, _, stderr = runIn(t, "eval", "--check", "-C", t.TempDir())
	if code != ExitFailure || !strings.Contains(stderr, "data/marketplace.toml: not found") {
		t.Fatalf("no marketplace: exit %d, stderr %q; want %d and data/marketplace.toml not found", code, stderr, ExitFailure)
	}
}

// TestEvalCheckPasses changes the fixture in ways the rules allow, each
// of which a wrong reading of a rule would turn into a finding.
func TestEvalCheckPasses(t *testing.T) {
	passRecs := []string{evalTrailer + claudeRecs + "real-run/", evalTrailer + claudeRecs + "trailer/", evalTrailer + codexRecs + "trailer/"}
	for _, tt := range []struct {
		name   string
		change func(t *testing.T, dir string)
	}{
		// Rule 2: cech eval writes its run results to evals/results/, which
		// git ignores.
		{"results of a run", writeTo("evals/results/run.json", `{"cwd":"/home/someone"}`+"\n")},
		// Rule 23: files lists the created files sorted, whatever the order
		// in created.txt.
		{"files sorted", func(t *testing.T, dir string) {
			replaceIn(evalTrailer+"cech.toml", `pattern = '^COMMIT_MSG$'`, `pattern = '^A\.txt\nCOMMIT_MSG$'`)(t, dir)
			for _, r := range passRecs {
				writeTo(r+"workspace/A.txt", "a\n")(t, dir)
				writeTo(r+"created.txt", "COMMIT_MSG\nA.txt\n")(t, dir)
			}
		}},
		// Rule 24: the Codex detector reads a line without its trailing
		// whitespace.
		{"trailing whitespace in Codex output", replaceIn(evalLoad+codexRecs+"loaded/trace.jsonl", `\nname: cech-example\n`, `\nname: cech-example \t\r\n`)},
		// Rule 24: the Codex detector finds the name line by the YAML
		// parser, so a quoted key is found.
		{"quoted name key", func(t *testing.T, dir string) {
			replaceIn("plugins/writing/cech-example/skills/cech-example/SKILL.md", "\nname: cech-example\n", "\n\"name\": cech-example\n")(t, dir)
			traces, err := filepath.Glob(filepath.Join(dir, "evals/writing/cech-example/*/recordings/codex/*/trace.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			for _, tr := range traces {
				src, err := os.ReadFile(tr)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(tr, []byte(strings.ReplaceAll(string(src), `\nname: cech-example\n`, `\n\"name\": cech-example\n`)), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}},
		// Rule 11: file_exists grades by the glob, here with ** matching
		// no folder and * a part of a name.
		{"file_exists globs", replaceIn(evalTrailer+"cech.toml", "type = \"file_exists\"\npath = \"COMMIT_MSG\"", "type = \"file_exists\"\npath = \"**/*_MSG\"")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			copyTree(t, evalFixture, dir, nil)
			tt.change(t, dir)
			if code, stdout, stderr := runIn(t, "eval", "--check", "-C", dir); code != ExitOK || stdout != "" || stderr != "" {
				t.Errorf("exit %d, stdout %q, stderr %q; want 0 and no output", code, stdout, stderr)
			}
		})
	}
}

// TestEvalCheckInCurrentFolder checks that without -C the check reads
// the marketplace in the current folder (spec rule 26).
func TestEvalCheckInCurrentFolder(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, evalFixture, dir, nil)
	remove(evalControl+"cech.toml")(t, dir)
	t.Chdir(dir)
	if code, stdout, _ := runIn(t, "eval", "--check"); code != ExitFailure || !strings.Contains(stdout, strings.TrimSuffix(evalControl, "/")+": has no cech.toml") {
		t.Errorf("exit %d, stdout %q; want %d and the finding", code, stdout, ExitFailure)
	}
}

// TestEvalCheckNoRoot checks that a root it cannot read leaves the check
// unable to decide, exit 3, not a pass (spec rule 30).
func TestEvalCheckNoRoot(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if code, stdout, stderr := runIn(t, "eval", "--check", "-C", missing); code != ExitUnreachable || stdout != "" || !strings.Contains(stderr, "missing") {
		t.Errorf("exit %d, stdout %q, stderr %q; want %d and an error naming the folder", code, stdout, stderr, ExitUnreachable)
	}
}

// TestEvalCheckPathOrder checks that findings come in path order,
// whatever order the check finds them in (spec rule 28).
func TestEvalCheckPathOrder(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, evalFixture, dir, nil)
	writeTo("evals/zz.md", "x\n")(t, dir)
	writeTo("evals/aa.md", "x\n")(t, dir)
	remove(evalControl+"cech.toml")(t, dir)
	remove(evalTrailer+"cech.toml")(t, dir)
	code, stdout, _ := runIn(t, "eval", "--check", "-C", dir)
	var paths []string
	for line := range strings.SplitSeq(strings.TrimSuffix(stdout, "\n"), "\n") {
		p, _, _ := strings.Cut(line, ": ")
		paths = append(paths, p)
	}
	if code != ExitFailure || len(paths) < 4 || !slices.IsSorted(paths) {
		t.Errorf("exit %d; want %d and at least 4 findings in path order; stdout:\n%s", code, ExitFailure, stdout)
	}
}

// writeTo returns a change that writes text to the file at p.
func writeTo(p, text string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		name := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// both returns a change that makes the changes in order.
func both(changes ...func(*testing.T, string)) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		for _, c := range changes {
			c(t, dir)
		}
	}
}

// copyFile returns a change that copies the file at from to to.
func copyFile(from, to string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(from)))
		if err != nil {
			t.Fatal(err)
		}
		writeTo(to, string(data))(t, dir)
	}
}

// TestEvalCheckFindings breaks the fixture one rule at a time. Each case
// guards a way the eval gate could pass when it should fail, and names
// the finding it must report: its path and part of its message.
func TestEvalCheckFindings(t *testing.T) {
	trailerToml := evalTrailer + "cech.toml"
	controlToml := evalControl + "cech.toml"
	loadToml := evalLoad + "cech.toml"
	realTrace := evalTrailer + claudeRecs + "real-run/trace.jsonl"
	codexReal := evalLoad + codexRecs + "real-run/trace.jsonl"
	codexRight := evalControl + codexRecs + "right/trace.jsonl"
	agents := evalProject + "AGENTS.md"
	tests := []struct {
		name   string
		change func(t *testing.T, dir string)
		path   string
		want   string
	}{
		// The heart of the gate: a grader that always passes is caught by
		// the recording that must fail it (ADR 25 item 4), on each harness.
		{
			"grader always passes", replaceIn(controlToml, `'^391$'`, `'\d'`),
			evalControl + claudeRecs + "wrong", "must fail a grader, but passes answer",
		},
		{"grader always passes on Codex", replaceIn(controlToml, `'^391$'`, `'\d'`), evalControl + codexRecs + "wrong", "must fail a grader"},
		{
			"grader never passes", replaceIn(controlToml, `'^391$'`, `'^390$'`),
			evalControl + claudeRecs + "right", "fails answer",
		},
		// On Codex, last_message is the last agent message, not the first.
		{
			"Codex grades the last message", replaceIn(codexRight, `{"type":"turn.completed"}`, `{"type":"item.completed","item":{"type":"agent_message","text":"done"}}`+"\n"+`{"type":"turn.completed"}`),
			evalControl + codexRecs + "right", "fails answer",
		},
		// A recording graded as the runner grades it: each type, target and
		// match mode, so a grader the gate reads wrongly shows here.
		{
			"count counts", replaceIn(trailerToml, "count = 1", "count = 2"),
			evalTrailer + claudeRecs + "real-run", "fails one-trailer",
		},
		{"count is exact", replaceIn(trailerToml, "count = 1", "count = 0"), evalTrailer + claudeRecs + "real-run", "fails one-trailer"},
		// On Claude Code, last_message is the result line, not the last text.
		{"Claude Code grades the result line", replaceIn(evalControl+claudeRecs+"right/trace.jsonl", `"result":"391"`, `"result":"390"`), evalControl + claudeRecs + "right", "fails answer"},
		{
			"not_contains negates", replaceIn(trailerToml, "change-kind: feat", "change-kind: fix"),
			evalTrailer + claudeRecs + "real-run", "fails no-feat",
		},
		{
			"file target reads the file", replaceIn(trailerToml, "target = \"file\"\npath = \"COMMIT_MSG\"", "target = \"file\"\npath = \"OTHER\""),
			evalTrailer + claudeRecs + "real-run", "fails file-trailer",
		},
		// An empty created.txt records a run that created nothing; reading
		// workspace/ instead would pass the file graders.
		{
			"files target lists created files", writeTo(evalTrailer+claudeRecs+"real-run/created.txt", ""),
			evalTrailer + claudeRecs + "real-run", "fails file-listed, file-written",
		},
		{"exists = false is graded", replaceIn(trailerToml, "type = \"file_exists\"\npath = \"COMMIT_MSG\"", "type = \"file_exists\"\npath = \"COMMIT_MSG\"\nexists = false"), evalTrailer + claudeRecs + "real-run", "fails file-written"},
		{"missing file target fails", remove(evalTrailer + claudeRecs + "real-run/workspace/COMMIT_MSG"), evalTrailer + claudeRecs + "real-run", "but fails file-listed, file-trailer, file-written"},
		// Rule 24: the load detectors decide a trigger case's recordings.
		// Claude Code's needs a Skill call for this skill whose result is
		// not an error: the fixture's unknown-skill recording calls Skill
		// and gets an error back, as a run without the plugin does.
		{
			"an errored Skill call is no load", replaceIn(evalLoad+claudeRecs+"unknown-skill/trace.jsonl", `,"is_error":true`, ""),
			evalLoad + claudeRecs + "unknown-skill", "reads a load",
		},
		{
			"a sibling skill is no load", replaceIn(evalLoad+claudeRecs+"loaded/trace.jsonl", `"skill":"cech-example:cech-example"`, `"skill":"cech-example:cech-example-x"`),
			evalLoad + claudeRecs + "loaded", "reads no load",
		},
		// A bare skill name is a load too, but another plugin's skill of the
		// same name is not.
		{
			"a bare skill name loads", both(copyFile(evalSkip+claudeRecs+"loaded/trace.jsonl", evalSkip+claudeRecs+"not-loaded/trace.jsonl"),
				replaceIn(evalSkip+claudeRecs+"not-loaded/trace.jsonl", `"skill":"cech-example:cech-example"`, `"skill":"cech-example"`)),
			evalSkip + claudeRecs + "not-loaded", "reads a load",
		},
		{
			"another plugin's skill is no load", replaceIn(evalLoad+claudeRecs+"loaded/trace.jsonl", `"skill":"cech-example:cech-example"`, `"skill":"other:cech-example"`),
			evalLoad + claudeRecs + "loaded", "reads no load",
		},
		{
			"a skip case's load is caught", copyFile(evalSkip+claudeRecs+"loaded/trace.jsonl", evalSkip+claudeRecs+"not-loaded/trace.jsonl"),
			evalSkip + claudeRecs + "not-loaded", "reads a load",
		},
		// Codex's needs the name line and the description's first line in
		// one command's output.
		{
			"Codex needs the description line", replaceIn(codexReal, "Ends it with the project's Change-Kind trailer.", "Ends it with a trailer."),
			evalLoad + codexRecs + "real-run", "reads no load",
		},
		{
			"Codex needs the name line", replaceIn(codexReal, `name: cech-example\n`, `name: cech-other\n`),
			evalLoad + codexRecs + "real-run", "reads no load",
		},
		{
			"Codex lines must end so", replaceIn(evalLoad+codexRecs+"loaded/trace.jsonl", `name: cech-example\n`, `name: cech-example (old)\n`),
			evalLoad + codexRecs + "loaded", "reads no load",
		},
		// A command that reads the skill and then fails still loaded it, and
		// a prefix such as grep -n adds still ends the line with the name.
		{
			"a failed command still loads", both(copyFile(evalSkip+codexRecs+"loaded/trace.jsonl", evalSkip+codexRecs+"not-loaded/trace.jsonl"),
				replaceIn(evalSkip+codexRecs+"not-loaded/trace.jsonl", `"exit_code":0,"status":"completed"`, `"exit_code":1,"status":"failed"`)),
			evalSkip + codexRecs + "not-loaded", "reads a load",
		},
		{
			"a grep prefix still loads", both(copyFile(evalSkip+codexRecs+"loaded/trace.jsonl", evalSkip+codexRecs+"not-loaded/trace.jsonl"),
				replaceIn(evalSkip+codexRecs+"not-loaded/trace.jsonl", `\nname: cech-example\ndescription: `, `\n2:name: cech-example\n3:description: `)),
			evalSkip + codexRecs + "not-loaded", "reads a load",
		},
		// Both lines must come from one command, and each must end there.
		{
			"name and description in two commands", replaceIn(evalLoad+codexRecs+"loaded/trace.jsonl", `"aggregated_output":"---\nname: cech-example\ndescription: `,
				`"aggregated_output":"---\nname: cech-example\n","exit_code":0,"status":"completed"}}`+"\n"+`{"type":"item.completed","item":{"type":"command_execution","command":"cat","aggregated_output":"description: `),
			evalLoad + codexRecs + "loaded", "reads no load",
		},
		{
			"the description line must end there", replaceIn(codexReal, `Change-Kind trailer.\n---`, `Change-Kind trailer. More.\n---`),
			evalLoad + codexRecs + "real-run", "reads no load",
		},
		// The name line is matched as SKILL.md writes it, quotes included.
		{
			"Codex name line as SKILL.md writes it", replaceIn("plugins/writing/cech-example/skills/cech-example/SKILL.md", "name: cech-example\n", "name: \"cech-example\"\n"),
			evalLoad + codexRecs + "real-run", "reads no load",
		},
		// A prefix ending in a letter makes another key, not the name line.
		{
			"Codex name line needs its own key", replaceIn(codexReal, `\nname: cech-example\n`, `\nrename: cech-example\n`),
			evalLoad + codexRecs + "real-run", "reads no load",
		},
		// Rules 1 to 2, 7 to 9 and 17: layout.
		{
			"eval folder without a plugin", copyDir(evalPlugin, "evals/writing/cech-other"),
			"evals/writing/cech-other", "has no plugin plugins/writing/cech-other",
		},
		{
			"eval folder in another category", move("evals/writing", "evals/meta"),
			"evals/meta/cech-example", "has no plugin plugins/meta/cech-example",
		},
		{"file loose in evals", writeTo("evals/notes.md", "x\n"), "evals/notes.md", "not a category folder"},
		{"a file where a case's project.staged/ belongs", writeTo(evalControl+"project.staged", "x\n"), evalControl + "project.staged", "is not a folder"},
		{"file loose in a plugin's eval folder", writeTo(evalPlugin+"notes.md", "x\n"), evalPlugin + "notes.md", "not project/, project.staged/, routing.md or a case folder"},
		{"case names repeat across plugins", func(t *testing.T, dir string) {
			copyDir("plugins/writing/cech-example", "plugins/writing/cech-other")(t, dir)
			replaceIn("plugins/writing/cech-other/.claude-plugin/plugin.json", `"cech-example"`, `"cech-other"`)(t, dir)
			move("plugins/writing/cech-other/skills/cech-example", "plugins/writing/cech-other/skills/cech-other")(t, dir)
			replaceIn("plugins/writing/cech-other/skills/cech-other/SKILL.md", "name: cech-example", "name: cech-other")(t, dir)
			copyDir(evalControl, "evals/writing/cech-other/cech-example-control")(t, dir)
		}, "evals/writing/cech-other/cech-example-control", "same case name"},
		{"a folder where prompt.md belongs", func(t *testing.T, dir string) {
			remove(evalControl+"prompt.md")(t, dir)
			writeTo(evalControl+"prompt.md/x", "x\n")(t, dir)
		}, evalControl + "prompt.md", "is not a file"},
		{"a file where workspace/ belongs", writeTo(evalControl+claudeRecs+"right/workspace", "x\n"), evalControl + claudeRecs + "right/workspace", "is not a folder"},
		{"file loose in a case folder", writeTo(evalControl+"notes.md", "x\n"), evalControl + "notes.md", "is not prompt.md, cech.toml, recordings/ or project.staged/"},
		{"graders in a folder", writeTo(evalControl+"graders/answer.md", "x\n"), evalControl + "graders", "graders go in cech.toml"},
		{"case.yaml in a case", writeTo(evalLoad+"case.yaml", "name: cech-example-load-1\n"), evalLoad + "case.yaml", "graders go in cech.toml"},
		{"no prompt.md", remove(evalControl + "prompt.md"), evalControl, "has no prompt.md"},
		{"prompt.md with frontmatter", writeTo(evalControl+"prompt.md", "---\nname: x\n---\n\nWhat is 17 times 23?\n"), evalControl + "prompt.md", "starts with frontmatter"},
		{"prompt.md without a prompt", writeTo(evalControl+"prompt.md", " \n\n"), evalControl + "prompt.md", "is empty"},
		// Rules 3 to 5: the fixture project.
		{"no project", removeAll(evalProject), evalPlugin[:len(evalPlugin)-1], "has no project/"},
		{"no AGENTS.md", remove(agents), evalProject[:len(evalProject)-1], "has no AGENTS.md"},
		{"no routing table", replaceIn(agents, "## Routing", "## Routes"), agents, "## Routing"},
		{"a routing table with no header", replaceIn(agents, "| --- | --- | --- |\n", ""), agents, "## Routing"},
		// An unrouted run must see no route to the skill, in any layer.
		{"AGENTS.md names the skill", appendTo(agents, "Use the `cech-example` skill.\n"), agents, "names the skill"},
		{"a nested AGENTS.md names the skill", writeTo(evalProject+"src/AGENTS.md", "Use the `cech-example` skill.\n"), evalProject + "src/AGENTS.md", "names the skill"},
		{"a staged AGENTS.md names the skill", writeTo(evalPlugin+"project.staged/AGENTS.md", "Use the `cech-example` skill.\n"), evalPlugin + "project.staged/AGENTS.md", "names the skill"},
		{"a case's staged AGENTS.md names the skill", writeTo(evalControl+"project.staged/AGENTS.md", "Use the `cech-example` skill.\n"), evalControl + "project.staged/AGENTS.md", "names the skill"},
		{"CLAUDE.md in the project", writeTo(evalProject+"CLAUDE.md", "x\n"), evalProject + "CLAUDE.md", "skip AGENTS.md"},
		{"harness settings in a staged change", writeTo(evalPlugin+"project.staged/.claude/settings.json", "{}\n"), evalPlugin + "project.staged/.claude", "settings or skills"},
		{"Codex settings in the project", writeTo(evalProject+".codex/config.toml", "x = 1\n"), evalProject + ".codex", "settings or skills"},
		{"local path in a staged change", writeTo(evalPlugin+"project.staged/notes.txt", "see /home/someone\n"), evalPlugin + "project.staged/notes.txt", "holds a local path"},
		{"skills in a case's staged change", writeTo(evalControl+"project.staged/.agents/skills/x/SKILL.md", "x\n"), evalControl + "project.staged/.agents", "settings or skills"},
		{"local path in the project", appendTo(evalProject+"README.md", "Kept in /home/someone/pager.\n"), evalProject + "README.md", "holds a local path"},
		// Rule 6: the routing rows.
		{"no routing.md", remove(evalRouting), evalPlugin[:len(evalPlugin)-1], "has no routing.md"},
		{"no row names the skill", replaceIn(evalRouting, "`cech-example`", "`cech-other`"), evalRouting, "no row naming the skill"},
		{"a row with too few cells", writeTo(evalRouting, "| A commit message | the `cech-example` skill |\n"), evalRouting, "has 2 cells"},
		// An escaped | is no cell border, so this row has two cells, not
		// three.
		{"an escaped bar", writeTo(evalRouting, "| A message \\| a note | the `cech-example` skill |\n"), evalRouting, "has 2 cells"},
		{"a header in routing.md", writeTo(evalRouting, "| --- | --- | --- |\n| A commit message | `git commit` | the `cech-example` skill |\n"), evalRouting, "header separator"},
		{"a blank line in routing.md", appendTo(evalRouting, "\n| A message | `git commit` | the `cech-example` skill |\n"), evalRouting, "line 2 is blank"},
		{"text in routing.md", appendTo(evalRouting, "Load it first.\n"), evalRouting, "not a Markdown table row"},
		// Rules 10 to 13: graders.
		{
			"unknown grader key", replaceIn(controlToml, "type = \"regex\"\n", "type = \"regex\"\nexists = true\n"),
			controlToml, `grader answer: takes no key "exists" for type regex`,
		},
		{
			"unknown grader type", replaceIn(controlToml, `type = "regex"`, `type = "regexx"`),
			controlToml, `type "regexx" is not regex or file_exists`,
		},
		{
			"unknown target", replaceIn(controlToml, "type = \"regex\"\n", "type = \"regex\"\ntarget = \"trace\"\n"),
			controlToml, `target "trace" is not last_message, files or file`,
		},
		// Criterion 13 of the eval-runs spec: weight and arm are gone.
		{
			"a weight", replaceIn(controlToml, "type = \"regex\"\n", "type = \"regex\"\nweight = 1\n"),
			controlToml, `grader answer: takes no key "weight" for type regex`,
		},
		{
			"an arm", replaceIn(controlToml, "type = \"regex\"\n", "type = \"regex\"\narm = \"both\"\n"),
			controlToml, `grader answer: takes no key "arm" for type regex`,
		},
		{"absolute glob", replaceIn(trailerToml, "type = \"file_exists\"\npath = \"COMMIT_MSG\"", "type = \"file_exists\"\npath = \"/COMMIT_MSG\""), trailerToml, "not a relative glob"},
		{"file target outside the workspace", replaceIn(trailerToml, "target = \"file\"\npath = \"COMMIT_MSG\"", "target = \"file\"\npath = \"../COMMIT_MSG\""), trailerToml, "not a relative path"},
		{"exists not a boolean", replaceIn(trailerToml, "type = \"file_exists\"\npath = \"COMMIT_MSG\"", "type = \"file_exists\"\npath = \"COMMIT_MSG\"\nexists = \"maybe\""), trailerToml, "exists is not a boolean"},
		{"glob with braces", replaceIn(trailerToml, "type = \"file_exists\"\npath = \"COMMIT_MSG\"", "type = \"file_exists\"\npath = \"{COMMIT_MSG,x}\""), trailerToml, "glob character"},
		{
			"no graders", replaceIn(controlToml, "[[graders]]\nname = \"answer\"\ntype = \"regex\"\npattern = '^391$'\n", ""),
			controlToml, "has no [[graders]]",
		},
		{"a trigger case with a grader", appendTo(loadToml, "\n[[graders]]\nname = \"x\"\ntype = \"regex\"\npattern = 'x'\n"), loadToml, "has [[graders]], but a trigger case has none"},
		// Rules 14 and 15: patterns.
		{
			"pattern that does not compile", replaceIn(controlToml, `'^391$'`, `'^391('`),
			controlToml, `grader answer: pattern "^391("`,
		},
		{
			"placeholder in a pattern", replaceIn(trailerToml, `pattern = 'Change-Kind:'`, `pattern = '\/cech\/workspace'`),
			trailerToml, "names /cech/",
		},
		// Rules 18 and 19: cech.toml and the minimums.
		{"no cech.toml", remove(controlToml), evalControl, "has no cech.toml"},
		{"unknown cech.toml key", replaceIn(controlToml, "family", "colour = \"red\"\nfamily"), controlToml, `unknown key "colour"`},
		{"expect not for the family", replaceIn(controlToml, `expect = "same"`, `expect = "better"`), controlToml, "expect"},
		{
			"claim on a control", replaceIn(controlToml, "expect = \"same\"\n", "expect = \"same\"\nclaim = \"x\"\n"),
			controlToml, "claim is for content cases only",
		},
		{"content without a claim", replaceIn(trailerToml, "claim = ", "# claim = "), trailerToml, "claim is not one line"},
		{"claim on two lines", replaceIn(trailerToml, `claim = "A commit`, `claim = "A\ncommit`), trailerToml, "claim is not one line"},
		// Criterion 13 of the eval-runs spec: critical is gone.
		{"critical on a content case", replaceIn(trailerToml, "expect = \"better\"\n", "expect = \"better\"\ncritical = true\n"), trailerToml, `unknown key "critical"`},
		{"unknown family", replaceIn(controlToml, `family = "control"`, `family = "other"`), controlToml, `family "other"`},
		{"trigger without a wording", replaceIn(loadToml, "wording = \"matched\"\n", ""), loadToml, "wording is missing"},
		{"wording not for the expect", replaceIn(loadToml, `wording = "matched"`, `wording = "near-miss"`), loadToml, `wording "near-miss"`},
		{"wording on a control", replaceIn(controlToml, "expect = \"same\"\n", "expect = \"same\"\nwording = \"matched\"\n"), controlToml, "wording is for trigger cases only"},
		{"no family", replaceIn(controlToml, "family = \"control\"\n", ""), controlToml, "family is missing"},
		{"no expect", replaceIn(controlToml, "expect = \"same\"\n", ""), controlToml, `expect "" is not one of`},
		{"empty claim", replaceIn(trailerToml, `claim = "A commit message ends with a Change-Kind trailer that matches the change."`, `claim = ""`), trailerToml, "claim is not one line"},
		{"too few load cases", removeAll(evalLoad), evalPlugin[:len(evalPlugin)-1], `2 of the 3 trigger cases with expect = "load"`},
		{"too few skip cases", removeAll(evalSkip), evalPlugin[:len(evalPlugin)-1], `2 of the 3 trigger cases with expect = "skip"`},
		{"no oblique prompt", replaceIn(evalOblique+"cech.toml", `"oblique"`, `"matched"`), evalPlugin[:len(evalPlugin)-1], `wording = "oblique"`},
		{"no near miss", replaceIn(evalNear+"cech.toml", `"near-miss"`, `"unrelated"`), evalPlugin[:len(evalPlugin)-1], `wording = "near-miss"`},
		{"no control case", removeAll(evalControl), evalPlugin[:len(evalPlugin)-1], "0 of the 1 control case"},
		{"plugin without evals", removeAll("evals"), "evals/writing/cech-example", "0 of the 1 content case"},
		// Rules 20 to 22: recordings.
		{"no Codex recordings", replaceIn(controlToml, "[recordings.codex]", "[recordings.other]"), controlToml, "has no recordings.codex"},
		{"recordings of no harness", replaceIn(controlToml, "[recordings.codex]", "[recordings.other]"), controlToml, "recordings.other names no harness"},
		{"a folder of no harness", copyDir(evalControl+codexRecs, evalControl+"recordings/other"), evalControl + "recordings/other", "not a harness's folder"},
		{"no pass recording", replaceIn(controlToml, "pass = [\"right\"]\nfail = [\"wrong\"]\n\n[recordings.codex]", "pass = []\nfail = [\"wrong\"]\n\n[recordings.codex]"), controlToml, "recordings.claude-code.pass names no recording"},
		{"no fail recording", replaceIn(controlToml, "fail = [\"wrong\"]\n\n[recordings.codex]", "fail = []\n\n[recordings.codex]"), controlToml, "recordings.claude-code.fail names no recording"},
		{
			"recording not declared", copyDir(evalControl+claudeRecs+"right", evalControl+claudeRecs+"extra"),
			evalControl + claudeRecs + "extra", "not named in recordings",
		},
		{"declared recording missing", removeAll(evalControl + codexRecs + "wrong"), controlToml, `"wrong" has no folder in recordings/codex/`},
		{"recording named twice", replaceIn(controlToml, "fail = [\"wrong\"]\n\n[recordings.codex]", "fail = [\"right\"]\n\n[recordings.codex]"), controlToml, "named twice"},
		{"no trace", remove(evalControl + claudeRecs + "wrong/trace.jsonl"), evalControl + claudeRecs + "wrong", "has no trace.jsonl"},
		{"another file in a recording", writeTo(evalControl+claudeRecs+"wrong/notes.txt", "x\n"), evalControl + claudeRecs + "wrong/notes.txt", "is not trace.jsonl"},
		{"a line that is not JSON", replaceIn(codexRight, `{"type":"turn.completed"}`, "not json\n"+`{"type":"turn.completed"}`), codexRight, "is not a JSON object"},
		{"an empty line", replaceIn(codexRight, `{"type":"turn.completed"}`, "\n"+`{"type":"turn.completed"}`), codexRight, "is empty"},
		{"turn.completed not last", appendTo(codexRight, `{"type":"item.completed","item":{"type":"agent_message","text":"391"}}`+"\n"), codexRight, "comes after the turn.completed line"},
		{"untrimmed message item", replaceIn(codexRight, `"type":"agent_message",`, `"type":"agent_message","id":"item_0",`), codexRight, `"id"`},
		{"text in a user line", replaceIn(realTrace, `{"type":"tool_result","tool_use_id":"toolu_01H39gg5SDfhkd2f5kCrgX45"}`, `{"type":"text","text":"x"}`), realTrace, `type "text"; keep only tool_result blocks`},
		{"private folder", replaceIn(codexReal, "/cech/plugin/", "/private/x/plugin/"), codexReal, "holds a local path"},
		{"untrimmed line", appendTo(realTrace, `{"type":"rate_limit_event"}`+"\n"), realTrace, `type "rate_limit_event"`},
		{"untrimmed key", replaceIn(realTrace, `{"type":"result",`, `{"type":"result","session_id":"s",`), realTrace, `"session_id"`},
		{
			"thinking block", replaceIn(realTrace, `"content":[{"type":"text",`, `"content":[{"type":"thinking","thinking":""},{"type":"text",`),
			realTrace, `type "thinking"`,
		},
		{"untrimmed tool result", replaceIn(realTrace, `"is_error":false}`, `"is_error":false,"content":"x"}`), realTrace, `"content"`},
		{"is_error not a boolean", replaceIn(realTrace, `"is_error":false}`, `"is_error":"no"}`), realTrace, "is_error is not a boolean"},
		// The placeholders are removed before the check, so a real path
		// around one is still caught.
		{"local path", replaceIn(realTrace, "/cech/workspace/COMMIT_MSG", "/home/someone/work/COMMIT_MSG"), realTrace, "holds a local path"},
		{"no result line", replaceAllIn(realTrace, `{"type":"result"`, `{"type":"assistant"`), realTrace, "no result line"},
		{"untrimmed message key", replaceIn(realTrace, `"role":"assistant",`, `"role":"assistant","model":"m",`), realTrace, `"model"`},
		{"untrimmed block key", replaceIn(realTrace, `"name":"Skill",`, `"name":"Skill","caller":{},`), realTrace, `"caller"`},
		{"result line not last", appendTo(realTrace, `{"type":"assistant","message":{"role":"assistant","content":[]}}`+"\n"), realTrace, "comes after the result line"},
		{"null result", replaceIn(realTrace, `"result":"Fix`, `"result":null,"x":"Fix`), realTrace, "result is not a string"},
		{"untrimmed Codex line", replaceIn(codexReal, `{"type":"turn.completed"}`, `{"type":"turn.started"}`+"\n"+`{"type":"turn.completed"}`), codexReal, `type "turn.started"`},
		{"untrimmed Codex item", appendTo(codexRight, `{"type":"item.completed","item":{"type":"reasoning","text":"x"}}`+"\n"), codexRight, `type "reasoning"`},
		{"untrimmed Codex item key", replaceIn(codexReal, `"exit_code":0,`, `"exit_code":0,"id":"item_1",`), codexReal, `"id"`},
		{"usage in turn.completed", replaceIn(codexReal, `{"type":"turn.completed"}`, `{"type":"turn.completed","usage":{}}`), codexReal, `"usage"`},
		{"no turn.completed line", replaceIn(codexRight, `{"type":"turn.completed"}`+"\n", ""), codexRight, "no turn.completed line"},
		{"local path in created.txt", writeTo(evalTrailer+claudeRecs+"real-run/created.txt", "/home/someone/COMMIT_MSG\n"), evalTrailer + claudeRecs + "real-run/created.txt", "holds a local path"},
		{"created.txt names a file not in workspace/", writeTo(evalTrailer+claudeRecs+"real-run/created.txt", "COMMIT_MSG\nOTHER\n"), evalTrailer + claudeRecs + "real-run/created.txt", `"OTHER" is not a file in workspace/`},
		// Rule 25: a verdict finding names what gave the other verdict.
		{"a pass recording names the detector", replaceIn(evalLoad+claudeRecs+"loaded/trace.jsonl", `"tool_use_id":"toolu_1"`, `"tool_use_id":"toolu_9"`), evalLoad + claudeRecs + "loaded", "the claude-code load detector reads no load"},
		{"a fail recording names the Codex detector", copyFile(evalSkip+codexRecs+"loaded/trace.jsonl", evalSkip+codexRecs+"not-loaded/trace.jsonl"), evalSkip + codexRecs + "not-loaded", "the codex load detector reads a load"},
		// Rule 4 holds at any depth.
		{"CLAUDE.md deep in the project", writeTo(evalProject+"src/CLAUDE.md", "x\n"), evalProject + "src/CLAUDE.md", "skip AGENTS.md"},
		{"skills deep in a staged change", writeTo(evalPlugin+"project.staged/src/.agents/skills/x/SKILL.md", "x\n"), evalPlugin + "project.staged/src/.agents", "settings or skills"},
		// Rules 10 and 11: each required key, and each value a key takes.
		{"regex without a pattern", replaceIn(controlToml, "pattern = '^391$'\n", ""), controlToml, "grader answer: has no pattern"},
		{"grader without a name", replaceIn(controlToml, "name = \"answer\"\n", ""), controlToml, "grader #1: has no name"},
		{"a grader name not in its form", replaceIn(controlToml, `name = "answer"`, `name = "Answer"`), controlToml, "grader #1: has no name of lowercase letters"},
		{"a grader name used twice", replaceIn(trailerToml, `name = "file-written"`, `name = "file-listed"`), trailerToml, "grader file-listed: the name is used twice"},
		{"count without match count", replaceIn(trailerToml, `name = "reply-trailer"`, "name = \"reply-trailer\"\ncount = 1"), trailerToml, "count is only for match"},
		{"match count without count", replaceIn(trailerToml, "count = 1\n", ""), trailerToml, `grader one-trailer: match = "count" has no count`},
		{"path without target file", replaceIn(trailerToml, `name = "reply-trailer"`, "name = \"reply-trailer\"\npath = \"COMMIT_MSG\""), trailerToml, "path is only for target"},
		{"target file without a path", replaceIn(trailerToml, "target = \"file\"\npath = \"COMMIT_MSG\"\n", "target = \"file\"\n"), trailerToml, "has no path"},
		{"grader without a type", replaceIn(controlToml, "type = \"regex\"\n", ""), controlToml, "grader answer: type is missing"},
		{"file_exists without a path", replaceIn(trailerToml, "type = \"file_exists\"\npath = \"COMMIT_MSG\"\n", "type = \"file_exists\"\n"), trailerToml, "grader file-written: has no path"},
		{"unknown match", replaceIn(trailerToml, `match = "not_contains"`, `match = "equals"`), trailerToml, `match "equals" is not contains`},
		{"count of no number", replaceIn(trailerToml, "count = 1", "count = -1"), trailerToml, "count is not a whole number"},
		{"a .. part in a glob", replaceIn(trailerToml, "type = \"file_exists\"\npath = \"COMMIT_MSG\"", "type = \"file_exists\"\npath = \"x/../COMMIT_MSG\""), trailerToml, "not a relative glob"},
		{"a drive letter in a file target", replaceIn(trailerToml, `path = "COMMIT_MSG"`+"\n\n", `path = "C:COMMIT_MSG"`+"\n\n"), trailerToml, "not a relative path inside the workspace"},
		{"a backslash in a glob", replaceIn(trailerToml, "type = \"file_exists\"\npath = \"COMMIT_MSG\"", "type = \"file_exists\"\npath = \"x\\\\COMMIT_MSG\""), trailerToml, "not a relative glob"},
		{"an empty part in a glob", replaceIn(trailerToml, "type = \"file_exists\"\npath = \"COMMIT_MSG\"", "type = \"file_exists\"\npath = \"x//COMMIT_MSG\""), trailerToml, "not a relative glob"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			copyTree(t, evalFixture, dir, nil)
			tt.change(t, dir)
			code, stdout, stderr := runIn(t, "eval", "--check", "-C", dir)
			if code != ExitFailure {
				t.Errorf("exit %d, want %d; stdout:\n%s", code, ExitFailure, stdout)
			}
			if !strings.Contains(stderr, "finding") {
				t.Errorf("stderr %q does not count the findings", stderr)
			}
			for line := range strings.SplitSeq(strings.TrimSuffix(stdout, "\n"), "\n") {
				if p, msg, _ := strings.Cut(line, ": "); p == strings.TrimSuffix(tt.path, "/") && strings.Contains(msg, tt.want) {
					return
				}
			}
			t.Errorf("no finding %s: …%s…; stdout:\n%s", tt.path, tt.want, stdout)
		})
	}
}

// TestEvalCheckKeepsChecking checks that a problem in one file does not
// hide the findings of the rest (spec rule 28): a cech.toml with an
// unknown key still lets the case's recordings be graded, and a file
// where recordings/ belongs is a finding, not an error that skips the
// case.
func TestEvalCheckKeepsChecking(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(t *testing.T, dir string)
		want   []string
	}{
		{"unknown cech.toml key", func(t *testing.T, dir string) {
			replaceIn(evalControl+"cech.toml", "family", "extra = 1\nfamily")(t, dir)
			replaceIn(evalControl+"cech.toml", `'^391$'`, `'\d'`)(t, dir)
		}, []string{evalControl + "cech.toml: has the unknown key", evalControl + claudeRecs + "wrong: must fail", evalControl + codexRecs + "wrong: must fail"}},
		{"a file where recordings/ belongs", func(t *testing.T, dir string) {
			removeAll(evalControl+"recordings")(t, dir)
			writeTo(evalControl+"recordings", "x\n")(t, dir)
		}, []string{evalControl + "recordings: is not a folder", evalControl + `cech.toml: recording "right" has no folder in recordings/claude-code/`}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			copyTree(t, evalFixture, dir, nil)
			tt.change(t, dir)
			code, stdout, stderr := runIn(t, "eval", "--check", "-C", dir)
			if code != ExitFailure {
				t.Errorf("exit %d, want %d", code, ExitFailure)
			}
			for _, w := range tt.want {
				if !strings.Contains(stdout, w) {
					t.Errorf("no finding %s; stdout:\n%s", w, stdout)
				}
			}
			if strings.Count(stderr, "\n") != 1 || !strings.Contains(stderr, "finding") {
				t.Errorf("stderr %q is not only the count of findings", stderr)
			}
		})
	}
}

// TestEvalCheckFindingsInOneFile checks that every problem in a file is
// named, not only the first (spec rule 28).
func TestEvalCheckFindingsInOneFile(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, evalFixture, dir, nil)
	replaceIn(evalControl+"cech.toml", "type = \"regex\"\n", "type = \"regex\"\nexists = true\nweight = 0\n")(t, dir)
	code, stdout, _ := runIn(t, "eval", "--check", "-C", dir)
	file := evalControl + "cech.toml: grader answer: "
	if code != ExitFailure || !strings.Contains(stdout, file+`takes no key "exists" for type regex`) || !strings.Contains(stdout, file+`takes no key "weight" for type regex`) {
		t.Errorf("exit %d; want %d and both findings in %s; stdout:\n%s", code, ExitFailure, file, stdout)
	}
}

// TestEvalCheckOneFinding checks that a grader key of the wrong type
// gives one finding, not a second one about the key it then lacks, and
// that a grader that did not parse leaves the recordings ungraded, whose
// verdicts the rest would give wrongly (spec rules 10 and 16).
func TestEvalCheckOneFinding(t *testing.T) {
	file := evalControl + "cech.toml"
	for _, tt := range []struct{ name, from, to, want string }{
		{"a name that is not a string", `name = "answer"`, "name = 1", "grader #1: name is not a string"},
		{"a type that is not a string", `type = "regex"`, "type = 1", "grader answer: type is not a string"},
		{"a table in a grader key", "pattern = '^391$'", "pattern = {a = 1}", "grader answer: pattern is not a string"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			copyTree(t, evalFixture, dir, nil)
			replaceIn(file, tt.from, tt.to)(t, dir)
			code, stdout, _ := runIn(t, "eval", "--check", "-C", dir)
			if want := file + ": " + tt.want + "\n"; code != ExitFailure || stdout != want {
				t.Errorf("exit %d, stdout:\n%s\nwant %d and only %s", code, stdout, ExitFailure, want)
			}
		})
	}
}

// TestEvalCheckRejectedSource checks that a source cech build rejects is
// an error that fails the check, with the cases skipped and said so
// (spec rule 27). It guards the error passing for a denied read, whose
// exit 3 a script would retry when no retry can fix it.
func TestEvalCheckRejectedSource(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, evalFixture, dir, nil)
	skill := "plugins/writing/cech-example/skills/cech-example/SKILL.md"
	replaceIn(skill, "name: cech-example", "name: other")(t, dir)
	code, stdout, stderr := runIn(t, "eval", "--check", "-C", dir)
	if code != ExitFailure || stdout != "" || !strings.Contains(stderr, skill) || !strings.Contains(stderr, "sources have errors") {
		t.Errorf("exit %d, stdout %q, stderr %q; want %d, no findings, and stderr naming %s and the skip", code, stdout, stderr, ExitFailure, skill)
	}
	code, stdout, stderr = runIn(t, "eval", "--check", "--json", "-C", dir)
	var res struct {
		OK       bool             `json:"ok"`
		Findings []jsontext.Value `json:"findings"`
		Errors   []string         `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatal(err)
	}
	errs := strings.Join(res.Errors, "\n")
	if code != ExitFailure || res.OK || res.Findings == nil || len(res.Findings) != 0 || stderr != "" ||
		!strings.Contains(errs, skill) || !strings.Contains(errs, "sources have errors") {
		t.Errorf("--json: exit %d, %s, stderr %q; want %d, findings [], and errors naming %s and the skip", code, stdout, stderr, ExitFailure, skill)
	}
}

// TestEvalCheckJSONFindings checks the JSON a script reads (spec rule
// 29), with names of its own, so a renamed field shows.
func TestEvalCheckJSONFindings(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, evalFixture, dir, nil)
	remove(evalControl+"cech.toml")(t, dir)
	code, stdout, stderr := runIn(t, "eval", "--check", "--json", "-C", dir)
	var res struct {
		OK       bool `json:"ok"`
		Findings []struct {
			Path    string `json:"path"`
			Message string `json:"message"`
		} `json:"findings"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatal(err)
	}
	if code != ExitFailure || res.OK || len(res.Errors) != 0 || stderr != "" {
		t.Errorf("exit %d, %+v, stderr %q; want a failure with findings only, and nothing on stderr", code, res, stderr)
	}
	want := strings.TrimSuffix(evalControl, "/")
	for _, f := range res.Findings {
		if f.Path == want && strings.Contains(f.Message, "has no cech.toml") {
			return
		}
	}
	t.Errorf("findings %+v hold no %s: has no cech.toml", res.Findings, want)
}

// TestEvalCheckUnreadable checks that a read the check is denied makes
// it exit 3, as it could not decide, unless a finding fails it, and that
// it names what it could not read, in --json too (ADR 15 item 5).
func TestEvalCheckUnreadable(t *testing.T) {
	skipWithoutModes(t)
	for _, tt := range []struct {
		name    string
		change  func(t *testing.T, dir string)
		code    int
		denied  string // a path the errors name
		skipped string // what the errors say was skipped
	}{
		{"one case file unreadable", chmod(evalControl+"cech.toml", 0), ExitUnreachable, evalControl + "cech.toml", "skipped the rest of case"},
		{"a fixture project file unreadable", chmod(evalPlugin+"project/AGENTS.md", 0), ExitUnreachable, evalPlugin + "project/AGENTS.md", "fixture project and routing rows"},
		{"a folder above the cases unreadable", chmodDir("evals/writing"), ExitUnreachable, "evals/writing", "every plugin's minimum cases"},
		// A denied read must not stop the walk, or exit 3 would hide the
		// finding after it.
		{"an unreadable folder and a later finding", func(t *testing.T, dir string) {
			chmodDir("evals/writing")(t, dir)
			writeTo("evals/zzz", "x\n")(t, dir)
		}, ExitFailure, "evals/writing", "every plugin's minimum cases"},
		{"unreadable and a finding", func(t *testing.T, dir string) {
			chmod(evalControl+"cech.toml", 0)(t, dir)
			writeTo("evals/notes.md", "x\n")(t, dir)
		}, ExitFailure, evalControl + "cech.toml", "skipped the rest of case"},
		// A denied plugin source leaves every plugin unknown, so the cases
		// are skipped, and the output says so.
		{"a plugin source unreadable", chmod("plugins/writing/cech-example/.claude-plugin/plugin.json", 0), ExitUnreachable, "sources could not be read", "skipped the eval cases"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			copyTree(t, evalFixture, dir, nil)
			tt.change(t, dir)
			code, stdout, stderr := runIn(t, "eval", "--check", "-C", dir)
			if code != tt.code || !strings.Contains(stderr, tt.denied) || !strings.Contains(stderr, tt.skipped) {
				t.Errorf("exit %d, want %d; stdout %q; stderr %q, want it to name %s and say %q", code, tt.code, stdout, stderr, tt.denied, tt.skipped)
			}
			code, stdout, _ = runIn(t, "eval", "--check", "--json", "-C", dir)
			var res struct {
				OK     bool     `json:"ok"`
				Errors []string `json:"errors"`
			}
			if err := json.Unmarshal([]byte(stdout), &res); err != nil {
				t.Fatalf("--json: %v in %q", err, stdout)
			}
			if errs := strings.Join(res.Errors, "\n"); code != tt.code || res.OK || !strings.Contains(errs, tt.denied) || !strings.Contains(errs, tt.skipped) {
				t.Errorf("--json: exit %d, %+v; want %d and errors naming %s and saying %q", code, res, tt.code, tt.denied, tt.skipped)
			}
		})
	}
}

// chmodDir returns a change that denies all access to the folder at p,
// and gives it back when the test ends so the folder can be removed.
func chmodDir(p string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		name := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.Chmod(name, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(name, 0o750) })
	}
}
