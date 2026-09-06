package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Link beschreibt einen Ort, an dem ein Assistent liest.
//
// Es gibt zwei Sorten. Ein Katalog-Link ist ein Verzeichnis, das mit
// Einzel-Symlinks aus dem aufgelösten Katalog bestückt wird — die Sorte steht
// in Kind. Eine Include-Datei ist eine erzeugte reguläre Datei, die eine andere
// Datei im Projekt per Import-Zeile einbindet; dort steht Source statt Kind.
type Link struct {
	// Path ist relativ zur Projektwurzel.
	Path string `json:"path"`
	// Assistant nennt, wer hier liest — nur für die Anzeige.
	Assistant string `json:"assistant"`
	// Kind nennt den Katalog hinter einem Verzeichnis. Leer bei Include-Dateien.
	Kind AssetKind `json:"kind,omitempty"`
	// Source ist die Datei, die eine Include-Datei einbindet, relativ zur
	// Projektwurzel.
	Source string `json:"source,omitempty"`
	// IsInclude unterscheidet die Include-Datei von Katalog-Links.
	IsInclude bool `json:"isInclude,omitempty"`
	// Optional markiert Links, deren Quelle dem Projekt gehört. Fehlt sie, ist
	// nichts zu tun — im Gegensatz zu einer fehlenden Quelle in der
	// Installation, die auf eine beschädigte Installation hindeutet.
	Optional bool `json:"optional,omitempty"`
}

// Links sind die Verlinkungen, die ein Zielprojekt braucht.
//
// Commands und Skills werden Eintrag für Eintrag verlinkt, nicht als
// Verzeichnis. Nur so lassen sich zwei Quellen zusammenführen: die
// mitgelieferten aus k-playbook/ und die projekteigenen aus
// k-playbook-local/, wobei ein gleichnamiger projekteigener Eintrag gewinnt.
//
// Skills stehen nur einmal: OpenCode durchsucht neben .opencode/skills/ auch
// .claude/skills/, ein zweiter Ort wäre Dopplung. Cursor kennt kein
// Skill-Konzept.
//
// CLAUDE.md ist eine Include-Datei mit der Import-Zeile @AGENTS.md, weil Claude
// Code ausschließlich CLAUDE.md liest und OpenCode wie Cursor AGENTS.md
// bevorzugen. Früher war es ein Symlink: der hielt beide Dateien zwangsläufig
// gleich, brach aber bei jedem Editor, der beim Speichern atomar ersetzt, und
// die Prüfung meldete danach „echte Datei statt Symlink". Der Import kennt
// diesen Fall nicht. Sein Preis: zwei Dateien, die auseinanderlaufen können —
// was Claude Code über /memory oder # nach CLAUDE.md schreibt, erreicht die
// anderen Assistenten nicht, und die Prüfung deckt das als StateOK zu. Das ist
// bewusst in Kauf genommen; die Gegenrichtung hieße, jeden Projektinhalt neben
// dem Include zum Konflikt zu erklären. Als Gegengewicht sagt der erzeugte Stub
// über der Import-Zeile, dass Projektregeln nach AGENTS.md gehören.
//
// Die Richtung ist überall dieselbe. Bringt ein Projekt nur eine echte
// CLAUDE.md mit, wird sie beim Einrichten nach AGENTS.md umbenannt statt
// ignoriert; sonst stünden zwei Instruktionsdateien mit verschiedenem Inhalt
// nebeneinander. Eine echte CLAUDE.md mit wirksamem Include gilt als
// eingerichtet, auch mit Projektinhalt daneben. Was sich nicht automatisch
// auflösen lässt — eine bewusst gesetzte Verlinkung auf ein fremdes Ziel, zwei
// echte Dateien ohne Include, ein git-ignoriertes AGENTS.md — wird als
// StateConflict gemeldet und nicht angefasst. Die Fallmatrix dazu steht in
// instructions_layout.go.
//
// Die Include-Datei ist Optional, weil ihre Quelle dem Projekt gehört.
// ApplyAssistantSetup legt AGENTS.md zwar immer an, der Lesepfad ruft HealLinks
// aber direkt und legt nie eines an. Dort bleibt es bei StateNoSource:
// Fixable() schließt den Zustand aus, aus NeedsAction() nimmt ihn allein
// Optional heraus. Ohne das Flag meldete `k-playbook context` in jedem Projekt
// ohne AGENTS.md dauerhaft einen offenen, nicht heilbaren Punkt.
func Links() []Link {
	return []Link{
		{Path: filepath.Join(".claude", "commands"), Kind: KindCommands, Assistant: "Claude Code"},
		{Path: filepath.Join(".claude", "skills"), Kind: KindSkills, Assistant: "Claude Code, OpenCode"},
		{Path: filepath.Join(".opencode", "commands"), Kind: KindCommands, Assistant: "OpenCode"},
		{Path: filepath.Join(".cursor", "commands"), Kind: KindCommands, Assistant: "Cursor"},
		{Path: ClaudeInstructionsFile, Source: RootInstructionsFile, Assistant: "Claude Code", IsInclude: true, Optional: true},
	}
}

// LinkState ist der Zustand einer einzelnen Verlinkung.
type LinkState string

const (
	// StateOK: alles steht so, wie es stehen soll.
	StateOK LinkState = "ok"
	// StateMissing: nichts vorhanden.
	StateMissing LinkState = "missing"
	// StateStale: vorhanden, aber in einer Bauform, die nicht mehr gilt und
	// verlustfrei ersetzt wird — der Symlink CLAUDE.md -> AGENTS.md aus einer
	// älteren Fassung, der zur Include-Datei wird, oder der Verzeichnis-Symlink
	// eines Katalog-Links, der zu Einzel-Links wird. Kein eigener Zustand für
	// die Migration: er zöge Fixable(), NeedsAction(), Kontextausgabe und
	// Oberfläche mit, für den Gewinn eines Textes. Die Fallunterscheidung
	// steht im Detail.
	StateStale LinkState = "stale"
	// StateIncomplete: das Verzeichnis steht, sein Inhalt weicht vom Katalog ab.
	StateIncomplete LinkState = "incomplete"
	// StateBlocked: etwas Echtes steht im Weg. Wird nicht angefasst.
	StateBlocked LinkState = "blocked"
	// StateNoSource: es gibt nichts zu verlinken.
	StateNoSource LinkState = "no-source"
	// StateConflict: die Lage lässt sich nicht automatisch auflösen, ohne
	// Inhalt zu verlieren oder zu verdoppeln. Anders als StateBlocked steht
	// nicht bloß etwas im Weg — es gibt zwei plausible Auflösungen, und die
	// Wahl gehört dem Projekt. Der Detailtext nennt sie.
	StateConflict LinkState = "conflict"
)

// LinkStatus ist der geprüfte Zustand einer Verlinkung.
type LinkStatus struct {
	Link
	State  LinkState `json:"state"`
	Detail string    `json:"detail"`
	// Die folgenden Felder gelten nur für Katalog-Links und nennen die
	// Einträge beim Namen, damit die Oberfläche zeigen kann, was nicht passt.
	Expected int `json:"expected,omitempty"`
	Linked   int `json:"linked,omitempty"`
	// Missing: im Katalog, aber nicht registriert.
	Missing []string `json:"missing,omitempty"`
	// Wrong: registriert, zeigt aber auf die falsche Quelle. Typisch, wenn das
	// Projekt einen mitgelieferten Eintrag neuerdings überschreibt.
	Wrong []string `json:"wrong,omitempty"`
	// Stale: registriert, steht aber nicht mehr im Katalog — entfernt oder
	// projekteigen abgeschaltet.
	Stale []string `json:"stale,omitempty"`
	// Blocked: eine echte Datei des Projekts steht an der Stelle. Sie gewinnt
	// und bleibt liegen.
	Blocked []string `json:"blocked,omitempty"`
}

// OK meldet, ob die Verlinkung steht.
func (s LinkStatus) OK() bool { return s.State == StateOK }

// NeedsAction meldet, ob noch etwas einzurichten ist. Ein optionaler Link ohne
// Quelle zählt nicht dazu: dort gibt es nichts zu tun, solange das Projekt die
// Datei nicht selbst anlegt.
//
// Die Ausnahme gilt nur bei StateNoSource und wird bewusst nicht ausgeweitet:
// ein Konflikt ist ein offener Punkt, auch am optionalen Link CLAUDE.md. Sonst
// bliebe LinksOK dabei true, und die Oberfläche meldete „eingerichtet".
func (s LinkStatus) NeedsAction() bool {
	if s.State == StateOK {
		return false
	}
	return !(s.Optional && s.State == StateNoSource)
}

// CheckLinks prüft den Zustand, ohne etwas zu verändern.
func CheckLinks(projectRoot string) []LinkStatus {
	statuses := make([]LinkStatus, 0, len(Links()))
	for _, link := range Links() {
		statuses = append(statuses, checkLink(projectRoot, link))
	}
	return statuses
}

// LinksOK meldet, ob nichts mehr einzurichten ist.
func LinksOK(statuses []LinkStatus) bool {
	for _, status := range statuses {
		if status.NeedsAction() {
			return false
		}
	}
	return len(statuses) > 0
}

// Fixable meldet, ob Einrichten an diesem Zustand etwas ändern kann.
//
// Blockiert, im Konflikt oder ohne Quelle kann es das nicht: dort liegt eine
// echte Datei des Projekts, es stehen zwei Auflösungen zur Wahl, oder es gibt
// nichts zu verlinken. Auf dem Lesepfad zählt der Unterschied, denn dort wird
// nur angewendet, wenn Anwenden auch etwas bewirkt. Ohne die Unterscheidung
// schriebe jeder Aufruf in einem blockierten Projekt dieselben Links neu.
func (s LinkStatus) Fixable() bool {
	switch s.State {
	case StateMissing, StateStale:
		return true
	case StateIncomplete:
		return len(s.Missing)+len(s.Wrong)+len(s.Stale) > 0
	default:
		return false
	}
}

// LinksFixable meldet, ob mindestens ein Ziel durch Einrichten besser wird.
func LinksFixable(statuses []LinkStatus) bool {
	for _, status := range statuses {
		if status.Fixable() {
			return true
		}
	}
	return false
}

// LinkChanges fasst zusammen, was ein Einrichten an der Registrierung ändern
// würde — über alle Ziele hinweg und ohne Dopplung.
//
// Ohne die Zusammenfassung wäre die Zahl irreführend: dieselben Commands
// stehen in .claude/, .opencode/ und .cursor/, ein neuer Command zählte also
// dreifach. Gemeint ist aber der Eintrag, nicht seine Kopien.
type LinkChanges struct {
	// Added kommen dazu, Removed fallen weg, Repointed wechseln die Quelle.
	Added     []string `json:"added,omitempty"`
	Removed   []string `json:"removed,omitempty"`
	Repointed []string `json:"repointed,omitempty"`
}

// Empty meldet, ob nichts zu tun wäre.
func (c LinkChanges) Empty() bool {
	return len(c.Added)+len(c.Removed)+len(c.Repointed) == 0
}

// PendingLinkChanges liest die Bilanz aus einem Prüfergebnis.
func PendingLinkChanges(statuses []LinkStatus) LinkChanges {
	added, removed, repointed := map[string]bool{}, map[string]bool{}, map[string]bool{}

	for _, status := range statuses {
		// Steht das Zielverzeichnis noch gar nicht, sind die Listen leer und die
		// Zahl sähe nach "nichts zu tun" aus. Der Fall gehört nicht hierher:
		// gemeint ist die Veränderung an einer bestehenden Registrierung.
		for _, name := range status.Missing {
			added[name] = true
		}
		for _, name := range status.Stale {
			removed[name] = true
		}
		for _, name := range status.Wrong {
			repointed[name] = true
		}
	}

	return LinkChanges{
		Added:     sortedKeys(added),
		Removed:   sortedKeys(removed),
		Repointed: sortedKeys(repointed),
	}
}

func sortedKeys(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}

	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func checkLink(projectRoot string, link Link) LinkStatus {
	if link.IsInclude {
		return checkIncludeFile(projectRoot, link)
	}
	return checkRegistryLink(projectRoot, link)
}

// checkIncludeFile prüft die Include-Datei am Inhalt, nicht an der Bauform:
// wirksamer Include vorhanden heißt eingerichtet, gleichgültig was daneben
// steht.
//
// Die Reihenfolge ist festgelegt: Konflikt, Blockade, fehlende Quelle,
// Zielzustand. Käme die Quellprüfung zuerst, bliebe der Konflikt gerade dort
// unsichtbar, wo er am meisten zählt — bei einem CLAUDE.md, das auf ein fremdes
// Ziel zeigt, während AGENTS.md fehlt. Der Detailtext nennte dann mit
// „AGENTS.md fehlt im Projekt" die falsche Ursache. Und fehlt AGENTS.md,
// gewinnt StateNoSource auch über den Migrationszweig: sonst wäre der Zustand
// Fixable(), applyIncludeFile schriebe aber nichts, und HealLinks meldete bei
// jedem `k-playbook context` erneut Applied.
func checkIncludeFile(projectRoot string, link Link) LinkStatus {
	status := LinkStatus{Link: link}

	switch plan := classifyInstructions(projectRoot); {
	case plan.conflict:
		status.State = StateConflict
		status.Detail = plan.detail
		return status

	case plan.blocked:
		status.State = StateBlocked
		status.Detail = plan.detail
		return status
	}

	target := filepath.Join(projectRoot, link.Path)
	source := filepath.Join(projectRoot, link.Source)
	if !fileExists(source) {
		status.State = StateNoSource
		status.Detail = noSourceDetail(target, link)
		return status
	}

	info, err := os.Lstat(target)
	switch {
	case err != nil && os.IsNotExist(err):
		status.State = StateMissing
		status.Detail = "nicht vorhanden"

	case err != nil:
		status.State = StateBlocked
		status.Detail = err.Error()

	case info.Mode()&os.ModeSymlink != 0:
		// Ein Symlink ist die ältere Bauform. Zeigt er auf AGENTS.md, ist das
		// die Migration; alles andere ist ein Rest — ein Fremdlink mit
		// vorhandenem Ziel wäre oben als Konflikt herausgefallen.
		status.State = StateStale
		destination, err := os.Readlink(target)
		switch {
		case err != nil:
			status.Detail = "Symlink mit unlesbarem Ziel, wird durch die Include-Datei ersetzt"
		case destination == relativeSource(target, source):
			status.Detail = "Symlink auf " + link.Source + " aus einer älteren Fassung, wird durch eine Include-Datei mit " +
				ClaudeIncludeLine + " ersetzt"
		default:
			status.Detail = "toter Symlink auf " + destination + ", wird durch die Include-Datei ersetzt"
		}

	case info.IsDir():
		status.State = StateBlocked
		status.Detail = "Verzeichnis steht im Weg"

	case info.Mode().IsRegular():
		data, err := os.ReadFile(target)
		switch {
		case err != nil:
			status.State = StateBlocked
			status.Detail = err.Error()
		case hasEffectiveInclude(string(data)):
			status.State = StateOK
			status.Detail = "Include-Datei, bindet " + link.Source + " mit " + ClaudeIncludeLine + " ein"
			if strings.TrimSpace(string(data)) != strings.TrimSpace(claudeIncludeStub()) {
				status.Detail += "; daneben eigener Inhalt"
			}
		default:
			// Eine echte Datei ohne wirksamen Include neben AGENTS.md ist der
			// Konflikt aus der Fallmatrix; hier nur, falls beide Prüfungen
			// je auseinanderliefen.
			status.State = StateConflict
			status.Detail = bothRealDetail()
		}

	default:
		status.State = StateBlocked
		status.Detail = "weder Datei noch Verzeichnis noch Symlink (" + info.Mode().String() + ")"
	}

	return status
}

// noSourceDetail benennt die Lage ohne AGENTS.md. Instruktionsdateien gehören
// dem Projekt; der Link-Mechanismus legt keine an.
//
// Steht die Include-Datei schon, zeigt ihr Import ins Leere: Optional nimmt
// den Zustand aus NeedsAction(), LinksOK bleibt true, die Assistenten-Karte
// meldet die Verlinkung als eingerichtet — und Claude Code lädt still nichts.
// Beim Symlink war derselbe Endzustand wenigstens als kaputter Link sichtbar;
// hier muss der Text es sagen.
func noSourceDetail(target string, link Link) string {
	base := link.Source + " fehlt im Projekt"

	info, err := os.Lstat(target)
	switch {
	case err != nil:
		return base

	case info.Mode()&os.ModeSymlink != 0:
		return base + "; der Symlink " + link.Path + " wartet auf sein Ziel und wird beim Einrichten durch die Include-Datei ersetzt"

	case info.Mode().IsRegular():
		data, err := os.ReadFile(target)
		if err == nil && hasEffectiveInclude(string(data)) {
			return base + "; " + link.Path + " steht schon als Include-Datei und bindet mit " + ClaudeIncludeLine +
				" ins Leere ein — Claude Code lädt daraus nichts, bis " + link.Source + " angelegt ist (Einrichten legt es an)"
		}
		return base
	}
	return base
}

// checkRegistryLink vergleicht den aufgelösten Katalog mit dem, was im
// Zielverzeichnis tatsächlich registriert ist.
func checkRegistryLink(projectRoot string, link Link) LinkStatus {
	status := LinkStatus{Link: link}

	if !RegistrySourcePresent(projectRoot, link.Kind) {
		status.State = StateNoSource
		status.Detail = string(link.Kind) + "/ fehlt in Installation und Projekt"
		return status
	}

	wanted := ActiveRegistry(projectRoot, link.Kind)
	status.Expected = len(wanted)

	target := filepath.Join(projectRoot, link.Path)
	info, err := os.Lstat(target)
	switch {
	// Steht das Zielverzeichnis noch gar nicht, wäre die Liste aller Namen nur
	// Lärm — die Zahl sagt alles. Namen nennen wir erst, wenn einzelne
	// Einträge abweichen.
	case err != nil && os.IsNotExist(err):
		status.State = StateMissing
		status.Detail = fmt.Sprintf("nicht vorhanden, %d einzurichten", len(wanted))
		return status

	case err != nil:
		status.State = StateBlocked
		status.Detail = err.Error()
		return status

	case info.Mode()&os.ModeSymlink != 0:
		// Fassungen bis 0.4 haben das ganze Verzeichnis verlinkt. Damit gilt
		// immer nur eine Quelle; projekteigene Einträge kämen nie an.
		status.State = StateStale
		status.Detail = "Verzeichnis-Symlink aus einer älteren Fassung, wird durch Einzel-Links ersetzt"
		return status

	case !info.IsDir():
		status.State = StateBlocked
		status.Detail = "Datei steht im Weg"
		return status
	}

	status.Linked, status.Missing, status.Wrong, status.Blocked = compareRegistry(target, wanted)
	status.Stale = staleRegistryLinks(projectRoot, target, wanted)

	// Blockierte Einträge zählen nicht als offener Punkt: dort liegt eine
	// echte Datei des Projekts, die absichtlich gewinnt. Sie bleiben in der
	// Liste sichtbar, aber Einrichten kann und soll daran nichts ändern.
	if len(status.Missing)+len(status.Wrong)+len(status.Stale) == 0 {
		status.State = StateOK
		status.Detail = fmt.Sprintf("%d Einträge registriert", status.Linked)
		if n := len(status.Blocked); n > 0 {
			status.Detail += fmt.Sprintf(", %d projekteigen", n)
		}
		return status
	}

	status.State = StateIncomplete
	status.Detail = registryDetail(status)
	return status
}

// registryDetail fasst die Abweichung in einem Satz zusammen; die Namen stehen
// in den Listen daneben.
func registryDetail(status LinkStatus) string {
	parts := []string{}
	if n := len(status.Missing); n > 0 {
		parts = append(parts, fmt.Sprintf("%d fehlen", n))
	}
	if n := len(status.Wrong); n > 0 {
		parts = append(parts, fmt.Sprintf("%d zeigen woandershin", n))
	}
	if n := len(status.Stale); n > 0 {
		parts = append(parts, fmt.Sprintf("%d verwaist", n))
	}
	if n := len(status.Blocked); n > 0 {
		parts = append(parts, fmt.Sprintf("%d projekteigen", n))
	}
	return strings.Join(parts, ", ")
}

// compareRegistry prüft je Katalogeintrag, ob der passende Symlink steht.
func compareRegistry(target string, wanted []RegistryEntry) (linked int, missing []string, wrong []string, blocked []string) {
	missing, wrong, blocked = []string{}, []string{}, []string{}

	for _, entry := range wanted {
		linkPath := filepath.Join(target, filepath.FromSlash(entry.Name))

		info, err := os.Lstat(linkPath)
		switch {
		case err != nil:
			missing = append(missing, entry.Name)

		case info.Mode()&os.ModeSymlink == 0:
			// Etwas Echtes mit demselben Namen gehört dem Projekt und gewinnt.
			blocked = append(blocked, entry.Name)

		default:
			destination, err := os.Readlink(linkPath)
			if err != nil || destination != relativeSource(linkPath, entry.Path) {
				wrong = append(wrong, entry.Name)
				continue
			}
			linked++
		}
	}
	return linked, missing, wrong, blocked
}

// staleRegistryLinks findet Symlinks, die in eine unserer Quellen zeigen, dort
// aber nicht mehr im Katalog stehen. Alles andere im Zielverzeichnis gehört
// dem Projekt und wird nicht bewertet.
func staleRegistryLinks(projectRoot string, target string, wanted []RegistryEntry) []string {
	inCatalog := map[string]bool{}
	for _, entry := range wanted {
		inCatalog[entry.Name] = true
	}

	stale := []string{}
	for _, name := range ownedLinks(projectRoot, target, "") {
		if !inCatalog[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	return stale
}

// ownedLinks sammelt rekursiv die Symlinks, die in die Installation oder ins
// lokale Verzeichnis zeigen — also die, die von uns stammen.
func ownedLinks(projectRoot string, dir string, prefix string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	sources := []string{PlaybookDir(projectRoot), LocalDir(projectRoot)}
	names := []string{}

	for _, entry := range entries {
		name := entry.Name()
		key := name
		if prefix != "" {
			key = prefix + "/" + name
		}

		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}

		// Reihenfolge zählt: ein Symlink auf ein Verzeichnis ist ein Eintrag,
		// kein Verzeichnis zum Hineinlaufen.
		if info.Mode()&os.ModeSymlink != 0 {
			destination, err := os.Readlink(path)
			if err != nil {
				continue
			}
			resolved := destination
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(dir, resolved)
			}
			resolved = filepath.Clean(resolved)

			for _, source := range sources {
				if isWithin(resolved, source) {
					names = append(names, key)
					break
				}
			}
			continue
		}

		if info.IsDir() {
			names = append(names, ownedLinks(projectRoot, path, key)...)
		}
	}
	return names
}

// ApplyLinks richtet die Verlinkung ein und meldet den Zustand danach.
func ApplyLinks(projectRoot string) ([]LinkStatus, error) {
	for _, link := range Links() {
		var err error
		if link.IsInclude {
			err = applyIncludeFile(projectRoot, link)
		} else {
			err = applyRegistryLink(projectRoot, link)
		}
		if err != nil {
			return CheckLinks(projectRoot), err
		}
	}

	return CheckLinks(projectRoot), nil
}

// LinkIssue ist ein Ziel, an dem nach dem Einrichten noch etwas zu tun ist.
// Knapp gehalten, weil es in der Kontextausgabe landet, die am Anfang jedes
// Commands gelesen wird.
type LinkIssue struct {
	Path   string    `json:"path"`
	State  LinkState `json:"state"`
	Detail string    `json:"detail"`
}

// LinkRepair ist das Ergebnis einer Selbstheilung.
type LinkRepair struct {
	// Applied meldet, ob eingerichtet wurde. Ohne heilbare Abweichung bleibt es
	// beim Lesen.
	Applied bool `json:"applied,omitempty"`
	// Changed nennt die Einträge, die dazukamen, wegfielen oder die Quelle
	// wechselten. Bei einem Ziel, das es vorher gar nicht gab, bleibt sie leer:
	// dort hat sich keine Registrierung verändert, dort ist eine entstanden.
	Changed LinkChanges `json:"changed,omitempty"`
	// IncludeMigrated: CLAUDE.md war ein Symlink auf AGENTS.md und ist jetzt
	// die Include-Datei. Das ist die eine Stelle, an der der Lesepfad eine
	// versionierte Datei im Hauptverzeichnis ändert — git zeigt den
	// Modewechsel 120000 → 100644 —, und deshalb wird sie eigens benannt. Sie
	// kommt nur einmal vor: danach ist die Datei StateOK und heilt nichts mehr.
	IncludeMigrated bool `json:"includeMigrated,omitempty"`
	// Open sind die Ziele, die danach noch offen sind — blockiert, im Konflikt
	// oder am Einrichten gescheitert.
	Open []LinkIssue `json:"open,omitempty"`
	// Error nennt den Grund, wenn das Einrichten abgebrochen ist. Auf dem
	// Lesepfad ist das kein Fehler des Aufrufs: gelesen wird trotzdem.
	Error string `json:"error,omitempty"`
	// registryApplied: mindestens ein Katalog-Link war heilbar. Nur für den
	// Hinweis auf die laufende Sitzung — die Migration der Include-Datei
	// allein ändert an der Command-Liste eines Assistenten nichts.
	registryApplied bool
}

// Quiet meldet, ob es nichts zu berichten gibt.
func (r LinkRepair) Quiet() bool {
	return !r.Applied && len(r.Open) == 0 && r.Error == ""
}

// HealLinks bringt die Registrierung von sich aus auf den Stand des Katalogs
// und meldet, was danach offen bleibt.
//
// Der Aufruf gehört auf den Lesepfad, nicht nur hinter einen Knopf. Welche
// Links gelten, hängt am Projekt — an seinem Katalog aus mitgelieferten und
// projekteigenen Einträgen —, nicht an der Installation. Wie die Installation
// zu ihrem Stand kam, ist dabei gleichgültig: ein Update über die Oberfläche,
// ein Ziel im Makefile, ein Clone von Hand. Hinge das Nachziehen an einem
// dieser Wege, bliebe die Registrierung bei allen anderen stehen, und zwar
// unbemerkt — der umbenannte Command fehlt, der alte Link zeigt ins Leere, und
// beides sieht nur, wer zufällig die Assistenten-Karte öffnet.
//
// Geschrieben wird nur, wenn Schreiben etwas ändert. Steht alles, bleibt es bei
// den Vergleichen; was das Einrichten ohnehin nicht auflösen kann — eine echte
// Projektdatei im Weg, ein Konflikt an CLAUDE.md — führt nicht zu einem
// Anwenden, das jedes Mal dieselben Links neu schriebe.
//
// Ein Symlink CLAUDE.md -> AGENTS.md ist StateStale und damit Fixable(): der
// Lesepfad ersetzt ihn durch die Include-Datei, jedes `k-playbook context` in
// einem Bestandsprojekt migriert also von selbst. Weil das eine versionierte
// Datei im Hauptverzeichnis ändert, steht es als IncludeMigrated im Ergebnis.
// Scheitert das Schreiben — etwa an einem nicht beschreibbaren
// Projektverzeichnis —, landet der Grund in Error, und gelesen wird trotzdem.
func HealLinks(projectRoot string) LinkRepair {
	statuses := CheckLinks(projectRoot)
	if !LinksFixable(statuses) {
		return LinkRepair{Open: openIssues(statuses)}
	}

	// Die Bilanz stammt aus dem Zustand davor: danach ist sie per Definition
	// leer, und genau sie ist das, was der Nutzer lesen will.
	changed := PendingLinkChanges(statuses)
	migrating := false
	registry := false
	for _, status := range statuses {
		switch {
		case status.IsInclude:
			migrating = status.State == StateStale
		case status.Fixable():
			registry = true
		}
	}

	after, err := ApplyLinks(projectRoot)
	repair := LinkRepair{Applied: true, Changed: changed, Open: openIssues(after), registryApplied: registry}
	if err != nil {
		repair.Error = err.Error()
	}
	// Migriert ist erst, was danach als Include-Datei steht. Blieb der Symlink
	// stehen, steht der Grund in Error, und die Datei bleibt StateStale.
	for _, status := range after {
		if status.IsInclude && migrating && status.State == StateOK {
			repair.IncludeMigrated = true
		}
	}
	return repair
}

// openIssues sind die Ziele, an denen noch etwas zu tun ist.
func openIssues(statuses []LinkStatus) []LinkIssue {
	issues := []LinkIssue{}
	for _, status := range statuses {
		if !status.NeedsAction() {
			continue
		}
		issues = append(issues, LinkIssue{Path: status.Path, State: status.State, Detail: status.Detail})
	}
	if len(issues) == 0 {
		return nil
	}
	return issues
}

// applyIncludeFile schreibt die Include-Datei. Eine echte Datei wird nie
// überschrieben: mit wirksamem Include ist sie eingerichtet, ohne ist sie der
// Konflikt aus der Fallmatrix. Ein alter Symlink weicht vorher — der Inhalt
// steht in AGENTS.md, das Ersetzen ist verlustfrei.
//
// Kein Include ins Leere: fehlt AGENTS.md, wird nichts geschrieben. Im
// Einrichten-Pfad kommt das nicht vor, dort legt ApplyAssistantSetup die Datei
// vorher an. HealLinks ruft ApplyLinks dagegen direkt; dort bleibt die
// Include-Datei bei StateNoSource, bis das Projekt ein AGENTS.md hat.
func applyIncludeFile(projectRoot string, link Link) error {
	// Ein gemeldeter Konflikt bleibt unangetastet — auch der Fall, in dem
	// CLAUDE.md auf ein fremdes Ziel zeigt und sonst unten in den
	// Symlink-Zweig fiele und still ersetzt würde.
	if plan := classifyInstructions(projectRoot); plan.conflict || plan.blocked {
		return nil
	}

	source := filepath.Join(projectRoot, link.Source)
	if !fileExists(source) {
		return nil
	}

	target := filepath.Join(projectRoot, link.Path)
	info, err := os.Lstat(target)
	switch {
	case err != nil && os.IsNotExist(err):

	case err != nil:
		return fmt.Errorf("%s prüfen: %w", link.Path, err)

	case info.Mode()&os.ModeSymlink != 0:
		// Migration von der älteren Bauform, oder ein Rest-Link ins Leere.
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("%s ersetzen: %w", link.Path, err)
		}

	default:
		return nil
	}

	if err := os.WriteFile(target, []byte(claudeIncludeStub()), 0o644); err != nil {
		return fmt.Errorf("%s schreiben: %w", link.Path, err)
	}
	return nil
}

// applyRegistryLink bringt das Zielverzeichnis auf den Stand des Katalogs:
// fehlende Links kommen dazu, falsch zeigende werden neu gesetzt, verwaiste
// verschwinden. Echte Dateien des Projekts bleiben unberührt.
func applyRegistryLink(projectRoot string, link Link) error {
	if !RegistrySourcePresent(projectRoot, link.Kind) {
		return nil
	}

	target := filepath.Join(projectRoot, link.Path)
	if err := prepareRegistryDir(target, link.Path); err != nil {
		return err
	}
	if !isDir(target) {
		// Etwas Echtes steht im Weg; die Prüfung meldet es.
		return nil
	}

	wanted := ActiveRegistry(projectRoot, link.Kind)

	// Erst räumen, dann setzen: ein abgeschalteter Eintrag soll verschwinden,
	// bevor ein gleichnamiger aus der anderen Quelle nachrückt.
	for _, name := range staleRegistryLinks(projectRoot, target, wanted) {
		if err := os.Remove(filepath.Join(target, filepath.FromSlash(name))); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("verwaisten Link %s entfernen: %w", name, err)
		}
	}

	for _, entry := range wanted {
		linkPath := filepath.Join(target, filepath.FromSlash(entry.Name))
		if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
			return fmt.Errorf("%s anlegen: %w", filepath.Dir(entry.Name), err)
		}

		if info, err := os.Lstat(linkPath); err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				// Projekteigen und damit stärker als der Katalog.
				continue
			}
			if err := os.Remove(linkPath); err != nil {
				return fmt.Errorf("%s ersetzen: %w", entry.Name, err)
			}
		}

		if err := os.Symlink(relativeSource(linkPath, entry.Path), linkPath); err != nil {
			return fmt.Errorf("%s verlinken: %w", entry.Name, err)
		}
	}

	removeEmptyDirs(target)
	return nil
}

// prepareRegistryDir sorgt dafür, dass am Ziel ein echtes Verzeichnis steht.
// Der Verzeichnis-Symlink älterer Fassungen weicht dabei.
func prepareRegistryDir(target string, displayPath string) error {
	info, err := os.Lstat(target)
	switch {
	case err != nil && os.IsNotExist(err):

	case err != nil:
		return fmt.Errorf("%s prüfen: %w", displayPath, err)

	case info.Mode()&os.ModeSymlink != 0:
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("%s ersetzen: %w", displayPath, err)
		}

	case info.IsDir():
		return nil

	default:
		// Eine echte Datei gehört dem Projekt und bleibt liegen.
		return nil
	}

	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", displayPath, err)
	}
	return nil
}

// removeEmptyDirs räumt Namensraum-Verzeichnisse weg, die nach dem Entfernen
// verwaister Links nichts mehr enthalten. Alles mit Inhalt bleibt.
func removeEmptyDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		removeEmptyDirs(path)

		if rest, err := os.ReadDir(path); err == nil && len(rest) == 0 {
			os.Remove(path)
		}
	}
}

// isWithin meldet, ob path in root liegt. Beide Pfade müssen bereinigt sein.
func isWithin(path string, root string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(os.PathSeparator))
}

// relativeSource bildet den Symlink-Wert relativ zum Verzeichnis des Links,
// damit das Projekt als Ganzes verschiebbar bleibt.
func relativeSource(target string, source string) string {
	relative, err := filepath.Rel(filepath.Dir(target), source)
	if err != nil {
		return source
	}
	return relative
}
