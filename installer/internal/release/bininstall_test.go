package release_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/release/releasetest"
)

// bin/install — die Erstinstallation — bestimmt Tag und Prüfsumme wie dieses
// Paket und liest dieselbe Umgebungsvariable (release.EnvSource). Die Tests
// hier lassen das echte Skript aus dem Arbeitsstand gegen eine Ersatzquelle
// laufen, in eigenem HOME: Kein Test fragt das echte GitHub, lädt das echte
// install.sh von Kephalaion oder berührt ~/.local/bin dieses Rechners.

// installScript ist bin/install des Arbeitsstands.
func installScript(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "bin", "install"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("bin/install fehlt: %v", err)
	}
	for _, tool := range []string{"bash", "curl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s fehlt", tool)
		}
	}
	return path
}

// installRun ist ein Lauf von bin/install.
type installRun struct {
	home   string
	target string
	// bin ist ein Verzeichnis vorn im PATH; ein kephalaion darin lässt den
	// Kephalaion-Schritt des Skripts nichts laden.
	bin  string
	path string
	dir  string
}

// newInstallRun legt HOME, Zielverzeichnis und PATH an. Mit kephalaion liegt
// ein Ersatz-kephalaion im PATH.
func newInstallRun(t *testing.T, kephalaion bool) *installRun {
	t.Helper()
	home := t.TempDir()
	bin := filepath.Join(t.TempDir(), "werkzeug")
	for _, dir := range []string{filepath.Join(home, ".local", "bin"), bin} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if kephalaion {
		if err := os.WriteFile(filepath.Join(bin, "kephalaion"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	} else {
		// Der Lauf soll kephalaion nicht finden. Liegt auf diesem Rechner eines
		// in einem Systemverzeichnis, das im PATH des Laufs steht, hat der Test
		// keine Grundlage.
		for _, dir := range []string{"/usr/local/bin", "/usr/bin", "/bin"} {
			if _, err := os.Stat(filepath.Join(dir, "kephalaion")); err == nil {
				t.Skipf("kephalaion liegt global unter %s", dir)
			}
		}
	}
	return &installRun{
		home:   home,
		target: filepath.Join(home, ".local", "bin", "k-playbook"),
		bin:    bin,
		path:   strings.Join([]string{filepath.Join(home, ".local", "bin"), bin, "/usr/local/bin", "/usr/bin", "/bin"}, string(os.PathListSeparator)),
		dir:    t.TempDir(),
	}
}

// run startet bash mit args; stdin ist die Datei unter stdin, wenn gesetzt —
// der lokale Ersatz für curl | bash.
func (r *installRun) run(t *testing.T, stdin string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command("bash", args...)
	cmd.Dir = r.dir
	cmd.Env = []string{
		"HOME=" + r.home,
		"PATH=" + r.path,
		"TMPDIR=" + t.TempDir(),
		release.EnvSource + "=" + os.Getenv(release.EnvSource),
		"NO_PROXY=127.0.0.1,localhost",
		"no_proxy=127.0.0.1,localhost",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1",
	}
	if stdin != "" {
		file, err := os.Open(stdin)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		cmd.Stdin = file
	}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0, output.String()
	case errors.As(err, &exitErr):
		return exitErr.ExitCode(), output.String()
	}
	t.Fatalf("bash nicht ausführbar: %v", err)
	return -1, ""
}

func (r *installRun) installed(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(r.target)
	if err != nil {
		return ""
	}
	return string(content)
}

// releases stellt die Ersatzquelle hin: v0.10.0 ist das neueste Release (der
// höchste Tag der Form vX.Y.Z) und trägt das Asset dieser Plattform.
func releases(t *testing.T) *releasetest.Source {
	t.Helper()
	source := releasetest.Serve(t)
	source.AddRelease("v0.9.4", []byte("alt v0.9.4\n"))
	source.AddRelease("v0.10.0", []byte("neu v0.10.0\n"))
	source.AddTag("v0.11.0-rc.1")
	return source
}

// Über stdin — der lokale Ersatz für curl | bash — und aus einem Clone
// aufgerufen verhält sich bin/install gleich: Es installiert den neuesten Tag,
// geprüft gegen dessen SHA256SUMS, mit Exit-Code 0. VERSION und SHA256SUMS
// neben dem Skript im Clone liest es nicht; sie weichen hier vom neuesten Tag
// ab.
func TestBinInstallInstalliertDenNeuestenTag(t *testing.T) {
	script := installScript(t)

	t.Run("über stdin", func(t *testing.T) {
		releases(t)
		r := newInstallRun(t, true)
		if err := os.WriteFile(r.target, []byte("vorher\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		// Die Binaries des alten Wrappers gehen, der Cache des neuesten
		// Release daneben bleibt.
		cache := filepath.Join(r.home, ".cache", "k-playbook")
		wrapperBinary := filepath.Join(cache, "bin", "v0.1.2", "k-playbook-linux-amd64")
		latest := filepath.Join(cache, release.CacheFileName)
		for _, path := range []string{wrapperBinary, latest} {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("alt\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		code, output := r.run(t, script)
		if code != 0 {
			t.Fatalf("Exit-Code %d:\n%s", code, output)
		}
		if got := r.installed(t); got != "neu v0.10.0\n" {
			t.Errorf("am Ziel liegt %q:\n%s", got, output)
		}
		if _, err := os.Stat(filepath.Join(cache, "bin")); !os.IsNotExist(err) {
			t.Errorf("die Binaries des alten Wrappers blieben liegen: %v", err)
		}
		if content, err := os.ReadFile(latest); err != nil || string(content) != "alt\n" {
			t.Errorf("der Cache des neuesten Release wurde gelöscht: %q, %v", content, err)
		}
		for _, want := range []string{"Neuestes Release: v0.10.0", "Installiert: " + r.target + " (v0.10.0, sha256 ", "Kephalaion vorhanden: ", "Release-Quelle ersetzt über " + release.EnvSource} {
			if !strings.Contains(output, want) {
				t.Errorf("%q fehlt in:\n%s", want, output)
			}
		}
		if info, err := os.Stat(r.target); err != nil || info.Mode().Perm() != 0o755 {
			t.Errorf("Rechte am Ziel: %v, %v", info, err)
		}
		entries, _ := os.ReadDir(filepath.Dir(r.target))
		if len(entries) != 1 {
			t.Errorf("neben dem Ziel liegt noch etwas: %v", entries)
		}
	})

	t.Run("aus dem Clone", func(t *testing.T) {
		releases(t)
		r := newInstallRun(t, true)
		clone := filepath.Join(r.dir, "k-playbook")
		if err := os.MkdirAll(filepath.Join(clone, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(script)
		if err != nil {
			t.Fatal(err)
		}
		for path, file := range map[string]string{
			filepath.Join(clone, "bin", "install"): string(content),
			// Der Clone steht hinter dem neuesten Tag, und seine Summe passt zu
			// nichts: Gelesen wird beides nicht.
			filepath.Join(clone, "VERSION"):    "v0.9.4\n",
			filepath.Join(clone, "SHA256SUMS"): strings.Repeat("0", 64) + "  " + release.AssetName() + "\n",
		} {
			if err := os.WriteFile(path, []byte(file), 0o755); err != nil {
				t.Fatal(err)
			}
		}

		// Aus dem Hauptverzeichnis, ohne Argument — so rufen Binaries von vor
		// Task 149 es auf.
		code, output := r.run(t, "", "k-playbook/bin/install")
		if code != 0 {
			t.Fatalf("Exit-Code %d:\n%s", code, output)
		}
		if got := r.installed(t); got != "neu v0.10.0\n" {
			t.Errorf("am Ziel liegt %q, erwartet der neueste Tag statt VERSION des Clones:\n%s", got, output)
		}
	})
}

// Ohne git installiert bin/install: Das neueste Release kommt aus der
// Weiterleitung von releases/latest, mit curl oder — ohne curl — mit wget.
func TestBinInstallOhneGit(t *testing.T) {
	script := installScript(t)
	base := []string{"bash", "sh", "uname", "awk", "sed", "sort", "tail", "head", "od", "tr", "mktemp", "rm", "mv", "chmod", "mkdir", "cat", "sha256sum"}
	for _, fall := range []struct {
		name  string
		fetch string
	}{
		{"mit curl", "curl"},
		{"mit wget", "wget"},
	} {
		t.Run(fall.name, func(t *testing.T) {
			source := releases(t)
			r := newInstallRun(t, true)
			if err := os.WriteFile(r.target, []byte("vorher\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			// Ein PATH ohne git und mit genau einem Werkzeug zum Laden.
			for _, tool := range append(append([]string(nil), base...), fall.fetch) {
				found, err := exec.LookPath(tool)
				if err != nil {
					t.Skipf("%s fehlt", tool)
				}
				if err := os.Symlink(found, filepath.Join(r.bin, tool)); err != nil {
					t.Fatal(err)
				}
			}
			r.path = filepath.Join(r.home, ".local", "bin") + string(os.PathListSeparator) + r.bin

			code, output := r.run(t, script)
			if code != 0 {
				t.Fatalf("Exit-Code %d:\n%s", code, output)
			}
			if got := r.installed(t); got != "neu v0.10.0\n" {
				t.Errorf("am Ziel liegt %q:\n%s", got, output)
			}
			if !strings.Contains(output, "Neuestes Release: v0.10.0") {
				t.Errorf("Ausgabe:\n%s", output)
			}
			if requests := source.Requests(); len(requests) == 0 || requests[0] != "/releases/latest" {
				t.Errorf("Anfragen: %v", requests)
			}
			for _, request := range source.Requests() {
				if strings.HasPrefix(request, "/releases/tag/") {
					t.Errorf("der Weiterleitung wurde gefolgt: %v", source.Requests())
				}
			}
		})
	}
}

// Scheitert die Installation, endet bin/install ungleich 0 und ersetzt nichts:
// es gibt kein veröffentlichtes Release, das neueste trägt einen Tag mit
// Zusatz, das Asset fehlt, die Prüfsumme weicht ab,
// der Eintrag fehlt in SHA256SUMS, die Quelle ist abgeschaltet, das
// Zielverzeichnis steht nicht im PATH.
func TestBinInstallScheitertOhneZuErsetzen(t *testing.T) {
	script := installScript(t)

	for _, fall := range []struct {
		name    string
		prepare func(t *testing.T, source *releasetest.Source, r *installRun)
		want    string
	}{
		{
			name: "kein veröffentlichtes Release",
			prepare: func(t *testing.T, _ *releasetest.Source, _ *installRun) {
				// Eine Ersatzquelle ohne Release: /releases/latest antwortet mit 404.
				releasetest.Serve(t)
			},
			want: "kein veröffentlichtes Release",
		},
		{
			name: "neuestes Release mit Zusatz",
			prepare: func(_ *testing.T, source *releasetest.Source, _ *installRun) {
				source.SetLatest("v0.11.0-rc.1")
			},
			want: "keinen der Form vX.Y.Z",
		},
		{
			name: "Asset fehlt",
			prepare: func(_ *testing.T, source *releasetest.Source, _ *installRun) {
				source.RemoveAsset("v0.10.0", release.AssetName())
			},
			want: "noch nicht fertig",
		},
		{
			name: "Prüfsumme weicht ab",
			prepare: func(_ *testing.T, source *releasetest.Source, _ *installRun) {
				source.SetSums("v0.10.0", strings.Repeat("0", 64)+"  "+release.AssetName()+"\n")
			},
			want: "Prüfsumme weicht ab",
		},
		{
			name: "Eintrag fehlt in SHA256SUMS",
			prepare: func(_ *testing.T, source *releasetest.Source, _ *installRun) {
				source.SetSums("v0.10.0", strings.Repeat("0", 64)+"  k-playbook-fremd-arch\n")
			},
			want: "Keine Prüfsumme für " + release.AssetName(),
		},
		{
			name: "Quelle abgeschaltet",
			prepare: func(t *testing.T, _ *releasetest.Source, _ *installRun) {
				releasetest.Off(t)
			},
			want: "abgeschaltet",
		},
		{
			name: "Zielverzeichnis nicht im PATH",
			prepare: func(_ *testing.T, _ *releasetest.Source, r *installRun) {
				r.path = strings.Join([]string{r.bin, "/usr/local/bin", "/usr/bin", "/bin"}, string(os.PathListSeparator))
			},
			want: "liegt nicht im PATH",
		},
	} {
		t.Run(fall.name, func(t *testing.T) {
			source := releases(t)
			r := newInstallRun(t, true)
			if err := os.WriteFile(r.target, []byte("vorher\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			fall.prepare(t, source, r)

			code, output := r.run(t, script)
			if code == 0 {
				t.Fatalf("Exit-Code 0:\n%s", output)
			}
			if !strings.Contains(output, fall.want) {
				t.Errorf("%q fehlt in:\n%s", fall.want, output)
			}
			if got := r.installed(t); got != "vorher\n" {
				t.Errorf("am Ziel liegt %q, die Datei wurde ersetzt", got)
			}
			entries, _ := os.ReadDir(filepath.Dir(r.target))
			if len(entries) != 1 {
				t.Errorf("neben dem Ziel blieb etwas liegen: %v", entries)
			}
		})
	}
}

// Fehlt Kephalaion, installiert bin/install es danach über dessen install.sh —
// hier ein Ersatz aus der Ersatzquelle. Scheitert das, endet der Lauf trotzdem
// mit 0: k-playbook selbst ist installiert, und Binaries von vor Task 149
// messen ihr Programm-Update an diesem Exit-Code.
func TestBinInstallInstalliertKephalaionDanach(t *testing.T) {
	script := installScript(t)

	t.Run("fehlt und wird installiert", func(t *testing.T) {
		source := releases(t)
		source.SetKephalaionInstall("printf 'Ersatz-install.sh\\n'\nprintf '#!/bin/sh\\n' > \"$HOME/.local/bin/kephalaion\"\nchmod 755 \"$HOME/.local/bin/kephalaion\"\n")
		r := newInstallRun(t, false)

		code, output := r.run(t, script)
		if code != 0 {
			t.Fatalf("Exit-Code %d:\n%s", code, output)
		}
		if got := r.installed(t); got != "neu v0.10.0\n" {
			t.Errorf("am Ziel liegt %q", got)
		}
		if _, err := os.Stat(filepath.Join(r.home, ".local", "bin", "kephalaion")); err != nil {
			t.Errorf("kephalaion liegt nicht unter dem HOME des Laufs: %v\n%s", err, output)
		}
		if !strings.Contains(output, "Kephalaion fehlt; lade "+source.URL()+"/kephalaion/install.sh") || !strings.Contains(output, "Ersatz-install.sh") {
			t.Errorf("Ausgabe:\n%s", output)
		}
	})

	t.Run("install.sh scheitert", func(t *testing.T) {
		releases(t).SetKephalaionInstall("echo 'geht nicht' >&2\nexit 3\n")
		r := newInstallRun(t, false)

		code, output := r.run(t, script)
		if code != 0 {
			t.Fatalf("Exit-Code %d, erwartet 0 — das Programm ist installiert:\n%s", code, output)
		}
		if got := r.installed(t); got != "neu v0.10.0\n" {
			t.Errorf("am Ziel liegt %q", got)
		}
		for _, want := range []string{"geht nicht", "Kephalaion ist nicht installiert", "Nachholen: k-playbook upgrade"} {
			if !strings.Contains(output, want) {
				t.Errorf("%q fehlt in:\n%s", want, output)
			}
		}
	})

	t.Run("install.sh nicht ladbar", func(t *testing.T) {
		releases(t)
		r := newInstallRun(t, false)

		code, output := r.run(t, script)
		if code != 0 || r.installed(t) != "neu v0.10.0\n" {
			t.Fatalf("Exit-Code %d, am Ziel %q:\n%s", code, r.installed(t), output)
		}
		if !strings.Contains(output, "install.sh von Kephalaion nicht ladbar") {
			t.Errorf("Ausgabe:\n%s", output)
		}
	})
}
