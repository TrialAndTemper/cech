package evalrun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The folder in a run's container where Claude Code is installed, which
// the eval image leaves out since its licence forbids passing it on
// (ADR 26 item 1), and the script in the image that installs it there
// from the image's pinned lockfile.
const (
	ctrTools         = "/cech/tools/claude-code"
	installScript    = "cech-install-claude-code"
	claudeCodeBinDir = ctrTools + "/node_modules/.bin"
	// installedMarker is the file installScript writes as its last step.
	installedMarker = ".cech-installed"
)

// userCacheDir is the user's cache folder; tests replace it, since
// os.UserCacheDir reads XDG_CACHE_HOME only on Linux and other Unix.
var userCacheDir = os.UserCacheDir

// InstallClaudeCode installs Claude Code, at the version the image's
// lockfile pins, into a cache folder on this machine, once per image,
// and returns the folder, which each Claude Code run mounts read-only
// (criterion 19). The install runs in a container from the image, on the
// engine's default network, with no credential.
func InstallClaudeCode(ctx context.Context, e *Engine, s Settings) (string, error) {
	cache, err := userCacheDir()
	if err != nil {
		return "", err
	}
	name := strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(s.Image)
	dir := filepath.Join(cache, "cech", "claude-code", name)
	if merr := os.MkdirAll(filepath.Dir(dir), 0o750); merr != nil {
		return "", merr
	}
	// Another cech eval may be installing into the same folder; this one
	// waits for it, then finds its marker.
	unlock, err := waitLock(ctx, dir+".lock")
	if err != nil {
		return "", err
	}
	defer unlock()
	// The script writes installedMarker last, so a folder without it is
	// an install that stopped, and starts again from empty.
	if _, serr := os.Stat(filepath.Join(dir, installedMarker)); serr == nil {
		return dir, nil
	}
	if rerr := os.RemoveAll(dir); rerr != nil {
		return "", rerr
	}
	if merr := os.MkdirAll(dir, 0o750); merr != nil {
		return "", merr
	}
	id, err := randomID()
	if err != nil {
		return "", err
	}
	if _, err := e.Start(ctx, nil, id, "cech-eval-"+id+"-install", e.mount(dir, ctrTools, false), s.Image, installScript); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("installing Claude Code from the image's lockfile: %w", err)
	}
	return dir, nil
}

// waitLock takes the lock file at path, waiting while another process
// holds it, until ctx ends.
func waitLock(ctx context.Context, path string) (func(), error) {
	for {
		unlock, err := Lock(path)
		if !errors.Is(err, ErrLocked) {
			return unlock, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// RefreshCodexLogin lets Codex refresh the ChatGPT login at login, so
// each run then gets a fresh copy (criterion 21). It copies the login
// into a folder of scratch, runs codex login status there in a
// container, and puts the file back in place of the login by a rename,
// so the container never sees the user's Codex home, and no label of
// that home changes. That this refreshes a login old enough to need it
// is not yet checked; once it is, the check belongs in the harness data
// file.
func RefreshCodexLogin(ctx context.Context, e *Engine, s Settings, login, scratch string) error {
	if filepath.Base(login) != "auth.json" {
		return errors.New("the Codex login is not an auth.json")
	}
	// The rename below replaces the file the login names, not a link to
	// it.
	if target, lerr := filepath.EvalSymlinks(login); lerr == nil {
		login = target
	}
	src, err := os.ReadFile(login) // #nosec G304 -- the user's own Codex login
	if err != nil {
		return err
	}
	home, err := os.MkdirTemp(scratch, "codex-login-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(home) }()
	if werr := os.WriteFile(filepath.Join(home, "auth.json"), src, 0o600); werr != nil { // #nosec G703 -- a file of a new temporary folder
		return werr
	}
	id, err := randomID()
	if err != nil {
		return err
	}
	if _, serr := e.Start(ctx, nil, id, "cech-eval-"+id+"-login", "--env", "CODEX_HOME=/cech/codex-home",
		e.mount(home, "/cech/codex-home", false), s.Image, "codex", "login", "status"); serr != nil {
		return serr
	}
	out, err := os.ReadFile(filepath.Join(home, "auth.json")) // #nosec G304 -- the copy in scratch
	if err != nil {
		return err
	}
	if bytes.Equal(out, src) {
		return nil
	}
	// A new file beside the login, renamed over it, so a reader never
	// sees half a login.
	tmp, err := os.CreateTemp(filepath.Dir(login), ".auth.json-")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), login)
}

// Version is the harness's version in the image, as its --version
// prints it.
func (b *Batch) Version(ctx context.Context) (string, error) {
	bin := "codex"
	if b.harness == claudeCode {
		bin = "claude"
	}
	args := append(b.toolMounts(), "--env", "PATH="+b.path(), b.s.Image, bin, "--version")
	out, err := b.e.Start(ctx, nil, b.id, "cech-eval-"+b.id+"-version", args...)
	return strings.TrimSpace(string(out)), err
}

// toolMounts are the read-only mounts of the tools a run uses that the
// image leaves out.
func (b *Batch) toolMounts() []string {
	if b.Tools == "" {
		return nil
	}
	return []string{b.e.shared(b.Tools, ctrTools, true)}
}

// path is a run's PATH, with Claude Code's folder first when mounted.
func (b *Batch) path() string {
	const base = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	if b.Tools == "" {
		return base
	}
	return claudeCodeBinDir + ":" + base
}
