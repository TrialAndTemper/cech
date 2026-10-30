package evals

import (
	"fmt"
	"regexp"
	"strings"
)

// The graders of claude plugin eval match JavaScript regular
// expressions, and cech eval --check matches them with Go's RE2. compile
// accepts only a subset that both read alike, and rewrites the two
// pieces whose meaning differs: JavaScript's . also stops at \r, U+2028
// and U+2029, and its \s also matches \v and Unicode spaces.
const (
	jsDot    = `[^\n\r\x{2028}\x{2029}]`
	jsSpace  = `\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`
	escapes  = `\^$.|?*+()[]{}/`
	classEsc = `\^$.|?*+()[]{}/-`
)

// compile checks that pattern, with JavaScript flags, keeps to the
// subset, and returns its Go equivalent.
func compile(pattern, flags string) (*regexp.Regexp, error) {
	var b strings.Builder
	dotAll := false
	for i, f := range flags {
		switch {
		case strings.ContainsRune(flags[:i], f):
			return nil, fmt.Errorf("flag %q is given twice", f)
		case f == 'i':
			b.WriteString("(?i)")
		case f == 's':
			dotAll = true
		default:
			return nil, fmt.Errorf("flag %q is not i or s", f)
		}
	}
	rs := []rune(pattern)
	inClass := false
	// quantifiable is whether the last piece was one a quantifier may
	// follow: JavaScript rejects one at the start of a pattern, group or
	// alternative, or after ^, $, \b or \B, where Go accepts it.
	// lazyOK is whether the last piece was a quantifier, which one lazy ?
	// may follow.
	quantifiable, lazyOK := false, false
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		lazy := !inClass && r == '?' && !quantifiable && lazyOK
		if !inClass && strings.ContainsRune("*+?{", r) && !quantifiable && !lazy {
			return nil, fmt.Errorf("pattern %q has %c with nothing to repeat, which JavaScript rejects", pattern, r)
		}
		next, nextLazyOK := true, false
		switch {
		case r == '\\':
			if i+1 == len(rs) {
				return nil, fmt.Errorf("pattern %q ends in a lone backslash", pattern)
			}
			i++
			e := rs[i]
			switch {
			case e == 's' || e == 'S':
				switch {
				case inClass && e == 's' && i+2 < len(rs) && rs[i+1] == '-' && rs[i+2] != ']':
					return nil, fmt.Errorf("pattern %q has \\s followed by - in a bracket class; write \\- for the character", pattern)
				case inClass && e == 's':
					b.WriteString(jsSpace)
				case inClass:
					return nil, fmt.Errorf("pattern %q has \\S inside a bracket class", pattern)
				case e == 's':
					b.WriteString("[" + jsSpace + "]")
				default:
					b.WriteString("[^" + jsSpace + "]")
				}
			case !inClass && (e == 'b' || e == 'B'):
				b.WriteRune('\\')
				b.WriteRune(e)
				next = false
			case strings.ContainsRune("dDwWntr", e),
				inClass && strings.ContainsRune(classEsc, e),
				!inClass && strings.ContainsRune(escapes, e):
				b.WriteRune('\\')
				b.WriteRune(e)
			default:
				return nil, fmt.Errorf("pattern %q has the escape \\%c, which JavaScript and Go read differently or not at all", pattern, e)
			}
		case inClass:
			switch r {
			case ']':
				inClass = false
			case '[':
				return nil, fmt.Errorf("pattern %q has an unescaped [ inside a bracket class", pattern)
			}
			b.WriteRune(r)
		case r == '[':
			inClass = true
			b.WriteRune(r)
			if i+1 < len(rs) && rs[i+1] == '^' {
				i++
				b.WriteRune('^')
			}
			if i+1 < len(rs) && rs[i+1] == ']' {
				return nil, fmt.Errorf("pattern %q has a bracket class that starts with ]", pattern)
			}
		case r == '(':
			if i+1 < len(rs) && rs[i+1] == '?' {
				if i+2 >= len(rs) || rs[i+2] != ':' {
					return nil, fmt.Errorf("pattern %q has a group (?…) other than (?:…): lookaround, named groups and inline flags read differently or not at all", pattern)
				}
				b.WriteString("(?:")
				i += 2
			} else {
				b.WriteRune(r)
			}
			next = false
		case r == '|' || r == '^' || r == '$':
			b.WriteRune(r)
			next = false
		case r == '{':
			n, ok := repeat(rs[i:])
			if !ok {
				return nil, fmt.Errorf("pattern %q has a { that is not a repeat {n}, {n,} or {n,m}; write \\{ for the character", pattern)
			}
			b.WriteString(string(rs[i : i+n]))
			i += n - 1
			next, nextLazyOK = false, true
		case r == '*' || r == '+' || r == '?':
			// A quantifier, or a lazy ? after one; nothing may repeat it,
			// and only a quantifier may take a lazy ?.
			b.WriteRune(r)
			next, nextLazyOK = false, !lazy
		case r == '}' || r == ']':
			return nil, fmt.Errorf("pattern %q has an unescaped %c; write \\%c", pattern, r, r)
		case r == '.' && !dotAll:
			b.WriteString(jsDot)
		case r == '.':
			b.WriteString(`[\s\S]`)
		default:
			b.WriteRune(r)
		}
		if !inClass {
			quantifiable, lazyOK = next, nextLazyOK
		}
	}
	if inClass {
		return nil, fmt.Errorf("pattern %q has a bracket class with no closing ]", pattern)
	}
	re, err := regexp.Compile(b.String())
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

// repeat returns the length of the repeat {n}, {n,} or {n,m} at the
// start of rs.
func repeat(rs []rune) (int, bool) {
	end := -1
	for i, r := range rs {
		if r == '}' {
			end = i
			break
		}
	}
	if end < 0 {
		return 0, false
	}
	return end + 1, repeatForm.MatchString(string(rs[:end+1]))
}

var repeatForm = regexp.MustCompile(`^\{[0-9]+(,[0-9]*)?\}$`)

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
