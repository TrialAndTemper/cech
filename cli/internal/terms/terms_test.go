package terms

import (
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// Values the term check would flag in this file are built at run time,
// so CECH's own gate passes it.
var (
	corp   = "big" + "corp"
	outer  = "jane@" + corp + ".com"
	author = "michal.b.bozek" + "@" + "gmail.com"
	// token has the shape of a GitHub personal access token.
	token  = "ghp_" + "aB3dE5fG7hI9jK1mN3pQ5rS7tU9vW1xY3zA5"
	secret = "zq" + "-planted-term"
)

// gitIn runs git in dir, away from any repository a hook points at.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git(t.Context(), dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// newRepo makes a git repository holding files, all committed.
func newRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "config", "user.name", "Test")
	gitIn(t, dir, "config", "user.email", "test@example.com")
	gitIn(t, dir, "config", "commit.gpgsign", "false")
	write(t, dir, files)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// writeList writes a private list holding secret on line 2.
func writeList(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(p, []byte("# list\n"+secret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// render is everything a result could print; it must never hold the
// private term.
func render(res *Result) string {
	var b strings.Builder
	for _, f := range res.Findings {
		b.WriteString(f.String() + "\n")
	}
	for _, p := range res.Problems {
		b.WriteString(p.String() + "\n")
	}
	for _, n := range append(slices.Clone(res.Skipped), res.Undecided...) {
		b.WriteString(n.String() + "\n")
	}
	return b.String()
}

func check(t *testing.T, dir string, ext Extension) *Result {
	t.Helper()
	if ext.Namespace == "" {
		ext.Namespace = "acme"
	}
	res, err := CheckExtension(t.Context(), dir, ext)
	if err != nil {
		t.Fatal(err)
	}
	if out := render(res); strings.Contains(strings.ToLower(out), secret) {
		t.Fatalf("output shows the private term:\n%s", out)
	}
	return res
}

// has reports whether res printed a line holding want.
func has(res *Result, want string) bool {
	return strings.Contains(render(res), want)
}

const acmeRules = `title = "acme"

[[allowlists]]
regexTarget = "match"
regexes = ['''^skills@acme\.example$''']

[[rules]]
id = "acme-ticket"
description = "Acme ticket"
regex = '''\bACME-T\d{4}\b'''
`

// TestGenericHoldsNoCECHAddress guards item 1: an address CECH
// allowlists for itself in checks/terms/rules.toml must not be allowed
// in every extension too.
func TestGenericHoldsNoCECHAddress(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "checks", "terms", "rules.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var own ruleFile
	if _, err := toml.Decode(string(data), &own); err != nil {
		t.Fatal(err)
	}
	addr := regexp.MustCompile(`[a-z0-9._%+-]+\\?@[a-z0-9.\\-]+`)
	var n int
	lists := slices.Clone(own.Allowlists)
	for _, r := range own.Rules {
		lists = append(lists, r.Allowlists...)
	}
	for _, a := range lists {
		// CECH's own addresses carry this description, as a global
		// allowlist or, before the gate moved, one of the email rule's.
		if !strings.HasPrefix(a.Description, "Authors, sign-offs") {
			continue
		}
		for _, re := range a.Regexes {
			for _, m := range addr.FindAllString(re, -1) {
				n++
				if strings.Contains(string(Generic), m) {
					t.Errorf("generic.toml allows CECH's own address %s", m)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("found no address in checks/terms/rules.toml to compare")
	}
}

// TestProject guards item 26: each file a project's scan reads is
// scanned even when git does not track it, and no other file is.
func TestProject(t *testing.T) {
	for _, name := range []string{".cech/config.toml", ".cech/acme/plugin.toml", ".claude/settings.json", ".codex/config.toml"} {
		t.Run(name, func(t *testing.T) {
			dir := newRepo(t, map[string]string{"src/notes.txt": "key " + token + "\n"})
			write(t, dir, map[string]string{name: "key = \"" + token + "\"\n"})
			res, err := CheckProject(t.Context(), dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Findings) != 1 || res.Findings[0].Where != name || res.Findings[0].Tier != TierGeneric {
				t.Fatalf("want one generic finding in %s:\n%s", name, render(res))
			}
		})
	}
}

// TestProjectStaysInTree guards a symlinked .cech/ that would pull in a
// file from outside the repository.
func TestProjectStaysInTree(t *testing.T) {
	outside := t.TempDir()
	write(t, outside, map[string]string{"config.toml": "key = \"" + token + "\"\n"})
	dir := newRepo(t, map[string]string{"a.md": "ok\n"})
	if err := os.Symlink(outside, filepath.Join(dir, ".cech")); err != nil {
		symlinkUnavailable(t, err)
	}
	res, err := CheckProject(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("scanned a file outside the repository:\n%s", render(res))
	}
}

// TestExtensionStaysInTree guards a tracked path whose directory is
// swapped for a symlink to outside the repository: the file is missing,
// not read.
func TestExtensionStaysInTree(t *testing.T) {
	outside := t.TempDir()
	write(t, outside, map[string]string{"x.md": "Mail " + outer + "\n"})
	dir := newRepo(t, map[string]string{"a/x.md": "ok\n"})
	if err := os.RemoveAll(filepath.Join(dir, "a")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "a")); err != nil {
		symlinkUnavailable(t, err)
	}
	res := check(t, dir, Extension{})
	// os.Root refuses a path that leaves it, so the file is unreadable.
	if has(res, "email") || !has(res, string(ProblemUnreadable)+": a/x.md") {
		t.Fatalf("want a/x.md reported unreadable, not scanned:\n%s", render(res))
	}
}

// TestSymlinkEntry guards item 11: a terms entry that is a tracked
// symlink is refused, so rules cannot be loaded from where it points.
func TestSymlinkEntry(t *testing.T) {
	dir := newRepo(t, map[string]string{"x.toml": "[[rules]]\nid = \"acme-x\"\nregex = 'x'\n"})
	if err := os.Symlink("x.toml", filepath.Join(dir, "r.toml")); err != nil {
		symlinkUnavailable(t, err)
	}
	gitIn(t, dir, "add", "r.toml")
	gitIn(t, dir, "commit", "-q", "-m", "link")
	res := check(t, dir, Extension{Rules: []string{"r.toml"}})
	if !has(res, "is not a regular file") {
		t.Fatalf("want the symlink refused:\n%s", render(res))
	}
}

// TestProjectNativeSymlink guards a symlinked .claude/ pointing outside
// the repository: its settings are not read.
func TestProjectNativeSymlink(t *testing.T) {
	outside := t.TempDir()
	write(t, outside, map[string]string{"settings.json": "key = \"" + token + "\"\n"})
	dir := newRepo(t, map[string]string{"a.md": "ok\n"})
	if err := os.Symlink(outside, filepath.Join(dir, ".claude")); err != nil {
		symlinkUnavailable(t, err)
	}
	res, err := CheckProject(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("read a file outside the repository:\n%s", render(res))
	}
}

// symlinkUnavailable skips where symlinks need a privilege, as on some
// Windows machines, and fails anywhere else.
func symlinkUnavailable(t *testing.T, err error) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skipf("cannot make a symlink here: %v", err)
	}
	t.Fatal(err)
}

func TestExtensionTiers(t *testing.T) {
	base := map[string]string{
		"checks/acme-rules.toml": acmeRules,
		"checks/acme-names.txt":  "# names\r\nQuuxly Cloud \r\n",
		"README.md": "Mail 12345+jane@users.noreply.github.com or bob@shop.test " +
			"or skills@acme.example.\n",
	}
	ext := Extension{Rules: []string{"checks/acme-rules.toml"}, Names: []string{"checks/acme-names.txt"}}
	tests := []struct {
		name  string
		add   string // a line for planted.md, or ""
		names string // the name file, if not the usual one
		want  []string
		clean bool
	}{
		{name: "allowed addresses pass", clean: true},
		{name: "CECH's author address fails", add: "Mail " + author + ".", want: []string{"planted.md:1: email"}},
		{name: "another address fails", add: "Mail " + outer + ".", want: []string{"planted.md:1: email"}},
		{name: "a listed name fails", add: "Runs on quuxly-cloud.", want: []string{"planted.md:1: name-1-2"}},
		{name: "an extension rule fails", add: "See ACME-T1234.", want: []string{"planted.md:1: acme-ticket"}},
		// Guard Betterleaks' own allow comments: only CECH's exceptions
		// count. Either comment, honoured, would hide the hit.
		{name: "the engine's allow comments hide nothing", add: "Mail " + outer + " betterleaks" + ":allow gitleaks" + ":allow", want: []string{"planted.md:1: email"}},
		{name: "the inline marker hides a public hit", add: "Mail " + outer + " cech-terms: allow example", clean: true},
		{name: "a private term fails by number", add: "see " + secret, want: []string{"planted.md:1: private term 2"}},
		// Item 17: the public tiers skip the files under terms, but the
		// private tier scans them.
		{name: "the private tier scans the name file", names: "# " + secret + "\n", want: []string{"checks/acme-names.txt:1: private term 2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := maps.Clone(base)
			if tt.add != "" {
				files["planted.md"] = tt.add + "\n"
			}
			if tt.names != "" {
				files["checks/acme-names.txt"] = tt.names
			}
			dir := newRepo(t, files)
			e := ext
			e.Private = writeList(t)
			res := check(t, dir, e)
			if tt.clean && res.Failed() {
				t.Fatalf("want a pass, got:\n%s", render(res))
			}
			for _, w := range tt.want {
				if !has(res, w) {
					t.Errorf("want %q in:\n%s", w, render(res))
				}
			}
		})
	}
}

// TestExtensionAllowlistNeverHidesPrivate guards item 21: an extension
// allowlist that matches a private term must not reach the private tier.
func TestExtensionAllowlistNeverHidesPrivate(t *testing.T) {
	dir := newRepo(t, map[string]string{
		"checks/acme-rules.toml": "[[allowlists]]\nregexes = ['''" + "zq-planted" + "''']\npaths = ['''.*''']\n",
		"planted.md":             "see " + secret + "\n",
	})
	res := check(t, dir, Extension{Rules: []string{"checks/acme-rules.toml"}, Private: writeList(t)})
	if !has(res, "private term 2") {
		t.Fatalf("private hit hidden:\n%s", render(res))
	}
}

func TestInvalidLists(t *testing.T) {
	tests := []struct {
		name  string
		ns    string
		rules string
		entry []string
		want  string
	}{
		{name: "a key outside item 12", rules: "[[rules]]\nid = \"acme-x\"\nregex = 'x'\nvalidate = 'true'\n", want: "key rules.validate is not allowed"},
		{name: "extend", rules: "[extend]\nuseDefault = true\n", want: "key extend is not allowed"},
		{name: "an id without the namespace", rules: "[[rules]]\nid = \"x\"\nregex = 'x'\n", want: "does not start with acme-"},
		{name: "an id used twice", rules: "[[rules]]\nid = \"acme-x\"\nregex = 'x'\n[[rules]]\nid = \"acme-x\"\nregex = 'y'\n", want: "already used"},
		{name: "a default rule's id", ns: "github", rules: "[[rules]]\nid = \"github-pat\"\nregex = 'x'\n", want: "github-pat is already used"},
		{name: "a reserved prefix", ns: "name", rules: "[[rules]]\nid = \"name-1-1\"\nregex = 'x'\n", want: "prefix CECH reserves"},
		{name: "an empty allowlist", rules: "[[allowlists]]\ndescription = \"none\"\n", want: "no regexes, paths or stopwords"},
		{name: "a secret group past the regex", rules: "[[rules]]\nid = \"acme-x\"\nregex = 'a(b)'\nsecretGroup = 2\n", want: "secretGroup 2"},
		{name: "a regex that does not compile", rules: "[[rules]]\nid = \"acme-x\"\nregex = 'a('\n", want: "does not compile"},
		{name: "a rule with no regex or path", rules: "[[rules]]\nid = \"acme-x\"\n", want: "neither regex nor path"},
		{name: "an entry outside the repository", entry: []string{"../x.toml"}, want: "is not a clean path"},
		{name: "an untracked entry", entry: []string{"nope.toml"}, want: "is not a tracked file"},
		{name: "an entry twice", entry: []string{"r.toml", "r.toml"}, want: "appears twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string]string{"r.toml": tt.rules, "planted.md": "Mail " + outer + "\n"}
			dir := newRepo(t, files)
			entries := tt.entry
			if entries == nil {
				entries = []string{"r.toml"}
			}
			res := check(t, dir, Extension{Namespace: tt.ns, Rules: entries})
			if !has(res, tt.want) {
				t.Fatalf("want %q in:\n%s", tt.want, render(res))
			}
			// An invalid list stops the public tiers, and says so (item 32).
			if !has(res, "generic and extension tiers") || slices.ContainsFunc(res.Findings, func(f Finding) bool { return f.Tier != TierPrivate }) {
				t.Errorf("public tiers ran with an invalid list:\n%s", render(res))
			}
		})
	}
}

// TestNameFileProblem guards a name of separators alone, which would
// match the empty string everywhere.
func TestNameFileProblem(t *testing.T) {
	dir := newRepo(t, map[string]string{"n.txt": "Quuxly\n--\n"})
	res := check(t, dir, Extension{Names: []string{"n.txt"}})
	if !has(res, "n.txt: line 2 holds no word") {
		t.Fatalf("want the line reported:\n%s", render(res))
	}
}

// TestGlobalPathAllowlist guards item 18: a global path allowlist hides
// public hits in those paths, through the engine's skip function.
func TestGlobalPathAllowlist(t *testing.T) {
	dir := newRepo(t, map[string]string{
		"r.toml":        "[[allowlists]]\npaths = ['^vendored/']\n",
		"vendored/a.md": "Mail " + outer + "\n",
		"own/a.md":      "Mail " + outer + "\n",
	})
	res := check(t, dir, Extension{Rules: []string{"r.toml"}})
	if has(res, "vendored/a.md") || !has(res, "own/a.md:1: email") {
		t.Fatalf("want only own/a.md:\n%s", render(res))
	}
}

// TestMergeDedup guards item 28: a hit in .cech/ that both scans find is
// reported once.
func TestMergeDedup(t *testing.T) {
	dir := newRepo(t, map[string]string{".cech/config.toml": "key = \"" + token + "\"\n"})
	proj, err := CheckProject(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	ext := check(t, dir, Extension{})
	if !slices.ContainsFunc(ext.Findings, func(f Finding) bool { return f.Where == ".cech/config.toml" && f.Rule == "github-pat" }) {
		t.Fatalf("want the extension scan to find the hit too:\n%s", render(ext))
	}
	proj.Merge(ext)
	n := 0
	for _, f := range proj.Findings {
		if f.Where == ".cech/config.toml" && f.Rule == "github-pat" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want the hit once, got %d:\n%s", n, render(proj))
	}
}

// TestMergeHides guards item 28 with items 24 and 31: an extension
// result that hides locations, merged into a project's result, hides
// the project's locations too.
func TestMergeHides(t *testing.T) {
	dir := newRepo(t, map[string]string{
		".cech/config.toml": "source = \"https://x:" + token + "@example.com/r.git\"\n",
		"b.md":              "see " + secret + "\n",
	})
	proj, err := CheckProject(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(proj.Findings) == 0 || proj.Findings[0].Where == "" {
		t.Fatalf("want a located project finding:\n%s", render(proj))
	}
	// Merge into the project's result, which hides nothing yet.
	proj.Merge(check(t, dir, Extension{Private: writeList(t)}))
	if !slices.ContainsFunc(proj.Findings, func(f Finding) bool { return f.Rule == "github-pat" }) {
		t.Fatalf("want the project's hit kept:\n%s", render(proj))
	}
	for _, f := range proj.Findings {
		if f.Tier != TierPrivate && (f.Where != "" || f.Match != "") {
			t.Errorf("merged public hit shows more than its rule: %v", f)
		}
	}
}

// TestHidden guards Result.Hidden, which the command uses to say why
// public hits show no location.
func TestHidden(t *testing.T) {
	dir := newRepo(t, map[string]string{"b.md": "see " + secret + "\n"})
	if check(t, dir, Extension{}).Hidden() {
		t.Error("hidden with the private tier skipped")
	}
	if !check(t, dir, Extension{Private: writeList(t)}).Hidden() {
		t.Error("not hidden with a private hit")
	}
}

// TestProblemDetailsHidden guards item 31: a problem naming an entry
// must not show a private term while a private hit exists. check()
// fails the test if any output holds the term.
func TestProblemDetailsHidden(t *testing.T) {
	dir := newRepo(t, map[string]string{"b.md": "see " + secret + "\n"})
	res := check(t, dir, Extension{Rules: []string{secret + ".toml"}, Private: writeList(t)})
	if !has(res, string(ProblemEntry)) {
		t.Fatalf("want the entry problem:\n%s", render(res))
	}
}

func TestPrivateList(t *testing.T) {
	dir := newRepo(t, map[string]string{"planted.md": "Mail " + outer + "\n"})
	missing := filepath.Join(t.TempDir(), "none.txt")
	empty := filepath.Join(t.TempDir(), "empty.txt")
	broken := filepath.Join(t.TempDir(), "broken.txt")
	write(t, filepath.Dir(empty), map[string]string{"empty.txt": "# nothing\n"})
	write(t, filepath.Dir(broken), map[string]string{"broken.txt": secret + "\n" + secret + "(\n"})
	tests := []struct {
		name      string
		private   string
		require   bool
		skipped   bool
		undecided bool
		located   bool
	}{
		{name: "no list is a skip", skipped: true, located: true},
		{name: "a missing list is a skip", private: missing, skipped: true, located: true},
		{name: "a list with no terms is a skip", private: empty, skipped: true, located: true},
		{name: "required and missing could not run", private: missing, require: true, undecided: true},
		{name: "required and empty could not run", private: empty, require: true, undecided: true},
		{name: "a line that does not compile could not run", private: broken, undecided: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := check(t, dir, Extension{Private: tt.private, RequirePrivate: tt.require})
			if got := slices.ContainsFunc(res.Skipped, func(n Note) bool { return n.Kind == NotePrivateSkipped }); got != tt.skipped {
				t.Errorf("skipped = %v, want %v:\n%s", got, tt.skipped, render(res))
			}
			if got := len(res.Undecided) > 0; got != tt.undecided {
				t.Errorf("undecided = %v, want %v:\n%s", got, tt.undecided, render(res))
			}
			// The public hit still counts, but shows its location only
			// when the private tier did not fail to run (item 24).
			if len(res.Findings) != 1 || res.Findings[0].Rule != "email" {
				t.Fatalf("want one email finding:\n%s", render(res))
			}
			if got := res.Findings[0].Where != ""; got != tt.located {
				t.Errorf("location shown = %v, want %v:\n%s", got, tt.located, render(res))
			}
		})
	}
	t.Run("a broken line is named by number only", func(t *testing.T) {
		res := check(t, dir, Extension{Private: broken})
		if !has(res, "line 2 of the list") {
			t.Errorf("want the line number:\n%s", render(res))
		}
	})
}

func TestPrivateHiding(t *testing.T) {
	t.Run("a term in a path hides later locations", func(t *testing.T) {
		dir := newRepo(t, map[string]string{
			"notes-" + secret + ".md": "Mail " + outer + "\n",
			"other.md":                "see " + secret + "\n",
		})
		// A tracked file missing from the work tree must not be named
		// either.
		if err := os.Remove(filepath.Join(dir, "notes-"+secret+".md")); err != nil {
			t.Fatal(err)
		}
		res := check(t, dir, Extension{Private: writeList(t)})
		if !has(res, "paths:") {
			t.Errorf("want the hit in the paths:\n%s", render(res))
		}
		for _, f := range res.Findings {
			if f.Where != "" && f.Where != WherePaths {
				t.Errorf("finding shows a location: %v", f)
			}
		}
		if !has(res, string(ProblemMissing)) {
			t.Errorf("want the missing file reported:\n%s", render(res))
		}
	})
	t.Run("a private hit leaves public hits only their rule", func(t *testing.T) {
		dir := newRepo(t, map[string]string{"a.md": "Mail " + outer + "\n", "b.md": "see " + secret + "\n"})
		res := check(t, dir, Extension{Private: writeList(t)})
		for _, f := range res.Findings {
			if f.Tier != TierPrivate && (f.Where != "" || f.Match != "") {
				t.Errorf("public hit shows more than its rule: %v", f)
			}
		}
	})
}

func TestRange(t *testing.T) {
	dir := newRepo(t, map[string]string{"a.md": "ok\n"})
	write(t, dir, map[string]string{"gone.md": "line\nnote " + secret + "\n"})
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "add")
	gitIn(t, dir, "rm", "-q", "gone.md")
	gitIn(t, dir, "commit", "-q", "-m", "drop", "-m", "Asked by "+outer+".")
	gitIn(t, dir, "branch", "topic-"+secret)

	t.Run("without a range the tree is clean", func(t *testing.T) {
		res := check(t, dir, Extension{Private: writeList(t)})
		if res.Failed() {
			t.Fatalf("want a pass:\n%s", render(res))
		}
	})
	t.Run("a range finds removed text, messages and ref names", func(t *testing.T) {
		res := check(t, dir, Extension{Private: writeList(t), Range: "HEAD~2..HEAD"})
		var inDiff, inRef, inMsg bool
		for _, f := range res.Findings {
			inDiff = inDiff || f.Tier == TierPrivate && f.Commit != ""
			inRef = inRef || f.Where == WhereRefNames
			inMsg = inMsg || f.Rule == "email"
		}
		if !inDiff || !inRef || !inMsg {
			t.Errorf("diff %v, ref %v, message %v:\n%s", inDiff, inRef, inMsg, render(res))
		}
	})
	t.Run("an unresolved range could not run", func(t *testing.T) {
		res := check(t, dir, Extension{Range: "nope..HEAD"})
		if !has(res, string(NoteRangeUnresolved)+": nope..HEAD") {
			t.Errorf("want the range named:\n%s", render(res))
		}
	})
	t.Run("an unresolved range holding a term is not named", func(t *testing.T) {
		// A repository with no private hit, so only the range is screened.
		clean := newRepo(t, map[string]string{"a.md": "ok\n"})
		res := check(t, clean, Extension{Private: writeList(t), Range: secret + "..HEAD"})
		if len(res.Undecided) != 1 {
			t.Errorf("want the range reported:\n%s", render(res))
		}
	})
	// Guards item 24: with no list to screen it, the range is not named.
	t.Run("an unresolved range is not named while the private tier could not run", func(t *testing.T) {
		res := check(t, dir, Extension{RequirePrivate: true, Range: "nope..HEAD"})
		if has(res, "nope") || !has(res, string(NoteRangeUnresolved)) {
			t.Errorf("want the range reported but not named:\n%s", render(res))
		}
	})
	// Guards a diff line whose text starts with "++ " being read as a
	// file header and never scanned.
	t.Run("an added line that looks like a header is scanned", func(t *testing.T) {
		d := newRepo(t, map[string]string{"a.md": "ok\n"})
		write(t, d, map[string]string{"a.md": "ok\n++ " + secret + "\n"})
		gitIn(t, d, "commit", "-q", "-am", "edit")
		res := check(t, d, Extension{Private: writeList(t), Range: "HEAD~1..HEAD"})
		if !slices.ContainsFunc(res.Findings, func(f Finding) bool { return f.Tier == TierPrivate && f.Commit != "" }) {
			t.Errorf("want the added line's private hit:\n%s", render(res))
		}
	})
	t.Run("a range that looks like an option is a usage error", func(t *testing.T) {
		_, err := CheckExtension(t.Context(), dir, Extension{Range: "--all"})
		if ue := (*UsageError)(nil); !errors.As(err, &ue) {
			t.Errorf("want a UsageError, got %v", err)
		}
	})
}

func TestBinaryAndSymlink(t *testing.T) {
	dir := newRepo(t, map[string]string{"bin.dat": "\x00" + outer, "a.md": "ok\n"})
	if err := os.Symlink(outer, filepath.Join(dir, "link")); err != nil {
		symlinkUnavailable(t, err)
	}
	gitIn(t, dir, "add", "link")
	gitIn(t, dir, "commit", "-q", "-m", "link")
	res := check(t, dir, Extension{})
	if !has(res, string(NoteBinarySkipped)+": 1") {
		t.Errorf("want the binary file counted:\n%s", render(res))
	}
	if !has(res, "link:1: email") {
		t.Errorf("want the symlink's target scanned:\n%s", render(res))
	}
}

func TestNameRegex(t *testing.T) {
	tests := []struct{ name, text string }{
		{"Quuxly Cloud", "on Quuxly-Cloud now"},
		{"Quuxly Cloud", "on quuxly_cloud now"},
		{"Quuxly Cloud", "on QuuxlyCloud now"},
		{"Zorp++", "uses Zorp++."},
	}
	for _, tt := range tests {
		if !regexp.MustCompile(nameRegex(tt.name)).MatchString(tt.text) {
			t.Errorf("%q misses %q", tt.name, tt.text)
		}
	}
	if regexp.MustCompile(nameRegex("Quuxly")).MatchString("Quuxlyish") {
		t.Error("a name matches inside a longer word")
	}
}
