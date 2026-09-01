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

func TestBuildContextZeigtReviewModi(t *testing.T) {
	root := newContextProject(t)

	write(t, filepath.Join(PlaybookDir(root), "reviews", "review-default.md"), "---\ntitle: Default\n---\n# Default\n")
	write(t, filepath.Join(PlaybookDir(root), "reviews", "review-audit-only.md"), "---\naudit:\n  enabled: true\n  resultRequired: true\n  defaultResult: review-audit-only.md\n  scope:\n    tools: [gitleaks, trufflehog]\nreview:\n  enabled: false\n---\n# Audit\n")

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	defaultEntry, ok := entryByName(context.Catalogs["reviews"], "review-default.md")
	if !ok {
		t.Fatal("review-default.md fehlt")
	}
	if defaultEntry.Audit == nil || defaultEntry.Audit.Enabled {
		t.Fatalf("default audit = %#v, erwartet false", defaultEntry.Audit)
	}
	if defaultEntry.Review == nil || !defaultEntry.Review.Enabled {
		t.Fatalf("default review = %#v, erwartet true", defaultEntry.Review)
	}

	auditEntry, ok := entryByName(context.Catalogs["reviews"], "review-audit-only.md")
	if !ok {
		t.Fatal("review-audit-only.md fehlt")
	}
	if auditEntry.Audit == nil || !auditEntry.Audit.Enabled {
		t.Fatalf("audit = %#v, erwartet true", auditEntry.Audit)
	}
	if auditEntry.Review == nil || auditEntry.Review.Enabled {
		t.Fatalf("review = %#v, erwartet false", auditEntry.Review)
	}
	if auditEntry.Audit.ResultRequired == nil || !*auditEntry.Audit.ResultRequired {
		t.Fatalf("resultRequired = %#v, erwartet true", auditEntry.Audit.ResultRequired)
	}
	if auditEntry.Audit.DefaultResult != "review-audit-only.md" {
		t.Fatalf("defaultResult = %q", auditEntry.Audit.DefaultResult)
	}
	if auditEntry.Audit.Scope == nil || len(auditEntry.Audit.Scope.Tools) != 2 || auditEntry.Audit.Scope.Tools[0] != "gitleaks" || auditEntry.Audit.Scope.Tools[1] != "trufflehog" {
		t.Fatalf("scope = %#v", auditEntry.Audit.Scope)
	}
}

func TestBuildContextLiestAuditScopeToolsAlsYAMLListe(t *testing.T) {
	root := newContextProject(t)

	write(t, filepath.Join(PlaybookDir(root), "reviews", "review-scope.md"), "---\naudit:\n  enabled: true\n  scope:\n    tools:\n      - pip-audit\n      - trivy\nreview:\n  enabled: true\n---\n# Scope\n")

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	entry, ok := entryByName(context.Catalogs["reviews"], "review-scope.md")
	if !ok {
		t.Fatal("review-scope.md fehlt")
	}
	if entry.Audit == nil || entry.Audit.Scope == nil || len(entry.Audit.Scope.Tools) != 2 {
		t.Fatalf("scope = %#v", entry.Audit)
	}
	if entry.Audit.Scope.Tools[0] != "pip-audit" || entry.Audit.Scope.Tools[1] != "trivy" {
		t.Fatalf("tools = %#v", entry.Audit.Scope.Tools)
	}
	// Ein Rezept ohne mode bleibt eine Perspektive: das Feld fehlt und keine
	// Evidence-Angabe entsteht daneben.
	if entry.Audit.Mode != "" {
		t.Errorf("mode = %q, erwartet leer", entry.Audit.Mode)
	}
	if len(entry.Audit.Scope.Paths) != 0 || len(entry.Audit.RuleIDs) != 0 {
		t.Errorf("Evidence-Felder gesetzt: %#v", entry.Audit)
	}
}

func TestBuildContextLiestEvidenceVertragAlsBlocklisten(t *testing.T) {
	root := newContextProject(t)

	write(t, filepath.Join(PlaybookDir(root), "reviews", "review-evidence.md"),
		"---\naudit:\n  enabled: true\n  mode: evidence\n  ruleIds:\n    - tech-veraltet\n    - tech-kopplung\n  scope:\n    paths:\n      - installer/**\n      - commands/**\nreview:\n  enabled: true\n---\n# Evidence\n")

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	entry, ok := entryByName(context.Catalogs["reviews"], "review-evidence.md")
	if !ok {
		t.Fatal("review-evidence.md fehlt")
	}
	if entry.Audit == nil || entry.Audit.Mode != "evidence" {
		t.Fatalf("mode = %#v", entry.Audit)
	}
	if entry.Audit.Scope == nil || len(entry.Audit.Scope.Paths) != 2 {
		t.Fatalf("paths = %#v", entry.Audit.Scope)
	}
	if entry.Audit.Scope.Paths[0] != "installer/**" || entry.Audit.Scope.Paths[1] != "commands/**" {
		t.Fatalf("paths = %#v", entry.Audit.Scope.Paths)
	}
	if len(entry.Audit.Scope.Tools) != 0 {
		t.Errorf("tools = %#v, erwartet leer", entry.Audit.Scope.Tools)
	}
	if len(entry.Audit.RuleIDs) != 2 || entry.Audit.RuleIDs[0] != "tech-veraltet" || entry.Audit.RuleIDs[1] != "tech-kopplung" {
		t.Fatalf("ruleIds = %#v", entry.Audit.RuleIDs)
	}
}

func TestBuildContextLiestEvidenceVertragAlsInlineListen(t *testing.T) {
	root := newContextProject(t)

	write(t, filepath.Join(PlaybookDir(root), "reviews", "review-evidence-inline.md"),
		"---\naudit:\n  enabled: true\n  mode: evidence\n  ruleIds: [tech-veraltet, tech-kopplung]\n  scope:\n    paths: [\"installer/**\"]\nreview:\n  enabled: false\n---\n# Evidence\n")

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}

	entry, ok := entryByName(context.Catalogs["reviews"], "review-evidence-inline.md")
	if !ok {
		t.Fatal("review-evidence-inline.md fehlt")
	}
	if entry.Audit == nil || entry.Audit.Mode != "evidence" {
		t.Fatalf("mode = %#v", entry.Audit)
	}
	if entry.Audit.Scope == nil || len(entry.Audit.Scope.Paths) != 1 || entry.Audit.Scope.Paths[0] != "installer/**" {
		t.Fatalf("paths = %#v", entry.Audit.Scope)
	}
	if len(entry.Audit.RuleIDs) != 2 {
		t.Fatalf("ruleIds = %#v", entry.Audit.RuleIDs)
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

// Der Zustand der Installation gehört in die Ausgabe: die Regel, dass dort nicht
// geschrieben wird, setzt sich nicht selbst durch.
func TestBuildContextLiefertSauberkeit(t *testing.T) {
	root := newContextProject(t)

	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	// Ohne .git in der Installation ist nichts zu prüfen — gemeldet wird das als
	// sauber, mit Begründung, nicht als Fehler.
	if !context.Cleanliness.Clean {
		t.Errorf("Cleanliness.Clean = false, erwartet true: %+v", context.Cleanliness)
	}
	if context.Cleanliness.Message == "" {
		t.Error("Cleanliness.Message ist leer")
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

// Der Kontextaufruf steht am Anfang jeder Sitzung. Er ist deshalb der Ort, an
// dem die Assistenten-Registrierung nachgezogen wird — unabhängig davon, auf
// welchem Weg die Installation zu ihrem Stand kam.
func TestContextForDirZiehtVerlinkungNach(t *testing.T) {
	root := newContextProject(t)
	writeFile(t, filepath.Join(PlaybookDir(root), "commands", "k-test.md"), "test\n")
	writeFile(t, filepath.Join(PlaybookDir(root), "skills", "beispiel", skillFileName), "# Beispiel\n")

	context, err := ContextForDir(root)
	if err != nil {
		t.Fatalf("ContextForDir: %v", err)
	}
	if context.Links == nil {
		t.Fatal("die Selbstheilung muss im Kontext stehen")
	}
	if context.Links.Note == "" {
		t.Error("ohne Klartext weiß ein Assistent nicht, was das für seine Sitzung heißt")
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "commands", "k-test.md")); err != nil {
		t.Errorf("der Command ist nicht registriert: %v", err)
	}

	// Beim zweiten Aufruf gibt es nichts mehr zu melden. Eine Meldung, die
	// jedes Mal dasselbe sagt, liest niemand mehr.
	again, err := ContextForDir(root)
	if err != nil {
		t.Fatalf("ContextForDir: %v", err)
	}
	if again.Links != nil {
		t.Errorf("Links = %+v, erwartet nichts zu melden", again.Links)
	}
}

// Versteht das Werkzeug die Konfiguration nicht, schreibt es auch nicht: die
// Registrierung nach den Regeln einer anderen Fassung umzubauen wäre schlimmer
// als sie stehen zu lassen.
func TestContextForDirHeiltNichtBeiUnbekannterFassung(t *testing.T) {
	root := newContextProject(t)
	writeFile(t, filepath.Join(PlaybookDir(root), "commands", "k-test.md"), "test\n")
	write(t, ConfigPath(root), "schema_version: 99\n\nproject:\n  repo_root: .\n  vcs: git\n")

	if _, err := ContextForDir(root); err == nil {
		t.Fatal("eine zu neue Fassung muss abbrechen")
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude")); !os.IsNotExist(err) {
		t.Error("bei abgebrochenem Aufbau darf nichts verlinkt worden sein")
	}
}
