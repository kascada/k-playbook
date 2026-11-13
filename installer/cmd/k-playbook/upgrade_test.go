package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/buildinfo"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
	"github.com/kascada/k-playbook/installer/internal/program"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/release/releasetest"
	"github.com/kascada/k-playbook/installer/internal/sharedir/sharetest"
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
// AGENTS.md auf und macht es zum Arbeitsverzeichnis. latest ist der neueste
// Release-Tag: Eine Ersatzquelle nennt ihn, ohne Assets — kein Test fragt das
// echte GitHub. Die VERSION im Clone weicht bewusst ab: Sie wird nicht gelesen.
func upgradeProjectDir(t *testing.T, latest string) string {
	t.Helper()

	root := todoProject(t)
	playbook := filepath.Join(root, project.PlaybookDirName)
	releasetest.Serve(t).AddTag(latest)
	for _, file := range []struct{ path, content string }{
		{filepath.Join(playbook, "VERSION"), "v0.0.1\n"},
		{filepath.Join(sharetest.Dir(t), "commands", "k-test.md"), "test\n"},
		{filepath.Join(sharetest.Dir(t), "skills", "beispiel", "SKILL.md"), "# Beispiel\n"},
		{filepath.Join(root, project.RootInstructionsFile), alteAgentsVorlage},
	} {
		if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file.path, []byte(file.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Ob kephalaion im PATH liegt und was es antwortet, hängt sonst am
	// Rechner des Laufs.
	stubKephalaion(t, kephalaionVorhanden, func(context.Context) (program.KephalaionResult, error) {
		return program.KephalaionResult{Path: "/opt/bin/kephalaion"}, nil
	})
	stubKephalaionUpgrade(t, kephalaionAutoAktuell, kephalaionCheckAktuell)
	return root
}

func kephalaionAutoAktuell(string) (program.KephalaionAuto, error) {
	return program.KephalaionAuto{Outcome: program.KephalaionCurrent, From: "v0.4.1", To: "v0.4.1"}, nil
}

func kephalaionCheckAktuell(string) (program.KephalaionCheck, error) {
	return program.KephalaionCheck{State: "ok", Version: "v0.4.1", Latest: "v0.4.1"}, nil
}

// stubKephalaionUpgrade ersetzt die Aufrufe von kephalaion upgrade --auto und
// --check.
func stubKephalaionUpgrade(t *testing.T, auto func(string) (program.KephalaionAuto, error), check func(string) (program.KephalaionCheck, error)) {
	t.Helper()
	beforeAuto, beforeCheck := upgradeAutoKephalaion, upgradeCheckKephalaion
	t.Cleanup(func() { upgradeAutoKephalaion, upgradeCheckKephalaion = beforeAuto, beforeCheck })
	upgradeAutoKephalaion, upgradeCheckKephalaion = auto, check
}

// fakeKephalaionUpgrade legt ein kephalaion an, das upgrade --auto und
// --check spielt und seine Aufrufe festhält, und lässt den Lauf es über die
// echten Aufrufe aus internal/program finden. Installiert werden darf nichts.
func fakeKephalaionUpgrade(t *testing.T) *kephtest.FakeUpgrade {
	t.Helper()
	fake := kephtest.NewFakeUpgrade(t)
	stubKephalaion(t, func() (string, bool) { return fake.Path, true }, func(context.Context) (program.KephalaionResult, error) {
		t.Error("kephalaion ist da und wurde trotzdem installiert")
		return program.KephalaionResult{}, nil
	})
	stubKephalaionUpgrade(t, program.KephalaionAutoUpgrade, program.KephalaionCheckUpgrade)
	return fake
}

func kephalaionVorhanden() (string, bool) { return "/opt/bin/kephalaion", true }

func kephalaionFehlt() (string, bool) { return "", false }

// stubKephalaion ersetzt die Suche nach kephalaion und seine Installation.
func stubKephalaion(t *testing.T, find func() (string, bool), install func(context.Context) (program.KephalaionResult, error)) {
	t.Helper()
	beforeFind, beforeInstall := upgradeFindKephalaion, upgradeInstallKephalaion
	t.Cleanup(func() { upgradeFindKephalaion, upgradeInstallKephalaion = beforeFind, beforeInstall })
	upgradeFindKephalaion, upgradeInstallKephalaion = find, install
}

func stubUpgradeSteps(t *testing.T, update func(string) (project.UpdateResult, error),
	install func(context.Context, string) (program.Result, error)) {
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

// installTargetInPath legt ein eigenes Home an, in dem ~/.local/bin/k-playbook
// existiert und im PATH steht. Ohne das hängt der Ausgang an der Umgebung des
// Laufs: program.CheckStatus hält das Programm nur dann für installierbar, wenn
// `k-playbook` im PATH auf das Installationsziel zeigt — lokal meist der Fall,
// auf dem GitHub-Runner nicht, und dort fielen die Tests mit Installation und
// Re-Exec durch.
func installTargetInPath(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	target, err := project.InstallTargetPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(target)+string(os.PathListSeparator)+os.Getenv("PATH"))
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
	// Die README unter keph:/docs/ übergeht --check; die meldet /k-docs.
	if strings.Contains(stdout, "README unter keph:/docs/") {
		t.Errorf("--check fasst die README an:\n%s", stdout)
	}
}

// Ein Block, dessen Ende sich nicht bestimmen lässt, bleibt bei einem Lauf
// stehen — und --check sagt genau das: nur der Hinweis, kein „würde
// ersetzt/entfernt". Einmal für den Befunde-Block (Fixture wie
// TestAbgewandelterZusatzblockBleibtMitHinweis in internal/project), einmal
// für den Anstoß.
func TestUpgradeCheckMeldetUnbestimmbarenBlockNurAlsHinweis(t *testing.T) {
	t.Run("Zusatzblock", func(t *testing.T) {
		root := upgradeProjectDir(t, "v0.16.8")
		setOwnVersion(t, "v0.16.8")
		agents := filepath.Join(root, project.RootInstructionsFile)
		// Die Vorlage in der Briefing-Fassung kommt vom Binary selbst, dahinter
		// ein abgewandelter Befunde-Block mit Prosa ohne Überschrift.
		if err := os.Remove(agents); err != nil {
			t.Fatal(err)
		}
		if _, err := project.ApplyRootInstructions(root); err != nil {
			t.Fatalf("Vorlage anlegen: %v", err)
		}
		vorlage, err := os.ReadFile(agents)
		if err != nil {
			t.Fatal(err)
		}
		eigen := string(vorlage) + "\n<!-- k-playbook:befunde -->\n## Befunde festhalten\n\n" +
			"Bei uns umformuliert, und dahinter steht Prosa ohne Überschrift.\n"
		if err := os.WriteFile(agents, []byte(eigen), 0o644); err != nil {
			t.Fatal(err)
		}

		stdout := captureStdout(t, func() {
			if err := runUpgrade([]string{"--check"}); err != nil {
				t.Errorf("upgrade --check: %v", err)
			}
		})
		for _, erwartet := range []string{"von Hand:", "Befunde-Block", "der Block bleibt stehen"} {
			if !strings.Contains(stdout, erwartet) {
				t.Errorf("%q fehlt in:\n%s", erwartet, stdout)
			}
		}
		for _, verboten := range []string{"würden aus AGENTS.md entfernt", "würde auf k-playbook briefing gebracht"} {
			if strings.Contains(stdout, verboten) {
				t.Errorf("--check verspricht %q, obwohl ein Lauf den Block stehen ließe:\n%s", verboten, stdout)
			}
		}
		if content, _ := os.ReadFile(agents); string(content) != eigen {
			t.Error("--check hat AGENTS.md verändert")
		}
	})

	t.Run("Anstoß", func(t *testing.T) {
		root := upgradeProjectDir(t, "v0.16.8")
		setOwnVersion(t, "v0.16.8")
		agents := filepath.Join(root, project.RootInstructionsFile)
		eigen := "# AGENTS.md\n\n<!-- k-playbook:anstoss -->\n## k-playbook\n\n" +
			"Für dieses Projekt gilt k-playbook, bei uns umformuliert. Rufe zu Beginn\n\n" +
			"    k-playbook context\n\n" +
			"auf und lies die Dateien aus `instructions`.\n\n" +
			"Diese Zeilen gehören dem Projekt und stehen ohne Überschrift dahinter.\n"
		if err := os.WriteFile(agents, []byte(eigen), 0o644); err != nil {
			t.Fatal(err)
		}

		stdout := captureStdout(t, func() {
			if err := runUpgrade([]string{"--check"}); err != nil {
				t.Errorf("upgrade --check: %v", err)
			}
		})
		for _, erwartet := range []string{"von Hand:", "Anstoßblocks", "bleibt unangetastet"} {
			if !strings.Contains(stdout, erwartet) {
				t.Errorf("%q fehlt in:\n%s", erwartet, stdout)
			}
		}
		if strings.Contains(stdout, "würde auf k-playbook briefing gebracht") {
			t.Errorf("--check verspricht den Ersatz, obwohl ein Lauf den Anstoß stehen ließe:\n%s", stdout)
		}
		if content, _ := os.ReadFile(agents); string(content) != eigen {
			t.Error("--check hat AGENTS.md verändert")
		}
	})
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
		// Ohne Anbindung des Testprojekts: der README-Schritt nur als Hinweis.
		"Projekt: README unter keph:/docs/: nicht versucht, Kephalaion ist für dieses Projekt nicht bereit",
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
	if !strings.Contains(second, "Projekt: nichts zu tun.") || !strings.Contains(second, "Projekt: README unter keph:/docs/") {
		t.Errorf("der zweite Lauf meldet nicht „nichts zu tun“ und den README-Hinweis:\n%s", second)
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
	installTargetInPath(t)
	setOwnVersion(t, "v0.16.8")

	stub := filepath.Join(t.TempDir(), "k-playbook-neu")
	// Das neue Programm repariert die MCP-Einträge im Projektschritt selbst:
	// kein --repair-mcp.
	script := "#!/bin/sh\n" +
		"[ \"$1\" = upgrade ] && [ \"$2\" = --project-only ] && [ $# = 2 ] || exit 7\n" +
		"printf 'Projekt: nachgezogen vom Stub.\\n'\n" +
		"printf '# vom neuen Programm\\n' > " + filepath.Join(root, project.RootInstructionsFile) + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	stubUpgradeSteps(t,
		func(string) (project.UpdateResult, error) {
			return project.UpdateResult{}, nil
		},
		func(_ context.Context, tag string) (program.Result, error) {
			if tag != "v0.17.0" {
				t.Errorf("installiert wird %q, erwartet der neueste Tag v0.17.0", tag)
			}
			return program.Result{Target: stub, Expected: "v0.17.0", Installed: true}, nil
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
		"Clone k-playbook/ per Pull auf dem Stand seines Remotes",
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

// Scheitert der Pull (hier: an lokalen Änderungen), wird trotzdem verglichen
// und installiert: Der Pull dient nur noch den Inhalten des Clones. Das neue
// Programm zieht das Projekt nach — ohne --repair-mcp, denn es repariert die
// MCP-Einträge im Projektschritt immer.
func TestUpgradeInstalliertNachGescheitertemPull(t *testing.T) {
	root := upgradeProjectDir(t, "v0.17.0")
	installTargetInPath(t)
	setOwnVersion(t, "v0.16.8")

	stub := filepath.Join(t.TempDir(), "k-playbook-neu")
	script := "#!/bin/sh\n" +
		"[ \"$1\" = upgrade ] && [ \"$2\" = --project-only ] && [ $# = 2 ] || exit 7\n" +
		"printf 'Projekt: nachgezogen vom Stub.\\n'\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	stubUpgradeSteps(t,
		func(string) (project.UpdateResult, error) {
			return project.UpdateResult{Output: "error: Your local changes would be overwritten"}, os.ErrInvalid
		},
		func(context.Context, string) (program.Result, error) {
			return program.Result{Target: stub, Expected: "v0.17.0", Installed: true}, nil
		})
	if err := os.MkdirAll(filepath.Join(root, project.PlaybookDirName, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("upgrade: %v", err)
		}
	})
	assertOrder(t, stdout,
		"Pull im Clone fehlgeschlagen",
		"Your local changes would be overwritten",
		"v0.17.0 installiert nach "+stub,
		"Projekt: nachgezogen vom Stub.",
		"Das Programm hat gewechselt: die Sitzung neu starten",
	)
	if strings.Contains(stdout, "konnte das Projekt nicht nachziehen") {
		t.Errorf("das neue Programm wurde mit --repair-mcp gerufen:\n%s", stdout)
	}
}

// --repair-mcp war der Schalter des Re-Exec und gilt nur mit --project-only.
// Dort wird er weiter angenommen — ein Binary von vor Task 149 startet das neu
// installierte Programm noch so —, entscheidet aber nichts mehr: Der
// Projektschritt läuft mit und ohne ihn gleich.
func TestUpgradeRepairMCPNurMitProjectOnly(t *testing.T) {
	upgradeProjectDir(t, "v0.16.8")
	err := runUpgrade([]string{"--repair-mcp"})
	if err == nil || !strings.Contains(err.Error(), "--project-only") {
		t.Errorf("erwartet eine Ablehnung von --repair-mcp ohne --project-only, bekommen: %v", err)
	}

	stdout := captureStdout(t, func() {
		if err := runUpgrade([]string{"--project-only", "--repair-mcp"}); err != nil {
			t.Errorf("upgrade --project-only --repair-mcp: %v", err)
		}
	})
	if !strings.Contains(stdout, "Verlinkung eingerichtet") || strings.Contains(stdout, "Programm") {
		t.Errorf("--project-only --repair-mcp zieht das Projekt nicht nach oder fasst das Programm an:\n%s", stdout)
	}
}

// Scheitert Download oder Install, bleibt das eine eigene Meldung mit dem Weg
// im Terminal; Schritt (2) läuft trotzdem, mit dem laufenden Programm.
func TestUpgradeInstallfehlerIstEigeneMeldung(t *testing.T) {
	root := upgradeProjectDir(t, "v0.17.0")
	installTargetInPath(t)
	setOwnVersion(t, "v0.16.8")
	stubUpgradeSteps(t,
		func(string) (project.UpdateResult, error) {
			return project.UpdateResult{}, nil
		},
		func(context.Context, string) (program.Result, error) {
			return program.Result{Output: "Geladen: https://example.invalid/k-playbook"}, os.ErrDeadlineExceeded
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
		// Der Ausweg ist die Erstinstallation: upgrade selbst ist gescheitert.
		"Nachholen im Terminal: " + project.FirstInstallCommand,
		"Geladen: https://example.invalid/k-playbook",
		"veraltete Anstoß in AGENTS.md wurde auf k-playbook briefing gebracht",
	} {
		if !strings.Contains(stdout, erwartet) {
			t.Errorf("%q fehlt in:\n%s", erwartet, stdout)
		}
	}
	if strings.Contains(stdout, "neu starten") {
		t.Errorf("ohne Binärwechsel kein Neustart-Hinweis:\n%s", stdout)
	}
	if strings.Contains(stdout, project.BootstrapCommand) {
		t.Errorf("die Meldung nennt noch bin/install des Clones:\n%s", stdout)
	}
}

// --json trägt dieselben Meldungen als Liste und die Teilergebnisse.
// Fehlt kephalaion, installiert der Lauf es in einem eigenen Schritt — auch
// dann, wenn das Programm aktuell ist und darum gar nicht installiert wird.
func TestUpgradeInstalliertFehlendesKephalaion(t *testing.T) {
	upgradeProjectDir(t, "v0.16.8")
	setOwnVersion(t, "v0.16.8")
	called := false
	stubKephalaion(t, kephalaionFehlt, func(context.Context) (program.KephalaionResult, error) {
		called = true
		return program.KephalaionResult{Path: "/home/u/.local/bin/kephalaion", Installed: true, Output: "Installiert: /home/u/.local/bin/kephalaion"}, nil
	})

	stdout := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("upgrade: %v", err)
		}
	})
	if !called {
		t.Fatal("der Lauf hat Kephalaion nicht installiert")
	}
	for _, erwartet := range []string{"Programm-Stand: nichts zu tun", "Kephalaion: installiert nach /home/u/.local/bin/kephalaion.", "  Installiert: /home/u/.local/bin/kephalaion", "Verlinkung eingerichtet"} {
		if !strings.Contains(stdout, erwartet) {
			t.Errorf("%q fehlt in:\n%s", erwartet, stdout)
		}
	}
	if strings.Index(stdout, "Kephalaion:") > strings.Index(stdout, "Programm-Stand") ||
		strings.Index(stdout, "Kephalaion:") > strings.Index(stdout, "Verlinkung eingerichtet") {
		t.Errorf("Kephalaion steht nicht vor Programm- und Projektschritt:\n%s", stdout)
	}
}

// Scheitert die Installation, ist das eine eigene Meldung mit dem Weg im
// Terminal; das Projekt wird trotzdem nachgezogen.
func TestUpgradeKephalaionfehlerHaeltDenLaufNichtAuf(t *testing.T) {
	upgradeProjectDir(t, "v0.16.8")
	setOwnVersion(t, "v0.16.8")
	stubKephalaion(t, kephalaionFehlt, func(context.Context) (program.KephalaionResult, error) {
		return program.KephalaionResult{Installed: true, Output: "kephalaion-Installation: Keine Verbindung"}, errors.New("das install.sh von Kephalaion endete mit Exit-Code 1")
	})

	stdout := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("upgrade: %v", err)
		}
	})
	for _, erwartet := range []string{
		// Der Weg von Hand ist das install.sh von Kephalaion selbst, nicht mehr
		// das Skript des Clones.
		"Kephalaion: nicht installiert — das install.sh von Kephalaion endete mit Exit-Code 1. Nachholen im Terminal: " + kephalaion.InstallCommand,
		"  kephalaion-Installation: Keine Verbindung",
		"Verlinkung eingerichtet",
	} {
		if !strings.Contains(stdout, erwartet) {
			t.Errorf("%q fehlt in:\n%s", erwartet, stdout)
		}
	}
}

// --check meldet ein fehlendes kephalaion und installiert es nicht.
func TestUpgradeCheckMeldetFehlendesKephalaion(t *testing.T) {
	upgradeProjectDir(t, "v0.16.8")
	setOwnVersion(t, "v0.16.8")
	stubKephalaion(t, kephalaionFehlt, func(context.Context) (program.KephalaionResult, error) {
		t.Error("--check hat installiert")
		return program.KephalaionResult{}, nil
	})

	stdout := captureStdout(t, func() {
		if err := runUpgrade([]string{"--check", "--json"}); err != nil {
			t.Errorf("upgrade --check: %v", err)
		}
	})
	for _, erwartet := range []string{`"kephalaion": {`, `"present": false`, "Kephalaion: fehlt im PATH; ein Lauf installiert es über das install.sh des neuesten Kephalaion-Releases"} {
		if !strings.Contains(stdout, erwartet) {
			t.Errorf("%q fehlt in:\n%s", erwartet, stdout)
		}
	}
}

func TestUpgradeJSON(t *testing.T) {
	upgradeProjectDir(t, "v0.16.8")
	setOwnVersion(t, "v0.16.8")

	stdout := captureStdout(t, func() {
		if err := runUpgrade([]string{"--check", "--json"}); err != nil {
			t.Errorf("upgrade --check --json: %v", err)
		}
	})
	for _, erwartet := range []string{`"schemaVersion": "2"`, `"check": true`, `"hasOutdatedAnstoss": true`, `"Session-Memory-Block"`, `"messages": [`} {
		if !strings.Contains(stdout, erwartet) {
			t.Errorf("%q fehlt in:\n%s", erwartet, stdout)
		}
	}
	if strings.Contains(stdout, "Programm-Stand") && !strings.HasPrefix(strings.TrimSpace(stdout), "{") {
		t.Errorf("JSON-Modus schreibt Klartext davor:\n%s", stdout)
	}
}

// Der Projektschritt zieht die Anbindung aus der alten
// k-playbook-local/K-PLAYBOOK.local.yaml um (Task 138): --check schreibt
// nichts, ein Lauf schreibt Hub und Collection in den Block kephalaion: von
// K-PLAYBOOK.yaml — nie den Account — und nennt ihn, wenn Kephalaion für den
// Hub keinen wählt. Ein vorhandener Block bleibt.
func TestUpgradeZiehtAnbindungUm(t *testing.T) {
	root := upgradeProjectDir(t, "v0.16.8")
	setOwnVersion(t, "v0.16.8")
	legacy := filepath.Join(project.LocalDir(root), kephalaion.LegacyFileName)
	if err := os.WriteFile(legacy, []byte("# Kopf\nkephalaion:\n  hub: vm\n  collection: omni\n  account: kamran-wsl\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tokens := filepath.Join(t.TempDir(), "tokens")
	kephtest.NewFakeClient(t).SetInfo(t, kephtest.Info("http://127.0.0.1:1/mcp", tokens, kephalaion.ClientHub{
		Alias: "vm", Choice: kephalaion.ChoiceMissing, Candidates: []string{"a", "b"}, Pending: []string{}, Notes: []string{}}))
	before, err := os.ReadFile(project.ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}

	check := captureStdout(t, func() {
		if err := runUpgrade([]string{"--check"}); err != nil {
			t.Errorf("upgrade --check: %v", err)
		}
	})
	for _, want := range []string{
		"Projekt: Anbindung — ein Lauf schriebe hub vm, collection omni aus k-playbook-local/K-PLAYBOOK.local.yaml in den Block kephalaion: von .k-playbook/K-PLAYBOOK.yaml.",
		"Projekt: Account — die alte Datei nannte für den Hub vm den Account kamran-wsl; Kephalaion wählt dort keinen.",
		"kephalaion node account default vm kamran-wsl",
	} {
		if !strings.Contains(check, want) {
			t.Errorf("--check: %q fehlt in:\n%s", want, check)
		}
	}
	if after, _ := os.ReadFile(project.ConfigPath(root)); string(after) != string(before) {
		t.Fatalf("--check hat geschrieben:\n%s", after)
	}

	run := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("upgrade: %v", err)
		}
	})
	for _, want := range []string{
		"Projekt: Anbindung umgezogen — hub vm, collection omni steht jetzt im Block kephalaion: von .k-playbook/K-PLAYBOOK.yaml",
		project.KephalaionCommitHint,
		"kephalaion node account default vm kamran-wsl",
	} {
		if !strings.Contains(run, want) {
			t.Errorf("Lauf: %q fehlt in:\n%s", want, run)
		}
	}
	after, _ := os.ReadFile(project.ConfigPath(root))
	if !strings.HasPrefix(string(after), string(before)) || !strings.Contains(string(after), "  hub: vm\n  collection: omni\n") ||
		strings.Contains(string(after), "account") || strings.Contains(string(after), "kamran-wsl") {
		t.Fatalf("K-PLAYBOOK.yaml nach dem Lauf:\n%s", after)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatal("die alte Datei löscht /k-upgrade, nicht k-playbook upgrade")
	}

	// Der zweite Lauf: Der Block steht und bleibt; die Zeile sagt das.
	second := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("zweiter Lauf: %v", err)
		}
	})
	if !strings.Contains(second, "Projekt: Anbindung — der Block kephalaion: von .k-playbook/K-PLAYBOOK.yaml steht (hub vm, collection omni)") {
		t.Errorf("zweiter Lauf:\n%s", second)
	}

	// Ein abweichender Block bleibt; die Meldung nennt beide Werte. Wählt
	// Kephalaion einen anderen Account, steht auch der da.
	changed := strings.Replace(string(after), "collection: omni", "collection: anders", 1)
	if err := os.WriteFile(project.ConfigPath(root), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(tokens, "vm", "zweit.token")
	account := "zweit"
	kephtest.NewFakeClient(t).SetInfo(t, kephtest.Info("http://127.0.0.1:1/mcp", tokens, kephalaion.ClientHub{
		Alias: "vm", Account: &account, Choice: kephalaion.ChoiceDefault, TokenFile: &other,
		Candidates: []string{"kamran-wsl", "zweit"}, Pending: []string{}, Notes: []string{}}))
	third := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("dritter Lauf: %v", err)
		}
	})
	for _, want := range []string{
		"steht (hub vm, collection anders) und bleibt; k-playbook-local/K-PLAYBOOK.local.yaml nennt anderes (hub vm, collection omni)",
		"Kephalaion wählt dort zweit (Standard)",
	} {
		if !strings.Contains(third, want) {
			t.Errorf("abweichend: %q fehlt in:\n%s", want, third)
		}
	}
	if final, _ := os.ReadFile(project.ConfigPath(root)); string(final) != changed {
		t.Fatalf("vorhandener Block verändert:\n%s", final)
	}
}

// --- Schritt Kephalaion: kephalaion upgrade --auto (Task 141) ---

var (
	upgradeAutoArgs  = []string{"upgrade", "--auto", "--json"}
	upgradeCheckArgs = []string{"upgrade", "--check", "--json"}
)

func pointerTo(value bool) *bool { return &value }

// runUpgradeJSON führt den Lauf mit --json aus und liest den Bericht.
func runUpgradeJSON(t *testing.T, args ...string) upgradeReport {
	t.Helper()
	stdout := captureStdout(t, func() {
		if err := runUpgrade(append(args, "--json")); err != nil {
			t.Errorf("upgrade %v: %v", args, err)
		}
	})
	var report upgradeReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("Bericht ist kein JSON: %v\n%s", err, stdout)
	}
	if report.Kephalaion == nil {
		t.Fatalf("Bericht ohne kephalaion:\n%s", stdout)
	}
	return report
}

// assertOrder prüft, dass die Teile in dieser Reihenfolge in text stehen.
func assertOrder(t *testing.T, text string, parts ...string) {
	t.Helper()
	last := -1
	for _, part := range parts {
		index := strings.Index(text, part)
		if index < 0 {
			t.Errorf("%q fehlt in:\n%s", part, text)
			return
		}
		if index < last {
			t.Errorf("%q steht nicht nach dem Vorigen in:\n%s", part, text)
			return
		}
		last = index
	}
}

// Kephalaion ist der erste Schritt, vor Programm und Projekt; der Lauf ruft
// genau einmal upgrade --auto --json auf.
func TestUpgradeKephalaionVorProgrammUndProjekt(t *testing.T) {
	upgradeProjectDir(t, "v0.16.8")
	setOwnVersion(t, "v0.16.8")
	fake := fakeKephalaionUpgrade(t)
	fake.SetAuto(t, kephtest.JSON(t, program.KephalaionAuto{Outcome: "current", From: "v0.4.1", To: "v0.4.1"}), "", 0)

	stdout := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("upgrade: %v", err)
		}
	})
	assertOrder(t, stdout, "Kephalaion: v0.4.1 ist aktuell.", "Programm-Stand: nichts zu tun", "Verlinkung eingerichtet")
	if calls := fake.Calls(t); !reflect.DeepEqual(calls, [][]string{upgradeAutoArgs}) {
		t.Errorf("Aufrufe von kephalaion = %q, erwartet genau upgrade --auto --json", calls)
	}
	if strings.Contains(stdout, "neu starten") {
		t.Errorf("bei current kein Neustart-Hinweis:\n%s", stdout)
	}
}

// --check ruft nur upgrade --check --json auf, nie --auto, und meldet
// Kephalaion vor dem Programm.
func TestUpgradeCheckRuftKeinAuto(t *testing.T) {
	upgradeProjectDir(t, "v0.16.8")
	setOwnVersion(t, "v0.16.8")
	fake := fakeKephalaionUpgrade(t)
	fake.SetAuto(t, kephtest.JSON(t, program.KephalaionAuto{Outcome: "upgraded", From: "v0.4.1", To: "v0.5.0", Restarted: true}), "", 0)
	fake.SetCheck(t, kephtest.JSON(t, program.KephalaionCheck{State: "ok", Version: "v0.4.1", Latest: "v0.5.0", UpdateAvailable: true, Hint: "kephalaion upgrade"}), "", 0)

	stdout := captureStdout(t, func() {
		if err := runUpgrade([]string{"--check"}); err != nil {
			t.Errorf("upgrade --check: %v", err)
		}
	})
	assertOrder(t, stdout, "Kephalaion: v0.4.1 installiert, v0.5.0 verfügbar", "Programm-Stand: nichts zu tun")
	if calls := fake.Calls(t); !reflect.DeepEqual(calls, [][]string{upgradeCheckArgs}) {
		t.Errorf("Aufrufe von kephalaion = %q, erwartet nur upgrade --check --json", calls)
	}
	if strings.Contains(stdout, "neu starten") {
		t.Errorf("--check verlangt keinen Neustart:\n%s", stdout)
	}
}

// restartRequired folgt dem Neustart des Nodes: bei upgraded (auch ohne
// Programmwechsel) und bei failed mit Neustart und Rückweg, nicht bei failed
// ohne Neustart. Programm- und Projektschritt laufen in jedem Fall.
func TestUpgradeKephalaionNeustartDerSitzung(t *testing.T) {
	for _, fall := range []struct {
		name     string
		auto     program.KephalaionAuto
		exit     int
		restart  bool
		erwartet []string
	}{
		{
			name:    "upgraded",
			auto:    program.KephalaionAuto{Outcome: "upgraded", From: "v0.4.1", To: "v0.5.0", Restarted: true, Warnings: []string{"VS Code: Erweiterung nicht erneuert"}},
			restart: true,
			erwartet: []string{
				"Kephalaion: von v0.4.1 auf v0.5.0 aktualisiert, Dienst neu gestartet.",
				"  VS Code: Erweiterung nicht erneuert",
			},
		},
		{
			name:     "upgraded ohne Dienst",
			auto:     program.KephalaionAuto{Outcome: "upgraded", From: "v0.4.1", To: "v0.5.0"},
			restart:  true,
			erwartet: []string{"Kephalaion: von v0.4.1 auf v0.5.0 aktualisiert."},
		},
		{
			name:    "failed restart mit Rückweg",
			auto:    program.KephalaionAuto{Outcome: "failed", Reason: "restart", From: "v0.4.1", To: "v0.5.0", Restarted: true, RolledBack: pointerTo(true), Error: "Node antwortete nicht mit v0.5.0", Hint: "kephalaion status"},
			exit:    1,
			restart: true,
			erwartet: []string{
				"Kephalaion: Upgrade gescheitert (restart) — Node antwortete nicht mit v0.5.0.",
				"  Der Node wurde neu gestartet; das alte Binary ist zurück und läuft wieder.",
				"  kephalaion status",
			},
		},
		{
			name:     "failed ohne Neustart",
			auto:     program.KephalaionAuto{Outcome: "failed", Reason: "download", From: "v0.4.1", To: "v0.5.0", Error: "Prüfsumme falsch"},
			exit:     1,
			erwartet: []string{"Kephalaion: Upgrade gescheitert (download) — Prüfsumme falsch."},
		},
	} {
		t.Run(fall.name, func(t *testing.T) {
			upgradeProjectDir(t, "v0.16.8")
			setOwnVersion(t, "v0.16.8")
			fake := fakeKephalaionUpgrade(t)
			fake.SetAuto(t, kephtest.JSON(t, fall.auto), "", fall.exit)

			report := runUpgradeJSON(t)
			if report.RestartRequired != fall.restart || report.Kephalaion.RestartRequired != fall.restart {
				t.Errorf("restartRequired = %v (kephalaion %v), erwartet %v", report.RestartRequired, report.Kephalaion.RestartRequired, fall.restart)
			}
			if report.Kephalaion.Auto == nil || report.Kephalaion.Auto.Outcome != fall.auto.Outcome {
				t.Errorf("kephalaion.auto = %+v, erwartet outcome %s", report.Kephalaion.Auto, fall.auto.Outcome)
			}
			messages := strings.Join(report.Messages, "\n")
			for _, erwartet := range fall.erwartet {
				if !strings.Contains(messages, erwartet) {
					t.Errorf("%q fehlt in:\n%s", erwartet, messages)
				}
			}
			hinweis := "Kephalaion wurde aktualisiert oder sein Node neu gestartet: die Sitzung neu starten."
			if strings.Contains(messages, hinweis) != fall.restart {
				t.Errorf("Neustart-Zeile da = %v, erwartet %v:\n%s", !fall.restart, fall.restart, messages)
			}
			if strings.Contains(messages, "Das Programm hat gewechselt") {
				t.Errorf("ohne Programmwechsel keine Zeile dazu:\n%s", messages)
			}
			assertOrder(t, messages, "Kephalaion:", "Programm-Stand: nichts zu tun", "Verlinkung eingerichtet")
		})
	}
}

// Ein gescheiterter Schritt Kephalaion — skipped, Weg 3 ohne --auto, kein
// JSON, Ausgang unbekannt — hält Programm- und Projektschritt nicht auf.
func TestUpgradeKephalaionOhneErfolgHaeltNichtAuf(t *testing.T) {
	for _, fall := range []struct {
		name     string
		setup    func(t *testing.T, fake *kephtest.FakeUpgrade)
		calls    [][]string
		erwartet []string
		report   func(t *testing.T, report upgradeReport)
	}{
		{
			name: "skipped hub_older",
			setup: func(t *testing.T, fake *kephtest.FakeUpgrade) {
				fake.SetAuto(t, kephtest.JSON(t, program.KephalaionAuto{Outcome: "skipped", Reason: "hub_older", From: "v0.4.1", To: "v0.5.0", Hint: "erst den Hub vm auf v0.5.0 bringen"}), "", 0)
			},
			calls:    [][]string{upgradeAutoArgs},
			erwartet: []string{"Kephalaion: v0.5.0 verfügbar, nicht automatisch aktualisiert (hub_older) — erst den Hub vm auf v0.5.0 bringen."},
		},
		{
			name: "Weg 3 mit neuerem Release",
			setup: func(t *testing.T, fake *kephtest.FakeUpgrade) {
				fake.SetCheck(t, kephtest.JSON(t, program.KephalaionCheck{State: "ok", Version: "v0.4.0", Latest: "v0.4.1", UpdateAvailable: true, Hint: "kephalaion upgrade"}), "", 0)
			},
			calls:    [][]string{upgradeAutoArgs, upgradeCheckArgs},
			erwartet: []string{"Kephalaion: v0.4.0 installiert, v0.4.1 verfügbar — kephalaion upgrade; dieses Kephalaion aktualisiert sich noch nicht selbst über k-playbook."},
			report: func(t *testing.T, report upgradeReport) {
				if !report.Kephalaion.AutoUnsupported || report.Kephalaion.Check == nil || report.Kephalaion.Check.Latest != "v0.4.1" {
					t.Errorf("kephalaion = %+v, erwartet autoUnsupported und den Report von --check", report.Kephalaion)
				}
			},
		},
		{
			name: "Weg 3 aktuell",
			setup: func(t *testing.T, fake *kephtest.FakeUpgrade) {
				fake.SetCheck(t, kephtest.JSON(t, program.KephalaionCheck{State: "ok", Version: "v0.4.0", Latest: "v0.4.0"}), "", 0)
			},
			calls:    [][]string{upgradeAutoArgs, upgradeCheckArgs},
			erwartet: []string{"Kephalaion: v0.4.0 ist aktuell."},
		},
		{
			name: "Weg 3, --check scheitert",
			setup: func(t *testing.T, fake *kephtest.FakeUpgrade) {
				fake.SetCheck(t, kephtest.JSON(t, program.KephalaionCheck{State: "failed", Version: "v0.4.0", Error: "GitHub nicht erreichbar"}), "", 1)
			},
			calls:    [][]string{upgradeAutoArgs, upgradeCheckArgs},
			erwartet: []string{"Kephalaion: Prüfung auf ein neueres Release gescheitert — GitHub nicht erreichbar. " + kephalaionNextStep},
		},
		{
			name: "kein JSON",
			setup: func(t *testing.T, fake *kephtest.FakeUpgrade) {
				fake.SetAuto(t, "panic: kaputt\n", "Fehler beim Start\n", 1)
			},
			calls:    [][]string{upgradeAutoArgs},
			erwartet: []string{"lieferte kein lesbares JSON (Exit-Code 1): Fehler beim Start", kephalaionNextStep},
			report: func(t *testing.T, report upgradeReport) {
				if report.Kephalaion.Error == "" || report.Kephalaion.OutcomeUnknown || report.RestartRequired {
					t.Errorf("kephalaion = %+v", report.Kephalaion)
				}
			},
		},
	} {
		t.Run(fall.name, func(t *testing.T) {
			upgradeProjectDir(t, "v0.16.8")
			setOwnVersion(t, "v0.16.8")
			fake := fakeKephalaionUpgrade(t)
			fall.setup(t, fake)

			report := runUpgradeJSON(t)
			messages := strings.Join(report.Messages, "\n")
			for _, erwartet := range fall.erwartet {
				if !strings.Contains(messages, erwartet) {
					t.Errorf("%q fehlt in:\n%s", erwartet, messages)
				}
			}
			assertOrder(t, messages, "Kephalaion:", "Programm-Stand: nichts zu tun", "Verlinkung eingerichtet")
			if report.RestartRequired {
				t.Errorf("kein Neustart erwartet:\n%s", messages)
			}
			if calls := fake.Calls(t); !reflect.DeepEqual(calls, fall.calls) {
				t.Errorf("Aufrufe von kephalaion = %q, erwartet %q", calls, fall.calls)
			}
			if fall.report != nil {
				fall.report(t, report)
			}
		})
	}

	// Ausgang unbekannt: der Aufruf wurde nach Fristablauf beendet. Den
	// Ablauf selbst prüft internal/program; hier zählt die Meldung.
	t.Run("Ausgang unbekannt", func(t *testing.T) {
		upgradeProjectDir(t, "v0.16.8")
		setOwnVersion(t, "v0.16.8")
		stubKephalaionUpgrade(t, func(path string) (program.KephalaionAuto, error) {
			return program.KephalaionAuto{}, &program.KephalaionUnknownOutcomeError{Command: path + " upgrade --auto --json", Timeout: program.KephalaionAutoTimeout}
		}, func(string) (program.KephalaionCheck, error) {
			t.Error("nach Ausgang unbekannt kein --check")
			return program.KephalaionCheck{}, nil
		})

		report := runUpgradeJSON(t)
		messages := strings.Join(report.Messages, "\n")
		for _, erwartet := range []string{"Ausgang unbekannt", "Prüfen: kephalaion status"} {
			if !strings.Contains(messages, erwartet) {
				t.Errorf("%q fehlt in:\n%s", erwartet, messages)
			}
		}
		if strings.Contains(messages, "nichts geändert") || !report.Kephalaion.OutcomeUnknown {
			t.Errorf("Ausgang unbekannt nicht als solcher gemeldet: %+v\n%s", report.Kephalaion, messages)
		}
		assertOrder(t, messages, "Kephalaion:", "Programm-Stand: nichts zu tun", "Verlinkung eingerichtet")
	})
}

// Nach einem Binärwechsel führt das neue Programm nur upgrade --project-only
// aus; das fasst Kephalaion nicht noch einmal an.
func TestUpgradeProjectOnlyFasstKephalaionNichtAn(t *testing.T) {
	upgradeProjectDir(t, "v0.16.8")
	setOwnVersion(t, "v0.16.8")
	fake := fakeKephalaionUpgrade(t)
	fake.SetAuto(t, kephtest.JSON(t, program.KephalaionAuto{Outcome: "upgraded", From: "v0.4.1", To: "v0.5.0", Restarted: true}), "", 0)

	stdout := captureStdout(t, func() {
		if err := runUpgrade([]string{"--project-only"}); err != nil {
			t.Errorf("upgrade --project-only: %v", err)
		}
	})
	if calls := fake.Calls(t); len(calls) != 0 {
		t.Errorf("--project-only hat kephalaion gerufen: %q", calls)
	}
	if strings.Contains(stdout, "Kephalaion:") || !strings.Contains(stdout, "Verlinkung eingerichtet") {
		t.Errorf("--project-only meldet Kephalaion oder zieht das Projekt nicht nach:\n%s", stdout)
	}
}

// removeClone nimmt dem Projekt die Installation k-playbook/ — Binary-only.
func removeClone(t *testing.T, root string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(root, project.PlaybookDirName)); err != nil {
		t.Fatal(err)
	}
}

// Ohne Clone geht der Schritt Kephalaion trotzdem über --auto, wenn
// kephalaion da ist.
func TestUpgradeOhneCloneRuftAuto(t *testing.T) {
	root := upgradeProjectDir(t, "v0.16.8")
	removeClone(t, root)
	setOwnVersion(t, "v0.16.8")
	fake := fakeKephalaionUpgrade(t)
	fake.SetAuto(t, kephtest.JSON(t, program.KephalaionAuto{Outcome: "current", From: "v0.4.1", To: "v0.4.1"}), "", 0)

	stdout := captureStdout(t, func() {
		if err := runUpgrade(nil); err != nil {
			t.Errorf("upgrade: %v", err)
		}
	})
	assertOrder(t, stdout, "Kephalaion: v0.4.1 ist aktuell.", "Programm: die Installation "+project.PlaybookDirName+"/ fehlt")
	if calls := fake.Calls(t); !reflect.DeepEqual(calls, [][]string{upgradeAutoArgs}) {
		t.Errorf("Aufrufe von kephalaion = %q, erwartet genau upgrade --auto --json", calls)
	}
}

// Ohne Clone und ohne kephalaion läuft der Schritt Kephalaion trotzdem: ein
// Lauf installiert es über das install.sh von Kephalaion, und --check meldet
// das, statt nur auf die Installation von Hand zu verweisen. --check
// installiert nicht.
func TestUpgradeOhneCloneInstalliertKephalaion(t *testing.T) {
	root := upgradeProjectDir(t, "v0.16.8")
	removeClone(t, root)
	setOwnVersion(t, "v0.16.8")
	installs := 0
	stubKephalaion(t, kephalaionFehlt, func(context.Context) (program.KephalaionResult, error) {
		installs++
		return program.KephalaionResult{Path: "/home/u/.local/bin/kephalaion", Installed: true, Output: "Installiert: /home/u/.local/bin/kephalaion"}, nil
	})
	stubKephalaionUpgrade(t, func(string) (program.KephalaionAuto, error) {
		t.Error("nach der Installation --auto gerufen")
		return program.KephalaionAuto{}, nil
	}, func(string) (program.KephalaionCheck, error) {
		t.Error("ohne kephalaion --check gerufen")
		return program.KephalaionCheck{}, nil
	})

	check := runUpgradeJSON(t, "--check")
	if installs != 0 {
		t.Fatalf("--check hat installiert (%d-mal)", installs)
	}
	if check.Kephalaion.Present || check.Kephalaion.Installed {
		t.Errorf("--check: kephalaion = %+v", check.Kephalaion)
	}
	messages := strings.Join(check.Messages, "\n")
	assertOrder(t, messages,
		"Kephalaion: fehlt im PATH; ein Lauf installiert es über das install.sh des neuesten Kephalaion-Releases (von Hand: "+kephalaion.InstallCommand+").",
		"Programm: die Installation "+project.PlaybookDirName+"/ fehlt",
	)

	report := runUpgradeJSON(t)
	if installs != 1 {
		t.Fatalf("der Lauf hat %d-mal installiert, erwartet einmal", installs)
	}
	if !report.Kephalaion.Present || !report.Kephalaion.Installed || report.Kephalaion.Path != "/home/u/.local/bin/kephalaion" {
		t.Errorf("kephalaion = %+v", report.Kephalaion)
	}
	messages = strings.Join(report.Messages, "\n")
	assertOrder(t, messages,
		"Kephalaion: installiert nach /home/u/.local/bin/kephalaion.",
		"  Installiert: /home/u/.local/bin/kephalaion",
		"Programm: die Installation "+project.PlaybookDirName+"/ fehlt; der Pull entfällt.",
		"Programm-Stand: nichts zu tun",
	)
	for _, gone := range []string{"installiert dieser Lauf es nicht", "bin/install-kephalaion"} {
		if strings.Contains(messages, gone) {
			t.Errorf("die Meldung nennt noch %q:\n%s", gone, messages)
		}
	}
}

// Derselbe Weg mit dem echten program.InstallKephalaion gegen eine
// Ersatzquelle: ohne Clone lädt der Lauf das Ersatz-install.sh und findet
// kephalaion danach im eigenen HOME. Kein Test lädt das echte install.sh.
func TestUpgradeOhneCloneLaedtInstallSkriptVonKephalaion(t *testing.T) {
	root := upgradeProjectDir(t, "v0.16.8")
	removeClone(t, root)
	installTargetInPath(t)
	setOwnVersion(t, "v0.16.8")
	source := releasetest.Serve(t)
	source.AddTag("v0.16.8")
	source.SetKephalaionInstall("printf 'Ersatz-install.sh\\n'\nprintf '#!/bin/sh\\n' > \"$HOME/.local/bin/kephalaion\"\nchmod 755 \"$HOME/.local/bin/kephalaion\"\n")
	// Die echte Suche und die echte Installation, im HOME dieses Tests.
	home := os.Getenv("HOME")
	find := func() (string, bool) {
		path := filepath.Join(home, ".local", "bin", kephalaion.BinaryName)
		if _, err := os.Stat(path); err != nil {
			return "", false
		}
		return path, true
	}
	beforeLookup := kephalaion.LookupBinary
	kephalaion.LookupBinary = find
	t.Cleanup(func() { kephalaion.LookupBinary = beforeLookup })
	stubKephalaion(t, find, program.InstallKephalaion)

	report := runUpgradeJSON(t)
	want := filepath.Join(home, ".local", "bin", kephalaion.BinaryName)
	if !report.Kephalaion.Installed || report.Kephalaion.Path != want || report.Kephalaion.Error != "" {
		t.Fatalf("kephalaion = %+v", report.Kephalaion)
	}
	messages := strings.Join(report.Messages, "\n")
	assertOrder(t, messages, "Kephalaion: installiert nach "+want+".", "  Ersatz-install.sh", "Programm: die Installation "+project.PlaybookDirName+"/ fehlt")
	found := false
	for _, request := range source.Requests() {
		found = found || request == "/kephalaion/install.sh"
	}
	if !found {
		t.Errorf("das install.sh wurde nicht von der Ersatzquelle geladen: %v", source.Requests())
	}
}

// kUpgradeHinweis ist der Anfang der letzten Zeile nach einer neuen Version,
// kUpgradeVorschau der Anfang der Vorschauzeile von --check.
const (
	kUpgradeHinweis  = "Neue Version geholt: die Umstellungen, die eine Beurteilung brauchen, zieht /k-upgrade nach"
	kUpgradeVorschau = "Ein Lauf holt Neues; bringt er eine neue Version, zieht danach /k-upgrade"
)

// stubCheckUpdate ersetzt die Abfrage des Remote-Stands bei --check.
func stubCheckUpdate(t *testing.T, status project.UpdateStatus) {
	t.Helper()
	before := upgradeCheckUpdate
	t.Cleanup(func() { upgradeCheckUpdate = before })
	upgradeCheckUpdate = func(string) (project.UpdateStatus, error) { return status, nil }
}

// withClone gibt der Installation ein .git-Verzeichnis, damit der Pull läuft.
func withClone(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, project.PlaybookDirName, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	return lines[len(lines)-1]
}

func runUpgradeText(t *testing.T, args ...string) string {
	t.Helper()
	return captureStdout(t, func() {
		if err := runUpgrade(args); err != nil {
			t.Errorf("upgrade %v: %v", args, err)
		}
	})
}

// Ist das Binary ersetzt, steht der Hinweis nach den Neustart-Hinweisen und
// sagt „in einer neuen Sitzung“. Der Hinweis hängt allein am Binärwechsel.
func TestUpgradeHinweisAufKUpgradeNachBinaerwechsel(t *testing.T) {
	t.Setenv(viaCommandEnv, "")
	root := upgradeProjectDir(t, "v0.17.0")
	installTargetInPath(t)
	setOwnVersion(t, "v0.16.8")
	withClone(t, root)

	stub := filepath.Join(t.TempDir(), "k-playbook-neu")
	script := "#!/bin/sh\n" +
		"[ \"$1\" = upgrade ] && [ \"$2\" = --project-only ] && [ $# = 2 ] || exit 7\n" +
		"printf 'Projekt: nachgezogen vom Stub.\\n'\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	stubUpgradeSteps(t,
		func(string) (project.UpdateResult, error) {
			return project.UpdateResult{}, nil
		},
		func(context.Context, string) (program.Result, error) {
			return program.Result{Target: stub, Expected: "v0.17.0", Installed: true}, nil
		})

	stdout := runUpgradeText(t)
	assertOrder(t, stdout, "Projekt: nachgezogen vom Stub.", "Das Programm hat gewechselt: die Sitzung neu starten", kUpgradeHinweis)
	if last := lastLine(stdout); !strings.HasPrefix(last, kUpgradeHinweis) || !strings.Contains(last, "— in einer neuen Sitzung des Assistenten aufrufen.") {
		t.Errorf("letzte Zeile ist nicht der Hinweis mit neuer Sitzung: %q\n%s", last, stdout)
	}
}

// Ohne Binärwechsel kein Hinweis — auch dann nicht, wenn der Pull gelaufen
// ist: Der Hinweis hängt allein daran, dass das Binary ersetzt wurde. Mit
// K_PLAYBOOK_VIA_COMMAND=1 — der Aufruf aus /k-upgrade — auch nach einem
// Binärwechsel nicht.
func TestUpgradeKeinHinweisAufKUpgrade(t *testing.T) {
	for _, fall := range []struct {
		name, via, own string
		installed      bool
	}{
		{"ohne Binärwechsel", "", "v0.17.0", false},
		{"aus /k-upgrade", "1", "v0.16.8", true},
	} {
		t.Run(fall.name, func(t *testing.T) {
			t.Setenv(viaCommandEnv, fall.via)
			root := upgradeProjectDir(t, "v0.17.0")
			installTargetInPath(t)
			setOwnVersion(t, fall.own)
			withClone(t, root)
			pulled := false
			stubUpgradeSteps(t, func(string) (project.UpdateResult, error) {
				pulled = true
				return project.UpdateResult{Output: "Updating 1111111..2222222"}, nil
			}, func(context.Context, string) (program.Result, error) {
				if !fall.installed {
					t.Error("ohne älteres Programm wurde installiert")
				}
				return program.Result{Target: filepath.Join(root, "k-playbook-neu"), Expected: "v0.17.0", Installed: true}, nil
			})
			upgradeReexec = func(string, string, bool, io.Writer) (any, []string, error) {
				return nil, []string{"Projekt: nachgezogen vom neuen Programm."}, nil
			}

			stdout := runUpgradeText(t)
			if strings.Contains(stdout, "/k-upgrade") {
				t.Errorf("unerwarteter Hinweis auf /k-upgrade:\n%s", stdout)
			}
			if !pulled {
				t.Errorf("der Pull ist nicht gelaufen:\n%s", stdout)
			}
			if fall.installed != strings.Contains(stdout, "Das Programm hat gewechselt") {
				t.Errorf("Binärwechsel = %v passt nicht zur Ausgabe:\n%s", fall.installed, stdout)
			}
		})
	}
}

// --project-only ist der Projektschritt im neuen Programm; der Hinweis kommt
// nur vom aufrufenden Prozess.
func TestUpgradeProjectOnlyOhneHinweisAufKUpgrade(t *testing.T) {
	t.Setenv(viaCommandEnv, "")
	root := upgradeProjectDir(t, "v0.17.0")
	setOwnVersion(t, "v0.16.8")
	withClone(t, root)
	stubUpgradeSteps(t,
		func(string) (project.UpdateResult, error) {
			t.Error("--project-only hat gepullt")
			return project.UpdateResult{}, nil
		},
		func(context.Context, string) (program.Result, error) {
			t.Error("--project-only hat installiert")
			return program.Result{}, nil
		})

	if stdout := runUpgradeText(t, "--project-only"); strings.Contains(stdout, "/k-upgrade") {
		t.Errorf("--project-only gibt den Hinweis auf /k-upgrade aus:\n%s", stdout)
	}
}

// --check sagt in einer eigenen Vorschauzeile, dass ein Lauf Neues holt: wenn
// der Clone hinter seinem Remote liegt oder das laufende Programm älter als
// der neueste Release-Tag und installierbar ist. Sonst keine Zeile, und mit K_PLAYBOOK_VIA_COMMAND=1 nie.
func TestUpgradeCheckVorschauAufKUpgrade(t *testing.T) {
	hinter := project.UpdateStatus{Available: true, Branch: "main", Local: "aaaaaaaaaaaa", Remote: "bbbbbbbbbbbb"}
	gleich := project.UpdateStatus{Branch: "main", Local: "aaaaaaaaaaaa", Remote: "aaaaaaaaaaaa"}
	for _, fall := range []struct {
		name, via, own string
		update         project.UpdateStatus
		erwartet       bool
	}{
		{"hinter dem Remote", "", "v0.17.0", hinter, true},
		{"älteres installierbares Programm", "", "v0.16.8", gleich, true},
		{"nichts Neues", "", "v0.17.0", gleich, false},
		{"aus /k-upgrade", "1", "v0.16.8", hinter, false},
	} {
		t.Run(fall.name, func(t *testing.T) {
			t.Setenv(viaCommandEnv, fall.via)
			upgradeProjectDir(t, "v0.17.0")
			installTargetInPath(t)
			setOwnVersion(t, fall.own)
			stubCheckUpdate(t, fall.update)

			stdout := runUpgradeText(t, "--check")
			if fall.erwartet {
				if last := lastLine(stdout); !strings.HasPrefix(last, kUpgradeVorschau) {
					t.Errorf("letzte Zeile ist nicht die Vorschau: %q\n%s", last, stdout)
				}
			} else if strings.Contains(stdout, "/k-upgrade") {
				t.Errorf("unerwartete Vorschau auf /k-upgrade:\n%s", stdout)
			}
			if strings.Contains(stdout, kUpgradeHinweis) || strings.Contains(stdout, "neuere Version") {
				t.Errorf("--check gibt den Hinweis eines Laufs aus:\n%s", stdout)
			}
		})
	}
}
