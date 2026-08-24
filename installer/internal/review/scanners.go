package review

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ScannerCatalogName ist der Katalog der Scan-Jobs, relativ zur Installation.
const ScannerCatalogName = "scripts/scanners.tsv"

// scannerColumns ist die Spaltenzahl einer Zeile. Sie steht fest, damit eine
// vergessene Spalte auffällt, statt still in der letzten zu landen.
const scannerColumns = 10

// SARIFMode sagt, ob ein Job SARIF liefert und auf welchem Weg.
type SARIFMode string

const (
	// SARIFNative: das Werkzeug schreibt SARIF selbst. Nur diese Jobs laufen.
	SARIFNative SARIFMode = "native"
	// SARIFConvert: das Werkzeug liefert etwas anderes, das umgewandelt werden
	// müsste. Der Konverter ist nicht gebaut, der Job wird übersprungen.
	SARIFConvert SARIFMode = "convert"
	// SARIFNone: das Werkzeug erzeugt überhaupt kein SARIF.
	SARIFNone SARIFMode = "none"
)

// WorkdirMode sagt, worin ein Job arbeitet.
type WorkdirMode string

const (
	// WorkdirTarget: Arbeitsverzeichnis ist {target}, die Projektwurzel.
	WorkdirTarget WorkdirMode = "target"
	// WorkdirModule: der Job braucht ein Modulverzeichnis. Er läuft je
	// gefundenem Modul einmal, mit dem Modul als Arbeitsverzeichnis.
	WorkdirModule WorkdirMode = "module"
	// WorkdirModuleFile: der Job läuft je gefundener Manifestdatei einmal —
	// dieselbe Suche und Auffächerung wie WorkdirModule, aber ohne Wechsel
	// hinein: in eine Datei ließe sich kein Prozess starten. Das
	// Arbeitsverzeichnis bleibt {target}, der Pfad geht über {module} als
	// Argument mit.
	//
	// Sinnvoll nur für Werkzeuge, die den Pfad selbst als Argument nehmen
	// (pip-audit -r <requirements-datei>) — nicht für solche, die ihn relativ
	// zum Arbeitsverzeichnis erwarten (govulncheck ./...). Getrennt von
	// WorkdirModule und nicht als dessen Sonderfall, damit die Go-Semantik
	// (Verzeichnis, cd hinein) unverändert bleibt.
	WorkdirModuleFile WorkdirMode = "module-file"
)

// OutputMode sagt, wer die Datei unter raw/ schreibt.
type OutputMode string

const (
	// OutputFile: das Werkzeug schreibt selbst nach {out}.
	OutputFile OutputMode = "file"
	// OutputStdout: der Ausführer leitet den Ausgabestrom dorthin um.
	OutputStdout OutputMode = "stdout"
)

// SoftSkipRule sagt: „Kombiniert der Prozess diesen Exit-Code mit dieser
// Meldung, hat er selbst erklärt, dass es nichts zu prüfen gab." Der Ausführer
// führt einen Treffer dann als skipped mit Grund, nicht als failed.
//
// Grammatik im Katalog: mehrere Regeln durch ; getrennt, jede Regel als
// <Exit-Code>:<Regex>. Der Regex nutzt den Go-regexp-Dialekt und darf kein ;
// enthalten — dieses Zeichen ist der Trenner der Regeln, und ohne Escaping
// bräche eine Regel mit ; in mehrere unvollständige.
type SoftSkipRule struct {
	// ExitCode ist der Prozess-Exit-Code, mit dem die Meldung zusammenfallen
	// muss. -1 fängt nur den Fall ab, dass der Start selbst misslang — dort
	// gibt es überhaupt keinen Exit-Code, und den soll man nie als Soft-Skip
	// deuten.
	ExitCode int `json:"exitCode"`
	// Pattern trifft in stderr oder stdout des Werkzeugs. Beide Ströme werden
	// geprüft, weil ein Scanner die Meldung mal in den einen, mal in den
	// anderen schreibt.
	Pattern *regexp.Regexp `json:"-"`
	// Raw ist das ursprüngliche Regex-Muster, damit MarshalJSON und Fehler-
	// meldungen es zeigen können, ohne den kompilierten Regex zu zerlegen.
	Raw string `json:"pattern"`
}

// Scanner ist eine Zeile aus scanners.tsv: ein Aufruf eines Werkzeugs.
//
// Der Eintrag eines Laufs ist das Werkzeug, nicht der Job — Tool ist deshalb
// die Spalte, über die ein Job an seinem Eintrag hängt.
type Scanner struct {
	Job       string `json:"job"`
	Tool      string `json:"tool"`
	Languages string `json:"languages"`
	// Candidates sagt, was unter dem Bezugspunkt als Gegenstand in Frage kommt.
	// Daraus entsteht die Kandidatenzahl am Job — die Auskunft, ohne die ein
	// leeres Ergebnis nicht von einem ungeprüften zu unterscheiden ist.
	Candidates CandidateKind `json:"candidates"`
	SARIF      SARIFMode     `json:"sarif"`
	Output     OutputMode    `json:"output"`
	Timeout    time.Duration `json:"timeout"`
	// SoftSkip sind Kombinationen aus Exit-Code und Meldung, unter denen der
	// Job als skipped mit Grund gilt — nicht als failed. Damit lässt sich das
	// Signal „das Werkzeug erklärt selbst, dass es unter dem Bezugspunkt
	// nichts zu prüfen gab" (osv-scanner: „No package sources found") aus dem
	// Katalog steuern, statt für jeden Scanner einen Sonderfall in den
	// Ausführer zu schreiben.
	SoftSkip []SoftSkipRule `json:"softSkip,omitempty"`
	// Workdir ist das Arbeitsverzeichnis des Aufrufs. module und module-file
	// fächern den Job zusätzlich auf: einen Aufruf je gefundenem Modul
	// beziehungsweise je gefundener Manifestdatei.
	Workdir WorkdirMode `json:"workdir"`
	// Args ist der Aufruf ohne das Programm selbst, bereits in einzelne
	// Argumente zerlegt. Die Platzhalter stehen noch darin.
	Args []string `json:"args"`
}

// ScannerCatalog ist der Ort des Katalogs in einer Installation.
func ScannerCatalog(playbookDir string) string {
	return filepath.Join(playbookDir, filepath.FromSlash(ScannerCatalogName))
}

// LoadScanners liest den Katalog und prüft ihn vollständig.
//
// Geprüft wird beim Lesen und nicht beim Ausführen: eine kaputte Zeile soll
// auffallen, bevor der erste Scanner startet, sonst hinge das Ergebnis eines
// Laufs davon ab, wie weit er vor dem Fehler gekommen ist.
func LoadScanners(path string) ([]Scanner, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseScanners(string(data), path)
}

// ParseScanners liest den Katalog aus dem Dateiinhalt. source benennt die
// Quelle in Fehlermeldungen.
func ParseScanners(content string, source string) ([]Scanner, error) {
	scanners := []Scanner{}
	seen := map[string]bool{}

	for number, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, "\t")
		if fields[0] == "job" {
			continue
		}
		where := fmt.Sprintf("%s, Zeile %d", source, number+1)
		if len(fields) != scannerColumns {
			return nil, fmt.Errorf("%s: %d Spalten, erwartet %d", where, len(fields), scannerColumns)
		}

		scanner := Scanner{
			Job:        strings.TrimSpace(fields[0]),
			Tool:       strings.TrimSpace(fields[1]),
			Languages:  strings.TrimSpace(fields[2]),
			Candidates: CandidateKind(strings.TrimSpace(fields[3])),
			SARIF:      SARIFMode(strings.TrimSpace(fields[4])),
			Output:     OutputMode(strings.TrimSpace(fields[5])),
			Workdir:    WorkdirMode(strings.TrimSpace(fields[8])),
			Args:       strings.Fields(fields[9]),
		}

		if err := checkScanner(&scanner, strings.TrimSpace(fields[6]), strings.TrimSpace(fields[7]), where); err != nil {
			return nil, err
		}
		if seen[scanner.Job] {
			return nil, fmt.Errorf("%s: Job %s steht doppelt im Katalog", where, scanner.Job)
		}
		seen[scanner.Job] = true

		scanners = append(scanners, scanner)
	}
	return scanners, nil
}

// checkScanner prüft eine gelesene Zeile und ergänzt die Laufzeit.
func checkScanner(scanner *Scanner, timeout string, softSkip string, where string) error {
	// Der Job-Name wird zu einem Dateinamen unter raw/ und muss deshalb
	// denselben Anspruch erfüllen wie ein Eintragsname, obwohl er keiner ist.
	if !ValidEntryName(scanner.Job) {
		return fmt.Errorf("%s: unzulässiger Job-Name %q", where, scanner.Job)
	}
	if scanner.Tool == "" {
		return fmt.Errorf("%s: Job %s nennt kein Werkzeug", where, scanner.Job)
	}
	// Der Name beginnt mit dem Tool-Namen, damit unter raw/ auf einen Blick
	// erkennbar bleibt, welcher Eintrag eine Datei geschrieben hat.
	if scanner.Job != scanner.Tool && !strings.HasPrefix(scanner.Job, scanner.Tool+"-") {
		return fmt.Errorf("%s: Job %s beginnt nicht mit dem Tool-Namen %s", where, scanner.Job, scanner.Tool)
	}
	if scanner.Languages == "" {
		return fmt.Errorf("%s: Job %s hat kein languages-Feld; nutze * für sprachunabhängig", where, scanner.Job)
	}

	switch scanner.Candidates {
	case CandidateSource, CandidateAny, CandidateManifest, CandidateNone:
	default:
		return fmt.Errorf("%s: Job %s hat unbekannten candidates-Wert %q", where, scanner.Job, scanner.Candidates)
	}
	switch scanner.SARIF {
	case SARIFNative, SARIFConvert, SARIFNone:
	default:
		return fmt.Errorf("%s: Job %s hat unbekannten sarif-Wert %q", where, scanner.Job, scanner.SARIF)
	}
	switch scanner.Output {
	case OutputFile, OutputStdout:
	default:
		return fmt.Errorf("%s: Job %s hat unbekannten output-Wert %q", where, scanner.Job, scanner.Output)
	}
	switch scanner.Workdir {
	case WorkdirTarget, WorkdirModule, WorkdirModuleFile:
	default:
		return fmt.Errorf("%s: Job %s hat unbekannten workdir-Wert %q", where, scanner.Job, scanner.Workdir)
	}

	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("%s: Job %s hat unlesbares timeout %q", where, scanner.Job, timeout)
	}
	if duration <= 0 {
		return fmt.Errorf("%s: Job %s hat ein timeout von %s", where, scanner.Job, duration)
	}
	scanner.Timeout = duration

	rules, err := parseSoftSkip(softSkip)
	if err != nil {
		return fmt.Errorf("%s: Job %s hat unlesbares soft_skip: %v", where, scanner.Job, err)
	}
	scanner.SoftSkip = rules

	if len(scanner.Args) == 0 {
		return fmt.Errorf("%s: Job %s hat keine Argumente", where, scanner.Job)
	}
	// Die beiden Ausgabewege schließen sich aus: schreibt das Werkzeug selbst,
	// braucht es das Ziel; leitet der Ausführer um, kennt das Werkzeug es nicht.
	uses := usesPlaceholder(scanner.Args, placeholderOut)
	if scanner.Output == OutputFile && !uses {
		return fmt.Errorf("%s: Job %s schreibt selbst, nennt aber %s nicht", where, scanner.Job, placeholderOut)
	}
	if scanner.Output == OutputStdout && uses {
		return fmt.Errorf("%s: Job %s wird umgeleitet und darf %s nicht nennen", where, scanner.Job, placeholderOut)
	}
	// Ohne Modulsuche gibt es kein Modul, auf das {module} zeigen könnte. Der
	// Platzhalter bliebe stehen und landete wörtlich im Aufruf — das soll beim
	// Lesen auffallen, nicht als unverständliche Meldung des Werkzeugs. Beide
	// Modul-Modi suchen und fächern auf und tragen ihn deshalb: bei module
	// zeigt er auf ein Verzeichnis, bei module-file auf eine Datei.
	if scanner.Workdir != WorkdirModule && scanner.Workdir != WorkdirModuleFile && usesPlaceholder(scanner.Args, placeholderModule) {
		return fmt.Errorf("%s: Job %s nennt %s, läuft aber mit workdir %s", where, scanner.Job, placeholderModule, scanner.Workdir)
	}
	return nil
}

// parseSoftSkip liest die Spalte soft_skip: leer heißt „kein Soft-Skip", sonst
// eine oder mehrere Regeln, durch ; getrennt, jede Regel als <Exit-Code>:<Regex>.
//
// Der Regex nutzt den Go-regexp-Dialekt. Das Zeichen ; wird als Trenner der
// Regeln reserviert und ist im Regex in dieser Fassung nicht unterstützt — wer
// es bräuchte, müsste die Grammatik um Escaping erweitern.
func parseSoftSkip(field string) ([]SoftSkipRule, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return nil, nil
	}
	rules := []SoftSkipRule{}
	for _, raw := range strings.Split(field, ";") {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			return nil, fmt.Errorf("leerer Eintrag in %q", field)
		}
		// Nur das erste : trennt Exit-Code vom Regex — der Regex selbst darf
		// weitere : enthalten.
		colon := strings.Index(entry, ":")
		if colon <= 0 || colon == len(entry)-1 {
			return nil, fmt.Errorf("Eintrag %q folgt nicht dem Muster <Exit-Code>:<Regex>", entry)
		}
		code, err := strconv.Atoi(strings.TrimSpace(entry[:colon]))
		if err != nil {
			return nil, fmt.Errorf("Exit-Code in %q ist keine Ganzzahl: %v", entry, err)
		}
		pattern := entry[colon+1:]
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("Regex in %q lässt sich nicht übersetzen: %v", entry, err)
		}
		rules = append(rules, SoftSkipRule{ExitCode: code, Pattern: compiled, Raw: pattern})
	}
	return rules, nil
}

// MatchSoftSkip sucht unter den Regeln des Jobs die erste, die zum Ausgang
// passt: der Prozess ist regulär mit exitCode geendet, und das Muster trifft
// in stderr oder stdout des Werkzeugs. Der Rückgabewert ist die Regel und der
// getroffene Textausschnitt — als Grund für die entries/<tool>.json.
//
// Ein leeres Regelwerk gibt (nil, "") zurück und heißt „kein Soft-Skip
// konfiguriert"; damit trifft der Aufruf keine Fallentscheidung, wenn der
// Katalog nichts sagt.
func (s Scanner) MatchSoftSkip(exitCode int, stderr string, stdout string) (*SoftSkipRule, string) {
	for index := range s.SoftSkip {
		rule := &s.SoftSkip[index]
		if rule.ExitCode != exitCode {
			continue
		}
		if match := firstMatchLine(rule.Pattern, stderr); match != "" {
			return rule, match
		}
		if match := firstMatchLine(rule.Pattern, stdout); match != "" {
			return rule, match
		}
	}
	return nil, ""
}

// firstMatchLine liefert die erste Zeile, in der das Muster trifft — gekürzt,
// damit der Grund in entries/<tool>.json lesbar bleibt und nicht die halbe
// Werkzeugausgabe schluckt.
func firstMatchLine(pattern *regexp.Regexp, text string) string {
	if pattern == nil || text == "" {
		return ""
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(strings.TrimRight(line, "\r"), " \t")
		if !pattern.MatchString(line) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 200 {
			trimmed = trimmed[:200] + "…"
		}
		return trimmed
	}
	return ""
}

const (
	placeholderOut     = "{out}"
	placeholderTarget  = "{target}"
	placeholderModule  = "{module}"
	placeholderScripts = "{scripts}"
)

func usesPlaceholder(args []string, placeholder string) bool {
	for _, arg := range args {
		if strings.Contains(arg, placeholder) {
			return true
		}
	}
	return false
}

// ScannersFor sind die Jobs eines Werkzeugs, in der Reihenfolge des Katalogs.
func ScannersFor(scanners []Scanner, tool string) []Scanner {
	found := []Scanner{}
	for _, scanner := range scanners {
		if scanner.Tool == tool {
			found = append(found, scanner)
		}
	}
	return found
}

// Command setzt die Argumente eines Aufrufs zusammen. Ersetzt wird erst nach
// dem Zerlegen, damit ein Pfad mit Leerzeichen ein Argument bleibt.
//
// module ist der Gegenstand dieses Aufrufs, absolut: bei workdir module das
// Modulverzeichnis, bei workdir module-file der Pfad der Manifestdatei; bei
// workdir target ist es leer, und dort weist checkScanner den Platzhalter
// ohnehin ab. target bleibt in allen Fällen die Projektwurzel — bei workdir
// module fällt sie mit dem Arbeitsverzeichnis auseinander, bei module-file
// bleibt sie es.
func (s Scanner) Command(out string, target string, module string, scriptsDir string) []string {
	replacer := strings.NewReplacer(
		placeholderOut, out,
		placeholderTarget, target,
		placeholderModule, module,
		placeholderScripts, scriptsDir,
	)
	args := make([]string, 0, len(s.Args))
	for _, arg := range s.Args {
		args = append(args, replacer.Replace(arg))
	}
	return args
}

// AppliesTo meldet, ob ein Job zur Sprachauswahl eines Laufs passt.
//
// Maßgeblich ist die Auswahl des Laufs, nicht die heutige Konfiguration: was
// gelaufen ist, soll nachvollziehbar bleiben.
func (s Scanner) AppliesTo(languages []string) bool {
	selected := map[string]bool{}
	for _, language := range languages {
		selected[strings.TrimSpace(language)] = true
	}
	for _, language := range strings.Split(s.Languages, ",") {
		language = strings.TrimSpace(language)
		if language == "*" || selected[language] {
			return true
		}
	}
	return false
}
