package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/project"
)

func TestParseMaintainArgs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want maintainOptions
	}{
		{nil, maintainOptions{}},
		{[]string{"--dry-run", "--commit"}, maintainOptions{dryRun: true, commit: true}},
		{[]string{"--only", "docs-backup,b"}, maintainOptions{only: []string{"docs-backup", "b"}}},
		{[]string{"--only=a", "--only", "b"}, maintainOptions{only: []string{"a", "b"}}},
		{[]string{"--help"}, maintainOptions{help: true}},
	} {
		got, err := parseMaintainArgs(tc.args)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%v: %+v %v, erwartet %+v", tc.args, got, err, tc.want)
		}
	}
	for _, args := range [][]string{{"--only"}, {"--bogus"}, {"docs-backup"}} {
		if _, err := parseMaintainArgs(args); err == nil {
			t.Errorf("%v: kein Fehler", args)
		}
	}
	if err := runMaintain([]string{"--bogus"}); exitCodeOf(err) != 2 {
		t.Errorf("unbekannte Option: Code %d", exitCodeOf(err))
	}
}

// maintainProject legt ein Projekt mit neuer Konfiguration an (Block
// maintenance: mit Sicherung aus) und macht es zum Arbeitsverzeichnis.
// kephalaion fehlt: Kein Test fragt einen Node.
func maintainProject(t *testing.T) string {
	t.Helper()
	kephtest.NoBinary(t)
	return todoProjectRoot(t)
}

func setMaintenanceBlock(t *testing.T, root, block string) {
	t.Helper()
	path := project.ConfigPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	index := strings.Index(content, "maintenance:\n")
	if index < 0 {
		t.Fatalf("kein Block maintenance:\n%s", content)
	}
	if err := os.WriteFile(path, []byte(content[:index]+block), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runMaintainIn(t *testing.T, root string, options maintainOptions) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := maintain(context.Background(), root, options, &out, &errOut)
	return out.String() + errOut.String(), err
}

// Tagesprogramm mit deaktivierter Sicherung: übersprungen, Exit 0.
func TestMaintainTagesprogrammDeaktiviert(t *testing.T) {
	root := maintainProject(t)
	out, err := runMaintainIn(t, root, maintainOptions{})
	if err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	want := "docs-backup: übersprungen — Sicherung deaktiviert (maintenance.backup.enabled ist nicht true)\n" +
		"Ergebnis: erfolgreich (Auswahl: maintenance.daily)\n"
	if out != want {
		t.Errorf("Ausgabe:\n%s\nerwartet:\n%s", out, want)
	}
	if _, err := os.Stat(filepath.Join(root, "keph-backup")); !os.IsNotExist(err) {
		t.Errorf("Ziel angelegt: %v", err)
	}
}

// Einzelauswahl und Vorschau.
func TestMaintainEinzelauswahlVorschau(t *testing.T) {
	root := maintainProject(t)
	out, err := runMaintainIn(t, root, maintainOptions{only: []string{"docs-backup"}, dryRun: true})
	if err != nil || !strings.Contains(out, "docs-backup: übersprungen") ||
		!strings.HasSuffix(out, "Ergebnis: erfolgreich (Auswahl: --only, Vorschau ohne Schreiben)\n") {
		t.Errorf("Ausgabe: %v\n%s", err, out)
	}
}

// Ungültige Schritte in --only oder maintenance.daily: Exit 2 vor jedem
// Schreiben, die Meldung nennt die gültigen Namen.
func TestMaintainUngueltigeSchritte(t *testing.T) {
	root := maintainProject(t)
	setMaintenanceBlock(t, root, "maintenance:\n  backup:\n    enabled: true\n")

	out, err := runMaintainIn(t, root, maintainOptions{only: []string{"nix"}})
	if exitCodeOf(err) != 2 || !strings.Contains(err.Error(), "gültig: docs-backup") || out != "" {
		t.Errorf("--only: %v (Code %d)\n%s", err, exitCodeOf(err), out)
	}

	setMaintenanceBlock(t, root, "maintenance:\n  daily:\n    - nix\n  backup:\n    enabled: true\n")
	out, err = runMaintainIn(t, root, maintainOptions{})
	if exitCodeOf(err) != 2 || !strings.Contains(err.Error(), "maintenance.daily: nix") || out != "" {
		t.Errorf("daily: %v (Code %d)\n%s", err, exitCodeOf(err), out)
	}
	if _, err := os.Stat(filepath.Join(root, "keph-backup")); !os.IsNotExist(err) {
		t.Errorf("geschrieben: %v", err)
	}
}

// Scheitert ein Schritt, ist das Gesamtergebnis Fehler und der Exit-Code 1.
func TestMaintainFehlerGesamtergebnis(t *testing.T) {
	root := maintainProject(t)
	setMaintenanceBlock(t, root, "maintenance:\n  backup:\n    enabled: true\n")
	out, err := runMaintainIn(t, root, maintainOptions{commit: true})
	if exitCodeOf(err) != 1 || err.Error() != "" {
		t.Fatalf("Fehler: %v (Code %d)", err, exitCodeOf(err))
	}
	if !strings.Contains(out, "docs-backup: Fehler — Kephalaion ist nicht bereit") ||
		!strings.HasSuffix(out, "Ergebnis: Fehler (1 von 1 Schritten; Auswahl: Vorgabe des Tagesprogramms)\n") {
		t.Errorf("Ausgabe:\n%s", out)
	}
}

// Ohne Terminal und ohne Eingabe läuft der Befehl gleich: kein Lesen von
// stdin, dieselbe Ausgabe.
func TestMaintainOhneTerminalUndEingabe(t *testing.T) {
	maintainProject(t)
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	beforeIn := os.Stdin
	os.Stdin = devNull
	t.Cleanup(func() { os.Stdin = beforeIn })

	stdout := captureStdout(t, func() {
		if err := runMaintain(nil); err != nil {
			t.Errorf("maintain: %v", err)
		}
	})
	if !strings.HasPrefix(stdout, "docs-backup: übersprungen") || !strings.Contains(stdout, "Ergebnis: erfolgreich") {
		t.Errorf("Ausgabe:\n%s", stdout)
	}
}

func TestMaintainHilfe(t *testing.T) {
	stdout := captureStdout(t, func() {
		if err := runMaintain([]string{"--help"}); err != nil {
			t.Errorf("--help: %v", err)
		}
	})
	for _, name := range project.MaintenanceStepNames() {
		if !strings.Contains(stdout, "  "+name+" ") {
			t.Errorf("Hilfe ohne Schritt %s:\n%s", name, stdout)
		}
	}
	if !strings.Contains(stdout, "--dry-run") || !strings.Contains(stdout, "--commit") {
		t.Errorf("Hilfe:\n%s", stdout)
	}
}

// k-playbook upgrade ergänzt einen fehlenden Block maintenance: deaktiviert,
// --check meldet „würde ergänzt“ und schreibt nichts, ein zweiter Lauf hat
// nichts zu tun.
func TestUpgradeErgaenztMaintenance(t *testing.T) {
	root := upgradeProjectDir(t, "v0.16.8")
	setOwnVersion(t, "v0.16.8")
	path := project.ConfigPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	withoutBlock := string(data[:strings.Index(string(data), "maintenance:\n")])
	if err := os.WriteFile(path, []byte(withoutBlock), 0o644); err != nil {
		t.Fatal(err)
	}

	check := captureStdout(t, func() {
		if err := runUpgrade([]string{"--check"}); err != nil {
			t.Errorf("upgrade --check: %v", err)
		}
	})
	if !strings.Contains(check, "Projekt: Block maintenance in K-PLAYBOOK.yaml würde ergänzt (Doku-Sicherung deaktiviert") {
		t.Errorf("--check:\n%s", check)
	}
	if after, _ := os.ReadFile(path); string(after) != withoutBlock {
		t.Error("--check hat K-PLAYBOOK.yaml verändert")
	}

	run := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("upgrade: %v", err)
		}
	})
	if !strings.Contains(run, "Projekt: Block maintenance in K-PLAYBOOK.yaml ergänzt (Doku-Sicherung deaktiviert") {
		t.Errorf("Lauf:\n%s", run)
	}
	config, err := project.ReadConfig(root)
	if err != nil || !config.Maintenance.Present || config.Maintenance.BackupEnabled || config.Maintenance.GitCommit {
		t.Errorf("Block nach dem Lauf: %+v %v", config.Maintenance, err)
	}
	written, _ := os.ReadFile(path)

	second := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("zweiter Lauf: %v", err)
		}
	})
	if strings.Contains(second, "maintenance") || !strings.Contains(second, "Projekt: nichts zu tun.") {
		t.Errorf("zweiter Lauf:\n%s", second)
	}
	if again, _ := os.ReadFile(path); string(again) != string(written) {
		t.Error("zweiter Lauf hat K-PLAYBOOK.yaml verändert")
	}
	check = captureStdout(t, func() { _ = runUpgrade([]string{"--check"}) })
	if strings.Contains(check, "maintenance") {
		t.Errorf("--check nach dem Lauf:\n%s", check)
	}
}
