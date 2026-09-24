package program

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// isolate gibt dem Test ein eigenes HOME und einen PATH, der mit dessen
// ~/.local/bin beginnt. Das echte ~/.local/bin/k-playbook bleibt so
// unerreichbar — auch für den Stub-Bootstrap, der über $HOME schreibt.
func isolate(t *testing.T) (home string, target string) {
	t.Helper()

	home = t.TempDir()
	bin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", bin, err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	return home, filepath.Join(bin, project.InstalledCommandName)
}

func writeExecutable(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
}

// programScript ist ein Programm, das auf `version` mit version antwortet.
// Ohne Version kennt es das Subkommando nicht — wie ein Programm vor 076.
func programScript(version string) string {
	if version == "" {
		return "#!/bin/sh\necho 'unbekanntes Kommando' >&2\nexit 1\n"
	}
	return fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = version ]; then echo %s; exit 0; fi\nexit 3\n", version)
}

func sum(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// stubClone baut ein Projekt mit Clone: VERSION, SHA256SUMS mit der Summe von
// asset und ein Stub-Bootstrap, der asset ans Ziel kopiert, einen Marker
// schreibt und mit exitCode endet. Gibt Hauptverzeichnis und Marker zurück.
func stubClone(t *testing.T, version string, asset string, sums string, exitCode int) (string, string) {
	t.Helper()

	projectDir := t.TempDir()
	playbook := filepath.Join(projectDir, project.PlaybookDirName)
	source := filepath.Join(t.TempDir(), "asset")
	writeExecutable(t, source, asset)
	if err := os.MkdirAll(playbook, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(playbook, project.VersionFileName), []byte(version+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if sums == "" {
		sums = sum(asset)
	}
	if err := os.WriteFile(filepath.Join(playbook, SumsFileName), []byte(sums+"  "+AssetName()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "bootstrap-gelaufen")
	writeExecutable(t, filepath.Join(playbook, "bin", "install"), fmt.Sprintf(
		"#!/bin/sh\necho \"Lade Stub aus $(pwd)\"\n: > %q\ncp %q \"$HOME/.local/bin/k-playbook\"\nexit %d\n",
		marker, source, exitCode))
	return projectDir, marker
}

func ran(marker string) bool {
	_, err := os.Stat(marker)
	return err == nil
}

// Erfolg: das Programm am Ziel ist älter, der Bootstrap läuft aus dem
// Hauptverzeichnis, und die Datei danach passt zu SHA256SUMS.
func TestInstallBootstrapErfolg(t *testing.T) {
	_, target := isolate(t)
	writeExecutable(t, target, programScript("v0.9.3"))
	asset := programScript("v0.9.4")
	projectDir, marker := stubClone(t, "v0.9.4", asset, "", 0)

	result, err := Install(projectDir)
	if err != nil {
		t.Fatalf("Install: %v\n%s", err, result.Output)
	}
	if !ran(marker) || !result.Bootstrapped {
		t.Fatalf("der Bootstrap ist nicht gelaufen: %+v", result)
	}
	if result.Expected != "v0.9.4" || result.Target != target || result.ExitCode != 0 {
		t.Errorf("Ergebnis = %+v", result)
	}
	// Aus dem Hauptverzeichnis aufgerufen, wie die Doku es sagt.
	resolved, _ := filepath.EvalSymlinks(projectDir)
	if !strings.Contains(result.Output, projectDir) && !strings.Contains(result.Output, resolved) {
		t.Errorf("Bootstrap lief nicht im Hauptverzeichnis: %q", result.Output)
	}
	if got := ReadVersion(target); got != "v0.9.4" {
		t.Errorf("Version am Ziel = %q", got)
	}
}

// Exit ≠ 0: Fehler mit Ausgabe und Exit-Code.
func TestInstallBootstrapScheitert(t *testing.T) {
	_, target := isolate(t)
	writeExecutable(t, target, programScript("v0.9.3"))
	projectDir, _ := stubClone(t, "v0.9.4", programScript("v0.9.4"), "", 7)

	result, err := Install(projectDir)
	if err == nil {
		t.Fatal("kein Fehler bei Exit 7")
	}
	if result.ExitCode != 7 || !strings.Contains(err.Error(), "Exit-Code 7") || !strings.Contains(result.Output, "Lade Stub") {
		t.Errorf("Ergebnis = %+v, Fehler %v", result, err)
	}
}

// Exit 0, aber die Datei passt nicht zu SHA256SUMS: geprüft wird die Datei,
// nicht der Exit-Code.
func TestInstallDateiPasstNichtZuSHA256SUMS(t *testing.T) {
	_, target := isolate(t)
	writeExecutable(t, target, programScript("v0.9.3"))
	projectDir, marker := stubClone(t, "v0.9.4", programScript("v0.9.4"), strings.Repeat("0", 64), 0)

	_, err := Install(projectDir)
	if err == nil || !strings.Contains(err.Error(), SumsFileName) {
		t.Fatalf("Fehler = %v, erwartet Abweichung gegen %s", err, SumsFileName)
	}
	if !ran(marker) {
		t.Error("der Bootstrap ist nicht gelaufen")
	}
}

// Die Datei am Ziel ist gleich alt oder neuer als VERSION: kein Download,
// erwartet wird die an der Datei gelesene Version. Herabgestuft wird nie.
func TestInstallOhneBootstrapBeiGleicherOderNeuererDatei(t *testing.T) {
	for _, have := range []string{"v0.9.4", "v0.10.0"} {
		t.Run(have, func(t *testing.T) {
			_, target := isolate(t)
			writeExecutable(t, target, programScript(have))
			projectDir, marker := stubClone(t, "v0.9.4", programScript("v0.9.4"), "", 0)

			result, err := Install(projectDir)
			if err != nil {
				t.Fatalf("Install: %v", err)
			}
			if ran(marker) || result.Bootstrapped {
				t.Error("der Bootstrap ist gelaufen")
			}
			if result.Expected != have {
				t.Errorf("erwartete Version = %q, gelesen war %q", result.Expected, have)
			}
		})
	}
}

// Ein Programm ohne Subkommando version — eines vor 076 — ist „unbekannt",
// und unbekannt heißt: Bootstrap.
func TestInstallBootstrapBeiDateiOhneVersion(t *testing.T) {
	_, target := isolate(t)
	writeExecutable(t, target, programScript(""))
	projectDir, marker := stubClone(t, "v0.9.4", programScript("v0.9.4"), "", 0)

	if _, err := Install(projectDir); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !ran(marker) {
		t.Error("der Bootstrap ist nicht gelaufen")
	}
}

// Zeigt der PATH auf ein anderes Programm, installiert Install nicht — auch
// dann nicht, wenn der Aufrufer die Prüfung vorher übergangen hätte.
func TestInstallNichtBeiFalschemPath(t *testing.T) {
	_, target := isolate(t)
	writeExecutable(t, target, programScript("v0.9.3"))
	other := filepath.Join(t.TempDir(), "anderswo")
	writeExecutable(t, filepath.Join(other, project.InstalledCommandName), programScript("v0.9.3"))
	t.Setenv("PATH", other+string(os.PathListSeparator)+os.Getenv("PATH"))
	projectDir, marker := stubClone(t, "v0.9.4", programScript("v0.9.4"), "", 0)

	if _, err := Install(projectDir); err == nil {
		t.Fatal("kein Fehler bei falschem PATH")
	}
	if ran(marker) {
		t.Error("der Bootstrap ist trotz falschem PATH gelaufen")
	}
}

func TestReadVersion(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{"liest die Version", programScript("v0.9.4"), "v0.9.4"},
		{"ohne Subkommando", programScript(""), ""},
		{"mehr als eine Zeile", "#!/bin/sh\necho v0.9.4\necho mehr\n", ""},
		{"keine Version", "#!/bin/sh\necho hallo\n", ""},
		{"Servermarke kommt nicht an", "#!/bin/sh\nif [ -n \"$K_PLAYBOOK_SERVE\" ]; then echo v9.9.9; else echo v0.9.4; fi\n", "v0.9.4"},
	}
	t.Setenv("K_PLAYBOOK_SERVE", "1")
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(dir, fmt.Sprintf("p%d", i))
			writeExecutable(t, path, test.script)
			if got := ReadVersion(path); got != test.want {
				t.Errorf("ReadVersion = %q, erwartet %q", got, test.want)
			}
		})
	}
	if got := ReadVersion(filepath.Join(dir, "fehlt")); got != "" {
		t.Errorf("fehlende Datei: %q", got)
	}
}

// PATH zeigt aufs Ziel → OK; auf ein anderes Programm oder auf nichts → kein
// Knopf, und der Hinweis nennt beide Pfade.
func TestCheckPath(t *testing.T) {
	t.Run("zeigt aufs Ziel", func(t *testing.T) {
		_, target := isolate(t)
		writeExecutable(t, target, programScript("v0.9.3"))
		check := CheckPath()
		if !check.OK || check.Found != target || check.Hint() != "" {
			t.Errorf("CheckPath = %+v, Hinweis %q", check, check.Hint())
		}
	})

	t.Run("Symlink im PATH aufs Ziel", func(t *testing.T) {
		_, target := isolate(t)
		writeExecutable(t, target, programScript("v0.9.3"))
		links := t.TempDir()
		if err := os.Symlink(target, filepath.Join(links, project.InstalledCommandName)); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", links+string(os.PathListSeparator)+os.Getenv("PATH"))
		if check := CheckPath(); !check.OK {
			t.Errorf("CheckPath = %+v", check)
		}
	})

	t.Run("anderes Programm vorn", func(t *testing.T) {
		_, target := isolate(t)
		writeExecutable(t, target, programScript("v0.9.3"))
		other := filepath.Join(t.TempDir(), project.InstalledCommandName)
		writeExecutable(t, other, programScript("v0.9.3"))
		t.Setenv("PATH", filepath.Dir(other)+string(os.PathListSeparator)+os.Getenv("PATH"))
		check := CheckPath()
		if check.OK || check.Found != other {
			t.Errorf("CheckPath = %+v", check)
		}
		for _, want := range []string{other, target, project.BootstrapCommand} {
			if !strings.Contains(check.Hint(), want) {
				t.Errorf("Hinweis nennt %q nicht: %s", want, check.Hint())
			}
		}
	})

	t.Run("nichts im PATH", func(t *testing.T) {
		_, target := isolate(t)
		check := CheckPath()
		if check.OK || check.Found != "" {
			t.Errorf("CheckPath = %+v", check)
		}
		for _, want := range []string{target, "nicht zu finden", filepath.Dir(target)} {
			if !strings.Contains(check.Hint(), want) {
				t.Errorf("Hinweis nennt %q nicht: %s", want, check.Hint())
			}
		}
	})
}
