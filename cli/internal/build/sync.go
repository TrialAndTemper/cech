package build

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

// Change is what the build does to a file.
type Change string

// The changes the build makes.
const (
	Create Change = "create"
	Update Change = "update"
	Remove Change = "remove"
)

// Diff is a file the build would change, and how.
type Diff struct {
	Path   string `json:"path"`
	Change Change `json:"change"`
}

// Compare returns the outputs that differ from the files under root, in
// the order given, and writes nothing. A read it was denied is
// marketplace.ErrUnreadable.
func Compare(root *os.Root, outs []Output) ([]Diff, error) {
	var diffs []Diff
	for _, o := range outs {
		have, err := root.ReadFile(filepath.FromSlash(o.Path))
		exists := err == nil
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, marketplace.ReadError(fileError(o.Path, err))
		}
		switch {
		case o.Content == nil && exists && bytes.HasPrefix(have, o.Mark):
			diffs = append(diffs, Diff{o.Path, Remove})
		case o.Content == nil:
		case !exists:
			diffs = append(diffs, Diff{o.Path, Create})
		case !bytes.Equal(have, o.Content):
			diffs = append(diffs, Diff{o.Path, Update})
		}
	}
	return diffs, nil
}

// Apply makes each change in diffs, taking contents from outs, and
// returns the changes it made; on an error, those made before it, so
// the rest of diffs, from diffs[len(done)], were not made.
func Apply(root *os.Root, outs []Output, diffs []Diff) ([]Diff, error) {
	content := make(map[string][]byte, len(outs))
	for _, o := range outs {
		content[o.Path] = o.Content
	}
	done := []Diff{}
	for _, d := range diffs {
		name := filepath.FromSlash(d.Path)
		var err error
		if d.Change == Remove {
			err = root.Remove(name)
		} else {
			err = writeFile(root, d.Path, content[d.Path])
		}
		if err != nil {
			return done, fileError(d.Path, err)
		}
		done = append(done, d)
	}
	return done, nil
}

// writeFile replaces the file at the slash path p with data atomically:
// it writes a temporary file beside it and renames it over the file. A
// file that exists keeps its mode.
func writeFile(root *os.Root, p string, data []byte) error {
	name := filepath.FromSlash(p)
	if err := root.MkdirAll(filepath.FromSlash(path.Dir(p)), 0o755); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if info, err := root.Stat(name); err == nil {
		mode = info.Mode().Perm()
	}
	tmp := filepath.FromSlash(path.Join(path.Dir(p), "."+path.Base(p)+"."+rand.Text()+".tmp"))
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	// The umask can remove bits from the mode OpenFile gives the file.
	err = f.Chmod(mode)
	if err == nil {
		_, err = f.Write(data)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = root.Rename(tmp, name)
	}
	if err != nil {
		_ = root.Remove(tmp)
	}
	return err
}

// fileError names the file at the slash path p in err, in place of the
// path the system gave, which can be absolute or a temporary file's.
func fileError(p string, err error) error {
	var pathErr *fs.PathError
	var linkErr *os.LinkError
	switch {
	case errors.As(err, &pathErr):
		err = pathErr.Err
	case errors.As(err, &linkErr):
		err = linkErr.Err
	}
	return fmt.Errorf("%s: %w", p, err)
}
