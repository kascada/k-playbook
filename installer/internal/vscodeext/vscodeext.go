// Package vscodeext trägt die VS-Code-Erweiterung „k-playbook Workspace
// Tools“ im Binary und kennt die Wege, sie in die VS-Code-Umgebung dieses
// Rechners zu bringen.
//
// Die VSIX ist **eingecheckt** und wird hier eingebettet, nicht bei jedem
// Build erzeugt: vsce schreibt Bauzeit und mtimes in das Zip, und
// SHA256SUMS muss bitgleich nachbaubar bleiben. Neu gebaut wird sie mit
// `make vscode-vsix` — der einzige Schritt des Projekts, der Node braucht.
// Dass die eingecheckte Datei zur Quelle unter installer/vscode/ passt,
// prüft TestVSIXPasstZurQuelle.
package vscodeext

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
)

// ID ist die Kennung der Erweiterung: publisher.name aus
// installer/vscode/package.json. VS Code vergleicht sie ohne Rücksicht auf
// Groß- und Kleinschreibung.
const ID = "kascada.k-playbook-workspace-tools"

// Name ist der Anzeigename, wie ihn displayName in package.json führt.
const Name = "k-playbook Workspace Tools"

// vsixPath ist der Ort der VSIX im eingebetteten Verzeichnis.
const vsixPath = "vsix/k-playbook-workspace-tools.vsix"

//go:embed vsix
var embedded embed.FS

// VSIX liefert die eingebettete VSIX; ok ist false, wenn das Binary ohne sie
// gebaut ist — dann ist nichts zu installieren, und jeder Weg sagt das
// deutlich statt eine leere Datei an die CLI zu geben.
func VSIX() (data []byte, ok bool) {
	return fromFS(embedded)
}

// fromFS liest die VSIX aus fsys; eine leere Datei zählt als fehlend.
func fromFS(fsys fs.FS) ([]byte, bool) {
	data, err := fs.ReadFile(fsys, vsixPath)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return data, true
}

// maxManifest begrenzt, wie viel von extension/package.json gelesen wird.
const maxManifest = 1 << 20

// Version liest die Version aus extension/package.json einer VSIX. Sie ist
// die eigene Version der Erweiterung aus installer/vscode/package.json und
// nicht die VERSION des Programms: die VSIX wird vor dem Tag eingecheckt und
// kennt die künftige Nummer nicht.
func Version(vsix []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(vsix), int64(len(vsix)))
	if err != nil {
		return "", fmt.Errorf("keine gültige VSIX: %w", err)
	}
	f, err := zr.Open("extension/package.json")
	if err != nil {
		return "", errors.New("keine gültige VSIX: extension/package.json fehlt")
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxManifest))
	if err != nil {
		return "", fmt.Errorf("keine gültige VSIX: %w", err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version == "" {
		return "", errors.New("keine gültige VSIX: extension/package.json ohne version")
	}
	return manifest.Version, nil
}

// EmbeddedVersion ist die Version der eingebetteten VSIX, eine Zeile für
// Ausgaben. Ohne eingebettete VSIX ist ok false; ein Lesefehler kommt als
// err zurück, damit ihn niemand für „nicht vorhanden“ nimmt.
func EmbeddedVersion() (version string, ok bool, err error) {
	data, ok := VSIX()
	if !ok {
		return "", false, nil
	}
	version, err = Version(data)
	if err != nil {
		return "", true, err
	}
	return version, true, nil
}
