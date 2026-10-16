package project

import (
	"context"
	"path"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Das Briefing (Task 086): ein Aufruf zu Beginn einer Sitzung, der als Daten
// auflistet, welche Anweisungen, Regeln, Reviews und Leitlinien für dieses
// Projekt gelten und was in der Projekt-Collection für die eigene Arbeit
// ansteht. Verweise im Präfixschema, keine Pfade; gelesen wird über
// k_playbook_doc_read bzw. auf dem Weg des Präfixes. Inhalte stehen nicht
// darin, Umgebungsangaben — Pfade, Werkzeuge, gh, Kephalaion-Anbindung —
// ebenso wenig: die bleiben allein in `k-playbook context`. Es kommt neben
// den bisherigen Einstieg (k-playbook.md, AGENTS.md, context); umgestellt wird
// danach Schritt für Schritt.
//
// Die Listen kommen aus den vorhandenen Funktionen: die Dokumente über
// dieselbe Überlagerung wie die Verweise und die Kataloge (overlayDocDir,
// assembleDocDir), die eigene Arbeit über UserDocs.List — die Funktion hinter
// k_playbook_doc_list.

// BriefingSchemaVersion ist die Fassung der Briefing-Ausgabe.
const BriefingSchemaVersion = "1"

// BriefingOriginDisabled ist die Herkunft eines Eintrags, den ein leerer
// lokaler Eintrag abgeschaltet hat. Im Briefing ersetzt sie die Herkunft, denn
// für die Sitzung zählt nur: diesen Eintrag gibt es nicht zu lesen.
const BriefingOriginDisabled = "disabled"

// briefingAlwaysDir ist das zusammengebaute Verzeichnis der immer geltenden
// Anweisungen, briefingOnDemandDir das der Anweisungen auf Abruf.
const (
	briefingAlwaysDir   = docInstructionsRoot + "/always"
	briefingOnDemandDir = docInstructionsRoot + "/on-demand"
)

// Briefing ist die Antwort von `k-playbook briefing` und k_playbook_briefing.
type Briefing struct {
	SchemaVersion string `json:"schemaVersion"`
	// Documents sind die Dokumente, die für dieses Projekt gelten — lokal und
	// mitgeliefert schon gegeneinander aufgelöst.
	Documents BriefingDocuments `json:"documents"`
	// Work ist, was in der Projekt-Collection für die eigene Arbeit ansteht.
	Work BriefingWork `json:"work"`
	// ProjectDocs ist der Einstieg der Projektdokumentation, als Verweis.
	ProjectDocs string `json:"projectDocs"`
}

// BriefingDocuments sind die Dokumentlisten je Sorte, in Lesereihenfolge.
type BriefingDocuments struct {
	// Always ist das zusammengebaute Verzeichnis instructions/always/ mit
	// seinen Teilen.
	Always BriefingAssembled `json:"always"`
	// OnDemand sind die Anweisungen auf Abruf, Rules, Reviews und Guidelines
	// die Einträge der Sorten — jede als Datei einzeln zu lesen.
	OnDemand   []BriefingEntry `json:"onDemand"`
	Rules      []BriefingEntry `json:"rules"`
	Reviews    []BriefingEntry `json:"reviews"`
	Guidelines []BriefingEntry `json:"guidelines"`
}

// BriefingAssembled ist ein zusammengebautes Verzeichnis: der Verweis, der es
// als einen Text liefert, und seine Teile in der Reihenfolge des Texts.
type BriefingAssembled struct {
	Ref   string          `json:"ref"`
	Parts []BriefingEntry `json:"parts"`
	// Problems nennt, was übergangen wurde — eine Datei ohne Ziffer vorn.
	Problems []string `json:"problems"`
}

// BriefingEntry ist ein Dokument im Briefing.
type BriefingEntry struct {
	// Ref ist der Verweis, über den das Dokument gelesen wird.
	Ref string `json:"ref"`
	// Title ist der Titel: Frontmatter title, sonst die erste Überschrift,
	// sonst der Dateiname.
	Title string `json:"title"`
	// Description ist die Beschreibung aus dem Frontmatter, sonst leer.
	Description string `json:"description"`
	// Origin ist die Herkunft: dist, local, override oder disabled.
	Origin string `json:"origin"`
}

// BriefingWork ist die eigene Arbeit aus der Projekt-Collection. Ist Kephalaion
// nicht nutzbar, steht das in Available und Message, die Listen bleiben leer.
type BriefingWork struct {
	// Available sagt, ob die Collection gelesen werden konnte.
	Available bool `json:"available"`
	// Message nennt, warum nicht; leer, wenn Available.
	Message string `json:"message,omitempty"`
	// User ist der eigene Kephalaion-User, dessen Arbeit hier steht.
	User string `json:"user,omitempty"`
	// Tasks sind die eigenen offenen Tasks, Workbench die eigenen aktiven
	// Arbeitsstände — offen und aktiv heißt: nicht in done/.
	Tasks     []BriefingDoc `json:"tasks"`
	Workbench []BriefingDoc `json:"workbench"`
	// OpenFindings ist die Zahl der offenen Befunde in queue/findings/.
	OpenFindings int `json:"openFindings"`
}

// BriefingDoc ist ein Dokument der Collection im Briefing.
type BriefingDoc struct {
	Ref     string `json:"ref"`
	Title   string `json:"title"`
	Updated string `json:"updated,omitempty"`
}

// openBriefingDocs ist der Zugang zur Collection; austauschbar, damit Tests
// den Node nicht fragen.
var openBriefingDocs = OpenUserDocs

// BuildBriefing stellt das Briefing eines Projekts zusammen. Ein Fehler ist
// nur die Umgebung — eine Ebene, die sich nicht lesen lässt; eine nicht
// nutzbare Collection ist ein Zustand in Work, kein Fehler.
func BuildBriefing(ctx context.Context, projectDir string) (Briefing, error) {
	layers := ProjectDocLayers(projectDir)
	briefing := Briefing{
		SchemaVersion: BriefingSchemaVersion,
		ProjectDocs:   DocSchemeProject + ":/" + LocalDirName + "/docs/README.md",
	}

	always, err := briefingAssembled(layers, briefingAlwaysDir)
	if err != nil {
		return Briefing{}, err
	}
	briefing.Documents.Always = always

	markdown := catalogKind{name: "on-demand", dirName: briefingOnDemandDir, suffix: ".md"}
	for _, item := range []struct {
		kind   catalogKind
		target *[]BriefingEntry
	}{
		{markdown, &briefing.Documents.OnDemand},
		{catalogKindNamed("rules"), &briefing.Documents.Rules},
		{catalogKindNamed("reviews"), &briefing.Documents.Reviews},
		{guidelinesKind, &briefing.Documents.Guidelines},
	} {
		entries, err := briefingEntries(layers, item.kind)
		if err != nil {
			return Briefing{}, err
		}
		*item.target = entries
	}

	briefing.Work = briefingWork(ctx, projectDir)
	return briefing, nil
}

// briefingAssembled listet die Teile eines zusammengebauten Verzeichnisses.
// Gibt es das Verzeichnis in keiner Ebene, bleibt die Liste leer: der Verweis
// steht trotzdem da, damit die Sitzung weiß, worüber sie es lesen würde.
func briefingAssembled(layers DocLayers, dir string) (BriefingAssembled, error) {
	assembled := BriefingAssembled{Ref: DocSchemeKPlaybook + ":/" + dir + "/", Parts: []BriefingEntry{}, Problems: []string{}}
	parts, problems, _, err := assembleDocDir(layers, dir)
	if err != nil {
		return BriefingAssembled{}, err
	}
	for _, part := range parts {
		assembled.Parts = append(assembled.Parts, briefingEntry(dir, docDirEntry{
			Name: part.Name, Origin: part.Origin, Disabled: part.Disabled, Content: part.content,
		}))
	}
	assembled.Problems = append(assembled.Problems, problems...)
	return assembled, nil
}

// briefingEntries listet die Einträge einer Sorte, nach Namen sortiert, mit
// dem Muster der Sorte gefiltert wie im Katalog.
func briefingEntries(layers DocLayers, kind catalogKind) ([]BriefingEntry, error) {
	resolved, _, err := overlayDocDir(layers, kind.dirName, func(name string) bool { return isCatalogEntry(name, kind) })
	if err != nil {
		return nil, err
	}
	entries := make([]BriefingEntry, 0, len(resolved))
	for _, item := range resolved {
		entries = append(entries, briefingEntry(kind.dirName, item))
	}
	return entries, nil
}

// briefingEntry macht aus einem aufgelösten Eintrag die Zeile im Briefing.
func briefingEntry(dir string, item docDirEntry) BriefingEntry {
	entry := BriefingEntry{
		Ref:    DocSchemeKPlaybook + ":/" + path.Join(dir, item.Name),
		Origin: item.Origin,
	}
	if item.Disabled {
		entry.Origin = BriefingOriginDisabled
	}
	entry.Title, entry.Description = docTitleAndDescription(item.Name, item.Content)
	return entry
}

// docTitleAndDescription liest Titel und Beschreibung eines Dokuments: beide
// aus dem Frontmatter, der Titel ersatzweise aus der ersten Überschrift
// beliebiger Tiefe, sonst der Dateiname. Ein Frontmatter, das yamllite nicht
// versteht, ist kein Fehler: dann gilt die Überschrift.
func docTitleAndDescription(name string, content []byte) (string, string) {
	title, description := "", ""
	if block, ok := inventory.FrontmatterBlock(content); ok {
		if root, err := yamllite.Parse([]byte(block)); err == nil {
			title = strings.TrimSpace(root.Get("title").Str())
			description = strings.TrimSpace(root.Get("description").Str())
		}
	}
	if title == "" {
		title = firstHeading(inventory.Body(content))
	}
	if title == "" {
		title = name
	}
	return title, description
}

// firstHeading liefert den Text der ersten Markdown-Überschrift eines Rumpfs,
// leer ohne. Überschriften in Code-Blöcken zählen nicht.
func firstHeading(body []byte) string {
	inFence := false
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.HasPrefix(trimmed, "#") {
			continue
		}
		text := strings.TrimLeft(trimmed, "#")
		if text == "" || !strings.HasPrefix(text, " ") {
			continue
		}
		return strings.TrimSpace(text)
	}
	return ""
}

// briefingWork liest die eigene Arbeit aus der Collection: offene Tasks,
// aktive Arbeitsstände, die Zahl der offenen Befunde — genau das, was
// k_playbook_doc_list ohne done liefert. Jeder Fehler dabei ist ein Zustand:
// Available false mit Message, die Listen leer.
func briefingWork(ctx context.Context, projectDir string) BriefingWork {
	work := BriefingWork{Tasks: []BriefingDoc{}, Workbench: []BriefingDoc{}}
	docs, err := openBriefingDocs(projectDir)
	if err != nil {
		work.Message = err.Error()
		return work
	}
	user := docs.User()

	tasks, err := briefingDocs(ctx, docs, "tasks")
	if err != nil {
		work.Message = err.Error()
		return work
	}
	workbench, err := briefingDocs(ctx, docs, "workbench")
	if err != nil {
		work.Message = err.Error()
		return work
	}
	findings, err := docs.List(ctx, DocListOptions{Kind: "findings"})
	if err != nil {
		work.Message = err.Error()
		return work
	}

	work.Available = true
	work.User = user
	work.Tasks = tasks
	work.Workbench = workbench
	work.OpenFindings = len(findings.Items)
	return work
}

// briefingDocs listet die eigenen Dokumente einer Sorte, die nicht in done/
// liegen.
func briefingDocs(ctx context.Context, docs *UserDocs, kind string) ([]BriefingDoc, error) {
	list, err := docs.List(ctx, DocListOptions{Kind: kind})
	if err != nil {
		return nil, err
	}
	items := make([]BriefingDoc, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, BriefingDoc{Ref: item.Ref, Title: item.Title, Updated: item.Updated})
	}
	return items, nil
}
