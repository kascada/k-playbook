package notify

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"unicode/utf16"
)

// call ist ein aufgezeichneter Programmaufruf.
type call struct {
	name string
	args []string
}

// fakeHost baut die Umgebung eines Rechners nach: welche Programme da sind,
// welche Variablen gesetzt sind, ob WSL oder Container. Zurück kommt die
// Liste der Aufrufe, die Send gemacht hat.
type fakeHost struct {
	programs  map[string]bool
	env       map[string]string
	os        string
	wsl       bool
	container string
	fail      map[string]string // Programm → stderr, mit dem es scheitert
	calls     *[]call
}

func install(t *testing.T, host fakeHost) *[]call {
	t.Helper()
	calls := &[]call{}

	before := struct {
		lookPath        func(string) (string, error)
		runCommand      func(context.Context, string, ...string) error
		getenv          func(string) string
		goos            string
		isWSL           func() bool
		containerMarker func() (string, bool)
	}{lookPath, runCommand, getenv, goos, isWSL, containerMarker}
	t.Cleanup(func() {
		lookPath, runCommand, getenv, goos, isWSL, containerMarker =
			before.lookPath, before.runCommand, before.getenv, before.goos, before.isWSL, before.containerMarker
	})

	lookPath = func(name string) (string, error) {
		if host.programs[name] {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("nicht installiert")
	}
	runCommand = func(_ context.Context, name string, args ...string) error {
		*calls = append(*calls, call{name: name, args: args})
		if stderr, failed := host.fail[name]; failed {
			return errors.New(name + ": " + stderr)
		}
		return nil
	}
	getenv = func(name string) string { return host.env[name] }
	goos = host.os
	if goos == "" {
		goos = "linux"
	}
	isWSL = func() bool { return host.wsl }
	containerMarker = func() (string, bool) { return host.container, host.container != "" }
	return calls
}

// decodePowerShell holt das Skript aus dem -EncodedCommand-Argument zurück.
func decodePowerShell(t *testing.T, args []string) string {
	t.Helper()
	for i, arg := range args {
		if arg == "-EncodedCommand" && i+1 < len(args) {
			raw, err := base64.StdEncoding.DecodeString(args[i+1])
			if err != nil {
				t.Fatalf("EncodedCommand ist kein Base64: %v", err)
			}
			units := make([]uint16, len(raw)/2)
			for j := range units {
				units[j] = uint16(raw[2*j]) | uint16(raw[2*j+1])<<8
			}
			return string(utf16.Decode(units))
		}
	}
	t.Fatalf("kein -EncodedCommand in %v", args)
	return ""
}

// WSL mit powershell.exe: ein Aufruf, Titel und Inhalt escaped im XML, der
// Ton nur, wenn verlangt.
func TestSendWSLToast(t *testing.T) {
	calls := install(t, fakeHost{programs: map[string]bool{"powershell.exe": true}, wsl: true})

	result, err := Send(context.Background(), Request{Title: "Task <1> fertig", Body: "Kleist's Lauf & Ende"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if result.Channel != ChannelWindowsToast || result.SoundPlayed {
		t.Fatalf("Ergebnis: %+v", result)
	}
	if len(*calls) != 1 || (*calls)[0].name != "powershell.exe" {
		t.Fatalf("Aufrufe: %+v", *calls)
	}
	script := decodePowerShell(t, (*calls)[0].args)
	for _, want := range []string{
		`<text id="1">Task &lt;1&gt; fertig</text>`,
		`<text id="2">Kleist&#39;s Lauf &amp; Ende</text>`,
		`<audio silent="true"/>`,
		"LoadXml(",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("Skript ohne %q:\n%s", want, script)
		}
	}
	if strings.Contains(script, "SoundPlayer") {
		t.Errorf("ohne Sound wird kein Ton gespielt:\n%s", script)
	}

	result, err = Send(context.Background(), Request{Title: "Fertig", Sound: true})
	if err != nil {
		t.Fatalf("Send mit Ton: %v", err)
	}
	if !result.SoundPlayed {
		t.Errorf("soundPlayed fehlt: %+v", result)
	}
	if script := decodePowerShell(t, (*calls)[1].args); !strings.Contains(script, "SoundPlayer") {
		t.Errorf("mit Sound fehlt der SoundPlayer:\n%s", script)
	}
}

// Scheitert PowerShell, kommt dessen stderr als Fehler zurück — kein
// no_channel, der Kanal war ja da.
func TestSendWSLProgrammScheitert(t *testing.T) {
	install(t, fakeHost{programs: map[string]bool{"powershell.exe": true}, wsl: true,
		fail: map[string]string{"powershell.exe": "Toast kaputt"}})

	_, err := Send(context.Background(), Request{Title: "Fertig"})
	if err == nil || !strings.Contains(err.Error(), "Toast kaputt") {
		t.Fatalf("Fehler = %v, erwartet die stderr des Programms", err)
	}
	if errors.Is(err, ErrNoChannel) {
		t.Error("ein scheiterndes Programm ist kein fehlender Kanal")
	}
}

// macOS: osascript mit display notification, Anführungszeichen escaped, der
// Ton als sound name.
func TestSendMacOS(t *testing.T) {
	calls := install(t, fakeHost{programs: map[string]bool{"osascript": true}, os: "darwin"})

	result, err := Send(context.Background(), Request{Title: `Sag "hallo"`, Body: "fertig", Sound: true})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if result.Channel != ChannelMacOS || !result.SoundPlayed {
		t.Fatalf("Ergebnis: %+v", result)
	}
	args := (*calls)[0].args
	want := `display notification "fertig" with title "Sag \"hallo\"" sound name "Glass"`
	if (*calls)[0].name != "osascript" || len(args) != 2 || args[1] != want {
		t.Errorf("Aufruf: %+v\nerwartet: %s", (*calls)[0], want)
	}
}

// Linux-Desktop: notify-send für das Popup; der Ton über paplay, und ohne
// Abspielprogramm geht die Meldung trotzdem — mit Grund.
func TestSendLinuxDesktop(t *testing.T) {
	calls := install(t, fakeHost{
		programs: map[string]bool{"notify-send": true, "paplay": true},
		env:      map[string]string{"DISPLAY": ":0"},
	})

	result, err := Send(context.Background(), Request{Title: "Fertig", Body: "Lauf 3", Sound: true})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if result.Channel != ChannelLinuxDesktop || !result.SoundPlayed || result.Reason != "" {
		t.Fatalf("Ergebnis: %+v", result)
	}
	if len(*calls) != 2 || (*calls)[0].name != "notify-send" || (*calls)[1].name != "paplay" {
		t.Fatalf("Aufrufe: %+v", *calls)
	}
	if args := (*calls)[0].args; args[len(args)-2] != "Fertig" || args[len(args)-1] != "Lauf 3" {
		t.Errorf("notify-send bekommt Titel und Inhalt nicht als letzte Argumente: %v", args)
	}

	install(t, fakeHost{
		programs: map[string]bool{"notify-send": true},
		env:      map[string]string{"WAYLAND_DISPLAY": "wayland-0"},
	})
	result, err = Send(context.Background(), Request{Title: "Fertig", Sound: true})
	if err != nil {
		t.Fatalf("Send ohne Abspielprogramm: %v", err)
	}
	if result.Channel != ChannelLinuxDesktop || result.SoundPlayed || !strings.Contains(result.Reason, "paplay") {
		t.Errorf("Ergebnis: %+v", result)
	}
}

// Im Container gibt es keinen Kanal, und der Grund nennt den Marker.
func TestSendImContainer(t *testing.T) {
	install(t, fakeHost{container: "DEVCONTAINER"})

	_, err := Send(context.Background(), Request{Title: "Fertig"})
	if !errors.Is(err, ErrNoChannel) {
		t.Fatalf("Fehler = %v, erwartet ErrNoChannel", err)
	}
	if !strings.Contains(err.Error(), "Container erkannt (DEVCONTAINER)") {
		t.Errorf("der Grund nennt den Marker nicht: %v", err)
	}
}

// Linux ohne Display: kein Kanal, der Grund nennt das Display. Mit Display,
// aber ohne notify-send: der Grund nennt das Programm. WSL ohne
// powershell.exe: der Grund nennt WSL.
func TestSendOhneKanalGruende(t *testing.T) {
	for _, tc := range []struct {
		name string
		host fakeHost
		want string
	}{
		{"ohne Display", fakeHost{}, "kein Display"},
		{"ohne notify-send", fakeHost{env: map[string]string{"DISPLAY": ":0"}}, "notify-send nicht installiert"},
		{"WSL ohne powershell", fakeHost{wsl: true}, "powershell.exe"},
		{"macOS ohne osascript", fakeHost{os: "darwin"}, "osascript"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			install(t, tc.host)
			_, err := Send(context.Background(), Request{Title: "Fertig"})
			if !errors.Is(err, ErrNoChannel) {
				t.Fatalf("Fehler = %v, erwartet ErrNoChannel", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Grund %q ohne %q", err, tc.want)
			}
		})
	}
}

// Ohne Titel wird nichts aufgerufen.
func TestSendOhneTitel(t *testing.T) {
	calls := install(t, fakeHost{programs: map[string]bool{"powershell.exe": true}, wsl: true})

	_, err := Send(context.Background(), Request{Title: "  ", Body: "x"})
	if !errors.Is(err, ErrEmptyTitle) {
		t.Fatalf("Fehler = %v, erwartet ErrEmptyTitle", err)
	}
	if len(*calls) != 0 {
		t.Errorf("trotz fehlendem Titel wurde aufgerufen: %+v", *calls)
	}
}

// Ein langer Inhalt wird gekürzt, und die Grenze zählt Zeichen, nicht Bytes.
func TestSendKuerztInhalt(t *testing.T) {
	calls := install(t, fakeHost{programs: map[string]bool{"osascript": true}, os: "darwin"})

	long := strings.Repeat("ä", bodyLimit+50)
	if _, err := Send(context.Background(), Request{Title: "Fertig", Body: long}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	script := (*calls)[0].args[1]
	if strings.Contains(script, long) {
		t.Error("der Inhalt wurde nicht gekürzt")
	}
	if !strings.Contains(script, strings.Repeat("ä", bodyLimit-1)+"…") {
		t.Errorf("die Kürzung endet nicht mit einer Ellipse nach %d Zeichen: %s", bodyLimit-1, script)
	}
}
