package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/vscodeext"
)

// vscodeUsage ist die Hilfe des Unterkommandos.
const vscodeUsage = `k-playbook vscode

Die VS-Code-Erweiterung „k-playbook Workspace Tools“ steckt in diesem
Programm. Beim Start der Oberfläche wird sie selbsttätig installiert und
nachgezogen; dieses Unterkommando ist der ausdrückliche Weg für den, der die
Oberfläche nie startet, und die Auskunft darüber, was installiert ist.

Unterkommandos:
  install          Installiert oder zieht nach:
                   <cli> --install-extension <datei> --force. Gewählt wird
                   die Remote-CLI eines laufenden VS-Code-Terminals, sonst
                   code-server des Servers, sonst code aus dem PATH. Als
                   eigener Benutzer aufrufen, nicht über sudo.
  status           Nennt die eingebettete Fassung, die installierten
                   Fassungen aus extensions.json und die CLI, die ein
                   Versuch jetzt nehmen würde.
  vsix -o <datei>  Schreibt nur die VSIX, etwa für „Extensions: Install from
                   VSIX…“ in einer Umgebung ohne CLI.
  collections [--json]
                   Nennt die Kephalaion-Collections, die ein VS-Code-Fenster
                   dieses Projekts eingebunden haben soll: die öffentliche
                   Collection k-playbook und die des Projekts, beide auf dem
                   Hub aus k-playbook-local/K-PLAYBOOK.local.yaml. Die
                   Erweiterung fragt das beim Start ab. Ohne Anbindung kein
                   Fehler, sondern eine Auskunft.
`

// runVSCode führt das Unterkommando `vscode` aus.
func runVSCode(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stdout, vscodeUsage)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(os.Stdout, vscodeUsage)
		return nil
	case "install":
		return runVSCodeInstall(args[1:], os.Stdout)
	case "status":
		return runVSCodeStatus(args[1:], os.Stdout)
	case "vsix":
		return runVSCodeVSIX(args[1:], os.Stdout)
	case "collections":
		return runVSCodeCollections(args[1:], os.Stdout)
	default:
		fmt.Fprint(os.Stderr, vscodeUsage)
		return fmt.Errorf("unbekanntes Kommando: vscode %s", args[0])
	}
}

func noArgs(name string, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("vscode %s erwartet keine Argumente: %s", name, args[0])
	}
	return nil
}

func runVSCodeInstall(args []string, out io.Writer) error {
	if err := noArgs("install", args); err != nil {
		return err
	}

	data, ok := vscodeext.VSIX()
	if !ok {
		return errors.New(vscodeext.NoVSIX)
	}
	version, err := vscodeext.Version(data)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("Heimatverzeichnis: %w", err)
	}

	cli, err := vscodeext.ChooseCLI(home, os.Getenv, vscodeext.LookPath)
	if err != nil {
		if errors.Is(err, vscodeext.ErrNoCLI) {
			return fmt.Errorf("%w — %s", err, vscodeext.ManualHint)
		}
		return err
	}

	fmt.Fprintf(out, "Installiere %s %s über %s (%s)\n", vscodeext.Name, version, cli.Kind, cli.Path)
	output, err := vscodeext.Install(context.Background(), cli, data)
	if output != "" {
		fmt.Fprintln(out, output)
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Erledigt. Ein Reload des Fensters ist nicht nötig.")
	return nil
}

func runVSCodeStatus(args []string, out io.Writer) error {
	if err := noArgs("status", args); err != nil {
		return err
	}

	status := vscodeext.CurrentStatus()
	if status.Error != "" {
		return errors.New(status.Error)
	}

	fmt.Fprintf(out, "Erweiterung: %s (%s)\n", status.Name, status.ID)
	if status.Embedded != "" {
		fmt.Fprintf(out, "Eingebettet: %s\n", status.Embedded)
	} else {
		fmt.Fprintf(out, "Eingebettet: keine — %s\n", status.EmbeddedError)
	}

	if len(status.Installed) == 0 {
		fmt.Fprintln(out, "Installiert: in keinem Verzeichnis (maßgeblich ist extensions.json)")
	} else {
		fmt.Fprintln(out, "Installiert:")
		for _, installation := range status.Installed {
			fmt.Fprintf(out, "  %s: %s\n", installation.Dir, installation.Version)
		}
	}

	if status.CLI != nil {
		fmt.Fprintf(out, "CLI: %s (%s)\n", status.CLI.Path, status.CLI.Kind)
		fmt.Fprintf(out, "Nachzug schreibt nach: %s\n", status.Target)
	} else {
		fmt.Fprintf(out, "CLI: keine — %s\n", status.Hint)
	}

	// Eine zweite Installation in abweichender Fassung wird nicht
	// weggerechnet: der Nachzug erreicht sie nicht, und wer sie nicht sieht,
	// hält sie für aktuell.
	for _, installation := range status.Divergent {
		fmt.Fprintf(out, "Abweichend, von dieser CLI nicht erreichbar: %s: %s\n",
			installation.Dir, installation.Version)
	}

	if status.NeedsInstall {
		fmt.Fprintln(out, "Nachzuziehen: ja")
	} else {
		fmt.Fprintln(out, "Nachzuziehen: nein")
	}
	return nil
}

func runVSCodeVSIX(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("vscode vsix", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	target := flags.String("o", "", "Datei, die geschrieben wird")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("vscode vsix: %w", err)
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("vscode vsix erwartet keine weiteren Argumente: %s", flags.Arg(0))
	}
	if *target == "" {
		return errors.New("vscode vsix: -o <datei> fehlt")
	}

	data, ok := vscodeext.VSIX()
	if !ok {
		return errors.New(vscodeext.NoVSIX)
	}
	if err := os.WriteFile(*target, data, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "Geschrieben: %s\n", *target)
	fmt.Fprintln(out, "In VS Code: „Extensions: Install from VSIX…“")
	return nil
}

// vscodeCollections ist die Antwort von `vscode collections`. Die Erweiterung
// liest sie als JSON; configured false ist ein Zustand, kein Fehler, und
// message sagt, warum.
type vscodeCollections struct {
	Configured bool                         `json:"configured"`
	Folders    []kephalaion.WorkspaceFolder `json:"folders"`
	Message    string                       `json:"message,omitempty"`
}

func runVSCodeCollections(args []string, out io.Writer) error {
	asJSON := false
	for _, arg := range args {
		if arg != "--json" {
			return fmt.Errorf("vscode collections: unbekannte Option %s — erwartet wird [--json]", arg)
		}
		asJSON = true
	}

	start, err := os.Getwd()
	if err != nil {
		return err
	}
	answer := collectWorkspaceFolders(start)

	if asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(answer)
	}
	if !answer.Configured {
		fmt.Fprintf(out, "Nicht eingerichtet: %s\n", answer.Message)
		return nil
	}
	for _, folder := range answer.Folders {
		fmt.Fprintf(out, "%s  %s\n", folder.URI, folder.Name)
	}
	return nil
}

// collectWorkspaceFolders leitet die Ordner aus der Anbindung des Projekts
// ab, in dem startDir liegt. Gelesen wird nur die Datei; der Node wird nicht
// gefragt, denn die Erweiterung ruft das bei jedem Fensterstart.
func collectWorkspaceFolders(startDir string) vscodeCollections {
	answer := vscodeCollections{Folders: []kephalaion.WorkspaceFolder{}}
	environment := project.DetectFrom(startDir)
	if !environment.Installed {
		answer.Message = fmt.Sprintf("kein k-playbook-Projekt — keine %s ab %s aufwärts",
			project.ConfigFileName, project.DisplayPath(startDir))
		return answer
	}

	binding, configured, err := kephalaion.ReadBinding(project.LocalDir(environment.ProjectDir))
	if err != nil {
		answer.Message = err.Error()
		return answer
	}
	if !configured {
		answer.Message = fmt.Sprintf("keine Anbindung an Kephalaion — Block %s: in %s/%s fehlt",
			kephalaion.BlockName, project.LocalDirName, kephalaion.LocalFileName)
		return answer
	}

	folders, err := kephalaion.WorkspaceFolders(binding)
	if err != nil {
		answer.Message = fmt.Sprintf("Anbindung in %s/%s: %v", project.LocalDirName, kephalaion.LocalFileName, err)
		return answer
	}
	answer.Configured = true
	answer.Folders = folders
	return answer
}
