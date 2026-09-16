package github

import (
	"context"
	"strings"
	"testing"
)

// Die Fixture ist das Log von Lauf 35002675449 (Workflow „Tests" auf `main`,
// Commit „Release v0.9.0"), gesichert mit `gh run view 35002675449
// --log-failed`. Sie steht hier, weil GitHub Logs nur begrenzt aufbewahrt und
// der Lauf nach einer CI-Reparatur aus der Liste fällt — die Prüfung darf
// daran nicht hängen.
//
// Nicht zu verwechseln mit dem Lauf zum Tag `v0.9.0` (35002537701), der
// ebenfalls rot ist.
const failureFixture = "run-35002675449-failed.log"

// Die Zählregel: gezählt wird ein Test mit eigener Meldungszeile.
// Übergeordnete Tests mit Subtests haben keine und zählen nicht. Das Log hat
// 37 `--- FAIL`-Zeilen und ergibt genau zwei Gruppen mit 32 und 1 Test.
func TestFailureLogErgibtGenauZweiGruppen(t *testing.T) {
	failure := ParseFailureLog(string(readFixture(t, failureFixture)))

	if failure.State != StateOK {
		t.Fatalf("Zustand %q, erwartet ok", failure.State)
	}
	if failure.FailLines != 37 {
		t.Errorf("%d FAIL-Zeilen, erwartet 37", failure.FailLines)
	}
	if len(failure.Groups) != 2 {
		t.Fatalf("%d Gruppen, erwartet 2: %+v", len(failure.Groups), failure.Groups)
	}
	if failure.Groups[0].Count != 32 {
		t.Errorf("größte Gruppe hat %d Tests, erwartet 32: %q", failure.Groups[0].Count, failure.Groups[0].Message)
	}
	if failure.Groups[1].Count != 1 {
		t.Errorf("zweite Gruppe hat %d Tests, erwartet 1: %q", failure.Groups[1].Count, failure.Groups[1].Message)
	}
	if failure.Counted != 33 {
		t.Errorf("%d gezählte Tests, erwartet 33 (32 + 1)", failure.Counted)
	}
}

func TestFailureLogNenntDieUrsacheJeGruppe(t *testing.T) {
	failure := ParseFailureLog(string(readFixture(t, failureFixture)))

	if !strings.Contains(failure.Groups[0].Message, "K-PLAYBOOK.yaml: no such file or directory") {
		t.Errorf("erste Gruppe: %q", failure.Groups[0].Message)
	}
	if failure.Groups[1].Message != ".python-version wurde nicht ausgewertet" {
		t.Errorf("zweite Gruppe: %q", failure.Groups[1].Message)
	}
	// Das Präfix `datei:zeile:` unterscheidet sich je Test und ließe gleiche
	// Ursachen auseinanderfallen — es gehört abgeschnitten.
	for _, group := range failure.Groups {
		if strings.Contains(group.Message, "_test.go:") {
			t.Errorf("Gruppe trägt noch ihr datei:zeile-Präfix: %q", group.Message)
		}
	}
	// Die Pfade des Runners sind vereinheitlicht.
	if strings.Contains(failure.Groups[0].Message, "/home/runner") {
		t.Errorf("Runner-Pfad nicht vereinheitlicht: %q", failure.Groups[0].Message)
	}
}

// Die Gruppe nennt die Tests, die an ihr hängen — hinter dem Aufklappen, nicht
// als 32 Zeilen.
func TestFailureLogSammeltDieTestnamenJeGruppe(t *testing.T) {
	failure := ParseFailureLog(string(readFixture(t, failureFixture)))

	if len(failure.Groups[0].Tests) != 32 {
		t.Fatalf("%d Testnamen in der größten Gruppe, erwartet 32", len(failure.Groups[0].Tests))
	}
	// Ein Subtest steht mit seinem vollen Namen da, sein Elternteil gar nicht:
	// nur der Subtest trägt die Meldung.
	namen := strings.Join(failure.Groups[0].Tests, " ")
	if !strings.Contains(namen, "TestBaseGuardJeEintrag/schreibender_Lauf_auf_fremdes_Ziel_bricht_ab") {
		t.Error("der Subtest von TestBaseGuardJeEintrag fehlt")
	}
	for _, name := range failure.Groups[0].Tests {
		if name == "TestBaseGuardJeEintrag" {
			t.Error("der übergeordnete TestBaseGuardJeEintrag wird mitgezählt, obwohl er keine eigene Meldung trägt")
		}
	}
	// Die Ursache umfasst auch release_asset_test.go, nicht nur die beiden
	// Installationstests.
	if !strings.Contains(namen, "TestPruefsummenWerdenNieGetroffen") {
		t.Error("der Test aus release_asset_test.go fehlt in der Gruppe")
	}
}

func TestNormalizeFailureMessageVereinheitlichtPfade(t *testing.T) {
	cases := map[string]string{
		"stat /home/runner/work/k-playbook/k-playbook/K-PLAYBOOK.yaml: no such file": "stat <runner>/K-PLAYBOOK.yaml: no such file",
		"Wurzel /home/runner/go/pkg passt nicht":                                     "Wurzel <runner>/go/pkg passt nicht",
		"Datei /tmp/TestBaseFall2/001/bin fehlt":                                     "Datei <tmp>/bin fehlt",
		"  doppelte   Leerzeichen  ":                                                 "doppelte Leerzeichen",
	}
	for input, want := range cases {
		if got := NormalizeFailureMessage(input); got != want {
			t.Errorf("NormalizeFailureMessage(%q) = %q, erwartet %q", input, got, want)
		}
	}
}

// Unbekanntes Logformat: gruppiert wird nichts, gezeigt werden die letzten
// Fehlerzeilen des Jobs. Eine leere Karte wäre die schlechtere Auskunft.
func TestFailureLogOhneTestformatZeigtDieLetztenFehlerzeilen(t *testing.T) {
	log := strings.Join([]string{
		"build\tmake\t2026-09-15T17:40:30.9067254Z npm ERR! code ELIFECYCLE",
		"build\tmake\t2026-09-15T17:40:31.0000000Z ##[error]Process completed with exit code 2.",
	}, "\n")

	failure := ParseFailureLog(log)
	if len(failure.Groups) != 0 {
		t.Errorf("%d Gruppen, erwartet keine", len(failure.Groups))
	}
	if len(failure.Lines) == 0 {
		t.Fatal("keine Fehlerzeilen als Rückfall")
	}
	if !strings.Contains(failure.Lines[0], "exit code 2") {
		t.Errorf("Rückfallzeile %q", failure.Lines[0])
	}
	if failure.Note == "" {
		t.Error("kein Hinweis, warum nicht gruppiert wurde")
	}
}

// Das Log ist die teuerste Abfrage. Sie holt genau einen Aufruf, und zwar den
// auf den Lauf mit dieser Kennung.
func TestFailureHoltGenauDasLogDiesesLaufs(t *testing.T) {
	runner := &fakeRunner{answers: []fakeAnswer{
		{prefix: "gh run view", out: string(readFixture(t, failureFixture))},
	}}
	failure := runner.client().FetchFailure(context.Background(), 35002675449)

	if failure.RunID != 35002675449 {
		t.Errorf("Lauf %d, erwartet 35002675449", failure.RunID)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("%d Aufrufe, erwartet 1: %v", len(runner.calls), runner.calls)
	}
	if runner.calls[0] != "gh run view 35002675449 --log-failed" {
		t.Errorf("Aufruf %q", runner.calls[0])
	}
}

// Die Meldungen sind an gh 2.46.0 gemessen (Befund
// github-stand-in-der-oberflaeche.md): ein verworfenes Log an Lauf 23831424363
// von cli/cli, die Gegenfälle an diesem Repo und an einem, das es nicht gibt.
func TestFailureTrenntVerworfenesLogVonKennungUndZugriff(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		want   State
	}{
		{
			name:   "Log verworfen (410, gemessen)",
			stderr: "failed to get run log: HTTP 410: Server Error (https://api.github.com/repos/cli/cli/actions/runs/23831424363/logs)\n",
			want:   StateLogGone,
		},
		{
			name:   "Log nicht gefunden (404 auf das Archiv, Quelltext)",
			stderr: "failed to get run log: log not found\n",
			want:   StateLogGone,
		},
		{
			name:   "falsche Kennung (gemessen)",
			stderr: "failed to get run: HTTP 404: Not Found (https://api.github.com/repos/kascada/k-playbook/actions/runs/1?exclude_pull_requests=true)\n",
			want:   StateNoAccess,
		},
		{
			name:   "kein Zugriff (gemessen)",
			stderr: "failed to get run: HTTP 404: Not Found (https://api.github.com/repos/kascada-nonexistent-zz9/nope/actions/runs/35002675449?exclude_pull_requests=true)\n",
			want:   StateNoAccess,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runner := &fakeRunner{answers: []fakeAnswer{{prefix: "gh run view", err: ghError(testCase.stderr)}}}
			failure := runner.client().FetchFailure(context.Background(), 35002675449)

			if failure.State != testCase.want {
				t.Fatalf("Zustand %q, erwartet %q (%s)", failure.State, testCase.want, failure.Message)
			}
			if failure.Message == "" {
				t.Error("Zustand ohne Erklärung")
			}
			if strings.Contains(failure.Message, "https://") {
				t.Errorf("Fehlertext von gh roh durchgereicht: %q", failure.Message)
			}
			// Weder das verworfene Log noch eine falsche Kennung ist ein
			// fehlendes Recht. Wo gh beides nicht trennt, sagt der Satz das.
			if testCase.want == StateNoAccess && !strings.Contains(failure.Message, "Kennung") {
				t.Errorf("Satz behauptet fehlenden Zugriff, ohne die falsche Kennung zu nennen: %q", failure.Message)
			}
		})
	}
}
