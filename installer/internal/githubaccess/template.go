package githubaccess

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Template ist der Inhalt der Vorlage, die Init anlegt (`k-playbook github
// init` und der Knopf der Oberfläche): services: {} und ein auskommentiertes
// Beispiel je Zugangsart, damit die Datei ohne den Vertrag ausgefüllt werden
// kann. Die Vorlage liest sich ohne Probleme; sie ist eine leere Datei.
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

// InitResult ist das Ergebnis von Init und das Objekt von `k-playbook github
// init --json`: der Pfad, ob die Vorlage eben angelegt wurde, und die
// Probleme — höchstens eines, immer ein error. Created falsch ohne Problem
// heißt: Die Datei lag schon da und ist unverändert. Problems ist nie nil.
type InitResult struct {
	File     string    `json:"file"`
	Created  bool      `json:"created"`
	Problems []Problem `json:"problems"`
}

// HasErrors sagt, ob Init gescheitert ist — dann endet init mit 1.
func (r InitResult) HasErrors() bool {
	return len(r.Problems) > 0
}

// Init legt die Zugangsdatei unter path als Vorlage an — der einzige Weg,
// auf dem k-playbook sie schreibt; `k-playbook github init` und der Knopf der
// Oberfläche rufen ihn, kein Leseweg. Liegt dort schon eine Datei, bleibt sie
// unverändert, ganz gleich, was darin steht. Ein belegter Pfad ohne lesbare
// Datei (ein symbolischer Link, dessen Ziel fehlt, oder ein Verzeichnis) ist
// ein error mit Abhilfe; ebenso ein Verzeichnis, in dem sich nichts anlegen
// lässt — der erwartete Fall ist ein schreibgeschützter Container, und die
// Abhilfe nennt, was auf dem Host anzulegen ist.
func Init(path string) InitResult {
	result := InitResult{File: path, Problems: []Problem{}}
	if _, err := os.Lstat(path); err == nil {
		info, statErr := os.Stat(path)
		switch {
		case statErr != nil:
			result.Problems = append(result.Problems, occupiedPathProblem(path))
		case info.IsDir():
			result.Problems = append(result.Problems, Problem{Level: LevelError, Path: PathFile,
				Message: "An der Stelle der Datei liegt ein Verzeichnis.",
				Hint:    "Das Verzeichnis " + path + " entfernen oder umbenennen; danach legt k-playbook github init die Vorlage an."})
		}
		return result
	}
	created, problem := createTemplate(path)
	result.Created = created
	if problem != nil {
		result.Problems = append(result.Problems, *problem)
	}
	return result
}

// createTemplate legt die fehlende Datei als Vorlage an: das Verzeichnis mit
// 0700, die Datei mit 0600, nur wenn sie noch nicht da ist. Gelingt das, ist
// created wahr. Scheitert es — der erwartete Fall ist ein schreibgeschütztes
// Verzeichnis in einem Container —, ist problem ein error, der den Pfad
// nennt, der auf dem Host anzulegen ist. Ist die Datei inzwischen da (ein
// anderer Aufruf war schneller), sind beide leer: Sie bleibt, wie sie ist.
func createTemplate(path string) (created bool, problem *Problem) {
	hint := "Auf dem Host anlegen: " + path + " mit dem Inhalt services: {} (Verzeichnis 0700, Datei 0600), etwa mit k-playbook github init dort; das Format steht in docs/github-access.md."
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, &Problem{Level: LevelError, Path: PathFile,
			Message: "Die Datei lässt sich nicht anlegen, denn das Verzeichnis " + filepath.Dir(path) + " lässt sich nicht anlegen: " + describeReadError(err) + ".",
			Hint:    hint}
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, &Problem{Level: LevelError, Path: PathFile,
			Message: "Die Datei lässt sich nicht anlegen: " + describeReadError(err) + ".",
			Hint:    hint}
	}
	_, writeErr := file.WriteString(Template)
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		_ = os.Remove(path)
		return false, &Problem{Level: LevelError, Path: PathFile,
			Message: "Die Vorlage lässt sich nicht schreiben: " + describeReadError(writeErr) + ".",
			Hint:    hint}
	}
	return true, nil
}
