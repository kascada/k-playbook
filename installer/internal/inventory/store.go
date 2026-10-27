package inventory

import (
	"context"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Das Inventar liegt in der Collection des Projekts, nicht mehr auf der
// Platte (Task 100, Entscheidung 2026-10-26): ein Dokument unter
// docs/generated/versions/, dem Verzeichnis dieses Erzeugers in der von
// Kephalaion indizierten Ablage docs/. Der Name steht hier einmal; Subkommando,
// Command und Oberfläche sprechen ihn nur über diese Konstanten an.
//
// Ohne bereite Anbindung gibt es kein Inventar — keinen Rückfall auf die
// Platte, keine zweite Fassung. Wer den Store öffnet, steht in internal/project
// (OpenInventoryStore); dieses Paket kennt nur die Schnittstelle, weil project
// es seinerseits importiert.
const (
	// DocName ist der Name des Inventars in der Collection, ohne führenden
	// Schrägstrich, so wie Kephalaion read, create und write ihn erwarten.
	DocName = "docs/generated/versions/inventory.md"
	// DocRef ist derselbe Ort als Verweis im Präfixschema; der steht in
	// Berichten, in der Oberfläche und in jedem Text, der das Inventar nennt.
	DocRef = "keph:/" + DocName
)

// Store sind die Aufrufe an Kephalaion, die das Inventar braucht.
// *kephalaion.Client erfüllt sie; Tests setzen kephtest.Store ein. Read liest
// die Replica des Nodes, Create und Write gehen über den Hub. Write mit einer
// baseRevision, die nicht mehr die des Dokuments ist, scheitert mit
// kephalaion.CodeStaleRevision und schreibt nichts.
type Store interface {
	Read(ctx context.Context, name string) (kephalaion.ReadResult, error)
	Create(ctx context.Context, name, content string) (kephalaion.Document, error)
	Write(ctx context.Context, name, content string, baseRevision int64) (kephalaion.Document, error)
}
