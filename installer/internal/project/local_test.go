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
	if !isDir(filepath.Join(local, "priv")) {
		t.Errorf("priv fehlt")
	}
	// Review-Ergebnisse haben unter k-playbook-local/ keinen Platz mehr (Task
	// 146): /k-upgrade löscht ein altes results/, und das Einrichten legt es
	// nicht wieder an.
	if pathExists(filepath.Join(local, "results")) {
		t.Errorf("results angelegt")
	}
	// Projekteigene Instructions, Includes und Review-Rezepte liegen in der
	// Collection des Projekts unter keph:/k-playbook/ (Task 143); angelegt
	// wird keines der alten Verzeichnisse.
	for _, name := range []string{"reviews", "instructions", "includes"} {
		if pathExists(filepath.Join(local, name)) {
			t.Errorf("%s angelegt", name)
		}
	}
	// Projekteigene Guidelines liegen unter keph:/docs/guidelines/ (Todo #40).
	if pathExists(filepath.Join(local, "guidelines")) {
		t.Errorf("guidelines angelegt")
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
	// Die Quellenkonfiguration liegt seit Task 144 in der Collection
	// (keph:/k-playbook/inventory.md); das Einrichten legt die alte Datei
	// nicht mehr an.
	if pathExists(filepath.Join(local, LegacyVersionSourcesFileName)) {
		t.Errorf("%s angelegt", LegacyVersionSourcesFileName)
	}
}

// Git speichert keine leeren Verzeichnisse; ohne README wären sie nach einem
// Clone des Projekts verschwunden.
func TestCreateLocalLegtReadmesAn(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	readme := filepath.Join(LocalDir(root), "priv", "README.md")
	content, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("README fehlt: %v", err)
	}
	if !strings.Contains(string(content), "eigene Notizen") {
		t.Errorf("README ohne Zweckbeschreibung:\n%s", content)
	}
}

// removedLocalEntries sind die zwölf Einträge, die nicht mehr zur Struktur
// gehören: docs bis material hat Task 107 herausgenommen, ihr Inhalt liegt in
// der Projekt-Collection; inbox, queue, knowledge und cache waren die
// Wissensablage auf der Platte, die Task 123 entfernt hat; rules ist die
// Sorte, die Task 135 entfernt hat, checks die lokale Ebene der Checks, die
// Task 134 entfernt hat; results war die Ablage der Review-Ergebnisse, die
// Task 146 herausgenommen hat.
var removedLocalEntries = []string{"docs", "tasks", "tasks/done", "data", "material", "inbox", "queue", "knowledge", "cache", "rules", "checks", "results"}

// Das Einrichten legt die zwölf Einträge nicht mehr an und zählt sie nicht als
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
// /k-upgrade.
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
	if err := os.MkdirAll(filepath.Join(local, "priv"), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	eigen := filepath.Join(local, "priv", "README.md")
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

// Keine Overlay-Sorte hat ihre lokale Ebene mehr unter k-playbook-local/:
// instructions, includes und reviews liegen seit Task 143 in der Collection
// des Projekts, commands in .k-playbook/commands/, checks hat seit Task 134
// keine lokale Ebene mehr. Die Struktur führt keine davon.
func TestLocalStructureOhneOverlaySorten(t *testing.T) {
	for _, entry := range LocalStructure() {
		for _, kind := range []string{"reviews", "instructions", "includes", "commands", "checks"} {
			if entry.Path == kind {
				t.Errorf("Overlay-Sorte %s steht noch in der lokalen Struktur", kind)
			}
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

// CreateLocal schreibt keine .gitignore: was versioniert wird, entscheidet
// allein das Projekt. Seit results/ nicht mehr zur Struktur gehört (Task 146),
// gibt es keinen Eintrag mehr, den das Einrichten schon privat anlegt.
func TestCreateLocalSchreibtKeineGitignore(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	for _, entry := range LocalStructure() {
		if entry.IsFile {
			continue
		}
		if pathExists(filepath.Join(LocalDir(root), entry.Path, PrivateIgnoreFile)) {
			t.Errorf("%s hat eine .gitignore, die CreateLocal nicht schreiben darf", entry.Path)
		}
	}
}

// Ein zweiter Lauf über ein Verzeichnis, dessen verwaltete .gitignore wieder
// entfernt wurde, bringt sie nicht zurück: die Umschaltung gehört dem Projekt.
func TestCreateLocalBringtEntfernteGitignoreNichtZurueck(t *testing.T) {
	root := t.TempDir()

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	ignore := filepath.Join(LocalDir(root), "priv", PrivateIgnoreFile)
	if err := os.WriteFile(ignore, []byte(managedIgnoreContent()), 0o644); err != nil {
		t.Fatalf("priv/.gitignore schreiben: %v", err)
	}
	if err := os.Remove(ignore); err != nil {
		t.Fatalf("priv/.gitignore entfernen: %v", err)
	}

	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal, zweiter Lauf: %v", err)
	}
	if pathExists(ignore) {
		t.Error("priv hat die .gitignore still zurückbekommen")
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
	readme := filepath.Join(LocalDir(root), "priv", "README.md")
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

// Eine Umschaltung bleibt über den Startlauf hinweg stehen, in beide
// Richtungen: der fasst ein vorhandenes k-playbook-local/ nicht an, nimmt die
// verwaltete .gitignore also weder weg noch bringt er sie zurück.
func TestEnsureLocalLaesstUmschaltungStehen(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	writeVCSConfig(t, root, "git")
	if _, err := CreateLocal(root); err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	entry, ok := PrivateEntry("priv")
	if !ok {
		t.Fatal("priv steht nicht als privates Verzeichnis in der lokalen Struktur")
	}
	ignore := filepath.Join(LocalDir(root), "priv", PrivateIgnoreFile)

	for _, private := range []bool{true, false} {
		want := PrivacyPublic
		if private {
			want = PrivacyPrivate
		}

		change, err := SetPrivate(root, entry, private)
		if err != nil {
			t.Fatalf("SetPrivate(%v): %v", private, err)
		}
		if change.Status.State != want {
			t.Fatalf("State nach dem Umschalten = %q, erwartet %q (%s)", change.Status.State, want, change.Status.Reason)
		}

		_, created, err := EnsureLocal(root)
		if err != nil {
			t.Fatalf("EnsureLocal: %v", err)
		}
		if created {
			t.Error("ein vorhandenes Verzeichnis wurde als angelegt gemeldet")
		}
		if pathExists(ignore) != private {
			t.Errorf("privat = %v, aber .gitignore vorhanden = %v nach dem Start", private, pathExists(ignore))
		}
		if status := PrivacyStatusFor(root, entry); status.State != want {
			t.Errorf("State nach dem Start = %q, erwartet %q (%s)", status.State, want, status.Reason)
		}
	}
}
