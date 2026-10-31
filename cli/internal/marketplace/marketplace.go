// Package marketplace reads the hand-written sources of a marketplace
// with CECH's layout (ADR 18 item 1): data/marketplace.toml for the
// catalogue (ADR 17 item 7), and per plugin its plugin.json, the
// SKILL.md of its one skill (ADR 11) and its reviewer agents.
package marketplace

import (
	_ "embed"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/TrialAndTemper/cech/cli/internal/jsonedit"
)

// DataFile describes the catalogue as a whole.
const DataFile = "data/marketplace.toml"

// LicenseFile is the marketplace's licence, which the build copies into
// each skill folder.
const LicenseFile = "LICENSE"

// Data is the content of DataFile, whose keys and rules ADR 17 item 7
// sets.
type Data struct {
	// Namespace is the catalogue's name outside a release tag, and the
	// prefix of every plugin name (ADR 12 items 4 and 5).
	Namespace   string `toml:"namespace"`
	Description string `toml:"description"`
	// CECH is the CECH release an extension builds on, as X.Y.Z or
	// X.Y.Z-rc.N; it is "" in CECH's own file.
	CECH string `toml:"cech"`
	// Team owns every path without a plugin line in the code owners
	// file, as @org/team (ADR 13 item 4); with none, there is no file.
	Team string `toml:"team"`
	// Owner is the catalogue's owner, which Claude Code requires.
	Owner Owner `toml:"owner"`
	// Categories are those an extension adds to CECH's (ADR 12 item 6).
	Categories []Category `toml:"categories"`
	// Roles are the roles the marketplace defines (ADR 21 item 2).
	Roles []Role `toml:"roles"`
	// Terms are an extension's own term lists.
	Terms Terms `toml:"terms"`
}

// Owner is the owner of the catalogue.
type Owner struct {
	Name  string `toml:"name" json:"name"`
	Email string `toml:"email" json:"email,omitempty"`
}

// Category is one of CECH's categories, or one an extension adds.
type Category struct {
	Name  string `toml:"name"`
	Holds string `toml:"holds"`
}

// Role is a kind of skill that other skills rely on.
type Role struct {
	Name  string `toml:"name"`
	Means string `toml:"means"`
	// Many lets the role take several bound skills.
	Many bool `toml:"many"`
}

// Terms are the paths of an extension's term lists, from the repository
// root.
type Terms struct {
	Rules []string `toml:"rules"`
	Names []string `toml:"names"`
}

// Org returns the organisation of the Team.
func (d Data) Org() string {
	org, _, _ := strings.Cut(strings.TrimPrefix(d.Team, "@"), "/")
	return org
}

// CECHCatalogue returns the catalogue name of the CECH release an
// extension builds on, cech-0-1-0 for 0.1.0 (ADR 14 item 3), or "" in
// CECH's own file.
func (d Data) CECHCatalogue() string {
	if d.CECH == "" {
		return ""
	}
	return CatalogueName("cech", d.CECH)
}

// CatalogueName returns the catalogue name of release v of namespace,
// with each . in v turned into -: acme-1-4-0, cech-0-2-0-rc-1 (ADR 14
// item 3).
func CatalogueName(namespace, v string) string {
	return namespace + "-" + strings.ReplaceAll(v, ".", "-")
}

// IsRelease reports whether v is a release version: X.Y.Z, or X.Y.Z-rc.N
// for a pre-release.
func IsRelease(v string) bool {
	return release.MatchString(v)
}

// IsPreRelease reports whether the release version v is a pre-release.
func IsPreRelease(v string) bool {
	return strings.Contains(v, "-rc.")
}

// Marketplace is the sources of one marketplace.
type Marketplace struct {
	Data Data
	// License is the content of LicenseFile, or nil without one.
	License []byte
	// Plugins are sorted by category, then name.
	Plugins []Plugin
}

// Plugin is one plugin, in plugins/<category>/<name>/.
type Plugin struct {
	Category, Name string
	// Manifest is .claude-plugin/plugin.json as read.
	Manifest []byte
	// AuthorURL is the url of the manifest's author, or "".
	AuthorURL string
	// Description is the description in SKILL.md.
	Description string
	// Maintainers are the GitHub user names in the manifest's
	// metadata.maintainers (ADR 13 item 2), in their order there.
	Maintainers []string
	// Agents are the reviewer agents in agents/, sorted by file name.
	Agents []Agent
	// AgentTOML are the names of the files in agents/ that end in .toml,
	// sorted.
	AgentTOML []string
}

// Dir is the plugin's folder, relative to the marketplace root.
func (p Plugin) Dir() string {
	return path.Join("plugins", p.Category, p.Name)
}

// ManifestPath is the path of the plugin's plugin.json.
func (p Plugin) ManifestPath() string {
	return path.Join(p.Dir(), ".claude-plugin", "plugin.json")
}

// SkillPath is the path of the plugin's SKILL.md.
func (p Plugin) SkillPath() string {
	return path.Join(p.Dir(), "skills", p.Name, "SKILL.md")
}

// LicensePath is the path of the copy of the marketplace's licence in
// the plugin's skill folder.
func (p Plugin) LicensePath() string {
	return path.Join(p.Dir(), "skills", p.Name, LicenseFile)
}

// Agent is a reviewer agent written for Claude Code, in agents/<File>.md.
type Agent struct {
	// File is the file name without .md.
	File        string
	Name        string
	Description string
	// Instructions is the Markdown after the frontmatter.
	Instructions string
}

// githubUser matches a GitHub user name, which has at most 39
// characters.
var githubUser = regexp.MustCompile(`^[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`)

// IsGitHubUser reports whether s has the form of a GitHub user name.
func IsGitHubUser(s string) bool {
	return len(s) <= 39 && githubUser.MatchString(s)
}

// team matches a GitHub team, as @org/team.
var team = regexp.MustCompile(`^@[A-Za-z0-9]+(-[A-Za-z0-9]+)*/[A-Za-z0-9_.-]+$`)

// release matches a release version as X.Y.Z or X.Y.Z-rc.N, in decimal
// with no leading zeros and N from 1 (ADR 14 item 2).
var release = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$`)

// versionEnd matches a name that ends in a version, as a release's
// catalogue name does: -1-4-0 or -1-4-0-rc-2.
var versionEnd = regexp.MustCompile(`(-[0-9]+){3}(-rc-[0-9]+)?$`)

//go:embed categories.toml
var categoriesFile string

// CECHCategories are CECH's categories, the closed list of ADR 12 item 1,
// from categories.toml, which the CLI builds in (ADR 20 item 5).
var CECHCategories = mustCategories(categoriesFile)

// mustCategories reads CECH's categories, holding them to the rules an
// extension's are held to; the file is built in, so a fault is a bug.
func mustCategories(src string) []Category {
	var f struct {
		Categories []Category `toml:"categories"`
	}
	meta, err := toml.Decode(src, &f)
	if err == nil && len(meta.Undecoded()) > 0 {
		err = fmt.Errorf("unknown key %q", meta.Undecoded()[0].String())
	}
	if err == nil {
		err = checkCategories(f.Categories, nil)
	}
	if err != nil {
		panic("categories.toml: " + err.Error())
	}
	return f.Categories
}

// nameForm matches the form of a plugin name that ADR 12 item 3 sets, which
// a category's folder also has: a-z, 0-9 and -, starting with a letter,
// with no -- and no - at the end.
var nameForm = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// nameRule states the form for an error message.
const nameRule = "use a-z, 0-9 and single hyphens, start with a letter, end with a letter or digit, " +
	"at most 64 characters (ADR 12 item 3)"

// isName reports whether s has the form of a plugin name, which has at
// most 64 characters.
func isName(s string) bool {
	return len(s) <= 64 && nameForm.MatchString(s)
}

// ErrUnreadable marks an error that left the sources out of reach: a
// root that could not be opened, or a file whose reading was denied. A
// retry may then succeed, while a tree in the wrong shape, such as a
// folder where a file belongs, is a failure.
var ErrUnreadable = errors.New("cannot read")

// Unreadable marks err as ErrUnreadable, keeping its message.
func Unreadable(err error) error {
	return unreadable{err}
}

// ReadError marks err as ErrUnreadable when reading was denied, and
// returns any other error as it is.
func ReadError(err error) error {
	if errors.Is(err, fs.ErrPermission) {
		return Unreadable(err)
	}
	return err
}

type unreadable struct{ error }

func (e unreadable) Unwrap() error        { return e.error }
func (e unreadable) Is(target error) bool { return target == ErrUnreadable }

// Load reads the marketplace rooted at fsys. Its error lists every
// problem it found, each naming its file; a read it was denied is
// ErrUnreadable.
func Load(fsys fs.FS) (*Marketplace, error) {
	var errs []error
	m := &Marketplace{}
	data, err := loadData(fsys)
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", DataFile, err))
	} else {
		m.Data = data
	}
	if m.License, err = loadLicense(fsys); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", LicenseFile, err))
	}
	categories, err := subdirs(fsys, "plugins")
	if err != nil {
		errs = append(errs, err)
	}
	seen := map[string]string{} // plugin name to its folder
	for _, category := range categories {
		dir := path.Join("plugins", category)
		if !isName(category) {
			errs = append(errs, fmt.Errorf("%s: %q is not a valid category name: %s", dir, category, nameRule))
			continue
		}
		names, err := subdirs(fsys, dir)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, name := range names {
			p := Plugin{Category: category, Name: name}
			if !isName(name) {
				errs = append(errs, fmt.Errorf("%s: %q is not a valid plugin name: %s", p.Dir(), name, nameRule))
				continue
			}
			if other, ok := seen[name]; ok {
				errs = append(errs, fmt.Errorf("%s: %s already has the name %q, and plugin names are unique (ADR 12)", p.Dir(), other, name))
				continue
			}
			seen[name] = p.Dir()
			p, perrs := loadPlugin(fsys, category, name)
			errs = append(errs, perrs...)
			if len(perrs) == 0 {
				m.Plugins = append(m.Plugins, p)
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return m, nil
}

func loadData(fsys fs.FS) (Data, error) {
	var d Data
	src, err := fs.ReadFile(fsys, DataFile)
	if errors.Is(err, fs.ErrNotExist) {
		return d, errors.New("not found; run cech build at the root of a marketplace with CECH's layout")
	}
	if err != nil {
		return d, ReadError(err)
	}
	meta, err := toml.Decode(string(src), &d)
	if err != nil {
		return d, err
	}
	if keys := meta.Undecoded(); len(keys) > 0 {
		return d, fmt.Errorf("unknown key %q", keys[0].String())
	}
	if err := d.check(meta); err != nil {
		return d, err
	}
	return d, d.checkTerms(fsys)
}

// loadLicense reads LicenseFile, or returns nil when there is none.
func loadLicense(fsys fs.FS) ([]byte, error) {
	info, err := fs.Stat(fsys, LicenseFile)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, ReadError(err)
	case !info.Mode().IsRegular():
		return nil, errors.New("not a file")
	}
	return readFile(fsys, LicenseFile)
}

// check applies the rules of ADR 17 item 7 that need no other file.
func (d Data) check(meta toml.MetaData) error {
	switch {
	case d.Namespace == "":
		return errors.New("namespace is missing")
	case !isName(d.Namespace):
		return fmt.Errorf("namespace %q is not a valid name: %s", d.Namespace, nameRule)
	case versionEnd.MatchString(d.Namespace):
		return fmt.Errorf("namespace %q ends in a version, which a release adds (ADR 17 item 7)", d.Namespace)
	case d.Description == "":
		return errors.New("description is missing")
	case d.Owner.Name == "":
		return errors.New("owner.name is missing")
	case meta.IsDefined("team") && !team.MatchString(d.Team):
		return fmt.Errorf("team is %q, not a team as @org/team", d.Team)
	}
	if !meta.IsDefined("cech") {
		// The file is CECH's own.
		switch {
		case d.Namespace != "cech":
			return fmt.Errorf("namespace is %q, but a file without cech is CECH's, whose namespace is cech (ADR 17 item 7)", d.Namespace)
		case meta.IsDefined("categories"):
			return errors.New("categories is out of place: only an extension, with a cech key, adds categories")
		case meta.IsDefined("terms"):
			return errors.New("terms is out of place: only an extension, with a cech key, has term lists")
		}
		return checkRoles(d.Roles, "")
	}
	switch {
	case !release.MatchString(d.CECH):
		return fmt.Errorf("cech is %q, not a CECH release as X.Y.Z or X.Y.Z-rc.N", d.CECH)
	case d.Namespace == "cech" || strings.HasPrefix(d.Namespace, "cech-"):
		return fmt.Errorf("namespace %q is reserved for CECH (ADR 12 item 5)", d.Namespace)
	}
	if err := checkCategories(d.Categories, CECHCategories); err != nil {
		return err
	}
	return checkRoles(d.Roles, d.Namespace+"-")
}

// checkRoles checks that each role has a valid name, which starts with
// prefix and not with cech-, is not given twice, and says what it means
// (ADR 21 item 2). CECH's roles have no prefix.
func checkRoles(rs []Role, prefix string) error {
	seen := map[string]bool{}
	for i, r := range rs {
		switch {
		case !isName(r.Name):
			return fmt.Errorf("roles[%d].name %q is not a valid name: %s", i, r.Name, nameRule)
		case strings.HasPrefix(r.Name, "cech-"):
			return fmt.Errorf("roles[%d].name %q starts with cech-, which no role name does (ADR 21 item 2)", i, r.Name)
		case !strings.HasPrefix(r.Name, prefix):
			return fmt.Errorf("roles[%d].name %q does not start with %s, as an extension's roles do (ADR 21 item 2)", i, r.Name, prefix)
		case seen[r.Name]:
			return fmt.Errorf("roles[%d].name %q is already defined", i, r.Name)
		case r.Means == "":
			return fmt.Errorf("roles[%d].means is missing", i)
		}
		seen[r.Name] = true
	}
	return nil
}

// checkCategories checks that each category has a valid name, is not in
// taken and is not given twice, and says what it holds.
func checkCategories(cs, taken []Category) error {
	seen := map[string]bool{}
	for i, c := range cs {
		switch {
		case !isName(c.Name):
			return fmt.Errorf("categories[%d].name %q is not a valid name: %s", i, c.Name, nameRule)
		case slices.ContainsFunc(taken, func(t Category) bool { return t.Name == c.Name }):
			return fmt.Errorf("categories[%d].name %q is one of CECH's categories (ADR 12 item 1)", i, c.Name)
		case seen[c.Name]:
			return fmt.Errorf("categories[%d].name %q is already added", i, c.Name)
		case c.Holds == "":
			return fmt.Errorf("categories[%d].holds is missing", i)
		}
		seen[c.Name] = true
	}
	return nil
}

// checkTerms checks that each term list is a file inside the root.
func (d Data) checkTerms(fsys fs.FS) error {
	for _, list := range []struct {
		key   string
		paths []string
	}{{"terms.rules", d.Terms.Rules}, {"terms.names", d.Terms.Names}} {
		for _, p := range list.paths {
			if !fs.ValidPath(p) || p == "." {
				return fmt.Errorf("%s: %q is not a path from the repository root to a file inside it", list.key, p)
			}
			info, err := fs.Stat(fsys, p)
			switch {
			case errors.Is(err, fs.ErrNotExist):
				return fmt.Errorf("%s: %q is not found", list.key, p)
			case err != nil:
				return fmt.Errorf("%s: %w", list.key, ReadError(err))
			case !info.Mode().IsRegular():
				return fmt.Errorf("%s: %q is not a file", list.key, p)
			}
		}
	}
	return nil
}

// subdirs returns the sorted names of the folders in dir; a missing dir
// has none.
func subdirs(fsys fs.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, ReadError(err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}

func loadPlugin(fsys fs.FS, category, name string) (Plugin, []error) {
	p := Plugin{Category: category, Name: name}
	var errs []error
	fail := func(file string, err error) {
		errs = append(errs, fmt.Errorf("%s: %w", file, err))
	}

	manifest, err := readFile(fsys, p.ManifestPath())
	if err != nil {
		fail(p.ManifestPath(), err)
	} else if merr := p.readManifest(manifest); merr != nil {
		fail(p.ManifestPath(), merr)
	}

	skill, err := readFile(fsys, p.SkillPath())
	if err != nil {
		fail(p.SkillPath(), err)
	} else if serr := p.readSkill(skill); serr != nil {
		fail(p.SkillPath(), serr)
	}

	agentsDir := path.Join(p.Dir(), "agents")
	entries, err := fs.ReadDir(fsys, agentsDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fail(agentsDir, ReadError(err))
	}
	for _, e := range entries {
		file := path.Join(agentsDir, e.Name())
		switch {
		case e.IsDir():
		case strings.HasSuffix(e.Name(), ".toml"):
			p.AgentTOML = append(p.AgentTOML, e.Name())
		case strings.HasSuffix(e.Name(), ".md"):
			src, err := readFile(fsys, file)
			if err != nil {
				fail(file, err)
				continue
			}
			a, err := readAgent(strings.TrimSuffix(e.Name(), ".md"), src)
			if err != nil {
				fail(file, err)
				continue
			}
			p.Agents = append(p.Agents, a)
		}
	}
	return p, errs
}

// readFile reads the file at p. Its error for a missing file says only
// that, since the caller names the file, and the system's own message
// differs by OS; a denied read is ErrUnreadable.
func readFile(fsys fs.FS, p string) ([]byte, error) {
	data, err := fs.ReadFile(fsys, p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, errors.New("not found")
	case err != nil:
		return nil, ReadError(err)
	}
	return data, nil
}

// readOptions read plugin.json as encoding/json did: invalid UTF-8 in
// a string reads as U+FFFD rather than failing.
var readOptions = jsontext.AllowInvalidUTF8(true)

func (p *Plugin) readManifest(src []byte) error {
	name, ok, err := jsonedit.String(src, "name")
	switch {
	case err != nil:
		return err
	case !ok:
		return errors.New("name is missing")
	case name != p.Name:
		return fmt.Errorf("name is %q, not the folder's name %q", name, p.Name)
	}
	var top map[string]jsontext.Value
	if err := json.Unmarshal(src, &top, readOptions); err != nil {
		return err
	}
	if author, ok := top["author"]; ok {
		url, _, err := jsonedit.String(author, "url")
		if err != nil {
			return fmt.Errorf("author: %w", err)
		}
		p.AuthorURL = url
	}
	if meta, ok := top["metadata"]; ok {
		if err := p.readMaintainers(meta); err != nil {
			return fmt.Errorf("metadata: %w", err)
		}
	}
	p.Manifest = src
	return nil
}

// readMaintainers reads the maintainers in the manifest's metadata, an
// object the harnesses leave to the plugin.
func (p *Plugin) readMaintainers(meta jsontext.Value) error {
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(meta, &fields, readOptions); err != nil || fields == nil {
		return errors.New("not an object")
	}
	list, ok := fields["maintainers"]
	if !ok {
		return nil
	}
	var users []string
	if err := json.Unmarshal(list, &users, readOptions); err != nil || users == nil {
		return errors.New("maintainers is not an array of strings")
	}
	for _, user := range users {
		if !IsGitHubUser(user) {
			return fmt.Errorf("maintainers: %q is not a GitHub user name", user)
		}
	}
	p.Maintainers = users
	return nil
}

func (p *Plugin) readSkill(src []byte) error {
	fm, err := parseFrontmatter(src)
	if err != nil {
		return err
	}
	name, err := str(fm.fields, "name")
	if err != nil {
		return err
	}
	if name != p.Name {
		return fmt.Errorf("name is %q, not the plugin's name %q", name, p.Name)
	}
	if p.Description, err = str(fm.fields, "description"); err != nil {
		return err
	}
	if p.Description == "" {
		return errors.New("description is missing")
	}
	return nil
}

func readAgent(file string, src []byte) (Agent, error) {
	a := Agent{File: file}
	fm, err := parseFrontmatter(src)
	if err != nil {
		return a, err
	}
	if a.Name, err = str(fm.fields, "name"); err != nil {
		return a, err
	}
	if a.Description, err = str(fm.fields, "description"); err != nil {
		return a, err
	}
	switch {
	case a.Name == "":
		return a, errors.New("name is missing")
	case a.Description == "":
		return a, errors.New("description is missing")
	}
	a.Instructions = fm.body
	return a, nil
}
