package hostinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPlatform = "k-playbook-linux-amd64"

// newSource baut eine Quell-Installation mit Wrapper und einem Binary auf.
func newSource(t *testing.T, binaryContent string) string {
	t.Helper()

	source := t.TempDir()
	writeFile(t, filepath.Join(source, binDirName, WrapperName), "#!/usr/bin/env bash\n")
	writeFile(t, filepath.Join(source, distDirName, testPlatform), binaryContent)
	return source
}

// newRequest verdrahtet Quelle und ein frisches Ziel.
func newRequest(t *testing.T, source string, stamp string) request {
	t.Helper()

	home := t.TempDir()
	return request{
		source:    source,
		target:    filepath.Join(home, ".local", "share", WrapperName, installDirName),
		linkDir:   filepath.Join(home, ".local", binDirName),
		platform:  testPlatform,
		stamp:     stamp,
		pathValue: filepath.Join(home, ".local", binDirName),
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
	if len(result.Copied) != 2 {
		t.Fatalf("erwartet 2 kopierte Dateien, bekommen %v", result.Copied)
	}

	binary := filepath.Join(req.target, distDirName, testPlatform)
	if got := readFile(t, binary); got != "binary-v1" {
		t.Errorf("Binary nicht gespiegelt: %q", got)
	}
	if !fileExists(filepath.Join(req.target, binDirName, WrapperName)) {
		t.Error("Wrapper fehlt im Ziel")
	}
	if got := readStamp(binary + stampSuffix); got != "1000" {
		t.Errorf("Stempel = %q, erwartet 1000", got)
	}

	// Der Wrapper leitet ueber ../dist ab; nur ein ausfuehrbares Binary hilft.
	info, err := os.Stat(binary)
	if err != nil {
		t.Fatalf("Binary pruefen: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("Binary ist nicht ausfuehrbar: %v", info.Mode())
	}
}

func TestMirrorVerlinktRelativ(t *testing.T) {
	req := newRequest(t, newSource(t, "binary-v1"), "1000")

	result, err := mirrorInto(req)
	if err != nil {
		t.Fatalf("mirrorInto: %v", err)
	}

	linkPath := filepath.Join(req.linkDir, WrapperName)
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

	// Derselbe Host, aber gestartet aus einem Clone mit aelterem Stand.
	alt := req
	alt.source = newSource(t, "binary-alt")
	alt.stamp = "1000"

	result, err := mirrorInto(alt)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if len(result.Copied) != 0 {
		t.Errorf("aelterer Stand hat ueberschrieben: %v", result.Copied)
	}
	if got := readFile(t, filepath.Join(req.target, distDirName, testPlatform)); got != "binary-neu" {
		t.Errorf("Binary = %q, erwartet binary-neu", got)
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
	if len(result.Copied) != 2 {
		t.Fatalf("erwartet 2 kopierte Dateien, bekommen %v", result.Copied)
	}
	if got := readFile(t, filepath.Join(req.target, distDirName, testPlatform)); got != "binary-neu" {
		t.Errorf("Binary = %q, erwartet binary-neu", got)
	}
	if got := readStamp(filepath.Join(req.target, distDirName, testPlatform) + stampSuffix); got != "2000" {
		t.Errorf("Stempel = %q, erwartet 2000", got)
	}
}

// Der Fall Mac-Host plus DevContainer bei geteiltem Home: gleicher Clone,
// gleicher Stand, aber die Plattform des Aufrufers liegt noch nicht im Ziel.
func TestMirrorErgaenztFehlendePlattform(t *testing.T) {
	req := newRequest(t, newSource(t, "linux"), "1000")
	if _, err := mirrorInto(req); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}

	andere := req
	andere.platform = "k-playbook-darwin-arm64"
	andere.source = t.TempDir()
	writeFile(t, filepath.Join(andere.source, binDirName, WrapperName), "#!/usr/bin/env bash\n")
	writeFile(t, filepath.Join(andere.source, distDirName, andere.platform), "darwin")

	result, err := mirrorInto(andere)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if len(result.Copied) != 2 {
		t.Fatalf("fehlende Plattform nicht ergaenzt: %v", result.Copied)
	}
	if got := readFile(t, filepath.Join(req.target, distDirName, andere.platform)); got != "darwin" {
		t.Errorf("zweites Binary = %q, erwartet darwin", got)
	}
	// Das zuerst gespiegelte Binary bleibt daneben liegen.
	if got := readFile(t, filepath.Join(req.target, distDirName, testPlatform)); got != "linux" {
		t.Errorf("erstes Binary = %q, erwartet linux", got)
	}
}

func TestMirrorOhneStempelNurWennZielFehlt(t *testing.T) {
	// Quelle ohne Git: kein Stempel ermittelbar.
	req := newRequest(t, newSource(t, "binary-v1"), "")

	result, err := mirrorInto(req)
	if err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	if len(result.Copied) != 2 {
		t.Fatalf("fehlendes Ziel nicht befuellt: %v", result.Copied)
	}
	if fileExists(filepath.Join(req.target, distDirName, testPlatform) + stampSuffix) {
		t.Error("ohne Stempel darf keine Stempeldatei entstehen")
	}

	zweite := req
	zweite.source = newSource(t, "binary-v2")

	result, err = mirrorInto(zweite)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if len(result.Copied) != 0 {
		t.Errorf("ohne Stempel wurde ueberschrieben: %v", result.Copied)
	}
}

func TestMirrorLaesstEchteDateiInRuhe(t *testing.T) {
	req := newRequest(t, newSource(t, "binary-v1"), "1000")
	eigene := filepath.Join(req.linkDir, WrapperName)
	writeFile(t, eigene, "#!/usr/bin/env bash\n# von Hand abgelegt\n")

	result, err := mirrorInto(req)
	if err != nil {
		t.Fatalf("mirrorInto: %v", err)
	}
	if result.Link != "" {
		t.Errorf("echte Datei wurde angefasst: %q", result.Link)
	}
	if got := readFile(t, eigene); !strings.Contains(got, "von Hand abgelegt") {
		t.Errorf("echte Datei ueberschrieben: %q", got)
	}
}

func TestMirrorRichtetFalschenLinkAus(t *testing.T) {
	req := newRequest(t, newSource(t, "binary-v1"), "1000")
	if err := os.MkdirAll(req.linkDir, 0o755); err != nil {
		t.Fatalf("linkDir anlegen: %v", err)
	}
	linkPath := filepath.Join(req.linkDir, WrapperName)
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
		t.Fatal("fehlendes Quell-Binary blieb unbemerkt")
	}
}

func TestPlatformBinary(t *testing.T) {
	if got := PlatformBinary("darwin", "arm64"); got != "k-playbook-darwin-arm64" {
		t.Errorf("PlatformBinary = %q", got)
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
		{"aelter", "1000", "2000", false},
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

// Liegt das Verzeichnis ausserhalb des Homes, waere $HOME falsch.
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
	// Ein Symlink, den niemand findet, nuetzt nichts.
	if (PathStatus{Linked: true}).OK() {
		t.Error("ohne PATH darf es nicht ok sein")
	}
	// Und ein PATH-Eintrag ohne Symlink genauso wenig.
	if (PathStatus{InPath: true}).OK() {
		t.Error("ohne Symlink darf es nicht ok sein")
	}
}
