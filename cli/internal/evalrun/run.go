package evalrun

import (
	"context"
	"crypto/sha256"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/TrialAndTemper/cech/cli/internal/evals"
	"github.com/TrialAndTemper/cech/cli/internal/gitenv"
)

// Credential is the one credential a run gets (criterion 20).
type Credential struct {
	// Kind is "api key" or "subscription", which the report names.
	Kind string
	// Env is the environment variable that carries it, such as
	// CLAUDE_CODE_OAUTH_TOKEN, passed to the container by name so its
	// value is on no command line; empty for a Codex login file.
	Env string
	// LoginFile is a refreshed Codex auth.json, copied into the run's
	// home; empty otherwise.
	LoginFile string
}

// Run is one model run of one case in one arm.
type Run struct {
	Harness string
	// ID names the run in its telemetry path.
	ID     string
	Prompt string
	// Project is the case's fixture project, and Staged the change staged
	// on top of it, or "".
	Project, Staged string
	// Routing are the rows a routed run adds to the table under
	// ## Routing in the root AGENTS.md; nil for an unrouted run.
	Routing []string
	// Marketplace is a folder holding the marketplace to install from
	// (its .claude-plugin/marketplace.json and plugins/), or "" for the
	// arm without the plugin; Catalogue is its name, and Install the
	// plugins to install from it (criterion 23).
	Marketplace, Catalogue string
	Install                []string
	Credential             Credential
	// Out is the run folder to write, as a recording (criterion 26).
	Out string
}

// Result is what a run left, besides the recording in its folder.
type Result struct {
	// Err is why the run counts as one that errors, or nil.
	Err      error
	Duration time.Duration
	Loaded   Loaded
}

// Loaded is what a run's harness loaded (criterion 12).
type Loaded struct {
	Plugins    []string `json:"plugins"`
	Skills     []string `json:"skills"`
	MCPServers []string `json:"mcp_servers"`
}

// The folders a run's container mounts, at the placeholders a recording
// writes for local paths (the evals reference, Recordings).
const (
	ctrWorkspace = "/cech/workspace"
	ctrHome      = "/cech/home"
	ctrPlugin    = "/cech/plugin"
)

// Execute makes the run r in batch b: it builds the workspace and home in
// scratch, installs the arm's plugins, runs the harness with a timeout,
// and writes the trimmed trace, the created files and what loaded into
// r.Out. A problem with the run itself is in Result.Err; an error
// return means the runner could not work, such as a folder it cannot
// write.
func (b *Batch) Execute(ctx context.Context, r Run, scratch string) (Result, error) {
	ws := filepath.Join(scratch, "workspace")
	home := filepath.Join(scratch, "home")
	if err := buildWorkspace(ctx, ws, r.Project, r.Staged, r.Routing); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return Result{}, err
	}
	if err := b.writeHome(home, r); err != nil {
		return Result{}, err
	}
	before, err := files(ws)
	if err != nil {
		return Result{}, err
	}
	var res Result
	mounts := []string{b.e.mount(ws, ctrWorkspace, false), b.e.mount(home, ctrHome, false)}
	if r.Marketplace != "" {
		mounts = append(mounts, b.e.shared(r.Marketplace, ctrPlugin, true))
		for i, cmd := range installCommands(r.Harness, r.Catalogue, r.Install) {
			if _, ierr := b.container(ctx, r, mounts, fmt.Sprintf("install-%d", i+1), cmd...); ierr != nil {
				res.Err = fmt.Errorf("installing the plugins: %w", ierr)
				ferr := b.finish(r, ws, before, nil, &res)
				return res, ferr
			}
		}
	}
	if r.Harness == codex {
		var lerr error
		res.Loaded, lerr = b.codexLoaded(ctx, r, mounts)
		if lerr != nil {
			res.Err = lerr
			ferr := b.finish(r, ws, before, nil, &res)
			return res, ferr
		}
	}
	start := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, b.s.Timeout)
	defer cancel()
	raw, err := b.container(runCtx, r, mounts, "run", b.harnessCommand(r)...)
	res.Duration = time.Since(start)
	switch {
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		res.Err = fmt.Errorf("the run took longer than %.0f minutes", b.s.Timeout.Minutes())
	case err != nil:
		res.Err = err
	}
	if r.Harness == claudeCode {
		res.Loaded = claudeLoaded(raw)
	}
	ferr := b.finish(r, ws, before, raw, &res)
	return res, ferr
}

// container runs args in a fresh container of the run, on the batch's
// internal network, with only the environment cech eval sets
// (criterion 14), and returns its stdout. step names the container
// within the run, whose name holds the batch, so concurrent cech evals
// never share a name.
func (b *Batch) container(ctx context.Context, r Run, mounts []string, step string, args ...string) ([]byte, error) {
	run := []string{"--network", b.internal, "--workdir", ctrWorkspace}
	run = append(run, mounts...)
	run = append(run, b.toolMounts()...)
	for _, kv := range b.environment(r) {
		run = append(run, "--env", kv)
	}
	if r.Credential.Env != "" {
		// By name only: the engine takes the value from its own
		// environment, which cech eval has, so it is on no command line.
		run = append(run, "--env", r.Credential.Env)
	}
	run = append(run, b.s.Image)
	run = append(run, args...)
	return b.e.Start(ctx, nil, b.id, "cech-eval-"+b.id+"-"+r.ID+"-"+step, run...)
}

// environment is what a run's container sees, and nothing else.
func (b *Batch) environment(r Run) []string {
	proxy := fmt.Sprintf("http://%s:%d", proxyHost, proxyPort)
	env := []string{
		"HOME=" + ctrHome,
		"HTTPS_PROXY=" + proxy, "HTTP_PROXY=" + proxy,
		"https_proxy=" + proxy, "http_proxy=" + proxy,
		"NO_PROXY=" + otlpHost, "no_proxy=" + otlpHost,
		"TMPDIR=" + ctrHome + "/tmp",
		"PATH=" + b.path(),
	}
	if r.Harness == claudeCode {
		env = append(env,
			"CLAUDE_CODE_ENABLE_TELEMETRY=1",
			"OTEL_LOGS_EXPORTER=otlp",
			"OTEL_METRICS_EXPORTER=otlp",
			"OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf",
			fmt.Sprintf("OTEL_EXPORTER_OTLP_ENDPOINT=http://%s:%d/%s", otlpHost, otlpPort, r.ID),
			"OTEL_LOG_TOOL_DETAILS=1",
			"OTEL_METRIC_EXPORT_INTERVAL=1000",
			"OTEL_LOGS_EXPORT_INTERVAL=1000",
			// The folder Claude Code runs from is read-only.
			"DISABLE_AUTOUPDATER=1",
		)
	}
	return env
}

// writeHome writes what the run's home needs before the run: a Codex
// login copy and Codex's telemetry settings.
func (b *Batch) writeHome(home string, r Run) error {
	root, err := os.OpenRoot(home)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if err := root.MkdirAll("tmp", 0o700); err != nil {
		return err
	}
	if r.Harness != codex {
		return nil
	}
	if err := root.MkdirAll(".codex", 0o700); err != nil {
		return err
	}
	if r.Credential.LoginFile != "" {
		// The login cech eval refreshed, at the path the user's Codex keeps.
		src, err := os.ReadFile(r.Credential.LoginFile) // #nosec G304 -- the user's own Codex login
		if err != nil {
			return err
		}
		if err := root.WriteFile(".codex/auth.json", src, 0o600); err != nil {
			return err
		}
	}
	conf := fmt.Sprintf("[otel]\nexporter = { otlp-http = { endpoint = %q, protocol = \"binary\" } }\n",
		fmt.Sprintf("http://%s:%d/%s/v1/logs", otlpHost, otlpPort, r.ID))
	return root.WriteFile(".codex/config.toml", []byte(conf), 0o600)
}

// installCommands are the commands that add the marketplace and install
// each plugin, natively, in the arm's home only.
func installCommands(harness, catalogue string, plugins []string) [][]string {
	var cmds [][]string
	if harness == claudeCode {
		cmds = append(cmds, []string{"claude", "plugin", "marketplace", "add", ctrPlugin})
		for _, p := range plugins {
			cmds = append(cmds, []string{"claude", "plugin", "install", p + "@" + catalogue})
		}
		return cmds
	}
	cmds = append(cmds, []string{"codex", "plugin", "marketplace", "add", ctrPlugin})
	for _, p := range plugins {
		cmds = append(cmds, []string{"codex", "plugin", "add", p + "@" + catalogue})
	}
	return cmds
}

// harnessCommand is the headless run of the case's prompt, as the evals
// brief notes it, with the harness's own permission checks off: the
// container is the sandbox (ADR 26 item 1).
func (b *Batch) harnessCommand(r Run) []string {
	if r.Harness == claudeCode {
		return []string{
			"claude", "-p", r.Prompt, "--output-format", "stream-json", "--verbose",
			"--model", b.s.Models[claudeCode], "--permission-mode", "bypassPermissions", "--strict-mcp-config",
		}
	}
	return []string{
		"codex", "exec", "--json", "--ephemeral", "--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox", "-m", b.s.Models[codex],
		"-c", `model_reasoning_effort="low"`, r.Prompt,
	}
}

// claudeLoaded reads the plugins, skills and MCP servers from the init
// line of Claude Code's stream-json output, as the harness data file's
// entry claude-code-run-loaded records.
func claudeLoaded(raw []byte) Loaded {
	var l Loaded
	for line := range strings.SplitSeq(string(raw), "\n") {
		var init struct {
			Type    string   `json:"type"`
			Subtype string   `json:"subtype"`
			Skills  []string `json:"skills"`
			Plugins []struct {
				Name string `json:"name"`
			} `json:"plugins"`
			MCPServers []struct {
				Name string `json:"name"`
			} `json:"mcp_servers"`
		}
		if json.Unmarshal([]byte(line), &init) != nil || init.Type != "system" || init.Subtype != "init" {
			continue
		}
		for _, p := range init.Plugins {
			l.Plugins = append(l.Plugins, p.Name)
		}
		for _, m := range init.MCPServers {
			l.MCPServers = append(l.MCPServers, m.Name)
		}
		l.Skills = init.Skills
		break
	}
	l.sort()
	return l
}

func (l *Loaded) sort() {
	for _, s := range []*[]string{&l.Plugins, &l.Skills, &l.MCPServers} {
		slices.Sort(*s)
		*s = slices.Compact(*s)
		if *s == nil {
			*s = []string{}
		}
	}
}

// codexLoaded asks Codex, in the run's container, which skills its
// prompt would list and which plugins and MCP servers it has, since
// codex exec --json names none (criterion 12): the workaround of the
// harness data file's entry codex-run-loaded.
func (b *Batch) codexLoaded(ctx context.Context, r Run, mounts []string) (Loaded, error) {
	var l Loaded
	pi, err := b.container(ctx, r, mounts, "prompt-input", "codex", "debug", "prompt-input", "x")
	if err != nil {
		return l, fmt.Errorf("reading what Codex loads: %w", err)
	}
	l.Skills = codexSkills(string(pi))
	if r.Marketplace != "" {
		l.Plugins = append(l.Plugins, r.Install...)
	}
	mcp, err := b.container(ctx, r, mounts, "mcp-list", "codex", "mcp", "list", "--json")
	if err != nil {
		return l, fmt.Errorf("reading Codex's MCP servers: %w", err)
	}
	var servers []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(mcp, &servers); err != nil {
		return l, fmt.Errorf("reading Codex's MCP servers: %w", err)
	}
	for _, s := range servers {
		l.MCPServers = append(l.MCPServers, s.Name)
	}
	l.sort()
	return l, nil
}

// codexSkills reads the skill names from the skills list of Codex's
// prompt input, whose entries are "- <name>: <description> (file: ...)".
func codexSkills(promptInput string) []string {
	var names []string
	text := strings.ReplaceAll(promptInput, `\n`, "\n")
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "- ")
		if !ok || !strings.Contains(rest, "(file: ") {
			continue
		}
		name, _, ok := strings.Cut(rest, ": ")
		if ok && name != "" && !strings.ContainsAny(name, " `") {
			names = append(names, name)
		}
	}
	return names
}

// finish writes the run's folder: the trimmed trace, the created files
// and created.txt, loaded.json, and run.json with the run's error, which
// a trace without its last line sets.
func (b *Batch) finish(r Run, ws string, before map[string][32]byte, raw []byte, res *Result) error {
	if err := os.MkdirAll(r.Out, 0o750); err != nil {
		return err
	}
	out, err := os.OpenRoot(r.Out)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if raw != nil {
		trimmed, terr := evals.Trim(r.Harness, raw)
		if terr != nil && res.Err == nil {
			res.Err = terr
		}
		if werr := out.WriteFile("trace.jsonl", trimmed, 0o600); werr != nil {
			return werr
		}
	}
	after, err := files(ws)
	if err != nil {
		return err
	}
	created := createdFiles(before, after)
	if cerr := copyFiles(ws, filepath.Join(r.Out, "workspace"), created); cerr != nil {
		return cerr
	}
	list := strings.Join(created, "\n")
	if list != "" {
		list += "\n"
	}
	loaded, err := json.Marshal(res.Loaded, json.Deterministic(true))
	if err != nil {
		return err
	}
	meta := struct {
		Error   string  `json:"error,omitempty"`
		Seconds float64 `json:"seconds"`
		Harness string  `json:"harness"`
		Image   string  `json:"image"`
		Model   string  `json:"model"`
		Kind    string  `json:"credential"`
	}{Seconds: res.Duration.Seconds(), Harness: r.Harness, Image: b.s.Image, Model: b.s.Models[r.Harness], Kind: r.Credential.Kind}
	if res.Err != nil {
		meta.Error, _ = evals.Scrub(res.Err.Error()).(string)
	}
	runJSON, err := json.Marshal(meta, json.Deterministic(true))
	if err != nil {
		return err
	}
	for name, data := range map[string][]byte{
		"created.txt": []byte(list),
		"loaded.json": append(loaded, '\n'),
		"run.json":    append(runJSON, '\n'),
	} {
		if werr := out.WriteFile(name, data, 0o600); werr != nil {
			return werr
		}
	}
	return nil
}

// copyFiles copies the files at the slash-separated paths from the
// folder src into the folder dst, through roots, so no path leaves
// either.
func copyFiles(src, dst string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	if err := os.MkdirAll(dst, 0o750); err != nil {
		return err
	}
	from, err := os.OpenRoot(src)
	if err != nil {
		return err
	}
	defer func() { _ = from.Close() }()
	to, err := os.OpenRoot(dst)
	if err != nil {
		return err
	}
	defer func() { _ = to.Close() }()
	for _, p := range paths {
		data, err := from.ReadFile(p)
		if err != nil {
			return err
		}
		if dir := path.Dir(p); dir != "." {
			if err := to.MkdirAll(dir, 0o750); err != nil {
				return err
			}
		}
		if err := to.WriteFile(p, data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// buildWorkspace copies the fixture project into ws, commits it as the
// first commit of a new git repository, copies the staged change on top
// and stages it, and adds the routing rows for a routed run, as the
// evals brief notes the runner does.
func buildWorkspace(ctx context.Context, ws, project, staged string, routing []string) error {
	if err := os.RemoveAll(ws); err != nil {
		return err
	}
	if err := copyTree(project, ws); err != nil {
		return err
	}
	if routing != nil {
		if err := addRouting(ws, routing); err != nil {
			return err
		}
	}
	git := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- fixed git arguments
		cmd.Dir = ws
		cmd.Env = gitenv.Env("GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=cech", "GIT_AUTHOR_EMAIL=cech@example.com",
			"GIT_COMMITTER_NAME=cech", "GIT_COMMITTER_EMAIL=cech@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %w: %s", args[0], err, out)
		}
		return nil
	}
	if err := git("init", "--quiet", "--initial-branch=main"); err != nil {
		return err
	}
	if err := git("add", "--all"); err != nil {
		return err
	}
	if err := git("commit", "--quiet", "--no-gpg-sign", "--message", "Start"); err != nil {
		return err
	}
	if staged == "" {
		return nil
	}
	if err := copyTree(staged, ws); err != nil {
		return err
	}
	return git("add", "--all")
}

// addRouting appends rows to the table under ## Routing in the root
// AGENTS.md of the workspace ws, after the table's last row.
func addRouting(ws string, rows []string) error {
	root, err := os.OpenRoot(ws)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	src, err := root.ReadFile("AGENTS.md")
	if err != nil {
		return err
	}
	lines := strings.Split(string(src), "\n")
	in, last := false, -1
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "## "):
			in = strings.TrimSpace(l) == "## Routing"
		case in && strings.HasPrefix(strings.TrimSpace(l), "|"):
			last = i
		}
	}
	if last < 0 {
		return errors.New("the project's AGENTS.md has no table under ## Routing")
	}
	lines = slices.Insert(lines, last+1, rows...)
	return root.WriteFile("AGENTS.md", []byte(strings.Join(lines, "\n")), 0o600)
}

// copyTree copies the regular files and folders of src into dst.
func copyTree(src, dst string) error {
	paths, err := files(src)
	if err != nil {
		return err
	}
	return copyFiles(src, dst, slices.Sorted(maps.Keys(paths)))
}

// files lists the regular files of root outside .git/, by path with
// forward slashes, each with its content's hash, read through a root
// so a symbolic link cannot lead outside it.
func files(root string) (map[string][32]byte, error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	fsys := r.FS()
	out := map[string][32]byte{}
	err = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && p == ".git":
			return fs.SkipDir
		case !d.Type().IsRegular():
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		out[p] = sha256.Sum256(data)
		return nil
	})
	return out, err
}

// createdFiles are the paths that are new or changed in after, sorted; a
// deleted path is not one (criterion 11).
func createdFiles(before, after map[string][32]byte) []string {
	var out []string
	for p, h := range after {
		if old, ok := before[p]; !ok || old != h {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}
