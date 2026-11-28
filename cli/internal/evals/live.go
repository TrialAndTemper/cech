package evals

import (
	"bufio"
	"bytes"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// Harness names, as files, messages and the runner use them.
const (
	HarnessClaude = harnessClaude
	HarnessCodex  = harnessCodex
)

// Harnesses are both harnesses, in the order runs and reports take them.
func Harnesses() []string { return append([]string(nil), harnesses...) }

// ErrNoEnd is returned by Trim for a trace without the line that ends a
// run, which makes the run one that errors (eval-runs spec criterion 9).
var ErrNoEnd = errors.New("the trace has no last line: no result line on claude-code, no turn.completed on codex")

// Trim reduces the raw output of a run on harness, from claude -p
// --output-format stream-json or codex exec --json, to what a recording
// keeps (the evals reference, Recordings), and scrubs it as a recording
// must be, so the run can be graded by the same code as a recording and
// attached to a release (eval-runs spec criteria 11 and 26). Lines,
// keys and blocks a recording leaves out are dropped.
func Trim(harness string, raw []byte) ([]byte, error) {
	keep := trimClaudeLine
	last := "result"
	if harness == harnessCodex {
		keep, last = trimCodexLine, "turn.completed"
	}
	var out bytes.Buffer
	ended := false
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	for sc.Scan() {
		if ended {
			break
		}
		var obj map[string]any
		if json.Unmarshal(sc.Bytes(), &obj) != nil {
			continue
		}
		typ, _ := obj["type"].(string)
		kept, ok := keep(typ, obj)
		if !ok {
			continue
		}
		line, err := json.Marshal(Scrub(kept), json.Deterministic(true))
		if err != nil {
			return nil, err
		}
		out.Write(line)
		out.WriteByte('\n')
		ended = typ == last
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if !ended {
		return out.Bytes(), ErrNoEnd
	}
	return out.Bytes(), nil
}

// pick returns the keys of m that are in keys.
func pick(m map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return out
}

func trimClaudeLine(typ string, obj map[string]any) (map[string]any, bool) {
	msg, _ := obj["message"].(map[string]any)
	switch typ {
	case "assistant", "user":
		if msg == nil {
			return nil, false
		}
		blocks, _ := msg["content"].([]any)
		var keptBlocks []any
		for _, b := range blocks {
			bm, _ := b.(map[string]any)
			switch t, _ := bm["type"].(string); {
			case typ == "assistant" && t == "text":
				keptBlocks = append(keptBlocks, pick(bm, "type", "text"))
			case typ == "assistant" && t == "tool_use":
				keptBlocks = append(keptBlocks, pick(bm, "type", "id", "name", "input"))
			case typ == "user" && t == "tool_result":
				keptBlocks = append(keptBlocks, pick(bm, "type", "tool_use_id", "is_error"))
			}
		}
		if len(keptBlocks) == 0 {
			return nil, false
		}
		m := pick(msg, "role")
		m["content"] = keptBlocks
		return map[string]any{"type": typ, "message": m}, true
	case "result":
		out := pick(obj, "type", "subtype", "is_error", "result")
		if _, ok := out["result"].(string); !ok {
			out["result"] = ""
		}
		return out, true
	}
	return nil, false
}

func trimCodexLine(typ string, obj map[string]any) (map[string]any, bool) {
	switch typ {
	case "item.completed":
		item, _ := obj["item"].(map[string]any)
		switch t, _ := item["type"].(string); t {
		case "agent_message":
			return map[string]any{"type": typ, "item": pick(item, "type", "text")}, true
		case "command_execution":
			return map[string]any{"type": typ, "item": pick(item, "type", "command", "aggregated_output", "exit_code", "status")}, true
		}
	case "turn.completed":
		return map[string]any{"type": typ}, true
	}
	return nil, false
}

// scrubs maps each local path prefix to the placeholder that stands for
// it in a run's container, where the home is /cech/home: anything a run
// writes under /tmp/ or a home folder is written under that home.
var scrubs = strings.NewReplacer(
	"/home/", placeholderHome+"/",
	"/Users/", placeholderHome+"/",
	"/private/", placeholderHome+"/private/",
	"/var/folders/", placeholderHome+"/var/folders/",
	"/tmp/", placeholderHome+"/tmp/",
)

// Scrub rewrites every string in v that holds a local path, so a run's
// trace passes the check of the evals reference that recordings pass.
// A run's container mounts its workspace, plugin and home at the
// placeholders, so most paths need nothing; the rest are put under the
// home placeholder.
func Scrub(v any) any {
	switch v := v.(type) {
	case string:
		if localPath(v) {
			s := scrubs.Replace(v)
			if localPath(s) {
				return "[path removed]"
			}
			return s
		}
		return v
	case []any:
		for i, e := range v {
			v[i] = Scrub(e)
		}
		return v
	case map[string]any:
		for k, e := range v {
			v[k] = Scrub(e)
		}
		return v
	}
	return v
}

// Verdict is how a run was graded: a content or control run passes or
// not, and a trigger run loaded its skill or not.
type Verdict struct {
	Family string
	Pass   bool
	// Loaded is set for a trigger run.
	Loaded bool
	// Failed names the graders that failed.
	Failed []string
}

// ResultFiles are the files cech eval writes in a run folder of its
// results beside the recording (docs/reference/evals.md, Run results and
// reports), which GradeRun takes.
var ResultFiles = []string{"loaded.json", "run.json", "telemetry.json", "key.json"}

// GradeRun grades the run folder runDir, which holds a recording as the
// evals reference describes it (trace.jsonl, and workspace/ and
// created.txt), on harness, for the case in caseDir of the plugin whose
// SKILL.md is skillPath, all in fsys. It reads the case's cech.toml and
// graders as cech eval --check does, and grades the run by the same
// code (eval-runs spec criterion 11). A case or a run that breaks the
// rules of the reference is an error naming its findings.
func GradeRun(fsys fs.FS, skillPath, plugin, caseDir, runDir, harness string) (Verdict, error) {
	c := &checker{fsys: fsys, names: map[string]string{}, families: map[string]map[string]int{}, unjudged: map[string]bool{}, skills: map[string]skillID{}}
	meta, err := c.readMeta(caseDir)
	if err != nil {
		return Verdict{}, err
	}
	var graders []*grader
	gradable := false
	if meta != nil {
		graders, gradable = c.checkGraders(caseDir, meta)
	}
	if meta == nil || !gradable || len(c.res.Findings) > 0 {
		return Verdict{}, findingsError(c.res.Findings)
	}
	r, finds, err := readRecording(fsys, runDir, harness, ResultFiles...)
	if err != nil {
		return Verdict{}, err
	}
	if len(finds) > 0 {
		return Verdict{}, findingsError(finds)
	}
	v := Verdict{Family: meta.Family}
	if meta.Family == "trigger" {
		src, err := fs.ReadFile(fsys, skillPath)
		if err != nil {
			return Verdict{}, err
		}
		nameLine, descLine := skillLines(string(src))
		v.Loaded = r.loads(skillID{plugin: plugin, name: plugin, nameLine: nameLine, descLine: descLine})
		return v, nil
	}
	for _, g := range graders {
		if !g.grade(r) {
			v.Failed = append(v.Failed, g.name)
		}
	}
	v.Pass = len(v.Failed) == 0
	return v, nil
}

func findingsError(finds []Finding) error {
	if len(finds) == 0 {
		return errors.New("the case cannot be graded")
	}
	msgs := make([]string, len(finds))
	for i, f := range finds {
		msgs[i] = fmt.Sprintf("%s: %s", path.Clean(f.Path), f.Message)
	}
	return errors.New(strings.Join(msgs, "; "))
}
