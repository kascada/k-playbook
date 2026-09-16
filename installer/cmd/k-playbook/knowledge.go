package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// knowledgeSearchOutput ist die Antwort von `knowledge search --json`.
// Die Treffer tragen den Vertrag aus project.Hit; query und kind stehen
// daneben, damit eine Antwort für sich lesbar bleibt.
type knowledgeSearchOutput struct {
	Query string        `json:"query"`
	Kind  string        `json:"kind,omitempty"`
	Hits  []project.Hit `json:"hits"`
}

// knowledgeListOutput ist die Antwort von `knowledge list --json`.
type knowledgeListOutput struct {
	Kind    string                   `json:"kind,omitempty"`
	Entries []project.KnowledgeEntry `json:"entries"`
}

// knowledgeReadOutput ist die Antwort von `knowledge read --json`: Markdown,
// nicht HTML.
type knowledgeReadOutput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// knowledgeWriteOutput ist die Antwort von `knowledge write --json`. path ist
// der bereinigte Ort relativ zur Wissensablage — so, wie read und search ihn
// nennen.
type knowledgeWriteOutput struct {
	Path     string `json:"path"`
	Producer string `json:"producer"`
	Written  bool   `json:"written"`
}

// knowledgeSupersedeOutput ist die Antwort von `knowledge supersede --json`.
type knowledgeSupersedeOutput struct {
	Path       string `json:"path"`
	Successor  string `json:"successor"`
	Superseded bool   `json:"superseded"`
}

// knowledgeInboxPutOutput ist die Antwort von `knowledge inbox put --json`.
type knowledgeInboxPutOutput struct {
	Path    string `json:"path"`
	Source  string `json:"source"`
	Written bool   `json:"written"`
}

// knowledgeInboxListOutput ist die Antwort von `knowledge inbox list --json`.
type knowledgeInboxListOutput struct {
	Source  string               `json:"source,omitempty"`
	Entries []project.InboxEntry `json:"entries"`
}

// knowledgeQueueAddOutput ist die Antwort von `knowledge queue add --json`.
type knowledgeQueueAddOutput struct {
	ID    string `json:"id"`
	Added bool   `json:"added"`
}

// knowledgeQueueListOutput ist die Antwort von `knowledge queue list --json`.
type knowledgeQueueListOutput struct {
	Entries []project.QueueEntry `json:"entries"`
}

// knowledgeQueueDropOutput ist die Antwort von `knowledge queue drop --json`.
type knowledgeQueueDropOutput struct {
	ID      string `json:"id"`
	Dropped bool   `json:"dropped"`
}

// knowledgeArgs sind die zerlegten Argumente eines Unterbefehls: die
// Positionsargumente, die einwertigen Optionen, die wiederholbaren Optionen
// und --json, das jeder kennt. Was ein Unterbefehl nicht kennt, weist der
// Parser ab.
type knowledgeArgs struct {
	positional []string
	options    map[string]string
	lists      map[string][]string
	json       bool
}

// get liefert eine einwertige Option, leer, wenn sie fehlt.
func (a knowledgeArgs) get(name string) string {
	return a.options[name]
}

// runKnowledge führt das Wissenstor aus: suchen, auflisten, lesen, schreiben,
// Status, Eingang und Warteschlange — dieselbe Fachlogik, die die
// MCP-Werkzeuge als Hüllen anbieten.
//
// Ohne Unterbefehl und mit --help gibt es nur die Kurzhilfe — auch hinter
// einem Unterbefehl (`knowledge write --help`): dieser Aufruf fasst keine
// Daten an, damit er als Rauchtest nach einem Release folgenlos bleibt und
// den Index nicht anlegt. Hilfe ist aber nur, was an Optionsposition steht
// (siehe errKnowledgeHelp): `search help` sucht nach „help", `--title -h`
// schreibt den Titel „-h".
func runKnowledge(args []string) error {
	if len(args) == 0 || isKnowledgeHelpWord(args[0]) {
		printKnowledgeUsage(os.Stdout)
		return nil
	}
	err := dispatchKnowledge(args)
	if errors.Is(err, errKnowledgeHelp) {
		printKnowledgeUsage(os.Stdout)
		return nil
	}
	return err
}

func dispatchKnowledge(args []string) error {
	switch args[0] {
	case "search":
		return runKnowledgeSearch(args[1:])
	case "list":
		return runKnowledgeList(args[1:])
	case "read":
		return runKnowledgeRead(args[1:])
	case "write":
		return runKnowledgeWrite(args[1:])
	case "publish":
		return runKnowledgePublish(args[1:])
	case "supersede":
		return runKnowledgeSupersede(args[1:])
	case "inbox":
		return runKnowledgeInbox(args[1:])
	case "queue":
		return runKnowledgeQueue(args[1:])
	case "status":
		return runKnowledgeStatus(args[1:])
	default:
		printKnowledgeUsage(os.Stderr)
		return fmt.Errorf("unbekannter Unterbefehl: knowledge %s", args[0])
	}
}

// errKnowledgeHelp meldet aus dem Parser, dass an einer Optionsposition nach
// der Hilfe gefragt wurde. runKnowledge übersetzt ihn in die Kurzhilfe auf
// stdout mit Exit 0.
//
// Hilfe ist `-h` oder `--help` dort, wo der Parser einen Optionsnamen
// erwartet — nie als Wert einer Option, die einen Wert nimmt. Das nackte Wort
// `help` gilt nur als erstes Token nach knowledge oder nach einer Gruppe
// (isKnowledgeHelpWord); überall sonst ist es ein Suchbegriff, eine Kennung,
// ein Grund. Ein Aufruf, der zufällig ein solches Wort trägt, darf nicht mit
// Exit 0 und ohne Wirkung enden — das sähe von außen wie Erfolg aus.
var errKnowledgeHelp = errors.New("Hilfe angefragt")

// isKnowledgeHelpWord prüft ein Token an Unterbefehlsposition — direkt nach
// knowledge oder nach inbox beziehungsweise queue.
func isKnowledgeHelpWord(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}

// parseKnowledgeArgs trennt Optionen von Positionsargumenten. single nennt die
// einwertigen Optionen des Unterbefehls, repeatable die, die mehrfach stehen
// dürfen; --json kennt jeder. Eine Option braucht einen Wert, und
// eine einwertige darf nur einmal stehen — zweimal --title wäre ein Aufruf,
// bei dem der Aufrufer selbst nicht weiß, was er meint.
//
// Die Hilfe wird in einem eigenen Durchlauf vor jeder Prüfung gesucht: eine
// Hilfe, die erst hinter einer vollständigen Argumentprüfung erschiene —
// hinter einer unbekannten Option, einer doppelten, einer fehlenden
// Pflichtoption —, wäre genau dann nicht zu haben, wenn man sie braucht.
// Übersprungen werden dabei die Werte der Optionen, die einen nehmen: dort
// ist `-h` ein Wert.
func parseKnowledgeArgs(args []string, usage string, single []string, repeatable []string) (knowledgeArgs, error) {
	parsed := knowledgeArgs{options: map[string]string{}, lists: map[string][]string{}}
	isSingle := map[string]bool{}
	for _, name := range single {
		isSingle[name] = true
	}
	isRepeatable := map[string]bool{}
	for _, name := range repeatable {
		isRepeatable[name] = true
	}

	for index := 0; index < len(args); index++ {
		arg := args[index]
		if isSingle[arg] || isRepeatable[arg] {
			index++
			continue
		}
		if arg == "-h" || arg == "--help" {
			return parsed, errKnowledgeHelp
		}
	}

	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--json":
			parsed.json = true
		case isSingle[arg] || isRepeatable[arg]:
			if index+1 >= len(args) {
				return parsed, fmt.Errorf("%s erwartet einen Wert — erwartet wird %s", arg, usage)
			}
			value := args[index+1]
			index++
			if isRepeatable[arg] {
				parsed.lists[arg] = append(parsed.lists[arg], value)
				continue
			}
			if _, twice := parsed.options[arg]; twice {
				return parsed, fmt.Errorf("%s steht zweimal — erwartet wird %s", arg, usage)
			}
			parsed.options[arg] = value
		case strings.HasPrefix(arg, "--"):
			return parsed, fmt.Errorf("unbekanntes Argument: %s — erwartet wird %s", arg, usage)
		default:
			parsed.positional = append(parsed.positional, arg)
		}
	}
	return parsed, nil
}

// knowledgeLimit liest --limit: eine Zahl ab 1, oder 0, wenn die Option fehlt.
func knowledgeLimit(parsed knowledgeArgs) (int, error) {
	raw, ok := parsed.options["--limit"]
	if !ok {
		return 0, nil
	}
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit < 1 {
		return 0, fmt.Errorf("--limit erwartet eine Zahl ab 1, nicht %q", raw)
	}
	return limit, nil
}

func runKnowledgeSearch(args []string) error {
	const usage = "knowledge search <anfrage> [--kind <art>] [--limit <n>] [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, []string{"--kind", "--limit"}, nil)
	if err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join(parsed.positional, " "))
	if query == "" {
		return fmt.Errorf("leere Anfrage: erwartet wird %s", usage)
	}
	limit, err := knowledgeLimit(parsed)
	if err != nil {
		return err
	}
	kind := parsed.get("--kind")

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	hits, err := knowledge.Search(query, project.KnowledgeFilter{Kind: kind}, limit)
	if err != nil {
		return err
	}
	reportKnowledgeNotes(knowledge)

	if parsed.json {
		return printKnowledgeJSON(knowledgeSearchOutput{Query: query, Kind: kind, Hits: hits})
	}
	if len(hits) == 0 {
		fmt.Printf("Keine Treffer für „%s“.\n", query)
		return nil
	}
	for _, hit := range hits {
		heading := hit.Heading
		if heading == "" {
			heading = "(Vorspann)"
		}
		fmt.Printf("%d. %s § %s (%s)\n", hit.Rank, hit.Path, heading, hit.Kind)
		if hit.Excerpt != "" {
			fmt.Printf("   %s\n", hit.Excerpt)
		}
	}
	return nil
}

func runKnowledgeList(args []string) error {
	const usage = "knowledge list [--kind <art>] [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, []string{"--kind"}, nil)
	if err != nil {
		return err
	}
	if len(parsed.positional) > 0 {
		return fmt.Errorf("unerwartetes Argument: %s — erwartet wird %s", parsed.positional[0], usage)
	}
	kind := parsed.get("--kind")

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	entries, err := knowledge.List(project.KnowledgeFilter{Kind: kind})
	if err != nil {
		return err
	}
	reportKnowledgeNotes(knowledge)

	if parsed.json {
		return printKnowledgeJSON(knowledgeListOutput{Kind: kind, Entries: entries})
	}
	if len(entries) == 0 {
		fmt.Println("Keine Dateien in der Wissensablage.")
		return nil
	}
	for _, entry := range entries {
		state := ""
		if entry.State != "" {
			state = ", " + entry.State
		}
		fmt.Printf("%s — %s (%s%s)\n", entry.Path, entry.Title, entry.Kind, state)
	}
	return nil
}

func runKnowledgeRead(args []string) error {
	const usage = "knowledge read <pfad> [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, nil, nil)
	if err != nil {
		return err
	}
	if len(parsed.positional) != 1 {
		return fmt.Errorf("erwartet wird %s", usage)
	}

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	content, err := knowledge.Read(parsed.positional[0])
	if err != nil {
		return err
	}
	reportKnowledgeNotes(knowledge)

	if parsed.json {
		return printKnowledgeJSON(knowledgeReadOutput{Path: filepath.ToSlash(parsed.positional[0]), Content: content})
	}
	fmt.Print(content)
	if !strings.HasSuffix(content, "\n") {
		fmt.Println()
	}
	return nil
}

// readKnowledgeBody liest den Rumpf aus --file oder von stdin.
func readKnowledgeBody(file string) (string, error) {
	var content []byte
	var err error
	if file != "" {
		content, err = os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("%s lesen: %w", file, err)
		}
	} else {
		content, err = io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("stdin lesen: %w", err)
		}
	}
	if strings.TrimSpace(string(content)) == "" {
		return "", fmt.Errorf("leerer Rumpf: --file <datei> angeben oder den Text über stdin hereinreichen")
	}
	return string(content), nil
}

func runKnowledgeWrite(args []string) error {
	const usage = "knowledge write <pfad> --producer <erzeuger> --title <titel> --subject <thema> --origin <herkunft> --state <raw|condensed|reviewed> [--format <format>] [--source <quellpfad>]… [--queue <kennung>] [--file <datei>] [--json]"
	parsed, err := parseKnowledgeArgs(args, usage,
		[]string{"--producer", "--title", "--subject", "--origin", "--state", "--format", "--queue", "--file"},
		[]string{"--source"})
	if err != nil {
		return err
	}
	if len(parsed.positional) != 1 {
		return fmt.Errorf("erwartet wird %s", usage)
	}
	for _, name := range []string{"--producer", "--title", "--subject", "--origin", "--state"} {
		if strings.TrimSpace(parsed.get(name)) == "" {
			return fmt.Errorf("%s fehlt: erwartet wird %s", name, usage)
		}
	}
	body, err := readKnowledgeBody(parsed.get("--file"))
	if err != nil {
		return err
	}

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	doc := project.KnowledgeDocument{
		Path:    parsed.positional[0],
		Title:   parsed.get("--title"),
		Subject: parsed.get("--subject"),
		Origin:  parsed.get("--origin"),
		State:   parsed.get("--state"),
		Format:  parsed.get("--format"),
		Sources: parsed.lists["--source"],
		Body:    body,
	}
	rel, err := knowledge.Write(parsed.get("--producer"), doc, parsed.get("--queue"))
	if err != nil {
		return err
	}
	reportKnowledgeNotes(knowledge)

	producer := strings.TrimSpace(parsed.get("--producer"))
	if parsed.json {
		return printKnowledgeJSON(knowledgeWriteOutput{Path: rel, Producer: producer, Written: true})
	}
	fmt.Printf("Geschrieben: %s/%s/%s (producer: %s)\n", project.LocalDirName, project.KnowledgeDirName, rel, producer)
	if queue := strings.TrimSpace(parsed.get("--queue")); queue != "" {
		fmt.Printf("Queue-Eintrag erledigt: %s\n", queue)
	}
	return nil
}

func runKnowledgePublish(args []string) error {
	const usage = "knowledge publish --producer <docs-code|docs-tools|inventory> --from <verzeichnis> [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, []string{"--producer", "--from"}, nil)
	if err != nil {
		return err
	}
	if len(parsed.positional) > 0 {
		return fmt.Errorf("unerwartetes Argument: %s — erwartet wird %s", parsed.positional[0], usage)
	}
	for _, name := range []string{"--producer", "--from"} {
		if strings.TrimSpace(parsed.get(name)) == "" {
			return fmt.Errorf("%s fehlt: erwartet wird %s", name, usage)
		}
	}
	documents, err := project.LoadKnowledgeDocuments(parsed.get("--from"))
	if err != nil {
		return err
	}

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	result, err := knowledge.Publish(parsed.get("--producer"), documents)
	if err != nil {
		return err
	}
	reportKnowledgeNotes(knowledge)

	if parsed.json {
		return printKnowledgeJSON(result)
	}
	fmt.Printf("Veröffentlicht: %s/%s/%s — %d geschrieben, %d entfernt (producer: %s)\n",
		project.LocalDirName, project.KnowledgeDirName, result.Dir, result.Written, result.Removed, result.Producer)
	return nil
}

func runKnowledgeSupersede(args []string) error {
	const usage = "knowledge supersede <pfad> --successor <pfad> --reason <grund> [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, []string{"--successor", "--reason"}, nil)
	if err != nil {
		return err
	}
	if len(parsed.positional) != 1 {
		return fmt.Errorf("erwartet wird %s", usage)
	}
	for _, name := range []string{"--successor", "--reason"} {
		if strings.TrimSpace(parsed.get(name)) == "" {
			return fmt.Errorf("%s fehlt: erwartet wird %s", name, usage)
		}
	}

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	rel, successor, err := knowledge.Supersede(parsed.positional[0], parsed.get("--successor"), parsed.get("--reason"))
	if err != nil {
		return err
	}
	reportKnowledgeNotes(knowledge)

	if parsed.json {
		return printKnowledgeJSON(knowledgeSupersedeOutput{Path: rel, Successor: successor, Superseded: true})
	}
	fmt.Printf("Abgelöst: %s/%s/%s → Nachfolger %s\n", project.LocalDirName, project.KnowledgeDirName, rel, successor)
	return nil
}

// runKnowledgeInbox verzweigt in die Gruppe des Eingangs: put, list, read.
func runKnowledgeInbox(args []string) error {
	const usage = "knowledge inbox put|list|read …"
	if len(args) == 0 {
		return fmt.Errorf("erwartet wird %s", usage)
	}
	if isKnowledgeHelpWord(args[0]) {
		return errKnowledgeHelp
	}
	switch args[0] {
	case "put":
		return runKnowledgeInboxPut(args[1:])
	case "list":
		return runKnowledgeInboxList(args[1:])
	case "read":
		return runKnowledgeInboxRead(args[1:])
	default:
		return fmt.Errorf("unbekannter Unterbefehl: knowledge inbox %s — erwartet wird %s", args[0], usage)
	}
}

func runKnowledgeInboxPut(args []string) error {
	const usage = "knowledge inbox put <quelle> <name> [--file <datei>] [--note <text>] [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, []string{"--file", "--note"}, nil)
	if err != nil {
		return err
	}
	if len(parsed.positional) != 2 {
		return fmt.Errorf("erwartet wird %s", usage)
	}
	var content []byte
	if file := parsed.get("--file"); file != "" {
		content, err = os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("%s lesen: %w", file, err)
		}
	} else {
		content, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("stdin lesen: %w", err)
		}
	}

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	rel, err := knowledge.InboxPut(parsed.positional[0], parsed.positional[1], content, parsed.get("--note"))
	if err != nil {
		return err
	}
	source := strings.TrimSpace(parsed.positional[0])
	if parsed.json {
		return printKnowledgeJSON(knowledgeInboxPutOutput{Path: rel, Source: source, Written: true})
	}
	fmt.Printf("Abgelegt: %s/%s/%s\n", project.LocalDirName, project.InboxDirName, rel)
	return nil
}

func runKnowledgeInboxList(args []string) error {
	const usage = "knowledge inbox list [<quelle>] [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, nil, nil)
	if err != nil {
		return err
	}
	if len(parsed.positional) > 1 {
		return fmt.Errorf("erwartet wird %s", usage)
	}
	source := ""
	if len(parsed.positional) == 1 {
		source = strings.TrimSpace(parsed.positional[0])
	}

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	entries, err := knowledge.InboxList(source)
	if err != nil {
		return err
	}
	if parsed.json {
		return printKnowledgeJSON(knowledgeInboxListOutput{Source: source, Entries: entries})
	}
	if len(entries) == 0 {
		fmt.Println("Der Eingang ist leer.")
		return nil
	}
	for _, entry := range entries {
		fmt.Printf("%s  %s  %d B  %s\n", entry.Path, entry.Format, entry.Size, entry.Modified.Format("2006-01-02 15:04"))
		if entry.Note != "" {
			fmt.Printf("   Notiz: %s\n", entry.Note)
		}
	}
	return nil
}

func runKnowledgeInboxRead(args []string) error {
	const usage = "knowledge inbox read <pfad> [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, nil, nil)
	if err != nil {
		return err
	}
	if len(parsed.positional) != 1 {
		return fmt.Errorf("erwartet wird %s", usage)
	}

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	content, err := knowledge.InboxRead(parsed.positional[0])
	if err != nil {
		return err
	}
	if parsed.json {
		return printKnowledgeJSON(knowledgeReadOutput{Path: filepath.ToSlash(strings.TrimSpace(parsed.positional[0])), Content: content})
	}
	fmt.Print(content)
	if !strings.HasSuffix(content, "\n") {
		fmt.Println()
	}
	return nil
}

// runKnowledgeQueue verzweigt in die Gruppe der Warteschlange: add, list, drop.
func runKnowledgeQueue(args []string) error {
	const usage = "knowledge queue add|list|drop …"
	if len(args) == 0 {
		return fmt.Errorf("erwartet wird %s", usage)
	}
	if isKnowledgeHelpWord(args[0]) {
		return errKnowledgeHelp
	}
	switch args[0] {
	case "add":
		return runKnowledgeQueueAdd(args[1:])
	case "list":
		return runKnowledgeQueueList(args[1:])
	case "drop":
		return runKnowledgeQueueDrop(args[1:])
	default:
		return fmt.Errorf("unbekannter Unterbefehl: knowledge queue %s — erwartet wird %s", args[0], usage)
	}
}

func runKnowledgeQueueAdd(args []string) error {
	const usage = "knowledge queue add --origin <eingangspfad|adresse> --target <verzeichnis> --reason <grund> [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, []string{"--origin", "--target", "--reason"}, nil)
	if err != nil {
		return err
	}
	if len(parsed.positional) > 0 {
		return fmt.Errorf("unerwartetes Argument: %s — erwartet wird %s", parsed.positional[0], usage)
	}
	for _, name := range []string{"--origin", "--target", "--reason"} {
		if strings.TrimSpace(parsed.get(name)) == "" {
			return fmt.Errorf("%s fehlt: erwartet wird %s", name, usage)
		}
	}

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	id, err := knowledge.QueueAdd(parsed.get("--origin"), parsed.get("--target"), parsed.get("--reason"))
	if err != nil {
		return err
	}
	if parsed.json {
		return printKnowledgeJSON(knowledgeQueueAddOutput{ID: id, Added: true})
	}
	fmt.Printf("Eingereiht: %s\n", id)
	return nil
}

func runKnowledgeQueueList(args []string) error {
	const usage = "knowledge queue list [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, nil, nil)
	if err != nil {
		return err
	}
	if len(parsed.positional) > 0 {
		return fmt.Errorf("unerwartetes Argument: %s — erwartet wird %s", parsed.positional[0], usage)
	}

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	entries, err := knowledge.QueueList()
	if err != nil {
		return err
	}
	reportKnowledgeNotes(knowledge)
	if parsed.json {
		return printKnowledgeJSON(knowledgeQueueListOutput{Entries: entries})
	}
	if len(entries) == 0 {
		fmt.Println("Nichts offen.")
		return nil
	}
	for _, entry := range entries {
		fmt.Printf("%s\n   %s → %s — %s\n", entry.ID, entry.Origin, entry.Target, entry.Reason)
		if entry.Notes != "" {
			fmt.Printf("   %s\n", strings.ReplaceAll(entry.Notes, "\n", "\n   "))
		}
	}
	return nil
}

func runKnowledgeQueueDrop(args []string) error {
	const usage = "knowledge queue drop <kennung> [--reason <grund>] [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, []string{"--reason"}, nil)
	if err != nil {
		return err
	}
	if len(parsed.positional) != 1 {
		return fmt.Errorf("erwartet wird %s", usage)
	}

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	if err := knowledge.QueueDrop(parsed.positional[0], parsed.get("--reason")); err != nil {
		return err
	}
	id := strings.TrimSpace(parsed.positional[0])
	if parsed.json {
		return printKnowledgeJSON(knowledgeQueueDropOutput{ID: id, Dropped: true})
	}
	fmt.Printf("Gestrichen: %s\n", id)
	return nil
}

func runKnowledgeStatus(args []string) error {
	const usage = "knowledge status [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, nil, nil)
	if err != nil {
		return err
	}
	if len(parsed.positional) > 0 {
		return fmt.Errorf("unerwartetes Argument: %s — erwartet wird %s", parsed.positional[0], usage)
	}

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	status, err := knowledge.Status()
	if err != nil {
		return err
	}
	reportKnowledgeNotes(knowledge)

	if parsed.json {
		return printKnowledgeJSON(status)
	}
	fmt.Printf("Index:    %s, Fassung %d, gebaut %s\n", status.IndexKind, status.IndexVersion, status.BuiltAt.Format("2006-01-02 15:04:05"))
	if status.Model != "" || status.Dims != 0 {
		fmt.Printf("Modell:   %s (%d Dimensionen)\n", status.Model, status.Dims)
	}
	fmt.Printf("Dateien:  %d, Chunks: %d\n", status.FileCount, status.ChunkCount)
	kinds := make([]string, 0, len(status.ByKind))
	for kind := range status.ByKind {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		count := status.ByKind[kind]
		fmt.Printf("  %-10s %d Dateien, %d Chunks\n", kind, count.Files, count.Chunks)
	}
	if status.Stale {
		fmt.Printf("Drift:    beim letzten Zugriff behoben, %d Datei(en) waren am Tor vorbei geändert\n", status.StaleFiles)
	} else {
		fmt.Println("Drift:    keine")
	}
	return nil
}

// openKnowledge löst das Projekt auf. Wie bei todo wird nur der Anker
// gebraucht: das Wissen liegt unter k-playbook-local/, nicht im Katalog der
// Installation.
func openKnowledge() (*project.Knowledge, error) {
	projectDir, err := todoProjectDir()
	if err != nil {
		return nil, err
	}
	return project.NewKnowledge(projectDir), nil
}

// reportKnowledgeNotes gibt auf stderr aus, was der Zugriff übergehen musste:
// ein nicht beschreibbares cache/, eine unlesbare Datei. Die Antwort steht und
// geht auf stdout — auch die JSON-Antwort bleibt dadurch für sich lesbar —,
// aber übergangen wird nichts stillschweigend.
func reportKnowledgeNotes(knowledge *project.Knowledge) {
	for _, note := range knowledge.Notes() {
		fmt.Fprintf(os.Stderr, "Hinweis: %s\n", note)
	}
}

func printKnowledgeJSON(payload any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func printKnowledgeUsage(out io.Writer) {
	fmt.Fprint(out, `k-playbook knowledge — die Wissensablage des Projekts

Liest und durchsucht k-playbook-local/knowledge/ entlang der Überschriften und
schreibt dorthin — als einziger Weg, mit geprüftem Erzeuger und erzeugtem
Frontmatter. Der Index liegt unter k-playbook-local/cache/knowledge/ und ist
jederzeit verwerfbar; er erkennt selbst, wenn Dateien an ihm vorbei geändert
wurden. Mit --json ist die Ausgabe JSON auf stdout, Fehler stehen auf stderr.

Unterbefehle:
  search <anfrage> [--kind <art>] [--limit <n>] [--json]
            Sucht in den Abschnitten. Ein Treffer nennt path, heading,
            excerpt, kind, origin, state, rank und anchor; --limit gilt 10,
            wenn es fehlt. Dokumente mit state raw oder superseded und die
            README in der Wurzel sind keine Treffer.
  list [--kind <art>] [--json]
            Listet die Dateien mit path, title, kind, origin und state —
            auch die rohen und abgelösten.
  read <pfad> [--json]
            Gibt eine Datei als Markdown aus; pfad relativ zu knowledge/.
  write <pfad> --producer <erzeuger> --title <titel> --subject <thema>
        --origin <herkunft> --state <raw|condensed|reviewed>
        [--format <markdown|text|html|image|pdf>] [--source <quellpfad>]…
        [--queue <kennung>] [--file <datei>] [--json]
            Schreibt ein Dokument; pfad relativ zu knowledge/ und im
            Verzeichnis des Erzeugers. Der Rumpf kommt aus --file oder von
            stdin, ohne Frontmatter — den Kopf baut das Werkzeug aus den
            Feldern und setzt updated selbst. --queue nennt den Eintrag der
            Warteschlange, der damit erledigt ist; er wird gelöscht, sobald
            das Dokument steht.
  publish --producer <docs-code|docs-tools|inventory> --from <verzeichnis> [--json]
            Veröffentlicht das Verzeichnis eines Generators als Ganzes: jede
            Markdown-Datei unter <verzeichnis> trägt die Felder von write im
            Kopf, das Werkzeug prüft sie und setzt den Kopf neu zusammen. Was
            nicht im Satz ist, wird entfernt — atomar, ein Abbruch lässt den
            vorherigen Stand stehen.
  supersede <pfad> --successor <pfad> --reason <grund> [--json]
            Löst ein Dokument ab: state wird superseded, Nachfolger und Grund
            kommen in den Kopf. Gelöscht wird nichts; abgelöste Dokumente
            fehlen in search und bleiben über read und list erreichbar.
  status [--json]
            Art und Größe des Index, Dateien und Chunks je kind, und ob der
            letzte Zugriff Drift behoben hat.

Eingang (k-playbook-local/inbox/, nie indiziert):
  inbox put <quelle> <name> [--file <datei>] [--note <text>] [--json]
            Legt ein Rohstück unter inbox/<quelle>/<name> ab, so wie es
            kommt; der Inhalt aus --file oder von stdin. Ein belegter Name
            wird abgewiesen. --note legt eine Notiz als <name>.note daneben.
  inbox list [<quelle>] [--json]
            Listet die Rohstücke mit Format, Größe, Datum und Notiz.
  inbox read <pfad> [--json]
            Gibt ein Rohstück aus — nur Textformate (md, markdown, txt, html, htm,
            json, yaml, yml, csv, xml, log).

Warteschlange (k-playbook-local/queue/, leer heißt: nichts offen):
  queue add --origin <eingangspfad|adresse> --target <verzeichnis> --reason <grund> [--json]
            Reiht ein Stück Arbeit ein und vergibt die Kennung.
  queue list [--json]
            Nennt den Rückstand, älteste zuerst.
  queue drop <kennung> [--reason <grund>] [--json]
            Streicht einen Eintrag ohne Übernahme; der Grund wird nirgends
            festgehalten. Die Übernahme selbst erledigt write --queue.

Erzeuger (producer) und ihr Verzeichnis unter knowledge/:
`)
	for _, row := range project.ProducerTable() {
		fmt.Fprintf(out, "  %s\n", row)
	}
	fmt.Fprint(out, `
Art (kind) ist der Ordner unter knowledge/ — oder root für Dateien in der
Wurzel. Die Herkunft (origin) steht im Frontmatter jedes Dokuments.

Ohne Unterbefehl und mit --help steht hier nur diese Übersicht: der Aufruf
liest und schreibt dabei nichts. -h und --help gelten dort, wo eine Option
erwartet wird — nicht als Wert einer Option; help nur direkt nach knowledge,
inbox oder queue.
`)
}
