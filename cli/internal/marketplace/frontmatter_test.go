package marketplace

import (
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name, src, description, body, err string
	}{
		{"with a body", "---\ndescription: d\n---\n# T\n", "d", "# T\n", ""},
		{"CRLF read as LF", "---\r\ndescription: d\r\n---\r\n# T\r\n", "d", "# T\n", ""},
		{"closing line at the end", "---\ndescription: d\n---", "d", "", ""},
		{"no opening line", "description: d\n", "", "", "no frontmatter"},
		{"no closing line", "---\ndescription: d\n", "", "", "no closing line"},
		{"empty", "---\n\n---\n", "", "", "the frontmatter is empty"},
		{"not a mapping", "---\n- a\n---\n", "", "", "not a mapping"},
		{"a number", "---\ndescription: 1.10\n---\n", "", "", "description is not a string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, err := parseFrontmatter([]byte(tt.src))
			var description string
			if err == nil {
				description, err = str(fm.fields, "description")
			}
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Errorf("error %v, want one holding %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if description != tt.description || fm.body != tt.body {
				t.Errorf("description %q, body %q; want %q, %q", description, fm.body, tt.description, tt.body)
			}
		})
	}
}
