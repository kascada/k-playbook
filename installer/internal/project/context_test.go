package project

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newContextProject baut ein Projekt mit Installation und lokalem Verzeichnis.
func newContextProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{
		filepath.Join(PlaybookDir(root), "rules"),
		filepath.Join(PlaybookDir(root), "reviews"),
		filepath.Join(PlaybookDir(root), "checks"),
		filepath.Join(LocalDir(root), "rules"),
		filepath.Join(LocalDir(root), "reviews"),
		filepath.Join(LocalDir(root), "checks"),
		filepath.Join(LocalDir(root), "guidelines"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", dir, err)
		}
	}
	if err := os.WriteFile(ConfigPath(root), []byte("schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n"), 0o644); err != nil {
		t.Fatalf("Config anlegen: %v", err)
	}
	return root
}

func write(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
}

func entryByName(entries []CatalogEntry, name string) (CatalogEntry, bool) {
	for _, entry := range entries {
		if entry.Name == name {
			return entry, true
		}
	}
	return CatalogEntry{}, false
}

func TestBuildContextLoestKatalogeAuf(t *testing.T) {
	root := newContextProject(t)

	write(t, filepath.Join(PlaybookDir(root), "rules", "docs-sync.md"), "# mitgeliefert\n")
	write(t, filepath.Join(PlaybookDir(root), "rules", "review-authoring.md"), "# mitgeliefert\n")
	write(t, filepath.Join(LocalDir(root), "rules", "docs-sync.md"), "# eigene Fassung\n")
	write(t, filepath.Join(LocalDir(root), "rules", "eigene.md"), "# nur lokal\n")

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	rules := context.Catalogs["rules"]
	if len(rules) != 3 {
		t.Fatalf("%d Regeln, erwartet 3: %+v", len(rules), rules)
	}

	cases := map[string]string{
		"docs-sync.md":        "override",
		"review-authoring.md": "dist",
		"eigene.md":           "local",
	}
	for name, origin := range cases {
		entry, ok := entryByName(rules, name)
		if !ok {
			t.Errorf("%s fehlt im Katalog", name)
			continue
		}
		if entry.Origin != origin {
			t.Errorf("%s: Origin = %q, erwartet %q", name, entry.Origin, origin)
		}
	}
}

// Eine leere lokale Datei schaltet den mitgelieferten Eintrag ab.
func TestBuildContextMeldetAbgeschaltet(t *testing.T) {
	root := newContextProject(t)

	write(t, filepath.Join(PlaybookDir(root), "checks", "check_secrets.sh"), "#!/bin/sh\necho x\n")
	write(t, filepath.Join(LocalDir(root), "checks", "check_secrets.sh"), "# Abgeschaltet: nicht nötig.\n")

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	entry, ok := entryByName(context.Catalogs["checks"], "check_secrets.sh")
	if !ok {
		t.Fatal("check_secrets.sh fehlt im Katalog")
	}
	if !entry.Disabled {
		t.Error("Disabled = false, obwohl die lokale Datei leer ist")
	}
	if entry.Origin != "override" {
		t.Errorf("Origin = %q, erwartet override", entry.Origin)
	}
}

// README und Unterverzeichnisse sind nie Einträge.
func TestBuildContextUeberspringtNichtEintraege(t *testing.T) {
	root := newContextProject(t)

	write(t, filepath.Join(PlaybookDir(root), "checks", "README.md"), "# Doku\n")
	write(t, filepath.Join(PlaybookDir(root), "checks", "check_echt.sh"), "#!/bin/sh\n")
	write(t, filepath.Join(PlaybookDir(root), "checks", ".versteckt.sh"), "#!/bin/sh\n")
	if err := os.MkdirAll(filepath.Join(PlaybookDir(root), "checks", "lib"), 0o755); err != nil {
		t.Fatalf("lib anlegen: %v", err)
	}
	write(t, filepath.Join(PlaybookDir(root), "checks", "lib", "common.py"), "# Hilfscode\n")

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if len(context.Catalogs["checks"]) != 1 {
		t.Errorf("Katalog = %+v, erwartet nur check_echt.sh", context.Catalogs["checks"])
	}
}

// Reviews tragen ein Präfix; der Key ist der handliche Aufrufname.
func TestBuildContextBildetKeys(t *testing.T) {
	root := newContextProject(t)

	write(t, filepath.Join(PlaybookDir(root), "reviews", "review-secret-scanning.md"), "# Rezept\n")
	write(t, filepath.Join(PlaybookDir(root), "reviews", "nicht-passend.md"), "# kein Review\n")

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	reviews := context.Catalogs["reviews"]
	if len(reviews) != 1 {
		t.Fatalf("%d Reviews, erwartet 1: %+v", len(reviews), reviews)
	}
	if reviews[0].Key != "secret-scanning" {
		t.Errorf("Key = %q, erwartet secret-scanning", reviews[0].Key)
	}
	if reviews[0].Name != "review-secret-scanning.md" {
		t.Errorf("Name = %q, erwartet den Dateinamen", reviews[0].Name)
	}
}

func TestBuildContextLiefertPfadeUndRemediation(t *testing.T) {
	root := newContextProject(t)
	write(t, filepath.Join(LocalDir(root), "guidelines", "stil.md"), "# Vorgabe\n")

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	if context.Playbook.Dir != PlaybookDir(root) {
		t.Errorf("Playbook.Dir = %q", context.Playbook.Dir)
	}
	if context.Local.Dir != LocalDir(root) {
		t.Errorf("Local.Dir = %q", context.Local.Dir)
	}
	if context.Project.VCS != "git" {
		t.Errorf("VCS = %q", context.Project.VCS)
	}
	if context.Remediation.Mode != DefaultRemediationMode {
		t.Errorf("Remediation.Mode = %q", context.Remediation.Mode)
	}
	if len(context.Guidelines) != 1 {
		t.Errorf("Guidelines = %v, erwartet eine Datei", context.Guidelines)
	}
}

func TestContextForDirOhneInstallation(t *testing.T) {
	if _, err := ContextForDir(t.TempDir()); err == nil {
		t.Error("fehlende Installation wurde nicht gemeldet")
	}
}

// Das Datum kommt aus dem Kontext, damit ein Assistent es nicht raten muss.
func TestBuildContextLiefertZeitpunkt(t *testing.T) {
	root := newContextProject(t)

	fixed := time.Date(2026, 8, 12, 14, 5, 0, 0, time.FixedZone("CEST", 2*60*60))
	original := now
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = original })

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if context.Now.Date != "2026-08-12" {
		t.Errorf("Now.Date = %q, erwartet %q", context.Now.Date, "2026-08-12")
	}
	if context.Now.Timestamp != "2026-08-12T14:05:00+02:00" {
		t.Errorf("Now.Timestamp = %q", context.Now.Timestamp)
	}
}

// Ein Kontext auf Basis einer fremden Fassung wäre irreführend.
func TestBuildContextLehntFremdesSchemaAb(t *testing.T) {
	root := newContextProject(t)
	write(t, ConfigPath(root), "schema_version: 2\n\nproject:\n  repo_root: .\n")

	if _, err := BuildContext(root); err == nil {
		t.Error("Kontext wurde trotz fremder schema_version gebaut")
	}
}
