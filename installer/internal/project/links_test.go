package project

import (
	"os"
	"path/filepath"
	"testing"
)

// newProject baut ein Zielprojekt mit Installation darin auf: ein Command, ein
// Command im Namensraum, ein Skill.
func newProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, PlaybookDirName, "commands", "k-test.md"), "test\n")
	writeFile(t, filepath.Join(root, PlaybookDirName, "commands", "_shared", "geteilt.md"), "geteilt\n")
	writeFile(t, filepath.Join(root, PlaybookDirName, "skills", "beispiel", skillFileName), "# Beispiel\n")
	return root
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
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

// claudeCommands ist der Pfad, an dem die meisten Faelle sichtbar werden.
func claudeCommands() string { return filepath.Join(".claude", "commands") }

func TestCheckLinksMeldetFehlend(t *testing.T) {
	root := newProject(t)

	statuses := CheckLinks(root)
	if LinksOK(statuses) {
		t.Fatal("frisches Projekt darf nicht als eingerichtet gelten")
	}

	status := statusFor(t, statuses, claudeCommands())
	if status.State != StateMissing {
		t.Errorf("State = %q, erwartet %q", status.State, StateMissing)
	}
	if status.Expected != 2 {
		t.Errorf("Expected = %d, erwartet beide Commands", status.Expected)
	}
}

func TestApplyLinksLegtEinzelLinksAn(t *testing.T) {
	root := newProject(t)

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if !LinksOK(statuses) {
		t.Fatalf("nach ApplyLinks nicht eingerichtet: %+v", statuses)
	}

	// Das Zielverzeichnis ist ein echtes Verzeichnis, kein Symlink.
	target := filepath.Join(root, ".claude", "commands")
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal(".claude/commands ist ein Verzeichnis-Symlink")
	}

	// Der Command liegt als Einzel-Link darin und greift auf die Quelle durch.
	link := filepath.Join(target, "k-test.md")
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("k-test.md ist kein Symlink: %v", err)
	}
	content, err := os.ReadFile(link)
	if err != nil || string(content) != "test\n" {
		t.Errorf("Inhalt ueber den Link = %q, %v", content, err)
	}

	// Der Namensraum bleibt als Verzeichnis erhalten.
	if _, err := os.Stat(filepath.Join(target, "_shared", "geteilt.md")); err != nil {
		t.Errorf("Namensraum nicht erreichbar: %v", err)
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

// Fassungen bis 0.4 haben das ganze Verzeichnis verlinkt. Damit kaeme nur eine
// Quelle an; der Link muss weichen.
func TestApplyLinksErsetztVerzeichnisSymlink(t *testing.T) {
	root := newProject(t)

	target := filepath.Join(root, ".claude", "commands")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	if err := os.Symlink(filepath.Join("..", PlaybookDirName, "commands"), target); err != nil {
		t.Fatalf("Symlink anlegen: %v", err)
	}

	if got := statusFor(t, CheckLinks(root), claudeCommands()).State; got != StateStale {
		t.Errorf("State vorher = %q, erwartet %q", got, StateStale)
	}

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if !LinksOK(statuses) {
		t.Fatalf("Verzeichnis-Symlink wurde nicht umgebaut: %+v", statuses)
	}

	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("Verzeichnis-Symlink steht noch")
	}
	// Die Quelle darf dabei nicht durch den Link hindurch geleert worden sein.
	if _, err := os.Stat(filepath.Join(root, PlaybookDirName, "commands", "k-test.md")); err != nil {
		t.Errorf("Quelle beschaedigt: %v", err)
	}
}

// Ein projekteigener Command mit gleichem Namen gewinnt gegen den
// mitgelieferten — der Link muss auf die lokale Fassung umgesetzt werden.
func TestApplyLinksZiehtOverrideNach(t *testing.T) {
	root := newProject(t)

	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}

	writeFile(t, filepath.Join(root, LocalDirName, "commands", "k-test.md"), "projekteigen\n")

	status := statusFor(t, CheckLinks(root), claudeCommands())
	if status.State != StateIncomplete {
		t.Fatalf("State = %q, erwartet %q", status.State, StateIncomplete)
	}
	if len(status.Wrong) != 1 || status.Wrong[0] != "k-test.md" {
		t.Errorf("Wrong = %v, erwartet [k-test.md]", status.Wrong)
	}

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if !LinksOK(statuses) {
		t.Fatalf("Override wurde nicht nachgezogen: %+v", statuses)
	}

	content, err := os.ReadFile(filepath.Join(root, ".claude", "commands", "k-test.md"))
	if err != nil {
		t.Fatalf("lesen: %v", err)
	}
	if string(content) != "projekteigen\n" {
		t.Errorf("Inhalt = %q, erwartet die projekteigene Fassung", content)
	}
}

// Ein projekteigener Command ohne Gegenstueck kommt einfach dazu.
func TestApplyLinksRegistriertLokaleCommands(t *testing.T) {
	root := newProject(t)
	writeFile(t, filepath.Join(root, LocalDirName, "commands", "k-eigen.md"), "eigen\n")

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if !LinksOK(statuses) {
		t.Fatalf("nicht eingerichtet: %+v", statuses)
	}

	if got := statusFor(t, statuses, claudeCommands()).Expected; got != 3 {
		t.Errorf("Expected = %d, erwartet 3 (zwei mitgeliefert, einer lokal)", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "commands", "k-eigen.md")); err != nil {
		t.Errorf("lokaler Command nicht registriert: %v", err)
	}
}

// Faellt ein Command aus dem Katalog, muss sein Link verschwinden.
func TestApplyLinksEntferntVerwaisteLinks(t *testing.T) {
	root := newProject(t)

	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	if err := os.Remove(filepath.Join(root, PlaybookDirName, "commands", "k-test.md")); err != nil {
		t.Fatalf("Command entfernen: %v", err)
	}

	status := statusFor(t, CheckLinks(root), claudeCommands())
	if status.State != StateIncomplete {
		t.Fatalf("State = %q, erwartet %q", status.State, StateIncomplete)
	}
	if len(status.Stale) != 1 || status.Stale[0] != "k-test.md" {
		t.Errorf("Stale = %v, erwartet [k-test.md]", status.Stale)
	}

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if !LinksOK(statuses) {
		t.Fatalf("verwaister Link blieb liegen: %+v", statuses)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude", "commands", "k-test.md")); err == nil {
		t.Error("verwaister Link steht noch")
	}
}

// Eine leere projekteigene Datei schaltet den mitgelieferten Command ab.
func TestApplyLinksEntferntAbgeschalteteCommands(t *testing.T) {
	root := newProject(t)

	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	writeFile(t, filepath.Join(root, LocalDirName, "commands", "k-test.md"), "# hier nicht gebraucht\n")

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if !LinksOK(statuses) {
		t.Fatalf("nicht eingerichtet: %+v", statuses)
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude", "commands", "k-test.md")); err == nil {
		t.Error("abgeschalteter Command ist weiterhin registriert")
	}
}

// Eine echte Datei des Projekts an derselben Stelle gewinnt und bleibt liegen.
func TestApplyLinksErhaeltEigeneDatei(t *testing.T) {
	root := newProject(t)

	target := filepath.Join(root, ".claude", "commands")
	eigen := filepath.Join(target, "k-test.md")
	writeFile(t, eigen, "projekteigen\n")

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}

	info, err := os.Lstat(eigen)
	if err != nil {
		t.Fatalf("eigene Datei fehlt: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("eigene Datei wurde durch einen Symlink ersetzt")
	}

	status := statusFor(t, statuses, claudeCommands())
	if len(status.Blocked) != 1 || status.Blocked[0] != "k-test.md" {
		t.Errorf("Blocked = %v, erwartet [k-test.md]", status.Blocked)
	}

	// Eine projekteigene Datei ist ein gewollter Zustand, kein offener Punkt:
	// Einrichten koennte daran nichts aendern.
	if status.State != StateOK {
		t.Errorf("State = %q, erwartet %q", status.State, StateOK)
	}
	if status.NeedsAction() {
		t.Error("projekteigene Datei wird als offener Punkt gemeldet")
	}

	// Der Rest des Katalogs wird trotzdem registriert.
	if _, err := os.Stat(filepath.Join(target, "_shared", "geteilt.md")); err != nil {
		t.Errorf("uebriger Katalog nicht registriert: %v", err)
	}
}

func TestApplyLinksLaesstDateiImWegLiegen(t *testing.T) {
	root := newProject(t)

	target := filepath.Join(root, ".claude", "commands")
	writeFile(t, target, "keine Verlinkung\n")

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if got := statusFor(t, statuses, claudeCommands()).State; got != StateBlocked {
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
	if got := statusFor(t, statuses, claudeCommands()).State; got != StateNoSource {
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

	for _, path := range []string{
		filepath.Join(".claude", "commands"),
		filepath.Join(".opencode", "commands"),
		filepath.Join(".cursor", "commands"),
	} {
		if _, err := os.Stat(filepath.Join(root, path, "k-test.md")); err != nil {
			t.Errorf("%s: Command nicht erreichbar: %v", path, err)
		}
	}

	// Skills stehen nur einmal; OpenCode liest .claude/skills mit.
	skillLinks := 0
	for _, status := range statuses {
		if status.Kind == KindSkills {
			skillLinks++
		}
	}
	if skillLinks != 1 {
		t.Errorf("%d Skill-Links, erwartet genau einen", skillLinks)
	}

	// Ein Skill wird als Verzeichnis verlinkt, nicht Datei fuer Datei.
	skill := filepath.Join(root, ".claude", "skills", "beispiel")
	info, err := os.Lstat(skill)
	if err != nil {
		t.Fatalf("Skill fehlt: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("Skill ist kein Symlink auf das Quellverzeichnis")
	}
	if _, err := os.Stat(filepath.Join(skill, skillFileName)); err != nil {
		t.Errorf("SKILL.md nicht erreichbar: %v", err)
	}
}

// CLAUDE.md zeigt auf AGENTS.md; beide gehoeren dem Projekt.
func TestApplyLinksVerknuepftClaudeMitAgents(t *testing.T) {
	root := newProject(t)
	agents := filepath.Join(root, "AGENTS.md")
	writeFile(t, agents, "# Projektregeln\n")

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
	writeFile(t, filepath.Join(root, "AGENTS.md"), "# A\n")
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "# eigenstaendig\n")

	if got := statusFor(t, CheckLinks(root), "CLAUDE.md").State; got != StateBlocked {
		t.Errorf("State = %q, erwartet %q", got, StateBlocked)
	}
}

// Was ein Update an der Registrierung aendern wuerde, muss den Eintrag zaehlen,
// nicht seine Kopien in .claude/, .opencode/ und .cursor/.
func TestPendingLinkChangesZaehltOhneDopplung(t *testing.T) {
	root := newProject(t)

	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}

	// Ein Update bringt einen Command mit, nimmt einen weg, und das Projekt
	// ueberschreibt einen dritten neuerdings selbst.
	writeFile(t, filepath.Join(root, PlaybookDirName, "commands", "k-neu.md"), "neu\n")
	if err := os.Remove(filepath.Join(root, PlaybookDirName, "commands", "k-test.md")); err != nil {
		t.Fatalf("Command entfernen: %v", err)
	}
	writeFile(t, filepath.Join(root, LocalDirName, "commands", "_shared", "geteilt.md"), "projekteigen\n")

	changes := PendingLinkChanges(CheckLinks(root))
	if len(changes.Added) != 1 || changes.Added[0] != "k-neu.md" {
		t.Errorf("Added = %v, erwartet [k-neu.md]", changes.Added)
	}
	if len(changes.Removed) != 1 || changes.Removed[0] != "k-test.md" {
		t.Errorf("Removed = %v, erwartet [k-test.md]", changes.Removed)
	}
	if len(changes.Repointed) != 1 || changes.Repointed[0] != "_shared/geteilt.md" {
		t.Errorf("Repointed = %v, erwartet [_shared/geteilt.md]", changes.Repointed)
	}

	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if !PendingLinkChanges(CheckLinks(root)).Empty() {
		t.Error("nach dem Einrichten darf nichts offen sein")
	}
}
