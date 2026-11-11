package project

import (
	"context"
	"fmt"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Die Quellen der Umgebungsprüfung (Task 098). Jede Funktion ist eine Quelle
// für health.Check: Sie bewertet einen Befund, den die Kontextausgabe heute
// schon trägt, und macht daraus Einträge mit Stufe, Bereich, Satz und
// Abhilfe. Die Quellen stehen hier und nicht im Paket health, weil sie aus
// diesem Paket lesen (DetectBaseTools, readVersionSources,
// kephalaionCheck) und das Briefing in diesem Paket den Report trägt — health
// kann project darum nicht importieren. Eingehängt werden sie beim Laden des
// Pakets; jedes Programm, das das Briefing, die CLI, den MCP-Server oder die
// GUI baut, importiert project und hat sie damit.
//
// Die vierte Quelle, github, wertet kein Feld der Kontextausgabe aus, sondern
// liest die persönliche GitHub-Zugangsdatei über das Paket githubaccess; sie
// steht hier neben den anderen, weil hier alle Quellen eingehängt werden und
// githubaccess project nicht kennt.
//
// Repariert wird nichts — auch die Quelle github legt eine fehlende
// Zugangsdatei nicht an; das tut nur `k-playbook github init` —, und kein
// Eintrag nennt einen Token oder einen Wert aus einer Datei; Dienst- und
// Schlüsselnamen dürfen vorkommen. Die Kontextausgabe bleibt unverändert;
// health ersetzt keine Felder, es bewertet sie.
func init() {
	health.Register(healthKephalaion)
	health.Register(healthBaseTools)
	health.Register(healthVersionSources)
	health.Register(healthGitHub)
}

// healthKephalaion bewertet die Anbindung an die Collection: Jede nicht
// bereite Anbindung ist ein Fehler, auch eine nicht eingetragene — ohne sie
// gibt es keine mitgelieferten Texte (k-playbook:, das Briefing; Task 131),
// keine keph:-Verweise und keine eigene Arbeit im Briefing. Was
// kephalaion.Check am Node fragt, bleibt wie es ist; der Satz ist dessen
// Message, die nie ein Token trägt. Nennt die Message ihre Abhilfe
// („Abhilfe: …“), wird sie dort geteilt: Satz davor, Abhilfe als Hint.
//
// Fehlt kephalaion an allen Orten der Suche, ist das die Abhilfe vor jeder
// anderen: Ohne Binary gibt es keine Auskunft und damit keine Anbindung.
//
// Dazu je eine Warnung, wenn im Block kephalaion: von K-PLAYBOOK.yaml ein
// account: steht — er wird nicht gelesen (Task 138), der Account kommt von
// Kephalaion — und wenn die alte k-playbook-local/K-PLAYBOOK.local.yaml noch
// einen Block kephalaion: trägt; den Umzug macht /k-upgrade.
func healthKephalaion(_ context.Context, projectDir string) []health.Problem {
	var problems []health.Problem
	if config, err := ReadConfig(projectDir); err == nil && config.Kephalaion.AccountSet {
		problems = append(problems, health.Problem{
			Level: health.LevelWarning,
			Area:  health.AreaKephalaion,
			Message: "Im Block kephalaion: von " + ConfigDisplayName + " steht ein account:; k-playbook liest ihn nicht, " +
				"der Account kommt von Kephalaion (kephalaion node client).",
			Hint: "Die Zeile account: aus dem Block entfernen und committen; welcher Account auf diesem Rechner gilt, " +
				"legt kephalaion node account default <hub> <account> fest.",
		})
	}
	if _, present, err := kephalaion.ReadLegacyBinding(LocalDir(projectDir)); err != nil || present {
		message := "In " + LegacyKephalaionDisplay + " steht noch ein Block kephalaion:; k-playbook liest die Anbindung " +
			"nicht mehr von dort, sondern aus dem Block kephalaion: von " + ConfigDisplayName + "."
		if err != nil {
			message = LegacyKephalaionDisplay + " liegt noch, ist aber nicht lesbar (" + err.Error() + "); k-playbook liest " +
				"die Anbindung nicht mehr von dort, sondern aus dem Block kephalaion: von " + ConfigDisplayName + "."
		}
		problems = append(problems, health.Problem{
			Level:   health.LevelWarning,
			Area:    health.AreaKephalaion,
			Message: message,
			Hint: "/k-upgrade ausführen: k-playbook upgrade zieht Hub und Collection in den Block um, danach löscht " +
				"/k-upgrade die alte Datei nach Rückfrage.",
		})
	}
	status := kephalaionCheck(projectDir)
	if status.OK {
		return problems
	}
	message, hint := splitRemedy(status.Message)
	if hint == "" || status.State == kephalaion.StateNotConfigured {
		// Nicht eingetragen: Die Abhilfe nennt dazu, was ohne Block fehlt.
		hint = kephalaionHint(status.State)
	}
	if _, binary := kephalaionBinary(); !binary {
		hint = kephalaionMissingHint
	}
	return append(problems, health.Problem{
		Level:   health.LevelError,
		Area:    health.AreaKephalaion,
		Message: message,
		Hint:    hint,
	})
}

// splitRemedy teilt eine Meldung an ihrer ersten „Abhilfe: “ in Satz und
// Abhilfe; ohne sie bleibt die Abhilfe leer.
func splitRemedy(message string) (sentence, remedy string) {
	before, after, found := strings.Cut(message, "Abhilfe: ")
	if !found {
		return message, ""
	}
	return strings.TrimSpace(before), strings.TrimSpace(after)
}

// kephalaionMissingHint ist die Abhilfe, wenn kephalaion nirgends liegt.
const kephalaionMissingHint = "Kephalaion ist auf diesem Rechner nicht installiert (kein " + kephalaion.BinaryName +
	" im PATH, unter ~/.local/bin oder unter /usr/local/bin). Installieren mit " + InstalledCommandName + " upgrade oder " +
	KephalaionInstallCommand + "; danach die Anbindung prüfen (kephalaion status, k-playbook context)."

// kephalaionHint nennt je Zustand den nächsten Schritt, wenn die Meldung
// keine eigene Abhilfe trägt.
func kephalaionHint(state kephalaion.State) string {
	switch state {
	case kephalaion.StateNotConfigured:
		return "Den Block kephalaion: mit hub und collection in " + ConfigDisplayName +
			" eintragen und committen oder in der Oberfläche einrichten (k-playbook gui, Abschnitt Kephalaion); " +
			"ohne ihn gibt es keine mitgelieferten Texte (k-playbook:, das Briefing), und keph:-Verweise, " +
			"Workbench und die eigene Arbeit sind nicht erreichbar."
	case kephalaion.StateInvalidEntry:
		return "Den Block kephalaion: in " + ConfigDisplayName +
			" prüfen oder die Anbindung in der Oberfläche neu einrichten (k-playbook gui, Abschnitt Kephalaion)."
	case kephalaion.StateNoKephalaion:
		return "Kephalaion aktualisieren: kephalaion upgrade; k-playbook braucht kephalaion node client im Format 1."
	case kephalaion.StateClientFailed:
		return "Die Auskunft von Hand prüfen: kephalaion node client --json --format 1 --no-probe."
	case kephalaion.StateNodeAddress, kephalaion.StateNodeUnreachable:
		return "Kephalaion auf diesem Rechner prüfen: kephalaion status; der Node muss laufen und unter seiner Loopback-Adresse antworten."
	case kephalaion.StateAccountChoiceMissing:
		return "Einen Standard-Account setzen: kephalaion node account default <hub> <account>; im DevContainer auf dem Host."
	case kephalaion.StateTokenMissing, kephalaion.StateLoginInvalid, kephalaion.StateLoginMissing:
		return "Token-Datei und Anmeldung des Accounts prüfen: kephalaion node account list, kephalaion status."
	case kephalaion.StateHubUnknown, kephalaion.StateHubNeverSynced, kephalaion.StateReplicaUnreadable:
		return "Den Hub am Node prüfen und abgleichen: kephalaion status, kephalaion node sync."
	case kephalaion.StateCollectionUnreadable:
		return "Leserecht auf die Collection beantragen oder in der Oberfläche eine lesbare Collection wählen (k-playbook gui, Abschnitt Kephalaion)."
	}
	return "Die Anbindung in der Oberfläche prüfen (k-playbook gui, Abschnitt Kephalaion)."
}

// healthBaseTools bewertet die Basis-Werkzeuge: je fehlendem eine Warnung mit
// dem Installationsbefehl als Abhilfe. Der bekannte Fehlalarm bei rg unter
// Claude Code (Shell-Funktion statt PATH) bleibt, wie am Typ BaseTools
// begründet.
func healthBaseTools(_ context.Context, projectDir string) []health.Problem {
	tools := DetectBaseTools(projectDir)
	problems := make([]health.Problem, 0, len(tools.Missing))
	for _, tool := range tools.Missing {
		problems = append(problems, health.Problem{
			Level:   health.LevelWarning,
			Area:    health.AreaBaseTools,
			Message: fmt.Sprintf("Das Basis-Werkzeug %s fehlt im PATH; k-playbook braucht es für: %s.", tool.Name, tool.Role),
			Hint:    "Installieren mit: " + tools.InstallCommand,
		})
	}
	return problems
}

// healthVersionSources bewertet die Quellenkonfiguration des
// Versionsinventars: Ist die Datei da, aber nicht lesbar oder von unbekannter
// Fassung, ist das ein Fehler — der Erhebungslauf bräche daran ab. Der Satz
// nennt den Pfad, nicht den Inhalt: Die Einzelheiten des Lesers stehen in
// der Kontextausgabe unter versionSources.error, die eine Zeile der Datei
// zitieren kann.
func healthVersionSources(_ context.Context, projectDir string) []health.Problem {
	state := readVersionSources(LocalDir(projectDir))
	if state.Error == "" {
		return nil
	}
	return []health.Problem{{
		Level:   health.LevelError,
		Area:    health.AreaVersionSources,
		Message: "Die Quellenkonfiguration des Versionsinventars " + state.Path + " ist nicht lesbar oder von unbekannter Fassung.",
		Hint: "Die Datei korrigieren oder entfernen; die Einzelheiten nennt k-playbook context unter versionSources.error, " +
			"den Vertrag docs/version-inventory.md der Installation.",
	}}
}

// healthGitHub trägt die Probleme der persönlichen GitHub-Zugangsdatei
// (docs/github-access.md) ein: jedes Problem des Lesers als Eintrag mit
// seiner Stufe, der Pfad in der Datei (file, services.<name>,
// services.<name>.access[<i>]) vorn im Satz, weil ein health.Problem kein
// eigenes Feld dafür hat. Eine fehlende Datei ergibt nichts — sie ist
// freiwillig, und die Quelle legt sie nicht an —, eine leere auch; jedes
// Problem der Datei ist ein error. Liest sich die Datei, beurteilt die Quelle dazu die
// Verfügbarkeit jedes Zugangs ohne Netz (githubaccess.Judge): Ein Dienst, in
// dem kein Zugang verfügbar ist, wird eine warning — die Datei trägt dann
// nichts, was sich nutzen ließe. Weiter als das geht die Quelle nicht; GitHub
// fragt sie nie. Die Datei ist persönlich, nicht projektbezogen — projectDir
// spielt keine Rolle.
func healthGitHub(_ context.Context, _ string) []health.Problem {
	path, err := githubaccess.Path()
	if err != nil {
		return []health.Problem{{
			Level:   health.LevelError,
			Area:    health.AreaGitHub,
			Message: "Der Ort der GitHub-Zugangsdatei ist unbekannt: " + err.Error() + ".",
			Hint:    "HOME oder XDG_CONFIG_HOME setzen oder den Pfad in " + githubaccess.EnvConfig + " nennen.",
		}}
	}
	result := githubaccess.Load(path)
	if !result.Present {
		return nil
	}
	problems := make([]health.Problem, 0, len(result.Problems))
	for _, problem := range result.Problems {
		problems = append(problems, health.Problem{
			Level:   health.Level(problem.Level),
			Area:    health.AreaGitHub,
			Message: problem.Path + ": " + problem.Message,
			Hint:    problem.Hint,
		})
	}
	if result.HasErrors() {
		return problems
	}
	for _, service := range githubaccess.Judge(result.Services) {
		if githubaccess.Usable(service) {
			continue
		}
		problems = append(problems, health.Problem{
			Level:   health.LevelWarning,
			Area:    health.AreaGitHub,
			Message: "services." + service.Name + ": kein verfügbarer Zugang — " + githubaccess.FirstReason(service),
			Hint:    "Einen Zugang des Dienstes " + service.Name + " in " + path + " einrichten; k-playbook github check nennt je Zugang den Grund.",
		})
	}
	return problems
}
