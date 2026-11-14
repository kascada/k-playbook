package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/program"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/release/releasetest"
)

// Programmschritt gegen den neuesten Release-Tag (Task 149): Verglichen wird
// die eigene Version mit dem Tag, nie mit VERSION im Clone, und installiert
// wird nur bei „älter".

// stubInstall ersetzt die Installation und zählt, mit welchem Tag sie gerufen
// wurde. Das Ergebnis ist ein gelungener Binärwechsel nach target.
func stubInstall(t *testing.T, target string) *[]string {
	t.Helper()
	tags := []string{}
	stubUpgradeSteps(t, nil, func(_ context.Context, tag string) (program.Result, error) {
		tags = append(tags, tag)
		return program.Result{Target: target, Expected: tag, Installed: true}, nil
	})
	// Wie der echte Aufruf: im Klartextmodus schreibt das neue Programm
	// selbst nach out, im JSON-Modus kommen seine Meldungen zurück.
	upgradeReexec = func(_ string, _ string, asJSON bool, out io.Writer) (any, []string, error) {
		if !asJSON {
			fmt.Fprintln(out, "Projekt: nachgezogen vom neuen Programm.")
		}
		return nil, []string{"Projekt: nachgezogen vom neuen Programm."}, nil
	}
	return &tags
}

func TestUpgradeVergleichtMitDemNeuestenTag(t *testing.T) {
	for _, fall := range []struct {
		name, own string
		installed bool
		erwartet  string
	}{
		{"älter", "v0.16.8", true, "Programm: v0.17.0 installiert nach "},
		{"gleich", "v0.17.0", false, "Programm-Stand: nichts zu tun — laufend v0.17.0, neuestes Release v0.17.0 (gleich)."},
		// Ein Build über dem neuesten Tag gilt als aktuell.
		{"neuer", "v0.18.0", false, "Programm-Stand: nichts zu tun — laufend v0.18.0, neuestes Release v0.17.0 (neuer)."},
	} {
		t.Run(fall.name, func(t *testing.T) {
			root := upgradeProjectDir(t, "v0.17.0")
			installTargetInPath(t)
			setOwnVersion(t, fall.own)
			tags := stubInstall(t, filepath.Join(root, "k-playbook-neu"))

			report := runUpgradeJSON(t)
			messages := strings.Join(report.Messages, "\n")
			if !strings.Contains(messages, fall.erwartet) {
				t.Errorf("%q fehlt in:\n%s", fall.erwartet, messages)
			}
			if fall.installed != (len(*tags) == 1) || (fall.installed && (*tags)[0] != "v0.17.0") {
				t.Errorf("Installation mit Tags %v, erwartet installiert = %v mit v0.17.0", *tags, fall.installed)
			}
			status := report.Program.Status
			if status == nil || status.Latest != "v0.17.0" || status.Running != fall.own || status.Outdated != fall.installed {
				t.Errorf("program.status = %+v", status)
			}
			if report.Program.Installed != fall.installed || report.RestartRequired != fall.installed {
				t.Errorf("installed %v, restartRequired %v", report.Program.Installed, report.RestartRequired)
			}
			// Die ersetzte Quelle steht da: die Testnaht wirkt auch im
			// ausgelieferten Programm und soll nicht unbemerkt bleiben.
			if !strings.Contains(messages, "Programm: Release-Quelle ersetzt über "+release.EnvSource) {
				t.Errorf("die ersetzte Release-Quelle wird nicht genannt:\n%s", messages)
			}
		})
	}
}

// Die JSON-Antwort trägt latest statt clone, kein versionChanged und kein
// mcpRepaired am Programmschritt mehr; die Fassung ist gestiegen.
func TestUpgradeJSONTraegtLatestStattClone(t *testing.T) {
	upgradeProjectDir(t, "v0.17.0")
	installTargetInPath(t)
	setOwnVersion(t, "v0.16.8")

	stdout := captureStdout(t, func() {
		if err := runUpgrade([]string{"--check", "--json"}); err != nil {
			t.Errorf("upgrade --check --json: %v", err)
		}
	})
	for _, erwartet := range []string{`"schemaVersion": "2"`, `"latest": "v0.17.0"`, `"running": "v0.16.8"`, `"outdated": true`} {
		if !strings.Contains(stdout, erwartet) {
			t.Errorf("%q fehlt in:\n%s", erwartet, stdout)
		}
	}
	for _, verboten := range []string{`"clone"`, `"versionChanged"`} {
		if strings.Contains(stdout, verboten) {
			t.Errorf("%s steht noch in:\n%s", verboten, stdout)
		}
	}
}

// Ohne eigene Version wird der Tag nicht abgerufen — kein Netzaufruf —, und
// installiert wird nicht. Das Projekt wird trotzdem nachgezogen.
func TestUpgradeOhneVersionRuftDenTagNichtAb(t *testing.T) {
	root := upgradeProjectDir(t, "v0.17.0")
	installTargetInPath(t)
	source := releasetest.Serve(t)
	source.AddTag("v0.17.0")
	setOwnVersion(t, "")
	tags := stubInstall(t, filepath.Join(root, "k-playbook-neu"))

	for _, args := range [][]string{{"--check"}, nil} {
		stdout := runUpgradeText(t, args...)
		if !strings.Contains(stdout, "dieses Programm trägt keine Version") {
			t.Errorf("upgrade %v: Meldung fehlt:\n%s", args, stdout)
		}
	}
	if len(*tags) != 0 {
		t.Errorf("ohne eigene Version wurde installiert: %v", *tags)
	}
	if requests := source.Requests(); len(requests) != 0 {
		t.Errorf("ohne eigene Version wurde die Release-Quelle gefragt: %v", requests)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "commands", "k-test.md")); err != nil {
		t.Errorf("das Projekt wurde nicht nachgezogen: %v", err)
	}
}

// Ist der neueste Tag unbekannt — ohne Netz, ohne git, Zeitüberschreitung;
// hier die abgeschaltete Quelle —, meldet upgrade das und zieht das Projekt
// trotzdem nach. Installiert wird nicht.
func TestUpgradeTagUnbekannt(t *testing.T) {
	root := upgradeProjectDir(t, "v0.17.0")
	installTargetInPath(t)
	releasetest.Off(t)
	setOwnVersion(t, "v0.16.8")
	tags := stubInstall(t, filepath.Join(root, "k-playbook-neu"))

	check := runUpgradeText(t, "--check")
	if !strings.Contains(check, "Programm-Stand: der neueste Release-Tag ist unbekannt — die Release-Abfrage ist abgeschaltet") ||
		strings.Contains(check, kUpgradeVorschau) {
		t.Errorf("--check:\n%s", check)
	}

	stdout := runUpgradeText(t)
	assertOrder(t, stdout,
		"Programm-Stand: der neueste Release-Tag ist unbekannt — die Release-Abfrage ist abgeschaltet",
		"das Projekt wird mit dem laufenden Programm nachgezogen",
		"Verlinkung eingerichtet",
	)
	if len(*tags) != 0 || strings.Contains(stdout, "neu starten") {
		t.Errorf("bei unbekanntem Tag installiert (%v) oder Neustart verlangt:\n%s", *tags, stdout)
	}
}

// Ohne Clone laufen Vergleich und Installation trotzdem: --check meldet den
// Stand, ein Lauf installiert, und der fehlende Clone wird gemeldet — ohne zu
// behaupten, es gebe nichts zu aktualisieren. Einen Pull gibt es nicht.
func TestUpgradeOhneCloneVergleichtUndInstalliert(t *testing.T) {
	t.Setenv(viaCommandEnv, "")
	root := upgradeProjectDir(t, "v0.17.0")
	removeClone(t, root)
	installTargetInPath(t)
	setOwnVersion(t, "v0.16.8")
	target := filepath.Join(root, "k-playbook-neu")
	tags := stubInstall(t, target)
	upgradeUpdate = func(string) (project.UpdateResult, error) {
		t.Error("ohne Clone wurde gepullt")
		return project.UpdateResult{}, nil
	}
	stubCheckUpdate(t, project.UpdateStatus{})
	upgradeCheckUpdate = func(string) (project.UpdateStatus, error) {
		t.Error("ohne Clone wurde der Remote-Stand gefragt")
		return project.UpdateStatus{}, nil
	}

	check := runUpgradeText(t, "--check")
	assertOrder(t, check,
		"Programm: die Installation "+project.PlaybookDirName+"/ fehlt",
		"Programm-Stand: v0.16.8 ist älter als das neueste Release (v0.17.0); ein Lauf installierte das Programm nach",
		kUpgradeVorschau,
	)
	if len(*tags) != 0 {
		t.Fatalf("--check hat installiert: %v", *tags)
	}

	stdout := runUpgradeText(t)
	assertOrder(t, stdout,
		"Programm: die Installation "+project.PlaybookDirName+"/ fehlt; der Pull entfällt.",
		"Programm: v0.17.0 installiert nach "+target,
		"Projekt: nachgezogen vom neuen Programm.",
		"Das Programm hat gewechselt: die Sitzung neu starten",
		kUpgradeHinweis,
	)
	if strings.Contains(stdout, "nichts zu aktualisieren") {
		t.Errorf("die Meldung zum fehlenden Clone behauptet, es gebe nichts zu aktualisieren:\n%s", stdout)
	}
	if len(*tags) != 1 || (*tags)[0] != "v0.17.0" {
		t.Errorf("installiert mit %v, erwartet einmal v0.17.0", *tags)
	}
}

// staleMCP ist ein Eintrag auf den abgelösten Wrapper — MCPStateOutdated.
const staleMCP = `{"mcpServers":{"` + project.MCPServerKey + `":{"command":"k-playbook/bin/k-playbook","args":["mcp"]}}}` + "\n"

// Die MCP-Reparatur hängt nicht mehr am Pull: Scheitert er, repariert der
// Projektschritt den veralteten Eintrag trotzdem und meldet es.
func TestUpgradeRepariertMCPNachGescheitertemPull(t *testing.T) {
	root := upgradeProjectDir(t, "v0.17.0")
	installTargetInPath(t)
	setOwnVersion(t, "v0.17.0")
	withClone(t, root)
	mustWriteMCPFile(t, filepath.Join(root, ".mcp.json"), staleMCP)
	stubUpgradeSteps(t, func(string) (project.UpdateResult, error) {
		return project.UpdateResult{Output: "fatal: unable to access"}, errors.New("git pull --ff-only fehlgeschlagen")
	}, nil)

	report := runUpgradeJSON(t)
	messages := strings.Join(report.Messages, "\n")
	assertOrder(t, messages,
		"Programm: Pull im Clone fehlgeschlagen — git pull --ff-only fehlgeschlagen",
		"Programm-Stand: nichts zu tun",
		"Projekt: MCP-Registrierung nachgezogen: .mcp.json",
	)
	if report.Program.Pulled || report.Program.Error == "" {
		t.Errorf("program = %+v, erwartet einen gemeldeten Fehlschlag des Pulls", report.Program)
	}
	content, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil || strings.Contains(string(content), "k-playbook/bin/k-playbook") {
		t.Errorf("der veraltete Eintrag steht noch: %v\n%s", err, content)
	}
	if status := mcpState(t, root, ".mcp.json"); !status.OK() {
		t.Errorf("nach dem Lauf nicht in Ordnung: %+v", status)
	}
}

// Nach gelungenem Pull repariert der Projektschritt im Umfang des Starts: Er
// ergänzt auch einen fehlenden Eintrag, wenn die Zieldatei nicht von git
// erfasst ist. Früher lief nach gelungenem Pull nur der enge Modus. --check
// meldet davon nichts — die hingenommene Lücke.
func TestUpgradeErgaenztFehlendenMCPEintragNachGelungenemPull(t *testing.T) {
	root := upgradeProjectDir(t, "v0.17.0")
	installTargetInPath(t)
	setOwnVersion(t, "v0.17.0")
	withClone(t, root)
	// project.vcs none: nichts ist von git erfasst.
	mustWriteMCPFile(t, project.ConfigPath(root), "schema_version: "+project.SchemaVersion+"\n\nproject:\n  repo_root: .\n  vcs: none\n")
	foreign := `{"mcpServers":{"fremd":{"command":"anderes","args":["dienen"]}}}` + "\n"
	mustWriteMCPFile(t, filepath.Join(root, ".mcp.json"), foreign)
	pulled := false
	stubUpgradeSteps(t, func(string) (project.UpdateResult, error) {
		pulled = true
		return project.UpdateResult{}, nil
	}, nil)
	stubCheckUpdate(t, project.UpdateStatus{Branch: "main", Local: "aaaaaaaaaaaa", Remote: "aaaaaaaaaaaa"})

	if check := runUpgradeText(t, "--check"); strings.Contains(check, "MCP") {
		t.Errorf("--check meldet den fehlenden Eintrag — die Lücke ist geschlossen, der Test und die Doku ziehen nach:\n%s", check)
	}

	stdout := runUpgradeText(t)
	if !pulled {
		t.Fatalf("der Pull ist nicht gelaufen:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Projekt: MCP-Registrierung nachgezogen: .mcp.json") {
		t.Errorf("die Ergänzung wird nicht gemeldet:\n%s", stdout)
	}
	content, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil || !strings.Contains(string(content), `"`+project.MCPServerKey+`"`) || !strings.Contains(string(content), `"fremd"`) {
		t.Errorf(".mcp.json nach dem Lauf: %v\n%s", err, content)
	}
}

// mcpState liefert den Zustand einer MCP-Datei des Projekts.
func mcpState(t *testing.T, root string, path string) project.MCPStatus {
	t.Helper()
	for _, status := range project.CheckMCP(root) {
		if status.Path == path {
			return status
		}
	}
	t.Fatalf("kein MCP-Status für %s", path)
	return project.MCPStatus{}
}

// `upgrade --check` fragt immer frisch, auch bei frischem Eintrag im Cache —
// sichtbar an der Ersatzquelle —, und schreibt das Ergebnis in den Cache.
func TestUpgradeCheckFragtTrotzFrischemCache(t *testing.T) {
	upgradeProjectDir(t, "v0.17.0")
	installTargetInPath(t)
	setOwnVersion(t, "v0.17.0")
	source := releasetest.Serve(t)
	source.AddTag("v0.17.0")

	// Ein frischer Eintrag dieser Quelle: v0.17.0.
	if tag, err := release.CachedLatestTag(context.Background()); err != nil || tag != "v0.17.0" {
		t.Fatalf("CachedLatestTag = %q, %v", tag, err)
	}
	asked := len(source.Requests())
	source.AddTag("v0.18.0")

	stdout := runUpgradeText(t, "--check")
	if len(source.Requests()) != asked+1 {
		t.Errorf("upgrade --check hat nicht frisch gefragt: %v", source.Requests())
	}
	if !strings.Contains(stdout, "neueste Release (v0.18.0)") {
		t.Errorf("upgrade --check sieht das neue Release nicht:\n%s", stdout)
	}
	if tag, err := release.CachedLatestTag(context.Background()); err != nil || tag != "v0.18.0" || len(source.Requests()) != asked+1 {
		t.Errorf("der Cache trägt nicht das frische Ergebnis: %q, %v, Anfragen %v", tag, err, source.Requests())
	}
}

// `upgrade` ohne --check fragt ebenso frisch, auch bei frischem Eintrag im
// Cache, installiert das neue Release — hier über die Testnaht, nichts landet
// in ~/.local/bin — und schreibt das Ergebnis in den Cache.
func TestUpgradeFragtTrotzFrischemCache(t *testing.T) {
	root := upgradeProjectDir(t, "v0.17.0")
	installTargetInPath(t)
	setOwnVersion(t, "v0.17.0")
	tags := stubInstall(t, filepath.Join(root, "k-playbook-neu"))
	source := releasetest.Serve(t)
	source.AddTag("v0.17.0")

	// Ein frischer Eintrag dieser Quelle: v0.17.0 — aus dem Cache wäre das
	// laufende Programm aktuell.
	if tag, err := release.CachedLatestTag(context.Background()); err != nil || tag != "v0.17.0" {
		t.Fatalf("CachedLatestTag = %q, %v", tag, err)
	}
	asked := len(source.Requests())
	source.AddTag("v0.18.0")

	report := runUpgradeJSON(t)
	if len(source.Requests()) != asked+1 {
		t.Errorf("upgrade hat nicht frisch gefragt: %v", source.Requests())
	}
	if status := report.Program.Status; status == nil || status.Latest != "v0.18.0" || !status.Outdated {
		t.Errorf("program.status = %+v", status)
	}
	if len(*tags) != 1 || (*tags)[0] != "v0.18.0" {
		t.Errorf("Installation mit Tags %v, erwartet v0.18.0", *tags)
	}
	if tag, err := release.CachedLatestTag(context.Background()); err != nil || tag != "v0.18.0" || len(source.Requests()) != asked+1 {
		t.Errorf("der Cache trägt nicht das frische Ergebnis: %q, %v, Anfragen %v", tag, err, source.Requests())
	}
}
