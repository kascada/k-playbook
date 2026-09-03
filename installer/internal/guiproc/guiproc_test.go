package guiproc

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Der Ort der Laufzeitdatei folgt einer festen Reihenfolge: das Verzeichnis
// der Sitzung zuerst, dann der Zustandsordner, zuletzt das Home.
func TestRuntimeDirReihenfolge(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		home    string
		want    string
		wantErr bool
	}{
		{
			name: "XDG_RUNTIME_DIR geht vor",
			env:  map[string]string{"XDG_RUNTIME_DIR": "/run/user/1000", "XDG_STATE_HOME": "/state"},
			home: "/home/x",
			want: "/run/user/1000/k-playbook",
		},
		{
			name: "XDG_STATE_HOME als Rückfall",
			env:  map[string]string{"XDG_STATE_HOME": "/state"},
			home: "/home/x",
			want: "/state/k-playbook",
		},
		{
			name: "Home als letzter Rückfall",
			env:  map[string]string{},
			home: "/home/x",
			want: "/home/x/.local/state/k-playbook",
		},
		{
			name: "Leerzeichen zählen nicht als gesetzt",
			env:  map[string]string{"XDG_RUNTIME_DIR": "  "},
			home: "/home/x",
			want: "/home/x/.local/state/k-playbook",
		},
		{
			name:    "ohne alles ein Fehler",
			env:     map[string]string{},
			home:    "",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getenv := func(name string) string { return test.env[name] }
			homeDir := func() (string, error) {
				if test.home == "" {
					return "", errors.New("kein Home")
				}
				return test.home, nil
			}
			got, err := runtimeDirFrom(getenv, homeDir)
			if test.wantErr {
				if err == nil {
					t.Fatalf("kein Fehler, Verzeichnis %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Fehler: %v", err)
			}
			if got != test.want {
				t.Errorf("Verzeichnis = %q, erwartet %q", got, test.want)
			}
		})
	}
}

// RuntimeDir legt das Verzeichnis an, und zwar nur für den Nutzer lesbar.
func TestRuntimeDirLegtVerzeichnisAn(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "run"))

	dir, err := RuntimeDir()
	if err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Verzeichnis fehlt: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("Rechte = %o, erwartet 700", perm)
	}
	// Ein zweiter Aufruf findet es vor und ist kein Fehler.
	if _, err := RuntimeDir(); err != nil {
		t.Errorf("zweiter Aufruf: %v", err)
	}
}

// Der Dateiname ist ein kurzer Hash des Schlüssels: gleich für gleiche
// Schlüssel, verschieden für verschiedene, und ohne Pfadzeichen.
func TestFileNameIstKurzerHashDesSchluessels(t *testing.T) {
	name := fileName("/home/x/projekt")
	if len(name) != 16 {
		t.Errorf("Länge = %d, erwartet 16", len(name))
	}
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(name) {
		t.Errorf("kein Hex: %q", name)
	}
	if name != fileName("/home/x/projekt") {
		t.Error("nicht deterministisch")
	}
	if name == fileName("/home/x/anderes") {
		t.Error("verschiedene Schlüssel, gleicher Name")
	}
}

// Der Schlüssel ist das aufgelöste ProjectDir; ohne eines das
// Arbeitsverzeichnis.
func TestKeyForNimmtProjectDirSonstArbeitsverzeichnis(t *testing.T) {
	workdir := t.TempDir()

	installed := project.Environment{Installed: true, ProjectDir: "/home/x/projekt"}
	if got := keyFor(installed, filepath.Join(workdir, "sub")); got != "/home/x/projekt" {
		t.Errorf("installiert: Schlüssel = %q", got)
	}

	// Erster Start aus der projektlokalen Installation: ProjectDir ist
	// aufgelöst, Installed aber noch nicht — der Schlüssel folgt trotzdem dem
	// Verzeichnis, damit er nach dem Anlegen der Konfiguration derselbe bleibt.
	setup := project.Environment{ProjectDir: "/home/x/projekt", PlaybookPresent: true}
	if got := keyFor(setup, workdir); got != "/home/x/projekt" {
		t.Errorf("Ersteinrichtung: Schlüssel = %q", got)
	}

	if got := keyFor(project.Environment{}, workdir); got != canonical(workdir) {
		t.Errorf("ohne Installation: Schlüssel = %q, erwartet %q", got, canonical(workdir))
	}
}

// Die Datei entsteht exklusiv: ein zweiter Schreiber verliert. Umschlüsseln
// schreibt die neue Datei und räumt die alte weg.
func TestRegisterSchreibtExklusiv(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	record := Record{Key: "/p", Addr: "127.0.0.1:4711", PID: 42, Version: "v1", StartTime: 1000}

	registration, err := Register(record)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := Register(record); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("zweites Register: %v, erwartet fs.ErrExist", err)
	}

	read, ok, err := Read(registration.Path())
	if err != nil || !ok {
		t.Fatalf("Read: ok=%v, err=%v", ok, err)
	}
	if read != record {
		t.Errorf("gelesen %+v, geschrieben %+v", read, record)
	}
	if got := read.URL(); got != "http://127.0.0.1:4711/" {
		t.Errorf("URL = %q", got)
	}

	previous := registration.Path()
	if err := registration.Rekey("/q"); err != nil {
		t.Fatalf("Rekey: %v", err)
	}
	if registration.Path() == previous {
		t.Error("Rekey hat den Pfad nicht gewechselt")
	}
	if _, ok, _ := Read(previous); ok {
		t.Error("die alte Datei liegt noch")
	}
	read, ok, err = Read(registration.Path())
	if err != nil || !ok {
		t.Fatalf("Read nach Rekey: ok=%v, err=%v", ok, err)
	}
	if read.Key != "/q" || read.Addr != record.Addr || read.PID != record.PID {
		t.Errorf("nach Rekey %+v", read)
	}
	// Gleicher Schlüssel: nichts zu tun.
	if err := registration.Rekey("/q"); err != nil {
		t.Errorf("Rekey auf denselben Schlüssel: %v", err)
	}

	if err := registration.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok, _ := Read(registration.Path()); ok {
		t.Error("die Datei liegt nach Remove noch")
	}
	if err := registration.Remove(); err != nil {
		t.Errorf("zweites Remove: %v", err)
	}
}

// Die Einordnung in zwei Stufen: erst die Prozessidentität, dann die Antwort.
func TestClassify(t *testing.T) {
	record := Record{Key: "/p", Addr: "127.0.0.1:1", PID: 4242, Version: "v1", StartTime: 1000}
	identity := func(pid int, start time.Time) bool { return pid == 4242 && start.Unix() == 1000 }
	healthy := func(string) (Health, error) {
		return Health{Status: "ok", Key: "/p", Version: "v1", PID: 4242}, nil
	}

	tests := []struct {
		name     string
		record   Record
		identity func(int, time.Time) bool
		health   func(string) (Health, error)
		version  string
		want     Status
	}{
		{
			name:     "PID tot: verwaist",
			record:   record,
			identity: func(int, time.Time) bool { return false },
			health:   healthy,
			version:  "v1",
			want:     StatusOrphaned,
		},
		{
			name:     "PID neu vergeben, Startzeit passt nicht: verwaist",
			record:   Record{Key: "/p", Addr: "127.0.0.1:1", PID: 4242, Version: "v1", StartTime: 5000},
			identity: identity,
			health:   healthy,
			version:  "v1",
			want:     StatusOrphaned,
		},
		{
			name:     "keine Antwort: lebt ohne Antwort",
			record:   record,
			identity: identity,
			health:   func(string) (Health, error) { return Health{}, errors.New("connection refused") },
			version:  "v1",
			want:     StatusUnresponsive,
		},
		{
			name:     "fremder Schlüssel: lebt ohne Antwort",
			record:   record,
			identity: identity,
			health: func(string) (Health, error) {
				return Health{Status: "ok", Key: "/anderes", Version: "v1", PID: 4242}, nil
			},
			version: "v1",
			want:    StatusUnresponsive,
		},
		{
			name:     "fremde PID hinter dem Port: lebt ohne Antwort",
			record:   record,
			identity: identity,
			health: func(string) (Health, error) {
				return Health{Status: "ok", Key: "/p", Version: "v1", PID: 99}, nil
			},
			version: "v1",
			want:    StatusUnresponsive,
		},
		{
			name:     "andere Version",
			record:   record,
			identity: identity,
			health:   healthy,
			version:  "v2",
			want:     StatusOtherVersion,
		},
		{
			name:     "läuft",
			record:   record,
			identity: identity,
			health:   healthy,
			version:  "v1",
			want:     StatusRunning,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			finding := Classify(test.record, "/p", test.version, Inspector{Identity: test.identity, Health: test.health})
			if finding.Status != test.want {
				t.Errorf("Status = %s, erwartet %s", finding.Status, test.want)
			}
			if finding.Record != test.record {
				t.Errorf("Record fehlt im Ergebnis")
			}
		})
	}
}

// Inspect ordnet auch die Fälle ohne lesbare Datei ein und nennt den Pfad.
func TestInspectOhneUndMitUnlesbarerDatei(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	finding, err := Inspect("/p", "v1", DefaultInspector())
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if finding.Status != StatusAbsent {
		t.Errorf("Status = %s, erwartet %s", finding.Status, StatusAbsent)
	}
	if finding.Path == "" {
		t.Error("Pfad fehlt")
	}

	if err := os.WriteFile(finding.Path, []byte("{halb"), 0o600); err != nil {
		t.Fatalf("Datei schreiben: %v", err)
	}
	finding, err = Inspect("/p", "v1", DefaultInspector())
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if finding.Status != StatusOrphaned {
		t.Errorf("unlesbare Datei: Status = %s, erwartet %s", finding.Status, StatusOrphaned)
	}
}

// Die Identität des eigenen Prozesses passt zu seiner Startzeit, aber nicht
// zu einer anderen; ein beendeter Prozess passt gar nicht mehr.
func TestIdentityMatches(t *testing.T) {
	start := OwnStartTime()
	if !IdentityMatches(os.Getpid(), start) {
		t.Error("der eigene Prozess wird nicht erkannt")
	}
	if IdentityMatches(os.Getpid(), start.Add(-time.Hour)) {
		t.Error("eine Stunde Abweichung gilt als passend")
	}
	if IdentityMatches(0, start) {
		t.Error("PID 0 gilt als lebend")
	}

	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Skipf("Kindprozess: %v", err)
	}
	if IdentityMatches(cmd.Process.Pid, time.Now()) {
		t.Error("ein beendeter Prozess gilt als lebend")
	}
}

// Die Statustexte sind die Sprache der Meldungen.
func TestStatusString(t *testing.T) {
	for status, want := range map[Status]string{
		StatusAbsent:       "nicht vorhanden",
		StatusRunning:      "läuft unter dieser URL",
		StatusOtherVersion: "läuft mit anderer Version",
		StatusOrphaned:     "verwaist",
		StatusUnresponsive: "lebt ohne Antwort",
	} {
		if got := status.String(); got != want {
			t.Errorf("%d: %q, erwartet %q", int(status), got, want)
		}
	}
}
