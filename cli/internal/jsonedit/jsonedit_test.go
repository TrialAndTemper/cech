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
			"added after name as the last member",
			"{\n  \"version\": \"1.0.0\",\n  \"name\": \"acme-tracker\"\n}\n",
			"Tracks work.",
			"{\n  \"version\": \"1.0.0\",\n  \"name\": \"acme-tracker\",\n  \"description\": \"Tracks work.\"\n}\n",
		},
		{
			"CRLF and tabs kept",
			"{\r\n\t\"name\": \"acme-tracker\",\r\n\t\"version\": \"1.0.0\"\r\n}\r\n",
			"Tracks work.",
			"{\r\n\t\"name\": \"acme-tracker\",\r\n\t\"description\": \"Tracks work.\",\r\n\t\"version\": \"1.0.0\"\r\n}\r\n",
		},
		{
			"odd spacing around the colon copied from name",
			"{ \"license\" :\"MIT\",   \"name\"  :   \"acme-tracker\" ,\"version\":\"1.0.0\"}",
			"Tracks work.",
			"{ \"license\" :\"MIT\",   \"name\"  :   \"acme-tracker\",   \"description\"  :   \"Tracks work.\" ,\"version\":\"1.0.0\"}",
		},
		{
			"minified",
			`{"name":"acme-tracker","author":{"name":"Acme"}}`,
			"Tracks work.",
			`{"name":"acme-tracker","description":"Tracks work.","author":{"name":"Acme"}}`,
		},
		{
			"existing value replaced in place",
			"{\n  \"description\":   \"Old.\" ,\n  \"name\": \"acme-tracker\"\n}",
			"New.",
			"{\n  \"description\":   \"New.\" ,\n  \"name\": \"acme-tracker\"\n}",
		},
		{
			"existing value of the same text rewritten in the canonical form",
			"{\"name\": \"x\", \"description\": \"caf\\u00e9\"}",
			"café",
			"{\"name\": \"x\", \"description\": \"café\"}",
		},
		{
			"HTML characters, quotes and newlines encoded plainly; no space before a first member",
			"{\"name\": \"x\"}",
			"Use <b> & \"quotes\"\nnext",
			"{\"name\": \"x\",\"description\": \"Use <b> & \\\"quotes\\\"\\nnext\"}",
		},
		{
			"nested description untouched",
			"{\"author\": {\"description\": \"keep\"}, \"name\": \"x\"}",
			"Top.",
			"{\"author\": {\"description\": \"keep\"}, \"name\": \"x\", \"description\": \"Top.\"}",
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
		{"an array", `["name"]`, "not a JSON object"},
		{"a duplicate top-level key", `{"name": "x", "name": "y"}`, `duplicate key "name"`},
		{"a duplicate nested key", `{"name": "x", "author": {"a": 1, "a": 2}}`, `duplicate key "a"`},
		{"a duplicate key in an array", `{"name": "x", "k": [{"a": 1, "a": 2}]}`, `duplicate key "a"`},
		{"no member to place after", `{"version": "1.0.0"}`, `no "name" member`},
		{"an empty object", `{}`, `no "name" member`},
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

func TestString(t *testing.T) {
	src := []byte(`{"name": "acme-tracker", "version": 1, "license": null}`)
	if got, ok, err := String(src, "name"); err != nil || !ok || got != "acme-tracker" {
		t.Errorf("name: %q, %v, %v", got, ok, err)
	}
	if _, ok, err := String(src, "homepage"); err != nil || ok {
		t.Errorf("homepage: %v, %v; want absent", ok, err)
	}
	if _, _, err := String(src, "version"); err == nil {
		t.Error("version: no error for a number")
	}
	if _, _, err := String(src, "license"); err == nil {
		t.Error("license: no error for null")
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
		{"minified", `{"deps":["a",{"n":1},[2,3],null]}`, []string{`"a"`, `{"n":1}`, `[2,3]`, `null`}, true},
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
