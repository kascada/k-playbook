package project

import (
	"os"
	"slices"
	"strings"
	"testing"
)

const configWithoutLanguages = `schema_version: 3

project:
  # Ort des Projekt-Repositorys, relativ zu dieser Datei.
  repo_root: .
  vcs: git

tools:
  gh:
    status: enabled
`

func TestReadLanguagesOhneEintragLiefertVorauswahl(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	languages, configured, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if configured {
		t.Error("fehlender Schlüssel wurde als konfiguriert gemeldet")
	}
	if !slices.Equal(languages, DefaultLanguages) {
		t.Errorf("Languages = %v, erwartet %v", languages, DefaultLanguages)
	}
}

func TestReadLanguagesBlockform(t *testing.T) {
	root := writeConfig(t, `schema_version: 3

project:
  repo_root: .
  languages:
    - python
    - go
  vcs: git

tools:
  gh:
    status: enabled
`)

	languages, configured, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !configured {
		t.Error("vorhandener Schlüssel wurde nicht als konfiguriert gemeldet")
	}
	if !slices.Equal(languages, []string{"python", "go"}) {
		t.Errorf("Languages = %v, erwartet [python go]", languages)
	}
}

// Von Hand geschriebene Konfigurationen dürfen die Flussform nutzen, auch wenn
// das Werkzeug selbst die Blockform schreibt.
func TestReadLanguagesFlussform(t *testing.T) {
	root := writeConfig(t, "schema_version: 3\n\nproject:\n  languages: [python, go]\n  vcs: git\n")

	languages, _, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !slices.Equal(languages, []string{"python", "go"}) {
		t.Errorf("Languages = %v, erwartet [python go]", languages)
	}
}

func TestReadLanguagesLeereListe(t *testing.T) {
	root := writeConfig(t, "schema_version: 3\n\nproject:\n  languages: []\n  vcs: git\n")

	languages, configured, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !configured {
		t.Error("leere Liste wurde nicht als konfiguriert gemeldet")
	}
	if len(languages) != 0 {
		t.Errorf("Languages = %v, erwartet leer", languages)
	}
}

// Der Wert wandert als Kommandozeilenargument in das Preflight-Skript. Was dort
// eine eigene Bedeutung hätte, darf gar nicht erst gelesen werden.
func TestReadLanguagesMeldetUnzulaessigenWert(t *testing.T) {
	root := writeConfig(t, "schema_version: 3\n\nproject:\n  languages:\n    - \"go; rm -rf /\"\n")

	if _, _, err := ReadLanguages(root); err == nil {
		t.Error("unzulässiger Sprachname wurde nicht gemeldet")
	}
}

func TestSetLanguagesLaesstDenRestStehen(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	if err := SetLanguages(root, []string{"python", "go"}); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}

	data, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("Konfiguration lesen: %v", err)
	}
	content := string(data)
	for _, want := range []string{"repo_root: .", "vcs: git", "status: enabled", "- python", "- go"} {
		if !strings.Contains(content, want) {
			t.Errorf("%q fehlt in der geschriebenen Konfiguration:\n%s", want, content)
		}
	}

	languages, configured, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !configured || !slices.Equal(languages, []string{"python", "go"}) {
		t.Errorf("Languages = %v (configured %v), erwartet [python go]", languages, configured)
	}
}

// Zweimal Schreiben darf den Block ersetzen und nicht ein zweites Mal anhängen.
func TestSetLanguagesErsetztStattAnzuhaengen(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	if err := SetLanguages(root, []string{"python", "go"}); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}
	if err := SetLanguages(root, []string{"go"}); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}

	data, _ := os.ReadFile(ConfigPath(root))
	if count := strings.Count(string(data), "languages:"); count != 1 {
		t.Errorf("%d languages-Blöcke, erwartet 1:\n%s", count, data)
	}

	languages, _, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !slices.Equal(languages, []string{"go"}) {
		t.Errorf("Languages = %v, erwartet [go]", languages)
	}
}

// Wiederholtes Umschalten darf die Datei nicht wachsen lassen: weder durch
// gestapelte Kommentare noch durch das Aufzehren der Leerzeile vor dem nächsten
// Block. Beides ist beim Bauen der Sprachauswahl aufgefallen.
func TestSetLanguagesBleibtStabilBeiWiederholtemSchreiben(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	if err := SetLanguages(root, []string{"python"}); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}
	first, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("Konfiguration lesen: %v", err)
	}

	for range 3 {
		if err := SetLanguages(root, []string{"python"}); err != nil {
			t.Fatalf("SetLanguages: %v", err)
		}
	}
	again, err := os.ReadFile(ConfigPath(root))
	if err != nil {
		t.Fatalf("Konfiguration lesen: %v", err)
	}

	if string(first) != string(again) {
		t.Errorf("Datei hat sich beim erneuten Schreiben verändert:\n--- erst ---\n%s\n--- dann ---\n%s", first, again)
	}
	if !strings.Contains(string(again), "\n\ntools:") {
		t.Errorf("Leerzeile vor dem nächsten Block ging verloren:\n%s", again)
	}
}

// Die leere Auswahl wird als Flussform geschrieben. Sie muss beim nächsten Mal
// wiedergefunden werden, sonst entstünde ein zweiter languages-Schlüssel.
func TestSetLanguagesErsetztAuchDieLeereListe(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	if err := SetLanguages(root, nil); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}
	if err := SetLanguages(root, []string{"go"}); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}

	data, _ := os.ReadFile(ConfigPath(root))
	if count := strings.Count(string(data), "languages:"); count != 1 {
		t.Errorf("%d languages-Schlüssel, erwartet 1:\n%s", count, data)
	}

	languages, _, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !slices.Equal(languages, []string{"go"}) {
		t.Errorf("Languages = %v, erwartet [go]", languages)
	}
}

func TestSetLanguagesWeistUnzulaessigenWertAb(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	if err := SetLanguages(root, []string{"go; rm -rf /"}); err == nil {
		t.Error("unzulässiger Sprachname wurde geschrieben")
	}
}

func TestSetLanguagesEntferntDoppelte(t *testing.T) {
	root := writeConfig(t, configWithoutLanguages)

	if err := SetLanguages(root, []string{"python", "Python", " python "}); err != nil {
		t.Fatalf("SetLanguages: %v", err)
	}

	languages, _, err := ReadLanguages(root)
	if err != nil {
		t.Fatalf("ReadLanguages: %v", err)
	}
	if !slices.Equal(languages, []string{"python"}) {
		t.Errorf("Languages = %v, erwartet [python]", languages)
	}
}
