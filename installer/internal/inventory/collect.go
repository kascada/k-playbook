package inventory

import (
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
		readCandidate(boundary, item, &result)
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
func readCandidate(boundary *Boundary, item candidate, result *Result) {
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

	entries, notes := parseFile(fileContext{
		Display:   display,
		Base:      filepath.Base(resolved),
		Kind:      item.Kind,
		Env:       item.Env,
		EnvOrigin: item.EnvOrigin,
		Data:      data,
	})
	result.Entries = append(result.Entries, entries...)
	result.Notes = append(result.Notes, notes...)
	result.Sources = append(result.Sources, SourceRead{
		File:       display,
		Kind:       item.Kind,
		Env:        item.Env,
		Entries:    len(entries),
		Configured: item.Configured,
	})
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
