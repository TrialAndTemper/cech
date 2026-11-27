// Package evals checks a marketplace's eval folders without running an
// agent: the layout and files of ADR 25, and the recorded runs of each
// harness that test the cases' graders and the load detectors. Grading
// a recorded run works around the harness gap
// claude-code-eval-offline-grading.
// docs/reference/evals.md describes what it checks, and the rules as
// specified are in changes/done/2026-10-29-evals/spec.md.
package evals

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"syscall"

	"github.com/BurntSushi/toml"

	"github.com/TrialAndTemper/cech/cli/internal/marketplace"
)

// Dir is the folder of eval cases at the marketplace root.
const Dir = "evals"

// resultsDir is where cech eval writes its run results; it is
// git-ignored and not checked.
const resultsDir = "evals/results"

// ReportsDir is where cech eval writes its report files, one per commit
// it ran on, which a pull request commits (eval-runs spec criteria 13 and 27).
const ReportsDir = "evals/reports"

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
	// Found are the cases whose cech.toml it read, in path order.
	Found []Case
}

// Case is a case Check read: its plugin, its folder and what its
// cech.toml says.
type Case struct {
	Plugin, Name, Dir, Family, Expect string
	// Graders are the names of its graders, in cech.toml's order.
	Graders []string
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
	Claim      *string                   `toml:"claim"`
	Recordings map[string]recordingLists `toml:"recordings"`
	Graders    []map[string]any          `toml:"graders"`
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
		// cech eval writes its run results to evals/results/ and its
		// reports to evals/reports/.
		switch path.Join(Dir, p.Category) {
		case resultsDir:
			c.bad(p.Dir(), "is in the category results, whose eval folder would be evals/results/, where cech eval writes its run results")
		case ReportsDir:
			c.bad(p.Dir(), "is in the category reports, whose eval folder would be evals/reports/, where cech eval writes its report files")
		}
	}
	c.walk(plugins)
	for _, p := range m.Plugins {
		if !c.walkFailed && !c.unjudged[p.Name] {
			c.minimums(p)
		}
	}
	sort.Slice(c.res.Found, func(i, j int) bool { return c.res.Found[i].Dir < c.res.Found[j].Dir })
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
		switch catDir {
		case resultsDir:
			continue
		case ReportsDir:
			c.checkReports(catDir, cat)
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
	nameLine, descLine := skillLines(string(src))
	c.skills[p.Name] = skillID{plugin: p.Name, name: p.Name, nameLine: nameLine, descLine: descLine}
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
		case d.IsDir():
			return nil
		case base == "CLAUDE.md":
			c.bad(f, "would make Claude Code skip AGENTS.md")
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
		c.bad(dir, "has the same case name as %s; run results name cases by name, so case names are unique across the marketplace", other)
	} else {
		c.names[name] = dir
	}
	if err := c.checkCaseFolder(p, dir); err != nil {
		return err
	}
	if err := c.checkPrompt(dir); err != nil {
		return err
	}
	meta, err := c.readMeta(dir)
	if err != nil {
		return err
	}
	if meta == nil {
		return nil
	}
	graders, gradable := c.checkGraders(dir, meta)
	found := Case{Plugin: p.Name, Name: name, Dir: dir, Family: meta.Family, Expect: meta.Expect}
	for _, g := range graders {
		found.Graders = append(found.Graders, g.name)
	}
	c.res.Found = append(c.res.Found, found)
	if c.families[p.Name] == nil {
		c.families[p.Name] = map[string]int{}
	}
	c.families[p.Name][meta.Family+"/"+meta.Expect]++
	if meta.Wording != nil {
		c.families[p.Name][meta.Expect+"/"+*meta.Wording]++
	}
	return c.checkRecordings(p, dir, meta, graders, gradable)
}

// checkCaseFolder checks that a case folder holds only prompt.md,
// cech.toml, recordings/ and project.staged/, which it checks as a
// fixture project of plugin pl (spec rules 7 and 17).
func (c *checker) checkCaseFolder(pl marketplace.Plugin, dir string) error {
	entries, err := c.readDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		p := path.Join(dir, e.Name())
		switch e.Name() {
		case "prompt.md", "cech.toml", "recordings":
		case stagedDir:
			if _, err := c.checkProject(pl, p, false); err != nil {
				return err
			}
		case "graders", "case.yaml":
			c.bad(p, "is not part of a case; graders go in cech.toml as [[graders]] tables")
		default:
			c.bad(p, "is not prompt.md, cech.toml, recordings/ or project.staged/")
		}
	}
	return nil
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

// checkPrompt checks prompt.md, whose whole text is the prompt (spec
// rule 9).
func (c *checker) checkPrompt(dir string) error {
	file := path.Join(dir, "prompt.md")
	src, exists, err := c.read(file)
	switch {
	case err != nil:
		return err
	case !exists:
		c.bad(dir, "has no prompt.md")
	case src == nil:
	case strings.TrimSpace(string(src)) == "":
		c.bad(file, "is empty")
	case firstLine(string(src)) == "---":
		c.bad(file, "starts with frontmatter; the whole file is the prompt")
	}
	return nil
}

// firstLine returns the first line of src, without surrounding space.
func firstLine(src string) string {
	line, _, _ := strings.Cut(src, "\n")
	return strings.TrimSpace(line)
}

// checkGraders checks the [[graders]] tables of a case's cech.toml (spec
// rules 10 to 16), and those its family needs (spec rule 13). It returns
// the graders, and whether every table parsed.
func (c *checker) checkGraders(dir string, meta *caseMeta) ([]*grader, bool) {
	file := path.Join(dir, "cech.toml")
	if meta.Family == "trigger" {
		if len(meta.Graders) > 0 {
			c.bad(file, "has [[graders]], but a trigger case has none; the load detector decides its verdict")
		}
		return nil, true
	}
	if len(meta.Graders) == 0 {
		c.bad(file, "has no [[graders]]; a %s case needs at least one", meta.Family)
	}
	var graders []*grader
	all := true
	seen := map[string]bool{}
	for i, fields := range meta.Graders {
		g, name, probs := parseGrader(i+1, fields)
		if seen[name] {
			c.bad(file, "grader %s: the name is used twice", name)
		}
		seen[name] = true
		for _, msg := range probs {
			c.bad(file, "grader %s: %s", name, msg)
		}
		if g == nil {
			all = false
			continue
		}
		graders = append(graders, g)
	}
	return graders, all
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
		// checkGraders reports every key of a grader.
		if k[0] == "graders" {
			continue
		}
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
	} else if meta.Claim != nil {
		c.bad(file, "claim is for content cases only")
	}
	if !ok {
		return nil, nil
	}
	return &meta, nil
}

// checkRecordings reads and grades a case's recordings of each harness
// (spec rules 20 to 25).
func (c *checker) checkRecordings(p marketplace.Plugin, dir string, meta *caseMeta, graders []*grader, gradable bool) error {
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
		if err := c.checkHarnessRecordings(p, dir, h, meta, graders, gradable); err != nil {
			return err
		}
	}
	return nil
}

// checkHarnessRecordings reads and grades a case's recordings of harness.
// A grader that did not parse would leave the rest to give a verdict
// the case does not declare, so without gradable the recordings are read
// but not graded.
func (c *checker) checkHarnessRecordings(p marketplace.Plugin, dir, harness string, meta *caseMeta, graders []*grader, gradable bool) error {
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
		if !gradable {
			continue
		}
		var failed, passed []string
		for _, g := range graders {
			if g.grade(r) {
				passed = append(passed, g.name)
			} else {
				failed = append(failed, g.name)
			}
		}
		switch {
		case pass && len(failed) > 0:
			c.bad(rd, "must pass every grader, but fails %s", strings.Join(failed, ", "))
		case !pass && len(failed) == 0:
			why := "the case has none"
			if len(passed) > 0 {
				why = "passes " + strings.Join(passed, ", ")
			}
			c.bad(rd, "must fail a grader, but %s", why)
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

// checkReports checks the report files in evals/reports/: each is a
// file <commit>.md, named by a full commit id, that holds no local path
// (eval-runs spec criterion 27).
func (c *checker) checkReports(dir string, d fs.DirEntry) {
	if !d.IsDir() {
		c.bad(dir, "is not a folder of report files")
		return
	}
	entries, err := c.readDir(dir)
	if err != nil {
		c.errs = append(c.errs, fmt.Errorf("%w; skipped the report files", err))
		return
	}
	for _, e := range entries {
		p := path.Join(dir, e.Name())
		name, isMD := strings.CutSuffix(e.Name(), ".md")
		if e.IsDir() || !isMD || !commitID.MatchString(name) {
			c.bad(p, "is not a report file <commit>.md, named by the full id of the commit it ran on")
			continue
		}
		src, exists, err := c.read(p)
		if err != nil || !exists {
			continue
		}
		for i, line := range strings.Split(string(src), "\n") {
			if localPath(line) {
				c.bad(p, "line %d holds a local path", i+1)
			}
		}
	}
}

// commitID is a full git commit id, in SHA-1 or SHA-256.
var commitID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
