package evals

import (
	"errors"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

const skillMD = "---\nname: cech-example\ndescription: Use when writing a commit message. Ends it with the project's Change-Kind trailer.\n---\n\n# Commit trailer\n"

const contentCase = `family = "content"
expect = "better"
claim = "A commit message ends with a Change-Kind trailer."

[[graders]]
name = "reply-trailer"
type = "regex"
pattern = 'Change-Kind: fix\s*$'

[[graders]]
name = "file-written"
type = "file_exists"
path = "COMMIT_MSG"

[recordings.claude-code]
pass = ["a"]
fail = ["b"]

[recordings.codex]
pass = ["a"]
fail = ["b"]
`

const triggerCase = `family = "trigger"
expect = "load"
wording = "matched"

[recordings.claude-code]
pass = ["a"]
fail = ["b"]

[recordings.codex]
pass = ["a"]
fail = ["b"]
`

func trimmed(t *testing.T, harness string) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/raw/" + harness + ".jsonl")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Trim(harness, raw)
	if err != nil {
		t.Fatalf("Trim: %v", err)
	}
	return out
}

// TestTrimMakesARecording checks that a raw trace, trimmed, is one the
// recording reader takes without a finding: only the lines, keys and
// blocks a recording keeps, and no local path (criteria 11 and 26).
func TestTrimMakesARecording(t *testing.T) {
	for _, h := range harnesses {
		t.Run(h, func(t *testing.T) {
			out := trimmed(t, h)
			for _, gone := range []string{"session_id", "thread_id", "thinking", "rate_limit_event", "usage", "item.started", "reasoning", "\"/tmp/", " /tmp/", "/home/runner"} {
				if strings.Contains(string(out), gone) {
					t.Errorf("trimmed trace still holds %q:\n%s", gone, out)
				}
			}
			fsys := fstest.MapFS{"run/trace.jsonl": {Data: out}, "run/created.txt": {Data: nil}}
			if _, finds, err := readRecording(fsys, "run", h); err != nil || len(finds) > 0 {
				t.Errorf("readRecording: %v, findings %v\ntrace:\n%s", err, finds, out)
			}
		})
	}
}

// TestTrimNeedsTheLastLine checks that a run cut off before its last line
// is an error, which makes it a run that errors (criterion 9).
func TestTrimNeedsTheLastLine(t *testing.T) {
	for _, h := range harnesses {
		raw, err := os.ReadFile("testdata/raw/" + h + ".jsonl")
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
		cut := strings.Join(lines[:len(lines)-1], "\n")
		if _, err := Trim(h, []byte(cut)); !errors.Is(err, ErrNoEnd) {
			t.Errorf("%s: Trim of a cut trace: %v; want ErrNoEnd", h, err)
		}
	}
}

// TestScrub checks that a local path is put under the home placeholder,
// and that the placeholders stay as they are.
func TestScrub(t *testing.T) {
	for in, want := range map[string]string{
		"/tmp/x/COMMIT_MSG":        "/cech/home/tmp/x/COMMIT_MSG",
		"cat /home/runner/a":       "cat /cech/home/runner/a",
		"/cech/workspace/src/a.py": "/cech/workspace/src/a.py",
		"no path":                  "no path",
	} {
		if got := Scrub(in); got != want {
			t.Errorf("Scrub(%q) = %q; want %q", in, got, want)
		}
	}
	if got := Scrub(`C:\Users\x`); got != "[path removed]" {
		t.Errorf("a drive letter path: %q; want it removed", got)
	}
}

func runFS(t *testing.T, h, caseToml, commitMsg string) fstest.MapFS {
	t.Helper()
	fsys := fstest.MapFS{
		"plugins/writing/cech-example/skills/cech-example/SKILL.md": {Data: []byte(skillMD)},
		"evals/writing/cech-example/c/cech.toml":                    {Data: []byte(caseToml)},
		"run/trace.jsonl":                                           {Data: trimmed(t, h)},
	}
	if commitMsg != "" {
		fsys["run/workspace/COMMIT_MSG"] = &fstest.MapFile{Data: []byte(commitMsg)}
	} else {
		fsys["run/created.txt"] = &fstest.MapFile{Data: nil}
	}
	return fsys
}

// TestGradeRun checks that a live run is graded by the case's graders,
// and a trigger run by the load detector of its harness (criterion 11).
func TestGradeRun(t *testing.T) {
	skill := "plugins/writing/cech-example/skills/cech-example/SKILL.md"
	for _, h := range harnesses {
		t.Run(h, func(t *testing.T) {
			v, err := GradeRun(runFS(t, h, contentCase, "Fix paging\n\nChange-Kind: fix\n"), skill, "cech-example", "evals/writing/cech-example/c", "run", h)
			if err != nil || !v.Pass {
				t.Errorf("with COMMIT_MSG: %+v, %v; want a pass", v, err)
			}
			v, err = GradeRun(runFS(t, h, contentCase, ""), skill, "cech-example", "evals/writing/cech-example/c", "run", h)
			if err != nil || v.Pass || len(v.Failed) != 1 || v.Failed[0] != "file-written" {
				t.Errorf("without COMMIT_MSG: %+v, %v; want file-written failed", v, err)
			}
			v, err = GradeRun(runFS(t, h, triggerCase, ""), skill, "cech-example", "evals/writing/cech-example/c", "run", h)
			if err != nil || !v.Loaded {
				t.Errorf("trigger: %+v, %v; want the skill loaded", v, err)
			}
		})
	}
}

// TestGradeRunResultFiles checks that a run folder of cech eval's
// results, which holds its own files beside the recording, is graded,
// and that any other file still makes it a recording with a problem.
func TestGradeRunResultFiles(t *testing.T) {
	skill := "plugins/writing/cech-example/skills/cech-example/SKILL.md"
	fsys := runFS(t, harnessClaude, contentCase, "Fix paging\n\nChange-Kind: fix\n")
	for _, name := range ResultFiles {
		fsys["run/"+name] = &fstest.MapFile{Data: []byte("{}\n")}
	}
	if v, err := GradeRun(fsys, skill, "cech-example", "evals/writing/cech-example/c", "run", harnessClaude); err != nil || !v.Pass {
		t.Errorf("with the result files: %+v, %v; want a pass", v, err)
	}
	fsys["run/notes.txt"] = &fstest.MapFile{Data: []byte("x\n")}
	if _, err := GradeRun(fsys, skill, "cech-example", "evals/writing/cech-example/c", "run", harnessClaude); err == nil || !strings.Contains(err.Error(), "notes.txt") {
		t.Errorf("with another file: %v; want an error naming it", err)
	}
}

// TestGradeRunBadCase checks that a case the reference rejects cannot
// grade a run.
func TestGradeRunBadCase(t *testing.T) {
	bad := strings.Replace(contentCase, `type = "regex"`, `type = "regexx"`, 1)
	_, err := GradeRun(runFS(t, harnessClaude, bad, ""), "plugins/writing/cech-example/skills/cech-example/SKILL.md", "cech-example", "evals/writing/cech-example/c", "run", harnessClaude)
	if err == nil || !strings.Contains(err.Error(), "regexx") {
		t.Errorf("GradeRun of a bad case: %v; want an error naming the grader", err)
	}
}
