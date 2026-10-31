package jsonedit

import (
	"slices"
	"strings"
	"testing"
)

func TestSetString(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		value string
		want  string
	}{
		{
			"added after name, two-space indent",
			"{\n  \"name\": \"acme-tracker\",\n  \"version\": \"1.0.0\"\n}\n",
			"Tracks work.",
			"{\n  \"name\": \"acme-tracker\",\n  \"description\": \"Tracks work.\",\n  \"version\": \"1.0.0\"\n}\n",
		},
		{
			"odd spacing around the colon copied from name",
			"{ \"license\" :\"MIT\",   \"name\"  :   \"acme-tracker\" ,\"version\":\"1.0.0\"}",
			"Tracks work.",
			"{ \"license\" :\"MIT\",   \"name\"  :   \"acme-tracker\",   \"description\"  :   \"Tracks work.\" ,\"version\":\"1.0.0\"}",
		},
		{
			"existing value of the same text rewritten in the canonical form",
			"{\"name\": \"x\", \"description\": \"caf\\u00e9\"}",
			"café",
			"{\"name\": \"x\", \"description\": \"café\"}",
		},
		{
			"U+2028 and U+2029 escaped, as encoding/json wrote them",
			"{\"name\": \"x\"}",
			"a\u2028b\u2029c",
			"{\"name\": \"x\",\"description\": \"a\\u2028b\\u2029c\"}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetString([]byte(tt.src), "description", tt.value, "name")
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got\n%q\nwant\n%q", got, tt.want)
			}
			again, err := SetString(got, "description", tt.value, "name")
			if err != nil {
				t.Fatal(err)
			}
			if string(again) != string(got) {
				t.Errorf("a second edit changed the file:\n%q", again)
			}
		})
	}
}

func TestSetStringRefuses(t *testing.T) {
	tests := []struct {
		name, src, err string
	}{
		{"invalid JSON", `{"name": "x",}`, "not valid JSON"},
		{"empty", ``, "not valid JSON: no value"},
		{"two values", `{"name": "x"} {}`, "not valid JSON"},
		{"an array", `["name"]`, "not a JSON object"},
		{"a duplicate nested key", `{"name": "x", "author": {"a": 1, "a": 2}}`, `duplicate key "a"`},
		{"no member to place after", `{"version": "1.0.0"}`, `no "name" member`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SetString([]byte(tt.src), "description", "d", "name")
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Errorf("error %v, want one holding %q", err, tt.err)
			}
		})
	}
}

// TestSetStringEncodesTheKey checks that a new key is encoded, as its
// value is.
func TestSetStringEncodesTheKey(t *testing.T) {
	got, err := SetString([]byte(`{"name": "x"}`), "a\"<b>", "v", "name")
	if want := `{"name": "x","a\"<b>": "v"}`; err != nil || string(got) != want {
		t.Errorf("got %s, %v; want %s", got, err, want)
	}
}

// TestVerify checks that an edit that changes another member, drops
// one or adds one does not read back.
func TestVerify(t *testing.T) {
	src := []byte(`{"name": "x", "license": "MIT"}`)
	members, err := parse(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ name, out, err string }{
		{"another member changed", `{"name": "x", "license": "ISC", "description": "d"}`, "members other than"},
		{"a member dropped", `{"name": "x", "description": "d"}`, "members other than"},
		{"a member added", `{"name": "x", "license": "MIT", "description": "d", "extra": 1}`, "members other than"},
		{"the wrong value", `{"name": "x", "license": "MIT", "description": "e"}`, "does not hold the new value"},
		{"the key missing", `{"name": "x", "license": "MIT"}`, "is missing"},
	} {
		err := verify(src, []byte(tt.out), members, "description", "d")
		if err == nil || !strings.Contains(err.Error(), tt.err) {
			t.Errorf("%s: error %v, want one holding %q", tt.name, err, tt.err)
		}
	}
}

// TestReadersRefuseDuplicates checks that String and Elements refuse a
// file with a duplicate key, as SetString does.
func TestReadersRefuseDuplicates(t *testing.T) {
	src := []byte(`{"name": "x", "deps": [], "k": {"a": 1, "a": 2}}`)
	if _, _, err := String(src, "name"); err == nil || !strings.Contains(err.Error(), `duplicate key "a"`) {
		t.Errorf("String: error %v", err)
	}
	if _, _, err := Elements(src, "deps"); err == nil || !strings.Contains(err.Error(), `duplicate key "a"`) {
		t.Errorf("Elements: error %v", err)
	}
}

func TestString(t *testing.T) {
	src := []byte(`{"name": "acme-tracker", "license": null}`)
	if _, ok, err := String(src, "homepage"); err != nil || ok {
		t.Errorf("homepage: %v, %v; want absent", ok, err)
	}
	if _, _, err := String(src, "license"); err == nil {
		t.Error("license: no error for null")
	}
	odd := []byte("{\"n\\u0061me\": \"caf\\u00e9 \xff\"}")
	if got, ok, err := String(odd, "name"); err != nil || !ok || got != "café \uFFFD" {
		t.Errorf("escaped name: %q, %v, %v; want invalid UTF-8 read as U+FFFD", got, ok, err)
	}
}

func TestElements(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string // the text of each element
		ok   bool
	}{
		{"absent", `{"name": "x"}`, nil, false},
		{"empty", `{"deps": [ ]}`, nil, true},
		{"a number too large for a float64", `{"deps":[1e400,{"n":-1e400}]}`, []string{`1e400`, `{"n":-1e400}`}, true},
		{
			"spread over lines, with a comma inside a string",
			"{\r\n\t\"deps\": [\r\n\t\t\"a,b\" ,\r\n\t\t{ \"n\": \"]\" }\r\n\t],\r\n\t\"name\": \"x\"\r\n}",
			[]string{`"a,b"`, `{ "n": "]" }`},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spans, ok, err := Elements([]byte(tt.src), "deps")
			if err != nil || ok != tt.ok {
				t.Fatalf("ok %v, error %v; want %v and none", ok, err, tt.ok)
			}
			var got []string
			for _, s := range spans {
				got = append(got, tt.src[s.Start:s.End])
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("elements %q, want %q", got, tt.want)
			}
		})
	}
	if _, _, err := Elements([]byte(`{"deps": {"a": 1}}`), "deps"); err == nil || !strings.Contains(err.Error(), "not an array") {
		t.Errorf("an object: error %v, want one saying it is not an array", err)
	}
}
