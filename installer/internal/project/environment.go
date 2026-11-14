package project

import (
	"os"
	"path/filepath"
)

// Environment beschreibt, worauf das Werkzeug gerade arbeitet.
//
// Es gibt keine Fallunterscheidung zwischen Entwicklungsrepo und Zielprojekt:
// beide tragen ihre eigene K-PLAYBOOK.yaml und werden gleich behandelt. Der
// einzige Unterschied, den es gibt, ist installiert oder nicht.
type Environment struct {
	// Installed meldet, ob eine K-PLAYBOOK.yaml gefunden wurde.
	Installed bool `json:"installed"`
	// ProjectDir ist das Hauptverzeichnis mit der Config. Leer, wenn nichts
	// gefunden wurde.
	ProjectDir string `json:"projectDir"`
	// SearchedFrom ist das Verzeichnis, ab dem gesucht wurde.
	SearchedFrom string `json:"searchedFrom"`
}

// Detect sucht das Projekt — die K-PLAYBOOK.yaml — ab dem Arbeitsverzeichnis
// aufwärts.
func Detect() Environment {
	start, err := os.Getwd()
	if err != nil {
		return Environment{}
	}
	return DetectFrom(start)
}

// DetectFrom sucht ab einem bestimmten Verzeichnis und ist dadurch prüfbar.
func DetectFrom(startDir string) Environment {
	environment := Environment{SearchedFrom: startDir}

	projectDir, err := Discover(startDir)
	if err != nil {
		return environment
	}

	environment.Installed = true
	environment.ProjectDir = projectDir
	return environment
}

// DisplayPath kürzt das Home-Verzeichnis zu ~, damit Pfade in der Oberfläche
// lesbar bleiben.
func DisplayPath(dir string) string {
	if dir == "" {
		return ""
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return dir
	}
	if dir == home {
		return "~"
	}
	if len(dir) > len(home) && dir[:len(home)] == home && dir[len(home)] == filepath.Separator {
		return "~" + dir[len(home):]
	}
	return dir
}
