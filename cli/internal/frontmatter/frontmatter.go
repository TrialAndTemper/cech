// Package frontmatter reads the YAML frontmatter at the top of a
// Markdown file, such as SKILL.md, a reviewer agent or an eval case.
package frontmatter

import (
	"bytes"
	"errors"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// Doc is the YAML mapping at the top of a Markdown file, and the
// Markdown after it.
type Doc struct {
	// Fields are the mapping's values by key.
	Fields map[string]*yaml.Node
	// Keys are the mapping's keys in the order written.
	Keys []string
	// Body is the Markdown after the frontmatter.
	Body string
}

// Parse splits the Markdown in src into its frontmatter and body.
// CRLF line ends are read as LF, so the output is the same on every OS.
// The frontmatter is a YAML mapping between two lines of ---; a
// duplicate key in it is an error.
func Parse(src []byte) (*Doc, error) {
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	rest, ok := bytes.CutPrefix(src, []byte("---\n"))
	if !ok {
		return nil, errors.New("no frontmatter: the file must start with a line of ---")
	}
	var head, body []byte
	if h, b, found := bytes.Cut(rest, []byte("\n---\n")); found {
		head, body = h, b
	} else if h, found := bytes.CutSuffix(rest, []byte("\n---")); found {
		head = h
	} else {
		return nil, errors.New("the frontmatter has no closing line of ---")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(head, &doc); err != nil {
		return nil, fmt.Errorf("frontmatter: %w", err)
	}
	if len(doc.Content) == 0 {
		return nil, errors.New("the frontmatter is empty")
	}
	fields, keys, err := Mapping(doc.Content[0])
	if err != nil {
		return nil, fmt.Errorf("frontmatter: %w", err)
	}
	return &Doc{Fields: fields, Keys: keys, Body: string(body)}, nil
}

// Mapping returns the values of the YAML mapping node n by key, and its
// keys in the order written. It is an error if n is nil, as Fields[key]
// is for a missing key, or is not a mapping, or has a duplicate key.
func Mapping(n *yaml.Node) (map[string]*yaml.Node, []string, error) {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil, nil, errors.New("not a mapping")
	}
	fields := make(map[string]*yaml.Node, len(n.Content)/2)
	keys := make([]string, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		if _, dup := fields[key]; dup {
			return nil, nil, fmt.Errorf("duplicate key %q", key)
		}
		fields[key] = n.Content[i+1]
		keys = append(keys, key)
	}
	return fields, keys, nil
}

// Str returns the string value of key in d's frontmatter, or "" when it
// is missing. A value that YAML reads as another type, such as a number,
// is an error.
func (d *Doc) Str(key string) (string, error) {
	n, ok := d.Fields[key]
	if !ok {
		return "", nil
	}
	if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!str" {
		return "", fmt.Errorf("%s is not a string", key)
	}
	return n.Value, nil
}
