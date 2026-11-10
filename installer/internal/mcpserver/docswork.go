package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
)

// Die Werkzeuge für erzeugte Doku mit mehreren Dokumenten (Task 109) —
// Hüllen über project.DocsWork.Checkout und .Publish, dem Weg, den auch
// `k-playbook docs checkout|publish` geht. Die erlaubten Erzeuger kommen aus
// der Erzeugertabelle project.DocsProducers.
//
// Dazu k_playbook_docs_exclude_add (Task 116): Hülle über
// project.AddDocsExclude wie `k-playbook docs exclude add`. Ein Command trägt
// damit dauerhafte Ausschlüsse ein, ohne K-PLAYBOOK.yaml selbst zu lesen. Und
// k_playbook_docs_readme (Task 137): Hülle über project.DocsReadmes wie
// `k-playbook docs readme`.
const (
	docsToolCheckout   = "k_playbook_docs_checkout"
	docsToolPublish    = "k_playbook_docs_publish"
	docsToolReadme     = "k_playbook_docs_readme"
	docsToolExcludeAdd = "k_playbook_docs_exclude_add"
)

// openDocsWork und openDocsReadmes sind austauschbar, damit Tests den Node
// nicht fragen.
var (
	openDocsWork    = project.OpenDocsWork
	openDocsReadmes = project.OpenDocsReadmes
)

type docsCheckoutInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Producer   string `json:"erzeuger" jsonschema:"Pflicht. Der Erzeuger; sein Verzeichnis ist keph:/docs/generated/<erzeuger>/."`
	Discard    bool   `json:"discard,omitempty" jsonschema:"Ein vorhandenes Arbeitsverzeichnis samt Merkdatei verwerfen und neu holen — nur, wenn der Nutzer das entschieden hat."`
}

type docsPublishInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Producer   string `json:"erzeuger" jsonschema:"Pflicht. Der Erzeuger, wie beim checkout."`
	AllowHints bool   `json:"allow_hints,omitempty" jsonschema:"hints ändern, ein Dokument mit hints löschen oder neu anlegen — nur nach Bestätigung durch den Nutzer. Die Antwort nennt jedes solche Dokument unter hintsChanged."`
}

type docsReadmeInput struct {
	ProjectDir string `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Path       string `json:"pfad,omitempty" jsonschema:"Ein Verzeichnis unter docs/, relativ zu docs/ (etwa generated/code) oder als Verweis keph:/docs/generated/code/; leer für alle Vorlagen."`
	Check      bool   `json:"check,omitempty" jsonschema:"Nichts schreiben, nur je Vorlage melden: missing, differs oder unchanged."`
}

type docsReadmeEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.DocsReadmeResult
	Error *errorEnvelope `json:"error,omitempty"`
}

type docsExcludeAddInput struct {
	ProjectDir string   `json:"projectDir,omitempty" jsonschema:"Pflicht. Verzeichnis, ab dem aufwärts nach K-PLAYBOOK.yaml gesucht wird. Relative Pfade werden relativ zum Arbeitsverzeichnis des MCP-Servers aufgelöst."`
	Patterns   []string `json:"patterns" jsonschema:"Pflicht. Die Muster, wie in einer .gitignore, relativ zu project.repo_root — nur die, deren Speichern der Nutzer bestätigt hat."`
}

type docsExcludeEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.DocsExcludeResult
	Error *errorEnvelope `json:"error,omitempty"`
}

// docsCheckoutEnvelope und docsPublishEnvelope sind die Antwortformen; die
// Felder des Ergebnisses fehlen bei einem Fehler.
type docsCheckoutEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.DocsCheckoutResult
	Error *errorEnvelope `json:"error,omitempty"`
}

type docsPublishEnvelope struct {
	OK         bool   `json:"ok"`
	Tool       string `json:"tool"`
	ProjectDir string `json:"projectDir"`
	*project.DocsPublishResult
	Error *errorEnvelope `json:"error,omitempty"`
}

// docsProducerSchema bildet das Eingabeschema aus dem Typ und setzt die
// erlaubten Werte von erzeuger aus der Erzeugertabelle.
func docsProducerSchema[T any]() *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("Schema für %T: %v", *new(T), err))
	}
	if prop, ok := schema.Properties["erzeuger"]; ok {
		for _, name := range project.DocsProducerNames() {
			prop.Enum = append(prop.Enum, name)
		}
	}
	return schema
}

func addDocsWorkTools(server *mcp.Server) {
	producers := strings.Join(project.DocsProducerNames(), ", ")
	mcp.AddTool(server, &mcp.Tool{
		Name:        docsToolCheckout,
		InputSchema: docsProducerSchema[docsCheckoutInput](),
		Description: "Holt erzeugte Doku eines Erzeugers (" + producers + ") aus der Projekt-Collection in ein lokales " +
			"Arbeitsverzeichnis: alle .md unter keph:/docs/generated/<erzeuger>/ außer README.md, rekursiv, nach " +
			"<projectDir>/.k-playbook/work/docs/<erzeuger>/ mit gleichem relativem Pfad; daneben die Merkdatei " +
			"<erzeuger>.checkout.json (Revision, Inhalts-Hash, hints je Dokument), an der publish misst. Dort bearbeitest " +
			"du mit den Datei-Werkzeugen und veröffentlichst mit k_playbook_docs_publish. Antwort: state checked_out " +
			"mit dir, record und docs (path, ref, revision) — oder state existing: Arbeitsverzeichnis oder Merkdatei " +
			"liegen schon da (existing: since, ageMinutes, files, record), nichts geholt, nichts überschrieben; dann " +
			"den Nutzer fragen: weitermachen oder verwerfen (discard: true holt neu). kephalaion_unavailable: nichts " +
			"geschrieben, kein Rückfall auf die Platte; ein unbekannter Erzeuger ist invalid_input. Hülle über " +
			"`k-playbook docs checkout`.",
	}, docsCheckoutTool)
	mcp.AddTool(server, &mcp.Tool{
		Name:        docsToolPublish,
		InputSchema: docsProducerSchema[docsPublishInput](),
		Description: "Veröffentlicht das Arbeitsverzeichnis eines Erzeugers (" + producers + ") nach " +
			"keph:/docs/generated/<erzeuger>/, gemessen an der Merkdatei vom checkout: neu → anlegen, geändert → " +
			"schreiben mit der Revision beim checkout, im Arbeitsverzeichnis gelöscht → löschen, unverändert → nichts. " +
			"Vorher geprüft, eine Ablehnung schreibt nichts: nur .md, keine README.md (invalid_input); jedes Dokument " +
			"mit Frontmatter title und description (invalid_input); hints unverändert, und kein Dokument mit hints " +
			"gelöscht oder neu angelegt — sonst hints_changed, außer mit allow_hints nach Bestätigung durch den Nutzer. " +
			"Ein Dokument, das seit dem checkout in Kephalaion geändert, angelegt oder gelöscht wurde, wird nicht " +
			"geschrieben, sondern als conflict gemeldet. Danach schreibt publish die README.md des Verzeichnisses aus ihrer " +
			"Vorlage k-playbook:/docs/generated/<erzeuger>/README.md, wenn sie fehlt oder abweicht; fehlt die Vorlage auf " +
			"dem Hub, ist readme failed mit template_missing, nichts davon geschrieben, aufgeräumt wird trotzdem. Antwort: docs und readme je mit path, ref, status (created, written, deleted, unchanged, " +
			"conflict, failed), revision, base (Revision beim checkout), bei conflict und failed code und message; " +
			"hintsChanged, open (offene Dateien) und cleaned. Arbeitsverzeichnis und Merkdatei werden nur entfernt, wenn " +
			"nichts offen ist; ein zweites publish schreibt dann nur den Rest. failed mit outcome_unknown: kann " +
			"angekommen sein — nicht noch einmal veröffentlichen, sondern nachsehen. not_checked_out: kein checkout. " +
			"kephalaion_unavailable: nichts geschrieben. Hülle über `k-playbook docs publish`.",
	}, docsPublishTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: docsToolReadme,
		Description: "Schreibt die README.md der Verzeichnisse unter keph:/docs/ aus ihren Vorlagen " +
			"k-playbook:/docs/<pfad>/README.md, die mit dem Hub ausgeliefert werden (nur mitgeliefert, keine lokale " +
			"Fassung): je Vorlage die README, die fehlt oder inhaltlich abweicht — anlegen, sonst schreiben mit ihrer " +
			"Revision, sonst nichts. Eine von Hand geänderte README wird überschrieben; Verzeichnisse ohne Vorlage und " +
			"docs/README.md ganz oben bleiben unberührt (dort mahnt Kephalaion). Ohne pfad alle Vorlagen, mit pfad die " +
			"eine. check: true schreibt nichts und meldet je Vorlage missing, differs oder unchanged. Antwort: readmes je " +
			"mit dir, ref, template, status (created, written, unchanged, failed; mit check missing, differs, unchanged), " +
			"revision, base, bei failed code und message — template_missing: die Vorlage fehlt auf dem Hub, nichts " +
			"geschrieben; outcome_unknown: kann angekommen sein, nachsehen —; hint, wenn es keine Vorlagen gibt. " +
			"kephalaion_unavailable: nichts geschrieben, kein Rückfall auf die Platte. Hülle über `k-playbook docs readme`.",
	}, docsReadmeTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: docsToolExcludeAdd,
		Description: "Trägt Muster dauerhaft in den Block docs.exclude von .k-playbook/K-PLAYBOOK.yaml ein — was die " +
			"Docs-Erzeuger beim Code-Scan zusätzlich zu ihren festen Ausschlüssen auslassen; gelesen wird die Liste aus " +
			"context.docs.exclude. Fehlt der Block, wird er angelegt; außerhalb des Blocks bleibt die Datei unverändert. " +
			"Muster gelten wie in einer .gitignore, relativ zu project.repo_root. Ein absolutes Muster, eines, das mit " +
			".. aus project.repo_root führt, oder eines mit Zeilenumbruch oder anderem Steuerzeichen wird nicht " +
			"eingetragen, sondern unter rejected mit reason gemeldet; eines, " +
			"das schon dasteht, wird übersprungen; ist nichts neu, bleibt die Datei unberührt (changed: false). Eingefügt " +
			"werden nur die neuen Zeilen, jede andere bleibt stehen; steht docs.exclude in einer Form, die sich nicht " +
			"sicher ergänzen lässt (etwa die Flussform exclude: [docs/]), ist das unsupported mit Zeile und Grund, und " +
			"nichts wird geschrieben. Nur nach " +
			"Bestätigung durch den Nutzer aufrufen. Antwort: config (Pfad), exclude (die Liste danach), added, rejected " +
			"(pattern, reason), changed. Danach das Briefing neu laden. Ohne Muster: invalid_input. Braucht keine " +
			"Kephalaion-Anbindung. Hülle über `k-playbook docs exclude add`.",
	}, docsExcludeAddTool)
}

func docsExcludeAddTool(_ context.Context, _ *mcp.CallToolRequest, input docsExcludeAddInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(docsExcludeEnvelope{OK: false, Tool: docsToolExcludeAdd, ProjectDir: dir,
			Error: &errorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	result, err := project.AddDocsExclude(projectDir, input.Patterns)
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	// Verworfene Muster sind kein Fehler des Werkzeugs: Die Antwort nennt sie
	// unter rejected, die gültigen sind trotzdem eingetragen.
	return jsonToolResult(docsExcludeEnvelope{OK: true, Tool: docsToolExcludeAdd, ProjectDir: projectDir, DocsExcludeResult: &result}, false), nil, nil
}

func docsReadmeTool(ctx context.Context, _ *mcp.CallToolRequest, input docsReadmeInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(docsReadmeEnvelope{OK: false, Tool: docsToolReadme, ProjectDir: dir,
			Error: &errorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	dir := ""
	if strings.TrimSpace(input.Path) != "" {
		if dir, err = project.CleanDocsDir(input.Path); err != nil {
			return fail(projectDir, docErrorCode(err), err), nil, nil
		}
	}
	readmes, err := openDocsReadmes(projectDir)
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	options := project.DocsReadmeOptions{Check: input.Check}
	var result project.DocsReadmeResult
	if dir == "" {
		if result, err = readmes.All(ctx, options); err != nil {
			return fail(projectDir, docErrorCode(err), err), nil, nil
		}
	} else {
		result = project.DocsReadmeResult{Check: input.Check, Readmes: []project.DocsReadmeItem{readmes.One(ctx, dir, options)}}
	}
	// Eine README mit failed ist kein Fehler des Werkzeugs: Die Antwort trägt
	// je README den Zustand.
	return jsonToolResult(docsReadmeEnvelope{OK: true, Tool: docsToolReadme, ProjectDir: projectDir, DocsReadmeResult: &result}, false), nil, nil
}

func docsCheckoutTool(ctx context.Context, _ *mcp.CallToolRequest, input docsCheckoutInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(docsCheckoutEnvelope{OK: false, Tool: docsToolCheckout, ProjectDir: dir,
			Error: &errorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	if _, err := project.DocsProducerByName(input.Producer); err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	work, err := openDocsWork(projectDir)
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	result, err := work.Checkout(ctx, input.Producer, project.DocsCheckoutOptions{Discard: input.Discard})
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	return jsonToolResult(docsCheckoutEnvelope{OK: true, Tool: docsToolCheckout, ProjectDir: projectDir, DocsCheckoutResult: &result}, false), nil, nil
}

func docsPublishTool(ctx context.Context, _ *mcp.CallToolRequest, input docsPublishInput) (*mcp.CallToolResult, any, error) {
	fail := func(dir string, code string, err error) *mcp.CallToolResult {
		return jsonToolResult(docsPublishEnvelope{OK: false, Tool: docsToolPublish, ProjectDir: dir,
			Error: &errorEnvelope{Code: code, Message: err.Error()}}, true)
	}
	projectDir, err := resolveProjectDir(input.ProjectDir)
	if err != nil {
		return fail(projectDir, projectDirErrorCode(err), err), nil, nil
	}
	if _, err := project.DocsProducerByName(input.Producer); err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	work, err := openDocsWork(projectDir)
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	result, err := work.Publish(ctx, input.Producer, project.DocsPublishOptions{AllowHints: input.AllowHints})
	if err != nil {
		return fail(projectDir, docErrorCode(err), err), nil, nil
	}
	// Ein publish mit offenen Dateien ist kein Fehler des Werkzeugs: Die
	// Antwort trägt je Dokument den Zustand, open und cleaned.
	return jsonToolResult(docsPublishEnvelope{OK: true, Tool: docsToolPublish, ProjectDir: projectDir, DocsPublishResult: &result}, false), nil, nil
}
