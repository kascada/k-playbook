package project

import (
	"os"
	"path/filepath"
	"testing"
)

// docsFixture legt ein Projekt mit Doku an und gibt dessen Hauptverzeichnis
// zurück.
func docsFixture(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(DocsDir(root), filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("%s anlegen: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("%s schreiben: %v", name, err)
		}
	}
	return root
}

func TestListDocsNimmtTitelUndUnterverzeichnisse(t *testing.T) {
	root := docsFixture(t, map[string]string{
		"handbuch.md":    "# Handbuch\n\nText\n",
		"libs/django.md": "Vorspann\n\n# Django\n",
		"ohne-titel.md":  "nur Text\n",
		"notiz.txt":      "keine Doku\n",
	})

	docs, err := ListDocs(root)
	if err != nil {
		t.Fatalf("ListDocs: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("erwartet 3 Markdown-Dateien, bekommen %d: %+v", len(docs), docs)
	}

	titles := map[string]string{}
	for _, doc := range docs {
		titles[doc.Path] = doc.Title
	}
	if titles["handbuch.md"] != "Handbuch" {
		t.Errorf("Titel aus Überschrift: %q", titles["handbuch.md"])
	}
	if titles["libs/django.md"] != "Django" {
		t.Errorf("Unterverzeichnis fehlt oder falscher Titel: %+v", titles)
	}
	// Ohne Überschrift bleibt nur der Dateiname.
	if titles["ohne-titel.md"] != "ohne-titel" {
		t.Errorf("Rückfall auf den Dateinamen: %q", titles["ohne-titel.md"])
	}
}

// Die README ist der Einstieg und steht deshalb vor allem anderen, auch wenn
// sie alphabetisch später käme.
func TestListDocsStelltReadmeVoran(t *testing.T) {
	root := docsFixture(t, map[string]string{
		"README.md":   "# Übersicht\n",
		"commands.md": "# Commands\n",
	})

	docs, err := ListDocs(root)
	if err != nil {
		t.Fatalf("ListDocs: %v", err)
	}
	if len(docs) != 2 || docs[0].Path != "README.md" {
		t.Fatalf("README nicht vorn: %+v", docs)
	}
}

func TestListDocsMeldetFehlendesVerzeichnis(t *testing.T) {
	if _, err := ListDocs(t.TempDir()); err == nil {
		t.Fatal("fehlende Doku wird nicht gemeldet")
	}
}

func TestReadDocLiefertInhalt(t *testing.T) {
	root := docsFixture(t, map[string]string{"handbuch.md": "# Handbuch\n\nText\n"})

	doc, content, err := ReadDoc(root, "handbuch.md")
	if err != nil {
		t.Fatalf("ReadDoc: %v", err)
	}
	if doc.Title != "Handbuch" || doc.Path != "handbuch.md" {
		t.Errorf("unerwarteter Eintrag: %+v", doc)
	}
	if string(content) != "# Handbuch\n\nText\n" {
		t.Errorf("unerwarteter Inhalt: %q", content)
	}
}

// Der Pfad kommt aus dem Browser. Nichts davon darf aus dem Doku-Verzeichnis
// herausführen oder etwas anderes als Markdown lesen.
func TestReadDocWeistFremdePfadeAb(t *testing.T) {
	root := docsFixture(t, map[string]string{"handbuch.md": "# Handbuch\n"})
	if err := os.WriteFile(filepath.Join(root, "geheim.md"), []byte("# Geheim\n"), 0o644); err != nil {
		t.Fatalf("Nachbardatei anlegen: %v", err)
	}

	for _, rel := range []string{
		"",
		"../geheim.md",
		"../../etc/passwd",
		"libs/../../geheim.md",
		filepath.Join(root, "geheim.md"),
		"handbuch.txt",
	} {
		if _, _, err := ReadDoc(root, rel); err == nil {
			t.Errorf("%q wurde angenommen", rel)
		}
	}
}
