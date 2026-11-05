package githubaccess

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// judged baut einen Dienst mit schon beurteilten Zugängen: Choose liest nur
// State, die Verfügbarkeit selbst prüft availability_test.go.
func judged(accesses ...Access) Service {
	for index := range accesses {
		if accesses[index].State == "" {
			accesses[index].State = StateAvailable
		}
	}
	return Service{Name: "s", Repo: "acme/web", Access: accesses}
}

// describeChoice nennt eine Wahl kurz, etwa „git=ssh host a api=gh login b“.
func describeChoice(choice Choice) string {
	part := func(access *Access) string {
		if access == nil {
			return "-"
		}
		return access.Describe()
	}
	return "git=" + part(choice.Git) + " api=" + part(choice.API)
}

var (
	sshHost = Access{Type: "ssh", Host: "github-example"}
	sshKey  = Access{Type: "ssh", Key: "/k/id"}
	ghAlice = Access{Type: "gh", Login: "alice"}
	ghBob   = Access{Type: "gh", Login: "bob"}
	tokenA  = Access{Type: "token", File: "/t/a.token"}
	public  = Access{Type: "public"}
)

func TestChooseFolgtDerReihenfolgeDerDatei(t *testing.T) {
	cases := []struct {
		name     string
		service  Service
		options  ChooseOptions
		expected string
	}{
		{"ssh vor gh: push über ssh, api über gh", judged(sshHost, ghAlice), ChooseOptions{}, "git=ssh host github-example api=gh login alice"},
		{"gh vor ssh: beide über gh", judged(ghAlice, sshHost), ChooseOptions{}, "git=gh login alice api=gh login alice"},
		{"token vor gh: api nimmt dasselbe token", judged(tokenA, ghAlice), ChooseOptions{}, "git=token file /t/a.token api=token file /t/a.token"},
		{"zwei gh: der erste", judged(ghAlice, ghBob), ChooseOptions{}, "git=gh login alice api=gh login alice"},
		{"nur ssh: api leer", judged(sshKey), ChooseOptions{}, "git=ssh key /k/id api=-"},
		{"nur public: git public, api leer", judged(public), ChooseOptions{}, "git=public api=-"},
		{"public vor gh: git public, api gh", judged(public, ghAlice), ChooseOptions{}, "git=public api=gh login alice"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := describeChoice(Choose(c.service, c.options)); got != c.expected {
				t.Errorf("Wahl %s, erwartet %s", got, c.expected)
			}
		})
	}
}

func TestChooseUeberspringtDeaktivierteUndNichtVerfuegbare(t *testing.T) {
	disabledToken := tokenA
	disabledToken.Disabled, disabledToken.State = true, StateDisabled
	missingKey := sshKey
	missingKey.State, missingKey.Reason = StateUnavailable, "Die Schlüsseldatei fehlt."
	service := judged(disabledToken, missingKey, ghAlice)

	choice := Choose(service, ChooseOptions{})
	if got := describeChoice(choice); got != "git=gh login alice api=gh login alice" {
		t.Errorf("Wahl %s", got)
	}
	if choice.Git != &service.Access[2] || !choice.SameAccess() {
		t.Errorf("die Zeiger zeigen nicht auf denselben Zugang des Dienstes")
	}

	// Alle drei ausgefallen: beide Kanäle leer, und die Meldung nennt je
	// Zugang den Grund.
	unknownGH := ghAlice
	unknownGH.State, unknownGH.Reason = StateUnavailable, "Der Login alice ist in gh nicht hinterlegt."
	service = judged(disabledToken, missingKey, unknownGH)
	choice = Choose(service, ChooseOptions{})
	if !choice.Empty() {
		t.Fatalf("Wahl %s, erwartet beide leer", describeChoice(choice))
	}
	message := NoAccessMessage(service, ChooseOptions{})
	for _, part := range []string{"access[0] token: deaktiviert", "access[1] ssh: nicht verfügbar — Die Schlüsseldatei fehlt.", "access[2] gh: nicht verfügbar — Der Login alice"} {
		if !strings.Contains(message, part) {
			t.Errorf("Meldung ohne %q: %s", part, message)
		}
	}
}

func TestChooseOptionen(t *testing.T) {
	cases := []struct {
		name     string
		service  Service
		options  ChooseOptions
		expected string
		message  string // Teil der Meldung, wenn beide Kanäle leer sind
	}{
		{"--write überspringt public", judged(public, sshHost), ChooseOptions{Write: true}, "git=ssh host github-example api=-", ""},
		{"--write mit nur public: beide leer", judged(public), ChooseOptions{Write: true}, "git=- api=-", "access[0] public: public, durch --write ausgeschlossen"},
		{"--use token", judged(sshHost, ghAlice, tokenA), ChooseOptions{Use: "token"}, "git=token file /t/a.token api=token file /t/a.token", ""},
		{"--use ssh: api leer", judged(ghAlice, sshHost), ChooseOptions{Use: "ssh"}, "git=ssh host github-example api=-", ""},
		{"--use gh ohne gh: beide leer", judged(sshHost), ChooseOptions{Use: "gh"}, "git=- api=-", "access[0] ssh: durch --use gh ausgeschlossen"},
		{"--skip ssh", judged(sshHost, ghAlice), ChooseOptions{Skip: []string{"ssh"}}, "git=gh login alice api=gh login alice", ""},
		{"--skip mehrfach", judged(sshHost, ghAlice, tokenA, public), ChooseOptions{Skip: []string{"ssh", "gh"}}, "git=token file /t/a.token api=token file /t/a.token", ""},
		{"--skip ssh --skip token: public, api leer", judged(sshHost, tokenA, public), ChooseOptions{Skip: []string{"ssh", "token"}}, "git=public api=-", ""},
		{"--skip alles: beide leer", judged(sshHost, ghAlice), ChooseOptions{Skip: []string{"ssh", "gh"}}, "git=- api=-", "access[1] gh: durch --skip gh ausgeschlossen"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			choice := Choose(c.service, c.options)
			if got := describeChoice(choice); got != c.expected {
				t.Errorf("Wahl %s, erwartet %s", got, c.expected)
			}
			if c.message != "" {
				if message := NoAccessMessage(c.service, c.options); !strings.Contains(message, c.message) {
					t.Errorf("Meldung ohne %q: %s", c.message, message)
				}
			}
		})
	}
}

func TestChooseBeideLeerOhneZugang(t *testing.T) {
	if choice := Choose(Service{Name: "s", Repo: "a/b"}, ChooseOptions{}); !choice.Empty() {
		t.Errorf("Wahl %s", describeChoice(choice))
	}
	if message := NoAccessMessage(Service{}, ChooseOptions{}); message != "kein Zugang eingetragen" {
		t.Errorf("Meldung %q", message)
	}
}

// Ohne Judge gilt kein Zugang als verfügbar: Choose rät nicht.
func TestChooseVerlangtBeurteilteZugaenge(t *testing.T) {
	service := Service{Name: "s", Repo: "a/b", Access: []Access{{Type: "public"}}}
	if choice := Choose(service, ChooseOptions{}); !choice.Empty() {
		t.Errorf("Wahl %s ohne Judge", describeChoice(choice))
	}
}

// Mit Judge: dieselbe Wahl aus echten Zuständen — die Token-Datei fehlt,
// gh ist ohne den Login, public bleibt.
func TestChooseNachJudge(t *testing.T) {
	fakeTools(t, true)
	ghHosts(t, "bob")
	dir := t.TempDir()
	token := filepath.Join(dir, "ok.token")
	if err := os.WriteFile(token, []byte("ghp_x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	services := Judge(Services{{Name: "s", Repo: "a/b", Access: []Access{
		{Type: "token", File: filepath.Join(dir, "fehlt.token")},
		{Type: "gh", Login: "alice"},
		{Type: "ssh", Host: "github-example"},
		{Type: "token", File: token},
		{Type: "public"},
	}}})
	choice := Choose(services[0], ChooseOptions{})
	if got := describeChoice(choice); got != "git=ssh host github-example api=token file "+token {
		t.Errorf("Wahl %s", got)
	}
	choice = Choose(services[0], ChooseOptions{Skip: []string{"ssh", "token"}})
	if got := describeChoice(choice); got != "git=public api=-" {
		t.Errorf("Wahl mit --skip ssh --skip token: %s", got)
	}
}

func TestValidType(t *testing.T) {
	for _, typ := range []string{"gh", "token", "ssh", "public"} {
		if !ValidType(typ) {
			t.Errorf("%s gilt nicht als Typ", typ)
		}
	}
	for _, typ := range []string{"", "https", "GH"} {
		if ValidType(typ) {
			t.Errorf("%q gilt als Typ", typ)
		}
	}
}

// describeIndex nennt die Indizes eines Kanalpaars kurz, etwa „git=0 api=-“.
func describeIndex(index ChannelIndex) string {
	part := func(value *int) string {
		if value == nil {
			return "-"
		}
		return strconv.Itoa(*value)
	}
	return "git=" + part(index.Git) + " api=" + part(index.API)
}

// Select übersetzt Choose ohne Optionen und mit --write in Indizes von
// access: Reihenfolge der Datei, public nur ohne --write, api nur über
// token oder gh, deaktivierte und nicht verfügbare nie.
func TestSelectNenntDieIndizesVonChoose(t *testing.T) {
	disabledToken := tokenA
	disabledToken.Disabled, disabledToken.State = true, StateDisabled
	missingKey := sshKey
	missingKey.State, missingKey.Reason = StateUnavailable, "Die Schlüsseldatei fehlt."
	cases := []struct {
		name        string
		service     Service
		read, write string
	}{
		{"Reihenfolge der Datei", judged(sshHost, ghAlice, tokenA), "git=0 api=1", "git=0 api=1"},
		{"gh vor ssh: beide Kanäle derselbe", judged(ghAlice, sshHost), "git=0 api=0", "git=0 api=0"},
		{"public nur in read", judged(public, sshKey), "git=0 api=-", "git=1 api=-"},
		{"nur public: write ganz leer", judged(public), "git=0 api=-", "git=- api=-"},
		{"api überspringt ssh und public", judged(public, sshHost, ghBob), "git=0 api=2", "git=1 api=2"},
		{"deaktiviert und nicht verfügbar nie", judged(disabledToken, missingKey, ghAlice), "git=2 api=2", "git=2 api=2"},
		{"kein verfügbarer Zugang: vier null", judged(disabledToken, missingKey), "git=- api=-", "git=- api=-"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			selection := Select(c.service)
			if got := describeIndex(selection.Read); got != c.read {
				t.Errorf("read %s, erwartet %s", got, c.read)
			}
			if got := describeIndex(selection.Write); got != c.write {
				t.Errorf("write %s, erwartet %s", got, c.write)
			}
			// Zurück in eine Wahl: dieselben Zeiger wie Choose.
			if got, want := describeChoice(selection.Read.Choice(c.service.Access)), describeChoice(Choose(c.service, ChooseOptions{})); got != want {
				t.Errorf("read als Wahl %s, Choose %s", got, want)
			}
			if got, want := describeChoice(selection.Write.Choice(c.service.Access)), describeChoice(Choose(c.service, ChooseOptions{Write: true})); got != want {
				t.Errorf("write als Wahl %s, Choose %s", got, want)
			}
		})
	}
}

// Im JSON steht null für einen leeren Kanal, nicht ein fehlendes Feld.
func TestSelectionJSON(t *testing.T) {
	encoded := string(mustJSON(t, Select(judged(public))))
	if encoded != `{"read":{"git":0,"api":null},"write":{"git":null,"api":null}}` {
		t.Errorf("JSON %s", encoded)
	}
}

// Judge setzt selected an jedem Dienst aus den Zuständen, die es eben
// eingetragen hat.
func TestJudgeSetztSelected(t *testing.T) {
	fakeTools(t, true)
	ghHosts(t, "bob")
	services := Judge(Services{
		{Name: "a", Repo: "x/a", Access: []Access{{Type: "gh", Login: "alice"}, {Type: "ssh", Host: "github-example"}, {Type: "public"}}},
		{Name: "b", Repo: "x/b", Access: []Access{{Type: "gh", Login: "alice", Disabled: true}}},
	})
	if services[0].Selected == nil || services[1].Selected == nil {
		t.Fatalf("selected fehlt: %+v", services)
	}
	if got := describeIndex(services[0].Selected.Read) + " / " + describeIndex(services[0].Selected.Write); got != "git=1 api=- / git=1 api=-" {
		t.Errorf("a: %s", got)
	}
	if !services[1].Selected.Read.Empty() || !services[1].Selected.Write.Empty() {
		t.Errorf("b: %+v", services[1].Selected)
	}
}
