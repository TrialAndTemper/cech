// Package gitenv gives the environment cech runs git in: the user's, less
// the variables that point git at another repository, which a git hook
// or git rebase --exec sets, so git always works on the repository in
// its working folder, and never prompts for credentials.
package gitenv

import (
	"os"
	"slices"
	"strings"
)

// repositoryVars are the variables that point git at a repository other
// than the one in its working folder, as git rev-parse --local-env-vars
// lists them.
var repositoryVars = []string{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
	"GIT_OBJECT_DIRECTORY", "GIT_DIR", "GIT_WORK_TREE", "GIT_IMPLICIT_WORK_TREE", "GIT_GRAFT_FILE",
	"GIT_INDEX_FILE", "GIT_NO_REPLACE_OBJECTS", "GIT_REPLACE_REF_BASE", "GIT_PREFIX",
	"GIT_SHALLOW_FILE", "GIT_COMMON_DIR",
}

// Env returns the environment of this process without repositoryVars,
// with git's credential prompt turned off and extra added.
func Env(extra ...string) []string {
	env := slices.DeleteFunc(os.Environ(), locatesRepository)
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	return append(env, extra...)
}

// locatesRepository reports whether kv, an environment entry NAME=value,
// sets one of repositoryVars; names match in any case, as on Windows.
func locatesRepository(kv string) bool {
	name, _, _ := strings.Cut(kv, "=")
	return slices.ContainsFunc(repositoryVars, func(v string) bool { return strings.EqualFold(v, name) })
}
