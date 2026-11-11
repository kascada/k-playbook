package kephtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// FakeUpgrade ist ein Ersatz für das Binary kephalaion, der `upgrade --auto`
// und `upgrade --check` spielt: ein Skript, das je Aufruf seine Argumente an
// Calls anhängt und dann die Antwort für --auto bzw. --check ausgibt — stdout,
// stderr und Exit-Code aus Dateien in Dir, die SetAuto und SetCheck ersetzen.
// Ohne Antwort für eine Art endet es mit Exit 2 (falscher Aufruf), wie ein
// Kephalaion, das die Option nicht kennt. Mit Hang wartet --auto, bis ein
// Signal kommt. Gebraucht wird /bin/sh.
type FakeUpgrade struct {
	Dir  string
	Path string
}

// NewFakeUpgrade legt das Skript an und lenkt kephalaion.LookupBinary darauf;
// nach dem Test gilt wieder die echte Suche.
func NewFakeUpgrade(t testing.TB) *FakeUpgrade {
	t.Helper()
	dir := t.TempDir()
	fake := &FakeUpgrade{Dir: dir, Path: filepath.Join(dir, "kephalaion")}
	script := "#!/bin/sh\n" +
		"dir=" + shellQuote(dir) + "\n" +
		"printf '%s\\n' \"$@\" -- >> \"$dir/calls\"\n" +
		"kind=other\n" +
		"for arg in \"$@\"; do case \"$arg\" in --auto) kind=auto;; --check) kind=check;; esac; done\n" +
		"if [ \"$kind\" = auto ] && [ -f \"$dir/hang\" ]; then\n" +
		// Auf SIGTERM: festhalten, eine Weile weiterarbeiten (wie ein
		// Rückweg), dann festhalten, dass das Aufräumen fertig wurde.
		"  trap 'kill $! 2>/dev/null; echo term >> \"$dir/signals\"; sleep \"$(cat \"$dir/hang\")\"; echo done >> \"$dir/signals\"; exit 143' TERM\n" +
		"  sleep 60 >/dev/null 2>&1 &\n" +
		"  wait $!\n" +
		"  exit 0\n" +
		"fi\n" +
		"[ -f \"$dir/$kind.exit\" ] || exit 2\n" +
		"cat \"$dir/$kind.stdout\"\n" +
		"cat \"$dir/$kind.stderr\" >&2\n" +
		"exit \"$(cat \"$dir/$kind.exit\")\"\n"
	if err := os.WriteFile(fake.Path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	Binary(t, fake.Path)
	return fake
}

// SetAuto legt die Antwort auf `upgrade --auto` fest.
func (f *FakeUpgrade) SetAuto(t testing.TB, stdout, stderr string, exit int) {
	t.Helper()
	f.set(t, "auto", stdout, stderr, exit)
}

// SetCheck legt die Antwort auf `upgrade --check` fest.
func (f *FakeUpgrade) SetCheck(t testing.TB, stdout, stderr string, exit int) {
	t.Helper()
	f.set(t, "check", stdout, stderr, exit)
}

// Hang lässt `upgrade --auto` warten, bis ein Signal kommt; auf SIGTERM
// arbeitet es noch cleanup Sekunden weiter, bevor es endet.
func (f *FakeUpgrade) Hang(t testing.TB, cleanup float64) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.Dir, "hang"), []byte(strconv.FormatFloat(cleanup, 'f', -1, 64)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Signals sind die Zeilen, die der wartende Fake geschrieben hat: „term" beim
// Eingang von SIGTERM, „done", wenn das Aufräumen danach fertig wurde.
func (f *FakeUpgrade) Signals(t testing.TB) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.Dir, "signals"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(data))
}

// Calls sind die Argumente aller Aufrufe, in ihrer Reihenfolge.
func (f *FakeUpgrade) Calls(t testing.TB) [][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.Dir, "calls"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	calls := [][]string{}
	current := []string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if line == "--" {
			calls = append(calls, current)
			current = []string{}
			continue
		}
		current = append(current, line)
	}
	return calls
}

func (f *FakeUpgrade) set(t testing.TB, kind, stdout, stderr string, exit int) {
	t.Helper()
	for suffix, content := range map[string]string{"stdout": stdout, "stderr": stderr, "exit": strconv.Itoa(exit)} {
		if err := os.WriteFile(filepath.Join(f.Dir, kind+"."+suffix), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// JSON ist value als JSON mit Zeilenumbruch, wie kephalaion es ausgibt.
func JSON(t testing.TB, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + "\n"
}
