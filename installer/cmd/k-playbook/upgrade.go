package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/program"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// k-playbook upgrade — der eine Nachzug für Programm und Projekt, den /k-upgrade
// aufruft.
//
// Zwei Schritte, je Schritt eine Meldung: (1) das Programm — `git pull` im
// Clone, danach das Binary über den Bootstrap des Clones, aber nur, wenn das
// laufende Programm älter ist als der Clone (Regel aus restartAfterPull in
// webui/update.go: nie „ungleich", sonst stufte das Entwicklungsrepo sein
// neueres Programm herab); (2) das Projekt — ApplyAssistantSetup, also
// Einordnen von CLAUDE.md/AGENTS.md, Anstoß ersetzen und Zusatzblöcke
// entfernen, Verlinkung; die MCP-Einträge repariert der Pull schon
// (UpdateResult.MCPRepaired), ein eigenes RepairMCP läuft nur, wenn der Pull
// entfiel. Am Ende der Hinweis auf den Neustart der Sitzung, wenn das Binary
// gewechselt hat: der MCP-Server der Sitzung behält das alte Programm.
//
// Hat Schritt (1) das Binary ersetzt, führt dieser — noch alte — Prozess
// Schritt (2) nicht selbst aus: er hätte alte Vorlagen und alte Erkennung.
// Er startet dafür das neu installierte Programm mit `upgrade --project-only`
// (reexecProjectStep) und gibt ihm mit `--repair-mcp` mit, ob der Pull entfiel:
// das neue Programm weiß sonst nicht, ob die MCP-Einträge schon repariert
// sind. Die Oberfläche braucht das nicht, weil sie neu startet; upgrade hat
// keinen Neustart.
//
// `--check` schreibt nichts: es meldet den Programm-Stand (CheckUpdate plus
// Vergleich mit der laufenden Version) und baut die Projektprüfung aus den
// Check-Funktionen — es meldet, was ein Lauf täte.

// upgradeSchemaVersion ist die Fassung der JSON-Antwort.
const upgradeSchemaVersion = "1"

// Austauschbar, damit die Abfolge ohne Netz, Clone und echtes Binary prüfbar
// ist: der Pull, der Bootstrap und der Aufruf des neu installierten Programms.
var (
	upgradeUpdate  = project.Update
	upgradeInstall = program.Install
	upgradeReexec  = reexecProjectStep
)

// upgradeOptions sind die Schalter des Aufrufs.
type upgradeOptions struct {
	check  bool
	asJSON bool
	// projectOnly: nur Schritt (2) — der Aufruf, mit dem ein alter Prozess das
	// neu installierte Programm für das Projekt arbeiten lässt.
	projectOnly bool
	// repairMCP: Schritt (2) soll die MCP-Einträge reparieren, weil der Pull
	// im alten Prozess entfiel; nur zusammen mit projectOnly.
	repairMCP bool
}

// upgradeReport ist die Antwort, als JSON oder — Zeile für Zeile aus Messages
// — als Klartext.
type upgradeReport struct {
	SchemaVersion string `json:"schemaVersion"`
	Check         bool   `json:"check"`
	// Program ist Schritt (1); fehlt bei --project-only.
	Program *upgradeProgramReport `json:"program,omitempty"`
	// Project ist Schritt (2): upgradeProjectReport, oder — nach einem
	// Binärwechsel — die Antwort des neuen Programms, so wie sie kam.
	Project any `json:"project,omitempty"`
	// RestartRequired: das Binary hat gewechselt, die Sitzung muss neu
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
	// Status vergleicht das laufende Programm mit dem Clone — nach dem Pull.
	Status *program.Status `json:"status,omitempty"`
	// Update ist der Remote-Stand (nur --check).
	Update *project.UpdateStatus `json:"update,omitempty"`
	// Pulled: der Pull im Clone ist gelaufen.
	Pulled     bool   `json:"pulled,omitempty"`
	PullOutput string `json:"pullOutput,omitempty"`
	// Installed: bin/install hat das Binary ersetzt.
	Installed     bool   `json:"installed,omitempty"`
	InstallOutput string `json:"installOutput,omitempty"`
	// MCPRepaired nennt die MCP-Dateien, deren veralteter Eintrag der Pull
	// korrigiert hat.
	MCPRepaired []string `json:"mcpRepaired,omitempty"`
	// Error nennt, was in diesem Schritt scheiterte; die weiteren Schritte
	// laufen trotzdem.
	Error string `json:"error,omitempty"`
}

// upgradeProjectReport ist Schritt (2).
type upgradeProjectReport struct {
	Root project.RootInstructionsState `json:"root"`
	// Setup ist das Ergebnis von ApplyAssistantSetup (nicht bei --check).
	Setup *project.AssistantSetup `json:"setup,omitempty"`
	// Links ist die Bilanz der Verlinkung: bei --check, was ein Lauf täte,
	// sonst, was er getan hat.
	Links project.LinkChanges `json:"links"`
	// LinksCreated: die Verlinkung stand vorher gar nicht und steht jetzt.
	LinksCreated bool `json:"linksCreated,omitempty"`
	// MCPRepaired nennt die MCP-Dateien, die dieser Schritt geschrieben hat —
	// nur, wenn der Pull entfiel.
	MCPRepaired []string `json:"mcpRepaired,omitempty"`
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
		result := upgradeProject(environment.ProjectDir, options.repairMCP)
		report.Project = &result
		report.Messages = append(report.Messages, result.messages()...)
		emit(result.messages())
		return report
	}

	if options.check {
		programReport, lines := checkUpgradeProgram(environment)
		report.Program = &programReport
		report.Messages = append(report.Messages, lines...)
		emit(lines)

		result := checkUpgradeProject(environment.ProjectDir)
		report.Project = &result
		report.Messages = append(report.Messages, result.messages()...)
		emit(result.messages())
		return report
	}

	programReport, lines, target := upgradeProgram(environment)
	report.Program = &programReport
	report.Messages = append(report.Messages, lines...)
	emit(lines)

	if programReport.Installed && target != "" {
		// Das neue Programm zieht das Projekt nach — mit seinen Vorlagen und
		// seiner Erkennung, nicht mit denen dieses Prozesses. Ob der Pull
		// entfiel, weiß nur dieser Prozess; die MCP-Reparatur reist als
		// Schalter mit, mit derselben Regel wie ohne Binärwechsel.
		result, lines, err := upgradeReexec(target, environment.ProjectDir, !programReport.Pulled, options.asJSON, out)
		report.Project = result
		if err != nil {
			lines = append(lines, "Projekt: das neue Programm konnte das Projekt nicht nachziehen — "+err.Error()+
				". Nachholen: "+target+" upgrade.")
			emit(lines[len(lines)-1:])
		}
		report.Messages = append(report.Messages, lines...)
	} else {
		result := upgradeProject(environment.ProjectDir, !programReport.Pulled)
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
	return report
}

// upgradeProgram ist Schritt (1): Pull im Clone, dann — nur bei „älter" — das
// Binary. Das dritte Ergebnis ist das Installationsziel, wenn das Binary
// gewechselt hat; damit läuft Schritt (2) im neuen Programm.
//
// Ohne Clone oder ohne Netz gibt es eine klare Meldung und keinen Abbruch:
// das Projekt wird trotzdem nachgezogen, mit dem laufenden Programm. Scheitert
// Download oder Install, ist das eine eigene Meldung mit dem Weg im Terminal.
func upgradeProgram(environment project.Environment) (upgradeProgramReport, []string, string) {
	report := upgradeProgramReport{}
	lines := []string{}
	running := guiproc.OwnVersion()

	switch {
	case !environment.PlaybookPresent:
		lines = append(lines, "Programm: die Installation "+project.PlaybookDirName+"/ fehlt; es gibt nichts zu aktualisieren. Einrichten: "+project.BootstrapHint+".")
	case !isDir(filepath.Join(environment.PlaybookDir, ".git")):
		lines = append(lines, "Programm: die Installation ist kein Git-Repository; der Pull entfällt.")
	default:
		result, err := upgradeUpdate(environment.ProjectDir)
		report.PullOutput = result.Output
		if err != nil {
			report.Error = err.Error()
			lines = append(lines, "Programm: Pull im Clone fehlgeschlagen — "+err.Error()+"; ohne Netz oder bei lokalen Änderungen im Clone ist das zu erwarten. Das Projekt wird mit dem laufenden Programm nachgezogen.")
			if result.Output != "" {
				lines = append(lines, indent(result.Output))
			}
		} else {
			report.Pulled = true
			report.MCPRepaired = result.MCPRepaired
			if result.VersionChanged {
				lines = append(lines, "Programm: Installation aktualisiert, VERSION jetzt "+displayVersion(result.Version)+".")
			} else {
				lines = append(lines, "Programm: Installation auf dem neuesten Stand"+describeVersionSuffix(result.Version)+".")
			}
			lines = append(lines, describeMCPRepairLines(result)...)
		}
	}

	if !environment.PlaybookPresent {
		return report, lines, ""
	}

	status := program.CheckStatus(running, environment.PlaybookDir)
	report.Status = status
	switch {
	case running == "":
		lines = append(lines, "Programm: dieses Programm trägt keine Version (ohne Build-Flags gebaut); installiert wird nicht.")
	case !status.Outdated:
		lines = append(lines, fmt.Sprintf("Programm-Stand: nichts zu tun — laufend %s, Installation %s (%s).",
			displayVersion(status.Running), displayVersion(status.Clone), status.Order))
	case !status.Installable:
		report.Error = status.Hint
		lines = append(lines, "Programm: "+status.Hint)
	default:
		result, err := upgradeInstall(environment.ProjectDir)
		report.InstallOutput = result.Output
		if err != nil {
			report.Error = err.Error()
			lines = append(lines, "Programm nicht aktualisiert — Download/Install nicht möglich: "+strings.TrimSuffix(err.Error(), ".")+
				". Das Projekt wird mit dem laufenden Programm nachgezogen. Nachholen im Terminal: "+project.BootstrapHint+".")
			if result.Output != "" {
				lines = append(lines, indent(result.Output))
			}
			break
		}
		if result.Bootstrapped {
			report.Installed = true
			lines = append(lines, fmt.Sprintf("Programm: %s installiert nach %s (vorher lief %s).",
				displayVersion(result.Expected), result.Target, displayVersion(running)))
			return report, lines, result.Target
		}
		lines = append(lines, fmt.Sprintf("Programm: am Installationsziel %s liegt schon %s; dieses Programm (%s) lief von woanders. Der nächste Aufruf nimmt das installierte.",
			result.Target, displayVersion(result.Expected), displayVersion(running)))
	}
	return report, lines, ""
}

// checkUpgradeProgram meldet den Programm-Stand ohne zu schreiben: der
// Remote-Stand des Clones und der Vergleich mit der laufenden Version.
func checkUpgradeProgram(environment project.Environment) (upgradeProgramReport, []string) {
	report := upgradeProgramReport{}
	lines := []string{}
	if !environment.PlaybookPresent {
		lines = append(lines, "Programm: die Installation "+project.PlaybookDirName+"/ fehlt; ein Lauf könnte nichts aktualisieren.")
		return report, lines
	}

	update, err := project.CheckUpdate(environment.ProjectDir)
	switch {
	case err != nil:
		report.Error = err.Error()
		lines = append(lines, "Installation: Prüfung fehlgeschlagen — "+err.Error())
	case update.Available:
		report.Update = &update
		lines = append(lines, fmt.Sprintf("Installation: hinter dem Remote (%s → %s auf %s); ein Lauf holte den neuen Stand.",
			shortHash(update.Local), shortHash(update.Remote), update.Branch))
	case update.Message != "":
		report.Update = &update
		lines = append(lines, "Installation: "+update.Message)
	default:
		report.Update = &update
		lines = append(lines, "Installation: auf dem Stand des Remotes ("+shortHash(update.Local)+").")
	}

	running := guiproc.OwnVersion()
	status := program.CheckStatus(running, environment.PlaybookDir)
	report.Status = status
	switch {
	case running == "":
		lines = append(lines, "Programm-Stand: dieses Programm trägt keine Version (ohne Build-Flags gebaut); ein Lauf installierte nicht.")
	case !status.Outdated:
		lines = append(lines, fmt.Sprintf("Programm-Stand: nichts zu tun — laufend %s, Installation %s (%s).",
			displayVersion(status.Running), displayVersion(status.Clone), status.Order))
	case !status.Installable:
		lines = append(lines, "Programm-Stand: älter als die Installation, aber "+status.Hint)
	default:
		lines = append(lines, fmt.Sprintf("Programm-Stand: %s ist älter als die Installation (%s); ein Lauf installierte das Programm nach %s, danach wäre die Sitzung neu zu starten.",
			displayVersion(status.Running), displayVersion(status.Clone), status.Target))
	}
	return report, lines
}

// upgradeProject ist Schritt (2): derselbe Ablauf wie „Einrichten" in der
// Oberfläche. repairMCP läuft nur, wenn der Pull entfiel — sonst hat der die
// veralteten Einträge schon korrigiert, und die Meldung stünde doppelt.
func upgradeProject(projectDir string, repairMCP bool) upgradeProjectReport {
	report := upgradeProjectReport{}
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
		setup.Instructions.Renamed || setup.Instructions.RemovedLink || !pending.Empty() || report.LinksCreated

	if repairMCP {
		writes, err := project.RepairMCP(projectDir, project.MCPWriteOutdatedAndUnversioned)
		report.MCPRepaired = project.MCPWritePaths(writes)
		if len(writes) > 0 {
			report.Changed = true
		}
		if err != nil {
			report.Error = strings.TrimSpace(report.Error + " MCP-Registrierung nicht vollständig nachgezogen: " + err.Error())
		}
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
	if r.Error != "" {
		lines = append(lines, "Projekt: nicht vollständig nachgezogen — "+r.Error)
	}
	if len(lines) == 0 {
		lines = append(lines, "Projekt: nichts zu tun.")
	}
	return lines
}

// checkUpgradeProject meldet aus den Check-Funktionen, was ein Lauf am Projekt
// täte — kein Trockenlauf von ApplyAssistantSetup, den es nicht gibt.
func checkUpgradeProject(projectDir string) upgradeProjectReport {
	report := upgradeProjectReport{}
	root := project.CheckRootInstructions(projectDir)
	report.Root = root

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
// repairMCP gibt ihm mit, dass der Pull entfiel und die MCP-Einträge noch zu
// reparieren sind. Im Klartextmodus schreibt es direkt nach out; im JSON-Modus
// kommt seine Antwort zurück, so wie sie kam — ein neueres Programm darf mehr
// Felder kennen als dieses.
func reexecProjectStep(target string, projectDir string, repairMCP bool, asJSON bool, out io.Writer) (any, []string, error) {
	args := []string{"upgrade", "--project-only"}
	if repairMCP {
		args = append(args, "--repair-mcp")
	}
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

func describeMCPRepairLines(result project.UpdateResult) []string {
	lines := []string{}
	if len(result.MCPRepaired) > 0 {
		lines = append(lines, "Programm: MCP-Registrierung auf das installierte "+project.InstalledCommandName+" korrigiert: "+strings.Join(result.MCPRepaired, ", ")+".")
	}
	if len(result.MCPRepairedPortable) > 0 {
		lines = append(lines, "Programm: in "+strings.Join(result.MCPRepairedPortable, ", ")+" steht der bloße Name "+project.InstalledCommandName+
			", weil die Datei von git erfasst ist; ein aus Dock oder Finder gestarteter Client braucht dort den absoluten Pfad.")
	}
	if result.Message != "" {
		lines = append(lines, "Programm: "+result.Message)
	}
	return lines
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

func describeVersionSuffix(version string) string {
	if version == "" {
		return ""
	}
	return " (VERSION " + version + ")"
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
	fmt.Fprint(out, `k-playbook upgrade — Programm aktualisieren und das Projekt nachziehen

Aufruf:
  k-playbook upgrade [--check] [--json]

Zwei Schritte, je Schritt eine Meldung:

  1. Programm: git pull --ff-only im Clone k-playbook/, danach das Binary über
     k-playbook/bin/install — aber nur, wenn das laufende Programm älter ist
     als die VERSION des Clones; gleich oder neuer bleibt stehen. Ohne Clone
     oder ohne Netz eine Meldung, kein Abbruch; scheitert Download oder
     Install, eine eigene Meldung, und Schritt 2 läuft mit dem laufenden
     Programm. Hat das Binary gewechselt, führt das neu installierte
     Programm Schritt 2 aus.
  2. Projekt: AGENTS.md (Anstoß anlegen oder anhängen, einen veralteten
     Anstoß auf k-playbook briefing bringen, die abgelösten Session-Memory-
     und Befunde-Blöcke entfernen), CLAUDE.md als Include-Datei, die
     Verlinkung der Commands und Skills; die MCP-Einträge korrigiert der
     Pull, ohne Pull dieser Schritt. Projekt-Prosa in AGENTS.md und
     k-playbook-local/ bleiben unangetastet.

Am Ende, wenn das Binary gewechselt hat: der Hinweis, die Sitzung neu zu
starten — der MCP-Server der Sitzung behält das alte Programm.

--check schreibt nichts: es meldet den Stand des Clones gegenüber dem Remote,
den Stand des laufenden Programms gegenüber dem Clone und, was ein Lauf am
Projekt täte. --json gibt den Bericht als JSON aus (schemaVersion, check,
program, project, restartRequired, messages).
`)
}
