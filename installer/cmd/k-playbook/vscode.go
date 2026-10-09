package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

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
