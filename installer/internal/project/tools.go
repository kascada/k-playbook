package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ToolScriptPath ist der Preflight, relativ zur Installation.
const ToolScriptPath = "scripts/install-security-tools.sh"

// ToolMatrixPath ist die kanonische Tool-Matrix, relativ zur Installation.
const ToolMatrixPath = "scripts/security-tools.tsv"

// toolPreflightTimeout begrenzt den Aufruf. Der Preflight ruft je Tool ein
// --version auf; hängt eines davon, darf die Oberfläche nicht mitwarten.
const toolPreflightTimeout = 30 * time.Second

// Tool ist der Zustand eines einzelnen Security-Tools.
type Tool struct {
	Name string `json:"name"`
	// Languages nennt die Projektsprachen, für die das Tool zuständig ist, oder
	// "*" für sprachunabhängig. Required berücksichtigt das bereits: ein
	// sprachgebundenes Tool ist nur Pflicht, wenn seine Sprache gefragt war.
	Languages     string `json:"languages"`
	Required      bool   `json:"required"`
	InstallMethod string `json:"installMethod"`
	Status        string `json:"status"`
	Version       string `json:"version"`
	Path          string `json:"path"`
	Role          string `json:"role"`
	DockerImage   string `json:"dockerImage"`
}

// ToolPreflight ist die Antwort des Skripts.
type ToolPreflight struct {
	PlaybookDir string `json:"playbookDir"`
	ToolMatrix  string `json:"toolMatrix"`
	BinDir      string `json:"binDir"`
	VenvRoot    string `json:"venvRoot"`
	// ToolScope benennt, ob der Status den Host-/User-PATH oder ein aktives
	// Projekt-venv gemessen hat.
	ToolScope        string `json:"toolScope"`
	ToolScopePath    string `json:"toolScopePath"`
	ToolScopeMessage string `json:"toolScopeMessage"`
	// Languages ist die Sprachliste, mit der der Preflight gerechnet hat. Leer
	// heißt: keine übergeben, also gilt nur Sprachunabhängiges als Pflicht.
	Languages       string `json:"languages"`
	MissingRequired int    `json:"missingRequired"`
	// MissingOptional zählt die optionalen Tools, die für die gewählten
	// Sprachen zuständig sind und fehlen. Sie blockieren nichts, sollen aber
	// nicht unerwähnt bleiben — sonst steht "fehlt" in der Liste und
	// "Vollständig" darüber.
	MissingOptional int `json:"missingOptional"`
	// InstallCommand holt die fehlende Pflicht, InstallCommandOptional zusätzlich
	// die optionalen. Die Venv-Varianten erzwingen dedizierte k-playbook-Tool-venvs.
	// Alle kommen fertig aus dem Skript, samt Sprachauswahl — zusammengesetzt
	// würden sie sonst an zwei Orten gepflegt.
	InstallCommand             string `json:"installCommand"`
	InstallCommandVenv         string `json:"installCommandVenv"`
	InstallCommandOptional     string `json:"installCommandOptional"`
	InstallCommandOptionalVenv string `json:"installCommandOptionalVenv"`
	Tools                      []Tool `json:"tools"`
}

// ToolScript ist der Ort des Preflight-Skripts in einer Installation.
func ToolScript(projectDir string) string {
	return filepath.Join(PlaybookDir(projectDir), filepath.FromSlash(ToolScriptPath))
}

// ToolMatrix ist der Ort der Tool-Matrix in einer Installation.
func ToolMatrix(projectDir string) string {
	return filepath.Join(PlaybookDir(projectDir), filepath.FromSlash(ToolMatrixPath))
}

// ReadToolLanguages liest nur die Sprachliste aus der Tool-Matrix. Das braucht
// die Oberfläche auch dann, wenn der eigentliche Preflight wegen eines aktiven
// Projekt-venv abbricht.
func ReadToolLanguages(projectDir string) ([]string, error) {
	data, err := os.ReadFile(ToolMatrix(projectDir))
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[0] == "name" {
			continue
		}
		for _, language := range strings.Split(fields[1], ",") {
			language = strings.TrimSpace(language)
			if language == "" || language == "*" {
				continue
			}
			seen[language] = true
		}
	}

	languages := make([]string, 0, len(seen))
	for language := range seen {
		languages = append(languages, language)
	}
	sort.Strings(languages)
	return languages, nil
}

// describePreflightError übersetzt die Skriptmeldung, wo sie für sich genommen
// in die Irre führt.
//
// Binary und Skript können auseinanderlaufen: aufgerufen wird das Skript der
// Installation des aktuellen Projekts, das Binary kann aber aus der host-weiten
// Kopie stammen, die ein anderes Projekt gespiegelt hat. Ist die Installation
// älter, kennt ihr Skript ein neueres Argument nicht — und "Unknown argument"
// sagt niemandem, dass ein Update fehlt.
func describePreflightError(message string, playbookDir string) error {
	if strings.Contains(message, "Unknown argument:") {
		return fmt.Errorf("die Installation unter %s ist älter als dieses Werkzeug: ihr Preflight-Skript meldet %q. Ziehe sie nach — in der Oberfläche über \"Update prüfen\", sonst mit git pull in diesem Verzeichnis",
			DisplayPath(playbookDir), message)
	}
	return fmt.Errorf("%s", message)
}

// CheckTools ruft den Preflight auf und liefert den Zustand.
//
// Die Sprachen kommen als Parameter statt aus der Konfiguration gelesen zu
// werden: die Oberfläche zeigt beim Umschalten sofort, was eine Auswahl
// bedeutet, noch bevor sie geschrieben ist. Eine leere Liste heißt "keine
// Angabe" — dann gilt nur Sprachunabhängiges als Pflicht.
//
// Das Skript wird ausschließlich lesend aufgerufen: --json prüft nur, ob die
// Binaries vorhanden sind, und installiert nichts. Installiert wird bewusst im
// Terminal, weil das den Host verändert und nicht das Projekt.
func CheckTools(projectDir string, languages []string) (ToolPreflight, error) {
	script := ToolScript(projectDir)
	if !fileExists(script) {
		return ToolPreflight{}, fmt.Errorf("%s fehlt in der Installation", ToolScriptPath)
	}

	args := []string{script, "--json"}
	if joined := strings.Join(languages, ","); joined != "" {
		args = append(args, "--languages", joined)
	}

	ctx, cancel := context.WithTimeout(context.Background(), toolPreflightTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", args...)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ToolPreflight{}, fmt.Errorf("Preflight hat nicht innerhalb von %s geantwortet", toolPreflightTimeout)
		}
		// Das Skript bricht mit einer Meldung auf stderr ab, etwa bei einem
		// aktiven Projekt-venv. Die gehört in die Fehlermeldung.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			message, _, _ := strings.Cut(strings.TrimSpace(string(exitErr.Stderr)), "\n")
			return ToolPreflight{}, describePreflightError(message, PlaybookDir(projectDir))
		}
		return ToolPreflight{}, fmt.Errorf("Preflight fehlgeschlagen: %w", err)
	}

	var preflight ToolPreflight
	if err := json.Unmarshal(output, &preflight); err != nil {
		return ToolPreflight{}, fmt.Errorf("Preflight-Ausgabe nicht lesbar: %w", err)
	}
	return preflight, nil
}
