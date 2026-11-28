package evalrun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrNoEngine is returned when neither Podman nor Docker is found; the
// caller exits with status 3 (criterion 29).
var ErrNoEngine = errors.New("found neither podman nor docker; cech eval runs each model run in a container (ADR 26)")

// Engine runs containers with Podman or Docker, whose command lines
// agree on what the runner uses.
type Engine struct {
	// Bin is the program, podman or docker.
	Bin string
	// Podman is set when Bin is Podman, which keeps the host's user id
	// in the container with --userns=keep-id; Docker takes --user.
	Podman bool
	// exec runs a command; tests replace it.
	exec func(ctx context.Context, env []string, name string, args ...string) ([]byte, []byte, error)
}

// Detect finds Podman, or else Docker, on PATH. A docker that is Podman
// underneath, as some systems install it, counts as Podman.
func Detect(ctx context.Context) (*Engine, error) {
	for _, bin := range []string{"podman", "docker"} {
		p, err := exec.LookPath(bin)
		if err != nil {
			continue
		}
		e := &Engine{Bin: p, exec: runCommand}
		out, _, err := e.exec(ctx, nil, p, "--version")
		if err != nil {
			continue
		}
		e.Podman = strings.Contains(strings.ToLower(string(out)), "podman")
		return e, nil
	}
	return nil, ErrNoEngine
}

// runCommand runs name with args and returns its stdout and stderr. env,
// when not nil, is added to the environment, so a credential reaches
// the container as -e NAME without its value on the command line.
func runCommand(ctx context.Context, env []string, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- the engine and its arguments are cech's own
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		err = fmt.Errorf("%s %s: %w: %s", name, firstArg(args), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// Run runs the engine with args.
func (e *Engine) Run(ctx context.Context, env []string, args ...string) ([]byte, []byte, error) {
	return e.exec(ctx, env, e.Bin, args...)
}

// user returns the flags that run the container as the host's user, so
// the files a run writes in its mounted folders are the host user's.
func (e *Engine) user() []string {
	if e.Podman {
		return []string{"--userns=keep-id"}
	}
	return []string{"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())}
}

// Label is the container label on every container cech eval starts,
// whose value is the batch, so Close can find and remove them all.
const Label = "cech-eval"

// confined are the flags of every container cech eval starts: no
// capabilities, no gaining of privileges, and bounds on processes and
// memory, since a run executes commands a model chose, with its
// harness's permission checks off.
func (e *Engine) confined(batch string) []string {
	return []string{
		"--label=" + Label + "=" + batch,
		"--cap-drop=all", "--security-opt=no-new-privileges",
		"--pids-limit=1024", "--memory=2g", "--memory-swap=2g",
	}
}

// Start runs a container of batch, named name, with args after the
// run command's own flags, and returns its stdout. When ctx ends first,
// it removes the container, since ending the engine's client leaves the
// container running.
func (e *Engine) Start(ctx context.Context, env []string, batch, name string, args ...string) ([]byte, error) {
	run := append([]string{"run", "--rm", "--name", name}, e.confined(batch)...)
	run = append(run, e.user()...)
	run = append(run, args...)
	out, _, err := e.Run(ctx, env, run...)
	if ctx.Err() != nil {
		_, _, _ = e.Run(context.WithoutCancel(ctx), nil, "rm", "--force", name)
	}
	return out, err
}

// ImageID returns the ID of image, pulling it when the engine does not
// hold it yet, so a run is tied to the image's content and not to a tag
// that a rebuild can move.
func (e *Engine) ImageID(ctx context.Context, image string) (string, error) {
	inspect := func() (string, error) {
		out, _, err := e.Run(ctx, nil, "image", "inspect", "--format", "{{.Id}}", image)
		return strings.TrimSpace(string(out)), err
	}
	if id, err := inspect(); err == nil && id != "" {
		return id, nil
	}
	if _, _, err := e.Run(ctx, nil, "pull", image); err != nil {
		return "", err
	}
	id, err := inspect()
	if err == nil && id == "" {
		err = fmt.Errorf("%s inspect %s gave no ID", e.Bin, image)
	}
	return id, err
}

// RemoveBatch removes every container of batch, running or not.
func (e *Engine) RemoveBatch(ctx context.Context, batch string) {
	out, _, err := e.Run(ctx, nil, "ps", "--all", "--quiet", "--filter", "label="+Label+"="+batch)
	if err != nil {
		return
	}
	if ids := strings.Fields(string(out)); len(ids) > 0 {
		_, _, _ = e.Run(ctx, nil, append([]string{"rm", "--force"}, ids...)...)
	}
}

// mount returns a --volume flag for host folder src at dst, for one
// container alone; shared mounts a folder several containers use at once.
// The folder is relabelled for SELinux, as private to one container or
// shared; Podman and Docker both take the option, and ignore it on a
// host without SELinux.
func (e *Engine) mount(src, dst string, readOnly bool) string {
	return e.volume(src, dst, readOnly, "Z")
}

func (e *Engine) shared(src, dst string, readOnly bool) string {
	return e.volume(src, dst, readOnly, "z")
}

// plain mounts src at dst read-write with no SELinux relabel, for a
// container that runs without its label.
func (e *Engine) plain(src, dst string) string {
	return e.volume(src, dst, false, "")
}

func (e *Engine) volume(src, dst string, readOnly bool, label string) string {
	opts := []string{}
	if readOnly {
		opts = append(opts, "ro")
	}
	if label != "" {
		opts = append(opts, label)
	}
	v := src + ":" + dst
	if len(opts) > 0 {
		v += ":" + strings.Join(opts, ",")
	}
	return "--volume=" + v
}
