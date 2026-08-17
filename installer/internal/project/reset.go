package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// ResetConfig sichert eine Konfiguration aus einem abgelösten Modell weg und
// legt eine frische an.
//
// Bewusst kein `migrate`: die Modelle beschreiben verschiedene Verzeichnis-
// Aufteilungen, und die Felder ineinander umzurechnen hieße, eine Übersetzung
// zu pflegen, die mit jedem Modell wächst. Zurückgesetzt wird stattdessen auf
// den Zustand nach dem ersten Anlegen — mit der alten Datei daneben, damit die
// eigenen Werte nachlesbar bleiben.
//
// Bewusst kein Löschen: `remediation`, `tools` und `project.repo_root` stehen
// nur dort und wären sonst weg.
type ResetResult struct {
	// BackupPath ist die weggesicherte alte Datei.
	BackupPath string `json:"backupPath"`
	// ConfigPath ist die neu angelegte Konfiguration.
	ConfigPath string `json:"configPath"`
	// PreviousVersion ist die schema_version, die zurückgesetzt wurde. Leer,
	// wenn die Datei keine trug.
	PreviousVersion string `json:"previousVersion"`
}

// LegacyContentError meldet Projektinhalte, die im Installationsverzeichnis
// liegen.
//
// Unter Modell 1 lagen Tasks, Checks, Reviews und die TODO.md in
// `k-playbook/` — genau dem Verzeichnis, das unter dem heutigen Modell der
// ersetzbare Clone ist. Wird nur die Konfiguration erneuert, sieht das Projekt
// gesund aus, und das nächste Update löscht die Inhalte. Deshalb bricht das
// Zurücksetzen hier ab, statt eine stille Falle zu hinterlassen.
type LegacyContentError struct {
	// Paths sind die gefundenen Inhalte, relativ zum Hauptverzeichnis.
	Paths []string
}

func (e *LegacyContentError) Error() string {
	return fmt.Sprintf("in %s/ liegen Projektinhalte aus dem abgelösten Modell (%s); "+
		"sie gehören nach %s/ und müssen zuerst dorthin umziehen — %s/ wird beim nächsten "+
		"Update ersetzt",
		PlaybookDirName, strings.Join(e.Paths, ", "), LocalDirName, PlaybookDirName)
}

// ResetConfig führt das Zurücksetzen aus. Geschrieben wird erst, wenn nichts
// mehr im Weg steht.
func ResetConfig(projectDir string, repoRoot string) (ResetResult, error) {
	if strings.TrimSpace(projectDir) == "" {
		return ResetResult{}, fmt.Errorf("kein Hauptverzeichnis angegeben")
	}

	path := ConfigPath(projectDir)
	if !fileExists(path) {
		return ResetResult{}, fmt.Errorf("%s existiert nicht", ConfigFileName)
	}

	config, err := ReadConfig(projectDir)
	if err != nil {
		return ResetResult{}, err
	}
	state := SchemaState(config)
	if !state.Resettable() {
		return ResetResult{}, fmt.Errorf("%s trägt schema_version %s; zurückgesetzt wird nur, "+
			"was zu einem abgelösten Modell gehört",
			ConfigFileName, config.SchemaVersion)
	}

	if paths := LegacyContent(projectDir); len(paths) > 0 {
		return ResetResult{}, &LegacyContentError{Paths: paths}
	}

	backup, err := backupPath(path, config.SchemaVersion)
	if err != nil {
		return ResetResult{}, err
	}
	if err := os.Rename(path, backup); err != nil {
		return ResetResult{}, fmt.Errorf("%s ließ sich nicht wegsichern: %w", ConfigFileName, err)
	}

	if err := CreateConfig(projectDir, repoRoot); err != nil {
		// Ohne das Zurückbenennen stünde das Projekt ganz ohne Konfiguration da,
		// und die Oberfläche böte das Anlegen an, als wäre nie eine dagewesen.
		if restoreErr := os.Rename(backup, path); restoreErr != nil {
			return ResetResult{}, fmt.Errorf("%w; die alte Datei liegt unter %s", err, backup)
		}
		return ResetResult{}, err
	}

	return ResetResult{
		BackupPath:      backup,
		ConfigPath:      path,
		PreviousVersion: config.SchemaVersion,
	}, nil
}

// backupPath sucht einen freien Namen neben der Konfiguration. Eine vorhandene
// Sicherung wird nie überschrieben: sie kann aus einem früheren Versuch stammen
// und die einzigen Werte enthalten, die es noch gibt.
func backupPath(configPath string, version string) (string, error) {
	suffix := ".alt"
	if version != "" {
		suffix = ".v" + version + "-alt"
	}

	candidate := configPath + suffix
	for attempt := 2; pathExists(candidate); attempt++ {
		if attempt > 50 {
			return "", fmt.Errorf("kein freier Name für die Sicherung neben %s", ConfigFileName)
		}
		candidate = fmt.Sprintf("%s%s-%d", configPath, suffix, attempt)
	}
	return candidate, nil
}

// LegacyContent sucht Projektinhalte im Installationsverzeichnis.
//
// Zwei Wege, weil keiner allein reicht: die alte Datei nennt ihre Orte in
// `paths.*` und ist damit die genaue Auskunft — aber nur, solange sie
// vollständig ist. Der Zustand des Verzeichnisses selbst fängt den Rest ab.
// Gemeldet wird die Vereinigung; gelöscht oder verschoben wird nichts.
func LegacyContent(projectDir string) []string {
	dir := PlaybookDir(projectDir)
	if !isDir(dir) {
		return nil
	}

	found := legacyPathsFromConfig(projectDir)
	for _, name := range legacyPathsFromDir(dir) {
		if !slices.Contains(found, name) {
			found = append(found, name)
		}
	}

	sort.Strings(found)
	if len(found) > maxReportedPaths {
		found = found[:maxReportedPaths]
	}
	return found
}

// legacyPathsFromConfig liest den paths-Block der alten Datei und behält, was
// unterhalb der Installation liegt und wirklich existiert.
//
// Zeigt ein Eintrag woandershin, ist der Inhalt nicht gefährdet: ersetzt wird
// nur das Installationsverzeichnis.
func legacyPathsFromConfig(projectDir string) []string {
	data, err := os.ReadFile(ConfigPath(projectDir))
	if err != nil {
		return nil
	}

	var paths []string
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		indented := strings.HasPrefix(key, " ") || strings.HasPrefix(key, "\t")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if !indented {
			section = key
			continue
		}
		// `paths.playbook` ist das Verzeichnis selbst und kein Inhalt darin.
		if section != "paths" || key == "playbook" || value == "" {
			continue
		}

		relative, ok := insidePlaybookDir(value)
		if !ok || !pathExists(filepath.Join(projectDir, relative)) {
			continue
		}
		if !slices.Contains(paths, relative) {
			paths = append(paths, relative)
		}
	}
	return paths
}

// insidePlaybookDir prüft, ob ein Pfad aus der alten Konfiguration in der
// Installation liegt, und liefert ihn relativ zum Hauptverzeichnis.
func insidePlaybookDir(value string) (string, bool) {
	value = strings.Trim(value, `"'`)
	if value == "" || filepath.IsAbs(value) {
		return "", false
	}

	cleaned := filepath.Clean(value)
	if cleaned == PlaybookDirName {
		return "", false
	}
	if !strings.HasPrefix(cleaned, PlaybookDirName+string(filepath.Separator)) {
		return "", false
	}
	return cleaned, true
}

// legacyPathsFromDir liest den Zustand des Installationsverzeichnisses.
//
// Unter Modell 1 war es kein Clone, sondern ein reines Projektverzeichnis —
// dann gehört alles darin dem Projekt. Ist es ein Clone, gehört ihm alles
// Verfolgte; projekteigen ist genau das Untracked. Verfolgte Dateien, die
// lokal geändert wurden, bleiben außen vor: die meldet der Update-Block, und
// sie sind kein Grund, das Zurücksetzen zu blockieren.
func legacyPathsFromDir(dir string) []string {
	if isDir(filepath.Join(dir, ".git")) {
		return untrackedInPlaybookDir(dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var paths []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		paths = append(paths, filepath.Join(PlaybookDirName, name))
	}
	return paths
}

func untrackedInPlaybookDir(dir string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), cleanlinessTimeout)
	defer cancel()

	output, err := GitOutput(ctx, dir, "status", "--porcelain")
	if err != nil {
		return nil
	}

	var paths []string
	for _, line := range strings.Split(output, "\n") {
		code, path, found := strings.Cut(strings.TrimSpace(line), " ")
		if !found || code != "??" {
			continue
		}
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		// Git meldet ein unverfolgtes Verzeichnis mit Schrägstrich am Ende; für
		// die Anzeige ist der Name genug.
		paths = append(paths, filepath.Join(PlaybookDirName, strings.TrimSuffix(path, "/")))
	}
	return paths
}
