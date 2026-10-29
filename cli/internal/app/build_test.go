package app

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/TrialAndTemper/cech/cli/internal/build"
)

// The build fixtures are marketplaces in testdata/build/<case>/: in/ is
// the tree before cech build, and out/ the tree after it, which -update
// rewrites. Their goldens are build-<case>*.golden.
var buildCases = []string{"acme", "empty"}

// TestBuild runs cech build on each fixture: --check and --dry-run list
// the changes and write nothing, the build writes exactly out/, a second
// build changes nothing, and with --json each mode prints its object.
func TestBuild(t *testing.T) {
	for _, name := range buildCases {
		t.Run(name, func(t *testing.T) {
			fixture := filepath.Join("testdata", "build", name)
			dir := t.TempDir()
			copyTree(t, filepath.Join(fixture, "in"), dir, nil)
			before := readTree(t, dir)

			code, stdout, stderr := runIn(t, "build", "--check", "-C", dir)
			if code != ExitFailure {
				t.Errorf("--check: exit code %d, want %d; stderr:\n%s", code, ExitFailure, stderr)
			}
			checkGolden(t, "build-"+name+"-check.golden", []byte(stdout))
			if !strings.HasSuffix(stderr, " from their sources; run cech build\n") {
				t.Errorf("--check: stderr = %q, want the count of files that differ", stderr)
			}
			code, stdout, _ = runIn(t, "build", "--check", "--json", "-C", dir)
			if code != ExitFailure {
				t.Errorf("--check --json: exit code %d, want %d", code, ExitFailure)
			}
			checkGolden(t, "build-"+name+"-check-json.golden", []byte(stdout))
			code, stdout, stderr = runIn(t, "build", "--dry-run", "-C", dir)
			if code != ExitOK || stderr != "" {
				t.Errorf("--dry-run: exit code %d, stderr %q; want %d and nothing", code, stderr, ExitOK)
			}
			checkGolden(t, "build-"+name+"-check.golden", []byte(stdout))
			code, stdout, _ = runIn(t, "build", "--dry-run", "--json", "-C", dir)
			if code != ExitOK {
				t.Errorf("--dry-run --json: exit code %d, want %d", code, ExitOK)
			}
			checkGolden(t, "build-"+name+"-dry-run-json.golden", []byte(stdout))
			if after := readTree(t, dir); !maps.Equal(before, after) {
				t.Fatal("--check or --dry-run changed the tree")
			}

			code, stdout, stderr = runIn(t, "build", "-C", dir)
			if code != ExitOK {
				t.Fatalf("build: exit code %d; stderr:\n%s", code, stderr)
			}
			checkGolden(t, "build-"+name+".golden", []byte(stdout))
			checkTree(t, filepath.Join(fixture, "out"), dir)
			fresh := t.TempDir()
			copyTree(t, filepath.Join(fixture, "in"), fresh, nil)
			code, stdout, stderr = runIn(t, "build", "--json", "-C", fresh)
			if code != ExitOK {
				t.Fatalf("build --json: exit code %d; stderr:\n%s", code, stderr)
			}
			checkGolden(t, "build-"+name+"-json.golden", []byte(stdout))

			for _, args := range [][]string{{"build"}, {"build", "--check"}} {
				code, stdout, stderr = runIn(t, append(args, "-C", dir)...)
				if code != ExitOK || stdout != "" || stderr != "" {
					t.Errorf("%v after a build: exit code %d, stdout %q, stderr %q; want 0 and no output",
						args, code, stdout, stderr)
				}
			}
			code, stdout, _ = runIn(t, "build", "--check", "--json", "-C", dir)
			if want := `{"mode":"check","ok":true,"files":[],"skipped":[],"errors":[]}` + "\n"; code != ExitOK || stdout != want {
				t.Errorf("--check --json after a build: exit code %d, stdout %q; want 0 and %q", code, stdout, want)
			}
		})
	}
}

// TestBuildInCurrentFolder checks that without -C the build works on
// the marketplace in the current folder.
func TestBuildInCurrentFolder(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "build", "acme", "out"), dir, nil)
	t.Chdir(dir)
	if code, stdout, stderr := runIn(t, "build", "--check"); code != ExitOK || stdout != "" {
		t.Errorf("exit code %d, stdout %q, stderr:\n%s\nwant 0 and nothing", code, stdout, stderr)
	}
}

// TestBuildRelease builds a release of the built acme fixture: the
// catalogue gets the release's name and every plugin its version, and
// --check passes only with the same --release (ADR 14 item 3).
func TestBuildRelease(t *testing.T) {
	want := "update .claude-plugin/marketplace.json\n" +
		"update plugins/tool/acme-docker/.claude-plugin/plugin.json\n" +
		"update plugins/tool/acme-git/.claude-plugin/plugin.json\n" +
		"update plugins/workflow/acme-release/.claude-plugin/plugin.json\n" +
		"update plugins/writing/acme-prose/.claude-plugin/plugin.json\n"
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "build", "acme", "out"), dir, nil)
	if code, stdout, _ := runIn(t, "build", "--check", "--release", "0.2.0-rc.1", "-C", dir); code != ExitFailure || stdout != want {
		t.Errorf("--check --release before: exit code %d, stdout:\n%s\nwant %d and:\n%s", code, stdout, ExitFailure, want)
	}
	if code, _, stderr := runIn(t, "build", "--release", "0.2.0-rc.1", "-C", dir); code != ExitOK {
		t.Fatalf("build --release: exit code %d; stderr:\n%s", code, stderr)
	}
	var catalogue struct{ Name string }
	readJSON(t, filepath.Join(dir, ".claude-plugin", "marketplace.json"), &catalogue)
	if catalogue.Name != "acme-0-2-0-rc-1" {
		t.Errorf("catalogue name %q, want acme-0-2-0-rc-1", catalogue.Name)
	}
	for _, p := range strings.Split(strings.TrimSpace(want), "\n")[1:] {
		var manifest struct{ Version string }
		readJSON(t, filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(p, "update "))), &manifest)
		if manifest.Version != "0.2.0-rc.1" {
			t.Errorf("%s: version %q, want 0.2.0-rc.1", p, manifest.Version)
		}
	}
	if code, stdout, _ := runIn(t, "build", "--check", "--release", "0.2.0-rc.1", "-C", dir); code != ExitOK || stdout != "" {
		t.Errorf("--check --release after: exit code %d, stdout %q; want 0 and nothing", code, stdout)
	}
	if code, stdout, _ := runIn(t, "build", "--check", "-C", dir); code != ExitFailure || stdout != want {
		t.Errorf("--check without --release: exit code %d, stdout:\n%s\nwant %d and:\n%s", code, stdout, ExitFailure, want)
	}
}

// TestBuildReleaseOnPreRelease checks that only a pre-release builds on
// a CECH pre-release (ADR 14 item 5), and that a dependency on a CECH
// plugin then names that pre-release's catalogue.
func TestBuildReleaseOnPreRelease(t *testing.T) {
	for _, tt := range []struct {
		release string
		code    int
		stderr  string
	}{
		{"0.2.0", ExitFailure, `cech: data/marketplace.toml: cech is "0.2.0-rc.1", a pre-release, but final release 0.2.0 must build on a final CECH release (ADR 14 item 5)` + "\n"},
		{"0.2.0-rc.1", ExitOK, ""},
	} {
		t.Run(tt.release, func(t *testing.T) {
			dir := t.TempDir()
			copyTree(t, filepath.Join("testdata", "build", "acme", "in"), dir, nil)
			replaceIn("data/marketplace.toml", `cech = "0.1.0"`, `cech = "0.2.0-rc.1"`)(t, dir)
			before := readTree(t, dir)
			code, _, stderr := runIn(t, "build", "--release", tt.release, "-C", dir)
			if code != tt.code || stderr != tt.stderr {
				t.Errorf("exit code %d, stderr %q; want %d and %q", code, stderr, tt.code, tt.stderr)
			}
			after := readTree(t, dir)
			if tt.code != ExitOK && !maps.Equal(before, after) {
				t.Error("a refused release build changed the tree")
			}
			dep := `{ "name": "cech-git", "marketplace": "cech-0-2-0-rc-1" }`
			if manifest := after["plugins/writing/acme-prose/.claude-plugin/plugin.json"]; tt.code == ExitOK && !strings.Contains(manifest, dep) {
				t.Errorf("acme-prose's plugin.json does not hold %s:\n%s", dep, manifest)
			}
		})
	}
}

// TestBuildWithoutLicence checks that without a root LICENSE the build
// leaves the copies in the skill folders, which it cannot tell from
// files written by hand (ADR 18 item 2).
func TestBuildWithoutLicence(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "build", "acme", "out"), dir, nil)
	remove("LICENSE")(t, dir)
	before := readTree(t, dir)
	if code, stdout, stderr := runIn(t, "build", "-C", dir); code != ExitOK || stdout != "" {
		t.Errorf("exit code %d, stdout %q, stderr:\n%s\nwant 0 and nothing", code, stdout, stderr)
	}
	if after := readTree(t, dir); !maps.Equal(before, after) {
		t.Error("the build changed the tree")
	}
}

// readJSON decodes the JSON file name into v.
func readJSON(t *testing.T, name string, v any) {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

// TestBuildCRLF builds the acme fixture with CRLF line ends in every
// hand-written file. A plugin.json keeps them, the inserted description
// member included, and so does each copy of the licence; every other
// generated file has LF.
func TestBuildCRLF(t *testing.T) {
	fixture := filepath.Join("testdata", "build", "acme")
	dir := t.TempDir()
	handWritten := func(p string) bool {
		return strings.HasSuffix(p, "/plugin.json") || strings.HasSuffix(p, ".md") || p == "LICENSE" ||
			(strings.HasSuffix(p, ".toml") && !strings.Contains(p, "/agents/"))
	}
	copyTree(t, filepath.Join(fixture, "in"), dir, handWritten)
	if code, _, stderr := runIn(t, "build", "-C", dir); code != ExitOK {
		t.Fatalf("build: exit code %d; stderr:\n%s", code, stderr)
	}
	want := t.TempDir()
	copyTree(t, filepath.Join(fixture, "out"), want, func(p string) bool {
		return handWritten(p) && !strings.HasPrefix(p, "docs/") || strings.HasSuffix(p, "/LICENSE")
	})
	if !*update {
		checkTree(t, want, dir)
	}
}

// TestBuildCheckFindsDrift changes one thing in a built tree and checks
// that --check fails naming exactly the files it affects.
func TestBuildCheckFindsDrift(t *testing.T) {
	prose := "plugins/writing/acme-prose/"
	git := "plugins/tool/acme-git/"
	tests := []struct {
		name   string
		change func(t *testing.T, dir string)
		want   string
	}{
		{
			"catalogue edited by hand", appendTo(".claude-plugin/marketplace.json", " "),
			"update .claude-plugin/marketplace.json\n",
		},
		{
			"catalogue removed", remove(".claude-plugin/marketplace.json"),
			"create .claude-plugin/marketplace.json\n",
		},
		{
			"code owners edited by hand", appendTo(".github/CODEOWNERS", "/docs/ @mallory\n"),
			"update .github/CODEOWNERS\n",
		},
		{
			"catalogue page with CRLF", toCRLF("docs/catalogue.md"),
			"update docs/catalogue.md\n",
		},
		{
			"page removed", remove("docs/catalogue.md"),
			"create docs/catalogue.md\n",
		},
		{
			"description edited in plugin.json", replaceIn(prose+".claude-plugin/plugin.json", "house style", "style"),
			"update " + prose + ".claude-plugin/plugin.json\n",
		},
		{
			"description removed from plugin.json", replaceIn(git+".claude-plugin/plugin.json",
				`"description":"Runs git the acme way: small commits & clear messages.",`, ""),
			"update " + git + ".claude-plugin/plugin.json\n",
		},
		{
			"description changed in SKILL.md", replaceIn(prose+"skills/acme-prose/SKILL.md", "house style", "style"),
			"update .claude-plugin/marketplace.json\nupdate docs/catalogue.md\n" +
				"update " + prose + ".claude-plugin/plugin.json\n",
		},
		{
			"maintainer added", replaceIn(git+".claude-plugin/plugin.json", `["dave"]`, `["dave","erin"]`),
			"update .github/CODEOWNERS\n",
		},
		{
			"owner changed", replaceIn(prose+".claude-plugin/plugin.json", "alice-dev", "frank"),
			"update .github/CODEOWNERS\n",
		},
		{
			"Codex agent missing", remove(prose + "agents/acme-prose-reviewer.toml"),
			"create " + prose + "agents/acme-prose-reviewer.toml\n",
		},
		{
			"Codex agent edited by hand", appendTo(prose+"agents/acme-prose-reviewer.toml", "model = \"x\"\n"),
			"update " + prose + "agents/acme-prose-reviewer.toml\n",
		},
		{
			"agent changed", replaceIn(prose+"agents/acme-prose-reviewer.md", "You review", "You check"),
			"update " + prose + "agents/acme-prose-reviewer.toml\n",
		},
		{
			"Codex agent left without its Markdown", remove(prose + "agents/acme-prose-reviewer.md"),
			"remove " + prose + "agents/acme-prose-reviewer.toml\n",
		},
		{
			"plugin moved to another category", move("plugins/tool/acme-git", "plugins/workflow/acme-git"),
			"update .claude-plugin/marketplace.json\nupdate .github/CODEOWNERS\nupdate docs/catalogue.md\n",
		},
		{
			"namespace changed", func(t *testing.T, dir string) {
				replaceIn("data/marketplace.toml", `namespace = "acme"`, `namespace = "acme-corp"`)(t, dir)
				replaceIn("data/marketplace.toml", `"acme-tracker"`, `"acme-corp-tracker"`)(t, dir)
				replaceIn("data/marketplace.toml", `"acme-brand-voice"`, `"acme-corp-brand-voice"`)(t, dir)
			},
			"update .claude-plugin/marketplace.json\nupdate docs/catalogue.md\n",
		},
		{
			// Only an extension's dependencies get a marketplace (ADR 20
			// item 3); CECH's own build leaves each one as written.
			"data file made CECH's own", func(t *testing.T, dir string) {
				cechOwn(t, dir)
				replaceIn("data/marketplace.toml", "[[categories]]\nname = \"brand\"\nholds = \"Brand voice and visual identity\"\n", "")(t, dir)
				replaceIn("data/marketplace.toml", "[terms]\n", "")(t, dir)
				replaceIn("data/marketplace.toml", `rules = ["checks/terms/rules.toml"]`+"\n", "")(t, dir)
				replaceIn("data/marketplace.toml", `names = ["checks/terms/names.txt"]`+"\n", "")(t, dir)
			},
			"update .claude-plugin/marketplace.json\nupdate docs/catalogue.md\n",
		},
		{
			"catalogue description changed", replaceIn("data/marketplace.toml", "built on CECH", "on CECH"),
			"update .claude-plugin/marketplace.json\n",
		},
		{
			"CECH release changed", replaceIn("data/marketplace.toml", `cech = "0.1.0"`, `cech = "0.2.0"`),
			"update " + git + ".claude-plugin/plugin.json\nupdate " + prose + ".claude-plugin/plugin.json\n",
		},
		{
			"CECH pre-release", replaceIn("data/marketplace.toml", `cech = "0.1.0"`, `cech = "0.2.0-rc.1"`),
			"update " + git + ".claude-plugin/plugin.json\nupdate " + prose + ".claude-plugin/plugin.json\n",
		},
		{
			"version edited by hand", replaceIn(prose+".claude-plugin/plugin.json", `"version": "0.0.0-dev"`, `"version": "1.2.0"`),
			"update " + prose + ".claude-plugin/plugin.json\n",
		},
		{
			"dependency marketplace edited by hand", replaceIn(prose+".claude-plugin/plugin.json",
				`"cech-git", "marketplace": "cech-0-1-0"`, `"cech-git", "marketplace": "cech"`),
			"update " + prose + ".claude-plugin/plugin.json\n",
		},
		{
			"added category's holds changed", replaceIn("data/marketplace.toml", "Brand voice", "House voice"),
			"update docs/catalogue.md\n",
		},
		{
			"catalogue owner changed", replaceIn("data/marketplace.toml", "skills@acme.example", "team@acme.example"),
			"update .claude-plugin/marketplace.json\n",
		},
		{
			"code owners team changed", replaceIn("data/marketplace.toml", "@acme/skill-maintainers", "@acme/maintainers"),
			"update .github/CODEOWNERS\n",
		},
		{
			"code owners team removed", replaceIn("data/marketplace.toml", `team = "@acme/skill-maintainers"`, ""),
			"remove .github/CODEOWNERS\n",
		},
		{
			"licence changed", appendTo("LICENSE", "More terms.\n"),
			"update plugins/tool/acme-docker/skills/acme-docker/LICENSE\n" +
				"update " + git + "skills/acme-git/LICENSE\n" +
				"update plugins/workflow/acme-release/skills/acme-release/LICENSE\n" +
				"update " + prose + "skills/acme-prose/LICENSE\n",
		},
		{
			"licence copy edited", appendTo(prose+"skills/acme-prose/LICENSE", "More terms.\n"),
			"update " + prose + "skills/acme-prose/LICENSE\n",
		},
		{
			"plugin removed", removeAll("plugins/workflow/acme-release"),
			"update .claude-plugin/marketplace.json\nupdate docs/catalogue.md\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			copyTree(t, filepath.Join("testdata", "build", "acme", "out"), dir, nil)
			tt.change(t, dir)
			before := readTree(t, dir)
			code, stdout, stderr := runIn(t, "build", "--check", "-C", dir)
			if code != ExitFailure {
				t.Errorf("exit code %d, want %d; stderr:\n%s", code, ExitFailure, stderr)
			}
			if stdout != tt.want {
				t.Errorf("stdout:\n%s\nwant:\n%s", stdout, tt.want)
			}
			if n := strings.Count(tt.want, "\n"); n == 1 && !strings.Contains(stderr, "1 generated file differs") ||
				n > 1 && !strings.Contains(stderr, "generated files differ") {
				t.Errorf("stderr = %q, want the count of files that differ", stderr)
			}
			if after := readTree(t, dir); !maps.Equal(before, after) {
				t.Error("--check changed the tree")
			}
		})
	}
}

// TestBuildErrors checks that the build refuses sources it cannot build
// from, names each problem with its file, writes nothing, and with
// --json still prints one object.
func TestBuildErrors(t *testing.T) {
	prose := "plugins/writing/acme-prose/"
	manifest := prose + ".claude-plugin/plugin.json"
	skill := prose + "skills/acme-prose/SKILL.md"
	release := "plugins/workflow/acme-release/.claude-plugin/plugin.json"
	gitSkill := "plugins/tool/acme-git/skills/acme-git/SKILL.md"
	gitManifest := "plugins/tool/acme-git/.claude-plugin/plugin.json"
	data := "data/marketplace.toml"
	agent := prose + "agents/acme-prose-reviewer.md"
	tests := []struct {
		name   string
		change func(t *testing.T, dir string)
		want   []string // each error, in order
	}{
		{
			"no data file", remove(data),
			[]string{"data/marketplace.toml: not found; run cech build at the root of a marketplace with CECH's layout"},
		},
		{
			"unknown key in the data file", replaceIn(data, `namespace = "acme"`, "colour = \"red\"\nnamespace = \"acme\""),
			[]string{`data/marketplace.toml: unknown key "colour"`},
		},
		{
			"team not a team", replaceIn(data, `"@acme/skill-maintainers"`, `"@alice-dev"`),
			[]string{`data/marketplace.toml: team is "@alice-dev", not a team as @org/team`},
		},
		{
			"empty team", replaceIn(data, `"@acme/skill-maintainers"`, `""`),
			[]string{`data/marketplace.toml: team is "", not a team as @org/team`},
		},
		{
			"no namespace", replaceIn(data, `namespace = "acme"`+"\n", ""),
			[]string{"data/marketplace.toml: namespace is missing"},
		},
		{
			"namespace not a name", replaceIn(data, `namespace = "acme"`, `namespace = "Acme"`),
			[]string{`data/marketplace.toml: namespace "Acme" is not a valid name: use a-z`},
		},
		{
			"namespace ending in a version", replaceIn(data, `namespace = "acme"`, `namespace = "acme-1-4-0"`),
			[]string{`data/marketplace.toml: namespace "acme-1-4-0" ends in a version, which a release adds (ADR 17 item 7)`},
		},
		{
			"namespace ending in a pre-release", replaceIn(data, `namespace = "acme"`, `namespace = "acme-1-4-0-rc-2"`),
			[]string{`data/marketplace.toml: namespace "acme-1-4-0-rc-2" ends in a version, which a release adds (ADR 17 item 7)`},
		},
		{
			"extension namespace reserved for CECH", replaceIn(data, `namespace = "acme"`, `namespace = "cech-acme"`),
			[]string{`data/marketplace.toml: namespace "cech-acme" is reserved for CECH (ADR 12 item 5)`},
		},
		{
			"extension namespace cech", replaceIn(data, `namespace = "acme"`, `namespace = "cech"`),
			[]string{`data/marketplace.toml: namespace "cech" is reserved for CECH (ADR 12 item 5)`},
		},
		{
			"no description", replaceIn(data, `description = "Acme's agent skills, built on CECH"`, ""),
			[]string{"data/marketplace.toml: description is missing"},
		},
		{
			"cech not a release", replaceIn(data, `cech = "0.1.0"`, `cech = "v0.1.0"`),
			[]string{`data/marketplace.toml: cech is "v0.1.0", not a CECH release as X.Y.Z or X.Y.Z-rc.N`},
		},
		{
			"cech with a leading zero", replaceIn(data, `cech = "0.1.0"`, `cech = "0.01.0"`),
			[]string{`data/marketplace.toml: cech is "0.01.0", not a CECH release as X.Y.Z or X.Y.Z-rc.N`},
		},
		{
			"cech with rc.0", replaceIn(data, `cech = "0.1.0"`, `cech = "0.1.0-rc.0"`),
			[]string{`data/marketplace.toml: cech is "0.1.0-rc.0", not a CECH release as X.Y.Z or X.Y.Z-rc.N`},
		},
		{
			"no cech key and another namespace", replaceIn(data, `cech = "0.1.0"`+"\n", ""),
			[]string{`data/marketplace.toml: namespace is "acme", but a file without cech is CECH's, whose namespace is cech (ADR 17 item 7)`},
		},
		{
			"categories in CECH's own file", cechOwn,
			[]string{"data/marketplace.toml: categories is out of place: only an extension, with a cech key, adds categories"},
		},
		{"terms in CECH's own file", func(t *testing.T, dir string) {
			cechOwn(t, dir)
			replaceIn(data, "[[categories]]\nname = \"brand\"\nholds = \"Brand voice and visual identity\"\n", "")(t, dir)
		}, []string{"data/marketplace.toml: terms is out of place: only an extension, with a cech key, has term lists"}},
		{
			"added category not a name", replaceIn(data, `name = "brand"`, `name = "Brand"`),
			[]string{`data/marketplace.toml: categories[0].name "Brand" is not a valid name: use a-z`},
		},
		{
			"category one of CECH's", replaceIn(data, `name = "brand"`, `name = "tool"`),
			[]string{`data/marketplace.toml: categories[0].name "tool" is one of CECH's categories (ADR 12 item 1)`},
		},
		{
			"category added twice", appendTo(data, "\n[[categories]]\nname = \"brand\"\nholds = \"Again\"\n"),
			[]string{`data/marketplace.toml: categories[1].name "brand" is already added`},
		},
		{
			"category without holds", replaceIn(data, `holds = "Brand voice and visual identity"`, ""),
			[]string{"data/marketplace.toml: categories[0].holds is missing"},
		},
		{
			"role not a name", replaceIn(data, `name = "acme-tracker"`, `name = "Acme-Tracker"`),
			[]string{`data/marketplace.toml: roles[0].name "Acme-Tracker" is not a valid name: use a-z`},
		},
		{
			"role named as CECH's", replaceIn(data, `name = "acme-tracker"`, `name = "cech-tracker"`),
			[]string{`data/marketplace.toml: roles[0].name "cech-tracker" starts with cech-, which no role name does (ADR 21 item 2)`},
		},
		{
			"role without the namespace", replaceIn(data, `name = "acme-tracker"`, `name = "tracker"`),
			[]string{`data/marketplace.toml: roles[0].name "tracker" does not start with acme-, as an extension's roles do (ADR 21 item 2)`},
		},
		{
			"role defined twice", appendTo(data, "\n[[roles]]\nname = \"acme-tracker\"\nmeans = \"Again\"\n"),
			[]string{`data/marketplace.toml: roles[2].name "acme-tracker" is already defined`},
		},
		{
			"role without means", replaceIn(data, `means = "The tracker where Acme files and moves issues"`, ""),
			[]string{"data/marketplace.toml: roles[0].means is missing"},
		},
		{
			"licence a folder", func(t *testing.T, dir string) {
				remove("LICENSE")(t, dir)
				if err := os.Mkdir(filepath.Join(dir, "LICENSE"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			[]string{"LICENSE: not a file"},
		},
		{
			"term list outside the repository", replaceIn(data, `"checks/terms/rules.toml"`, `"../rules.toml"`),
			[]string{`data/marketplace.toml: terms.rules: "../rules.toml" is not a path from the repository root to a file inside it`},
		},
		{
			"term list missing", remove("checks/terms/names.txt"),
			[]string{`data/marketplace.toml: terms.names: "checks/terms/names.txt" is not found`},
		},
		{
			"term list a folder", replaceIn(data, `"checks/terms/names.txt"`, `"checks/terms"`),
			[]string{`data/marketplace.toml: terms.names: "checks/terms" is not a file`},
		},
		{
			"dependencies not an array", replaceIn(gitManifest, `"dependencies":[{"name":"cech-git"}]`, `"dependencies":{"name":"cech-git"}`),
			[]string{gitManifest + `: "dependencies" is not an array`},
		},
		{
			"no owner name", replaceIn(data, `name = "Acme skill maintainers"`, ""),
			[]string{"data/marketplace.toml: owner.name is missing"},
		},
		{
			"category folder not a name", move("plugins/tool", "plugins/Tools"),
			[]string{`plugins/Tools: "Tools" is not a valid category name: use a-z, 0-9 and single hyphens, start with a letter, end with a letter or digit, at most 64 characters (ADR 12 item 3)`},
		},
		{
			"category folder over 64 characters", move("plugins/tool", "plugins/"+strings.Repeat("t", 65)),
			[]string{"plugins/" + strings.Repeat("t", 65) + `: "` + strings.Repeat("t", 65) + `" is not a valid category name`},
		},
		{
			"plugin folder not a name", move("plugins/tool/acme-git", "plugins/tool/acme--git"),
			[]string{`plugins/tool/acme--git: "acme--git" is not a valid plugin name: use a-z, 0-9 and single hyphens, start with a letter, end with a letter or digit, at most 64 characters (ADR 12 item 3)`},
		},
		{
			"two plugins with one name", copyDir("plugins/tool/acme-git", "plugins/workflow/acme-git"),
			[]string{`plugins/workflow/acme-git: plugins/tool/acme-git already has the name "acme-git", and plugin names are unique (ADR 12)`},
		},
		{"no plugin.json", remove(manifest), []string{manifest + ": not found"}},
		{
			"plugin.json without a name", replaceIn(manifest, `"name": "acme-prose",`, ""),
			[]string{manifest + ": name is missing"},
		},
		{
			"plugin.json not JSON", replaceIn(manifest, `"license": "MIT"`, `"license": MIT`),
			[]string{manifest + ": not valid JSON"},
		},
		{
			"duplicate key in plugin.json", replaceIn(manifest, `"license": "MIT"`, `"license": "MIT", "license": "ISC"`),
			[]string{manifest + `: duplicate key "license"`},
		},
		{
			"plugin.json name not the folder's", replaceIn(manifest, `"name": "acme-prose"`, `"name": "acme-words"`),
			[]string{manifest + `: name is "acme-words", not the folder's name "acme-prose"`},
		},
		{
			"author not an object", replaceIn(release, `"version": "0.1.0"`, `"version": "0.1.0", "author": "Acme"`),
			[]string{release + ": author: not a JSON object"},
		},
		{
			"author url not a string", replaceIn(manifest, `"url": "https://github.com/alice-dev"`, `"url": null`),
			[]string{manifest + `: author: "url" is not a string`},
		},
		{"no SKILL.md", remove(skill), []string{skill + ": not found"}},
		{"SKILL.md without frontmatter", replaceIn(skill, "---\nname", "name"), []string{skill + ": no frontmatter"}},
		{
			"SKILL.md name not the plugin's", replaceIn(skill, "name: acme-prose", "name: acme-words"),
			[]string{skill + `: name is "acme-words", not the plugin's name "acme-prose"`},
		},
		{
			"SKILL.md name not a string", replaceIn(skill, "name: acme-prose", "name: [acme-prose]"),
			[]string{skill + ": name is not a string"},
		},
		{
			"SKILL.md without a description", replaceIn(skill, "description:", "summary:"),
			[]string{skill + ": description is missing"},
		},
		{
			"description not a string", replaceIn(gitSkill,
				`description: "Runs git the acme way: small commits & clear messages."`, "description: 42"),
			[]string{gitSkill + ": description is not a string"},
		},
		{
			"duplicate key in SKILL.md", replaceIn(skill, "description:", "name: acme-prose\ndescription:"),
			[]string{skill + `: frontmatter: duplicate key "name"`},
		},
		{
			"maintainer not a user name", replaceIn(manifest, `"bob-dev"`, `"bob_dev"`),
			[]string{manifest + `: metadata: maintainers: "bob_dev" is not a GitHub user name`},
		},
		{
			"maintainer over 39 characters", replaceIn(manifest, `"bob-dev"`, `"`+strings.Repeat("b", 40)+`"`),
			[]string{manifest + `: metadata: maintainers: "` + strings.Repeat("b", 40) + `" is not a GitHub user name`},
		},
		{
			"metadata not an object", replaceIn(manifest, `"metadata": {`, `"metadata": null, "x": {`),
			[]string{manifest + ": metadata: not an object"},
		},
		{
			"maintainers not an array", replaceIn(manifest, `["bob-dev", "carol", "Alice-Dev"]`, `"bob-dev carol"`),
			[]string{manifest + ": metadata: maintainers is not an array of strings"},
		},
		{
			"maintainer not a string", replaceIn(manifest, `"carol"`, `42`),
			[]string{manifest + ": metadata: maintainers is not an array of strings"},
		},
		{
			"maintainers null", replaceIn(manifest, `["bob-dev", "carol", "Alice-Dev"]`, `null`),
			[]string{manifest + ": metadata: maintainers is not an array of strings"},
		},
		{"agent without frontmatter", replaceIn(agent, "---\nname", "name"), []string{agent + ": no frontmatter"}},
		{"agent without a name", replaceIn(agent, "name:", "title:"), []string{agent + ": name is missing"}},
		{
			"agent name not a string", replaceIn(agent, "name: acme-prose-reviewer", "name: [acme-prose-reviewer]"),
			[]string{agent + ": name is not a string"},
		},
		{"agent without a description", replaceIn(agent, "description:", "summary:"), []string{agent + ": description is missing"}},
		{"two problems, both reported", func(t *testing.T, dir string) {
			remove("data/marketplace.toml")(t, dir)
			remove(skill)(t, dir)
		}, []string{"data/marketplace.toml: not found", skill + ": not found"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			copyTree(t, filepath.Join("testdata", "build", "acme", "in"), dir, nil)
			tt.change(t, dir)
			before := readTree(t, dir)
			code, stdout, stderr := runIn(t, "build", "-C", dir)
			if code != ExitFailure {
				t.Errorf("exit code %d, want %d", code, ExitFailure)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want nothing", stdout)
			}
			if lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n"); !errorsMatch(lines, tt.want, "cech: ") {
				t.Errorf("stderr:\n%s\nwant a line for each of %q", stderr, tt.want)
			}
			if after := readTree(t, dir); !maps.Equal(before, after) {
				t.Error("a failed build changed the tree")
			}
			code, stdout, _ = runIn(t, "build", "--json", "-C", dir)
			var res buildResult
			if err := json.Unmarshal([]byte(stdout), &res); err != nil {
				t.Fatalf("--json: %v in %q", err, stdout)
			}
			if code != ExitFailure || res.Mode != "build" || res.OK || len(res.Files) != 0 || !errorsMatch(res.Errors, tt.want, "") {
				t.Errorf("--json: exit code %d, result %+v", code, res)
			}
		})
	}
}

// cechOwn makes the acme data file CECH's own: no cech key, namespace
// cech.
func cechOwn(t *testing.T, dir string) {
	t.Helper()
	replaceIn("data/marketplace.toml", `cech = "0.1.0"`+"\n", "")(t, dir)
	replaceIn("data/marketplace.toml", `namespace = "acme"`, `namespace = "cech"`)(t, dir)
}

// errorsMatch reports whether got has one message for each of want, in
// order, each starting with prefix and then the message in want.
func errorsMatch(got, want []string, prefix string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if !strings.HasPrefix(got[i], prefix+want[i]) {
			return false
		}
	}
	return true
}

// TestBuildUnreadable checks that a source the build is denied makes it
// exit 3, since it could not decide, unless it also met a failure, which
// wins (ADR 15 item 5). A tree in the wrong shape is a failure.
func TestBuildUnreadable(t *testing.T) {
	skipWithoutModes(t)
	skill := "plugins/writing/acme-prose/skills/acme-prose/SKILL.md"
	for _, tt := range []struct {
		name   string
		change func(t *testing.T, dir string)
		code   int
		file   string // the file the error names
	}{
		{"one source unreadable", chmod(skill, 0), ExitUnreachable, skill},
		{"unreadable and invalid", func(t *testing.T, dir string) {
			chmod(skill, 0)(t, dir)
			remove("data/marketplace.toml")(t, dir)
		}, ExitFailure, skill},
		{"generated file unreadable", chmod(".claude-plugin/marketplace.json", 0), ExitUnreachable, ".claude-plugin/marketplace.json"},
		{"folder at a generated path", func(t *testing.T, dir string) {
			if err := os.MkdirAll(filepath.Join(dir, "docs", "catalogue.md"), 0o750); err != nil {
				t.Fatal(err)
			}
		}, ExitFailure, "docs/catalogue.md"},
		{"a link out of the root", func(t *testing.T, dir string) {
			outside := filepath.Join(t.TempDir(), "SKILL.md")
			if err := os.WriteFile(outside, []byte("---\nname: acme-prose\ndescription: x\n---\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			remove(skill)(t, dir)
			if err := os.Symlink(outside, filepath.Join(dir, filepath.FromSlash(skill))); err != nil {
				t.Fatal(err)
			}
		}, ExitFailure, skill},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			copyTree(t, filepath.Join("testdata", "build", "acme", "in"), dir, nil)
			tt.change(t, dir)
			code, stdout, stderr := runIn(t, "build", "--check", "-C", dir)
			if code != tt.code || stdout != "" || !strings.Contains(stderr, "cech: "+tt.file) {
				t.Errorf("exit code %d, stdout %q, stderr %q; want %d and the file named", code, stdout, stderr, tt.code)
			}
		})
	}
}

// TestBuildKeepsMode checks that a file the build rewrites keeps its
// mode, even bits that a usual umask such as 022 or 002 removes.
func TestBuildKeepsMode(t *testing.T) {
	skipWithoutModes(t)
	manifest := "plugins/writing/acme-prose/.claude-plugin/plugin.json"
	dir := t.TempDir()
	copyTree(t, filepath.Join("testdata", "build", "acme", "in"), dir, nil)
	name := filepath.Join(dir, filepath.FromSlash(manifest))
	if err := os.Chmod(name, 0o666); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runIn(t, "build", "-C", dir); code != ExitOK {
		t.Fatalf("build: exit code %d; stderr:\n%s", code, stderr)
	}
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o666 {
		t.Errorf("mode %v after the build, want %v", info.Mode().Perm(), os.FileMode(0o666))
	}
}

// TestBuildPartial makes a write fail partway: the build reports the
// changes it made on stdout and those it skipped on stderr and in --json,
// and fails (ADR 15 item 5).
func TestBuildPartial(t *testing.T) {
	skipWithoutModes(t)
	made := []build.Diff{
		{Path: ".claude-plugin/marketplace.json", Change: build.Update},
		{Path: ".github/CODEOWNERS", Change: build.Create},
	}
	skipped := []build.Diff{
		{Path: "docs/catalogue.md", Change: build.Create},
		{Path: "plugins/tool/acme-docker/.claude-plugin/plugin.json", Change: build.Update},
		{Path: "plugins/tool/acme-docker/skills/acme-docker/LICENSE", Change: build.Create},
		{Path: "plugins/tool/acme-git/.claude-plugin/plugin.json", Change: build.Update},
		{Path: "plugins/tool/acme-git/agents/acme-git-reviewer.toml", Change: build.Update},
		{Path: "plugins/tool/acme-git/agents/old-reviewer.toml", Change: build.Remove},
		{Path: "plugins/tool/acme-git/skills/acme-git/LICENSE", Change: build.Create},
		{Path: "plugins/workflow/acme-release/.claude-plugin/plugin.json", Change: build.Update},
		{Path: "plugins/workflow/acme-release/skills/acme-release/LICENSE", Change: build.Create},
		{Path: "plugins/writing/acme-prose/.claude-plugin/plugin.json", Change: build.Update},
		{Path: "plugins/writing/acme-prose/agents/acme-prose-reviewer.toml", Change: build.Create},
		{Path: "plugins/writing/acme-prose/skills/acme-prose/LICENSE", Change: build.Create},
	}
	// setup gives the build a docs/ it cannot write to: the writes go in
	// path order, so the two before docs/catalogue.md are made.
	setup := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		copyTree(t, filepath.Join("testdata", "build", "acme", "in"), dir, nil)
		if err := os.Mkdir(filepath.Join(dir, "docs"), 0o500); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	t.Run("text", func(t *testing.T) {
		code, stdout, stderr := runIn(t, "build", "-C", setup(t))
		if code != ExitFailure {
			t.Errorf("exit code %d, want %d", code, ExitFailure)
		}
		var want strings.Builder
		for _, d := range made {
			want.WriteString(string(d.Change) + "d " + d.Path + "\n")
		}
		if stdout != want.String() {
			t.Errorf("stdout = %q, want %q", stdout, want.String())
		}
		if !strings.Contains(stderr, "cech: docs/catalogue.md: ") {
			t.Errorf("stderr = %q, want the error to name docs/catalogue.md", stderr)
		}
		for _, d := range skipped {
			if line := "cech: skipped: " + string(d.Change) + " " + d.Path + "\n"; !strings.Contains(stderr, line) {
				t.Errorf("stderr = %q, want it to hold %q", stderr, line)
			}
		}
	})
	t.Run("json", func(t *testing.T) {
		code, stdout, _ := runIn(t, "build", "--json", "-C", setup(t))
		var res buildResult
		if err := json.Unmarshal([]byte(stdout), &res); err != nil {
			t.Fatalf("%v in %q", err, stdout)
		}
		if code != ExitFailure || res.Mode != "build" || res.OK || len(res.Errors) != 1 ||
			!strings.HasPrefix(res.Errors[0], "docs/catalogue.md: ") ||
			!slices.Equal(res.Files, made) || !slices.Equal(res.Skipped, skipped) {
			t.Errorf("exit code %d, result %+v", code, res)
		}
	})
}

// TestBuildMissingDir checks that a marketplace root it cannot open
// exits 3: it could not decide.
func TestBuildMissingDir(t *testing.T) {
	code, stdout, stderr := runIn(t, "build", "--check", "--json", "-C", filepath.Join(t.TempDir(), "none"))
	var res buildResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("%v in %q", err, stdout)
	}
	if code != ExitUnreachable || res.OK || res.Mode != "check" || len(res.Errors) != 1 || stderr != "" {
		t.Errorf("exit code %d, result %+v, stderr %q", code, res, stderr)
	}
}

// skipWithoutModes skips a test that needs mode bits to deny access.
func skipWithoutModes(t *testing.T) {
	t.Helper()
	switch {
	case runtime.GOOS == "windows":
		t.Skip("Windows does not deny access by mode bits")
	case os.Geteuid() == 0:
		t.Skip("root is not denied access by mode bits")
	}
}

// runIn runs cech with args and returns its exit code and output.
func runIn(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut, info)
	return code, out.String(), errOut.String()
}

// copyTree copies the files under src to dst, turning LF into CRLF in
// those whose slash path crlf reports true for.
func copyTree(t *testing.T, src, dst string, crlf func(string) bool) {
	t.Helper()
	for p, data := range readTree(t, src) {
		if crlf != nil && crlf(p) {
			data = strings.ReplaceAll(data, "\n", "\r\n")
		}
		name := filepath.Join(dst, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// readTree returns every file under dir, by slash path.
func readTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, name)
		files[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// checkTree compares the files under got with those under want, or with
// -update makes want a copy of got.
func checkTree(t *testing.T, want, got string) {
	t.Helper()
	if *update {
		if err := os.RemoveAll(want); err != nil {
			t.Fatal(err)
		}
		copyTree(t, got, want, nil)
		return
	}
	w, g := readTree(t, want), readTree(t, got)
	for _, p := range slices.Sorted(maps.Keys(w)) {
		if g[p] != w[p] {
			t.Errorf("%s differs\ngot:\n%q\nwant:\n%q", p, g[p], w[p])
		}
	}
	for p := range g {
		if _, ok := w[p]; !ok {
			t.Errorf("%s is not in %s", p, want)
		}
	}
}

// appendTo returns a change that appends text to the file at p.
func appendTo(p, text string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		name := filepath.Join(dir, filepath.FromSlash(p))
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, append(data, text...), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// replaceIn returns a change that replaces old with new in the file at p,
// which must hold old.
func replaceIn(p, old, replacement string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		name := filepath.Join(dir, filepath.FromSlash(p))
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), old) {
			t.Fatalf("%s does not hold %q", p, old)
		}
		data = []byte(strings.Replace(string(data), old, replacement, 1))
		if err := os.WriteFile(name, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// toCRLF returns a change that gives the file at p CRLF line ends.
func toCRLF(p string) func(*testing.T, string) {
	return replaceAllIn(p, "\n", "\r\n")
}

func replaceAllIn(p, old, replacement string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		name := filepath.Join(dir, filepath.FromSlash(p))
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(strings.ReplaceAll(string(data), old, replacement)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// remove returns a change that removes the file at p.
func remove(p string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			t.Fatal(err)
		}
	}
}

// removeAll returns a change that removes the folder at p.
func removeAll(p string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		if err := os.RemoveAll(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			t.Fatal(err)
		}
	}
}

// copyDir returns a change that copies the folder at from to to.
func copyDir(from, to string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		copyTree(t, filepath.Join(dir, filepath.FromSlash(from)), filepath.Join(dir, filepath.FromSlash(to)), nil)
	}
}

// chmod returns a change that sets the mode of the file at p, and puts
// it back when the test ends so the temporary folder can be removed.
func chmod(p string, mode os.FileMode) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		name := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.Chmod(name, mode); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(name, 0o600) })
	}
}

// move returns a change that moves the folder at from to to.
func move(from, to string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(path.Dir(to))), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(filepath.Join(dir, filepath.FromSlash(from)), filepath.Join(dir, filepath.FromSlash(to))); err != nil {
			t.Fatal(err)
		}
	}
}
