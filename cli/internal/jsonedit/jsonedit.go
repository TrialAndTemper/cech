// Package jsonedit changes one member of a JSON object and leaves every
// other byte of the file as it was (ADR 18 item 6, ADR 15 item 3).
package jsonedit

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
)

// readOptions read JSON as encoding/json did: invalid UTF-8 in a string
// reads as U+FFFD rather than failing. jsontext refuses duplicate keys
// by default, and these options keep that.
var readOptions = jsontext.AllowInvalidUTF8(true)

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
	if err := verify(src, out, members, key, value); err != nil {
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
	s, err := decodeString(src[m.valueStart:m.valueEnd])
	if err != nil {
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
	if jsontext.Value(src[m.valueStart:m.valueEnd]).Kind() != '[' {
		return nil, false, fmt.Errorf("%q is not an array", key)
	}
	// parse has checked the whole file, so reading the array cannot fail.
	dec := jsontext.NewDecoder(bytes.NewReader(src[m.valueStart:m.valueEnd]), readOptions)
	if _, err := dec.ReadToken(); err != nil {
		return nil, false, err
	}
	var spans []Span
	for dec.PeekKind() != ']' {
		start, end, err := readValue(dec)
		if err != nil {
			return nil, false, err
		}
		spans = append(spans, Span{m.valueStart + start, m.valueStart + end})
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

// encodeString encodes s as a JSON string as encoding/json did with
// HTML escaping off: <, > and & as they are, and U+2028 and U+2029
// escaped. It refuses invalid UTF-8, which would not read back as s.
func encodeString(s string) ([]byte, error) {
	return json.Marshal(s, jsontext.EscapeForJS(true))
}

// decodeString reads a JSON string. It refuses any value that is not a
// string, null included, which a plain Unmarshal into a string would
// read as "".
func decodeString(raw []byte) (string, error) {
	if jsontext.Value(raw).Kind() != '"' {
		return "", errors.New("not a string")
	}
	var s string
	err := json.Unmarshal(raw, &s, readOptions)
	return s, err
}

// readValue reads the next value from dec, a name or any other, and
// returns where it starts and ends in dec's input.
func readValue(dec *jsontext.Decoder) (start, end int, err error) {
	raw, err := dec.ReadValue()
	if err != nil {
		return 0, 0, err
	}
	end = int(dec.InputOffset())
	return end - len(raw), end, nil
}

// parse checks that src holds one JSON object with no duplicate key in
// any object, and locates the members of the top-level one.
func parse(src []byte) ([]member, error) {
	if err := check(src); err != nil {
		return nil, err
	}
	// check has read the whole file, so reading it again cannot fail.
	dec := jsontext.NewDecoder(bytes.NewReader(src), readOptions)
	if _, err := dec.ReadToken(); err != nil {
		return nil, err
	}
	var members []member
	// prev is where the last token ended.
	prev := int(dec.InputOffset())
	for dec.PeekKind() != '}' {
		keyStart, keyEnd, err := readValue(dec)
		if err != nil {
			return nil, err
		}
		key, err := decodeString(src[keyStart:keyEnd])
		if err != nil {
			return nil, err
		}
		valueStart, valueEnd, err := readValue(dec)
		if err != nil {
			return nil, err
		}
		// Between the end of the previous token and the key lie a comma,
		// for every member but the first, and whitespace.
		leadStart := prev
		if i := bytes.IndexByte(src[prev:keyStart], ','); i >= 0 {
			leadStart = prev + i + 1
		}
		members = append(members, member{
			key:        key,
			valueStart: valueStart,
			valueEnd:   valueEnd,
			lead:       src[leadStart:keyStart],
			colon:      src[keyEnd:valueStart],
		})
		prev = valueEnd
	}
	return members, nil
}

// check refuses src unless it holds one JSON value, an object, with no
// duplicate key in any object.
func check(src []byte) error {
	dec := jsontext.NewDecoder(bytes.NewReader(src), readOptions)
	value, err := dec.ReadValue()
	// value holds only until the next read, so take its kind first.
	kind := value.Kind()
	if err == nil {
		// Anything but whitespace after the value.
		if _, next := dec.ReadToken(); !errors.Is(next, io.EOF) {
			return errors.New("not valid JSON: text after the value")
		}
	}
	var serr *jsontext.SyntacticError
	switch {
	case errors.Is(err, jsontext.ErrDuplicateName) && errors.As(err, &serr):
		return fmt.Errorf("duplicate key %q", serr.JSONPointer.LastToken())
	case errors.As(err, &serr):
		return fmt.Errorf("not valid JSON: %w at offset %d", serr.Err, serr.ByteOffset)
	case errors.Is(err, io.EOF):
		return errors.New("not valid JSON: no value")
	case err != nil:
		return fmt.Errorf("not valid JSON: %w", err)
	case kind != '{':
		return errors.New("not a JSON object")
	}
	return nil
}

// verify reads out back: key must hold value, and every other member of
// the top-level object, as given in members, must be as in src.
func verify(src, out []byte, members []member, key, value string) error {
	edited, err := parse(out)
	if err != nil {
		return err
	}
	m, ok := find(edited, key)
	if !ok {
		return fmt.Errorf("%q is missing", key)
	}
	if got, err := decodeString(out[m.valueStart:m.valueEnd]); err != nil || got != value {
		return fmt.Errorf("%q does not hold the new value", key)
	}
	kept := 0
	for _, b := range members {
		if b.key == key {
			continue
		}
		a, ok := find(edited, b.key)
		if !ok || !bytes.Equal(src[b.valueStart:b.valueEnd], out[a.valueStart:a.valueEnd]) {
			return fmt.Errorf("members other than %q changed", key)
		}
		kept++
	}
	if kept != len(edited)-1 {
		return fmt.Errorf("members other than %q changed", key)
	}
	return nil
}
