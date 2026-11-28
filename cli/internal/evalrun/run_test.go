package evalrun

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/TrialAndTemper/cech/cli/internal/evals"
)

// fakeEngine records each command and answers the harness's run with
// out, writing COMMIT_MSG into the mounted workspace the way a run
// would.
type fakeEngine struct {
	calls [][]string
	out   map[string]string // first word of the container's command to its stdout
	write bool
	// login, when set, is the login codex login status writes back.
	login string
	// block makes the harness's run wait until its context ends.
	block bool
}

func (f *fakeEngine) exec(ctx context.Context, env []string, _ string, args ...string) ([]byte, []byte, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	if len(args) == 0 || args[0] != "run" {
		return nil, nil, nil
	}
	if env != nil {
		return nil, nil, nil
	}
	img := slices.Index(args, "img@sha256:x")
	if img < 0 || img+1 >= len(args) {
		return nil, nil, nil
	}
	cmd := args[img+1:]
	if f.block && (cmd[0] == "claude" && cmd[1] == "-p" || cmd[0] == "codex" && cmd[1] == "exec") {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	if f.login != "" && cmd[0] == "codex" && cmd[1] == "login" {
		for _, a := range args {
			v, _ := strings.CutPrefix(a, "--volume=")
			if home, _, ok := strings.Cut(v, ":/cech/codex-home"); ok && home != v {
				_ = os.WriteFile(filepath.Join(home, "auth.json"), []byte(f.login), 0o600)
			}
		}
	}
	if f.write && (cmd[0] == "claude" && cmd[1] == "-p" || cmd[0] == "codex" && cmd[1] == "exec") {
		for _, a := range args {
			v, _ := strings.CutPrefix(a, "--volume=")
			if ws, _, ok := strings.Cut(v, ":"+ctrWorkspace); ok && ws != v {
				_ = os.WriteFile(filepath.Join(ws, "COMMIT_MSG"), []byte("Fix\n\nChange-Kind: fix\n"), 0o600)
			}
		}
	}
	return []byte(f.out[cmd[0]+" "+cmd[1]]), nil, nil
}

func fixture(t *testing.T) (project, staged string) {
	t.Helper()
	dir := t.TempDir()
	project = filepath.Join(dir, "project")
	staged = filepath.Join(dir, "staged")
	for f, body := range map[string]string{
		filepath.Join(project, "AGENTS.md"):       "# Contract\n\n## Routing\n\n| Change | Destination |\n| --- | --- |\n| Code | `src/` |\n\n## How to work\n",
		filepath.Join(project, "src", "pager.py"): "def page(): pass\n",
		filepath.Join(staged, "src", "pager.py"):  "def page(): return 1\n",
	} {
		if err := os.MkdirAll(filepath.Dir(f), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return project, staged
}

func testBatch(f *fakeEngine) *Batch {
	s := Defaults
	s.Image = "img@sha256:x"
	s.Timeout = time.Minute
	return &Batch{e: &Engine{Bin: "podman", Podman: true, exec: f.exec}, s: s, id: "1", internal: "cech-eval-1", outside: "cech-eval-1-out", proxy: "cech-eval-1-proxy", otlp: "cech-eval-1-otlp"}
}

const claudeRaw = `{"type":"system","subtype":"init","cwd":"/cech/workspace","session_id":"s","skills":["cech-example:cech-example","batch"],"plugins":[{"name":"cech-example","path":"/cech/home/x"}],"mcp_servers":[]}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Fix\n\nChange-Kind: fix"}]}}
{"type":"result","subtype":"success","is_error":false,"result":"Fix\n\nChange-Kind: fix","session_id":"s"}
`

// TestExecuteClaude checks a whole run with a fake engine: the plugin is
// installed before the run, the run sees only its mounts and the
// environment cech eval sets, the credential is passed by name only,
// and the run folder holds the trimmed trace, the created file and what
// loaded (criteria 11, 12, 14, 15, 20, 23 and 26).
func TestExecuteClaude(t *testing.T) {
	f := &fakeEngine{out: map[string]string{"claude -p": claudeRaw}, write: true}
	b := testBatch(f)
	project, staged := fixture(t)
	out := filepath.Join(t.TempDir(), "run")
	mkt := t.TempDir()
	res, err := b.Execute(context.Background(), Run{
		Harness: claudeCode, ID: "r1", Prompt: "Write a commit message.",
		Project: project, Staged: staged, Routing: []string{"| A commit message | `git commit` |"},
		Marketplace: mkt, Catalogue: "cech", Install: []string{"cech-example"},
		Credential: Credential{Kind: "subscription", Env: "CLAUDE_CODE_OAUTH_TOKEN"},
		Out:        out,
	}, t.TempDir())
	if err != nil || res.Err != nil {
		t.Fatalf("Execute: %v, run error %v", err, res.Err)
	}
	var install, run []string
	for _, c := range f.calls {
		joined := strings.Join(c, " ")
		switch {
		case strings.Contains(joined, "plugin install cech-example@cech"):
			install = c
		case strings.Contains(joined, "claude -p"):
			run = c
		}
	}
	if install == nil || run == nil {
		t.Fatalf("calls %v; want an install and a run", f.calls)
	}
	joined := strings.Join(run, " ")
	for _, want := range []string{
		"--network cech-eval-1", "--userns=keep-id", "--env CLAUDE_CODE_OAUTH_TOKEN ", "--env HOME=/cech/home",
		"--env OTEL_EXPORTER_OTLP_ENDPOINT=http://otlp:4318/r1", ":/cech/plugin:ro,z", "--model claude-haiku-5-5",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("run command lacks %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "CLAUDE_CODE_OAUTH_TOKEN=") {
		t.Errorf("the credential's value is on the command line:\n%s", joined)
	}
	// Every container is confined, labelled with its batch, and named
	// with it.
	for _, c := range f.calls {
		if c[0] != "run" {
			continue
		}
		joined := strings.Join(c, " ")
		for _, want := range []string{"--label=cech-eval=1", "--cap-drop=all", "--security-opt=no-new-privileges", "--pids-limit=", "--memory=", "--name cech-eval-1-r1-"} {
			if !strings.Contains(joined, want) {
				t.Errorf("container lacks %q:\n%s", want, joined)
			}
		}
	}
	// What the run leaves is a recording and the files GradeRun takes
	// beside it, so grading never rejects a run folder.
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if n := e.Name(); n != "trace.jsonl" && n != "created.txt" && n != "workspace" && !slices.Contains(evals.ResultFiles, n) {
			t.Errorf("the run folder holds %s, which grading rejects", n)
		}
	}
	for f, want := range map[string]string{
		"created.txt":          "COMMIT_MSG\n",
		"workspace/COMMIT_MSG": "Fix\n\nChange-Kind: fix\n",
		"loaded.json":          `{"plugins":["cech-example"],"skills":["batch","cech-example:cech-example"],"mcp_servers":[]}` + "\n",
	} {
		got, rerr := os.ReadFile(filepath.Join(out, f))
		if rerr != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", f, got, rerr, want)
		}
	}
	trace, err := os.ReadFile(filepath.Join(out, "trace.jsonl"))
	if err != nil || strings.Contains(string(trace), "session_id") || !strings.Contains(string(trace), `"type":"result"`) {
		t.Errorf("trace.jsonl = %q, %v; want it trimmed with its result line", trace, err)
	}
}

// TestExecuteCodexLogin checks that a Codex run gets a copy of the login
// in its home, its telemetry settings, and a list of what it loaded
// (criteria 12 and 21).
func TestExecuteCodexLogin(t *testing.T) {
	f := &fakeEngine{out: map[string]string{
		"codex debug": `<skills_instructions>\n- cech-example:cech-example: Use when writing a commit message. (file: r1/x/SKILL.md)\n- skill-creator: Make skills. (file: r0/y/SKILL.md)\n</skills_instructions>`,
		"codex mcp":   `[]`,
		"codex exec":  "{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"done\"}}\n{\"type\":\"turn.completed\"}\n",
	}}
	b := testBatch(f)
	project, _ := fixture(t)
	login := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(login, []byte(`{"tokens":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	out := filepath.Join(t.TempDir(), "run")
	res, err := b.Execute(context.Background(), Run{
		Harness: codex, ID: "r2", Prompt: "x", Project: project,
		Credential: Credential{Kind: "subscription", LoginFile: login}, Out: out,
	}, scratch)
	if err != nil || res.Err != nil {
		t.Fatalf("Execute: %v, %v", err, res.Err)
	}
	if got, _ := os.ReadFile(filepath.Join(scratch, "home", ".codex", "auth.json")); string(got) != `{"tokens":{}}` {
		t.Errorf("auth.json in the run's home = %q", got)
	}
	conf, _ := os.ReadFile(filepath.Join(scratch, "home", ".codex", "config.toml"))
	if !strings.Contains(string(conf), `endpoint = "http://otlp:4318/r2/v1/logs", protocol = "binary"`) {
		t.Errorf("config.toml = %q; want the run's telemetry endpoint", conf)
	}
	if !slices.Equal(res.Loaded.Skills, []string{"cech-example:cech-example", "skill-creator"}) {
		t.Errorf("loaded skills = %v", res.Loaded.Skills)
	}
	if got, _ := os.ReadFile(filepath.Join(out, "created.txt")); len(got) != 0 {
		t.Errorf("created.txt = %q; want none created", got)
	}
}

// TestExecuteCutOff checks that a run whose trace has no last line is a
// run that errors (criterion 9).
func TestExecuteCutOff(t *testing.T) {
	f := &fakeEngine{out: map[string]string{"claude -p": `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"x"}]}}` + "\n"}}
	project, _ := fixture(t)
	out := filepath.Join(t.TempDir(), "run")
	res, err := testBatch(f).Execute(context.Background(), Run{Harness: claudeCode, ID: "r3", Prompt: "x", Project: project, Out: out}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if res.Err == nil {
		t.Error("a trace without its result line gave no run error")
	}
	meta, _ := os.ReadFile(filepath.Join(out, "run.json"))
	if !strings.Contains(string(meta), `"error":`) {
		t.Errorf("run.json = %q; want the error recorded", meta)
	}
}

// TestBuildWorkspace checks the workspace a run starts in: the project
// committed, the staged change staged, and the routing rows after the
// last row of the table under ## Routing (criteria 11 and 28).
func TestBuildWorkspace(t *testing.T) {
	project, staged := fixture(t)
	ws := filepath.Join(t.TempDir(), "ws")
	// As under a git hook: the workspace's repository is its own all
	// the same.
	elsewhere := filepath.Join(t.TempDir(), "elsewhere.git")
	t.Setenv("GIT_DIR", elsewhere)
	if err := buildWorkspace(context.Background(), ws, project, staged, []string{"| Commit | `git` |"}); err != nil {
		t.Fatal(err)
	}
	agents, _ := os.ReadFile(filepath.Join(ws, "AGENTS.md"))
	if !strings.Contains(string(agents), "| Code | `src/` |\n| Commit | `git` |\n\n## How to work") {
		t.Errorf("AGENTS.md = %q; want the row after the table's last row", agents)
	}
	if got, _ := os.ReadFile(filepath.Join(ws, "src", "pager.py")); string(got) != "def page(): return 1\n" {
		t.Errorf("pager.py = %q; want the staged change", got)
	}
	if _, err := os.Stat(filepath.Join(ws, ".git")); err != nil {
		t.Errorf("no git repository: %v", err)
	}
	if _, err := os.Stat(elsewhere); !os.IsNotExist(err) {
		t.Errorf("git followed GIT_DIR: %v", err)
	}
}

// TestCreatedFiles checks criterion 11: new or changed paths, sorted;
// not deleted ones, and nothing in .git/.
func TestCreatedFiles(t *testing.T) {
	ws := t.TempDir()
	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(ws, p)), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, p), []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("same", "a")
	write("changed", "a")
	write("gone", "a")
	before, err := files(ws)
	if err != nil {
		t.Fatal(err)
	}
	write("changed", "b")
	write("new/file", "c")
	write(".git/index", "d")
	if rerr := os.Remove(filepath.Join(ws, "gone")); rerr != nil {
		t.Fatal(rerr)
	}
	after, err := files(ws)
	if err != nil {
		t.Fatal(err)
	}
	if got := createdFiles(before, after); !slices.Equal(got, []string{"changed", "new/file"}) {
		t.Errorf("created = %v; want [changed new/file]", got)
	}
}

// TestFilter checks that the proxy's filter matches exactly the listed
// hosts (criterion 15).
func TestFilter(t *testing.T) {
	if got := Filter([]string{"api.example.com", "a-b.example.org"}); got != "^api\\.example\\.com$\n^a-b\\.example\\.org$\n" {
		t.Errorf("Filter = %q", got)
	}
}

// TestStartBatch checks the batch's two containers: the proxy, which
// reaches outside, keeps its SELinux label, and the forwarder, which
// alone runs without it, stays on the internal network with no
// capabilities (criteria 15 and 16).
func TestStartBatch(t *testing.T) {
	f := &fakeEngine{}
	e := &Engine{Bin: "podman", Podman: true, exec: f.exec}
	s := Defaults
	s.Image = "img@sha256:x"
	b, err := StartBatch(t.Context(), e, s, "codex", "/sock", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var proxy, otlp string
	for _, c := range f.calls {
		line := strings.Join(c, " ")
		switch {
		case strings.Contains(line, "--name "+b.proxy+" "):
			proxy = line
		case strings.Contains(line, "--name "+b.otlp+" "):
			otlp = line
		}
	}
	if proxy == "" || strings.Contains(proxy, "label=disable") || strings.Contains(proxy, "/sock") {
		t.Errorf("proxy %q; want it labelled and without the socket", proxy)
	}
	for _, want := range []string{"--cap-drop=all", "--read-only", "--security-opt=no-new-privileges", "--label=cech-eval=" + b.id} {
		if !strings.Contains(proxy, want) {
			t.Errorf("proxy %q; want %q", proxy, want)
		}
	}
	for _, want := range []string{"--network " + b.internal + " ", "--security-opt=label=disable", "--cap-drop=all", "--read-only", "--volume=/sock:/cech/otlp ", "UNIX-CONNECT:/cech/otlp/otlp.sock"} {
		if !strings.Contains(otlp, want) {
			t.Errorf("forwarder %q; want %q", otlp, want)
		}
	}
	for _, c := range f.calls {
		if c[0] == "network" && c[1] == "connect" && slices.Contains(c, b.otlp) {
			t.Errorf("the forwarder joined another network: %v", c)
		}
	}
}

// TestCloseRemovesTheBatch checks that Close removes every container of
// the batch by its label, which Docker's network removal does not, and
// then both its networks (criterion 30).
func TestCloseRemovesTheBatch(t *testing.T) {
	f := &fakeEngine{out: map[string]string{}}
	b := testBatch(f)
	b.e.exec = func(ctx context.Context, env []string, name string, args ...string) ([]byte, []byte, error) {
		if args[0] == "ps" {
			f.calls = append(f.calls, args)
			return []byte("c1\nc2\n"), nil, nil
		}
		return f.exec(ctx, env, name, args...)
	}
	b.Close(t.Context())
	var ps, rm []string
	var networks []string
	rmAt, firstNetwork := -1, -1
	for i, c := range f.calls {
		switch c[0] {
		case "ps":
			ps = c
		case "rm":
			rm, rmAt = c, i
		case "network":
			if firstNetwork < 0 {
				firstNetwork = i
			}
			if len(c) == 4 && c[1] == "rm" && c[2] == "--force" {
				networks = append(networks, c[3])
			}
		}
	}
	if !slices.Contains(ps, "label=cech-eval=1") || !slices.Equal(rm, []string{"rm", "--force", "c1", "c2"}) {
		t.Errorf("ps %v, rm %v; want the batch's containers found by label and removed", ps, rm)
	}
	// An engine refuses to remove a network a container is still on.
	slices.Sort(networks)
	if !slices.Equal(networks, []string{"cech-eval-1", "cech-eval-1-out"}) || rmAt > firstNetwork {
		t.Errorf("calls %v; want both networks removed after the containers", f.calls)
	}
}

// TestRefreshCodexLogin checks that the refresh runs on a copy, never
// mounting the folder of the user's login, and puts the refreshed login
// in its place (criterion 21).
func TestRefreshCodexLogin(t *testing.T) {
	home := t.TempDir()
	login := filepath.Join(home, "auth.json")
	if err := os.WriteFile(login, []byte(`{"old":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &fakeEngine{login: `{"new":true}`}
	e := &Engine{Bin: "podman", Podman: true, exec: f.exec}
	s := Defaults
	s.Image = "img@sha256:x"
	if err := RefreshCodexLogin(t.Context(), e, s, login, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.calls {
		if strings.Contains(strings.Join(c, " "), home) {
			t.Errorf("a container mounted the login's folder: %v", c)
		}
	}
	got, err := os.ReadFile(login)
	if err != nil || string(got) != `{"new":true}` {
		t.Errorf("login = %q, %v; want the refreshed one", got, err)
	}
	if entries, _ := os.ReadDir(home); len(entries) != 1 {
		t.Errorf("the login's folder holds %d files; want the login alone", len(entries))
	}
}

// TestImageID checks that runs go by the image's ID: an image the
// engine holds is inspected, one it does not hold is pulled first, and
// one it cannot pull is an error (criterion 29).
func TestImageID(t *testing.T) {
	var calls []string
	held := map[string]bool{"img:local": true}
	e := &Engine{Bin: "podman", Podman: true, exec: func(_ context.Context, _ []string, _ string, args ...string) ([]byte, []byte, error) {
		calls = append(calls, strings.Join(args, " "))
		image := args[len(args)-1]
		switch args[0] {
		case "image":
			if held[image] {
				return []byte("abc123\n"), nil, nil
			}
			return nil, nil, errors.New("no such image")
		case "pull":
			if image == "img@sha256:x" {
				held[image] = true
				return nil, nil, nil
			}
			return nil, nil, errors.New("not found")
		}
		return nil, nil, nil
	}}
	if id, err := e.ImageID(t.Context(), "img:local"); err != nil || id != "abc123" || len(calls) != 1 {
		t.Errorf("held image: %q, %v after %v; want its ID from one inspect", id, err, calls)
	}
	calls = nil
	if id, err := e.ImageID(t.Context(), "img@sha256:x"); err != nil || id != "abc123" || !slices.Contains(calls, "pull img@sha256:x") {
		t.Errorf("image to pull: %q, %v after %v; want it pulled, then its ID", id, err, calls)
	}
	if _, err := e.ImageID(t.Context(), "img:missing"); err == nil {
		t.Error("an image that cannot be pulled gave no error")
	}
}

// TestExecuteTimeout checks that a run past the run timeout ends as a
// run that errors, saying why (criterion 9).
func TestExecuteTimeout(t *testing.T) {
	f := &fakeEngine{block: true}
	b := testBatch(f)
	b.s.Timeout = 20 * time.Millisecond
	project, _ := fixture(t)
	res, err := b.Execute(t.Context(), Run{Harness: claudeCode, ID: "r9", Prompt: "x", Project: project, Out: filepath.Join(t.TempDir(), "run")}, t.TempDir())
	if err != nil || res.Err == nil || !strings.Contains(res.Err.Error(), "longer than") {
		t.Errorf("Execute: %v, run error %v; want a run that took longer than the timeout", err, res.Err)
	}
}

// TestInstallClaudeCode checks that a cached install without its marker
// is emptied and installed again, and one with it is kept (criterion
// 19).
func TestInstallClaudeCode(t *testing.T) {
	cache := t.TempDir()
	old := userCacheDir
	userCacheDir = func() (string, error) { return cache, nil }
	t.Cleanup(func() { userCacheDir = old })
	s := Defaults
	s.Image = "abc123"
	dir := filepath.Join(cache, "cech", "claude-code", "abc123")
	if merr := os.MkdirAll(dir, 0o750); merr != nil {
		t.Fatal(merr)
	}
	if werr := os.WriteFile(filepath.Join(dir, "stale"), []byte("x"), 0o600); werr != nil {
		t.Fatal(werr)
	}
	f := &fakeEngine{}
	e := &Engine{Bin: "podman", Podman: true, exec: f.exec}
	got, err := InstallClaudeCode(t.Context(), e, s)
	if err != nil || got != dir {
		t.Fatalf("InstallClaudeCode = %q, %v; want %q", got, err, dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "stale")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a file of the stopped install is kept: %v", err)
	}
	if !slices.ContainsFunc(f.calls, func(c []string) bool { return slices.Contains(c, installScript) }) {
		t.Errorf("calls %v; want the install script run", f.calls)
	}
	if err := os.WriteFile(filepath.Join(dir, installedMarker), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f.calls = nil
	if _, err := InstallClaudeCode(t.Context(), e, s); err != nil || len(f.calls) != 0 {
		t.Errorf("with the marker: %v after %v; want the install kept, with no container", err, f.calls)
	}
}
