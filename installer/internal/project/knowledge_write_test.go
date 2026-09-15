package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/inventory"
)

// readKnowledgeFile liest eine Datei der Wissensablage roh.
func readKnowledgeFile(t *testing.T, root, rel string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(KnowledgeDir(root), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s lesen: %v", rel, err)
	}
	return string(content)
}

// fixUpdated hält den Tag in updated für die Dauer des Tests fest.
func fixUpdated(t *testing.T, day string) {
	t.Helper()
	before := knowledgeUpdatedNow
	knowledgeUpdatedNow = func() string { return day }
	t.Cleanup(func() { knowledgeUpdatedNow = before })
}

func sessionDoc(path string) KnowledgeDocument {
	return KnowledgeDocument{
		Path: path, Title: "Befund", Subject: "Release", Origin: "Sitzung 42", State: KnowledgeStateReviewed,
		Body: "# Befund\n\nDer Wächter prüft VERSION.\n",
	}
}

// Write baut das Frontmatter aus den Feldern, in fester Reihenfolge, mit
// updated vom Werkzeug — und legt Ordner an, tauscht die Chunks und zählt die
// eigene Schreibung nicht als Drift.
func TestKnowledgeWriteVertrag(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)
	fixUpdated(t, "2026-09-12")

	doc := sessionDoc("findings/sitzung/befund.md")
	doc.Sources = []string{"chat/2026-09-12.md", " ", "mail/bericht.eml"}
	rel, err := knowledge.Write("session", doc, "")
	if err != nil || rel != "findings/sitzung/befund.md" {
		t.Fatalf("Write: %q, %v", rel, err)
	}
	want := "---\ntitle: Befund\nsubject: Release\norigin: Sitzung 42\nstate: reviewed\nformat: markdown\n" +
		"sources:\n  - chat/2026-09-12.md\n  - mail/bericht.eml\nupdated: 2026-09-12\n---\n\n# Befund\n\nDer Wächter prüft VERSION.\n"
	if got := readKnowledgeFile(t, root, rel); got != want {
		t.Errorf("geschrieben:\n%q\nerwartet:\n%q", got, want)
	}

	// Die Chunks sind da, und die eigene Schreibung ist keine Drift.
	hits, err := knowledge.Search("Wächter", KnowledgeFilter{Kind: "findings"}, 0)
	if err != nil || len(hits) != 1 || hits[0].Path != rel || hits[0].Heading != "Befund" {
		t.Errorf("Treffer nach Write: %+v, %v", hits, err)
	}
	status, err := knowledge.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Stale || status.StaleFiles != 0 {
		t.Errorf("Write als Drift gemeldet: %+v", status)
	}
	if status.ByKind["findings"].Files != 2 {
		t.Errorf("findings = %+v", status.ByKind["findings"])
	}

	// Überschreiben ersetzt die Chunks, statt sie zu verdoppeln; format
	// fehlt und wird markdown; Zeilenenden werden LF, Leerzeilen am Rand
	// des Rumpfs fallen.
	doc = sessionDoc("findings/sitzung/befund.md")
	doc.Body = "\r\n\r\n# Befund\r\n\r\nJetzt ohne den Begriff.\r\n\r\n"
	doc.State = KnowledgeStateRaw
	if _, err := knowledge.Write("session", doc, ""); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if hits, err := knowledge.Search("Wächter", KnowledgeFilter{}, 0); err != nil || len(hits) != 0 {
		t.Errorf("alter Chunk geblieben: %+v, %v", hits, err)
	}
	if got := readKnowledgeFile(t, root, rel); !strings.HasSuffix(got, "---\n\n# Befund\n\nJetzt ohne den Begriff.\n") || !strings.Contains(got, "state: raw\nformat: markdown\n") {
		t.Errorf("Überschreiben: %q", got)
	}

	// Jeder Erzeuger, der schreibt, darf das in sein Verzeichnis — auch der
	// Connector und die Person nach pitfalls/.
	for producer, path := range map[string]string{
		"docs-extract":         "extracted/thema.md",
		"connector:confluence": "external/confluence/seite.md",
		"person":               "pitfalls/falle.md",
	} {
		if rel, err := knowledge.Write(producer, sessionDoc(path), ""); err != nil || rel != path {
			t.Errorf("%s → %s: %q, %v", producer, path, rel, err)
		}
	}
}

// Die Felder werden geprüft, bevor irgendetwas geschrieben wird: jede
// Ablehnung hinterlässt keine Datei. superseded wird als state abgewiesen —
// es entsteht nur über supersede —, ein Kopf im Rumpf wird nicht
// durchgereicht, und die Generatoren dürfen nicht schreiben.
func TestKnowledgeWriteWeistAb(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)

	cases := map[string]struct {
		producer string
		doc      KnowledgeDocument
		message  string
	}{
		"Zone-Ausbruch":        {"session", sessionDoc("../x.md"), "führt aus"},
		"fremdes Verzeichnis":  {"session", sessionDoc("manual/x.md"), "außerhalb des Erzeugerverzeichnisses"},
		"unbekannter Erzeuger": {"gate", sessionDoc("findings/x.md"), "unbekannter Erzeuger"},
		"Generator docs-code":  {"docs-code", sessionDoc("code/x.md"), "keine Einzeldatei"},
		"Generator docs-tools": {"docs-tools", sessionDoc("libs/x.md"), "keine Einzeldatei"},
		"Generator inventory":  {"inventory", sessionDoc("versions/x.md"), "keine Einzeldatei"},
		"kein title":           {"session", with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.Title = " " }), "kein title"},
		"kein subject":         {"session", with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.Subject = "" }), "kein subject"},
		"kein origin":          {"session", with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.Origin = "" }), "kein origin"},
		"title mehrzeilig":     {"session", with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.Title = "a\nb" }), "mehrere Zeilen"},
		"kein state":           {"session", with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.State = "" }), "kein state"},
		"state superseded":     {"session", with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.State = "superseded" }), "nur über supersede"},
		"state unbekannt":      {"session", with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.State = "final" }), "unbekannter state"},
		"format unbekannt":     {"session", with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.Format = "docx" }), "unbekanntes format"},
		"leerer body":          {"session", with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.Body = " \n" }), "leerer body"},
		"Kopf im body":         {"session", with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.Body = "---\ntitle: Fremd\nsource: alt\n---\n\n# X\n" }), "Dateikopf"},
		"leerer Kopf im body":  {"session", with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.Body = "---\n---\n# X\n" }), "Dateikopf"},
		"source mehrzeilig":    {"session", with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.Sources = []string{"a\nb"} }), "mehrere Zeilen"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rel, err := knowledge.Write(tc.producer, tc.doc, "")
			if err == nil {
				t.Fatalf("angenommen als %q", rel)
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Errorf("Meldung %q trägt %q nicht", err, tc.message)
			}
		})
	}
	for _, rel := range []string{"findings/x.md", "manual/x.md", "code/x.md", "libs/x.md", "versions/x.md", "x.md"} {
		if pathExists(filepath.Join(KnowledgeDir(root), filepath.FromSlash(rel))) {
			t.Errorf("%s ist trotz Ablehnung entstanden", rel)
		}
	}
	if pathExists(filepath.Join(LocalDir(root), "x.md")) {
		t.Error("Ausbruch hat geschrieben")
	}
}

func with(doc KnowledgeDocument, change func(*KnowledgeDocument)) KnowledgeDocument {
	change(&doc)
	return doc
}

// Werte, die YAML anders läse — ein Doppelpunkt mit Leerzeichen, ein
// führender Stern, eine Zahl, „true" —, werden zitiert und kommen über den
// Index zeichengleich zurück; schlichte Werte bleiben schlicht.
func TestKnowledgeWriteZitiertHeikleWerte(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)
	fixUpdated(t, "2026-09-12")

	doc := sessionDoc("findings/heikel.md")
	doc.Title = "Release: was der Wächter prüft"
	doc.Subject = "*Sterne* und #Rauten"
	doc.Origin = "2026"
	if _, err := knowledge.Write("session", doc, ""); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := readKnowledgeFile(t, root, "findings/heikel.md")
	if !strings.Contains(got, "title: \"Release: was der Wächter prüft\"\n") || !strings.Contains(got, "subject: \"*Sterne* und #Rauten\"\n") || !strings.Contains(got, "origin: \"2026\"\n") {
		t.Errorf("Zitat fehlt:\n%s", got)
	}
	parsed := parseKnowledgeFrontmatter([]byte(got))
	if parsed.Title != doc.Title {
		t.Errorf("Titel kommt als %q zurück", parsed.Title)
	}
	for _, value := range []string{"Sitzung 42", "Ablauf", "k-playbook v0.7.0", "ä ö ü ß"} {
		if yamlScalar(value) != value {
			t.Errorf("%q wurde zitiert: %s", value, yamlScalar(value))
		}
	}
	for _, value := range []string{"", " x", "x ", "- a", "a: b", "a #b", "true", "null", "3.5", "[a]", "\"q\"", "a:"} {
		if yamlScalar(value) == value {
			t.Errorf("%q wurde nicht zitiert", value)
		}
	}
}

// Ein führender Thematic Break ist kein Kopf: der Rumpf bleibt, wie er
// übergeben wurde, hinter dem erzeugten Frontmatter — und list und search
// sehen dasselbe Dokument.
func TestKnowledgeWriteFuehrenderThematicBreak(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)
	fixUpdated(t, "2026-09-12")

	doc := sessionDoc("findings/bruch.md")
	doc.Body = "---\n\n# Titel\n\nAbsatz mit Kennwort.\n\n---\n\nRest nach dem zweiten Strich.\n"
	rel, err := knowledge.Write("session", doc, "")
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := readKnowledgeFile(t, root, rel)
	if !strings.HasSuffix(got, "---\n\n"+doc.Body) {
		t.Errorf("Rumpf verändert:\n%q", got)
	}
	body := string(inventory.Body([]byte(got)))
	if !strings.Contains(body, "# Titel") || !strings.Contains(body, "Absatz mit Kennwort.") {
		t.Errorf("Rumpf hat Titel oder Absatz verloren: %q", body)
	}
	entries, err := knowledge.List(KnowledgeFilter{Kind: "findings"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	title := ""
	for _, entry := range entries {
		if entry.Path == rel {
			title = entry.Title
		}
	}
	// List führt das Dokument unter seinem Frontmatter-Titel; die Suche
	// trifft den Abschnitt unter der Überschrift im Rumpf — beide sehen
	// dieselbe Datei.
	if title != doc.Title {
		t.Errorf("List meldet Titel %q für %s, erwartet %q", title, rel, doc.Title)
	}
	hits, err := knowledge.Search("Kennwort", KnowledgeFilter{}, 0)
	if err != nil || len(hits) != 1 || hits[0].Path != rel || hits[0].Heading != "Titel" {
		t.Errorf("Search: %+v, %v — erwartet einen Treffer in %s unter %q", hits, err, rel, "Titel")
	}
}

// Write meldet den Ort, den es selbst berechnet hat, und genau dieser Ort ist
// der, unter dem Read, List und Search die Datei kennen.
func TestKnowledgeWriteMeldetDenEigenenPfad(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)

	for _, tc := range []struct{ path, want string }{
		{"findings/notiz-neu.md", "findings/notiz-neu.md"},
		{"./findings/notiz-neu.md", "findings/notiz-neu.md"},
		{"findings/tief/unten/../notiz.md", "findings/tief/notiz.md"},
	} {
		rel, err := knowledge.Write("session", sessionDoc(tc.path), "")
		if err != nil {
			t.Fatalf("Write(%q): %v", tc.path, err)
		}
		if rel != tc.want {
			t.Errorf("Write(%q) = %q, erwartet %q", tc.path, rel, tc.want)
		}
		if _, err := knowledge.Read(rel); err != nil {
			t.Errorf("Read(%q): %v", rel, err)
		}
		entries, err := knowledge.List(KnowledgeFilter{Kind: "findings"})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		found := false
		for _, entry := range entries {
			found = found || entry.Path == rel
		}
		if !found {
			t.Errorf("List kennt %q nicht: %+v", rel, entries)
		}
	}
}

// docs-index schreibt seine README über denselben Weg wie jeder andere: mit
// allen Pflichtfeldern, an genau eine Stelle.
func TestKnowledgeWriteDocsIndexReadme(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)
	fixUpdated(t, "2026-09-12")

	doc := KnowledgeDocument{
		Path: "README.md", Title: "Index", Subject: "Navigation", Origin: "/k-docs-index 2026-09-12",
		State: KnowledgeStateCondensed, Body: "# Projektwissen\n\n## Übersicht\n\n- code/links.md\n",
	}
	rel, err := knowledge.Write("docs-index", doc, "")
	if err != nil || rel != "README.md" {
		t.Fatalf("Write: %q, %v", rel, err)
	}
	if got := readKnowledgeFile(t, root, "README.md"); !strings.HasPrefix(got, "---\ntitle: Index\nsubject: Navigation\norigin: /k-docs-index 2026-09-12\nstate: condensed\n") {
		t.Errorf("README: %q", got)
	}
	if _, err := knowledge.Write("docs-index", with(doc, func(d *KnowledgeDocument) { d.Path = "manual/README.md" }), ""); err == nil {
		t.Error("docs-index durfte außerhalb der Wurzel schreiben")
	}
}

// Der Queue-Eintrag fällt, nachdem das Dokument steht — und nur dann. Schlägt
// das Schreiben fehl, liegt er noch da; ein genannter Eintrag, den es nicht
// gibt, ist ein Fehler vor dem Schreiben.
func TestKnowledgeWriteLoeschtQueueEintragNurBeiErfolg(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)
	queue := QueueDir(root)
	if err := os.MkdirAll(queue, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(queue, "20260912-chat.md")
	if err := os.WriteFile(entry, []byte("---\norigin: chat/x.md\ntarget: findings/\nreason: Befund\nadded: 2026-09-12\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Fehlschlag vor dem Schreiben: fremdes Verzeichnis, Kopf im Rumpf.
	for name, doc := range map[string]KnowledgeDocument{
		"fremdes Verzeichnis": sessionDoc("manual/x.md"),
		"Kopf im body":        with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.Body = "---\na: b\n---\n# X\n" }),
	} {
		if _, err := knowledge.Write("session", doc, "20260912-chat"); err == nil {
			t.Fatalf("%s: angenommen", name)
		}
		if !fileExists(entry) {
			t.Fatalf("%s: Queue-Eintrag trotz Fehlschlag gelöscht", name)
		}
	}

	// Fehlschlag beim Schreiben selbst: das Zielverzeichnis ist gesperrt.
	findings := filepath.Join(KnowledgeDir(root), "findings")
	denyWrite(t, findings)
	if _, err := knowledge.Write("session", sessionDoc("findings/gesperrt.md"), "20260912-chat"); err == nil {
		t.Fatal("Schreiben in ein gesperrtes Verzeichnis gelang")
	}
	if !fileExists(entry) {
		t.Fatal("Queue-Eintrag trotz gescheitertem Schreiben gelöscht")
	}
	if err := os.Chmod(findings, 0o755); err != nil {
		t.Fatal(err)
	}

	// Unbekannter Eintrag: Fehler, und kein Dokument.
	if _, err := knowledge.Write("session", sessionDoc("findings/x.md"), "gibt-es-nicht"); err == nil || !strings.Contains(err.Error(), "gibt es nicht") {
		t.Fatalf("unbekannter Eintrag: %v", err)
	}
	if pathExists(filepath.Join(findings, "x.md")) {
		t.Fatal("Dokument trotz unbekanntem Queue-Eintrag geschrieben")
	}
	for _, id := range []string{"../x", "a/b", ".versteckt", "README", "x.md"} {
		if _, err := knowledge.Write("session", sessionDoc("findings/x.md"), id); err == nil {
			t.Errorf("Queue-Kennung %q angenommen", id)
		}
	}

	// Erfolg: Dokument steht, Eintrag ist weg.
	if _, err := knowledge.Write("session", sessionDoc("findings/x.md"), "20260912-chat"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if fileExists(entry) {
		t.Error("Queue-Eintrag nach Erfolg noch da")
	}
	if !fileExists(filepath.Join(findings, "x.md")) {
		t.Error("Dokument fehlt")
	}
}

// Entscheidung 7 aus Task 063: write überschreibt kein Dokument mit state
// superseded. Sonst setzte ein späterer Lauf desselben Erzeugers die Ablösung
// still zurück und verlöre successor. Datei, Index und Queue-Eintrag bleiben.
func TestKnowledgeWriteWeistAbgeloestesDokumentAb(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)
	if _, err := knowledge.Write("session", sessionDoc("findings/neu.md"), ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := knowledge.Supersede("findings/notiz.md", "findings/neu.md", "Ersetzt"); err != nil {
		t.Fatal(err)
	}
	before := readKnowledgeFile(t, root, "findings/notiz.md")
	if _, err := knowledge.Status(); err != nil {
		t.Fatal(err)
	}
	indexBefore, err := os.ReadFile(KnowledgeIndexFile(root))
	if err != nil {
		t.Fatal(err)
	}
	id, err := knowledge.QueueAdd("chat/x.md", "findings/", "Nachtrag")
	if err != nil {
		t.Fatal(err)
	}

	again := with(sessionDoc("findings/notiz.md"), func(d *KnowledgeDocument) { d.Body = "# Gelernt\n\nJetzt doch sichern.\n" })
	_, err = knowledge.Write("session", again, id)
	if err == nil {
		t.Fatal("write auf ein abgelöstes Dokument angenommen")
	}
	if !IsInputError(err) || !strings.Contains(err.Error(), "abgelöst") || !strings.Contains(err.Error(), "findings/neu.md") {
		t.Errorf("Meldung/Klasse: %v", err)
	}
	if got := readKnowledgeFile(t, root, "findings/notiz.md"); got != before {
		t.Errorf("abgelöstes Dokument überschrieben:\n%s", got)
	}
	if indexAfter, err := os.ReadFile(KnowledgeIndexFile(root)); err != nil || string(indexAfter) != string(indexBefore) {
		t.Errorf("Index verändert: %v", err)
	}
	if entries, err := knowledge.QueueList(); err != nil || len(entries) != 1 {
		t.Errorf("Queue-Eintrag nicht mehr da: %+v, %v", entries, err)
	}
	if hits, err := knowledge.Search("sichern", KnowledgeFilter{}, 0); err != nil || len(hits) != 0 {
		t.Errorf("abgelöstes Dokument wieder in der Suche: %+v, %v", hits, err)
	}
}

// bodyCarriesFrontmatter schneidet wie composeKnowledgeFile auch führende
// Zeilenumbrüche ab (Task 063, Etappe 4): ein Rumpf mit Leerzeile und Kopf
// wird über write und publish abgewiesen, statt als zweiter Kopfblock in der
// Datei zu landen.
func TestKnowledgeRumpfMitLeerzeileUndKopfWirdAbgewiesen(t *testing.T) {
	for name, body := range map[string]string{
		"Leerzeile":               "\n---\ntitle: Fremd\n---\n\n# X\n",
		"Leerraum und Leerzeilen": " \n\t\n---\ntitle: Fremd\n---\n# X\n",
		"CRLF":                    "\r\n---\r\ntitle: Fremd\r\n---\r\n# X\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := knowledgeFixture(t)
			knowledge := NewKnowledge(root)
			_, err := knowledge.Write("session", with(sessionDoc("findings/x.md"), func(d *KnowledgeDocument) { d.Body = body }), "")
			if err == nil || !IsInputError(err) || !strings.Contains(err.Error(), "Dateikopf") {
				t.Errorf("write: %v", err)
			}
			if pathExists(filepath.Join(KnowledgeDir(root), "findings", "x.md")) {
				t.Error("write hat trotz Kopf geschrieben")
			}
			_, err = knowledge.Publish("docs-code", []KnowledgeDocument{codeDoc("x.md", body)})
			if err == nil || !IsInputError(err) || !strings.Contains(err.Error(), "Dateikopf") {
				t.Errorf("publish: %v", err)
			}
			if pathExists(filepath.Join(KnowledgeDir(root), "code", "x.md")) || !pathExists(filepath.Join(KnowledgeDir(root), "code", "links.md")) {
				t.Error("publish hat trotz Kopf getauscht")
			}
		})
	}
}

// write über ein symbolisch verlinktes Verzeichnis unter knowledge/ ist ein
// Eingabefehler (Task 064, Entscheidung 3): scanKnowledgeTree steigt dort
// nicht ab, eine Datei darin sähe der Index nie. Geprüft wird vor MkdirAll —
// im Linkziel entsteht weder die Datei noch ein Zwischenverzeichnis, und der
// Index bleibt unverändert.
func TestKnowledgeWriteUeberVerlinktesVerzeichnis(t *testing.T) {
	root := knowledgeFixture(t)
	outside := filepath.Join(t.TempDir(), "extern")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(KnowledgeDir(root), "findings", "link")); err != nil {
		t.Skipf("symbolische Verknüpfung nicht anlegbar: %v", err)
	}
	knowledge := NewKnowledge(root)
	if _, err := knowledge.Status(); err != nil {
		t.Fatal(err)
	}
	indexBefore, err := os.ReadFile(KnowledgeIndexFile(root))
	if err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"findings/link/x.md", "findings/link/neu/x.md"} {
		_, err := knowledge.Write("session", sessionDoc(rel), "")
		if err == nil {
			t.Errorf("%s: write über ein verlinktes Verzeichnis angenommen", rel)
		} else if !IsInputError(err) || !strings.Contains(err.Error(), "verlinktes Verzeichnis") {
			t.Errorf("%s: Meldung/Klasse: %v", rel, err)
		}
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("im Linkziel entstanden: %v", entries)
	}
	if indexAfter, err := os.ReadFile(KnowledgeIndexFile(root)); err != nil || string(indexAfter) != string(indexBefore) {
		t.Errorf("Index verändert: %v", err)
	}
}
