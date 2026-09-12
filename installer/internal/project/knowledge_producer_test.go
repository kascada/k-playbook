package project

import (
	"strings"
	"testing"
)

// Jede Zeile der Erzeugertabelle aus docs/knowledge-layout.md, positiv: der
// Erzeuger darf in sein Verzeichnis, und das Ergebnis ist der bereinigte Pfad
// relativ zu knowledge/.
func TestProducerTabellePositiv(t *testing.T) {
	for _, tc := range []struct{ producer, path, want string }{
		{"docs-code", "code/links.md", "code/links.md"},
		{"docs-code", "code/tief/unten.md", "code/tief/unten.md"},
		{"docs-tools", "libs/goldmark.md", "libs/goldmark.md"},
		{"inventory", "versions/inventory.md", "versions/inventory.md"},
		{"docs-extract", "extracted/sitzung.md", "extracted/sitzung.md"},
		{"connector:confluence", "external/confluence/seite.md", "external/confluence/seite.md"},
		{"connector:jira", "external/jira/proj/ticket.md", "external/jira/proj/ticket.md"},
		{"session", "findings/befund.md", "findings/befund.md"},
		{"person", "manual/ablauf.md", "manual/ablauf.md"},
		{"person", "pitfalls/falle.md", "pitfalls/falle.md"},
		{"docs-index", "README.md", "README.md"},
		// Bereinigung: ./ und innere .. bleiben in der Zone.
		{"session", "./findings/x.md", "findings/x.md"},
		{"session", "findings/tief/../x.md", "findings/x.md"},
		{"docs-index", "./README.md", "README.md"},
		{" session ", "findings/x.md", "findings/x.md"},
	} {
		producer, rel, err := ResolveProducerPath(tc.producer, tc.path)
		if err != nil {
			t.Errorf("%s → %s: %v", tc.producer, tc.path, err)
			continue
		}
		if rel != tc.want || string(producer) != strings.TrimSpace(tc.producer) {
			t.Errorf("%s → %s = (%q, %q), erwartet %q", tc.producer, tc.path, producer, rel, tc.want)
		}
	}
}

// Jede Zeile negativ: das Ziel liegt in der Zone, aber in einem fremden
// Verzeichnis. Die Meldung ist die dritte der drei — sie nennt das
// Erzeugerverzeichnis, nicht die Zone und nicht den Erzeuger als unbekannt.
func TestProducerTabelleNegativFremdesVerzeichnis(t *testing.T) {
	for _, tc := range []struct{ producer, path string }{
		{"docs-code", "libs/goldmark.md"},
		{"docs-code", "README.md"},
		{"docs-tools", "code/links.md"},
		{"inventory", "manual/x.md"},
		{"docs-extract", "findings/x.md"},
		{"connector:confluence", "external/jira/seite.md"},
		{"connector:confluence", "external/seite.md"},
		{"connector:confluence", "extracted/seite.md"},
		{"session", "manual/x.md"},
		{"session", "pitfalls/x.md"},
		{"person", "findings/x.md"},
		{"person", "code/x.md"},
		{"docs-index", "manual/README.md"},
		{"docs-index", "index.md"},
		{"docs-index", "code/x.md"},
		// Das Verzeichnis selbst ist kein Ziel, und ein gleichnamiger Nachbar
		// auch nicht: „code-alt/" ist nicht „code/".
		{"docs-code", "code.md"},
		{"docs-code", "code-alt/x.md"},
		{"session", "findings.md"},
		// Eine flache Datei in der Wurzel gehört niemandem außer docs-index.
		{"person", "notiz.md"},
		{"session", "notiz.md"},
	} {
		_, rel, err := ResolveProducerPath(tc.producer, tc.path)
		if err == nil {
			t.Errorf("%s → %s angenommen als %q", tc.producer, tc.path, rel)
			continue
		}
		if !strings.Contains(err.Error(), "außerhalb des Erzeugerverzeichnisses") {
			t.Errorf("%s → %s: falsche Meldung %q", tc.producer, tc.path, err)
		}
	}
}

// Unbekannter Erzeuger: die erste der drei Meldungen, und sie nennt die
// bekannte Liste. Der Pfad spielt dabei keine Rolle — auch ein gültiger
// hilft einem unbekannten Erzeuger nicht.
func TestProducerUnbekannt(t *testing.T) {
	for _, producer := range []string{"", "  ", "gate", "Session", "connector", "connector:", "connector:a/b", "connector:..", "connector:.hidden", "connector: x", "learned"} {
		_, _, err := ResolveProducerPath(producer, "findings/x.md")
		if err == nil {
			t.Errorf("Erzeuger %q angenommen", producer)
			continue
		}
		if strings.Contains(err.Error(), "außerhalb des Erzeugerverzeichnisses") || strings.Contains(err.Error(), "führt aus") {
			t.Errorf("Erzeuger %q: falsche Meldung %q", producer, err)
		}
	}
	// "docs-code " mit Leerraum wird getrimmt und ist damit bekannt.
	if _, err := ParseProducer("docs-code "); err != nil {
		t.Errorf("Leerraum um den Namen nicht getrimmt: %v", err)
	}
	if _, err := ParseProducer("gate"); err == nil || !strings.Contains(err.Error(), "bekannt sind docs-code, docs-tools, inventory, docs-extract, connector:<system>, session, person, docs-index") {
		t.Errorf("Meldung ohne Liste: %v", err)
	}
}

// Ausbruch aus der Zone: die zweite Meldung. Sie greift vor der Zuordnung —
// ein Pfad, der die Zone verlässt, ist für jeden Erzeuger falsch, und die
// Meldung darf nicht so tun, als läge es am Erzeugerverzeichnis.
func TestProducerPfadAusbruch(t *testing.T) {
	for _, path := range []string{"", "  ", "../x.md", "../../k-playbook.md", "/etc/passwd.md", "findings/../../x.md", "..", ".", "findings/.versteckt.md", ".git/x.md", "findings/x.txt", "findings/x", "findings/", "code/x.MD.txt"} {
		_, rel, err := ResolveProducerPath("session", path)
		if err == nil {
			t.Errorf("Pfad %q angenommen als %q", path, rel)
			continue
		}
		if strings.Contains(err.Error(), "außerhalb des Erzeugerverzeichnisses") || strings.Contains(err.Error(), "unbekannter Erzeuger") {
			t.Errorf("Pfad %q: falsche Meldung %q", path, err)
		}
	}
}

// Die drei Generatoren, und nur sie.
func TestProducerGeneratoren(t *testing.T) {
	want := map[Producer]bool{
		ProducerDocsCode: true, ProducerDocsTools: true, ProducerInventory: true,
		ProducerDocsExtract: false, ProducerSession: false, ProducerPerson: false, ProducerDocsIndex: false,
		Producer("connector:confluence"): false,
	}
	for producer, generator := range want {
		if producer.IsGenerator() != generator {
			t.Errorf("%s: IsGenerator = %v, erwartet %v", producer, producer.IsGenerator(), generator)
		}
	}
	if dirs := Producer("connector:confluence").Dirs(); len(dirs) != 1 || dirs[0] != "external/confluence/" {
		t.Errorf("Connector-Verzeichnis = %v", dirs)
	}
	if dirs := ProducerPerson.Dirs(); strings.Join(dirs, ",") != "manual/,pitfalls/" {
		t.Errorf("person = %v", dirs)
	}
	if dirs := ProducerDocsIndex.Dirs(); len(dirs) != 0 {
		t.Errorf("docs-index besitzt ein Verzeichnis: %v", dirs)
	}
	if rows := ProducerTable(); len(rows) != 8 || !strings.Contains(rows[0], "docs-code") || !strings.Contains(rows[0], "nur publish") || !strings.Contains(rows[7], "README.md") {
		t.Errorf("Tabelle = %v", rows)
	}
}
