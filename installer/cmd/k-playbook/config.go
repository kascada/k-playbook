package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// runConfig verwaltet ausschließlich den Projektanker. Der explizite Weg ist
// auch dann nutzbar, wenn ein Anker eines übergeordneten Projekts die GUI-
// Ersteinrichtung überdeckt.
func runConfig(args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("erwartet: k-playbook config create [--repo-root <pfad>] [hauptverzeichnis]")
	}

	flags := flag.NewFlagSet("config create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	repoRoot := flags.String("repo-root", ".", "Projekt-Repository relativ zum Hauptverzeichnis")
	if err := flags.Parse(args[1:]); err != nil {
		return fmt.Errorf("config create: %w", err)
	}
	if flags.NArg() > 1 {
		return fmt.Errorf("config create erwartet höchstens ein Hauptverzeichnis")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Arbeitsverzeichnis lesen: %w", err)
	}
	if flags.NArg() == 1 {
		projectDir = flags.Arg(0)
	}
	if err := project.CreateConfig(projectDir, *repoRoot); err != nil {
		return fmt.Errorf("Konfiguration nicht angelegt: %w", err)
	}

	config, err := project.ReadConfig(projectDir)
	if err != nil {
		return fmt.Errorf("angelegte Konfiguration lesen: %w", err)
	}
	fmt.Printf("Angelegt: %s\n", project.ConfigPath(projectDir))
	fmt.Printf("Repository: %s\n", project.RepoRootDir(projectDir, config))
	fmt.Printf("Versionskontrolle: %s\n", config.VCS)
	return nil
}
