package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Erzeugte Doku mit mehreren Dokumenten: auschecken → bearbeiten →
// veröffentlichen (Task 109, Workbench 004 Entscheidungen 14–17).
//
// `k-playbook docs checkout <erzeuger>` holt docs/generated/<erzeuger>/ der
// Projekt-Collection in ein rechnerlokales Arbeitsverzeichnis unter
// .k-playbook/work/docs/<erzeuger>/ und merkt sich daneben je Dokument
// Revision, Inhalts-Hash und hints. Die KI bearbeitet dort mit gewöhnlichen
// Datei-Werkzeugen. `k-playbook docs publish <erzeuger>` gleicht ab: neu →
// create, geändert → write mit der Revision beim checkout, gelöscht → delete
// mit dieser Revision. Was inzwischen in Kephalaion geändert ist, wird
// gemeldet, nicht überschrieben. Die README des Verzeichnisses schreibt
// publish aus der Erzeugertabelle; die KI schreibt sie nie.
//
// Ohne bereite Anbindung gibt es keinen Lauf und keinen Rückfall auf die
// Platte.

// DocsProducer ist ein Eintrag der Erzeugertabelle: ein Verzeichnis unter
// docs/generated/ der Projekt-Collection, das über checkout/publish
// geschrieben wird. Title, Description und Text bilden die README des
// Verzeichnisses.
type DocsProducer struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Text        string `json:"text"`
}

// DocsProducers ist die Erzeugertabelle. Ein Verzeichnis kann mehrere
// Schreiber haben (in code/ /k-docs-code und ks-overlay-repo-analyse,
// erkennbar an generated.by); die Tabelle kennt das Verzeichnis, nicht die
// Schreiber. libs/ schreibt /k-docs-tools (Task 120). versions/ steht nicht
// darin: das Inventar ist ein Dokument und schreibt selbst (inventory.DocName).
var DocsProducers = []DocsProducer{
	{
		Name:  "code",
		Title: "Doku aus dem Code",
		Description: "Erzeugte Projektdoku aus dem Code — ein Dokument je Thema, nach Bedeutung statt nach Dateien, " +
			"geschrieben von /k-docs-code und dem Skill ks-overlay-repo-analyse.",
		Text: "Jedes Dokument beschreibt ein Thema des Codes — Zweck, Ablauf, wichtige Stellen als `pfad:zeile`, " +
			"Randfälle. Wer es geschrieben hat, steht im Frontmatter unter `generated.by` (`k-docs-code` oder " +
			"`ks-overlay-repo-analyse`); `questions` nennt die Fragen, die das Dokument beantwortet.\n\n" +
			"Von Hand gepflegt sind nur die `hints` im Frontmatter: kurze Sätze, worauf der nächste Lauf achten soll. " +
			"Alles andere wird beim nächsten Lauf neu geschrieben.\n\n" +
			"Geschrieben wird über `k-playbook docs checkout code` und `k-playbook docs publish code`. Diese README " +
			"schreibt `publish` selbst.",
	},
	{
		Name:  "libs",
		Title: "Steckbriefe zu Libraries und Tools",
		Description: "Kuratierte Steckbriefe zu den nicht-trivialen Libraries und Tools des Projekts — Pitfalls und Idiome, " +
			"je Gegenstand des Versionsinventars ein Dokument, geschrieben von /k-docs-tools.",
		Text: "Jedes Dokument beschreibt eine Library oder ein Tool aus dem Versionsinventar " +
			"(`keph:/docs/generated/versions/inventory.md`) — Rolle im Projekt, Version und Migration, Pitfalls, Idiome " +
			"und Anti-Patterns; kein Ersatz für die offizielle Doku. Gegen welchen Eintrag und welche Version des " +
			"Inventars gelesen wurde, steht im Frontmatter unter `sources`; weicht das Inventar inzwischen ab, gibt es " +
			"die Auskunft. Wer es geschrieben hat, steht unter `generated.by` (`k-docs-tools`); `questions` nennt die " +
			"Fragen, die das Dokument beantwortet.\n\n" +
			"Von Hand gepflegt sind nur die `hints` im Frontmatter: kurze Sätze, worauf der nächste Lauf achten soll. " +
			"Alles andere wird beim nächsten Lauf neu geschrieben.\n\n" +
			"Geschrieben wird über `k-playbook docs checkout libs` und `k-playbook docs publish libs`. Diese README " +
			"schreibt `publish` selbst.",
	},
}

// Die Codes von docs checkout und publish über die der doc-Werkzeuge hinaus.
const (
	// DocCodeNotCheckedOut: publish ohne vorheriges checkout — Merkdatei
	// oder Arbeitsverzeichnis fehlen.
	DocCodeNotCheckedOut = "not_checked_out"
	// DocCodeHintsChanged: hints eines Dokuments weichen vom checkout ab, ein
	// Dokument mit hints wird gelöscht oder neu angelegt — ohne allow_hints.
	// Nichts wurde geschrieben.
	DocCodeHintsChanged = "hints_changed"
)

// docsGeneratedRoot ist das Verzeichnis der Erzeuger in der Collection.
const docsGeneratedRoot = "docs/generated"

// docsReadmeName ist die README eines Erzeuger-Verzeichnisses, geschrieben
// von publish.
const docsReadmeName = "README.md"

// docsWorkRoot ist das Arbeitsverzeichnis unter ProjectDirName;
// docsIgnoreLine nimmt es aus der Versionskontrolle.
const (
	docsWorkRoot   = "work"
	docsIgnoreLine = "/" + docsWorkRoot
)

// docsRecordSuffix ist die Endung der Merkdatei neben dem Arbeitsverzeichnis.
const docsRecordSuffix = ".checkout.json"

// DocsProducerByName sucht einen Erzeuger; ein unbekannter ist
// invalid_input.
func DocsProducerByName(name string) (DocsProducer, error) {
	for _, producer := range DocsProducers {
		if producer.Name == name {
			return producer, nil
		}
	}
	return DocsProducer{}, docRefErrorf(DocCodeInvalidInput, "unbekannter Erzeuger %q — erlaubt sind %s",
		name, strings.Join(DocsProducerNames(), ", "))
}

// DocsProducerNames sind die Namen der Erzeugertabelle in ihrer Reihenfolge.
func DocsProducerNames() []string {
	names := make([]string, 0, len(DocsProducers))
	for _, producer := range DocsProducers {
		names = append(names, producer.Name)
	}
	return names
}

// ValidateDocsProducers prüft eine Erzeugertabelle.
func ValidateDocsProducers(producers []DocsProducer) error {
	seen := map[string]bool{}
	for i, producer := range producers {
		label := fmt.Sprintf("Erzeuger %d (%q)", i+1, producer.Name)
		if !docNamePattern.MatchString(producer.Name) || strings.Contains(producer.Name, ".") {
			return fmt.Errorf("%s: ungültiger Name", label)
		}
		if seen[producer.Name] {
			return fmt.Errorf("%s: doppelt", label)
		}
		seen[producer.Name] = true
		if strings.TrimSpace(producer.Title) == "" || strings.TrimSpace(producer.Description) == "" || strings.TrimSpace(producer.Text) == "" {
			return fmt.Errorf("%s: title, description und text sind Pflicht", label)
		}
		if strings.ContainsAny(producer.Title+producer.Description, "\n") {
			return fmt.Errorf("%s: title und description sind je eine Zeile", label)
		}
	}
	return nil
}

// dir ist das Verzeichnis des Erzeugers in der Collection.
func (p DocsProducer) dir() string { return docsGeneratedRoot + "/" + p.Name }

// Ref ist das Verzeichnis als keph:-Verweis.
func (p DocsProducer) Ref() string { return "keph:/" + p.dir() + "/" }

// docName ist der Name eines Dokuments in der Collection zu einem Pfad im
// Arbeitsverzeichnis.
func (p DocsProducer) docName(rel string) string { return p.dir() + "/" + rel }

// readme ist der Inhalt der README des Verzeichnisses.
func (p DocsProducer) readme() string {
	return "---\ntitle: " + docsYAMLScalar(p.Title) + "\ndescription: " + docsYAMLScalar(p.Description) +
		"\n---\n\n# " + p.Title + "\n\n" + strings.TrimRight(p.Text, "\n") + "\n"
}

// docsYAMLScalar setzt einen einzeiligen Wert in doppelte Anführungszeichen,
// wenn er sonst als YAML anders gelesen würde.
func docsYAMLScalar(value string) string {
	if strings.ContainsAny(value, ":#'\"[]{},&*!|>%@`") || strings.TrimSpace(value) != value {
		quoted, _ := json.Marshal(value)
		return string(quoted)
	}
	return value
}

// DocsWorkDir ist das Arbeitsverzeichnis eines Erzeugers im Projekt.
func DocsWorkDir(projectDir, producer string) string {
	return filepath.Join(projectDir, ProjectDirName, docsWorkRoot, "docs", producer)
}

// DocsRecordFile ist die Merkdatei neben dem Arbeitsverzeichnis.
func DocsRecordFile(projectDir, producer string) string {
	return DocsWorkDir(projectDir, producer) + docsRecordSuffix
}

// DocsStore sind die Aufrufe an Kephalaion, die checkout und publish
// brauchen. *kephalaion.Client erfüllt sie; Tests setzen kephtest.Store ein.
// List und Read lesen die Replica des Nodes, Create, Write und Delete gehen
// über den Hub; Write und Delete mit einer baseRevision, die nicht mehr die
// des Dokuments ist, scheitern mit kephalaion.CodeStaleRevision.
type DocsStore interface {
	List(ctx context.Context, req kephalaion.ListRequest) ([]kephalaion.Entry, error)
	Read(ctx context.Context, name string) (kephalaion.ReadResult, error)
	Create(ctx context.Context, name, content string) (kephalaion.Document, error)
	Write(ctx context.Context, name, content string, baseRevision int64) (kephalaion.Document, error)
	Delete(ctx context.Context, name string, baseRevision int64) (kephalaion.Document, error)
}

// DocsWork ist der Zugang zu checkout und publish für ein Projekt.
type DocsWork struct {
	projectDir string
	store      DocsStore
	address    string
	now        func() time.Time
}

// NewDocsWork baut den Zugang. address ist die Collection
// (<hub>:<collection>); sie steht in der Merkdatei, und publish lehnt ein
// checkout aus einer anderen ab.
func NewDocsWork(projectDir string, store DocsStore, address string) *DocsWork {
	return &DocsWork{projectDir: projectDir, store: store, address: address, now: time.Now}
}

// OpenDocsWork prüft die Anbindung des Projekts und baut den Zugang —
// derselbe Weg wie OpenUserDocs, Fehler mit DocCodeKephalaionUnavailable.
// Austauschbar, damit Subkommando und MCP-Tests den Node nicht fragen.
var OpenDocsWork = func(projectDir string) (*DocsWork, error) {
	client, _, err := openKephClient(projectDir)
	if err != nil {
		return nil, err
	}
	return NewDocsWork(projectDir, client, client.Address()), nil
}

// docsRecord ist die Merkdatei: der Stand beim checkout, an dem publish
// „geändert“ und „hints unverändert“ misst — nicht am aktuellen Stand in
// Kephalaion.
type docsRecord struct {
	Producer   string           `json:"producer"`
	Collection string           `json:"collection"`
	CheckedOut string           `json:"checkedOut"`
	Docs       []docsRecordItem `json:"docs"`
}

// docsRecordItem ist ein geholtes (oder seither veröffentlichtes) Dokument.
// Path ist der Pfad im Arbeitsverzeichnis mit Schrägstrichen, Hash der
// SHA-256 des Inhalts (hex), Hints die hints aus seinem Frontmatter.
type docsRecordItem struct {
	Path     string   `json:"path"`
	Revision int64    `json:"revision"`
	Hash     string   `json:"hash"`
	Hints    []string `json:"hints,omitempty"`
}

// find liefert den Eintrag zu einem Pfad.
func (r *docsRecord) find(rel string) (docsRecordItem, bool) {
	for _, item := range r.Docs {
		if item.Path == rel {
			return item, true
		}
	}
	return docsRecordItem{}, false
}

// set ersetzt oder ergänzt den Eintrag zu einem Pfad.
func (r *docsRecord) set(item docsRecordItem) {
	for i := range r.Docs {
		if r.Docs[i].Path == item.Path {
			r.Docs[i] = item
			return
		}
	}
	r.Docs = append(r.Docs, item)
	slices.SortFunc(r.Docs, func(a, b docsRecordItem) int { return strings.Compare(a.Path, b.Path) })
}

// remove streicht den Eintrag zu einem Pfad.
func (r *docsRecord) remove(rel string) {
	r.Docs = slices.DeleteFunc(r.Docs, func(item docsRecordItem) bool { return item.Path == rel })
}

// readDocsRecord liest die Merkdatei; ok ist false, wenn sie fehlt.
func readDocsRecord(file string) (docsRecord, bool, error) {
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return docsRecord{}, false, nil
	}
	if err != nil {
		return docsRecord{}, false, fmt.Errorf("Merkdatei %s lesen: %w", file, err)
	}
	var record docsRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return docsRecord{}, false, docRefErrorf(DocCodeInvalidInput, "Merkdatei %s nicht lesbar: %v — mit checkout --discard neu holen", file, err)
	}
	return record, true, nil
}

// writeDocsRecord schreibt die Merkdatei.
func writeDocsRecord(file string, record docsRecord) error {
	if record.Docs == nil {
		record.Docs = []docsRecordItem{}
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", filepath.Dir(file), err)
	}
	if err := os.WriteFile(file, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("Merkdatei %s schreiben: %w", file, err)
	}
	return nil
}

// docsHash ist der Inhalts-Hash der Merkdatei.
func docsHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// docsRelPath prüft einen Pfad im Arbeitsverzeichnis (Schrägstriche, ohne
// führenden): jedes Segment folgt der Namensregel von Kephalaion, das letzte
// endet auf .md und heißt nicht README.md. Zurück kommt die Ablehnung als
// Satz, leer bei einem gültigen Pfad.
func docsRelPath(rel string) string {
	if rel == "" || strings.HasPrefix(rel, "/") || path.Clean(rel) != rel || rel == ".." || strings.HasPrefix(rel, "../") {
		return "kein Pfad unterhalb des Arbeitsverzeichnisses"
	}
	segments := strings.Split(rel, "/")
	last := segments[len(segments)-1]
	if strings.EqualFold(last, docsReadmeName) {
		return "README.md schreibt publish selbst; im Arbeitsverzeichnis ist sie nicht erlaubt"
	}
	if !strings.HasSuffix(last, ".md") {
		return "nur .md-Dateien werden veröffentlicht"
	}
	for _, segment := range segments {
		if !docNamePattern.MatchString(segment) {
			return fmt.Sprintf("%q folgt nicht der Namensregel von Kephalaion (a–z, 0–9, . _ -, vorn Buchstabe oder Ziffer, höchstens 63 Zeichen)", segment)
		}
	}
	return ""
}

// docsMeta ist, was publish am Frontmatter eines Dokuments prüft.
type docsMeta struct {
	Title       string
	Description string
	Hints       []string
}

// parseDocsMeta liest title, description und hints aus dem Frontmatter.
// Zurück kommen die Ablehnungen als Sätze; leer heißt gültig: Frontmatter
// vorhanden und lesbar, title und description nicht leer, hints fehlt, ist
// leer oder eine Liste einzeiliger Sätze.
func parseDocsMeta(content []byte) (docsMeta, []string) {
	block, ok := inventory.FrontmatterBlock(content)
	if !ok {
		return docsMeta{}, []string{"Frontmatter fehlt (--- am Anfang, title und description)"}
	}
	root, err := yamllite.Parse([]byte(block))
	if err != nil {
		return docsMeta{}, []string{fmt.Sprintf("Frontmatter nicht lesbar: %v", err)}
	}
	var problems []string
	meta := docsMeta{
		Title:       strings.TrimSpace(root.Get("title").Str()),
		Description: strings.TrimSpace(root.Get("description").Str()),
	}
	if meta.Title == "" {
		problems = append(problems, "title fehlt im Frontmatter")
	}
	if meta.Description == "" {
		problems = append(problems, "description fehlt im Frontmatter")
	}
	hints, problem := docsHintsOf(root.Get("hints"))
	if problem != "" {
		problems = append(problems, problem)
	}
	meta.Hints = hints
	return meta, problems
}

// docsHintsOf liest hints: eine Liste einzeiliger Sätze. Fehlt das Feld oder
// ist es leer, gibt es keine.
func docsHintsOf(node *yamllite.Node) ([]string, string) {
	if node == nil {
		return nil, ""
	}
	switch node.Kind {
	case yamllite.Scalar:
		if strings.TrimSpace(node.Value) == "" {
			return nil, ""
		}
		return nil, "hints ist keine Liste — erwartet hints: [ \"…\", … ] oder eine Liste mit - je Satz"
	case yamllite.Sequence:
		hints := []string{}
		for _, item := range node.Items {
			if item.Kind != yamllite.Scalar {
				return nil, "hints enthält etwas anderes als Sätze"
			}
			if text := strings.TrimSpace(item.Value); text != "" {
				hints = append(hints, text)
			}
		}
		if len(hints) == 0 {
			return nil, ""
		}
		return hints, ""
	}
	return nil, "hints ist keine Liste"
}

// docsHintsOfContent liest die hints eines Dokuments so gut es geht — für
// die Merkdatei beim checkout, wo ein kaputtes Frontmatter kein Grund ist,
// das Dokument nicht zu holen.
func docsHintsOfContent(content []byte) []string {
	block, ok := inventory.FrontmatterBlock(content)
	if !ok {
		return nil
	}
	root, err := yamllite.Parse([]byte(block))
	if err != nil {
		return nil
	}
	hints, _ := docsHintsOf(root.Get("hints"))
	return hints
}

// sameHints vergleicht zwei Listen von hints; nil und leer sind gleich.
func sameHints(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return slices.Equal(a, b)
}

// ensureProjectIgnored ergänzt eine Zeile in .k-playbook/.gitignore, ohne
// Vorhandenes anzufassen. header ist die Kommentarzeile, wenn die Datei neu
// entsteht.
func ensureProjectIgnored(projectRoot, line, header string) error {
	path := shareIgnorePath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s lesen: %w", path, err)
	}
	for existing := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(existing) == line {
			return nil
		}
	}
	content := string(data)
	if content == "" {
		content = header
	} else if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += line + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("%s schreiben: %w", path, err)
	}
	return nil
}

// DocsCheckoutOptions sind die Schalter von checkout. Discard verwirft ein
// vorhandenes Arbeitsverzeichnis samt Merkdatei und holt neu.
type DocsCheckoutOptions struct {
	Discard bool
}

// DocsCheckoutDoc ist ein geholtes Dokument.
type DocsCheckoutDoc struct {
	Path     string `json:"path"`
	Ref      string `json:"ref"`
	Revision int64  `json:"revision"`
}

// DocsExisting beschreibt ein Arbeitsverzeichnis, das beim checkout schon
// da ist: seit wann (aus der Merkdatei, ersatzweise der Änderungszeit), wie
// alt in Minuten und wie viele Dateien darin liegen.
type DocsExisting struct {
	Since      string `json:"since"`
	AgeMinutes int    `json:"ageMinutes"`
	Files      int    `json:"files"`
	Record     bool   `json:"record"`
}

// DocsCheckoutResult ist die Antwort von checkout. State ist checked_out
// (geholt) oder existing (ein Arbeitsverzeichnis lag schon da; nichts
// geholt, nichts überschrieben — weitermachen oder mit Discard verwerfen).
type DocsCheckoutResult struct {
	Producer  string            `json:"producer"`
	Ref       string            `json:"ref"`
	Dir       string            `json:"dir"`
	Record    string            `json:"record"`
	State     string            `json:"state"`
	Discarded bool              `json:"discarded,omitempty"`
	Docs      []DocsCheckoutDoc `json:"docs"`
	Existing  *DocsExisting     `json:"existing,omitempty"`
}

// Die Zustände der Antwort von checkout.
const (
	DocsStateCheckedOut = "checked_out"
	DocsStateExisting   = "existing"
)

// Checkout holt docs/generated/<erzeuger>/ (alle .md außer README.md,
// rekursiv) in das Arbeitsverzeichnis und schreibt die Merkdatei daneben,
// auch für ein leeres Verzeichnis. Liegt Arbeitsverzeichnis oder Merkdatei
// schon da, wird nichts geholt und nichts überschrieben, außer mit Discard:
// dann wird erst vollständig gelesen und danach das Vorhandene ersetzt —
// scheitert das Lesen, bleibt es liegen.
func (w *DocsWork) Checkout(ctx context.Context, name string, opts DocsCheckoutOptions) (DocsCheckoutResult, error) {
	producer, err := DocsProducerByName(name)
	if err != nil {
		return DocsCheckoutResult{}, err
	}
	dir, recordFile := DocsWorkDir(w.projectDir, producer.Name), DocsRecordFile(w.projectDir, producer.Name)
	result := DocsCheckoutResult{Producer: producer.Name, Ref: producer.Ref(), Dir: dir, Record: recordFile, Docs: []DocsCheckoutDoc{}}

	existing, err := w.existing(dir, recordFile)
	if err != nil {
		return DocsCheckoutResult{}, err
	}
	if existing != nil && !opts.Discard {
		result.State, result.Existing = DocsStateExisting, existing
		return result, nil
	}

	entries, err := w.store.List(ctx, kephalaion.ListRequest{Path: producer.dir(), Recursive: true})
	if err != nil {
		return DocsCheckoutResult{}, kephError(err)
	}
	type fetched struct {
		rel      string
		content  []byte
		revision int64
	}
	var docs []fetched
	for _, entry := range entries {
		rel, ok := strings.CutPrefix(entry.Name, producer.dir()+"/")
		if !ok || !strings.HasSuffix(rel, ".md") || strings.EqualFold(path.Base(rel), docsReadmeName) {
			continue
		}
		if problem := docsRelPath(rel); problem != "" {
			return DocsCheckoutResult{}, docRefErrorf(DocCodeInvalidInput, "%s in der Collection: %s", entry.Name, problem)
		}
		read, err := w.store.Read(ctx, entry.Name)
		if err != nil {
			return DocsCheckoutResult{}, kephError(err)
		}
		if read.Kind != kephalaion.ReadKindDocument || read.Content == nil {
			// Zwischen list und read verschwunden: nicht mitnehmen.
			continue
		}
		docs = append(docs, fetched{rel: rel, content: []byte(*read.Content), revision: read.Revision})
	}
	slices.SortFunc(docs, func(a, b fetched) int { return strings.Compare(a.rel, b.rel) })

	if existing != nil {
		if err := removeDocsWork(dir, recordFile); err != nil {
			return DocsCheckoutResult{}, err
		}
		result.Discarded = true
	}
	if err := ensureProjectIgnored(w.projectDir, docsIgnoreLine,
		"# Rechnerlokal: Arbeitsverzeichnisse von k-playbook (docs checkout).\n"); err != nil {
		return DocsCheckoutResult{}, err
	}

	record := docsRecord{Producer: producer.Name, Collection: w.address, CheckedOut: w.now().UTC().Format(time.RFC3339)}
	fail := func(err error) (DocsCheckoutResult, error) {
		_ = removeDocsWork(dir, recordFile)
		return DocsCheckoutResult{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fail(fmt.Errorf("%s anlegen: %w", dir, err))
	}
	for _, doc := range docs {
		file := filepath.Join(dir, filepath.FromSlash(doc.rel))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return fail(fmt.Errorf("%s anlegen: %w", filepath.Dir(file), err))
		}
		if err := os.WriteFile(file, doc.content, 0o644); err != nil {
			return fail(fmt.Errorf("%s schreiben: %w", file, err))
		}
		record.set(docsRecordItem{Path: doc.rel, Revision: doc.revision, Hash: docsHash(doc.content), Hints: docsHintsOfContent(doc.content)})
		result.Docs = append(result.Docs, DocsCheckoutDoc{Path: doc.rel, Ref: "keph:/" + producer.docName(doc.rel), Revision: doc.revision})
	}
	if err := writeDocsRecord(recordFile, record); err != nil {
		return fail(err)
	}
	result.State = DocsStateCheckedOut
	return result, nil
}

// existing beschreibt ein vorhandenes Arbeitsverzeichnis oder eine
// vorhandene Merkdatei; nil, wenn beides fehlt.
func (w *DocsWork) existing(dir, recordFile string) (*DocsExisting, error) {
	dirInfo, dirErr := os.Stat(dir)
	recordInfo, recordErr := os.Stat(recordFile)
	if os.IsNotExist(dirErr) && os.IsNotExist(recordErr) {
		return nil, nil
	}
	existing := &DocsExisting{Record: recordErr == nil}
	var since time.Time
	if record, ok, err := readDocsRecord(recordFile); err == nil && ok {
		since, _ = time.Parse(time.RFC3339, record.CheckedOut)
	}
	if since.IsZero() {
		switch {
		case recordErr == nil:
			since = recordInfo.ModTime()
		case dirErr == nil:
			since = dirInfo.ModTime()
		}
	}
	if !since.IsZero() {
		existing.Since = since.UTC().Format(time.RFC3339)
		existing.AgeMinutes = int(w.now().Sub(since).Minutes())
	}
	if dirErr == nil {
		_ = filepath.WalkDir(dir, func(_ string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				existing.Files++
			}
			return nil
		})
	}
	return existing, nil
}

// removeDocsWork entfernt Arbeitsverzeichnis und Merkdatei.
func removeDocsWork(dir, recordFile string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("%s entfernen: %w", dir, err)
	}
	if err := os.Remove(recordFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s entfernen: %w", recordFile, err)
	}
	return nil
}

// removeEmptyDocsWorkParents räumt nach einem aufgeräumten publish die
// Elternverzeichnisse des Arbeitsverzeichnisses ab — .k-playbook/work/docs/
// und .k-playbook/work/ —, wenn sie danach leer sind (Task 124). Nur der
// publish-Pfad ruft es, nicht removeDocsWork: checkout mit discard legt das
// Arbeitsverzeichnis gleich wieder an. Das Abräumen ist Ordnung, kein Teil
// des Abgleichs; ein Fehler dabei ändert das Ergebnis von publish nicht, und
// ein nicht leeres Verzeichnis — etwa das Arbeitsverzeichnis eines anderen
// Erzeugers — bleibt stehen.
func removeEmptyDocsWorkParents(dir string) {
	docsDir := filepath.Dir(dir)
	for _, parent := range []string{docsDir, filepath.Dir(docsDir)} {
		entries, err := os.ReadDir(parent)
		if err != nil || len(entries) != 0 {
			return
		}
		if err := os.Remove(parent); err != nil {
			return
		}
	}
}

// DocsPublishOptions sind die Schalter von publish. AllowHints erlaubt,
// hints zu ändern, ein Dokument mit hints zu löschen oder neu anzulegen —
// nur nach Bestätigung durch den Nutzer; die Antwort nennt jedes solche
// Dokument.
type DocsPublishOptions struct {
	AllowHints bool
}

// Die Zustände eines Dokuments in der Antwort von publish.
const (
	DocsCreated   = "created"
	DocsWritten   = "written"
	DocsDeleted   = "deleted"
	DocsUnchanged = "unchanged"
	// DocsConflict: inzwischen in Kephalaion geändert, angelegt oder gelöscht;
	// nichts geschrieben. Revision ist der aktuelle Stand, soweit lesbar.
	DocsConflict = "conflict"
	// DocsFailed: der Aufruf scheiterte; Code sagt warum. Bei
	// outcome_unknown kann er angekommen sein — nicht wiederholen, nachsehen.
	DocsFailed = "failed"
)

// DocsPublishItem ist ein Dokument in der Antwort von publish. Base ist die
// Revision beim checkout, Revision die nach dem Vorgang (bei conflict die
// aktuelle in Kephalaion).
type DocsPublishItem struct {
	Path     string `json:"path"`
	Ref      string `json:"ref"`
	Status   string `json:"status"`
	Revision int64  `json:"revision,omitempty"`
	Base     int64  `json:"base,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
}

// DocsPublishResult ist die Antwort von publish. HintsChanged nennt jedes
// Dokument, dessen hints mit AllowHints geändert, gelöscht oder neu
// angelegt wurden; Open die Dateien, die in Konflikt oder fehlgeschlagen
// sind. Cleaned sagt, ob Arbeitsverzeichnis und Merkdatei entfernt wurden.
type DocsPublishResult struct {
	Producer     string            `json:"producer"`
	Ref          string            `json:"ref"`
	Dir          string            `json:"dir"`
	Docs         []DocsPublishItem `json:"docs"`
	Readme       DocsPublishItem   `json:"readme"`
	HintsChanged []string          `json:"hintsChanged,omitempty"`
	Open         []string          `json:"open,omitempty"`
	Cleaned      bool              `json:"cleaned"`
}

// docsPlanned ist ein Vorgang, den publish ausführen wird.
type docsPlanned struct {
	rel     string
	action  string // DocsCreated, DocsWritten, DocsDeleted, DocsUnchanged
	content []byte
	hints   []string
	base    int64
}

// Publish gleicht das Arbeitsverzeichnis mit Kephalaion ab, gemessen an der
// Merkdatei: neu → create, geändert → write mit der Revision beim checkout,
// gelöscht → delete mit dieser Revision, unverändert → nichts. Vorher wird
// alles geprüft (nur .md, keine README.md, Frontmatter mit title und
// description, hints unverändert außer mit AllowHints); eine Ablehnung
// schreibt nichts. Danach schreibt publish die README des Verzeichnisses aus
// der Erzeugertabelle, wenn sie abweicht. Arbeitsverzeichnis und Merkdatei
// werden nur entfernt, wenn nichts in Konflikt oder fehlgeschlagen ist;
// sonst trägt die Merkdatei danach den Stand des Veröffentlichten, und ein
// zweites publish schreibt nur noch den Rest.
func (w *DocsWork) Publish(ctx context.Context, name string, opts DocsPublishOptions) (DocsPublishResult, error) {
	producer, err := DocsProducerByName(name)
	if err != nil {
		return DocsPublishResult{}, err
	}
	dir, recordFile := DocsWorkDir(w.projectDir, producer.Name), DocsRecordFile(w.projectDir, producer.Name)
	record, ok, err := readDocsRecord(recordFile)
	if err != nil {
		return DocsPublishResult{}, err
	}
	if !ok {
		return DocsPublishResult{}, docRefErrorf(DocCodeNotCheckedOut,
			"keine Merkdatei %s — zuerst k-playbook docs checkout %s", recordFile, producer.Name)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return DocsPublishResult{}, docRefErrorf(DocCodeNotCheckedOut,
			"Arbeitsverzeichnis %s fehlt, die Merkdatei liegt noch da — nichts veröffentlicht; mit k-playbook docs checkout %s --discard neu holen",
			dir, producer.Name)
	}
	if record.Producer != producer.Name || (record.Collection != "" && record.Collection != w.address) {
		return DocsPublishResult{}, docRefErrorf(DocCodeInvalidInput,
			"die Merkdatei %s gehört zu %s in %s, nicht zu %s in %s — mit checkout --discard neu holen",
			recordFile, record.Producer, record.Collection, producer.Name, w.address)
	}

	plan, hintsChanged, err := w.planPublish(producer, dir, record, opts)
	if err != nil {
		return DocsPublishResult{}, err
	}

	result := DocsPublishResult{Producer: producer.Name, Ref: producer.Ref(), Dir: dir, Docs: []DocsPublishItem{}, HintsChanged: hintsChanged}
	for _, step := range plan {
		item := w.publishStep(ctx, producer, step)
		switch item.Status {
		case DocsCreated, DocsWritten:
			record.set(docsRecordItem{Path: step.rel, Revision: item.Revision, Hash: docsHash(step.content), Hints: step.hints})
		case DocsDeleted:
			record.remove(step.rel)
		case DocsConflict, DocsFailed:
			result.Open = append(result.Open, step.rel)
		}
		result.Docs = append(result.Docs, item)
	}
	result.Readme = w.publishReadme(ctx, producer)

	if len(result.Open) == 0 && (result.Readme.Status == DocsCreated || result.Readme.Status == DocsWritten || result.Readme.Status == DocsUnchanged) {
		if err := removeDocsWork(dir, recordFile); err != nil {
			return result, err
		}
		result.Cleaned = true
		removeEmptyDocsWorkParents(dir)
		return result, nil
	}
	if err := writeDocsRecord(recordFile, record); err != nil {
		return result, err
	}
	return result, nil
}

// planPublish prüft das Arbeitsverzeichnis und legt fest, was mit jedem
// Dokument geschieht, sortiert nach Pfad. Ablehnungen kommen gesammelt als
// invalid_input, Änderungen an hints ohne AllowHints als hints_changed.
func (w *DocsWork) planPublish(producer DocsProducer, dir string, record docsRecord, opts DocsPublishOptions) ([]docsPlanned, []string, error) {
	var problems []string
	files := map[string][]byte{}
	walkErr := filepath.WalkDir(dir, func(file string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if file == dir {
			return nil
		}
		rel, err := filepath.Rel(dir, file)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			problems = append(problems, rel+": ein Link wird nicht veröffentlicht")
			return nil
		case entry.IsDir():
			if !docNamePattern.MatchString(entry.Name()) {
				problems = append(problems, rel+"/: Verzeichnisname folgt nicht der Namensregel von Kephalaion")
				return filepath.SkipDir
			}
			return nil
		case !entry.Type().IsRegular():
			problems = append(problems, rel+": keine gewöhnliche Datei")
			return nil
		}
		if problem := docsRelPath(rel); problem != "" {
			problems = append(problems, rel+": "+problem)
			return nil
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		files[rel] = content
		return nil
	})
	if walkErr != nil {
		return nil, nil, fmt.Errorf("Arbeitsverzeichnis %s lesen: %w", dir, walkErr)
	}

	var plan []docsPlanned
	var hintsChanged []string
	for rel, content := range files {
		meta, metaProblems := parseDocsMeta(content)
		for _, problem := range metaProblems {
			problems = append(problems, rel+": "+problem)
		}
		old, known := record.find(rel)
		step := docsPlanned{rel: rel, content: content, hints: meta.Hints, base: old.Revision}
		switch {
		case !known:
			step.action = DocsCreated
			if len(meta.Hints) > 0 {
				hintsChanged = append(hintsChanged, rel)
			}
		case old.Hash == docsHash(content):
			step.action = DocsUnchanged
		default:
			step.action = DocsWritten
			if !sameHints(old.Hints, meta.Hints) {
				hintsChanged = append(hintsChanged, rel)
			}
		}
		plan = append(plan, step)
	}
	for _, old := range record.Docs {
		if _, ok := files[old.Path]; ok {
			continue
		}
		plan = append(plan, docsPlanned{rel: old.Path, action: DocsDeleted, base: old.Revision, hints: old.Hints})
		if len(old.Hints) > 0 {
			hintsChanged = append(hintsChanged, old.Path)
		}
	}
	slices.SortFunc(plan, func(a, b docsPlanned) int { return strings.Compare(a.rel, b.rel) })
	slices.Sort(problems)
	slices.Sort(hintsChanged)

	if len(problems) > 0 {
		return nil, nil, docRefErrorf(DocCodeInvalidInput, "nichts veröffentlicht, %d Ablehnung(en) im Arbeitsverzeichnis %s:\n- %s",
			len(problems), dir, strings.Join(problems, "\n- "))
	}
	if len(hintsChanged) > 0 && !opts.AllowHints {
		return nil, nil, docRefErrorf(DocCodeHintsChanged,
			"nichts veröffentlicht: hints geändert, oder ein Dokument mit hints wird gelöscht oder neu angelegt — %s. "+
				"hints ändern sich nur nach Bestätigung durch den Nutzer; dann mit allow_hints (--allow-hints) veröffentlichen",
			strings.Join(hintsChanged, ", "))
	}
	return plan, hintsChanged, nil
}

// publishStep führt einen Vorgang aus und macht aus dem Ergebnis einen
// Eintrag der Antwort.
func (w *DocsWork) publishStep(ctx context.Context, producer DocsProducer, step docsPlanned) DocsPublishItem {
	name := producer.docName(step.rel)
	item := DocsPublishItem{Path: step.rel, Ref: "keph:/" + name, Status: step.action, Base: step.base}
	var doc kephalaion.Document
	var err error
	switch step.action {
	case DocsUnchanged:
		item.Revision = step.base
		return item
	case DocsCreated:
		doc, err = w.store.Create(ctx, name, string(step.content))
	case DocsWritten:
		doc, err = w.store.Write(ctx, name, string(step.content), step.base)
	case DocsDeleted:
		doc, err = w.store.Delete(ctx, name, step.base)
	}
	if err == nil {
		item.Revision = doc.Revision
		return item
	}
	mapped := kephError(err)
	item.Code, item.Message = DocRefErrorCode(mapped), mapped.Error()
	switch item.Code {
	case DocCodeStaleRevision, DocCodeNameTaken, DocCodeNotFound:
		item.Status = DocsConflict
		item.Message = map[string]string{
			DocCodeStaleRevision: "inzwischen in Kephalaion geändert; nichts geschrieben",
			DocCodeNameTaken:     "inzwischen in Kephalaion angelegt; nichts geschrieben",
			DocCodeNotFound:      "inzwischen in Kephalaion gelöscht; nichts geschrieben",
		}[item.Code]
		if read, err := w.store.Read(ctx, name); err == nil && read.Kind == kephalaion.ReadKindDocument {
			item.Revision = read.Revision
		}
	default:
		item.Status = DocsFailed
	}
	return item
}

// publishReadme schreibt die README des Verzeichnisses aus der
// Erzeugertabelle, nur wenn sie inhaltlich abweicht.
func (w *DocsWork) publishReadme(ctx context.Context, producer DocsProducer) DocsPublishItem {
	name := producer.docName(docsReadmeName)
	item := DocsPublishItem{Path: docsReadmeName, Ref: "keph:/" + name}
	fail := func(err error) DocsPublishItem {
		mapped := kephError(err)
		item.Status, item.Code, item.Message = DocsFailed, DocRefErrorCode(mapped), mapped.Error()
		return item
	}
	want := producer.readme()
	read, err := w.store.Read(ctx, name)
	if err != nil {
		return fail(err)
	}
	var doc kephalaion.Document
	switch read.Kind {
	case kephalaion.ReadKindNone:
		item.Status = DocsCreated
		doc, err = w.store.Create(ctx, name, want)
	case kephalaion.ReadKindDocument:
		item.Base = read.Revision
		if read.Content != nil && *read.Content == want {
			item.Status, item.Revision = DocsUnchanged, read.Revision
			return item
		}
		item.Status = DocsWritten
		doc, err = w.store.Write(ctx, name, want, read.Revision)
	default:
		item.Status, item.Code, item.Message = DocsFailed, DocCodeInvalidInput, name+" ist ein Verzeichnis"
		return item
	}
	if err != nil {
		return fail(err)
	}
	item.Revision = doc.Revision
	return item
}
