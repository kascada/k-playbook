package inventory

import (
	"strings"
	"testing"
)

// Body trennt den Frontmatter-Block ab und deutet den Rumpf nicht: was nach
// der zweiten `---`-Zeile steht, kommt unverändert zurück.
func TestBodyTrenntDasFrontmatterAb(t *testing.T) {
	data := []byte("---\ntitle: Versionsinventar\ninventory:\n  entries: 3\n---\n\n# Versionsinventar\n\n| a | b |\n")

	got := string(Body(data))
	if got != "# Versionsinventar\n\n| a | b |\n" {
		t.Errorf("Body = %q", got)
	}
}

// Zeilenenden werden in beiden Zweigen gleich behandelt: CRLF kommt als LF
// zurück, ob ein Frontmatter davorsteht oder nicht.
func TestBodyNormalisiertZeilenendenInBeidenZweigen(t *testing.T) {
	for name, data := range map[string]string{
		"mit Frontmatter":  "---\r\ntitle: x\r\n---\r\n\r\n# Rumpf\r\n",
		"ohne Frontmatter": "# Rumpf\r\n",
	} {
		if got := string(Body([]byte(data))); got != "# Rumpf\n" {
			t.Errorf("%s: Body = %q", name, got)
		}
	}
}

// Ohne Frontmatter ist der Rumpf die ganze Datei — auch dann, wenn ein
// öffnendes `---` kein schließendes findet.
func TestBodyOhneFrontmatterIstDieGanzeDatei(t *testing.T) {
	for name, data := range map[string]string{
		"kein Block":       "# Versionsinventar\n",
		"nur öffnend":      "---\ntitle: x\n# Rumpf\n",
		"Trenner im Rumpf": "# Versionsinventar\n\n---\n",
	} {
		if got := string(Body([]byte(data))); got != data {
			t.Errorf("%s: Body = %q, erwartet die Datei selbst", name, got)
		}
	}
}

// legacyHead ist der Kopf eines Inventars vor Task 120: type, tags, status
// und die Zahlen im Block `inventory:`.
const legacyHead = "---\n" +
	"type: Version Inventory\n" +
	"title: Versionsinventar\n" +
	"description: Vollständige Übersicht der deklarierten Versionen dieses Projekts, nach Umgebung getrennt und mit Herkunft je Zeile.\n" +
	"tags: [versions, inventory, dependencies]\n" +
	"status: stable\n" +
	"generated: { by: k-doc-inventory, at: 2026-10-25T10:00:00+02:00 }\n" +
	"inventory:\n" +
	"  sources-configured: 1\n" +
	"  sources-read: 2\n" +
	"  entries: 3\n" +
	"  deviations: 4\n" +
	"  rejected: 5\n" +
	"  sources-excluded: 6\n" +
	"  sources-unevaluable: 7\n" +
	"---\n\n# Versionsinventar\n"

// Der neue Kopf trägt die Zahlen unter generated.stats; der Stand kommt von
// dort, und type, status und tags verlangt niemand mehr.
func TestStatusLiestDenNeuenKopf(t *testing.T) {
	data := "---\n" +
		"title: Versionsinventar\n" +
		"description: Vollständige Übersicht.\n" +
		"generated:\n" +
		"  by: k-doc-inventory\n" +
		"  at: 2026-11-06T10:00:00+01:00\n" +
		"  stats:\n" +
		"    sources-configured: 1\n" +
		"    sources-read: 2\n" +
		"    entries: 3\n" +
		"    deviations: 4\n" +
		"    rejected: 5\n" +
		"    sources-excluded: 6\n" +
		"    sources-unevaluable: 7\n" +
		"---\n\n# Versionsinventar\n"
	status := StatusOf([]byte(data))
	if status.Problem != "" {
		t.Fatalf("Problem = %q", status.Problem)
	}
	want := Status{Path: DocRef, Present: true, GeneratedBy: GeneratedBy, GeneratedAt: "2026-11-06T10:00:00+01:00",
		SourcesConfigured: 1, SourcesRead: 2, Entries: 3, Deviations: 4, Rejected: 5, SourcesExcluded: 6, SourcesUnevaluable: 7}
	if status != want {
		t.Errorf("Status = %+v, erwartet %+v", status, want)
	}
}

// Ein Inventar im alten Kopf wird bis zum nächsten Lauf weiter gelesen: ohne
// generated.stats gilt der Block inventory:.
func TestStatusLiestDenAltenKopfBisZumNaechstenLauf(t *testing.T) {
	status := StatusOf([]byte(legacyHead))
	if status.Problem != "" {
		t.Fatalf("Problem = %q", status.Problem)
	}
	if status.SourcesConfigured != 1 || status.SourcesRead != 2 || status.Entries != 3 || status.Deviations != 4 ||
		status.Rejected != 5 || status.SourcesExcluded != 6 || status.SourcesUnevaluable != 7 ||
		status.GeneratedAt != "2026-10-25T10:00:00+02:00" {
		t.Errorf("Status = %+v", status)
	}

	// Fehlt im alten Kopf eine Zahl, nennt der Befund den alten Schlüssel.
	broken := strings.Replace(legacyHead, "  entries: 3\n", "", 1)
	if got := StatusOf([]byte(broken)).Problem; !strings.Contains(got, "inventory.entries") {
		t.Errorf("Problem = %q", got)
	}
}

// Fehlen die Zahlen ganz, oder ist eine ungültig, ist das ein sichtbarer
// Befund mit dem Schlüssel unter generated.stats — kein stilles Nullergebnis.
func TestStatusOhneZahlenIstUnvollstaendig(t *testing.T) {
	data := "---\ntitle: Versionsinventar\ndescription: D\ngenerated: { by: k-doc-inventory, at: 2026-11-06T10:00:00+01:00 }\n---\n"
	problem := StatusOf([]byte(data)).Problem
	for _, key := range statKeys {
		if !strings.Contains(problem, "generated.stats."+key) {
			t.Errorf("Problem nennt generated.stats.%s nicht: %q", key, problem)
		}
	}
	for _, stale := range []string{"type", "tags", "status,", "inventory."} {
		if strings.Contains(problem, stale) {
			t.Errorf("Problem verlangt noch %q: %q", stale, problem)
		}
	}

	invalid := strings.Replace(Render(Result{}, "2026-11-06T10:00:00+01:00"), "    entries: 0\n", "    entries: viele\n", 1)
	if got := StatusOf([]byte(invalid)).Problem; !strings.Contains(got, "generated.stats.entries (ungültig)") {
		t.Errorf("Problem = %q", got)
	}
}
