package frontmatter

import (
	"slices"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name, src, description, body, err string
	}{
		{"with a body", "---\ndescription: d\n---\n# T\n", "d", "# T\n", ""},
		{"CRLF read as LF", "---\r\ndescription: d\r\n---\r\n# T\r\n", "d", "# T\n", ""},
		{"closing line at the end", "---\ndescription: d\n---", "d", "", ""},
		{"missing key read as empty", "---\nname: n\n---\n", "", "", ""},
		{"no opening line", "description: d\n", "", "", "no frontmatter"},
		{"no closing line", "---\ndescription: d\n", "", "", "no closing line"},
		{"empty", "---\n\n---\n", "", "", "the frontmatter is empty"},
		{"not a mapping", "---\n- a\n---\n", "", "", "not a mapping"},
		{"duplicate key", "---\ndescription: d\ndescription: e\n---\n", "", "", `duplicate key "description"`},
		{"a number", "---\ndescription: 1.10\n---\n", "", "", "description is not a string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, err := Parse([]byte(tt.src))
			var description string
			if err == nil {
				description, err = fm.Str("description")
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
			if description != tt.description || fm.Body != tt.body {
				t.Errorf("description %q, body %q; want %q, %q", description, fm.Body, tt.description, tt.body)
			}
		})
	}
}

func TestParseKeepsKeyOrder(t *testing.T) {
	fm, err := Parse([]byte("---\nname: n\ndescription: d\nmodel: m\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"name", "description", "model"}; !slices.Equal(fm.Keys, want) {
		t.Errorf("keys %q, want %q", fm.Keys, want)
	}
}

func TestMappingNil(t *testing.T) {
	if _, _, err := Mapping(nil); err == nil || !strings.Contains(err.Error(), "not a mapping") {
		t.Errorf("error %v, want one holding %q", err, "not a mapping")
	}
}

func TestMapping(t *testing.T) {
	tests := []struct {
		name, target string
		keys, values []string
		err          string
	}{
		{"keys in the order written", "{b: 1, a: 2}", []string{"b", "a"}, []string{"1", "2"}, ""},
		{"duplicate key", "{b: 1, b: 2}", nil, nil, `duplicate key "b"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, err := Parse([]byte("---\ntarget: " + tt.target + "\n---\n"))
			if err != nil {
				t.Fatal(err)
			}
			fields, keys, err := Mapping(fm.Fields["target"])
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Errorf("error %v, want one holding %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(keys, tt.keys) || len(fields) != len(tt.keys) {
				t.Fatalf("keys %q, %d fields; want %q", keys, len(fields), tt.keys)
			}
			for i, key := range tt.keys {
				if got := fields[key].Value; got != tt.values[i] {
					t.Errorf("%s is %q, want %q", key, got, tt.values[i])
				}
			}
		})
	}
}
