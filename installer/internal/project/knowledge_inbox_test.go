package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Der Eingang nimmt, was kommt: jedes Format, unter <quelle>/<name>, ohne
// Prüfung des Inhalts. Eine Notiz liegt als Sidecar daneben und erscheint in
// der Liste am Eintrag des Rohstücks, nie als eigener Eintrag. Ein belegter
// Name wird abgewiesen — der Eingang überschreibt nicht.
func TestKnowledgeInboxPutListRead(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateLocal(root); err != nil {
		t.Fatal(err)
	}
	knowledge := NewKnowledge(root)

	rel, err := knowledge.InboxPut("chat", "2026-09-12.md", []byte("# Mitschnitt\n\nKennwort.\n"), "Aus dem Standup")
	if err != nil || rel != "chat/2026-09-12.md" {
		t.Fatalf("InboxPut: %q, %v", rel, err)
	}
	if got, err := os.ReadFile(filepath.Join(InboxDir(root), "chat", "2026-09-12.md.note")); err != nil || string(got) != "Aus dem Standup\n" {
		t.Errorf("Notiz: %q, %v", got, err)
	}
	if rel, err := knowledge.InboxPut("scan", "2026/seite.pdf", []byte{0x25, 0x50, 0x44, 0x46}, ""); err != nil || rel != "scan/2026/seite.pdf" {
		t.Errorf("PDF: %q, %v", rel, err)
	}
	if pathExists(filepath.Join(InboxDir(root), "scan", "2026", "seite.pdf.note")) {
		t.Error("Notiz ohne Inhalt angelegt")
	}
	if _, err := knowledge.InboxPut("mail", "bericht.eml", []byte("Subject: x\n"), "  "); err != nil {
		t.Fatal(err)
	}
	if _, err := knowledge.InboxPut("chat", "2026-09-12.md", []byte("anders"), ""); err == nil || !strings.Contains(err.Error(), "liegt schon im Eingang") {
		t.Errorf("Überschreiben: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(InboxDir(root), "chat", "2026-09-12.md")); err != nil || string(got) != "# Mitschnitt\n\nKennwort.\n" {
		t.Errorf("Rohstück überschrieben: %q", got)
	}

	entries, err := knowledge.InboxList("")
	if err != nil {
		t.Fatalf("InboxList: %v", err)
	}
	paths := []string{}
	for _, entry := range entries {
		paths = append(paths, entry.Path+"|"+entry.Source+"|"+entry.Name+"|"+entry.Format)
	}
	want := "chat/2026-09-12.md|chat|2026-09-12.md|markdown,mail/bericht.eml|mail|bericht.eml|eml,scan/2026/seite.pdf|scan|2026/seite.pdf|pdf"
	if strings.Join(paths, ",") != want {
		t.Errorf("Liste = %v", paths)
	}
	if entries[0].Note != "Aus dem Standup" || entries[1].Note != "" || entries[0].Size != 24 || time.Since(entries[0].Modified) > time.Minute {
		t.Errorf("Eintrag = %+v", entries[0])
	}
	if entries, err := knowledge.InboxList("scan"); err != nil || len(entries) != 1 || entries[0].Path != "scan/2026/seite.pdf" {
		t.Errorf("Filter scan: %+v, %v", entries, err)
	}
	if entries, err := knowledge.InboxList("gibt-es-nicht"); err != nil || len(entries) != 0 {
		t.Errorf("unbekannte Quelle: %+v, %v", entries, err)
	}
	if _, err := knowledge.InboxList("a/b"); err == nil {
		t.Error("Quelle mit Trenner angenommen")
	}

	content, err := knowledge.InboxRead("chat/2026-09-12.md")
	if err != nil || content != "# Mitschnitt\n\nKennwort.\n" {
		t.Errorf("InboxRead: %q, %v", content, err)
	}
	if _, err := knowledge.InboxRead("scan/2026/seite.pdf"); err == nil || !strings.Contains(err.Error(), "kein Textformat") {
		t.Errorf("PDF gelesen: %v", err)
	}
	if _, err := knowledge.InboxRead("mail/bericht.eml"); err == nil {
		t.Error("eml gelesen")
	}
	for _, path := range []string{"", "chat", "../x.md", "chat/../../k-playbook.md", "/etc/passwd.txt", "chat/fehlt.md", "chat/.versteckt.md", "chat/2026-09-12.md.note"} {
		if _, err := knowledge.InboxRead(path); err == nil {
			t.Errorf("InboxRead(%q) angenommen", path)
		}
	}
}

// Quelle und Name dürfen nicht aus der Zone herausführen, und der Inhalt
// darf nicht leer sein. Nichts davon hinterlässt eine Datei.
func TestKnowledgeInboxPutWeistAb(t *testing.T) {
	root := t.TempDir()
	knowledge := NewKnowledge(root)

	for name, tc := range map[string][2]string{
		"leere Quelle":         {"", "x.md"},
		"Quelle mit Trenner":   {"a/b", "x.md"},
		"Quelle versteckt":     {".git", "x.md"},
		"Quelle ..":            {"..", "x.md"},
		"leerer Name":          {"chat", " "},
		"Name absolut":         {"chat", "/etc/x"},
		"Name führt heraus":    {"chat", "../../x.md"},
		"Name versteckt":       {"chat", "tief/.x"},
		"Name ist Notiz":       {"chat", "x.md.note"},
		"Name ist Verzeichnis": {"chat", "."},
	} {
		if rel, err := knowledge.InboxPut(tc[0], tc[1], []byte("x"), ""); err == nil {
			t.Errorf("%s angenommen als %q", name, rel)
		}
	}
	if _, err := knowledge.InboxPut("chat", "leer.md", nil, ""); err == nil {
		t.Error("leerer Inhalt angenommen")
	}
	if pathExists(InboxDir(root)) {
		t.Error("Eingang trotz Ablehnungen entstanden")
	}
	if entries, err := knowledge.InboxList(""); err != nil || len(entries) != 0 {
		t.Errorf("fehlender Eingang: %+v, %v", entries, err)
	}
}

// Ein Eintrag entsteht mit vergebener Kennung, die Liste nennt ihn mit den
// vier Feldern, drop löscht ihn — und hält den Grund nirgends fest.
func TestKnowledgeQueueAddListDrop(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateLocal(root); err != nil {
		t.Fatal(err)
	}
	knowledge := NewKnowledge(root)
	fixed := time.Date(2026, 9, 12, 11, 49, 18, 0, time.FixedZone("CEST", 2*3600))
	before := queueNow
	queueNow = func() time.Time { return fixed }
	t.Cleanup(func() { queueNow = before })

	id, err := knowledge.QueueAdd("chat/2026-09-12.md", "extracted", "Standup-Befund destillieren")
	if err != nil || id != "20260912-114918-chat-2026-09-12-md" {
		t.Fatalf("QueueAdd: %q, %v", id, err)
	}
	got, err := os.ReadFile(filepath.Join(QueueDir(root), id+".md"))
	if err != nil || string(got) != "---\norigin: chat/2026-09-12.md\ntarget: extracted/\nreason: Standup-Befund destillieren\nadded: 2026-09-12T11:49:18+02:00\n---\n" {
		t.Errorf("Eintrag: %q, %v", got, err)
	}

	// Dieselbe Herkunft zur selben Sekunde: die Kennung bekommt eine Nummer.
	second, err := knowledge.QueueAdd("chat/2026-09-12.md", "findings/", "Zweiter Blick")
	if err != nil || second != id+"-2" {
		t.Errorf("Kollision: %q, %v", second, err)
	}
	third, err := knowledge.QueueAdd("https://confluence.example/pages/12345?x=1", "external/confluence", "Seite holen")
	if err != nil || third != "20260912-114918-https-confluence-example-pages-12345-x-1" {
		t.Errorf("Adresse: %q, %v", third, err)
	}
	if id, err := knowledge.QueueAdd("Ärger & Ösen", "manual", "x"); err != nil || id != "20260912-114918-rger-sen" {
		t.Errorf("Umlaute: %q, %v", id, err)
	}
	if id, err := knowledge.QueueAdd("###", "manual", "x"); err != nil || id != "20260912-114918-eintrag" {
		t.Errorf("leerer Slug: %q, %v", id, err)
	}

	entries, err := knowledge.QueueList()
	if err != nil || len(entries) != 5 {
		t.Fatalf("QueueList: %+v, %v", entries, err)
	}
	first := entries[0]
	if first.ID != id || first.Origin != "chat/2026-09-12.md" || first.Target != "extracted/" || first.Reason != "Standup-Befund destillieren" || !first.Added.Equal(fixed) || first.Notes != "" {
		t.Errorf("erster Eintrag = %+v", first)
	}
	// Alphabetisch nach Kennung: chat…, chat…-2, eintrag, https…, rger-sen.
	if entries[3].ID != third || entries[3].Target != "external/confluence/" {
		t.Errorf("Ziel normalisiert: %+v", entries[3])
	}

	// Notizen im Rumpf kommen mit; eine Fremddatei wird übersprungen und gemeldet.
	if err := os.WriteFile(filepath.Join(QueueDir(root), id+".md"), append(got, "\nErster Lauf scheiterte: Quelle unlesbar.\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(QueueDir(root), "fremd.md"), []byte("# Kein Eintrag\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	knowledge = NewKnowledge(root)
	entries, err = knowledge.QueueList()
	if err != nil || len(entries) != 5 || entries[0].Notes != "Erster Lauf scheiterte: Quelle unlesbar." {
		t.Errorf("Notizen: %+v, %v", entries, err)
	}
	if notes := strings.Join(knowledge.Notes(), ";"); !strings.Contains(notes, "fremd.md") {
		t.Errorf("Fremddatei nicht gemeldet: %q", notes)
	}

	if err := knowledge.QueueDrop(id, "doch nicht nötig"); err != nil {
		t.Fatalf("QueueDrop: %v", err)
	}
	if pathExists(filepath.Join(QueueDir(root), id+".md")) {
		t.Error("Eintrag nach drop noch da")
	}
	if err := knowledge.QueueDrop(id, "noch einmal"); err == nil || !strings.Contains(err.Error(), "gibt es nicht") {
		t.Errorf("zweiter drop: %v", err)
	}
	for _, bad := range []string{"", "../x", "README", "fremd.md"} {
		if err := knowledge.QueueDrop(bad, "x"); err == nil {
			t.Errorf("drop(%q) angenommen", bad)
		}
	}
	if !pathExists(filepath.Join(QueueDir(root), "README.md")) || !pathExists(filepath.Join(QueueDir(root), "fremd.md")) {
		t.Error("drop hat eine fremde Datei gelöscht")
	}
}

// Die Prüfung der Argumente: origin und reason einzeilig und nicht leer,
// target relativ zu knowledge/ und darunter — nicht die Wurzel, kein
// Ausbruch, nicht versteckt. Die Erzeugertabelle wird nicht geprüft.
func TestKnowledgeQueueAddWeistAb(t *testing.T) {
	root := t.TempDir()
	knowledge := NewKnowledge(root)

	for name, tc := range map[string][3]string{
		"leerer origin":       {"", "extracted", "x"},
		"origin mehrzeilig":   {"a\nb", "extracted", "x"},
		"leeres target":       {"chat/x.md", " ", "x"},
		"target Wurzel":       {"chat/x.md", ".", "x"},
		"target führt heraus": {"chat/x.md", "../docs", "x"},
		"target absolut":      {"chat/x.md", "/tmp", "x"},
		"target versteckt":    {"chat/x.md", ".git", "x"},
		"leerer reason":       {"chat/x.md", "extracted", " "},
	} {
		if id, err := knowledge.QueueAdd(tc[0], tc[1], tc[2]); err == nil {
			t.Errorf("%s angenommen als %q", name, id)
		}
	}
	if pathExists(QueueDir(root)) {
		t.Error("Warteschlange trotz Ablehnungen entstanden")
	}
	if entries, err := knowledge.QueueList(); err != nil || len(entries) != 0 {
		t.Errorf("fehlende Warteschlange: %+v, %v", entries, err)
	}
	// Ein Ziel außerhalb der Erzeugertabelle geht durch: geprüft wird nur der Pfad.
	if _, err := knowledge.QueueAdd("chat/x.md", "irgendwo/tief", "x"); err != nil {
		t.Errorf("freies Ziel abgewiesen: %v", err)
	}
}

// Liegt am Quellpfad eine Datei — die README aus CreateLocal als Quelle, eine
// abgelegte Datei als Zwischenverzeichnis im Namen —, ist das ein Eingabefehler
// mit sprechender Meldung, bevor irgendetwas angelegt wird (Task 063, Etappe 5).
func TestKnowledgeInboxPutQuellpfadIstDatei(t *testing.T) {
	root := t.TempDir()
	knowledge := NewKnowledge(root)
	readme := filepath.Join(InboxDir(root), "README.md")
	if err := os.MkdirAll(filepath.Dir(readme), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readme, []byte("# inbox\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := knowledge.InboxPut("chat", "a.md", []byte("x"), ""); err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string][2]string{
		"Quelle ist Datei":       {"README.md", "x.md"},
		"Name führt durch Datei": {"chat", "a.md/b.md"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := knowledge.InboxPut(tc[0], tc[1], []byte("y"), "Notiz")
			if err == nil {
				t.Fatal("angenommen")
			}
			if !IsInputError(err) || !strings.Contains(err.Error(), "Datei") {
				t.Errorf("Meldung/Klasse: %v", err)
			}
		})
	}
	if content, err := os.ReadFile(readme); err != nil || string(content) != "# inbox\n" {
		t.Errorf("README verändert: %q, %v", content, err)
	}
	if content, err := os.ReadFile(filepath.Join(InboxDir(root), "chat", "a.md")); err != nil || string(content) != "x" {
		t.Errorf("chat/a.md verändert: %q, %v", content, err)
	}
}

// .markdown ist Text: inbox_list nennt es markdown, inbox_read liefert es.
func TestKnowledgeInboxReadLiestMarkdownEndung(t *testing.T) {
	root := t.TempDir()
	knowledge := NewKnowledge(root)
	if _, err := knowledge.InboxPut("chat", "notiz.markdown", []byte("# Notiz\n"), ""); err != nil {
		t.Fatal(err)
	}
	content, err := knowledge.InboxRead("chat/notiz.markdown")
	if err != nil || content != "# Notiz\n" {
		t.Errorf("InboxRead: %q, %v", content, err)
	}

	// Auch die Liste führt .markdown mit dem Format aus inboxFormats (Task 064,
	// Etappe 6). Weil inboxFormat ohne Zuordnung die Endung selbst nennt — hier
	// ebenfalls „markdown" —, prüft erst der Abgleich mit der Zuordnung, dass
	// sie besteht und dieselbe ist wie für .md.
	if format := inboxFormats["markdown"]; format == "" || format != inboxFormats["md"] {
		t.Fatalf("inboxFormats ordnet markdown nicht wie md zu: %q", format)
	}
	entries, err := knowledge.InboxList("chat")
	if err != nil || len(entries) != 1 || entries[0].Path != "chat/notiz.markdown" || entries[0].Format != inboxFormats["markdown"] {
		t.Errorf("InboxList: %+v, %v", entries, err)
	}
}
