// Package build generates the files of a marketplace with CECH's layout
// from their hand-written sources (ADR 18): the description in each
// plugin.json, the catalogue, Codex TOML for reviewer agents, the code
// owners file and the catalogue page. Each output has its own file
// here, so a later change to one leaves the others alone.
package build

import (
	"errors"
	"path"
	"slices"
	"strings"

	"github.com/TrialAndTemper/cech/cli/internal/harness/codex"
	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

// Output is one generated file: its path, relative to the marketplace
// root with / between names, and its content. A nil Content means the
// file must not exist, as for the Codex TOML of a removed agent, or,
// with a Mark, that a file starting with Mark must not.
type Output struct {
	Path    string
	Content []byte
	// Mark is how a file the build generated starts. With a nil
	// Content, a file that starts otherwise was written by hand and
	// stays.
	Mark []byte
}

// Outputs returns every generated file of m, sorted by path. Its error
// names each file it could not generate.
func Outputs(m *marketplace.Marketplace) ([]Output, error) {
	outs := []Output{
		{Path: CataloguePath, Content: Catalogue(m)},
		{Path: PagePath, Content: Page(m)},
	}
	// With no team there is no code owners file (ADR 13 item 4): the
	// build removes one it generated and leaves one written by hand.
	if m.Data.Team != "" {
		outs = append(outs, Output{Path: CodeOwnersPath, Content: CodeOwners(m)})
	} else {
		outs = append(outs, Output{Path: CodeOwnersPath, Mark: []byte(codeOwnersMark)})
	}
	var errs []error
	for _, p := range m.Plugins {
		manifest, err := Manifest(p, m.Data.CECHCatalogue())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		outs = append(outs, Output{Path: p.ManifestPath(), Content: manifest})
		outs = append(outs, agents(p)...)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	slices.SortFunc(outs, func(a, b Output) int { return strings.Compare(a.Path, b.Path) })
	return outs, nil
}

// agents returns the Codex TOML of each reviewer agent of p. The build
// owns every .toml file in agents/ (ADR 18 item 2), so one without its
// Markdown is removed.
func agents(p marketplace.Plugin) []Output {
	dir := path.Join(p.Dir(), "agents")
	var outs []Output
	keep := map[string]bool{}
	for _, a := range p.Agents {
		file := a.File + ".toml"
		keep[file] = true
		outs = append(outs, Output{
			Path:    path.Join(dir, file),
			Content: codex.Agent(a.File+".md", a.Name, a.Description, a.Instructions),
		})
	}
	for _, file := range p.AgentTOML {
		if !keep[file] {
			outs = append(outs, Output{Path: path.Join(dir, file)})
		}
	}
	return outs
}
