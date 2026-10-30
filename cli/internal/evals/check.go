// Package evals checks a marketplace's eval folders without a model: the
// layout and files of ADR 25, and the recorded runs of each harness that
// test the cases' graders and the load detectors. Grading a recorded run
// works around the harness gap claude-code-eval-offline-grading.
// docs/reference/evals.md describes what it checks, and the rules as
// specified are in changes/2026-10-29-evals/spec.md.
package evals

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"sort"
	"strings"
	"syscall"

	"github.com/BurntSushi/toml"
	"go.yaml.in/yaml/v3"

	"github.com/TrialAndTemper/cech/cli/internal/frontmatter"
	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

// Dir is the folder of eval cases at the marketplace root.
const Dir = "evals"

// resultsDir is where claude plugin eval writes its results; it is
// git-ignored and not checked.
const resultsDir = "evals/results"

// Finding is one way the eval cases break a rule, at a path from the
// marketplace root.
type Finding struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// Result is what Check found.
type Result struct {
	// Cases and Recordings are the numbers of each it checked.
	Cases, Recordings int
	// Findings are sorted by path, then message.
	Findings []Finding
}

// The folders and file of a plugin's eval folder that are not cases
// (spec rule 1).
const (
	projectDir = "project"
	stagedDir  = "project.staged"
	routingMD  = "routing.md"
)

// The families of case and what each may expect (spec rule 18).
var expects = map[string][]string{
	"content": {"better"},
	"control": {"same"},
	"trigger": {"load", "skip"},
}

// The wordings of a trigger case's prompt, by what it expects.
var wordings = map[string][]string{
	"load": {"matched", "oblique"},
	"skip": {"unrelated", "near-miss"},
}

// recordingLists are the recordings of one harness that must pass and
// fail.
type recordingLists struct {
	Pass []string `toml:"pass"`
	Fail []string `toml:"fail"`
}

// caseMeta is a case's cech.toml.
type caseMeta struct {
	Family     string                    `toml:"family"`
	Expect     string                    `toml:"expect"`
	Wording    *string                   `toml:"wording"`
	Critical   *bool                     `toml:"critical"`
	Claim      *string                   `toml:"claim"`
	Recordings map[string]recordingLists `toml:"recordings"`
}

// checker collects the findings of one Check.
type checker struct {
	fsys  fs.FS
	res   Result
	names map[string]string // case name to its folder
	// errs are the reads that failed; each skips the rest of its case.
	errs []error
	// unjudged are the plugins whose minimums it cannot judge, since one
	// of their cases could not be read.
	unjudged map[string]bool
	// walkFailed is whether a folder above the cases could not be read,
	// so no plugin's minimums can be judged.
	walkFailed bool
	// families counts each plugin's cases by family and expect, and by
	// wording. It and unjudged key plugins by name alone, which ADR 12
	// keeps unique.
	families map[string]map[string]int
	// skills are the skills the load detectors look for, by plugin name.
	skills map[string]skillID
}

func (c *checker) bad(p, format string, a ...any) {
	c.res.Findings = append(c.res.Findings, Finding{p, fmt.Sprintf(format, a...)})
}

// Check checks the eval cases of the marketplace m, rooted at fsys. Its
// error joins the reads that were denied or failed, each of which skips
// the rest of its case; the rules a case breaks are findings, kept even
// when there is an error.
func Check(fsys fs.FS, m *marketplace.Marketplace) (Result, error) {
	c := &checker{fsys: fsys, names: map[string]string{}, families: map[string]map[string]int{}, unjudged: map[string]bool{}, skills: map[string]skillID{}}
	plugins := map[string]marketplace.Plugin{}
	for _, p := range m.Plugins {
		plugins[path.Join(p.Category, p.Name)] = p
		// claude plugin eval writes its results to evals/results/.
		if path.Join(Dir, p.Category) == resultsDir {
			c.bad(p.Dir(), "is in the category results, whose eval folder would be evals/results/, where claude plugin eval writes its results")
		}
	}
	c.walk(plugins)
	for _, p := range m.Plugins {
		if !c.walkFailed && !c.unjudged[p.Name] {
			c.minimums(p)
		}
	}
	sort.Slice(c.res.Findings, func(i, j int) bool {
		a, b := c.res.Findings[i], c.res.Findings[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Message < b.Message
	})
	return c.res, errors.Join(c.errs...)
}

// walk checks evals/: categories, plugin folders and case folders. A
// folder it cannot read is skipped, and the walk goes on.
func (c *checker) walk(plugins map[string]marketplace.Plugin) {
	skip := func(err error) {
		c.errs = append(c.errs, fmt.Errorf("%w; skipped what it holds, and the check of every plugin's minimum cases", err))
		c.walkFailed = true
	}
	categories, err := c.readDir(Dir)
	if err != nil {
		skip(err)
		return
	}
	for _, cat := range categories {
		catDir := path.Join(Dir, cat.Name())
		if catDir == resultsDir {
			continue
		}
		if !cat.IsDir() {
			c.bad(catDir, "is not a category folder; evals/ holds only evals/<category>/<plugin>/<case>/")
			continue
		}
		pluginDirs, err := c.readDir(catDir)
		if err != nil {
			skip(err)
			continue
		}
		for _, pd := range pluginDirs {
			dir := path.Join(catDir, pd.Name())
			p, ok := plugins[path.Join(cat.Name(), pd.Name())]
			switch {
			case !pd.IsDir():
				c.bad(dir, "is not a plugin folder; evals/ holds only evals/<category>/<plugin>/<case>/")
				continue
			case !ok:
				c.bad(dir, "has no plugin plugins/%s/%s", cat.Name(), pd.Name())
				continue
			}
			entries, err := c.readDir(dir)
			if err != nil {
				skip(err)
				continue
			}
			if err := c.checkPluginEvals(p, dir, entries); err != nil {
				c.errs = append(c.errs, fmt.Errorf("%w; skipped the rest of the fixture project and routing rows in %s", err, dir))
			}
			for _, cd := range entries {
				caseDir := path.Join(dir, cd.Name())
				switch {
				case cd.Name() == projectDir || cd.Name() == stagedDir || cd.Name() == routingMD:
					continue
				case cd.Name() == "mocks":
					c.bad(caseDir, "mocks are not allowed in CECH's eval folders")
					continue
				case !cd.IsDir():
					c.bad(caseDir, "is not project/, project.staged/, routing.md or a case folder")
					continue
				}
				if err := c.checkCase(p, caseDir); err != nil {
					c.errs = append(c.errs, fmt.Errorf("%w; skipped the rest of case %s, and the check of %s's minimum cases", err, caseDir, p.Name))
					c.unjudged[p.Name] = true
				}
			}
		}
	}
}

// checkPluginEvals checks what a plugin's eval folder dir holds besides
// its cases: the fixture project and the routing rows (spec rules 3 to
// 6). It also reads the skill the load detectors look for.
func (c *checker) checkPluginEvals(p marketplace.Plugin, dir string, entries []fs.DirEntry) error {
	src, _, err := c.read(p.SkillPath())
	if err != nil {
		return err
	}
	c.skills[p.Name] = skillID{plugin: p.Name, name: p.Name, descLine: descriptionLine(string(src))}
	has := map[string]bool{}
	for _, e := range entries {
		has[e.Name()] = true
	}
	cells := 0
	if !has[projectDir] {
		c.bad(dir, "has no project/, the project every run starts in")
	} else if cells, err = c.checkProject(p, path.Join(dir, projectDir), true); err != nil {
		return err
	}
	if has[stagedDir] {
		if _, err := c.checkProject(p, path.Join(dir, stagedDir), false); err != nil {
			return err
		}
	}
	return c.checkRouting(p, path.Join(dir, routingMD), cells)
}

// checkProject checks a fixture project, project/ or a project.staged/
// (spec rules 2, 4 and 5). For project/, top, it also checks the routing
// table of its AGENTS.md, and returns how many cells its header has, or
// 0 when it has none.
func (c *checker) checkProject(p marketplace.Plugin, root string, top bool) (int, error) {
	if info, err := fs.Stat(c.fsys, root); err == nil && !info.IsDir() {
		c.bad(root, "is not a folder")
		return 0, nil
	}
	skill := "`" + p.Name + "`"
	err := fs.WalkDir(c.fsys, root, func(f string, d fs.DirEntry, err error) error {
		if err != nil {
			return marketplace.ReadError(err)
		}
		if f == root {
			return nil
		}
		base := path.Base(f)
		switch {
		case d.IsDir() && (base == ".claude" || base == ".codex" || base == ".agents"):
			c.bad(f, "would give the run settings or skills of its own")
			return fs.SkipDir
		case d.IsDir() && base == "mocks":
			c.bad(f, "mocks are not allowed in CECH's eval folders")
			return fs.SkipDir
		case d.IsDir():
			return nil
		case base == "CLAUDE.md":
			c.bad(f, "would make Claude Code skip AGENTS.md")
		case base == "prompt.md" || base == "case.yaml":
			c.bad(f, "claude plugin eval would load this as a case of its own")
		}
		if !d.Type().IsRegular() {
			c.bad(f, "is not a file")
			return nil
		}
		src, err := fs.ReadFile(c.fsys, f)
		if err != nil {
			return marketplace.ReadError(err)
		}
		if base == "AGENTS.md" && strings.Contains(string(src), skill) {
			c.bad(f, "names the skill %s, so an unrouted run would see a route to it", skill)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if localPath(line) {
				c.bad(f, "line %d holds a local path", i+1)
			}
		}
		return nil
	})
	if err != nil || !top {
		return 0, err
	}
	agents := path.Join(root, "AGENTS.md")
	src, exists, err := c.read(agents)
	switch {
	case err != nil:
		return 0, err
	case !exists:
		c.bad(root, "has no AGENTS.md")
		return 0, nil
	case src == nil:
		return 0, nil
	}
	cells := routingHeader(string(src))
	if cells == 0 {
		c.bad(agents, "has no section headed ## Routing with a Markdown table whose first row is a header")
	}
	return cells, nil
}

// routingHeader returns how many cells the header of the table under
// ## Routing in the AGENTS.md src has, or 0 when it has no such table.
func routingHeader(src string) int {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "## Routing" {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			t := strings.TrimSpace(lines[j])
			switch {
			case strings.HasPrefix(t, "#"):
				return 0
			case !strings.HasPrefix(t, "|"):
				continue
			}
			n := tableCells(t)
			if n == 0 || j+1 >= len(lines) || !separatorRow(strings.TrimSpace(lines[j+1])) || tableCells(strings.TrimSpace(lines[j+1])) != n {
				return 0
			}
			return n
		}
		return 0
	}
	return 0
}

// tableCells returns how many cells the table row line has, split at
// each | not escaped as \|, or 0 when it is not a row.
func tableCells(line string) int {
	if len(line) < 2 || line[0] != '|' || line[len(line)-1] != '|' || strings.HasSuffix(line, `\|`) {
		return 0
	}
	n := 0
	for i := 1; i < len(line); i++ {
		if line[i] == '|' && line[i-1] != '\\' {
			n++
		}
	}
	return n
}

// separatorRow reports whether line is a table's header separator, such
// as | --- | :-: |.
func separatorRow(line string) bool {
	return tableCells(line) > 0 && strings.Contains(line, "-") && strings.Trim(line, "|-: ") == ""
}

// checkRouting checks routing.md, the rows a routed run adds to the
// table under ## Routing in project/AGENTS.md, whose header has cells
// cells, or 0 when it is unknown (spec rule 6).
func (c *checker) checkRouting(p marketplace.Plugin, file string, cells int) error {
	src, exists, err := c.read(file)
	switch {
	case err != nil:
		return err
	case !exists:
		c.bad(path.Dir(file), "has no routing.md, the routing rows of a routed run")
		return nil
	case src == nil:
		return nil
	}
	skill := "`" + p.Name + "`"
	named := false
	text := strings.TrimSuffix(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		n := tableCells(line)
		switch {
		case line == "":
			c.bad(file, "line %d is blank; routing.md holds table rows only", i+1)
		case n == 0:
			c.bad(file, "line %d is not a Markdown table row", i+1)
		case separatorRow(line):
			c.bad(file, "line %d is a header separator; routing.md holds rows only, no header", i+1)
		case cells > 0 && n != cells:
			c.bad(file, "line %d has %d cells, but the table under ## Routing in project/AGENTS.md has %d", i+1, n, cells)
		}
		named = named || (n > 0 && strings.Contains(line, skill))
	}
	if !named {
		c.bad(file, "has no row naming the skill as %s", skill)
	}
	return nil
}

// notExist reports whether err says nothing is at a path, also when a
// file stands where a folder on the way to it belongs, which a finding
// about that file already reports.
func notExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// readDir lists dir, or returns nil when it does not exist. Something at
// dir that is not a folder is a finding.
func (c *checker) readDir(dir string) ([]fs.DirEntry, error) {
	info, err := fs.Stat(c.fsys, dir)
	if notExist(err) {
		return nil, nil
	}
	if err == nil && !info.IsDir() {
		c.bad(dir, "is not a folder")
		return nil, nil
	}
	entries, err := fs.ReadDir(c.fsys, dir)
	if err != nil {
		return nil, marketplace.ReadError(err)
	}
	return entries, nil
}

// checkCase checks one case folder of plugin p.
func (c *checker) checkCase(p marketplace.Plugin, dir string) error {
	c.res.Cases++
	name := path.Base(dir)
	if other, ok := c.names[name]; ok {
		c.bad(dir, "has the same case name as %s; claude plugin eval reports cases by name", other)
	} else {
		c.names[name] = dir
	}
	if err := c.checkTree(p, dir); err != nil {
		return err
	}
	if err := c.checkPrompt(p, dir); err != nil {
		return err
	}
	history, err := c.checkCaseYAML(dir)
	if err != nil {
		return err
	}
	graders, err := c.readGraders(dir)
	if err != nil {
		return err
	}
	meta, err := c.readMeta(dir)
	if err != nil {
		return err
	}
	if meta == nil {
		return nil
	}
	if c.families[p.Name] == nil {
		c.families[p.Name] = map[string]int{}
	}
	c.families[p.Name][meta.Family+"/"+meta.Expect]++
	if meta.Wording != nil {
		c.families[p.Name][meta.Expect+"/"+*meta.Wording]++
	}
	if history && meta.Family != "trigger" {
		c.bad(path.Join(dir, "case.yaml"), "a %s case resumes a transcript with context.history_file, and claude plugin eval runs such a case in one arm only", meta.Family)
	}
	c.checkFamily(p, dir, meta, graders)
	return c.checkRecordings(p, dir, meta, graders)
}

// checkTree finds what claude plugin eval would misread below a case:
// a nested prompt.md or case.yaml loads as a case of its own, and mocks
// are not allowed. It checks the case's own project.staged/ as a fixture
// project of plugin pl, and holds every other .jsonl to the local-path
// rule, but for a recording's trace.jsonl, which readRecording reads.
func (c *checker) checkTree(pl marketplace.Plugin, dir string) error {
	return fs.WalkDir(c.fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return marketplace.ReadError(err)
		}
		if p == dir {
			return nil
		}
		base := path.Base(p)
		switch {
		case path.Dir(p) == dir && base == stagedDir:
			if _, err := c.checkProject(pl, p, false); err != nil {
				return err
			}
			if d.IsDir() {
				return fs.SkipDir
			}
		case d.IsDir() && base == "mocks":
			c.bad(p, "mocks are not allowed in CECH's eval folders")
			return fs.SkipDir
		case !d.IsDir() && path.Dir(p) != dir && (base == "prompt.md" || base == "case.yaml"):
			c.bad(p, "claude plugin eval would load this as a case of its own")
		case d.Type().IsRegular() && strings.HasSuffix(base, ".jsonl") && !isRecordingTrace(dir, p):
			src, err := fs.ReadFile(c.fsys, p)
			if err != nil {
				return marketplace.ReadError(err)
			}
			for i, line := range strings.Split(string(src), "\n") {
				for _, msg := range localStrings([]byte(line)) {
					c.bad(p, "line %d: %s", i+1, msg)
				}
			}
		}
		return nil
	})
}

// isRecordingTrace reports whether p is the trace.jsonl of a recording
// of the case in dir, which readRecording checks.
func isRecordingTrace(dir, p string) bool {
	rd := path.Dir(p)
	return path.Base(p) == "trace.jsonl" && slices.Contains(harnesses, path.Base(path.Dir(rd))) && path.Dir(path.Dir(rd)) == path.Join(dir, "recordings")
}

// read returns the file p, and whether anything is at p. Something at p
// that is not a file is a finding, and gives no content.
func (c *checker) read(p string) (src []byte, exists bool, err error) {
	return readFile(c.fsys, p, func(msg string) { c.bad(p, "%s", msg) })
}

// readFile returns the file p, and whether anything is at p. Something
// at p that is not a file is reported through bad, and gives no
// content; a denied read is marketplace.ErrUnreadable.
func readFile(fsys fs.FS, p string, bad func(msg string)) (src []byte, exists bool, err error) {
	info, err := fs.Stat(fsys, p)
	if notExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, marketplace.ReadError(err)
	}
	if !info.Mode().IsRegular() {
		bad("is not a file")
		return nil, true, nil
	}
	src, err = fs.ReadFile(fsys, p)
	if err != nil {
		return nil, true, marketplace.ReadError(err)
	}
	return src, true, nil
}

// checkPrompt checks prompt.md (spec rules 7 to 9).
func (c *checker) checkPrompt(p marketplace.Plugin, dir string) error {
	file := path.Join(dir, "prompt.md")
	src, exists, err := c.read(file)
	switch {
	case err != nil:
		return err
	case !exists:
		c.bad(dir, "has no prompt.md")
		return nil
	case src == nil:
		return nil
	}
	doc, err := frontmatter.Parse(src)
	if err != nil {
		c.bad(file, "%v", err)
		return nil
	}
	if _, ok := doc.Fields["name"]; ok {
		c.bad(file, "has a name; a case's name is its folder's name")
	}
	want := "../../../../" + p.Dir()
	n := doc.Fields["plugins"]
	if n == nil || n.Kind != yaml.SequenceNode || len(n.Content) != 1 || n.Content[0].ShortTag() != "!!str" || n.Content[0].Value != want {
		c.bad(file, "plugins is not [%q], the case's own plugin", want)
	}
	if strings.TrimSpace(doc.Body) == "" {
		c.bad(file, "has no prompt after the frontmatter")
	}
	return nil
}

// caseYAMLKeys are the keys a case.yaml may have (spec rule 9).
var caseYAMLKeys = []string{"schema_version", "name", "description", "tags", "context"}

// checkCaseYAML checks an optional case.yaml (spec rules 7 and 9), and
// reports whether it resumes a transcript.
func (c *checker) checkCaseYAML(dir string) (history bool, err error) {
	file := path.Join(dir, "case.yaml")
	src, _, err := c.read(file)
	if err != nil || src == nil {
		return false, err
	}
	var doc yaml.Node
	if yerr := yaml.Unmarshal(src, &doc); yerr != nil || len(doc.Content) == 0 {
		c.bad(file, "is not a YAML mapping")
		return false, nil
	}
	fields, keys, err := frontmatter.Mapping(doc.Content[0])
	if err != nil {
		c.bad(file, "%v", err)
		return false, nil
	}
	for _, k := range keys {
		if !slices.Contains(caseYAMLKeys, k) {
			c.bad(file, "has the key %q; keep graders, the prompt and plugins in graders/ and prompt.md", k)
		}
	}
	if n := fields["name"]; n == nil || n.Value != path.Base(dir) {
		c.bad(file, "name is not %q, the case folder's name", path.Base(dir))
	}
	if ctx := fields["context"]; ctx != nil {
		if cf, _, err := frontmatter.Mapping(ctx); err == nil {
			_, history = cf["history_file"]
		}
	}
	return history, nil
}

// readGraders reads graders/*.md, in name order.
func (c *checker) readGraders(dir string) ([]*grader, error) {
	gdir := path.Join(dir, "graders")
	entries, err := c.readDir(gdir)
	if err != nil {
		return nil, err
	}
	var graders []*grader
	files := 0
	for _, e := range entries {
		file := path.Join(gdir, e.Name())
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			c.bad(file, "is not a grader file, graders/<name>.md")
			continue
		}
		src, _, err := c.read(file)
		if err != nil {
			return nil, err
		}
		if src == nil {
			continue
		}
		files++
		g, probs := parseGrader(strings.TrimSuffix(e.Name(), ".md"), src)
		for _, msg := range probs {
			c.bad(file, "%s", msg)
		}
		if g != nil {
			graders = append(graders, g)
		}
	}
	if files == 0 {
		c.bad(dir, "has no graders/<name>.md")
	}
	return graders, nil
}

// readMeta reads cech.toml (spec rule 18), or returns nil when it is
// missing or unreadable as TOML, or its family, or a trigger case's
// expect, is not valid, so the case's recordings cannot be judged.
func (c *checker) readMeta(dir string) (*caseMeta, error) {
	file := path.Join(dir, "cech.toml")
	src, exists, err := c.read(file)
	switch {
	case err != nil:
		return nil, err
	case !exists:
		c.bad(dir, "has no cech.toml")
		return nil, nil
	case src == nil:
		return nil, nil
	}
	var meta caseMeta
	md, err := toml.Decode(string(src), &meta)
	if err != nil {
		c.bad(file, "%v", err)
		return nil, nil
	}
	for _, k := range md.Undecoded() {
		c.bad(file, "has the unknown key %q", k.String())
	}
	// A trigger case's verdict turns on its expect, so without a valid one
	// its recordings cannot be judged; any other problem leaves the rest
	// of the case to check.
	ok := true
	allowed, known := expects[meta.Family]
	switch {
	case meta.Family == "":
		c.bad(file, "family is missing")
		return nil, nil
	case !known:
		c.bad(file, "family %q is not content, control or trigger", meta.Family)
		return nil, nil
	case !slices.Contains(allowed, meta.Expect):
		c.bad(file, "expect %q is not one of %s for a %s case", meta.Expect, strings.Join(allowed, ", "), meta.Family)
		ok = meta.Family != "trigger"
	}
	if meta.Family == "trigger" {
		allowed := wordings[meta.Expect]
		switch {
		case meta.Wording == nil:
			c.bad(file, "wording is missing; a trigger case says how its prompt is worded")
		case allowed != nil && !slices.Contains(allowed, *meta.Wording):
			c.bad(file, "wording %q is not one of %s for expect = %q", *meta.Wording, strings.Join(allowed, ", "), meta.Expect)
		}
	} else if meta.Wording != nil {
		c.bad(file, "wording is for trigger cases only")
	}
	for _, h := range harnesses {
		if _, has := meta.Recordings[h]; !has {
			c.bad(file, "has no recordings.%s; each case keeps recordings of both harnesses", h)
		}
	}
	for h := range meta.Recordings {
		if !slices.Contains(harnesses, h) {
			c.bad(file, "recordings.%s names no harness; use %s", h, strings.Join(harnesses, " or "))
		}
	}
	if meta.Family == "content" {
		if meta.Claim == nil || strings.TrimSpace(*meta.Claim) == "" || strings.Contains(*meta.Claim, "\n") {
			c.bad(file, "claim is not one line naming the skill rule the case tests")
		}
	} else {
		if meta.Critical != nil {
			c.bad(file, "critical is for content cases only")
		}
		if meta.Claim != nil {
			c.bad(file, "claim is for content cases only")
		}
	}
	if !ok {
		return nil, nil
	}
	return &meta, nil
}

// checkFamily checks the graders a case's family needs (spec rule 13).
func (c *checker) checkFamily(p marketplace.Plugin, dir string, meta *caseMeta, graders []*grader) {
	if meta.Family != "trigger" {
		if !slices.ContainsFunc(graders, func(g *grader) bool { return g.typ == typeRegex || g.typ == typeFileExists }) {
			c.bad(dir, "has no regex or file_exists grader of the reply or files, which both harnesses can grade")
		}
		return
	}
	if len(graders) != 1 || graders[0].typ != typeToolUsed || graders[0].use.tool != "Skill" {
		c.bad(dir, "a trigger case needs exactly one grader, tool_used with tool: Skill")
		return
	}
	g := graders[0]
	skill := p.Name
	switch {
	case g.use.input == nil:
		c.bad(dir, "the trigger grader has no input_match naming the skill %q", skill)
	case !g.use.input.MatchString(`{"skill":"` + p.Name + `:` + skill + `","args":"x"}`),
		!g.use.input.MatchString(`{"skill":"` + skill + `"}`),
		g.use.input.MatchString(`{"skill":"` + skill + `-x"}`),
		g.use.input.MatchString(`{"skill":"x-` + skill + `"}`):
		c.bad(dir, "the trigger grader's input_match must match the skill %q, alone or as %s:%s, and no other name", skill, p.Name, skill)
	}
	if meta.Expect == "load" && g.min < 1 {
		c.bad(dir, `a trigger case with expect = "load" has a grader with min below 1`)
	}
	if meta.Expect == "skip" && (g.min != 0 || g.max != 0) {
		c.bad(dir, `a trigger case with expect = "skip" has a grader with min or max other than 0`)
	}
}

// checkRecordings reads and grades a case's recordings of each harness
// (spec rules 20 to 25).
func (c *checker) checkRecordings(p marketplace.Plugin, dir string, meta *caseMeta, graders []*grader) error {
	rdir := path.Join(dir, "recordings")
	entries, err := c.readDir(rdir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() || !slices.Contains(harnesses, e.Name()) {
			c.bad(path.Join(rdir, e.Name()), "is not a harness's folder, recordings/%s/", strings.Join(harnesses, "/ or recordings/"))
		}
	}
	for _, h := range harnesses {
		if err := c.checkHarnessRecordings(p, dir, h, meta, graders); err != nil {
			return err
		}
	}
	return nil
}

// checkHarnessRecordings reads and grades a case's recordings of harness.
func (c *checker) checkHarnessRecordings(p marketplace.Plugin, dir, harness string, meta *caseMeta, graders []*grader) error {
	hdir := path.Join(dir, "recordings", harness)
	if info, err := fs.Stat(c.fsys, hdir); err == nil && !info.IsDir() {
		return nil
	}
	entries, err := c.readDir(hdir)
	if err != nil {
		return err
	}
	var folders []string
	for _, e := range entries {
		if !e.IsDir() {
			c.bad(path.Join(hdir, e.Name()), "is not a recording folder")
			continue
		}
		folders = append(folders, e.Name())
	}
	lists, declaredHarness := meta.Recordings[harness]
	file := path.Join(dir, "cech.toml")
	if declaredHarness {
		if len(lists.Pass) == 0 {
			c.bad(file, "recordings.%s.pass names no recording", harness)
		}
		if len(lists.Fail) == 0 {
			c.bad(file, "recordings.%s.fail names no recording", harness)
		}
	}
	declared := map[string]bool{}
	for _, list := range []struct {
		names []string
		pass  bool
	}{{lists.Pass, true}, {lists.Fail, false}} {
		for _, name := range list.names {
			if _, dup := declared[name]; dup {
				c.bad(file, "recording %s/%s is named twice", harness, name)
				continue
			}
			declared[name] = list.pass
			if !slices.Contains(folders, name) {
				c.bad(file, "recording %q has no folder in recordings/%s/", name, harness)
			}
		}
	}
	for _, name := range folders {
		rd := path.Join(hdir, name)
		pass, ok := declared[name]
		if !ok {
			c.bad(rd, "is not named in recordings.%s.pass or recordings.%s.fail", harness, harness)
			continue
		}
		c.res.Recordings++
		r, finds, err := readRecording(c.fsys, rd, harness)
		if err != nil {
			return err
		}
		c.res.Findings = append(c.res.Findings, finds...)
		if len(finds) > 0 {
			continue
		}
		if meta.Family == "trigger" {
			wantLoad := (meta.Expect == "load") == pass
			c.checkLoad(rd, harness, wantLoad, r.loads(c.skills[p.Name]))
			continue
		}
		var failed, passed []string
		for _, g := range graders {
			if !g.deterministic() || !g.appliesTo(harness) {
				continue
			}
			if g.grade(r) {
				passed = append(passed, g.name)
			} else {
				failed = append(failed, g.name)
			}
		}
		switch {
		case pass && len(failed) > 0:
			c.bad(rd, "must pass every deterministic grader that applies on %s, but fails %s", harness, strings.Join(failed, ", "))
		case !pass && len(failed) == 0:
			why := "the case has none on " + harness
			if len(passed) > 0 {
				why = "passes " + strings.Join(passed, ", ")
			}
			c.bad(rd, "must fail a deterministic grader, but %s", why)
		}
	}
	return nil
}

// checkLoad checks a trigger case's recording rd against the load
// detector of harness, which read loaded (spec rule 24).
func (c *checker) checkLoad(rd, harness string, wantLoad, loaded bool) {
	switch {
	case wantLoad && !loaded:
		c.bad(rd, "must show the skill loaded, but the %s load detector reads no load", harness)
	case !wantLoad && loaded:
		c.bad(rd, "must show the skill not loaded, but the %s load detector reads a load", harness)
	}
}

// minimums checks the cases a plugin must have (spec rule 19).
func (c *checker) minimums(p marketplace.Plugin) {
	dir := path.Join(Dir, p.Category, p.Name)
	have := c.families[p.Name]
	for _, need := range []struct {
		key, what string
		n         int
	}{
		{"content/better", "content case", 1},
		{"control/same", "control case", 1},
		{"trigger/load", `trigger cases with expect = "load"`, 3},
		{"trigger/skip", `trigger cases with expect = "skip"`, 3},
		{"load/oblique", `trigger case with expect = "load" and wording = "oblique"`, 1},
		{"skip/near-miss", `trigger case with expect = "skip" and wording = "near-miss"`, 1},
	} {
		if have[need.key] < need.n {
			c.bad(dir, "has %d of the %d %s each plugin needs", have[need.key], need.n, need.what)
		}
	}
}
