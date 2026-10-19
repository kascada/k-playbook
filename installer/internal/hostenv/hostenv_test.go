package hostenv

import (
	"os"
	"path/filepath"
	"testing"
)

// clearWSLEnv nimmt die WSL-Marken aus der Umgebung, damit der Test auf einem
// WSL-Rechner dasselbe sieht wie überall sonst.
func clearWSLEnv(t *testing.T) {
	t.Helper()
	t.Setenv("WSL_DISTRO_NAME", "")
	t.Setenv("WSL_INTEROP", "")
}

// useProcVersion lässt IsWSL eine nachgebaute /proc/version lesen.
func useProcVersion(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "version")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("version schreiben: %v", err)
	}
	before := procVersion
	procVersion = path
	t.Cleanup(func() { procVersion = before })
}

// Die Umgebungsvariable entscheidet vor dem Kernel.
func TestIsWSLUeberUmgebung(t *testing.T) {
	clearWSLEnv(t)
	useProcVersion(t, "Linux version 6.8.0 (gcc) #1 SMP")
	if IsWSL() {
		t.Fatal("ohne Marke und mit gewöhnlichem Kernel gilt kein WSL")
	}

	t.Setenv("WSL_DISTRO_NAME", "Ubuntu")
	if !IsWSL() {
		t.Error("WSL_DISTRO_NAME wurde nicht erkannt")
	}
}

// Ohne Marken in der Umgebung zählt der Kernelname, unabhängig von der
// Schreibweise.
func TestIsWSLUeberKernel(t *testing.T) {
	clearWSLEnv(t)
	useProcVersion(t, "Linux version 6.6.87.2-microsoft-standard-WSL2 (root@...) #1 SMP")
	if !IsWSL() {
		t.Error("der Microsoft-Kernel wurde nicht erkannt")
	}

	useProcVersion(t, "Linux version 6.6.0-Microsoft-Standard (root@...) #1 SMP")
	if !IsWSL() {
		t.Error("die Großschreibung wurde nicht erkannt")
	}
}

// Fehlt die Datei, ist das kein WSL und kein Fehler.
func TestIsWSLOhneProcVersion(t *testing.T) {
	clearWSLEnv(t)
	before := procVersion
	procVersion = filepath.Join(t.TempDir(), "fehlt")
	t.Cleanup(func() { procVersion = before })
	if IsWSL() {
		t.Error("ohne /proc/version gilt kein WSL")
	}
}

// Die Container-Marken der Umgebung werden mit ihrem Namen gemeldet.
func TestContainerMarkerUeberUmgebung(t *testing.T) {
	for _, name := range []string{"REMOTE_CONTAINERS", "DEVCONTAINER", "CODESPACES"} {
		t.Setenv(name, "")
	}
	t.Chdir(t.TempDir())
	if _, err := os.Stat("/.dockerenv"); err == nil {
		t.Skip("dieser Test läuft selbst in einem Container")
	}
	if marker, ok := ContainerMarker(); ok {
		t.Fatalf("ohne Marke wurde %q gemeldet", marker)
	}

	t.Setenv("DEVCONTAINER", "true")
	marker, ok := ContainerMarker()
	if !ok || marker != "DEVCONTAINER" {
		t.Errorf("Marke = %q, %v; erwartet DEVCONTAINER", marker, ok)
	}
}
