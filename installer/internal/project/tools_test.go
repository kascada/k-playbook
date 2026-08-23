package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newInstallationWithScript legt ein Projekt an, dessen Installation ein
// Preflight-Skript mit vorgegebener Ausgabe enthält.
func newInstallationWithScript(t *testing.T, body string) string {
	t.Helper()

	root := t.TempDir()
	scriptDir := filepath.Join(PlaybookDir(root), "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	script := filepath.Join(scriptDir, "install-security-tools.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\n"+body), 0o755); err != nil {
		t.Fatalf("Skript anlegen: %v", err)
	}
	return root
}

func TestCheckToolsLiestPreflight(t *testing.T) {
	root := newInstallationWithScript(t, `cat <<'JSON'
{"playbookDir":"/x","toolMatrix":"/x/scripts/security-tools.tsv","binDir":"/x/bin",
 "venvRoot":"/x/venv","languages":"go","missingRequired":1,"installCommand":"bash x --install missing","installCommandVenv":"bash x --install missing --method venv",
 "tools":[{"name":"gitleaks","languages":"*","required":true,"installMethod":"github","status":"ok","version":"8.30.1","path":"/x/gitleaks","role":"Secret-Scanning","dockerImage":"img"},
          {"name":"gosec","languages":"go","required":true,"installMethod":"go","status":"missing","version":"","path":"","role":"Go-Security","dockerImage":""}]}
JSON
`)

	preflight, err := CheckTools(root, []string{"go"})
	if err != nil {
		t.Fatalf("CheckTools: %v", err)
	}
	if preflight.MissingRequired != 1 {
		t.Errorf("MissingRequired = %d, erwartet 1", preflight.MissingRequired)
	}
	if preflight.Languages != "go" {
		t.Errorf("Languages = %q, erwartet %q", preflight.Languages, "go")
	}
	if preflight.VenvRoot != "/x/venv" {
		t.Errorf("VenvRoot = %q, erwartet %q", preflight.VenvRoot, "/x/venv")
	}
	if preflight.InstallCommandVenv != "bash x --install missing --method venv" {
		t.Errorf("InstallCommandVenv = %q, erwartet venv-Installationsbefehl", preflight.InstallCommandVenv)
	}
	if len(preflight.Tools) != 2 {
		t.Fatalf("%d Tools, erwartet 2", len(preflight.Tools))
	}
	if preflight.Tools[0].Version != "8.30.1" {
		t.Errorf("Version = %q, erwartet %q", preflight.Tools[0].Version, "8.30.1")
	}
	if preflight.Tools[0].Languages != "*" {
		t.Errorf("Languages = %q, erwartet %q", preflight.Tools[0].Languages, "*")
	}
	if preflight.Tools[1].Status != "missing" {
		t.Errorf("Status = %q, erwartet %q", preflight.Tools[1].Status, "missing")
	}
	if preflight.Tools[1].InstallMethod != "go" {
		t.Errorf("InstallMethod = %q, erwartet %q", preflight.Tools[1].InstallMethod, "go")
	}
}

func TestCheckToolsOhneSkript(t *testing.T) {
	if _, err := CheckTools(t.TempDir(), nil); err == nil {
		t.Error("fehlendes Skript wurde nicht gemeldet")
	}
}

// Bricht das Skript ab — etwa wegen eines aktiven Projekt-venv —, muss seine
// Meldung durchgereicht werden statt eines nackten Exit-Codes.
func TestCheckToolsReichtFehlermeldungDurch(t *testing.T) {
	root := newInstallationWithScript(t, "echo 'ERROR: venv ist aktiv' >&2\nexit 1\n")

	_, err := CheckTools(root, []string{"go"})
	if err == nil {
		t.Fatal("Abbruch wurde nicht gemeldet")
	}
	if got := err.Error(); got != "ERROR: venv ist aktiv" {
		t.Errorf("Fehler = %q, erwartet die Skriptmeldung", got)
	}
}

// Binary und Skript können auseinanderlaufen, wenn das Binary aus der
// host-weiten Kopie stammt und die Installation des Projekts älter ist. "Unknown
// argument" allein sagt nicht, dass ein Update fehlt.
func TestCheckToolsErklaertVeralteteInstallation(t *testing.T) {
	root := newInstallationWithScript(t, "echo 'ERROR: Unknown argument: --languages' >&2\nexit 1\n")

	_, err := CheckTools(root, []string{"python"})
	if err == nil {
		t.Fatal("Abbruch wurde nicht gemeldet")
	}
	got := err.Error()
	for _, want := range []string{"älter als dieses Werkzeug", "Update prüfen", "--languages"} {
		if !strings.Contains(got, want) {
			t.Errorf("Fehler = %q, erwartet einen Hinweis auf %q", got, want)
		}
	}
}

func TestCheckToolsMeldetUnlesbareAusgabe(t *testing.T) {
	root := newInstallationWithScript(t, "echo 'kein JSON'\n")

	if _, err := CheckTools(root, []string{"go"}); err == nil {
		t.Error("unlesbare Ausgabe wurde nicht gemeldet")
	}
}
