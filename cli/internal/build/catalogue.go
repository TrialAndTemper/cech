package build

import (
	"encoding/json/jsontext"
	"encoding/json/v2"

	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

// CataloguePath is the marketplace catalogue that Claude Code and Codex
// read.
const CataloguePath = ".claude-plugin/marketplace.json"

type catalogue struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Owner       marketplace.Owner `json:"owner"`
	Plugins     []entry           `json:"plugins"`
}

// entry is one plugin in the catalogue. Other fields, such as version,
// stay in the plugin's own plugin.json.
type entry struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// Catalogue returns the marketplace catalogue: its name, the namespace
// or a release's name, and its description and owner from
// data/marketplace.toml, and each plugin with its category taken from
// its folder (ADR 12 item 2). JSON allows no comments, so it has no line
// saying it is generated (ADR 18 item 4).
func Catalogue(m *marketplace.Marketplace, name string) []byte {
	c := catalogue{Name: name, Description: m.Data.Description, Owner: m.Data.Owner, Plugins: []entry{}}
	for _, p := range m.Plugins {
		c.Plugins = append(c.Plugins, entry{
			Name:        p.Name,
			Source:      "./" + p.Dir(),
			Description: p.Description,
			Category:    p.Category,
		})
	}
	// The catalogue is laid out and escaped as encoding/json wrote it:
	// <, > and & as they are, and U+2028 and U+2029 escaped. Invalid
	// UTF-8 becomes U+FFFD, so the encode cannot fail.
	out, _ := json.Marshal(c,
		jsontext.WithIndent("  "),
		jsontext.EscapeForJS(true),
		jsontext.AllowInvalidUTF8(true),
	)
	return append(out, '\n')
}
