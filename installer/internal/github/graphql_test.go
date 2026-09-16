package github

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Die Rümpfe und stderr-Zeilen dieser Tests sind am echten gh 2.46.0 gemessen,
// außer beim Rate-Limit (siehe dort). gh endet bei jedem Feld `errors` mit
// Exit 1, schreibt den Rumpf auf stdout und `gh: <message>` auf stderr — den
// Typ nur auf stdout. Befund: k-playbook-local/material/befunde/
// github-stand-in-der-oberflaeche.md.

// graphQLFailure ist ein gescheiterter `gh api graphql`: Rumpf auf stdout,
// Meldung auf stderr, Exit 1.
func graphQLFailure(stdout, stderr string) *fakeRunner {
	return &fakeRunner{answers: []fakeAnswer{{
		prefix: "gh api graphql",
		out:    stdout,
		err:    &CommandError{Name: "gh", Args: []string{"api", "graphql"}, ExitCode: 1, Stderr: stderr, Err: errors.New("exit status 1")},
	}}}
}

const (
	// Gemessen: repository(owner:"kascada-nonexistent-zz9", name:"nope").
	notFoundBody   = `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"],"locations":[{"line":1,"column":37}],"message":"Could not resolve to a Repository with the name 'kascada-nonexistent-zz9/nope'."}]}`
	notFoundStderr = "gh: Could not resolve to a Repository with the name 'kascada-nonexistent-zz9/nope'.\n"

	// Gemessen: ein Teilfehler. Das Repo antwortet, nur das Feld collaborators
	// ist dem Konto mit READ verweigert.
	forbiddenBody   = `{"data":{"repository":{"name":"k-playbook","collaborators":null}},"errors":[{"type":"FORBIDDEN","path":["repository","collaborators"],"locations":[{"line":1,"column":58}],"message":"You do not have permission to view repository collaborators."}]}`
	forbiddenStderr = "gh: You do not have permission to view repository collaborators.\n"

	// Nicht gemessen — dafür müsste das Kontingent verbraucht werden. Der Typ
	// ist der dokumentierte, der Wortlaut der aus Fremdberichten (cli/cli
	// #8321, GitHub Community #184351); die Form `gh: …` folgt aus
	// pkg/cmd/api/api.go:451.
	rateLimitedBody   = `{"data":null,"errors":[{"type":"RATE_LIMITED","message":"API rate limit already exceeded for user ID 1."}]}`
	rateLimitedStderr = "gh: API rate limit already exceeded for user ID 1.\n"
)

func TestPullsOrdnetGraphQLFehlerEin(t *testing.T) {
	cases := []struct {
		name   string
		stdout string
		stderr string
		want   State
	}{
		{"NOT_FOUND", notFoundBody, notFoundStderr, StateNoAccess},
		{"RATE_LIMITED am Typ", rateLimitedBody, rateLimitedStderr, StateRateLimited},
		// Ohne Rumpf bleibt der Text. Beide belegten Wortlaute müssen treffen.
		{"RATE_LIMITED am Text, zweiter Wortlaut", "", rateLimitedStderr, StateRateLimited},
		{"RATE_LIMITED am Text, erster Wortlaut", "", "gh: API rate limit exceeded for user ID 1.\n", StateRateLimited},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			pulls := graphQLFailure(testCase.stdout, testCase.stderr).client().FetchPulls(context.Background(), "kascada/k-playbook")
			if pulls.State != testCase.want {
				t.Fatalf("Zustand %q, erwartet %q (%s)", pulls.State, testCase.want, pulls.Message)
			}
			if pulls.Message == "" {
				t.Error("Zustand ohne Erklärung")
			}
		})
	}
}

// Ein Teilfehler ist kein Leerzustand und kein fehlender Zugriff aufs Repo:
// das Repo hat geantwortet, verweigert war ein Feld. Die Karte nennt die
// Meldung von gh, denn sie sagt hier mehr als jeder feste Satz — und zeigt
// keine halben Daten, deren fehlende Felder wie leere aussähen.
func TestPullsZeigtTeilfehlerAlsFehlerUndNichtAlsLeer(t *testing.T) {
	pulls := graphQLFailure(forbiddenBody, forbiddenStderr).client().FetchPulls(context.Background(), "kascada/k-playbook")

	if pulls.State != StateError {
		t.Fatalf("Zustand %q, erwartet error (%s)", pulls.State, pulls.Message)
	}
	if !strings.Contains(pulls.Message, "You do not have permission to view repository collaborators.") {
		t.Errorf("Meldung nennt die Ursache nicht: %q", pulls.Message)
	}
	if strings.Contains(pulls.Message, "gh: ") {
		t.Errorf("Präfix von gh in der Meldung: %q", pulls.Message)
	}
	if len(pulls.Open) != 0 || len(pulls.Closed) != 0 {
		t.Errorf("halbe Daten gezeigt: %d offen, %d geschlossen", len(pulls.Open), len(pulls.Closed))
	}
}

// Kommt ein Feld `errors` doch einmal mit Exit 0 an, wird daraus nicht
// „keine Pull Requests".
func TestParsePullsMachtAusFehlerfeldKeinenLeerzustand(t *testing.T) {
	pulls := ParsePulls([]byte(notFoundBody))
	if pulls.State != StateNoAccess {
		t.Fatalf("Zustand %q, erwartet no-access (%s)", pulls.State, pulls.Message)
	}
}

// Die Frist geht dem Rumpf vor: eine abgebrochene Abfrage ist ein Zeitfehler,
// auch wenn stdout schon einen Fehlertyp trägt.
func TestPullsFristGehtDemRumpfVor(t *testing.T) {
	runner := &stallingRunner{stderr: notFoundStderr}
	client := &Client{Runner: runner, Dir: "/nicht/vorhanden", Timeout: 30 * time.Millisecond}
	if state := client.FetchPulls(context.Background(), "kascada/k-playbook").State; state != StateTimeout {
		t.Errorf("Zustand %q, erwartet timeout", state)
	}
}
