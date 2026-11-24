package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
	"github.com/kascada/k-playbook/installer/internal/procgroup"
	"github.com/kascada/k-playbook/installer/internal/project"
)

// runBriefing gibt das Briefing aus — dieselbe Fachlogik, die das MCP-Werkzeug
// k_playbook_briefing als Hülle anbietet: welche Anweisungen, Reviews und
// Leitlinien für dieses Projekt gelten und was in der Projekt-Collection
// für die eigene Arbeit ansteht, als Verweise im Präfixschema; vorweg Datum,
// Projekt und Anbindung, der Text der immer geltenden Anweisungen und die
// Umgebungsprüfung. Ohne --json steht der Klartext zum Lesen; mit --json die
// Antwort als JSON, dieselbe Serialisierung wie über MCP. Mit --part
// environment kommt statt des Briefings nur der Teilabruf environment
// (project.BuildBriefingEnvironment).
func runBriefing(args []string) error {
	asJSON := false
	part, partGiven := "", false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			asJSON = true
		case arg == "--part":
			if i+1 >= len(args) {
				return fmt.Errorf("--part braucht einen Wert — erwartet wird --part %s", project.BriefingPartEnvironment)
			}
			i++
			part, partGiven = args[i], true
		case strings.HasPrefix(arg, "--part="):
			part, partGiven = strings.TrimPrefix(arg, "--part="), true
		case arg == "-h" || arg == "--help" || arg == "help":
			printBriefingUsage(os.Stdout)
			return nil
		default:
			return fmt.Errorf("unbekannte Option %s — erwartet wird briefing [--part %s] [--json]", arg, project.BriefingPartEnvironment)
		}
	}
	// Ein unbekannter Teil ist ein Fehler, kein volles Briefing: Wer einen
	// Teil verlangt, soll nicht stattdessen eine Sitzungskennung bekommen.
	if partGiven && part != project.BriefingPartEnvironment {
		return fmt.Errorf("unbekannter Teil %q — erwartet wird --part %s", part, project.BriefingPartEnvironment)
	}

	projectDir, err := todoProjectDir()
	if err != nil {
		return err
	}
	// github ab dem Arbeitsverzeichnis: Liegt es in einem eigenen Repo
	// unterhalb des Projekts, zählt dessen Remote.
	workdir, err := os.Getwd()
	if err != nil {
		return err
	}
	if part == project.BriefingPartEnvironment {
		return writeBriefingEnvironment(os.Stdout, project.BuildBriefingEnvironment(projectDir, workdir), asJSON)
	}
	ctx, stop := procgroup.NotifyContext(context.Background())
	defer stop()
	briefing, err := project.BuildBriefingFrom(ctx, projectDir, workdir)
	if err != nil {
		return err
	}

	if asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(briefing)
	}
	return writeBriefingText(os.Stdout, briefing)
}

// briefingEnvironmentOutput ist die JSON-Form des Teilabrufs: der Teil unter
// seinem Namen, damit die Felder so heißen, wie die Texte sie nennen
// (environment.gh.ready).
type briefingEnvironmentOutput struct {
	Environment project.BriefingEnvironment `json:"environment"`
}

// writeBriefingEnvironment gibt den Teilabruf environment aus: als JSON das
// Objekt unter environment, als Klartext dieselben Felder eingerückt unter der
// Zeile „environment:“ — je Feld eine Zeile, Objekte mit ihren Feldern
// alphabetisch, Listen von Werten in einer Zeile, Listen von Objekten mit
// einem Eintrag je Zeile. Ein Teil, der sich nicht bauen lässt, ist ein
// Zustand in der Ausgabe, kein Fehler des Aufrufs.
func writeBriefingEnvironment(out io.Writer, environment project.BriefingEnvironment, asJSON bool) error {
	if asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(briefingEnvironmentOutput{Environment: environment})
	}
	var b strings.Builder
	if !environment.Available {
		fmt.Fprintf(&b, "environment: nicht verfügbar — %s\n", environment.Message)
		_, err := io.WriteString(out, b.String())
		return err
	}
	raw, err := json.Marshal(environment)
	if err != nil {
		return err
	}
	value, ok := decodeContextValue(raw).(map[string]any)
	if !ok {
		return fmt.Errorf("Teilabruf environment lässt sich nicht darstellen")
	}
	delete(value, "available")
	b.WriteString("environment:\n")
	// Die Reihenfolge der Abschnitte ist die des Typs, nicht das Alphabet.
	for _, name := range []string{"gh", "github", "baseTools", "versionSources", "docs", "remediation"} {
		if field, ok := value[name]; ok {
			writeContextValue(&b, "  ", name, field)
		}
	}
	_, err = io.WriteString(out, b.String())
	return err
}

// writeBriefingText rendert das Briefing zum Lesen: zuerst der Kopf
// (briefingTextHead), dann der Text der immer geltenden Anweisungen, so wie `k-playbook read` ihn liefert — ohne die Liste
// seiner Teile, die Herkunft steht nur in der JSON-Antwort —, dann der Block
// der Umgebungsprüfung (healthText, derselbe wie bei `k-playbook health`),
// dann je Abschnitt die Dokumente als Verweis und Titel (briefingEntryLine),
// dann die eigene Arbeit (writeBriefingWorkList).
func writeBriefingText(out io.Writer, briefing project.Briefing) error {
	var b strings.Builder
	always := briefing.Documents.Always
	b.WriteString(briefingTextHead(briefing))
	if strings.TrimSpace(always.Content) == "" {
		fmt.Fprintf(&b, "(keine immer geltenden Anweisungen unter %s)\n", always.Ref)
	} else {
		b.WriteString(strings.TrimRight(always.Content, "\n") + "\n")
	}
	for _, problem := range always.Problems {
		fmt.Fprintf(&b, "Hinweis: %s\n", problem)
	}
	// Die Umgebungsprüfung unmittelbar nach dem Text der immer geltenden
	// Anweisungen und vor den Verweisen: so liest die Sitzung sie, bevor sie
	// etwas tut. Ohne Problem eine Zeile.
	b.WriteString("\n" + healthText(briefing.Health))
	b.WriteString("\n")
	entries := func(list []project.BriefingEntry) {
		if len(list) == 0 {
			b.WriteString("  (keine)\n")
			return
		}
		for _, entry := range list {
			b.WriteString(briefingEntryLine(entry))
		}
	}

	b.WriteString("Auf Abruf:\n")
	entries(briefing.Documents.OnDemand)
	b.WriteString("Reviews:\n")
	entries(briefing.Documents.Reviews)
	b.WriteString("Leitlinien:\n")
	entries(briefing.Documents.Guidelines)

	work := briefing.Work
	if !work.Available {
		fmt.Fprintf(&b, "Eigene Arbeit: Kephalaion nicht nutzbar — %s\n", work.Message)
	} else {
		fmt.Fprintf(&b, "Eigene Arbeit (%s):\n", work.User)
		writeBriefingWorkList(&b, "Offene Tasks", "", "tasks", work.User, work.Tasks, work.Total.Tasks)
		writeBriefingWorkList(&b, "Aktive Arbeitsstände", "zuletzt geänderte zuerst", "workbench", work.User, work.Workbench, work.Total.Workbench)
		fmt.Fprintf(&b, "  Offene Befunde: %d\n", work.OpenFindings)
	}
	fmt.Fprintf(&b, "Projektdoku: %s\n", briefing.ProjectDocs)
	// Die Dienste der GitHub-Zugangsdatei als eigene Zeile, nicht im Block
	// Umgebung: Der gehört zur Umgebungsprüfung (healthText).
	if line := briefingGitHubLine(briefing.GitHub.Access); line != "" {
		b.WriteString(line + "\n")
	}
	b.WriteString(briefingEnvironmentHint)

	_, err := io.WriteString(out, b.String())
	return err
}

// briefingEnvironmentHint ist die letzte Zeile des Klartexts: wo die
// Umgebungsangaben liegen, die das Briefing nicht mehr trägt.
const briefingEnvironmentHint = "Umgebungsdetails (gh, github, baseTools, versionSources, docs, remediation) auf Abruf: " +
	"k-playbook briefing --part environment bzw. k_playbook_briefing mit part \"environment\"\n"

// briefingEntryLine ist die Zeile eines Dokuments unter „Auf Abruf“, „Reviews“
// und „Leitlinien“: Verweis und Titel. Die Herkunft steht nur da, wenn sie
// nicht dist ist — mitgeliefert ist der Normalfall und sagt der Sitzung
// nichts —, bei einem projekteigenen Teil aus der Collection mit seinem
// keph:-Ort daneben: [local keph:/k-playbook/…]. Die Beschreibung steht nur
// im JSON (Task 187): Sie machte „Auf Abruf“ fünfmal so lang, und gefunden
// wird ein Dokument über Verweis und Titel.
func briefingEntryLine(entry project.BriefingEntry) string {
	line := "  " + entry.Ref
	if entry.Origin != project.DocOriginDist && entry.Origin != "" {
		origin := entry.Origin
		if entry.Location != "" {
			origin += " " + entry.Location
		}
		line += "  [" + origin + "]"
	}
	return line + "  " + entry.Title + "\n"
}

// writeBriefingWorkList rendert eine Liste der eigenen Arbeit: eine Kopfzeile
// mit der Zahl aller Dokumente und dem Verzeichnis-Verweis der Ablage,
// darunter je Eintrag Dateiname und Titel — der Verweis eines Eintrags ist
// Verzeichnis plus Dateiname; ihn je Zeile auszuschreiben kostete in einem
// Projekt mit viel offener Arbeit mehrere tausend Zeichen (Task 187). Zeigt
// die Liste nicht alle, nennt die letzte Zeile, wie viele fehlen und wie sie
// zu holen sind. total unter der Länge der Liste — ein Briefing aus einer
// älteren Fassung ohne total — gilt als die Länge.
func writeBriefingWorkList(b *strings.Builder, label string, order string, kind string, user string, list []project.BriefingDoc, total int) {
	if total < len(list) {
		total = len(list)
	}
	count := fmt.Sprint(total)
	if order != "" && total > 1 {
		count += ", " + order
	}
	fmt.Fprintf(b, "  %s (%s) in %s:\n", label, count, project.BriefingWorkDirRef(kind, user))
	if len(list) == 0 {
		b.WriteString("    (keine)\n")
		return
	}
	for _, doc := range list {
		fmt.Fprintf(b, "    %s  %s", path.Base(doc.Ref), doc.Title)
		// Ein Frontmatter-Problem steht hinter dem Titel, dort, wo die
		// Sitzung ohnehin hinsieht — nicht still der Dateiname als Titel.
		if doc.Problem != "" {
			fmt.Fprintf(b, " — %s", doc.Problem)
		}
		b.WriteString("\n")
	}
	if rest := total - len(list); rest > 0 {
		fmt.Fprintf(b, "    + %d weitere: k_playbook_doc_list (kind %s)\n", rest, kind)
	}
}

// briefingTextHead sind die ersten Zeilen des Klartexts: die Sitzungskennung,
// der Block der Abschnitte now, project, kephalaion und links unter ihren
// Feldnamen (briefingTextSections, höchstens acht Zeilen), die Aufforderung,
// die Ausgabe ganz zu lesen, und die Titel der immer geltenden Teile. Sie
// stehen vorn, weil Sitzungen das Briefing in der Shell kürzen — gemessen
// 2026-10-25: Claude Code las es mit `| head -80` und sah weder die Anweisung
// zu Befunden noch die Kennung, die damals am Ende stand. Was hier steht,
// übersteht jede Kürzung und sagt, was dahinter fehlt.
func briefingTextHead(briefing project.Briefing) string {
	var titles []string
	for _, part := range briefing.Documents.Always.Parts {
		if part.Origin != project.BriefingOriginDisabled {
			titles = append(titles, part.Title)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Sitzung: %s\n", briefing.Session)
	b.WriteString(briefingTextSections(briefing))
	b.WriteString("Lies diese Ausgabe ganz, nicht gekürzt: Alles darin gilt für die ganze Sitzung.\n")
	if len(titles) > 0 {
		fmt.Fprintf(&b, "Es gelten: %s\n", strings.Join(titles, " · "))
	}
	b.WriteString("\n")
	return b.String()
}

// briefingFieldSeparator trennt im Kopf die Felder eines Abschnitts.
const briefingFieldSeparator = " · "

// briefingTextSections ist der Block der Abschnitte, die jede Sitzung liest,
// je Abschnitt eine Zeile unter seinem Feldnamen, darin je Feld Name und
// Wert: `now: date … · timestamp …`. So findet ein Text, der project.dir oder
// kephalaion.user nennt, den Wert auch im Klartext — dem Ersatzweg über die
// Shell. Ein Abschnitt, der sich nicht bauen ließ, sagt das an seiner Stelle
// mit der Meldung, statt zu fehlen. links steht nur da, wenn es etwas meldet.
// Der Block hat höchstens acht Zeilen: drei feste, für links bis zu drei, eine
// je Unterfeld.
func briefingTextSections(briefing project.Briefing) string {
	var b strings.Builder
	fields := func(name string, pairs ...string) {
		parts := make([]string, 0, len(pairs)/2)
		for i := 0; i+1 < len(pairs); i += 2 {
			if pairs[i+1] == "" {
				continue
			}
			parts = append(parts, pairs[i]+" "+pairs[i+1])
		}
		fmt.Fprintf(&b, "%s: %s\n", name, strings.Join(parts, briefingFieldSeparator))
	}
	fields("now", "date", briefing.Now.Date, "timestamp", briefing.Now.Timestamp)
	section := briefing.Project
	if section.Available {
		fields("project", "dir", section.Dir, "repoRoot", section.RepoRoot, "vcs", section.VCS,
			"languages", strings.Join(section.Languages, ", "))
	} else {
		fields("project", "dir", section.Dir, "nicht verfügbar —", oneLine(section.Message))
	}
	keph := briefing.Kephalaion
	fields("kephalaion", "ok", fmt.Sprint(keph.OK), "address", keph.Address, "user", keph.User, "message", oneLine(keph.Message))
	if links := briefing.Links; links != nil {
		var healed []string
		for _, change := range []struct {
			label string
			names []string
		}{{"added", links.Healed.Added}, {"removed", links.Healed.Removed}, {"repointed", links.Healed.Repointed}} {
			if len(change.names) > 0 {
				healed = append(healed, change.label+" "+strings.Join(change.names, ", "))
			}
		}
		open := make([]string, 0, len(links.Open))
		for _, issue := range links.Open {
			open = append(open, fmt.Sprintf("%s (%s): %s", issue.Path, issue.State, oneLine(issue.Detail)))
		}
		for _, line := range []struct{ name, value string }{
			{"links.healed", strings.Join(healed, "; ")},
			{"links.open", strings.Join(open, "; ")},
			{"links.note", oneLine(links.Note)},
		} {
			if line.value != "" {
				fmt.Fprintf(&b, "%s: %s\n", line.name, line.value)
			}
		}
	}
	return b.String()
}

// oneLine macht aus einem Text eine Zeile: Zeilenumbrüche und Folgen von
// Leerraum werden ein Leerzeichen. Der Kopf des Klartexts hält so seine
// Zeilenzahl, auch wenn eine Meldung mehrzeilig ist.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// briefingGitHubLine ist die Zeile zu github: welcher Dienst der
// Zugangsdatei zu diesem Repo gehört und was `k-playbook github run` dafür
// wählt — aus denselben Teilen wie bei `github show` (githubSelectionLines,
// ChannelParts). Je nach Lage „keine Zugangsdatei“, „kein Dienst für …“ oder
// alle passenden Dienste. Leer ohne das Objekt. Das
// Briefing legt die Datei nicht an — weder das Feld noch die
// Umgebungsprüfung —; fehlt sie, sagt die Zeile „keine Zugangsdatei“. Ist der
// Pfad anders belegt (ein Symlink ohne Ziel, ein Verzeichnis an der Stelle
// der Datei), meldet Load einen error, und die Zeile nennt Fehler.
func briefingGitHubLine(access *project.ContextGitHub) string {
	if access == nil {
		return ""
	}
	field := *access
	const prefix = "GitHub: "
	hasError := false
	for _, problem := range field.Problems {
		if problem.Level == githubaccess.LevelError {
			hasError = true
		}
	}
	switch {
	case field.File == "" && hasError:
		return prefix + "Ort der Zugangsdatei unbekannt — " + field.Problems[0].Message
	case !field.Present:
		return prefix + "keine Zugangsdatei"
	case hasError:
		return prefix + "die Zugangsdatei hat Fehler — k-playbook github check nennt sie"
	}
	names := "keine"
	if len(field.Services) > 0 {
		names = strings.Join(field.Services.Names(), ", ")
	}
	if field.Repo == "" {
		return prefix + "kein GitHub-Remote erkannt (Dienste: " + names + ")"
	}
	if len(field.ProjectServices) == 0 {
		return prefix + "kein Dienst für " + field.Repo + " (Dienste: " + names + ")"
	}
	home, _ := os.UserHomeDir()
	parts := make([]string, 0, len(field.ProjectServices))
	for _, name := range field.ProjectServices {
		service, ok := field.Services.Lookup(name)
		if !ok {
			continue
		}
		lines := githubSelectionLines(service.Selected, service.Access, home, nil)
		if len(lines) == 0 {
			lines = []string{"run: nicht beurteilt"}
		}
		text := strings.Join(lines, "; ")
		if len(field.ProjectServices) > 1 {
			text = name + " " + text
		}
		parts = append(parts, text)
	}
	label := "Dienst "
	if len(field.ProjectServices) > 1 {
		label = "Dienste "
	}
	return prefix + "dieses Repo = " + label + strings.Join(field.ProjectServices, ", ") + " — " + strings.Join(parts, "; ")
}

// decodeContextValue liest einen JSON-Wert in seine Go-Form; Zahlen bleiben
// Text, damit sie so dastehen wie in der Ausgabe.
func decodeContextValue(raw json.RawMessage) any {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return string(raw)
	}
	return value
}

func writeContextValue(b *strings.Builder, indent string, name string, value any) {
	switch v := value.(type) {
	case map[string]any:
		if len(v) == 0 {
			fmt.Fprintf(b, "%s%s: {}\n", indent, name)
			return
		}
		fmt.Fprintf(b, "%s%s:\n", indent, name)
		for _, key := range sortedKeys(v) {
			writeContextValue(b, indent+"  ", key, v[key])
		}
	case []any:
		if len(v) == 0 {
			fmt.Fprintf(b, "%s%s: (keine)\n", indent, name)
			return
		}
		if scalars, ok := scalarList(v); ok {
			fmt.Fprintf(b, "%s%s: %s\n", indent, name, strings.Join(scalars, ", "))
			return
		}
		fmt.Fprintf(b, "%s%s:\n", indent, name)
		for _, item := range v {
			fmt.Fprintf(b, "%s  - %s\n", indent, formatContextItem(item))
		}
	default:
		fmt.Fprintf(b, "%s%s: %s\n", indent, name, formatContextScalar(v))
	}
}

// formatContextItem schreibt einen Listeneintrag in eine Zeile: ein Objekt
// als `feld=wert`-Paare, alles andere als Wert.
func formatContextItem(item any) string {
	object, ok := item.(map[string]any)
	if !ok {
		return formatContextScalar(item)
	}
	parts := make([]string, 0, len(object))
	for _, key := range sortedKeys(object) {
		parts = append(parts, key+"="+formatContextScalar(object[key]))
	}
	return strings.Join(parts, ", ")
}

// formatContextScalar schreibt einen Wert in eine Zeile; was kein Skalar ist,
// steht als kompaktes JSON.
func formatContextScalar(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return v
	case bool, json.Number:
		return fmt.Sprint(v)
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(encoded)
	}
}

func scalarList(items []any) ([]string, bool) {
	scalars := make([]string, 0, len(items))
	for _, item := range items {
		switch item.(type) {
		case map[string]any, []any:
			return nil, false
		}
		scalars = append(scalars, formatContextScalar(item))
	}
	return scalars, true
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func printBriefingUsage(out io.Writer) {
	fmt.Fprint(out, `k-playbook briefing — was für dieses Projekt gilt und was ansteht

Aufruf:
  k-playbook briefing [--json]
  k-playbook briefing --part environment [--json]

Gibt aus, was eine Sitzung zu Beginn wissen muss:

  - vorweg die Sitzungskennung und je eine Zeile für die Abschnitte, die
    jede Sitzung braucht, unter ihren Feldnamen: now (date, timestamp),
    project (dir, repoRoot, vcs, languages), kephalaion (ok, address, user,
    message) und — nur wenn die Assistenten-Verlinkung nachgezogen wurde
    oder etwas offen blieb — links (healed, open, note). Lässt sich die
    Konfiguration nicht lesen, steht das in der Zeile project;
  - den Text der immer geltenden Anweisungen
    k-playbook:/instructions/always/, zusammengebaut wie bei k-playbook read
    (lokal und mitgeliefert aufgelöst, in der festgelegten Reihenfolge) — der
    einzige Inhalt im Briefing, alles Weitere sind Verweise im Präfixschema
    (k-playbook:/docs/handbook/verweisschema.md), keine Pfade;
  - gleich danach die Umgebungsprüfung: Fehler und Warnungen über die
    Umgebung des Projekts (Kephalaion-Anbindung, gh, Basis-Werkzeuge,
    Quellenkonfiguration), je Eintrag Stufe, Bereich, Satz und Abhilfe —
    dieselbe Liste wie k-playbook health; ohne Problem die eine Zeile
    „Umgebung: keine Probleme“;
  - die Anweisungen auf Abruf, Reviews und Leitlinien — lokal und
    mitgeliefert schon gegeneinander aufgelöst, je Eintrag Verweis und Titel;
    die Herkunft (local, override, disabled) nur, wenn sie nicht dist ist,
    bei einem projekteigenen Teil mit seinem keph:-Ort. Die Beschreibung
    steht nur im JSON;
  - aus der Projekt-Collection die eigenen offenen Tasks (nach Nummer) und
    aktiven Arbeitsstände (zuletzt geänderte zuerst), je höchstens 10: eine
    Kopfzeile mit der Zahl aller und dem Verzeichnis-Verweis, darunter je
    Eintrag Dateiname und Titel, bei mehr „+ <n> weitere: k_playbook_doc_list
    (kind tasks bzw. workbench)“ — dazu die Zahl der offenen Befunde; offen
    heißt nicht in done/; ist Kephalaion nicht nutzbar, steht das dort;
  - den Einstieg der Projektdoku als Verweis;
  - eine Zeile GitHub: welcher Dienst der persönlichen Zugangsdatei
    (k-playbook github show) zum Repo des Arbeitsverzeichnisses gehört und
    was k-playbook github run dafür wählt, sonst „kein Dienst für
    <owner>/<repo>“ oder „keine Zugangsdatei“ — das Briefing legt sie nicht
    an, das tut nur k-playbook github init; im JSON stehen die Dienste
    dieses Repos unter github.projectServices, das volle Objekt liefert der
    Teilabruf environment;
  - zuletzt eine Zeile, die den Teilabruf environment nennt.

Gelesen wird ein Eintrag über k_playbook_doc_read bzw. k-playbook read mit
dem Verweis. Gesucht wird das Projekt ab dem Arbeitsverzeichnis aufwärts. Der
Aufruf zieht dabei die Assistenten-Verlinkung nach; was sich geändert hat,
steht unter links.

Ohne --json steht der Klartext zum Lesen; er zeigt jedes Feld, das ein Text
aus dem Briefing liest, und ist der Weg über die Shell. Mit --json kommt die
Antwort als JSON mit schemaVersion, session (die Kennung dieser Sitzung, je
Aufruf neu), now (date, timestamp), project (available, message, dir,
repoRoot, vcs, languages), kephalaion (ok, message, address, user),
documents (always mit content und parts, onDemand,
reviews, guidelines), work (available, message, user, tasks, workbench — je
höchstens 10 —, total mit der Zahl aller je Liste, openFindings), projectDocs,
github (projectServices), health (ok, problems — je Eintrag level, area,
message, hint) und, nur wenn es etwas meldet, links (healed, open, note) —
dieselbe Antwort wie das MCP-Werkzeug k_playbook_briefing.

Teilabruf environment:
  --part environment liefert statt des Briefings nur die Umgebungsangaben,
  die einzelne Commands an der Stelle brauchen, an der sie sie lesen:
  gh (ready, account), github (file, present, problems, remote, repo,
  projectServices, services — wie k-playbook github show --json), baseTools
  (present, error, missing, ok, installCommand), versionSources (ref, state,
  present, sources, exclude, helmValues, error), docs (exclude, problems)
  und remediation. Im Klartext eingerückt unter „environment:“, mit --json
  als Objekt unter environment (available, message, dazu die Felder) — über
  MCP k_playbook_briefing mit part "environment". Der Teilabruf vergibt keine
  Sitzungskennung, liefert weder Anweisungen noch Arbeitsliste, zieht keine
  Verlinkung nach und gilt nicht als Neuladen des Briefings: Er wird bei
  Bedarf frisch geholt, auch mehrmals je Sitzung. Die Umgebungsprüfung
  (health) bewertet und nennt Abhilfen; environment liefert die Rohwerte.
`)
}
