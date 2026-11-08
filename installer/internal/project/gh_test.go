package project

import (
	"encoding/json"
	"strings"
	"testing"
)

// Ein altes K-PLAYBOOK.yaml mit tools.gh bricht nichts (Task 128): Der Block
// wird nicht mehr gelesen, auch nicht mit einem unbekannten Wert, und die
// Kontextausgabe trägt nur noch den Befund dieses Rechners.
func TestBuildContextIgnoriertAltenGHBlock(t *testing.T) {
	for _, value := range []string{"disabled", "enabled", "unknown", "vielleicht"} {
		t.Run(value, func(t *testing.T) {
			root := newContextProject(t)
			write(t, ConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n\ntools:\n  gh:\n    status: "+value+"\n")

			built, err := BuildContext(root)
			if err != nil {
				t.Fatalf("BuildContext mit tools.gh.status: %s: %v", value, err)
			}
			if built.GH.Host != GHHost {
				t.Errorf("context.gh.host = %q, erwartet %q", built.GH.Host, GHHost)
			}
			data, err := json.Marshal(built.GH)
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{`"status"`, `"configured"`} {
				if strings.Contains(string(data), field) {
					t.Errorf("context.gh trägt noch %s: %s", field, data)
				}
			}
		})
	}
}

func TestDetectGHOhneAnmeldung(t *testing.T) {
	// Ein leeres Konfigurationsverzeichnis: gh mag installiert sein, angemeldet
	// ist hier niemand.
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	state := DetectGH()
	if state.LoggedIn {
		t.Errorf("LoggedIn = true ohne hosts.yml")
	}
	if state.Ready {
		t.Errorf("Ready = true ohne Anmeldung")
	}
	if state.Host != GHHost {
		t.Errorf("Host = %q, erwartet %q", state.Host, GHHost)
	}
}

// Ein Token in der Umgebung sticht die Konfigurationsdatei; ohne diesen Fall
// meldete die Oberfläche „nicht angemeldet", während gh läuft.
func TestDetectGHMitTokenAusDerUmgebung(t *testing.T) {
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_TOKEN", "gho_geheim")

	state := DetectGH()
	if !state.LoggedIn {
		t.Errorf("LoggedIn = false trotz GH_TOKEN")
	}
	if !state.TokenFromEnv {
		t.Errorf("TokenFromEnv = false trotz GH_TOKEN")
	}
	if state.Account != "" {
		t.Errorf("Account = %q, erwartet leer: die Umgebung kennt keinen Namen", state.Account)
	}
}
