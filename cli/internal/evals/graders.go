package evals

import (
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
)

// The grader types.
const (
	typeRegex      = "regex"
	typeFileExists = "file_exists"
)

// graderKeys are the keys each grader type takes, besides name, type,
// weight and arm (spec rule 10).
var graderKeys = map[string][]string{
	typeRegex:      {"pattern", "match", "count", "target", "path"},
	typeFileExists: {"path", "exists"},
}

// The targets a regex grader reads.
const (
	targetLastMessage = "last_message"
	targetFiles       = "files"
	targetFile        = "file"
)

// graderName is the form of a grader's name.
var graderName = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// The ways a regex grader matches its target.
const (
	matchContains    = "contains"
	matchNotContains = "not_contains"
	matchCount       = "count"
)

// grader is one [[graders]] table of a case's cech.toml. Only the fields
// of its type are set.
type grader struct {
	name string
	typ  string

	// regex
	re       *regexp.Regexp
	match    string
	count    int // for match count
	target   string
	filePath string // for target file

	// file_exists
	glob   *regexp.Regexp
	exists bool
}

// parseGrader reads the [[graders]] table fields, the i-th of its case,
// counting from 1. It returns the grader's name, or #i when it has no
// valid one, and every problem it finds, so a finding names them all.
func parseGrader(i int, fields map[string]any) (*grader, string, []string) {
	name := fmt.Sprintf("#%d", i)
	var probs []string
	bad := func(format string, a ...any) { probs = append(probs, fmt.Sprintf(format, a...)) }
	// str returns the string at key, whether the key is present, and
	// whether its value is a string; one that is not is a problem.
	str := func(key string) (s string, has, ok bool) {
		v, has := fields[key]
		if !has {
			return "", false, false
		}
		s, ok = v.(string)
		if !ok {
			bad("%s is not a string", key)
		}
		return s, true, ok
	}

	switch n, has, ok := str("name"); {
	case ok && graderName.MatchString(n):
		name = n
	case ok || !has:
		bad("has no name of lowercase letters and digits in words joined by single hyphens")
	}
	typ, hasType, typeOK := str("type")
	keys, known := graderKeys[typ]
	switch {
	case !hasType:
		bad("type is missing")
	case typeOK && !known:
		bad("type %q is not regex or file_exists", typ)
	}
	g := &grader{name: name, typ: typ, match: matchContains, target: targetLastMessage, exists: true}
	if known {
		for _, k := range slices.Sorted(maps.Keys(fields)) {
			if k != "name" && k != "type" && k != "weight" && k != "arm" && !slices.Contains(keys, k) {
				bad("takes no key %q for type %s", k, typ)
			}
		}
	}
	if w, has := fields["weight"]; has {
		var f float64
		switch w := w.(type) {
		case int64:
			f = float64(w)
		case float64:
			f = w
		}
		if !(f > 0) || math.IsInf(f, 0) {
			bad("weight is not a finite positive number")
		}
	}
	if arm, _, ok := str("arm"); ok && arm != "with-only" && arm != "both" {
		bad("arm %q is not with-only or both", arm)
	}
	switch typ {
	case typeRegex:
		g.parseRegex(fields, str, bad)
	case typeFileExists:
		g.parseFileExists(fields, str, bad)
	}
	if len(probs) > 0 {
		return nil, name, probs
	}
	return g, name, nil
}

func (g *grader) parseRegex(fields map[string]any, str func(string) (string, bool, bool), bad func(string, ...any)) {
	if v, has := fields["pattern"]; !has || v == "" {
		bad("has no pattern")
	} else if pattern, _, ok := str("pattern"); ok {
		re, err := compile(pattern)
		if err != nil {
			bad("%v", err)
		}
		g.re = re
	}
	// A match or target that is not a string is reported once, so the
	// keys that depend on it are not checked.
	if match, has, ok := str("match"); !has || ok {
		count, hasCount := fields["count"]
		switch match {
		case "", matchContains, matchNotContains:
			if match != "" {
				g.match = match
			}
			if hasCount {
				bad("count is only for match = \"count\"")
			}
		case matchCount:
			g.match = matchCount
			n, ok := count.(int64)
			switch {
			case !hasCount:
				bad("match = \"count\" has no count")
			case !ok || n < 0:
				bad("count is not a whole number")
			default:
				g.count = int(n)
			}
		default:
			bad("match %q is not contains, not_contains or count", match)
		}
	}
	target, hasTarget, targetOK := str("target")
	if hasTarget && !targetOK {
		return
	}
	pathValue, hasPath := fields["path"]
	switch target {
	case "", targetLastMessage, targetFiles:
		if target != "" {
			g.target = target
		}
		if hasPath {
			bad("path is only for target = \"file\"")
		}
	case targetFile:
		g.target = targetFile
		p, _, ok := str("path")
		switch {
		case !hasPath || pathValue == "":
			bad("target = \"file\" has no path")
		case !ok:
		case !isRelative(p):
			bad("path %q is not a relative path inside the workspace", p)
		default:
			g.filePath = p
		}
	default:
		bad("target %q is not last_message, files or file", target)
	}
}

func (g *grader) parseFileExists(fields map[string]any, str func(string) (string, bool, bool), bad func(string, ...any)) {
	v := fields["path"]
	p, has, ok := str("path")
	switch {
	case !has || v == "":
		bad("has no path")
	case !ok:
	case !isRelative(p):
		bad("path %q is not a relative glob inside the workspace", p)
	case strings.ContainsAny(p, "[]{}!"):
		bad("path %q uses a glob character other than *, ? and **", p)
	default:
		g.glob = globRegexp(p)
	}
	if v, has := fields["exists"]; has {
		if b, isBool := v.(bool); isBool {
			g.exists = b
		} else {
			bad("exists is not a boolean")
		}
	}
}

// compile compiles the regex grader's pattern, in Go's syntax.
func compile(pattern string) (*regexp.Regexp, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("pattern %q: %w", pattern, err)
	}
	// A recording writes local paths as placeholders under /cech/, so a
	// pattern that names one would pass offline and fail on a live run.
	if strings.Contains(literal(pattern), placeholderRoot) {
		return nil, fmt.Errorf("pattern %q names %s, the root of the placeholders a recording writes for local paths, which a live run does not hold", pattern, placeholderRoot)
	}
	return re, nil
}

// literal returns pattern with each escape of a character that is not a
// letter or digit removed, so \/cech\/ reads as /cech/.
func literal(pattern string) string {
	var b strings.Builder
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		if c == '\\' && i+1 < len(pattern) {
			next := pattern[i+1]
			if (next < 'a' || 'z' < next) && (next < 'A' || 'Z' < next) && (next < '0' || '9' < next) {
				b.WriteByte(next)
				i++
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// isRelative reports whether p is a slash-separated path inside the
// workspace, in the form the run's created paths take: not absolute, and
// no part of it empty, . or .. .
func isRelative(p string) bool {
	if strings.Contains(p, `\`) || (len(p) > 1 && p[1] == ':') {
		return false
	}
	return !slices.ContainsFunc(strings.Split(p, "/"), func(part string) bool {
		return part == "" || part == "." || part == ".."
	})
}

// globRegexp turns a glob of *, ? and ** into a regular expression
// over a relative path: * and ? stay inside one folder, ** crosses
// folders.
func globRegexp(glob string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(glob); i++ {
		switch {
		case strings.HasPrefix(glob[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(glob[i:], "**"):
			b.WriteString(".*")
			i++
		case glob[i] == '*':
			b.WriteString("[^/]*")
		case glob[i] == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(glob[i : i+1]))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

// grade reports whether the grader passes on the recording r.
func (g *grader) grade(r *recording) bool {
	if g.typ == typeFileExists {
		return slices.ContainsFunc(r.created, g.glob.MatchString) == g.exists
	}
	var text string
	switch g.target {
	case targetLastMessage:
		text = r.lastMessage
	case targetFiles:
		text = strings.Join(r.created, "\n")
	case targetFile:
		content, ok := r.files[g.filePath]
		if !ok {
			return false
		}
		text = content
	}
	switch g.match {
	case matchCount:
		return len(g.re.FindAllStringIndex(text, -1)) == g.count
	case matchNotContains:
		return !g.re.MatchString(text)
	}
	return g.re.MatchString(text)
}
