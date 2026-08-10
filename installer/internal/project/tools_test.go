package project

import (
	"os"
	"path/filepath"
	"testing"
)

// newInstallationWithScript legt ein Projekt an, dessen Installation ein
// Preflight-Skript mit vorgegebener Ausgabe enthaelt.
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
 "venvDir":"/x/venv","missingRequired":1,"installCommand":"bash x --install missing",
 "tools":[{"name":"gitleaks","required":true,"status":"ok","version":"8.30.1","path":"/x/gitleaks","role":"Secret-Scanning","dockerImage":"img"},
          {"name":"trivy","required":true,"status":"missing","version":"","path":"","role":"CVEs","dockerImage":""}]}
JSON
`)

	preflight, err := CheckTools(root)
	if err != nil {
		t.Fatalf("CheckTools: %v", err)
	}
	if preflight.MissingRequired != 1 {
		t.Errorf("MissingRequired = %d, erwartet 1", preflight.MissingRequired)
	}
	if len(preflight.Tools) != 2 {
		t.Fatalf("%d Tools, erwartet 2", len(preflight.Tools))
	}
	if preflight.Tools[0].Version != "8.30.1" {
		t.Errorf("Version = %q, erwartet %q", preflight.Tools[0].Version, "8.30.1")
	}
	if preflight.Tools[1].Status != "missing" {
		t.Errorf("Status = %q, erwartet %q", preflight.Tools[1].Status, "missing")
	}
}

func TestCheckToolsOhneSkript(t *testing.T) {
	if _, err := CheckTools(t.TempDir()); err == nil {
		t.Error("fehlendes Skript wurde nicht gemeldet")
	}
}

// Bricht das Skript ab — etwa wegen eines aktiven Projekt-venv —, muss seine
// Meldung durchgereicht werden statt eines nackten Exit-Codes.
func TestCheckToolsReichtFehlermeldungDurch(t *testing.T) {
	root := newInstallationWithScript(t, "echo 'ERROR: venv ist aktiv' >&2\nexit 1\n")

	_, err := CheckTools(root)
	if err == nil {
		t.Fatal("Abbruch wurde nicht gemeldet")
	}
	if got := err.Error(); got != "ERROR: venv ist aktiv" {
		t.Errorf("Fehler = %q, erwartet die Skriptmeldung", got)
	}
}

func TestCheckToolsMeldetUnlesbareAusgabe(t *testing.T) {
	root := newInstallationWithScript(t, "echo 'kein JSON'\n")

	if _, err := CheckTools(root); err == nil {
		t.Error("unlesbare Ausgabe wurde nicht gemeldet")
	}
}
