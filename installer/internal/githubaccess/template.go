package githubaccess

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Template ist der Inhalt der Vorlage, die Load bei fehlender Datei anlegt:
// services: {} und ein auskommentiertes Beispiel je Zugangsart, damit die
// Datei ohne den Vertrag ausgefüllt werden kann. Die Vorlage liest sich ohne
// Probleme; sie ist eine leere Datei.
const Template = `# GitHub-Zugänge für k-playbook — eine Datei je Person, für alle Projekte
# dieses Rechners. Hier stehen keine Geheimnisse, nur wo ein Zugang liegt:
# ein gh-Login, der Pfad einer Token-Datei, ein SSH-Schlüssel oder ein
# Host-Alias. Der Vertrag: docs/github-access.md in der k-playbook-Installation.
#
# Ein Dienst ist der kurze Name, den Skripte und KI-Sitzungen nennen; er
# zeigt auf ein Repository und auf die Zugänge in der Reihenfolge, in der
# sie versucht werden. Beispiel für jede Zugangsart:
#
# services:
#   website:
#     repo: acme/website                       # <owner>/<repo> auf github.com
#     access:
#       - type: gh                             # das Token dieses gh-Logins
#         login: alice
#       - type: token                          # ein Token aus einer Datei (Modus 0600)
#         file: ~/.config/github-tokens/acme.token
#         disabled: true                       # vorhanden, aber noch nicht nutzbar
#
#   backend:
#     repo: acme-labs/backend
#     access:
#       - type: ssh                            # dieser private Schlüssel, nur er
#         key: ~/.ssh/id_ed25519_acme_labs
#
#   playbook:
#     repo: example/k-playbook
#     access:
#       - type: ssh                            # ein Host-Alias aus ~/.ssh/config,
#         host: github-example                 # so richtet man einen Deploy Key ein
#       - type: public                         # ohne Anmeldung, nur lesen
#
# Pfade absolut oder mit ~/ — ein relativer Pfad hieße je Aufrufer eine andere Datei.

services: {}
`

// createTemplate legt die fehlende Datei als Vorlage an: das Verzeichnis mit
// 0700, die Datei mit 0600, nur wenn sie noch nicht da ist. Gelingt das, ist
// created wahr und problem die einmalige warning. Scheitert es — der
// erwartete Fall ist ein schreibgeschütztes Verzeichnis in einem Container —,
// ist problem ein error, der den Pfad nennt, der auf dem Host anzulegen ist.
// Ist die Datei inzwischen da (ein anderer Aufruf war schneller), sind beide
// leer; der Aufrufer liest sie dann wie eine vorhandene.
func createTemplate(path string) (created bool, problem *Problem) {
	hint := "Auf dem Host anlegen: " + path + " mit dem Inhalt services: {} (Verzeichnis 0700, Datei 0600); das Format steht in docs/github-access.md."
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, &Problem{Level: LevelError, Path: PathFile,
			Message: "Die Datei fehlt, und das Verzeichnis " + filepath.Dir(path) + " lässt sich nicht anlegen: " + describeReadError(err) + ".",
			Hint:    hint}
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, &Problem{Level: LevelError, Path: PathFile,
			Message: "Die Datei fehlt und lässt sich nicht anlegen: " + describeReadError(err) + ".",
			Hint:    hint}
	}
	_, writeErr := file.WriteString(Template)
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		_ = os.Remove(path)
		return false, &Problem{Level: LevelError, Path: PathFile,
			Message: "Die Datei fehlt, und die Vorlage lässt sich nicht schreiben: " + describeReadError(writeErr) + ".",
			Hint:    hint}
	}
	return true, &Problem{Level: LevelWarning, Path: PathFile,
		Message: "Die Datei wurde gerade angelegt und ist leer — trage deine Dienste ein.",
		Hint:    "Die Vorlage " + path + " ausfüllen; das Format steht in docs/github-access.md der k-playbook-Installation."}
}
