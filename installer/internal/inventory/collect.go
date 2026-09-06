package inventory

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/versionsources"
)

// Options beschreibt einen Erhebungslauf. Der Aufrufer — Subkommando, Command
// oder später die Web-API — löst Pfade auf und reicht sie herein; die Fachlogik
// sucht sich nichts selbst.
type Options struct {
	// ProjectDir ist die Projektwurzel, das Verzeichnis der K-PLAYBOOK.yaml.
	// Sie ist immer eine erlaubte Wurzel.
	ProjectDir string
	// SourcesFile ist k-playbook-local/version-sources.yaml.
	SourcesFile string
	// InventoryFile ist k-playbook-local/docs/versions/inventory.md.
	InventoryFile string
	// Now liefert den Erhebungszeitpunkt. Fehlt es, gilt time.Now.
	Now func() time.Time
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// Der Ort der Inventardatei innerhalb des lokalen Verzeichnisses. Die Namen
// stehen im Vertrag und werden hier einmal gebunden, damit jeder Aufrufweg —
// Subkommando, Command und die Oberfläche aus Task 043 — denselben Pfad
// benutzt, statt ihn ein zweites Mal zusammenzusetzen.
const (
	localDocsDirName = "docs"
	// VersionsDirName ist die fünfte Docs-Herkunft. Sie entsteht beim ersten
	// Lauf ihres Erzeugers, nicht beim Einrichten.
	VersionsDirName = "versions"
	// FileName ist die Inventardatei selbst.
	FileName = "inventory.md"
)

// FilePath ist der Ort der Inventardatei zu einem lokalen Verzeichnis:
// <local.dir>/docs/versions/inventory.md.
func FilePath(localDir string) string {
	return filepath.Join(localDir, localDocsDirName, VersionsDirName, FileName)
}

// Collect erhebt das Inventar, ohne etwas zu schreiben.
//
// Der Rückgabefehler ist der abbrechende Fall und nur er: eine unlesbare oder
// fremd versionierte Quellenkonfiguration. Alles andere — abgelehnte Pfade,
// fehlende Quellen, defekte Dateien — steht im Ergebnis, weil eine Lücke, die
// niemand sieht, schlimmer ist als ein Fehler.
func Collect(options Options) (Result, error) {
	config, err := versionsources.Read(options.SourcesFile)
	if err != nil {
		return Result{}, err
	}
	boundary, err := NewBoundary(options.ProjectDir, config.Roots)
	if err != nil {
		return Result{}, err
	}

	result := Result{ConfiguredSources: len(config.Sources)}
	for _, rejection := range config.Rejections {
		result.Rejections = append(result.Rejections, Rejection{
			Requested: rejection.Path,
			Reason: fmt.Sprintf("%s (%s, Zeile %d)", rejection.Reason,
				filepath.Base(options.SourcesFile), rejection.Line),
		})
	}

	candidates, exclusions, rejections := planSources(boundary, config)
	result.Rejections = append(result.Rejections, rejections...)
	result.Exclusions = exclusions

	for _, item := range candidates {
		readCandidate(boundary, item, config.Exclude, &result)
	}

	sortEntries(result.Entries)
	result.Deviations = buildDeviations(result.Entries)
	sortSources(result.Sources)
	sortRejections(result.Rejections)
	sortNotes(result.Notes)
	return result, nil
}

// planSources führt Standardquellen und konfigurierte Zusatzquellen zusammen.
//
// Die Standardquellen werden ergänzt, nicht ersetzt: es gibt keinen Schalter,
// der die Standarderkennung abschaltet — ein Inventar, das die Projektquellen
// nicht führt, wäre keines. Nennt die Konfiguration dieselbe Datei noch einmal,
// gewinnt sie: ihr Label ist ausdrücklich gesetzt, das der Standarderkennung
// abgeleitet.
//
// Die Ausschlüsse wirken deshalb auch nur auf die Standarderkennung. Wer die
// Installation oder einen ausgeschlossenen Bereich doch im Inventar haben will,
// schreibt die Quelle in `sources:` — dann steht sie da, ausdrücklich und mit
// eigenem Label.
func planSources(boundary *Boundary, config versionsources.Config) ([]candidate, []Exclusion, []Rejection) {
	var rejections []Rejection
	skip := newExcluder(config.Exclude)
	order := []string{}
	byPath := map[string]candidate{}

	remember := func(item candidate) {
		resolved, err := boundary.Check(item.Requested)
		if err != nil {
			rejections = append(rejections, err.(*PathError).Rejection())
			return
		}
		if _, seen := byPath[resolved]; !seen {
			order = append(order, resolved)
		}
		byPath[resolved] = item
	}

	for _, item := range discoverDefaults(boundary.ProjectRoot(), skip) {
		remember(item)
	}

	for _, source := range config.Valid() {
		paths, globRejections := boundary.Expand(source.Path)
		rejections = append(rejections, globRejections...)
		for _, path := range paths {
			kind, known := resolveKind(source.Kind, path)
			if !known {
				rejections = append(rejections, Rejection{
					Requested: path,
					Reason:    "Quellart nicht am Namen erkennbar; `kind:` in der Quellenkonfiguration ausdrücklich setzen",
				})
				continue
			}
			remember(candidate{
				Requested:  path,
				Kind:       kind,
				Env:        source.Env,
				EnvOrigin:  ContextConfigured,
				Note:       source.Note,
				Optional:   source.Optional,
				Configured: true,
			})
		}
	}

	sort.Strings(order)
	planned := make([]candidate, 0, len(order))
	for _, resolved := range order {
		planned = append(planned, byPath[resolved])
	}
	return planned, skip.exclusions(), rejections
}

// readCandidate öffnet eine geplante Quelle — ausschließlich über die
// Vertrauensgrenze — und wertet sie aus.
func readCandidate(boundary *Boundary, item candidate, excludes []string, result *Result) {
	data, resolved, exists, err := boundary.ReadFile(item.Requested)
	if err != nil {
		var pathError *PathError
		if errorsAs(err, &pathError) {
			result.Rejections = append(result.Rejections, pathError.Rejection())
			return
		}
		result.Rejections = append(result.Rejections, Rejection{
			Requested: item.Requested, Resolved: resolved, Reason: err.Error()})
		return
	}
	display := displayPath(boundary.ProjectRoot(), resolved)
	if !exists {
		if item.Optional {
			return
		}
		result.Notes = append(result.Notes, Note{Source: display,
			Text: "konfigurierte Quelle liegt nicht auf der Platte"})
		return
	}

	context := fileContext{
		Display:   display,
		Base:      filepath.Base(resolved),
		Kind:      item.Kind,
		Env:       item.Env,
		EnvOrigin: item.EnvOrigin,
		Data:      data,
	}
	if manifest := lockManifest(item.Kind, filepath.Base(resolved)); manifest != "" {
		direct, note := lockDirect(boundary, resolved, manifest, item, excludes)
		if note != "" {
			result.Notes = append(result.Notes, Note{Source: display, Text: note})
		}
		context.Direct = direct
	}
	entries, notes := parseFile(context)
	result.Entries = append(result.Entries, entries...)
	result.Notes = append(result.Notes, notes...)
	result.Sources = append(result.Sources, SourceRead{
		File:       display,
		Kind:       item.Kind,
		Env:        item.Env,
		Entries:    len(entries),
		Configured: item.Configured,
		Note:       item.Note,
	})
}

// lockManifest benennt die direkte Referenzquelle eines Lockfiles. Die
// Workspace-Erweiterungen werden beim Manifestlesen ergänzt; ohne lesbares
// Wurzelmanifest gibt es bewusst keine Lockfile-Einträge.
var lockManifests = map[string]string{
	"yarn.lock":     "package.json",
	"poetry.lock":   "pyproject.toml",
	"uv.lock":       "pyproject.toml",
	"Pipfile.lock":  "Pipfile",
	"Cargo.lock":    "Cargo.toml",
	"composer.lock": "composer.json",
	"mix.lock":      "mix.exs",
}

func lockManifest(kind, base string) string { return lockManifests[base] }

func lockDirect(boundary *Boundary, lockPath, manifest string, item candidate, excludes []string) (map[string]string, string) {
	manifestPath := filepath.Join(filepath.Dir(lockPath), manifest)
	displayLock := displayPath(boundary.ProjectRoot(), lockPath)
	displayManifest := displayPath(boundary.ProjectRoot(), manifestPath)
	if relative, err := filepath.Rel(boundary.ProjectRoot(), manifestPath); err == nil {
		relative = filepath.ToSlash(relative)
		for _, pattern := range excludes {
			if matchExclude(pattern, relative) {
				return nil, fmt.Sprintf("zugehöriges Manifest %s ist durch Ausschlussregel %q ausgenommen", displayManifest, pattern)
			}
		}
	}
	data, resolved, exists, err := boundary.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Sprintf("zugehöriges Manifest %s für Lockfile %s nicht lesbar: %v", displayManifest, displayLock, err)
	}
	if !exists {
		return nil, fmt.Sprintf("zugehöriges Manifest %s für Lockfile %s fehlt", displayManifest, displayLock)
	}
	entries, notes := parseFile(fileContext{Display: displayPath(boundary.ProjectRoot(), resolved), Base: manifest,
		Kind: item.Kind, Env: item.Env, EnvOrigin: item.EnvOrigin, Data: data})
	if len(notes) > 0 {
		return nil, fmt.Sprintf("zugehöriges Manifest %s für Lockfile %s ist nicht lesbar: %s", displayManifest, displayLock, notes[0].Text)
	}
	direct := map[string]string{}
	for _, entry := range entries {
		if entry.KindOfThing == ThingPackage {
			direct[entry.Name] = entry.Scope
		}
	}
	for _, declaration := range workspaceManifests(manifestPath, manifest, data) {
		members, rejections := boundary.Expand(declaration)
		if len(rejections) > 0 {
			return nil, fmt.Sprintf("Workspace-Mitglied %s für Lockfile %s nicht auflösbar: %s", declaration, displayLock, rejections[0].Reason)
		}
		if len(members) == 0 {
			return nil, fmt.Sprintf("Workspace-Mitglied %s für Lockfile %s nicht auflösbar", declaration, displayLock)
		}
		for _, member := range members {
			if relative, err := filepath.Rel(boundary.ProjectRoot(), member); err == nil {
				relative = filepath.ToSlash(relative)
				for _, pattern := range excludes {
					if matchExclude(pattern, relative) {
						return nil, fmt.Sprintf("Workspace-Manifest %s ist durch Ausschlussregel %q ausgenommen", relative, pattern)
					}
				}
			}
			memberData, memberResolved, memberExists, memberErr := boundary.ReadFile(member)
			if memberErr != nil || !memberExists {
				reason := "fehlt"
				if memberErr != nil {
					reason = memberErr.Error()
				}
				return nil, fmt.Sprintf("Workspace-Manifest %s für Lockfile %s %s", displayPath(boundary.ProjectRoot(), member), displayLock, reason)
			}
			memberEntries, memberNotes := parseFile(fileContext{Display: displayPath(boundary.ProjectRoot(), memberResolved), Base: manifest,
				Kind: item.Kind, Env: item.Env, EnvOrigin: item.EnvOrigin, Data: memberData})
			if len(memberNotes) > 0 {
				return nil, fmt.Sprintf("Workspace-Manifest %s für Lockfile %s ist nicht lesbar: %s", displayPath(boundary.ProjectRoot(), member), displayLock, memberNotes[0].Text)
			}
			for _, entry := range memberEntries {
				if entry.KindOfThing == ThingPackage {
					direct[entry.Name] = entry.Scope
				}
			}
			if manifest == "Cargo.toml" {
				addCargoWorkspaceDependencies(direct, data, memberData)
			}
		}
	}
	return direct, ""
}

// addCargoWorkspaceDependencies ergänzt nur die zentral deklarierte Abhängigkeit,
// die ein Mitglied tatsächlich mit `workspace = true` verwendet.
func addCargoWorkspaceDependencies(direct map[string]string, root, member []byte) {
	rootDoc := parseTOML(root)
	dependencies := rootDoc.table("workspace.dependencies")
	if dependencies == nil {
		return
	}
	memberDoc := parseTOML(member)
	for _, section := range []string{"dependencies", "dev-dependencies", "build-dependencies"} {
		table := memberDoc.table(section)
		if table == nil {
			continue
		}
		for _, name := range table.Keys {
			entry := table.Entries[name]
			fields := tomlInline(entry.Raw)
			if fields == nil || strings.TrimSpace(fields["workspace"]) != "true" {
				continue
			}
			if _, declared := dependencies.get(name); declared {
				direct[normalizeName(EcoRust, name)] = map[string]string{"dependencies": "main", "dev-dependencies": "dev", "build-dependencies": "build"}[section]
			}
		}
	}
}

// workspaceManifests löst die deklarierte Workspace-Mitgliedsmenge gegen das
// Wurzelmanifest auf. Ein leeres Ergebnis bedeutet kein Workspace, nicht einen
// Fehler; nicht auflösbare Mitglieder werden beim anschließenden Boundary-Lesen
// sichtbar und machen die gesamte Lockfile-Referenzmenge ungültig.
func workspaceManifests(manifestPath, base string, data []byte) []string {
	dir := filepath.Dir(manifestPath)
	var members []string
	switch base {
	case "package.json":
		var doc struct {
			Workspaces json.RawMessage `json:"workspaces"`
		}
		if json.Unmarshal(data, &doc) == nil {
			var paths []string
			if json.Unmarshal(doc.Workspaces, &paths) != nil {
				var object struct {
					Packages []string `json:"packages"`
				}
				_ = json.Unmarshal(doc.Workspaces, &object)
				paths = object.Packages
			}
			for _, path := range paths {
				members = append(members, filepath.Join(dir, path, "package.json"))
			}
		}
	case "Cargo.toml", "pyproject.toml":
		doc := parseTOML(data)
		table := "workspace"
		if base == "pyproject.toml" {
			table = "tool.uv.workspace"
		}
		if workspace := doc.table(table); workspace != nil {
			if entry, ok := workspace.get("members"); ok {
				for _, member := range tomlArray(entry.Raw, entry.Line) {
					if value, ok := tomlString(member.Raw); ok {
						members = append(members, filepath.Join(dir, value, base))
					}
				}
			}
		}
	case "mix.exs":
		appsPath := "apps"
		hasAppsPath := false
		for _, line := range strings.Split(string(data), "\n") {
			if index := strings.Index(line, "apps_path:"); index >= 0 {
				value := strings.TrimSpace(line[index+len("apps_path:"):])
				if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') {
					if end := strings.IndexByte(value[1:], value[0]); end >= 0 {
						value = value[1 : end+1]
					}
				}
				value = strings.Trim(value, "\"', ")
				if value != "" {
					appsPath = value
					hasAppsPath = true
				}
			}
		}
		if hasAppsPath {
			members = append(members, filepath.Join(dir, appsPath, "*", "mix.exs"))
		}
	}
	sort.Strings(members)
	return members
}

// displayPath ist der Wert für Entry.SourceFile: projektrelativ mit `/` als
// Trenner, auch unter Windows. Liegt die Datei außerhalb der Projektwurzel,
// steht dort der absolute Pfad — sonst führte ein `../../..` aus dem Inventar
// heraus und sagte weniger als der Pfad selbst.
func displayPath(projectRoot string, resolved string) string {
	relative, err := filepath.Rel(projectRoot, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(resolved)
	}
	return filepath.ToSlash(relative)
}

// errorsAs hält die eine Typprüfung an einer Stelle; das Paket kennt genau
// einen Fehlertyp mit Zusatzinformation.
func errorsAs(err error, target **PathError) bool {
	if pathError, ok := err.(*PathError); ok {
		*target = pathError
		return true
	}
	return false
}
