package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/buildinfo"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/release/releasetest"
	"github.com/kascada/k-playbook/installer/internal/vscodeext"
)

// `k-playbook version` gibt nur die Version aus: eine Zeile, nichts davor und
// nichts danach. Die Oberfläche liest sie an der Datei am Installationsziel.
func TestVersionGibtNurDieVersionAus(t *testing.T) {
	before := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = before })

	buildinfo.Version = "v0.9.4"
	var out bytes.Buffer
	if err := runVersion(&out, nil); err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	if out.String() != "v0.9.4\n" {
		t.Errorf("Ausgabe = %q, erwartet genau die Version", out.String())
	}

	// Ohne gestempelte Version keine erfundene Angabe, sondern ein Fehler.
	buildinfo.Version = ""
	out.Reset()
	if err := runVersion(&out, nil); err == nil {
		t.Error("ohne Version kein Fehler")
	}
	if out.Len() != 0 {
		t.Errorf("ohne Version steht etwas auf stdout: %q", out.String())
	}
}

// Startet `k-playbook` aus einem Programm, das älter ist als das neueste
// Release, steht genau eine Zeile im Terminal, mit `k-playbook upgrade` als
// Weg. Gleich, neuer und unbekannt bleiben still; ohne eigene Version wird der
// Tag gar nicht abgerufen. Die VERSION im Clone spielt keine Rolle.
func TestStartAusAelteremProgrammMeldetEineZeile(t *testing.T) {
	root := t.TempDir()
	mustWriteMCPFile(t, project.ConfigPath(root), "schema_version: 3\n")
	if err := os.MkdirAll(filepath.Join(root, project.PlaybookDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, project.PlaybookDirName, "VERSION"), []byte("v9.9.9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(before) })
	source := releasetest.Serve(t)
	source.AddTag("v0.9.4")

	var out bytes.Buffer
	noteOutdatedProgram(&out, "v0.9.3")
	line := out.String()
	if strings.Count(line, "\n") != 1 {
		t.Errorf("nicht genau eine Zeile: %q", line)
	}
	for _, want := range []string{"v0.9.3", "v0.9.4", "neueste Release", "Programm aktualisieren", project.UpgradeCommand} {
		if !strings.Contains(line, want) {
			t.Errorf("die Zeile nennt %q nicht: %q", want, line)
		}
	}
	for _, gone := range []string{"v9.9.9", project.BootstrapCommand} {
		if strings.Contains(line, gone) {
			t.Errorf("die Zeile nennt noch %q: %q", gone, line)
		}
	}

	for _, running := range []string{"v0.9.4", "v0.10.0"} {
		out.Reset()
		noteOutdatedProgram(&out, running)
		if out.Len() != 0 {
			t.Errorf("laufend %q: %q", running, out.String())
		}
	}

	// Ohne eigene Version: still, und kein Abruf.
	asked := len(source.Requests())
	for _, running := range []string{"", "dev"} {
		out.Reset()
		noteOutdatedProgram(&out, running)
		if out.Len() != 0 {
			t.Errorf("laufend %q: %q", running, out.String())
		}
	}
	if len(source.Requests()) != asked {
		t.Errorf("ohne eigene Version wurde der Tag abgerufen: %v", source.Requests()[asked:])
	}

	// Tag unbekannt: still.
	releasetest.Off(t)
	out.Reset()
	noteOutdatedProgram(&out, "v0.9.3")
	if out.Len() != 0 {
		t.Errorf("Tag unbekannt: %q", out.String())
	}
}

// versionTerminal stellt für einen Test, ob stdout ein Terminal ist, und fängt
// den Hinweis auf, der sonst nach stderr ginge.
func versionTerminal(t *testing.T, terminal bool) *bytes.Buffer {
	t.Helper()
	beforeOut, beforeTerminal := versionHintOut, versionStdoutIsTerminal
	t.Cleanup(func() { versionHintOut, versionStdoutIsTerminal = beforeOut, beforeTerminal })
	hint := &bytes.Buffer{}
	versionHintOut = hint
	versionStdoutIsTerminal = func() bool { return terminal }
	return hint
}

// `version --all` nennt zusätzlich die Erweiterung und den neuesten
// Release-Tag — und nur hinter dem Schalter: program.ReadVersion verwirft eine
// Antwort mit mehr als einer Zeile, und mit einer zweiten Zeile am Standardweg
// erkennt kein Aufruf mehr einen Programmwechsel.
func TestVersionAllNenntErweiterungUndNeuestesRelease(t *testing.T) {
	before := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = before })
	buildinfo.Version = "v0.9.4"
	hint := versionTerminal(t, true)
	source := releasetest.Serve(t)
	source.AddTag("v0.9.4")
	source.AddTag("v0.10.0")
	source.AddTag("v0.11.0-rc.1")

	var out bytes.Buffer
	if err := runVersion(&out, []string{"--all"}); err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || lines[0] != "v0.9.4" || !strings.HasPrefix(lines[1], "Erweiterung: ") {
		t.Fatalf("Ausgabe = %q", out.String())
	}
	embedded, ok, err := vscodeext.EmbeddedVersion()
	if !ok || err != nil {
		t.Fatalf("eingebettete Version: %v, %v", ok, err)
	}
	if lines[1] != "Erweiterung: "+embedded {
		t.Errorf("zweite Zeile = %q, eingebettet %q", lines[1], embedded)
	}
	// Der höchste Tag der Form vX.Y.Z; die ersetzte Quelle wird genannt, damit
	// die Testnaht im ausgelieferten Programm nicht unbemerkt bleibt.
	if !strings.HasPrefix(lines[2], "Neuestes Release: v0.10.0 (Release-Quelle ersetzt über "+release.EnvSource+": "+source.URL()+")") {
		t.Errorf("dritte Zeile = %q", lines[2])
	}
	// --all ist schon die ausführliche Fassung: kein Hinweis darauf.
	if hint.Len() != 0 {
		t.Errorf("--all weist auf --all hin: %q", hint.String())
	}

	out.Reset()
	if err := runVersion(&out, []string{"--irgendwas"}); err == nil {
		t.Error("unbekannter Schalter ohne Fehler")
	}
}

// Ist der neueste Tag unbekannt, steht dort „unbekannt“ mit dem Grund, und
// der Exit-Code ändert sich dadurch nicht: kein Fehler.
func TestVersionAllMitUnbekanntemTag(t *testing.T) {
	before := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = before })
	buildinfo.Version = "v0.9.4"
	versionTerminal(t, false)
	releasetest.Off(t)

	var out bytes.Buffer
	if err := runVersion(&out, []string{"--all"}); err != nil {
		t.Fatalf("unbekannter Tag ändert den Exit-Code: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || lines[0] != "v0.9.4" {
		t.Fatalf("Ausgabe = %q", out.String())
	}
	if lines[2] != "Neuestes Release: unbekannt — die Release-Abfrage ist abgeschaltet ("+release.EnvSource+"="+release.SourceOff+")" {
		t.Errorf("dritte Zeile = %q", lines[2])
	}

	// Die Quelle antwortet, nennt aber keinen Release-Tag.
	releasetest.Serve(t).AddTag("v0.11.0-rc.1")
	out.Reset()
	if err := runVersion(&out, []string{"--all"}); err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	if !strings.Contains(out.String(), "Neuestes Release: unbekannt — ") {
		t.Errorf("Ausgabe = %q", out.String())
	}
}

// `version` ohne Schalter bleibt auf stdout genau eine Zeile und ruft den
// neuesten Tag nicht ab. Im Terminal weist es auf stderr auf --all hin; ist
// stdout kein Terminal — ReadVersion, ein Skript —, schweigt auch stderr.
func TestVersionWeistImTerminalAufAllHin(t *testing.T) {
	before := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = before })
	buildinfo.Version = "v0.9.4"
	source := releasetest.Serve(t)
	source.AddTag("v0.10.0")

	for _, terminal := range []bool{true, false} {
		hint := versionTerminal(t, terminal)
		var out bytes.Buffer
		if err := runVersion(&out, nil); err != nil {
			t.Fatalf("Fehler: %v", err)
		}
		if out.String() != "v0.9.4\n" {
			t.Errorf("Terminal %v: stdout = %q, erwartet genau die eine Zeile", terminal, out.String())
		}
		if terminal {
			if hint.String() != versionAllHint+"\n" || !strings.Contains(versionAllHint, "version --all") {
				t.Errorf("Hinweis = %q", hint.String())
			}
		} else if hint.Len() != 0 {
			t.Errorf("ohne Terminal ein Hinweis: %q", hint.String())
		}
	}
	if requests := source.Requests(); len(requests) != 0 {
		t.Errorf("version ohne --all hat den Tag abgerufen: %v", requests)
	}

	// Ohne Version auch im Terminal kein Hinweis: der Aufruf endet mit Fehler.
	buildinfo.Version = ""
	hint := versionTerminal(t, true)
	var out bytes.Buffer
	if err := runVersion(&out, nil); err == nil || out.Len() != 0 || hint.Len() != 0 {
		t.Errorf("ohne Version: Fehler %v, stdout %q, Hinweis %q", err, out.String(), hint.String())
	}
}

// Der Hinweis beim Start der Oberfläche liest den Cache: Ein frischer Eintrag
// kostet keine Anfrage, auch wenn die Quelle inzwischen ein neueres Release
// nennt.
func TestStartHinweisLiestDenCache(t *testing.T) {
	root := t.TempDir()
	mustWriteMCPFile(t, project.ConfigPath(root), "schema_version: 3\n")
	before, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(before) })
	source := releasetest.Serve(t)
	source.AddTag("v0.9.4")
	if _, err := release.LatestTag(context.Background()); err != nil {
		t.Fatal(err)
	}
	asked := len(source.Requests())
	source.AddTag("v0.10.0")

	var out bytes.Buffer
	noteOutdatedProgram(&out, "v0.9.4")
	if out.Len() != 0 {
		t.Errorf("aus dem Cache ist v0.9.4 das neueste, trotzdem: %q", out.String())
	}
	if len(source.Requests()) != asked {
		t.Errorf("trotz frischem Eintrag gefragt: %v", source.Requests()[asked:])
	}
}

// `version --all` fragt immer frisch, auch bei frischem Eintrag, und schreibt
// das Ergebnis in den Cache.
func TestVersionAllFragtTrotzFrischemCache(t *testing.T) {
	before := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = before })
	buildinfo.Version = "v0.9.4"
	versionTerminal(t, false)
	source := releasetest.Serve(t)
	source.AddTag("v0.9.4")
	if tag, err := release.CachedLatestTag(context.Background()); err != nil || tag != "v0.9.4" {
		t.Fatalf("CachedLatestTag = %q, %v", tag, err)
	}
	asked := len(source.Requests())
	source.AddTag("v0.10.0")

	var out bytes.Buffer
	if err := runVersion(&out, []string{"--all"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Neuestes Release: v0.10.0") || len(source.Requests()) != asked+1 {
		t.Errorf("Ausgabe %q, Anfragen %v", out.String(), source.Requests())
	}
	if tag, _ := release.CachedLatestTag(context.Background()); tag != "v0.10.0" || len(source.Requests()) != asked+1 {
		t.Errorf("der Cache trägt %q, Anfragen %v", tag, source.Requests())
	}
}
