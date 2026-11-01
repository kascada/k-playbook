package project

import (
	"context"
	"regexp"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// grep (Task 104): Zeilen eines Dokuments oder aller Dokumente einer Sorte
// nach einem Muster finden, ohne den Inhalt in den Kontext des Aufrufers zu
// holen. Gesucht wird im gespeicherten Text — ohne eingesetzte Includes —,
// damit Zeilen und Treffer zu dem passen, was replace und append ändern.

// docGrepMaxMatches ist die Höchstzahl der Treffer einer Antwort; darüber
// ist sie truncated.
const docGrepMaxMatches = 200

// DocGrepOptions sind die Eingaben von grep, außer dem Ziel.
type DocGrepOptions struct {
	// Pattern ist ein regulärer Ausdruck nach Go (RE2), zeilenweise.
	Pattern string
	// CountOnly liefert nur die Zahl der Treffer je Dokument, mit Revision.
	CountOnly bool
}

// DocGrepMatch ist ein Treffer.
type DocGrepMatch struct {
	Ref string `json:"ref"`
	// Line ist die Zeilennummer, 1-basiert.
	Line int `json:"line"`
	// Text ist die Zeile ohne Zeilenumbruch.
	Text string `json:"text"`
}

// DocGrepCount ist die Zahl der Treffer in einem Dokument (count_only).
type DocGrepCount struct {
	Ref   string `json:"ref"`
	Count int    `json:"count"`
	// Revision steht bei keph:.
	Revision int64 `json:"revision,omitempty"`
}

// DocGrepResult ist die Antwort von grep.
type DocGrepResult struct {
	// Ref ist das Ziel, wenn es ein Verweis war, in kanonischer Form.
	Ref string `json:"ref,omitempty"`
	// Kind, User und Done beschreiben das Ziel, wenn es eine Sorte war —
	// User wie in DocList.
	Kind string `json:"kind,omitempty"`
	User string `json:"user,omitempty"`
	Done bool   `json:"done,omitempty"`
	// Revision ist die Revision des Dokuments bei einem keph:-Verweis als
	// Ziel.
	Revision int64 `json:"revision,omitempty"`
	// Matches sind die Treffer in Lesereihenfolge, höchstens
	// docGrepMaxMatches; fehlt ohne Treffer und mit count_only.
	Matches []DocGrepMatch `json:"matches,omitempty"`
	// Counts nennt mit count_only die Treffer je Dokument, auch 0.
	Counts []DocGrepCount `json:"counts,omitempty"`
	// Count ist die Zahl aller Treffer, auch über truncated hinaus.
	Count int `json:"count"`
	// Truncated sagt, dass es mehr Treffer gibt als in Matches stehen.
	Truncated bool `json:"truncated,omitempty"`
	// Hint erklärt, warum ein Filter nicht gegriffen hat (wie DocList).
	Hint string `json:"hint,omitempty"`
}

// compileDocGrep prüft das Muster.
func compileDocGrep(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, docRefErrorf(DocCodeInvalidInput, "kein Muster angegeben")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, docRefErrorf(DocCodeInvalidInput, "Muster %q ist kein regulärer Ausdruck nach RE2: %v", pattern, err)
	}
	return re, nil
}

// add durchsucht den Inhalt eines Dokuments und nimmt die Treffer auf.
func (r *DocGrepResult) add(re *regexp.Regexp, ref, content string, revision int64, countOnly bool) {
	count := 0
	for i, line := range docLines(content) {
		text := strings.TrimRight(line, "\r\n")
		if !re.MatchString(text) {
			continue
		}
		count++
		if countOnly {
			continue
		}
		if len(r.Matches) >= docGrepMaxMatches {
			r.Truncated = true
			continue
		}
		r.Matches = append(r.Matches, DocGrepMatch{Ref: ref, Line: i + 1, Text: text})
	}
	r.Count += count
	if countOnly {
		r.Counts = append(r.Counts, DocGrepCount{Ref: ref, Count: count, Revision: revision})
	}
}

// GrepDocRef durchsucht eine Einzeldatei hinter einem Verweis — dieselben
// Verweise wie ResolveDocRef, ohne Verzeichnisse und ohne eingesetzte
// Includes. Bei keph: trägt die Antwort die Revision.
func GrepDocRef(ctx context.Context, layers DocLayers, raw string, opts DocGrepOptions) (DocGrepResult, error) {
	re, err := compileDocGrep(opts.Pattern)
	if err != nil {
		return DocGrepResult{}, err
	}
	ref, err := ParseDocRef(raw)
	if err != nil {
		return DocGrepResult{}, err
	}
	if ref.Dir {
		return DocGrepResult{}, docRefErrorf(DocCodeInvalidInput,
			"%s: grep durchsucht eine Einzeldatei; für alle Dokumente einer Sorte kind angeben", ref.Raw)
	}
	doc, err := resolveDocRefFull(ctx, layers, ref, DocReadOptions{Raw: true})
	if err != nil {
		return DocGrepResult{}, err
	}
	result := DocGrepResult{Ref: doc.Ref, Revision: doc.Revision}
	result.add(re, doc.Ref, doc.Content, doc.Revision, opts.CountOnly)
	return result, nil
}

// Grep durchsucht alle Dokumente einer Sorte, ausgewählt wie bei List (user,
// done). Gelesen wird jedes über den Node; was zwischen list und read
// verschwunden ist, fällt weg.
func (u *UserDocs) Grep(ctx context.Context, list DocListOptions, opts DocGrepOptions) (DocGrepResult, error) {
	re, err := compileDocGrep(opts.Pattern)
	if err != nil {
		return DocGrepResult{}, err
	}
	docs, err := u.List(ctx, list)
	if err != nil {
		return DocGrepResult{}, err
	}
	result := DocGrepResult{Kind: docs.Kind, User: docs.User, Done: docs.Done, Hint: docs.Hint}
	for _, item := range docs.Items {
		name := strings.TrimPrefix(item.Ref, DocSchemeKeph+":/")
		read, err := u.store.Read(ctx, name)
		if err != nil {
			return DocGrepResult{}, kephError(err)
		}
		if read.Kind != kephalaion.ReadKindDocument || read.Content == nil {
			continue
		}
		result.add(re, item.Ref, *read.Content, read.Revision, opts.CountOnly)
	}
	return result, nil
}
