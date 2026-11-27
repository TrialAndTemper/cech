package app

import (
	"encoding/json/v2"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/TrialAndTemper/cech/cli/internal/evalchange"
	"github.com/TrialAndTemper/cech/cli/internal/evalrun"
	"github.com/TrialAndTemper/cech/cli/internal/evalstats"
	"github.com/TrialAndTemper/cech/cli/internal/gitenv"
	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
	"github.com/TrialAndTemper/cech/cli/internal/releasecheck"
)

// gitFixture copies the eval fixture into a new git repository with one
// commit, as cech eval needs a commit to name, and returns its folder.
func gitFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	copyTree(t, evalFixture, dir, nil)
	gitIn(t, dir, "init", "--quiet", "--initial-branch=main")
	gitIn(t, dir, "add", "--all")
	gitIn(t, dir, "commit", "--quiet", "--no-gpg-sign", "--message", "Start")
	return dir
}

// gitIn runs git in dir with no config of the machine's.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = gitenv.Env("GIT_CONFIG_GLOBAL="+filepath.Join(dir, ".nogitconfig"), "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", args[0], err, out)
	}
}

// TestEvalDryRun checks --dry-run: it plans five runs for each arm and at
// most the cap for each arm a bar reads, estimates their time, and
// writes nothing (eval-runs spec criteria 1, 27 and 28).
func TestEvalDryRun(t *testing.T) {
	dir := gitFixture(t)
	code, stdout, stderr := runIn(t, "eval", "--dry-run", "--json", "-C", dir)
	if code != ExitOK {
		t.Fatalf("exit %d; stderr %s", code, stderr)
	}
	var res evalRunResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("stdout is no JSON: %v\n%s", err, stdout)
	}
	if !res.DryRun || len(res.Plugins) != 1 || res.Plugins[0] != "cech-example" || len(res.Plan) != 2 {
		t.Fatalf("result %+v; want a dry run of cech-example on both harnesses", res)
	}
	// 8 cases with 2 arms each: 80 runs to start; the content case's arm
	// with the plugin, the control's two arms and the six routed trigger
	// arms grow to the cap of 500, the other 7 arms stay at 5.
	for _, p := range res.Plan {
		if p.FirstRuns != 80 || p.MostRuns != 4535 || p.FirstMinutes <= 0 {
			t.Errorf("%s plan %+v; want 80 runs to start and at most 4535", p.Harness, p)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "evals", "results")); !os.IsNotExist(err) {
		t.Errorf("a dry run wrote evals/results/: %v", err)
	}
}

// TestEvalRefusesADirtyTree checks that cech eval will not run on a
// change it could not name a commit for (criterion 29).
func TestEvalRefusesADirtyTree(t *testing.T) {
	dir := gitFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "evals", "writing", "cech-example", "routing.md"), []byte("| x | y | z |\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runIn(t, "eval", "--dry-run", "-C", dir)
	if code != ExitFailure || !strings.Contains(stderr, "differ from HEAD") {
		t.Errorf("exit %d, stderr %q; want 1 and the refusal", code, stderr)
	}
	// A report file of its own is no reason to refuse.
	if err := exec.CommandContext(t.Context(), "git", "-C", dir, "checkout", "--", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "evals", "reports"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evals", "reports", strings.Repeat("ab", 20)+".md"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runIn(t, "eval", "--dry-run", "-C", dir); code != ExitOK {
		t.Errorf("with an uncommitted report file: exit %d, stderr %q; want 0", code, stderr)
	}
}

// TestEvalUnknownPlugin checks that a --plugin naming no plugin is a
// wrong command line (criterion 28).
func TestEvalUnknownPlugin(t *testing.T) {
	dir := gitFixture(t)
	code, _, stderr := runIn(t, "eval", "--dry-run", "--plugin", "cech-nothing", "-C", dir)
	if code != ExitUsage || !strings.Contains(stderr, "names no plugin") {
		t.Errorf("exit %d, stderr %q; want 2", code, stderr)
	}
}

// TestEvalNoImage checks that without an image to run, cech eval says so
// and exits 3, since what it needs cannot be reached (criterion 29).
func TestEvalNoImage(t *testing.T) {
	dir := gitFixture(t)
	t.Setenv("CECH_EVAL_IMAGE", "")
	code, _, stderr := runIn(t, "eval", "-C", dir)
	if code != ExitUnreachable || !strings.Contains(stderr, "no eval image") {
		t.Errorf("exit %d, stderr %q; want 3 and no eval image", code, stderr)
	}
}

// TestEvalComparesAChangedPlugin checks that a content case of a plugin
// changed since the latest final release, which that release also had,
// gets the latest-release arm (criterion 1).
func TestEvalComparesAChangedPlugin(t *testing.T) {
	dir := gitFixture(t)
	gitIn(t, dir, "switch", "--quiet", "--detach")
	gitIn(t, dir, "commit", "--quiet", "--no-gpg-sign", "--allow-empty", "--message", "chore: release 0.1.0")
	gitIn(t, dir, "tag", "v0.1.0")
	gitIn(t, dir, "switch", "--quiet", "main")
	skill := filepath.Join(dir, "plugins", "writing", "cech-example", "skills", "cech-example", "SKILL.md")
	src, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, append(src, []byte("\nA new rule.\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "commit", "--quiet", "--no-gpg-sign", "--all", "--message", "feat: change the skill")
	code, stdout, stderr := runIn(t, "eval", "--dry-run", "--json", "--harness", "codex", "-C", dir)
	if code != ExitOK {
		t.Fatalf("exit %d; stderr %s", code, stderr)
	}
	var res evalRunResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatal(err)
	}
	// The 80 runs of TestEvalDryRun's first round, and 5 for the trailer
	// case's latest-release arm.
	if len(res.Plan) != 1 || res.Plan[0].FirstRuns != 85 {
		t.Errorf("plan %+v; want 85 runs to start", res.Plan)
	}
}

// TestEvalUnknownPluginInADirtyTree checks that a wrong --plugin is a
// wrong command line in any checkout, with no JSON (criterion 28), and
// that a plugin named twice runs once.
func TestEvalUnknownPluginInADirtyTree(t *testing.T) {
	dir := gitFixture(t)
	code, stdout, _ := runIn(t, "eval", "--dry-run", "--json", "--plugin", "cech-example,cech-example", "-C", dir)
	var res evalRunResult
	if code != ExitOK || json.Unmarshal([]byte(stdout), &res) != nil || len(res.Plugins) != 1 || res.Plan[0].FirstRuns != 80 {
		t.Errorf("exit %d, %s; want cech-example once", code, stdout)
	}
	if err := os.WriteFile(filepath.Join(dir, "evals", "writing", "cech-example", "routing.md"), []byte("| x | y | z |\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runIn(t, "eval", "--dry-run", "--json", "--plugin", "cech-nothing", "-C", dir)
	if code != ExitUsage || stdout != "" || !strings.Contains(stderr, "names no plugin") {
		t.Errorf("exit %d, stdout %q, stderr %q; want 2 and no JSON", code, stdout, stderr)
	}
}

// TestCredential checks the order criterion 20 sets: an API key before
// a subscription, and CODEX_HOME's login.
func TestCredential(t *testing.T) {
	for _, v := range []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN", "CODEX_API_KEY"} {
		t.Setenv(v, "")
	}
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	check := func(harness string, want evalrun.Credential) {
		t.Helper()
		got, err := credential(harness)
		if err != nil || got != want {
			t.Errorf("%s: %+v, %v; want %+v", harness, got, err, want)
		}
	}
	if _, err := credential("claude-code"); err == nil {
		t.Error("claude-code with no credential: no error")
	}
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "token")
	check("claude-code", evalrun.Credential{Kind: "a subscription", Env: "CLAUDE_CODE_OAUTH_TOKEN"})
	t.Setenv("ANTHROPIC_API_KEY", "from-env")
	check("claude-code", evalrun.Credential{Kind: "an API key", Env: "ANTHROPIC_API_KEY"})

	if _, err := credential("codex"); err == nil {
		t.Error("codex with no credential: no error")
	}
	login := filepath.Join(codexHome, "auth.json")
	if err := os.WriteFile(login, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	check("codex", evalrun.Credential{Kind: "a subscription", LoginFile: login})
	t.Setenv("CODEX_API_KEY", "ckey")
	check("codex", evalrun.Credential{Kind: "an API key", Env: "CODEX_API_KEY"})
}

// TestExitFor checks criterion 29's choice between 1 and 3: 3 only when
// every harness error is one a retry may fix and every other harness
// passed.
func TestExitFor(t *testing.T) {
	harness := func(name string, v evalstats.Verdict) releasecheck.HarnessReport {
		return releasecheck.HarnessReport{Name: name, Result: releasecheck.Result{Runs: 10, Bars: []releasecheck.BarResult{{Verdict: v}}}}
	}
	unreachable := retryable(errors.New("codex: every run errored"))
	if !errors.Is(unreachable, errUnreachable) || !strings.HasSuffix(unreachable.Error(), "; a retry may work") {
		t.Fatalf("retryable error %q", unreachable)
	}
	for _, tt := range []struct {
		name      string
		harnesses []releasecheck.HarnessReport
		errs      map[string]error
		want      int
	}{
		{"every bar passed", []releasecheck.HarnessReport{harness("codex", evalstats.Pass)}, nil, ExitOK},
		{"a bar has no result", []releasecheck.HarnessReport{harness("codex", evalstats.NoResult)}, nil, ExitFailure},
		{"a bar failed", []releasecheck.HarnessReport{harness("codex", evalstats.Fail)}, nil, ExitFailure},
		{"every run errored", []releasecheck.HarnessReport{harness("codex", evalstats.NoResult)}, map[string]error{"codex": unreachable}, ExitUnreachable},
		{"unreachable, and the other harness passed", []releasecheck.HarnessReport{harness("codex", evalstats.NoResult), harness("claude-code", evalstats.Pass)}, map[string]error{"codex": unreachable}, ExitUnreachable},
		{"unreachable, and a bar of the other harness failed", []releasecheck.HarnessReport{harness("codex", evalstats.NoResult), harness("claude-code", evalstats.Fail)}, map[string]error{"codex": unreachable}, ExitFailure},
		{"unreachable, and a bar of the other harness has no result", []releasecheck.HarnessReport{harness("codex", evalstats.NoResult), harness("claude-code", evalstats.NoResult)}, map[string]error{"codex": unreachable}, ExitFailure},
		{"unreachable, and no credential", []releasecheck.HarnessReport{harness("codex", evalstats.NoResult), harness("claude-code", evalstats.NoResult)}, map[string]error{"codex": unreachable, "claude-code": errors.New("claude-code: no credential")}, ExitFailure},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := exitFor(releasecheck.Report{Harnesses: tt.harnesses}, tt.errs); got != tt.want {
				t.Errorf("exitFor = %d; want %d", got, tt.want)
			}
		})
	}
}

// TestStage checks what the arms read: the commit's tree, not a file
// that is not committed, and the latest final
// release's plugins as its tag holds them, without a plugin added since
// (criteria 23 and 28).
func TestStage(t *testing.T) {
	dir := gitFixture(t)
	skill := filepath.Join("plugins", "writing", "cech-example", "skills", "cech-example", "SKILL.md")
	gitIn(t, dir, "switch", "--quiet", "--detach")
	gitIn(t, dir, "commit", "--quiet", "--no-gpg-sign", "--allow-empty", "--message", "chore: release 0.1.0")
	gitIn(t, dir, "tag", "v0.1.0")
	gitIn(t, dir, "switch", "--quiet", "main")
	released, err := os.ReadFile(filepath.Join(dir, skill))
	if err != nil {
		t.Fatal(err)
	}
	if werr := os.WriteFile(filepath.Join(dir, skill), append(released, []byte("\nA new rule.\n")...), 0o600); werr != nil {
		t.Fatal(werr)
	}
	gitIn(t, dir, "commit", "--quiet", "--no-gpg-sign", "--all", "--message", "feat: change the skill")
	commit, err := gitOut(t.Context(), dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	// This file is not in the commit.
	fixture := filepath.Join("evals", "writing", "cech-example", "project", "notes.md")
	if werr := os.WriteFile(filepath.Join(dir, fixture), []byte("not committed\n"), 0o600); werr != nil {
		t.Fatal(werr)
	}
	m, err := marketplace.Load(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	changes, err := evalchange.Changed(t.Context(), dir, m.Plugins)
	if err != nil {
		t.Fatal(err)
	}
	m.Plugins = append(m.Plugins, marketplace.Plugin{Category: "writing", Name: "cech-new"})
	st, err := stage(t.Context(), dir, commit, t.TempDir(), m, []string{"cech-example"},
		map[string]bool{"cech-example": true, "cech-new": true}, changes.Latest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(st.fixtures, fixture)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an uncommitted fixture file reached the runs: %v", err)
	}
	if _, err := os.Stat(filepath.Join(st.fixtures, "evals", "writing", "cech-example", "routing.md")); err != nil {
		t.Errorf("the commit's routing rows are not staged: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(st.marketplace, skill)); !strings.Contains(string(got), "A new rule.") {
		t.Errorf("the staged marketplace's SKILL.md is not the commit's")
	}
	if got, _ := os.ReadFile(filepath.Join(st.latest, skill)); string(got) != string(released) {
		t.Errorf("the staged release's SKILL.md is not the tag's")
	}
	if _, err := os.Stat(filepath.Join(st.latest, "plugins", "writing", "cech-new")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a plugin added since the release is in the staged release: %v", err)
	}
	if st.latestVersion != "0.1.0" {
		t.Errorf("latestVersion = %q; want 0.1.0", st.latestVersion)
	}
}

// TestEvalRefusals checks the refusals of criterion 29 that a dry
// run reaches: an untracked file under plugins/, a change under
// .claude-plugin/, a problem cech eval --check finds, and a dependency
// the marketplace does not hold.
func TestEvalRefusals(t *testing.T) {
	manifest := "plugins/writing/cech-example/.claude-plugin/plugin.json"
	for _, tt := range []struct {
		name   string
		change func(t *testing.T, dir string)
		want   string
	}{
		{"an untracked plugin file", writeTo("plugins/writing/cech-example/notes.md", "x\n"), "differ from HEAD"},
		{"a change under .claude-plugin", writeTo(".claude-plugin/marketplace.json", "{}\n"), "differ from HEAD"},
		{"a problem in the cases", func(t *testing.T, dir string) {
			writeTo("evals/notes.md", "x\n")(t, dir)
			gitIn(t, dir, "add", "--all")
			gitIn(t, dir, "commit", "--quiet", "--no-gpg-sign", "--message", "docs: add notes")
		}, "cech eval --check finds problems"},
		// The generated manifest is edited by hand on purpose: it is
		// where a plugin's dependencies live.
		{"a dependency the marketplace does not hold", func(t *testing.T, dir string) {
			writeTo(manifest, "{\n  \"name\": \"cech-example\",\n  \"author\": {\n    \"name\": \"CECH maintainers\"\n  },\n  \"dependencies\": [\"cech-missing\"]\n}\n")(t, dir)
			gitIn(t, dir, "commit", "--quiet", "--no-gpg-sign", "--all", "--message", "feat: depend on a plugin")
		}, "which this marketplace does not hold"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := gitFixture(t)
			tt.change(t, dir)
			code, _, stderr := runIn(t, "eval", "--dry-run", "-C", dir)
			if code != ExitFailure || !strings.Contains(stderr, tt.want) {
				t.Errorf("exit %d, stderr %q; want 1 and %q", code, stderr, tt.want)
			}
		})
	}
}

// TestEvalDeniedRead checks that a case the check is denied reading
// exits 3, since a retry may work (criterion 29).
func TestEvalDeniedRead(t *testing.T) {
	skipWithoutModes(t)
	dir := gitFixture(t)
	file := evalControl + "cech.toml"
	// git status would read the file too; it is told the file is
	// unchanged, so only the check meets the denied read.
	gitIn(t, dir, "update-index", "--skip-worktree", file)
	chmod(file, 0)(t, dir)
	code, _, stderr := runIn(t, "eval", "--dry-run", "-C", dir)
	if code != ExitUnreachable || !strings.Contains(stderr, file) {
		t.Errorf("exit %d, stderr %q; want 3 and the denied file named", code, stderr)
	}
}

// TestEvalShallowClone checks that a clone without the release tags
// exits 3 and says to fetch them (criterion 29).
func TestEvalShallowClone(t *testing.T) {
	dir := gitFixture(t)
	gitIn(t, dir, "commit", "--quiet", "--no-gpg-sign", "--allow-empty", "--message", "chore: a second commit")
	clone := filepath.Join(t.TempDir(), "clone")
	gitIn(t, t.TempDir(), "clone", "--quiet", "--depth", "1", "file://"+filepath.ToSlash(dir), clone)
	code, _, stderr := runIn(t, "eval", "--dry-run", "-C", clone)
	if code != ExitUnreachable || !strings.Contains(stderr, "fetch") {
		t.Errorf("exit %d, stderr %q; want 3 and the advice to fetch the tags", code, stderr)
	}
}

// TestEvalNoEngine checks that without Podman or Docker on PATH the
// command exits 3, since a retry may work once one is installed
// (criterion 29).
func TestEvalNoEngine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH holds git by a symbolic link")
	}
	dir := gitFixture(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	// git, where it is a program rather than a script, runs without PATH.
	bin := t.TempDir()
	if err := os.Symlink(git, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("CECH_EVAL_IMAGE", "img")
	code, _, stderr := runIn(t, "eval", "--harness", "codex", "-C", dir)
	if code != ExitUnreachable || !strings.Contains(stderr, "neither podman nor docker") {
		t.Errorf("exit %d, stderr %q; want 3 and no engine found", code, stderr)
	}
}

// TestEvalProxyFails checks that a harness whose proxy cannot start has
// no result and, with no other harness, makes the command exit 3, since a
// retry may work (criterion 29). A fake podman on PATH answers every
// command but the creation of the batch's networks.
func TestEvalProxyFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake podman is a shell script")
	}
	dir := gitFixture(t)
	bin := t.TempDir()
	fake := `#!/bin/sh
case "$1 $2" in
"--version "*) echo "podman version 5.0.0" ;;
"image inspect") echo "sha256:0000" ;;
"network create") echo "network create refused" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "podman"), []byte(fake), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	// The telemetry receiver's socket goes in the user's cache folder; a
	// short temporary one keeps the test off the real one and the socket's
	// path under its length limit.
	cache, err := os.MkdirTemp("", "c")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cache) })
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("HOME", cache)
	t.Setenv("CECH_EVAL_IMAGE", "img")
	t.Setenv("CODEX_API_KEY", "not-a-key")
	code, _, stderr := runIn(t, "eval", "--harness", "codex", "-C", dir)
	if code != ExitUnreachable || !strings.Contains(stderr, "starting the proxy") || !strings.Contains(stderr, "a retry may work") {
		t.Errorf("exit %d, stderr %q; want 3 and the proxy named", code, stderr)
	}
}

// TestNoResult checks that a harness that could not run reports every
// bar it would have read with no result (criterion 29).
func TestNoResult(t *testing.T) {
	cases := []releasecheck.Case{
		{Plugin: "p", Name: "c", Family: "content", Expect: "better"},
		{Plugin: "p", Name: "k", Family: "control", Expect: "same"},
		{Plugin: "p", Name: "t", Family: "trigger", Expect: "load"},
	}
	hr := harnessCheck{harness: "codex", cases: cases}.noResult(releasecheck.HarnessReport{Name: "codex"})
	if len(hr.Result.Bars) != 3 || hr.Result.Passed() {
		t.Fatalf("bars %+v; want the content, control and load bars, not passed", hr.Result.Bars)
	}
	for _, b := range hr.Result.Bars {
		if b.Verdict != evalstats.NoResult {
			t.Errorf("%s: %v; want no result", b.Kind, b.Verdict)
		}
	}
}
