package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/sharedir"
)

// fakeContainer stellt die Prüfpunkte der Quelle share nach: ob eine
// Container-Markierung existiert, was mountinfo zeigt und welche Variable
// gesetzt ist.
func fakeContainer(t *testing.T, container bool, mountinfo string, env map[string]string) {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "dockerenv")
	if container {
		writeFile(t, marker, "")
	}
	info := filepath.Join(dir, "mountinfo")
	writeFile(t, info, mountinfo)

	oldMarkers, oldInfo, oldGetenv := containerMarkers, mountInfoPath, shareGetenv
	containerMarkers = []string{marker}
	mountInfoPath = info
	shareGetenv = func(key string) string { return env[key] }
	t.Cleanup(func() { containerMarkers, mountInfoPath, shareGetenv = oldMarkers, oldInfo, oldGetenv })
}

// Zeilen wie gemessen mit Docker Desktop 29.7.2 unter WSL.
const (
	mountRoot   = "1691 1438 0:149 / / rw,relatime - overlay overlay rw,lowerdir=/a:/b"
	mountBind   = "1701 1691 8:48 /home/kleist/dev/omni /workspaces/omni rw,relatime - ext4 /dev/sdd rw"
	mountVolume = "1700 1691 8:80 /data/docker/volumes/omni-src/_data /workspaces/omni rw,relatime master:254 - ext4 /dev/sdf rw"
)

func shareProblems(t *testing.T, projectDir string) []health.Problem {
	t.Helper()
	return healthShare(context.Background(), projectDir)
}

func TestHealthShareWarntImContainerMitHostOrdnerOhneVariable(t *testing.T) {
	fakeContainer(t, true, mountRoot+"\n"+mountBind+"\n", nil)
	problems := shareProblems(t, "/workspaces/omni/sub")
	if len(problems) != 1 {
		t.Fatalf("Probleme = %+v, erwartet eines", problems)
	}
	p := problems[0]
	if p.Level != health.LevelWarning || p.Area != health.AreaShare {
		t.Errorf("Stufe/Bereich = %s/%s", p.Level, p.Area)
	}
	if !strings.Contains(p.Message, "/workspaces/omni") || !strings.Contains(p.Hint, sharedir.EnvDir) ||
		!strings.Contains(p.Hint, "${localEnv:HOME}") {
		t.Errorf("Satz oder Abhilfe unvollständig: %+v", p)
	}
}

func TestHealthShareSchweigt(t *testing.T) {
	cases := []struct {
		name      string
		container bool
		mountinfo string
		env       map[string]string
		project   string
	}{
		{name: "auf dem Host", container: false, mountinfo: mountRoot + "\n" + mountBind, project: "/workspaces/omni"},
		{name: "Variable gesetzt", container: true, mountinfo: mountRoot + "\n" + mountBind, env: map[string]string{sharedir.EnvDir: "/home/kleist/.local/share/k-playbook"}, project: "/workspaces/omni"},
		{name: "Projekt in einem Volume", container: true, mountinfo: mountRoot + "\n" + mountVolume, project: "/workspaces/omni"},
		{name: "Projekt im Container-Dateisystem", container: true, mountinfo: mountRoot + "\n" + mountBind, project: "/srv/projekt"},
		{name: "Nachbarordner mit gleichem Anfang", container: true, mountinfo: mountRoot + "\n" + mountBind, project: "/workspaces/omni-zwei"},
		{name: "mountinfo leer", container: true, mountinfo: "", project: "/workspaces/omni"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeContainer(t, c.container, c.mountinfo, c.env)
			if problems := shareProblems(t, c.project); len(problems) != 0 {
				t.Fatalf("Probleme = %+v, erwartet keine", problems)
			}
		})
	}
}

// Eine relative Variable meldet die Quelle überall, auch auf dem Host.
func TestHealthShareMeldetUngueltigeVariable(t *testing.T) {
	fakeContainer(t, false, mountRoot, nil)
	t.Setenv(sharedir.EnvDir, "relativ")
	problems := shareProblems(t, "/irgendwo")
	if len(problems) != 1 || !strings.Contains(problems[0].Message, sharedir.EnvDir) {
		t.Fatalf("Probleme = %+v", problems)
	}
}

// Leerzeichen im Pfad schreibt der Kernel als \040.
func TestParseMountInfoLoestEscapesAuf(t *testing.T) {
	entry, ok := parseMountInfo(`36 35 98:0 /home/a\040b /workspaces/a\040b rw - ext4 /dev/x rw`)
	if !ok || entry.root != "/home/a b" || entry.point != "/workspaces/a b" || entry.fsType != "ext4" {
		t.Fatalf("Eintrag = %+v, ok = %v", entry, ok)
	}
	if _, ok := parseMountInfo("kaputt"); ok {
		t.Error("kaputte Zeile gelesen")
	}
}

// Der Host dieses Testlaufs ist kein Container: die echten Prüfpunkte dürfen
// dort nichts melden, solange die Variable fehlt.
func TestHealthShareEchteUmgebung(t *testing.T) {
	for _, marker := range containerMarkers {
		if _, err := os.Stat(marker); err == nil {
			t.Skip("läuft in einem Container")
		}
	}
	t.Setenv(sharedir.EnvDir, "")
	if problems := shareProblems(t, t.TempDir()); len(problems) != 0 {
		t.Fatalf("Probleme = %+v", problems)
	}
}
