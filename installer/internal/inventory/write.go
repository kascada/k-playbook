package inventory

import (
	"context"
	"fmt"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Outcome sagt, was ein Lauf mit dem Inventar gemacht hat.
type Outcome struct {
	// Path ist der Ort des Inventars als Verweis: DocRef.
	Path string `json:"path"`
	// Written ist false, wenn der Bestand inhaltlich derselbe war und das
	// Dokument deshalb gar nicht angefasst wurde.
	Written bool `json:"written"`
	// At ist der Zeitstempel, der jetzt im Dokument steht: bei einem
	// geschriebenen Lauf der dieses Laufs, sonst der des Bestands.
	At string `json:"at"`
	// Revision ist die Revision des Dokuments nach dem Lauf: die des
	// Bestands, wenn nichts geschrieben wurde, sonst die neue.
	Revision int64 `json:"revision,omitempty"`
	// Problem übernimmt einen sichtbaren Befund zum Bestand, etwa ein defektes
	// Frontmatter. Dann wird neu geschrieben, weil ein Vergleich nicht möglich
	// ist.
	Problem string `json:"problem,omitempty"`
}

// Write setzt die Byte-Stabilitäts- und Zeitstempelregel des Vertrags um,
// gegen die Collection des Projekts:
//
//  1. Das vollständige Ergebnis wird im Speicher gerendert.
//  2. Liegt das Dokument in der Collection, wird verglichen — ausgenommen
//     `generated.at`, und ausgenommen nichts anderes.
//  3. Sind sie gleich, wird gar nicht geschrieben.
//  4. Unterscheiden sie sich, wird vollständig neu geschrieben, mit
//     `generated.at` = Zeitpunkt dieses Laufs und der gelesenen Revision als
//     base_revision: hat inzwischen jemand anders geschrieben, ist das ein
//     sichtbarer Fehler (stale_revision), kein stilles Überschreiben.
//  5. Fehlt das Dokument, wird es angelegt.
//
// Der Vergleich läuft über den Bestand selbst, nicht über eine Prüfsumme
// daneben: derselbe Bestand wird ein zweites Mal gerendert, mit dem Zeitstempel,
// der schon dort steht. Sind die Bytes dann gleich, hat sich nichts geändert.
//
// Ein Fehler des Stores kommt unverändert zurück; ohne erreichbares Kephalaion
// gibt es kein Inventar und keinen Rückfall auf die Platte.
func Write(ctx context.Context, store Store, options Options, result Result) (Outcome, error) {
	if store == nil {
		return Outcome{Path: DocRef}, fmt.Errorf("kein Zugang zur Collection des Projekts")
	}
	outcome := Outcome{Path: DocRef}

	read, err := store.Read(ctx, DocName)
	if err != nil {
		return outcome, fmt.Errorf("Bestand %s lesen: %w", DocRef, err)
	}

	var existing *kephalaion.ReadResult
	switch read.Kind {
	case kephalaion.ReadKindNone:
		// Nichts da: anlegen.
	case kephalaion.ReadKindDirectory:
		return outcome, fmt.Errorf("unter %s liegt ein Verzeichnis, kein Dokument", DocRef)
	default:
		existing = &read
		status := StatusOf([]byte(*read.Content))
		if status.Problem != "" {
			outcome.Problem = status.Problem
		} else if Render(result, status.GeneratedAt) == *read.Content {
			outcome.At = status.GeneratedAt
			outcome.Revision = read.Revision
			return outcome, nil
		}
	}

	at := options.now().Format(time.RFC3339)
	content := Render(result, at)
	var doc kephalaion.Document
	if existing == nil {
		doc, err = store.Create(ctx, DocName, content)
	} else {
		doc, err = store.Write(ctx, DocName, content, existing.Revision)
	}
	if err != nil {
		return outcome, fmt.Errorf("%s schreiben: %w", DocRef, err)
	}
	outcome.Written = true
	outcome.At = at
	outcome.Revision = doc.Revision
	return outcome, nil
}

// Run erhebt und schreibt in einem Zug. Das ist der Weg, den jeder Einstieg
// nimmt; die Vertrauensgrenze liegt dahinter und nicht beim Aufrufer.
func Run(ctx context.Context, store Store, options Options) (Result, Outcome, error) {
	result, err := Collect(options)
	if err != nil {
		return Result{}, Outcome{Path: DocRef}, err
	}
	outcome, err := Write(ctx, store, options, result)
	return result, outcome, err
}
