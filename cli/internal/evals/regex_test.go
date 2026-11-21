package evals

import (
	"strings"
	"testing"
)

func TestCompile(t *testing.T) {
	tests := []struct {
		name, pattern, flags string
		match, noMatch       []string
		err                  string
	}{
		{name: "dot stops at CR as in JavaScript", pattern: `a.b`, match: []string{"a-b"}, noMatch: []string{"a\rb", "a\u2028b", "a\nb"}},
		{name: "dot with s matches line ends", pattern: `a.b`, flags: "s", match: []string{"a\nb", "a\rb"}},
		{name: "space class as in JavaScript", pattern: `a\sb`, match: []string{"a\vb", "a\u00a0b"}},
		{name: "not space", pattern: `a\Sb`, match: []string{"a-b"}, noMatch: []string{"a\u00a0b"}},
		{name: "space in a bracket class", pattern: `a[\s-]b`, match: []string{"a\u3000b", "a-b"}},
		{name: "case-insensitive", pattern: `fix`, flags: "i", match: []string{"FIX"}},
		{name: "JSON-escaped quote", pattern: `"skill"\s*:\s*"(?:[\w-]+:)?cech-x"`, match: []string{`"skill":"p:cech-x"`}},
		{name: "lookahead", pattern: `a(?=b)`, err: "other than (?:…)"},
		{name: "named group", pattern: `(?<n>a)`, err: "other than (?:…)"},
		{name: "inline flag", pattern: `(?i)a`, err: "other than (?:…)"},
		{name: "back-reference", pattern: `(a)\1`, err: `escape \1`},
		{name: "unicode property", pattern: `\p{L}`, err: `escape \p`},
		{name: "unicode escape", pattern: `\u0041`, err: `escape \u`},
		{name: "hex escape", pattern: `\x41`, err: `escape \x`},
		{name: "backspace in a class", pattern: `[\b]`, err: `escape \b`},
		{name: "not-space in a class", pattern: `[\S]`, err: `\S inside`},
		{name: "POSIX class", pattern: `[[:alpha:]]`, err: "unescaped [ inside"},
		{name: "lone brace", pattern: `a{`, err: "not a repeat"},
		{name: "unescaped closing brace", pattern: `a}`, err: "unescaped }"},
		{name: "unescaped closing bracket", pattern: `a]`, err: "unescaped ]"},
		{name: "class starting with ]", pattern: `[]a]`, err: "starts with ]"},
		{name: "unclosed class", pattern: `[a`, err: "no closing ]"},
		{name: "trailing backslash", pattern: `a\`, err: "lone backslash"},
		{name: "flag m", pattern: `a`, flags: "m", err: `flag 'm'`},
		{name: "flag g", pattern: `a`, flags: "g", err: `flag 'g'`},
		{name: "flag twice", pattern: `a`, flags: "ii", err: "twice"},
		{name: "nested repeat", pattern: `a**`, err: "nothing to repeat"},
		{name: "repeat of a repeat", pattern: `a{2}{3}`, err: "nothing to repeat"},
		{name: "two lazy marks", pattern: `a*??`, err: "nothing to repeat"},
		{name: "lazy mark on a lazy repeat", pattern: `a{2}??`, err: "nothing to repeat"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := compile(tt.pattern, tt.flags)
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("error %v, want one holding %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range tt.match {
				if !re.MatchString(s) {
					t.Errorf("%q does not match %q", tt.pattern, s)
				}
			}
			for _, s := range tt.noMatch {
				if re.MatchString(s) {
					t.Errorf("%q matches %q", tt.pattern, s)
				}
			}
		})
	}
}

func TestCompileJavaScriptRejects(t *testing.T) {
	// Each is a SyntaxError in JavaScript, reads differently there, or
	// names the placeholders, so the eval gate must refuse it with its own
	// message, not only because Go rejects it too.
	for pattern, msg := range map[string]string{
		`^*`:                "nothing to repeat",
		`$?`:                "nothing to repeat",
		`\b+`:               "nothing to repeat",
		`\B{2}`:             "nothing to repeat",
		`*a`:                "nothing to repeat",
		`(*a)`:              "nothing to repeat",
		`a|*b`:              "nothing to repeat",
		`[\s-x]`:            "followed by -",
		`/cech/workspace/a`: "names /cech/",
		`\/cech\/plugin`:    "names /cech/",
		`a/cech/home`:       "names /cech/",
	} {
		if _, err := compile(pattern, ""); err == nil || !strings.Contains(err.Error(), msg) {
			t.Errorf("%q: %v, want an error saying %q", pattern, err, msg)
		}
	}
	// A lazy quantifier, a group's repeat and escaped braces stay allowed.
	for _, pattern := range []string{`a+?`, `a{2}?`, `(a)*`, `\{a\}`} {
		if _, err := compile(pattern, ""); err != nil {
			t.Errorf("%q: %v", pattern, err)
		}
	}
}
