package githubaccess

import (
	"fmt"
	"strings"
)

// Die Auswahl für `k-playbook github run` (docs/github-access.md, Abschnitt
// Running a command under a service, Choosing the accesses): zu einem Dienst
// je ein Zugang für die beiden Kanäle git und api, ohne Netz, aus der
// Verfügbarkeit (Judge) und in der Reihenfolge der Datei. Es gibt keinen
// Rückfall: run startet den Befehl genau einmal mit dieser Wahl.

// Channel benennt einen der beiden Kanäle von run.
type Channel string

const (
	// ChannelGit ist der Transport von git: clone, fetch, pull, push, ls-remote.
	ChannelGit Channel = "git"
	// ChannelAPI ist gh und alles andere, was GH_TOKEN/GITHUB_TOKEN liest.
	ChannelAPI Channel = "api"
)

// ChooseOptions engt die Wahl ein; jede Option gilt für beide Kanäle.
type ChooseOptions struct {
	// Write überspringt public, den einzigen Nur-Lese-Zugang.
	Write bool
	// Use lässt nur Zugänge dieses Typs zu; leer heißt jeder Typ.
	Use string
	// Skip lässt Zugänge dieser Typen aus.
	Skip []string
}

// Choice ist die Wahl für einen Dienst: je Kanal höchstens ein Zugang. Ein
// leerer Kanal ist kein Fehler; sind beide leer, startet run nicht.
type Choice struct {
	Git *Access
	API *Access
}

// Empty sagt, ob beide Kanäle leer sind.
func (c Choice) Empty() bool {
	return c.Git == nil && c.API == nil
}

// SameAccess sagt, ob beide Kanäle denselben Zugang tragen — das ist so,
// sobald git einen Zugang vom Typ token oder gh gewählt hat: Der erste
// verfügbare ist dann auch der erste verfügbare mit Token.
func (c Choice) SameAccess() bool {
	return c.Git != nil && c.API != nil && c.Git == c.API
}

// ValidType sagt, ob typ einer der vier Zugangstypen ist — für --use und
// --skip.
func ValidType(typ string) bool {
	_, ok := accessKeys[typ]
	return ok
}

// AccessTypes nennt die erlaubten Typen in einem Satzteil („gh, token, ssh
// und public“), für Meldungen.
const AccessTypes = accessTypes

// Choose wählt zu einem beurteilten Dienst (Judge ist gelaufen) je Kanal den
// ersten Zugang in Dateireihenfolge, der available ist und den Optionen
// genügt: git jeden Typ, api nur token oder gh. Die Zeiger zeigen in
// service.Access. Choose startet keinen Prozess und fragt kein Netz.
func Choose(service Service, options ChooseOptions) Choice {
	var choice Choice
	for index := range service.Access {
		access := &service.Access[index]
		if !admitted(*access, options) {
			continue
		}
		if choice.Git == nil {
			choice.Git = access
		}
		if choice.API == nil && (access.Type == "token" || access.Type == "gh") {
			choice.API = access
		}
	}
	return choice
}

// admitted sagt, ob ein Zugang unter den Optionen gewählt werden darf.
func admitted(access Access, options ChooseOptions) bool {
	if access.State != StateAvailable {
		return false
	}
	if options.Write && access.Type == "public" {
		return false
	}
	if options.Use != "" && access.Type != options.Use {
		return false
	}
	for _, skip := range options.Skip {
		if access.Type == skip {
			return false
		}
	}
	return true
}

// NoAccessMessage beschreibt, warum für einen Dienst kein Kanal einen Zugang
// hat: je Zugang ein Halbsatz mit seinem Zustand oder der Option, die ihn
// ausschließt. Für die Meldung von run vor dem Exit-Code 125.
func NoAccessMessage(service Service, options ChooseOptions) string {
	var parts []string
	for index, access := range service.Access {
		var why string
		switch {
		case access.State == StateDisabled:
			why = "deaktiviert"
		case access.State == StateUnavailable:
			why = "nicht verfügbar — " + access.Reason
		case access.State != StateAvailable:
			why = "nicht beurteilt"
		case options.Write && access.Type == "public":
			why = "public, durch --write ausgeschlossen"
		case options.Use != "" && access.Type != options.Use:
			why = "durch --use " + options.Use + " ausgeschlossen"
		default:
			why = "durch --skip " + access.Type + " ausgeschlossen"
		}
		parts = append(parts, fmt.Sprintf("access[%d] %s: %s", index, access.Type, why))
	}
	if len(parts) == 0 {
		return "kein Zugang eingetragen"
	}
	return strings.Join(parts, "; ")
}

// Selection ist die Wahl von run für einen Dienst als Indizes in seinem
// access (docs/github-access.md, Abschnitt JSON, Feld selected): Read ist
// run ohne Optionen, Write ist run --write. Sie kommt aus Choose und aus
// nichts sonst; Kombinationen mit --use und --skip nennt sie nicht.
type Selection struct {
	Read  ChannelIndex `json:"read"`
	Write ChannelIndex `json:"write"`
}

// ChannelIndex ist je Kanal der Index des gewählten Zugangs in access; nil
// (im JSON null) heißt, der Kanal hat keinen Zugang.
type ChannelIndex struct {
	Git *int `json:"git"`
	API *int `json:"api"`
}

// Empty sagt, ob beide Kanäle leer sind — genau dann endet run in dieser
// Form mit „kein verwendbarer Zugang“.
func (c ChannelIndex) Empty() bool {
	return c.Git == nil && c.API == nil
}

// Equal sagt, ob beide Kanäle dieselben Zugänge nennen.
func (c ChannelIndex) Equal(other ChannelIndex) bool {
	same := func(a, b *int) bool {
		return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
	}
	return same(c.Git, other.Git) && same(c.API, other.API)
}

// Choice macht aus den Indizes wieder eine Wahl mit Zeigern in accesses —
// für die Textzeilen, die RunLine mit run teilen.
func (c ChannelIndex) Choice(accesses []Access) Choice {
	at := func(index *int) *Access {
		if index == nil || *index < 0 || *index >= len(accesses) {
			return nil
		}
		return &accesses[*index]
	}
	return Choice{Git: at(c.Git), API: at(c.API)}
}

// Select ist die Wahl von run zu einem beurteilten Dienst: Choose ohne
// Optionen und mit Write, die Zeiger übersetzt in ihre Position in
// service.Access. Eine zweite Regel gibt es nicht.
func Select(service Service) Selection {
	return Selection{
		Read:  indexOf(service, Choose(service, ChooseOptions{})),
		Write: indexOf(service, Choose(service, ChooseOptions{Write: true})),
	}
}

// indexOf übersetzt die Zeiger einer Wahl in Indizes von service.Access.
func indexOf(service Service, choice Choice) ChannelIndex {
	find := func(access *Access) *int {
		if access == nil {
			return nil
		}
		for index := range service.Access {
			if &service.Access[index] == access {
				return &index
			}
		}
		return nil
	}
	return ChannelIndex{Git: find(choice.Git), API: find(choice.API)}
}

// SelectionOK sagt je Kanal von Selection, ob der gewählte Zugang den Test
// bestanden hat: true bei ok, false bei failed oder skipped, nil, wo
// Selection nil ist. Nur im Report von test.
type SelectionOK struct {
	Read  ChannelOK `json:"read"`
	Write ChannelOK `json:"write"`
}

// ChannelOK ist das Testergebnis je Kanal.
type ChannelOK struct {
	Git *bool `json:"git"`
	API *bool `json:"api"`
}
