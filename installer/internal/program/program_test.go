package program

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/release/releasetest"
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

func ran(marker string) bool {
	_, err := os.Stat(marker)
	return err == nil
}

// TestMain: Kein Test dieses Pakets fragt das echte GitHub nach dem neuesten
// Release-Tag oder lädt ein Release-Binary. Wer die Quelle braucht, stellt mit
// releasetest.Serve eine Ersatzquelle hin.
func TestMain(m *testing.M) {
	cleanup := releasetest.Isolate()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// Erfolg: das Programm am Ziel ist älter, das Release-Binary des Tags wird
// geladen, gegen die SHA256SUMS des Tags geprüft und ersetzt die Datei. Ein
// Clone wird dafür weder gelesen noch gebraucht.
func TestInstallLaedtDasReleaseDesTags(t *testing.T) {
	_, target := isolate(t)
	writeExecutable(t, target, programScript("v0.9.3"))
	source := releasetest.Serve(t)
	source.AddRelease("v0.9.4", []byte(programScript("v0.9.4")))

	result, err := Install(context.Background(), "v0.9.4")
	if err != nil {
		t.Fatalf("Install: %v\n%s", err, result.Output)
	}
	if !result.Installed || result.Expected != "v0.9.4" || result.Target != target {
		t.Errorf("Ergebnis = %+v", result)
	}
	for _, want := range []string{source.URL() + "/download/v0.9.4/" + AssetName(), "Geprüft gegen SHA256SUMS von v0.9.4"} {
		if !strings.Contains(result.Output, want) {
			t.Errorf("Ausgabe nennt %q nicht: %q", want, result.Output)
		}
	}
	if got := ReadVersion(target); got != "v0.9.4" {
		t.Errorf("Version am Ziel = %q", got)
	}
	if err := Verify(context.Background(), "v0.9.4", target); err != nil {
		t.Errorf("Verify gegen die Summen des Tags: %v", err)
	}
	// Eine Datei, die nicht das Release-Binary ist, besteht Verify nicht.
	writeExecutable(t, target, programScript("v0.9.3"))
	if err := Verify(context.Background(), "v0.9.4", target); err == nil || !strings.Contains(err.Error(), "SHA256SUMS") {
		t.Errorf("Verify an fremder Datei: %v", err)
	}
}

// Die Installation scheitert, ohne etwas zu ersetzen: das Asset fehlt noch
// (der Tag steht, die CI baut), die Prüfsumme weicht ab, oder die Quelle ist
// nicht erreichbar.
func TestInstallScheitertOhneZuErsetzen(t *testing.T) {
	old := programScript("v0.9.3")

	t.Run("Asset fehlt", func(t *testing.T) {
		_, target := isolate(t)
		writeExecutable(t, target, old)
		source := releasetest.Serve(t)
		source.AddRelease("v0.9.4", []byte(programScript("v0.9.4")))
		source.RemoveAsset("v0.9.4", AssetName())

		result, err := Install(context.Background(), "v0.9.4")
		if err == nil || !strings.Contains(err.Error(), "noch nicht fertig") || result.Installed {
			t.Fatalf("Ergebnis = %+v, Fehler %v", result, err)
		}
		if content, _ := os.ReadFile(target); string(content) != old {
			t.Error("die Datei am Ziel wurde ersetzt")
		}
	})

	t.Run("Prüfsumme weicht ab", func(t *testing.T) {
		_, target := isolate(t)
		writeExecutable(t, target, old)
		source := releasetest.Serve(t)
		source.AddAsset("v0.9.4", AssetName(), []byte(programScript("v0.9.4")))
		source.SetSums("v0.9.4", strings.Repeat("0", 64)+"  "+AssetName()+"\n")

		_, err := Install(context.Background(), "v0.9.4")
		if err == nil || !strings.Contains(err.Error(), "Prüfsumme weicht ab") {
			t.Fatalf("Fehler = %v", err)
		}
		if content, _ := os.ReadFile(target); string(content) != old {
			t.Error("die Datei am Ziel wurde ersetzt")
		}
	})

	t.Run("Quelle abgeschaltet", func(t *testing.T) {
		_, target := isolate(t)
		writeExecutable(t, target, old)
		releasetest.Off(t)

		if _, err := Install(context.Background(), "v0.9.4"); !errors.Is(err, release.ErrDisabled) {
			t.Fatalf("Fehler = %v", err)
		}
		if content, _ := os.ReadFile(target); string(content) != old {
			t.Error("die Datei am Ziel wurde ersetzt")
		}
	})

	t.Run("kein Tag", func(t *testing.T) {
		_, target := isolate(t)
		writeExecutable(t, target, old)
		for _, tag := range []string{"", "unbekannt", "v0.9.4-rc.1"} {
			if _, err := Install(context.Background(), tag); err == nil || !strings.Contains(err.Error(), "kein Release-Tag") {
				t.Errorf("Tag %q: %v", tag, err)
			}
		}
	})
}

// Die Datei am Ziel ist gleich alt oder neuer als der Tag: kein Download,
// erwartet wird die an der Datei gelesene Version. Herabgestuft wird nie.
func TestInstallOhneDownloadBeiGleicherOderNeuererDatei(t *testing.T) {
	for _, have := range []string{"v0.9.4", "v0.10.0"} {
		t.Run(have, func(t *testing.T) {
			_, target := isolate(t)
			writeExecutable(t, target, programScript(have))
			source := releasetest.Serve(t)
			source.AddRelease("v0.9.4", []byte(programScript("v0.9.4")))

			result, err := Install(context.Background(), "v0.9.4")
			if err != nil {
				t.Fatalf("Install: %v", err)
			}
			if result.Installed || len(source.Requests()) != 0 {
				t.Errorf("es wurde geladen: %+v, Anfragen %v", result, source.Requests())
			}
			if result.Expected != have {
				t.Errorf("erwartete Version = %q, gelesen war %q", result.Expected, have)
			}
		})
	}
}

// Ein Programm ohne Subkommando version — eines vor 076 — ist „unbekannt",
// und unbekannt heißt: installieren.
func TestInstallBeiDateiOhneVersion(t *testing.T) {
	_, target := isolate(t)
	writeExecutable(t, target, programScript(""))
	source := releasetest.Serve(t)
	source.AddRelease("v0.9.4", []byte(programScript("v0.9.4")))

	result, err := Install(context.Background(), "v0.9.4")
	if err != nil || !result.Installed {
		t.Fatalf("Install: %+v, %v", result, err)
	}
	if got := ReadVersion(target); got != "v0.9.4" {
		t.Errorf("Version am Ziel = %q", got)
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
	source := releasetest.Serve(t)
	source.AddRelease("v0.9.4", []byte(programScript("v0.9.4")))

	if _, err := Install(context.Background(), "v0.9.4"); err == nil {
		t.Fatal("kein Fehler bei falschem PATH")
	}
	if len(source.Requests()) != 0 {
		t.Errorf("trotz falschem PATH wurde gefragt: %v", source.Requests())
	}
}

// CheckStatus vergleicht laufend gegen den neuesten Tag: älter meldet, gleich
// und neuer nicht. Ohne eigene Version wird nichts abgerufen, und ein
// unbekannter Tag ist kein Nachweis.
func TestCheckStatus(t *testing.T) {
	t.Run("älter, gleich, neuer", func(t *testing.T) {
		_, target := isolate(t)
		writeExecutable(t, target, programScript("v0.9.3"))
		source := releasetest.Serve(t)
		source.AddTag("v0.9.4")
		source.AddTag("v0.9.5-rc.1")

		older := CheckStatus(context.Background(), "v0.9.3", Fresh)
		if !older.Outdated || older.Latest != "v0.9.4" || older.Order != "älter" || !older.Installable || older.Target != target || older.Hint != "" {
			t.Errorf("älter: %+v", older)
		}
		for running, order := range map[string]string{"v0.9.4": "gleich", "v0.10.0": "neuer"} {
			status := CheckStatus(context.Background(), running, Fresh)
			if status.Outdated || status.Latest != "v0.9.4" || status.Order != order || status.LatestError != "" {
				t.Errorf("%s: %+v", running, status)
			}
		}
	})

	t.Run("älter, aber der PATH zeigt woandershin", func(t *testing.T) {
		isolate(t)
		source := releasetest.Serve(t)
		source.AddTag("v0.9.4")

		status := CheckStatus(context.Background(), "v0.9.3", Fresh)
		if !status.Outdated || status.Installable || !strings.Contains(status.Hint, project.FirstInstallCommand) {
			t.Errorf("Status = %+v", status)
		}
	})

	t.Run("ohne Version kein Abruf", func(t *testing.T) {
		source := releasetest.Serve(t)
		source.AddTag("v0.9.4")
		for _, running := range []string{"", "dev"} {
			status := CheckStatus(context.Background(), running, Fresh)
			if status.Outdated || status.Latest != "" || status.LatestError != "" || status.Order != "unbekannt" {
				t.Errorf("laufend %q: %+v", running, status)
			}
		}
		if requests := source.Requests(); len(requests) != 0 {
			t.Errorf("ohne eigene Version wurde gefragt: %v", requests)
		}
	})

	t.Run("Tag unbekannt", func(t *testing.T) {
		releasetest.Off(t)
		status := CheckStatus(context.Background(), "v0.9.3", Fresh)
		if status.Outdated || status.Latest != "" || status.Order != "unbekannt" || !strings.Contains(status.LatestError, "abgeschaltet") {
			t.Errorf("abgeschaltet: %+v", status)
		}

		// Die Quelle antwortet, nennt aber keinen Release-Tag.
		source := releasetest.Serve(t)
		source.AddTag("v0.9.5-rc.1")
		status = CheckStatus(context.Background(), "v0.9.3", Fresh)
		if status.Outdated || status.Latest != "" || status.LatestError == "" {
			t.Errorf("ohne Release-Tag: %+v", status)
		}
	})
}

// Fresh fragt immer und schreibt den Cache; Cached nimmt einen frischen
// Eintrag ohne Anfrage — sichtbar an der Ersatzquelle.
func TestCheckStatusFrischOderAusDemCache(t *testing.T) {
	isolate(t)
	source := releasetest.Serve(t)
	source.AddTag("v0.9.4")

	if status := CheckStatus(context.Background(), "v0.9.4", Fresh); status.Latest != "v0.9.4" || len(source.Requests()) != 1 {
		t.Fatalf("Fresh: %+v, Anfragen %v", status, source.Requests())
	}
	if _, err := os.Stat(filepath.Join(release.CacheDir(), release.CacheFileName)); err != nil {
		t.Fatalf("Fresh hat den Cache nicht geschrieben: %v", err)
	}

	// Ein neues Release erscheint. Cached sieht noch den Eintrag, ohne zu
	// fragen; Fresh fragt und sieht es.
	source.AddTag("v0.9.5")
	if status := CheckStatus(context.Background(), "v0.9.4", Cached); status.Latest != "v0.9.4" || status.Outdated || len(source.Requests()) != 1 {
		t.Errorf("Cached: %+v, Anfragen %v", status, source.Requests())
	}
	if status := CheckStatus(context.Background(), "v0.9.4", Fresh); status.Latest != "v0.9.5" || !status.Outdated || len(source.Requests()) != 2 {
		t.Errorf("Fresh: %+v, Anfragen %v", status, source.Requests())
	}
	// Der neue Eintrag gilt jetzt auch für Cached.
	if status := CheckStatus(context.Background(), "v0.9.4", Cached); status.Latest != "v0.9.5" || len(source.Requests()) != 2 {
		t.Errorf("Cached nach Fresh: %+v, Anfragen %v", status, source.Requests())
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
		for _, want := range []string{other, target, project.FirstInstallCommand} {
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
		for _, want := range []string{target, "nicht zu finden", filepath.Dir(target), project.FirstInstallCommand} {
			if !strings.Contains(check.Hint(), want) {
				t.Errorf("Hinweis nennt %q nicht: %s", want, check.Hint())
			}
		}
	})
}

// Liefert die Quelle das Asset nie aus, endet der Download mit InstallTimeout:
// die Meldung nennt Adresse und Frist, der Aufruf kehrt mit der Frist zurück,
// und die Datei am Ziel bleibt, wie sie war.
func TestInstallFristAbgelaufen(t *testing.T) {
	_, target := isolate(t)
	old := programScript("v0.9.3")
	writeExecutable(t, target, old)
	source := releasetest.Serve(t)
	source.AddRelease("v0.9.4", []byte(programScript("v0.9.4")))
	source.StallDownloads()
	shortenInstallTimeout(t, 400*time.Millisecond)

	start := time.Now()
	result, err := Install(context.Background(), "v0.9.4")
	elapsed := time.Since(start)
	want := "der Download von " + source.URL() + "/download/v0.9.4/" + AssetName() + " hat nach 400ms nicht geendet"
	if err == nil || err.Error() != want {
		t.Fatalf("Fehler = %v, erwartet %q", err, want)
	}
	if result.Installed {
		t.Errorf("Ergebnis = %+v", result)
	}
	if elapsed > 400*time.Millisecond+time.Second {
		t.Errorf("Install kehrte erst nach %s zurück", elapsed)
	}
	if content, _ := os.ReadFile(target); string(content) != old {
		t.Error("die Datei am Ziel wurde ersetzt")
	}
	entries, _ := os.ReadDir(filepath.Dir(target))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".k-playbook.") {
			t.Errorf("die temporäre Datei blieb liegen: %s", entry.Name())
		}
	}
}
