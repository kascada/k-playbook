package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const scannerHeader = "job\ttool\tlanguages\tcandidates\tsarif\toutput\ttimeout\tworkdir\targs\n"

func parseLine(t *testing.T, line string) ([]Scanner, error) {
	t.Helper()
	return ParseScanners(scannerHeader+line+"\n", "test.tsv")
}

func TestParseScannersLiestZeile(t *testing.T) {
	scanners, err := parseLine(t, "trivy-fs\ttrivy\t*\tmanifest\tnative\tfile\t15m\ttarget\tfs --output {out} {target}")
	if err != nil {
		t.Fatalf("ParseScanners: %v", err)
	}
	if len(scanners) != 1 {
		t.Fatalf("%d Jobs, erwartet 1", len(scanners))
	}

	scanner := scanners[0]
	if scanner.Job != "trivy-fs" || scanner.Tool != "trivy" {
		t.Errorf("Job/Tool falsch gelesen: %+v", scanner)
	}
	if scanner.SARIF != SARIFNative || scanner.Output != OutputFile {
		t.Errorf("sarif/output falsch gelesen: %+v", scanner)
	}
	if scanner.Timeout != 15*time.Minute {
		t.Errorf("Timeout = %s, erwartet 15m", scanner.Timeout)
	}
	if scanner.Workdir != WorkdirTarget {
		t.Errorf("workdir falsch gelesen: %+v", scanner)
	}
	if len(scanner.Args) != 4 {
		t.Errorf("Args = %v, erwartet vier Argumente", scanner.Args)
	}
}

// Ein Job, der ein Modulverzeichnis braucht, und der Platzhalter dazu.
func TestParseScannersLiestWorkdirModule(t *testing.T) {
	scanners, err := parseLine(t, "govulncheck	govulncheck	go	manifest	native	stdout	15m	module	-format sarif ./...")
	if err != nil {
		t.Fatalf("ParseScanners: %v", err)
	}
	if scanners[0].Workdir != WorkdirModule {
		t.Errorf("workdir = %q, erwartet %q", scanners[0].Workdir, WorkdirModule)
	}
	if _, err := parseLine(t, "gosec	gosec	go	source	native	file	15m	module	-out={out} {module}"); err != nil {
		t.Errorf("{module} bei workdir module wurde abgewiesen: %v", err)
	}
}

// Kommentare und die Kopfzeile sind keine Jobs.
func TestParseScannersUeberspringtKommentare(t *testing.T) {
	content := "# Kommentar\n" + scannerHeader + "\nruff\truff\tpython\tsource\tnative\tfile\t10m\ttarget\tcheck -o {out} {target}\n"
	scanners, err := ParseScanners(content, "test.tsv")
	if err != nil {
		t.Fatalf("ParseScanners: %v", err)
	}
	if len(scanners) != 1 || scanners[0].Job != "ruff" {
		t.Fatalf("unerwartetes Ergebnis: %+v", scanners)
	}
}

func TestParseScannersWeistFehlerhafteZeilenAb(t *testing.T) {
	fälle := map[string]string{
		"zu wenige Spalten":      "ruff\truff\tpython\tnative\tfile\t10m",
		"Job führt aus raw/":     "../ruff\truff\tpython\tsource\tnative\tfile\t10m\ttarget\tcheck -o {out} {target}",
		"Job ohne Tool-Präfix":   "prüfung\truff\tpython\tsource\tnative\tfile\t10m\ttarget\tcheck -o {out} {target}",
		"leeres languages":       "ruff\truff\t\tsource\tnative\tfile\t10m\ttarget\tcheck -o {out} {target}",
		"leeres candidates":      "ruff\truff\tpython\t\tnative\tfile\t10m\ttarget\tcheck -o {out} {target}",
		"unbekanntes candidates": "ruff\truff\tpython\tvielleicht\tnative\tfile\t10m\ttarget\tcheck -o {out} {target}",
		"unbekanntes sarif":      "ruff\truff\tpython\tsource\tvielleicht\tfile\t10m\ttarget\tcheck -o {out} {target}",
		"unbekanntes output":     "ruff\truff\tpython\tsource\tnative\tirgendwohin\t10m\ttarget\tcheck -o {out} {target}",
		"unlesbares timeout":     "ruff\truff\tpython\tsource\tnative\tfile\tbald\ttarget\tcheck -o {out} {target}",
		"timeout null":           "ruff\truff\tpython\tsource\tnative\tfile\t0s\ttarget\tcheck -o {out} {target}",
		"file ohne {out}":        "ruff\truff\tpython\tsource\tnative\tfile\t10m\ttarget\tcheck {target}",
		"stdout mit {out}":       "ruff\truff\tpython\tsource\tnative\tstdout\t10m\ttarget\tcheck -o {out} {target}",
		"keine Argumente":        "ruff\truff\tpython\tsource\tnative\tstdout\t10m\ttarget\t",
		"doppelter Job im Satz":  "ruff\truff\tpython\tsource\tnative\tfile\t10m\ttarget\tcheck -o {out} {target}\nruff\truff\tpython\tsource\tnative\tfile\t10m\ttarget\tcheck -o {out} {target}",
		"unbekanntes workdir":    "ruff\truff\tpython\tsource\tnative\tfile\t10m\tirgendwo\tcheck -o {out} {target}",
		"leeres workdir":         "ruff\truff\tpython\tsource\tnative\tfile\t10m\t\tcheck -o {out} {target}",
		// {module} ohne Modulsuche: der Platzhalter bliebe stehen und landete
		// wörtlich im Aufruf.
		"{module} bei workdir target": "ruff\truff\tpython\tsource\tnative\tfile\t10m\ttarget\tcheck -o {out} {module}",
	}

	for name, line := range fälle {
		if _, err := parseLine(t, line); err == nil {
			t.Errorf("%s wurde angenommen", name)
		}
	}
}

func TestScannerCommandErsetztPlatzhalter(t *testing.T) {
	scanner := Scanner{Args: []string{"dir:{target}", "--file", "{out}", "-c", "{scripts}/gitleaks.toml", "{module}"}}
	args := scanner.Command("/lauf/raw/x.sarif", "/mit platz/repo", "/mit platz/repo/installer", "/skripte")

	want := []string{"dir:/mit platz/repo", "--file", "/lauf/raw/x.sarif", "-c", "/skripte/gitleaks.toml", "/mit platz/repo/installer"}
	for index, value := range want {
		if args[index] != value {
			t.Errorf("Argument %d = %q, erwartet %q", index, args[index], value)
		}
	}
}

func TestScannerAppliesTo(t *testing.T) {
	fälle := []struct {
		languages string
		selected  []string
		want      bool
	}{
		{"*", nil, true},
		{"*", []string{"go"}, true},
		{"python", []string{"go"}, false},
		{"python,go", []string{"go"}, true},
		{"python", []string{"python", "go"}, true},
	}
	for _, fall := range fälle {
		scanner := Scanner{Languages: fall.languages}
		if got := scanner.AppliesTo(fall.selected); got != fall.want {
			t.Errorf("languages %q gegen %v = %v, erwartet %v", fall.languages, fall.selected, got, fall.want)
		}
	}
}

// Der ausgelieferte Katalog muss lesbar sein und zur Tool-Matrix passen: ein
// Job, dessen Werkzeug es nicht gibt, liefe nie und fiele auch nie auf.
func TestAusgelieferterKatalogPasstZurToolMatrix(t *testing.T) {
	scanners, err := LoadScanners(filepath.Join("..", "..", "..", "scripts", "scanners.tsv"))
	if err != nil {
		t.Fatalf("scanners.tsv: %v", err)
	}

	matrix, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "security-tools.tsv"))
	if err != nil {
		t.Fatalf("security-tools.tsv: %v", err)
	}
	known := map[string]bool{}
	for _, line := range strings.Split(string(matrix), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name := strings.Split(line, "\t")[0]
		if name != "name" {
			known[name] = true
		}
	}

	for _, scanner := range scanners {
		if !known[scanner.Tool] {
			t.Errorf("Job %s nennt das Werkzeug %s, das die Tool-Matrix nicht kennt", scanner.Job, scanner.Tool)
		}
	}

	// Die Kardinalität, an der die Trennung Eintrag/Job hängt.
	if count := len(ScannersFor(scanners, "gitleaks")); count != 2 {
		t.Errorf("gitleaks hat %d Jobs, erwartet 2", count)
	}
	if count := len(ScannersFor(scanners, "ruff")); count != 1 {
		t.Errorf("ruff hat %d Jobs, erwartet 1", count)
	}
	if count := len(ScannersFor(scanners, "syft")); count != 0 {
		t.Errorf("syft hat %d Jobs, erwartet 0 — es erzeugt eine SBOM, keine Befunde", count)
	}

	// Jede Zeile trägt eine Kandidatensorte — ohne sie wäre ein Ergebnis mit 0
	// Befunden nicht zu lesen. Geprüft wird die Zuordnung an je einem Vertreter
	// der vier Sorten; dass der Wert überhaupt bekannt ist, weist ParseScanners
	// schon beim Lesen ab.
	kinds := map[string]CandidateKind{}
	for _, scanner := range scanners {
		kinds[scanner.Job] = scanner.Candidates
	}
	for job, want := range map[string]CandidateKind{
		"gosec":        CandidateSource,
		"gitleaks-dir": CandidateAny,
		"osv-scanner":  CandidateManifest,
		// Die ausdrückliche Ausnahme: IaC-Konfigurationen lassen sich ohne
		// trivys eigene Erkennungslogik nicht abgrenzen.
		"trivy-config": CandidateNone,
	} {
		if kinds[job] != want {
			t.Errorf("Job %s hat candidates %q, erwartet %q", job, kinds[job], want)
		}
	}

	// Die drei Go-Jobs brauchen alle das Modul. gosec stand zunächst auf
	// target, weil es seine Verzeichnisse selbst sucht — nachgemessen prüft es
	// so aber nichts: ohne Modulkontext kommt es über das Importieren nicht
	// hinaus und schreibt ein leeres SARIF (0 Befunde gegen 154 im Modul).
	workdirs := map[string]WorkdirMode{}
	for _, scanner := range scanners {
		workdirs[scanner.Job] = scanner.Workdir
	}
	for job, want := range map[string]WorkdirMode{
		"govulncheck":   WorkdirModule,
		"golangci-lint": WorkdirModule,
		"gosec":         WorkdirModule,
	} {
		if workdirs[job] != want {
			t.Errorf("Job %s hat workdir %q, erwartet %q", job, workdirs[job], want)
		}
	}
}
