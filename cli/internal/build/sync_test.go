package build

import (
	"io/fs"
	"os"
	"testing"
)

// TestFileError checks that an error names the file by its slash path,
// not the path the system gave, for each kind of error that carries one.
func TestFileError(t *testing.T) {
	for _, err := range []error{
		&fs.PathError{Op: "open", Path: "/tmp/x/docs/.catalogue.md.tmp", Err: fs.ErrPermission},
		&os.LinkError{Op: "rename", Old: "/tmp/x/a", New: "/tmp/x/b", Err: fs.ErrPermission},
	} {
		got := fileError("docs/catalogue.md", err)
		if want := "docs/catalogue.md: permission denied"; got.Error() != want {
			t.Errorf("fileError(%v) = %q, want %q", err, got, want)
		}
	}
}
