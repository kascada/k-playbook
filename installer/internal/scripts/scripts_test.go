package scripts

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot ist die Wurzel des Arbeitsstands. Die Tests laufen im
// Paketverzeichnis, also drei Ebenen tiefer.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("Wurzel auflösen: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "K-PLAYBOOK.yaml")); err != nil {
		t.Fatalf("Wurzel %s sieht nicht nach dem Arbeitsstand aus: %v", root, err)
	}
	return root
}

func scriptPath(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), "scripts", name)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Skript fehlt: %v", err)
	}
	return path
}

// result ist der Ausgang eines Skriptaufrufs. stdout und stderr bleiben
// getrennt, weil das Skript Meldungen nach stderr und die Tabelle nach stdout
// schreibt.
type result struct {
	code   int
	stdout string
	stderr string
}

func (r result) all() string {
	return r.stdout + "\n" + r.stderr
}

// runScript ruft ein Skript mit einer kontrollierten Umgebung auf. Vererbt wird
// nur, was ein Skript wirklich braucht; alles Weitere kommt aus env, damit ein
// Test nicht davon abhängt, wie die Shell des Entwicklers gesetzt ist.
func runScript(t *testing.T, script string, env []string, args ...string) result {
	t.Helper()

	command := exec.Command("bash", append([]string{script}, args...)...)
	command.Env = append([]string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"LC_ALL=C",
	}, env...)

	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()

	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("Skriptaufruf fehlgeschlagen: %v", err)
		}
		code = exitErr.ExitCode()
	}
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// runBash führt ein kurzes Bash-Programm mit Argumenten aus. Gebraucht wird
// das, um eine einzelne Funktion der gesourcten Bibliothek zu prüfen, ohne ein
// ganzes Skript zu starten.
func runBash(t *testing.T, program string, args ...string) result {
	t.Helper()

	command := exec.Command("bash", append([]string{"-c", program, "bash"}, args...)...)
	command.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"LC_ALL=C",
	}

	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()

	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("Bash-Aufruf fehlgeschlagen: %v", err)
		}
		code = exitErr.ExitCode()
	}
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// matrixEntry ist eine Zeile einer Werkzeugmatrix, soweit der Release-Weg sie
// braucht.
type matrixEntry struct {
	name     string
	ref      string
	assetRef string
	pattern  string
}

// matrixColumns sagt, in welcher Spalte was steht. Die beiden Matrizen haben
// verschiedene Zuschnitte: die Security-Matrix führt eine Referenz je Eintrag,
// die Basis-Matrix je Methode eine eigene.
type matrixColumns struct {
	name     int
	method   int
	ref      int
	assetRef int
	pattern  int
}

// githubMatrixEntries liest die Einträge einer Matrix, die einen github-Weg
// führen. Gelesen wird die ausgelieferte Datei: ein Test gegen eine Abschrift
// der Muster prüfte seine eigene Abschrift.
func githubMatrixEntries(t *testing.T, path string, columns matrixColumns) []matrixEntry {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Matrix lesen: %v", err)
	}

	var entries []matrixEntry
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) <= columns.pattern || fields[columns.name] == "name" {
			continue
		}
		methods := false
		for _, method := range strings.Split(fields[columns.method], ",") {
			if strings.TrimSpace(method) == "github" {
				methods = true
			}
		}
		if !methods {
			continue
		}

		entry := matrixEntry{
			name:    fields[columns.name],
			ref:     fields[columns.ref],
			pattern: fields[columns.pattern],
		}
		// Ohne eigene Spalte bindet {tool} an den Programmnamen — so hält es
		// die Security-Matrix.
		entry.assetRef = entry.name
		if columns.assetRef > 0 && fields[columns.assetRef] != "-" {
			entry.assetRef = fields[columns.assetRef]
		}
		entries = append(entries, entry)
	}
	return entries
}

// writeMatrix legt eine Matrix in einem eigenen Verzeichnis ab. Die Tests
// arbeiten nie gegen die ausgelieferte Matrix, wenn sie deren Inhalt variieren
// müssen — sonst hinge ein Test an einer Produktdatei.
func writeMatrix(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "matrix.tsv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("Matrix schreiben: %v", err)
	}
	return path
}
