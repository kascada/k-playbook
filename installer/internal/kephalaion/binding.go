// Package kephalaion prüft die Anbindung eines Projekts an seine
// Kephalaion-Collection über das MCP-Werkzeug whoami am Node dieses Rechners
// und ruft dort die Werkzeuge der Collection auf.
//
// Die Anbindung sind Hub-Alias und Collection aus dem Block kephalaion: der
// versionierten Projektkonfiguration (.k-playbook/K-PLAYBOOK.yaml, gelesen
// vom Paket project). Account, Token-Datei und Adresse des Nodes kommen allein
// aus der Auskunft kephalaion node client (clientinfo.go); eine eigene Suche
// nach config und Token gibt es nicht. Ein Token steht nie in einer Datei von
// k-playbook: Es bleibt in Kephalaions eigener Datei
// <tokensDir>/<hub>/<account>.token.
//
// Sicherheitsvertrag, für alle Aufrufer gleich (Kontextausgabe, Oberfläche):
// Hub und Collection kommen aus der Projektdatei oder aus einer Anfrage. Der
// Hub wird ein Argument der Auskunft und ein Header-Name, deshalb werden beide
// nach der Namensregel von Kephalaion geprüft. Gelesen wird ein Token nur aus
// der Token-Datei, die die Auskunft nennt, und nur, wenn sie genau
// <tokensDir>/<hub>/<account>.token ist — eine reguläre Datei, kein Symlink,
// in der Form eines Tokens. Die Collection wird nur mit der Antwort von
// whoami verglichen. Gesendet wird nur an die Loopback-Adresse des Nodes aus
// der Auskunft. Ein Token erscheint in keinem Ergebnis, keinem Fehler und
// keinem Log.
//
// Das Paket hängt nur von der Standardbibliothek und yamllite ab, damit
// project es für die Kontextausgabe einbinden kann.
package kephalaion

import (
	"fmt"
	"regexp"
	"strings"
)

// Binding ist die Anbindung eines Projekts: Hub-Alias und Collection aus dem
// Block kephalaion: von K-PLAYBOOK.yaml. Der Account gehört nicht dazu; er
// kommt von Kephalaion.
type Binding struct {
	Hub        string `json:"hub"`
	Collection string `json:"collection"`
}

// Address ist die Adresse der Collection für Clients: <hub>:<collection>.
func (b Binding) Address() string {
	if b.Hub == "" || b.Collection == "" {
		return ""
	}
	return b.Hub + ":" + b.Collection
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

// ValidateBinding prüft Hub und Collection nach der Namensregel. Der erste
// Verstoß wird gemeldet.
func ValidateBinding(b Binding) error {
	if err := CheckName("Hub", b.Hub); err != nil {
		return err
	}
	return CheckName("Collection", b.Collection)
}
