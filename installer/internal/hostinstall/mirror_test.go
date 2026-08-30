package hostinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// newSource baut eine Quell-Installation aus genau den Dateien, die gespiegelt
// werden. Das Argument landet in VERSION und macht zwei Quellen unterscheidbar.
func newSource(t *testing.T, version string) string {
	t.Helper()

	source := t.TempDir()
	writeFile(t, filepath.Join(source, project.BinDirName, project.WrapperName), "#!/usr/bin/env bash\n")
	writeFile(t, filepath.Join(source, project.VersionFileName), version)
	writeFile(t, filepath.Join(source, project.SumsFileName), "summe  "+version+"\n")
	return source
}

// mirroredVersion liest die VERSION aus der host-weiten Kopie.
func mirroredVersion(t *testing.T, req request) string {
	t.Helper()

	return readFile(t, filepath.Join(req.target, project.VersionFileName))
}

// newRequest verdrahtet Quelle und ein frisches Ziel.
func newRequest(t *testing.T, source string, stamp string) request {
	t.Helper()

	home := t.TempDir()
	return request{
		source:    source,
		target:    filepath.Join(home, ".local", "share", project.WrapperName, installDirName),
		linkDir:   filepath.Join(home, ".local", project.BinDirName),
		stamp:     stamp,
		pathValue: filepath.Join(home, ".local", project.BinDirName),
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%s anlegen: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("%s schreiben: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s lesen: %v", path, err)
	}
	return string(content)
}

func TestMirrorLegtZielAn(t *testing.T) {
	req := newRequest(t, newSource(t, "binary-v1"), "1000")

	result, err := mirrorInto(req)
	if err != nil {
		t.Fatalf("mirrorInto: %v", err)
	}
	if len(result.Copied) != 3 {
		t.Fatalf("erwartet 3 kopierte Dateien, bekommen %v", result.Copied)
	}

	wrapper := filepath.Join(req.target, project.BinDirName, project.WrapperName)
	if !fileExists(wrapper) {
		t.Error("Wrapper fehlt im Ziel")
	}
	if got := mirroredVersion(t, req); got != "binary-v1" {
		t.Errorf("VERSION = %q, erwartet binary-v1", got)
	}
	if !fileExists(filepath.Join(req.target, project.SumsFileName)) {
		t.Error("SHA256SUMS fehlt im Ziel")
	}
	if got := readStamp(filepath.Join(req.target, stampFileName)); got != "1000" {
		t.Errorf("Stempel = %q, erwartet 1000", got)
	}

	// Gestartet wird über den Wrapper; nur ausführbar hilft er.
	info, err := os.Stat(wrapper)
	if err != nil {
		t.Fatalf("Wrapper prüfen: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("Wrapper ist nicht ausführbar: %v", info.Mode())
	}
}

func TestMirrorVerlinktRelativ(t *testing.T) {
	req := newRequest(t, newSource(t, "binary-v1"), "1000")

	result, err := mirrorInto(req)
	if err != nil {
		t.Fatalf("mirrorInto: %v", err)
	}

	linkPath := filepath.Join(req.linkDir, project.WrapperName)
	if result.Link != linkPath {
		t.Errorf("Link = %q, erwartet %q", result.Link, linkPath)
	}

	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if filepath.IsAbs(target) {
		t.Errorf("Link ist absolut: %q", target)
	}
	if !fileExists(linkPath) {
		t.Errorf("Link zeigt ins Leere: %q", target)
	}
}

func TestMirrorUeberspringtGleichenStand(t *testing.T) {
	req := newRequest(t, newSource(t, "binary-v1"), "1000")

	if _, err := mirrorInto(req); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}

	result, err := mirrorInto(req)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if len(result.Copied) != 0 {
		t.Errorf("zweiter Lauf hat kopiert: %v", result.Copied)
	}
	if result.Link != "" {
		t.Errorf("zweiter Lauf hat verlinkt: %q", result.Link)
	}
	if !result.Empty() {
		t.Error("zweiter Lauf sollte still bleiben")
	}
}

func TestMirrorUeberspringtAelterenStand(t *testing.T) {
	req := newRequest(t, newSource(t, "binary-neu"), "2000")
	if _, err := mirrorInto(req); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}

	// Derselbe Host, aber gestartet aus einem Clone mit älterem Stand.
	alt := req
	alt.source = newSource(t, "binary-alt")
	alt.stamp = "1000"

	result, err := mirrorInto(alt)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if len(result.Copied) != 0 {
		t.Errorf("älterer Stand hat überschrieben: %v", result.Copied)
	}
	if got := mirroredVersion(t, req); got != "binary-neu" {
		t.Errorf("VERSION = %q, erwartet binary-neu", got)
	}
}

func TestMirrorHebtAufNeuerenStand(t *testing.T) {
	req := newRequest(t, newSource(t, "binary-alt"), "1000")
	if _, err := mirrorInto(req); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}

	neu := req
	neu.source = newSource(t, "binary-neu")
	neu.stamp = "2000"

	result, err := mirrorInto(neu)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if len(result.Copied) != 3 {
		t.Fatalf("erwartet 3 kopierte Dateien, bekommen %v", result.Copied)
	}
	if got := mirroredVersion(t, req); got != "binary-neu" {
		t.Errorf("VERSION = %q, erwartet binary-neu", got)
	}
	if got := readStamp(filepath.Join(req.target, stampFileName)); got != "2000" {
		t.Errorf("Stempel = %q, erwartet 2000", got)
	}
}

// Bis die Binaries Release-Assets wurden, lag im Ziel eine Kopie unter dist/.
// Sie muss weg: der Wrapper zieht ein vorhandenes dist/ dem Cache vor und
// startete sonst auf Dauer den alten Stand.
func TestMirrorRaeumtAltesDistWeg(t *testing.T) {
	req := newRequest(t, newSource(t, "v1"), "1000")
	altes := filepath.Join(req.target, legacyDistDirName, "k-playbook-linux-amd64")
	writeFile(t, altes, "altes Binary")

	result, err := mirrorInto(req)
	if err != nil {
		t.Fatalf("mirrorInto: %v", err)
	}
	if len(result.Copied) != 3 {
		t.Fatalf("erwartet 3 kopierte Dateien, bekommen %v", result.Copied)
	}
	if fileExists(altes) {
		t.Error("altes Binary liegt weiterhin im Ziel")
	}
	if dirExists(filepath.Join(req.target, legacyDistDirName)) {
		t.Error("dist/ liegt weiterhin im Ziel")
	}
}

// Ein zurückgebliebenes dist/ löst die Spiegelung auch dann aus, wenn der
// Stempel gleich geblieben ist — sonst überlebte es jeden weiteren Start.
func TestMirrorRaeumtAltesDistAuchBeiGleichemStandWeg(t *testing.T) {
	req := newRequest(t, newSource(t, "v1"), "1000")
	if _, err := mirrorInto(req); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	altes := filepath.Join(req.target, legacyDistDirName, "k-playbook-linux-amd64")
	writeFile(t, altes, "altes Binary")

	if _, err := mirrorInto(req); err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if fileExists(altes) {
		t.Error("altes Binary liegt weiterhin im Ziel")
	}
}

func TestMirrorOhneStempelNurWennZielFehlt(t *testing.T) {
	// Quelle ohne Git: kein Stempel ermittelbar.
	req := newRequest(t, newSource(t, "binary-v1"), "")

	result, err := mirrorInto(req)
	if err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	if len(result.Copied) != 3 {
		t.Fatalf("fehlendes Ziel nicht befüllt: %v", result.Copied)
	}
	if fileExists(filepath.Join(req.target, stampFileName)) {
		t.Error("ohne Stempel darf keine Stempeldatei entstehen")
	}

	zweite := req
	zweite.source = newSource(t, "binary-v2")

	result, err = mirrorInto(zweite)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if len(result.Copied) != 0 {
		t.Errorf("ohne Stempel wurde überschrieben: %v", result.Copied)
	}
}

func TestMirrorLaesstEchteDateiInRuhe(t *testing.T) {
	req := newRequest(t, newSource(t, "binary-v1"), "1000")
	eigene := filepath.Join(req.linkDir, project.WrapperName)
	writeFile(t, eigene, "#!/usr/bin/env bash\n# von Hand abgelegt\n")

	result, err := mirrorInto(req)
	if err != nil {
		t.Fatalf("mirrorInto: %v", err)
	}
	if result.Link != "" {
		t.Errorf("echte Datei wurde angefasst: %q", result.Link)
	}
	if got := readFile(t, eigene); !strings.Contains(got, "von Hand abgelegt") {
		t.Errorf("echte Datei überschrieben: %q", got)
	}
}

func TestMirrorRichtetFalschenLinkAus(t *testing.T) {
	req := newRequest(t, newSource(t, "binary-v1"), "1000")
	if err := os.MkdirAll(req.linkDir, 0o755); err != nil {
		t.Fatalf("linkDir anlegen: %v", err)
	}
	linkPath := filepath.Join(req.linkDir, project.WrapperName)
	if err := os.Symlink("/woanders/k-playbook", linkPath); err != nil {
		t.Fatalf("Symlink anlegen: %v", err)
	}

	result, err := mirrorInto(req)
	if err != nil {
		t.Fatalf("mirrorInto: %v", err)
	}
	if result.Link != linkPath {
		t.Errorf("falscher Link nicht korrigiert: %q", result.Link)
	}
	if !fileExists(linkPath) {
		t.Error("Link zeigt weiterhin ins Leere")
	}
}

func TestMirrorMeldetFehlendenPath(t *testing.T) {
	req := newRequest(t, newSource(t, "binary-v1"), "1000")
	req.pathValue = "/usr/bin:/bin"

	result, err := mirrorInto(req)
	if err != nil {
		t.Fatalf("mirrorInto: %v", err)
	}
	if result.PathHint != req.linkDir {
		t.Errorf("PathHint = %q, erwartet %q", result.PathHint, req.linkDir)
	}
}

func TestMirrorMeldetVorhandenenPathNicht(t *testing.T) {
	req := newRequest(t, newSource(t, "binary-v1"), "1000")
	req.pathValue = strings.Join([]string{"/usr/bin", req.linkDir, "/bin"}, string(filepath.ListSeparator))

	result, err := mirrorInto(req)
	if err != nil {
		t.Fatalf("mirrorInto: %v", err)
	}
	if result.PathHint != "" {
		t.Errorf("PathHint = %q, erwartet leer", result.PathHint)
	}
}

func TestMirrorMeldetFehlendeQuelle(t *testing.T) {
	req := newRequest(t, t.TempDir(), "1000")

	if _, err := mirrorInto(req); err == nil {
		t.Fatal("fehlende Quelldateien blieben unbemerkt")
	}
}

func TestNewer(t *testing.T) {
	tests := []struct {
		name   string
		source string
		target string
		want   bool
	}{
		{"neuer", "2000", "1000", true},
		{"älter", "1000", "2000", false},
		{"gleich", "1000", "1000", false},
		{"Ziel unbekannt", "1000", "", true},
		{"Quelle unbekannt", "", "1000", false},
		{"beide unbekannt", "", "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := newer(test.source, test.target); got != test.want {
				t.Errorf("newer(%q, %q) = %v", test.source, test.target, got)
			}
		})
	}
}

func TestExportLineSetztHomeEin(t *testing.T) {
	line := ExportLine(filepath.Join("/home/jemand", ".local", "bin"), "/home/jemand")
	if want := `export PATH="$HOME/.local/bin:$PATH"`; line != want {
		t.Errorf("ExportLine = %q, erwartet %q", line, want)
	}
}

// Liegt das Verzeichnis außerhalb des Homes, wäre $HOME falsch.
func TestExportLineBehaeltFremdenPfad(t *testing.T) {
	line := ExportLine("/opt/bin", "/home/jemand")
	if want := `export PATH="/opt/bin:$PATH"`; line != want {
		t.Errorf("ExportLine = %q, erwartet %q", line, want)
	}
}

func TestPathStatusOK(t *testing.T) {
	if (PathStatus{Linked: true, InPath: true}).OK() != true {
		t.Error("verlinkt und im PATH muss ok sein")
	}
	// Ein Symlink, den niemand findet, nützt nichts.
	if (PathStatus{Linked: true}).OK() {
		t.Error("ohne PATH darf es nicht ok sein")
	}
	// Und ein PATH-Eintrag ohne Symlink genauso wenig.
	if (PathStatus{InPath: true}).OK() {
		t.Error("ohne Symlink darf es nicht ok sein")
	}
}
