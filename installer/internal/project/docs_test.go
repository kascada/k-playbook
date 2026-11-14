package project

import (
	"errors"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/kephalaion/kephtest"
)

// handbookDefaults baut die öffentliche Collection im Speicher mit den
// gegebenen Dateien unter defaults/ — VERSION steht immer darin, wie nach
// einem Release — und gibt das Handbuch von k-playbook darüber zurück.
func handbookDefaults(t *testing.T, files map[string]string) Handbook {
	t.Helper()
	store := kephtest.New()
	store.Put("defaults/VERSION", "v0.0.0-test\n")
	for name, content := range files {
		store.Put("defaults/"+name, content)
	}
	source := NewKephDefaultsSource(func() (DefaultsStore, string, error) { return store, testDefaultsAddress, nil })
	return KPlaybookHandbookFrom(source, testDefaultsAddress)
}

// handbookCodeOf ist der Code eines HandbookError, leer bei anderen Fehlern.
func handbookCodeOf(err error) string {
	var handbookErr *HandbookError
	if errors.As(err, &handbookErr) {
		return handbookErr.Code
	}
	return ""
}

// Das Handbuch von k-playbook: alle Seiten unter docs/handbook/, auch in
// Unterverzeichnissen, mit dem Titel aus der ersten Überschrift nach dem
// Frontmatter; overview.md steht vorn, der Rest alphabetisch. Andere Dateien
// und die übrigen Vorlagen unter docs/ gehören nicht dazu.
func TestKPlaybookHandbookListe(t *testing.T) {
	handbook := handbookDefaults(t, map[string]string{
		"docs/handbook/manual.md":      "---\ntitle: Manual\n---\n\n# Manual\n\nText\n",
		"docs/handbook/overview.md":    "# Overview\n",
		"docs/handbook/libs/django.md": "Vorspann\n\n# Django\n",
		"docs/handbook/ohne-titel.md":  "nur Text\n",
		"docs/handbook/notiz.txt":      "keine Doku\n",
		"docs/manual/README.md":        "# Vorlage\n",
	})

	docs, err := handbook.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	paths := []string{}
	titles := map[string]string{}
	for _, doc := range docs {
		paths = append(paths, doc.Path)
		titles[doc.Path] = doc.Title
	}
	if strings.Join(paths, " ") != "overview.md libs/django.md manual.md ohne-titel.md" {
		t.Fatalf("Seiten: %v", paths)
	}
	if titles["manual.md"] != "Manual" || titles["libs/django.md"] != "Django" || titles["ohne-titel.md"] != "ohne-titel" {
		t.Errorf("Titel: %v", titles)
	}
}

// Eine Seite kommt als Rohtext; der Pfad bleibt relativ zum Handbuch.
func TestKPlaybookHandbookSeite(t *testing.T) {
	handbook := handbookDefaults(t, map[string]string{"docs/handbook/manual.md": "# Manual\n\nText\n"})

	doc, content, err := handbook.Read("manual.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if doc.Title != "Manual" || doc.Path != "manual.md" || string(content) != "# Manual\n\nText\n" {
		t.Errorf("Seite: %+v %q", doc, content)
	}
	if _, _, err := handbook.Read("fehlt.md"); err == nil || handbookCodeOf(err) != "" {
		t.Errorf("fehlende Seite: %v", err)
	}
}

// Ein Lieferumfang ohne docs/handbook/ — ein älteres Release auf dem Hub —
// ist handbook_missing mit der Abhilfe fetch_release, für die Liste wie für
// eine Seite.
func TestKPlaybookHandbookLieferumfangOhneHandbuch(t *testing.T) {
	handbook := handbookDefaults(t, map[string]string{"docs/manual/README.md": "# Vorlage\n"})

	_, err := handbook.List()
	if handbookCodeOf(err) != HandbookCodeMissing || !strings.Contains(err.Error(), "fetch_release") ||
		!strings.Contains(err.Error(), testDefaultsAddress) {
		t.Fatalf("List: %v", err)
	}
	if _, _, err := handbook.Read("overview.md"); handbookCodeOf(err) != HandbookCodeMissing {
		t.Errorf("Read: %v", err)
	}
}

// Ohne Kephalaion gibt es kein Handbuch: kephalaion_unavailable, kein
// Rückfall auf die Platte.
func TestKPlaybookHandbookOhneKephalaion(t *testing.T) {
	source := NewKephDefaultsSource(func() (DefaultsStore, string, error) {
		return nil, testDefaultsAddress, errors.New("Kephalaion ist für dieses Projekt nicht bereit")
	})
	handbook := KPlaybookHandbookFrom(source, testDefaultsAddress)

	_, err := handbook.List()
	if handbookCodeOf(err) != DocCodeKephalaionUnavailable || !strings.Contains(err.Error(), "nicht erreichbar") {
		t.Fatalf("List: %v", err)
	}
	if _, _, err := handbook.Read("overview.md"); handbookCodeOf(err) != DocCodeKephalaionUnavailable {
		t.Errorf("Read: %v", err)
	}
}

// Der Pfad kommt aus dem Browser. Nichts davon darf aus dem Handbuch
// herausführen oder etwas anderes als Markdown lesen.
func TestHandbookWeistFremdePfadeAb(t *testing.T) {
	handbook := handbookDefaults(t, map[string]string{
		"docs/handbook/manual.md": "# Manual\n",
		"docs/geheim.md":          "# Geheim\n",
		"instructions/x.md":       "# X\n",
	})

	for _, rel := range []string{
		"",
		"../geheim.md",
		"../../instructions/x.md",
		"libs/../../geheim.md",
		"/docs/handbook/manual.md",
		"..\\geheim.md",
		"handbuch.txt",
	} {
		if _, _, err := handbook.Read(rel); err == nil {
			t.Errorf("%q wurde angenommen", rel)
		}
	}
}

// handbookProject baut die Collection des Projekts im Speicher mit den
// gegebenen Dokumenten und gibt das Handbuch des Projekts darüber zurück.
func handbookProject(t *testing.T, docs map[string]string) Handbook {
	t.Helper()
	store := kephtest.New()
	for name, content := range docs {
		store.Put(name, content)
	}
	source := NewKephProjectHandbookSource(func() (DefaultsStore, string, error) { return store, testProjectAddress, nil })
	return ProjectHandbookFrom(source, testProjectAddress)
}

// Das Handbuch des Projekts: alle Seiten unter keph:/docs/manual/handbook/,
// README.md vorn — sie wird immer gezeigt, auch als unveränderte Vorlage.
// Die übrige Handdoku gehört nicht dazu.
func TestProjectHandbookMitReadme(t *testing.T) {
	handbook := handbookProject(t, map[string]string{
		"docs/manual/handbook/README.md":      "---\ntitle: \"Project handbook\"\n---\n\n# Project handbook\n",
		"docs/manual/handbook/betrieb.md":     "# Betrieb\n",
		"docs/manual/handbook/anfang/erst.md": "# Erste Schritte\n",
		"docs/manual/entscheidung.md":         "# Entscheidung\n",
	})

	docs, err := handbook.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	paths := []string{}
	for _, doc := range docs {
		paths = append(paths, doc.Path)
	}
	if strings.Join(paths, " ") != "README.md anfang/erst.md betrieb.md" || docs[0].Title != "Project handbook" {
		t.Fatalf("Seiten: %+v", docs)
	}
	doc, content, err := handbook.Read("anfang/erst.md")
	if err != nil || doc.Title != "Erste Schritte" || string(content) != "# Erste Schritte\n" {
		t.Errorf("Read: %+v %q %v", doc, content, err)
	}
	if _, _, err := handbook.Read("../entscheidung.md"); err == nil {
		t.Error("../entscheidung.md wurde angenommen")
	}
}

// Ohne keph:/docs/manual/handbook/ gibt es kein Handbuch des Projekts:
// handbook_missing mit dem Weg zur Startseite, auch wenn die übrige
// Handdoku da ist.
func TestProjectHandbookOhneVerzeichnis(t *testing.T) {
	handbook := handbookProject(t, map[string]string{"docs/manual/README.md": "# Handdoku\n"})

	_, err := handbook.List()
	if handbookCodeOf(err) != HandbookCodeMissing || !strings.Contains(err.Error(), "keph:/docs/manual/handbook/") ||
		!strings.Contains(err.Error(), "k-playbook docs readme manual/handbook") {
		t.Fatalf("List: %v", err)
	}
	if _, _, err := handbook.Read("README.md"); handbookCodeOf(err) != HandbookCodeMissing {
		t.Errorf("Read: %v", err)
	}
}

// Ohne Kephalaion gibt es kein Handbuch des Projekts: kephalaion_unavailable
// mit der Abhilfe, kein Rückfall.
func TestProjectHandbookOhneKephalaion(t *testing.T) {
	source := NewKephProjectHandbookSource(func() (DefaultsStore, string, error) {
		return nil, testProjectAddress, errors.New("Kephalaion ist für dieses Projekt nicht bereit")
	})
	handbook := ProjectHandbookFrom(source, testProjectAddress)

	_, err := handbook.List()
	if handbookCodeOf(err) != DocCodeKephalaionUnavailable || !strings.Contains(err.Error(), "nicht bereit") ||
		!strings.Contains(err.Error(), "Abhilfe") {
		t.Fatalf("List: %v", err)
	}
	if _, _, err := handbook.Read("README.md"); handbookCodeOf(err) != DocCodeKephalaionUnavailable {
		t.Errorf("Read: %v", err)
	}
}

// Die Vorlagen im Repo: manual/handbook trägt eine README und ist damit eine
// Vorlage, das Handbuch von k-playbook unter docs/handbook/ nicht — sonst
// schriebe k-playbook eine keph:/docs/handbook/README.md in jedes Projekt.
// Die Seiten des Handbuchs sind über k-playbook:/docs/handbook/ lesbar.
func TestDefaultsHandbuchIstKeineVorlage(t *testing.T) {
	dist := repoDefaultsSource(t)
	dirs, err := ListDocsTemplates(dist)
	if err != nil {
		t.Fatal(err)
	}
	joined := " " + strings.Join(dirs, " ") + " "
	if !strings.Contains(joined, " manual/handbook ") || strings.Contains(joined, " handbook ") {
		t.Errorf("Vorlagen: %v", dirs)
	}
	docs, err := KPlaybookHandbookFrom(dist, testDefaultsAddress).List()
	if err != nil || len(docs) == 0 || docs[0].Path != KPlaybookHandbookStart {
		t.Fatalf("Handbuch des Repos: %+v %v", docs, err)
	}
	for _, doc := range docs {
		if doc.Path == "README.md" {
			t.Errorf("defaults/docs/handbook/ trägt eine README.md")
		}
	}
	got := mustResolveDoc(t, repoDefaultsLayers(t), "k-playbook:/docs/handbook/"+KPlaybookHandbookStart)
	if !strings.Contains(got.Content, "# ") {
		t.Errorf("k-playbook:/docs/handbook/%s: %q", KPlaybookHandbookStart, got.Content)
	}
}
