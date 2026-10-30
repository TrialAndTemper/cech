package evals

import (
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/TrialAndTemper/cech/cli/internal/frontmatter"
)

// The grader types of claude plugin eval.
const (
	typeRegex      = "regex"
	typeToolUsed   = "tool_used"
	typeToolOrder  = "tool_order"
	typeFileExists = "file_exists"
	typeLLM        = "llm"
	typeBaseline   = "baseline"
)

// graderKeys are the keys each grader type takes, besides type, weight
// and arm (spec rule 10).
var graderKeys = map[string][]string{
	typeRegex:      {"pattern", "flags", "match", "target"},
	typeToolUsed:   {"tool", "input_match", "min", "max"},
	typeToolOrder:  {"before", "after"},
	typeFileExists: {"path", "exists"},
	typeLLM:        {"criteria", "focus"},
	typeBaseline:   {"baseline_file", "criteria"},
}

// The targets a regex grader reads offline.
const (
	targetLastMessage = "last_message"
	targetFiles       = "files"
	targetFile        = "file"
)

// toolMatch picks tool_use blocks by tool name and, optionally, by a
// pattern over their JSON input.
type toolMatch struct {
	tool  string
	input *regexp.Regexp
}

// matches reports whether the tool_use block u is one toolMatch picks.
func (m toolMatch) matches(u toolUse) bool {
	return u.name == m.tool && (m.input == nil || m.input.MatchString(u.input))
}

// grader is one grader file. Only the fields of its type are set.
type grader struct {
	name string
	typ  string

	// regex
	re       *regexp.Regexp
	negate   bool
	count    int // -1 unless match is count:N
	target   string
	filePath string // for target file

	// tool_used
	use      toolMatch
	min, max int // max -1 for unlimited

	// tool_order
	before, after toolMatch

	// file_exists
	glob   *regexp.Regexp
	exists bool
}

// deterministic reports whether the grader runs without a model.
func (g *grader) deterministic() bool {
	return g.typ != typeLLM && g.typ != typeBaseline
}

// appliesTo reports whether the grader grades a run on harness offline:
// regex and file_exists graders read the result on both harnesses, and
// tool graders name Claude Code tools (spec rule 12).
func (g *grader) appliesTo(harness string) bool {
	switch g.typ {
	case typeRegex, typeFileExists:
		return true
	case typeToolUsed, typeToolOrder:
		return harness == harnessClaude
	}
	return false
}

// parseGrader reads the grader file name. It returns every problem it
// finds, so a finding names them all.
func parseGrader(name string, src []byte) (*grader, []string) {
	doc, err := frontmatter.Parse(src)
	if err != nil {
		return nil, []string{err.Error()}
	}
	g := &grader{name: name, count: -1, max: -1, min: 1, target: targetLastMessage, exists: true}
	var probs []string
	bad := func(format string, a ...any) { probs = append(probs, fmt.Sprintf(format, a...)) }

	typ, err := doc.Str("type")
	switch {
	case err != nil:
		return nil, []string{err.Error()}
	case typ == "":
		return nil, []string{"type is missing"}
	}
	keys, ok := graderKeys[typ]
	if !ok {
		return nil, []string{fmt.Sprintf("type %q is not regex, tool_used, tool_order, file_exists, llm or baseline", typ)}
	}
	g.typ = typ
	for _, k := range doc.Keys {
		if k != "type" && k != "weight" && k != "arm" && !slices.Contains(keys, k) {
			bad("%s grader takes no key %q", typ, k)
		}
	}
	if n, ok := doc.Fields["weight"]; ok {
		if w, err := strconv.ParseFloat(n.Value, 64); n.Kind != yaml.ScalarNode || (n.ShortTag() != "!!int" && n.ShortTag() != "!!float") || err != nil || w <= 0 {
			bad("weight is not a positive number")
		}
	}
	if arm, err := doc.Str("arm"); err != nil {
		bad("%v", err)
	} else if _, ok := doc.Fields["arm"]; ok && arm != "with-only" && arm != "both" {
		bad("arm %q is not with-only or both", arm)
	}

	switch typ {
	case typeRegex:
		g.parseRegex(doc, bad)
	case typeToolUsed:
		g.parseToolUsed(doc, bad)
	case typeToolOrder:
		var err error
		if g.before, err = parseToolMatch(doc.Fields["before"]); err != nil {
			bad("before: %v", err)
		}
		if g.after, err = parseToolMatch(doc.Fields["after"]); err != nil {
			bad("after: %v", err)
		}
	case typeFileExists:
		g.parseFileExists(doc, bad)
	case typeLLM:
		criteria, err := doc.Str("criteria")
		switch {
		case err != nil:
			bad("%v", err)
		case criteria != "" && strings.TrimSpace(doc.Body) != "":
			bad("llm grader has criteria and a body; give one")
		case criteria == "" && strings.TrimSpace(doc.Body) == "":
			bad("llm grader has no criteria and no body")
		}
		if n, ok := doc.Fields["focus"]; ok {
			if _, _, err := parseTarget(n); err != nil {
				bad("focus: %v", err)
			}
		}
	case typeBaseline:
		if f, err := doc.Str("baseline_file"); err != nil {
			bad("%v", err)
		} else if f == "" {
			bad("baseline grader has no baseline_file")
		}
		if _, err := doc.Str("criteria"); err != nil {
			bad("%v", err)
		}
	}
	if len(probs) > 0 {
		return nil, probs
	}
	return g, nil
}

func (g *grader) parseRegex(doc *frontmatter.Doc, bad func(string, ...any)) {
	pattern, perr := doc.Str("pattern")
	flags, ferr := doc.Str("flags")
	switch {
	case perr != nil:
		bad("%v", perr)
	case pattern == "":
		bad("regex grader has no pattern")
	}
	if ferr != nil {
		bad("%v", ferr)
	}
	if perr == nil && ferr == nil && pattern != "" {
		var err error
		if g.re, err = compile(pattern, flags); err != nil {
			bad("%v", err)
		}
	}
	match, err := doc.Str("match")
	switch {
	case err != nil:
		bad("%v", err)
	case match == "" || match == "contains":
	case match == "not_contains":
		g.negate = true
	case strings.HasPrefix(match, "count:"):
		n, aerr := strconv.Atoi(strings.TrimPrefix(match, "count:"))
		if aerr != nil || n < 0 || !wholeNumber.MatchString(strings.TrimPrefix(match, "count:")) {
			bad("match %q is not count:N with N a whole number", match)
			break
		}
		g.count = n
		if g.re != nil && canMatchEmpty(g.re) {
			bad("pattern %q can match the empty string, which JavaScript and Go count differently under count:N", pattern)
		}
	default:
		bad("match %q is not contains, not_contains or count:N", match)
	}
	if n, ok := doc.Fields["target"]; ok {
		if g.target, g.filePath, err = parseTarget(n); err != nil {
			bad("target: %v", err)
		}
	}
}

var wholeNumber = regexp.MustCompile(`^[0-9]+$`)

// parseTarget reads a target or focus: last_message, files, or a map
// {source: file, path: P} with a relative P. trace and mock_calls are
// refused (spec rule 11).
func parseTarget(n *yaml.Node) (target, filePath string, err error) {
	if n.Kind == yaml.ScalarNode && n.ShortTag() == "!!str" {
		switch n.Value {
		case targetLastMessage, targetFiles:
			return n.Value, "", nil
		case "trace":
			return "", "", errors.New("trace is not allowed: a live trace holds lines and paths that a recording leaves out")
		case "mock_calls":
			return "", "", errors.New("mock_calls is not allowed: CECH's eval cases have no mocks")
		}
		return "", "", fmt.Errorf("%q is not last_message, files or {source: file, path: <path>}", n.Value)
	}
	fields, keys, err := frontmatter.Mapping(n)
	if err != nil {
		return "", "", fmt.Errorf("not last_message, files or {source: file, path: <path>}")
	}
	for _, k := range keys {
		if k != "source" && k != "path" {
			return "", "", fmt.Errorf("takes no key %q", k)
		}
	}
	src, p := fields["source"], fields["path"]
	if src == nil || src.Value != "file" || p == nil || p.ShortTag() != "!!str" || p.Value == "" {
		return "", "", errors.New("a map must be {source: file, path: <path>}")
	}
	if !isRelative(p.Value) {
		return "", "", fmt.Errorf("path %q is not a relative path inside the workspace", p.Value)
	}
	return targetFile, p.Value, nil
}

func (g *grader) parseToolUsed(doc *frontmatter.Doc, bad func(string, ...any)) {
	tool, err := doc.Str("tool")
	switch {
	case err != nil:
		bad("%v", err)
	case tool == "":
		bad("tool_used grader has no tool")
	}
	g.use.tool = tool
	if in, err := doc.Str("input_match"); err != nil {
		bad("%v", err)
	} else if in != "" {
		if g.use.input, err = compile(in, ""); err != nil {
			bad("input_match: %v", err)
		}
	}
	valid := true
	for _, key := range []string{"min", "max"} {
		n, ok := doc.Fields[key]
		if !ok {
			continue
		}
		v, err := strconv.Atoi(n.Value)
		if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!int" || err != nil || v < 0 {
			bad("%s is not a whole number", key)
			valid = false
			continue
		}
		if key == "min" {
			g.min = v
		} else {
			g.max = v
		}
	}
	if valid && g.max >= 0 && g.min > g.max {
		bad("min %d is above max %d", g.min, g.max)
	}
}

// parseToolMatch reads a tool_order end: a tool name, or a map of tool
// and an optional input_match.
func parseToolMatch(n *yaml.Node) (toolMatch, error) {
	if n == nil {
		return toolMatch{}, errors.New("missing")
	}
	if n.Kind == yaml.ScalarNode && n.ShortTag() == "!!str" && n.Value != "" {
		return toolMatch{tool: n.Value}, nil
	}
	fields, keys, err := frontmatter.Mapping(n)
	if err != nil {
		return toolMatch{}, errors.New("not a tool name or a map of tool and input_match")
	}
	for _, k := range keys {
		if k != "tool" && k != "input_match" {
			return toolMatch{}, fmt.Errorf("takes no key %q", k)
		}
	}
	t := fields["tool"]
	if t == nil || t.ShortTag() != "!!str" || t.Value == "" {
		return toolMatch{}, errors.New("has no tool")
	}
	m := toolMatch{tool: t.Value}
	if in := fields["input_match"]; in != nil {
		if in.ShortTag() != "!!str" {
			return toolMatch{}, errors.New("input_match is not a string")
		}
		if m.input, err = compile(in.Value, ""); err != nil {
			return toolMatch{}, fmt.Errorf("input_match: %w", err)
		}
	}
	return m, nil
}

func (g *grader) parseFileExists(doc *frontmatter.Doc, bad func(string, ...any)) {
	p, err := doc.Str("path")
	switch {
	case err != nil:
		bad("%v", err)
	case p == "":
		bad("file_exists grader has no path")
	case !isRelative(p):
		bad("path %q is not a relative glob inside the workspace", p)
	case strings.ContainsAny(p, "[]{}!"):
		bad("path %q uses a glob character other than *, ? and **, which claude plugin eval may read differently", p)
	default:
		g.glob = globRegexp(p)
	}
	if n, ok := doc.Fields["exists"]; ok {
		if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!bool" {
			bad("exists is not a boolean")
		} else {
			g.exists = n.Value == "true"
		}
	}
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

// canMatchEmpty reports whether re can match the empty string at some
// position, as an anchor or a starred group can.
func canMatchEmpty(re *regexp.Regexp) bool {
	s, err := syntax.Parse(re.String(), syntax.Perl)
	if err != nil {
		return true
	}
	return minLen(s) == 0
}

// minLen is the fewest characters a match of s takes.
func minLen(s *syntax.Regexp) int {
	switch s.Op {
	case syntax.OpLiteral:
		return len(s.Rune)
	case syntax.OpCharClass, syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		return 1
	case syntax.OpCapture, syntax.OpPlus:
		return minLen(s.Sub[0])
	case syntax.OpRepeat:
		return s.Min * minLen(s.Sub[0])
	case syntax.OpConcat:
		n := 0
		for _, sub := range s.Sub {
			n += minLen(sub)
		}
		return n
	case syntax.OpAlternate:
		n := -1
		for _, sub := range s.Sub {
			if m := minLen(sub); n < 0 || m < n {
				n = m
			}
		}
		return max(n, 0)
	case syntax.OpNoMatch, syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine,
		syntax.OpBeginText, syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary,
		syntax.OpStar, syntax.OpQuest:
		return 0
	}
	return 0
}

// grade reports whether the grader passes on the recording r.
func (g *grader) grade(r *recording) bool {
	switch g.typ {
	case typeRegex:
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
		switch {
		case g.count >= 0:
			return len(g.re.FindAllStringIndex(text, -1)) == g.count
		case g.negate:
			return !g.re.MatchString(text)
		default:
			return g.re.MatchString(text)
		}
	case typeToolUsed:
		n := 0
		for _, u := range r.toolUses {
			if g.use.matches(u) {
				n++
			}
		}
		return n >= g.min && (g.max < 0 || n <= g.max)
	case typeToolOrder:
		b := slices.IndexFunc(r.toolUses, g.before.matches)
		a := slices.IndexFunc(r.toolUses, g.after.matches)
		return b >= 0 && a >= 0 && b < a
	case typeFileExists:
		found := slices.ContainsFunc(r.created, g.glob.MatchString)
		return found == g.exists
	}
	return false
}
