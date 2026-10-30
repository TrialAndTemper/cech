package terms

import (
	"bytes"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// ruleFile holds the keys a rule file may use; decoding
// leaves any other key undecoded, so it is reported. extend is allowed
// only in generic.toml.
type ruleFile struct {
	Title      string      `toml:"title,omitempty"`
	Extend     *extend     `toml:"extend,omitempty"`
	Allowlists []allowlist `toml:"allowlists,omitempty"`
	Rules      []rule      `toml:"rules,omitempty"`
}

type extend struct {
	UseDefault bool `toml:"useDefault"`
}

type rule struct {
	ID          string      `toml:"id"`
	Description string      `toml:"description,omitempty"`
	Regex       string      `toml:"regex,omitempty"`
	Keywords    []string    `toml:"keywords,omitempty"`
	Path        string      `toml:"path,omitempty"`
	Entropy     float64     `toml:"entropy,omitempty"`
	SecretGroup int         `toml:"secretGroup,omitempty"`
	Tags        []string    `toml:"tags,omitempty"`
	Allowlists  []allowlist `toml:"allowlists,omitempty"`
}

type allowlist struct {
	Description string   `toml:"description,omitempty"`
	Condition   string   `toml:"condition,omitempty"`
	RegexTarget string   `toml:"regexTarget,omitempty"`
	Regexes     []string `toml:"regexes,omitempty"`
	Paths       []string `toml:"paths,omitempty"`
	Stopwords   []string `toml:"stopwords,omitempty"`
}

// decodeRuleFile decodes data and lists the keys it does not allow.
func decodeRuleFile(data []byte) (ruleFile, []string, error) {
	var rf ruleFile
	md, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&rf)
	if err != nil {
		return rf, nil, err
	}
	var bad []string
	for _, k := range md.Undecoded() {
		bad = append(bad, k.String())
	}
	return rf, bad, nil
}

// checkRuleFile checks one extension rule file: its keys, ids and
// regexes.
// taken holds the ids already in use, and gets this file's.
func checkRuleFile(name string, rf ruleFile, bad []string, ns string, taken map[string]bool) []Problem {
	var ps []Problem
	add := func(format string, a ...any) {
		ps = append(ps, Problem{ProblemRuleFile, name + ": " + fmt.Sprintf(format, a...)})
	}
	for _, k := range bad {
		add("key %s is not allowed", k)
	}
	if rf.Extend != nil {
		add("key extend is not allowed")
	}
	for _, a := range rf.Allowlists {
		for _, msg := range checkAllowlist(a) {
			add("allowlist: %s", msg)
		}
	}
	for i, r := range rf.Rules {
		id := r.ID
		switch {
		case id == "":
			add("rule %d has no id", i+1)
			id = fmt.Sprintf("rule %d", i+1)
		case !strings.HasPrefix(id, ns+"-"):
			add("rule id %s does not start with %s-", id, ns)
		case strings.HasPrefix(id, "name-") || strings.HasPrefix(id, privatePrefix):
			add("rule id %s starts with a prefix CECH reserves", id)
		case taken[id]:
			add("rule id %s is already used", id)
		}
		taken[r.ID] = true
		if r.Regex == "" && r.Path == "" {
			add("rule %s has neither regex nor path", id)
		}
		for _, f := range []struct{ key, re string }{{"regex", r.Regex}, {"path", r.Path}} {
			key, re := f.key, f.re
			if re == "" {
				continue
			}
			c, err := regexp.Compile(re)
			if err != nil {
				add("rule %s: %s does not compile: %v", id, key, err)
				continue
			}
			if key == "regex" && r.SecretGroup > c.NumSubexp() {
				add("rule %s: secretGroup %d is more than the regex's %d groups", id, r.SecretGroup, c.NumSubexp())
			}
		}
		if r.SecretGroup < 0 {
			add("rule %s: secretGroup is negative", id)
		}
		for _, a := range r.Allowlists {
			for _, msg := range checkAllowlist(a) {
				add("rule %s: allowlist: %s", id, msg)
			}
		}
	}
	return ps
}

func checkAllowlist(a allowlist) []string {
	var msgs []string
	if len(a.Regexes) == 0 && len(a.Paths) == 0 && len(a.Stopwords) == 0 {
		msgs = append(msgs, "it has no regexes, paths or stopwords")
	}
	switch a.Condition {
	case "", "OR", "AND":
	default:
		msgs = append(msgs, fmt.Sprintf("condition %q is not OR or AND", a.Condition))
	}
	switch a.RegexTarget {
	case "", "secret", "match", "line":
	default:
		msgs = append(msgs, fmt.Sprintf("regexTarget %q is not secret, match or line", a.RegexTarget))
	}
	for _, re := range append(append([]string{}, a.Regexes...), a.Paths...) {
		if _, err := regexp.Compile(re); err != nil {
			msgs = append(msgs, fmt.Sprintf("%q does not compile: %v", re, err))
		}
	}
	return msgs
}

// nameRules turns a file under terms.names into rules; k is
// the file's place in terms.names, from 1. A name of separators alone
// would match everywhere, so it is a problem.
func nameRules(name string, data []byte, k int) ([]rule, []Problem) {
	var rs []rule
	var ps []Problem
	for n, line := range strings.Split(string(data), "\n") {
		v := strings.TrimSpace(strings.ReplaceAll(line, "\r", ""))
		if v == "" || strings.HasPrefix(v, "#") {
			continue
		}
		if len(strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(v))) == 0 {
			ps = append(ps, Problem{ProblemNameFile, fmt.Sprintf("%s: line %d holds no word", name, n+1)})
			continue
		}
		rs = append(rs, rule{
			ID:          fmt.Sprintf("name-%d-%d", k, n+1),
			Description: fmt.Sprintf("Name on line %d of %s", n+1, name),
			Regex:       nameRegex(v),
		})
	}
	return rs, ps
}

// nameRegex matches a name as CECH's gate does: without regard to case;
// spaces, - and _ between words match each other or nothing; and a word
// boundary at an end that is a word character.
func nameRegex(name string) string {
	words := strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(name))
	for i, w := range words {
		words[i] = regexp.QuoteMeta(w)
	}
	re := "(?i)"
	if isWordByte(name[0]) {
		re += `\b`
	}
	re += strings.Join(words, `[\s_-]*`)
	if isWordByte(name[len(name)-1]) {
		re += `\b`
	}
	return re
}

func isWordByte(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// checkEntry checks one entry of terms.rules or terms.names
// and returns its slash-separated path.
func checkEntry(repo *repo, key, entry string, seen map[string]bool) (string, *Problem) {
	bad := func(why string) (string, *Problem) {
		return "", &Problem{ProblemEntry, fmt.Sprintf("terms.%s entry %q %s", key, entry, why)}
	}
	p := filepath.ToSlash(entry)
	switch {
	case p == "" || path.IsAbs(p) || filepath.IsAbs(entry) || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../"):
		return bad("is not a clean path from the repository root")
	case seen[p]:
		return bad("appears twice")
	case !repo.tracked[p]:
		return bad("is not a tracked file")
	}
	seen[p] = true
	fi, err := repo.fs.Lstat(filepath.FromSlash(p))
	if err != nil || !fi.Mode().IsRegular() {
		return bad("is not a regular file")
	}
	return p, nil
}
