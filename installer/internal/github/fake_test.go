package github

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// fakeRunner ist der Grund für das Runner-Interface: die Tests dieses Pakets
// laufen ohne Netz, ohne gh und ohne Repo. Geantwortet wird auf das erste
// passende Präfix, in der angegebenen Reihenfolge — eine Map wäre hier falsch,
// weil sich Präfixe überlappen und ihre Reihenfolge zufällig wäre.
type fakeRunner struct {
	answers []fakeAnswer
	// calls hält jeden Aufruf fest. Nur so lässt sich belegen, dass ein
	// abgeschalteter Zustand wirklich keinen Subprozess startet.
	calls []string
}

type fakeAnswer struct {
	prefix string
	out    string
	err    error
}

func (f *fakeRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, error) {
	call := strings.TrimSpace(name + " " + strings.Join(args, " "))
	f.calls = append(f.calls, call)
	for _, answer := range f.answers {
		if strings.HasPrefix(call, answer.prefix) {
			if answer.err != nil {
				return nil, answer.err
			}
			return []byte(answer.out), nil
		}
	}
	return nil, fmt.Errorf("unerwarteter Aufruf: %s", call)
}

func (f *fakeRunner) client() *Client {
	return &Client{Runner: f, Dir: "/nicht/vorhanden", Timeout: time.Second}
}

// ghError baut den Fehler, wie ihn ein gescheitertes gh liefert: Exit-Code und
// die Zeile auf stderr, an der die Einordnung hängt.
func ghError(stderr string) error {
	return &CommandError{Name: "gh", Args: []string{"repo", "view"}, ExitCode: 1, Stderr: stderr, Err: fmt.Errorf("exit status 1")}
}
