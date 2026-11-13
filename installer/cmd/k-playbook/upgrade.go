package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/program"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/version"
)

// k-playbook upgrade — der eine Nachzug für Kephalaion, Programm und Projekt,
// den /k-upgrade aufruft.
//
// Drei Schritte, je Schritt eine Meldung, in dieser Reihenfolge:
//
// (0) Kephalaion — zuerst, weil k-playbook von Kephalaion abhängt und nicht
// umgekehrt: neue k-playbook-Releases setzen neue Kephalaion-Fähigkeiten
// voraus, Projektschritt und /k-upgrade brauchen den Node, und ein Neustart
// des Nodes ist so am Anfang erledigt und von Kephalaion selbst geprüft. Fehlt
// kephalaion, lädt der Schritt das install.sh des neuesten Kephalaion-Releases
// und führt es aus (program.InstallKephalaion) — ohne Clone, das Skript
// bin/install-kephalaion ruft dieses Programm nicht mehr auf. Liegt es da, ruft
// der Schritt `kephalaion upgrade --auto
// --json` auf und meldet allein nach dessen Antwort; ob ein Upgrade gefahrlos
// ist (Hub, globale Installation, dev build, zu alter Hub, DevContainer),
// entscheidet Kephalaion. Kennt das Binary --auto nicht (Exit 2), meldet der
// Schritt den Report von `upgrade --check --json`. Bei `upgraded` oder einem
// Neustart des Nodes verlangt der Lauf den Neustart der Sitzung. Der Schritt
// läuft im aufrufenden Prozess, vor dem Pull; das neue Programm nach einem
// Binärwechsel fasst Kephalaion nicht noch einmal an.
//
// (1) das Programm — zuerst `git pull` im Clone, solange es ihn gibt: Er dient
// nur noch den Inhalten, die am Clone hängen, und entscheidet nicht mehr über
// Version und Installation. Danach der Vergleich der eigenen Version mit dem
// neuesten Release-Tag auf GitHub (program.CheckStatus) und — nur bei „älter"
// — das Release-Binary dieses Tags, geprüft gegen die SHA256SUMS des Tags
// (program.Install über release.Install). Nie „ungleich", sonst stufte ein
// Build über dem neuesten Tag sich herab. Vergleich und Installation laufen,
// ob es den Clone gibt oder nicht und ob der Pull gelingt oder nicht; ein
// fehlender Clone und ein gescheiterter Pull werden gemeldet. Ohne eigene
// Version wird der Tag nicht abgerufen; ist er unbekannt (ohne Netz, ohne git,
// Zeitüberschreitung), sagt die Meldung das, und das Projekt wird trotzdem
// nachgezogen.
//
// (2) das Projekt — zuerst der Umzug der Anbindung aus der alten
// k-playbook-local/K-PLAYBOOK.local.yaml in den Block kephalaion: von
// K-PLAYBOOK.yaml (project.MigrateKephalaionBinding, Task 138), dann
// ApplyAssistantSetup, also
// Einordnen von CLAUDE.md/AGENTS.md, Anstoß ersetzen und Zusatzblöcke
// entfernen, Verlinkung; danach immer die MCP-Einträge
// (project.RepairMCPReport im Umfang des Starts,
// MCPWriteOutdatedAndUnversioned) — die Reparatur hängt nicht mehr am Pull.
// Am Ende der Hinweis auf den Neustart der Sitzung, wenn das Binary
// gewechselt hat — der MCP-Server der Sitzung behält das alte Programm — und
// ein eigener, wenn Kephalaion aktualisiert oder sein Node neu gestartet
// wurde.
//
// Hat Schritt (1) das Binary ersetzt, führt dieser — noch alte — Prozess
// Schritt (2) nicht selbst aus: er hätte alte Vorlagen und alte Erkennung.
// Er startet dafür das neu installierte Programm mit `upgrade --project-only`
// (reexecProjectStep). Die Oberfläche braucht das nicht, weil sie neu startet;
// upgrade hat keinen Neustart. Den Schalter `--repair-mcp` nimmt
// `--project-only` weiter an, er entscheidet aber nichts mehr: Ein Binary von
// vor Task 149 gab damit mit, dass sein Pull entfiel, und startet das neu
// installierte Programm noch so.
//
// `--check` schreibt nichts: es meldet den Kephalaion-Stand (nur
// `upgrade --check --json`, nie --auto), den Stand des Clones gegenüber seinem
// Remote (CheckUpdate), den Programm-Stand gegenüber dem neuesten Release-Tag
// — der Tag ist auch hier bekannt — und baut die Projektprüfung aus den
// Check-Funktionen — es meldet, was ein Lauf täte. Bei den MCP-Einträgen sagt
// es weniger, als ein Lauf täte: gemeldet werden nur veraltete Einträge, nicht
// die fehlenden, die ein Lauf in einer nicht von git erfassten Datei ergänzt.
//
// Hinweis auf /k-upgrade: Hat ein Lauf das Binary ersetzt, sagt die letzte
// Zeile, dass /k-upgrade in einer Sitzung die Umstellungen nachzieht, die eine
// Beurteilung brauchen. `--check` sagt in einer eigenen Vorschauzeile, dass
// ein Lauf Neues holt. Beide entfallen, wenn
// K_PLAYBOOK_VIA_COMMAND=1 gesetzt ist: so ruft /k-upgrade selbst das
// Programm auf (Hülle und Ersatzaufruf in der Shell), und dort stünde der
// Hinweis mitten im Command. Eine Variable statt eines Schalters, weil ein
// altes Binary einen unbekannten Schalter abweist und damit die Einbettung
// der Hülle scheitern ließe; die Variable übergeht es. Der Hinweis kommt nur
// vom aufrufenden Prozess, nie von `--project-only`.

// upgradeSchemaVersion ist die Fassung der JSON-Antwort. Fassung 2 (Task 149):
// program.status trägt latest statt clone (dazu latestError),
// program.versionChanged und program.mcpRepaired entfallen, project trägt
// mcpRepairedPortable und mcpMessage.
const upgradeSchemaVersion = "2"

// viaCommandEnv ist die Umgebungsvariable, die Hülle und Ersatzaufruf von
// /k-upgrade auf 1 setzen.
const viaCommandEnv = "K_PLAYBOOK_VIA_COMMAND"

// Austauschbar, damit die Abfolge ohne Netz, Clone und echtes Binary prüfbar
// ist: der Pull, die Installation des Release-Binaries und der Aufruf des neu
// installierten Programms.
var (
	upgradeUpdate  = project.Update
	upgradeInstall = program.Install
	upgradeReexec  = reexecProjectStep
	// Der Remote-Stand des Clones bei --check.
	upgradeCheckUpdate = project.CheckUpdate
	// upgradeViaCommand sagt, ob /k-upgrade den Lauf ausführt; die eine
	// Stelle, an der K_PLAYBOOK_VIA_COMMAND gelesen wird.
	upgradeViaCommand = func() bool { return os.Getenv(viaCommandEnv) == "1" }
	// Die Suche nach kephalaion, seine Installation und die Aufrufe von
	// kephalaion upgrade --auto bzw. --check.
	upgradeFindKephalaion    = kephalaion.FindBinary
	upgradeInstallKephalaion = program.InstallKephalaion
	upgradeAutoKephalaion    = program.KephalaionAutoUpgrade
	upgradeCheckKephalaion   = program.KephalaionCheckUpgrade
)

// upgradeOptions sind die Schalter des Aufrufs.
type upgradeOptions struct {
	check  bool
	asJSON bool
	// projectOnly: nur Schritt (2) — der Aufruf, mit dem ein alter Prozess das
	// neu installierte Programm für das Projekt arbeiten lässt.
	projectOnly bool
	// repairMCP: der Schalter --repair-mcp, nur zusammen mit projectOnly. Er
	// wird angenommen und entscheidet nichts mehr — Schritt (2) repariert die
	// MCP-Einträge immer. Ein Binary von vor Task 149 startet das neu
	// installierte Programm noch mit ihm.
	repairMCP bool
}

// upgradeReport ist die Antwort, als JSON oder — Zeile für Zeile aus Messages
// — als Klartext.
type upgradeReport struct {
	SchemaVersion string `json:"schemaVersion"`
	Check         bool   `json:"check"`
	// Program ist Schritt (1); fehlt bei --project-only.
	Program *upgradeProgramReport `json:"program,omitempty"`
	// Kephalaion ist der erste Schritt, vor (1); fehlt bei --project-only.
	Kephalaion *upgradeKephalaionReport `json:"kephalaion,omitempty"`
	// Project ist Schritt (2): upgradeProjectReport, oder — nach einem
	// Binärwechsel — die Antwort des neuen Programms, so wie sie kam.
	Project any `json:"project,omitempty"`
	// RestartRequired: das Binary hat gewechselt, oder Kephalaion wurde
	// aktualisiert bzw. sein Node neu gestartet — die Sitzung muss neu
	// gestartet werden.
	RestartRequired bool `json:"restartRequired"`
	// Messages sind die Meldungen je Schritt, in der Reihenfolge des Laufs.
	Messages []string `json:"messages"`
}

func (r *upgradeReport) say(format string, args ...any) {
	r.Messages = append(r.Messages, fmt.Sprintf(format, args...))
}

// upgradeProgramReport ist Schritt (1).
type upgradeProgramReport struct {
	// Status vergleicht das laufende Programm mit dem neuesten Release-Tag.
	Status *program.Status `json:"status,omitempty"`
	// Update ist der Remote-Stand des Clones (nur --check).
	Update *project.UpdateStatus `json:"update,omitempty"`
	// Pulled: der Pull im Clone ist gelaufen.
	Pulled     bool   `json:"pulled,omitempty"`
	PullOutput string `json:"pullOutput,omitempty"`
	// RemoteSwitched: der Upstream trug die SSH-Adresse des offiziellen Repos
	// und zeigt jetzt auf diese HTTPS-Adresse.
	RemoteSwitched string `json:"remoteSwitched,omitempty"`
	// Installed: das Release-Binary des neuesten Tags hat das Binary ersetzt.
	Installed     bool   `json:"installed,omitempty"`
	InstallOutput string `json:"installOutput,omitempty"`
	// Error nennt, was in diesem Schritt scheiterte; die weiteren Schritte
	// laufen trotzdem.
	Error string `json:"error,omitempty"`

	// fetchesNew (nur --check): das laufende Programm ist älter als der
	// neueste Release-Tag und installierbar, oder der Clone liegt hinter
	// seinem Remote — ein Lauf holt Neues.
	fetchesNew bool
}

// upgradeKephalaionReport ist der Schritt Kephalaion.
type upgradeKephalaionReport struct {
	// Present: kephalaion liegt im PATH — vorher oder nach der Installation.
	Present bool   `json:"present"`
	Path    string `json:"path,omitempty"`
	// Installed: das install.sh von Kephalaion ist gelaufen (nicht bei
	// --check).
	Installed bool   `json:"installed,omitempty"`
	Output    string `json:"output,omitempty"`
	// Auto ist die Antwort von `kephalaion upgrade --auto --json`.
	Auto *program.KephalaionAuto `json:"auto,omitempty"`
	// AutoUnsupported: das installierte kephalaion kennt --auto noch nicht
	// (Exit 2); Check ist dann der Report von --check.
	AutoUnsupported bool `json:"autoUnsupported,omitempty"`
	// Check ist der Report von `kephalaion upgrade --check --json` — bei
	// --check und wo --auto fehlt.
	Check *program.KephalaionCheck `json:"check,omitempty"`
	// OutcomeUnknown: der Aufruf wurde nach Fristablauf beendet; was er
	// getan hat, ist offen.
	OutcomeUnknown bool `json:"outcomeUnknown,omitempty"`
	// RestartRequired: Kephalaion wurde aktualisiert oder der Node neu
	// gestartet.
	RestartRequired bool   `json:"restartRequired,omitempty"`
	Error           string `json:"error,omitempty"`
}

// upgradeProjectReport ist Schritt (2).
type upgradeProjectReport struct {
	Root project.RootInstructionsState `json:"root"`
	// Kephalaion ist der Umzug der Anbindung aus der alten
	// k-playbook-local/K-PLAYBOOK.local.yaml (Task 138); fehlt ohne alte
	// Datei.
	Kephalaion *project.KephalaionMigration `json:"kephalaion,omitempty"`
	// Setup ist das Ergebnis von ApplyAssistantSetup (nicht bei --check).
	Setup *project.AssistantSetup `json:"setup,omitempty"`
	// Links ist die Bilanz der Verlinkung: bei --check, was ein Lauf täte,
	// sonst, was er getan hat.
	Links project.LinkChanges `json:"links"`
	// LinksCreated: die Verlinkung stand vorher gar nicht und steht jetzt.
	LinksCreated bool `json:"linksCreated,omitempty"`
	// MCPRepaired nennt die MCP-Dateien, die dieser Schritt geschrieben hat;
	// MCPRepairedPortable die davon, in denen jetzt der bloße Kommandoname
	// steht. MCPMessage sagt, was an der Reparatur nicht gelang.
	MCPRepaired         []string `json:"mcpRepaired,omitempty"`
	MCPRepairedPortable []string `json:"mcpRepairedPortable,omitempty"`
	MCPMessage          string   `json:"mcpMessage,omitempty"`
	// Pending nennt bei --check, was ein Lauf an AGENTS.md, CLAUDE.md,
	// Verlinkung und MCP-Einträgen täte.
	Pending []string `json:"pending,omitempty"`
	// Changed: der Lauf hat etwas geschrieben.
	Changed bool   `json:"changed"`
	Error   string `json:"error,omitempty"`
}

func runUpgrade(args []string) error {
	options := upgradeOptions{}
	for _, arg := range args {
		switch arg {
		case "--check":
			options.check = true
		case "--json":
			options.asJSON = true
		case "--project-only":
			options.projectOnly = true
		case "--repair-mcp":
			options.repairMCP = true
		case "-h", "--help", "help":
			printUpgradeUsage(os.Stdout)
			return nil
		default:
			return fmt.Errorf("unbekannte Option %s — erwartet wird upgrade [--check] [--json]", arg)
		}
	}
	if options.check && options.projectOnly {
		return errors.New("--check und --project-only schließen sich aus")
	}
	if options.repairMCP && !options.projectOnly {
		return errors.New("--repair-mcp gilt nur zusammen mit --project-only")
	}

	environment := project.Detect()
	if !environment.Installed {
		return fmt.Errorf("keine %s gefunden — gesucht ab %s aufwärts",
			project.ConfigFileName, project.DisplayPath(environment.SearchedFrom))
	}

	report := upgrade(environment, options, os.Stdout)
	if options.asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	return nil
}

// upgrade führt den Lauf aus und schreibt im Klartextmodus jede Meldung sofort
// nach out — so steht Schritt (1) schon da, bevor das neue Programm für Schritt
// (2) seine eigenen Zeilen schreibt. Im JSON-Modus sammelt der Bericht alles
// und wird am Ende ausgegeben.
func upgrade(environment project.Environment, options upgradeOptions, out io.Writer) upgradeReport {
	report := upgradeReport{SchemaVersion: upgradeSchemaVersion, Check: options.check, Messages: []string{}}
	emit := func(lines []string) {
		if options.asJSON {
			return
		}
		for _, line := range lines {
			fmt.Fprintln(out, line)
		}
	}

	if options.projectOnly {
		result := upgradeProject(environment.ProjectDir)
		report.Project = &result
		report.Messages = append(report.Messages, result.messages()...)
		emit(result.messages())
		return report
	}

	if options.check {
		kephalaionReport, lines := checkUpgradeKephalaion()
		report.Kephalaion = &kephalaionReport
		report.Messages = append(report.Messages, lines...)
		emit(lines)

		programReport, lines := checkUpgradeProgram(environment)
		report.Program = &programReport
		report.Messages = append(report.Messages, lines...)
		emit(lines)

		result := checkUpgradeProject(environment.ProjectDir)
		report.Project = &result
		report.Messages = append(report.Messages, result.messages()...)
		emit(result.messages())

		if programReport.fetchesNew && !upgradeViaCommand() {
			line := "Ein Lauf holt Neues; bringt er eine neue Version, zieht danach /k-upgrade in einer Sitzung des Assistenten die Umstellungen nach, die eine Beurteilung brauchen."
			report.Messages = append(report.Messages, line)
			emit([]string{line})
		}
		return report
	}

	// Kephalaion zuerst: k-playbook hängt von Kephalaion ab, nicht umgekehrt,
	// und Programm- wie Projektschritt brauchen den Node. Ein Neustart des
	// Nodes ist so am Anfang erledigt und von --auto selbst geprüft.
	kephalaionReport, lines := upgradeKephalaion()
	report.Kephalaion = &kephalaionReport
	report.Messages = append(report.Messages, lines...)
	emit(lines)

	programReport, lines, target := upgradeProgram(environment)
	report.Program = &programReport
	report.Messages = append(report.Messages, lines...)
	emit(lines)

	if programReport.Installed && target != "" {
		// Das neue Programm zieht das Projekt nach — mit seinen Vorlagen und
		// seiner Erkennung, nicht mit denen dieses Prozesses. Die MCP-Einträge
		// repariert es dabei selbst, ob der Pull lief oder nicht.
		result, lines, err := upgradeReexec(target, environment.ProjectDir, options.asJSON, out)
		report.Project = result
		if err != nil {
			lines = append(lines, "Projekt: das neue Programm konnte das Projekt nicht nachziehen — "+err.Error()+
				". Nachholen: "+target+" upgrade.")
			emit(lines[len(lines)-1:])
		}
		report.Messages = append(report.Messages, lines...)
	} else {
		result := upgradeProject(environment.ProjectDir)
		report.Project = &result
		report.Messages = append(report.Messages, result.messages()...)
		emit(result.messages())
	}

	if programReport.Installed {
		report.RestartRequired = true
		line := "Das Programm hat gewechselt: die Sitzung neu starten — der MCP-Server der Sitzung behält das alte Programm, bis der Assistent ihn neu startet."
		report.Messages = append(report.Messages, line)
		emit([]string{line})
	}
	if kephalaionReport.RestartRequired {
		report.RestartRequired = true
		line := "Kephalaion wurde aktualisiert oder sein Node neu gestartet: die Sitzung neu starten. Auch andere offene Sitzungen an diesem Node können ihre Verbindung zu Kephalaion verloren haben."
		report.Messages = append(report.Messages, line)
		emit([]string{line})
	}
	if programReport.Installed && !upgradeViaCommand() {
		session := "in einer Sitzung"
		if report.RestartRequired {
			session = "in einer neuen Sitzung"
		}
		line := "Neue Version geholt: die Umstellungen, die eine Beurteilung brauchen, zieht /k-upgrade nach — " + session + " des Assistenten aufrufen."
		report.Messages = append(report.Messages, line)
		emit([]string{line})
	}
	return report
}

// upgradeProgram ist Schritt (1): erst der Pull im Clone, dann — nur bei
// „älter" als der neueste Release-Tag — das Binary. Das dritte Ergebnis ist das
// Installationsziel, wenn das Binary gewechselt hat; damit läuft Schritt (2) im
// neuen Programm.
//
// Der Pull dient nur noch den Inhalten des Clones. Ohne Clone oder nach einem
// gescheiterten Pull gibt es eine Meldung und keinen Abbruch: Vergleich und
// Installation laufen trotzdem, und das Projekt wird nachgezogen. Scheitert
// Download oder Install, ist das eine eigene Meldung mit dem Weg im Terminal.
func upgradeProgram(environment project.Environment) (upgradeProgramReport, []string, string) {
	report := upgradeProgramReport{}
	lines := []string{}
	running := guiproc.OwnVersion()

	switch {
	case !environment.PlaybookPresent:
		lines = append(lines, "Programm: die Installation "+project.PlaybookDirName+"/ fehlt; der Pull entfällt. Einrichten: "+project.BootstrapHint+".")
	case !isDir(filepath.Join(environment.PlaybookDir, ".git")):
		lines = append(lines, "Programm: die Installation ist kein Git-Repository; der Pull entfällt.")
	default:
		result, err := upgradeUpdate(environment.ProjectDir)
		report.PullOutput = result.Output
		report.RemoteSwitched = result.RemoteSwitched
		if result.RemoteSwitched != "" {
			lines = append(lines, "Programm: Remote des Clones von SSH auf "+result.RemoteSwitched+" umgestellt; das Repo ist öffentlich, der Pull braucht keinen Schlüssel.")
		}
		if err != nil {
			report.Error = err.Error()
			lines = append(lines, "Programm: Pull im Clone fehlgeschlagen — "+err.Error()+"; ohne Netz oder bei lokalen Änderungen im Clone ist das zu erwarten. Verglichen und installiert wird trotzdem.")
			if result.Output != "" {
				lines = append(lines, indent(result.Output))
			}
		} else {
			report.Pulled = true
			lines = append(lines, "Programm: Clone "+project.PlaybookDirName+"/ per Pull auf dem Stand seines Remotes.")
		}
	}

	status := program.CheckStatus(context.Background(), running)
	report.Status = status
	lines = append(lines, releaseSourceLines()...)
	switch {
	case !version.Valid(running):
		lines = append(lines, "Programm: dieses Programm trägt keine Version (ohne Build-Flags gebaut); installiert wird nicht.")
	case status.Latest == "":
		lines = append(lines, "Programm-Stand: der neueste Release-Tag ist unbekannt — "+status.LatestError+
			". Verglichen und installiert wird nicht; das Projekt wird mit dem laufenden Programm nachgezogen.")
	case !status.Outdated:
		lines = append(lines, fmt.Sprintf("Programm-Stand: nichts zu tun — laufend %s, neuestes Release %s (%s).",
			displayVersion(status.Running), displayVersion(status.Latest), status.Order))
	case !status.Installable:
		report.Error = status.Hint
		lines = append(lines, "Programm: "+status.Hint)
	default:
		result, err := upgradeInstall(context.Background(), status.Latest)
		report.InstallOutput = result.Output
		if err != nil {
			report.Error = err.Error()
			// Der Ausweg ist hier nicht `k-playbook upgrade` — das ist eben
			// gescheitert —, sondern die Erstinstallation.
			lines = append(lines, "Programm nicht aktualisiert — Download/Install nicht möglich: "+strings.TrimSuffix(err.Error(), ".")+
				". Das Projekt wird mit dem laufenden Programm nachgezogen. Nachholen im Terminal: "+project.FirstInstallCommand)
			if result.Output != "" {
				lines = append(lines, indent(result.Output))
			}
			break
		}
		if result.Installed {
			report.Installed = true
			lines = append(lines, fmt.Sprintf("Programm: %s installiert nach %s (vorher lief %s).",
				displayVersion(result.Expected), result.Target, displayVersion(running)))
			if result.Output != "" {
				lines = append(lines, indent(result.Output))
			}
			return report, lines, result.Target
		}
		lines = append(lines, fmt.Sprintf("Programm: am Installationsziel %s liegt schon %s; dieses Programm (%s) lief von woanders. Der nächste Aufruf nimmt das installierte.",
			result.Target, displayVersion(result.Expected), displayVersion(running)))
	}
	return report, lines, ""
}

// releaseSourceLines nennt eine ersetzte oder abgeschaltete Release-Quelle
// (K_PLAYBOOK_RELEASE_SOURCE). Die Variable ist die Testnaht des Pakets
// release und wirkt auch im ausgelieferten Programm; wer sie gesetzt hat,
// liest hier, dass nicht GitHub gefragt wurde. Im Normalfall keine Zeile.
func releaseSourceLines() []string {
	if note := release.CurrentSource().Note(); note != "" {
		return []string{"Programm: " + note + "."}
	}
	return nil
}

// checkUpgradeProgram meldet den Programm-Stand ohne zu schreiben: der
// Remote-Stand des Clones und der Vergleich der laufenden Version mit dem
// neuesten Release-Tag. Der Vergleich läuft auch ohne Clone.
func checkUpgradeProgram(environment project.Environment) (upgradeProgramReport, []string) {
	report := upgradeProgramReport{}
	lines := []string{}
	if !environment.PlaybookPresent {
		lines = append(lines, "Programm: die Installation "+project.PlaybookDirName+"/ fehlt; ein Lauf zöge keinen Clone nach.")
	} else {
		update, err := upgradeCheckUpdate(environment.ProjectDir)
		switch {
		case err != nil:
			report.Error = err.Error()
			lines = append(lines, "Installation: Prüfung fehlgeschlagen — "+err.Error())
		case update.Available:
			report.Update = &update
			report.fetchesNew = true
			lines = append(lines, fmt.Sprintf("Installation: hinter dem Remote (%s → %s auf %s); ein Lauf holte den neuen Stand.",
				shortHash(update.Local), shortHash(update.Remote), update.Branch))
		case update.Message != "":
			report.Update = &update
			lines = append(lines, "Installation: "+update.Message)
		default:
			report.Update = &update
			lines = append(lines, "Installation: auf dem Stand des Remotes ("+shortHash(update.Local)+").")
		}
	}

	running := guiproc.OwnVersion()
	status := program.CheckStatus(context.Background(), running)
	report.Status = status
	lines = append(lines, releaseSourceLines()...)
	switch {
	case !version.Valid(running):
		lines = append(lines, "Programm-Stand: dieses Programm trägt keine Version (ohne Build-Flags gebaut); ein Lauf installierte nicht.")
	case status.Latest == "":
		lines = append(lines, "Programm-Stand: der neueste Release-Tag ist unbekannt — "+status.LatestError+
			"; ein Lauf vergliche und installierte nicht.")
	case !status.Outdated:
		lines = append(lines, fmt.Sprintf("Programm-Stand: nichts zu tun — laufend %s, neuestes Release %s (%s).",
			displayVersion(status.Running), displayVersion(status.Latest), status.Order))
	case !status.Installable:
		lines = append(lines, "Programm-Stand: älter als das neueste Release ("+displayVersion(status.Latest)+"), aber "+status.Hint)
	default:
		report.fetchesNew = true
		lines = append(lines, fmt.Sprintf("Programm-Stand: %s ist älter als das neueste Release (%s); ein Lauf installierte das Programm nach %s, danach wäre die Sitzung neu zu starten.",
			displayVersion(status.Running), displayVersion(status.Latest), status.Target))
	}
	return report, lines
}

// upgradeKephalaion ist der Schritt Kephalaion, drei Wege:
//
//  1. kephalaion fehlt: das install.sh des neuesten Kephalaion-Releases
//     (program.InstallKephalaion); danach kein --auto.
//  2. kephalaion da: `kephalaion upgrade --auto --json`; je outcome eine
//     Meldung. Was gefahrlos ist, entscheidet Kephalaion, nicht k-playbook.
//  3. kephalaion kennt --auto nicht (Exit 2): `upgrade --check --json` und
//     die Meldung, ob ein neueres Release da ist.
//
// Kein Weg braucht den Clone, und keiner hält den Lauf auf.
func upgradeKephalaion() (upgradeKephalaionReport, []string) {
	path, ok := upgradeFindKephalaion()
	if !ok {
		return installKephalaion()
	}

	report := upgradeKephalaionReport{Present: true, Path: path}
	auto, err := upgradeAutoKephalaion(path)
	if errors.Is(err, program.ErrKephalaionAutoUnsupported) {
		report.AutoUnsupported = true
		check, lines := checkKephalaionRelease(path, &report)
		report.Check = check
		if check != nil {
			lines = describeKephalaionWithoutAuto(*check)
		}
		return report, lines
	}
	if err != nil {
		return report, kephalaionCallFailed(&report, err)
	}
	report.Auto = &auto
	report.RestartRequired = auto.Outcome == program.KephalaionUpgraded || auto.NodeRestarted()
	return report, describeKephalaionAuto(auto)
}

// installKephalaion ist Weg 1: das install.sh des neuesten
// Kephalaion-Releases. Scheitert es, nennt die Meldung die Installation von
// Hand (kephalaion.InstallCommand) — dasselbe Skript ohne k-playbook.
func installKephalaion() (upgradeKephalaionReport, []string) {
	report := upgradeKephalaionReport{}
	result, err := upgradeInstallKephalaion(context.Background())
	report.Path, report.Installed, report.Output = result.Path, result.Installed, result.Output
	report.Present = result.Path != ""
	switch {
	case err != nil:
		report.Error = err.Error()
		lines := []string{"Kephalaion: nicht installiert — " + strings.TrimSuffix(err.Error(), ".") +
			". Nachholen im Terminal: " + kephalaion.InstallCommand}
		if result.Output != "" {
			lines = append(lines, indent(result.Output))
		}
		return report, lines
	case result.Installed:
		lines := []string{"Kephalaion: installiert nach " + result.Path + "."}
		if result.Output != "" {
			lines = append(lines, indent(result.Output))
		}
		return report, lines
	}
	return report, []string{"Kephalaion: vorhanden (" + result.Path + ")."}
}

// checkUpgradeKephalaion meldet, was ein Lauf versuchen würde, ohne etwas zu
// ändern: fehlt kephalaion, die Installation — auch ohne Clone; sonst der
// Report von `upgrade --check --json`. --auto ruft es nie auf.
func checkUpgradeKephalaion() (upgradeKephalaionReport, []string) {
	path, ok := upgradeFindKephalaion()
	if !ok {
		return upgradeKephalaionReport{}, []string{"Kephalaion: fehlt im PATH; ein Lauf installiert es über das install.sh des neuesten Kephalaion-Releases (von Hand: " +
			kephalaion.InstallCommand + ")."}
	}
	report := upgradeKephalaionReport{Present: true, Path: path}
	check, lines := checkKephalaionRelease(path, &report)
	report.Check = check
	if check == nil {
		return report, lines
	}
	switch {
	case check.State != "ok":
		return report, describeKephalaionCheckFailed(*check)
	case check.DevBuild:
		return report, []string{"Kephalaion: " + check.Version + " ist ein selbst gebautes Binary; ein Lauf ruft kephalaion upgrade --auto auf, das es nicht ersetzt."}
	case check.UpdateAvailable:
		line := "Kephalaion: " + check.Version + " installiert, " + check.Latest + " verfügbar"
		if check.Hint != "" {
			line += " — " + check.Hint
		}
		return report, []string{line + "; ein Lauf ruft kephalaion upgrade --auto auf, ob es aktualisiert, entscheidet Kephalaion."}
	}
	return report, []string{"Kephalaion: " + check.Version + " ist aktuell."}
}

// checkKephalaionRelease ruft `upgrade --check --json` auf. Ohne Report
// kommen die Zeilen zum Fehlschlag zurück, und report trägt den Fehler.
func checkKephalaionRelease(path string, report *upgradeKephalaionReport) (*program.KephalaionCheck, []string) {
	check, err := upgradeCheckKephalaion(path)
	if err != nil {
		return nil, kephalaionCallFailed(report, err)
	}
	return &check, nil
}

// kephalaionNextStep ist der Prüfschritt von Hand nach einem Aufruf ohne
// verwertbare Antwort.
const kephalaionNextStep = "Nächster Schritt: kephalaion status, dann kephalaion upgrade --check von Hand."

// kephalaionCallFailed meldet einen Aufruf ohne verwertbare Antwort: kein
// JSON, nicht startbar oder nach Fristablauf beendet. Kein Abbruch.
func kephalaionCallFailed(report *upgradeKephalaionReport, err error) []string {
	report.Error = err.Error()
	var unknown *program.KephalaionUnknownOutcomeError
	if errors.As(err, &unknown) {
		report.OutcomeUnknown = true
		return []string{"Kephalaion: " + strings.TrimSuffix(err.Error(), ".") +
			". Prüfen: kephalaion status — wurde der Node neu gestartet, die Sitzung neu starten."}
	}
	return []string{"Kephalaion: " + strings.TrimSuffix(err.Error(), ".") + ". " + kephalaionNextStep}
}

// describeKephalaionAuto formuliert die Antwort von --auto.
func describeKephalaionAuto(auto program.KephalaionAuto) []string {
	lines := []string{}
	switch auto.Outcome {
	case program.KephalaionUpgraded:
		line := "Kephalaion: von " + auto.From + " auf " + auto.To + " aktualisiert"
		if auto.Restarted {
			line += ", Dienst neu gestartet"
		}
		lines = append(lines, line+".")
	case program.KephalaionCurrent:
		version := auto.From
		if version == "" && auto.Check != nil {
			version = auto.Check.Version
		}
		lines = append(lines, "Kephalaion: "+displayVersion(version)+" ist aktuell.")
	case program.KephalaionSkipped:
		line := "Kephalaion: "
		if auto.To != "" {
			line += auto.To + " verfügbar, "
		}
		line += "nicht automatisch aktualisiert (" + auto.Reason + ")"
		if auto.Hint != "" {
			line += " — " + auto.Hint
		}
		lines = append(lines, strings.TrimSuffix(line, ".")+".")
	case program.KephalaionFailed:
		line := "Kephalaion: Upgrade gescheitert (" + auto.Reason + ")"
		if auto.Error != "" {
			line += " — " + auto.Error
		}
		lines = append(lines, strings.TrimSuffix(line, ".")+".")
		if auto.Reason == program.KephalaionReasonRestart {
			switch {
			case auto.RolledBack != nil && *auto.RolledBack:
				lines = append(lines, "  Der Node wurde neu gestartet; das alte Binary ist zurück und läuft wieder.")
			default:
				lines = append(lines, "  Der Node wurde neu gestartet; das alte Binary ist nicht zurückgesetzt.")
			}
		}
		if auto.Hint != "" {
			lines = append(lines, "  "+auto.Hint)
		}
	default:
		lines = append(lines, "Kephalaion: unbekannter Ausgang „"+auto.Outcome+"“ von kephalaion upgrade --auto. "+kephalaionNextStep)
	}
	for _, warning := range auto.Warnings {
		lines = append(lines, "  "+warning)
	}
	return lines
}

// describeKephalaionWithoutAuto ist Weg 3: der Report von --check eines
// kephalaion, das --auto noch nicht kennt.
func describeKephalaionWithoutAuto(check program.KephalaionCheck) []string {
	switch {
	case check.State != "ok":
		return describeKephalaionCheckFailed(check)
	case check.UpdateAvailable:
		line := "Kephalaion: " + check.Version + " installiert, " + check.Latest + " verfügbar"
		if check.Hint != "" {
			line += " — " + check.Hint
		}
		return []string{line + "; dieses Kephalaion aktualisiert sich noch nicht selbst über k-playbook."}
	case check.DevBuild:
		return []string{"Kephalaion: " + check.Version + " ist ein selbst gebautes Binary; k-playbook aktualisiert es nicht."}
	}
	return []string{"Kephalaion: " + check.Version + " ist aktuell."}
}

// describeKephalaionCheckFailed: --check hat geantwortet, die Frage an
// GitHub ist gescheitert.
func describeKephalaionCheckFailed(check program.KephalaionCheck) []string {
	reason := check.Error
	if reason == "" {
		reason = "state " + check.State
	}
	return []string{"Kephalaion: Prüfung auf ein neueres Release gescheitert — " + strings.TrimSuffix(reason, ".") + ". " + kephalaionNextStep}
}

// upgradeProject ist Schritt (2): derselbe Ablauf wie „Einrichten" in der
// Oberfläche, danach immer die MCP-Einträge.
//
// Die MCP-Reparatur hängt nicht am Pull und läuft auch unter --project-only:
// immer im Umfang des Starts (MCPWriteOutdatedAndUnversioned). Das ist
// gewollt mehr als früher nach einem gelungenen Pull, der nur veraltete
// Einträge ersetzte: Ein Lauf ergänzt auch einen fehlenden Eintrag bzw. eine
// fehlende Datei, wenn die Zieldatei nicht von git erfasst ist — der Stand,
// der ohne Pull ohnehin gilt.
func upgradeProject(projectDir string) upgradeProjectReport {
	report := upgradeProjectReport{}
	// Zuerst die Anbindung: Ohne Block kephalaion: ist Kephalaion nicht
	// bereit, und die Schritte danach (READMEs unter keph:/docs/) brauchen
	// sie.
	if migration := project.MigrateKephalaionBinding(projectDir, false); migration.Legacy {
		report.Kephalaion = &migration
	}
	before := project.CheckLinks(projectDir)
	pending := project.PendingLinkChanges(before)

	setup, err := project.ApplyAssistantSetup(projectDir)
	report.Setup = &setup
	report.Root = setup.Root
	report.Links = pending
	if err != nil {
		report.Error = err.Error()
	}
	// Ein Ziel, das es vorher gar nicht gab, zählt in der Bilanz nicht
	// (PendingLinkChanges); da ist keine Registrierung verändert, da ist eine
	// entstanden.
	report.LinksCreated = !project.LinksOK(before) && pending.Empty() && project.LinksOK(setup.Links)
	report.Changed = setup.RootCreated || setup.RootExtended || setup.RootRefreshed ||
		setup.Instructions.Renamed || setup.Instructions.RemovedLink || !pending.Empty() || report.LinksCreated ||
		len(setup.ConfigBlocksRemoved) > 0 || setup.Readmes.Changed() ||
		(report.Kephalaion != nil && report.Kephalaion.Written)

	repair := project.RepairMCPReport(projectDir, project.MCPWriteOutdatedAndUnversioned)
	report.MCPRepaired, report.MCPRepairedPortable, report.MCPMessage = repair.Repaired, repair.Portable, repair.Message
	if len(repair.Repaired) > 0 {
		report.Changed = true
	}
	return report
}

// messages formuliert Schritt (2) als Zeilen.
func (r upgradeProjectReport) messages() []string {
	lines := []string{}
	if len(r.Pending) > 0 {
		for _, line := range r.Pending {
			lines = append(lines, "Projekt: "+line)
		}
		return lines
	}
	if r.Kephalaion != nil {
		for _, line := range r.Kephalaion.Messages() {
			lines = append(lines, "Projekt: "+line)
		}
	}
	if r.Setup != nil {
		setup := r.Setup
		if detail := setup.Instructions.Detail; detail != "" && (setup.Instructions.Renamed || setup.Instructions.RemovedLink || setup.Instructions.Outcome == project.InstructionsConflict) {
			lines = append(lines, "Projekt: "+detail)
		}
		switch {
		case setup.RootCreated:
			lines = append(lines, "Projekt: "+project.RootInstructionsFile+" aus der Vorlage angelegt.")
		case setup.RootExtended:
			lines = append(lines, "Projekt: der Anstoß steht jetzt in "+project.RootInstructionsFile+".")
		}
		if setup.RootAnstossRefreshed {
			lines = append(lines, "Projekt: der veraltete Anstoß in "+project.RootInstructionsFile+" wurde auf "+project.InstalledCommandName+" briefing gebracht.")
		}
		if len(setup.RootBlocksRemoved) > 0 {
			lines = append(lines, "Projekt: "+strings.Join(setup.RootBlocksRemoved, " und ")+" aus "+project.RootInstructionsFile+" entfernt; der Inhalt kommt mit dem Briefing.")
		}
		if len(setup.ConfigBlocksRemoved) > 0 {
			lines = append(lines, "Projekt: "+project.DescribeConfigBlocksRemoved(setup.ConfigBlocksRemoved))
		}
	}
	switch {
	case !r.Links.Empty():
		lines = append(lines, "Projekt: Verlinkung nachgezogen — "+describeLinkCounts(r.Links)+".")
	case r.LinksCreated:
		lines = append(lines, "Projekt: Verlinkung eingerichtet.")
	}
	if len(r.MCPRepaired) > 0 {
		lines = append(lines, "Projekt: MCP-Registrierung nachgezogen: "+strings.Join(r.MCPRepaired, ", ")+". Der Assistent liest den Eintrag erst nach einem Neustart.")
	}
	if len(r.MCPRepairedPortable) > 0 {
		lines = append(lines, "Projekt: in "+strings.Join(r.MCPRepairedPortable, ", ")+" steht der bloße Name "+project.InstalledCommandName+
			", weil die Datei von git erfasst ist; ein aus Dock oder Finder gestarteter Client braucht dort den absoluten Pfad.")
	}
	if r.MCPMessage != "" {
		lines = append(lines, "Projekt: "+r.MCPMessage)
	}
	if r.Error != "" {
		lines = append(lines, "Projekt: nicht vollständig nachgezogen — "+r.Error)
	}
	if len(lines) == 0 {
		lines = append(lines, "Projekt: nichts zu tun.")
	}
	// Der README-Schritt steht immer da, als Hinweis: Er ist kein Teil von
	// „nichts zu tun“, und ein Fehler darin bricht den Lauf nicht ab.
	if r.Setup != nil {
		if note := project.DescribeSetupReadmes(r.Setup.Readmes); note != "" {
			lines = append(lines, "Projekt: "+note)
		}
	}
	return lines
}

// checkUpgradeProject meldet aus den Check-Funktionen, was ein Lauf am Projekt
// täte — kein Trockenlauf von ApplyAssistantSetup, den es nicht gibt. Die
// README unter keph:/docs/ übergeht es: fehlende und abweichende meldet
// /k-docs (k-playbook docs readme --check).
func checkUpgradeProject(projectDir string) upgradeProjectReport {
	report := upgradeProjectReport{}
	root := project.CheckRootInstructions(projectDir)
	report.Root = root

	if migration := project.MigrateKephalaionBinding(projectDir, true); migration.Legacy {
		report.Kephalaion = &migration
		report.Pending = append(report.Pending, migration.Messages()...)
	}

	switch {
	case !root.Present:
		report.Pending = append(report.Pending, project.RootInstructionsFile+" fehlt; ein Lauf legte sie aus der Vorlage an, sofern die Einordnung von "+
			project.ClaudeInstructionsFile+"/"+project.RootInstructionsFile+" keinen Konflikt meldet.")
	case !root.HasMarker:
		report.Pending = append(report.Pending, "der Anstoß würde an "+project.RootInstructionsFile+" angehängt.")
	}
	// Gemeldet wird nur, was ein Lauf wirklich täte: ein Block, dessen Ende
	// sich nicht bestimmen ließ, steht zwar in HasOutdatedAnstoss bzw.
	// ObsoleteBlocks, bleibt aber stehen — er kommt allein über den Hinweis.
	if root.AnstossRefreshable {
		report.Pending = append(report.Pending, "der veraltete Anstoß in "+project.RootInstructionsFile+" würde auf "+project.InstalledCommandName+" briefing gebracht.")
	}
	if len(root.RemovableBlocks) > 0 {
		report.Pending = append(report.Pending, strings.Join(root.RemovableBlocks, " und ")+" würden aus "+project.RootInstructionsFile+" entfernt; der Inhalt kommt mit dem Briefing.")
	}
	if root.Hint != "" {
		report.Pending = append(report.Pending, "von Hand: "+root.Hint)
	}
	if project.LegacyGHBlockPresent(projectDir) {
		report.Pending = append(report.Pending, "der abgelöste Block "+project.LegacyGHBlockName+" würde aus "+project.ConfigFileName+" entfernt.")
	}

	statuses := project.CheckLinks(projectDir)
	report.Links = project.PendingLinkChanges(statuses)
	for _, status := range statuses {
		if status.IsInclude && status.NeedsAction() {
			report.Pending = append(report.Pending, status.Path+" würde als Include-Datei mit "+project.ClaudeIncludeLine+
				" angelegt oder nachgezogen (heute: "+status.Detail+").")
		}
	}
	switch {
	case !report.Links.Empty():
		report.Pending = append(report.Pending, "Verlinkung würde nachgezogen — "+describeLinkCounts(report.Links)+".")
	case !project.LinksOK(statuses):
		report.Pending = append(report.Pending, "Verlinkung würde eingerichtet.")
	}
	for _, status := range project.CheckMCP(projectDir) {
		if status.State == project.MCPStateOutdated {
			report.Pending = append(report.Pending, "veralteter MCP-Eintrag in "+status.Path+" würde korrigiert.")
		}
	}
	if len(report.Pending) == 0 {
		report.Pending = []string{"nichts zu tun."}
	}
	return report
}

// reexecProjectStep lässt das neu installierte Programm Schritt (2) ausführen.
// Im Klartextmodus schreibt es direkt nach out; im JSON-Modus kommt seine
// Antwort zurück, so wie sie kam — ein neueres Programm darf mehr Felder
// kennen als dieses. --repair-mcp reist nicht mehr mit: Das neue Programm ist
// neuer als dieses und repariert die MCP-Einträge im Projektschritt immer.
func reexecProjectStep(target string, projectDir string, asJSON bool, out io.Writer) (any, []string, error) {
	args := []string{"upgrade", "--project-only"}
	if asJSON {
		args = append(args, "--json")
	}
	cmd := exec.Command(target, args...)
	cmd.Dir = projectDir
	cmd.Env = withoutEnv(os.Environ(), guiproc.ServeEnv)
	cmd.Stderr = os.Stderr

	if !asJSON {
		// Die Zeilen des neuen Programms gehen direkt nach out; im Bericht
		// steht nur, wer sie geschrieben hat.
		cmd.Stdout = out
		if err := cmd.Run(); err != nil {
			return nil, nil, err
		}
		return map[string]string{"program": target}, []string{"Projekt: nachgezogen vom neuen Programm " + target + "."}, nil
	}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, nil, err
	}
	var child upgradeReport
	if err := json.Unmarshal(stdout.Bytes(), &child); err != nil {
		return nil, nil, fmt.Errorf("Antwort des neuen Programms nicht lesbar: %w", err)
	}
	return child.Project, child.Messages, nil
}

func withoutEnv(environ []string, name string) []string {
	result := make([]string, 0, len(environ))
	for _, entry := range environ {
		if strings.HasPrefix(entry, name+"=") {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func describeLinkCounts(changes project.LinkChanges) string {
	parts := []string{}
	for _, part := range []struct {
		names []string
		label string
	}{
		{changes.Added, "dazugekommen"},
		{changes.Removed, "entfernt"},
		{changes.Repointed, "auf eine andere Quelle umgesetzt"},
	} {
		if len(part.names) > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", len(part.names), part.label))
		}
	}
	return strings.Join(parts, ", ")
}

func displayVersion(version string) string {
	if version == "" {
		return "ohne Version"
	}
	return version
}

func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

func indent(text string) string {
	return "  " + strings.ReplaceAll(strings.TrimSpace(text), "\n", "\n  ")
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func printUpgradeUsage(out io.Writer) {
	fmt.Fprint(out, `k-playbook upgrade — Kephalaion und Programm aktualisieren, das Projekt nachziehen

Aufruf:
  k-playbook upgrade [--check] [--json]

Drei Schritte, je Schritt eine Meldung, in dieser Reihenfolge:

  0. Kephalaion: zuerst, weil k-playbook von Kephalaion abhängt und die
     Schritte danach den Node brauchen. Liegt kein kephalaion im PATH, lädt
     der Schritt das install.sh des neuesten Kephalaion-Releases und führt
     es aus — auch ohne Clone. Liegt
     es da, ruft der Schritt kephalaion upgrade --auto --json auf:
     Kephalaion aktualisiert sich selbst und startet seinen Node neu, wenn
     es das für gefahrlos hält; einen Hub, eine globale Installation, einen
     dev build oder einen Node vor einem zu alten Hub fasst es nicht an und
     nennt den Weg von Hand. Kennt das installierte kephalaion --auto noch
     nicht, meldet der Schritt nach kephalaion upgrade --check --json, ob ein
     neueres Release da ist. Scheitert der Schritt, eine Meldung, und die
     Schritte danach laufen trotzdem.
  1. Programm: zuerst git pull --ff-only im Clone k-playbook/, solange es
     ihn gibt — er dient nur noch den Inhalten des Clones. Danach der
     Vergleich der eigenen Version mit dem neuesten Release-Tag auf GitHub
     (git ls-remote über HTTPS, anonym) und, nur wenn das laufende Programm
     älter ist, das Release-Binary dieses Tags: direkt von GitHub geladen,
     gegen die SHA256SUMS des Tags geprüft und nach ~/.local/bin/k-playbook
     gelegt; gleich oder neuer bleibt stehen. Vergleich und Installation
     laufen auch ohne Clone und nach einem gescheiterten Pull; beides wird
     gemeldet. Ist der neueste Tag unbekannt (ohne Netz, ohne git,
     Zeitüberschreitung) oder trägt das Programm keine Version, eine Meldung,
     kein Abbruch; scheitert Download oder Install, eine eigene Meldung, und
     Schritt 2 läuft mit dem laufenden Programm. Hat das Binary gewechselt,
     führt das neu installierte Programm Schritt 2 aus. Trägt der Clone die
     SSH-Adresse des öffentlichen Repos (git@github.com:kascada/k-playbook),
     stellt der Lauf sie vor dem Pull auf
     https://github.com/kascada/k-playbook.git um; ein Alias-Host aus
     ~/.ssh/config bleibt stehen.
  2. Projekt: AGENTS.md (Anstoß anlegen oder anhängen, einen veralteten
     Anstoß auf k-playbook briefing bringen, die abgelösten Session-Memory-
     und Befunde-Blöcke entfernen), CLAUDE.md als Include-Datei, die
     Verlinkung der Commands und Skills, danach immer die MCP-Einträge: ein
     veralteter wird ersetzt, ein fehlender ergänzt, wenn die Zieldatei nicht
     von git erfasst ist. Projekt-Prosa in AGENTS.md und k-playbook-local/
     bleiben unangetastet.

Am Ende der Hinweis, die Sitzung neu zu starten: wenn das Binary gewechselt
hat — der MCP-Server der Sitzung behält das alte Programm —, und wenn
Kephalaion aktualisiert oder sein Node neu gestartet wurde; das kann auch
andere offene Sitzungen an diesem Node treffen. Hat der Lauf das Binary
ersetzt, sagt die letzte Zeile, dass /k-upgrade in einer Sitzung die
Umstellungen nachzieht, die eine Beurteilung brauchen.

--check schreibt nichts: es meldet nach kephalaion upgrade --check --json, ob
ein neueres Kephalaion da ist (--auto ruft es nie auf), den Stand des Clones
gegenüber dem Remote (bei der SSH-Adresse des öffentlichen Repos über HTTPS
gefragt), den Stand des laufenden Programms gegenüber dem neuesten
Release-Tag und, was ein Lauf am Projekt täte. Holte ein Lauf Neues, sagt
eine letzte Zeile, dass danach /k-upgrade die Umstellungen nachzieht. --json
gibt den Bericht als JSON aus (schemaVersion, check, kephalaion, program,
project, restartRequired, messages).

Mit K_PLAYBOOK_VIA_COMMAND=1 — so rufen Hülle und Ersatzaufruf von
/k-upgrade das Programm auf — entfallen beide Hinweise auf /k-upgrade.
`)
}
