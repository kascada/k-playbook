package vscodeext

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Die Erweiterungsverzeichnisse von VS Code unter dem Heimatverzeichnis: der
// Server einer Remote-Sitzung (WSL, SSH, Dev Container) und die lokale
// Installation. Insiders, Cursor und VSCodium sind bewusst nicht dabei.
const (
	serverExtensions = ".vscode-server/extensions"
	localExtensions  = ".vscode/extensions"
)

// extensionDirs ist die Reihenfolge, in der gesucht wird: der Server zuerst.
var extensionDirs = []string{serverExtensions, localExtensions}

// Installation ist die Erweiterung, wie sie in einem Verzeichnis liegt.
type Installation struct {
	// Dir ist das Erweiterungsverzeichnis, in dem sie gefunden wurde —
	// nicht das Verzeichnis der Erweiterung selbst.
	Dir string `json:"dir"`
	// Version ist die Fassung, die VS Code dort führt.
	Version string `json:"version"`
}

// Find sucht die Erweiterung in den Verzeichnissen unter home.
//
// Maßgeblich ist ausschließlich extensions.json, nie der Verzeichnisname:
// nach einem --uninstall-extension verschwindet der Eintrag sofort, das
// Verzeichnis <id>-<version>/ bleibt aber zunächst liegen (gemessen am
// 2026-10-08). Eine Erkennung über Verzeichnisnamen würde eine
// deinstallierte Erweiterung als vorhanden melden und den Nachzug
// stillschweigend ausfallen lassen.
func Find(home string) []Installation {
	var found []Installation
	for _, rel := range extensionDirs {
		dir := filepath.Join(home, filepath.FromSlash(rel))
		if version, ok := findIn(dir); ok {
			found = append(found, Installation{Dir: dir, Version: version})
		}
	}
	return found
}

// findIn liest extensions.json eines Erweiterungsverzeichnisses. Fehlt oder
// bricht die Datei, gilt die Erweiterung dort als nicht installiert — ein
// Rückfall auf Verzeichnisnamen wäre hier gerade der Fehler.
func findIn(dir string) (version string, ok bool) {
	data, err := os.ReadFile(filepath.Join(dir, "extensions.json"))
	if err != nil {
		return "", false
	}
	var entries []struct {
		Identifier struct {
			ID string `json:"id"`
		} `json:"identifier"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return "", false
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Identifier.ID, ID) {
			return entry.Version, true
		}
	}
	return "", false
}

// NeedsInstall sagt, ob die Erweiterung irgendwo fehlt oder irgendwo in einer
// anderen Fassung als der eingebetteten liegt. Verglichen wird auf Gleichheit,
// nicht auf „älter“ — auch ein Rückschritt auf ein älteres Programm soll die
// passende Fassung bekommen.
//
// „Irgendwo“ ist die richtige Frage nur dort, wo keine CLI zu finden ist: dann
// installiert der Benutzer von Hand über die VSIX, und das kann jedes der
// Verzeichnisse treffen. Wo eine CLI gewählt ist, fragt NeedsInstallIn — ein
// Nachzug beschreibt immer genau ein Verzeichnis.
func NeedsInstall(embedded string, found []Installation) bool {
	if embedded == "" {
		return false
	}
	if len(found) == 0 {
		return true
	}
	for _, installation := range found {
		if installation.Version != embedded {
			return true
		}
	}
	return false
}

// NeedsInstallIn sagt, ob in dir installiert werden muss: wenn dort keine
// Fassung liegt oder eine andere als die eingebettete.
//
// Die Frage muss sich auf dasselbe Verzeichnis beziehen, das der Nachzug
// beschreibt. Fragte sie „irgendwo“, bliebe sie nach einem erfolgreichen
// Nachzug wahr, solange im anderen Verzeichnis eine abweichende Fassung
// liegt: der Dienst installierte bei jedem Start erneut, schriebe jedes Mal
// „installiert“ ins Log, und Karte wie `vscode status` sagten dauerhaft
// „Nachzuziehen“. Was im anderen Verzeichnis liegt, verschwindet damit nicht
// aus der Anzeige — dafür ist Divergent da.
func NeedsInstallIn(embedded string, dir string, found []Installation) bool {
	if embedded == "" {
		return false
	}
	for _, installation := range found {
		if installation.Dir == dir {
			return installation.Version != embedded
		}
	}
	return true
}

// Divergent sind die Installationen außerhalb von dir, deren Fassung von der
// eingebetteten abweicht. Sie bleiben liegen, weil ein Nachzug über die
// gewählte CLI nur dir beschreibt.
//
// Weggerechnet wird dieser Zustand nicht: er gehört sichtbar in die Anzeige,
// damit niemand eine zweite, alte Fassung für aktuell hält. Nachzuziehen ist
// er nur über die CLI, die das andere Verzeichnis bedient, oder von Hand über
// die VSIX.
func Divergent(embedded string, dir string, found []Installation) []Installation {
	if embedded == "" {
		return nil
	}
	var other []Installation
	for _, installation := range found {
		if installation.Dir != dir && installation.Version != embedded {
			other = append(other, installation)
		}
	}
	return other
}
