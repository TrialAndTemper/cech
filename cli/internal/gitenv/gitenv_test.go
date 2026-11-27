package gitenv

import (
	"slices"
	"strings"
	"testing"
)

// TestEnv checks that the variables a git hook sets to point git at its
// repository are gone, in any case, and that others stay.
func TestEnv(t *testing.T) {
	t.Setenv("GIT_DIR", "/elsewhere/.git")
	t.Setenv("git_work_tree", "/elsewhere")
	t.Setenv("GIT_INDEX_FILE", "/elsewhere/index")
	t.Setenv("GIT_AUTHOR_NAME", "someone")
	env := Env("X=1")
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if strings.EqualFold(name, "GIT_DIR") || strings.EqualFold(name, "GIT_WORK_TREE") || name == "GIT_INDEX_FILE" {
			t.Errorf("Env kept %s", kv)
		}
	}
	for _, want := range []string{"GIT_AUTHOR_NAME=someone", "GIT_TERMINAL_PROMPT=0", "X=1"} {
		if !slices.Contains(env, want) {
			t.Errorf("Env lacks %s", want)
		}
	}
}
