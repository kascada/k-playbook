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
// Einzel-Symlinks aus dem aufgeloesten Katalog bestueckt wird — die Sorte steht
// in Kind. Ein Datei-Link ist ein einzelner Symlink auf eine Datei im Projekt;
// dort steht Source statt Kind.
type Link struct {
	// Path ist relativ zur Projektwurzel.
	Path string `json:"path"`
	// Assistant nennt, wer hier liest — nur fuer die Anzeige.
	Assistant string `json:"assistant"`
	// Kind nennt den Katalog hinter einem Verzeichnis. Leer bei Datei-Links.
	Kind AssetKind `json:"kind,omitempty"`
	// Source ist die Quelle eines Datei-Links, relativ zur Projektwurzel.
	Source string `json:"source,omitempty"`
	// IsFile unterscheidet Datei- von Katalog-Links.
	IsFile bool `json:"isFile,omitempty"`
	// Optional markiert Links, deren Quelle dem Projekt gehoert. Fehlt sie, ist
	// nichts zu tun — im Gegensatz zu einer fehlenden Quelle in der
	// Installation, die auf eine beschaedigte Installation hindeutet.
	Optional bool `json:"optional,omitempty"`
}

// Links sind die Verlinkungen, die ein Zielprojekt braucht.
//
// Commands und Skills werden Eintrag fuer Eintrag verlinkt, nicht als
// Verzeichnis. Nur so lassen sich zwei Quellen zusammenfuehren: die
// mitgelieferten aus k-playbook/ und die projekteigenen aus
// k-playbook-local/, wobei ein gleichnamiger projekteigener Eintrag gewinnt.
//
// Skills stehen nur einmal: OpenCode durchsucht neben .opencode/skills/ auch
// .claude/skills/, ein zweiter Ort waere Dopplung. Cursor kennt kein
// Skill-Konzept.
//
// CLAUDE.md zeigt auf AGENTS.md, weil Claude Code ausschliesslich CLAUDE.md
// liest und OpenCode AGENTS.md bevorzugt. Ein Symlink statt eines Imports,
// damit eine Aenderung immer in beiden ankommt — wer in CLAUDE.md schreibt,
// schreibt durch den Link hindurch in AGENTS.md.
func Links() []Link {
	return []Link{
		{Path: filepath.Join(".claude", "commands"), Kind: KindCommands, Assistant: "Claude Code"},
		{Path: filepath.Join(".claude", "skills"), Kind: KindSkills, Assistant: "Claude Code, OpenCode"},
		{Path: filepath.Join(".opencode", "commands"), Kind: KindCommands, Assistant: "OpenCode"},
		{Path: filepath.Join(".cursor", "commands"), Kind: KindCommands, Assistant: "Cursor"},
		{Path: "CLAUDE.md", Source: "AGENTS.md", Assistant: "Claude Code", IsFile: true, Optional: true},
	}
}

// LinkState ist der Zustand einer einzelnen Verlinkung.
type LinkState string

const (
	// StateOK: alles steht so, wie es stehen soll.
	StateOK LinkState = "ok"
	// StateMissing: nichts vorhanden.
	StateMissing LinkState = "missing"
	// StateStale: vorhanden, aber in einer Form, die nicht mehr gilt — ein
	// Datei-Link auf ein falsches Ziel oder der Verzeichnis-Symlink aus einer
	// aelteren Fassung.
	StateStale LinkState = "stale"
	// StateIncomplete: das Verzeichnis steht, sein Inhalt weicht vom Katalog ab.
	StateIncomplete LinkState = "incomplete"
	// StateBlocked: etwas Echtes steht im Weg. Wird nicht angefasst.
	StateBlocked LinkState = "blocked"
	// StateNoSource: es gibt nichts zu verlinken.
	StateNoSource LinkState = "no-source"
)

// LinkStatus ist der gepruefte Zustand einer Verlinkung.
type LinkStatus struct {
	Link
	State  LinkState `json:"state"`
	Detail string    `json:"detail"`
	// Die folgenden Felder gelten nur fuer Katalog-Links und nennen die
	// Eintraege beim Namen, damit die Oberflaeche zeigen kann, was nicht passt.
	Expected int `json:"expected,omitempty"`
	Linked   int `json:"linked,omitempty"`
	// Missing: im Katalog, aber nicht registriert.
	Missing []string `json:"missing,omitempty"`
	// Wrong: registriert, zeigt aber auf die falsche Quelle. Typisch, wenn das
	// Projekt einen mitgelieferten Eintrag neuerdings ueberschreibt.
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
// Quelle zaehlt nicht dazu: dort gibt es nichts zu tun, solange das Projekt die
// Datei nicht selbst anlegt.
func (s LinkStatus) NeedsAction() bool {
	if s.State == StateOK {
		return false
	}
	return !(s.Optional && s.State == StateNoSource)
}

// CheckLinks prueft den Zustand, ohne etwas zu veraendern.
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

// LinkChanges fasst zusammen, was ein Einrichten an der Registrierung aendern
// wuerde — ueber alle Ziele hinweg und ohne Dopplung.
//
// Ohne die Zusammenfassung waere die Zahl irrefuehrend: dieselben Commands
// stehen in .claude/, .opencode/ und .cursor/, ein neuer Command zaehlte also
// dreifach. Gemeint ist aber der Eintrag, nicht seine Kopien.
type LinkChanges struct {
	// Added kommen dazu, Removed fallen weg, Repointed wechseln die Quelle.
	Added     []string `json:"added,omitempty"`
	Removed   []string `json:"removed,omitempty"`
	Repointed []string `json:"repointed,omitempty"`
}

// Empty meldet, ob nichts zu tun waere.
func (c LinkChanges) Empty() bool {
	return len(c.Added)+len(c.Removed)+len(c.Repointed) == 0
}

// PendingLinkChanges liest die Bilanz aus einem Pruefergebnis.
func PendingLinkChanges(statuses []LinkStatus) LinkChanges {
	added, removed, repointed := map[string]bool{}, map[string]bool{}, map[string]bool{}

	for _, status := range statuses {
		// Steht das Zielverzeichnis noch gar nicht, sind die Listen leer und die
		// Zahl saehe nach "nichts zu tun" aus. Der Fall gehoert nicht hierher:
		// gemeint ist die Veraenderung an einer bestehenden Registrierung.
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
	if link.IsFile {
		return checkFileLink(projectRoot, link)
	}
	return checkRegistryLink(projectRoot, link)
}

// checkFileLink prueft einen einzelnen Symlink auf eine Datei im Projekt.
func checkFileLink(projectRoot string, link Link) LinkStatus {
	status := LinkStatus{Link: link}

	source := filepath.Join(projectRoot, link.Source)
	if !fileExists(source) {
		status.State = StateNoSource
		// Instruktionsdateien gehoeren dem Projekt; wir legen keine an.
		status.Detail = link.Source + " fehlt im Projekt"
		return status
	}

	target := filepath.Join(projectRoot, link.Path)
	wanted := relativeSource(target, source)

	info, err := os.Lstat(target)
	switch {
	case err != nil && os.IsNotExist(err):
		status.State = StateMissing
		status.Detail = "nicht vorhanden"

	case err != nil:
		status.State = StateBlocked
		status.Detail = err.Error()

	case info.Mode()&os.ModeSymlink != 0:
		destination, err := os.Readlink(target)
		switch {
		case err != nil:
			status.State = StateStale
			status.Detail = "Ziel nicht lesbar"
		case destination == wanted:
			status.State = StateOK
			status.Detail = "-> " + destination
		default:
			status.State = StateStale
			status.Detail = "zeigt auf " + destination
		}

	case info.IsDir():
		status.State = StateBlocked
		status.Detail = "Verzeichnis steht im Weg"

	default:
		// Typisch nach einem Editor, der "atomar" speichert: er ersetzt den
		// Symlink durch eine echte Datei. Ab dann laufen beide auseinander.
		status.State = StateBlocked
		status.Detail = "echte Datei statt Symlink, Aenderungen erreichen " + link.Source + " nicht"
	}

	return status
}

// checkRegistryLink vergleicht den aufgeloesten Katalog mit dem, was im
// Zielverzeichnis tatsaechlich registriert ist.
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
	// Steht das Zielverzeichnis noch gar nicht, waere die Liste aller Namen nur
	// Laerm — die Zahl sagt alles. Namen nennen wir erst, wenn einzelne
	// Eintraege abweichen.
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
		// immer nur eine Quelle; projekteigene Eintraege kaemen nie an.
		status.State = StateStale
		status.Detail = "Verzeichnis-Symlink aus einer aelteren Fassung, wird durch Einzel-Links ersetzt"
		return status

	case !info.IsDir():
		status.State = StateBlocked
		status.Detail = "Datei steht im Weg"
		return status
	}

	status.Linked, status.Missing, status.Wrong, status.Blocked = compareRegistry(target, wanted)
	status.Stale = staleRegistryLinks(projectRoot, target, wanted)

	// Blockierte Eintraege zaehlen nicht als offener Punkt: dort liegt eine
	// echte Datei des Projekts, die absichtlich gewinnt. Sie bleiben in der
	// Liste sichtbar, aber Einrichten kann und soll daran nichts aendern.
	if len(status.Missing)+len(status.Wrong)+len(status.Stale) == 0 {
		status.State = StateOK
		status.Detail = fmt.Sprintf("%d Eintraege registriert", status.Linked)
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

// compareRegistry prueft je Katalogeintrag, ob der passende Symlink steht.
func compareRegistry(target string, wanted []RegistryEntry) (linked int, missing []string, wrong []string, blocked []string) {
	missing, wrong, blocked = []string{}, []string{}, []string{}

	for _, entry := range wanted {
		linkPath := filepath.Join(target, filepath.FromSlash(entry.Name))

		info, err := os.Lstat(linkPath)
		switch {
		case err != nil:
			missing = append(missing, entry.Name)

		case info.Mode()&os.ModeSymlink == 0:
			// Etwas Echtes mit demselben Namen gehoert dem Projekt und gewinnt.
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
// aber nicht mehr im Katalog stehen. Alles andere im Zielverzeichnis gehoert
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

		// Reihenfolge zaehlt: ein Symlink auf ein Verzeichnis ist ein Eintrag,
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
		if link.IsFile {
			err = applyFileLink(projectRoot, link)
		} else {
			err = applyRegistryLink(projectRoot, link)
		}
		if err != nil {
			return CheckLinks(projectRoot), err
		}
	}

	return CheckLinks(projectRoot), nil
}

// applyFileLink setzt einen einzelnen Symlink. Etwas Echtes im Weg gehoert dem
// Projekt und bleibt liegen; die Pruefung meldet den Zustand.
func applyFileLink(projectRoot string, link Link) error {
	source := filepath.Join(projectRoot, link.Source)
	if !fileExists(source) {
		return nil
	}

	target := filepath.Join(projectRoot, link.Path)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", filepath.Dir(link.Path), err)
	}

	info, err := os.Lstat(target)
	switch {
	case err != nil && os.IsNotExist(err):

	case err != nil:
		return fmt.Errorf("%s pruefen: %w", link.Path, err)

	case info.Mode()&os.ModeSymlink != 0:
		// Neu setzen: ein bestehender Link kann nach einem Umzug ins Leere zeigen.
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("%s ersetzen: %w", link.Path, err)
		}

	default:
		return nil
	}

	if err := os.Symlink(relativeSource(target, source), target); err != nil {
		return fmt.Errorf("%s verlinken: %w", link.Path, err)
	}
	return nil
}

// applyRegistryLink bringt das Zielverzeichnis auf den Stand des Katalogs:
// fehlende Links kommen dazu, falsch zeigende werden neu gesetzt, verwaiste
// verschwinden. Echte Dateien des Projekts bleiben unberuehrt.
func applyRegistryLink(projectRoot string, link Link) error {
	if !RegistrySourcePresent(projectRoot, link.Kind) {
		return nil
	}

	target := filepath.Join(projectRoot, link.Path)
	if err := prepareRegistryDir(target, link.Path); err != nil {
		return err
	}
	if !isDir(target) {
		// Etwas Echtes steht im Weg; die Pruefung meldet es.
		return nil
	}

	wanted := ActiveRegistry(projectRoot, link.Kind)

	// Erst raeumen, dann setzen: ein abgeschalteter Eintrag soll verschwinden,
	// bevor ein gleichnamiger aus der anderen Quelle nachrueckt.
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
				// Projekteigen und damit staerker als der Katalog.
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

// prepareRegistryDir sorgt dafuer, dass am Ziel ein echtes Verzeichnis steht.
// Der Verzeichnis-Symlink aelterer Fassungen weicht dabei.
func prepareRegistryDir(target string, displayPath string) error {
	info, err := os.Lstat(target)
	switch {
	case err != nil && os.IsNotExist(err):

	case err != nil:
		return fmt.Errorf("%s pruefen: %w", displayPath, err)

	case info.Mode()&os.ModeSymlink != 0:
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("%s ersetzen: %w", displayPath, err)
		}

	case info.IsDir():
		return nil

	default:
		// Eine echte Datei gehoert dem Projekt und bleibt liegen.
		return nil
	}

	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", displayPath, err)
	}
	return nil
}

// removeEmptyDirs raeumt Namensraum-Verzeichnisse weg, die nach dem Entfernen
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

// isWithin meldet, ob path in root liegt. Beide Pfade muessen bereinigt sein.
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
