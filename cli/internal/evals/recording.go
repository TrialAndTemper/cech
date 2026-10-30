package evals

import (
	"bufio"
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

// The harnesses a case is recorded on, as files and messages name them.
const (
	harnessClaude = "claude-code"
	harnessCodex  = "codex"
)

var harnesses = []string{harnessClaude, harnessCodex}

// recording is one recorded run of a case on one harness (spec rules 20
// to 22), as its graders and the load detectors read it.
type recording struct {
	harness     string
	lastMessage string
	// toolUses are Claude Code's tool_use blocks.
	toolUses []toolUse
	results  map[string]bool // tool_use id to whether its result is an error
	// outputs are the aggregated_output of each of Codex's commands.
	outputs []string
	// created are the paths of the files the run created, sorted.
	created []string
	// files maps each path in workspace/ to its content.
	files map[string]string
}

// toolUse is one tool_use block: its id, the tool's name, and its input
// as the trace wrote it with whitespace outside strings removed.
type toolUse struct {
	id, name, input string
}

// The placeholders that stand for local paths in a recording (spec rule
// 22), all under a root no real system has, and the path prefixes that
// are local wherever they appear.
const (
	placeholderRoot      = "/cech/"
	placeholderWorkspace = "/cech/workspace"
	placeholderPlugin    = "/cech/plugin"
	placeholderHome      = "/cech/home"
)

var localPaths = []string{"/home/", "/Users/", "/tmp/", "/private/", "/var/folders/"}

// placeholderPaths finds a placeholder and the parts of the path it
// starts: each a / and characters a file name commonly holds, but not a
// .. part, so a separator such as : or , and a climb out of the
// placeholder end it.
var placeholderPaths = regexp.MustCompile(`(?:` + regexp.QuoteMeta(placeholderWorkspace) + `|` + regexp.QuoteMeta(placeholderPlugin) + `|` + regexp.QuoteMeta(placeholderHome) + `)(?:/\.?[A-Za-z0-9_@+~-][A-Za-z0-9._@+~-]*)*`)

// localPath reports whether s holds a local path once each placeholder,
// with the parts of the path it starts, is removed: /cech/home and
// /cech/workspace/tmp/x are no local paths, but /tmp/x/cech/workspace
// and /cech/home/bin:/home/me are.
func localPath(s string) bool {
	s = placeholderPaths.ReplaceAllString(s, "\x00")
	for _, p := range localPaths {
		if strings.Contains(s, p) {
			return true
		}
	}
	// A drive letter starts a path only where no word goes on before it,
	// so a regex such as name:\s in a command is no path.
	for i := 1; i+1 < len(s); i++ {
		c := s[i-1] | 0x20
		if s[i] == ':' && s[i+1] == '\\' && 'a' <= c && c <= 'z' && (i == 1 || !wordByte(s[i-2])) {
			return true
		}
	}
	return false
}

// wordByte reports whether b is an ASCII letter, digit or _.
func wordByte(b byte) bool {
	c := b | 0x20
	return 'a' <= c && c <= 'z' || '0' <= b && b <= '9' || b == '_'
}

// readRecording reads the recording folder dir of a run on harness. It
// returns every problem it finds; a recording with problems is not
// graded.
func readRecording(fsys fs.FS, dir, harness string) (*recording, []Finding, error) {
	r := &recording{harness: harness, files: map[string]string{}, results: map[string]bool{}}
	var finds []Finding
	bad := func(p, format string, a ...any) { finds = append(finds, Finding{p, fmt.Sprintf(format, a...)}) }
	badAt := func(p string) func(string) { return func(msg string) { bad(p, "%s", msg) } }

	tracePath := path.Join(dir, "trace.jsonl")
	src, exists, err := readFile(fsys, tracePath, badAt(tracePath))
	switch {
	case err != nil:
		return nil, nil, err
	case !exists:
		bad(dir, "has no trace.jsonl")
	case src != nil:
		read := r.readClaudeTrace
		if harness == harnessCodex {
			read = r.readCodexTrace
		}
		for _, msg := range read(src) {
			bad(tracePath, "%s", msg)
		}
	}

	ws := path.Join(dir, "workspace")
	if info, serr := fs.Stat(fsys, ws); serr == nil && !info.IsDir() {
		bad(ws, "is not a folder")
	} else if werr := fs.WalkDir(fsys, ws, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && p == ws {
				return fs.SkipDir
			}
			return err
		}
		if p == ws {
			return nil
		}
		// A path read relative to workspace/ cannot hold a local one.
		rel := strings.TrimPrefix(p, ws+"/")
		switch {
		case d.IsDir():
			return nil
		case !d.Type().IsRegular():
			bad(p, "is not a file")
			return nil
		}
		content, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		r.files[rel] = string(content)
		return nil
	}); werr != nil {
		return nil, nil, marketplace.ReadError(werr)
	}

	// Without created.txt, every file in workspace/ counts as created;
	// an empty one records a run that created none.
	createdPath := path.Join(dir, "created.txt")
	created, exists, err := readFile(fsys, createdPath, badAt(createdPath))
	switch {
	case err != nil:
		return nil, nil, err
	case !exists:
		for p := range r.files {
			r.created = append(r.created, p)
		}
	case len(created) > 0:
		for line := range strings.SplitSeq(strings.TrimSuffix(strings.ReplaceAll(string(created), "\r\n", "\n"), "\n"), "\n") {
			_, inWorkspace := r.files[line]
			switch {
			case line == "":
				bad(createdPath, "has an empty line")
			case localPath(line):
				bad(createdPath, "%q holds a local path", line)
			case !inWorkspace:
				bad(createdPath, "%q is not a file in workspace/", line)
			default:
				r.created = append(r.created, line)
			}
		}
	}
	sort.Strings(r.created)
	r.created = slices.Compact(r.created)

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, nil, marketplace.ReadError(err)
	}
	for _, e := range entries {
		switch e.Name() {
		case "trace.jsonl", "created.txt", "workspace":
		default:
			bad(path.Join(dir, e.Name()), "is not trace.jsonl, workspace/ or created.txt")
		}
	}
	return r, finds, nil
}

// readLines reads trace.jsonl line by line, giving each JSON object and
// its type to line, which returns its problems, and whether the type is
// one the trim keeps (spec rule 21). The line of type last must end the
// trace. It returns each way the trace breaks spec rules 21 and 22.
func readLines(src []byte, last, keep string, line func(obj map[string]jsontext.Value, typ string) ([]string, bool)) []string {
	var probs []string
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	n, lasts := 0, 0
	for sc.Scan() {
		n++
		text := sc.Bytes()
		if len(bytes.TrimSpace(text)) == 0 {
			probs = append(probs, fmt.Sprintf("line %d is empty", n))
			continue
		}
		if lasts > 0 {
			probs = append(probs, fmt.Sprintf("line %d comes after the %s line, which must be the last", n, last))
		}
		for _, msg := range localStrings(text) {
			probs = append(probs, fmt.Sprintf("line %d: %s", n, msg))
		}
		// json/v2 reads strictly: a repeated name, invalid UTF-8 or a lone
		// surrogate makes the line no JSON object, so the error says which.
		var obj map[string]jsontext.Value
		if err := json.Unmarshal(text, &obj); err != nil {
			probs = append(probs, fmt.Sprintf("line %d is not a JSON object: %v", n, err))
			continue
		}
		var typ string
		_ = json.Unmarshal(obj["type"], &typ)
		if typ == last {
			lasts++
		}
		msgs, kept := line(obj, typ)
		if !kept {
			msgs = []string{fmt.Sprintf("has type %q; keep only %s", typ, keep)}
		}
		probs = append(probs, prefixed(n, msgs)...)
	}
	if err := sc.Err(); err != nil {
		probs = append(probs, err.Error())
	}
	if lasts == 0 {
		probs = append(probs, fmt.Sprintf("has no %s line, which must be the last", last))
	}
	return probs
}

// readClaudeTrace reads a trace of claude -p --output-format stream-json.
func (r *recording) readClaudeTrace(src []byte) []string {
	return readLines(src, "result", "assistant lines, user lines and the result line", func(obj map[string]jsontext.Value, typ string) ([]string, bool) {
		switch typ {
		case "assistant":
			return r.readAssistant(obj), true
		case "user":
			return r.readUser(obj), true
		case "result":
			return r.readResult(obj), true
		}
		return nil, false
	})
}

// readCodexTrace reads a trace of codex exec --json.
func (r *recording) readCodexTrace(src []byte) []string {
	return readLines(src, "turn.completed", "item.completed lines and the turn.completed line", func(obj map[string]jsontext.Value, typ string) ([]string, bool) {
		switch typ {
		case "item.completed":
			return r.readItem(obj), true
		case "turn.completed":
			return onlyKeys("the turn.completed line", obj, []string{"type"}, nil), true
		}
		return nil, false
	})
}

func prefixed(n int, msgs []string) []string {
	for i, m := range msgs {
		msgs[i] = fmt.Sprintf("line %d: %s", n, m)
	}
	return msgs
}

// onlyKeys returns a problem for each key of obj not in keys, and for
// each of required that obj lacks.
func onlyKeys(what string, obj map[string]jsontext.Value, keys, required []string) []string {
	var probs []string
	var names []string
	for k := range obj {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if !slices.Contains(keys, k) {
			probs = append(probs, fmt.Sprintf("%s has the key %q, which a recording leaves out", what, k))
		}
	}
	for _, k := range required {
		if _, ok := obj[k]; !ok {
			probs = append(probs, fmt.Sprintf("%s has no %q", what, k))
		}
	}
	return probs
}

func (r *recording) readAssistant(obj map[string]jsontext.Value) []string {
	probs := onlyKeys("an assistant line", obj, []string{"type", "message"}, []string{"message"})
	var msg map[string]jsontext.Value
	if err := json.Unmarshal(obj["message"], &msg); err != nil {
		return append(probs, "message is not an object")
	}
	probs = append(probs, onlyKeys("message", msg, []string{"role", "content"}, []string{"content"})...)
	var blocks []map[string]jsontext.Value
	if err := json.Unmarshal(msg["content"], &blocks); err != nil {
		return append(probs, "content is not a list of blocks")
	}
	for i, b := range blocks {
		var typ string
		_ = json.Unmarshal(b["type"], &typ)
		what := fmt.Sprintf("content block %d", i+1)
		switch typ {
		case "text":
			probs = append(probs, onlyKeys(what, b, []string{"type", "text"}, []string{"text"})...)
		case "tool_use":
			probs = append(probs, onlyKeys(what, b, []string{"type", "id", "name", "input"}, []string{"name", "input"})...)
			var name string
			if err := json.Unmarshal(b["name"], &name); err != nil {
				probs = append(probs, what+": name is not a string")
				continue
			}
			// Only the Claude Code load detector reads a tool input, but
			// one that is not JSON is a finding of its own.
			in := b["input"]
			if err := in.Compact(); err != nil {
				probs = append(probs, what+": input is not JSON")
				continue
			}
			var id string
			_ = json.Unmarshal(b["id"], &id)
			r.toolUses = append(r.toolUses, toolUse{id: id, name: name, input: in.String()})
		default:
			probs = append(probs, fmt.Sprintf("%s has type %q; keep only text and tool_use blocks", what, typ))
		}
	}
	return probs
}

func (r *recording) readResult(obj map[string]jsontext.Value) []string {
	probs := onlyKeys("the result line", obj, []string{"type", "subtype", "is_error", "result"}, []string{"result"})
	if raw, ok := obj["result"]; ok && (json.Unmarshal(raw, &r.lastMessage) != nil || string(bytes.TrimSpace(raw)) == "null") {
		probs = append(probs, "result is not a string")
	}
	return probs
}

// readUser reads a user line, which keeps only whether each tool's
// result is an error; a missing is_error means false, since the harness
// leaves the key out of a result that is not an error.
func (r *recording) readUser(obj map[string]jsontext.Value) []string {
	probs := onlyKeys("a user line", obj, []string{"type", "message"}, []string{"message"})
	var msg map[string]jsontext.Value
	if err := json.Unmarshal(obj["message"], &msg); err != nil {
		return append(probs, "message is not an object")
	}
	probs = append(probs, onlyKeys("message", msg, []string{"role", "content"}, []string{"content"})...)
	var blocks []map[string]jsontext.Value
	if err := json.Unmarshal(msg["content"], &blocks); err != nil {
		return append(probs, "content is not a list of blocks")
	}
	for i, b := range blocks {
		var typ string
		_ = json.Unmarshal(b["type"], &typ)
		what := fmt.Sprintf("content block %d", i+1)
		if typ != "tool_result" {
			probs = append(probs, fmt.Sprintf("%s has type %q; keep only tool_result blocks", what, typ))
			continue
		}
		probs = append(probs, onlyKeys(what, b, []string{"type", "tool_use_id", "is_error"}, []string{"tool_use_id"})...)
		var id string
		if err := json.Unmarshal(b["tool_use_id"], &id); err != nil {
			probs = append(probs, what+": tool_use_id is not a string")
			continue
		}
		isError := false
		if raw, ok := b["is_error"]; ok {
			if err := json.Unmarshal(raw, &isError); err != nil {
				probs = append(probs, what+": is_error is not a boolean")
				continue
			}
		}
		r.results[id] = isError
	}
	return probs
}

// readItem reads one of Codex's completed items: a message, the last of
// which is last_message, or a command, whose output the load detector
// reads.
func (r *recording) readItem(obj map[string]jsontext.Value) []string {
	probs := onlyKeys("an item.completed line", obj, []string{"type", "item"}, []string{"item"})
	var item map[string]jsontext.Value
	if err := json.Unmarshal(obj["item"], &item); err != nil {
		return append(probs, "item is not an object")
	}
	var typ string
	_ = json.Unmarshal(item["type"], &typ)
	switch typ {
	case "agent_message":
		probs = append(probs, onlyKeys("the item", item, []string{"type", "text"}, []string{"text"})...)
		if err := json.Unmarshal(item["text"], &r.lastMessage); err != nil {
			probs = append(probs, "the item's text is not a string")
		}
	case "command_execution":
		probs = append(probs, onlyKeys("the item", item, []string{"type", "command", "aggregated_output", "exit_code", "status"}, []string{"aggregated_output"})...)
		var out string
		if err := json.Unmarshal(item["aggregated_output"], &out); err != nil {
			probs = append(probs, "the item's aggregated_output is not a string")
		} else {
			r.outputs = append(r.outputs, out)
		}
	default:
		probs = append(probs, fmt.Sprintf("the item has type %q; keep only agent_message and command_execution items", typ))
	}
	return probs
}

// localStrings returns a problem for each string in the JSON line that
// holds a local path, or one for a line that is not JSON and holds one.
func localStrings(line []byte) []string {
	var v any
	if json.Unmarshal(line, &v) != nil {
		if localPath(string(line)) {
			return []string{"the line holds a local path"}
		}
		return nil
	}
	var probs []string
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case string:
			if localPath(v) {
				probs = append(probs, fmt.Sprintf("%q holds a local path; write the workspace as %s, the plugin as %s and the harness's home as %s", clip(v), placeholderWorkspace, placeholderPlugin, placeholderHome))
			}
		case []any:
			for _, e := range v {
				walk(e)
			}
		case map[string]any:
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(k)
				walk(v[k])
			}
		}
	}
	walk(v)
	return probs
}

// clip shortens s for a message.
func clip(s string) string {
	if r := []rune(s); len(r) > 60 {
		return string(r[:57]) + "..."
	}
	return s
}
