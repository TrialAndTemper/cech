// Package terms runs the term tiers of ADR 3 for `cech check`, with the
// engine and rules that ADR 24 sets: Betterleaks v1, the generic rules
// in generic.toml, an extension's own rule and name files, and its
// private list.
//
// The package reports what it finds and leaves the output format, and
// the exit status, to the command.
package terms

import (
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"slices"
	"strings"
)

// Generic is the generic tier, which the CLI builds in and CECH's gate
// extends from checks/terms/rules.toml (ADR 24 item 3).
//
//go:embed generic.toml
var Generic []byte

// Tier names the tier a finding comes from.
type Tier string

// The tiers.
const (
	TierGeneric   Tier = "generic"
	TierExtension Tier = "extension"
	TierPrivate   Tier = "private"
)

// Places a finding can be in other than a file.
const (
	WherePaths       = "paths"
	WhereMessage     = "commit message"
	WhereRefNames    = "ref names"
	WhereTagMessages = "tag messages"
)

// Finding is one hit. Fields that could show a private term are empty
// while a private hit exists or the private tier could not run.
type Finding struct {
	Tier Tier
	Rule string
	// Where is a path, one of the Where constants, or "" when hidden.
	Where string
	// Line is the line in Where, from 1, or 0 when hidden.
	Line int
	// Commit is the commit of a hit from a range, or "".
	Commit string
	// Match is the matched text of a public hit, or "" when hidden.
	Match string
	// Term is the line of a private hit's term in the private list.
	Term int
}

// String formats f as CECH's gate does, with the match quoted as %q
// quotes it, so that no control character reaches a terminal.
func (f Finding) String() string {
	var b strings.Builder
	if f.Where != "" {
		fmt.Fprintf(&b, "%s:%d: ", f.Where, f.Line)
	}
	if f.Tier == TierPrivate {
		fmt.Fprintf(&b, "private term %d", f.Term)
	} else {
		b.WriteString(f.Rule)
	}
	if f.Commit != "" {
		fmt.Fprintf(&b, " in commit %s", f.Commit)
	}
	if f.Match != "" {
		fmt.Fprintf(&b, ": %q", f.Match)
	}
	return b.String()
}

// ProblemKind names a kind of Problem.
type ProblemKind string

// The kinds of Problem.
const (
	ProblemEntry      ProblemKind = "invalid terms entry"
	ProblemRuleFile   ProblemKind = "invalid rule file"
	ProblemNameFile   ProblemKind = "invalid name file"
	ProblemMissing    ProblemKind = "tracked file missing"
	ProblemUnreadable ProblemKind = "tracked file unreadable"
	ProblemRepo       ProblemKind = "git cannot read the repository"
)

// Problem is a failure other than a hit: an invalid input, or a file the
// check could not scan. Detail names files, keys and rules; it is empty
// while a private hit exists or the private tier could not run.
type Problem struct {
	Kind   ProblemKind
	Detail string
}

func (p Problem) String() string { return kindDetail(string(p.Kind), p.Detail) }

// NoteKind names a kind of Note.
type NoteKind string

// The kinds of Note.
const (
	NotePrivateSkipped   NoteKind = "private tier skipped"
	NotePrivateUndecided NoteKind = "private tier could not run"
	NoteTiersSkipped     NoteKind = "generic and extension tiers skipped"
	NoteTiersUndecided   NoteKind = "generic and extension tiers could not run"
	NoteProjectUndecided NoteKind = "project scan could not run"
	NoteBinarySkipped    NoteKind = "binary files skipped"
	NoteSubmodules       NoteKind = "submodules skipped"
	NoteRangeEmpty       NoteKind = "the revision range holds no commits"
	NoteRangeUnresolved  NoteKind = "the revision range could not be resolved"
)

// Note is something the output says about what was or was not checked.
// Only a NoteRangeUnresolved detail, the range, can hold text from the
// user, so it alone is emptied while hiding.
type Note struct {
	Kind   NoteKind
	Detail string
}

func (n Note) String() string { return kindDetail(string(n.Kind), n.Detail) }

func kindDetail(kind, detail string) string {
	if detail == "" {
		return kind
	}
	return kind + ": " + detail
}

// Result is what a check found. Skipped and Undecided are reported in
// the output; per ADR 15 item 5, a Result with Findings or Problems
// fails, and one with only Undecided could not decide.
type Result struct {
	Findings  []Finding
	Problems  []Problem
	Skipped   []Note
	Undecided []Note
	// hidden is set while a private hit exists or the private tier
	// could not run: then nothing that could show a term is kept.
	hidden bool
}

// Hidden reports whether r hides what could show a private term, so the
// output can say why public hits show no location.
func (r *Result) Hidden() bool { return r.hidden }

// Failed reports whether r holds a hit or a problem.
func (r *Result) Failed() bool {
	return len(r.Findings) > 0 || len(r.Problems) > 0
}

// Extension is what `cech check` passes for an extension.
type Extension struct {
	// Namespace is the extension's namespace, which its rule ids start
	// with.
	Namespace string
	// Rules and Names are the entries of terms.rules and terms.names in
	// data/marketplace.toml.
	Rules []string
	Names []string
	// Private is the value of CECH_PRIVATE_TERMS, and RequirePrivate
	// whether CECH_REQUIRE_PRIVATE_TERMS is 1.
	Private        string
	RequirePrivate bool
	// Range is the --range value, or "".
	Range string
}

// Merge adds the findings, problems and notes of o to r, and reports a
// hit both found once. If either hides what could show a term, so does
// the result.
func (r *Result) Merge(o *Result) {
	r.Findings = append(r.Findings, o.Findings...)
	r.Problems = append(r.Problems, o.Problems...)
	r.Skipped = append(r.Skipped, o.Skipped...)
	r.Undecided = append(r.Undecided, o.Undecided...)
	r.hidden = r.hidden || o.hidden
	r.finish()
}

// finish applies the hiding rules, then sorts the findings and drops
// those that are now the same.
func (r *Result) finish() {
	if r.hidden {
		for i, f := range r.Findings {
			if f.Tier != TierPrivate {
				r.Findings[i] = Finding{Tier: f.Tier, Rule: f.Rule}
			}
		}
		for i := range r.Problems {
			r.Problems[i].Detail = ""
		}
		for _, notes := range [][]Note{r.Skipped, r.Undecided} {
			for i := range notes {
				if notes[i].Kind == NoteRangeUnresolved {
					notes[i].Detail = ""
				}
			}
		}
	}
	slices.SortStableFunc(r.Findings, func(a, b Finding) int {
		return cmp.Or(
			cmp.Compare(a.Where, b.Where),
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Commit, b.Commit),
			cmp.Compare(a.Tier, b.Tier),
			cmp.Compare(a.Rule, b.Rule),
			cmp.Compare(a.Term, b.Term),
			cmp.Compare(a.Match, b.Match),
		)
	})
	r.Findings = slices.Compact(r.Findings)
}

// UsageError is a wrong value on the command line (exit 2).
type UsageError struct{ msg string }

func (e *UsageError) Error() string { return e.msg }

// CheckProject scans the files `cech` owns or renders in a project with
// the generic rules. root is the repository root. The error is only for
// broken built-in rules; a scan that fails is a NoteProjectUndecided
// note, in a fixed message, since the cause could name a path.
func CheckProject(ctx context.Context, root string) (*Result, error) {
	cfg, err := genericConfig()
	if err != nil {
		return nil, err
	}
	res := &Result{}
	frags, problems, err := projectFragments(root)
	if err != nil {
		res.Undecided = append(res.Undecided, Note{NoteProjectUndecided, "cannot read the project's files"})
		return res, nil
	}
	res.Problems = problems
	hits, err := scan(ctx, cfg, frags)
	if err != nil {
		res.Undecided = append(res.Undecided, Note{NoteProjectUndecided, "the scan failed"})
	}
	for _, h := range hits {
		res.Findings = append(res.Findings, h.public(TierGeneric))
	}
	res.finish()
	return res, nil
}

// CheckExtension runs every tier over an extension. root is the
// repository root. The error is a *UsageError for a --range value that
// starts with -, or else broken built-in rules; every other failure is a
// Problem or a note.
func CheckExtension(ctx context.Context, root string, ext Extension) (*Result, error) {
	if strings.HasPrefix(ext.Range, "-") {
		return nil, &UsageError{"--range value must not start with -"}
	}
	res := &Result{}
	// The private tier first: while it has hits, or cannot run, the
	// public tiers hide what could show a term.
	priv := loadPrivate(ext.Private)
	switch {
	case priv.err != nil:
		res.Undecided = append(res.Undecided, Note{NotePrivateUndecided, priv.err.Error()})
	case priv.none != "" && ext.RequirePrivate:
		res.Undecided = append(res.Undecided, Note{NotePrivateUndecided, priv.none + ", but CECH_REQUIRE_PRIVATE_TERMS=1"})
	case priv.none != "":
		res.Skipped = append(res.Skipped, Note{NotePrivateSkipped, priv.none})
	}
	privateRan := priv.err == nil && priv.none == ""
	privateUndecided := !privateRan && len(res.Undecided) > 0

	repo, err := openRepo(ctx, root)
	if err != nil {
		res.Problems = append(res.Problems, Problem{ProblemRepo, err.Error()})
		res.hidden = privateUndecided
		res.finish()
		return res, nil
	}
	defer repo.close()

	lists := listedFiles(ext)
	frags, problems, notes, undecided := repo.fragments(ctx, ext.Range)
	res.Problems = append(res.Problems, problems...)
	res.Skipped = append(res.Skipped, notes...)

	privHits := 0
	hideRange := false
	if privateRan {
		hits, why := scanPrivate(ctx, priv, frags)
		switch {
		case why != "":
			// A fixed message: the engine's own could quote a term.
			res.Undecided = append(res.Undecided, Note{NotePrivateUndecided, why})
			privateUndecided = true
		default:
			hideAll := privateHidesLocations(hits)
			for _, h := range hits {
				res.Findings = append(res.Findings, h.private(priv.lines, hideAll))
			}
			privHits = len(hits)
			hideRange = undecided && matchesAny(priv.res, ext.Range)
		}
	}
	if undecided {
		n := Note{NoteRangeUnresolved, ext.Range}
		if hideRange {
			n.Detail = ""
		}
		res.Undecided = append(res.Undecided, n)
	}

	cfg, ruleProblems, err := extensionConfig(repo, ext)
	if err != nil {
		return nil, err
	}
	res.Problems = append(res.Problems, ruleProblems...)
	if len(ruleProblems) > 0 {
		res.Skipped = append(res.Skipped, Note{NoteTiersSkipped, "an extension rule or name file is invalid"})
	} else {
		hits, err := scan(ctx, cfg.config, frags.public(lists))
		if err != nil {
			// A fixed message, and the private results so far are kept.
			res.Undecided = append(res.Undecided, Note{NoteTiersUndecided, "the scan failed"})
		}
		for _, h := range hits {
			res.Findings = append(res.Findings, h.public(cfg.tier(h.rule)))
		}
	}

	res.hidden = privHits > 0 || privateUndecided
	res.finish()
	return res, nil
}

// scanPrivate runs the private list over every fragment, the files under
// terms included. It returns why it could not, in a fixed message, or "".
func scanPrivate(ctx context.Context, priv privateList, frags fragments) ([]hit, string) {
	cfg, err := buildConfig(priv.file)
	if err != nil {
		return nil, "the engine refuses the private list"
	}
	hits, err := scan(ctx, cfg, frags.all())
	if err != nil {
		return nil, "the private scan failed"
	}
	return hits, ""
}
