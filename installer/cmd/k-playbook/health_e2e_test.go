package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// healthE2EProject legt ein Projekt an, in dem genau ein Problem steht: die
// Anbindung an Kephalaion ist nicht eingetragen (eine Warnung). gh ist nicht
// vorausgesetzt, eine Matrix der Basis-Werkzeuge fehlt, die
// Quellenkonfiguration auch — alles drei schweigt.
func healthE2EProject(t *testing.T) string {
	t.Helper()
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	return todoProjectRoot(t)
}

// runHealthCaptured führt das Subkommando aus und fängt stdout ab.
func runHealthCaptured(t *testing.T, args ...string) (string, error) {
	t.Helper()

	file, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatalf("Ausgabedatei: %v", err)
	}
	before := os.Stdout
	os.Stdout = file
	runErr := runHealth(args)
	os.Stdout = before
	file.Close()

	stdout, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("Ausgabe lesen: %v", err)
	}
	return string(stdout), runErr
}

func TestHealthKlartextMitWarnungEndetMitNull(t *testing.T) {
	healthE2EProject(t)

	stdout, err := runHealthCaptured(t)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if !strings.HasPrefix(stdout, "Umgebung:\n  [warning] kephalaion: Keine Collection eingetragen") || !strings.Contains(stdout, " — ") {
		t.Errorf("Ausgabe:\n%s", stdout)
	}
	if strings.Contains(stdout, "[error]") {
		t.Errorf("Ausgabe nennt einen Fehler:\n%s", stdout)
	}
}

func TestHealthFehlerEndetMitEinsUndJSONTraegtDenReport(t *testing.T) {
	root := healthE2EProject(t)
	path := filepath.Join(project.LocalDir(root), project.VersionSourcesFileName)
	if err := os.WriteFile(path, []byte(": ungültig\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, err := runHealthCaptured(t)
	if err == nil || err.Error() != "" || exitCodeOf(err) != healthExitProblems {
		t.Fatalf("Fehler: %v (Code %d)", err, exitCodeOf(err))
	}
	if !strings.Contains(stdout, "  [error] versionSources: Die Quellenkonfiguration des Versionsinventars "+path+" ist nicht lesbar") {
		t.Errorf("Ausgabe:\n%s", stdout)
	}
	if strings.Contains(stdout, "ungültig") {
		t.Errorf("Ausgabe zitiert die Datei:\n%s", stdout)
	}

	stdout, err = runHealthCaptured(t, "--json")
	if exitCodeOf(err) != healthExitProblems {
		t.Fatalf("--json: %v", err)
	}
	var report health.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("kein JSON: %v\n%s", err, stdout)
	}
	if report.OK || len(report.Problems) != 2 {
		t.Fatalf("Report: %+v", report)
	}
	// Sortiert nach Bereich: kephalaion vor versionSources.
	if report.Problems[0].Area != health.AreaKephalaion || report.Problems[0].Level != health.LevelWarning ||
		report.Problems[1].Area != health.AreaVersionSources || report.Problems[1].Level != health.LevelError || report.Problems[1].Hint == "" {
		t.Errorf("Problems: %+v", report.Problems)
	}
	// Der Report ist das ganze JSON — kein Umschlag auf der Kommandozeile.
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &top); err != nil || len(top) != 2 {
		t.Errorf("JSON oben: %v", top)
	}
}

func TestHealthAufruffehlerEndenMitZwei(t *testing.T) {
	healthE2EProject(t)

	_, err := runHealthCaptured(t, "--nein")
	if err == nil || exitCodeOf(err) != healthExitUsage || !strings.Contains(err.Error(), "unbekannte Option") {
		t.Errorf("unbekannte Option: %v", err)
	}

	// Außerhalb eines Projekts.
	outside := t.TempDir()
	before, _ := os.Getwd()
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(before) })
	_, err = runHealthCaptured(t)
	if err == nil || exitCodeOf(err) != healthExitUsage || !strings.Contains(err.Error(), project.ConfigFileName) {
		t.Errorf("ohne Projekt: %v", err)
	}
}

// Ohne Problem ist der Klartext genau eine Zeile.
func TestHealthTextOhneProblemeIstEineZeile(t *testing.T) {
	if got := healthText(health.Report{OK: true, Problems: []health.Problem{}}); got != "Umgebung: keine Probleme\n" {
		t.Errorf("Klartext: %q", got)
	}
	got := healthText(health.Report{Problems: []health.Problem{
		{Level: health.LevelError, Area: "kephalaion", Message: "Satz.", Hint: "Abhilfe."},
		{Level: health.LevelWarning, Area: "gh", Message: "Satz ohne Abhilfe."},
	}})
	if got != "Umgebung:\n  [error] kephalaion: Satz. — Abhilfe.\n  [warning] gh: Satz ohne Abhilfe.\n" {
		t.Errorf("Klartext: %q", got)
	}
}

func TestExitCodeOf(t *testing.T) {
	if exitCodeOf(errors.New("x")) != 1 {
		t.Error("gewöhnlicher Fehler endet nicht mit 1")
	}
	coded := exitError{code: 2, err: errors.New("Aufruf")}
	if exitCodeOf(coded) != 2 || coded.Error() != "Aufruf" || !errors.Is(coded, coded.err) {
		t.Errorf("exitError: %v", coded)
	}
	if (exitError{code: 1}).Error() != "" {
		t.Error("exitError ohne err hat Text")
	}
}
