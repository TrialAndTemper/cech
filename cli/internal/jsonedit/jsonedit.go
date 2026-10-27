// Package jsonedit changes one member of a JSON object and leaves every
// other byte of the file as it was (ADR 18 item 6, ADR 15 item 3).
package jsonedit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
)

// member is one member of the top-level object, located in the source.
type member struct {
	key string
	// valueStart and valueEnd bound the value.
	valueStart, valueEnd int
	// lead is the whitespace before the key, colon what lies between the
	// key and the value, such as ": ".
	lead, colon []byte
}

// SetString sets the string member key of the top-level object in src to
// value, and returns the new file. An existing member keeps its place.
// A missing one goes right after the member named after, in the layout
// of that member: the same whitespace before the key and around the
// colon. It refuses src when it is not one JSON object, when any object
// in it has a duplicate key, or when there is no member named after to
// place a new member by.
func SetString(src []byte, key, value, after string) ([]byte, error) {
	members, err := parse(src)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeString(value)
	if err != nil {
		return nil, err
	}
	var out []byte
	if m, ok := find(members, key); ok {
		out = splice(src, m.valueStart, m.valueEnd, encoded)
	} else {
		anchor, ok := find(members, after)
		if !ok {
			return nil, fmt.Errorf("no %q member to place %q after", after, key)
		}
		quoted, err := encodeString(key)
		if err != nil {
			return nil, err
		}
		var ins []byte
		ins = append(ins, ',')
		ins = append(ins, anchor.lead...)
		ins = append(ins, quoted...)
		ins = append(ins, anchor.colon...)
		ins = append(ins, encoded...)
		out = splice(src, anchor.valueEnd, anchor.valueEnd, ins)
	}
	if err := verify(src, out, key, value); err != nil {
		return nil, fmt.Errorf("the edit did not read back: %w", err)
	}
	return out, nil
}

// String returns the string member key of the top-level object in src,
// and whether it is there; a member of another type is an error.
func String(src []byte, key string) (string, bool, error) {
	members, err := parse(src)
	if err != nil {
		return "", false, err
	}
	m, ok := find(members, key)
	if !ok {
		return "", false, nil
	}
	// json.Unmarshal would read null as "".
	var s string
	if src[m.valueStart] != '"' || json.Unmarshal(src[m.valueStart:m.valueEnd], &s) != nil {
		return "", false, fmt.Errorf("%q is not a string", key)
	}
	return s, true, nil
}

// Span is where a value lies in a file: src[Start:End].
type Span struct{ Start, End int }

// Elements returns the span of each element of the array member key of
// the top-level object in src, and whether the member is there; a
// member of another type is an error. Edit an element with SetString on
// src[Start:End], and edit the last first, so the spans before it hold.
func Elements(src []byte, key string) ([]Span, bool, error) {
	members, err := parse(src)
	if err != nil {
		return nil, false, err
	}
	m, ok := find(members, key)
	if !ok {
		return nil, false, nil
	}
	if src[m.valueStart] != '[' {
		return nil, false, fmt.Errorf("%q is not an array", key)
	}
	value := src[:m.valueEnd]
	dec := json.NewDecoder(bytes.NewReader(value[m.valueStart:]))
	if _, err := dec.Token(); err != nil {
		return nil, false, err
	}
	var spans []Span
	// As in parse, prev is taken before dec.More moves the offset.
	prev := m.valueStart + int(dec.InputOffset())
	for dec.More() {
		start := prev + len(value[prev:]) - len(bytes.TrimLeft(value[prev:], " \t\r\n,"))
		if err := skipValue(dec); err != nil {
			return nil, false, err
		}
		prev = m.valueStart + int(dec.InputOffset())
		spans = append(spans, Span{start, prev})
	}
	return spans, true, nil
}

func find(members []member, key string) (member, bool) {
	for _, m := range members {
		if m.key == key {
			return m, true
		}
	}
	return member{}, false
}

func splice(src []byte, start, end int, with []byte) []byte {
	out := make([]byte, 0, len(src)-(end-start)+len(with))
	out = append(out, src[:start]...)
	out = append(out, with...)
	return append(out, src[end:]...)
}

// encodeString encodes s as a JSON string, leaving <, > and & as they
// are, which json.Marshal would escape for HTML.
func encodeString(s string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// parse checks that src holds one JSON object with no duplicate key in
// any object, and locates the members of the top-level one.
func parse(src []byte) ([]member, error) {
	if !json.Valid(src) {
		return nil, errors.New("not valid JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(src))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if tok != json.Delim('{') {
		return nil, errors.New("not a JSON object")
	}
	var members []member
	seen := map[string]bool{}
	// prev is where the last token ended. It is taken before dec.More,
	// which moves the offset past the whitespace it peeks over.
	prev := int(dec.InputOffset())
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected token %v", tok)
		}
		if seen[key] {
			return nil, fmt.Errorf("duplicate key %q", key)
		}
		seen[key] = true
		keyEnd := int(dec.InputOffset())
		// Between the end of the previous token and the key lie a comma,
		// for every member but the first, and whitespace.
		keyStart := bytes.IndexByte(src[prev:keyEnd], '"') + prev
		leadStart := prev
		if i := bytes.IndexByte(src[prev:keyStart], ','); i >= 0 {
			leadStart = prev + i + 1
		}
		colon := bytes.IndexByte(src[keyEnd:], ':') + keyEnd
		valueStart := colon + 1 + len(src[colon+1:]) - len(bytes.TrimLeft(src[colon+1:], " \t\r\n"))
		if err := skipValue(dec); err != nil {
			return nil, err
		}
		prev = int(dec.InputOffset())
		members = append(members, member{
			key:        key,
			valueStart: valueStart,
			valueEnd:   prev,
			lead:       src[leadStart:keyStart],
			colon:      src[keyEnd:valueStart],
		})
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("more than one JSON value")
	}
	return members, nil
}

// skipValue reads one value from dec, failing on a duplicate key in any
// object inside it.
func skipValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	switch tok {
	case json.Delim('{'):
		keys := map[string]bool{}
		for dec.More() {
			k, kerr := dec.Token()
			if kerr != nil {
				return kerr
			}
			key, _ := k.(string)
			if keys[key] {
				return fmt.Errorf("duplicate key %q", key)
			}
			keys[key] = true
			if verr := skipValue(dec); verr != nil {
				return verr
			}
		}
		_, err = dec.Token()
		return err
	case json.Delim('['):
		for dec.More() {
			if verr := skipValue(dec); verr != nil {
				return verr
			}
		}
		_, err = dec.Token()
		return err
	default:
		return nil
	}
}

// verify reads out back: key must hold value, and every other member
// must be as in src.
func verify(src, out []byte, key, value string) error {
	var before, after map[string]json.RawMessage
	if err := json.Unmarshal(src, &before); err != nil {
		return err
	}
	if err := json.Unmarshal(out, &after); err != nil {
		return err
	}
	var got string
	if err := json.Unmarshal(after[key], &got); err != nil || got != value {
		return fmt.Errorf("%q does not hold the new value", key)
	}
	delete(before, key)
	delete(after, key)
	if !maps.EqualFunc(before, after, func(a, b json.RawMessage) bool { return bytes.Equal(a, b) }) {
		return fmt.Errorf("members other than %q changed", key)
	}
	return nil
}
