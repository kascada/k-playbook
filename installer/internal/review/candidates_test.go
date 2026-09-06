package review

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// dateiBaum legt unter einem frischen Verzeichnis die angegebenen Dateien an.
// Der Pfad ist mit / getrennt.
func dateiBaum(t *testing.T, dateien ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, pfad := range dateien {
		voll := filepath.Join(root, filepath.FromSlash(pfad))
		if err := os.MkdirAll(filepath.Dir(voll), 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", pfad, err)
		}
		if err := os.WriteFile(voll, []byte("inhalt\n"), 0o644); err != nil {
			t.Fatalf("%s schreiben: %v", pfad, err)
		}
	}
	return root
}

func zähle(t *testing.T, root string, kind CandidateKind, languages string) int {
	t.Helper()
	count, err := countCandidates(root, kind, languages)
	if err != nil {
		t.Fatalf("countCandidates(%s, %s): %v", kind, languages, err)
	}
	return count
}

// Quelldateien zählen nach der Endung, und die Endungen kommen aus languages.
func TestCountCandidatesQuelldateien(t *testing.T) {
	root := dateiBaum(t,
		"haupt.go", "unten/tief.go", "haupt_test.go",
		"skript.py", "typen.pyi",
		"liesmich.md", "go.mod",
	)

	if got := zähle(t, root, CandidateSource, "go"); got != 3 {
		t.Errorf("go = %d, erwartet 3", got)
	}
	if got := zähle(t, root, CandidateSource, "python"); got != 2 {
		t.Errorf("python = %d, erwartet 2", got)
	}
	if got := zähle(t, root, CandidateSource, "python,go"); got != 5 {
		t.Errorf("python,go = %d, erwartet 5", got)
	}
	// * heißt hier alle bekannten Sprachen, nicht alle Dateien.
	if got := zähle(t, root, CandidateSource, "*"); got != 5 {
		t.Errorf("* = %d, erwartet 5", got)
	}
}

// Ein Secret kann überall stehen: any zählt jede Datei.
func TestCountCandidatesJedeDatei(t *testing.T) {
	root := dateiBaum(t, "haupt.go", "liesmich.md", "unten/.env", "unten/bild.png")

	if got := zähle(t, root, CandidateAny, "*"); got != 4 {
		t.Errorf("any = %d, erwartet 4", got)
	}
}

// Manifeste: die Namen, nicht die Endungen — und 0 ist hier die richtige
// Aussage „nichts zu prüfen".
func TestCountCandidatesManifeste(t *testing.T) {
	root := dateiBaum(t,
		"go.mod", "go.sum", "unten/go.mod",
		"requirements.txt", "requirements-dev.txt", "pyproject.toml", "poetry.lock",
		"haupt.go", "skript.py",
	)

	if got := zähle(t, root, CandidateManifest, "go"); got != 3 {
		t.Errorf("go = %d, erwartet 3", got)
	}
	if got := zähle(t, root, CandidateManifest, "python"); got != 4 {
		t.Errorf("python = %d, erwartet 4", got)
	}
	if got := zähle(t, root, CandidateManifest, "*"); got != 7 {
		t.Errorf("* = %d, erwartet 7", got)
	}

	ohne := dateiBaum(t, "haupt.go", "liesmich.md")
	if got := zähle(t, ohne, CandidateManifest, "*"); got != 0 {
		t.Errorf("ohne Manifest = %d, erwartet 0", got)
	}
}

// Die Ausschlüsse: die Installationskopie und die Ergebnisse früherer Läufe —
// beide an ihrem Ort und nicht über den bloßen Namen —, dazu alles mit
// führendem Punkt.
func TestCountCandidatesUeberspringtAusschluesse(t *testing.T) {
	root := dateiBaum(t,
		"haupt.go",
		"k-playbook/installer/beispiel.go",
		"k-playbook-local/results/2026-08-16/raw/gosec.sarif",
		"k-playbook-local/tasks/notiz.go",
		".git/objects/etwas.go",
		"unten/.venv/lib/paket.go",
	)

	// Übrig bleiben haupt.go und k-playbook-local/tasks/notiz.go: von
	// k-playbook-local nimmt jeder Job nur results/ aus, und was die Jobs
	// sehen, muss die Zählung mitzählen.
	if got := zähle(t, root, CandidateSource, "go"); got != 2 {
		t.Errorf("Quelldateien = %d, erwartet 2", got)
	}
	if got := zähle(t, root, CandidateAny, "*"); got != 2 {
		t.Errorf("alle Dateien = %d, erwartet 2", got)
	}
}

// Der Fehler aus Task 004, in neuer Gestalt: ein Ausschluss über den bloßen
// Namen fräße installer/cmd/k-playbook/ mit — Code dieses Projekts. Verankert
// ist er, und tiefer im Baum trifft er nichts.
func TestCountCandidatesAusschlussIstVerankert(t *testing.T) {
	root := dateiBaum(t,
		"k-playbook/beispiel.go",
		"installer/cmd/k-playbook/haupt.go",
		"dienste/k-playbook-local/results/eigen.go",
	)

	if got := zähle(t, root, CandidateSource, "go"); got != 2 {
		t.Errorf("Quelldateien = %d, erwartet 2", got)
	}
	// Und vom Modul aus gesehen liegt die Installationskopie gar nicht darunter.
	modul := filepath.Join(root, "installer")
	if got := zähle(t, modul, CandidateSource, "go"); got != 1 {
		t.Errorf("Quelldateien im Modul = %d, erwartet 1", got)
	}
}

// Anders als die Modulsuche zählt sie vendor/, node_modules/ und testdata/ mit:
// die Werkzeuge sehen dort hinein, und eine Zahl unter der Zahl der geprüften
// Dateien behauptete „nichts zu prüfen", wo es etwas zu prüfen gab.
func TestCountCandidatesZaehltFremdquellenMit(t *testing.T) {
	root := dateiBaum(t,
		"haupt.go",
		"vendor/fremd/paket.go",
		"node_modules/paket/skript.py",
		"installer/testdata/kaputt/probe.go",
	)

	if got := zähle(t, root, CandidateSource, "go"); got != 3 {
		t.Errorf("Quelldateien = %d, erwartet 3", got)
	}
}

// Eine Sprache ohne bekannte Muster ist nicht zu zählen — und das ist etwas
// anderes als „nichts gefunden": eine 0 behauptete, es gebe nichts zu prüfen.
func TestCountCandidatesUnbekannteSpracheIstFehler(t *testing.T) {
	root := dateiBaum(t, "haupt.rs")
	if _, err := countCandidates(root, CandidateSource, "rust"); err == nil {
		t.Error("unbekannte Sprache wurde gezählt")
	}
	if _, err := countCandidates(root, CandidateNone, "*"); err == nil {
		t.Error("Sorte none wurde gezählt")
	}
}

// Der Bezugspunkt ist das Verzeichnis, das übergeben wird — bei workdir module
// also das Modul und nicht das Projekt.
func TestCandidateRoot(t *testing.T) {
	if got := candidateRoot("/projekt", ""); got != "/projekt" {
		t.Errorf("ohne Modul = %q, erwartet /projekt", got)
	}
	if got := candidateRoot("/projekt", "."); got != "/projekt" {
		t.Errorf("Wurzelmodul = %q, erwartet /projekt", got)
	}
	if got := candidateRoot("/projekt", "installer"); got != filepath.Join("/projekt", "installer") {
		t.Errorf("Modul = %q, erwartet /projekt/installer", got)
	}
}

// Je Bezugspunkt und Sorte einmal, auch bei vielen Fragern gleichzeitig.
func TestCandidateCacheZaehltJeSchluesselEinmal(t *testing.T) {
	var mutex sync.Mutex
	läufe := map[string]int{}
	cache := newCandidateCache(func(root string, kind CandidateKind, languages string) (int, error) {
		mutex.Lock()
		läufe[root+" "+string(kind)]++
		mutex.Unlock()
		return 7, nil
	})

	var group sync.WaitGroup
	for index := 0; index < 20; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			cache.of("/projekt", CandidateSource, "go")
			// Dieselbe Auswahl, andere Schreibweise: derselbe Schlüssel.
			cache.of("/projekt", CandidateSource, "go, go")
			cache.of("/projekt", CandidateAny, "*")
			cache.of("/projekt/installer", CandidateSource, "go")
		}()
	}
	group.Wait()

	for schlüssel, count := range läufe {
		if count != 1 {
			t.Errorf("%s wurde %d mal gezählt, erwartet einmal", schlüssel, count)
		}
	}
	if len(läufe) != 3 {
		t.Errorf("%d Schlüssel gezählt, erwartet 3: %v", len(läufe), läufe)
	}
}

// Ein Fehler beim Baumlauf ist kein Ergebnis: das Feld bleibt ungesetzt, statt
// eine 0 zu behaupten.
func TestCandidateCacheFehlerBleibtUngesetzt(t *testing.T) {
	cache := newCandidateCache(func(string, CandidateKind, string) (int, error) {
		return 0, errors.New("nicht lesbar")
	})
	if got := cache.of("/projekt", CandidateSource, "go"); got != nil {
		t.Errorf("Kandidaten = %v, erwartet ungesetzt", *got)
	}

	// Die Sorten ohne Zählung ebenso — und ohne die Zählung überhaupt zu rufen.
	gezählt := 0
	cache = newCandidateCache(func(string, CandidateKind, string) (int, error) {
		gezählt++
		return 1, nil
	})
	if got := cache.of("/projekt", CandidateNone, "*"); got != nil {
		t.Errorf("none = %v, erwartet ungesetzt", *got)
	}
	if got := cache.of("", CandidateAny, "*"); got != nil {
		t.Errorf("ohne Bezugspunkt = %v, erwartet ungesetzt", *got)
	}
	if gezählt != 0 {
		t.Errorf("%d Zählungen, erwartet keine", gezählt)
	}
}

// Bei workdir module-file ist der Bezugspunkt eine Datei, kein Verzeichnis.
// filepath.WalkDir ruft die Walk-Funktion dann genau einmal für diesen einen
// Eintrag auf: die Zählung ergibt 1 — die Manifestdatei trifft ihr eigenes
// Muster — und keinen Fehler. Die 1 bedeutet dort etwas anderes als bei einem
// Verzeichnis („alle passenden Dateien darunter"), bleibt aber eine stimmige
// Lesart derselben Zahl.
func TestCountCandidatesMitDateiAlsBezugspunkt(t *testing.T) {
	root := dateiBaum(t, "requirements.txt", "requirements-dev.txt", "haupt.py")

	manifest := candidateRoot(root, "requirements.txt")
	if manifest != filepath.Join(root, "requirements.txt") {
		t.Fatalf("candidateRoot = %q, erwartet den Pfad der Manifestdatei", manifest)
	}
	if got := zähle(t, manifest, CandidateManifest, "python"); got != 1 {
		t.Errorf("Kandidaten = %d, erwartet 1 — die Datei selbst", got)
	}

	// Eine Datei, die das Muster nicht trifft, zählt auch als Bezugspunkt
	// nicht mit. Ein Fehler ist sie trotzdem nicht.
	if got := zähle(t, candidateRoot(root, "haupt.py"), CandidateManifest, "python"); got != 0 {
		t.Errorf("Kandidaten = %d, erwartet 0 — haupt.py ist kein Manifest", got)
	}
}

// JavaScript und TypeScript sind zwei Schlüssel und keiner: ein reines
// TypeScript-Projekt soll seine Endungen gezählt bekommen, ohne ersatzweise
// javascript ankreuzen zu müssen.
func TestCountCandidatesJavaScriptUndTypeScript(t *testing.T) {
	root := dateiBaum(t,
		"src/index.ts", "src/tabellen.ts", "src/komponente.tsx", "typen.mts", "alt.cts",
		"test/anfuegen.test.mjs", "skripte/rauch.mjs", "werkzeug.js", "alt.cjs", "sicht.jsx",
		"liesmich.md", "package.json",
	)

	if got := zähle(t, root, CandidateSource, "typescript"); got != 5 {
		t.Errorf("typescript = %d, erwartet 5", got)
	}
	if got := zähle(t, root, CandidateSource, "javascript"); got != 5 {
		t.Errorf("javascript = %d, erwartet 5", got)
	}
	if got := zähle(t, root, CandidateSource, "javascript,typescript"); got != 10 {
		t.Errorf("javascript,typescript = %d, erwartet 10", got)
	}
	// Die Sprachen dürfen sich nicht überschneiden, sonst zählte ein Projekt
	// mit beiden Schlüsseln seine Dateien doppelt.
	if got := zähle(t, root, CandidateSource, "go"); got != 0 {
		t.Errorf("go = %d, erwartet 0", got)
	}
}

// Beide Node-Schlüssel tragen dieselbe Manifestliste: welches Manifest ein
// Projekt hat, sagt nichts darüber, ob darin JavaScript oder TypeScript steht.
func TestCountCandidatesNodeManifeste(t *testing.T) {
	root := dateiBaum(t,
		"package.json", "package-lock.json", "unten/package.json",
		"yarn.lock", "pnpm-lock.yaml", "npm-shrinkwrap.json",
		"src/index.ts", "liesmich.md",
	)

	if got := zähle(t, root, CandidateManifest, "javascript"); got != 6 {
		t.Errorf("javascript = %d, erwartet 6", got)
	}
	if got := zähle(t, root, CandidateManifest, "typescript"); got != 6 {
		t.Errorf("typescript = %d, erwartet 6", got)
	}
	// Beide zusammen ergeben dieselbe Menge und nicht die doppelte: die Muster
	// werden gegen jeden Dateinamen geprüft, nicht je Sprache aufaddiert.
	if got := zähle(t, root, CandidateManifest, "javascript,typescript"); got != 6 {
		t.Errorf("javascript,typescript = %d, erwartet 6", got)
	}
}

// node_modules/ fällt über den Namen heraus, auf jeder Ebene — anders als die
// verankerten Ausschlüsse aus candidateExcluded. Sonst stünden die wenigen
// echten Dateien eines Node-Projekts gegen die tausenden seiner Abhängigkeiten.
func TestCountCandidatesNodeModulesZähltNicht(t *testing.T) {
	root := dateiBaum(t,
		"src/index.ts", "werkzeug.js",
		"node_modules/paket/index.js", "node_modules/paket/package.json",
		"unten/node_modules/tief/index.js",
		"package.json",
	)

	if got := zähle(t, root, CandidateSource, "javascript,typescript"); got != 2 {
		t.Errorf("source = %d, erwartet 2", got)
	}
	if got := zähle(t, root, CandidateManifest, "javascript,typescript"); got != 1 {
		t.Errorf("manifest = %d, erwartet 1", got)
	}
	// Auch die Secret-Sucher sehen dort nicht hinein.
	if got := zähle(t, root, CandidateAny, "*"); got != 3 {
		t.Errorf("any = %d, erwartet 3", got)
	}
}
