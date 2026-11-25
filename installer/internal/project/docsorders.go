package project

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Die Doku-Aufträge in keph:/queue/docs/ (Task 179): lesen und bewerten,
// einreihen und zerlegte Aufträge schreiben — als Rechnung im Programm, die
// die Docs-Commands bisher im Text erledigten. Die Regeln stehen in
// k-playbook:/instructions/on-demand/doku-auftraege.md und in Schritt 1 und 4
// von k-playbook:/commands/k-docs-code.md. Das Programm entscheidet nur, was
// sich rechnen lässt: Die Zuordnung Pfad → Dokument, neu/entfällt und jede
// Nutzerentscheidung bleiben bei der Sitzung.

// Die Erzeuger der Aufträge: code (keph:/docs/generated/code/, /k-docs-code)
// und tools (keph:/docs/generated/libs/, /k-docs-tools).
const (
	DocsOrderCode  = "code"
	DocsOrderTools = "tools"
)

// DocsOrderProducers sind die Erzeuger, für die es Aufträge gibt.
var DocsOrderProducers = []string{DocsOrderCode, DocsOrderTools}

// docsOrderGeneratedDir ist das Verzeichnis der erzeugten Doku eines
// Erzeugers, ab der Wurzel der Collection.
func docsOrderGeneratedDir(producer string) string {
	if producer == DocsOrderTools {
		return docsGeneratedRoot + "/libs"
	}
	return docsGeneratedRoot + "/" + producer
}

// DocsOrderWait ist die Wartezeit eines rohen Auftrags für code: Der
// Verarbeiter nimmt ihn erst, wenn seine letzte Änderung — ohne lesbare
// Änderung das Anlegen (docsRawOrderAge) — so lange zurückliegt. Der Wert
// steht nur hier (Task 179); gerechnet wird mit der Uhr des Programms, nicht
// mit dem Zeitstempel des Briefings.
const DocsOrderWait = 2 * time.Hour

// Die Zustände eines Laufs von /k-docs-code.
const (
	// DocsRunFull: ein roher Auftrag mit pfade: . — alle offenen rohen
	// Aufträge gehen darin auf, auch die wartenden.
	DocsRunFull = "full"
	// DocsRunPartial: nur reife rohe Aufträge.
	DocsRunPartial = "partial"
	// DocsRunResume: kein roher Auftrag, aber offene zerlegte — ein früherer
	// Lauf ist unterbrochen.
	DocsRunResume = "resume"
	// DocsRunNone: nichts zu tun.
	DocsRunNone = "none"
)

// DocsOrderKindRaw und DocsOrderKindSplit sind die Reifegrade.
const (
	DocsOrderKindRaw   = "raw"
	DocsOrderKindSplit = "split"
)

// Die Aktionen eines zerlegten Auftrags.
const (
	DocsActionNew      = "neu"
	DocsActionRewrite  = "umschreiben"
	DocsActionObsolete = "entfaellt"
)

// docsOrdersDir ist die Ablage der Aufträge, ab der Wurzel der Collection.
const docsOrdersDir = "queue/docs"

// DocsOrderLine ist ein Anlass mit Datum: die Zeile des Rumpfs eines Auftrags
// oder sein Kopf.
type DocsOrderLine struct {
	Datum  string `json:"datum,omitempty"`
	Anlass string `json:"anlass"`
}

// DocsOrder ist ein offener Auftrag. Kind ist raw oder split; Waiting und
// RemainingMinutes gelten nur für rohe Aufträge von code.
type DocsOrder struct {
	Ref              string          `json:"ref"`
	Kind             string          `json:"kind"`
	Title            string          `json:"title"`
	Producer         string          `json:"erzeuger"`
	Dokument         string          `json:"dokument,omitempty"`
	Aktion           string          `json:"aktion,omitempty"`
	Pfade            []string        `json:"pfade"`
	Anlass           string          `json:"anlass"`
	Sitzung          string          `json:"sitzung,omitempty"`
	Entstanden       string          `json:"entstanden,omitempty"`
	Created          string          `json:"created,omitempty"`
	Updated          string          `json:"updated,omitempty"`
	Revision         int64           `json:"revision"`
	Waiting          bool            `json:"waiting,omitempty"`
	RemainingMinutes int             `json:"remainingMinutes,omitempty"`
	Lines            []DocsOrderLine `json:"lines,omitempty"`
	// Incomplete: der Auftrag trägt nach dem Lesen keinen gültigen Pfad
	// (Task 193). Er bleibt in der Liste und steht unter problems.
	Incomplete bool `json:"incomplete,omitempty"`
	// notes sind die Zeilen für problems, die beim Lesen zu diesem Auftrag
	// entstanden — weggelassene und berichtigte Pfade. Orders schreibt sie
	// erst nach dem Filtern nach Erzeuger, damit sie nur beim gefragten
	// stehen.
	notes []string
}

// DocsRawOrder ist, was aus rohen Aufträgen nach dem Zusammenlegen wird
// (RAW_ORDERS): abhängige gelten als einer mit vereinigten Pfaden und
// gesammelten Anlässen; Refs nennt die Aufträge, die darin aufgegangen sind.
type DocsRawOrder struct {
	Refs       []string        `json:"refs"`
	Pfade      []string        `json:"pfade"`
	Anlaesse   []DocsOrderLine `json:"anlaesse"`
	Entstanden string          `json:"entstanden"`
}

// DocsOrderWaiting ist ein wartender Auftrag mit seiner Restzeit
// (WAITING).
type DocsOrderWaiting struct {
	Ref              string   `json:"ref"`
	Pfade            []string `json:"pfade"`
	Updated          string   `json:"updated"`
	RemainingMinutes int      `json:"remainingMinutes"`
}

// DocsOrdersResult ist die Antwort von Orders. Für tools stehen nur Orders
// und NothingToDo da: Eine Wartezeit gibt es dort nicht, und der Lauf ist
// immer ganz.
type DocsOrdersResult struct {
	Producer    string             `json:"producer"`
	WaitMinutes int                `json:"waitMinutes,omitempty"`
	Run         string             `json:"run,omitempty"`
	Orders      []DocsOrder        `json:"orders"`
	RawOrders   []DocsRawOrder     `json:"rawOrders,omitempty"`
	SplitOrders []DocsOrder        `json:"splitOrders,omitempty"`
	Waiting     []DocsOrderWaiting `json:"waiting,omitempty"`
	NothingToDo bool               `json:"nothingToDo"`
	Reason      string             `json:"reason,omitempty"`
	Problems    []string           `json:"problems,omitempty"`
}

// DocsOrders ist der Zugang zu den Aufträgen: lesen, einreihen, zerlegte
// schreiben. Alles geht über die Collection des Projekts, ohne Rückfall.
type DocsOrders struct {
	docs *UserDocs
	// repoRoot ist project.repoRoot (RepoRootDir), an dem Pfade geprüft und
	// berichtigt werden. Leer — etwa in Tests ohne Projekt — entfallen
	// Prüfung und Berichtigung.
	repoRoot string
}

// NewDocsOrders baut den Zugang über einen Store; user ist kephalaion.user.
func NewDocsOrders(store DocStore, user string) (*DocsOrders, error) {
	docs, err := NewUserDocs(DocKinds, store, user)
	if err != nil {
		return nil, err
	}
	return &DocsOrders{docs: docs}, nil
}

// OpenDocsOrders prüft die Anbindung des Projekts und baut den Zugang, Fehler
// mit DocCodeKephalaionUnavailable. Austauschbar, damit Subkommando und
// MCP-Tests den Node nicht fragen.
var OpenDocsOrders = func(projectDir string) (*DocsOrders, error) {
	client, user, err := openKephClient(projectDir)
	if err != nil {
		return nil, err
	}
	orders, err := NewDocsOrders(client, user)
	if err != nil {
		return nil, err
	}
	if config, err := ReadConfig(projectDir); err == nil {
		orders.repoRoot = RepoRootDir(projectDir, config)
	}
	return orders, nil
}

// ValidDocsOrderProducer prüft den Erzeuger eines Auftrags.
func ValidDocsOrderProducer(name string) error {
	for _, producer := range DocsOrderProducers {
		if producer == name {
			return nil
		}
	}
	return docRefErrorf(DocCodeInvalidInput, "unbekannter Erzeuger %q — erlaubt sind %s", name, strings.Join(DocsOrderProducers, ", "))
}

var docsOrderLinePattern = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}) — (.+)$`)

// cleanDocsOrderPath normiert einen Pfad eines Auftrags: relativ zu
// project.repoRoot, mit Schrägstrichen, ohne ./ vorn und / hinten; . ist das
// ganze Projekt. Ein leerer, absoluter oder aus dem Projekt führender Pfad
// ist ungültig.
func cleanDocsOrderPath(raw string) (string, error) {
	text := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if text == "" {
		return "", docRefErrorf(DocCodeInvalidInput, "ein Pfad ist leer")
	}
	if strings.HasPrefix(text, "/") || strings.ContainsAny(text, "\r\n") {
		return "", docRefErrorf(DocCodeInvalidInput, "%q: erwartet ein Pfad relativ zu project.repoRoot", raw)
	}
	cleaned := path.Clean(text)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", docRefErrorf(DocCodeInvalidInput, "%q führt aus project.repoRoot heraus", raw)
	}
	return cleaned, nil
}

// cleanDocsOrderPaths normiert eine Liste, ohne Doppelte; enthält sie ., steht
// nur . da.
func cleanDocsOrderPaths(raw []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, entry := range raw {
		cleaned, err := cleanDocsOrderPath(entry)
		if err != nil {
			return nil, err
		}
		if cleaned == "." {
			return []string{"."}, nil
		}
		if !seen[cleaned] {
			seen[cleaned] = true
			out = append(out, cleaned)
		}
	}
	return out, nil
}

// DocsPathCorrection ist eine Berichtigung: From der Pfad, wie er kam, To
// der berichtigte.
type DocsPathCorrection struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// docsPathExists sagt, ob es einen Pfad (normiert, relativ) unter repoRoot
// gibt; . gibt es immer. Geprüft wird am Dateisystem, nicht an git.
func docsPathExists(repoRoot, p string) bool {
	if p == "." {
		return true
	}
	_, err := os.Lstat(filepath.Join(repoRoot, filepath.FromSlash(p)))
	return err == nil
}

// docsRepoPrefixFix berichtigt einen Pfad (normiert, relativ), dem das
// Repo-Verzeichnis ein zweites Mal vorangestellt ist: Den Pfad gibt es unter
// repoRoot nicht, sein erstes Segment ist der Basisname von repoRoot, und den
// Rest (nicht leer) gibt es dort. Nur dieser eindeutige Fall wird berichtigt
// — gibt es beide Schreibweisen oder keine, bleibt der Pfad. Ohne repoRoot
// gibt es nichts zu berichtigen.
func docsRepoPrefixFix(repoRoot, p string) (string, bool) {
	if repoRoot == "" || p == "." {
		return "", false
	}
	first, rest, ok := strings.Cut(p, "/")
	if !ok || rest == "" || first != filepath.Base(repoRoot) {
		return "", false
	}
	if docsPathExists(repoRoot, p) || !docsPathExists(repoRoot, rest) {
		return "", false
	}
	return rest, true
}

// correctPaths berichtigt eine normierte Pfadliste nach docsRepoPrefixFix,
// ohne Doppelte danach, und nennt jede Berichtigung.
func (o *DocsOrders) correctPaths(paths []string) ([]string, []DocsPathCorrection) {
	if o.repoRoot == "" {
		return paths, nil
	}
	out := make([]string, 0, len(paths))
	var corrected []DocsPathCorrection
	for _, p := range paths {
		if fixed, ok := docsRepoPrefixFix(o.repoRoot, p); ok {
			corrected = append(corrected, DocsPathCorrection{From: p, To: fixed})
			p = fixed
		}
		if !containsString(out, p) {
			out = append(out, p)
		}
	}
	return out, corrected
}

// missingPaths nennt die Pfade, die es unter project.repoRoot nicht gibt;
// ohne repoRoot keine.
func (o *DocsOrders) missingPaths(paths []string) []string {
	if o.repoRoot == "" {
		return nil
	}
	var missing []string
	for _, p := range paths {
		if !docsPathExists(o.repoRoot, p) {
			missing = append(missing, p)
		}
	}
	return missing
}

// docsPathContains sagt, ob der Pfad a gleich b ist oder b darin liegt; . ist
// alles.
func docsPathContains(a, b string) bool {
	return a == "." || a == b || strings.HasPrefix(b, a+"/")
}

// docsPathsDepend sagt, ob zwei Pfadlisten abhängig sind: Ein Pfad der einen
// ist gleich einem Pfad der anderen oder liegt darin.
func docsPathsDepend(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if docsPathContains(x, y) || docsPathContains(y, x) {
				return true
			}
		}
	}
	return false
}

// docsPathsCovered sagt, ob jeder Pfad aus add gleich einem Pfad aus have ist
// oder darin liegt.
func docsPathsCovered(have, add []string) bool {
	for _, p := range add {
		covered := false
		for _, h := range have {
			if docsPathContains(h, p) {
				covered = true
				break
			}
		}
		if !covered {
			return false
		}
	}
	return true
}

// parseDocsOrder liest einen Auftrag aus seinem Dokument. Ein Frontmatter,
// das sich nicht lesen lässt, gibt problem zurück. dropped nennt die Pfade,
// die als ungültig weggelassen wurden, je mit Grund.
func parseDocsOrder(ref string, entry kephalaion.Entry, read kephalaion.ReadResult) (order DocsOrder, dropped []string, problem string) {
	order = DocsOrder{Ref: ref, Pfade: []string{}, Revision: read.Revision}
	order.Created = docsOrderStamp(entry.Created, read.Created)
	order.Updated = docsOrderStamp(entry.Updated, read.Updated)
	content := ""
	if read.Content != nil {
		content = *read.Content
	}
	block, ok := inventory.FrontmatterBlock([]byte(content))
	if !ok {
		return order, nil, "Frontmatter fehlt"
	}
	root, err := yamllite.Parse([]byte(block))
	if err != nil {
		return order, nil, fmt.Sprintf("Frontmatter nicht lesbar: %v", err)
	}
	order.Title = strings.TrimSpace(root.Get("title").Str())
	order.Producer = strings.TrimSpace(root.Get("erzeuger").Str())
	order.Dokument = strings.TrimSpace(root.Get("dokument").Str())
	order.Aktion = strings.TrimSpace(root.Get("aktion").Str())
	order.Anlass = strings.TrimSpace(root.Get("anlass").Str())
	order.Sitzung = strings.TrimSpace(root.Get("sitzung").Str())
	order.Entstanden = strings.TrimSpace(root.Get("entstanden").Str())
	order.Kind = DocsOrderKindRaw
	if order.Dokument != "" {
		order.Kind = DocsOrderKindSplit
	}
	takePath := func(value string) {
		if value = strings.TrimSpace(value); value == "" {
			return
		}
		cleaned, err := cleanDocsOrderPath(value)
		if err != nil {
			dropped = append(dropped, "Pfad weggelassen — "+err.Error())
			return
		}
		order.Pfade = append(order.Pfade, cleaned)
	}
	switch node := root.Get("pfade"); {
	case node == nil:
	case node.Kind == yamllite.Sequence:
		for _, item := range node.Items {
			takePath(item.Value)
		}
	case node.Kind == yamllite.Scalar:
		takePath(node.Value)
	}
	for _, line := range strings.Split(string(inventory.Body([]byte(content))), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := docsOrderLinePattern.FindStringSubmatch(line); m != nil {
			order.Lines = append(order.Lines, DocsOrderLine{Datum: m[1], Anlass: m[2]})
			continue
		}
		order.Lines = append(order.Lines, DocsOrderLine{Anlass: line})
	}
	return order, dropped, ""
}

// load liest alle offenen Aufträge (ohne done/) mit Inhalt, sortiert nach
// Verweis. Problems nennt die Aufträge, deren Frontmatter sich nicht lesen
// ließ; sie fehlen in der Liste. Die Pfade jedes Auftrags sind berichtigt
// (correctPaths), damit Orders, Add und Split dieselben sehen; der Auftrag in
// der Queue bleibt dafür, wie er ist. Was zu einem gelesenen Auftrag zu sagen
// ist — weggelassene und berichtigte Pfade —, steht in seinen notes.
func (o *DocsOrders) load(ctx context.Context) ([]DocsOrder, []string, error) {
	kind, ok := o.docs.anyKind("docs")
	if !ok {
		return nil, nil, docRefErrorf(DocCodeInvalidInput, "die Sorte docs fehlt in der Sorten-Tabelle")
	}
	entries, matches, err := o.docs.listKind(ctx, kind, o.docs.user, false)
	if err != nil {
		return nil, nil, err
	}
	orders := []DocsOrder{}
	problems := []string{}
	for i, entry := range entries {
		if matches[i].done {
			continue
		}
		ref := DocSchemeKeph + ":/" + entry.Name
		read, err := o.docs.store.Read(ctx, entry.Name)
		if err != nil {
			return nil, nil, kephError(err)
		}
		if read.Kind != kephalaion.ReadKindDocument {
			continue
		}
		order, dropped, problem := parseDocsOrder(ref, entry, read)
		if problem != "" {
			problems = append(problems, ref+": "+problem)
			continue
		}
		for _, note := range dropped {
			order.notes = append(order.notes, ref+": "+note)
		}
		var corrected []DocsPathCorrection
		order.Pfade, corrected = o.correctPaths(order.Pfade)
		for _, c := range corrected {
			order.notes = append(order.notes, fmt.Sprintf("%s: Pfad %q berichtigt zu %q — Repo-Verzeichnis doppelt vorangestellt", ref, c.From, c.To))
		}
		orders = append(orders, order)
	}
	sort.SliceStable(orders, func(i, j int) bool { return orders[i].Ref < orders[j].Ref })
	return orders, problems, nil
}

// docsOrderStamp ist der erste nicht leere Zeitpunkt aus list und read.
func docsOrderStamp(stamps ...*kephalaion.Stamp) string {
	for _, stamp := range stamps {
		if stamp != nil && strings.TrimSpace(stamp.At) != "" {
			return stamp.At
		}
	}
	return ""
}

// docsRawOrderAge ist das Alter eines rohen Auftrags, an dem die Wartezeit
// (DocsOrderWait) gemessen wird: Maßgeblich ist die letzte Änderung; fehlt
// sie oder ist sie nicht lesbar, der Anlegezeitpunkt. ok ist false, wenn
// keiner von beiden vorliegt — „kein Zeitpunkt“. Orders und der Wartewächter
// von Split rechnen beide hiermit, damit ein Auftrag nicht hier als wartend
// gelistet und dort als reif zerlegt wird. entstanden im Kopf zählt nicht.
func docsRawOrderAge(order DocsOrder, at time.Time) (time.Duration, bool) {
	if age, ok := docsOrderAge(order.Updated, at); ok {
		return age, true
	}
	return docsOrderAge(order.Created, at)
}

// docsOrderAge rechnet, wie lange ein Zeitpunkt zurückliegt. ok ist false,
// wenn kein Zeitpunkt vorliegt oder er sich nicht lesen lässt.
func docsOrderAge(stamp string, at time.Time) (time.Duration, bool) {
	if strings.TrimSpace(stamp) == "" {
		return 0, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return 0, false
	}
	return at.Sub(parsed), true
}

// Orders liest die offenen Aufträge eines Erzeugers und bewertet sie. Für
// code: roh oder zerlegt, wartend oder reif, der Lauf (voll, Teil,
// Wiederaufnahme), die zusammengelegten rohen Aufträge (RawOrders), die
// offenen zerlegten (SplitOrders) und die wartenden (Waiting). Für tools nur
// die Liste.
func (o *DocsOrders) Orders(ctx context.Context, producer string) (DocsOrdersResult, error) {
	if err := ValidDocsOrderProducer(producer); err != nil {
		return DocsOrdersResult{}, err
	}
	all, problems, err := o.load(ctx)
	if err != nil {
		return DocsOrdersResult{}, err
	}
	result := DocsOrdersResult{Producer: producer, Orders: []DocsOrder{}, Problems: problems}
	at := now()
	for _, order := range all {
		if order.Producer != producer {
			continue
		}
		result.Problems = append(result.Problems, order.notes...)
		if len(order.Pfade) == 0 {
			order.Incomplete = true
			result.Problems = append(result.Problems, order.Ref+": ohne gültigen Pfad — unvollständig")
		}
		if producer == DocsOrderCode && order.Kind == DocsOrderKindRaw {
			// Ohne lesbaren Zeitpunkt (docsRawOrderAge) ist der Auftrag reif —
			// er darf nie dauerhaft warten — und steht unter problems, damit
			// der Rückfall sichtbar ist.
			age, ok := docsRawOrderAge(order, at)
			switch {
			case !ok:
				result.Problems = append(result.Problems, order.Ref+
					": ohne lesbaren Zeitpunkt der Änderung und des Anlegens — gilt als reif, wird abgearbeitet")
			case age < DocsOrderWait:
				order.Waiting = true
				order.RemainingMinutes = int((DocsOrderWait - age + time.Minute - 1) / time.Minute)
			}
		}
		result.Orders = append(result.Orders, order)
	}

	if producer == DocsOrderTools {
		result.NothingToDo = len(result.Orders) == 0
		if result.NothingToDo {
			result.Reason = "keine offenen Aufträge für tools"
		}
		return result, nil
	}

	result.WaitMinutes = int(DocsOrderWait / time.Minute)
	var raw, ready []DocsOrder
	for _, order := range result.Orders {
		if order.Kind == DocsOrderKindSplit {
			result.SplitOrders = append(result.SplitOrders, order)
			continue
		}
		raw = append(raw, order)
		if !order.Waiting {
			ready = append(ready, order)
		}
	}
	full := false
	for _, order := range raw {
		if len(order.Pfade) == 1 && order.Pfade[0] == "." {
			full = true
			break
		}
	}
	taken := ready
	switch {
	case full:
		result.Run = DocsRunFull
		taken = raw
	case len(ready) > 0:
		result.Run = DocsRunPartial
	case len(result.SplitOrders) > 0:
		result.Run = DocsRunResume
	default:
		result.Run = DocsRunNone
	}
	for _, order := range raw {
		if order.Waiting && !full {
			result.Waiting = append(result.Waiting, DocsOrderWaiting{
				Ref: order.Ref, Pfade: order.Pfade, Updated: order.Updated, RemainingMinutes: order.RemainingMinutes})
		}
	}
	result.RawOrders = mergeDocsRawOrders(taken)
	if result.Run == DocsRunNone {
		result.NothingToDo = true
		switch {
		case len(result.Waiting) > 0:
			result.Reason = "nur wartende Aufträge"
		default:
			result.Reason = "keine offenen Aufträge für code"
		}
	}
	return result, nil
}

// mergeDocsRawOrders legt abhängige rohe Aufträge zusammen — auch über
// mehrere hinweg — zu einem mit vereinigten Pfaden, gesammelten Anlässen und
// dem ältesten Datum als entstanden. Die Reihenfolge folgt dem ersten
// Auftrag jeder Gruppe.
func mergeDocsRawOrders(orders []DocsOrder) []DocsRawOrder {
	parent := make([]int, len(orders))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for i := range orders {
		for j := i + 1; j < len(orders); j++ {
			if docsPathsDepend(orders[i].Pfade, orders[j].Pfade) {
				if a, b := find(i), find(j); a != b {
					parent[b] = a
				}
			}
		}
	}
	groups := map[int]*DocsRawOrder{}
	var rootsInOrder []int
	for i, order := range orders {
		root := find(i)
		group, ok := groups[root]
		if !ok {
			group = &DocsRawOrder{Refs: []string{}, Pfade: []string{}, Anlaesse: []DocsOrderLine{}}
			groups[root] = group
			rootsInOrder = append(rootsInOrder, root)
		}
		group.Refs = append(group.Refs, order.Ref)
		for _, p := range order.Pfade {
			if !containsString(group.Pfade, p) {
				group.Pfade = append(group.Pfade, p)
			}
		}
		lines := []DocsOrderLine{}
		if order.Anlass != "" {
			lines = append(lines, DocsOrderLine{Datum: order.Entstanden, Anlass: order.Anlass})
		}
		lines = append(lines, order.Lines...)
		for _, line := range lines {
			if !containsDocsLine(group.Anlaesse, line) {
				group.Anlaesse = append(group.Anlaesse, line)
			}
		}
		if order.Entstanden != "" && (group.Entstanden == "" || order.Entstanden < group.Entstanden) {
			group.Entstanden = order.Entstanden
		}
	}
	merged := make([]DocsRawOrder, 0, len(rootsInOrder))
	for _, root := range rootsInOrder {
		group := groups[root]
		if containsString(group.Pfade, ".") {
			group.Pfade = []string{"."}
		}
		merged = append(merged, *group)
	}
	return merged
}

func containsDocsLine(lines []DocsOrderLine, line DocsOrderLine) bool {
	for _, have := range lines {
		if have == line {
			return true
		}
	}
	return false
}
