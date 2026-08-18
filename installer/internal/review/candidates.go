package review

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// CandidateKind sagt, was unter dem Bezugspunkt eines Jobs überhaupt als
// Gegenstand in Frage kommt. Sie steht als Spalte candidates im Katalog.
//
// Die Sorte ist ein Datum und keine Regel im Code: ein Sonderfall, der auf
// einen Werkzeugnamen prüft, wäre genau das, was diese Auskunft vermeiden soll.
// Der Ausführer kennt nur die vier Sorten; welcher Job welche hat, sagt der
// Katalog.
type CandidateKind string

const (
	// CandidateSource: Dateien mit den Endungen der Sprachen aus languages.
	// Für Werkzeuge, die Quellcode lesen (gosec, ruff, golangci-lint, semgrep).
	CandidateSource CandidateKind = "source"
	// CandidateAny: jede Datei unter dem Bezugspunkt. Für die Secret-Sucher —
	// ein Secret kann in jeder Datei stehen.
	CandidateAny CandidateKind = "any"
	// CandidateManifest: Abhängigkeits-Manifeste. Hier ist 0 die richtige
	// Aussage „nichts zu prüfen" und kein Alarm: ein Projekt ohne Manifest hat
	// keine Abhängigkeiten, über die ein Werkzeug etwas sagen könnte.
	CandidateManifest CandidateKind = "manifest"
	// CandidateNone: keine Zählung. Das Feld am Job bleibt ungesetzt.
	//
	// Für Jobs, deren Gegenstand sich ohne die Erkennungslogik des Werkzeugs
	// nicht abgrenzen lässt — trivy config sucht IaC-Konfigurationen
	// (Dockerfile, Terraform, Kubernetes-Manifeste), und eine Näherung nach
	// Dateinamensmuster erzeugte eher Falschalarme, als dass sie einordnete.
	CandidateNone CandidateKind = "none"
)

// candidatePatterns sagt je Sorte und Sprache, welche Dateinamen zählen.
//
// Die Muster laufen gegen den Dateinamen, nicht gegen den Pfad (path.Match);
// ein Muster ohne Platzhalter ist damit ein Name.
//
// Woher die Sprache kommt: aus der Spalte languages derselben Zeile. Sie ist im
// Lauf der Sprachfilter (AppliesTo), taugt hier aber als Quelle — ein Job für
// go prüft Go-Dateien. Ein * heißt alle bekannten Sprachen.
var candidatePatterns = map[CandidateKind]map[string][]string{
	CandidateSource: {
		"go":     {"*.go"},
		"python": {"*.py", "*.pyi"},
	},
	CandidateManifest: {
		"go": {"go.mod", "go.sum"},
		// requirements*.txt deckt requirements-dev.txt und requirements.txt
		// gemeinsam ab; die Namen sind nicht abzählbar.
		"python": {
			"requirements*.txt", "constraints*.txt", "pyproject.toml",
			"setup.py", "setup.cfg", "Pipfile", "Pipfile.lock", "poetry.lock",
		},
	},
}

// candidateExcluded sind die Verzeichnisse, die kein Job sieht — als Pfad
// **relativ zum Bezugspunkt**, mit / getrennt.
//
// Über den Pfad und nicht über den Namen, und das ist der Punkt: unter
// installer/cmd/k-playbook/ steht Code dieses Projekts, unter results/
// womöglich in jedem anderen. Eine Regel über den bloßen Namen fräße beides
// mit — genau der Fehler aus Task 004, wo ein Ausschlussmuster ohne Anker in
// einem Projekt namens k-playbook die ganze Projektwurzel traf und der Lauf
// stumm nichts mehr fand. Verankert trifft der Ausschluss nur den einen Ort,
// den er meint; liegt dort nichts, fällt auch nichts aus.
//
// Warum diese Liste nicht dieselbe ist wie moduleSearchExcluded: die Modulsuche
// fragt, wo ein Modul des Projekts liegt, und lässt deshalb vendor/,
// node_modules/ und testdata/ aus — dort steht fremder oder Prüfcode. Die
// Zählung fragt etwas anderes, nämlich was ein Werkzeug hätte sehen können, und
// die Werkzeuge sehen dort sehr wohl hinein. Sie hier auszunehmen ergäbe eine
// Zahl unter der Zahl der geprüften Dateien — und damit ein „nichts zu prüfen",
// wo es etwas zu prüfen gab.
//
// Was bleibt, sind die beiden generischen Ausschlüsse, die jeder Job in
// scanners.tsv mitführt.
var candidateExcluded = map[string]bool{
	// Die Installationskopie. Ihr ausgeliefertes Beispielmaterial ist kein Code
	// des Projekts.
	"k-playbook": true,
	// Die Ergebnisse früherer Läufe. Ohne den Ausschluss zählten die
	// Secret-Sucher die SARIF-Dateien des vorigen Laufs als Gegenstand mit.
	"k-playbook-local/results": true,
}

// countCandidates zählt die Dateien unter root, die für kind und languages als
// Gegenstand in Frage kommen.
//
// **Die Zahl ist eine Obergrenze, keine Abdeckungsmessung.** Die werkzeugeigenen
// Ausschlüsse kann sie nicht kennen: sie stehen in der Spalte args, und jedes
// Werkzeug schreibt sie anders (--exclude, --skip-dirs, bei gitleaks eine eigene
// Konfigurationsdatei). Es gilt deshalb nur: Kandidaten ≥ tatsächlich geprüfte
// Dateien. 0 heißt sicher „nichts zu tun"; eine hohe Zahl heißt „hier hätte
// etwas sein können" — und mehr behauptet die Angabe nicht.
func countCandidates(root string, kind CandidateKind, languages string) (int, error) {
	if root == "" {
		return 0, errors.New("kein Bezugspunkt angegeben")
	}
	match, err := candidateMatcher(kind, candidateLanguages(languages))
	if err != nil {
		return 0, err
	}

	count := 0
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == root {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if skipCandidateDir(entry.Name(), filepath.ToSlash(relative)) {
				return fs.SkipDir
			}
			return nil
		}
		// Nur reguläre Dateien. Ein Symlink zeigt entweder auf etwas, das schon
		// gezählt ist, oder aus dem Bezugspunkt heraus.
		if !entry.Type().IsRegular() {
			return nil
		}
		if match(entry.Name()) {
			count++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// skipCandidateDir meldet, ob ein Verzeichnis für die Zählung ausfällt.
//
// Verzeichnisse mit führendem Punkt fallen als Gruppe heraus, und die einzig
// über den Namen: dort liegen Versionsverwaltung, Werkzeug-Caches und virtuelle
// Umgebungen, und zwar auf jeder Ebene. Sie mitzuzählen hieße, die Auskunft in
// Zehntausenden Dateien aus .git und .venv zu ertränken.
func skipCandidateDir(name string, relative string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	return candidateExcluded[relative]
}

// candidateMatcher liefert die Prüfung, ob ein Dateiname als Kandidat zählt.
//
// Ein Fehler heißt „nicht zu zählen" und nicht „nichts gefunden": eine Sprache
// ohne bekannte Muster ergäbe sonst eine 0, die fälschlich behauptete, es gebe
// nichts zu prüfen.
func candidateMatcher(kind CandidateKind, languages []string) (func(name string) bool, error) {
	if kind == CandidateAny {
		return func(string) bool { return true }, nil
	}
	byLanguage, known := candidatePatterns[kind]
	if !known {
		return nil, fmt.Errorf("Kandidatensorte %q wird nicht gezählt", kind)
	}

	patterns := []string{}
	for _, language := range languages {
		if language == "*" {
			for _, list := range byLanguage {
				patterns = append(patterns, list...)
			}
			continue
		}
		list, found := byLanguage[language]
		if !found {
			return nil, fmt.Errorf("für die Sprache %s ist nicht bekannt, was als %s zählt", language, kind)
		}
		patterns = append(patterns, list...)
	}
	if len(patterns) == 0 {
		return nil, errors.New("die Spalte languages nennt keine Sprache")
	}

	return func(name string) bool {
		for _, pattern := range patterns {
			// Der Fehler von path.Match trifft nur ein fehlerhaftes Muster;
			// die Muster stehen hier im Code und werden vom Test abgedeckt.
			if matched, err := path.Match(pattern, name); err == nil && matched {
				return true
			}
		}
		return false
	}, nil
}

// candidateLanguages zerlegt die Spalte languages — sortiert und ohne Doppelte,
// damit zwei Schreibweisen derselben Auswahl denselben Zwischenspeicher treffen.
func candidateLanguages(languages string) []string {
	found := map[string]bool{}
	for _, language := range strings.Split(languages, ",") {
		language = strings.TrimSpace(language)
		if language != "" {
			found[language] = true
		}
	}
	list := make([]string, 0, len(found))
	for language := range found {
		list = append(list, language)
	}
	sort.Strings(list)
	return list
}

// candidateRoot ist der Bezugspunkt eines Jobs: bei workdir module das Modul,
// sonst das Ziel.
//
// Sonst zählte ein aufgefächerter Job die Dateien des ganzen Projekts gegen die
// Befunde eines einzelnen Moduls.
func candidateRoot(target string, module string) string {
	if module == "" {
		return target
	}
	return filepath.Join(target, module)
}

// candidateCache hält die Zählungen eines Laufs fest: höchstens eine je
// Bezugspunkt und Sorte, über den ganzen Lauf.
//
// Ohne ihn liefe derselbe Baumlauf einmal je Job — bei acht Werkzeugen über
// demselben Ziel achtmal. Der Schlüssel ist deshalb der Bezugspunkt samt Sorte
// und Sprachen und nicht der Job: zwei Jobs, die dasselbe zählen würden, zählen
// einmal.
//
// Ein Vorlauf in Execute() täte es auch, verlangte aber, den Bestand an
// Bezugspunkten vorher zu kennen — und der entsteht erst in planJobs(), wo die
// Modulsuche läuft.
type candidateCache struct {
	// count ist die Zählung selbst. Als Feld, damit ein Test sie ersetzen und
	// mitzählen kann, wie oft sie wirklich läuft.
	count func(root string, kind CandidateKind, languages string) (int, error)

	mutex sync.Mutex
	known map[string]*candidateCount
}

// candidateCount ist eine Zählung: erst nach once.Do stehen value und err.
type candidateCount struct {
	once  sync.Once
	value int
	err   error
}

func newCandidateCache(count func(root string, kind CandidateKind, languages string) (int, error)) *candidateCache {
	return &candidateCache{count: count, known: map[string]*candidateCount{}}
}

// of liefert die Kandidatenzahl eines Bezugspunkts oder nil, wenn nicht gezählt
// wird.
//
// nil ist der einzige Weg, „nicht gemessen" auszudrücken — eine 0 hieße
// „gemessen und null". Deshalb kommt nil auch dann zurück, wenn die Zählung
// scheitert: sie ist eine Zusatzauskunft, kein Ergebnis, und ein Fehler macht
// keinen Job zum Fehlschlag.
func (c *candidateCache) of(root string, kind CandidateKind, languages string) *int {
	// Ohne Zwischenspeicher wird nicht gezählt. Execute legt ihn an; wer
	// planJobs() unmittelbar aufruft, bekommt die Auskunft nur, wenn er einen
	// mitgibt — eine Zählung ohne laufweite Klammer soll nicht nebenbei
	// entstehen.
	if c == nil {
		return nil
	}
	if root == "" || kind == CandidateNone || kind == "" {
		return nil
	}

	key := strings.Join([]string{string(kind), strings.Join(candidateLanguages(languages), ","), root}, "\n")
	c.mutex.Lock()
	counted := c.known[key]
	if counted == nil {
		counted = &candidateCount{}
		c.known[key] = counted
	}
	c.mutex.Unlock()

	// Außerhalb der Sperre: der Baumlauf dauert, und zwei Bezugspunkte sollen
	// sich dabei nicht gegenseitig aufhalten. Wer denselben Schlüssel will,
	// wartet auf demselben Once.
	counted.once.Do(func() { counted.value, counted.err = c.count(root, kind, languages) })
	if counted.err != nil {
		return nil
	}
	value := counted.value
	return &value
}
