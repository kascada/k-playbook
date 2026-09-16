package github

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// withAnswer setzt eine Antwort vor die gesunden, sodass sie gewinnt.
func withAnswer(answer fakeAnswer) *fakeRunner {
	runner := healthyRunner()
	runner.answers = append([]fakeAnswer{answer}, runner.answers...)
	return runner
}

func TestOverviewHinweiseSindImGutfallOK(t *testing.T) {
	notes := healthyRunner().client().FetchOverview(context.Background()).Notes
	for name, note := range map[string]FieldNote{"Konto": notes.Account, "Remote": notes.Remote, "Tag": notes.Tag, "Workflows": notes.Workflows} {
		if note.State != FieldOK {
			t.Errorf("%s: Hinweis %+v, erwartet ok", name, note)
		}
	}
}

// Je Feld ein Leerzustand und ein Fehler. Beide lassen das Feld leer; nur der
// Hinweis unterscheidet sie, und genau darauf kommt es an.
func TestOverviewTrenntNichtsDaVonNichtLesbar(t *testing.T) {
	notARepo := ghError("fatal: not a git repository (or any of the parent directories): .git")
	cases := []struct {
		name   string
		answer fakeAnswer
		note   func(Overview) FieldNote
		want   FieldState
	}{
		// Die git-Meldungen sind an git 2.53 gemessen.
		{"kein Tag", fakeAnswer{prefix: "git describe", err: ghError("fatal: No names found, cannot describe anything.")}, tagNote, FieldNone},
		{"Tag nicht lesbar", fakeAnswer{prefix: "git describe", err: notARepo}, tagNote, FieldUnreadable},
		{"Commits seit dem Tag nicht lesbar", fakeAnswer{prefix: "git rev-list", err: notARepo}, tagNote, FieldUnreadable},
		{"Datum des Tags nicht lesbar", fakeAnswer{prefix: "git log", err: notARepo}, tagNote, FieldUnreadable},
		{"kein Remote", fakeAnswer{prefix: "git remote get-url", err: ghError("error: No such remote 'origin'")}, remoteNote, FieldNone},
		{"Remote nicht lesbar", fakeAnswer{prefix: "git remote get-url", err: notARepo}, remoteNote, FieldUnreadable},
		{"keine Läufe", fakeAnswer{prefix: "gh run list", out: "[]"}, workflowsNote, FieldNone},
		{"Läufe nicht lesbar (gh)", fakeAnswer{prefix: "gh run list", err: ghError("dial tcp: lookup api.github.com: no such host")}, workflowsNote, FieldUnreadable},
		{"Läufe nicht lesbar (JSON)", fakeAnswer{prefix: "gh run list", out: "kein json"}, workflowsNote, FieldUnreadable},
		{"Konto nicht lesbar", fakeAnswer{prefix: "gh api user", err: ghError("dial tcp: lookup api.github.com: no such host")}, accountNote, FieldUnreadable},
		{"Konto ohne Namen", fakeAnswer{prefix: "gh api user", out: "{}"}, accountNote, FieldUnreadable},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			overview := withAnswer(testCase.answer).client().FetchOverview(context.Background())
			if overview.State != StateOK {
				t.Fatalf("Zustand %q, erwartet ok: ein einzelnes Feld macht die Kopfzeile nicht falsch", overview.State)
			}
			note := testCase.note(overview)
			if note.State != testCase.want {
				t.Fatalf("Hinweis %+v, erwartet %q", note, testCase.want)
			}
			if note.Message == "" {
				t.Error("Hinweis ohne Satz")
			}
		})
	}
}

// Scheitert erst die Zählung, bleibt der Tag stehen — aber „0 Commits seitdem"
// wird nicht behauptet.
func TestOverviewBehauptetOhneZaehlungKeineNullCommits(t *testing.T) {
	overview := withAnswer(fakeAnswer{prefix: "git rev-list", err: ghError("fatal: bad revision")}).client().FetchOverview(context.Background())
	if overview.LastTag != "v0.9.0" {
		t.Errorf("Tag %q, erwartet v0.9.0", overview.LastTag)
	}
	if overview.Notes.Tag.State == FieldOK {
		t.Errorf("Hinweis %+v: die Zählung ist gescheitert, der Tag gilt nicht als vollständig gelesen", overview.Notes.Tag)
	}
}

// expiringRunner beantwortet den ersten Aufruf richtig — aber erst, nachdem
// das Budget der Anfrage abgelaufen ist. Jeder weitere Aufruf hätte eine
// gesunde Antwort; er darf nur gar nicht mehr stattfinden.
type expiringRunner struct {
	mu    sync.Mutex
	after time.Duration
	calls []string
}

func (e *expiringRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, error) {
	e.mu.Lock()
	e.calls = append(e.calls, name+" "+strings.Join(args, " "))
	first := len(e.calls) == 1
	e.mu.Unlock()
	if first {
		time.Sleep(e.after)
	}
	return healthyRunner().Run(context.Background(), "", name, args...)
}

// Seit dem Budget je Anfrage gibt es eine weitere Ursache für ein leeres Feld:
// der erste Aufruf gelingt, das Budget ist danach verbraucht. Dann steht das
// Repo da, und jedes übrige Feld sagt „Frist abgelaufen" — nicht „kein Tag",
// nicht „keine Läufe", nicht „unbekannt".
func TestOverviewMeldetVerbrauchtesBudgetJeFeld(t *testing.T) {
	const budget = 30 * time.Millisecond
	runner := &expiringRunner{after: 2 * budget}
	client := &Client{Runner: runner, Dir: "/nicht/vorhanden", Timeout: DefaultTimeout}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	overview := client.FetchOverview(ctx)
	if overview.State != StateOK || overview.Repo != "kascada/k-playbook" {
		t.Fatalf("Zustand %q, Repo %q: der erste Aufruf ist gelungen", overview.State, overview.Repo)
	}
	for name, note := range map[string]FieldNote{"Konto": overview.Notes.Account, "Remote": overview.Notes.Remote, "Tag": overview.Notes.Tag, "Workflows": overview.Notes.Workflows} {
		if note.State != FieldTimeout {
			t.Errorf("%s: Hinweis %+v, erwartet timeout", name, note)
		}
		if !strings.Contains(note.Message, "Frist") {
			t.Errorf("%s: Satz nennt die Frist nicht: %q", name, note.Message)
		}
	}
	// Nach dem verbrauchten Budget startet kein Prozess mehr.
	if len(runner.calls) != 1 {
		t.Errorf("%d Aufrufe, erwartet 1: %v", len(runner.calls), runner.calls)
	}
}

func tagNote(o Overview) FieldNote       { return o.Notes.Tag }
func remoteNote(o Overview) FieldNote    { return o.Notes.Remote }
func workflowsNote(o Overview) FieldNote { return o.Notes.Workflows }
func accountNote(o Overview) FieldNote   { return o.Notes.Account }
