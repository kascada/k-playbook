package review

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ScannerCatalogName ist der Katalog der Scan-Jobs, relativ zur Installation.
const ScannerCatalogName = "scripts/scanners.tsv"

// scannerColumns ist die Spaltenzahl einer Zeile. Sie steht fest, damit eine
// vergessene Spalte auffällt, statt still in der letzten zu landen.
const scannerColumns = 9

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
)

// OutputMode sagt, wer die Datei unter raw/ schreibt.
type OutputMode string

const (
	// OutputFile: das Werkzeug schreibt selbst nach {out}.
	OutputFile OutputMode = "file"
	// OutputStdout: der Ausführer leitet den Ausgabestrom dorthin um.
	OutputStdout OutputMode = "stdout"
)

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
	// Workdir ist das Arbeitsverzeichnis des Aufrufs. module fächert den Job
	// zusätzlich auf: einen Aufruf je gefundenem Modul.
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
			Workdir:    WorkdirMode(strings.TrimSpace(fields[7])),
			Args:       strings.Fields(fields[8]),
		}

		if err := checkScanner(&scanner, strings.TrimSpace(fields[6]), where); err != nil {
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
func checkScanner(scanner *Scanner, timeout string, where string) error {
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
	case WorkdirTarget, WorkdirModule:
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
	// Lesen auffallen, nicht als unverständliche Meldung des Werkzeugs.
	if scanner.Workdir != WorkdirModule && usesPlaceholder(scanner.Args, placeholderModule) {
		return fmt.Errorf("%s: Job %s nennt %s, läuft aber mit workdir %s", where, scanner.Job, placeholderModule, scanner.Workdir)
	}
	return nil
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
// module ist das Modulverzeichnis dieses Aufrufs, absolut; bei workdir target
// ist es leer, und dort weist checkScanner den Platzhalter ohnehin ab. target
// bleibt in beiden Fällen die Projektwurzel — bei workdir module fällt sie mit
// dem Arbeitsverzeichnis auseinander.
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
