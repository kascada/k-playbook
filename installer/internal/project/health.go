package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Die Quellen der Umgebungsprüfung (Task 098). Jede Funktion ist eine Quelle
// für health.Check: Sie bewertet einen Befund, den die Kontextausgabe heute
// schon trägt, und macht daraus Einträge mit Stufe, Bereich, Satz und
// Abhilfe. Die Quellen stehen hier und nicht im Paket health, weil sie aus
// diesem Paket lesen (GHState, DetectBaseTools, readVersionSources,
// kephalaionCheck) und das Briefing in diesem Paket den Report trägt — health
// kann project darum nicht importieren. Eingehängt werden sie beim Laden des
// Pakets; jedes Programm, das das Briefing, die CLI, den MCP-Server oder die
// GUI baut, importiert project und hat sie damit.
//
// Die fünfte Quelle, github, wertet kein Feld der Kontextausgabe aus, sondern
// liest die persönliche GitHub-Zugangsdatei über das Paket githubaccess; sie
// steht hier neben den anderen, weil hier alle Quellen eingehängt werden und
// githubaccess project nicht kennt.
//
// Repariert wird nichts — außer dass die Quelle github die fehlende
// Zugangsdatei einmal als Vorlage anlegt, wie ihr Vertrag es will —, und kein
// Eintrag nennt einen Token oder einen Wert aus einer Datei; Dienst- und
// Schlüsselnamen dürfen vorkommen. Die Kontextausgabe bleibt unverändert;
// health ersetzt keine Felder, es bewertet sie.
func init() {
	health.Register(healthKephalaion)
	health.Register(healthGH)
	health.Register(healthBaseTools)
	health.Register(healthVersionSources)
	health.Register(healthGitHub)
}

// healthKephalaion bewertet die Anbindung an die Collection: eingetragen, aber
// nicht ok, ist ein Fehler — keph:-Verweise und die eigene Arbeit im Briefing
// sind dann nicht erreichbar; nicht eingetragen ist eine Warnung. Was
// kephalaion.Check am Node fragt, bleibt wie es ist; der Satz ist dessen
// Message, die nie ein Token trägt.
func healthKephalaion(_ context.Context, projectDir string) []health.Problem {
	status := kephalaionCheck(projectDir)
	if status.OK {
		return nil
	}
	if !status.Configured {
		return []health.Problem{{
			Level:   health.LevelWarning,
			Area:    health.AreaKephalaion,
			Message: status.Message,
			Hint: "Die Anbindung in der Oberfläche einrichten (k-playbook gui, Abschnitt Kephalaion); " +
				"ohne sie sind keph:-Verweise, Workbench und die eigene Arbeit im Briefing nicht erreichbar.",
		}}
	}
	return []health.Problem{{
		Level:   health.LevelError,
		Area:    health.AreaKephalaion,
		Message: status.Message,
		Hint:    kephalaionHint(status.State),
	}}
}

// kephalaionHint nennt je Zustand den nächsten Schritt.
func kephalaionHint(state kephalaion.State) string {
	switch state {
	case kephalaion.StateInvalidEntry:
		return "Den Block kephalaion: in " + LocalDirName + "/" + kephalaion.LocalFileName +
			" prüfen oder die Anbindung in der Oberfläche neu einrichten (k-playbook gui, Abschnitt Kephalaion)."
	case kephalaion.StateNoKephalaion, kephalaion.StateNodeAddress, kephalaion.StateNodeUnreachable:
		return "Kephalaion auf diesem Rechner prüfen: kephalaion status; der Node muss laufen und unter seiner Loopback-Adresse antworten."
	case kephalaion.StateTokenMissing, kephalaion.StateLoginInvalid, kephalaion.StateLoginMissing:
		return "Token-Datei und Anmeldung des Accounts prüfen (kephalaion status) oder in der Oberfläche einen anderen Account wählen (k-playbook gui, Abschnitt Kephalaion)."
	case kephalaion.StateHubUnknown, kephalaion.StateHubNeverSynced, kephalaion.StateReplicaUnreadable:
		return "Den Hub am Node prüfen und abgleichen: kephalaion status, kephalaion node sync."
	case kephalaion.StateCollectionUnreadable:
		return "Leserecht auf die Collection beantragen oder in der Oberfläche eine lesbare Collection wählen (k-playbook gui, Abschnitt Kephalaion)."
	}
	return "Die Anbindung in der Oberfläche prüfen (k-playbook gui, Abschnitt Kephalaion)."
}

// healthGH bewertet die GitHub CLI: Setzt das Projekt gh voraus (status
// enabled), aber die CLI ist nicht nutzbar, ist das eine Warnung — Commands
// wie /k-pr-review brechen dann ab. Ein unbekannter Wert in tools.gh.status
// ist ein Fehler: Er lässt auch die Kontextausgabe scheitern. Der Fehler aus
// GHState wird nicht angehängt: Er zitiert den Wert aus der Datei wörtlich,
// und der gehört in die Kontextausgabe, nicht in den Report. Der Satz nennt
// Datei, Feld und die erlaubten Werte.
func healthGH(_ context.Context, projectDir string) []health.Problem {
	gh, err := GHState(projectDir)
	if err != nil {
		message := "Das Feld tools.gh.status in " + ConfigFileName + " hat einen unbekannten Wert; erlaubt sind unknown, enabled und disabled."
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			message = "Die Datei " + ConfigFileName + " ist nicht lesbar; die Entscheidung zur GitHub CLI (tools.gh.status) fehlt damit."
		}
		return []health.Problem{{
			Level:   health.LevelError,
			Area:    health.AreaGH,
			Message: message,
			Hint:    "tools.gh.status in " + ConfigFileName + " auf unknown, enabled oder disabled setzen — oder in der Oberfläche wählen (k-playbook gui).",
		}}
	}
	if gh.Status != GHEnabled || gh.Ready {
		return nil
	}
	problem := health.Problem{Level: health.LevelWarning, Area: health.AreaGH}
	switch {
	case !gh.Installed:
		problem.Message = "Das Projekt setzt die GitHub CLI voraus, aber gh liegt nicht im PATH."
		problem.Hint = "gh installieren (https://cli.github.com) und mit gh auth login anmelden."
	default:
		problem.Message = "Das Projekt setzt die GitHub CLI voraus, aber für " + gh.Host + " ist kein Konto hinterlegt."
		problem.Hint = "gh auth login ausführen."
	}
	return []health.Problem{problem}
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
// eigenes Feld dafür hat. Eine leere Datei ergibt nichts; die einmalige
// Meldung der frisch angelegten Vorlage ist die einzige warning, alles
// andere error. Die Datei ist persönlich, nicht projektbezogen — projectDir
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
	problems := make([]health.Problem, 0, len(result.Problems))
	for _, problem := range result.Problems {
		problems = append(problems, health.Problem{
			Level:   health.Level(problem.Level),
			Area:    health.AreaGitHub,
			Message: problem.Path + ": " + problem.Message,
			Hint:    problem.Hint,
		})
	}
	return problems
}
