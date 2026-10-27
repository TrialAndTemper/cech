package marketplace

import (
	"bytes"
	"errors"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// frontmatter is the YAML mapping at the top of a Markdown file, such as
// SKILL.md or a reviewer agent, and the Markdown after it.
type frontmatter struct {
	fields map[string]*yaml.Node
	body   string
}

// parseFrontmatter splits a Markdown file into its frontmatter and body.
// CRLF line ends are read as LF, so the output is the same on every OS.
// The frontmatter is a YAML mapping between two lines of ---; a
// duplicate key in it is an error.
func parseFrontmatter(src []byte) (*frontmatter, error) {
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
	fields, err := mapping(doc.Content[0])
	if err != nil {
		return nil, fmt.Errorf("frontmatter: %w", err)
	}
	return &frontmatter{fields: fields, body: string(body)}, nil
}

// mapping returns the keys of a YAML mapping node and their values.
func mapping(n *yaml.Node) (map[string]*yaml.Node, error) {
	if n.Kind != yaml.MappingNode {
		return nil, errors.New("not a mapping")
	}
	fields := make(map[string]*yaml.Node, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		if _, dup := fields[key]; dup {
			return nil, fmt.Errorf("duplicate key %q", key)
		}
		fields[key] = n.Content[i+1]
	}
	return fields, nil
}

// str returns the string value of key, or "" when it is missing. A value
// that YAML reads as another type, such as a number, is an error.
func str(fields map[string]*yaml.Node, key string) (string, error) {
	n, ok := fields[key]
	if !ok {
		return "", nil
	}
	if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!str" {
		return "", fmt.Errorf("%s is not a string", key)
	}
	return n.Value, nil
}
