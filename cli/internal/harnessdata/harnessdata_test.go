package harnessdata

import (
	"slices"
	"strings"
	"testing"
)

// TestFile holds the committed harnesses.toml to ADR 5 item 5, so the
// merge gate fails on a wrong entry. The file is built in, so go test's
// cache sees a change to it.
func TestFile(t *testing.T) {
	entries, err := Parse(File)
	if err != nil {
		t.Fatalf("harnesses.toml:\n%v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	// The entries that code cites, or relies on.
	for _, want := range []string{
		"claude-code-dependency-marketplace",
		"claude-code-plugin-metadata",
		"codex-plugin-agents",
		"codex-plugin-metadata",
	} {
		if !slices.Contains(names, want) {
			t.Errorf("harnesses.toml has no entry %s", want)
		}
	}
}

// gap is a valid gap entry, which each case below changes.
const gap = `[codex-x]
harness = "codex"
kind = "gap"
summary = "s"
workaround = "w"
decision = "ADR 6 item 3"
probe = "p"
expect = "e"
verified = { version = "0.165.0", date = 2026-10-29 }

[[codex-x.sources]]
source = "https://example.com/docs"
read = 2026-10-29
version = "0.165.0"

[[codex-x.sources]]
source = "codex plugin --help"
read = 2026-10-29
version = "0.165.0"
`

// behaviour is a valid behaviour entry, with only the required keys.
const behaviour = `[claude-code-y]
harness = "claude-code"
kind = "behaviour"
summary = "s"
probe = "p"
expect = "e"
verified = { version = "2.1.312", date = 2026-10-29 }
`

func TestParse(t *testing.T) {
	entries, err := Parse(gap + "\n" + behaviour)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name != "claude-code-y" || entries[1].Name != "codex-x" {
		t.Fatalf("got %+v, want claude-code-y then codex-x", entries)
	}
	// Go visits a map in a random order, so the order of one parse could
	// be right by chance.
	for range 20 {
		again, _ := Parse(gap + "\n" + behaviour)
		if again[0].Name != "claude-code-y" {
			t.Fatal("Parse does not sort the entries by name")
		}
	}
	e := entries[1]
	if e.Workaround != "w" || e.Verified.Version != "0.165.0" || e.Verified.Date.String() != "2026-10-29" ||
		len(e.Sources) != 2 || e.Sources[1].Source != "codex plugin --help" {
		t.Errorf("got %+v", e)
	}
	if entries, err := Parse(""); err != nil || len(entries) != 0 {
		t.Errorf("Parse of an empty file = %v, %v; want no entries and no error", entries, err)
	}
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name string
		src  string
		// want is the whole error.
		want string
	}{
		{"unknown key in a source", gap + `typo = "x"` + "\n", `codex-x: unknown key "sources[1].typo"`},
		{"unknown key", strings.Replace(gap, `expect = "e"`, "expect = \"e\"\nexpects = \"e\"", 1), `codex-x: unknown key "expects"`},
		{"unknown table, reported once", gap + "[codex-x.extra]\nk = 1\n", `codex-x: unknown key "extra"`},
		{"unknown key in verified", strings.Replace(gap, `date = 2026-10-29 }`, `date = 2026-10-29, by = "me" }`, 1), `codex-x: unknown key "verified.by"`},
		{"a value outside a table", "codex-z = 1\n", "codex-z: is not a table"},
		{"an array of tables", "[[codex-z]]\n", "codex-z: is not a table"},
		{"a wrong type", strings.Replace(gap, `summary = "s"`, `summary = 1`, 1), "codex-x: summary is an integer, not a string"},
		{"wrong types, each reported", strings.Replace(strings.Replace(gap, `summary = "s"`, `summary = 1`, 1), `probe = "p"`, `probe = true`, 1), "codex-x: summary is an integer, not a string\ncodex-x: probe is a boolean, not a string"},
		{"verified as a string", strings.Replace(behaviour, `verified = { version = "2.1.312", date = 2026-10-29 }`, `verified = "x"`, 1), "claude-code-y: verified is a string, not a table"},
		{"sources as an array of strings", behaviour + `sources = ["x"]` + "\n", "claude-code-y: sources is an array, not [[claude-code-y.sources]] tables"},
		{"sources as an inline array of tables", behaviour + `sources = [{ source = "claude", read = 2026-10-29, version = "2.1.312" }]` + "\n", "claude-code-y: sources is an array, not [[claude-code-y.sources]] tables"},
		{"unknown key in one of two entries", gap + "\n" + behaviour + "typo = 1\n", `claude-code-y: unknown key "typo"`},
		{"harness missing", strings.Replace(gap, "harness = \"codex\"\n", "", 1), "codex-x: harness is missing"},
		{"unknown harness, its command source unchecked", strings.Replace(gap, `harness = "codex"`, `harness = "cursor"`, 1), `codex-x: harness is "cursor", not "claude-code" or "codex"`},
		{"name of another harness", strings.Replace(gap, `harness = "codex"`, `harness = "claude-code"`, 1), "codex-x: the name must be claude-code- and then parts of lower-case letters and digits joined by single hyphens\n" +
			`codex-x: sources[1].source is "codex plugin --help", not an https URL or a command that starts with claude`},
		{"name with only the harness", strings.ReplaceAll(gap, "codex-x", "codex-"), "codex-: the name must be codex- and then parts of lower-case letters and digits joined by single hyphens"},
		{"name with a double hyphen", strings.ReplaceAll(gap, "codex-x", "codex--x"), "codex--x: the name must be codex- and then parts of lower-case letters and digits joined by single hyphens"},
		{"name in upper case", strings.ReplaceAll(gap, "codex-x", "codex-X"), "codex-X: the name must be codex- and then parts of lower-case letters and digits joined by single hyphens"},
		{"kind missing", strings.Replace(gap, "kind = \"gap\"\n", "", 1), "codex-x: kind is missing"},
		{"unknown kind", strings.Replace(gap, `kind = "gap"`, `kind = "bug"`, 1), `codex-x: kind is "bug", not "gap" or "behaviour"`},
		{"gap without a workaround", strings.Replace(gap, "workaround = \"w\"\n", "", 1), "codex-x: workaround is missing, and a gap needs one"},
		{"gap with a blank workaround", strings.Replace(gap, `workaround = "w"`, `workaround = " "`, 1), "codex-x: workaround is missing, and a gap needs one"},
		{"behaviour with a workaround", behaviour + "workaround = \"\"\n", "claude-code-y: workaround is out of place: only a gap has one"},
		{"summary missing", strings.Replace(gap, "summary = \"s\"\n", "", 1), "codex-x: summary is missing"},
		{"probe empty", strings.Replace(gap, `probe = "p"`, `probe = ""`, 1), "codex-x: probe is missing"},
		{"expect blank", strings.Replace(gap, `expect = "e"`, `expect = "\n"`, 1), "codex-x: expect is missing"},
		{"decision in another form", strings.Replace(gap, `"ADR 6 item 3"`, `"ADR 6"`, 1), `codex-x: decision is "ADR 6", not an ADR item as ADR N item M`},
		{"decision empty", strings.Replace(gap, `"ADR 6 item 3"`, `""`, 1), `codex-x: decision is "", not an ADR item as ADR N item M`},
		{"verified missing", strings.Replace(behaviour, "verified = { version = \"2.1.312\", date = 2026-10-29 }\n", "", 1), "claude-code-y: verified.version is missing\nclaude-code-y: verified.date is missing"},
		{"verified version with a v", strings.Replace(gap, `version = "0.165.0", date`, `version = "v0.165.0", date`, 1), `codex-x: verified.version is "v0.165.0", not a version as X.Y.Z`},
		{"verified version with a leading zero", strings.Replace(gap, `version = "0.165.0", date`, `version = "0.0165.0", date`, 1), `codex-x: verified.version is "0.0165.0", not a version as X.Y.Z`},
		{"verified date missing", strings.Replace(gap, `, date = 2026-10-29 }`, ` }`, 1), "codex-x: verified.date is missing"},
		{"verified date and time", strings.Replace(gap, `date = 2026-10-29 }`, `date = 2026-10-29T10:00:00 }`, 1), "codex-x: verified.date is a date and time, not a date such as 2026-10-29"},
		{"verified date with an offset", strings.Replace(gap, `date = 2026-10-29 }`, `date = 2026-10-29T00:00:00Z }`, 1), "codex-x: verified.date is a date and time with an offset, not a date such as 2026-10-29"},
		{"verified time", strings.Replace(gap, `date = 2026-10-29 }`, `date = 10:00:00 }`, 1), "codex-x: verified.date is a time, not a date such as 2026-10-29"},
		{"verified date as a string", strings.Replace(gap, `date = 2026-10-29 }`, `date = "2026-10-29" }`, 1), "codex-x: verified.date is a string, not a date such as 2026-10-29"},
		{"verified date as a table", strings.Replace(gap, `date = 2026-10-29 }`, `date = { y = 1 } }`, 1), "codex-x: verified.date is a table, not a date such as 2026-10-29"},
		{"source over http", strings.Replace(gap, "https://example.com", "http://example.com", 1), `codex-x: sources[0].source is "http://example.com/docs", not an https URL or a command that starts with codex`},
		{"source in upper case", strings.Replace(gap, "https://example.com", "HTTPS://example.com", 1), `codex-x: sources[0].source is "HTTPS://example.com/docs", not an https URL or a command that starts with codex`},
		{"source URL with a space", strings.Replace(gap, "https://example.com/docs", "https://example.com/a b", 1), `codex-x: sources[0].source is "https://example.com/a b", not an https URL or a command that starts with codex`},
		{"source of another harness", strings.Replace(gap, `"codex plugin --help"`, `"claude plugin --help"`, 1), `codex-x: sources[1].source is "claude plugin --help", not an https URL or a command that starts with codex`},
		{"source that only starts like a command", strings.Replace(gap, `"codex plugin --help"`, `"codexx --help"`, 1), `codex-x: sources[1].source is "codexx --help", not an https URL or a command that starts with codex`},
		{"source missing", strings.Replace(gap, "source = \"codex plugin --help\"\n", "", 1), `codex-x: sources[1].source is "", not an https URL or a command that starts with codex`},
		{"wrong type in the first of two sources", strings.Replace(gap, "version = \"0.165.0\"\n\n[[", "version = 1\n\n[[", 1), "codex-x: sources[0].version is an integer, not a string"},
		{"read as a string in the first of two sources", strings.Replace(gap, "read = 2026-10-29\nversion = \"0.165.0\"\n\n[[", "read = \"2026-10-29\"\nversion = \"0.165.0\"\n\n[[", 1), "codex-x: sources[0].read is a string, not a date such as 2026-10-29"},
		{"source as an integer in the last source", strings.Replace(gap, `source = "codex plugin --help"`, `source = 1`, 1), "codex-x: sources[1].source is an integer, not a string"},
		{"unknown key in the first of two sources", strings.Replace(gap, "read = 2026-10-29\nversion = \"0.165.0\"\n\n[[", "read = 2026-10-29\nversion = \"0.165.0\"\nnote = \"x\"\n\n[[", 1), `codex-x: unknown key "sources[0].note"`},
		{"a date where a string belongs", strings.Replace(gap, `summary = "s"`, `summary = 2026-10-29`, 1), "codex-x: summary is a date or time, not a string"},
		{"source read missing", strings.Replace(gap, "read = 2026-10-29\nversion = \"0.165.0\"\n\n[[", "version = \"0.165.0\"\n\n[[", 1), "codex-x: sources[0].read is missing"},
		{"source version missing", strings.TrimSuffix(gap, "version = \"0.165.0\"\n"), "codex-x: sources[1].version is missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.src)
			if err == nil {
				t.Fatalf("no error, want %q", tt.want)
			}
			if err.Error() != tt.want {
				t.Errorf("error\n%s\nwant\n%s", err, tt.want)
			}
		})
	}
}

// TestParseListsEveryProblem checks that one entry's problem, even one
// that stops its decoding, does not hide another's.
func TestParseListsEveryProblem(t *testing.T) {
	src := strings.Replace(strings.Replace(gap, `summary = "s"`, `summary = 1`, 1), "read = 2026-10-29\nversion = \"0.165.0\"\n\n[[", "read = 1\nversion = \"0.165.0\"\n\n[[", 1) + "\n" +
		strings.Replace(behaviour, `probe = "p"`, `probe = ""`, 1)
	_, err := Parse(src)
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{
		"codex-x: summary is an integer, not a string",
		"codex-x: sources[0].read is an integer, not a date such as 2026-10-29",
		"claude-code-y: probe is missing",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}
