// Package harnessdata holds the harness data file, harnesses.toml: the
// harness behaviours CECH relies on, and its workarounds for harness
// gaps, in the format ADR 9 item 5 sets. Code that knows one harness's
// behaviour lives in internal/harness instead (ADR 15 item 6).
package harnessdata

import (
	_ "embed"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// File is the content of harnesses.toml, which the CLI builds in (ADR 9
// item 3).
//
//go:embed harnesses.toml
var File string

// The harnesses an entry can be about.
const (
	ClaudeCode = "claude-code"
	Codex      = "codex"
)

// The kinds of entry.
const (
	// Gap is a harness gap that CECH works around (ADR 9 item 2).
	Gap = "gap"
	// Behaviour is a harness behaviour CECH relies on (ADR 9 item 3).
	Behaviour = "behaviour"
)

// commands holds each harness's command, which a source that is not a
// URL starts with.
var commands = map[string]string{ClaudeCode: "claude", Codex: "codex"}

// Entry is one entry of the file.
type Entry struct {
	// Name is the name of the entry's table, which code that works
	// around the gap cites.
	Name       string `toml:"-"`
	Harness    string `toml:"harness"`
	Kind       string `toml:"kind"`
	Summary    string `toml:"summary"`
	Workaround string `toml:"workaround"`
	// Decision is the ADR item that sets the workaround or relies on the
	// behaviour, as ADR N item M, or "".
	Decision string `toml:"decision"`
	// Probe is the steps that check the entry, and Expect the result
	// while it holds.
	Probe    string   `toml:"probe"`
	Expect   string   `toml:"expect"`
	Verified Verified `toml:"verified"`
	// Sources are decoded one by one, since the TOML package records
	// one type for a key in every [[<entry>.sources]] table.
	Sources []Source `toml:"-"`
}

// Verified is the last run of the probe that gave the expected result.
type Verified struct {
	Version string `toml:"version"`
	Date    Date   `toml:"date"`
}

// Source is the docs or --help an entry rests on.
type Source struct {
	// Source is an https URL, or a command of the entry's harness.
	Source string `toml:"source"`
	// Read is the date it was read, and Version the harness version it
	// was read for.
	Read    Date   `toml:"read"`
	Version string `toml:"version"`
}

// Date is a value that should be a TOML local date, such as 2026-10-29.
// It keeps the value as decoded, since decoding into a time.Time would
// lose what kind of TOML value it was.
type Date struct{ value any }

// UnmarshalTOML keeps v for check to judge.
func (d *Date) UnmarshalTOML(v any) error {
	d.value = v
	return nil
}

// Time returns the date at midnight, and whether it is a TOML local
// date. The TOML package gives each kind of local value a zone of its
// own: date-local, datetime-local or time-local.
func (d Date) Time() (time.Time, bool) {
	t, ok := d.value.(time.Time)
	return t, ok && t.Location().String() == "date-local"
}

// String returns the date as 2026-10-29.
func (d Date) String() string {
	if t, ok := d.Time(); ok {
		return t.Format(time.DateOnly)
	}
	return fmt.Sprint(d.value)
}

// name matches an entry's name after its harness and hyphen: parts of
// lower-case letters and digits joined by single hyphens.
var name = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// decision matches an ADR item, as ADR N item M.
var decision = regexp.MustCompile(`^ADR [1-9][0-9]* item [1-9][0-9]*$`)

// version matches a harness version, as X.Y.Z in decimal with no
// leading zeros.
var version = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// Parse reads the file's content, src, and returns its entries sorted by
// name. Its error lists every problem it found, each naming the entry
// and the key, apart from TOML that does not parse, which stops it.
func Parse(src string) ([]Entry, error) {
	var tables map[string]toml.Primitive
	meta, err := toml.Decode(src, &tables)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(tables))
	for n := range tables {
		names = append(names, n)
	}
	slices.Sort(names)
	var errs []error
	var entries []Entry
	for _, n := range names {
		e, problems := decode(meta, n, tables[n])
		for _, p := range problems {
			errs = append(errs, fmt.Errorf("%s: %w", n, p))
		}
		if len(problems) == 0 {
			entries = append(entries, e)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return entries, nil
}

// keys holds the TOML type of each key an entry may have, as the TOML
// package names it. Datetime covers every kind of date and time, which
// checkDate tells apart.
var keys = map[string]string{
	"harness":          "String",
	"kind":             "String",
	"summary":          "String",
	"workaround":       "String",
	"decision":         "String",
	"probe":            "String",
	"expect":           "String",
	"verified":         "Hash",
	"verified.version": "String",
	"verified.date":    "Datetime",
	// sources must be [[<entry>.sources]] tables, whose keys decode
	// checks one table at a time.
	"sources": "ArrayHash",
}

// kinds names each TOML type, as the TOML package names it, for an
// error message.
var kinds = map[string]string{
	"String":    "a string",
	"Integer":   "an integer",
	"Float":     "a float",
	"Bool":      "a boolean",
	"Datetime":  "a date or time",
	"Array":     "an array",
	"Hash":      "a table",
	"ArrayHash": "an array of tables",
}

// decode decodes and checks the entry named n. It decodes only an entry
// whose keys are all known and of the right type, so each problem gets
// a message of its own rather than the TOML package's.
func decode(meta toml.MetaData, n string, p toml.Primitive) (Entry, []error) {
	// The TOML package calls a table, inline or not, a Hash.
	if meta.Type(n) != "Hash" {
		return Entry{}, []error{errors.New("is not a table")}
	}
	var errs []error
	// reported holds the keys reported so far, whose keys go unreported.
	var reported []string
	for _, k := range meta.Keys() {
		if len(k) < 2 || k[0] != n {
			continue
		}
		key := strings.Join(k[1:], ".")
		if strings.HasPrefix(key, "sources.") {
			continue
		}
		if slices.ContainsFunc(reported, func(r string) bool { return strings.HasPrefix(key, r+".") }) {
			continue
		}
		want, ok := keys[key]
		got := meta.Type(k...)
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("unknown key %q", key))
		case key == "sources" && got != want:
			errs = append(errs, fmt.Errorf("sources is %s, not [[%s.sources]] tables", kinds[got], n))
		case got != want && want == "Datetime":
			errs = append(errs, fmt.Errorf("%s is %s, not a date such as 2026-10-29", key, kinds[got]))
		case got != want:
			errs = append(errs, fmt.Errorf("%s is %s, not %s", key, kinds[got], kinds[want]))
		default:
			continue
		}
		reported = append(reported, key)
	}
	var raw struct {
		Sources []map[string]any `toml:"sources"`
	}
	if !slices.Contains(reported, "sources") {
		// sources is absent or [[<entry>.sources]] tables, so this cannot
		// fail, whatever is wrong with the other keys.
		if err := meta.PrimitiveDecode(p, &raw); err != nil {
			errs = append(errs, err)
		}
	}
	sources, serrs := decodeSources(raw.Sources)
	errs = append(errs, serrs...)
	if len(errs) > 0 {
		return Entry{}, errs
	}
	e := Entry{Name: n}
	if err := meta.PrimitiveDecode(p, &e); err != nil {
		return e, []error{err}
	}
	e.Sources = sources
	return e, e.check(meta)
}

// decodeSources builds each source from its table as decoded, checking
// each key and its type.
func decodeSources(raw []map[string]any) ([]Source, []error) {
	var errs []error
	sources := make([]Source, len(raw))
	for i, m := range raw {
		for _, k := range slices.Sorted(maps.Keys(m)) {
			key := fmt.Sprintf("sources[%d].%s", i, k)
			v := m[k]
			text, isText := v.(string)
			_, isTime := v.(time.Time)
			switch {
			case k == "source" && isText:
				sources[i].Source = text
			case k == "version" && isText:
				sources[i].Version = text
			case k == "read" && isTime:
				sources[i].Read = Date{v}
			case k == "source" || k == "version":
				errs = append(errs, fmt.Errorf("%s is %s, not a string", key, describe(v)))
			case k == "read":
				errs = append(errs, fmt.Errorf("%s is %s, not a date such as 2026-10-29", key, describe(v)))
			default:
				errs = append(errs, fmt.Errorf("unknown key %q", key))
			}
		}
	}
	return sources, errs
}

// describe names the TOML type of a value as the TOML package decodes
// it, for an error message.
func describe(v any) string {
	switch v.(type) {
	case string:
		return "a string"
	case int64:
		return "an integer"
	case float64:
		return "a float"
	case bool:
		return "a boolean"
	case time.Time:
		return "a date or time"
	case map[string]any:
		return "a table"
	}
	return "an array"
}

// check applies the rules of ADR 9 item 5 to e.
func (e Entry) check(meta toml.MetaData) []error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	command, known := commands[e.Harness]
	switch {
	case e.Harness == "":
		add("harness is missing")
	case !known:
		// The name and the command sources depend on the harness, so
		// they go unchecked until it is fixed.
		add("harness is %q, not %q or %q", e.Harness, ClaudeCode, Codex)
	case !strings.HasPrefix(e.Name, e.Harness+"-") || !name.MatchString(strings.TrimPrefix(e.Name, e.Harness+"-")):
		add("the name must be %s- and then parts of lower-case letters and digits joined by single hyphens", e.Harness)
	}
	switch e.Kind {
	case "":
		add("kind is missing")
	case Gap:
		if blank(e.Workaround) {
			add("workaround is missing, and a gap needs one")
		}
	case Behaviour:
		if meta.IsDefined(e.Name, "workaround") {
			add("workaround is out of place: only a gap has one")
		}
	default:
		add("kind is %q, not %q or %q", e.Kind, Gap, Behaviour)
	}
	for _, f := range []struct{ key, value string }{
		{"summary", e.Summary}, {"probe", e.Probe}, {"expect", e.Expect},
	} {
		if blank(f.value) {
			add("%s is missing", f.key)
		}
	}
	if meta.IsDefined(e.Name, "decision") && !decision.MatchString(e.Decision) {
		add("decision is %q, not an ADR item as ADR N item M", e.Decision)
	}
	errs = append(errs, checkVersion("verified.version", e.Verified.Version)...)
	errs = append(errs, checkDate("verified.date", e.Verified.Date)...)
	for i, s := range e.Sources {
		key := fmt.Sprintf("sources[%d]", i)
		if known && !isURL(s.Source) && !isCommand(s.Source, command) {
			add("%s.source is %q, not an https URL or a command that starts with %s", key, s.Source, command)
		}
		errs = append(errs, checkDate(key+".read", s.Read)...)
		errs = append(errs, checkVersion(key+".version", s.Version)...)
	}
	return errs
}

// blank reports whether s holds nothing but white space.
func blank(s string) bool {
	return strings.TrimSpace(s) == ""
}

func checkVersion(key, v string) []error {
	switch {
	case v == "":
		return []error{fmt.Errorf("%s is missing", key)}
	case !version.MatchString(v):
		return []error{fmt.Errorf("%s is %q, not a version as X.Y.Z", key, v)}
	}
	return nil
}

// checkDate checks that d, the value of key, is a TOML local date.
func checkDate(key string, d Date) []error {
	if d.value == nil {
		return []error{fmt.Errorf("%s is missing", key)}
	}
	if _, ok := d.Time(); ok {
		return nil
	}
	kind := describe(d.value)
	if t, ok := d.value.(time.Time); ok {
		switch t.Location().String() {
		case "datetime-local":
			kind = "a date and time"
		case "time-local":
			kind = "a time"
		default:
			kind = "a date and time with an offset"
		}
	}
	return []error{fmt.Errorf("%s is %s, not a date such as 2026-10-29", key, kind)}
}

// isURL reports whether s is an https URL with a host and no spaces.
func isURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && strings.HasPrefix(s, "https://") && u.Host != "" && !strings.ContainsAny(s, " \t\n")
}

// isCommand reports whether s is command or starts with it and a space.
func isCommand(s, command string) bool {
	return s == command || strings.HasPrefix(s, command+" ")
}
