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
