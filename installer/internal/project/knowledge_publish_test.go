package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func codeDoc(path, body string) KnowledgeDocument {
	return KnowledgeDocument{Path: path, Title: "Code", Subject: "Quelle", Origin: "/k-docs-code", State: KnowledgeStateCondensed, Body: body}
}

// hiddenSiblings zählt versteckte Verzeichnisse unter knowledge/ — die
// Zwischen- und Ausweichverzeichnisse des Tauschs, die nach einem Lauf nicht
// liegen bleiben dürfen.
func hiddenSiblings(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(KnowledgeDir(root))
	if err != nil {
		t.Fatal(err)
	}
	hidden := []string{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			hidden = append(hidden, entry.Name())
		}
	}
	return hidden
}

// Publish tauscht das Verzeichnis des Generators als Ganzes: der neue Satz
// steht, was nicht darin war, ist weg, der Index kennt nur noch den neuen
// Stand, und nichts davon zählt als Drift. Kein Zwischenverzeichnis bleibt.
func TestKnowledgePublishTauschtDasVerzeichnis(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)
	fixUpdated(t, "2026-09-12")
	if _, err := knowledge.Status(); err != nil {
		t.Fatal(err)
	}

	result, err := knowledge.Publish("docs-code", []KnowledgeDocument{
		codeDoc("neu.md", "# Neu\n\nDer Kennwortchunk.\n"),
		codeDoc("tief/unten.md", "# Unten\n\nTief.\n"),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.Producer != "docs-code" || result.Dir != "code/" || result.Written != 2 || result.Removed != 1 {
		t.Errorf("Ergebnis = %+v", result)
	}
	if pathExists(filepath.Join(KnowledgeDir(root), "code", "links.md")) {
		t.Error("code/links.md hat den Tausch überlebt")
	}
	if got := readKnowledgeFile(t, root, "code/neu.md"); !strings.HasPrefix(got, "---\ntitle: Code\nsubject: Quelle\norigin: /k-docs-code\nstate: condensed\nformat: markdown\nupdated: 2026-09-12\n---\n\n# Neu\n") {
		t.Errorf("code/neu.md = %q", got)
	}
	if hidden := hiddenSiblings(t, root); len(hidden) != 0 {
		t.Errorf("Zwischenverzeichnisse geblieben: %v", hidden)
	}

	hits, err := knowledge.Search("Kennwortchunk", KnowledgeFilter{}, 0)
	if err != nil || len(hits) != 1 || hits[0].Path != "code/neu.md" {
		t.Errorf("neuer Stand nicht im Index: %+v, %v", hits, err)
	}
	if hits, err := knowledge.Search("Symlinks", KnowledgeFilter{}, 0); err != nil || len(hits) != 0 {
		t.Errorf("alter Stand noch im Index: %+v, %v", hits, err)
	}
	status, err := knowledge.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.Stale || status.ByKind["code"].Files != 2 {
		t.Errorf("Status = %+v", status)
	}

	// Ein zweiter Lauf mit einem Dokument weniger entfernt genau dieses.
	result, err = knowledge.Publish("docs-code", []KnowledgeDocument{codeDoc("neu.md", "# Neu\n\nAnders.\n")})
	if err != nil || result.Written != 1 || result.Removed != 1 {
		t.Errorf("zweiter Lauf: %+v, %v", result, err)
	}
	if pathExists(filepath.Join(KnowledgeDir(root), "code", "tief")) {
		t.Error("code/tief/ hat den zweiten Tausch überlebt")
	}

	// Ein Verzeichnis, das es noch nicht gab, entsteht durch den Tausch.
	result, err = knowledge.Publish("docs-tools", []KnowledgeDocument{codeDoc("neu.md", "# Lib\n")})
	if err != nil || result.Dir != "libs/" || result.Written != 1 || result.Removed != 1 {
		t.Errorf("libs: %+v, %v", result, err)
	}
	if err := os.RemoveAll(filepath.Join(KnowledgeDir(root), "versions")); err != nil {
		t.Fatal(err)
	}
	result, err = knowledge.Publish("inventory", []KnowledgeDocument{codeDoc("inventory.md", "# Inventar\n")})
	if err != nil || result.Dir != "versions/" || result.Written != 1 || result.Removed != 0 {
		t.Errorf("versions neu: %+v, %v", result, err)
	}
}

// Ein Abbruch vor dem Tausch lässt den vorherigen Stand vollständig stehen —
// bei einer ungültigen Datei im Satz ebenso wie bei einem Fehlschlag mitten im
// Schreiben. Der zweite Fall ist echt: „a.md" und „a.md/b.md" im selben Satz
// lassen die zweite Datei am Anlegen ihres Verzeichnisses scheitern, nachdem
// die erste schon geschrieben ist.
func TestKnowledgePublishAbbruchLaesstDenStandStehen(t *testing.T) {
	for name, documents := range map[string][]KnowledgeDocument{
		"ungültiges Dokument im Satz": {
			codeDoc("neu.md", "# Neu\n"),
			with(codeDoc("kaputt.md", "# Kaputt\n"), func(d *KnowledgeDocument) { d.State = "superseded" }),
		},
		"Kopf im Rumpf": {
			codeDoc("neu.md", "# Neu\n"),
			codeDoc("kopf.md", "---\ntitle: X\n---\n# X\n"),
		},
		"Pfad außerhalb": {
			codeDoc("neu.md", "# Neu\n"),
			codeDoc("../libs/x.md", "# X\n"),
		},
		"doppelter Pfad": {
			codeDoc("neu.md", "# Neu\n"),
			codeDoc("./neu.md", "# Neu\n"),
		},
		"Fehlschlag mitten im Schreiben": {
			codeDoc("a.md", "# A\n"),
			codeDoc("a.md/b.md", "# B\n"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := knowledgeFixture(t)
			knowledge := NewKnowledge(root)
			before := readKnowledgeFile(t, root, "code/links.md")
			if _, err := knowledge.Status(); err != nil {
				t.Fatal(err)
			}

			if _, err := knowledge.Publish("docs-code", documents); err == nil {
				t.Fatal("Publish gelang")
			}
			if got := readKnowledgeFile(t, root, "code/links.md"); got != before {
				t.Error("code/links.md verändert")
			}
			if pathExists(filepath.Join(KnowledgeDir(root), "code", "neu.md")) || pathExists(filepath.Join(KnowledgeDir(root), "code", "a.md")) {
				t.Error("halber Stand geschrieben")
			}
			if hidden := hiddenSiblings(t, root); len(hidden) != 0 {
				t.Errorf("Zwischenverzeichnisse geblieben: %v", hidden)
			}
			status, err := knowledge.Status()
			if err != nil {
				t.Fatal(err)
			}
			if status.Stale || status.ByKind["code"].Files != 1 {
				t.Errorf("Index nach Abbruch: %+v", status)
			}
		})
	}
}

// Ein leerer Satz veröffentlicht nichts: nil und [] werden abgewiesen, bevor
// irgendetwas angelegt wird. Das ist der Wächter im Kern, der beide Wege
// schützt — die Kommandozeile prüft schon im Lader, der MCP-Weg reicht
// documents: [] und ein weggelassenes Feld (nil) sonst ungeprüft durch, und
// danach stünde ein leeres Zwischenverzeichnis am Platz des alten Stands.
func TestKnowledgePublishLeererSatzWirdAbgewiesen(t *testing.T) {
	for name, documents := range map[string][]KnowledgeDocument{"nil": nil, "leer": {}} {
		t.Run(name, func(t *testing.T) {
			root := knowledgeFixture(t)
			knowledge := NewKnowledge(root)
			if _, err := knowledge.Publish("docs-code", []KnowledgeDocument{codeDoc("a.md", "# A\n\nKennwortchunk.\n")}); err != nil {
				t.Fatalf("erster Lauf: %v", err)
			}
			before := readKnowledgeFile(t, root, "code/a.md")

			result, err := knowledge.Publish("docs-code", documents)
			if err == nil {
				t.Fatalf("leerer Satz angenommen: %+v", result)
			}
			if !strings.Contains(err.Error(), "ein leerer Satz veröffentlicht nichts") {
				t.Errorf("Meldung = %q", err)
			}
			if got := readKnowledgeFile(t, root, "code/a.md"); got != before {
				t.Error("code/a.md verändert oder verschwunden")
			}
			if hidden := hiddenSiblings(t, root); len(hidden) != 0 {
				t.Errorf("Zwischenverzeichnisse geblieben: %v", hidden)
			}
			hits, err := NewKnowledge(root).Search("Kennwortchunk", KnowledgeFilter{}, 0)
			if err != nil || len(hits) != 1 || hits[0].Path != "code/a.md" {
				t.Errorf("Index nach abgewiesenem Satz: %+v, %v", hits, err)
			}
		})
	}
}

// Das getauschte Generatorverzeichnis trägt dieselben Rechte wie die
// Unterverzeichnisse, die MkdirAll(…, 0o755) darin anlegt — also 0o755 unter
// der umask, nicht die 0o700 von os.MkdirTemp. Verglichen wird nicht mit einem
// festen Wert, sondern mit einem frisch angelegten Nachbarn: so misst der
// Test unter jeder umask das Richtige. Sonst wäre knowledge/code/ für jeden
// anderen Benutzer und jeden Container mit anderer uid unlesbar.
func TestKnowledgePublishVerzeichnisrechteWieUnterverzeichnisse(t *testing.T) {
	root := knowledgeFixture(t)
	if _, err := NewKnowledge(root).Publish("docs-code", []KnowledgeDocument{codeDoc("tief/unten.md", "# Unten\n")}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	neighbour := filepath.Join(KnowledgeDir(root), "nachbar")
	if err := os.MkdirAll(neighbour, 0o755); err != nil {
		t.Fatal(err)
	}
	want, err := os.Stat(neighbour)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"code", "code/tief"} {
		info, err := os.Stat(filepath.Join(KnowledgeDir(root), filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if info.Mode().Perm() != want.Mode().Perm() {
			t.Errorf("%s: Rechte %v, erwartet %v wie ein frisch per MkdirAll angelegtes Verzeichnis", rel, info.Mode().Perm(), want.Mode().Perm())
		}
	}
}

// Nur die drei Generatoren dürfen publish; alle anderen bekommen einen Fehler,
// der sie auf write verweist.
func TestKnowledgePublishNurGeneratoren(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)

	for _, producer := range []string{"docs-extract", "session", "person", "docs-index", "connector:confluence"} {
		_, err := knowledge.Publish(producer, []KnowledgeDocument{codeDoc("x.md", "# X\n")})
		if err == nil || !strings.Contains(err.Error(), "den Generatoren") {
			t.Errorf("%s: %v", producer, err)
		}
	}
	if _, err := knowledge.Publish("gate", nil); err == nil || !strings.Contains(err.Error(), "unbekannter Erzeuger") {
		t.Errorf("unbekannt: %v", err)
	}
	for _, rel := range []string{"extracted/x.md", "findings/x.md", "manual/x.md", "x.md", "external/confluence/x.md"} {
		if pathExists(filepath.Join(KnowledgeDir(root), filepath.FromSlash(rel))) {
			t.Errorf("%s entstanden", rel)
		}
	}
}

// publish --from <dir>: die Dateien tragen die Felder im Kopf, das Werkzeug
// liest sie, prüft sie und setzt den Kopf neu zusammen — updated setzt es
// selbst, ein fremdes Feld wird nicht durchgereicht.
func TestKnowledgeLoadDocumentsLiestKopfUndRumpf(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("links.md", "---\ntitle: Verlinkung\nsubject: Symlinks\norigin: /k-docs-code 2026-09-12\nstate: condensed\nformat: markdown\nsources:\n  - chat/a.md\nupdated: 1999-01-01\nfremd: bleibt draußen\n---\n\n# Verlinkung\n\nText.\n")
	write("tief/unten.md", "---\ntitle: Unten\nsubject: Tiefe\norigin: hier\nstate: raw\n---\n# Unten\n")
	write(".versteckt/x.md", "---\ntitle: X\n---\n# X\n")
	write("notiz.txt", "kein Markdown")

	documents, err := LoadKnowledgeDocuments(dir)
	if err != nil {
		t.Fatalf("LoadKnowledgeDocuments: %v", err)
	}
	if len(documents) != 2 {
		t.Fatalf("Dokumente = %+v", documents)
	}
	first := documents[0]
	if first.Path != "links.md" || first.Title != "Verlinkung" || first.Subject != "Symlinks" || first.Origin != "/k-docs-code 2026-09-12" || first.State != "condensed" || first.Format != "markdown" || strings.Join(first.Sources, ",") != "chat/a.md" || first.Body != "# Verlinkung\n\nText.\n" {
		t.Errorf("erstes Dokument = %+v", first)
	}
	if documents[1].Path != "tief/unten.md" || documents[1].Format != "" || documents[1].Body != "# Unten\n" {
		t.Errorf("zweites Dokument = %+v", documents[1])
	}

	root := knowledgeFixture(t)
	fixUpdated(t, "2026-09-12")
	if _, err := NewKnowledge(root).Publish("docs-code", documents); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	got := readKnowledgeFile(t, root, "code/links.md")
	if strings.Contains(got, "fremd") || strings.Contains(got, "1999") || !strings.Contains(got, "updated: 2026-09-12\n") {
		t.Errorf("Kopf durchgereicht statt neu gebaut:\n%s", got)
	}

	// Ohne Kopf, ohne Pflichtfeld, leeres Verzeichnis: Fehler, die die Datei nennen.
	write("ohne.md", "# Ohne Kopf\n")
	if _, err := LoadKnowledgeDocuments(dir); err == nil || !strings.Contains(err.Error(), "ohne.md") {
		t.Errorf("ohne Kopf: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "ohne.md")); err != nil {
		t.Fatal(err)
	}
	write("halb.md", "---\ntitle: Halb\n---\n# Halb\n")
	documents, err = LoadKnowledgeDocuments(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewKnowledge(root).Publish("docs-code", documents); err == nil || !strings.Contains(err.Error(), "halb.md") || !strings.Contains(err.Error(), "subject") {
		t.Errorf("halbes Frontmatter: %v", err)
	}
	if _, err := LoadKnowledgeDocuments(t.TempDir()); err == nil {
		t.Error("leeres Verzeichnis angenommen")
	}
	if _, err := LoadKnowledgeDocuments(filepath.Join(dir, "gibt-es-nicht")); err == nil {
		t.Error("fehlendes Verzeichnis angenommen")
	}
}

// Supersede setzt state, successor, superseded_reason und updated, lässt den
// Rumpf und die übrigen Felder stehen, löscht nichts, und der Index kennt das
// Dokument weiter. Der Nachfolger muss existieren.
func TestKnowledgeSupersede(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)
	fixUpdated(t, "2026-09-12")

	if _, err := knowledge.Write("session", sessionDoc("findings/neu.md"), ""); err != nil {
		t.Fatal(err)
	}
	rel, _, err := knowledge.Supersede("./findings/notiz.md", "findings/neu.md", "Befund war unvollständig")
	if err != nil || rel != "findings/notiz.md" {
		t.Fatalf("Supersede: %q, %v", rel, err)
	}
	want := "---\ntitle: Gelernt\nsubject: Release\norigin: Sitzung 42\nstate: superseded\nformat: markdown\n" +
		"successor: findings/neu.md\nsuperseded_reason: Befund war unvollständig\nupdated: 2026-09-12\n---\n\n# Gelernt\n\nDer Befehl war make sichern.\n"
	if got := readKnowledgeFile(t, root, rel); got != want {
		t.Errorf("abgelöst:\n%q\nerwartet:\n%q", got, want)
	}
	entries, err := knowledge.List(KnowledgeFilter{Kind: "findings"})
	if err != nil || len(entries) != 2 {
		t.Errorf("List: %+v, %v", entries, err)
	}
	if content, err := knowledge.Read(rel); err != nil || !strings.Contains(content, "state: superseded") {
		t.Errorf("Read: %v", err)
	}
	if status, err := knowledge.Status(); err != nil || status.Stale {
		t.Errorf("Supersede als Drift gemeldet: %+v, %v", status, err)
	}

	// Ein zweites Mal ist ein Eingabefehler (Task 063, Entscheidung 3): wer den
	// Nachfolger ändern will, löst den Nachfolger ab. Verweis und Grund bleiben.
	if _, _, err := knowledge.Supersede("findings/notiz.md", "findings/neu.md", "Anderer Grund"); err == nil || !IsInputError(err) {
		t.Errorf("zweites Ablösen: %v", err)
	}
	if got := readKnowledgeFile(t, root, rel); got != want {
		t.Errorf("zweites Ablösen hat die Datei verändert:\n%s", got)
	}

	// Eine Datei ohne Kopf — an der Prüfung vorbei entstanden — bekommt einen.
	rel, _, err = knowledge.Supersede("manual/release.md", "findings/neu.md", "Ersetzt")
	if err != nil {
		t.Fatal(err)
	}
	if got := readKnowledgeFile(t, root, rel); !strings.HasPrefix(got, "---\nstate: superseded\nsuccessor: findings/neu.md\nsuperseded_reason: Ersetzt\nupdated: 2026-09-12\n---\n\n# Release\n") {
		t.Errorf("ohne Kopf:\n%s", got)
	}

	for name, args := range map[string][3]string{
		"fehlendes Dokument":   {"findings/fehlt.md", "findings/neu.md", "x"},
		"fehlender Nachfolger": {"findings/neu.md", "findings/fehlt.md", "x"},
		"eigener Nachfolger":   {"findings/neu.md", "findings/neu.md", "x"},
		"leerer Grund":         {"findings/neu.md", "findings/notiz.md", " "},
		"Ausbruch":             {"../k-playbook.md", "findings/neu.md", "x"},
		"Nachfolger Ausbruch":  {"findings/neu.md", "../x.md", "x"},
	} {
		if _, _, err := knowledge.Supersede(args[0], args[1], args[2]); err == nil {
			t.Errorf("%s angenommen", name)
		}
	}
	if got := readKnowledgeFile(t, root, "findings/neu.md"); strings.Contains(got, "superseded") {
		t.Error("findings/neu.md wurde trotz Ablehnung abgelöst")
	}
}

// Entscheidung 1 aus Task 063: Die Pfade bei publish sind relativ zum
// Generatorverzeichnis. Ein Pfad, der es schon trägt (code/…), wird als
// Eingabefehler abgewiesen und nicht still nach code/code/ geschrieben — der
// Tausch hätte sonst den ganzen Bestand entfernt.
func TestKnowledgePublishPfadMitErzeugerverzeichnisWirdAbgewiesen(t *testing.T) {
	for producer, rel := range map[string]string{"docs-code": "code/overview.md", "docs-tools": "libs/x.md", "inventory": "versions/tief/x.md"} {
		t.Run(producer, func(t *testing.T) {
			root := knowledgeFixture(t)
			knowledge := NewKnowledge(root)
			if _, err := knowledge.Status(); err != nil {
				t.Fatal(err)
			}
			dir := strings.SplitN(rel, "/", 2)[0]
			before := map[string]string{
				"code":     readKnowledgeFile(t, root, "code/links.md"),
				"libs":     readKnowledgeFile(t, root, "libs/goldmark.md"),
				"versions": readKnowledgeFile(t, root, "versions/inventory.md"),
			}

			_, err := knowledge.Publish(producer, []KnowledgeDocument{codeDoc("neu.md", "# Neu\n"), codeDoc(rel, "# Doppelt\n")})
			if err == nil {
				t.Fatal("Pfad mit vorangestelltem Erzeugerverzeichnis angenommen")
			}
			if !IsInputError(err) {
				t.Errorf("kein Eingabefehler: %v", err)
			}
			if !strings.Contains(err.Error(), "relativ zu "+dir+"/") {
				t.Errorf("Meldung nennt die Konvention nicht: %q", err)
			}
			if pathExists(filepath.Join(KnowledgeDir(root), dir, dir)) || pathExists(filepath.Join(KnowledgeDir(root), dir, "neu.md")) {
				t.Error("trotz Abweisung geschrieben")
			}
			for kind, content := range before {
				file := map[string]string{"code": "code/links.md", "libs": "libs/goldmark.md", "versions": "versions/inventory.md"}[kind]
				if got := readKnowledgeFile(t, root, file); got != content {
					t.Errorf("%s verändert", file)
				}
			}
			if hidden := hiddenSiblings(t, root); len(hidden) != 0 {
				t.Errorf("Zwischenverzeichnisse geblieben: %v", hidden)
			}
			if status, err := knowledge.Status(); err != nil || status.Stale {
				t.Errorf("Status nach Abweisung: %+v, %v", status, err)
			}
		})
	}
}

// Scheitert das Schreiben des Index, steht der vorherige Stand auf der Platte
// und im Index: der Index wird vor dem Tausch geschrieben, ein Fehlschlag dort
// ist ein Abbruch ohne Tausch. Der Fehlschlag greift beim Schreiben, nicht beim
// Öffnen — der Index ist gebaut und ohne Drift, open() schreibt nichts.
func TestKnowledgePublishFehlschlagBeimIndexschreibenLaesstAltenStand(t *testing.T) {
	root := knowledgeFixture(t)
	if _, err := NewKnowledge(root).Status(); err != nil {
		t.Fatal(err)
	}
	before := readKnowledgeFile(t, root, "code/links.md")
	denyWrite(t, KnowledgeCacheDir(root))

	knowledge := NewKnowledge(root)
	_, err := knowledge.Publish("docs-code", []KnowledgeDocument{codeDoc("neu.md", "# Neu\n\nKennwortchunk.\n")})
	if err == nil {
		t.Fatal("Publish gelang trotz gesperrtem Index")
	}
	if !strings.Contains(err.Error(), "Wissensindex schreiben") {
		t.Errorf("Fehlschlag nicht beim Schreiben des Index: %v", err)
	}
	if IsInputError(err) {
		t.Errorf("Umgebungsfehler als Eingabefehler: %v", err)
	}
	if !pathExists(filepath.Join(KnowledgeDir(root), "code", "links.md")) {
		t.Fatal("code/links.md ist weg — der Tausch lief vor dem Index")
	}
	if got := readKnowledgeFile(t, root, "code/links.md"); got != before {
		t.Error("code/links.md verändert")
	}
	if pathExists(filepath.Join(KnowledgeDir(root), "code", "neu.md")) {
		t.Error("code/neu.md steht trotz Fehlschlag")
	}
	if hidden := hiddenSiblings(t, root); len(hidden) != 0 {
		t.Errorf("Zwischenverzeichnisse geblieben: %v", hidden)
	}
	stored := readIndexFile(t, root)
	if _, ok := stored.Files["code/links.md"]; !ok {
		t.Error("Index kennt code/links.md nicht mehr")
	}
	if _, ok := stored.Files["code/neu.md"]; ok {
		t.Error("Index beschreibt den neuen Stand")
	}
	status, err := NewKnowledge(root).Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.Stale || status.ByKind["code"].Files != 1 {
		t.Errorf("Status nach Fehlschlag: %+v", status)
	}
	if hits, err := NewKnowledge(root).Search("Symlinks", KnowledgeFilter{}, 0); err != nil || len(hits) != 1 || hits[0].Path != "code/links.md" {
		t.Errorf("alter Stand nicht im Index: %+v, %v", hits, err)
	}
}

// Die beiden übrigen Fehlschläge lassen sich ohne Fehler-Hook im Produktivpfad
// nicht erzeugen; ihre Fälle stehen in
// material/befunde/wissensablage-schreibseite.md.
func TestKnowledgePublishFehlschlagBeimChunkenUndBeimTausch(t *testing.T) {
	t.Run("Chunken", func(t *testing.T) {
		t.Skip("Chunken liest die eben mit 0o644 geschriebenen Dateien des Zwischenverzeichnisses; ein Lesefehler dazwischen ist über Rechte nicht erzeugbar, ohne das Schreiben selbst zu verhindern. Liegt vor dem Tausch und ist ein Abbruch ohne Tausch.")
	})
	t.Run("Tausch", func(t *testing.T) {
		t.Skip("rename innerhalb von knowledge/ braucht nur das Schreibrecht auf knowledge/, das schon das Zwischenverzeichnis braucht; ein Verzeichnis im selben Elternverzeichnis umzubenennen verlangt unter Linux kein Schreibrecht auf das Verzeichnis selbst. Ohne root (chattr, Mount) nicht erzeugbar.")
	})
}

// Stirbt der Prozess zwischen Indexschreiben und Tausch, stehen der neue Index
// und der alte Stand auf der Platte. Nachgestellt über einen erfolgreichen Lauf,
// dessen Verzeichnis danach auf den alten Stand zurückgesetzt wird. Der nächste
// Zugriff liefert den alten Stand; stale: true ist dabei die erwartete Meldung.
func TestKnowledgePublishAbbruchZwischenIndexUndTausch(t *testing.T) {
	root := knowledgeFixture(t)
	if _, err := NewKnowledge(root).Status(); err != nil {
		t.Fatal(err)
	}
	before := readKnowledgeFile(t, root, "code/links.md")
	if _, err := NewKnowledge(root).Publish("docs-code", []KnowledgeDocument{codeDoc("neu.md", "# Neu\n\nKennwortchunk.\n")}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(KnowledgeDir(root), "code")); err != nil {
		t.Fatal(err)
	}
	writeKnowledgeFile(t, root, "code/links.md", before)

	status, err := NewKnowledge(root).Status()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Stale || status.ByKind["code"].Files != 1 {
		t.Errorf("Status nach Abbruch: %+v", status)
	}
	if hits, err := NewKnowledge(root).Search("Symlinks", KnowledgeFilter{}, 0); err != nil || len(hits) != 1 || hits[0].Path != "code/links.md" {
		t.Errorf("alter Stand nicht zurück im Index: %+v, %v", hits, err)
	}
	if hits, err := NewKnowledge(root).Search("Kennwortchunk", KnowledgeFilter{}, 0); err != nil || len(hits) != 0 {
		t.Errorf("neuer Stand noch im Index: %+v, %v", hits, err)
	}
	if status, err := NewKnowledge(root).Status(); err != nil || status.Stale {
		t.Errorf("zweiter Zugriff: %+v, %v", status, err)
	}
}

// swapCrash stellt einen Absturz zwischen den beiden Umbenennungen nach: der
// neue Satz ist veröffentlicht und im Index, dann wird der Zustand der Platte
// auf „Ziel beiseitegestellt, Zwischenverzeichnis nicht eingesetzt"
// zurückgedreht — code/ fehlt, der alte Stand liegt als .code-alt-*, der neue
// als .code-neu-*.
func swapCrash(t *testing.T, root string) (old string) {
	t.Helper()
	if _, err := NewKnowledge(root).Status(); err != nil {
		t.Fatal(err)
	}
	old = readKnowledgeFile(t, root, "code/links.md")
	if _, err := NewKnowledge(root).Publish("docs-code", []KnowledgeDocument{codeDoc("neu.md", "# Neu\n\nKennwortchunk.\n")}); err != nil {
		t.Fatal(err)
	}
	dir := KnowledgeDir(root)
	if err := os.Rename(filepath.Join(dir, "code"), filepath.Join(dir, ".code-neu-111111")); err != nil {
		t.Fatal(err)
	}
	writeKnowledgeFile(t, root, ".code-alt-222222/links.md", old)
	return old
}

// Ein verwaistes .<dir>-alt-* ohne Zielverzeichnis stellt der nächste Zugriff
// zurück, bevor die Drift-Erkennung läuft: Platte und Index zeigen den alten
// Stand. Das versteckte -neu-* bleibt liegen, und status nennt es.
func TestKnowledgeVerwaistesAltVerzeichnisWirdZurueckgestellt(t *testing.T) {
	t.Run("zurückgestellt", func(t *testing.T) {
		root := knowledgeFixture(t)
		old := swapCrash(t, root)

		knowledge := NewKnowledge(root)
		status, err := knowledge.Status()
		if err != nil {
			t.Fatal(err)
		}
		if !pathExists(filepath.Join(KnowledgeDir(root), "code", "links.md")) {
			t.Fatal("code/ nicht zurückgestellt")
		}
		if got := readKnowledgeFile(t, root, "code/links.md"); got != old {
			t.Error("code/links.md trägt nicht den alten Stand")
		}
		if status.ByKind["code"].Files != 1 || !status.Stale {
			t.Errorf("Status: %+v", status)
		}
		if hits, err := NewKnowledge(root).Search("Symlinks", KnowledgeFilter{}, 0); err != nil || len(hits) != 1 || hits[0].Path != "code/links.md" {
			t.Errorf("alter Stand nicht im Index: %+v, %v", hits, err)
		}
		if hits, err := NewKnowledge(root).Search("Kennwortchunk", KnowledgeFilter{}, 0); err != nil || len(hits) != 0 {
			t.Errorf("neuer Stand im Index: %+v, %v", hits, err)
		}
		if hidden := hiddenSiblings(t, root); len(hidden) != 1 || hidden[0] != ".code-neu-111111" {
			t.Errorf("versteckte Verzeichnisse: %v", hidden)
		}
		notes := strings.Join(knowledge.Notes(), "; ")
		if !strings.Contains(notes, ".code-alt-222222") || !strings.Contains(notes, ".code-neu-111111") {
			t.Errorf("Notizen nennen Rückstellung und Rest nicht: %q", notes)
		}
	})

	t.Run("vorhandenes leeres Ziel wird nicht ersetzt", func(t *testing.T) {
		root := knowledgeFixture(t)
		swapCrash(t, root)
		if err := os.Mkdir(filepath.Join(KnowledgeDir(root), "code"), 0o755); err != nil {
			t.Fatal(err)
		}
		knowledge := NewKnowledge(root)
		if _, err := knowledge.Status(); err != nil {
			t.Fatalf("Zugriff scheitert: %v", err)
		}
		if pathExists(filepath.Join(KnowledgeDir(root), "code", "links.md")) {
			t.Error("vorhandenes leeres Ziel ersetzt")
		}
		if !pathExists(filepath.Join(KnowledgeDir(root), ".code-alt-222222", "links.md")) {
			t.Error(".code-alt-222222 verschwunden")
		}
		if notes := strings.Join(knowledge.Notes(), "; "); !strings.Contains(notes, ".code-alt-222222") {
			t.Errorf("Notiz nennt das verbliebene Verzeichnis nicht: %q", notes)
		}
	})

	t.Run("mehrere Kandidaten", func(t *testing.T) {
		root := knowledgeFixture(t)
		old := swapCrash(t, root)
		writeKnowledgeFile(t, root, ".code-alt-333333/links.md", old)
		knowledge := NewKnowledge(root)
		if _, err := knowledge.Status(); err != nil {
			t.Fatalf("Zugriff scheitert: %v", err)
		}
		if pathExists(filepath.Join(KnowledgeDir(root), "code")) {
			t.Error("bei zwei Kandidaten zurückgestellt")
		}
		notes := strings.Join(knowledge.Notes(), "; ")
		if !strings.Contains(notes, ".code-alt-222222") || !strings.Contains(notes, ".code-alt-333333") {
			t.Errorf("Notiz nennt die Kandidaten nicht: %q", notes)
		}
	})

	t.Run("Rückstellung scheitert", func(t *testing.T) {
		root := knowledgeFixture(t)
		swapCrash(t, root)
		denyWrite(t, KnowledgeDir(root))
		knowledge := NewKnowledge(root)
		if _, err := knowledge.Status(); err != nil {
			t.Fatalf("Zugriff scheitert: %v", err)
		}
		if pathExists(filepath.Join(KnowledgeDir(root), "code")) {
			t.Error("trotz gesperrtem knowledge/ zurückgestellt")
		}
		if notes := strings.Join(knowledge.Notes(), "; "); !strings.Contains(notes, ".code-alt-222222") {
			t.Errorf("Notiz nennt das Verzeichnis nicht: %q", notes)
		}
	})
}

// Entscheidungen 2 bis 4 aus Task 063: supersede weist Ziele und Nachfolger in
// den Generatorverzeichnissen und die Wurzel-README ab, ebenso ein schon
// abgelöstes Ziel und einen Nachfolger, der kein Suchtreffer sein kann (raw,
// superseded). Keine Abweisung verändert eine Datei oder den Index.
func TestKnowledgeSupersedeWeistZielZustandUndNachfolgerAb(t *testing.T) {
	root := knowledgeFixture(t)
	knowledge := NewKnowledge(root)
	if _, err := knowledge.Write("session", sessionDoc("findings/neu.md"), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := knowledge.Write("session", with(sessionDoc("findings/roh.md"), func(d *KnowledgeDocument) { d.State = KnowledgeStateRaw }), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := knowledge.Write("session", sessionDoc("findings/weg.md"), ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := knowledge.Supersede("findings/weg.md", "findings/neu.md", "Ersetzt"); err != nil {
		t.Fatal(err)
	}
	if _, err := knowledge.Status(); err != nil {
		t.Fatal(err)
	}

	cases := map[string]struct{ path, successor, message string }{
		"Ziel unter code/":           {"code/links.md", "findings/neu.md", "code/"},
		"Ziel unter libs/":           {"libs/goldmark.md", "findings/neu.md", "libs/"},
		"Ziel unter versions/":       {"versions/inventory.md", "findings/neu.md", "versions/"},
		"Ziel README":                {"README.md", "findings/neu.md", "README.md"},
		"Nachfolger unter code/":     {"findings/notiz.md", "code/links.md", "code/"},
		"Nachfolger unter libs/":     {"findings/notiz.md", "libs/goldmark.md", "libs/"},
		"Nachfolger unter versions/": {"findings/notiz.md", "versions/inventory.md", "versions/"},
		"Nachfolger README":          {"findings/notiz.md", "README.md", "README.md"},
		"schon abgelöst":             {"findings/weg.md", "findings/notiz.md", "schon abgelöst"},
		"Nachfolger raw":             {"findings/notiz.md", "findings/roh.md", "condensed oder reviewed"},
		"Nachfolger superseded":      {"findings/notiz.md", "findings/weg.md", "condensed oder reviewed"},
	}
	files := []string{"README.md", "code/links.md", "libs/goldmark.md", "versions/inventory.md", "findings/notiz.md", "findings/neu.md", "findings/roh.md", "findings/weg.md"}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			before := map[string]string{}
			for _, rel := range files {
				before[rel] = readKnowledgeFile(t, root, rel)
			}
			indexBefore, err := os.ReadFile(KnowledgeIndexFile(root))
			if err != nil {
				t.Fatal(err)
			}

			_, _, err = knowledge.Supersede(tc.path, tc.successor, "Grund")
			if err == nil {
				t.Fatal("angenommen")
			}
			if !IsInputError(err) {
				t.Errorf("kein Eingabefehler: %v", err)
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Errorf("Meldung %q trägt %q nicht", err, tc.message)
			}
			for _, rel := range files {
				if got := readKnowledgeFile(t, root, rel); got != before[rel] {
					t.Errorf("%s verändert", rel)
					writeKnowledgeFile(t, root, rel, before[rel])
				}
			}
			if indexAfter, err := os.ReadFile(KnowledgeIndexFile(root)); err != nil || string(indexAfter) != string(indexBefore) {
				t.Errorf("Index verändert: %v", err)
			}
		})
	}
}
