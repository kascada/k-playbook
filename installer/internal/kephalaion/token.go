package kephalaion

import (
	"errors"
	"os"
	"strings"
)

// tokenPrefix und tokenLength beschreiben die Form eines Tokens (ident.go in
// Kephalaion): keph_ und 32 Bytes base64url ohne Padding, also 43 Zeichen.
const (
	tokenPrefix = "keph_"
	tokenLength = len(tokenPrefix) + 43
)

// errTokenUnusable meldet eine Token-Datei, die sich nicht lesen lässt oder
// kein Token der erwarteten Form enthält. Der Fehler nennt weder Inhalt noch
// Länge.
var errTokenUnusable = errors.New("Token-Datei nicht lesbar oder ohne gültiges Token")

// readTokenFile liest ein Token aus einer Token-Datei: nur eine reguläre
// Datei, kein Symlink, höchstens 4 KiB. Das Token wird nur über die Form
// geprüft — keph_ und 43 Zeichen aus dem base64url-Alphabet —, damit kein
// Zeilenumbruch oder Steuerzeichen in einen Header gerät.
func readTokenFile(path string) (string, error) {
	if path == "" {
		return "", errTokenUnusable
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return "", errTokenUnusable
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", errTokenUnusable
	}
	token := strings.TrimSpace(string(data))
	if len(token) != tokenLength || !strings.HasPrefix(token, tokenPrefix) {
		return "", errTokenUnusable
	}
	for _, r := range token[len(tokenPrefix):] {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return "", errTokenUnusable
		}
	}
	return token, nil
}
