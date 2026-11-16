package kephalaion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testToken hat die Form eines Kephalaion-Tokens und ist leicht wiederzufinden:
// Taucht diese Zeichenfolge in einem Ergebnis, einem Fehler oder einer Antwort
// auf, ist das Token durchgesickert.
const testToken = "keph_TOKENLEAKCANARYabcdefghijklmnopqrstuvw12345"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTestTokenHasTokenForm(t *testing.T) {
	if len(testToken) != tokenLength {
		t.Fatalf("Testtoken hat %d Zeichen, erwartet %d", len(testToken), tokenLength)
	}
}

func TestValidateBindingRejectsNamesOutsideTheRule(t *testing.T) {
	good := Binding{Hub: "vm", Collection: "k-playbook"}
	if err := ValidateBinding(good); err != nil || good.Address() != "vm:k-playbook" {
		t.Fatalf("gültig abgewiesen: %v", err)
	}
	cases := []Binding{
		{Hub: "../vm", Collection: "c"},
		{Hub: "vm/x", Collection: "c"},
		{Hub: "..", Collection: "c"},
		{Hub: "VM", Collection: "c"},
		{Hub: "-vm", Collection: "c"},
		{Hub: "vm", Collection: "a:b"},
		{Hub: "vm", Collection: "system-x"},
		{Hub: "vm", Collection: ""},
		{Hub: "vm", Collection: testToken},
	}
	for _, c := range cases {
		err := ValidateBinding(c)
		if err == nil {
			t.Errorf("%+v: Fehler erwartet", c)
			continue
		}
		if strings.Contains(err.Error(), testToken) {
			t.Errorf("Fehler zitiert einen unzulässigen Wert: %v", err)
		}
	}
	for _, account := range []string{"../../etc/passwd", "admin", "", testToken} {
		if err := checkAccountName(account); err == nil || strings.Contains(err.Error(), testToken) {
			t.Errorf("Account %q: %v", account, err)
		}
	}
}

func TestReadTokenChecksForm(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.token")
	writeFile(t, good, testToken+"\n")
	if token, err := readTokenFile(good); err != nil || token != testToken {
		t.Fatalf("gültig: err=%v", err)
	}
	for _, content := range []string{"", "keph_kurz", testToken + "\nX-Evil: 1", "abcd" + testToken[4:], strings.Replace(testToken, "a", "\r", 1)} {
		path := filepath.Join(dir, "bad.token")
		writeFile(t, path, content)
		_, err := readTokenFile(path)
		if err == nil {
			t.Errorf("%q: Fehler erwartet", content)
			continue
		}
		if strings.Contains(err.Error(), "keph_") {
			t.Errorf("Fehler nennt den Inhalt: %v", err)
		}
	}
}
