package terms

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/betterleaks/betterleaks/config"
	"github.com/betterleaks/betterleaks/detect"
	"github.com/betterleaks/betterleaks/logging"
	"github.com/betterleaks/betterleaks/sources"
)

// Betterleaks logs to stderr on its own; with the private list, a
// message could quote a term, so nothing it logs is shown.
func init() {
	logging.Logger = logging.Logger.Output(io.Discard)
}

// attrIndex is the fragment attribute that carries the fragment's place
// in the list scanned, so a finding leads back to it.
const attrIndex = "cech.index"

// Configs are parsed without a path: the engine skips any file at its
// config's path, which here would be a file of the repository checked.
func genericConfig() (*config.Config, error) {
	cfg, err := config.ParseTOML(Generic, "")
	if err != nil {
		return nil, fmt.Errorf("built-in generic rules: %w", err)
	}
	return cfg, nil
}

// buildConfig hands checked rules to the engine. They are encoded again
// rather than passed as read, so that the engine sees only the keys
// ruleFile allows.
func buildConfig(rf ruleFile) (*config.Config, error) {
	var b bytes.Buffer
	if err := toml.NewEncoder(&b).Encode(rf); err != nil {
		return nil, err
	}
	return config.ParseTOML(b.Bytes(), "")
}

// extConfig is the generic rules with an extension's added.
type extConfig struct {
	config *config.Config
	// own holds the ids of the extension's rules.
	own map[string]bool
}

func (c extConfig) tier(id string) Tier {
	if c.own[id] {
		return TierExtension
	}
	return TierGeneric
}

// extensionConfig checks an extension's term lists and adds them to the
// generic rules.
func extensionConfig(repo *repo, ext Extension) (extConfig, []Problem, error) {
	combined, bad, err := decodeRuleFile(Generic)
	if err != nil {
		return extConfig{}, nil, fmt.Errorf("built-in generic rules: %w", err)
	}
	if len(bad) > 0 {
		return extConfig{}, nil, fmt.Errorf("built-in generic rules: keys not allowed: %v", bad)
	}
	gen, err := genericConfig()
	if err != nil {
		return extConfig{}, nil, err
	}
	taken := map[string]bool{}
	for id := range gen.Rules {
		taken[id] = true
	}
	own := map[string]bool{}
	var ps []Problem
	seen := map[string]bool{}
	for _, entry := range ext.Rules {
		p, prob := checkEntry(repo, "rules", entry, seen)
		if prob != nil {
			ps = append(ps, *prob)
			continue
		}
		data, rerr := repo.fs.ReadFile(filepath.FromSlash(p))
		if rerr != nil {
			ps = append(ps, Problem{ProblemRuleFile, p + ": cannot read it"})
			continue
		}
		rf, bad, derr := decodeRuleFile(data)
		if derr != nil {
			ps = append(ps, Problem{ProblemRuleFile, p + ": " + derr.Error()})
			continue
		}
		fileProblems := checkRuleFile(p, rf, bad, ext.Namespace, taken)
		ps = append(ps, fileProblems...)
		combined.Allowlists = append(combined.Allowlists, rf.Allowlists...)
		combined.Rules = append(combined.Rules, rf.Rules...)
		for _, r := range rf.Rules {
			own[r.ID] = true
		}
	}
	for k, entry := range ext.Names {
		p, prob := checkEntry(repo, "names", entry, seen)
		if prob != nil {
			ps = append(ps, *prob)
			continue
		}
		data, rerr := repo.fs.ReadFile(filepath.FromSlash(p))
		if rerr != nil {
			ps = append(ps, Problem{ProblemNameFile, p + ": cannot read it"})
			continue
		}
		rs, nameProblems := nameRules(p, data, k+1)
		ps = append(ps, nameProblems...)
		for _, r := range rs {
			combined.Rules = append(combined.Rules, r)
			own[r.ID] = true
		}
	}
	if len(ps) > 0 {
		return extConfig{}, ps, nil
	}
	// The checks above cover what the engine checks; should it still
	// refuse the rules, that is the extension's to fix, not a crash.
	cfg, err := buildConfig(combined)
	if err != nil {
		return extConfig{}, []Problem{{ProblemRuleFile, "the engine refuses the extension's rules: " + err.Error()}}, nil
	}
	return extConfig{cfg, own}, nil, nil
}

// hit is one engine finding, traced back to its fragment.
type hit struct {
	rule   string
	where  string
	line   int
	commit string
	match  string
}

func (h hit) public(t Tier) Finding {
	return Finding{Tier: t, Rule: h.rule, Where: h.where, Line: h.line, Commit: h.commit, Match: h.match}
}

// private formats a private hit: never the term or the match. When hide
// is set, the only places it shows are the list of paths and the ref
// names, whose line numbers reveal nothing.
func (h hit) private(lines map[string]int, hide bool) Finding {
	f := Finding{Tier: TierPrivate, Rule: h.rule, Where: h.where, Line: h.line, Commit: h.commit, Term: lines[h.rule]}
	if hide && h.where != WherePaths && h.where != WhereRefNames {
		f.Where, f.Line = "", 0
	}
	return f
}

// privateHidesLocations reports whether a private hit is in a path or a
// ref name, which later output would otherwise show.
func privateHidesLocations(hits []hit) bool {
	for _, h := range hits {
		if h.where == WherePaths || h.where == WhereRefNames {
			return true
		}
	}
	return false
}

// scan runs the engine over frags. Betterleaks never contacts a
// provider, since validation is off (ADR 24 item 1), and its own allow
// comments are ignored. Path allowlists that apply to every rule become
// the engine's skip function, which a source must call itself.
func scan(ctx context.Context, cfg *config.Config, frags []fragment) ([]hit, error) {
	d := detect.NewDetectorContext(ctx, cfg, detect.ValidationOptions{})
	d.IgnoreGitleaksAllow = true
	var hits []hit
	for r := range d.Run(ctx, fragmentSource{frags, d.SkipFunc()}) {
		if r.Err != nil {
			return nil, fmt.Errorf("term scan: %w", r.Err)
		}
		i, err := strconv.Atoi(r.Finding.Attributes[attrIndex])
		if err != nil || i < 0 || i >= len(frags) {
			return nil, fmt.Errorf("term scan: finding without its fragment")
		}
		fr := frags[i]
		hits = append(hits, hit{
			rule:   r.Finding.RuleID,
			where:  fr.where,
			line:   r.Finding.StartLine,
			commit: fr.commit,
			match:  r.Finding.Match,
		})
	}
	return hits, nil
}

// fragmentSource yields frags to the engine, each with its index, and
// its path for path allowlists, leaving out those skip drops.
type fragmentSource struct {
	frags []fragment
	skip  sources.SkipFunc
}

func (fs fragmentSource) Fragments(ctx context.Context, yield sources.FragmentsFunc) error {
	for i, f := range fs.frags {
		if err := ctx.Err(); err != nil {
			return err
		}
		attrs := map[string]string{attrIndex: strconv.Itoa(i)}
		if f.file != "" {
			attrs[sources.AttrPath] = f.file
			if fs.skip != nil && fs.skip(maps.Clone(attrs)) {
				continue
			}
		}
		if err := yield(sources.Fragment{Raw: f.text, Attributes: attrs}, nil); err != nil {
			return err
		}
	}
	return nil
}

// fragment is one text to scan.
type fragment struct {
	// where is the path, or a Where constant, that findings name.
	where  string
	commit string
	// file is the tracked path the text comes from, or "".
	file string
	text string
}

type fragments []fragment

func (fs fragments) all() []fragment { return fs }

// public leaves out the files listed under terms.
func (fs fragments) public(lists map[string]bool) []fragment {
	var out []fragment
	for _, f := range fs {
		if f.file == "" || !lists[f.file] {
			out = append(out, f)
		}
	}
	return out
}

// listedFiles is the set of paths named under terms. An entry that is
// not a clean path stops the public tiers anyway.
func listedFiles(ext Extension) map[string]bool {
	m := map[string]bool{}
	for _, e := range append(append([]string{}, ext.Rules...), ext.Names...) {
		m[filepath.ToSlash(e)] = true
	}
	return m
}
