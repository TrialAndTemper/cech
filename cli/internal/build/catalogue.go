package build

import (
	"bytes"
	"encoding/json"

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

// Catalogue returns the marketplace catalogue: its name, the namespace,
// and its description and owner from data/marketplace.toml, and each
// plugin with its category taken from its folder (ADR 12 item 2). JSON
// allows no comments, so it has no line saying it is generated (ADR 18
// item 4).
func Catalogue(m *marketplace.Marketplace) []byte {
	c := catalogue{Name: m.Data.Namespace, Description: m.Data.Description, Owner: m.Data.Owner, Plugins: []entry{}}
	for _, p := range m.Plugins {
		c.Plugins = append(c.Plugins, entry{
			Name:        p.Name,
			Source:      "./" + p.Dir(),
			Description: p.Description,
			Category:    p.Category,
		})
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	// Strings, and a struct of them, always encode.
	_ = enc.Encode(c)
	return buf.Bytes()
}
