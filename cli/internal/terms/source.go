package terms

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// binaryPeek is how many leading bytes are searched for a NUL, as git
// does, to tell a binary file.
const binaryPeek = 8000

// shortCommit is the length of a commit hash in findings, as in CECH's
// gate.
const shortCommit = 12

// repo is a git work tree and its index. Files are read through fs, an
// os.Root, so that no read leaves the work tree, even through a symlink
// in a directory of a tracked path.
type repo struct {
	root    string
	fs      *os.Root
	entries []indexEntry
	tracked map[string]bool
}

type indexEntry struct {
	path      string
	submodule bool
}

// git runs git in root. It drops every GIT_ variable, such as those a
// git hook sets to point git at another repository, so that a check run
// from a hook reads root. Its error holds git's stderr, which can quote
// a path or ref name: callers report a fixed message instead.
func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	//nolint:gosec // The arguments are this package's own git commands.
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root, "-c", "core.quotePath=false"}, args...)...)
	cmd.Env = slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(kv, "GIT_")
	})
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func openRepo(ctx context.Context, root string) (*repo, error) {
	// Fixed messages: git's own error text could quote a path.
	if _, err := git(ctx, root, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, errors.New("git cannot read a work tree here")
	}
	out, err := git(ctx, root, "ls-files", "-z", "-s")
	if err != nil {
		return nil, errors.New("git cannot list the tracked files")
	}
	rfs, err := os.OpenRoot(root)
	if err != nil {
		return nil, errors.New("cannot open the work tree")
	}
	r := &repo{root: root, fs: rfs, tracked: map[string]bool{}}
	for rec := range strings.SplitSeq(string(out), "\x00") {
		// <mode> <object> <stage>\t<path>
		meta, p, ok := strings.Cut(rec, "\t")
		if !ok || r.tracked[p] {
			continue
		}
		r.tracked[p] = true
		r.entries = append(r.entries, indexEntry{p, strings.HasPrefix(meta, "160000 ")})
	}
	return r, nil
}

func (r *repo) close() { _ = r.fs.Close() }

// fragments collects what an extension's tiers scan: the tracked paths
// first, then each tracked file, and with a range the commits'
// messages, added lines and paths, and the ref names and tag messages.
// It returns problems, notes for the output, and whether git could not
// resolve the range.
func (r *repo) fragments(ctx context.Context, rng string) (fragments, []Problem, []Note, bool) {
	var (
		files    fragments
		problems []Problem
		notes    []Note
		binary   int
		subs     int
	)
	paths := make([]string, 0, len(r.entries))
	listed := map[string]bool{}
	for _, e := range r.entries {
		paths = append(paths, e.path)
		listed[e.path] = true
		if e.submodule {
			subs++
			continue
		}
		text, kind := readText(r.fs, e.path)
		switch kind {
		case textMissing:
			problems = append(problems, Problem{ProblemMissing, e.path})
		case textUnreadable:
			problems = append(problems, Problem{ProblemUnreadable, e.path})
		case textBinary:
			binary++
		case textOK:
			files = append(files, fragment{where: e.path, file: e.path, text: text})
		}
	}
	if binary > 0 {
		notes = append(notes, Note{NoteBinarySkipped, strconv.Itoa(binary)})
	}
	if subs > 0 {
		notes = append(notes, Note{NoteSubmodules, strconv.Itoa(subs)})
	}

	var history fragments
	undecided := false
	if rng != "" {
		commits, err := git(ctx, r.root, "rev-list", rng, "--")
		switch {
		case err != nil:
			undecided = true
		case len(bytes.TrimSpace(commits)) == 0:
			notes = append(notes, Note{Kind: NoteRangeEmpty})
		default:
			var touched []string
			history, touched, err = r.history(ctx, strings.Fields(string(commits)))
			if err != nil {
				// A fixed message: git's error text could quote a path.
				problems = append(problems, Problem{ProblemRepo, "git cannot read the commits in the range"})
			}
			for _, p := range touched {
				if !listed[p] {
					listed[p] = true
					paths = append(paths, p)
				}
			}
		}
		refs, err := r.refs(ctx)
		if err != nil {
			// A fixed message: git's error text could quote a ref name
			// the private tier has not scanned.
			problems = append(problems, Problem{ProblemRepo, "git cannot list the refs"})
		}
		history = append(history, refs...)
	}

	out := fragments{{where: WherePaths, text: strings.Join(paths, "\n")}}
	out = append(out, files...)
	out = append(out, history...)
	return out, problems, notes, undecided
}

type textKind int

const (
	textOK textKind = iota
	textMissing
	textUnreadable
	textBinary
)

// readText reads the file at the slash path rel in rfs for a scan: a
// symlink gives its target path, never what it points to, and a binary
// file nothing.
func readText(rfs *os.Root, rel string) (string, textKind) {
	name := filepath.FromSlash(rel)
	fi, err := rfs.Lstat(name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", textMissing
	case err != nil:
		return "", textUnreadable
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		target, lerr := rfs.Readlink(name)
		if lerr != nil {
			return "", textUnreadable
		}
		return filepath.ToSlash(target), textOK
	}
	if !fi.Mode().IsRegular() {
		return "", textUnreadable
	}
	data, err := rfs.ReadFile(name)
	if err != nil {
		return "", textUnreadable
	}
	if bytes.IndexByte(data[:min(len(data), binaryPeek)], 0) >= 0 {
		return "", textBinary
	}
	return string(data), textOK
}

// history reads commits: each message, the lines each adds against each
// parent, kept at their line numbers, and the paths each touches.
func (r *repo) history(ctx context.Context, commits []string) (fragments, []string, error) {
	var frags fragments
	var touched []string
	seen := map[string]bool{}
	for _, c := range commits {
		short := c[:min(len(c), shortCommit)]
		msg, err := git(ctx, r.root, "log", "-1", "--no-show-signature", "--format=%B", c)
		if err != nil {
			return nil, nil, err
		}
		frags = append(frags, fragment{where: WhereMessage, commit: short, text: string(msg)})
		names, err := git(ctx, r.root, "show", "-m", "--format=", "--name-only", "-z", "--no-renames", c)
		if err != nil {
			return nil, nil, err
		}
		for p := range strings.SplitSeq(string(names), "\x00") {
			if p != "" && !seen[p] {
				seen[p] = true
				touched = append(touched, p)
			}
		}
		//nolint:misspell // git's own option.
		diff, err := git(ctx, r.root, "show", "-m", "--format=", "--unified=0", "--no-color", "--no-ext-diff",
			"--no-renames", "--src-prefix=a/", "--dst-prefix=b/", c)
		if err != nil {
			return nil, nil, err
		}
		for _, f := range addedLines(string(diff)) {
			f.commit = short
			frags = append(frags, f)
		}
	}
	return frags, touched, nil
}

// addedLines turns a diff into one fragment per file, holding its added
// lines at their line numbers and blank lines elsewhere. A file's header
// lines are read only before its first hunk, so an added line that
// starts with "++ " is content.
func addedLines(diff string) fragments {
	var order []string
	byFile := map[string][]string{}
	file, line, header := "", 0, false
	for l := range strings.SplitSeq(diff, "\n") {
		switch {
		case strings.HasPrefix(l, "diff "):
			file, line, header = "", 0, true
		case header && strings.HasPrefix(l, "+++ "):
			file = diffPath(strings.TrimPrefix(l, "+++ "))
		case strings.HasPrefix(l, "@@ "):
			line, header = hunkStart(l), false
		case header:
		case strings.HasPrefix(l, "+") && file != "" && line > 0:
			lines, ok := byFile[file]
			if !ok {
				order = append(order, file)
			}
			for len(lines) < line {
				lines = append(lines, "")
			}
			lines[line-1] = l[1:]
			byFile[file] = lines
			line++
		}
	}
	var frags fragments
	for _, f := range order {
		frags = append(frags, fragment{where: f, file: f, text: strings.Join(byFile[f], "\n")})
	}
	return frags
}

// diffPath reads the new path from a "+++ " line: b/<path>, quoted as
// git quotes a path with special characters, or /dev/null for none.
func diffPath(s string) string {
	if strings.HasPrefix(s, `"`) {
		if u, err := strconv.Unquote(s); err == nil {
			s = u
		}
	}
	p, ok := strings.CutPrefix(s, "b/")
	if !ok {
		return ""
	}
	return p
}

// hunkStart reads c from a hunk header "@@ -a,b +c,d @@".
func hunkStart(header string) int {
	_, rest, ok := strings.Cut(header, " +")
	if !ok {
		return 0
	}
	num, _, _ := strings.Cut(rest, ",")
	num, _, _ = strings.Cut(num, " ")
	n, err := strconv.Atoi(num)
	if err != nil {
		return 0
	}
	return n
}

// refs reads the names of local branches, tags and remote-tracking refs,
// and the messages of annotated tags.
func (r *repo) refs(ctx context.Context) (fragments, error) {
	names, err := git(ctx, r.root, "for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/tags", "refs/remotes")
	if err != nil {
		return nil, err
	}
	msgs, err := git(ctx, r.root, "for-each-ref", "--format=%(contents)", "refs/tags")
	if err != nil {
		return nil, err
	}
	return fragments{
		{where: WhereRefNames, text: string(names)},
		{where: WhereTagMessages, text: string(msgs)},
	}, nil
}

// projectFiles are the native files a project's scan reads.
var projectFiles = []string{".claude/settings.json", ".codex/config.toml"}

// projectFragments reads every file under .cech/ and the native files,
// tracked or not, through an os.Root on root, so that a symlinked
// directory cannot pull in a file from elsewhere.
func projectFragments(root string) ([]fragment, []Problem, error) {
	rfs, err := os.OpenRoot(root)
	if err != nil {
		return nil, nil, fmt.Errorf("opening %s: %w", root, err)
	}
	defer func() { _ = rfs.Close() }()
	var frags []fragment
	var problems []Problem
	add := func(rel string) {
		text, kind := readText(rfs, rel)
		switch kind {
		case textOK:
			frags = append(frags, fragment{where: rel, file: rel, text: text})
		case textUnreadable:
			problems = append(problems, Problem{ProblemUnreadable, rel})
		case textMissing, textBinary:
		}
	}
	// A symlinked .cech is scanned as its target path, never followed.
	if fi, lerr := rfs.Lstat(".cech"); lerr == nil && fi.Mode()&fs.ModeSymlink != 0 {
		add(".cech")
		for _, rel := range projectFiles {
			add(rel)
		}
		return frags, problems, nil
	}
	err = fs.WalkDir(rfs.FS(), ".cech", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == ".cech" && errors.Is(err, fs.ErrNotExist) {
				return err
			}
			// One unreadable folder does not stop the others.
			problems = append(problems, Problem{ProblemUnreadable, p})
			return fs.SkipDir
		}
		if !d.IsDir() {
			add(p)
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, fmt.Errorf("reading .cech: %w", err)
	}
	for _, rel := range projectFiles {
		add(rel)
	}
	return frags, problems, nil
}
