package project

import (
	"fmt"
	"os"
	"strings"
)

// ResetConfig sichert eine Konfiguration mit alter oder fehlender
// schema_version weg und legt eine frische an — das Neueinrichten.
//
// Bewusst kein `migrate`: die Felder einer älteren Fassung werden nicht
// umgerechnet. Zurückgesetzt wird auf den Zustand nach dem ersten Anlegen —
// mit der alten Datei daneben, damit die eigenen Werte nachlesbar bleiben.
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
		return ResetResult{}, fmt.Errorf("%s trägt schema_version %s; zurückgesetzt wird nur "+
			"eine ältere oder fehlende Fassung",
			ConfigFileName, config.SchemaVersion)
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
