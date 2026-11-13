package program

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/release/releasetest"
)

// Kein Test lädt oder startet das echte install.sh von Kephalaion: Die Quelle
// ist in TestMain abgeschaltet, und wer sie braucht, legt mit
// kephalaionInstallScript ein Ersatz-install.sh in eine Ersatzquelle.

// kephalaionInstallScript ist ein Ersatz-install.sh: Es schreibt einen Marker,
// legt bei install ein kephalaion nach ~/.local/bin und endet mit exitCode.
func kephalaionInstallScript(marker string, install bool, exitCode int) string {
	place := ""
	if install {
		place = "mkdir -p \"$HOME/.local/bin\"\nprintf '#!/bin/sh\\n' > \"$HOME/.local/bin/kephalaion\"\nchmod 755 \"$HOME/.local/bin/kephalaion\"\n"
	}
	return fmt.Sprintf("echo \"Ersatz-install.sh von Kephalaion\"\n: > %q\n%sexit %d\n", marker, place, exitCode)
}

// Fehlt kephalaion, lädt InstallKephalaion das install.sh und führt es aus —
// ohne Projekt und ohne Clone; Erfolg heißt, danach liegt kephalaion da.
func TestInstallKephalaionInstalliertFehlendes(t *testing.T) {
	home, _ := isolate(t)
	marker := filepath.Join(t.TempDir(), "install-gelaufen")
	source := releasetest.Serve(t)
	source.SetKephalaionInstall(kephalaionInstallScript(marker, true, 0))

	result, err := InstallKephalaion(context.Background())
	if err != nil {
		t.Fatalf("InstallKephalaion: %v\n%s", err, result.Output)
	}
	if !ran(marker) || !result.Installed {
		t.Fatalf("das install.sh ist nicht gelaufen: %+v", result)
	}
	if want := filepath.Join(home, ".local", "bin", "kephalaion"); result.Path != want {
		t.Errorf("Path = %q, erwartet %q", result.Path, want)
	}
	if !strings.Contains(result.Output, "Ersatz-install.sh von Kephalaion") {
		t.Errorf("Ausgabe des install.sh fehlt: %q", result.Output)
	}
	if requests := source.Requests(); len(requests) != 1 || requests[0] != "/kephalaion/install.sh" {
		t.Errorf("Anfragen = %v, erwartet genau das install.sh", requests)
	}
}

// Ein vorhandenes kephalaion bleibt unangetastet: kein Abruf, kein Skript.
func TestInstallKephalaionLaesstVorhandenesStehen(t *testing.T) {
	home, _ := isolate(t)
	present := filepath.Join(home, ".local", "bin", "kephalaion")
	writeExecutable(t, present, "#!/bin/sh\n")
	marker := filepath.Join(t.TempDir(), "install-gelaufen")
	source := releasetest.Serve(t)
	source.SetKephalaionInstall(kephalaionInstallScript(marker, true, 0))

	result, err := InstallKephalaion(context.Background())
	if err != nil || ran(marker) || result.Installed || result.Path != present {
		t.Errorf("Ergebnis = %+v, Fehler %v, gelaufen %v", result, err, ran(marker))
	}
	if requests := source.Requests(); len(requests) != 0 {
		t.Errorf("trotz vorhandenem kephalaion wurde geladen: %v", requests)
	}
}

// Der Download scheitert, das Skript scheitert, und nach der Installation
// liegt kephalaion nirgends: je ein eigener Fehler.
func TestInstallKephalaionFehler(t *testing.T) {
	t.Run("Download scheitert", func(t *testing.T) {
		isolate(t)
		// Die Ersatzquelle trägt kein install.sh: 404.
		releasetest.Serve(t)

		result, err := InstallKephalaion(context.Background())
		if err == nil || !strings.Contains(err.Error(), "ließ sich nicht laden") || !strings.Contains(err.Error(), "404") {
			t.Errorf("Fehler = %v", err)
		}
		if result.Installed || result.Output != "" {
			t.Errorf("ohne Download ist ein Skript gelaufen: %+v", result)
		}
	})

	t.Run("Quelle abgeschaltet", func(t *testing.T) {
		isolate(t)
		releasetest.Off(t)

		if _, err := InstallKephalaion(context.Background()); !errors.Is(err, release.ErrDisabled) {
			t.Errorf("Fehler = %v, erwartet ErrDisabled", err)
		}
	})

	t.Run("Skript scheitert", func(t *testing.T) {
		isolate(t)
		marker := filepath.Join(t.TempDir(), "install-gelaufen")
		releasetest.Serve(t).SetKephalaionInstall(kephalaionInstallScript(marker, false, 4))

		result, err := InstallKephalaion(context.Background())
		if err == nil || !strings.Contains(err.Error(), "install.sh von Kephalaion endete mit Exit-Code 4") {
			t.Errorf("Fehler = %v", err)
		}
		if !ran(marker) || !result.Installed || !strings.Contains(result.Output, "Ersatz-install.sh") {
			t.Errorf("Ergebnis = %+v", result)
		}
	})

	t.Run("nach der Installation nicht im PATH", func(t *testing.T) {
		isolate(t)
		marker := filepath.Join(t.TempDir(), "install-gelaufen")
		releasetest.Serve(t).SetKephalaionInstall(kephalaionInstallScript(marker, false, 0))

		result, err := InstallKephalaion(context.Background())
		if err == nil || !strings.Contains(err.Error(), "nach der Installation weder im PATH") {
			t.Errorf("Fehler = %v", err)
		}
		if !ran(marker) || result.Path != "" {
			t.Errorf("Ergebnis = %+v", result)
		}
	})
}

// Das geladene Skript bleibt nicht liegen, und der Weg von Hand in den
// Hinweisen nennt dieselbe Adresse, die das Programm lädt.
func TestInstallKephalaionRaeumtAufUndNenntDieselbeAdresse(t *testing.T) {
	isolate(t)
	work := t.TempDir()
	t.Setenv("TMPDIR", work)
	marker := filepath.Join(t.TempDir(), "install-gelaufen")
	releasetest.Serve(t).SetKephalaionInstall(kephalaionInstallScript(marker, true, 0))

	if _, err := InstallKephalaion(context.Background()); err != nil {
		t.Fatalf("InstallKephalaion: %v", err)
	}
	entries, err := os.ReadDir(work)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "k-playbook-kephalaion-") {
			t.Errorf("das Arbeitsverzeichnis blieb liegen: %s", entry.Name())
		}
	}

	if !strings.Contains(kephalaion.InstallCommand, release.KephalaionInstallURL) {
		t.Errorf("kephalaion.InstallCommand (%q) nennt nicht release.KephalaionInstallURL (%q)",
			kephalaion.InstallCommand, release.KephalaionInstallURL)
	}
}
