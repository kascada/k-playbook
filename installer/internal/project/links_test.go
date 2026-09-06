package project

import (
	"os"
	"path/filepath"
	"strings"
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
	t.Fatalf("kein Status für %s", path)
	return LinkStatus{}
}

// claudeCommands ist der Pfad, an dem die meisten Fälle sichtbar werden.
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
		t.Errorf("Inhalt über den Link = %q, %v", content, err)
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

// Fassungen bis 0.4 haben das ganze Verzeichnis verlinkt. Damit käme nur eine
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
		t.Errorf("Quelle beschädigt: %v", err)
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

// Ein projekteigener Command ohne Gegenstück kommt einfach dazu.
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

// Fällt ein Command aus dem Katalog, muss sein Link verschwinden.
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
	// Einrichten könnte daran nichts ändern.
	if status.State != StateOK {
		t.Errorf("State = %q, erwartet %q", status.State, StateOK)
	}
	if status.NeedsAction() {
		t.Error("projekteigene Datei wird als offener Punkt gemeldet")
	}

	// Der Rest des Katalogs wird trotzdem registriert.
	if _, err := os.Stat(filepath.Join(target, "_shared", "geteilt.md")); err != nil {
		t.Errorf("übriger Katalog nicht registriert: %v", err)
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
		t.Error("Datei wurde verändert")
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
		t.Fatalf("nicht vollständig eingerichtet: %+v", statuses)
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

	// Ein Skill wird als Verzeichnis verlinkt, nicht Datei für Datei.
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

// CLAUDE.md bindet AGENTS.md per Import-Zeile ein; beide gehören dem Projekt.
func TestApplyLinksSchreibtIncludeAufAgents(t *testing.T) {
	root := newProject(t)
	agents := filepath.Join(root, "AGENTS.md")
	writeFile(t, agents, "# Projektregeln\n")

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}

	claude := filepath.Join(root, "CLAUDE.md")
	info, err := os.Lstat(claude)
	if err != nil {
		t.Fatalf("CLAUDE.md fehlt: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("CLAUDE.md ist keine reguläre Datei: %s", info.Mode())
	}
	content := readFile(t, claude)
	if content != claudeIncludeStub() {
		t.Errorf("CLAUDE.md = %q, erwartet den Stub", content)
	}
	if !hasEffectiveInclude(content) {
		t.Error("der Stub trägt keine wirksame Import-Zeile")
	}
	// Der Hinweis über der Import-Zeile: Projektregeln gehören nach AGENTS.md.
	if !strings.Contains(content, "Projektregeln gehören nach\nAGENTS.md") {
		t.Errorf("der Hinweis auf AGENTS.md fehlt im Stub:\n%s", content)
	}
	// Genau ein Import: der Hinweis darf nicht selbst einer sein.
	if strings.Count(content, ClaudeIncludeLine) != 1 {
		t.Errorf("die Import-Zeile steht nicht genau einmal:\n%s", content)
	}

	status := statusFor(t, statuses, "CLAUDE.md")
	if status.State != StateOK {
		t.Errorf("State = %q (%s), erwartet %q", status.State, status.Detail, StateOK)
	}
	if !strings.Contains(status.Detail, ClaudeIncludeLine) || strings.Contains(status.Detail, "eigener Inhalt") {
		t.Errorf("Detail = %q, erwartet den reinen Include", status.Detail)
	}
	if readFile(t, agents) != "# Projektregeln\n" {
		t.Error("AGENTS.md wurde angefasst")
	}
}

// Ein Bestandsprojekt trägt CLAUDE.md noch als Symlink. Der Lesepfad ersetzt
// ihn verlustfrei — der Inhalt steht in AGENTS.md — und benennt die Migration,
// weil sie eine versionierte Datei ändert. Ein zweiter Aufruf schweigt.
func TestHealLinksMigriertSymlinkZurIncludeDatei(t *testing.T) {
	root := newProject(t)
	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	writeFile(t, filepath.Join(root, "AGENTS.md"), "# Projektregeln\n")
	if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatalf("Symlink anlegen: %v", err)
	}

	status := statusFor(t, CheckLinks(root), "CLAUDE.md")
	if status.State != StateStale {
		t.Fatalf("State = %q, erwartet %q", status.State, StateStale)
	}
	if !strings.Contains(status.Detail, "älteren Fassung") || !strings.Contains(status.Detail, ClaudeIncludeLine) {
		t.Errorf("Detailtext benennt die Migration nicht: %s", status.Detail)
	}
	if !status.Fixable() {
		t.Error("die Migration muss heilbar sein")
	}

	repair := HealLinks(root)
	if !repair.Applied || !repair.IncludeMigrated {
		t.Fatalf("Applied = %v, IncludeMigrated = %v, erwartet beides", repair.Applied, repair.IncludeMigrated)
	}
	if !repair.Changed.Empty() {
		t.Errorf("Changed = %+v, die Registrierung war unverändert", repair.Changed)
	}
	if len(repair.Open) != 0 {
		t.Errorf("Open = %+v, erwartet nichts Offenes", repair.Open)
	}

	claude := filepath.Join(root, "CLAUDE.md")
	if info, err := os.Lstat(claude); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("CLAUDE.md ist nach der Migration keine reguläre Datei: %v", err)
	}
	if readFile(t, claude) != claudeIncludeStub() {
		t.Errorf("CLAUDE.md = %q, erwartet den Stub", readFile(t, claude))
	}
	if readFile(t, filepath.Join(root, "AGENTS.md")) != "# Projektregeln\n" {
		t.Error("AGENTS.md wurde bei der Migration angefasst")
	}

	// Danach ist nichts mehr zu melden — sonst erschiene die Migration bei
	// jedem Aufruf erneut.
	second := HealLinks(root)
	if !second.Quiet() {
		t.Errorf("zweiter Lauf meldet noch etwas: %+v", second)
	}
	if second.IncludeMigrated {
		t.Error("zweiter Lauf meldet die Migration erneut")
	}
}

// Die Toleranz: Projektinhalt neben dem Include gehört dem Projekt, bleibt
// unangetastet und ist kein Konflikt.
func TestCheckLinksToleriertProjektinhaltNebenInclude(t *testing.T) {
	root := newProject(t)
	writeFile(t, filepath.Join(root, "AGENTS.md"), "# Projektregeln\n")
	eigen := "# Für Claude Code\n\nSiehe " + ClaudeIncludeLine + " für alles Weitere.\n\n## Hausregeln\n\nPlan-Modus unter src/.\n"
	writeFile(t, filepath.Join(root, "CLAUDE.md"), eigen)

	status := statusFor(t, CheckLinks(root), "CLAUDE.md")
	if status.State != StateOK {
		t.Fatalf("State = %q (%s), erwartet %q", status.State, status.Detail, StateOK)
	}
	if !strings.Contains(status.Detail, "eigener Inhalt") {
		t.Errorf("Detail = %q, erwartet den Hinweis auf eigenen Inhalt", status.Detail)
	}

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if !LinksOK(statuses) {
		t.Errorf("nicht eingerichtet: %+v", statuses)
	}
	if readFile(t, filepath.Join(root, "CLAUDE.md")) != eigen {
		t.Error("der Projektinhalt neben dem Include wurde verändert")
	}
	if HealLinks(root).Applied {
		t.Error("an einer eingerichteten Include-Datei darf nichts angewendet werden")
	}
}

// Ein Vorkommen in Backticks oder in einem Code-Block überliest Claude Code
// beim Import-Parsing. Es lädt nichts und gilt deshalb nicht als eingerichtet.
func TestIncludeInBackticksOderCodeBlockGiltNicht(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"allein auf einer Zeile", ClaudeIncludeLine + "\n", true},
		{"der Stub", claudeIncludeStub(), true},
		{"mitten im Satz", "Lies " + ClaudeIncludeLine + " zuerst.\n", true},
		{"mit Satzzeichen dahinter", "Lies " + ClaudeIncludeLine + ".\n", false},
		{"in Backticks", "Die Zeile `" + ClaudeIncludeLine + "` gehört hierher.\n", false},
		{"im Code-Block", "```\n" + ClaudeIncludeLine + "\n```\n", false},
		{"im Code-Block mit Sprache", "```markdown\n" + ClaudeIncludeLine + "\n```\n", false},
		{"im Tilde-Block", "~~~\n" + ClaudeIncludeLine + "\n~~~\n", false},
		{"nach dem Code-Block", "```\nBeispiel\n```\n\n" + ClaudeIncludeLine + "\n", true},
		{"Backticks und wirksam", "Nicht `" + ClaudeIncludeLine + "` in Backticks, sondern " + ClaudeIncludeLine + "\n", true},
		{"unpaariger Backtick", "Ein ` Backtick, dann " + ClaudeIncludeLine + "\n", true},
		{"anderes Ziel", "@docs/AGENTS.md\n", false},
		{"leer", "", false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := hasEffectiveInclude(testCase.content); got != testCase.want {
				t.Errorf("hasEffectiveInclude = %v, erwartet %v für %q", got, testCase.want, testCase.content)
			}
		})
	}

	// Am Dateisystem: ein unwirksamer Include neben AGENTS.md ist der Konflikt.
	root := newProject(t)
	writeFile(t, filepath.Join(root, "AGENTS.md"), "# A\n")
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "```\n"+ClaudeIncludeLine+"\n```\n")
	if got := statusFor(t, CheckLinks(root), "CLAUDE.md").State; got != StateConflict {
		t.Errorf("State = %q, erwartet %q", got, StateConflict)
	}
}

// Kein Include ins Leere: fehlt AGENTS.md, wird auch dann kein Stub
// geschrieben, wenn ein alter Symlink zu ersetzen wäre. Der Zustand bleibt
// StateNoSource, und ein zweiter HealLinks-Lauf meldet kein erneutes Applied.
func TestHealLinksSchreibtKeinenIncludeOhneAgents(t *testing.T) {
	root := newProject(t)
	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatalf("Symlink anlegen: %v", err)
	}

	status := statusFor(t, CheckLinks(root), "CLAUDE.md")
	if status.State != StateNoSource {
		t.Fatalf("State = %q (%s), erwartet %q", status.State, status.Detail, StateNoSource)
	}
	if !strings.Contains(status.Detail, "wartet auf sein Ziel") {
		t.Errorf("Detail = %q, erwartet den wartenden Symlink", status.Detail)
	}
	if status.Fixable() || status.NeedsAction() {
		t.Error("ohne AGENTS.md ist nichts heilbar und nichts offen")
	}

	for run := 1; run <= 2; run++ {
		repair := HealLinks(root)
		if repair.Applied {
			t.Errorf("Lauf %d: Applied trotz fehlendem AGENTS.md", run)
		}
		if !repair.Quiet() {
			t.Errorf("Lauf %d: meldet %+v", run, repair)
		}
	}
	if _, err := os.Readlink(filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Errorf("der Symlink wurde ohne Ziel ersetzt: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "AGENTS.md")); err == nil {
		t.Error("AGENTS.md wurde angelegt")
	}
}

// Die Restlage: der Stub steht, AGENTS.md fehlt. Optional nimmt den Zustand
// aus NeedsAction(), die Karte meldet die Verlinkung deshalb als eingerichtet —
// der Detailtext muss sagen, dass der Import ins Leere zeigt.
func TestCheckLinksBenenntIncludeInsLeere(t *testing.T) {
	root := newProject(t)
	writeFile(t, filepath.Join(root, "CLAUDE.md"), claudeIncludeStub())

	status := statusFor(t, CheckLinks(root), "CLAUDE.md")
	if status.State != StateNoSource {
		t.Fatalf("State = %q (%s), erwartet %q", status.State, status.Detail, StateNoSource)
	}
	for _, phrase := range []string{"fehlt im Projekt", "ins Leere", "lädt daraus nichts"} {
		if !strings.Contains(status.Detail, phrase) {
			t.Errorf("Detailtext nennt %q nicht: %s", phrase, status.Detail)
		}
	}
	if status.NeedsAction() {
		t.Error("ohne AGENTS.md gibt es auf dem Lesepfad nichts zu tun")
	}
	if HealLinks(root).Applied && statusFor(t, CheckLinks(root), "CLAUDE.md").State != StateNoSource {
		t.Error("HealLinks hat an der Restlage etwas geändert")
	}
}

// Ohne AGENTS.md wird nichts angelegt: die Datei gehört dem Projekt.
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

// Eine echte CLAUDE.md ohne wirksamen Include neben AGENTS.md ist ein Konflikt:
// ob ihr Inhalt für alle Assistenten gilt oder nur für Claude Code, entscheidet
// das Projekt. Der Detailtext muss beide Auswege nennen, sonst leitet er in
// einem der Fälle falsch an — und nichts wird angefasst.
func TestCheckLinksMeldetEchteDateiOhneInclude(t *testing.T) {
	root := newProject(t)
	writeFile(t, filepath.Join(root, "AGENTS.md"), "# A\n")
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "# eigenständig\n")

	status := statusFor(t, CheckLinks(root), "CLAUDE.md")
	if status.State != StateConflict {
		t.Errorf("State = %q, erwartet %q", status.State, StateConflict)
	}
	for _, phrase := range []string{
		"keine wirksame Import-Zeile",
		"nach AGENTS.md übernehmen",
		"vor den vorhandenen Inhalt setzen",
		"sieht Claude Code den Anstoß nicht",
	} {
		if !strings.Contains(status.Detail, phrase) {
			t.Errorf("Detailtext nennt %q nicht: %s", phrase, status.Detail)
		}
	}
	if strings.Contains(status.Detail, "Editor") {
		t.Errorf("Detailtext nennt noch die abgelöste Ursache: %s", status.Detail)
	}

	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}
	if readFile(t, filepath.Join(root, "CLAUDE.md")) != "# eigenständig\n" {
		t.Error("die eigenständige CLAUDE.md wurde angefasst")
	}
}

// Was ein Update an der Registrierung ändern würde, muss den Eintrag zählen,
// nicht seine Kopien in .claude/, .opencode/ und .cursor/.
func TestPendingLinkChangesZaehltOhneDopplung(t *testing.T) {
	root := newProject(t)

	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}

	// Ein Update bringt einen Command mit, nimmt einen weg, und das Projekt
	// überschreibt einen dritten neuerdings selbst.
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

// Der Fall, um den es geht: in der Installation wurde ein Command umbenannt.
// Der alte Link zeigt danach ins Leere, der neue fehlt — und keiner der beiden
// Zustände fällt jemandem auf, solange nur die Karte ihn zeigt.
func TestHealLinksZiehtUmbenanntenCommandNach(t *testing.T) {
	root := newProject(t)
	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}

	commands := filepath.Join(PlaybookDir(root), "commands")
	if err := os.Rename(filepath.Join(commands, "k-test.md"), filepath.Join(commands, "k-task-test.md")); err != nil {
		t.Fatalf("umbenennen: %v", err)
	}

	repair := HealLinks(root)
	if !repair.Applied {
		t.Fatal("eine heilbare Abweichung muss angewendet werden")
	}
	if got := repair.Changed.Added; len(got) != 1 || got[0] != "k-task-test.md" {
		t.Errorf("Added = %v, erwartet [k-task-test.md]", got)
	}
	if got := repair.Changed.Removed; len(got) != 1 || got[0] != "k-test.md" {
		t.Errorf("Removed = %v, erwartet [k-test.md]", got)
	}
	if len(repair.Open) != 0 {
		t.Errorf("Open = %v, erwartet nichts Offenes", repair.Open)
	}

	if !LinksOK(CheckLinks(root)) {
		t.Error("nach der Heilung muss die Verlinkung stehen")
	}
	if _, err := os.Lstat(filepath.Join(root, claudeCommands(), "k-test.md")); !os.IsNotExist(err) {
		t.Error("der verwaiste Link muss verschwunden sein")
	}
	// Stat statt Lstat: der Link muss auch etwas treffen.
	if _, err := os.Stat(filepath.Join(root, claudeCommands(), "k-task-test.md")); err != nil {
		t.Errorf("neuer Link zeigt ins Leere: %v", err)
	}
}

func TestHealLinksSchweigtWennAllesSteht(t *testing.T) {
	root := newProject(t)
	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}

	repair := HealLinks(root)
	if repair.Applied {
		t.Error("ohne Abweichung darf nicht geschrieben werden")
	}
	if !repair.Quiet() {
		t.Errorf("ohne Abweichung gibt es nichts zu melden, war %+v", repair)
	}
}

// Was das Einrichten nicht auflösen kann, darf es auch nicht bei jedem Aufruf
// versuchen: sonst schriebe der Lesepfad in einem blockierten Projekt endlos
// dieselben Links neu.
func TestHealLinksWendetNichtsAnWennNichtsHeilbarIst(t *testing.T) {
	root := newProject(t)
	if _, err := ApplyLinks(root); err != nil {
		t.Fatalf("ApplyLinks: %v", err)
	}

	blocked := filepath.Join(root, ".cursor", "commands")
	if err := os.RemoveAll(blocked); err != nil {
		t.Fatalf("Zielverzeichnis entfernen: %v", err)
	}
	writeFile(t, blocked, "echte Datei des Projekts\n")

	repair := HealLinks(root)
	if repair.Applied {
		t.Error("ein blockiertes Ziel darf kein Anwenden auslösen")
	}
	if len(repair.Open) != 1 || repair.Open[0].State != StateBlocked {
		t.Errorf("Open = %+v, erwartet ein blockiertes Ziel", repair.Open)
	}
	if repair.Quiet() {
		t.Error("ein offener Punkt muss gemeldet werden")
	}
}
