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
	Name string `json:"name"`
	// Languages nennt die Projektsprachen, fuer die das Tool zustaendig ist, oder
	// "*" fuer sprachunabhaengig. Required beruecksichtigt das bereits: ein
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
	// Languages ist die Sprachliste, mit der der Preflight gerechnet hat. Leer
	// heisst: keine uebergeben, also gilt nur Sprachunabhaengiges als Pflicht.
	Languages       string `json:"languages"`
	MissingRequired int    `json:"missingRequired"`
	InstallCommand  string `json:"installCommand"`
	Tools           []Tool `json:"tools"`
}

// ToolScript ist der Ort des Preflight-Skripts in einer Installation.
func ToolScript(projectDir string) string {
	return filepath.Join(PlaybookDir(projectDir), filepath.FromSlash(ToolScriptPath))
}

// describePreflightError uebersetzt die Skriptmeldung, wo sie fuer sich genommen
// in die Irre fuehrt.
//
// Binary und Skript koennen auseinanderlaufen: aufgerufen wird das Skript der
// Installation des aktuellen Projekts, das Binary kann aber aus der host-weiten
// Kopie stammen, die ein anderes Projekt gespiegelt hat. Ist die Installation
// aelter, kennt ihr Skript ein neueres Argument nicht — und "Unknown argument"
// sagt niemandem, dass ein Update fehlt.
func describePreflightError(message string, playbookDir string) error {
	if strings.Contains(message, "Unknown argument:") {
		return fmt.Errorf("die Installation unter %s ist aelter als dieses Werkzeug: ihr Preflight-Skript meldet %q. Ziehe sie nach — in der Oberflaeche ueber \"Update pruefen\", sonst mit git pull in diesem Verzeichnis",
			DisplayPath(playbookDir), message)
	}
	return fmt.Errorf("%s", message)
}

// CheckTools ruft den Preflight auf und liefert den Zustand.
//
// Die Sprachen kommen als Parameter statt aus der Konfiguration gelesen zu
// werden: die Oberflaeche zeigt beim Umschalten sofort, was eine Auswahl
// bedeutet, noch bevor sie geschrieben ist. Eine leere Liste heisst "keine
// Angabe" — dann gilt nur Sprachunabhaengiges als Pflicht.
//
// Das Skript wird ausschliesslich lesend aufgerufen: --json prueft nur, ob die
// Binaries vorhanden sind, und installiert nichts. Installiert wird bewusst im
// Terminal, weil das den Host veraendert und nicht das Projekt.
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
		// aktiven Projekt-venv. Die gehoert in die Fehlermeldung.
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
