package program

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
)

func autoArgs() []string  { return []string{"upgrade", "--auto", "--json"} }
func checkArgs() []string { return []string{"upgrade", "--check", "--json"} }

func boolPointer(value bool) *bool { return &value }

// Je outcome liest KephalaionAutoUpgrade das JSON von stdout — auch bei
// Exit 1 — und sagt, ob der Node neu gestartet wurde.
func TestKephalaionAutoUpgradeJeOutcome(t *testing.T) {
	check := &KephalaionCheck{State: "ok", Version: "v0.4.1", Latest: "v0.5.0", UpdateAvailable: true, Method: "self"}
	for _, fall := range []struct {
		name      string
		auto      KephalaionAuto
		exit      int
		restarted bool
	}{
		{"upgraded mit Neustart", KephalaionAuto{Outcome: "upgraded", From: "v0.4.1", To: "v0.5.0", Restarted: true, Warnings: []string{"VS Code: Erweiterung veraltet"}, Check: check}, 0, true},
		{"upgraded ohne Neustart", KephalaionAuto{Outcome: "upgraded", From: "v0.4.1", To: "v0.5.0", Check: check}, 0, false},
		{"current", KephalaionAuto{Outcome: "current", From: "v0.5.0", To: "v0.5.0"}, 0, false},
		{"skipped hub", KephalaionAuto{Outcome: "skipped", Reason: "hub", From: "v0.4.1", To: "v0.5.0", Hint: "von Hand: kephalaion upgrade, danach die Nodes"}, 0, false},
		{"failed restart mit Rückweg", KephalaionAuto{Outcome: "failed", Reason: "restart", From: "v0.4.1", To: "v0.5.0", Restarted: true, RolledBack: boolPointer(true), Error: "Dienst lief nicht an", Hint: "Log ansehen"}, 1, true},
		{"failed restart ohne restarted", KephalaionAuto{Outcome: "failed", Reason: "restart", RolledBack: boolPointer(false), Error: "Dienst lief nicht an"}, 1, true},
		{"failed ohne Neustart", KephalaionAuto{Outcome: "failed", Reason: "download", From: "v0.4.1", To: "v0.5.0", Error: "Prüfsumme falsch"}, 1, false},
	} {
		t.Run(fall.name, func(t *testing.T) {
			fake := kephtest.NewFakeUpgrade(t)
			fake.SetAuto(t, kephtest.JSON(t, fall.auto), "", fall.exit)

			result, err := KephalaionAutoUpgrade(fake.Path)
			if err != nil {
				t.Fatalf("Fehler: %v", err)
			}
			if !reflect.DeepEqual(result, fall.auto) {
				t.Errorf("Ergebnis = %+v, erwartet %+v", result, fall.auto)
			}
			if result.NodeRestarted() != fall.restarted {
				t.Errorf("NodeRestarted = %v, erwartet %v", result.NodeRestarted(), fall.restarted)
			}
			if calls := fake.Calls(t); len(calls) != 1 || !reflect.DeepEqual(calls[0], autoArgs()) {
				t.Errorf("Aufrufe = %q", calls)
			}
		})
	}
}

// Exit 2 heißt: --auto unbekannt; KephalaionCheckUpgrade liefert dann den
// Report, mit und ohne neueres Release.
func TestKephalaionAutoUnbekanntUndCheck(t *testing.T) {
	for _, available := range []bool{true, false} {
		fake := kephtest.NewFakeUpgrade(t)
		fake.SetAuto(t, "", "flag provided but not defined: -auto\n", 2)
		report := KephalaionCheck{State: "ok", Version: "v0.4.0", Latest: "v0.4.1", UpdateAvailable: available, Method: "self", Command: "kephalaion upgrade", Hint: "kephalaion upgrade"}
		fake.SetCheck(t, kephtest.JSON(t, report), "", 0)

		if _, err := KephalaionAutoUpgrade(fake.Path); !errors.Is(err, ErrKephalaionAutoUnsupported) {
			t.Fatalf("Exit 2: Fehler %v, erwartet ErrKephalaionAutoUnsupported", err)
		}
		check, err := KephalaionCheckUpgrade(fake.Path)
		if err != nil || !reflect.DeepEqual(check, report) {
			t.Errorf("update_available %v: Ergebnis %+v, Fehler %v", available, check, err)
		}
		if calls := fake.Calls(t); len(calls) != 2 || !reflect.DeepEqual(calls[0], autoArgs()) || !reflect.DeepEqual(calls[1], checkArgs()) {
			t.Errorf("Aufrufe = %q", calls)
		}
	}
}

// --check mit state failed (Exit 1) ist ein Ergebnis, kein Fehler.
func TestKephalaionCheckUpgradeStateFailed(t *testing.T) {
	fake := kephtest.NewFakeUpgrade(t)
	fake.SetCheck(t, kephtest.JSON(t, KephalaionCheck{State: "failed", Error: "rate limit", Version: "v0.4.0"}), "", 1)
	check, err := KephalaionCheckUpgrade(fake.Path)
	if err != nil || check.State != "failed" || check.Error != "rate limit" {
		t.Errorf("Ergebnis %+v, Fehler %v", check, err)
	}
}

// Kein JSON ist ein eigener Fehler mit stdout und stderr; Text auf stderr
// neben JSON auf stdout stört nicht.
func TestKephalaionAutoKeinJSONUndStderr(t *testing.T) {
	fake := kephtest.NewFakeUpgrade(t)
	fake.SetAuto(t, "panic: irgendwas\n", "goroutine 1 [running]\n", 1)
	_, err := KephalaionAutoUpgrade(fake.Path)
	var noJSON *KephalaionNoJSONError
	if !errors.As(err, &noJSON) || noJSON.ExitCode != 1 || !strings.Contains(err.Error(), "panic: irgendwas") || !strings.Contains(err.Error(), "goroutine 1") {
		t.Errorf("kein JSON: Fehler %v", err)
	}

	fake.SetCheck(t, "", "", 0)
	if _, err := KephalaionCheckUpgrade(fake.Path); !errors.As(err, &noJSON) {
		t.Errorf("--check ohne Ausgabe: Fehler %v", err)
	}

	fake.SetAuto(t, kephtest.JSON(t, KephalaionAuto{Outcome: "current", From: "v0.4.1"}), "Hinweis: config gelesen\n", 0)
	result, err := KephalaionAutoUpgrade(fake.Path)
	if err != nil || result.Outcome != "current" {
		t.Errorf("stderr neben JSON: Ergebnis %+v, Fehler %v", result, err)
	}
}

// setKephalaionTimeouts setzt Frist und Wartezeit für die Dauer des Tests.
func setKephalaionTimeouts(t *testing.T, timeout, grace time.Duration) {
	t.Helper()
	beforeAuto, beforeGrace := KephalaionAutoTimeout, KephalaionStopGrace
	t.Cleanup(func() { KephalaionAutoTimeout, KephalaionStopGrace = beforeAuto, beforeGrace })
	KephalaionAutoTimeout, KephalaionStopGrace = timeout, grace
}

// Bei Fristablauf bekommt --auto SIGTERM und darf aufräumen; SIGKILL kommt
// nicht vor Ablauf der Wartezeit. Das Ergebnis ist „Ausgang unbekannt".
func TestKephalaionAutoFristSanftBeendet(t *testing.T) {
	setKephalaionTimeouts(t, 200*time.Millisecond, 5*time.Second)
	fake := kephtest.NewFakeUpgrade(t)
	fake.Hang(t, 0.5)

	started := time.Now()
	_, err := KephalaionAutoUpgrade(fake.Path)
	elapsed := time.Since(started)

	var unknown *KephalaionUnknownOutcomeError
	if !errors.As(err, &unknown) || !strings.Contains(err.Error(), "Ausgang unbekannt") {
		t.Fatalf("Fehler %v, erwartet Ausgang unbekannt", err)
	}
	if signals := fake.Signals(t); !reflect.DeepEqual(signals, []string{"term", "done"}) {
		t.Errorf("Signale = %q — erwartet SIGTERM und ein fertiges Aufräumen, kein SIGKILL davor", signals)
	}
	if elapsed < 700*time.Millisecond || elapsed > 4*time.Second {
		t.Errorf("Dauer %s — erwartet Frist plus Aufräumen, nicht die ganze Wartezeit", elapsed)
	}
}

// Räumt --auto länger auf als die Wartezeit, kommt danach SIGKILL — nicht
// früher.
func TestKephalaionAutoFristNachWartezeitKill(t *testing.T) {
	setKephalaionTimeouts(t, 200*time.Millisecond, 600*time.Millisecond)
	fake := kephtest.NewFakeUpgrade(t)
	fake.Hang(t, 5)

	started := time.Now()
	_, err := KephalaionAutoUpgrade(fake.Path)
	elapsed := time.Since(started)

	var unknown *KephalaionUnknownOutcomeError
	if !errors.As(err, &unknown) {
		t.Fatalf("Fehler %v, erwartet Ausgang unbekannt", err)
	}
	if signals := fake.Signals(t); !reflect.DeepEqual(signals, []string{"term"}) {
		t.Errorf("Signale = %q", signals)
	}
	if elapsed < 800*time.Millisecond || elapsed > 4*time.Second {
		t.Errorf("Dauer %s — erwartet Frist plus Wartezeit", elapsed)
	}
}
