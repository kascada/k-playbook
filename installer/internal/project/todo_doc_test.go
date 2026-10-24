package project

import (
	"reflect"
	"testing"
	"time"
)

func TestTodoDocOhneFelderIstNurText(t *testing.T) {
	content := composeTodoDoc(todoDoc{Text: "  Einen Punkt erledigen \n"})
	if content != "Einen Punkt erledigen\n" {
		t.Fatalf("Inhalt: %q", content)
	}
	doc := parseTodoDoc(content)
	if doc.Text != "Einen Punkt erledigen" || doc.HasCreated || doc.Origin != "" || doc.Done != "" || doc.DoneMigrated {
		t.Fatalf("gelesen: %+v", doc)
	}
}

func TestTodoDocFelderHinUndZurueck(t *testing.T) {
	want := todoDoc{Text: "Zeile eins\n\nZeile zwei: mit Doppelpunkt", Origin: "Task 026: Etappe 3",
		Done: "2026-10-20", DoneMigrated: true, Created: "2026-09-08", HasCreated: true}
	content := composeTodoDoc(want)
	wantContent := "---\norigin: \"Task 026: Etappe 3\"\ncreated: 2026-09-08\ndone: 2026-10-20\ndoneMigrated: true\n---\n\n" +
		"Zeile eins\n\nZeile zwei: mit Doppelpunkt\n"
	if content != wantContent {
		t.Fatalf("Inhalt:\n%s\nerwartet:\n%s", content, wantContent)
	}
	if got := parseTodoDoc(content); !reflect.DeepEqual(got, want) {
		t.Fatalf("gelesen: %+v\nerwartet: %+v", got, want)
	}
}

func TestTodoDocLeeresCreatedGilt(t *testing.T) {
	// Ein übernommener Eintrag ohne Datum: created: steht da, leer.
	content := composeTodoDoc(todoDoc{Text: "x", HasCreated: true})
	doc := parseTodoDoc(content)
	if !doc.HasCreated || doc.Created != "" {
		t.Fatalf("leeres created verloren: %q → %+v", content, doc)
	}
	for _, raw := range []string{"---\ncreated:\n---\n\nx\n", "---\ncreated: \"\"\n---\nx"} {
		if doc := parseTodoDoc(raw); !doc.HasCreated || doc.Created != "" || doc.Text != "x" {
			t.Errorf("%q: %+v", raw, doc)
		}
	}
	if doc := parseTodoDoc("---\norigin: a\n---\nx"); doc.HasCreated {
		t.Errorf("created ohne Feld gesetzt: %+v", doc)
	}
}

func TestTodoDocTextMitStrichBekommtKopf(t *testing.T) {
	want := todoDoc{Text: "---\nkein Kopf"}
	content := composeTodoDoc(want)
	if got := parseTodoDoc(content); got.Text != want.Text {
		t.Fatalf("Text mit --- verfälscht: %q → %+v", content, got)
	}
}

func TestTodoDocFremdeFelderBleiben(t *testing.T) {
	raw := "---\ntitle: Etwas\norigin: alt\ntags:\n  - a\n  - b\ndone: 2026-01-01\n---\n\nText\n"
	doc := parseTodoDoc(raw)
	if doc.Origin != "alt" || doc.Done != "2026-01-01" || doc.Text != "Text" {
		t.Fatalf("gelesen: %+v", doc)
	}
	doc.Done = ""
	doc.Origin = "neu"
	want := "---\norigin: neu\ntitle: Etwas\ntags:\n  - a\n  - b\n---\n\nText\n"
	if got := composeTodoDoc(doc); got != want {
		t.Fatalf("geschrieben:\n%s\nerwartet:\n%s", got, want)
	}
}

func TestTodoDocUnlesbarerKopfGehtNichtVerloren(t *testing.T) {
	raw := "---\n: : kaputt [\n---\nText\n"
	doc := parseTodoDoc(raw)
	if doc.Text != "Text" {
		t.Fatalf("Text: %+v", doc)
	}
	if got := composeTodoDoc(doc); got != "---\n: : kaputt [\n---\n\nText\n" {
		t.Fatalf("Kopf verloren: %q", got)
	}
	if doc := parseTodoDoc("---\nnur ein Strich\n"); doc.Text != "---\nnur ein Strich" {
		t.Fatalf("ohne schließenden Strich: %+v", doc)
	}
}

func TestStampDate(t *testing.T) {
	at := "2026-10-14T10:00:00.000Z"
	want := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC).Local().Format("2006-01-02")
	if got := stampDate(at); got != want {
		t.Fatalf("stampDate(%q) = %q, erwartet %q", at, got, want)
	}
	if got := stampDate("2026-10-14 kaputt"); got != "2026-10-14" {
		t.Fatalf("Rückfall: %q", got)
	}
	if got := stampDate(""); got != "" {
		t.Fatalf("leer: %q", got)
	}
}
