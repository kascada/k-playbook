package project

import (
	"path/filepath"
	"testing"
)

// Die Prüfungen der Schreibseite liefern einen erkennbaren Eingabefehler —
// auch „nicht vorhanden": ein fehlender Pfad bei read und supersede, ein
// fehlender Nachfolger, ein unbekannter Queue-Eintrag. Ein vorhandener, aber
// unlesbarer Pfad ist keiner: das ist die Umgebung. Daran ordnen die Hüllen
// invalid_input, write_failed und read_failed zu.
func TestKnowledgeEingabefehlerSindErkennbar(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)
	good := sessionDoc("findings/neu.md")

	inputs := map[string]func() error{
		"unbekannter Erzeuger": func() error { _, err := knowledge.Write("gate", good, ""); return err },
		"Pfad aus der Zone": func() error {
			_, err := knowledge.Write("session", with(good, func(d *KnowledgeDocument) { d.Path = "../x.md" }), "")
			return err
		},
		"fremdes Verzeichnis": func() error {
			_, err := knowledge.Write("session", with(good, func(d *KnowledgeDocument) { d.Path = "manual/x.md" }), "")
			return err
		},
		"Generator bei write": func() error {
			_, err := knowledge.Write("docs-code", with(good, func(d *KnowledgeDocument) { d.Path = "code/x.md" }), "")
			return err
		},
		"fehlender title": func() error {
			_, err := knowledge.Write("session", with(good, func(d *KnowledgeDocument) { d.Title = " " }), "")
			return err
		},
		"state superseded": func() error {
			_, err := knowledge.Write("session", with(good, func(d *KnowledgeDocument) { d.State = "superseded" }), "")
			return err
		},
		"unbekanntes format": func() error {
			_, err := knowledge.Write("session", with(good, func(d *KnowledgeDocument) { d.Format = "docx" }), "")
			return err
		},
		"Kopf im Rumpf": func() error {
			_, err := knowledge.Write("session", with(good, func(d *KnowledgeDocument) { d.Body = "---\ntitle: X\n---\n# X\n" }), "")
			return err
		},
		"unbekannter Queue-Eintrag": func() error { _, err := knowledge.Write("session", good, "gibt-es-nicht"); return err },
		"leerer Satz":               func() error { _, err := knowledge.Publish("docs-code", nil); return err },
		"kein Generator bei publish": func() error {
			_, err := knowledge.Publish("session", []KnowledgeDocument{codeDoc("x.md", "# X\n")})
			return err
		},
		"doppelter Pfad im Satz": func() error {
			_, err := knowledge.Publish("docs-code", []KnowledgeDocument{codeDoc("x.md", "# X\n"), codeDoc("./x.md", "# X\n")})
			return err
		},
		"ungültiges Dokument im Satz": func() error {
			_, err := knowledge.Publish("docs-code", []KnowledgeDocument{with(codeDoc("x.md", "# X\n"), func(d *KnowledgeDocument) { d.State = "superseded" })})
			return err
		},
		"fehlender Pfad bei read":    func() error { _, err := knowledge.Read("manual/fehlt.md"); return err },
		"Pfad aus der Zone bei read": func() error { _, err := knowledge.Read("../geheim.md"); return err },
		"fehlendes Dokument bei supersede": func() error {
			_, _, err := knowledge.Supersede("manual/fehlt.md", "manual/release.md", "x")
			return err
		},
		"fehlender Nachfolger bei supersede": func() error {
			_, _, err := knowledge.Supersede("manual/release.md", "manual/fehlt.md", "x")
			return err
		},
		"eigener Nachfolger": func() error {
			_, _, err := knowledge.Supersede("manual/release.md", "manual/release.md", "x")
			return err
		},
		"leere Suchanfrage":                  func() error { _, err := knowledge.Search(" ", KnowledgeFilter{}, 0); return err },
		"unbekannter Queue-Eintrag bei drop": func() error { return knowledge.QueueDrop("gibt-es-nicht", "") },
		"ungültige Queue-Kennung":            func() error { return knowledge.QueueDrop("../x", "") },
		"target aus der Zone":                func() error { _, err := knowledge.QueueAdd("chat/x.md", "../x", "Grund"); return err },
		"fehlender reason":                   func() error { _, err := knowledge.QueueAdd("chat/x.md", "extracted/", " "); return err },
		"leerer Inhalt im Eingang":           func() error { _, err := knowledge.InboxPut("chat", "x.md", nil, ""); return err },
		"versteckter Name im Eingang":        func() error { _, err := knowledge.InboxPut("chat", ".x.md", []byte("x"), ""); return err },
		"fehlendes Rohstück":                 func() error { _, err := knowledge.InboxRead("chat/fehlt.md"); return err },
		"kein Textformat":                    func() error { _, err := knowledge.InboxRead("scan/bild.png"); return err },
		"ungültige Quelle bei inbox list":    func() error { _, err := knowledge.InboxList("a/b"); return err },
	}
	for name, call := range inputs {
		err := call()
		if err == nil {
			t.Errorf("%s: kein Fehler", name)
			continue
		}
		if !IsInputError(err) {
			t.Errorf("%s: kein Eingabefehler: %v", name, err)
		}
	}
	if IsInputError(nil) {
		t.Error("nil ist ein Eingabefehler")
	}

	// Belegter Name im Eingang: der Aufrufer wählt einen anderen — Eingabe.
	if _, err := knowledge.InboxPut("chat", "x.md", []byte("x"), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := knowledge.InboxPut("chat", "x.md", []byte("y"), ""); !IsInputError(err) {
		t.Errorf("belegter Name: %v", err)
	}

	// Vorhanden, aber unlesbar: die Umgebung, kein Eingabefehler.
	denyRead(t, filepath.Join(KnowledgeDir(root), "manual", "release.md"))
	if _, err := knowledge.Read("manual/release.md"); err == nil || IsInputError(err) {
		t.Errorf("unlesbare Datei als Eingabefehler gemeldet: %v", err)
	}
}
