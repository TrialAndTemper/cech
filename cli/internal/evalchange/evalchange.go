// Package evalchange tells cech eval which plugins the release check
// covers: those new or changed since the latest final release (eval-runs
// spec criterion 22), and the plugins they depend on, which an arm
// installs from the same tree (criterion 23). It also writes out files as
// a commit or the latest final release holds them (criteria 23 and 28).
//
// It reads the marketplace's git repository with the git command, at the
// marketplace's root. Role bindings in .cech/roles.toml have no format
// yet, so a plugin is not yet counted as changed when a plugin its
// fixture project binds to a role has changed; the spec leaves that out
// until the format exists.
package evalchange

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/TrialAndTemper/cech/cli/internal/gitenv"
	"github.com/TrialAndTemper/cech/cli/internal/jsonedit"
	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

// ErrShallow marks a shallow clone, whose history and tags may not hold
// the latest final release; fetching them may make a retry succeed.
var ErrShallow = errors.New("the clone is shallow, so the latest final release may be out of reach; " +
	"fetch the history and tags with git fetch --unshallow --tags")

// Release is a final catalogue release (ADR 14 items 1 and 2).
type Release struct {
	// Tag is vX.Y.Z.
	Tag string
	// Commit is the release commit the tag points to.
	Commit string
	// Base is the commit of main the release was cut from: the release
	// commit's parent, which a release build has not changed.
	Base string
}

// Version returns the release's version, X.Y.Z.
func (r Release) Version() string {
	return strings.TrimPrefix(r.Tag, "v")
}

// LatestFinal returns the final release with the highest version among
// the tags of the repository at dir, or nil when it has none, in which
// case every plugin is new. Pre-releases are left out (ADR 14 item 5).
// In a shallow clone it returns ErrShallow.
func LatestFinal(ctx context.Context, dir string) (*Release, error) {
	shallow, err := git(ctx, dir, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(shallow)) != "false" {
		return nil, ErrShallow
	}
	tags, err := git(ctx, dir, "tag", "--list")
	if err != nil {
		return nil, err
	}
	var latest string
	var latestV [3]int
	for tag := range strings.Lines(string(tags)) {
		tag = strings.TrimSpace(tag)
		v, ok := finalVersion(tag)
		if ok && (latest == "" || compareVersions(v, latestV) > 0) {
			latest, latestV = tag, v
		}
	}
	if latest == "" {
		return nil, nil
	}
	r := &Release{Tag: latest}
	if r.Commit, err = revParse(ctx, dir, "refs/tags/"+latest+"^{commit}"); err != nil {
		return nil, err
	}
	if r.Base, err = revParse(ctx, dir, r.Commit+"^1"); err != nil {
		return nil, fmt.Errorf("%s: the release commit has no parent on main (ADR 14 item 1): %w", latest, err)
	}
	return r, nil
}

// finalVersion returns the major, minor and patch numbers of tag when it
// names a final release, vX.Y.Z.
func finalVersion(tag string) ([3]int, bool) {
	var v [3]int
	s, ok := strings.CutPrefix(tag, "v")
	if !ok || !marketplace.IsRelease(s) || marketplace.IsPreRelease(s) {
		return v, false
	}
	for i, part := range strings.SplitN(s, ".", 3) {
		n, err := strconv.Atoi(part)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func compareVersions(a, b [3]int) int {
	for i := range a {
		if c := cmp.Compare(a[i], b[i]); c != 0 {
			return c
		}
	}
	return 0
}

func revParse(ctx context.Context, dir, rev string) (string, error) {
	out, err := git(ctx, dir, "rev-parse", "--verify", "--quiet", "--end-of-options", rev)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Status is how a plugin stands against the latest final release.
type Status int

// The statuses a plugin can have.
const (
	// Unchanged is a plugin the release check leaves out.
	Unchanged Status = iota
	// Modified is a plugin that has changed (criterion 22).
	Modified
	// New is a plugin the latest final release does not hold.
	New
)

// String names the status as the spec does.
func (s Status) String() string {
	switch s {
	case Unchanged:
		return "unchanged"
	case Modified:
		return "changed"
	case New:
		return "new"
	}
	return fmt.Sprintf("Status(%d)", int(s))
}

// Changes are the plugins of a marketplace against its latest final
// release.
type Changes struct {
	// Latest is the latest final release, or nil when there is none.
	Latest *Release
	// Status is each plugin's status, by plugin name.
	Status map[string]Status
	// Deps are the plugins each plugin's manifest depends on, by plugin
	// name, as the manifest names them.
	Deps map[string][]string
}

// Needs returns the sorted names of the plugins name depends on,
// directly or through others, without name itself; cycles end.
func (c Changes) Needs(name string) []string {
	seen := map[string]bool{name: true}
	var out []string
	next := []string{name}
	for len(next) > 0 {
		n := next[0]
		next = next[1:]
		for _, d := range c.Deps[n] {
			if !seen[d] {
				seen[d] = true
				out = append(out, d)
				next = append(next, d)
			}
		}
	}
	slices.Sort(out)
	return out
}

// InTree reports whether the tree of rev in the repository at dir holds
// a file at p.
func InTree(ctx context.Context, dir, rev, p string) bool {
	_, err := git(ctx, dir, "cat-file", "-e", rev+":"+p)
	return err == nil
}

// Covered returns the sorted names of the plugins the release check
// covers: those new or changed.
func (c Changes) Covered() []string {
	var names []string
	for name, s := range c.Status {
		if s != Unchanged {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// Changed returns the status of each of plugins, the plugins of the
// marketplace at dir as HEAD holds them. A plugin is new when the latest
// final release holds no folder plugins/<category>/<name>/, and changed
// when a file in that folder differs between the release's Base and
// HEAD. Generated files are committed, so a change cech build writes into
// every plugin folder changes every plugin (criterion 22), while the
// versions the release commit writes do not count. A plugin is changed,
// too, when a plugin of the same marketplace that it depends on,
// directly or through others, is new or changed. Its errors are those
// of LatestFinal, of git, and of a dependency list that does not parse.
func Changed(ctx context.Context, dir string, plugins []marketplace.Plugin) (Changes, error) {
	deps := map[string][]string{}
	c := Changes{Status: map[string]Status{}, Deps: deps}
	for _, p := range plugins {
		d, err := dependencies(p.Manifest)
		if err != nil {
			return c, fmt.Errorf("%s: %w", p.ManifestPath(), err)
		}
		deps[p.Name] = d
		c.Status[p.Name] = New
	}
	latest, err := LatestFinal(ctx, dir)
	if err != nil || latest == nil {
		return c, err
	}
	c.Latest = latest

	held, err := git(ctx, dir, "ls-tree", "-r", "-z", "--name-only", "--full-tree", latest.Commit, "--", "plugins/")
	if err != nil {
		return c, err
	}
	diff, err := git(ctx, dir, "diff-tree", "-r", "-z", "--name-only", latest.Base, "HEAD", "--", "plugins/")
	if err != nil {
		return c, err
	}
	heldDirs, changedDirs := pluginDirs(held), pluginDirs(diff)
	for _, p := range plugins {
		switch {
		case !heldDirs[p.Dir()]:
		case changedDirs[p.Dir()]:
			c.Status[p.Name] = Modified
		default:
			c.Status[p.Name] = Unchanged
		}
	}

	// Follow dependencies until no status moves; cycles end too, since a
	// status only moves from Unchanged.
	for moved := true; moved; {
		moved = false
		for name, s := range c.Status {
			if s != Unchanged {
				continue
			}
			if slices.ContainsFunc(deps[name], func(d string) bool {
				ds, ok := c.Status[d]
				return ok && ds != Unchanged
			}) {
				c.Status[name] = Modified
				moved = true
			}
		}
	}
	return c, nil
}

// pluginDirs returns the plugin folders, plugins/<category>/<name>, of
// the NUL-separated paths in out.
func pluginDirs(out []byte) map[string]bool {
	dirs := map[string]bool{}
	for p := range strings.SplitSeq(string(out), "\x00") {
		if d, ok := pluginDir(p); ok {
			dirs[d] = true
		}
	}
	return dirs
}

// pluginDir returns the plugin folder, plugins/<category>/<name>, that
// holds the file at p.
func pluginDir(p string) (string, bool) {
	parts := strings.SplitN(p, "/", 4)
	if len(parts) < 4 || parts[0] != "plugins" {
		return "", false
	}
	return path.Join(parts[:3]...), true
}

// dependencies returns the names of the plugins the manifest src
// depends on, each given as a string or as an object with a name (ADR 11
// item 3).
func dependencies(src []byte) ([]string, error) {
	spans, _, err := jsonedit.Elements(src, "dependencies")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, s := range spans {
		dep := src[s.Start:s.End]
		var name string
		if dep[0] == '{' {
			name, _, err = jsonedit.String(dep, "name")
		} else if err = json.Unmarshal(dep, &name); err != nil {
			err = errors.New("a dependency is neither a string nor an object")
		}
		if err != nil {
			return nil, fmt.Errorf("dependencies: %w", err)
		}
		names = append(names, name)
	}
	return names, nil
}

// ExportRelease writes the folders of plugins, as the tree of tag in the
// repository at dir holds them, into dest at the same paths, such as
// dest/plugins/<category>/<name>/, with their bytes as committed, and
// the tag's catalogue .claude-plugin/marketplace.json. Its error names
// each plugin the tag does not hold.
func ExportRelease(ctx context.Context, dir, tag string, plugins []marketplace.Plugin, dest string) error {
	paths := []string{catalogue}
	for _, p := range plugins {
		paths = append(paths, p.Dir()+"/")
	}
	held, err := ExportTree(ctx, dir, "refs/tags/"+tag, paths, dest)
	if err != nil {
		return err
	}
	var missing []string
	for _, p := range plugins {
		if !slices.ContainsFunc(held, func(f string) bool { return strings.HasPrefix(f, p.Dir()+"/") }) {
			missing = append(missing, p.Dir())
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s does not hold %s", tag, strings.Join(missing, ", "))
	}
	return nil
}

// catalogue is the path of a marketplace's catalogue.
const catalogue = ".claude-plugin/marketplace.json"

// ExportTree writes the files at paths, each a file or a folder ending in
// /, as the tree of rev in the repository at dir holds them, into dest at
// the same paths, with their bytes and modes as committed, so nothing git
// ignores or a change not committed reaches dest. It returns the files it
// wrote, in git's order.
func ExportTree(ctx context.Context, dir, rev string, paths []string, dest string) ([]string, error) {
	if len(paths) == 0 {
		// With no path, git ls-tree would list the whole tree.
		return nil, nil
	}
	args := append([]string{"ls-tree", "-r", "-z", "--full-tree", rev + "^{tree}", "--"}, paths...)
	out, err := git(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	var entries []treeEntry
	var files []string
	for line := range strings.SplitSeq(string(out), "\x00") {
		if line == "" {
			continue
		}
		meta, p, ok := strings.Cut(line, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("git ls-tree: unexpected line %q", line)
		}
		entries = append(entries, treeEntry{fields[0], fields[2], p})
		files = append(files, p)
	}
	return files, writeEntries(ctx, dir, rev, dest, entries)
}

// treeEntry is a file of a git tree: its mode, blob and path.
type treeEntry struct{ mode, object, path string }

// writeEntries writes each entry's blob into dest at its path.
func writeEntries(ctx context.Context, dir, rev, dest string, entries []treeEntry) error {
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return err
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	for _, e := range entries {
		data, err := git(ctx, dir, "cat-file", "blob", e.object)
		if err != nil {
			return err
		}
		if err = root.MkdirAll(path.Dir(e.path), 0o750); err != nil {
			return err
		}
		switch e.mode {
		case "100644":
			err = root.WriteFile(e.path, data, 0o644)
		case "100755":
			err = root.WriteFile(e.path, data, 0o755)
		case "120000":
			err = root.Symlink(string(data), e.path)
		default:
			err = fmt.Errorf("%s at %s: mode %s is not a file or a symbolic link", e.path, rev, e.mode)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// git runs git with args, in dir unless it is "", and returns its
// standard output. The repository is the one at dir, whatever variables
// such as GIT_DIR say, and git never prompts for credentials.
func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // Each argument from outside follows -- or cannot start with -.
	cmd.Env = gitenv.Env()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		sub := args[0]
		if dir != "" {
			sub = args[2]
		}
		return nil, fmt.Errorf("git %s: %w: %s", sub, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
