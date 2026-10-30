package evals

import (
	"encoding/json/v2"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/TrialAndTemper/cech/cli/internal/frontmatter"
)

// skillID is what the load detectors look for: the plugin's one skill,
// by name, and its name line and the first line of its description as
// SKILL.md writes them.
type skillID struct {
	plugin, name, nameLine, descLine string
}

// skillLines returns the name line and the first line of the
// description in the frontmatter of the SKILL.md src, as the file writes
// them, quotes included: the name line from the name key on, and the
// description from its value on or, for a block scalar, its first
// content line. The YAML parser finds both, so a key in any form YAML
// allows is found. Each is "" when there is none.
func skillLines(src string) (name, desc string) {
	// The plugin check already reports a SKILL.md that does not parse.
	fm, err := frontmatter.Parse([]byte(src))
	if err != nil {
		return "", ""
	}
	// Parse reads CRLF as LF, so the lines are cut from the same text.
	lines := yamlLines(strings.ReplaceAll(src, "\r\n", "\n"))
	// from returns the file's text from the node n on to the end of its
	// line. Line 0 of lines is the opening ---, so n.Line indexes them.
	// The column counts characters, not bytes.
	from := func(n *yaml.Node) string {
		if n.Line >= len(lines) {
			return ""
		}
		line := []rune(lines[n.Line])
		return strings.TrimSpace(string(line[min(n.Column-1, len(line)):]))
	}
	if k, v := fm.KeyNodes["name"], fm.Fields["name"]; k != nil && v.Value != "" {
		name = from(k)
	}
	switch v := fm.Fields["description"]; {
	case v == nil || v.Value == "":
	case v.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0:
		for _, line := range lines[min(v.Line+1, len(lines)):] {
			if t := strings.TrimSpace(line); t != "" {
				desc = t
				break
			}
		}
	default:
		desc = from(v)
	}
	return name, desc
}

// yamlLines splits src into lines at each line break YAML reads: CRLF,
// LF, CR, U+0085, U+2028 and U+2029, so a node's line number indexes
// them.
func yamlLines(src string) []string {
	var lines []string
	start := 0
	for i, r := range src {
		switch r {
		case '\n', '\u0085', '\u2028', '\u2029':
		case '\r':
			if strings.HasPrefix(src[i+1:], "\n") {
				continue
			}
		default:
			continue
		}
		lines = append(lines, strings.TrimSuffix(src[start:i], "\r"))
		start = i + utf8.RuneLen(r)
	}
	return append(lines, src[start:])
}

// loads reports whether the recording r shows the skill s loaded, as the
// runner's detector for its harness reads a run (spec rule 24). It works
// around the harness gap codex-skill-load-event.
func (r *recording) loads(s skillID) bool {
	if r.harness == harnessCodex {
		return codexLoads(r.outputs, s)
	}
	for _, u := range r.toolUses {
		if u.name != "Skill" || u.id == "" {
			continue
		}
		var in struct {
			Skill string `json:"skill"`
		}
		if json.Unmarshal([]byte(u.input), &in) != nil || (in.Skill != s.name && in.Skill != s.plugin+":"+s.name) {
			continue
		}
		if isError, ok := r.results[u.id]; ok && !isError {
			return true
		}
	}
	return false
}

// codexLoads reports whether one command's output holds the skill's
// name line and the first line of its description, as SKILL.md writes
// them, each at the end of a line, after any prefix such as grep -n
// adds that does not end in a letter or digit. The command's exit
// status plays no part, since a command can read the file and then fail.
func codexLoads(outputs []string, s skillID) bool {
	if s.nameLine == "" || s.descLine == "" {
		return false
	}
	for _, out := range outputs {
		var name, desc bool
		for line := range strings.SplitSeq(out, "\n") {
			line = strings.TrimRight(line, " \t\r")
			name = name || endsWith(line, s.nameLine)
			desc = desc || endsWith(line, s.descLine)
		}
		if name && desc {
			return true
		}
	}
	return false
}

// endsWith reports whether line ends with text after nothing, or after a
// prefix that does not end in a letter or digit, so rename: x is not
// name: x but 2:name: x is.
func endsWith(line, text string) bool {
	before, ok := strings.CutSuffix(line, text)
	if !ok || before == "" {
		return ok
	}
	c := before[len(before)-1]
	return (c < 'a' || 'z' < c) && (c < 'A' || 'Z' < c) && (c < '0' || '9' < c)
}
