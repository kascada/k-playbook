package kephtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// FakeClient ist ein Ersatz für das Binary kephalaion: ein Skript, das die
// Auskunft kephalaion node client spielt. Es schreibt seine Argumente nach
// Args, gibt den Inhalt von stdout auf stdout und den von stderr auf stderr
// aus und endet mit dem Exit-Code aus exit — alles drei Dateien in Dir, die
// Set jederzeit ersetzt. Gebraucht wird /bin/sh.
type FakeClient struct {
	Dir  string
	Path string
}

// NewFakeClient legt das Skript an und lenkt kephalaion.LookupBinary darauf;
// nach dem Test gilt wieder die echte Suche. Ohne Set antwortet es mit einer
// leeren Ausgabe und Exit 0.
func NewFakeClient(t testing.TB) *FakeClient {
	t.Helper()
	dir := t.TempDir()
	fake := &FakeClient{Dir: dir, Path: filepath.Join(dir, "kephalaion")}
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(filepath.Join(dir, "args")) + "\n" +
		"cat " + shellQuote(filepath.Join(dir, "stdout")) + "\n" +
		"cat " + shellQuote(filepath.Join(dir, "stderr")) + " >&2\n" +
		"exit $(cat " + shellQuote(filepath.Join(dir, "exit")) + ")\n"
	if err := os.WriteFile(fake.Path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	fake.Set(t, "", "", 0)
	Binary(t, fake.Path)
	return fake
}

// Set legt fest, was der nächste Aufruf ausgibt und mit welchem Code er endet.
func (f *FakeClient) Set(t testing.TB, stdout, stderr string, exit int) {
	t.Helper()
	for name, content := range map[string]string{"stdout": stdout, "stderr": stderr, "exit": strconv.Itoa(exit)} {
		if err := os.WriteFile(filepath.Join(f.Dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// SetInfo gibt info als JSON aus, Exit 0.
func (f *FakeClient) SetInfo(t testing.TB, info kephalaion.ClientInfo) {
	t.Helper()
	f.Set(t, InfoJSON(t, info), "", 0)
}

// Args sind die Argumente des letzten Aufrufs; nil, wenn es keinen gab.
func (f *FakeClient) Args(t testing.TB) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.Dir, "args"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// Binary lenkt kephalaion.LookupBinary für die Dauer des Tests auf path.
func Binary(t testing.TB, path string) {
	t.Helper()
	original := kephalaion.LookupBinary
	kephalaion.LookupBinary = func() (string, bool) { return path, true }
	t.Cleanup(func() { kephalaion.LookupBinary = original })
}

// NoBinary lässt kephalaion.LookupBinary für die Dauer des Tests nichts
// finden.
func NoBinary(t testing.TB) {
	t.Helper()
	original := kephalaion.LookupBinary
	kephalaion.LookupBinary = func() (string, bool) { return "", false }
	t.Cleanup(func() { kephalaion.LookupBinary = original })
}

// Info baut eine Auskunft im Format 1: die config des Users, der Node unter
// nodeURL, das Token-Verzeichnis tokensDir und die Hubs.
func Info(nodeURL, tokensDir string, hubs ...kephalaion.ClientHub) kephalaion.ClientInfo {
	config := filepath.Join(filepath.Dir(tokensDir), "config.yaml")
	if hubs == nil {
		hubs = []kephalaion.ClientHub{}
	}
	return kephalaion.ClientInfo{
		Format: kephalaion.ClientFormat, Kephalaion: "v0.4.0",
		Config:    kephalaion.ClientConfig{Path: &config, Source: "user", Exists: true},
		Node:      kephalaion.ClientNode{URL: nodeURL, Source: "config"},
		TokensDir: tokensDir,
		Hubs:      hubs,
	}
}

// Hub ist der Eintrag eines Hubs mit genau einer Token-Datei für account
// (choice only).
func Hub(tokensDir, hub, account string) kephalaion.ClientHub {
	file := filepath.Join(tokensDir, hub, account+".token")
	return kephalaion.ClientHub{
		Alias: hub, Account: &account, Choice: kephalaion.ChoiceOnly, TokenFile: &file,
		Candidates: []string{account}, Pending: []string{}, Notes: []string{},
	}
}

// Token legt die Token-Datei <tokensDir>/<hub>/<account>.token mit token an
// (0600) und liefert tokensDir.
func Token(t testing.TB, tokensDir, hub, account, token string) string {
	t.Helper()
	path := filepath.Join(tokensDir, hub, account+".token")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return tokensDir
}

// InfoJSON ist info als JSON, wie kephalaion node client es ausgibt.
func InfoJSON(t testing.TB, info kephalaion.ClientInfo) string {
	t.Helper()
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + "\n"
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// IsolateBinary lässt kephalaion.LookupBinary nichts finden — für TestMain:
// Kein Test des Pakets fragt dann das echte kephalaion dieses Rechners, das
// mit dem echten Token-Verzeichnis und dem echten Node arbeitet; wer die
// Auskunft braucht, setzt ein Ersatz-Binary (NewFakeClient, Binary). Zurück
// kommt das Aufräumen.
func IsolateBinary() func() {
	original := kephalaion.LookupBinary
	kephalaion.LookupBinary = func() (string, bool) { return "", false }
	return func() { kephalaion.LookupBinary = original }
}
