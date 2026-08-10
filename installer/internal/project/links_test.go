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
