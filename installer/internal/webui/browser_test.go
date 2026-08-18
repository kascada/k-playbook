package webui

import (
	"slices"
	"testing"
)

// TestEnvOpeners prüft das Zerlegen von $BROWSER nach der freedesktop-Konvention.
func TestEnvOpeners(t *testing.T) {
	tests := []struct {
		name    string
		browser string
		want    []browserOpener
	}{
		{
			name:    "leer",
			browser: "",
		},
		{
			name:    "nur Leerzeichen",
			browser: "  : ",
		},
		{
			name:    "einzelner Helfer",
			browser: "/vscode/bin/helpers/browser.sh",
			want:    []browserOpener{{command: "/vscode/bin/helpers/browser.sh"}},
		},
		{
			name:    "mehrere Einträge",
			browser: "firefox:chromium",
			want: []browserOpener{
				{command: "firefox"},
				{command: "chromium"},
			},
		},
		{
			name:    "Argumente und Platzhalter",
			browser: "firefox --new-tab %s",
			want: []browserOpener{
				{command: "firefox", args: []string{"--new-tab", "%s"}, placeholder: true},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("BROWSER", test.browser)

			got := envOpeners()
			if len(got) != len(test.want) {
				t.Fatalf("%d Kandidaten, erwartet %d: %+v", len(got), len(test.want), got)
			}
			for i, want := range test.want {
				if got[i].command != want.command || got[i].placeholder != want.placeholder ||
					!slices.Equal(got[i].args, want.args) {
					t.Errorf("Kandidat %d ist %+v, erwartet %+v", i, got[i], want)
				}
			}
		})
	}
}

// TestCommandLine prüft, dass die URL angehängt wird, solange kein Platzhalter
// da ist, und sonst an dessen Stelle tritt.
func TestCommandLine(t *testing.T) {
	const url = "http://127.0.0.1:8080/"

	tests := []struct {
		name   string
		opener browserOpener
		want   []string
	}{
		{
			name:   "ohne Argumente",
			opener: browserOpener{command: "xdg-open"},
			want:   []string{url},
		},
		{
			name:   "Argumente vor der URL",
			opener: browserOpener{command: "gio", args: []string{"open"}},
			want:   []string{"open", url},
		},
		{
			name:   "Platzhalter",
			opener: browserOpener{command: "firefox", args: []string{"--new-tab", "%s"}, placeholder: true},
			want:   []string{"--new-tab", url},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.opener.commandLine(url); !slices.Equal(got, test.want) {
				t.Errorf("%v, erwartet %v", got, test.want)
			}
		})
	}
}

// TestBrowserOpenersEnvFirst hält fest, dass $BROWSER vor den geratenen
// Kandidaten steht.
func TestBrowserOpenersEnvFirst(t *testing.T) {
	t.Setenv("BROWSER", "meinbrowser")

	openers := browserOpeners()
	if len(openers) < 2 {
		t.Fatalf("nur %d Kandidaten", len(openers))
	}
	if openers[0].command != "meinbrowser" {
		t.Errorf("erster Kandidat ist %q, erwartet %q", openers[0].command, "meinbrowser")
	}
}
