package project

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/buildinfo"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Die Meldungen des Projekts (Task 198): Fehler und Wünsche, die jemand
// anderes erledigen soll — an k-playbook, an Kephalaion oder am Projekt
// selbst. Jede Meldung ist ein Dokument feedback/<NNN>-<slug>.md in der
// Collection des Projekts, projektweit, nicht im Userbereich; erledigte und
// abgelehnte liegen unter feedback/done/. Eine Meldung ist kein Befund: Sie
// wartet nicht in der Queue auf die Einordnung, sondern bleibt liegen, bis
// jemand sie erledigt.
//
// Das Format gehört Go wie bei den Todos: gelesen und geschrieben wird es
// allein hier, über MCP (k_playbook_feedback_*), das Subkommando
// `k-playbook feedback` und /k-feedback. Den Kopf baut das Programm: Die
// Sitzung liefert Titel, Art, Dringlichkeit, Betreff und Text; user und
// account kommen vom Node, version ist die eigene, status beginnt bei offen,
// von, client, modell und sitzung sind ausdrückliche Parameter — nie freier
// Text im Rumpf.

// FeedbackKindName ist die Sorte der Meldungen in der Sorten-Tabelle DocKinds.
const FeedbackKindName = "feedback"

// Die Werte der Aufzählungsfelder.
var (
	FeedbackArten           = []string{"fehler", "wunsch"}
	FeedbackDringlichkeiten = []string{"hoch", "mittel", "niedrig"}
	FeedbackBetreffe        = []string{"k-playbook", "kephalaion", "projekt"}
	FeedbackStatus          = []string{"offen", "angenommen", "erledigt", "abgelehnt"}
	FeedbackVon             = []string{"ki", "mensch"}
)

// Status und Herkunft, die der Code selbst unterscheidet.
const (
	FeedbackStatusOffen     = "offen"
	FeedbackStatusErledigt  = "erledigt"
	FeedbackStatusAbgelehnt = "abgelehnt"
	FeedbackVonKI           = "ki"
	FeedbackVonMensch       = "mensch"
)

// feedbackShortNameWords und feedbackMaxShortName begrenzen den Kurznamen im
// Dateinamen, der aus dem Titel entsteht.
const (
	feedbackShortNameWords = 6
	feedbackMaxShortName   = 40
)

// feedbackFieldOrder sind die Felder des Kopfs in der Reihenfolge, in der sie
// geschrieben werden.
var feedbackFieldOrder = []string{"title", "art", "dringlichkeit", "betrifft", "status", "von", "user", "account",
	"client", "modell", "sitzung", "version"}

// feedbackQuoted sind die Felder mit freiem Text: Sie stehen immer in
// doppelten Anführungszeichen (Regel aus tasks.md, „Kopf“). Die
// Aufzählungsfelder bleiben ohne.
var feedbackQuoted = map[string]bool{"title": true, "user": true, "account": true, "client": true, "modell": true,
	"sitzung": true, "version": true}

// Feedback ist eine Meldung, wie alle Zugriffswege sie ausgeben.
type Feedback struct {
	// Ref ist der keph:-Verweis, Name der Dateiname, Number die Nummer darin.
	Ref    string `json:"ref"`
	Name   string `json:"name"`
	Number int    `json:"number"`
	// Done sagt, ob die Meldung unter feedback/done/ liegt.
	Done     bool  `json:"done"`
	Revision int64 `json:"revision,omitempty"`

	Title         string `json:"title"`
	Art           string `json:"art"`
	Dringlichkeit string `json:"dringlichkeit"`
	Betrifft      string `json:"betrifft"`
	Status        string `json:"status"`
	Von           string `json:"von"`
	User          string `json:"user,omitempty"`
	Account       string `json:"account,omitempty"`
	Client        string `json:"client,omitempty"`
	Modell        string `json:"modell,omitempty"`
	Sitzung       string `json:"sitzung,omitempty"`
	Version       string `json:"version,omitempty"`

	// Created und Updated sind die Stempel von Kephalaion (Datum und Zeit).
	Created string `json:"created,omitempty"`
	Updated string `json:"updated,omitempty"`
	// Body ist der Text der Meldung, ohne Kopf und ohne Überschrift.
	Body string `json:"body,omitempty"`
	// Problem nennt, was am Kopf nicht stimmt; gemeldet, nicht repariert.
	Problem string `json:"problem,omitempty"`
}

// feedbackDoc ist eine Meldung, zerlegt: die Felder des Kopfs, der Text und
// die Zeilen fremder Felder, die beim Schreiben nicht verloren gehen.
type feedbackDoc struct {
	fields map[string]string
	body   string
	extra  []string
	// problem nennt, was beim Lesen nicht stimmte.
	problem string
}

// feedbackHeading erkennt die Überschrift, die composeFeedbackDoc setzt.
var feedbackHeading = regexp.MustCompile(`^# Meldung [0-9]+ — `)

// feedbackKeyLine erkennt eine Zeile, die ein Feld der obersten Ebene beginnt.
var feedbackKeyLine = regexp.MustCompile(`^([A-Za-z0-9_][A-Za-z0-9_.-]*)\s*:`)

// checkFeedbackEnum prüft einen Wert gegen seine Aufzählung.
func checkFeedbackEnum(field, value string, allowed []string) error {
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	if value == "" {
		return docRefErrorf(DocCodeInvalidInput, "%s fehlt; erlaubt sind %s", field, strings.Join(allowed, ", "))
	}
	return docRefErrorf(DocCodeInvalidInput, "%s %q ist nicht erlaubt; erlaubt sind %s", field, value, strings.Join(allowed, ", "))
}

// checkFeedbackFields prüft einen Kopf vor dem Schreiben: Titel, die
// Aufzählungen und die Regel, dass client und modell nur bei von: ki stehen.
// Mehrere Verstöße kommen gesammelt.
func checkFeedbackFields(f Feedback) error {
	var problems []string
	add := func(err error) {
		if err != nil {
			problems = append(problems, err.Error())
		}
	}
	if strings.TrimSpace(f.Title) == "" {
		problems = append(problems, "title fehlt")
	} else if strings.ContainsAny(f.Title, "\r\n") {
		problems = append(problems, "title enthält einen Zeilenumbruch")
	}
	add(checkFeedbackEnum("art", f.Art, FeedbackArten))
	add(checkFeedbackEnum("dringlichkeit", f.Dringlichkeit, FeedbackDringlichkeiten))
	add(checkFeedbackEnum("betrifft", f.Betrifft, FeedbackBetreffe))
	add(checkFeedbackEnum("status", f.Status, FeedbackStatus))
	add(checkFeedbackEnum("von", f.Von, FeedbackVon))
	if f.Von == FeedbackVonMensch && (f.Client != "" || f.Modell != "") {
		problems = append(problems, "client und modell stehen nur bei von: ki")
	}
	for field, value := range map[string]string{"user": f.User, "account": f.Account, "client": f.Client,
		"modell": f.Modell, "sitzung": f.Sitzung, "version": f.Version} {
		if strings.ContainsAny(value, "\r\n") {
			problems = append(problems, field+" enthält einen Zeilenumbruch")
		}
	}
	if len(problems) == 0 {
		return nil
	}
	return docRefErrorf(DocCodeInvalidInput, "Meldung ungültig: %s", strings.Join(problems, "; "))
}

// feedbackValues sind die Felder des Kopfs einer Meldung als Abbildung.
func feedbackValues(f Feedback) map[string]string {
	return map[string]string{
		"title": f.Title, "art": f.Art, "dringlichkeit": f.Dringlichkeit, "betrifft": f.Betrifft,
		"status": f.Status, "von": f.Von, "user": f.User, "account": f.Account, "client": f.Client,
		"modell": f.Modell, "sitzung": f.Sitzung, "version": f.Version,
	}
}

// composeFeedbackDoc baut den Inhalt einer Meldung: Kopf in fester
// Reihenfolge, die Überschrift „# Meldung <NNN> — <Titel>“ und der Text.
// Leere Felder fallen weg; extra hängt unverändert hinter den eigenen. Die
// Datei endet mit genau einem Zeilenumbruch.
func composeFeedbackDoc(number string, fields map[string]string, extra []string, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, key := range feedbackFieldOrder {
		value := strings.TrimSpace(fields[key])
		if value == "" {
			continue
		}
		if feedbackQuoted[key] {
			value = quoteYAMLTitle(value)
		}
		b.WriteString(key + ": " + value + "\n")
	}
	for _, line := range extra {
		b.WriteString(line + "\n")
	}
	b.WriteString("---\n\n# Meldung " + number + " — " + strings.TrimSpace(fields["title"]) + "\n")
	if text := strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n")); text != "" {
		b.WriteString("\n" + text + "\n")
	}
	return b.String()
}

// parseFeedbackDoc zerlegt den Inhalt einer Meldung. Ein Kopf, den yamllite
// nicht versteht, ist kein Fehler: Seine Zeilen bleiben als fremde Felder
// erhalten, die eigenen gelten als leer, problem sagt es.
func parseFeedbackDoc(content string) feedbackDoc {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	doc := feedbackDoc{fields: map[string]string{}}
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		doc.body = feedbackBody(content)
		doc.problem = "kein Kopf"
		return doc
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		doc.body = feedbackBody(content)
		doc.problem = "Kopf ohne schließenden Strich"
		return doc
	}
	doc.body = feedbackBody(strings.Join(lines[end+1:], "\n"))
	block := lines[1:end]
	root, err := yamllite.Parse([]byte(strings.Join(block, "\n")))
	if err != nil || root == nil || root.Kind != yamllite.Mapping {
		for _, line := range block {
			if strings.TrimSpace(line) != "" {
				doc.extra = append(doc.extra, line)
			}
		}
		doc.problem = "Kopf nicht lesbar"
		if err != nil {
			doc.problem += ": " + err.Error()
		}
		return doc
	}
	known := map[string]bool{}
	for _, key := range feedbackFieldOrder {
		known[key] = true
		doc.fields[key] = strings.TrimSpace(root.Get(key).Str())
	}
	keep := false
	for _, line := range block {
		if m := feedbackKeyLine.FindStringSubmatch(line); m != nil {
			keep = !known[m[1]]
		} else if strings.TrimSpace(line) == "" {
			continue
		}
		if keep {
			doc.extra = append(doc.extra, line)
		}
	}
	return doc
}

// feedbackBody ist der Text ohne die Überschrift, die composeFeedbackDoc
// setzt, und ohne Leerraum an den Rändern.
func feedbackBody(text string) string {
	text = strings.TrimSpace(text)
	first, rest, _ := strings.Cut(text, "\n")
	if feedbackHeading.MatchString(first) {
		text = strings.TrimSpace(rest)
	}
	return text
}

// feedbackFromDoc baut die Ausgabe aus einem zerlegten Dokument.
func feedbackFromDoc(doc feedbackDoc) Feedback {
	f := doc.fields
	return Feedback{
		Title: f["title"], Art: f["art"], Dringlichkeit: f["dringlichkeit"], Betrifft: f["betrifft"],
		Status: f["status"], Von: f["von"], User: f["user"], Account: f["account"], Client: f["client"],
		Modell: f["modell"], Sitzung: f["sitzung"], Version: f["version"], Body: doc.body, Problem: doc.problem,
	}
}

// feedbackShortName ist der Kurzname im Dateinamen: die ersten Wörter des
// Titels, normalisiert wie bei doc_create. Bleibt nichts übrig, heißt er
// meldung.
func feedbackShortName(title string) string {
	words := strings.Fields(title)
	if len(words) > feedbackShortNameWords {
		words = words[:feedbackShortNameWords]
	}
	name, err := NormalizeDocName(strings.Join(words, " "))
	if err != nil {
		return "meldung"
	}
	if len(name) > feedbackMaxShortName {
		name = strings.TrimRight(name[:feedbackMaxShortName], "-")
	}
	return name
}

// FeedbackIdentity sind die Angaben, die das Programm über den Meldenden
// selbst kennt: User und Account vom Node, die Adresse der eigenen
// Collection.
type FeedbackIdentity struct {
	User    string
	Account string
	Address string
}

// FeedbackStore sind die Aufrufe an Kephalaion, die die Meldungen brauchen;
// *kephalaion.Client erfüllt sie, Tests setzen kephtest.Store ein.
type FeedbackStore interface {
	DocStore
}

// OpenFeedbackStore öffnet den Zugang zur Collection des Projekts;
// austauschbar, damit Tests den Node nicht fragen — auch die anderer Pakete
// (Hüllen, Subkommando), die dafür kephtest.Store einsetzen. Ist
// kephalaion.ok nicht wahr, ist der Fehler DocCodeKephalaionUnavailable.
var OpenFeedbackStore = func(projectDir string) (FeedbackStore, FeedbackIdentity, error) {
	status := KephalaionStatus(projectDir)
	client, user, err := openKephClientStatus(projectDir, "", status)
	if err != nil {
		return nil, FeedbackIdentity{}, err
	}
	return client, FeedbackIdentity{User: user, Account: status.Account, Address: status.Address}, nil
}

// feedbackAccess ist der geöffnete Zugang: Store, Sorte, Nummernvergabe der
// Sorten-Tabelle und wer meldet.
type feedbackAccess struct {
	docs     *UserDocs
	store    FeedbackStore
	kind     DocKind
	identity FeedbackIdentity
}

func openFeedback(projectDir string) (*feedbackAccess, error) {
	store, identity, err := OpenFeedbackStore(projectDir)
	if err != nil {
		return nil, err
	}
	docs, err := NewUserDocs(DocKinds, store, identity.User)
	if err != nil {
		return nil, err
	}
	kind, ok := docs.anyKind(FeedbackKindName)
	if !ok {
		return nil, fmt.Errorf("Sorten-Tabelle ohne %s", FeedbackKindName)
	}
	return &feedbackAccess{docs: docs, store: store, kind: kind, identity: identity}, nil
}

// feedbackKephError übersetzt einen Fehler von Kephalaion wie kephError, mit
// dem Ausweg der Feedback-Werkzeuge bei offenem Ausgang.
func feedbackKephError(err error) error {
	var refErr *DocRefError
	if errors.As(err, &refErr) {
		return err
	}
	if kephalaion.ErrorCode(err) == kephalaion.CodeOutcomeUnknown {
		return &DocRefError{Code: DocCodeOutcomeUnknown, err: fmt.Errorf(
			"%w Nachsehen mit k_playbook_feedback_list (includeDone) bzw. k-playbook feedback list --all, ob der Vorgang geschehen ist; "+
				"nicht noch einmal ausführen", err)}
	}
	return kephError(err)
}

// FeedbackVersion ist die Programmversion, die eine neue Meldung trägt;
// austauschbar für Tests.
var FeedbackVersion = func() string {
	if version := strings.TrimSpace(buildinfo.Version); version != "" {
		return version
	}
	return "unbekannt"
}

// FeedbackCreateOptions sind die Eingaben von CreateFeedback. Title, Art,
// Dringlichkeit, Betrifft und Body liefert die Sitzung; Von, Client, Modell
// und Sitzung setzt der Zugriffsweg (MCP: von ki, client aus clientInfo,
// sonst als Parameter). Bei von: ki sind Client, Modell und Sitzung Pflicht,
// bei von: mensch stehen Client und Modell nicht da und Sitzung ist
// wahlweise. User, Account, Version und Status setzt CreateFeedback selbst.
type FeedbackCreateOptions struct {
	Title         string
	Art           string
	Dringlichkeit string
	Betrifft      string
	Body          string
	Von           string
	Client        string
	Modell        string
	Sitzung       string
}

// FeedbackCreated ist die Antwort von CreateFeedback.
type FeedbackCreated struct {
	Ref      string   `json:"ref"`
	Number   int      `json:"number"`
	Name     string   `json:"name"`
	Revision int64    `json:"revision"`
	Feedback Feedback `json:"feedback"`
}

// CreateFeedback legt eine Meldung mit der nächsten Nummer aus feedback/
// samt done/ an, mit fertigem Inhalt in einem Zug — ein leeres
// Zwischendokument gibt es nicht. Ist der Name belegt, versucht es die
// nächste Nummer; ein offener Ausgang zieht nie die nächste Nummer.
func CreateFeedback(ctx context.Context, projectDir string, opts FeedbackCreateOptions) (FeedbackCreated, error) {
	f := Feedback{
		Title: strings.TrimSpace(opts.Title), Art: strings.TrimSpace(opts.Art),
		Dringlichkeit: strings.TrimSpace(opts.Dringlichkeit), Betrifft: strings.TrimSpace(opts.Betrifft),
		Status: FeedbackStatusOffen, Von: strings.TrimSpace(opts.Von), Client: strings.TrimSpace(opts.Client),
		Modell: strings.TrimSpace(opts.Modell), Sitzung: strings.TrimSpace(opts.Sitzung),
		Body: strings.TrimSpace(strings.ReplaceAll(opts.Body, "\r\n", "\n")),
	}
	if err := checkFeedbackFields(f); err != nil {
		return FeedbackCreated{}, err
	}
	if f.Body == "" {
		return FeedbackCreated{}, docRefErrorf(DocCodeInvalidInput, "body fehlt: eine Meldung braucht einen Text")
	}
	if f.Von == FeedbackVonKI {
		var missing []string
		for _, field := range []struct{ name, value string }{{"client", f.Client}, {"modell", f.Modell}, {"sitzung", f.Sitzung}} {
			if field.value == "" {
				missing = append(missing, field.name)
			}
		}
		if len(missing) > 0 {
			return FeedbackCreated{}, docRefErrorf(DocCodeInvalidInput,
				"bei von: ki fehlt %s — eine Sitzung nennt Client, Modell und die Sitzung aus der Zeile Sitzung: des Briefings",
				strings.Join(missing, ", "))
		}
	}
	access, err := openFeedback(projectDir)
	if err != nil {
		return FeedbackCreated{}, err
	}
	f.User, f.Account, f.Version = access.identity.User, access.identity.Account, FeedbackVersion()
	if err := checkFeedbackFields(f); err != nil {
		return FeedbackCreated{}, err
	}
	fields := feedbackValues(f)
	created, doc, err := access.docs.createNumberedWith(ctx, access.kind, access.kind.Dir, feedbackShortName(f.Title),
		func(number int) string { return composeFeedbackDoc(fmt.Sprintf("%03d", number), fields, nil, f.Body) },
		feedbackKephError)
	if err != nil {
		return FeedbackCreated{}, err
	}
	f.Ref, f.Name, f.Number, f.Revision = created.Ref, created.Name, created.Number, doc.Revision
	if doc.Created != nil {
		f.Created = doc.Created.At
	}
	if doc.Updated != nil {
		f.Updated = doc.Updated.At
	}
	return FeedbackCreated{Ref: created.Ref, Number: created.Number, Name: created.Name, Revision: doc.Revision, Feedback: f}, nil
}
