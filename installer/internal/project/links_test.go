package project

import (
	"os"
	"path/filepath"
	"testing"
)

// newProject baut ein Zielprojekt mit Installation darin auf.
func newProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, name := range []string{"commands", "skills"} {
		dir := filepath.Join(root, PlaybookDirName, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "k-test.md"), []byte("test\n"), 0o644); err != nil {
			t.Fatalf("Beispieldatei anlegen: %v", err)
		}
	}
	return root
}

func statusFor(t *testing.T, statuses []LinkStatus, path string) LinkStatus {
	t.Helper()

	for _, status := range statuses {
		if status.Path == path {
			return status
		}
	}
	t.Fatalf("kein Status fuer %s", path)
	return LinkStatus{}
}

func TestCheckLinksMeldetFehlend(t *testing.T) {
	root := newProject(t)

	statuses := CheckLinks(root)
	if LinksOK(statuses) {
		t.Fatal("frisches Projekt darf nicht als eingerichtet gelten")
	}
	if got := statusFor(t, statuses, filepath.Join(".claude", "commands")).State; got != StateMissing {
		t.Errorf("State = %q, erwartet %q", got, StateMissing)
	}
}

func TestApplyLinksLegtSymlinksAn(t *testing.T) {
	root := newProject(t)

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if !LinksOK(statuses) {
		t.Fatalf("nach ApplyLinks nicht eingerichtet: %+v", statuses)
	}

	target := filepath.Join(root, ".claude", "commands")
	destination, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if want := filepath.Join("..", PlaybookDirName, "commands"); destination != want {
		t.Errorf("Ziel = %q, erwartet %q", destination, want)
	}

	// Der Link muss auf die Beispieldatei durchgreifen.
	if _, err := os.Stat(filepath.Join(target, "k-test.md")); err != nil {
		t.Errorf("Datei ueber den Link nicht erreichbar: %v", err)
	}
}

func TestApplyLinksIstIdempotent(t *testing.T) {
	root := newProject(t)

	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if !LinksOK(statuses) {
		t.Errorf("zweiter Lauf nicht eingerichtet: %+v", statuses)
	}
}

func TestApplyLinksSetztFalschesZielNeu(t *testing.T) {
	root := newProject(t)

	target := filepath.Join(root, ".claude", "commands")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	if err := os.Symlink("/irgendwo/anders", target); err != nil {
		t.Fatalf("Symlink anlegen: %v", err)
	}

	if got := statusFor(t, CheckLinks(root), filepath.Join(".claude", "commands")).State; got != StateStale {
		t.Errorf("State vorher = %q, erwartet %q", got, StateStale)
	}

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if !LinksOK(statuses) {
		t.Errorf("veralteter Link wurde nicht korrigiert: %+v", statuses)
	}
}

func TestApplyLinksErhaeltEigenesVerzeichnis(t *testing.T) {
	root := newProject(t)

	target := filepath.Join(root, ".claude", "commands")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	eigen := filepath.Join(target, "eigenes.md")
	if err := os.WriteFile(eigen, []byte("projekteigen\n"), 0o644); err != nil {
		t.Fatalf("eigene Datei anlegen: %v", err)
	}

	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}

	// Die projekteigene Datei bleibt eine echte Datei.
	info, err := os.Lstat(eigen)
	if err != nil {
		t.Fatalf("eigene Datei fehlt: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("eigene Datei wurde durch einen Symlink ersetzt")
	}

	// Die Datei aus der Installation ist als Symlink dazugekommen.
	installed := filepath.Join(target, "k-test.md")
	info, err = os.Lstat(installed)
	if err != nil {
		t.Fatalf("Datei aus der Installation fehlt: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Datei aus der Installation ist kein Symlink")
	}
}

func TestApplyLinksLaesstDateiImWegLiegen(t *testing.T) {
	root := newProject(t)

	target := filepath.Join(root, ".claude", "commands")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	if err := os.WriteFile(target, []byte("keine Verlinkung\n"), 0o644); err != nil {
		t.Fatalf("Datei anlegen: %v", err)
	}

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if got := statusFor(t, statuses, filepath.Join(".claude", "commands")).State; got != StateBlocked {
		t.Errorf("State = %q, erwartet %q", got, StateBlocked)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("Datei lesen: %v", err)
	}
	if string(content) != "keine Verlinkung\n" {
		t.Error("Datei wurde veraendert")
	}
}

func TestCheckLinksMeldetFehlendeQuelle(t *testing.T) {
	root := t.TempDir()

	statuses := CheckLinks(root)
	if got := statusFor(t, statuses, filepath.Join(".claude", "commands")).State; got != StateNoSource {
		t.Errorf("State = %q, erwartet %q", got, StateNoSource)
	}
}

func TestApplyLinksBedientAlleAssistenten(t *testing.T) {
	root := newProject(t)

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if !LinksOK(statuses) {
		t.Fatalf("nicht vollstaendig eingerichtet: %+v", statuses)
	}

	// Alle Command-Verzeichnisse zeigen auf dieselbe Quelle in der Installation.
	for _, path := range []string{
		filepath.Join(".claude", "commands"),
		filepath.Join(".opencode", "commands"),
		filepath.Join(".cursor", "commands"),
	} {
		target := filepath.Join(root, path)
		if _, err := os.Stat(filepath.Join(target, "k-test.md")); err != nil {
			t.Errorf("%s: Datei nicht erreichbar: %v", path, err)
		}
	}

	// Skills stehen nur einmal; OpenCode liest .claude/skills mit.
	skillLinks := 0
	for _, status := range statuses {
		if status.Source == filepath.Join(PlaybookDirName, "skills") {
			skillLinks++
		}
	}
	if skillLinks != 1 {
		t.Errorf("%d Skill-Links, erwartet genau einen", skillLinks)
	}
}

// CLAUDE.md zeigt auf AGENTS.md; beide gehoeren dem Projekt.
func TestApplyLinksVerknuepftClaudeMitAgents(t *testing.T) {
	root := newProject(t)
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("# Projektregeln\n"), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}

	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}

	claude := filepath.Join(root, "CLAUDE.md")
	destination, err := os.Readlink(claude)
	if err != nil {
		t.Fatalf("CLAUDE.md ist kein Symlink: %v", err)
	}
	if destination != "AGENTS.md" {
		t.Errorf("Ziel = %q, erwartet %q", destination, "AGENTS.md")
	}

	// Ein Schreibzugriff auf CLAUDE.md muss in AGENTS.md ankommen.
	if err := os.WriteFile(claude, []byte("# Geaendert\n"), 0o644); err != nil {
		t.Fatalf("ueber den Link schreiben: %v", err)
	}
	content, err := os.ReadFile(agents)
	if err != nil {
		t.Fatalf("AGENTS.md lesen: %v", err)
	}
	if string(content) != "# Geaendert\n" {
		t.Errorf("AGENTS.md = %q, Aenderung kam nicht an", content)
	}
}

// Ohne AGENTS.md wird nichts angelegt: die Datei gehoert dem Projekt.
func TestApplyLinksLegtKeineAgentsAn(t *testing.T) {
	root := newProject(t)

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "AGENTS.md")); err == nil {
		t.Error("AGENTS.md wurde angelegt")
	}
	if _, err := os.Lstat(filepath.Join(root, "CLAUDE.md")); err == nil {
		t.Error("CLAUDE.md wurde ohne Quelle angelegt")
	}
	if got := statusFor(t, statuses, "CLAUDE.md").State; got != StateNoSource {
		t.Errorf("State = %q, erwartet %q", got, StateNoSource)
	}
}

// Ein Editor, der "atomar" speichert, ersetzt den Symlink durch eine echte
// Datei. Das muss auffallen, sonst laufen beide still auseinander.
func TestCheckLinksMeldetErsetztenSymlink(t *testing.T) {
	root := newProject(t)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# A\n"), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# eigenstaendig\n"), 0o644); err != nil {
		t.Fatalf("CLAUDE.md anlegen: %v", err)
	}

	if got := statusFor(t, CheckLinks(root), "CLAUDE.md").State; got != StateBlocked {
		t.Errorf("State = %q, erwartet %q", got, StateBlocked)
	}
}
