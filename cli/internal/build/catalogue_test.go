package build

import (
	"testing"

	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

// TestCatalogue checks the layout and escaping the catalogue sets, as
// encoding/json wrote them: two-space indent, a final newline, <, > and
// & as they are, U+2028 and U+2029 escaped, and invalid UTF-8 as U+FFFD.
func TestCatalogue(t *testing.T) {
	m := &marketplace.Marketplace{Data: marketplace.Data{
		Description: "Tools <for> R&D\u2028and\u2029more \xff",
		Owner:       marketplace.Owner{Name: "Acme"},
	}, Plugins: []marketplace.Plugin{{Category: "tool", Name: "acme-git", Description: "Git <&> more"}}}
	want := `{
  "name": "acme",
  "description": "Tools <for> R&D\u2028and\u2029more ` + "\uFFFD" + `",
  "owner": {
    "name": "Acme"
  },
  "plugins": [
    {
      "name": "acme-git",
      "source": "./plugins/tool/acme-git",
      "description": "Git <&> more",
      "category": "tool"
    }
  ]
}
`
	if got := string(Catalogue(m, "acme")); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
