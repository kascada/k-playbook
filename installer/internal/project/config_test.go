package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSuggestRepoRootFindetRepoImHauptverzeichnis(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf(".git anlegen: %v", err)
	}

	repoRoot, candidates := suggestRepoRoot(root)
	if repoRoot != "." {
		t.Errorf("repoRoot = %q, erwartet %q", repoRoot, ".")
	}
	if len(candidates) != 1 {
		t.Errorf("candidates = %v, erwartet einen Eintrag", candidates)
	}
}

// Das Repo liegt eine Ebene unter dem Hauptverzeichnis, nicht darin selbst.
func TestSuggestRepoRootFindetParallelesRepo(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "G", ".git"), 0o755); err != nil {
		t.Fatalf("G/.git anlegen: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "notizen"), 0o755); err != nil {
		t.Fatalf("Nachbarverzeichnis anlegen: %v", err)
	}

	repoRoot, _ := suggestRepoRoot(root)
	if repoRoot != "G" {
		t.Errorf("repoRoot = %q, erwartet %q", repoRoot, "G")
	}
}

// Bei mehreren Kandidaten wird nichts vorgeschlagen; die Auswahl trifft der Nutzer.
func TestSuggestRepoRootBleibtBeiMehrerenLeer(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"eins", "zwei"} {
		if err := os.MkdirAll(filepath.Join(root, name, ".git"), 0o755); err != nil {
			t.Fatalf("%s/.git anlegen: %v", name, err)
		}
	}

	repoRoot, candidates := suggestRepoRoot(root)
	if repoRoot != "" {
		t.Errorf("repoRoot = %q, erwartet leer", repoRoot)
	}
	if len(candidates) != 2 {
		t.Errorf("candidates = %v, erwartet zwei Einträge", candidates)
	}
}

func TestCreateConfigSchreibtAuffindbareDatei(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf(".git anlegen: %v", err)
	}

	if err := CreateConfig(root, "."); err != nil {
		t.Fatalf("CreateConfig: %v", err)
	}

	content, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("Config lesen: %v", err)
	}
	for _, want := range []string{"schema_version: 3", "repo_root: .", "vcs: git"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("Config enthält %q nicht:\n%s", want, content)
		}
	}

	// Die angelegte Datei muss von Discover gefunden werden.
	found, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover nach CreateConfig: %v", err)
	}
	if want, _ := filepath.EvalSymlinks(root); found != want {
		t.Errorf("Discover = %q, erwartet %q", found, want)
	}
}

func TestCreateConfigSetztVcsNoneOhneRepo(t *testing.T) {
	root := t.TempDir()

	if err := CreateConfig(root, "."); err != nil {
		t.Fatalf("CreateConfig: %v", err)
	}

	content, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("Config lesen: %v", err)
	}
	if !strings.Contains(string(content), "vcs: none") {
		t.Errorf("erwartet vcs: none:\n%s", content)
	}
}

func TestCreateConfigUeberschreibtNicht(t *testing.T) {
	root := t.TempDir()
	original := "schema_version: 2\n# von Hand gepflegt\n"
	writeFile(t, ConfigPath(root), original)

	if err := CreateConfig(root, "."); err == nil {
		t.Fatal("CreateConfig hat eine vorhandene Datei nicht abgelehnt")
	}

	content, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("Config lesen: %v", err)
	}
	if string(content) != original {
		t.Error("vorhandene Config wurde verändert")
	}
}

func TestCreateConfigLehntFehlendesVerzeichnisAb(t *testing.T) {
	if err := CreateConfig(filepath.Join(t.TempDir(), "gibtsnicht"), "."); err == nil {
		t.Error("CreateConfig hat ein fehlendes Verzeichnis akzeptiert")
	}
}

func TestReadConfigLiestProjektwerte(t *testing.T) {
	root := t.TempDir()
	content := `# Kommentar
schema_version: 2

project:
  # noch ein Kommentar
  repo_root: G
  vcs: git
`
	writeFile(t, ConfigPath(root), content)

	config, err := ReadConfig(root)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if config.SchemaVersion != "2" {
		t.Errorf("SchemaVersion = %q, erwartet %q", config.SchemaVersion, "2")
	}
	if config.RepoRoot != "G" {
		t.Errorf("RepoRoot = %q, erwartet %q", config.RepoRoot, "G")
	}
	if config.VCS != "git" {
		t.Errorf("VCS = %q, erwartet %q", config.VCS, "git")
	}

	if got, want := RepoRootDir(root, config), filepath.Join(root, "G"); got != want {
		t.Errorf("RepoRootDir = %q, erwartet %q", got, want)
	}
}

// Ein gleichnamiger Schlüssel in einer anderen Sektion darf nicht durchschlagen.
func TestReadConfigTrenntSektionen(t *testing.T) {
	root := t.TempDir()
	content := `schema_version: 2

andere:
  repo_root: falsch

project:
  repo_root: richtig
`
	writeFile(t, ConfigPath(root), content)

	config, err := ReadConfig(root)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if config.RepoRoot != "richtig" {
		t.Errorf("RepoRoot = %q, erwartet %q", config.RepoRoot, "richtig")
	}
}

func TestGitWorktreeRootFindetRepoAufwaerts(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf(".git anlegen: %v", err)
	}
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("Unterverzeichnis anlegen: %v", err)
	}

	found, ok := gitWorktreeRoot(deep)
	if !ok {
		t.Fatal("kein Repository gefunden")
	}
	if want, _ := filepath.EvalSymlinks(root); found != want {
		t.Errorf("gitWorktreeRoot = %q, erwartet %q", found, want)
	}
}

func TestGitWorktreeRootOhneRepo(t *testing.T) {
	if _, ok := gitWorktreeRoot(t.TempDir()); ok {
		t.Error("Repository gemeldet, obwohl keins vorhanden ist")
	}
}

func TestAddUniqueHaeltReihenfolgeUndUeberspringtDoppelte(t *testing.T) {
	list := addUnique(nil, "a")
	list = addUnique(list, "b")
	list = addUnique(list, "a")
	list = addUnique(list, "")

	if len(list) != 2 || list[0] != "a" || list[1] != "b" {
		t.Errorf("list = %v, erwartet [a b]", list)
	}
}

// Eine neue Konfiguration bringt den Remediation-Standard gleich mit, damit
// nicht erst beim ersten Review entschieden werden muss.
func TestCreateConfigSchreibtRemediationStandard(t *testing.T) {
	root := t.TempDir()

	if err := CreateConfig(root, "."); err != nil {
		t.Fatalf("CreateConfig: %v", err)
	}

	remediation, err := ReadRemediation(root)
	if err != nil {
		t.Fatalf("ReadRemediation: %v", err)
	}
	if !remediation.Configured {
		t.Error("Configured = false, der Block fehlt in der neuen Datei")
	}
	if remediation.Mode != DefaultRemediationMode {
		t.Errorf("Mode = %q, erwartet %q", remediation.Mode, DefaultRemediationMode)
	}
}

func TestCheckSchema(t *testing.T) {
	if err := CheckSchema(Config{SchemaVersion: SchemaVersion}); err != nil {
		t.Errorf("aktuelle Fassung wurde abgelehnt: %v", err)
	}
	if err := CheckSchema(Config{}); err == nil {
		t.Error("fehlende schema_version wurde akzeptiert")
	}
	// Eine ältere Fassung wird abgelehnt: das Projekt ist neu einzurichten.
	if err := CheckSchema(Config{SchemaVersion: "2"}); err == nil {
		t.Error("schema_version 2 wurde akzeptiert")
	}
	if err := CheckSchema(Config{SchemaVersion: "9"}); err == nil {
		t.Error("unbekannte Fassung wurde akzeptiert")
	}
}

// Ist die Datei neuer als das Werkzeug, nennt die Meldung k-playbook upgrade
// als Abhilfe, nicht mehr git pull im Clone (Task 154).
func TestCheckSchemaNeuereFassungNenntUpgrade(t *testing.T) {
	err := CheckSchema(Config{SchemaVersion: "9"})
	if err == nil {
		t.Fatal("neuere Fassung wurde akzeptiert")
	}
	got := err.Error()
	if !strings.Contains(got, "k-playbook upgrade") {
		t.Errorf("Fehler = %q, erwartet den Hinweis auf k-playbook upgrade", got)
	}
	for _, unwanted := range []string{"git pull"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("Fehler = %q, enthält noch %q", got, unwanted)
		}
	}
}

// Eine neu erzeugte Konfiguration muss die eigene Prüfung bestehen.
func TestCreateConfigSchreibtGueltigesSchema(t *testing.T) {
	root := t.TempDir()

	if err := CreateConfig(root, "."); err != nil {
		t.Fatalf("CreateConfig: %v", err)
	}
	config, err := ReadConfig(root)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if err := CheckSchema(config); err != nil {
		t.Errorf("selbst erzeugte Konfiguration ist ungültig: %v", err)
	}
}

// ReadConfig nimmt einem Skalar ein Paar umschließender Anführungszeichen ab
// (Task 191): schema_version: "3" wurde vorher als neuere Fassung abgelehnt
// („hat schema_version "3", dieses Werkzeug versteht 3“).
func TestReadConfigNimmtAnfuehrungszeichenAb(t *testing.T) {
	for _, test := range []struct {
		name, content         string
		schema, vcs, repoRoot string
	}{
		{"doppelte", "schema_version: \"3\"\nproject:\n  repo_root: \"src\"\n  vcs: \"git\"\n", "3", "git", "src"},
		{"einfache", "schema_version: '3'\nproject:\n  repo_root: 'src'\n  vcs: 'git'\n", "3", "git", "src"},
		{"ohne", "schema_version: 3\nproject:\n  repo_root: src\n  vcs: git\n", "3", "git", "src"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, ConfigPath(root), test.content)
			config, err := ReadConfig(root)
			if err != nil {
				t.Fatal(err)
			}
			if config.SchemaVersion != test.schema || config.VCS != test.vcs || config.RepoRoot != test.repoRoot {
				t.Errorf("gelesen: schema_version %q, vcs %q, repo_root %q", config.SchemaVersion, config.VCS, config.RepoRoot)
			}
			if err := CheckSchema(config); err != nil {
				t.Errorf("die Fassung wird abgelehnt: %v", err)
			}
		})
	}
}

// Nur ein Paar zählt: Ein Wert mit einem einzelnen Anführungszeichen, vorn
// oder hinten, oder mit zwei verschiedenen bleibt unverändert — mehr YAML
// versteht ReadConfig nicht, und es rät nicht.
func TestReadConfigLaesstEinzelnesAnfuehrungszeichenStehen(t *testing.T) {
	for _, value := range []string{`"3`, `3"`, `'3`, `"3'`, `"`} {
		root := t.TempDir()
		writeFile(t, ConfigPath(root), "schema_version: "+value+"\nproject:\n  vcs: "+value+"\n")
		config, err := ReadConfig(root)
		if err != nil {
			t.Fatal(err)
		}
		if config.SchemaVersion != value || config.VCS != value {
			t.Errorf("Wert %s: gelesen schema_version %q, vcs %q", value, config.SchemaVersion, config.VCS)
		}
	}
	root := t.TempDir()
	writeFile(t, ConfigPath(root), "schema_version: \"3\n")
	config, err := ReadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckSchema(config); err == nil {
		t.Error("schema_version mit einem einzelnen Anführungszeichen wurde akzeptiert")
	}
}
