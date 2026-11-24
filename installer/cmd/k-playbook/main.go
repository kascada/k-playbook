// Command k-playbook ist das Werkzeug für die projektlokale k-playbook-Installation.
//
// Ohne Argument startet es die lokale Oberfläche. Das Unterkommando `mcp`
// bietet Briefing, Dokumente, Todos und die übrigen Auskünfte einem Assistenten
// als MCP-Werkzeuge an, `scan` führt die Werkzeug-Einträge
// eines Review-Laufs aus, `merge` fasst einen Lauf als Review-Input zusammen,
// `inventory` erhebt das Versionsinventar des Projekts, `docs` holt erzeugte
// Doku in ein Arbeitsverzeichnis und veröffentlicht sie, `todo` verwaltet die
// Todos, `findings check` prüft die Befund-Queue vor ihrer Einordnung, `read` löst einen Verweis im Präfixschema auf, `briefing` listet, was für das
// Projekt gilt und was ansteht, `health` nennt Fehler und Warnungen über die
// Umgebung des Projekts, `github` liest und prüft die persönliche
// GitHub-Zugangsdatei, `upgrade` aktualisiert das Programm und zieht
// das Projekt nach, `maintain` führt den Wartungslauf aus (Doku-Sicherung),
// `vscode` installiert die mitgereiste VS-Code-Erweiterung ausdrücklich,
// `version` nennt die Version des Programms, `gui status` nennt Zustand und
// Adresse der Oberfläche, ohne sie anzufassen, `stop` beendet den
// Hintergrunddienst der Oberfläche, `notify` zeigt dem Nutzer eine
// Meldung auf diesem Rechner, und `lock` ist die Sperre des Arbeitsbaums.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/legacy"
	"github.com/kascada/k-playbook/installer/internal/mcpserver"
	"github.com/kascada/k-playbook/installer/internal/procgroup"
	"github.com/kascada/k-playbook/installer/internal/release"
	"github.com/kascada/k-playbook/installer/internal/vscodeext"
	"github.com/kascada/k-playbook/installer/internal/webui"
)

func main() {
	ensureShare(os.Args[1:])
	err := run(os.Args[1:])
	// Beendet gerade ein Signal das Programm (procgroup.NotifyContext), endet
	// es durch das Signal: kein eigener Fehler, kein eigener Exit-Code.
	procgroup.HoldIfEnding()
	if err != nil {
		// Ein exitError ohne Text hat schon alles ausgegeben und setzt nur
		// den Code; alles andere endet mit 1.
		if err.Error() != "" {
			fmt.Fprintf(os.Stderr, "Fehler: %v\n", err)
		}
		os.Exit(exitCodeOf(err))
	}
}

func run(args []string) error {
	if len(args) == 0 {
		if guiproc.ServeMode() {
			// Der abgekoppelte Server, gestartet vom argumentlosen Aufruf.
			// Keine Wirt-Pflege und kein Browser: beides gehört zum Aufruf,
			// der jedes Mal läuft — hier liefe es nur beim allerersten Start.
			// Die Marke wird gelöscht, damit kein Kindprozess des Servers sie
			// erbt und selbst zum Server wird.
			os.Unsetenv(guiproc.ServeEnv)
			if guiproc.HostCareMode() {
				// Neustart nach einer Programmaktualisierung: die Pflege, die
				// sonst der argumentlose Aufruf übernimmt, läuft hier mit dem
				// Code dieses Programms. Ausgaben gehen ins Log.
				os.Unsetenv(guiproc.HostCareEnv)
				careForHost()
			}
			return webui.Serve()
		}
		return runGUI()
	}

	switch args[0] {
	case "config":
		return runConfig(args[1:])
	case "context":
		// Entfernt mit Task 188. Kein stilles „unbekanntes Kommando“: Eine
		// Sitzung in einem Projekt mit altem Anstoß in AGENTS.md ruft den
		// Befehl noch, bevor Einrichten, Oberfläche oder upgrade den Anstoß
		// ersetzt haben — sie soll in einem Satz lesen, was an seine Stelle
		// tritt. Ohne Hilfetext, ohne Wirt-Pflege.
		return errContextRemoved
	case "mcp":
		// Ohne Wirt-Pflege: stdout trägt den JSON-RPC-Strom, und eine einzige
		// fremde Zeile daneben macht die Verbindung unbrauchbar.
		// Mit Signal-Handler: Ein SIGINT oder SIGTERM beendet erst die
		// Prozessgruppen, die ein Werkzeugaufruf gestartet hat, dann den
		// Server (Task 152). Der Kontext reicht bis in die Werkzeugaufrufe.
		ctx, stop := procgroup.NotifyContext(context.Background())
		defer stop()
		return mcpserver.Run(ctx)
	case "scan":
		// Ohne Wirt-Pflege: ein Scan liest nur und soll den Host nicht nebenbei
		// anfassen, während er läuft. Zurzeit gesperrt (review.Paused).
		return runScan(args[1:])
	case "merge":
		// Wie scan: ein Merge arbeitet auf einem vorhandenen Lauf und fasst dessen
		// Artefakte zusammen, ohne den Host nebenbei anzufassen. Zurzeit gesperrt
		// wie scan.
		return runMerge(args[1:])
	case "inventory":
		// Wie scan und merge ohne Wirt-Pflege: die Erhebung liest die
		// deklarativen Quellen des Projekts und schreibt genau eine Datei
		// darunter — den Host fasst sie dabei nicht nebenbei an. Dazu kommt:
		// die Meldungen der Bereinigung gingen nach stdout und stünden mitten
		// im Bericht des Laufs, in dem jede Ablehnung sichtbar sein muss.
		return runInventory(args[1:])
	case "todo":
		// Ohne Wirt-Pflege wie briefing: die Ausgabe ist
		// maschinenlesbar und darf keine Bereinigungsmeldungen tragen.
		return runTodo(args[1:])
	case "task":
		// Ohne Wirt-Pflege wie todo: die Ausgabe ist JSON und sonst nichts.
		return runTask(args[1:])
	case "read":
		// Ohne Wirt-Pflege wie todo: stdout trägt den Inhalt hinter dem
		// Verweis und sonst nichts.
		return runRead(args[1:])
	case "doc":
		// Ohne Wirt-Pflege wie read: die Ausgabe ist der Verweis oder JSON
		// und sonst nichts.
		return runDoc(args[1:])
	case "docs":
		// Ohne Wirt-Pflege wie doc: die Ausgabe ist je Dokument eine Zeile
		// oder JSON und sonst nichts.
		return runDocs(args[1:])
	case "findings":
		// Ohne Wirt-Pflege wie health: die Ausgabe ist das Ergebnis der
		// Vorprüfung, als Klartext oder JSON, und ein Skript liest den
		// Exit-Code.
		return runFindings(args[1:])
	case "briefing":
		// Ohne Wirt-Pflege: die Ausgabe ist das Briefing, als Klartext oder
		// JSON, und sonst nichts — eine Bereinigungsmeldung daneben störte
		// die JSON-Ausgabe.
		return runBriefing(args[1:])
	case "health":
		// Ohne Wirt-Pflege wie briefing: die Ausgabe ist die Liste der
		// Probleme, als Klartext oder JSON, und sonst nichts — und ein
		// Skript liest den Exit-Code.
		return runHealth(args[1:])
	case "github":
		// Ohne Wirt-Pflege wie health: die Ausgabe ist die Konfiguration
		// oder die Problemliste der persönlichen GitHub-Zugangsdatei, als
		// Klartext oder JSON, und ein Skript liest den Exit-Code. Die Datei
		// ist persönlich, kein Projekt wird gesucht.
		return runGitHub(args[1:])
	case "upgrade":
		// Ohne Wirt-Pflege: der Nachzug meldet selbst je Schritt, was er tut,
		// und seine Ausgabe wird von /k-upgrade wörtlich wiedergegeben — eine
		// Bereinigungsmeldung dazwischen gehörte nicht dazu.
		return runUpgrade(args[1:])
	case "maintain":
		// Ohne Wirt-Pflege: der Wartungslauf soll genau seine Schritte tun
		// und nichts daneben — und er läuft später aus einem Cron-Job, dessen
		// Ausgabe gelesen wird.
		return runMaintain(args[1:])
	case "vscode":
		// Ohne Wirt-Pflege: der ausdrückliche Weg zur VS-Code-Erweiterung
		// soll genau das tun, wonach er gefragt wurde, und nichts daneben.
		return runVSCode(args[1:])
	case "version":
		// Ohne Wirt-Pflege: die Ausgabe ist die Version und sonst nichts —
		// die Oberfläche liest sie an der Datei am Installationsziel, bevor
		// sie entscheidet, ob nachinstalliert wird.
		return runVersion(os.Stdout, args[1:])
	case "gui":
		// Ohne Wirt-Pflege aus demselben Grund wie briefing und todo: die
		// Ausgabe ist maschinenlesbar und darf keine Meldungen der
		// Bereinigung tragen. Dazu kommt der Grund von stop, nur umgekehrt:
		// wer nach dem Zustand fragt, will nichts eingerichtet bekommen.
		return runGUICommand(args[1:])
	case "stop":
		// Ohne Wirt-Pflege: wer beendet, will nichts einrichten.
		return runStop(os.Stdout)
	case "notify":
		// Ohne Wirt-Pflege: eine Meldung soll genau das tun, wonach sie
		// gefragt wurde, und nichts daneben — und sie läuft später aus
		// Hooks, deren Ausgabe niemand liest.
		return runNotify(os.Stdout, args[1:])
	case "lock":
		// Ohne Wirt-Pflege wie health: die Ausgabe ist der Stand der Sperre,
		// als Klartext oder JSON, und ein Skript liest den Exit-Code.
		return runLock(os.Stdout, args[1:])
	case "troubleshooting":
		// Ohne Wirt-Pflege wie health: der Befehl soll gerade bei einer
		// kaputten Umgebung laufen und nichts daneben anfassen.
		return runTroubleshooting(os.Stdout, args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unbekanntes Kommando: %s", args[0])
	}
}

// Für Tests austauschbar: wohin der Hinweis auf --all geht (stderr) und ob
// stdout ein Terminal ist.
var (
	versionHintOut          io.Writer = os.Stderr
	versionStdoutIsTerminal           = func() bool {
		info, err := os.Stdout.Stat()
		return err == nil && info.Mode()&os.ModeCharDevice != 0
	}
)

// versionAllHint ist der Hinweis von `version` auf `version --all`.
const versionAllHint = "Hinweis: k-playbook version --all nennt zusätzlich die mitgereiste VS-Code-Erweiterung und das neueste Release."

// runVersion gibt die gestempelte Version aus, eine Zeile, sonst nichts. Ein
// Programm ohne Version — ein Ad-hoc-`go build` ohne Build-Flags — endet mit
// Fehler und leerer Standardausgabe: eine erfundene Angabe wäre schlechter als
// keine, denn der Leser vergleicht sie.
//
// `version --all` nennt zusätzlich die Version der mitgereisten
// VS-Code-Erweiterung und den neuesten Release-Tag auf GitHub. Nur hinter dem
// Schalter: `program.ReadVersion` führt `<datei> version` aus und verwirft die
// Antwort, sobald sie mehr als eine Zeile trägt — mit einer zweiten Zeile am
// Standardweg erkennt kein Aufruf mehr einen Programmwechsel. Aus demselben
// Grund ruft `version` ohne Schalter den Tag nicht ab: kein Netzaufruf, auch
// nicht ohne Netz ein Warten.
//
// Auf `--all` weist `version` selbst hin — nur auf stderr und nur, wenn stdout
// ein Terminal ist: Wer die Ausgabe liest oder umleitet (ReadVersion, ein
// Skript), bekommt genau die eine Zeile.
//
// Ist der neueste Tag unbekannt (ohne Netz, ohne veröffentlichtes Release, Zeitüberschreitung),
// steht dort „unbekannt“ mit dem Grund; der Exit-Code ändert sich dadurch
// nicht. Eine über K_PLAYBOOK_RELEASE_SOURCE ersetzte Quelle wird genannt.
func runVersion(out io.Writer, args []string) error {
	all := false
	switch {
	case len(args) == 0:
	case len(args) == 1 && args[0] == "--all":
		all = true
	default:
		return fmt.Errorf("version erwartet höchstens --all: %s", args[0])
	}

	version := guiproc.OwnVersion()
	if version == "" {
		return errors.New("dieses Programm trägt keine Version; es wurde ohne Build-Flags gebaut")
	}
	if _, err := fmt.Fprintln(out, version); err != nil {
		return err
	}
	if !all {
		if versionStdoutIsTerminal() {
			fmt.Fprintln(versionHintOut, versionAllHint)
		}
		return nil
	}

	extension, ok, err := vscodeext.EmbeddedVersion()
	switch {
	case err != nil:
		_, err = fmt.Fprintf(out, "Erweiterung: unlesbar — %v\n", err)
	case !ok:
		_, err = fmt.Fprintf(out, "Erweiterung: keine — %s\n", vscodeext.NoVSIX)
	default:
		_, err = fmt.Fprintf(out, "Erweiterung: %s\n", extension)
	}
	if err != nil {
		return err
	}

	ctx, stop := procgroup.NotifyContext(context.Background())
	defer stop()
	latest, tagErr := release.LatestTag(ctx)
	switch note := release.CurrentSource().Note(); {
	case tagErr != nil:
		_, err = fmt.Fprintf(out, "Neuestes Release: unbekannt — %s\n", strings.TrimSuffix(tagErr.Error(), "."))
	case note != "":
		_, err = fmt.Fprintf(out, "Neuestes Release: %s (%s)\n", latest, note)
	default:
		_, err = fmt.Fprintf(out, "Neuestes Release: %s\n", latest)
	}
	return err
}

// errContextRemoved ist die Antwort auf das entfernte Unterkommando context:
// ein Satz auf stderr (über main, mit „Fehler: “ davor) und Exit-Code 1.
var errContextRemoved = errors.New("das Kommando context ist entfernt — an seine Stelle tritt k-playbook briefing, " +
	"die Umgebungsangaben liefert k-playbook briefing --part environment")

func printUsage() {
	fmt.Fprint(os.Stderr, `k-playbook

Ohne Argument:  startet die lokale Oberfläche im Browser. Der Server läuft
                dazu als Hintergrunddienst je Projekt weiter; ein zweiter
                Aufruf findet ihn und öffnet nur den Browser.

Unterkommandos:
  config create [--repo-root <pfad>] [hauptverzeichnis]
             Legt K-PLAYBOOK.yaml explizit an. Ohne Hauptverzeichnis wird das
             Arbeitsverzeichnis verwendet; eine vorhandene Datei bleibt immer
             unverändert.
  mcp       Startet einen MCP-Server über stdin/stdout, der Briefing,
            Dokumente, Todos und die übrigen Auskünfte als Werkzeuge anbietet.
            Gedacht für den Aufruf durch einen Assistenten, nicht für die Hand.
  scan      Vorübergehend nicht verfügbar: Die Ablage der Review-Ergebnisse
            wird umgebaut; das Kommando endet mit der Sperre und Exit-Code 1
            und schreibt nichts. Sonst: führt die Werkzeug-Einträge eines
            Review-Laufs aus, k-playbook scan <lauf> [eintrag …]. Ohne
            Eintragsangabe alle, die noch nicht gelaufen sind. Die Namen sind
            die der Werkzeuge. Das Kommando kehrt zurück, wenn alle Einträge
            durch sind; der Fortschritt steht währenddessen unter entries/.
  merge     Vorübergehend nicht verfügbar, wie scan. Sonst: fasst einen
            bestehenden Review-Lauf zusammen, k-playbook merge <lauf>. Das
            Ergebnis wird als Review-Input in das Laufverzeichnis geschrieben.
  inventory Erhebt das Versionsinventar dieses Projekts und schreibt es in
            die Collection des Projekts nach
            keph:/docs/generated/versions/inventory.md: k-playbook inventory,
            ohne weitere Argumente. Gelesen werden nur deklarative Quellen —
            Manifeste, Lockfiles, Container-, DevContainer-, Helm- und
            CI-Dateien —, nur unterhalb der Projektwurzel, dazu die im
            Frontmatter von keph:/k-playbook/inventory.md konfigurierten.
            Ein Lauf ohne inhaltliche Änderung lässt das Dokument
            unangetastet. Ohne bereite Kephalaion-Anbindung bricht der Lauf
            ab (kephalaion_unavailable); einen Rückfall auf die Platte gibt
            es nicht. Vertrag: k-playbook:/docs/handbook/version-inventory.md.
  todo      Verwaltet die Todos des Projekts in der Projekt-Collection
            (todos/): k-playbook todo list|add|update|delete|import. Die
            Ausgabe ist JSON auf stdout. Ohne Unterbefehl
            und mit --help erscheint nur die Kurzhilfe, ohne dass dabei Daten
            gelesen oder geschrieben werden.
  task      Legt Tasks in der Projekt-Collection an: k-playbook task create
            --title <titel> --name <kurzname> [--dry-run] [--no-intent],
            Rumpf von stdin. Prüft, nummeriert und schreibt in einem Zug;
            die Ausgabe ist JSON. Ohne Unterbefehl nur die Kurzhilfe.
  read      Löst einen Verweis im Präfixschema auf und gibt aus, was
            dahinter steht: k-playbook read <verweis> [--raw] [--json].
            project: eine Datei des Projekts, keph: ein Dokument der
            Collection über den Node, k-playbook: die Sorten instructions/,
            reviews/ und commands/, lokal vor mitgeliefert, und guidelines/
            sowie docs/ (das Handbuch docs/handbook/<seite>.md und die
            Vorlagen docs/<pfad>/README.md), nur mitgeliefert; ein Verweis auf instructions/always/ mit / am Ende
            baut das Verzeichnis zu einem Text zusammen. Include-Zeilen
            (include: <verweis>) werden eingesetzt, --raw lässt sie stehen.
            Hinweise stehen auf stderr. Dasselbe wie das MCP-Werkzeug
            k_playbook_doc_read.
  findings  Die Vorprüfung der Befund-Queue vor /k-befunde-einordnen:
            k-playbook findings check [--json]. Gleicht die Replica ab
            (kephalaion node sync), prüft die Rechte read, write und
            supersede, jeden Eintrag unter keph:/queue/findings/ auf das
            Namensmuster eines Befunds und liest jeden Befund; leere und
            in den letzten sechs Stunden geänderte werden übersprungen. Ergebnis: bereit oder
            nicht, mit allen Gründen, dazu der Bestand je Nutzer. Schreibt
            nichts. Exit-Code 0 bereit, 1 nicht bereit, 2 bei
            Aufruffehlern. Dasselbe wie das MCP-Werkzeug
            k_playbook_findings_check.
  briefing  Gibt aus, was für dieses Projekt gilt und was ansteht:
            k-playbook briefing [--json]. Vorweg Datum, Projekt und
            Kephalaion-Anbindung (now, project, kephalaion), dann der Text
            der immer geltenden Anweisungen
            k-playbook:/instructions/always/, die Umgebungsprüfung, dann
            Anweisungen auf Abruf, Reviews und Leitlinien — lokal und
            mitgeliefert aufgelöst, je Eintrag Verweis und Titel —, dazu aus
            der Projekt-Collection die eigenen offenen Tasks und
            Arbeitsstände (je höchstens 10) und die Zahl offener Befunde,
            der Einstieg der Projektdoku und der GitHub-Dienst dieses Repos.
            Verweise, keine Pfade. Zieht dabei die Assistenten-Verlinkung
            nach. Mit --part environment kommen statt des Briefings nur die
            Umgebungsangaben, die einzelne Commands brauchen (gh, github,
            baseTools, versionSources, docs, remediation). Dasselbe wie das
            MCP-Werkzeug k_playbook_briefing.
  health    Nennt Fehler und Warnungen über die Umgebung des Projekts:
            k-playbook health [--json]. Je Eintrag Stufe (error, warning),
            Bereich (kephalaion, gh, baseTools, versionSources, github,
            program), ein
            Satz und die Abhilfe; alle auf einmal, nichts wird repariert
            oder angelegt, kein Token oder Wert steht darin. Ohne Problem die eine Zeile
            „Umgebung: keine Probleme“ — derselbe Block wie im Briefing.
            Exit-Code 0 ohne Fehler (Warnungen zählen nicht), 1 mit
            Fehler, 2 bei Aufruffehlern. Dasselbe wie das MCP-Werkzeug
            k_playbook_health.
  troubleshooting
            Hilfe bei Fehlern der Umgebung: k-playbook troubleshooting.
            Soll zuerst prüfen, ob Kephalaion erreichbar ist, und dann
            Schritt für Schritt den nächsten Fehler suchen. Noch nicht
            umgesetzt: gibt nur aus, was der Befehl werden soll, und endet
            mit 0. Das Briefing nennt ihn, wenn Kephalaion nicht erreichbar
            ist.
  github    Legt an, liest, prüft und testet die persönliche GitHub-
            Zugangsdatei ~/.config/k-playbook/github.yaml: k-playbook github
            init|check|show|test [<dienst>] [--json]. init legt die Vorlage
            an (als einziger; eine vorhandene Datei bleibt). check nennt alle Probleme,
            sortiert nach Position in der Datei, sonst Datei, Zahl der
            Dienste und die Zugänge, die auf diesem Rechner nicht verfügbar
            sind; show zeigt die Konfiguration, je Dienst Repository und
            Zugänge mit Verfügbarkeit; test fragt GitHub, ob die verfügbaren
            Zugänge funktionieren — als einziger mit Netz, nur auf diesen
            Aufruf hin, dasselbe wie das MCP-Werkzeug k_playbook_github_test.
            Fehlt die Datei, nennen sie den Hinweis auf init und enden mit 0.
            Exit-Code 0 ohne Problem, 1 mit error (test: mit failed; init:
            nicht anlegbar), 2 bei Aufruffehlern.
            Vertrag: k-playbook:/docs/handbook/github-access.md.
  doc       Nummerierte Dokumente in der Kephalaion-Collection des
            Projekts: k-playbook doc create|list|done|reopen [--json].
            Legt an, listet und verschiebt nach done/ und zurück; den
            Inhalt liest und schreibt man direkt über Kephalaion. Dasselbe
            wie die MCP-Werkzeuge k_playbook_doc_create, _list, _done und
            _reopen.
  docs      Erzeugte Doku in der Projekt-Collection auschecken und
            veröffentlichen: k-playbook docs checkout <erzeuger> [--discard]
            und k-playbook docs publish <erzeuger> [--allow-hints], je mit
            --json. checkout holt keph:/docs/generated/<erzeuger>/ nach
            .k-playbook/work/docs/<erzeuger>/, publish gleicht ab und meldet
            Konflikte, statt zu überschreiben. Erzeuger: code, libs. Dazu
            k-playbook docs exclude add <muster>… [--json]: trägt dauerhafte
            Ausschlüsse für den Code-Scan in docs.exclude von K-PLAYBOOK.yaml
            ein. Dasselbe wie die MCP-Werkzeuge k_playbook_docs_checkout,
            _publish und _exclude_add.
  upgrade   Aktualisiert Kephalaion und das Programm und zieht das Projekt
            nach: k-playbook upgrade [--check] [--json]. Zuerst Kephalaion
            (kephalaion upgrade --auto; fehlt es im PATH, wird es
            installiert); dann das Release-Binary des neuesten Tags direkt
            von GitHub, geprüft gegen die SHA256SUMS des Tags — nur wenn das
            laufende Programm älter ist als der neueste Release-Tag; dann
            AGENTS.md (Anstoß), CLAUDE.md, Verlinkung und MCP-Einträge. Je
            Schritt eine Meldung, am Ende der Hinweis auf den Neustart der
            Sitzung, wenn das Binary gewechselt hat. --check schreibt nichts
            und meldet, was ein Lauf täte. Das Gegenstück zu /k-upgrade.
  maintain  Wartungslauf des Projekts, ohne KI und ohne Rückfrage, auch für
            einen Cron-Job: k-playbook maintain [--only <schritt>[,…]]
            [--dry-run] [--commit]. Ohne --only das Tagesprogramm aus
            maintenance.daily von K-PLAYBOOK.yaml (ohne Feld docs-backup).
            Schritt docs-backup: spiegelt keph:/docs/ über kephalaion node
            dir pull nach maintenance.backup.target, nur wenn
            maintenance.backup.enabled true ist; mit --commit oder
            maintenance.backup.git.commit danach ein Commit nur dieses
            Ziels, nie ein Push. Je Schritt Status und Grund; Exit-Code 0
            ohne Fehler, 1 mit Fehler, 2 bei Aufruffehlern. Mehr:
            k-playbook maintain --help.
  vscode    Die VS-Code-Erweiterung „k-playbook Workspace Tools“, die in
            diesem Programm steckt: k-playbook vscode install|status|
            vsix -o <datei>. Beim Start der Oberfläche wird sie selbsttätig
            installiert und nachgezogen; dieses Kommando ist der
            ausdrückliche Weg für den, der die Oberfläche nie startet.
  version   Gibt die Version dieses Programms aus, eine Zeile, sonst nichts.
            Ohne gestempelte Version endet es mit Fehler. Mit --all stehen
            darunter die Version der mitgereisten VS-Code-Erweiterung und der
            neueste Release-Tag auf GitHub (ohne Netz: unbekannt, mit Grund);
            im Terminal weist version auf stderr auf --all hin.
  gui       Auskunft über die Oberfläche dieses Projekts, ohne sie anzufassen:
            k-playbook gui status. Die Antwort ist JSON mit "state" (none,
            running, otherVersion, orphaned, unresponsive) und dazu, soweit
            gemessen, "url", "pid", "version" und "build"; nicht gemessene
            Felder fehlen. Gestartet wird dabei nichts — das tut der
            argumentlose Aufruf. Dasselbe wie das MCP-Werkzeug
            k_playbook_gui_status.
  stop      Beendet den Hintergrunddienst der Oberfläche für dieses Projekt.
            Ohne laufenden Server eine Auskunft, kein Fehler; eine verwaiste
            Laufzeitdatei wird dabei entfernt.
  notify    Zeigt dem Nutzer eine Meldung auf diesem Rechner: k-playbook
            notify --title <text> [--body <text>] [--sound] [--json]. Ein
            Popup, mit --sound dazu ein Ton; unter WSL über die Windows-
            Seite, auf macOS über osascript, auf einem Linux-Desktop über
            notify-send. Ohne Weg zum Nutzer — etwa im Devcontainer — endet
            es mit Fehler und nennt den Grund. Dasselbe wie das MCP-Werkzeug
            k_playbook_notify.
  lock      Die Sperre des Arbeitsbaums, die /k-task-run nimmt: k-playbook
            lock status|release|acquire|wait|check [--token <marke>]
            [--force] [--json]. status nennt den Halter oder frei; release
            --force gibt eine Sperre frei, deren Sitzung ohne Freigabe
            endete. Exit-Code 3 bei belegt oder verloren. Dasselbe wie das
            MCP-Werkzeug k_playbook_lock.
  help      Diese Übersicht.
`)
}

// cleanUpLegacy räumt die host-globale Registrierung des alten Modells weg.
// Sie läuft bei jedem Start und meldet sich nur, wenn tatsächlich etwas
// wegfällt — auf einem sauberen Rechner bleibt sie still. Ein Fehler dabei
// hält den Start nicht auf: die Oberfläche ist auch mit Altlasten bedienbar.
func cleanUpLegacy() {
	removals, err := legacy.RemoveGlobalLinks()
	if len(removals) > 0 {
		fmt.Printf("Alte globale Verlinkung entfernt (%d):\n", len(removals))
		for _, removal := range removals {
			fmt.Printf("  - %s\n", removal)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: alte globale Verlinkung nicht vollständig entfernt: %v\n", err)
	}
}
