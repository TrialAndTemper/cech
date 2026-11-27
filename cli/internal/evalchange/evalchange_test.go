package evalchange

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

// isolate makes git in this test read no global or system config, so it
// neither signs nor asks for an identity, whatever the machine has.
func isolate(t *testing.T) {
	t.Helper()
	empty := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL":   empty,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "Test",
		"GIT_AUTHOR_EMAIL":    "test@example.com",
		"GIT_COMMITTER_NAME":  "Test",
		"GIT_COMMITTER_EMAIL": "test@example.com",
	} {
		t.Setenv(k, v)
	}
}

// run runs git in dir and returns its output without the final newline.
func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git(t.Context(), dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// repo is a marketplace repository under test, whose plugins have the
// version 0.0.0-dev on main.
type repo struct {
	t   *testing.T
	dir string
	// deps are each plugin's dependencies as JSON, by its folder.
	deps map[string]string
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	isolate(t)
	r := &repo{t: t, dir: t.TempDir(), deps: map[string]string{}}
	run(t, r.dir, "init", "--quiet", "--initial-branch=main")
	return r
}

func (r *repo) write(file, content string) {
	r.t.Helper()
	p := filepath.Join(r.dir, filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		r.t.Fatal(err)
	}
}

func manifest(name, version, deps string) string {
	return fmt.Sprintf("{\n  \"name\": %q,\n  \"version\": %q,\n  \"dependencies\": %s\n}\n", name, version, deps)
}

// add writes plugin dir, plugins/<category>/<name>, with deps as its
// dependencies and skill as its SKILL.md.
func (r *repo) add(dir, deps, skill string) {
	r.t.Helper()
	r.deps[dir] = deps
	name := filepath.Base(dir)
	r.write(dir+"/.claude-plugin/plugin.json", manifest(name, "0.0.0-dev", deps))
	r.write(dir+"/skills/"+name+"/SKILL.md", skill)
}

// commit commits every change and returns the commit.
func (r *repo) commit(msg string) string {
	r.t.Helper()
	run(r.t, r.dir, "add", "--all")
	run(r.t, r.dir, "commit", "--quiet", "--message", msg)
	return run(r.t, r.dir, "rev-parse", "HEAD")
}

// release makes a release commit on top of main's HEAD that writes
// version into every plugin.json, as a release build does, tags it, and
// returns to main. It returns the release commit.
func (r *repo) release(version string) string {
	r.t.Helper()
	run(r.t, r.dir, "switch", "--quiet", "--detach")
	for dir, deps := range r.deps {
		r.write(dir+"/.claude-plugin/plugin.json", manifest(filepath.Base(dir), version, deps))
	}
	c := r.commit("chore: release " + version)
	run(r.t, r.dir, "tag", "--annotate", "--message", "v"+version, "v"+version)
	run(r.t, r.dir, "switch", "--quiet", "main")
	return c
}

// plugins returns the plugins of main's HEAD, as Changed takes them.
func (r *repo) plugins() []marketplace.Plugin {
	r.t.Helper()
	var ps []marketplace.Plugin
	for dir, deps := range r.deps {
		if _, err := os.Stat(filepath.Join(r.dir, filepath.FromSlash(dir))); err != nil {
			continue
		}
		parts := strings.Split(dir, "/")
		ps = append(ps, marketplace.Plugin{
			Category: parts[1], Name: parts[2],
			Manifest: []byte(manifest(parts[2], "0.0.0-dev", deps)),
		})
	}
	return ps
}

func TestChanged(t *testing.T) {
	r := newRepo(t)
	r.add("plugins/write/alpha", `[]`, "alpha 1")
	r.commit("feat: add alpha")
	r.release("0.9.0")

	r.add("plugins/write/beta", `["alpha"]`, "beta")
	r.add("plugins/write/theta", `[{"name": "beta"}]`, "theta")
	r.add("plugins/code/gamma", `[{"name": "delta"}]`, "gamma")
	r.add("plugins/code/delta", `[]`, "delta 1")
	r.add("plugins/code/eta", `["zeta"]`, "eta")
	r.add("plugins/code/eps", `[{"name": "cech-prose", "marketplace": "cech-0-1-0"}]`, "eps")
	base := r.commit("feat: add more plugins")
	commit := r.release("0.10.0")

	r.add("plugins/write/alpha", `[]`, "alpha 2")
	r.add("plugins/code/delta", `[]`, "delta 2")
	r.add("plugins/code/zeta", `[]`, "zeta")
	r.commit("feat: change alpha and delta, add zeta")
	// A pre-release cut from main's HEAD is left out; were it the latest
	// release, alpha and delta would be unchanged.
	r.release("0.11.0-rc.1")

	got, err := Changed(t.Context(), r.dir, r.plugins())
	if err != nil {
		t.Fatal(err)
	}
	// v0.10.0 is above v0.9.0, which an order of strings would pick, and
	// whose base holds alpha alone.
	wantRelease := Release{Tag: "v0.10.0", Commit: commit, Base: base}
	if got.Latest == nil || *got.Latest != wantRelease {
		t.Errorf("Latest = %+v, want %+v", got.Latest, wantRelease)
	}
	want := map[string]Status{
		"alpha": Modified,
		"delta": Modified,
		// beta depends on alpha as a string, and theta on beta.
		"beta":  Modified,
		"theta": Modified,
		// gamma depends on delta as an object.
		"gamma": Modified,
		"zeta":  New,
		// eta depends on zeta, which is new.
		"eta": Modified,
		// eps's plugin.json differs at the tag, by the release commit's
		// version only, and its CECH dependency is not one of these
		// plugins.
		"eps": Unchanged,
	}
	if !maps.Equal(got.Status, want) {
		t.Errorf("Status = %v, want %v", got.Status, want)
	}
	wantCovered := "alpha beta delta eta gamma theta zeta"
	if c := strings.Join(got.Covered(), " "); c != wantCovered {
		t.Errorf("Covered() = %s, want %s", c, wantCovered)
	}
}

func TestChangedWithoutFinalRelease(t *testing.T) {
	r := newRepo(t)
	r.add("plugins/write/alpha", `[]`, "alpha")
	r.commit("feat: add alpha")
	r.release("0.1.0-rc.1")

	got, err := Changed(t.Context(), r.dir, r.plugins())
	if err != nil {
		t.Fatal(err)
	}
	if got.Latest != nil {
		t.Errorf("Latest = %+v, want nil", got.Latest)
	}
	if want := map[string]Status{"alpha": New}; !maps.Equal(got.Status, want) {
		t.Errorf("Status = %v, want %v", got.Status, want)
	}
}

func TestChangedRefusesBadDependencies(t *testing.T) {
	isolate(t)
	p := marketplace.Plugin{Category: "write", Name: "alpha", Manifest: []byte(`{"name": "alpha", "dependencies": [1]}`)}
	_, err := Changed(t.Context(), t.TempDir(), []marketplace.Plugin{p})
	if err == nil || !strings.Contains(err.Error(), "plugins/write/alpha/.claude-plugin/plugin.json: dependencies") {
		t.Errorf("err = %v, want one naming the manifest's dependencies", err)
	}
}

func TestLatestFinalInShallowClone(t *testing.T) {
	r := newRepo(t)
	r.add("plugins/write/alpha", `[]`, "alpha")
	r.commit("feat: add alpha")
	r.release("0.1.0")
	r.add("plugins/write/alpha", `[]`, "alpha 2")
	r.commit("feat: change alpha")

	clone := filepath.Join(t.TempDir(), "clone")
	run(t, "", "clone", "--quiet", "--depth", "1", fileURL(r.dir), clone)
	if _, err := LatestFinal(t.Context(), clone); !errors.Is(err, ErrShallow) {
		t.Errorf("err = %v, want ErrShallow", err)
	}
}

func fileURL(dir string) string {
	p := filepath.ToSlash(dir)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "file://" + p
}

func TestExportRelease(t *testing.T) {
	r := newRepo(t)
	r.add("plugins/write/alpha", `[]`, "alpha 1")
	r.add("plugins/write/beta", `[]`, "beta 1")
	r.commit("feat: add alpha and beta")
	r.release("0.1.0")
	r.add("plugins/write/alpha", `[]`, "alpha 2")
	r.write("plugins/write/alpha/skills/alpha/extra.md", "extra")
	r.commit("feat: change alpha")

	dest := t.TempDir()
	alpha := marketplace.Plugin{Category: "write", Name: "alpha"}
	if err := ExportRelease(t.Context(), r.dir, "v0.1.0", []marketplace.Plugin{alpha}, dest); err != nil {
		t.Fatal(err)
	}
	for file, want := range map[string]string{
		"plugins/write/alpha/skills/alpha/SKILL.md":      "alpha 1",
		"plugins/write/alpha/.claude-plugin/plugin.json": manifest("alpha", "0.1.0", `[]`),
	} {
		got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(file)))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", file, got, err, want)
		}
	}
	for _, file := range []string{"plugins/write/alpha/skills/alpha/extra.md", "plugins/write/beta"} {
		if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(file))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s is written, want only alpha as the tag holds it", file)
		}
	}
}

func TestExportReleaseNamesMissingPlugins(t *testing.T) {
	r := newRepo(t)
	r.add("plugins/write/alpha", `[]`, "alpha")
	r.commit("feat: add alpha")
	r.release("0.1.0")

	plugins := []marketplace.Plugin{
		{Category: "write", Name: "alpha"},
		{Category: "write", Name: "beta"},
		{Category: "code", Name: "gamma"},
	}
	err := ExportRelease(t.Context(), r.dir, "v0.1.0", plugins, t.TempDir())
	want := "v0.1.0 does not hold plugins/write/beta, plugins/code/gamma"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %s", err, want)
	}
}

// TestNeeds checks that a plugin needs its dependencies and theirs,
// once each, and that a cycle ends.
func TestNeeds(t *testing.T) {
	c := Changes{Deps: map[string][]string{
		"a": {"b", "c"}, "b": {"d"}, "c": {"d", "a"}, "d": {},
	}}
	if got := c.Needs("a"); !slices.Equal(got, []string{"b", "c", "d"}) {
		t.Errorf("Needs(a) = %v; want b, c and d", got)
	}
	if got := c.Needs("d"); len(got) != 0 {
		t.Errorf("Needs(d) = %v; want none", got)
	}
}

// TestExportTree checks that the export holds the commit's files and
// nothing git ignores or that is not committed, and that InTree tells a
// file the tree holds.
func TestExportTree(t *testing.T) {
	r := newRepo(t)
	r.add("plugins/write/alpha", `[]`, "alpha 1")
	r.write(".gitignore", "*.log\n")
	r.write(catalogue, "{}\n")
	head := r.commit("feat: add alpha")
	r.write("plugins/write/alpha/skills/alpha/SKILL.md", "alpha changed")
	r.write("plugins/write/alpha/debug.log", "ignored")
	r.write("plugins/write/alpha/new.md", "not committed")

	dest := t.TempDir()
	files, err := ExportTree(t.Context(), r.dir, head, []string{catalogue, "plugins/write/alpha/"}, dest)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(files, catalogue) || !slices.Contains(files, "plugins/write/alpha/skills/alpha/SKILL.md") {
		t.Errorf("files = %v; want the catalogue and the skill", files)
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "plugins/write/alpha/skills/alpha/SKILL.md")); string(got) != "alpha 1" {
		t.Errorf("SKILL.md = %q; want it as committed", got)
	}
	for _, f := range []string{"plugins/write/alpha/debug.log", "plugins/write/alpha/new.md"} {
		if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(f))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s is exported; want only committed files", f)
		}
	}
	if !InTree(t.Context(), r.dir, head, catalogue) || InTree(t.Context(), r.dir, head, "plugins/write/alpha/new.md") {
		t.Error("InTree does not tell the committed catalogue from a file not committed")
	}
}
