package main

import (
	"encoding/json"
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
// Die Treffer tragen den Vertrag aus project.Hit; query und source stehen
// daneben, damit eine Antwort für sich lesbar bleibt.
type knowledgeSearchOutput struct {
	Query  string        `json:"query"`
	Source string        `json:"source,omitempty"`
	Hits   []project.Hit `json:"hits"`
}

// knowledgeListOutput ist die Antwort von `knowledge list --json`.
type knowledgeListOutput struct {
	Source  string                   `json:"source,omitempty"`
	Entries []project.KnowledgeEntry `json:"entries"`
}

// knowledgeReadOutput ist die Antwort von `knowledge read --json`: Markdown,
// nicht HTML.
type knowledgeReadOutput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// knowledgeWriteOutput ist die Antwort von `knowledge write --json`. path ist
// der Ort relativ zum Wissensverzeichnis, also mit learned/ davor — so, wie
// read und search ihn nennen.
type knowledgeWriteOutput struct {
	Path    string `json:"path"`
	Source  string `json:"source"`
	Written bool   `json:"written"`
}

// knowledgeArgs sind die Optionen, die alle Unterbefehle teilen. Was ein
// Unterbefehl nicht kennt, weist er ab.
type knowledgeArgs struct {
	positional []string
	source     string
	limit      int
	file       string
	json       bool
}

// runKnowledge führt das Wissenstor aus: suchen, auflisten, lesen, schreiben,
// Status — dieselbe Fachlogik, die die MCP-Werkzeuge als Hüllen anbieten.
//
// Ohne Unterbefehl und mit --help gibt es nur die Kurzhilfe: dieser Aufruf
// fasst keine Daten an, damit er als Rauchtest nach einem Release folgenlos
// bleibt und den Index nicht anlegt.
func runKnowledge(args []string) error {
	if len(args) == 0 {
		printKnowledgeUsage(os.Stdout)
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		printKnowledgeUsage(os.Stdout)
		return nil
	case "search":
		return runKnowledgeSearch(args[1:])
	case "list":
		return runKnowledgeList(args[1:])
	case "read":
		return runKnowledgeRead(args[1:])
	case "write":
		return runKnowledgeWrite(args[1:])
	case "status":
		return runKnowledgeStatus(args[1:])
	default:
		printKnowledgeUsage(os.Stderr)
		return fmt.Errorf("unbekannter Unterbefehl: knowledge %s", args[0])
	}
}

// parseKnowledgeArgs trennt Optionen von Positionsargumenten. allowed nennt
// die Optionen, die der Unterbefehl kennt; --json kennt jeder.
func parseKnowledgeArgs(args []string, usage string, allowed ...string) (knowledgeArgs, error) {
	parsed := knowledgeArgs{}
	known := map[string]bool{}
	for _, name := range allowed {
		known[name] = true
	}
	value := func(index int) (string, error) {
		if index+1 >= len(args) {
			return "", fmt.Errorf("%s erwartet einen Wert — erwartet wird %s", args[index], usage)
		}
		return args[index+1], nil
	}

	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--json":
			parsed.json = true
		case known[arg] && arg == "--source":
			source, err := value(index)
			if err != nil {
				return parsed, err
			}
			parsed.source = source
			index++
		case known[arg] && arg == "--limit":
			raw, err := value(index)
			if err != nil {
				return parsed, err
			}
			limit, err := strconv.Atoi(strings.TrimSpace(raw))
			if err != nil || limit < 1 {
				return parsed, fmt.Errorf("--limit erwartet eine Zahl ab 1, nicht %q", raw)
			}
			parsed.limit = limit
			index++
		case known[arg] && arg == "--file":
			file, err := value(index)
			if err != nil {
				return parsed, err
			}
			parsed.file = file
			index++
		case strings.HasPrefix(arg, "--"):
			return parsed, fmt.Errorf("unbekanntes Argument: %s — erwartet wird %s", arg, usage)
		default:
			parsed.positional = append(parsed.positional, arg)
		}
	}
	return parsed, nil
}

func runKnowledgeSearch(args []string) error {
	const usage = "knowledge search <anfrage> [--source <herkunft>] [--limit <n>] [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, "--source", "--limit")
	if err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join(parsed.positional, " "))
	if query == "" {
		return fmt.Errorf("leere Anfrage: erwartet wird %s", usage)
	}

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	hits, err := knowledge.Search(query, project.KnowledgeFilter{Source: parsed.source}, parsed.limit)
	if err != nil {
		return err
	}
	reportKnowledgeNotes(knowledge)

	if parsed.json {
		return printKnowledgeJSON(knowledgeSearchOutput{Query: query, Source: parsed.source, Hits: hits})
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
		fmt.Printf("%d. %s § %s (%s)\n", hit.Rank, hit.Path, heading, hit.Source)
		if hit.Excerpt != "" {
			fmt.Printf("   %s\n", hit.Excerpt)
		}
	}
	return nil
}

func runKnowledgeList(args []string) error {
	const usage = "knowledge list [--source <herkunft>] [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, "--source")
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
	entries, err := knowledge.List(project.KnowledgeFilter{Source: parsed.source})
	if err != nil {
		return err
	}
	reportKnowledgeNotes(knowledge)

	if parsed.json {
		return printKnowledgeJSON(knowledgeListOutput{Source: parsed.source, Entries: entries})
	}
	if len(entries) == 0 {
		fmt.Println("Keine Dateien im Wissensverzeichnis.")
		return nil
	}
	for _, entry := range entries {
		fmt.Printf("%s — %s (%s)\n", entry.Path, entry.Title, entry.Source)
	}
	return nil
}

func runKnowledgeRead(args []string) error {
	const usage = "knowledge read <pfad> [--json]"
	parsed, err := parseKnowledgeArgs(args, usage)
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

	if parsed.json {
		return printKnowledgeJSON(knowledgeReadOutput{Path: filepath.ToSlash(parsed.positional[0]), Content: content})
	}
	fmt.Print(content)
	if !strings.HasSuffix(content, "\n") {
		fmt.Println()
	}
	return nil
}

func runKnowledgeWrite(args []string) error {
	const usage = "knowledge write <pfad> --source <herkunft> [--file <datei>] [--json]"
	parsed, err := parseKnowledgeArgs(args, usage, "--source", "--file")
	if err != nil {
		return err
	}
	if len(parsed.positional) != 1 {
		return fmt.Errorf("erwartet wird %s", usage)
	}
	if strings.TrimSpace(parsed.source) == "" {
		return fmt.Errorf("--source fehlt: erwartet wird %s", usage)
	}

	var content []byte
	if parsed.file != "" {
		content, err = os.ReadFile(parsed.file)
		if err != nil {
			return fmt.Errorf("%s lesen: %w", parsed.file, err)
		}
	} else {
		content, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("stdin lesen: %w", err)
		}
	}
	if strings.TrimSpace(string(content)) == "" {
		return fmt.Errorf("leerer Inhalt: --file <datei> angeben oder den Text über stdin hereinreichen")
	}

	knowledge, err := openKnowledge()
	if err != nil {
		return err
	}
	rel, err := knowledge.Write(parsed.positional[0], string(content), parsed.source)
	if err != nil {
		return err
	}
	reportKnowledgeNotes(knowledge)

	if parsed.json {
		return printKnowledgeJSON(knowledgeWriteOutput{Path: rel, Source: strings.TrimSpace(parsed.source), Written: true})
	}
	fmt.Printf("Geschrieben: %s/%s/%s (source: %s)\n", project.LocalDirName, project.KnowledgeDirName, rel, strings.TrimSpace(parsed.source))
	return nil
}

func runKnowledgeStatus(args []string) error {
	const usage = "knowledge status [--json]"
	parsed, err := parseKnowledgeArgs(args, usage)
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
	sources := make([]string, 0, len(status.BySource))
	for source := range status.BySource {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	for _, source := range sources {
		count := status.BySource[source]
		fmt.Printf("  %-10s %d Dateien, %d Chunks\n", source, count.Files, count.Chunks)
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
	fmt.Fprint(out, `k-playbook knowledge — das Wissensverzeichnis des Projekts

Liest und durchsucht k-playbook-local/docs/ entlang der Überschriften und
schreibt neue Dokumente ausschließlich nach k-playbook-local/docs/learned/.
Der Index liegt unter k-playbook-local/cache/knowledge/ und ist jederzeit
verwerfbar; er erkennt selbst, wenn Dateien an ihm vorbei geändert wurden.
Mit --json ist die Ausgabe JSON auf stdout, Fehler stehen auf stderr.

Unterbefehle:
  search <anfrage> [--source <herkunft>] [--limit <n>] [--json]
            Sucht in den Abschnitten. Ein Treffer nennt path, heading,
            excerpt, source, rank und anchor; --limit gilt 10, wenn es fehlt.
  list [--source <herkunft>] [--json]
            Listet die Dateien mit path, title und source.
  read <pfad> [--json]
            Gibt eine Datei als Markdown aus; pfad relativ zu docs/.
  write <pfad> --source <herkunft> [--file <datei>] [--json]
            Schreibt eine Datei unterhalb von docs/learned/; pfad relativ
            dazu. Der Inhalt kommt aus --file oder von stdin, source geht
            als Herkunftsvermerk ins Frontmatter.
  status [--json]
            Art und Größe des Index, Dateien und Chunks je Herkunft, und ob
            der letzte Zugriff Drift behoben hat.

Herkunft (source) ist der Ordner unter docs/: code, libs, extracted,
versions, manual, learned — oder root für Dateien in der Wurzel.

Ohne Unterbefehl und mit --help steht hier nur diese Übersicht: der Aufruf
liest und schreibt dabei nichts.
`)
}
