// Package kephalaion liest die Anbindung eines Projekts an eine
// Kephalaion-Collection, prüft sie über das MCP-Werkzeug whoami am Node dieses
// Rechners und schreibt sie nach bestandenem Test.
//
// Die Anbindung steht in k-playbook-local/K-PLAYBOOK.local.yaml, Block
// kephalaion: — Hub-Alias, Collection und Account. Die Datei ist rechnerlokal
// und nicht versioniert. Ein Token steht nie darin: Es bleibt in Kephalaions
// eigener Datei tokens/<hub>/<account>.token neben der config des Users.
//
// Sicherheitsvertrag, für alle Aufrufer gleich (Kontextausgabe, Oberfläche):
// Hub und Account kommen aus der Projektdatei oder aus einer Anfrage und
// fließen in einen Dateipfad und in Header. Deshalb werden sie nach der
// Namensregel von Kephalaion geprüft und müssen genau einem Kandidaten aus
// dem Token-Verzeichnis entsprechen; gelesen wird der Pfad des Kandidaten, nie
// ein aus den rohen Werten gebauter. Die Collection wird nach der Namensregel
// geprüft und nur mit der Antwort von whoami verglichen. Gesendet wird nur an
// die Loopback-Adresse des Nodes. Ein Token erscheint in keinem Ergebnis,
// keinem Fehler und keinem Log.
//
// Das Paket hängt nur von der Standardbibliothek und yamllite ab, damit
// project es für die Kontextausgabe einbinden kann.
package kephalaion

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// LocalFileName ist die rechnerlokale Ergänzung zu K-PLAYBOOK.yaml unter
// k-playbook-local/.
const LocalFileName = "K-PLAYBOOK.local.yaml"

// BlockName ist der Block in LocalFileName, den dieses Paket liest und
// schreibt; alles andere in der Datei bleibt unberührt.
const BlockName = "kephalaion"

// Binding ist der Inhalt des Blocks kephalaion: aus LocalFileName.
type Binding struct {
	Hub        string `json:"hub"`
	Collection string `json:"collection"`
	Account    string `json:"account"`
}

// Address ist die Adresse der Collection für Clients: <hub>:<collection>.
func (b Binding) Address() string {
	if b.Hub == "" || b.Collection == "" {
		return ""
	}
	return b.Hub + ":" + b.Collection
}

// LocalFilePath ist der Ort von LocalFileName unter localDir.
func LocalFilePath(localDir string) string {
	return filepath.Join(localDir, LocalFileName)
}

// ReadBinding liest den Block kephalaion: aus LocalFileName unter localDir.
//
// configured ist false, wenn die Datei oder der Block fehlt oder der Block
// keinen der drei Werte trägt — das ist ein Zustand, kein Fehler. Ein Fehler
// heißt: Die Datei ist da, lässt sich aber nicht lesen oder nicht als YAML
// verstehen. Die Namen prüft ReadBinding nicht; das macht ValidateBinding.
func ReadBinding(localDir string) (binding Binding, configured bool, err error) {
	data, err := os.ReadFile(LocalFilePath(localDir))
	if errors.Is(err, fs.ErrNotExist) {
		return Binding{}, false, nil
	}
	if err != nil {
		return Binding{}, false, fmt.Errorf("%s nicht lesbar: %w", LocalFileName, err)
	}
	root, err := yamllite.Parse(data)
	if err != nil {
		return Binding{}, false, fmt.Errorf("%s ist kein lesbares YAML (%s)", LocalFileName, describeYAMLError(err))
	}
	block := root.Get(BlockName)
	if block == nil {
		return Binding{}, false, nil
	}
	binding = Binding{
		Hub:        strings.TrimSpace(block.Get("hub").Str()),
		Collection: strings.TrimSpace(block.Get("collection").Str()),
		Account:    strings.TrimSpace(block.Get("account").Str()),
	}
	if binding == (Binding{}) {
		return Binding{}, false, nil
	}
	return binding, true, nil
}

// yamlLinePattern liest die Zeilennummer vom Anfang eines Fehlers von
// yamllite.Parse; jeder Hinweis dort beginnt mit „Zeile <n>: “.
var yamlLinePattern = regexp.MustCompile(`^Zeile ([0-9]+): `)

// yamlProblemKinds sind die festen Anfänge der Hinweise von yamllite und ihr
// Satz für eine Meldung — ohne den Teil, der Text aus der Datei zitiert.
var yamlProblemKinds = []struct{ prefix, kind string }{
	{"Tabulator in der Einrückung", "Tabulator in der Einrückung"},
	{"Einrückung passt zu keiner offenen Ebene", "Einrückung passt zu keiner offenen Ebene"},
	{"weder Schlüssel noch Listeneintrag", "weder Schlüssel noch Listeneintrag"},
	{"nicht geschlossene Flow-Liste", "nicht geschlossene Flow-Liste"},
	{"nicht geschlossene Flow-Abbildung", "nicht geschlossene Flow-Abbildung"},
	{"überzähliges Zeichen", "überzähliges Zeichen nach einem Flow-Wert"},
	{"Schlüssel ", "Schlüssel ohne Doppelpunkt in einer Flow-Abbildung"},
}

// describeYAMLError beschreibt einen Fehler von yamllite.Parse nur mit
// Zeilennummer und Art des ersten Hinweises. Der Fehler selbst zitiert Text
// aus der Datei — steht dort versehentlich ein Token auf einer eigenen Zeile,
// landete es sonst in einer Meldung. Deshalb wird aus dem Fehlertext nur
// gelesen, was vor jedem zitierten Inhalt steht.
func describeYAMLError(err error) string {
	text := err.Error()
	match := yamlLinePattern.FindStringSubmatch(text)
	if match == nil {
		return "Aufbau nicht lesbar; der Inhalt wird nicht zitiert"
	}
	rest := text[len(match[0]):]
	kind := "Aufbau nicht lesbar"
	for _, candidate := range yamlProblemKinds {
		if strings.HasPrefix(rest, candidate.prefix) {
			kind = candidate.kind
			break
		}
	}
	return "Zeile " + match[1] + ": " + kind + "; der Inhalt wird nicht zitiert"
}

// namePattern ist die Namensregel von Kephalaion (internal/ident/ident.go
// dort): klein, beginnt mit Buchstabe oder Ziffer, höchstens 63 Zeichen, kein
// Doppelpunkt, kein Schrägstrich, kein „..“ am Anfang.
var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// reservedPrefix darf kein Name tragen, in keiner Schreibweise.
const reservedPrefix = "system"

// reservedAccount ist als Account-Name reserviert (der Verwalter am Hub).
const reservedAccount = "admin"

// CheckName prüft einen Namen gegen die Namensregel. what nennt die Art im
// Fehler. Der Fehler zitiert den Namen nur, wenn er druckbar und kurz ist —
// sonst landete ein versehentlich eingetragenes Token in einer Meldung.
func CheckName(what, name string) error {
	if name == "" {
		return fmt.Errorf("%s fehlt", what)
	}
	if strings.HasPrefix(strings.ToLower(name), reservedPrefix) {
		return fmt.Errorf("%s %s: Namen mit dem Präfix %q sind reserviert", what, quoteName(name), reservedPrefix)
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("%s %s: ungültiger Name; erlaubt sind a–z, 0–9, '.', '_' und '-', am Anfang a–z oder 0–9, höchstens 63 Zeichen", what, quoteName(name))
	}
	return nil
}

// checkAccountName prüft einen Account-Namen: die Namensregel und dazu der
// reservierte Name des Verwalters.
func checkAccountName(name string) error {
	if err := CheckName("Account", name); err != nil {
		return err
	}
	if name == reservedAccount {
		return fmt.Errorf("Account %q: der Name ist reserviert", name)
	}
	return nil
}

// quoteName zitiert einen Namen für eine Meldung, wenn er der Namensregel
// folgt; sonst steht dort nur, dass er unzulässig ist.
func quoteName(name string) string {
	if namePattern.MatchString(name) {
		return fmt.Sprintf("%q", name)
	}
	return "(unzulässiger Wert)"
}

// ValidateBinding prüft alle drei Namen nach der Namensregel. Der erste Verstoß
// wird gemeldet.
func ValidateBinding(b Binding) error {
	if err := CheckName("Hub", b.Hub); err != nil {
		return err
	}
	if err := CheckName("Collection", b.Collection); err != nil {
		return err
	}
	return checkAccountName(b.Account)
}
