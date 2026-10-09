package vscodeext

import (
	"os"
	"path/filepath"
	"testing"
)

func schreibe(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
}

func manifest(entries ...string) string {
	out := "["
	for i, entry := range entries {
		if i > 0 {
			out += ","
		}
		out += entry
	}
	return out + "]"
}

func eintrag(id string, version string) string {
	return `{"identifier":{"id":"` + id + `"},"version":"` + version + `"}`
}

func TestFindOhneAlles(t *testing.T) {
	home := t.TempDir()
	if found := Find(home); len(found) != 0 {
		t.Errorf("Find = %+v, erwartet nichts", found)
	}
}

// extensions.json ist maßgeblich, nicht der Verzeichnisname: nach einem
// --uninstall-extension bleibt das Verzeichnis liegen, der Eintrag ist weg.
// Ein Rückfall auf Verzeichnisnamen würde die Erweiterung als vorhanden
// melden und den Nachzug ausfallen lassen.
func TestFindNurAusExtensionsJSON(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".vscode-server", "extensions")
	if err := os.MkdirAll(filepath.Join(dir, ID+"-0.1.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	schreibe(t, filepath.Join(dir, "extensions.json"), manifest(eintrag("other.ext", "9.9.9")))

	if found := Find(home); len(found) != 0 {
		t.Errorf("Verzeichnis ohne Eintrag zählt mit: %+v", found)
	}

	schreibe(t, filepath.Join(dir, "extensions.json"),
		manifest(eintrag("other.ext", "9.9.9"), eintrag("Kascada.K-Playbook-Workspace-Tools", "0.1.0")))
	found := Find(home)
	if len(found) != 1 || found[0].Version != "0.1.0" || found[0].Dir != dir {
		t.Fatalf("Find = %+v", found)
	}
}

// Eine unlesbare oder fehlende extensions.json heißt: nicht installiert.
func TestFindBeiKaputterDatei(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".vscode", "extensions")
	if err := os.MkdirAll(filepath.Join(dir, ID+"-0.1.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	schreibe(t, filepath.Join(dir, "extensions.json"), "{")

	if found := Find(home); len(found) != 0 {
		t.Errorf("Find = %+v, erwartet nichts", found)
	}
}

// Server- und lokales Verzeichnis werden beide gelesen, der Server zuerst.
func TestFindBeideVerzeichnisse(t *testing.T) {
	home := t.TempDir()
	server := filepath.Join(home, ".vscode-server", "extensions")
	lokal := filepath.Join(home, ".vscode", "extensions")
	schreibe(t, filepath.Join(server, "extensions.json"), manifest(eintrag(ID, "0.1.0")))
	schreibe(t, filepath.Join(lokal, "extensions.json"), manifest(eintrag(ID, "0.0.9")))

	found := Find(home)
	if len(found) != 2 {
		t.Fatalf("Find = %+v", found)
	}
	if found[0].Dir != server || found[0].Version != "0.1.0" {
		t.Errorf("erster Eintrag = %+v", found[0])
	}
	if found[1].Dir != lokal || found[1].Version != "0.0.9" {
		t.Errorf("zweiter Eintrag = %+v", found[1])
	}
}

func TestNeedsInstall(t *testing.T) {
	fälle := []struct {
		name     string
		embedded string
		found    []Installation
		want     bool
	}{
		{"ohne eingebettete Fassung nichts tun", "", nil, false},
		{"nicht installiert", "0.1.0", nil, true},
		{"gleiche Fassung", "0.1.0", []Installation{{Version: "0.1.0"}}, false},
		{"ältere Fassung", "0.2.0", []Installation{{Version: "0.1.0"}}, true},
		{"neuere Fassung zählt auch", "0.1.0", []Installation{{Version: "0.2.0"}}, true},
		{"eine von zwei weicht ab", "0.1.0", []Installation{{Version: "0.1.0"}, {Version: "0.0.9"}}, true},
	}
	for _, fall := range fälle {
		if got := NeedsInstall(fall.embedded, fall.found); got != fall.want {
			t.Errorf("%s: NeedsInstall = %v, erwartet %v", fall.name, got, fall.want)
		}
	}
}

// NeedsInstallIn fragt nach genau einem Verzeichnis — dem, das der Nachzug
// beschreibt. Der letzte Fall ist der, an dem „irgendwo" scheitert: dort
// bliebe die Antwort nach jedem erfolgreichen Nachzug wahr.
func TestNeedsInstallIn(t *testing.T) {
	server := filepath.Join("home", ".vscode-server", "extensions")
	lokal := filepath.Join("home", ".vscode", "extensions")
	fälle := []struct {
		name     string
		embedded string
		dir      string
		found    []Installation
		want     bool
	}{
		{"ohne eingebettete Fassung nichts tun", "", server, nil, false},
		{"nicht installiert", "0.1.0", server, nil, true},
		{"passende Fassung im Ziel", "0.1.0", server,
			[]Installation{{Dir: server, Version: "0.1.0"}}, false},
		{"abweichende Fassung im Ziel", "0.1.0", server,
			[]Installation{{Dir: server, Version: "0.0.9"}}, true},
		{"nur daneben installiert", "0.1.0", server,
			[]Installation{{Dir: lokal, Version: "0.1.0"}}, true},
		{"Ziel passt, daneben weicht ab", "0.1.0", server,
			[]Installation{{Dir: server, Version: "0.1.0"}, {Dir: lokal, Version: "0.0.9"}}, false},
	}
	for _, fall := range fälle {
		if got := NeedsInstallIn(fall.embedded, fall.dir, fall.found); got != fall.want {
			t.Errorf("%s: NeedsInstallIn = %v, erwartet %v", fall.name, got, fall.want)
		}
	}
}

// Divergent nennt, was neben dem Ziel in anderer Fassung liegt — und nur das.
func TestDivergent(t *testing.T) {
	server := filepath.Join("home", ".vscode-server", "extensions")
	lokal := filepath.Join("home", ".vscode", "extensions")
	found := []Installation{{Dir: server, Version: "0.1.0"}, {Dir: lokal, Version: "0.0.9"}}

	divergent := Divergent("0.1.0", server, found)
	if len(divergent) != 1 || divergent[0].Dir != lokal {
		t.Fatalf("Divergent = %+v", divergent)
	}
	if got := Divergent("0.1.0", lokal, []Installation{{Dir: lokal, Version: "0.0.9"}}); len(got) != 0 {
		t.Errorf("das Ziel selbst zählt nicht als daneben: %+v", got)
	}
	if got := Divergent("0.0.9", server, found); len(got) != 0 {
		t.Errorf("die passende Fassung daneben ist keine Abweichung: %+v", got)
	}
}

// Wohin ein Nachzug schreibt, hängt allein an der Art der CLI.
func TestExtensionsDirFolgtDerArt(t *testing.T) {
	home := filepath.Join("home", "nutzer")
	server := filepath.Join(home, ".vscode-server", "extensions")
	lokal := filepath.Join(home, ".vscode", "extensions")

	for _, kind := range []Kind{KindRemote, KindServer} {
		if got := (CLI{Kind: kind}).ExtensionsDir(home); got != server {
			t.Errorf("%s schreibt nach %q, erwartet %q", kind, got, server)
		}
	}
	if got := (CLI{Kind: KindDesktop}).ExtensionsDir(home); got != lokal {
		t.Errorf("%s schreibt nach %q, erwartet %q", KindDesktop, got, lokal)
	}
}
