package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckLocalMeldetFehlendeStruktur(t *testing.T) {
	root := t.TempDir()

	statuses := CheckLocal(root)
	if LocalOK(statuses) {
		t.Fatal("leeres Projekt gilt als vollständig")
	}
	for _, status := range statuses {
		if status.Present {
			t.Errorf("%s als vorhanden gemeldet", status.Path)
		}
	}
}

func TestCreateLocalLegtStrukturAn(t *testing.T) {
	root := t.TempDir()

	statuses, err := CreateLocal(root)
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	if !LocalOK(statuses) {
		t.Fatalf("nach CreateLocal unvollständig: %+v", statuses)
	}

	local := LocalDir(root)
	for _, name := range []string{"reviews", "results", "guidelines", "priv"} {
		if !isDir(filepath.Join(local, name)) {
			t.Errorf("%s fehlt", name)
		}
	}
	// Projekteigene Commands und Skills liegen in .k-playbook/ (Task 129);
	// angelegt wird weder der alte noch der neue Ort.
	for _, dir := range []string{
		filepath.Join(local, "commands"),
		filepath.Join(local, "skills"),
		ProjectAssetDir(root, KindCommands),
		ProjectAssetDir(root, KindSkills),
	} {
		if pathExists(dir) {
			t.Errorf("%s angelegt", dir)
		}
	}
	if !fileExists(filepath.Join(local, InstructionsFileName)) {
		t.Errorf("%s fehlt", InstructionsFileName)
	}
	if !fileExists(filepath.Join(local, VersionSourcesFileName)) {
		t.Errorf("%s fehlt", VersionSourcesFileName)
	}
}

// fileTemplate fällt ohne eigenen Zweig auf den neutralen default:-Zweig
// zurück, der aus dem Zweck des Eintrags eine Markdown-Überschrift baut. Die
// Quellenkonfiguration bekäme dann einen Markdown-Rumpf statt einer gültigen,
// leeren Konfiguration — und ihr Leser bräche beim ersten Lauf ab.
func TestCreateLocalLegtVersionsquellenAlsGueltigeKonfigurationAn(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	path := filepath.Join(LocalDir(root), VersionSourcesFileName)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s lesen: %v", VersionSourcesFileName, err)
	}
	text := string(content)
	if strings.HasPrefix(text, "# "+VersionSourcesFileName) {
		t.Fatalf("%s trägt den neutralen Rumpf des default:-Zweigs:\n%s", VersionSourcesFileName, text)
	}
	for _, want := range []string{"schema_version: 1", "roots: []", "sources: []", "version-inventory.md"} {
		if !strings.Contains(text, want) {
			t.Errorf("%s enthält %q nicht:\n%s", VersionSourcesFileName, want, text)
		}
	}
}

// Git speichert keine leeren Verzeichnisse; ohne README wären sie nach einem
// Clone des Projekts verschwunden.
func TestCreateLocalLegtReadmesAn(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	readme := filepath.Join(LocalDir(root), "reviews", "README.md")
	content, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("README fehlt: %v", err)
	}
	if !strings.Contains(string(content), "Review-Rezepte") {
		t.Errorf("README ohne Zweckbeschreibung:\n%s", content)
	}
}

// removedLocalEntries sind die elf Einträge, die nicht mehr zur Struktur
// gehören: docs bis material hat Task 107 herausgenommen, ihr Inhalt liegt in
// der Projekt-Collection; inbox, queue, knowledge und cache waren die
// Wissensablage auf der Platte, die Task 123 entfernt hat; rules ist die
// Sorte, die Task 135 entfernt hat, checks die lokale Ebene der Checks, die
// Task 134 entfernt hat.
var removedLocalEntries = []string{"docs", "tasks", "tasks/done", "data", "material", "inbox", "queue", "knowledge", "cache", "rules", "checks"}

// Das Einrichten legt die elf Einträge nicht mehr an und zählt sie nicht als
// fehlend; ein Projekt ohne sie ist vollständig. Auch cache/ entsteht nicht
// nebenbei.
func TestCreateLocalLegtEntfernteEintraegeNichtAn(t *testing.T) {
	inStructure := map[string]bool{}
	for _, entry := range LocalStructure() {
		inStructure[filepath.ToSlash(entry.Path)] = true
	}
	for _, name := range removedLocalEntries {
		if inStructure[name] {
			t.Errorf("%s steht noch in LocalStructure", name)
		}
	}

	root := t.TempDir()
	statuses, err := CreateLocal(root)
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	if !LocalOK(statuses) {
		t.Fatalf("ohne die entfernten Einträge unvollständig: %+v", statuses)
	}
	for _, name := range removedLocalEntries {
		if pathExists(filepath.Join(LocalDir(root), filepath.FromSlash(name))) {
			t.Errorf("%s wurde beim Einrichten angelegt", name)
		}
	}
}

// Ein Bestandsprojekt, in dem die alten Verzeichnisse noch liegen, bleibt
// vollständig, und CreateLocal lässt sie stehen: abgeräumt wird allein über
// /k-upgrade, nach Bestätigung.
func TestCreateLocalLaesstAlteVerzeichnisseStehen(t *testing.T) {
	root := t.TempDir()
	for _, name := range removedLocalEntries {
		readme := filepath.Join(LocalDir(root), filepath.FromSlash(name), "README.md")
		if err := os.MkdirAll(filepath.Dir(readme), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(readme, []byte("# alt\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	statuses, err := CreateLocal(root)
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	if !LocalOK(statuses) {
		t.Fatalf("Bestandsprojekt unvollständig: %+v", statuses)
	}
	for _, status := range statuses {
		for _, name := range removedLocalEntries {
			if filepath.ToSlash(status.Path) == name {
				t.Errorf("CheckLocal meldet den entfernten Eintrag %s", name)
			}
		}
	}
	for _, name := range removedLocalEntries {
		content, err := os.ReadFile(filepath.Join(LocalDir(root), filepath.FromSlash(name), "README.md"))
		if err != nil || string(content) != "# alt\n" {
			t.Errorf("%s/README.md verändert: %q, %v", name, content, err)
		}
	}
}

// Die Zwecktexte der übrigen Einträge zeigen nicht auf entfernte Verzeichnisse;
// sie stehen in den READMEs, die das Einrichten schreibt.
func TestLocalStructureZweckZeigtNichtAufEntfernteEintraege(t *testing.T) {
	for _, entry := range LocalStructure() {
		for _, name := range removedLocalEntries {
			if strings.Contains(entry.Purpose, "../"+name+"/") {
				t.Errorf("%s nennt ../%s/:\n%s", entry.Path, name, entry.Purpose)
			}
		}
	}
}

func TestCreateLocalUeberschreibtNichts(t *testing.T) {
	root := t.TempDir()
	local := LocalDir(root)

	// Ein Projekt, das schon Inhalte hat.
	if err := os.MkdirAll(filepath.Join(local, "reviews"), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	eigen := filepath.Join(local, "reviews", "README.md")
	if err := os.WriteFile(eigen, []byte("# eigene Beschreibung\n"), 0o644); err != nil {
		t.Fatalf("README anlegen: %v", err)
	}
	instructions := filepath.Join(local, InstructionsFileName)
	if err := os.WriteFile(instructions, []byte("# eigene Projektregeln\n"), 0o644); err != nil {
		t.Fatalf("%s anlegen: %v", InstructionsFileName, err)
	}

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	for path, want := range map[string]string{
		eigen:        "# eigene Beschreibung\n",
		instructions: "# eigene Projektregeln\n",
	} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s lesen: %v", path, err)
		}
		if string(content) != want {
			t.Errorf("%s wurde verändert: %q", path, content)
		}
	}
}

func TestCreateLocalIstIdempotent(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	statuses, err := CreateLocal(root)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if !LocalOK(statuses) {
		t.Errorf("zweiter Lauf unvollständig: %+v", statuses)
	}
}

// Die Overlay-Sorte reviews muss ein lokales Gegenstück haben, sonst greift
// die Overlay-Auflösung ins Leere. checks hat seit Task 134 keine lokale
// Ebene mehr.
func TestLocalStructureDecktOverlaySortenAb(t *testing.T) {
	vorhanden := map[string]bool{}
	for _, entry := range LocalStructure() {
		vorhanden[entry.Path] = true
	}

	for _, kind := range []string{"reviews"} {
		if !vorhanden[kind] {
			t.Errorf("Overlay-Sorte %s fehlt in der lokalen Struktur", kind)
		}
	}
}

// priv/ wird angelegt wie jedes andere Verzeichnis. Dass sein Inhalt oft
// privat bleiben soll, steht in seiner README — entschieden wird es vom
// Projekt, nicht von k-playbook.
func TestCreateLocalLegtPrivateVerzeichnisseAn(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	for _, name := range []string{"priv"} {
		dir := filepath.Join(LocalDir(root), name)
		if !isDir(dir) {
			t.Errorf("%s/ fehlt", name)
			continue
		}
		readme := filepath.Join(dir, "README.md")
		if !fileExists(readme) {
			t.Errorf("%s/README.md fehlt", name)
			continue
		}
		content, err := os.ReadFile(readme)
		if err != nil {
			t.Fatalf("%s/README.md lesen: %v", name, err)
		}
		if !strings.Contains(string(content), ".gitignore") {
			t.Errorf("%s/README.md erklärt den .gitignore-Weg nicht:\n%s", name, content)
		}
	}
}

// CreateLocal schreibt eine .gitignore nur für Einträge mit PrivateByDefault.
// Für alle anderen gilt weiterhin: was versioniert wird, entscheidet allein das
// Projekt.
func TestCreateLocalSchreibtGitignoreNurFuerVorbelegteEintraege(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	for _, entry := range LocalStructure() {
		if entry.IsFile || entry.PrivateByDefault {
			continue
		}
		if pathExists(filepath.Join(LocalDir(root), entry.Path, PrivateIgnoreFile)) {
			t.Errorf("%s hat eine .gitignore, die CreateLocal nicht schreiben darf", entry.Path)
		}
	}
}

// Ein frisch angelegter vorbelegter Eintrag bekommt genau den verwalteten
// Inhalt — nicht mehr und nicht weniger.
func TestCreateLocalLegtVorbelegteGitignoreAn(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	vorbelegt := 0
	for _, entry := range LocalStructure() {
		if !entry.PrivateByDefault {
			continue
		}
		vorbelegt++
		if !entry.Private {
			t.Errorf("%s ist vorbelegt, aber nicht als privat geführt", entry.Path)
		}
		ignore := filepath.Join(LocalDir(root), entry.Path, PrivateIgnoreFile)
		if !hasManagedContent(ignore) {
			content, _ := os.ReadFile(ignore)
			t.Errorf("%s trägt nicht den verwalteten Inhalt:\n%s", entry.Path, content)
		}
	}
	if vorbelegt == 0 {
		t.Fatalf("kein vorbelegter Eintrag in der Struktur")
	}
}

// Zweiter Lauf über ein bestehendes Verzeichnis ohne .gitignore: nichts wird
// geschrieben. Sonst käme der Default nach jedem makePublic() still zurück, und
// Bestandsprojekte mit getrackten Dateien landeten in PrivacyPartial.
func TestCreateLocalBringtEntfernteGitignoreNichtZurueck(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	for _, entry := range LocalStructure() {
		if !entry.PrivateByDefault {
			continue
		}
		if err := os.Remove(filepath.Join(LocalDir(root), entry.Path, PrivateIgnoreFile)); err != nil {
			t.Fatalf("%s/.gitignore entfernen: %v", entry.Path, err)
		}
	}

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal, zweiter Lauf: %v", err)
	}

	for _, entry := range LocalStructure() {
		if !entry.PrivateByDefault {
			continue
		}
		if pathExists(filepath.Join(LocalDir(root), entry.Path, PrivateIgnoreFile)) {
			t.Errorf("%s hat die .gitignore still zurückbekommen", entry.Path)
		}
	}
}

// Der selbsttätige Weg beim Start: fehlt k-playbook-local/ ganz, entsteht die
// vollständige Struktur; ein zweiter Lauf hat nichts mehr zu tun.
func TestEnsureLocalLegtFehlendeStrukturAn(t *testing.T) {
	root := t.TempDir()

	statuses, created, err := EnsureLocal(root)
	if err != nil {
		t.Fatalf("EnsureLocal: %v", err)
	}
	if !created {
		t.Fatal("das fehlende Verzeichnis wurde nicht angelegt")
	}
	if !LocalOK(statuses) {
		t.Fatalf("nach EnsureLocal unvollständig: %+v", statuses)
	}
	if !LocalOK(CheckLocal(root)) {
		t.Fatal("die Struktur ist nach dem Anlegen nicht vollständig")
	}

	statuses, created, err = EnsureLocal(root)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if created {
		t.Error("der zweite Lauf hat erneut angelegt")
	}
	if statuses != nil {
		t.Errorf("der zweite Lauf meldet Zustände, obwohl nichts angelegt wurde: %+v", statuses)
	}
}

// Ein vorhandenes, aber unvollständiges Verzeichnis bleibt dem Knopf: eine
// fehlende README kann bewusst entfernt worden sein und käme sonst bei jedem
// Start zurück.
func TestEnsureLocalLaesstUnvollstaendigesVerzeichnisStehen(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	readme := filepath.Join(LocalDir(root), "reviews", "README.md")
	if err := os.Remove(readme); err != nil {
		t.Fatalf("README entfernen: %v", err)
	}

	_, created, err := EnsureLocal(root)
	if err != nil {
		t.Fatalf("EnsureLocal: %v", err)
	}
	if created {
		t.Error("ein vorhandenes Verzeichnis wurde als angelegt gemeldet")
	}
	if pathExists(readme) {
		t.Error("die entfernte README ist beim Start zurückgekommen")
	}
}

// Ein über makePublic() bewusst öffentlich geschaltetes Verzeichnis bleibt
// öffentlich: der Startlauf fasst ein vorhandenes k-playbook-local/ nicht an
// und bringt die verwaltete .gitignore nicht zurück.
func TestEnsureLocalLaesstOeffentlichGeschaltetesVerzeichnisOeffentlich(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	writeVCSConfig(t, root, "git")
	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	entry, ok := PrivateEntry("results")
	if !ok {
		t.Fatal("results steht nicht als privates Verzeichnis in der lokalen Struktur")
	}
	change, err := SetPrivate(root, entry, false)
	if err != nil {
		t.Fatalf("SetPrivate: %v", err)
	}
	if change.Status.State != PrivacyPublic {
		t.Fatalf("State nach dem Umschalten = %q, erwartet %q (%s)", change.Status.State, PrivacyPublic, change.Status.Reason)
	}

	_, created, err := EnsureLocal(root)
	if err != nil {
		t.Fatalf("EnsureLocal: %v", err)
	}
	if created {
		t.Error("ein vorhandenes Verzeichnis wurde als angelegt gemeldet")
	}
	if pathExists(filepath.Join(LocalDir(root), "results", PrivateIgnoreFile)) {
		t.Error("die bewusst entfernte .gitignore ist beim Start zurückgekommen")
	}
	if status := PrivacyStatusFor(root, entry); status.State != PrivacyPublic {
		t.Errorf("State nach dem Start = %q, erwartet %q (%s)", status.State, PrivacyPublic, status.Reason)
	}
}
