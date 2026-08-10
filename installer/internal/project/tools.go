package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ToolScriptPath ist der Preflight, relativ zur Installation.
const ToolScriptPath = "scripts/install-security-tools.sh"

// toolPreflightTimeout begrenzt den Aufruf. Der Preflight ruft je Tool ein
// --version auf; haengt eines davon, darf die Oberflaeche nicht mitwarten.
const toolPreflightTimeout = 30 * time.Second

// Tool ist der Zustand eines einzelnen Security-Tools.
type Tool struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Status      string `json:"status"`
	Version     string `json:"version"`
	Path        string `json:"path"`
	Role        string `json:"role"`
	DockerImage string `json:"dockerImage"`
}

// ToolPreflight ist die Antwort des Skripts.
type ToolPreflight struct {
	PlaybookDir     string `json:"playbookDir"`
	ToolMatrix      string `json:"toolMatrix"`
	BinDir          string `json:"binDir"`
	VenvDir         string `json:"venvDir"`
	MissingRequired int    `json:"missingRequired"`
	InstallCommand  string `json:"installCommand"`
	Tools           []Tool `json:"tools"`
}

// ToolScript ist der Ort des Preflight-Skripts in einer Installation.
func ToolScript(projectDir string) string {
	return filepath.Join(PlaybookDir(projectDir), filepath.FromSlash(ToolScriptPath))
}

// CheckTools ruft den Preflight auf und liefert den Zustand.
//
// Das Skript wird ausschliesslich lesend aufgerufen: --json prueft nur, ob die
// Binaries vorhanden sind, und installiert nichts. Installiert wird bewusst im
// Terminal, weil das den Host veraendert und nicht das Projekt.
func CheckTools(projectDir string) (ToolPreflight, error) {
	script := ToolScript(projectDir)
	if !fileExists(script) {
		return ToolPreflight{}, fmt.Errorf("%s fehlt in der Installation", ToolScriptPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), toolPreflightTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", script, "--json")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ToolPreflight{}, fmt.Errorf("Preflight hat nicht innerhalb von %s geantwortet", toolPreflightTimeout)
		}
		// Das Skript bricht mit einer Meldung auf stderr ab, etwa bei einem
		// aktiven Projekt-venv. Die gehoert in die Fehlermeldung.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			message, _, _ := strings.Cut(strings.TrimSpace(string(exitErr.Stderr)), "\n")
			return ToolPreflight{}, fmt.Errorf("%s", message)
		}
		return ToolPreflight{}, fmt.Errorf("Preflight fehlgeschlagen: %w", err)
	}

	var preflight ToolPreflight
	if err := json.Unmarshal(output, &preflight); err != nil {
		return ToolPreflight{}, fmt.Errorf("Preflight-Ausgabe nicht lesbar: %w", err)
	}
	return preflight, nil
}
