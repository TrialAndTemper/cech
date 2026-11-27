package evalrun

import (
	"errors"
	"path/filepath"
	"testing"
)

// TestLock checks that a second hold on a commit's results fails with
// ErrLocked while the first lasts, and works once it is released
// (criterion 29).
func TestLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".lock")
	unlock, err := Lock(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, lerr := Lock(path); !errors.Is(lerr, ErrLocked) {
		t.Errorf("second Lock: %v; want ErrLocked", lerr)
	}
	unlock()
	again, err := Lock(path)
	if err != nil {
		t.Fatalf("Lock after release: %v", err)
	}
	again()
}
