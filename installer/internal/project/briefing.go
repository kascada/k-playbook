package project

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Das Briefing (Task 086): ein Aufruf zu Beginn einer Sitzung, der als Daten
// auflistet, welche Anweisungen, Regeln, Reviews und Leitlinien für dieses
// Projekt gelten und was in der Projekt-Collection für die eigene Arbeit
// ansteht. Verweise im Präfixschema, keine Pfade; gelesen wird über
// k_playbook_doc_read bzw. auf dem Weg des Präfixes. Inhalte stehen nicht
// darin — mit einer Ausnahme: den Text der immer geltenden Anweisungen
// (k-playbook:/instructions/always/) trägt das Briefing selbst
// (Documents.Always.Content), denn jede Sitzung muss ihn sofort lesen; ein
// zweiter Aufruf dafür wäre nur Umweg. Was an der Umgebung nicht stimmt,
// steht bewertet unter Health (Task 098). Umgebungsangaben — Pfade, Werkzeuge, gh, Kephalaion-Anbindung —
// stehen nur im Übergangsabschnitt Context (Task 091): dem Rest der
// Kontextausgabe von `k-playbook context`, den das Briefing noch nicht selbst
// abdeckt. Ein Aufruf des Briefings genügt damit für den Einstieg; die
// Kontextausgabe bleibt daneben unverändert, bis der Rest leer ist und sie
// entfallen kann. Umgestellt wird Schritt für Schritt.
//
// Die Listen kommen aus den vorhandenen Funktionen: die Dokumente über
// dieselbe Überlagerung wie die Verweise und die Kataloge (overlayDocDir,
// assembleDocDir), die eigene Arbeit über UserDocs.List — die Funktion hinter
// k_playbook_doc_list —, der Rest über ContextForDir, dieselbe Funktion wie
// `k-playbook context`.

// BriefingSchemaVersion ist die Fassung der Briefing-Ausgabe. Fassung 2 hat den
// Abschnitt context, Fassung 3 den Text der immer geltenden Anweisungen in
// documents.always.content, Fassung 4 den Abschnitt health.
const BriefingSchemaVersion = "4"

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
	// Health ist die Umgebungsprüfung (Task 098): Fehler und Warnungen über
	// die Umgebung des Projekts, alle auf einmal, je Eintrag Stufe, Bereich,
	// Satz und Abhilfe — dieselbe Antwort wie `k-playbook health --json` und
	// k_playbook_health. Die Sitzung nennt die Einträge in ihrer ersten
	// Antwort; health bewertet Felder des Abschnitts Context, ersetzt sie
	// aber nicht.
	Health health.Report `json:"health"`
	// Context ist der Übergangsabschnitt: was von `k-playbook context` noch
	// nicht in den eigenen Abschnitten des Briefings steht.
	Context BriefingContext `json:"context"`
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
// als einen Text liefert, der Text selbst und seine Teile in der Reihenfolge
// des Texts.
type BriefingAssembled struct {
	Ref string `json:"ref"`
	// Content ist der zusammengebaute Text — derselbe, den `k-playbook read`
	// und k_playbook_doc_read für Ref liefern (joinDocDirParts): die Teile
	// ohne Frontmatter, durch eine Leerzeile getrennt, abgeschaltete
	// übergangen. Leer ohne aufgenommene Teile.
	Content string          `json:"content"`
	Parts   []BriefingEntry `json:"parts"`
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

// BriefingContext ist der Übergangsabschnitt context des Briefings: die
// Kontextausgabe von `k-playbook context` ohne die Felder, die das Briefing
// schon selbst liefert (briefingCoveredContextFields). Er ist erkennbar ein
// Übergang, kein Teil des Briefings: Jeder spätere Schritt nimmt ein Feld
// heraus und gibt ihm einen eigenen Platz — oder lässt es entfallen. Ist der
// Abschnitt leer, können `k-playbook context` und k_playbook_context weg.
//
// Lässt sich die Kontextausgabe nicht bauen — keine Installation, unbekannte
// Fassung, unlesbare Konfiguration —, ist das ein Zustand wie bei
// BriefingWork: Available false mit Message, das übrige Briefing bleibt.
//
// Im JSON stehen die übrigen Felder unmittelbar unter context, unter ihren
// Namen aus der Kontextausgabe (context.project, context.links …), neben
// available und message. Darum serialisiert der Typ sich selbst.
type BriefingContext struct {
	// Available sagt, ob die Kontextausgabe gebaut werden konnte.
	Available bool
	// Message nennt, warum nicht; leer, wenn Available.
	Message string
	// Fields sind die übrigen Felder der Kontextausgabe, in deren Reihenfolge.
	Fields []BriefingContextField
}

// BriefingContextField ist ein Feld der Kontextausgabe im Abschnitt context:
// sein Name und sein Wert, so wie `k-playbook context` ihn ausgibt.
type BriefingContextField struct {
	Name  string
	Value json.RawMessage
}

// Field liefert den Wert eines Felds des Abschnitts, false ohne.
func (c BriefingContext) Field(name string) (json.RawMessage, bool) {
	for _, field := range c.Fields {
		if field.Name == name {
			return field.Value, true
		}
	}
	return nil, false
}

// MarshalJSON schreibt available, message und die übrigen Felder als ein
// Objekt.
func (c BriefingContext) MarshalJSON() ([]byte, error) {
	fields := []BriefingContextField{{Name: "available", Value: json.RawMessage(strconv.FormatBool(c.Available))}}
	if c.Message != "" {
		message, err := json.Marshal(c.Message)
		if err != nil {
			return nil, err
		}
		fields = append(fields, BriefingContextField{Name: "message", Value: message})
	}
	fields = append(fields, c.Fields...)
	return marshalOrderedObject(fields)
}

// UnmarshalJSON liest die Form von MarshalJSON zurück.
func (c *BriefingContext) UnmarshalJSON(data []byte) error {
	fields, err := unmarshalOrderedObject(data)
	if err != nil {
		return err
	}
	*c = BriefingContext{}
	for _, field := range fields {
		switch field.Name {
		case "available":
			if err := json.Unmarshal(field.Value, &c.Available); err != nil {
				return err
			}
		case "message":
			if err := json.Unmarshal(field.Value, &c.Message); err != nil {
				return err
			}
		default:
			c.Fields = append(c.Fields, field)
		}
	}
	return nil
}

// briefingCoveredContextFields nennt die Felder der Kontextausgabe, die das
// Briefing schon in eigenen Abschnitten liefert und die darum im Abschnitt
// context fehlen — als Pfade mit Punkt, je Eintrag der eigene Platz. Das ist
// die eine Stelle für diese Liste; der Test vergleicht gegen BuildContext,
// dass jedes andere Feld der Kontextausgabe im Abschnitt steht.
var briefingCoveredContextFields = []string{
	"catalogs.rules",   // documents.rules
	"catalogs.reviews", // documents.reviews
	"guidelines",       // documents.guidelines — dort überlagert, hier nur lokal
}

// briefingContextForDir ist die Quelle des Abschnitts: ContextForDir, dieselbe
// Funktion wie `k-playbook context` — kein Nachbau. Sie heilt dabei die
// Assistenten-Links; das Ergebnis steht wie dort unter links, hier als
// context.links. Austauschbar für Tests.
var briefingContextForDir = ContextForDir

// briefingContext baut den Abschnitt. Jeder Fehler ist ein Zustand.
func briefingContext(projectDir string) BriefingContext {
	built, err := briefingContextForDir(projectDir)
	if err != nil {
		return BriefingContext{Message: err.Error()}
	}
	fields, err := briefingContextFields(built)
	if err != nil {
		return BriefingContext{Message: err.Error()}
	}
	return BriefingContext{Available: true, Fields: fields}
}

// briefingContextFields serialisiert die Kontextausgabe und nimmt die
// abgedeckten Felder heraus. Der Umweg über JSON hält den Abschnitt an die
// Ausgabe von `k-playbook context` gebunden: ein Feld, das dort dazukommt,
// steht von selbst auch hier.
func briefingContextFields(built Context) ([]BriefingContextField, error) {
	data, err := json.Marshal(built)
	if err != nil {
		return nil, err
	}
	fields, err := unmarshalOrderedObject(data)
	if err != nil {
		return nil, err
	}
	for _, covered := range briefingCoveredContextFields {
		if fields, err = dropOrderedField(fields, strings.Split(covered, ".")); err != nil {
			return nil, fmt.Errorf("Kontextfeld %s auslassen: %w", covered, err)
		}
	}
	return fields, nil
}

// dropOrderedField nimmt das Feld unter path aus einem Objekt heraus, auch in
// der Tiefe. Fehlt es, bleibt das Objekt, wie es ist.
func dropOrderedField(fields []BriefingContextField, path []string) ([]BriefingContextField, error) {
	kept := make([]BriefingContextField, 0, len(fields))
	for _, field := range fields {
		if field.Name != path[0] {
			kept = append(kept, field)
			continue
		}
		if len(path) == 1 {
			continue
		}
		inner, err := unmarshalOrderedObject(field.Value)
		if err != nil {
			return nil, err
		}
		if inner, err = dropOrderedField(inner, path[1:]); err != nil {
			return nil, err
		}
		value, err := marshalOrderedObject(inner)
		if err != nil {
			return nil, err
		}
		kept = append(kept, BriefingContextField{Name: field.Name, Value: value})
	}
	return kept, nil
}

// unmarshalOrderedObject liest ein JSON-Objekt in seine Felder, in der
// Reihenfolge der Eingabe — die geht in einer map verloren.
func unmarshalOrderedObject(data []byte) ([]BriefingContextField, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("JSON-Objekt erwartet, %v bekommen", token)
	}
	fields := []BriefingContextField{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("Feldname erwartet, %v bekommen", token)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		fields = append(fields, BriefingContextField{Name: name, Value: value})
	}
	return fields, nil
}

// marshalOrderedObject schreibt Felder als JSON-Objekt in ihrer Reihenfolge.
func marshalOrderedObject(fields []BriefingContextField) ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, field := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		name, err := json.Marshal(field.Name)
		if err != nil {
			return nil, err
		}
		b.Write(name)
		b.WriteByte(':')
		b.Write(field.Value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// openBriefingDocs ist der Zugang zur Collection; austauschbar, damit Tests
// den Node nicht fragen.
var openBriefingDocs = OpenUserDocs

// BuildBriefing stellt das Briefing eines Projekts zusammen. Ein Fehler ist
// nur die Umgebung — eine Ebene, die sich nicht lesen lässt; eine nicht
// nutzbare Collection ist ein Zustand in Work, eine nicht baubare
// Kontextausgabe einer in Context, beides kein Fehler.
func BuildBriefing(ctx context.Context, projectDir string) (Briefing, error) {
	layers := ProjectDocLayers(projectDir)
	briefing := Briefing{
		SchemaVersion: BriefingSchemaVersion,
		ProjectDocs:   DocSchemeProject + ":/" + LocalDirName + "/docs/README.md",
	}

	always, err := briefingAssembled(ctx, layers, briefingAlwaysDir)
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
	briefing.Health = health.Check(ctx, projectDir)
	briefing.Context = briefingContext(projectDir)
	return briefing, nil
}

// briefingAssembled liefert ein zusammengebautes Verzeichnis: seine Teile
// mit Herkunft und den Text daraus, Includes eingesetzt wie bei
// `k-playbook read`; ein Teil, dessen Include scheitert, steht mit Kette
// unter problems und fehlt — das Briefing kommt trotzdem zustande.
// Gibt es das Verzeichnis in keiner Ebene, bleibt die Liste leer: der Verweis
// steht trotzdem da, damit die Sitzung weiß, worüber sie es lesen würde.
func briefingAssembled(ctx context.Context, layers DocLayers, dir string) (BriefingAssembled, error) {
	assembled := BriefingAssembled{Ref: DocSchemeKPlaybook + ":/" + dir + "/", Parts: []BriefingEntry{}, Problems: []string{}}
	parts, _, problems, _, err := assembleDocDir(ctx, layers, dir, DocReadOptions{})
	if err != nil {
		return BriefingAssembled{}, err
	}
	for _, part := range parts {
		assembled.Parts = append(assembled.Parts, briefingEntry(dir, docDirEntry{
			Name: part.Name, Origin: part.Origin, Disabled: part.Disabled, Content: part.content,
		}))
	}
	assembled.Content = joinDocDirParts(parts)
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
