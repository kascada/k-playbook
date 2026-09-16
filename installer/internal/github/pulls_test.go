package github

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readFixture liest eine Fixture und überspringt die Prüfung, wenn sie gar
// nicht da ist.
//
// Das ist kein Nachlassen, sondern Absicht: Das Log eines Laufs fällt unter die
// Ignore-Regel `*.log`, liegt also außerhalb von git und wird extern gesichert.
// Im Clone eines Runners fehlt es deshalb. Ein Test, der dort nie grün werden
// kann, schützt nichts mehr — er färbt die Anzeige dauerhaft rot, und rot, das
// immer rot ist, wird nach kurzer Zeit nicht mehr gelesen. Dann fällt auch ein
// echter Fehler nicht mehr auf. Übersprungen bleibt die Lücke sichtbar, ohne
// das Signal zu verbrennen.
//
// Ein Lesefehler aus einem anderen Grund bleibt ein Fehlschlag.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if errors.Is(err, os.ErrNotExist) {
		t.Skipf("Fixture %s liegt nicht vor (nicht in git, siehe .gitignore) — Prüfung übersprungen.", name)
	}
	if err != nil {
		t.Fatalf("Fixture %s: %v", name, err)
	}
	return raw
}

func pullByNumber(t *testing.T, pulls []PullRequest, number int) PullRequest {
	t.Helper()
	for _, pull := range pulls {
		if pull.Number == number {
			return pull
		}
	}
	t.Fatalf("PR %d fehlt in der Liste", number)
	return PullRequest{}
}

func TestPullsTrenntOffeneVonGeschlossenen(t *testing.T) {
	pulls := ParsePulls(readFixture(t, "pulls.json"))

	if pulls.State != StateOK {
		t.Fatalf("Zustand %q, erwartet ok", pulls.State)
	}
	if len(pulls.Open) != 4 {
		t.Errorf("%d offene PRs, erwartet 4", len(pulls.Open))
	}
	if len(pulls.Closed) != 2 {
		t.Errorf("%d geschlossene PRs, erwartet 2", len(pulls.Closed))
	}
	if pulls.ClosedLimit != ClosedPullRequestLimit {
		t.Errorf("ClosedLimit %d, erwartet %d — ohne die Zahl sähe eine gedeckelte Liste vollständig aus", pulls.ClosedLimit, ClosedPullRequestLimit)
	}
}

func TestPullsZeigtQuelleUndZiel(t *testing.T) {
	pulls := ParsePulls(readFixture(t, "pulls.json"))

	normal := pullByNumber(t, pulls.Open, 42)
	if normal.Head != "feature/github-kopfzeile" || normal.Base != "main" {
		t.Errorf("PR 42: %s → %s, erwartet feature/github-kopfzeile → main", normal.Head, normal.Base)
	}
	// Ziel ist der Default-Branch: keine Marke.
	if normal.NonDefaultBase {
		t.Error("PR 42 ist als Ziel außerhalb des Default-Branchs markiert, geht aber nach main")
	}
	if normal.Fork {
		t.Error("PR 42 ist als Fork markiert, kommt aber aus dem Repo selbst")
	}
	if normal.Checks != "SUCCESS" || normal.ReviewDecision != "APPROVED" || normal.Mergeable != "MERGEABLE" {
		t.Errorf("PR 42: Checks %q, Review %q, Mergebarkeit %q", normal.Checks, normal.ReviewDecision, normal.Mergeable)
	}
	if normal.Additions != 320 || normal.Deletions != 18 || normal.ChangedFiles != 7 {
		t.Errorf("PR 42: +%d/−%d in %d Dateien, erwartet +320/−18 in 7", normal.Additions, normal.Deletions, normal.ChangedFiles)
	}
}

func TestPullsKennzeichnetEntwurfForkUndFremdesZiel(t *testing.T) {
	pulls := ParsePulls(readFixture(t, "pulls.json"))

	// Ein Entwurf ist ein eigener Zustand, kein Zusatz zu „offen".
	if state := pullByNumber(t, pulls.Open, 43).State; state != "draft" {
		t.Errorf("PR 43: Zustand %q, erwartet draft", state)
	}

	fremd := pullByNumber(t, pulls.Open, 44)
	if !fremd.Fork || fremd.ForkOwner != "fremde-person" {
		t.Errorf("PR 44: Fork %v von %q, erwartet true von fremde-person", fremd.Fork, fremd.ForkOwner)
	}
	if !fremd.NonDefaultBase {
		t.Error("PR 44 geht nach release/0.9 und ist nicht als Ziel außerhalb des Default-Branchs markiert")
	}
	if fremd.Mergeable != "CONFLICTING" {
		t.Errorf("PR 44: Mergebarkeit %q, erwartet CONFLICTING", fremd.Mergeable)
	}

	bot := pullByNumber(t, pulls.Open, 45)
	if !bot.Dependabot {
		t.Errorf("PR 45 von %q ist nicht als Dependabot erkannt", bot.Author)
	}
}

func TestPullsErkenntGemergtUndGeschlossen(t *testing.T) {
	pulls := ParsePulls(readFixture(t, "pulls.json"))

	gemergt := pullByNumber(t, pulls.Closed, 40)
	if gemergt.State != "merged" || gemergt.MergedAt == "" {
		t.Errorf("PR 40: Zustand %q, gemergt am %q", gemergt.State, gemergt.MergedAt)
	}

	// Ein gelöschtes Konto liefert author: null. Das darf die Liste nicht
	// zerreißen.
	geschlossen := pullByNumber(t, pulls.Closed, 39)
	if geschlossen.State != "closed" {
		t.Errorf("PR 39: Zustand %q, erwartet closed", geschlossen.State)
	}
	if geschlossen.Author != "" {
		t.Errorf("PR 39: Autor %q, erwartet leer", geschlossen.Author)
	}
}

// Die Ansicht schreibt nichts nach GitHub. Was ein PR anbietet, ist der Befehl
// zum Kopieren — kein Knopf, der etwas auslöst.
func TestPullsNenntDenEinstiegUeberKPrReview(t *testing.T) {
	pulls := ParsePulls(readFixture(t, "pulls.json"))

	if command := pullByNumber(t, pulls.Open, 42).Command; command != "/k-pr-review 42" {
		t.Errorf("Befehl %q, erwartet /k-pr-review 42", command)
	}
}

// Dieses Repo hat keine PRs. Die leere Liste ist ein gültiger Zustand und muss
// als solcher aussehen — nicht als Fehler und nicht als null.
func TestPullsOhneEintraegeIstEinGueltigerZustand(t *testing.T) {
	pulls := ParsePulls(readFixture(t, "pulls-leer.json"))

	if pulls.State != StateOK {
		t.Fatalf("Zustand %q, erwartet ok", pulls.State)
	}
	if pulls.Open == nil || pulls.Closed == nil {
		t.Fatal("leere Listen sind null statt []; die Seite müsste den Fall sonst eigens abfangen")
	}
	if len(pulls.Open) != 0 || len(pulls.Closed) != 0 {
		t.Errorf("%d offene und %d geschlossene PRs, erwartet keine", len(pulls.Open), len(pulls.Closed))
	}
}

// Eine Abfrage statt vieler: die Karte darf nicht je PR einen Netzaufruf
// machen.
func TestPullsFragtGenauEinmalAb(t *testing.T) {
	runner := &fakeRunner{answers: []fakeAnswer{
		{prefix: "gh api graphql", out: string(readFixture(t, "pulls.json"))},
	}}
	pulls := runner.client().FetchPulls(context.Background(), "kascada/k-playbook")

	if pulls.State != StateOK {
		t.Fatalf("Zustand %q, erwartet ok (%s)", pulls.State, pulls.Message)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("%d Aufrufe, erwartet 1: %v", len(runner.calls), runner.calls)
	}
	if !strings.Contains(runner.calls[0], "gh api graphql") {
		t.Errorf("Aufruf %q, erwartet eine GraphQL-Abfrage", runner.calls[0])
	}
}

func TestPullsOhneRepoMeldetDenZustand(t *testing.T) {
	runner := &fakeRunner{}
	pulls := runner.client().FetchPulls(context.Background(), "")

	if pulls.State != StateNoRemote {
		t.Errorf("Zustand %q, erwartet no-remote", pulls.State)
	}
	if len(runner.calls) != 0 {
		t.Errorf("ohne Repo wurde trotzdem gefragt: %v", runner.calls)
	}
}

func TestValidRepoWeistFreienTextAb(t *testing.T) {
	cases := map[string]bool{
		"kascada/k-playbook":    true,
		"kascada/k-playbook.go": true,
		"kascada":               false,
		"kascada/a/b":           false,
		"kascada/k playbook":    false,
		"--version":             false,
		"":                      false,
	}
	for repo, want := range cases {
		if got := ValidRepo(repo); got != want {
			t.Errorf("ValidRepo(%q) = %v, erwartet %v", repo, got, want)
		}
	}
}
