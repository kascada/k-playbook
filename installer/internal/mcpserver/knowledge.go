package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Wissens-Werkzeuge — Suche und Ablage, Eingang und Warteschlange — sind
// dünne Hüllen über project.Knowledge, dem Zugriffsweg, den auch das
// Subkommando `k-playbook knowledge` benutzt.
// Zweite Fachlogik gibt es hier nicht: gesucht, gechunkt und indiziert wird
// ausschließlich in project/.
//
// Tragende Schicht ist das Subkommando: der MCP-Server wird pro Projekt
// registriert und kann fehlen, das Binary ist mit der Installation da.
const (
	knowledgeToolSearch = "k_playbook_knowledge_search"
	knowledgeToolList   = "k_playbook_knowledge_list"
	knowledgeToolRead   = "k_playbook_knowledge_read"
	knowledgeToolWrite  = "k_playbook_knowledge_write"
	knowledgeToolStatus = "k_playbook_knowledge_status"

	knowledgeToolPublish   = "k_playbook_knowledge_publish"
	knowledgeToolSupersede = "k_playbook_knowledge_supersede"

	knowledgeToolInboxPut  = "k_playbook_knowledge_inbox_put"
	knowledgeToolInboxList = "k_playbook_knowledge_inbox_list"
	knowledgeToolInboxRead = "k_playbook_knowledge_inbox_read"
	knowledgeToolQueueAdd  = "k_playbook_knowledge_queue_add"
	knowledgeToolQueueList = "k_playbook_knowledge_queue_list"
	knowledgeToolQueueDrop = "k_playbook_knowledge_queue_drop"
)

type knowledgeBaseInput struct {
	ProjectDir string `json:"projectDir" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
}

type knowledgeSearchInput struct {
	knowledgeBaseInput
	Query string `json:"query" jsonschema:"Pflicht. Die Suchanfrage, Volltext über alle Abschnitte der Wissensablage. Dokumente mit state raw oder superseded fehlen in den Treffern; list und read führen sie weiter."`
	Kind  string `json:"kind,omitempty" jsonschema:"Grenzt auf eine Art ein — das Verzeichnis unter knowledge/: code, libs, versions, extracted, external, findings, pitfalls, manual oder root. Ohne Angabe alle."`
	Limit int    `json:"limit,omitempty" jsonschema:"Höchstzahl der Treffer. Ohne Angabe 10."`
}

type knowledgeListInput struct {
	knowledgeBaseInput
	Kind string `json:"kind,omitempty" jsonschema:"Grenzt auf eine Art ein — das Verzeichnis unter knowledge/: code, libs, versions, extracted, external, findings, pitfalls, manual oder root. Ohne Angabe alle."`
}

type knowledgeReadInput struct {
	knowledgeBaseInput
	Path string `json:"path" jsonschema:"Pflicht. Pfad relativ zu k-playbook-local/knowledge/, so wie search und list ihn nennen, etwa manual/ablauf.md oder README.md."`
}

// knowledgeDocumentInput sind die Felder eines Dokuments, wie write und
// publish sie annehmen — die Frontmatter-Felder einzeln, der Rumpf ohne Kopf.
// Das Werkzeug setzt den Kopf zusammen und setzt updated selbst.
type knowledgeDocumentInput struct {
	Path    string   `json:"path" jsonschema:"Pflicht. Pfad relativ zu k-playbook-local/knowledge/, im Verzeichnis des Erzeugers: session → findings/…, person → manual/… oder pitfalls/…, docs-extract → extracted/…, connector:<system> → external/<system>/…, docs-index → genau README.md; bei publish relativ zum Verzeichnis des Generators (code/, libs/, versions/). Eine vorhandene Datei wird ersetzt."`
	Title   string   `json:"title" jsonschema:"Pflicht. Die eigene Formulierung des Dokuments, einzeilig."`
	Subject string   `json:"subject" jsonschema:"Pflicht. Das Thema — die Achse, nach der Index und Suche sortieren. Frei."`
	Origin  string   `json:"origin" jsonschema:"Pflicht. Die tatsächliche Herkunft: System, Kennung dort, Adresse, Zeitpunkt des Abrufs — etwa \"Sitzung 2026-09-12, Task 056\" oder \"Confluence DEV/12345, abgerufen 2026-09-12\"."`
	State   string   `json:"state" jsonschema:"Pflicht. raw, condensed oder reviewed. superseded wird abgewiesen; es entsteht nur über k_playbook_knowledge_supersede."`
	Format  string   `json:"format,omitempty" jsonschema:"Was das Original war: markdown (Standard), text, html, image oder pdf."`
	Sources []string `json:"sources,omitempty" jsonschema:"Die Eingangspfade unter k-playbook-local/inbox/, aus denen das Dokument destilliert wurde — wo es welche gibt."`
	Body    string   `json:"body" jsonschema:"Pflicht. Der Rumpf als Markdown ohne Frontmatter; ein übergebener Kopf (---…---) wird abgewiesen."`
}

func (input knowledgeDocumentInput) document() project.KnowledgeDocument {
	return project.KnowledgeDocument{
		Path: input.Path, Title: input.Title, Subject: input.Subject, Origin: input.Origin,
		State: input.State, Format: input.Format, Sources: input.Sources, Body: input.Body,
	}
}

type knowledgeWriteInput struct {
	knowledgeBaseInput
	Producer string `json:"producer" jsonschema:"Pflicht. Der Erzeuger aus der geschlossenen Liste: docs-extract, connector:<system>, session, person, docs-index. Die Generatoren docs-code, docs-tools und inventory schreiben nie eine Einzeldatei, sondern ihr Verzeichnis als Ganzes über k_playbook_knowledge_publish."`
	knowledgeDocumentInput
	Queue string `json:"queue,omitempty" jsonschema:"Kennung des Queue-Eintrags, den dieses Dokument erledigt. Er wird gelöscht, nachdem das Dokument steht — und nur dann."`
}

type knowledgeStatusInput struct {
	knowledgeBaseInput
}

type knowledgePublishInput struct {
	knowledgeBaseInput
	Producer  string                   `json:"producer" jsonschema:"Pflicht. Einer der drei Generatoren: docs-code (knowledge/code/), docs-tools (knowledge/libs/), inventory (knowledge/versions/). Alle anderen Erzeuger schreiben über k_playbook_knowledge_write."`
	Documents []knowledgeDocumentInput `json:"documents" jsonschema:"Pflicht. Der vollständige Satz der Dokumente; path je Dokument relativ zum Verzeichnis des Generators. Was nicht im Satz ist, wird entfernt."`
}

type knowledgeInboxPutInput struct {
	knowledgeBaseInput
	Source  string `json:"source" jsonschema:"Pflicht. Die Quelle, ein einzelner Verzeichnisname unter k-playbook-local/inbox/: confluence, chat, mail, scan — frei."`
	Name    string `json:"name" jsonschema:"Pflicht. Der Dateiname unter der Quelle, mit Endung; Unterverzeichnisse sind erlaubt. Ein belegter Name wird abgewiesen."`
	Content string `json:"content,omitempty" jsonschema:"Der Inhalt als Text. Entweder content oder file."`
	File    string `json:"file,omitempty" jsonschema:"Ein Dateipfad auf diesem Rechner, dessen Inhalt abgelegt wird — für Binärdateien wie PDFs und Bilder. Entweder content oder file."`
	Note    string `json:"note,omitempty" jsonschema:"Eine Notiz zum Rohstück; liegt als <name>.note daneben und erscheint in inbox_list am Eintrag."`
}

type knowledgeInboxListInput struct {
	knowledgeBaseInput
	Source string `json:"source,omitempty" jsonschema:"Grenzt auf eine Quelle ein. Ohne Angabe alle."`
}

type knowledgeInboxReadInput struct {
	knowledgeBaseInput
	Path string `json:"path" jsonschema:"Pflicht. Pfad relativ zu k-playbook-local/inbox/, so wie inbox_list ihn nennt (<quelle>/<name>). Nur Textformate: md, markdown, txt, html, htm, json, yaml, yml, csv, xml, log."`
}

type knowledgeQueueAddInput struct {
	knowledgeBaseInput
	Origin string `json:"origin" jsonschema:"Pflicht. Ein Eingangspfad (<quelle>/<name>) oder eine Adresse draußen."`
	Target string `json:"target" jsonschema:"Pflicht. Das Zielverzeichnis relativ zu k-playbook-local/knowledge/, etwa extracted/ oder external/confluence/."`
	Reason string `json:"reason" jsonschema:"Pflicht. Warum das Rohstück Wissen werden soll, einzeilig."`
}

type knowledgeQueueListInput struct {
	knowledgeBaseInput
}

type knowledgeQueueDropInput struct {
	knowledgeBaseInput
	ID     string `json:"id" jsonschema:"Pflicht. Die Kennung des Eintrags, wie queue_list sie nennt."`
	Reason string `json:"reason,omitempty" jsonschema:"Warum der Eintrag ohne Übernahme fällt. Wird entgegengenommen, aber nirgends festgehalten: die Warteschlange hält Arbeit, nicht Geschichte."`
}

type knowledgeSupersedeInput struct {
	knowledgeBaseInput
	Path      string `json:"path" jsonschema:"Pflicht. Das abzulösende Dokument, relativ zu k-playbook-local/knowledge/."`
	Successor string `json:"successor" jsonschema:"Pflicht. Der Nachfolger, relativ zu k-playbook-local/knowledge/; er muss schon dort liegen."`
	Reason    string `json:"reason" jsonschema:"Pflicht. Warum das Dokument abgelöst wird, einzeilig; landet als superseded_reason im Frontmatter."`
}

// knowledgeEnvelope ist die Antwortform aller Wissens-Werkzeuge. Die Felder
// tragen dieselben Namen wie die Antworten von `k-playbook knowledge --json`.
//
// Hits und Entries sind Zeiger auf Scheiben, wie Content und Status: nur das
// Werkzeug, das sie beantwortet, setzt sie. Eine trefferlose Suche antwortet
// deshalb mit "hits": [] und nicht mit einer Antwort, in der der Schlüssel
// fehlt — das Subkommando tut dasselbe, und ein Aufrufer, der hits.length
// liest, soll nicht am leeren Ergebnis auflaufen. Umgekehrt trägt eine Antwort
// von read, write oder status den Schlüssel gar nicht erst, statt ihn auf null
// zu setzen und einen leeren Treffer vorzutäuschen.
//
// Hint meldet, was der Zugriff übergehen musste — ein nicht beschreibbares
// cache/, eine unlesbare Datei. Es ist kein Fehler: die Antwort steht.
type knowledgeEnvelope struct {
	OK         bool                            `json:"ok"`
	Tool       string                          `json:"tool"`
	ProjectDir string                          `json:"projectDir"`
	Query      string                          `json:"query,omitempty"`
	Source     string                          `json:"source,omitempty"`
	Kind       string                          `json:"kind,omitempty"`
	Hits       *[]project.Hit                  `json:"hits,omitempty"`
	Entries    *[]project.KnowledgeEntry       `json:"entries,omitempty"`
	Path       string                          `json:"path,omitempty"`
	Producer   string                          `json:"producer,omitempty"`
	Content    *string                         `json:"content,omitempty"`
	Written    bool                            `json:"written,omitempty"`
	Publish    *project.KnowledgePublishResult `json:"publish,omitempty"`
	Successor  string                          `json:"successor,omitempty"`
	Superseded bool                            `json:"superseded,omitempty"`
	Inbox      *[]project.InboxEntry           `json:"inbox,omitempty"`
	Queue      *[]project.QueueEntry           `json:"queue,omitempty"`
	ID         string                          `json:"id,omitempty"`
	Added      bool                            `json:"added,omitempty"`
	Dropped    bool                            `json:"dropped,omitempty"`
	Status     *project.KnowledgeStatus        `json:"status,omitempty"`
	Hint       string                          `json:"hint,omitempty"`
	Error      *knowledgeErrorEnvelope         `json:"error,omitempty"`
}

type knowledgeErrorEnvelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func addKnowledgeTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolSearch,
		Description: "Durchsucht die Wissensablage k-playbook-local/knowledge/ des Projekts abschnittsweise. " +
			"Jeder Treffer trägt path, heading, excerpt, kind (das Verzeichnis, der Eigentümer), origin und state " +
			"aus dem Frontmatter, rank (ab 1, die Reihenfolge ist die Aussage) und anchor für den Sprung in der " +
			"Oberfläche. Dokumente mit state raw oder superseded und die README in der Wurzel fehlen in den " +
			"Treffern. Hülle über `k-playbook knowledge search`.",
	}, knowledgeSearchTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolList,
		Description: "Listet die Dateien der Wissensablage k-playbook-local/knowledge/ mit path, title, kind sowie " +
			"origin und state aus dem Frontmatter; die README steht vorn, abgelöste und rohe Dokumente sind dabei. " +
			"Hülle über `k-playbook knowledge list`.",
	}, knowledgeListTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolRead,
		Description: "Liest eine Datei der Wissensablage k-playbook-local/knowledge/ als Markdown. " +
			"Hülle über `k-playbook knowledge read`.",
	}, knowledgeReadTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolWrite,
		Description: "Schreibt ein einzelnes Dokument in die Wissensablage k-playbook-local/knowledge/. Der Erzeuger " +
			"nennt sich (producer), der Pfad muss in seinem Verzeichnis liegen, sonst wird abgewiesen; die Generatoren " +
			"docs-code, docs-tools und inventory schreiben nur über publish. Das Frontmatter (title, subject, origin, " +
			"state, format, sources, updated) baut das Werkzeug aus den Feldern, der body ist Markdown ohne Kopf. " +
			"Ein genannter queue-Eintrag wird gelöscht, sobald das Dokument steht. Der Suchindex wird im selben Zug " +
			"nachgezogen. Hülle über `k-playbook knowledge write`.",
	}, knowledgeWriteTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolPublish,
		Description: "Veröffentlicht das Verzeichnis eines Generators (docs-code, docs-tools, inventory) in der " +
			"Wissensablage als Ganzes: der vollständige Satz wird geschrieben, was nicht darin ist, entfernt — atomar, " +
			"ein Abbruch lässt den vorherigen Stand stehen. Die Felder je Dokument sind die von write. " +
			"Hülle über `k-playbook knowledge publish`.",
	}, knowledgePublishTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolSupersede,
		Description: "Löst ein Dokument der Wissensablage ab: state wird superseded, successor und superseded_reason " +
			"kommen ins Frontmatter, der Rumpf bleibt, gelöscht wird nichts. Abgelöste Dokumente fehlen in search " +
			"und bleiben über read und list erreichbar. Hülle über `k-playbook knowledge supersede`.",
	}, knowledgeSupersedeTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolInboxPut,
		Description: "Legt ein Rohstück im Eingang k-playbook-local/inbox/<source>/<name> ab, so wie es kommt — jedes " +
			"Format, kein Frontmatter, keine Indizierung. Eine note liegt als <name>.note daneben. Ein belegter Name " +
			"wird abgewiesen; der Eingang ist ein Archiv. Hülle über `k-playbook knowledge inbox put`.",
	}, knowledgeInboxPutTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolInboxList,
		Description: "Listet die Rohstücke des Eingangs mit path, source, name, format, size, modified und der Notiz, " +
			"wo es eine gibt — alle oder die einer Quelle. Hülle über `k-playbook knowledge inbox list`.",
	}, knowledgeInboxListTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolInboxRead,
		Description: "Liest ein Rohstück des Eingangs als Text — nur Textformate, an der Endung erkannt; Bilder und " +
			"PDFs werden mit Meldung abgewiesen. Hülle über `k-playbook knowledge inbox read`.",
	}, knowledgeInboxReadTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolQueueAdd,
		Description: "Stellt ein Stück Arbeit in die Warteschlange k-playbook-local/queue/: dieses Rohstück (origin) " +
			"soll Wissen werden, in diesem Zielverzeichnis (target), aus diesem Grund (reason). Die Kennung vergibt " +
			"das Werkzeug. Der Eintrag fällt, wenn write ihn mit queue nennt und das Dokument steht. " +
			"Hülle über `k-playbook knowledge queue add`.",
	}, knowledgeQueueAddTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolQueueList,
		Description: "Nennt den Rückstand der Warteschlange: id, origin, target, reason, added und Notizen je Eintrag, " +
			"älteste zuerst. Leer heißt: nichts offen. Hülle über `k-playbook knowledge queue list`.",
	}, knowledgeQueueListTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolQueueDrop,
		Description: "Löscht einen Eintrag der Warteschlange ohne Übernahme. Der Grund wird nirgends festgehalten. " +
			"Hülle über `k-playbook knowledge queue drop`.",
	}, knowledgeQueueDropTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: knowledgeToolStatus,
		Description: "Meldet Art und Größe des Suchindex über k-playbook-local/knowledge/: indexKind, fileCount, chunkCount, " +
			"byKind, builtAt sowie stale/staleFiles, wenn der letzte Zugriff am Tor vorbei geänderte Dateien " +
			"neu eingelesen hat. Hülle über `k-playbook knowledge status`.",
	}, knowledgeStatusTool)
}

func knowledgeSearchTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeSearchInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolSearch, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		query := strings.TrimSpace(input.Query)
		kind := strings.TrimSpace(input.Kind)
		knowledge := project.NewKnowledge(projectDir)
		hits, err := knowledge.Search(query, project.KnowledgeFilter{Kind: kind}, input.Limit)
		if err != nil {
			return knowledgeFailure(knowledgeToolSearch, projectDir, "read_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolSearch, ProjectDir: projectDir,
			Query: query, Kind: kind, Hits: &hits, Hint: knowledgeHint(knowledge),
		}, false)
	}), nil, nil
}

func knowledgeListTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeListInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolList, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		kind := strings.TrimSpace(input.Kind)
		knowledge := project.NewKnowledge(projectDir)
		entries, err := knowledge.List(project.KnowledgeFilter{Kind: kind})
		if err != nil {
			return knowledgeFailure(knowledgeToolList, projectDir, "read_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolList, ProjectDir: projectDir,
			Kind: kind, Entries: &entries, Hint: knowledgeHint(knowledge),
		}, false)
	}), nil, nil
}

func knowledgeReadTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeReadInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolRead, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		content, err := project.NewKnowledge(projectDir).Read(input.Path)
		if err != nil {
			return knowledgeFailure(knowledgeToolRead, projectDir, "read_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolRead, ProjectDir: projectDir,
			Path: filepath.ToSlash(input.Path), Content: &content,
		}, false)
	}), nil, nil
}

func knowledgeWriteTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeWriteInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolWrite, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		knowledge := project.NewKnowledge(projectDir)
		// Der gemeldete Pfad kommt aus Write: der bereinigte Ort relativ zu
		// knowledge/ — so, wie read und search ihn nennen. Nachgebaut wird
		// er hier nicht; auch die Prüfung von Erzeuger, Ziel und Feldern
		// steht allein in project/.
		rel, err := knowledge.Write(input.Producer, input.document(), input.Queue)
		if err != nil {
			return knowledgeFailure(knowledgeToolWrite, projectDir, "write_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolWrite, ProjectDir: projectDir,
			Path: rel, Producer: strings.TrimSpace(input.Producer), Written: true,
			Hint: knowledgeHint(knowledge),
		}, false)
	}), nil, nil
}

func knowledgePublishTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgePublishInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolPublish, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		documents := make([]project.KnowledgeDocument, 0, len(input.Documents))
		for _, doc := range input.Documents {
			documents = append(documents, doc.document())
		}
		knowledge := project.NewKnowledge(projectDir)
		result, err := knowledge.Publish(input.Producer, documents)
		if err != nil {
			return knowledgeFailure(knowledgeToolPublish, projectDir, "write_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolPublish, ProjectDir: projectDir,
			Producer: result.Producer, Publish: &result, Hint: knowledgeHint(knowledge),
		}, false)
	}), nil, nil
}

func knowledgeSupersedeTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeSupersedeInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolSupersede, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		knowledge := project.NewKnowledge(projectDir)
		// Beide Pfade kommen bereinigt aus dem Kern — ./findings/y.md wird als
		// findings/y.md gemeldet, so, wie read und list es nennen.
		rel, next, err := knowledge.Supersede(input.Path, input.Successor, input.Reason)
		if err != nil {
			return knowledgeFailure(knowledgeToolSupersede, projectDir, "write_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolSupersede, ProjectDir: projectDir,
			Path: rel, Successor: next, Superseded: true,
			Hint: knowledgeHint(knowledge),
		}, false)
	}), nil, nil
}

func knowledgeInboxPutTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeInboxPutInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolInboxPut, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		content, err := inboxContent(input.Content, input.File)
		if err != nil {
			return knowledgeFailure(knowledgeToolInboxPut, projectDir, "write_failed", err)
		}
		knowledge := project.NewKnowledge(projectDir)
		rel, err := knowledge.InboxPut(input.Source, input.Name, content, input.Note)
		if err != nil {
			return knowledgeFailure(knowledgeToolInboxPut, projectDir, "write_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolInboxPut, ProjectDir: projectDir,
			Path: rel, Source: strings.TrimSpace(input.Source), Written: true,
		}, false)
	}), nil, nil
}

// inboxContent liest den Inhalt für inbox_put: content als Text oder file
// als Pfad auf diesem Rechner — genau eines von beiden. Beides, keines und
// eine Datei, die es nicht gibt, sind Eingabefehler; eine vorhandene, aber
// unlesbare Datei ist die Umgebung.
func inboxContent(content string, file string) ([]byte, error) {
	file = strings.TrimSpace(file)
	switch {
	case content != "" && file != "":
		return nil, project.InputErrorf("content und file zugleich — erwartet wird genau eines")
	case file != "":
		data, err := os.ReadFile(file)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, project.InputErrorf("%s gibt es nicht", file)
			}
			return nil, fmt.Errorf("%s lesen: %w", file, err)
		}
		return data, nil
	case content != "":
		return []byte(content), nil
	}
	return nil, project.InputErrorf("kein Inhalt: content oder file angeben")
}

func knowledgeInboxListTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeInboxListInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolInboxList, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		source := strings.TrimSpace(input.Source)
		entries, err := project.NewKnowledge(projectDir).InboxList(source)
		if err != nil {
			return knowledgeFailure(knowledgeToolInboxList, projectDir, "read_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolInboxList, ProjectDir: projectDir,
			Source: source, Inbox: &entries,
		}, false)
	}), nil, nil
}

func knowledgeInboxReadTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeInboxReadInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolInboxRead, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		content, err := project.NewKnowledge(projectDir).InboxRead(input.Path)
		if err != nil {
			return knowledgeFailure(knowledgeToolInboxRead, projectDir, "read_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolInboxRead, ProjectDir: projectDir,
			Path: filepath.ToSlash(strings.TrimSpace(input.Path)), Content: &content,
		}, false)
	}), nil, nil
}

func knowledgeQueueAddTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeQueueAddInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolQueueAdd, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		id, err := project.NewKnowledge(projectDir).QueueAdd(input.Origin, input.Target, input.Reason)
		if err != nil {
			return knowledgeFailure(knowledgeToolQueueAdd, projectDir, "write_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolQueueAdd, ProjectDir: projectDir, ID: id, Added: true,
		}, false)
	}), nil, nil
}

func knowledgeQueueListTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeQueueListInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolQueueList, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		knowledge := project.NewKnowledge(projectDir)
		entries, err := knowledge.QueueList()
		if err != nil {
			return knowledgeFailure(knowledgeToolQueueList, projectDir, "read_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolQueueList, ProjectDir: projectDir, Queue: &entries, Hint: knowledgeHint(knowledge),
		}, false)
	}), nil, nil
}

func knowledgeQueueDropTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeQueueDropInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolQueueDrop, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		if err := project.NewKnowledge(projectDir).QueueDrop(input.ID, input.Reason); err != nil {
			return knowledgeFailure(knowledgeToolQueueDrop, projectDir, "write_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolQueueDrop, ProjectDir: projectDir, ID: strings.TrimSpace(input.ID), Dropped: true,
		}, false)
	}), nil, nil
}

func knowledgeStatusTool(ctx context.Context, req *mcp.CallToolRequest, input knowledgeStatusInput) (*mcp.CallToolResult, any, error) {
	return wrapKnowledgeTool(knowledgeToolStatus, input.ProjectDir, func(projectDir string) *mcp.CallToolResult {
		knowledge := project.NewKnowledge(projectDir)
		status, err := knowledge.Status()
		if err != nil {
			return knowledgeFailure(knowledgeToolStatus, projectDir, "read_failed", err)
		}
		return knowledgeResult(knowledgeEnvelope{
			OK: true, Tool: knowledgeToolStatus, ProjectDir: projectDir,
			Status: &status, Hint: knowledgeHint(knowledge),
		}, false)
	}), nil, nil
}

// wrapKnowledgeTool löst das Projekt auf und übergibt dessen
// Hauptverzeichnis. Aufgelöst wird über resolveProjectDir, gemeinsam mit den
// übrigen Werkzeugfamilien.
func wrapKnowledgeTool(tool string, inputDir string, fn func(projectDir string) *mcp.CallToolResult) *mcp.CallToolResult {
	projectDir, err := resolveProjectDir(inputDir)
	if err != nil {
		return knowledgeErrorResult(tool, projectDir, "project_not_found", err)
	}
	return fn(projectDir)
}

// knowledgeHint fasst zusammen, was der Zugriff übergehen musste. Leer, wenn
// nichts anlag.
func knowledgeHint(knowledge *project.Knowledge) string {
	return strings.Join(knowledge.Notes(), "; ")
}

// knowledgeFailure ordnet einem Fehler aus project/ seinen Code zu: ein
// Eingabefehler (project.InputError — auch „nicht vorhanden") ist
// invalid_input, alles andere bekommt den Code der Hülle: write_failed bei
// den schreibenden (write, publish, supersede, inbox_put, queue_add,
// queue_drop), read_failed bei den lesenden. Ein Aufrufer, der am Code
// entscheidet, ob er die Argumente korrigiert oder aufgibt, korrigierte sonst
// bei einem nicht beschreibbaren knowledge/ endlos. Die Unterscheidung trifft
// der Kern, nicht die Hülle — hier wird nur gelesen, was er sagt.
func knowledgeFailure(tool string, projectDir string, fallback string, err error) *mcp.CallToolResult {
	code := fallback
	if project.IsInputError(err) {
		code = "invalid_input"
	}
	return knowledgeErrorResult(tool, projectDir, code, err)
}

func knowledgeErrorResult(tool string, projectDir string, code string, err error) *mcp.CallToolResult {
	return knowledgeResult(knowledgeEnvelope{
		OK:         false,
		Tool:       tool,
		ProjectDir: projectDir,
		Error:      &knowledgeErrorEnvelope{Code: code, Message: err.Error()},
	}, true)
}

func knowledgeResult(envelope knowledgeEnvelope, toolError bool) *mcp.CallToolResult {
	return jsonToolResult(envelope, toolError)
}
