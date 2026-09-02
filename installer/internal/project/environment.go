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
	// PlaybookDir ist die Installation darin. Leer, wenn nichts gefunden wurde.
	PlaybookDir string `json:"playbookDir"`
	// PlaybookPresent meldet, ob die Installation tatsächlich vorliegt. Nach
	// einem frischen Clone des Projekts fehlt sie, die Config ist aber da.
	PlaybookPresent bool `json:"playbookPresent"`
	// SearchedFrom ist das Verzeichnis, ab dem gesucht wurde.
	SearchedFrom string `json:"searchedFrom"`
}

// Detect sucht die Installation ab dem Arbeitsverzeichnis aufwärts.
func Detect() Environment {
	start, err := os.Getwd()
	if err != nil {
		return Environment{}
	}
	if environment, ok := setupEnvironment(start); ok {
		return environment
	}
	return DetectFrom(start)
}

// setupEnvironment erkennt den ersten Start aus einer projektlokalen
// Installation. Ohne diesen Vorrang würde ein Anker eines übergeordneten
// Projekts die noch fehlende Konfiguration des Clones verdecken.
func setupEnvironment(startDir string) (Environment, bool) {
	installDir, ok := InstallDir()
	if !ok || filepath.Base(installDir) != PlaybookDirName {
		return Environment{}, false
	}

	projectDir := filepath.Dir(installDir)
	if fileExists(ConfigPath(projectDir)) || !isWithinResolved(startDir, projectDir) {
		return Environment{}, false
	}
	return Environment{
		ProjectDir:      projectDir,
		PlaybookDir:     installDir,
		PlaybookPresent: true,
		SearchedFrom:    startDir,
	}, true
}

func isWithinResolved(dir string, parent string) bool {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	parent, err = filepath.Abs(parent)
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	if resolved, err := filepath.EvalSymlinks(parent); err == nil {
		parent = resolved
	}
	relative, err := filepath.Rel(parent, dir)
	return err == nil && relative != ".." && !filepath.IsAbs(relative)
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
	environment.PlaybookDir = PlaybookDir(projectDir)
	environment.PlaybookPresent = isDir(environment.PlaybookDir)
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
