// Package workdir ist der Weg eines eigenständigen Reviews (/k-review, Task
// 197) im Programm: das Arbeitsverzeichnis, in das ein Rezept seine Fundliste
// als SARIF schreibt, und der Merge auf dieses Verzeichnis samt Prüfung gegen
// den Vertrag des Rezepts. CLI (`k-playbook review`) und MCP
// (k_playbook_review_workdir, k_playbook_review_merge_dir) sind Hüllen
// darüber.
//
// Dieser Weg legt keinen Audit-Lauf an und liest keinen; er steht deshalb
// nicht hinter review.Paused. Die übrigen Review-Wege bleiben gesperrt.
package workdir

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review"
	"github.com/kascada/k-playbook/installer/internal/review/merge"
)

// dirPerm sind die Rechte eines Arbeitsverzeichnisses: nur der eigene
// Benutzer. Die Rohdaten eines Reviews können Secrets enthalten.
const dirPerm os.FileMode = 0o700

// TempRoot ist das Verzeichnis, unter dem das Standardverzeichnis entsteht:
// fest /tmp, nicht os.TempDir(). Zeigt $TMPDIR auf ein Unterverzeichnis,
// läge dazwischen eine Zwischenstufe, die niemand prüft (Task 197, Etappe 2).
// Nur Tests setzen es um.
var TempRoot = "/tmp"

// Prepared ist das Ergebnis von Prepare.
type Prepared struct {
	// Dir ist der absolute Pfad des Arbeitsverzeichnisses.
	Dir string `json:"dir"`
	// Created meldet, ob das Verzeichnis eben angelegt wurde.
	Created bool `json:"created"`
	// Default meldet das Standardverzeichnis unter /tmp.
	Default bool `json:"default"`
	// InsideProject meldet ein Verzeichnis im Projekt: Scanner späterer Läufe
	// läsen es mit.
	InsideProject bool `json:"insideProject"`
	// Warnings sind Sätze für den Nutzer.
	Warnings []string `json:"warnings,omitempty"`
}

// slugPattern trifft alles, was in einem Namensteil nichts zu suchen hat.
var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slug(value string, fallback string) string {
	text := strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(value), "-"), "-")
	if text == "" {
		return fallback
	}
	if len(text) > 40 {
		text = strings.TrimRight(text[:40], "-")
	}
	return text
}

// Prepare bestimmt das Arbeitsverzeichnis eines Reviews.
//
// Ohne dir legt es ein neues Verzeichnis unmittelbar unter /tmp an (TempRoot,
// $TMPDIR gilt nicht), über os.MkdirTemp: Der Name trägt Projekt, Rezept und
// Zeitstempel und endet auf einen zufälligen Teil, das Verzeichnis entsteht
// exklusiv mit 0700. Es gibt keine Zwischenstufe zwischen /tmp und dem Blatt,
// die ein anderer Benutzer vorher hätte anlegen können; ein je Benutzer
// vorhersagbarer Name reichte dafür nicht.
//
// Mit dir nimmt es diesen Pfad: absolut gemacht, angelegt mit 0700, wenn er
// fehlt, sonst muss er ein Verzeichnis sein. Liegt er im Projekt oder ist er
// für andere offen, kommt eine Warnung; geändert wird an einem vorhandenen
// Verzeichnis nichts.
func Prepare(projectDir string, recipe string, dir string, now time.Time) (Prepared, error) {
	if strings.TrimSpace(dir) == "" {
		pattern := fmt.Sprintf("k-review-%s-%s-%s-",
			slug(filepath.Base(filepath.Clean(projectDir)), "projekt"), slug(recipe, "review"), now.Format("20060102-150405"))
		created, err := os.MkdirTemp(TempRoot, pattern)
		if err != nil {
			return Prepared{}, err
		}
		// MkdirTemp legt mit 0700 an, abzüglich umask; das kann nur enger
		// werden. Gesetzt wird trotzdem, damit der Wert nicht an der umask
		// hängt.
		if err := os.Chmod(created, dirPerm); err != nil {
			return Prepared{}, err
		}
		absolute, err := filepath.Abs(created)
		if err != nil {
			return Prepared{}, err
		}
		prepared := Prepared{Dir: absolute, Created: true, Default: true}
		prepared.InsideProject = inside(projectDir, absolute)
		if prepared.InsideProject {
			prepared.Warnings = append(prepared.Warnings, insideWarning)
		}
		return prepared, nil
	}

	absolute, err := filepath.Abs(dir)
	if err != nil {
		return Prepared{}, err
	}
	prepared := Prepared{Dir: absolute}
	info, err := os.Lstat(absolute)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		return Prepared{}, fmt.Errorf("Arbeitsverzeichnis %s ist ein Symlink — gib das Ziel selbst an", absolute)
	case err == nil && !info.IsDir():
		return Prepared{}, fmt.Errorf("Arbeitsverzeichnis %s ist kein Verzeichnis", absolute)
	case err == nil:
		if info.Mode().Perm()&0o077 != 0 {
			prepared.Warnings = append(prepared.Warnings, fmt.Sprintf(
				"Das Arbeitsverzeichnis ist für andere Benutzer zugänglich (Rechte %04o); die Rohdaten eines Reviews können Secrets enthalten.", info.Mode().Perm()))
		}
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(absolute, dirPerm); err != nil {
			return Prepared{}, err
		}
		if err := os.Chmod(absolute, dirPerm); err != nil {
			return Prepared{}, err
		}
		prepared.Created = true
	default:
		return Prepared{}, err
	}
	prepared.InsideProject = inside(projectDir, absolute)
	if prepared.InsideProject {
		prepared.Warnings = append(prepared.Warnings, insideWarning)
	}
	return prepared, nil
}

const insideWarning = "Das Arbeitsverzeichnis liegt im Projekt: Scanner späterer Läufe lesen es mit, solange es dort liegt."

func inside(projectDir string, dir string) bool {
	if strings.TrimSpace(projectDir) == "" {
		return false
	}
	root, err := filepath.Abs(projectDir)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

// CatalogCheck liefert die Prüfung der Quellen gegen den Review-Katalog des
// Projekts: Eine Quelle <eintrag>.sarif gehört zum Rezept mit dem Schlüssel
// <eintrag>; es muss im Katalog stehen, darf nicht abgeschaltet sein und muss
// einen gültigen Evidence-Vertrag tragen (audit.mode: evidence mit
// audit.ruleIds und audit.scope.paths). Geprüft wird dann wie beim Melden im
// Audit-Lauf mit review.CheckEvidenceSARIF. Der Katalog wird einmal gelesen.
func CatalogCheck(projectDir string) (func(entry string, data []byte) (review.EvidenceReport, error), error) {
	built, err := project.BuildContext(projectDir)
	if err != nil {
		return nil, err
	}
	if built.CatalogsProblem != "" {
		return nil, fmt.Errorf("Review-Katalog ist nicht lesbar: %s", built.CatalogsProblem)
	}
	entries := built.Catalogs["reviews"]
	return func(entry string, data []byte) (review.EvidenceReport, error) {
		contract, err := contractFor(entries, entry)
		if err != nil {
			return review.EvidenceReport{}, err
		}
		var paths []string
		if contract.Scope != nil {
			paths = contract.Scope.Paths
		}
		return review.CheckEvidenceSARIF(data, entry, contract.RuleIDs, paths)
	}, nil
}

func contractFor(entries []project.CatalogEntry, key string) (review.AuditContract, error) {
	for _, entry := range entries {
		if entry.Key != key {
			continue
		}
		if entry.Disabled {
			return review.AuditContract{}, fmt.Errorf("das Rezept %s ist im Projekt abgeschaltet", entry.Name)
		}
		if entry.Audit == nil {
			return review.AuditContract{}, fmt.Errorf("das Rezept %s trägt keinen audit-Block", entry.Name)
		}
		contract := review.AuditContract{
			Mode:              review.Mode(entry.Audit.Mode),
			RuleIDs:           append([]string{}, entry.Audit.RuleIDs...),
			ResultRequiredSet: entry.Audit.ResultRequired != nil,
			DefaultResult:     entry.Audit.DefaultResult,
		}
		if entry.Audit.Scope != nil {
			contract.Scope = &review.Scope{Tools: append([]string{}, entry.Audit.Scope.Tools...), Paths: append([]string{}, entry.Audit.Scope.Paths...)}
		}
		if review.NormalizeMode(contract.Mode) != review.ModeEvidence {
			return review.AuditContract{}, fmt.Errorf("das Rezept %s erhebt keine Fundliste (audit.mode ist nicht %s)", entry.Name, review.ModeEvidence)
		}
		if err := review.ValidateAuditContract(contract); err != nil {
			return review.AuditContract{}, fmt.Errorf("das Rezept %s erfüllt den Evidence-Vertrag nicht: %w", entry.Name, err)
		}
		return contract, nil
	}
	return review.AuditContract{}, fmt.Errorf("kein Rezept mit dem Schlüssel %s im Review-Katalog", key)
}

// MergeOutput ist das Ergebnis von Merge, gleich für CLI und MCP.
type MergeOutput struct {
	// Dir ist das Arbeitsverzeichnis.
	Dir string `json:"dir"`
	// CheckOnly meldet, dass nur geprüft und nichts geschrieben wurde.
	CheckOnly bool `json:"checkOnly,omitempty"`
	// Sources sind die geprüften Quellen mit ihren Fundzahlen.
	Sources []merge.DirSource `json:"sources"`
	// Outputs nennt die geschriebenen Dateien; fehlt bei CheckOnly.
	Outputs map[string]string `json:"outputs,omitempty"`
	// Findings ist die Zahl der Funde.
	Findings int `json:"findings"`
	// Groups ist die Zahl der Gruppen; fehlt bei CheckOnly.
	Groups *int `json:"groups,omitempty"`
	// Warnings sind die Hinweise des Belegs, etwa dass keine bekannten
	// Entscheidungen abgeglichen wurden.
	Warnings []string `json:"warnings,omitempty"`
}

// Merge prüft die Fundlisten eines Arbeitsverzeichnisses gegen den
// Review-Katalog des Projekts und fasst sie zusammen (merge.MergeDir); mit
// checkOnly nur die Prüfung (merge.CheckDir), ohne zu schreiben.
func Merge(projectDir string, dir string, checkOnly bool, version string, severityMappingPath string) (MergeOutput, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return MergeOutput{}, err
	}
	checker, err := CatalogCheck(projectDir)
	if err != nil {
		return MergeOutput{}, err
	}
	options := merge.DirOptions{
		ProjectDir:          projectDir,
		Dir:                 absolute,
		KPlaybookVersion:    version,
		SeverityMappingPath: severityMappingPath,
		Check:               checker,
	}
	if checkOnly {
		sources, err := merge.CheckDir(options)
		if err != nil {
			return MergeOutput{}, err
		}
		output := MergeOutput{Dir: absolute, CheckOnly: true, Sources: sources}
		for _, source := range sources {
			output.Findings += source.Findings
		}
		return output, nil
	}
	result, sources, written, err := merge.MergeDir(options)
	if err != nil {
		return MergeOutput{}, err
	}
	groups := len(result.Groups)
	return MergeOutput{
		Dir:      absolute,
		Sources:  sources,
		Outputs:  map[string]string{"reviewInputJSON": written.JSON, "reviewInputMarkdown": written.Markdown},
		Findings: len(result.Findings),
		Groups:   &groups,
		Warnings: result.KnownDecisions.Warnings,
	}, nil
}
