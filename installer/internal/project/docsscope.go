package project

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Der Scope der Docs-Erzeuger, die den Code durchsuchen (/k-docs-code und
// /k-docs-tools), als Rechnung im Programm (Task 179): k-playbook docs scope.
// Die Regeln sind die des Abschnitts „Scope“ in k-playbook:/commands/_shared/docs.md;
// das Programm entscheidet nur, was sich rechnen lässt. Ob ein anderer Stand
// beschrieben wird und was der Nutzer im Preflight ergänzt, bleibt bei der
// Sitzung. Der Scope liest nur lokal (K-PLAYBOOK.yaml, git, Dateisystem) und
// braucht keine Kephalaion-Anbindung.

// docsScopeAlways sind die Verzeichnisse, die nie zum Scan gehören.
var docsScopeAlways = []string{"k-playbook/", ".k-playbook/", "keph-backup/", ".git/"}

// docsScopeFurther sind die weiteren Ausschlüsse ohne Rückfrage; sie gelten
// auf jeder Ebene.
var docsScopeFurther = []string{
	"venv/", "env/", "node_modules/", "dist/", "build/", "target/", "_bases/", "__pycache__/",
	"coverage/", "htmlcov/", "tests/fixtures/", "**/*.min.js", "**/*.lock", "**/*.bundle.*",
}

// docsScopeMaxGitignores begrenzt, wie viele .gitignore der Scope einsammelt.
const docsScopeMaxGitignores = 200

// DocsScopeStand ist der Stand von SCAN_ROOT: Branch, Commit und ob
// `git status --porcelain` leer ist. Git ist false, wenn SCAN_ROOT kein
// git-Arbeitsbaum ist oder git nicht antwortet; Message sagt dann warum.
type DocsScopeStand struct {
	Git     bool   `json:"git"`
	Branch  string `json:"branch,omitempty"`
	Commit  string `json:"commit,omitempty"`
	Clean   bool   `json:"clean"`
	Message string `json:"message,omitempty"`
}

// DocsScopeExtra ist der zusätzliche Pfad EXTRA_PATH samt Prüfung. Path ist
// der Pfad ab Projektverzeichnis, Abs der absolute.
type DocsScopeExtra struct {
	Path           string `json:"path"`
	Abs            string `json:"abs"`
	Exists         bool   `json:"exists"`
	InProject      bool   `json:"inProject"`
	AlwaysExcluded bool   `json:"alwaysExcluded"`
}

// DocsScopeHidden sagt, ob versteckte Pfade ausgeschlossen sind, und nennt
// die Ausnahmen aus dem Argument.
type DocsScopeHidden struct {
	Excluded bool     `json:"excluded"`
	Excepted []string `json:"excepted,omitempty"`
}

// DocsScopeIgnoreFile sind die Muster einer .gitignore unter SCAN_ROOT; File
// ist ihr Pfad ab SCAN_ROOT.
type DocsScopeIgnoreFile struct {
	File     string   `json:"file"`
	Patterns []string `json:"patterns"`
}

// DocsScopeExclude sind die wirksamen Ausschlüsse nach Herkunft.
type DocsScopeExclude struct {
	Always    []string              `json:"always"`
	Hidden    DocsScopeHidden       `json:"hidden"`
	Further   []string              `json:"further"`
	Gitignore []DocsScopeIgnoreFile `json:"gitignore"`
	Config    []string              `json:"config"`
	Rejected  []DocsExcludeProblem  `json:"rejected"`
}

// DocsScopeCheck ist das Ergebnis von --check für einen Pfad: Excluded, ob er
// ausgeschlossen ist, Reason die Herkunft (always, hidden, further,
// gitignore, config) und By das Muster oder der Name, der ihn trifft.
type DocsScopeCheck struct {
	Path     string `json:"path"`
	Excluded bool   `json:"excluded"`
	Reason   string `json:"reason,omitempty"`
	By       string `json:"by,omitempty"`
}

// DocsScopeResult ist die Antwort von docs scope.
type DocsScopeResult struct {
	ScanRoot   string           `json:"scanRoot"`
	ProjectDir string           `json:"projectDir"`
	Stand      DocsScopeStand   `json:"stand"`
	Extra      *DocsScopeExtra  `json:"extra,omitempty"`
	Exclude    DocsScopeExclude `json:"exclude"`
	Checks     []DocsScopeCheck `json:"checks,omitempty"`
}

// DocsScopeOptions sind die Eingaben: ExtraPath ergänzt den Scope um genau
// einen Pfad, Check nennt Pfade, für die gemeldet wird, ob sie ausgeschlossen
// sind (relativ zu SCAN_ROOT oder absolut).
type DocsScopeOptions struct {
	ExtraPath string
	Check     []string
}

type docsScope struct {
	projectDir string
	scanRoot   string
	always     []ignoreRule
	further    []ignoreRule
	config     []ignoreRule
	ignores    []docsScopeIgnores
	extraRel   string // ab SCAN_ROOT, leer ohne EXTRA_PATH
}

type docsScopeIgnores struct {
	dir   string // ab SCAN_ROOT, "" für die Wurzel
	rules []ignoreRule
}

// DocsScope rechnet den Scope aus K-PLAYBOOK.yaml, dem Stand von git und dem
// Dateisystem.
func DocsScope(ctx context.Context, projectDir string, opts DocsScopeOptions) (DocsScopeResult, error) {
	config, err := ReadConfig(projectDir)
	if err != nil {
		return DocsScopeResult{}, docRefErrorf(DocCodeInvalidInput, "K-PLAYBOOK.yaml nicht lesbar: %v", err)
	}
	scanRoot := RepoRootDir(projectDir, config)
	docs := contextDocs(projectDir, config)
	result := DocsScopeResult{ScanRoot: scanRoot, ProjectDir: projectDir}
	result.Stand = docsScopeStand(ctx, scanRoot)

	scope := &docsScope{projectDir: projectDir, scanRoot: scanRoot}
	scope.always = compileIgnoreRules(docsScopeAlways, true)
	scope.further = compileIgnoreRules(docsScopeFurther, true)
	scope.config = compileIgnoreRules(docs.Exclude, false)

	if strings.TrimSpace(opts.ExtraPath) != "" {
		extra, err := scope.extra(opts.ExtraPath)
		if err != nil {
			return DocsScopeResult{}, err
		}
		result.Extra = &extra
	}

	result.Exclude = DocsScopeExclude{
		Always:   append([]string{}, docsScopeAlways...),
		Hidden:   DocsScopeHidden{Excluded: true},
		Further:  append([]string{}, docsScopeFurther...),
		Config:   append([]string{}, docs.Exclude...),
		Rejected: append([]DocsExcludeProblem{}, docs.Problems...),
	}
	if scope.extraRel != "" && docsHiddenPath(scope.extraRel) {
		result.Exclude.Hidden.Excepted = []string{scope.extraRel}
	}
	files := scope.collectGitignores()
	result.Exclude.Gitignore = make([]DocsScopeIgnoreFile, 0, len(files))
	for _, file := range files {
		result.Exclude.Gitignore = append(result.Exclude.Gitignore, file)
	}

	for _, raw := range opts.Check {
		result.Checks = append(result.Checks, scope.check(raw))
	}
	return result, nil
}

// docsScopeStand liest Branch, Commit und Sauberkeit von SCAN_ROOT.
func docsScopeStand(ctx context.Context, scanRoot string) DocsScopeStand {
	stand := DocsScopeStand{}
	branch, code, reason := runGit(ctx, scanRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if code != 0 {
		stand.Message = "kein git-Arbeitsbaum oder git antwortet nicht"
		if reason != "" {
			stand.Message += ": " + reason
		}
		return stand
	}
	stand.Git = true
	stand.Branch = strings.TrimSpace(branch)
	if commit, code, _ := runGit(ctx, scanRoot, "log", "-1", "--format=%h"); code == 0 {
		stand.Commit = strings.TrimSpace(commit)
	}
	if status, code, reason := runGit(ctx, scanRoot, "status", "--porcelain"); code == 0 {
		stand.Clean = strings.TrimSpace(status) == ""
	} else if reason != "" {
		stand.Message = "git status: " + reason
	}
	return stand
}

// extra prüft EXTRA_PATH: Er muss existieren, innerhalb des
// Projektverzeichnisses liegen und nicht in einem immer ausgeschlossenen
// Verzeichnis. Relativ gilt er ab dem Projektverzeichnis.
func (s *docsScope) extra(arg string) (DocsScopeExtra, error) {
	abs := arg
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(s.projectDir, abs)
	}
	abs = filepath.Clean(abs)
	extra := DocsScopeExtra{Abs: abs}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		extra.Exists = true
		abs = resolved
	}
	root := s.projectDir
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	rel, err := filepath.Rel(root, abs)
	extra.InProject = err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
	if extra.InProject {
		extra.Path = filepath.ToSlash(rel)
	}
	switch {
	case !extra.InProject:
		return extra, docRefErrorf(DocCodeInvalidInput,
			"%s liegt außerhalb von %s — ein anderes Projekt wird dort dokumentiert, nicht von hier", arg, s.projectDir)
	case !extra.Exists:
		return extra, docRefErrorf(DocCodeInvalidInput, "%s existiert nicht", extra.Abs)
	}
	if s.alwaysExcludedAbs(abs, root) {
		extra.AlwaysExcluded = true
		return extra, docRefErrorf(DocCodeInvalidInput,
			"%s liegt in einem Verzeichnis, das nie zum Scan gehört (k-playbook/, .k-playbook/, keph-backup/, .git/)", arg)
	}
	if scanRel, err := filepath.Rel(s.scanRoot, abs); err == nil && !strings.HasPrefix(scanRel, "..") {
		s.extraRel = filepath.ToSlash(scanRel)
		if s.extraRel == "." {
			s.extraRel = ""
		}
	}
	return extra, nil
}

// alwaysExcludedAbs prüft einen absoluten Pfad gegen die immer
// ausgeschlossenen Verzeichnisse: <projectDir>/.k-playbook/ und
// <projectDir>/k-playbook/, dazu keph-backup/ und .git/ auf jeder Ebene.
func (s *docsScope) alwaysExcludedAbs(abs string, projectRoot string) bool {
	for _, name := range []string{ProjectDirName, "k-playbook"} {
		for _, base := range []string{s.projectDir, projectRoot} {
			dir := filepath.Join(base, name)
			if abs == dir || strings.HasPrefix(abs, dir+string(filepath.Separator)) {
				return true
			}
		}
	}
	for _, part := range strings.Split(filepath.ToSlash(abs), "/") {
		if part == "keph-backup" || part == ".git" {
			return true
		}
	}
	return false
}

// docsHiddenPath sagt, ob ein Pfadteil mit . beginnt.
func docsHiddenPath(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if strings.HasPrefix(part, ".") && part != "." && part != ".." {
			return true
		}
	}
	return false
}

// collectGitignores sammelt die .gitignore unter SCAN_ROOT, die Wurzel
// zuerst. Verzeichnisse, die ohnehin ausgeschlossen sind, werden nicht
// betreten.
func (s *docsScope) collectGitignores() []DocsScopeIgnoreFile {
	files := []DocsScopeIgnoreFile{}
	filepath.WalkDir(s.scanRoot, func(full string, entry fs.DirEntry, err error) error {
		if err != nil {
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(s.scanRoot, full)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel == "." {
				return nil
			}
			if _, ok := ignoreVerdict(s.always, rel, true); ok {
				return filepath.SkipDir
			}
			if strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			if _, ok := ignoreVerdict(s.further, rel, true); ok {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != ".gitignore" || len(files) >= docsScopeMaxGitignores {
			return nil
		}
		data, readErr := os.ReadFile(full)
		if readErr != nil {
			return nil
		}
		patterns := []string{}
		for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			line = strings.TrimRight(line, " \t")
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			patterns = append(patterns, line)
		}
		if len(patterns) == 0 {
			return nil
		}
		dir := filepath.ToSlash(filepath.Dir(rel))
		if dir == "." {
			dir = ""
		}
		files = append(files, DocsScopeIgnoreFile{File: rel, Patterns: patterns})
		s.ignores = append(s.ignores, docsScopeIgnores{dir: dir, rules: compileIgnoreRules(patterns, false)})
		return nil
	})
	depth := func(path string) int { return strings.Count(path, "/") }
	sort.SliceStable(files, func(i, j int) bool { return depth(files[i].File) < depth(files[j].File) })
	sort.SliceStable(s.ignores, func(i, j int) bool {
		return depth(gitignoreFileName(s.ignores[i].dir)) < depth(gitignoreFileName(s.ignores[j].dir))
	})
	return files
}

// check meldet, ob ein Pfad ausgeschlossen ist und warum. Relativ gilt der
// Pfad ab SCAN_ROOT.
func (s *docsScope) check(raw string) DocsScopeCheck {
	item := DocsScopeCheck{Path: raw}
	abs := raw
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(s.scanRoot, abs)
	}
	abs = filepath.Clean(abs)
	root := s.scanRoot
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	isDir := strings.HasSuffix(raw, "/")
	exists := false
	if info, err := os.Stat(abs); err == nil {
		isDir, exists = info.IsDir(), true
	}
	// Einen Pfad, den es nicht gibt, kann niemand als Datei oder Verzeichnis
	// belegen: Er gilt als Verzeichnis, wenn ihn ein Muster nur für
	// Verzeichnisse trifft.
	if !exists {
		isDir = true
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		item.Excluded, item.Reason, item.By = true, "outside", "liegt außerhalb von "+s.scanRoot
		return item
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return item
	}
	if s.alwaysExcludedAbs(abs, s.projectDir) {
		item.Excluded, item.Reason = true, "always"
		item.By = docsScopeAlwaysName(abs, s.projectDir)
		return item
	}
	explicit := s.extraRel != "" && (rel == s.extraRel || strings.HasPrefix(rel, s.extraRel+"/"))
	if !explicit {
		if docsHiddenPath(rel) {
			item.Excluded, item.Reason, item.By = true, "hidden", "."
			return item
		}
	} else if strings.Contains("/"+rel+"/", "/.git/") {
		item.Excluded, item.Reason, item.By = true, "always", ".git/"
		return item
	}
	if hit, ok := ignoreVerdict(s.further, rel, isDir); ok {
		item.Excluded, item.Reason, item.By = true, "further", hit.source
		return item
	}
	if file, hit, ok := s.gitignoreVerdict(rel, isDir); ok {
		item.Excluded, item.Reason, item.By = true, "gitignore", file+": "+hit.source
		return item
	}
	if !explicit {
		if hit, ok := ignoreVerdict(s.config, rel, isDir); ok {
			item.Excluded, item.Reason, item.By = true, "config", hit.source
		}
	}
	return item
}

// docsScopeAlwaysName nennt, welches immer ausgeschlossene Verzeichnis einen
// Pfad trifft.
func docsScopeAlwaysName(abs string, projectDir string) string {
	for _, name := range []string{ProjectDirName, "k-playbook"} {
		dir := filepath.Join(projectDir, name)
		if abs == dir || strings.HasPrefix(abs, dir+string(filepath.Separator)) {
			return name + "/"
		}
	}
	for _, part := range strings.Split(filepath.ToSlash(abs), "/") {
		if part == "keph-backup" {
			return "keph-backup/"
		}
	}
	return ".git/"
}

// gitignoreVerdict wertet die gesammelten .gitignore für einen Pfad aus: Je
// Ebene gilt ihre Datei mit dem Pfad ab ihrem Verzeichnis, die tiefere
// zuletzt.
func (s *docsScope) gitignoreVerdict(rel string, isDir bool) (string, ignoreRule, bool) {
	parts := strings.Split(rel, "/")
	for i := range parts {
		prefix := strings.Join(parts[:i+1], "/")
		dir := i < len(parts)-1 || isDir
		var hit ignoreRule
		file := ""
		excluded := false
		for _, entry := range s.ignores {
			below := prefix
			if entry.dir != "" {
				trimmed, ok := strings.CutPrefix(prefix, entry.dir+"/")
				if !ok {
					continue
				}
				below = trimmed
			}
			for _, rule := range entry.rules {
				if rule.dirOnly && !dir {
					continue
				}
				if rule.re.MatchString(below) {
					excluded = !rule.negate
					hit, file = rule, gitignoreFileName(entry.dir)
				}
			}
		}
		if excluded {
			return file, hit, true
		}
	}
	return "", ignoreRule{}, false
}

func gitignoreFileName(dir string) string {
	if dir == "" {
		return ".gitignore"
	}
	return dir + "/.gitignore"
}
