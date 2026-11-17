package project

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const maintenanceConfigHead = "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n"

// Ohne Block gelten die Vorgaben: Tagesprogramm docs-backup, Sicherung aus.
func TestMaintenanceOhneBlockVorgaben(t *testing.T) {
	config := readDocsConfig(t, maintenanceConfigHead)
	m := config.Maintenance
	if m.Present || m.BackupEnabled || m.GitCommit || len(m.Problems) != 0 {
		t.Fatalf("ohne Block: %+v", m)
	}
	if got := m.EffectiveDaily(); !reflect.DeepEqual(got, []string{MaintenanceStepDocsBackup}) {
		t.Errorf("EffectiveDaily = %v", got)
	}
	if m.BackupSource != "docs" || m.BackupTarget != DefaultMaintenanceBackupTarget {
		t.Errorf("Vorgaben: %+v", m)
	}
}

func TestMaintenanceDeaktivierterBlock(t *testing.T) {
	config := readDocsConfig(t, maintenanceConfigHead+"\n"+maintenanceBlock())
	m := config.Maintenance
	if !m.Present || m.BackupEnabled || m.GitCommit || len(m.Problems) != 0 {
		t.Fatalf("deaktiviert: %+v", m)
	}
	if !m.DailySet || !reflect.DeepEqual(m.Daily, []string{"docs-backup"}) {
		t.Errorf("daily: %+v", m)
	}
}

func TestMaintenanceAktivierterBlock(t *testing.T) {
	block := "maintenance:\n  daily:\n    - docs-backup\n  backup:\n    enabled: true  # an\n    source: docs\n" +
		"    target: \"sicherung/doku\"\n    git:\n      commit: true\n"
	m := readDocsConfig(t, maintenanceConfigHead+block).Maintenance
	if !m.BackupEnabled || !m.GitCommit || m.BackupTarget != "sicherung/doku" || len(m.Problems) != 0 {
		t.Fatalf("aktiviert: %+v", m)
	}
}

// Fehlt daily, gilt die Vorgabe; teilweise vorhandene Blöcke lesen die
// fehlenden Felder als Vorgabe.
func TestMaintenanceTeilweiseVorhanden(t *testing.T) {
	m := readDocsConfig(t, maintenanceConfigHead+"maintenance:\n  backup:\n    enabled: true\n").Maintenance
	if m.DailySet || !reflect.DeepEqual(m.EffectiveDaily(), DefaultMaintenanceDaily) {
		t.Errorf("daily: %+v", m)
	}
	if !m.BackupEnabled || m.BackupTarget != DefaultMaintenanceBackupTarget || m.GitCommit || len(m.Problems) != 0 {
		t.Errorf("teilweise: %+v", m)
	}
}

// Die einzeilige Form wird gemeldet, nicht still als leer gelesen; [] ist die
// leere Liste; eine Liste in derselben Spalte wie daily: gilt.
func TestMaintenanceDailyFormen(t *testing.T) {
	m := readDocsConfig(t, maintenanceConfigHead+"maintenance:\n  daily: [docs-backup]\n").Maintenance
	if len(m.Problems) != 1 || !strings.Contains(m.Problems[0], "Wert in einer Zeile") || !strings.Contains(m.Problems[0], "Zeile 7") {
		t.Errorf("Flussform: %v", m.Problems)
	}
	m = readDocsConfig(t, maintenanceConfigHead+"maintenance:\n  daily: []\n").Maintenance
	if len(m.Problems) != 0 || !m.DailySet || len(m.EffectiveDaily()) != 0 {
		t.Errorf("leere Liste: %+v", m)
	}
	m = readDocsConfig(t, maintenanceConfigHead+"maintenance:\n  daily:\n  - docs-backup\n  - fremd # Kommentar\n").Maintenance
	if len(m.Problems) != 0 || !reflect.DeepEqual(m.Daily, []string{"docs-backup", "fremd"}) {
		t.Errorf("gleiche Spalte: %+v", m)
	}
}

func TestMaintenanceFalscheWerte(t *testing.T) {
	block := "maintenance:\n  backup:\n    enabled: yes\n    source: queue\n    git: {commit: true}\n"
	m := readDocsConfig(t, maintenanceConfigHead+block).Maintenance
	if m.BackupEnabled || m.GitCommit || len(m.Problems) != 3 {
		t.Fatalf("falsche Werte: %+v", m)
	}
	for index, want := range []string{"true oder false", "nur docs", "Wert in einer Zeile"} {
		if !strings.Contains(m.Problems[index], want) {
			t.Errorf("Problem %d = %q, erwartet %q", index, m.Problems[index], want)
		}
	}
}

func TestMaintenanceUnklarerAufbau(t *testing.T) {
	for name, block := range map[string]string{
		"mehrfach":  "maintenance:\n  daily: []\nmaintenance:\n  daily: []\n",
		"kopfzeile": "maintenance: {}\n",
		"tabulator": "maintenance:\n\tdaily: []\n",
		"doppelt":   "maintenance:\n  backup:\n    enabled: false\n    enabled: true\n",
		"spalte":    "maintenance:\n  backup:\n    enabled: false\n     source: docs\n",
		"gemischt":  "maintenance:\n  daily:\n    - docs-backup\n    target: x\n",
		"kein feld": "maintenance:\n  irgendwas\n",
	} {
		m := readDocsConfig(t, maintenanceConfigHead+block).Maintenance
		if len(m.Problems) == 0 {
			t.Errorf("%s: kein Problem gemeldet: %+v", name, m)
		}
		content := maintenanceConfigHead + block
		updated, added, problem := planMaintenanceBlock(content)
		if problem == "" || updated != content || added != nil {
			t.Errorf("%s: Ergänzen schrieb trotz unklarem Aufbau: %q %v %q", name, problem, added, updated)
		}
	}
}

// Das Upgrade ergänzt einen fehlenden Block mit den Vorgaben, bei Wiederholung
// nichts mehr.
func TestEnsureMaintenanceFehlenderBlockIdempotent(t *testing.T) {
	root := t.TempDir()
	original := maintenanceConfigHead + "\n# Schluss\ntools:\n  mcp:\n    required:\n      - k-playbook\n"
	writeFile(t, ConfigPath(root), original)

	result, err := EnsureMaintenanceBlock(root)
	if err != nil || !result.Changed || !reflect.DeepEqual(result.Added, []string{"maintenance"}) || result.Problem != "" {
		t.Fatalf("Ensure: %+v %v", result, err)
	}
	after := readConfigFile(t, root)
	if !strings.HasPrefix(after, original) || !strings.HasSuffix(after, "\n"+maintenanceBlock()) {
		t.Errorf("Block nicht hinten angehängt oder Rest verändert:\n%s", after)
	}
	m, _ := ReadConfig(root)
	if !m.Maintenance.Present || m.Maintenance.BackupEnabled || len(m.Maintenance.Problems) != 0 {
		t.Errorf("gelesen: %+v", m.Maintenance)
	}

	again, err := EnsureMaintenanceBlock(root)
	if err != nil || again.Changed || len(again.Added) != 0 {
		t.Errorf("zweiter Lauf: %+v %v", again, err)
	}
	if readConfigFile(t, root) != after {
		t.Error("zweiter Lauf hat die Datei geändert")
	}
}

// Teilweise vorhanden: nur die fehlenden Blätter kommen dazu, bestehende Werte
// (auch enabled: true und ein eigenes Ziel), fremde Schlüssel und Kommentare
// bleiben byte-gleich.
func TestEnsureMaintenanceErgaenztNurFehlendeBlaetter(t *testing.T) {
	root := t.TempDir()
	block := "maintenance:\n  # eigener Kommentar\n  backup:\n    enabled: true # an\n    target: sicherung\n    fremd: bleibt\n" +
		"  eigenes: wert\n"
	original := maintenanceConfigHead + "\n" + block + "\nkephalaion:\n  hub: vm\n  collection: x\n"
	writeFile(t, ConfigPath(root), original)

	result, err := EnsureMaintenanceBlock(root)
	if err != nil || !result.Changed || result.Problem != "" {
		t.Fatalf("Ensure: %+v %v", result, err)
	}
	wantAdded := []string{"maintenance.daily", "maintenance.backup.source", "maintenance.backup.git"}
	if !reflect.DeepEqual(result.Added, wantAdded) {
		t.Errorf("Added = %v, erwartet %v", result.Added, wantAdded)
	}
	want := maintenanceConfigHead + "\nmaintenance:\n  # eigener Kommentar\n  backup:\n    enabled: true # an\n    target: sicherung\n" +
		"    fremd: bleibt\n    source: docs\n    git:\n      commit: false\n  eigenes: wert\n  daily:\n    - docs-backup\n" +
		"\nkephalaion:\n  hub: vm\n  collection: x\n"
	if got := readConfigFile(t, root); got != want {
		t.Errorf("Datei:\n%s\nerwartet:\n%s", got, want)
	}
	config, _ := ReadConfig(root)
	if m := config.Maintenance; !m.BackupEnabled || m.BackupTarget != "sicherung" || len(m.Problems) != 0 || config.Kephalaion.Hub != "vm" {
		t.Errorf("gelesen: %+v", config)
	}
}

// Ein explizites enabled: false und eine einzeilige daily-Liste bleiben, wie
// sie sind; ergänzt wird nur, was fehlt.
func TestEnsureMaintenanceUeberschreibtNichts(t *testing.T) {
	block := "maintenance:\n  daily: [docs-backup]\n  backup:\n    enabled: false\n    source: docs\n    target: x\n    git:\n      commit: true\n"
	content := maintenanceConfigHead + block
	updated, added, problem := planMaintenanceBlock(content)
	if updated != content || len(added) != 0 || problem != "" {
		t.Errorf("geändert: %q %v %q", updated, added, problem)
	}
}

// Eine Zuordnung in einer Zeile lässt sich nicht ergänzen: gemeldet, nicht
// geschrieben.
func TestEnsureMaintenanceMeldetNichtZuordenbares(t *testing.T) {
	root := t.TempDir()
	original := maintenanceConfigHead + "maintenance:\n  backup: {enabled: true}\n"
	writeFile(t, ConfigPath(root), original)
	result, err := EnsureMaintenanceBlock(root)
	if err != nil || result.Changed || result.Problem == "" || !strings.Contains(result.Problem, "maintenance.backup") {
		t.Fatalf("Ensure: %+v %v", result, err)
	}
	if readConfigFile(t, root) != original {
		t.Error("Datei geändert")
	}
	if note := DescribeMaintenanceBlock(result, true); !strings.Contains(note, "nicht eindeutig") {
		t.Errorf("Meldung: %q", note)
	}
}

// PlanMaintenanceBlock schreibt nichts und meldet „würde ergänzt“.
func TestPlanMaintenanceBlockSchreibtNicht(t *testing.T) {
	root := t.TempDir()
	writeFile(t, ConfigPath(root), maintenanceConfigHead)
	result, err := PlanMaintenanceBlock(root)
	if err != nil || result.Changed || !reflect.DeepEqual(result.Added, []string{"maintenance"}) {
		t.Fatalf("Plan: %+v %v", result, err)
	}
	if readConfigFile(t, root) != maintenanceConfigHead {
		t.Error("Plan hat geschrieben")
	}
	if note := DescribeMaintenanceBlock(result, true); !strings.Contains(note, "würde ergänzt") {
		t.Errorf("Meldung: %q", note)
	}
}

// Eine Datei mit fremder schema_version wird nicht angefasst.
func TestEnsureMaintenanceFremdeFassung(t *testing.T) {
	root := t.TempDir()
	original := "schema_version: 2\n"
	writeFile(t, ConfigPath(root), original)
	result, err := EnsureMaintenanceBlock(root)
	if err != nil || result.Changed || len(result.Added) != 0 || readConfigFile(t, root) != original {
		t.Errorf("fremde Fassung: %+v %v", result, err)
	}
}

// CRLF bleibt CRLF, auch in den ergänzten Zeilen.
func TestEnsureMaintenanceCRLF(t *testing.T) {
	content := strings.ReplaceAll(maintenanceConfigHead+"maintenance:\n  backup:\n    enabled: true\n", "\n", "\r\n")
	updated, _, problem := planMaintenanceBlock(content)
	if problem != "" {
		t.Fatalf("Problem: %s", problem)
	}
	if strings.Count(updated, "\n") != strings.Count(updated, "\r\n") {
		t.Errorf("gemischte Zeilenenden:\n%q", updated)
	}
}

// Eine neue Konfiguration trägt den Block mit den Vorgaben.
func TestCreateConfigTraegtMaintenance(t *testing.T) {
	root := t.TempDir()
	if err := CreateConfig(root, "."); err != nil {
		t.Fatalf("CreateConfig: %v", err)
	}
	if !strings.Contains(readConfigFile(t, root), maintenanceBlock()) {
		t.Error("Block maintenance fehlt")
	}
	config, err := ReadConfig(root)
	if err != nil || !config.Maintenance.Present || config.Maintenance.BackupEnabled || len(config.Maintenance.Problems) != 0 {
		t.Errorf("gelesen: %+v %v", config.Maintenance, err)
	}
	if result, _ := PlanMaintenanceBlock(root); len(result.Added) != 0 || result.Problem != "" {
		t.Errorf("nach Anlegen fehlt etwas: %+v", result)
	}
}

// maintenanceProject legt ein Projekt mit Git-Verzeichnis und dem Block an.
func maintenanceProject(t *testing.T, repoRoot string, target string) string {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	block := "maintenance:\n  backup:\n    enabled: false\n"
	if target != "" {
		block += "    target: " + target + "\n"
	}
	writeFile(t, ConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: "+repoRoot+"\n  vcs: git\n\n"+block)
	if err := os.MkdirAll(filepath.Join(root, repoRoot, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func resolveTarget(t *testing.T, root string) (MaintenanceTarget, error) {
	t.Helper()
	config, err := ReadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	return ResolveMaintenanceTarget(root, config)
}

func TestResolveMaintenanceTargetGefaehrlicheZiele(t *testing.T) {
	for _, target := range []string{
		".", "./", ".git", ".git/x", ".k-playbook", ".k-playbook/work/b",
		"..", "../anderswo", "/tmp/x", "~/x", "a/../..", "C:\\x",
	} {
		root := maintenanceProject(t, ".", target)
		if _, err := resolveTarget(t, root); err == nil {
			t.Errorf("Ziel %q nicht abgelehnt", target)
		}
	}
	// Ein Vorfahre geschützter Verzeichnisse: repo_root liegt darunter.
	root := maintenanceProject(t, "app", "app/../app")
	if _, err := resolveTarget(t, root); err == nil {
		t.Error("Ziel gleich repo_root nicht abgelehnt")
	}
	// Ziel über dem Hauptverzeichnis, aber im Repo (repo_root: ..).
	root = t.TempDir()
	project := filepath.Join(root, "sub")
	writeFile(t, ConfigPath(project), "schema_version: 3\n\nproject:\n  repo_root: ..\n\nmaintenance:\n  backup:\n    target: ..\n")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveTarget(t, project); err == nil {
		t.Error("Vorfahre des Hauptverzeichnisses nicht abgelehnt")
	}
}

func TestResolveMaintenanceTargetSymlinks(t *testing.T) {
	root := maintenanceProject(t, ".", "verweis/docs")
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "verweis")); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveTarget(t, root); err == nil || !strings.Contains(err.Error(), "nach dem Auswerten der Symlinks nicht unterhalb des Hauptverzeichnisses") {
		t.Errorf("Symlink nach außen: %v", err)
	}

	root = maintenanceProject(t, ".", "verweis/docs")
	if err := os.Symlink(filepath.Join(root, ".git"), filepath.Join(root, "verweis")); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveTarget(t, root); err == nil {
		t.Error("Symlink in .git nicht abgelehnt")
	}

	root = maintenanceProject(t, ".", "verweis/docs")
	if err := os.MkdirAll(filepath.Join(root, "echt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "echt"), filepath.Join(root, "verweis")); err != nil {
		t.Fatal(err)
	}
	target, err := resolveTarget(t, root)
	if err != nil || target.Path != filepath.Join(root, "echt", "docs") || target.Rel != "echt/docs" {
		t.Errorf("Symlink im Repo: %+v %v", target, err)
	}

	root = maintenanceProject(t, ".", "kaputt/docs")
	if err := os.Symlink(filepath.Join(root, "fehlt"), filepath.Join(root, "kaputt")); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveTarget(t, root); err == nil {
		t.Error("ins Leere zeigender Symlink nicht abgelehnt")
	}
}

func TestResolveMaintenanceTargetGueltig(t *testing.T) {
	root := maintenanceProject(t, ".", "")
	target, err := resolveTarget(t, root)
	if err != nil || target.Rel != "keph-backup/docs" || target.Path != filepath.Join(root, "keph-backup", "docs") {
		t.Errorf("Vorgabe: %+v %v", target, err)
	}
}

// Ohne Symlink nennt die Ablehnung keine Symlinks; ein Ziel außerhalb des
// Hauptverzeichnisses bleibt abgelehnt.
func TestResolveMaintenanceTargetMeldungOhneSymlinks(t *testing.T) {
	for _, target := range []string{"../x", "../anderswo/docs"} {
		root := maintenanceProject(t, ".", target)
		_, err := resolveTarget(t, root)
		if err == nil || !strings.Contains(err.Error(), "liegt nicht unterhalb des Hauptverzeichnisses") || strings.Contains(err.Error(), "Auswerten der Symlinks") {
			t.Errorf("Ziel %q: %v", target, err)
		}
	}
}

// omniProject legt die Form an, in der project.repoRoot ein Unter-Repo des
// Hauptverzeichnisses ist: Hauptverzeichnis mit .git, repo_root omni-gw mit
// eigenem .git.
func omniProject(t *testing.T, target string) string {
	t.Helper()
	root := maintenanceProject(t, "omni-gw", target)
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// Liegt project.repoRoot als Unter-Repo im Hauptverzeichnis, ist ein Ziel im
// Hauptverzeichnis neben dem Unter-Repo zulässig; context.docs.exclude bekommt
// dann keinen Eintrag. Liegt das Ziel im Unter-Repo, gibt es ihn relativ zu
// project.repoRoot. project.repoRoot, ihm Übergeordnetes und die .git beider
// Repos bleiben abgelehnt.
func TestResolveMaintenanceTargetUnterRepo(t *testing.T) {
	root := omniProject(t, "")
	target, err := resolveTarget(t, root)
	if err != nil || target.Rel != "keph-backup/docs" || target.Path != filepath.Join(root, "keph-backup", "docs") || target.ProjectDir != root {
		t.Fatalf("Ziel im Hauptverzeichnis: %+v %v", target, err)
	}
	config, _ := ReadConfig(root)
	if entry, ok := maintenanceDocsExclude(root, config); ok {
		t.Errorf("Eintrag außerhalb von project.repoRoot: %q", entry)
	}
	if docs := contextDocs(root, config); len(docs.Exclude) != 0 {
		t.Errorf("context.docs.exclude = %v", docs.Exclude)
	}

	root = omniProject(t, "omni-gw/keph-backup/docs")
	target, err = resolveTarget(t, root)
	if err != nil || target.Rel != "omni-gw/keph-backup/docs" {
		t.Fatalf("Ziel im Unter-Repo: %+v %v", target, err)
	}
	config, _ = ReadConfig(root)
	if entry, ok := maintenanceDocsExclude(root, config); !ok || entry != "/keph-backup/docs/" {
		t.Errorf("Eintrag im Unter-Repo: %q %v", entry, ok)
	}

	for target, want := range map[string]string{
		"omni-gw":          "ist project.repoRoot",
		"omni-gw/.git":     "ist .git",
		"omni-gw/.git/x":   "liegt in .git",
		".git/x":           "liegt in .git",
		"../x":             "nicht unterhalb des Hauptverzeichnisses",
		".k-playbook/x":    "liegt in .k-playbook/",
	} {
		root := omniProject(t, target)
		if _, err := resolveTarget(t, root); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Ziel %q: %v, erwartet %q", target, err, want)
		}
	}
	root = maintenanceProject(t, "sub/app", "sub")
	if _, err := resolveTarget(t, root); err == nil || !strings.Contains(err.Error(), "ist project.repoRoot übergeordnet") {
		t.Errorf("Vorfahre von project.repoRoot: %v", err)
	}
}

// context.docs.exclude trägt das Ziel relativ zu project.repoRoot, verankert —
// auch wenn das Hauptverzeichnis nicht die Wurzel des Repositorys ist —, und
// docs.exclude der Datei bleibt unverändert.
func TestContextDocsExcludeEnthaeltSicherungsziel(t *testing.T) {
	repo := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(repo); err == nil {
		repo = resolved
	}
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(repo, "sub")
	content := "schema_version: 3\n\nproject:\n  repo_root: ..\n  vcs: git\n\ndocs:\n  exclude:\n    - vendor/\n\n" + maintenanceBlock()
	writeFile(t, ConfigPath(projectDir), content)

	config, err := ReadConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	docs := contextDocs(projectDir, config)
	if want := []string{"vendor/", "/sub/keph-backup/docs/"}; !reflect.DeepEqual(docs.Exclude, want) {
		t.Errorf("exclude = %v, erwartet %v", docs.Exclude, want)
	}
	if !reflect.DeepEqual(config.DocsExclude, []string{"vendor/"}) || readConfigFile(t, projectDir) != content {
		t.Error("docs.exclude der Datei verändert")
	}

	// Ohne Block kein Eintrag; ein abgelehntes Ziel auch keiner.
	writeFile(t, ConfigPath(projectDir), "schema_version: 3\n\nproject:\n  repo_root: ..\n")
	config, _ = ReadConfig(projectDir)
	if docs := contextDocs(projectDir, config); len(docs.Exclude) != 0 {
		t.Errorf("ohne Block: %v", docs.Exclude)
	}
	writeFile(t, ConfigPath(projectDir), "schema_version: 3\n\nproject:\n  repo_root: ..\n\nmaintenance:\n  backup:\n    target: .k-playbook\n")
	config, _ = ReadConfig(projectDir)
	if docs := contextDocs(projectDir, config); len(docs.Exclude) != 0 {
		t.Errorf("abgelehntes Ziel: %v", docs.Exclude)
	}
}

// Über BuildContext kommt der Eintrag in die Kontextausgabe.
func TestBuildContextDocsExcludeMitSicherungsziel(t *testing.T) {
	root := maintenanceProject(t, ".", "")
	context, err := BuildContext(root)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if !containsString(context.Docs.Exclude, "/keph-backup/docs/") {
		t.Errorf("context.docs.exclude = %v", context.Docs.Exclude)
	}
}
