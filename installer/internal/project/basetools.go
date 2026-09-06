package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BaseToolMatrixPath ist die Matrix der Basis-Werkzeuge, relativ zur
// Installation. Sie ist bewusst nicht die Security-Matrix: scanners.tsv
// referenziert jene über die Spalte tool, und jeder Eintrag dort wird zu einem
// Lauf-Zustand in jedem Review.
const BaseToolMatrixPath = "scripts/base-tools.tsv"

// BaseToolScriptPath ist der Installer der Basis-Werkzeuge, relativ zur
// Installation. Aufgerufen wird er von hier aus nie — er steht nur als Befehl
// in der Antwort.
const BaseToolScriptPath = "scripts/install-base-tools.sh"

// BaseTool ist ein fehlendes Basis-Werkzeug.
//
// Methods ist die Methodenspalte aus der Matrix, unverändert durchgereicht:
// eine Komma-Liste aus apt, github und none. Die Rangfolge, die das Skript
// daraus ableitet, wird hier nicht nachgebaut — sie läge sonst zweimal vor und
// liefe nach der ersten einseitigen Änderung auseinander.
type BaseTool struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Methods string `json:"methods"`
}

// BaseTools ist der Host-Befund zu den Werkzeugen, die k-playbook selbst
// aufruft.
//
// Gemessen wird reine Anwesenheit im PATH über exec.LookPath: kein Unterprozess
// je Werkzeug, kein --version, keine Shell. Das unterscheidet den Befund vom
// Security-Preflight, der je Tool ein --version startet und deshalb nicht im
// Kontext steht.
//
// Bekannter Fehlalarm: Eine Shell-Funktion oder ein Alias steht nicht im PATH
// und wird deshalb nicht gesehen. Claude Code setzt eine Shell-Funktion `rg`
// auf das im eigenen Binary mitgelieferte ripgrep; dort meldet der Befund `rg`
// dauerhaft als fehlend, obwohl der Aufruf funktioniert. Das ist bekannt und
// wird hingenommen: In genau den Umgebungen, für die dieser Befund existiert —
// OpenCode, Cursor, ein normales Terminal —, ist er richtig, und dort fällt der
// Command sonst wortlos um. Die Alternative wäre, je Werkzeug eine Shell zu
// starten; das kostet den Unterprozess, den der Kontext gerade nicht ausgeben
// soll, und machte den Befund von der Shell des Aufrufers abhängig. Ein falsch
// gemeldeter Hinweis, der zu einer Installation rät, die ohnehin nicht schadet,
// wiegt leichter als ein stiller Fehlschlag.
type BaseTools struct {
	// Matrix ist der Ort der Werkzeugliste — auch wenn sie fehlt, damit klar
	// ist, wo sie hingehört.
	Matrix string `json:"matrix"`
	// Present sagt, ob die Matrix gelesen werden konnte. Fehlt sie, ist das
	// kein Abbruch: der Kontext steht am Anfang jedes Commands und bleibt
	// nutzbar. Der Zustand wird gemeldet statt verschwiegen.
	Present bool `json:"present"`
	// Error ist gesetzt, wenn die Matrix da, aber nicht lesbar ist.
	Error string `json:"error,omitempty"`
	// Missing sind die Werkzeuge, die im PATH fehlen. Von einer Gruppe steht
	// hier höchstens ein Eintrag: von curl und wget genügt eines.
	Missing []BaseTool `json:"missing"`
	// OK meldet, dass nichts fehlt. Ohne gelesene Matrix ist es false, und
	// Missing bleibt leer — dann sagt ein Command nichts, statt zu raten.
	OK bool `json:"ok"`
	// InstallCommand ist ein einzelner Aufruf des Skripts. Welchen Weg es je
	// Eintrag geht, entscheidet das Skript; hier steht kein zweiter Entscheider.
	InstallCommand string `json:"installCommand"`
}

// BaseToolMatrix ist der Ort der Matrix in einer Installation.
func BaseToolMatrix(projectDir string) string {
	return filepath.Join(PlaybookDir(projectDir), filepath.FromSlash(BaseToolMatrixPath))
}

// BaseToolScript ist der Ort des Installers in einer Installation.
func BaseToolScript(projectDir string) string {
	return filepath.Join(PlaybookDir(projectDir), filepath.FromSlash(BaseToolScriptPath))
}

// baseToolEntry ist eine Zeile der Matrix, soweit der Befund sie braucht.
type baseToolEntry struct {
	name    string
	group   string
	role    string
	methods string
}

// DetectBaseTools liest die Matrix und prüft je Eintrag den PATH.
//
// Im Muster von DetectGH(): ein billiger Host-Befund ohne Unterprozess, der
// deshalb in der Kontextausgabe stehen darf. Das Skript wird nicht aufgerufen —
// weder mit --json noch sonstwie.
func DetectBaseTools(projectDir string) BaseTools {
	state := BaseTools{
		Matrix:         BaseToolMatrix(projectDir),
		Missing:        []BaseTool{},
		InstallCommand: fmt.Sprintf("bash %q --install", BaseToolScript(projectDir)),
	}

	entries, err := readBaseToolMatrix(state.Matrix)
	if err != nil {
		if !os.IsNotExist(err) {
			state.Error = err.Error()
		} else {
			state.Error = fmt.Sprintf("%s fehlt in der Installation", BaseToolMatrixPath)
		}
		return state
	}
	state.Present = true

	// Eine Gruppe gilt als vorhanden, sobald eines ihrer Mitglieder da ist.
	// Genauso wertet das Skript sie aus; ohne das widersprächen sich Skript und
	// Kontextbefund auf demselben Host, und auf einem Rechner mit curl stünde
	// wget dauerhaft als fehlend da.
	groupPresent := map[string]bool{}
	for _, entry := range entries {
		if entry.group == "" || entry.group == "-" {
			continue
		}
		if lookPathOK(entry.name) {
			groupPresent[entry.group] = true
		}
	}

	reported := map[string]bool{}
	for _, entry := range entries {
		grouped := entry.group != "" && entry.group != "-"
		if grouped {
			if groupPresent[entry.group] || reported[entry.group] {
				continue
			}
			reported[entry.group] = true
		} else if lookPathOK(entry.name) {
			continue
		}
		state.Missing = append(state.Missing, BaseTool{
			Name:    entry.name,
			Role:    entry.role,
			Methods: entry.methods,
		})
	}

	state.OK = len(state.Missing) == 0
	return state
}

// lookPathOK ist die Messgröße: liegt das Programm im PATH. Kein Aufruf, keine
// Shell, kein --version.
func lookPathOK(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// readBaseToolMatrix liest name, group, role und methods. Die übrigen Spalten
// gehören dem Skript: Paketnamen, Repos und Asset-Muster braucht ein Befund
// nicht, der nichts installiert.
func readBaseToolMatrix(path string) ([]baseToolEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []baseToolEntry
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 || fields[0] == "name" {
			continue
		}
		entries = append(entries, baseToolEntry{
			name:    strings.TrimSpace(fields[0]),
			group:   strings.TrimSpace(fields[1]),
			role:    strings.TrimSpace(fields[2]),
			methods: strings.TrimSpace(fields[4]),
		})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%s enthält keine Einträge", path)
	}
	return entries, nil
}
