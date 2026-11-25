package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Werkzeuge für Scope und Doku-Aufträge der Docs-Erzeuger (Task 179) —
// Hüllen über project.DocsScope und project.DocsOrders, dem Weg, den auch
// `k-playbook docs scope|orders|order add|orders split` geht.
const (
	docsToolScope       = "k_playbook_docs_scope"
	docsToolOrders      = "k_playbook_docs_orders"
	docsToolOrderAdd    = "k_playbook_docs_order_add"
	docsToolOrdersSplit = "k_playbook_docs_orders_split"
)

type docsScopeInput struct {
	ProjectDir string   `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Path       string   `json:"pfad,omitempty" jsonschema:"EXTRA_PATH: genau ein zusätzlicher Pfad für den Scope, relativ zu project.dir oder absolut; er muss existieren und innerhalb von project.dir liegen. Leer: kein zusätzlicher Pfad."`
	Check      []string `json:"check,omitempty" jsonschema:"Pfade (relativ zu project.repoRoot oder absolut), für die gemeldet wird, ob sie ausgeschlossen sind und warum."`
}

type docsScopeEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.DocsScopeResult
	Error *errorEnvelope `json:"error,omitempty"`
}

// openDocsOrders ist austauschbar, damit Tests den Node nicht fragen.
var openDocsOrders = project.OpenDocsOrders

type docsOrdersInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Producer   string `json:"erzeuger" jsonschema:"Pflicht. Der Erzeuger der Aufträge: code (/k-docs-code) oder tools (/k-docs-tools)."`
}

type docsOrdersEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.DocsOrdersResult
	Error *errorEnvelope `json:"error,omitempty"`
}

// docsOrderProducerSchema bildet das Eingabeschema aus dem Typ und setzt die
// erlaubten Werte von erzeuger aus project.DocsOrderProducers.
func docsOrderProducerSchema[T any]() *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("Schema für %T: %v", *new(T), err))
	}
	if prop, ok := schema.Properties["erzeuger"]; ok {
		for _, name := range project.DocsOrderProducers {
			prop.Enum = append(prop.Enum, name)
		}
	}
	return schema
}

type docsOrderAddInput struct {
	ProjectDir string   `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Producer   string   `json:"erzeuger" jsonschema:"Pflicht. Der Erzeuger des Auftrags: code (/k-docs-code) oder tools (/k-docs-tools)."`
	Paths      []string `json:"pfade" jsonschema:"Pflicht. Die geänderten Dateien oder Verzeichnisse, relativ zu project.repoRoot, so fein wie möglich; . heißt ganzes Projekt (der volle Lauf, den /k-docs einreiht)."`
	Anlass     string   `json:"anlass" jsonschema:"Pflicht. Ein Satz: was sich geändert hat, nicht was die Doku sagen soll."`
	Session    string   `json:"sitzung" jsonschema:"Pflicht. Die Kennung der Sitzung aus dem Briefing (Feld session, im Klartext die Zeile Sitzung:). Ein Auftrag je Sitzung und Erzeuger."`
	Title      string   `json:"title,omitempty" jsonschema:"Der Titel des Auftrags; ohne Angabe ist es der Anlass."`
}

type docsOrderAddEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.DocsOrderAddResult
	Error *errorEnvelope `json:"error,omitempty"`
}

type docsOrdersSplitInput struct {
	ProjectDir string                      `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Session    string                      `json:"sitzung" jsonschema:"Pflicht. Die Kennung der Sitzung des Verarbeiters aus dem Briefing (Feld session)."`
	Documents  []project.DocsSplitDocument `json:"documents" jsonschema:"Die vom Nutzer bestätigten Zeilen der Liste, je Dokument: dokument (<NN>-<slug>.md), aktion (neu, umschreiben, entfaellt), pfade (die Pfade der aufgegangenen Aufträge, die es betreffen; im vollen Lauf nur .), anlaesse (datum und anlass je aufgegangenem Auftrag), optional anlass (Zusammenfassung), entstanden (ältestes Datum) und abgelehnt (der Nutzer hat ein neu oder entfaellt abgelehnt)."`
	Raw        []string                    `json:"raw" jsonschema:"Die aufgegangenen rohen Aufträge als keph:-Verweise (keph:/queue/docs/<datei>); auch die, aus denen kein Dokument folgt."`
}

type docsOrdersSplitEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.DocsSplitResult
	Error *errorEnvelope `json:"error,omitempty"`
}

func addDocsOrderTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: docsToolScope,
		Description: "Rechnet den Scope der Docs-Erzeuger, die den Code durchsuchen (/k-docs-code, /k-docs-tools), nach den Regeln " +
			"in k-playbook:/commands/_docs/scope.md — nur lokal, ohne Kephalaion. Antwort: scanRoot " +
			"(project.repoRoot), stand (git, branch, commit, clean: `git status --porcelain` leer), bei pfad extra (path, abs, " +
			"exists, inProject, alwaysExcluded) und exclude: always (nie im Scan: /k-playbook/ und /.k-playbook/ nur direkt unter " +
			"project.dir, keph-backup/ und .git/ auf jeder Ebene), hidden (excluded, excepted), further (die weiteren festen Ausschlüsse), gitignore (je .gitignore unter " +
			"project.repoRoot file und patterns), config (docs.exclude) und rejected (verworfene Einträge mit reason); gitignoreTruncated (nur gesetzt, wenn die Grenze von 200 .gitignore gegriffen hat: die Zahl der nicht gelesenen). pfad ergänzt " +
			"den Scope um genau einen Pfad; fehlt er, liegt er außerhalb von project.dir oder in einem immer ausgeschlossenen " +
			"Verzeichnis, ist das invalid_input. check meldet je Pfad checks (path, exists — ob es ihn gibt —, suggest — der berichtigte Pfad bei doppelt " +
			"vorangestelltem Repo-Verzeichnis —, excluded, reason: always, hidden, further, gitignore, config oder outside, by). Ob ein anderer Stand beschrieben wird und was der Nutzer im Preflight ergänzt, bleibt bei der " +
			"Sitzung. Hülle über `k-playbook docs scope`.",
	}, docsScopeTool)
	mcp.AddTool(server, &mcp.Tool{
		Name:        docsToolOrders,
		InputSchema: docsOrderProducerSchema[docsOrdersInput](),
		Description: "Liest die offenen Doku-Aufträge eines Erzeugers (code, tools) aus keph:/queue/docs/ und bewertet sie — " +
			"die Rechnung aus Schritt 1 von /k-docs-code und aus k-playbook:/instructions/on-demand/doku-auftraege.md. Antwort: " +
			"orders je Auftrag mit ref, kind (raw oder split), title, erzeuger, dokument und aktion (nur zerlegt), pfade, anlass, " +
			"sitzung, entstanden, created, updated, revision, lines (Zeilen des Rumpfs) und bei einem rohen Auftrag für code waiting mit " +
			"remainingMinutes. Nur für code zusätzlich: waitMinutes (die Wartezeit, 120, nur im Programm; gerechnet mit der Uhr des " +
			"Programms an updated, ohne lesbares updated am Anlegezeitpunkt created), run (full: ein roher Auftrag mit pfade . nimmt alle rohen auf, auch die wartenden; partial: nur " +
			"die reifen; resume: nur offene zerlegte, ein früherer Lauf ist unterbrochen; none), rawOrders (RAW_ORDERS: die " +
			"genommenen rohen Aufträge nach dem Zusammenlegen — abhängige, also Pfad gleich oder darin, gelten als einer mit " +
			"vereinigten pfade, gesammelten anlaesse, dem ältesten entstanden und den aufgegangenen refs), splitOrders " +
			"(SPLIT_ORDERS) und waiting (WAITING: ref, pfade, remainingMinutes). nothingToDo (mit reason) heißt: kein Lauf — " +
			"keine offenen Aufträge oder nur wartende. problems nennt Aufträge, deren Frontmatter nicht lesbar ist (sie fehlen " +
			"in der Liste), und rohe Aufträge für code ohne lesbaren Zeitpunkt der Änderung und des Anlegens (sie gelten als reif " +
			"und werden abgearbeitet), Aufträge ohne gültigen Pfad (sie tragen incomplete und bleiben in orders), weggelassene " +
			"ungültige Pfade und berichtigte (Repo-Verzeichnis doppelt vorangestellt; der Auftrag in der Queue bleibt) — außer zu " +
			"unlesbaren Aufträgen nur für den gefragten Erzeuger. Für tools stehen nur orders und nothingToDo da. Die Zuordnung Pfad → Dokument und jede " +
			"Nutzerentscheidung bleiben bei der Sitzung. kephalaion_unavailable: Anbindung nicht bereit, kein Rückfall auf die " +
			"Platte. Hülle über `k-playbook docs orders`.",
	}, docsOrdersTool)
	mcp.AddTool(server, &mcp.Tool{
		Name:        docsToolOrderAdd,
		InputSchema: docsOrderProducerSchema[docsOrderAddInput](),
		Description: "Reiht einen rohen Doku-Auftrag in keph:/queue/docs/ ein oder ergänzt den offenen rohen Auftrag derselben " +
			"Sitzung — „Kein Doppel“ und „Wann nicht eingereiht wird“ aus k-playbook:/instructions/on-demand/doku-auftraege.md. " +
			"Für code werden die pfade mit denen des vorhandenen Auftrags vereinigt und der Anlass wird eine Zeile im Rumpf " +
			"(Datum — Anlass), mit der Revision gelesen und bei stale_revision neu; deckt der Auftrag die neuen pfade schon, " +
			"wird nichts geschrieben. Für tools genügt der vorhandene Auftrag. Ein Auftrag mit pfade . ist der volle Lauf von " +
			"/k-docs: Für ihn gelten „Ein Auftrag je Sitzung“, „Kein Doppel“ und „Noch keine erzeugte Doku“ nicht; ein offener " +
			"roher Auftrag mit pfade . deckt ihn (covered). Antwort: status — created (neuer Auftrag), extended (ergänzt), covered " +
			"(ein offener Auftrag deckt schon, nichts geschrieben) oder skipped mit reason: no_generated_docs (unter " +
			"keph:/docs/generated/<erzeuger>/ liegt noch kein Dokument) oder kephalaion_unavailable (Anbindung nicht bereit, " +
			"nicht auf die Platte eingereiht; das ist kein Fehler des Werkzeugs, der Abschluss nennt den Nachzug als offen) —, " +
			"dazu ref, revision, pfade und message, corrected (from, to: ein doppelt vorangestelltes Repo-Verzeichnis, " +
			"berichtigt) und missing (Pfade, die es auch danach nicht gibt; angenommen, der Status bleibt). Fehlende Eingaben sind invalid_input. Hülle über `k-playbook docs order add`.",
	}, docsOrderAddTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: docsToolOrdersSplit,
		Description: "Schreibt die zerlegten Doku-Aufträge von /k-docs-code in keph:/queue/docs/ und schließt, was darin aufgegangen " +
			"ist — „In die Queue schreiben“, Schritt 4 von k-playbook:/commands/k-docs-code.md. Eingabe: die vom Nutzer bestätigten " +
			"Zeilen (documents) und die aufgegangenen rohen Aufträge (raw); alles wird geprüft, bevor etwas geschrieben wird " +
			"(invalid_input). Je Dokument wird ein zerlegter Auftrag angelegt (Kurzname code-<dokument ohne .md>, Kopf mit " +
			"dokument, aktion, pfade, anlass, sitzung, entstanden, im Rumpf je Anlass eine Zeile Datum — Anlass) oder der offene " +
			"für dasselbe Dokument ergänzt (pfade vereinigt, aktion neu gesetzt, nur neue Anlass-Zeilen, entstanden bleibt das " +
			"älteste Datum; Neulesen bei stale_revision). Danach wandern die rohen Aufträge nach done/ — nur wenn jeder zerlegte " +
			"Auftrag steht, sonst bleiben sie offen (kept). Ein roher Auftrag, der noch wartet (Wartezeit nicht um), wird mit invalid_input abgelehnt und nichts geschrieben, außer unter den rohen der Eingabe steht ein voller Lauf (pfade .). Ein abgelehntes neu oder entfaellt (abgelehnt) wird geschrieben und " +
			"gleich geschlossen, ohne publish. Antwort: documents je Dokument mit status (created, extended, skipped mit reason " +
			"nothing_to_add, failed mit code und message), ref, revision, closed und doneRef; raw je roher Auftrag mit status " +
			"(closed mit doneRef, gone: ein anderer Lauf hat ihn schon abgeschlossen, kept, failed); closedRaw, die geschlossenen " +
			"rohen Aufträge. outcome_unknown bei failed: nicht wiederholen, mit k_playbook_doc_list nachsehen. " +
			"kephalaion_unavailable: Anbindung nicht bereit, nichts geschrieben. Die Zuordnung Pfad → Dokument und neu/entfällt " +
			"entscheidet die Sitzung mit dem Nutzer. Hülle über `k-playbook docs orders split --json`.",
	}, docsOrdersSplitTool)
}

func docsOrdersSplitTool(ctx context.Context, _ *mcp.CallToolRequest, input docsOrdersSplitInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(docsOrdersSplitEnvelope{OK: false, Tool: docsToolOrdersSplit, ProjectDir: dir,
			Error: &errorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	orders, err := openDocsOrders(projectDir)
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	result, err := orders.Split(ctx, project.DocsSplitInput{Sitzung: input.Session, Documents: input.Documents, Raw: input.Raw})
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	// Ein Dokument mit failed ist kein Fehler des Werkzeugs: Die Antwort trägt
	// je Dokument und je rohem Auftrag den Zustand.
	return jsonToolResult(docsOrdersSplitEnvelope{OK: true, Tool: docsToolOrdersSplit, ProjectDir: projectDir, DocsSplitResult: &result}, false), nil, nil
}

func docsOrderAddTool(ctx context.Context, _ *mcp.CallToolRequest, input docsOrderAddInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(docsOrderAddEnvelope{OK: false, Tool: docsToolOrderAdd, ProjectDir: dir,
			Error: &errorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	if err := project.ValidDocsOrderProducer(input.Producer); err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	var result project.DocsOrderAddResult
	orders, err := openDocsOrders(projectDir)
	switch {
	case err == nil:
		result, err = orders.Add(ctx, project.DocsOrderAddInput{
			Producer: input.Producer, Pfade: input.Paths, Anlass: input.Anlass, Title: input.Title, Sitzung: input.Session})
		if err != nil {
			return fail(projectDir, docErrorCode(err), err), nil, nil
		}
	case project.DocRefErrorCode(err) == project.DocCodeKephalaionUnavailable:
		result = project.DocsOrderAddUnavailable(input.Producer, err)
	default:
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	return jsonToolResult(docsOrderAddEnvelope{OK: true, Tool: docsToolOrderAdd, ProjectDir: projectDir, DocsOrderAddResult: &result}, false), nil, nil
}

func docsOrdersTool(ctx context.Context, _ *mcp.CallToolRequest, input docsOrdersInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(docsOrdersEnvelope{OK: false, Tool: docsToolOrders, ProjectDir: dir,
			Error: &errorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	if err := project.ValidDocsOrderProducer(input.Producer); err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	orders, err := openDocsOrders(projectDir)
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	result, err := orders.Orders(ctx, input.Producer)
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	return jsonToolResult(docsOrdersEnvelope{OK: true, Tool: docsToolOrders, ProjectDir: projectDir, DocsOrdersResult: &result}, false), nil, nil
}

func docsScopeTool(ctx context.Context, _ *mcp.CallToolRequest, input docsScopeInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(docsScopeEnvelope{OK: false, Tool: docsToolScope, ProjectDir: dir,
			Error: &errorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	result, err := project.DocsScope(ctx, projectDir, project.DocsScopeOptions{ExtraPath: input.Path, Check: input.Check})
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	return jsonToolResult(docsScopeEnvelope{OK: true, Tool: docsToolScope, ProjectDir: projectDir, DocsScopeResult: &result}, false), nil, nil
}
