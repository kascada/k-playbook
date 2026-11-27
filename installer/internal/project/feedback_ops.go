package project

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Listen und Status ändern (Task 198, Etappe 2). Beide arbeiten nur in der
// eigenen Collection; über andere Collections gibt es nur die Zahl offener
// Meldungen (feedback_count.go).

// feedbackEntry ist eine gelesene Meldung samt dem, was Ändern braucht.
type feedbackEntry struct {
	feedback Feedback
	name     string
	doc      feedbackDoc
}

// loadAll liest alle Meldungen, offene und erledigte, sortiert nach Nummer.
// Dokumente ohne Nummer vorn im Namen gehören nicht dazu.
func (a *feedbackAccess) loadAll(ctx context.Context) ([]feedbackEntry, error) {
	entries, matches, err := a.docs.listKind(ctx, a.kind, a.docs.User(), false)
	if err != nil {
		return nil, err
	}
	var found []feedbackEntry
	for i, entry := range entries {
		number, ok := docNumber(matches[i].file)
		if !ok {
			continue
		}
		read, err := a.store.Read(ctx, entry.Name)
		if err != nil {
			return nil, feedbackKephError(err)
		}
		if read.Kind != kephalaion.ReadKindDocument || read.Content == nil {
			// Zwischen list und read gelöscht oder verschoben.
			continue
		}
		doc := parseFeedbackDoc(*read.Content)
		f := feedbackFromDoc(doc)
		f.Ref, f.Name, f.Number, f.Done, f.Revision = DocSchemeKeph+":/"+entry.Name, matches[i].file, number, matches[i].done, read.Revision
		if read.Created != nil {
			f.Created = read.Created.At
		}
		if read.Updated != nil {
			f.Updated = read.Updated.At
		}
		found = append(found, feedbackEntry{feedback: f, name: entry.Name, doc: doc})
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].feedback.Number != found[j].feedback.Number {
			return found[i].feedback.Number < found[j].feedback.Number
		}
		return found[i].name < found[j].name
	})
	return found, nil
}

// FeedbackListOptions sind die Filter von ListFeedback; ein leerer Filter
// greift nicht.
type FeedbackListOptions struct {
	// IncludeDone nimmt die Meldungen unter feedback/done/ mit auf. Ein
	// Filter status auf erledigt oder abgelehnt tut das von selbst.
	IncludeDone bool
	Art         string
	Betrifft    string
	Status      string
	// AllCollections zählt zusätzlich die offenen Meldungen in jeder anderen
	// lesbaren Collection aller Hubs (feedback_count.go); die Filter gelten
	// dafür nicht.
	AllCollections bool
}

// FeedbackListing ist die Antwort von ListFeedback.
type FeedbackListing struct {
	// Items sind die Meldungen der eigenen Collection nach Nummer, samt Text.
	Items []Feedback `json:"items"`
	// Collections ist mit AllCollections die Zählung der anderen lesbaren
	// Collections, nach Adresse; leer, wenn es keine gibt.
	Collections []FeedbackCollectionCount `json:"collections,omitempty"`
	// CollectionsError nennt, woran die Auskunft über die Hubs scheiterte;
	// die eigene Liste steht trotzdem da.
	CollectionsError string `json:"collectionsError,omitempty"`
}

// ListFeedback liefert die Meldungen der eigenen Collection nach Nummer,
// samt Text, und mit AllCollections die Zählung über die anderen.
func ListFeedback(ctx context.Context, projectDir string, opts FeedbackListOptions) (FeedbackListing, error) {
	filters := []struct {
		name, value string
		allowed     []string
	}{
		{"art", strings.TrimSpace(opts.Art), FeedbackArten},
		{"betrifft", strings.TrimSpace(opts.Betrifft), FeedbackBetreffe},
		{"status", strings.TrimSpace(opts.Status), FeedbackStatus},
	}
	for _, filter := range filters {
		if filter.value != "" {
			if err := checkFeedbackEnum(filter.name, filter.value, filter.allowed); err != nil {
				return FeedbackListing{}, err
			}
		}
	}
	art, betrifft, status := filters[0].value, filters[1].value, filters[2].value
	includeDone := opts.IncludeDone || feedbackStatusDone(status)
	access, err := openFeedback(projectDir)
	if err != nil {
		return FeedbackListing{}, err
	}
	entries, err := access.loadAll(ctx)
	if err != nil {
		return FeedbackListing{}, err
	}
	out := []Feedback{}
	for _, entry := range entries {
		f := entry.feedback
		switch {
		case f.Done && !includeDone:
		case art != "" && f.Art != art:
		case betrifft != "" && f.Betrifft != betrifft:
		case status != "" && f.Status != status:
		default:
			out = append(out, f)
		}
	}
	listing := FeedbackListing{Items: out}
	if opts.AllCollections {
		counts, err := CountFeedbackElsewhere(ctx, access.identity.Address)
		if err != nil {
			listing.CollectionsError = err.Error()
		}
		listing.Collections = counts
	}
	return listing, nil
}

// feedbackStatusDone sagt, ob eine Meldung mit diesem Status unter done/
// liegt.
func feedbackStatusDone(status string) bool {
	return status == FeedbackStatusErledigt || status == FeedbackStatusAbgelehnt
}

// FeedbackUpdated ist die Antwort von UpdateFeedback.
type FeedbackUpdated struct {
	Feedback Feedback `json:"feedback"`
	// From ist der Verweis vor dem Ändern; Ref in Feedback der danach.
	From string `json:"from"`
	// Changed sagt, ob geschrieben wurde. Derselbe Status schreibt nichts.
	Changed bool `json:"changed"`
}

// find sucht eine Meldung nach Verweis oder Nummer, offen oder erledigt.
func (a *feedbackAccess) find(ctx context.Context, target string) (feedbackEntry, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return feedbackEntry{}, docRefErrorf(DocCodeInvalidInput, "erwartet wird ref oder die Nummer einer Meldung")
	}
	var name string
	number := 0
	if rest, ok := strings.CutPrefix(target, DocSchemeKeph+":/"); ok {
		if _, ok := a.kind.match(rest); !ok {
			return feedbackEntry{}, docRefErrorf(DocCodeWrongDir, "%s liegt nicht unter %s/ oder %s/%s/", target, a.kind.Dir, a.kind.Dir, docDoneDir)
		}
		name = rest
	} else {
		n, err := strconv.Atoi(strings.TrimPrefix(target, "#"))
		if err != nil || n < 1 {
			return feedbackEntry{}, docRefErrorf(DocCodeInvalidInput, "%q ist weder ein keph:-Verweis noch eine Nummer", target)
		}
		number = n
	}
	entries, err := a.loadAll(ctx)
	if err != nil {
		return feedbackEntry{}, err
	}
	var hits []feedbackEntry
	for _, entry := range entries {
		if (name != "" && entry.name == name) || (number > 0 && entry.feedback.Number == number) {
			hits = append(hits, entry)
		}
	}
	switch len(hits) {
	case 0:
		return feedbackEntry{}, docRefErrorf(DocCodeNotFound, "keine Meldung %s", target)
	case 1:
		return hits[0], nil
	}
	refs := make([]string, 0, len(hits))
	for _, hit := range hits {
		refs = append(refs, hit.feedback.Ref)
	}
	return feedbackEntry{}, docRefErrorf(DocCodeInvalidInput, "die Nummer %d ist mehrdeutig: %s — mit ref wählen",
		number, strings.Join(refs, ", "))
}

// UpdateFeedback setzt den Status einer Meldung der eigenen Collection.
// erledigt und abgelehnt verschieben nach feedback/done/, offen und
// angenommen holen sie von dort zurück. Verschoben wird nach dem Schreiben
// des Status, zurückgeholt davor — wie bei den Todos, damit ein halber
// Vorgang den Status am richtigen Ort zeigt oder beim nächsten Ändern
// bereinigt wird.
//
// sinceRevision ist die Revision, die der Aufrufer zuletzt gesehen hat; 0
// heißt ohne Vorbedingung. Weicht die Meldung davon ab, ist das
// foreign_change und nichts wird geschrieben; liegt die gelesene Revision
// darunter, läuft die Replica nach, und es wird kurz gewartet und neu
// gelesen.
func UpdateFeedback(ctx context.Context, projectDir, target, status string, sinceRevision int64) (FeedbackUpdated, error) {
	status = strings.TrimSpace(status)
	if err := checkFeedbackEnum("status", status, FeedbackStatus); err != nil {
		return FeedbackUpdated{}, err
	}
	if sinceRevision < 0 {
		return FeedbackUpdated{}, docRefErrorf(DocCodeInvalidInput, "since_revision %d ist keine Revision", sinceRevision)
	}
	access, err := openFeedback(projectDir)
	if err != nil {
		return FeedbackUpdated{}, err
	}
	entry, err := access.find(ctx, target)
	for attempt := 1; err == nil && sinceRevision > 0 && entry.feedback.Revision < sinceRevision && attempt <= docEditRetries; attempt++ {
		time.Sleep(docEditWait(attempt))
		entry, err = access.find(ctx, entry.feedback.Ref)
	}
	if err != nil {
		return FeedbackUpdated{}, err
	}
	f := entry.feedback
	if sinceRevision > 0 && f.Revision != sinceRevision {
		return FeedbackUpdated{}, &DocRefError{Code: DocCodeForeignChange, err: &DocForeignChange{
			Ref: f.Ref, SinceRevision: sinceRevision, Revision: f.Revision, UpdatedAt: f.Updated, Tail: ""}}
	}
	if entry.doc.problem != "" || !slices.Contains(FeedbackStatus, f.Status) {
		return FeedbackUpdated{}, docRefErrorf(DocCodeInvalidInput,
			"der Kopf von %s ist nicht lesbar oder trägt keinen gültigen Status (%s) — von Hand in Kephalaion prüfen, nichts geschrieben",
			f.Ref, strings.TrimSpace(entry.doc.problem+" status: "+f.Status))
	}
	result := FeedbackUpdated{From: f.Ref}
	toDone := feedbackStatusDone(status)
	if f.Status == status && f.Done == toDone {
		result.Feedback = f
		return result, nil
	}
	fields := map[string]string{}
	for key, value := range entry.doc.fields {
		fields[key] = value
	}
	fields["status"] = status
	content := composeFeedbackDoc(fmt.Sprintf("%03d", f.Number), fields, entry.doc.extra, entry.doc.body)
	openName := access.kind.Dir + "/" + f.Name
	doneName := access.kind.Dir + "/" + docDoneDir + "/" + f.Name
	name, revision := entry.name, f.Revision

	write := func() error {
		if f.Status == status {
			return nil
		}
		doc, err := access.store.Write(ctx, name, content, revision)
		if err != nil {
			return err
		}
		revision = doc.Revision
		return nil
	}
	move := func(to string) error {
		doc, err := access.store.Rename(ctx, name, to)
		if err != nil {
			return err
		}
		name, revision = to, doc.Revision
		if doc.Name != "" {
			name = doc.Name
		}
		return nil
	}
	switch {
	case toDone && !f.Done:
		if err := write(); err != nil {
			return FeedbackUpdated{}, feedbackKephError(err)
		}
		if err := move(doneName); err != nil {
			return FeedbackUpdated{}, feedbackStepError(err, "status ist geschrieben, das Verschieben nach done/ nicht")
		}
	case !toDone && f.Done:
		if err := move(openName); err != nil {
			return FeedbackUpdated{}, feedbackKephError(err)
		}
		if err := write(); err != nil {
			return FeedbackUpdated{}, feedbackStepError(err, "die Meldung ist zurück unter feedback/, der Status noch nicht geschrieben")
		}
	default:
		if err := write(); err != nil {
			return FeedbackUpdated{}, feedbackKephError(err)
		}
	}
	f.Status, f.Done, f.Revision, f.Ref = status, toDone, revision, DocSchemeKeph+":/"+name
	result.Feedback, result.Changed = f, true
	return result, nil
}

// feedbackStepError ist der Fehler des zweiten Schritts: der Code wie sonst,
// die Meldung sagt, was schon geschehen ist.
func feedbackStepError(err error, state string) error {
	mapped := feedbackKephError(err)
	return &DocRefError{Code: DocRefErrorCode(mapped), err: fmt.Errorf("%s: %w", state, mapped)}
}
