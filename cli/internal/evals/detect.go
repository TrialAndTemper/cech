package evals

import (
	"encoding/json/v2"
	"strings"
)

// skillID is what the load detectors look for: the plugin's one skill,
// by name, and the first line of its description as SKILL.md writes it.
type skillID struct {
	plugin, name, descLine string
}

// descriptionLine returns the first line of the description value in
// the frontmatter of the SKILL.md src, as the file writes it, quotes
// included; for a block scalar, its first content line. It returns ""
// when there is none.
func descriptionLine(src string) string {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for i := 1; i < len(lines) && strings.TrimSpace(lines[i]) != "---"; i++ {
		rest, ok := strings.CutPrefix(lines[i], "description:")
		if !ok {
			continue
		}
		v := strings.TrimSpace(rest)
		if v != "" && v[0] != '|' && v[0] != '>' {
			return v
		}
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) == "---" {
				return ""
			}
			if t := strings.TrimSpace(next); t != "" {
				return t
			}
		}
		return ""
	}
	return ""
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
// name line and the first line of its description, each at the end of a
// line, after any prefix such as grep -n adds that does not end in a
// letter or digit. The command's exit status
// plays no part, since a command can read the file and then fail.
func codexLoads(outputs []string, s skillID) bool {
	if s.descLine == "" {
		return false
	}
	for _, out := range outputs {
		var name, desc bool
		for line := range strings.SplitSeq(out, "\n") {
			line = strings.TrimRight(line, " \t\r")
			name = name || endsWith(line, "name: "+s.name)
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
