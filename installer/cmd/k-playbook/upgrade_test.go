package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/buildinfo"
	"github.com/kascada/k-playbook/installer/internal/program"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// alteAgentsVorlage ist eine AGENTS.md, wie das Binary sie bis zum Briefing
// schrieb: Wrapper-Anstoß plus beide Zusatzblöcke, dahinter eigene Prosa ohne
// Überschrift.
const alteAgentsVorlage = `# AGENTS.md

<!-- k-playbook:anstoss -->
## k-playbook

Für dieses Projekt gilt k-playbook. Rufe zu Beginn

    k-playbook/bin/k-playbook context

auf und lies die Dateien aus ` + "`instructions`" + ` in der angegebenen Reihenfolge,
bevor du arbeitest. Die Ausgabe nennt außerdem die aufgelösten Verzeichnisse und
die effektiven Kataloge für Regeln, Reviews und Checks.

<!-- k-playbook:session-memory -->
## Projektwissen zuerst

Die autoritative Projektdokumentation beginnt bei
` + "`k-playbook-local/docs/README.md`" + `. Lies diesen Index zuerst, bevor du den
Code analysierst. Erst wenn die Dokumentation fehlt, nicht passt oder ein
konkreter Fix den aktuellen Code verlangt, ist eine Code-Recherche nötig.

<!-- k-playbook:befunde -->
## Befunde festhalten

Wenn du Code analysierst, einer Ursache nachgehst oder eine These durch einen
Test prüfst, ist die Arbeit erst abgeschlossen, wenn der Befund geschrieben ist.
Er gehört nach ` + "`k-playbook-local/material/befunde/`" + `, eine Datei je Thema; neue
Erkenntnisse werden angehängt. Nenne in deiner Antwort die geschriebene Datei.

Knapp festhalten, sobald etwas belegt ist. Ausführlicher, sobald ein Problem
erkannt und gelöst wurde — dann auch, welche Wege du ausgeschlossen hast. Jeder
Eintrag nennt Befund, Beleg (` + "`pfad:zeile`" + ` oder Testausgabe) und Sicherheit
(bestaetigt, unbestaetigt oder widerlegt). Das vollständige Format steht in
` + "`k-playbook/rules/befunde.md`" + `.

Nicht festgehalten werden Zwischenschritte und alles, was der Code selbst sagt.

Am Ende einer Sitzung schließt **/k-danke** die Arbeit ab: er legt die Befunde
vor, befördert Bestätigtes in die Dokumentation und prüft, ob die Doku
nachzuziehen ist.

Diese Zeilen gehören dem Projekt und stehen ohne Überschrift dahinter.
`

// upgradeProject baut ein Projekt mit Installation (ohne Git) und alter
// AGENTS.md auf und macht es zum Arbeitsverzeichnis.
func upgradeProjectDir(t *testing.T, version string) string {
	t.Helper()

	root := todoProject(t)
	playbook := filepath.Join(root, project.PlaybookDirName)
	for _, file := range []struct{ path, content string }{
		{filepath.Join(playbook, project.VersionFileName), version + "\n"},
		{filepath.Join(playbook, "commands", "k-test.md"), "test\n"},
		{filepath.Join(playbook, "skills", "beispiel", "SKILL.md"), "# Beispiel\n"},
		{filepath.Join(root, project.RootInstructionsFile), alteAgentsVorlage},
	} {
		if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file.path, []byte(file.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func stubUpgradeSteps(t *testing.T, update func(string) (project.UpdateResult, error),
	install func(string) (program.Result, error)) {
	t.Helper()
	beforeUpdate, beforeInstall, beforeReexec := upgradeUpdate, upgradeInstall, upgradeReexec
	t.Cleanup(func() { upgradeUpdate, upgradeInstall, upgradeReexec = beforeUpdate, beforeInstall, beforeReexec })
	if update != nil {
		upgradeUpdate = update
	}
	if install != nil {
		upgradeInstall = install
	}
}

func setOwnVersion(t *testing.T, version string) {
	t.Helper()
	before := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = before })
	buildinfo.Version = version
}

// --check meldet, was ein Lauf täte, und schreibt nichts.
func TestUpgradeCheckMeldetUndSchreibtNicht(t *testing.T) {
	root := upgradeProjectDir(t, "v0.16.8")
	setOwnVersion(t, "v0.16.8")

	stdout := captureStdout(t, func() {
		if err := runUpgrade([]string{"--check"}); err != nil {
			t.Errorf("upgrade --check: %v", err)
		}
	})
	for _, erwartet := range []string{
		"Programm-Stand: nichts zu tun",
		"veraltete Anstoß in AGENTS.md würde auf k-playbook briefing gebracht",
		"Session-Memory-Block und Befunde-Block würden aus AGENTS.md entfernt",
		"Verlinkung würde eingerichtet",
	} {
		if !strings.Contains(stdout, erwartet) {
			t.Errorf("%q fehlt in:\n%s", erwartet, stdout)
		}
	}
	if content, _ := os.ReadFile(filepath.Join(root, project.RootInstructionsFile)); string(content) != alteAgentsVorlage {
		t.Error("--check hat AGENTS.md verändert")
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "commands")); err == nil {
		t.Error("--check hat die Verlinkung angelegt")
	}
}

// Der Lauf ohne Clone-Git: der Pull entfällt mit Meldung, das Projekt wird
// trotzdem nachgezogen — AGENTS.md auf die neue Vorlage, Prosa bleibt,
// CLAUDE.md ist die Include-Datei —, und der zweite Lauf hat nichts zu tun.
func TestUpgradeZiehtProjektNach(t *testing.T) {
	root := upgradeProjectDir(t, "v0.16.8")
	setOwnVersion(t, "v0.16.8")

	stdout := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("upgrade: %v", err)
		}
	})
	for _, erwartet := range []string{
		"kein Git-Repository; der Pull entfällt",
		"Programm-Stand: nichts zu tun",
		"veraltete Anstoß in AGENTS.md wurde auf k-playbook briefing gebracht",
		"Session-Memory-Block und Befunde-Block aus AGENTS.md entfernt",
		"Verlinkung eingerichtet",
	} {
		if !strings.Contains(stdout, erwartet) {
			t.Errorf("%q fehlt in:\n%s", erwartet, stdout)
		}
	}
	if strings.Contains(stdout, "neu starten") {
		t.Errorf("ohne Binärwechsel kein Neustart-Hinweis:\n%s", stdout)
	}

	content, err := os.ReadFile(filepath.Join(root, project.RootInstructionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "\n    k-playbook briefing\n") ||
		strings.Contains(string(content), "k-playbook context") ||
		strings.Contains(string(content), "k-playbook:session-memory") ||
		strings.Contains(string(content), "k-playbook:befunde") ||
		!strings.HasSuffix(string(content), "Was du sonst brauchst, erreichst du über die Verweise darin.\n\nDiese Zeilen gehören dem Projekt und stehen ohne Überschrift dahinter.\n") {
		t.Errorf("AGENTS.md nach dem Lauf:\n%s", content)
	}
	claude, err := os.ReadFile(filepath.Join(root, project.ClaudeInstructionsFile))
	if err != nil || !strings.Contains(string(claude), "\n"+project.ClaudeIncludeLine+"\n") {
		t.Errorf("CLAUDE.md ist keine Include-Datei: %v\n%s", err, claude)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "commands", "k-test.md")); err != nil {
		t.Errorf("die Verlinkung steht nicht: %v", err)
	}

	second := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("zweiter Lauf: %v", err)
		}
	})
	if !strings.Contains(second, "Projekt: nichts zu tun.") {
		t.Errorf("der zweite Lauf meldet nicht „nichts zu tun“:\n%s", second)
	}
	after, _ := os.ReadFile(filepath.Join(root, project.RootInstructionsFile))
	if string(after) != string(content) {
		t.Error("der zweite Lauf hat AGENTS.md verändert")
	}
}

// Wechselt das Binary, führt nicht dieser Prozess Schritt (2) aus, sondern das
// neu installierte Programm — mit seinen Vorlagen. Das Stub-Programm schreibt
// seine Spur in AGENTS.md; die alte Vorlage dieses Prozesses darf nicht
// darüberlaufen. Am Ende steht der Hinweis auf den Neustart der Sitzung.
func TestUpgradeFuehrtProjektschrittMitNeuemProgrammAus(t *testing.T) {
	root := upgradeProjectDir(t, "v0.17.0")
	setOwnVersion(t, "v0.16.8")

	stub := filepath.Join(t.TempDir(), "k-playbook-neu")
	script := "#!/bin/sh\n" +
		"[ \"$1\" = upgrade ] && [ \"$2\" = --project-only ] || exit 7\n" +
		"printf 'Projekt: nachgezogen vom Stub.\\n'\n" +
		"printf '# vom neuen Programm\\n' > " + filepath.Join(root, project.RootInstructionsFile) + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	stubUpgradeSteps(t,
		func(string) (project.UpdateResult, error) {
			return project.UpdateResult{VersionChanged: true, Version: "v0.17.0"}, nil
		},
		func(string) (program.Result, error) {
			return program.Result{Target: stub, Expected: "v0.17.0", Bootstrapped: true}, nil
		})
	// Den Pull nicht entfallen lassen: der Clone bekommt ein .git-Verzeichnis.
	if err := os.MkdirAll(filepath.Join(root, project.PlaybookDirName, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("upgrade: %v", err)
		}
	})
	for _, erwartet := range []string{
		"VERSION jetzt v0.17.0",
		"v0.17.0 installiert nach " + stub,
		"Projekt: nachgezogen vom Stub.",
		"Das Programm hat gewechselt: die Sitzung neu starten",
	} {
		if !strings.Contains(stdout, erwartet) {
			t.Errorf("%q fehlt in:\n%s", erwartet, stdout)
		}
	}
	content, _ := os.ReadFile(filepath.Join(root, project.RootInstructionsFile))
	if string(content) != "# vom neuen Programm\n" {
		t.Errorf("der alte Prozess hat Schritt (2) selbst ausgeführt:\n%s", content)
	}
}

// Scheitert Download oder Install, bleibt das eine eigene Meldung mit dem Weg
// im Terminal; Schritt (2) läuft trotzdem, mit dem laufenden Programm.
func TestUpgradeInstallfehlerIstEigeneMeldung(t *testing.T) {
	root := upgradeProjectDir(t, "v0.17.0")
	setOwnVersion(t, "v0.16.8")
	stubUpgradeSteps(t,
		func(string) (project.UpdateResult, error) {
			return project.UpdateResult{Version: "v0.17.0"}, nil
		},
		func(string) (program.Result, error) {
			return program.Result{Output: "curl: (6) Could not resolve host"}, os.ErrDeadlineExceeded
		})
	if err := os.MkdirAll(filepath.Join(root, project.PlaybookDirName, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("upgrade: %v", err)
		}
	})
	for _, erwartet := range []string{
		"Download/Install nicht möglich",
		project.BootstrapHint,
		"Could not resolve host",
		"veraltete Anstoß in AGENTS.md wurde auf k-playbook briefing gebracht",
	} {
		if !strings.Contains(stdout, erwartet) {
			t.Errorf("%q fehlt in:\n%s", erwartet, stdout)
		}
	}
	if strings.Contains(stdout, "neu starten") {
		t.Errorf("ohne Binärwechsel kein Neustart-Hinweis:\n%s", stdout)
	}
}

// --json trägt dieselben Meldungen als Liste und die Teilergebnisse.
func TestUpgradeJSON(t *testing.T) {
	upgradeProjectDir(t, "v0.16.8")
	setOwnVersion(t, "v0.16.8")

	stdout := captureStdout(t, func() {
		if err := runUpgrade([]string{"--check", "--json"}); err != nil {
			t.Errorf("upgrade --check --json: %v", err)
		}
	})
	for _, erwartet := range []string{`"schemaVersion": "1"`, `"check": true`, `"hasOutdatedAnstoss": true`, `"Session-Memory-Block"`, `"messages": [`} {
		if !strings.Contains(stdout, erwartet) {
			t.Errorf("%q fehlt in:\n%s", erwartet, stdout)
		}
	}
	if strings.Contains(stdout, "Programm-Stand") && !strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Errorf("JSON-Modus schreibt Klartext davor:\n%s", stdout)
	}
}
