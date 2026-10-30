package terms

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// privateList is the private list: one case-insensitive regex per
// line, with blank lines and # comments ignored.
type privateList struct {
	file ruleFile
	// res matches each term, for a check of text the engine does not
	// scan, such as an unresolved range.
	res []*regexp.Regexp
	// lines maps a rule id to the term's line.
	lines map[string]int
	// none says why no list was read; err why the list cannot be used.
	none string
	err  error
}

const privatePrefix = "private-term-"

// loadPrivate reads the list named by CECH_PRIVATE_TERMS. An error never
// quotes a line of the list, only its number.
func loadPrivate(name string) privateList {
	pl := privateList{lines: map[string]int{}}
	if name == "" {
		pl.none = "CECH_PRIVATE_TERMS is not set"
		return pl
	}
	data, err := os.ReadFile(name) //nolint:gosec // The user names the list.
	switch {
	case os.IsNotExist(err):
		pl.none = "the file CECH_PRIVATE_TERMS names does not exist"
		return pl
	case err != nil:
		pl.err = fmt.Errorf("cannot read the list CECH_PRIVATE_TERMS names")
		return pl
	}
	for n, line := range strings.Split(string(data), "\n") {
		term := strings.TrimSpace(strings.ReplaceAll(line, "\r", ""))
		if term == "" || strings.HasPrefix(term, "#") {
			continue
		}
		if strings.Contains(term, "'''") {
			pl.err = fmt.Errorf("line %d of the list holds ''' and cannot be used", n+1)
			return pl
		}
		re, err := regexp.Compile("(?i)" + term)
		if err != nil {
			pl.err = fmt.Errorf("line %d of the list does not compile", n+1)
			return pl
		}
		id := privatePrefix + strconv.Itoa(n+1)
		pl.lines[id] = n + 1
		pl.res = append(pl.res, re)
		pl.file.Rules = append(pl.file.Rules, rule{ID: id, Regex: "(?i)" + term})
	}
	if len(pl.res) == 0 {
		pl.none = "the list CECH_PRIVATE_TERMS names holds no terms"
	}
	return pl
}

func matchesAny(res []*regexp.Regexp, s string) bool {
	for _, re := range res {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
