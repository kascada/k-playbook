package yamllite

import (
	"strings"
	"testing"
)

func TestParseLiestAbbildungenListenUndZeilen(t *testing.T) {
	document := []byte(`# Kommentar
name: beispiel
services:
  db:
    image: postgres:16.2   # mit Port-Doppelpunkt
  cache:
    image: redis:7.0
liste:
  - eins
  - zwei
eintraege:
  - path: /srv/a
    kind: helm
  - path: /srv/b
    kind: auto
`)
	root, err := Parse(document)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := root.Get("name").Str(); got != "beispiel" {
		t.Errorf("name = %q", got)
	}
	image := root.Get("services", "db", "image")
	if image.Str() != "postgres:16.2" {
		t.Errorf("image = %q", image.Str())
	}
	if image.At() != 5 {
		t.Errorf("Zeile des Images = %d, erwartet 5", image.At())
	}
	if keys := root.Get("services").MapKeys(); len(keys) != 2 || keys[0] != "db" {
		t.Errorf("Schlüsselreihenfolge = %v", keys)
	}
	if items := root.Get("liste").List(); len(items) != 2 || items[1].Str() != "zwei" {
		t.Errorf("liste = %+v", items)
	}
	entries := root.Get("eintraege").List()
	if len(entries) != 2 {
		t.Fatalf("eintraege = %d Einträge", len(entries))
	}
	if entries[1].Get("path").Str() != "/srv/b" || entries[1].Get("kind").Str() != "auto" {
		t.Errorf("zweiter Eintrag = %+v", entries[1])
	}
	if entries[0].At() != 12 {
		t.Errorf("Zeile des ersten Eintrags = %d, erwartet 12", entries[0].At())
	}
}

func TestParseLiestFlowSchreibweise(t *testing.T) {
	root, err := Parse([]byte("tags: [versions, inventory]\ngenerated: { by: k-doc-inventory, at: 2026-09-05T12:00:00+02:00 }\nroots: []\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if items := root.Get("tags").List(); len(items) != 2 || items[0].Str() != "versions" {
		t.Errorf("tags = %+v", items)
	}
	if got := root.Get("generated", "by").Str(); got != "k-doc-inventory" {
		t.Errorf("generated.by = %q", got)
	}
	if got := root.Get("generated", "at").Str(); got != "2026-09-05T12:00:00+02:00" {
		t.Errorf("generated.at = %q", got)
	}
	if items := root.Get("roots").List(); len(items) != 0 {
		t.Errorf("leere Liste = %+v", items)
	}
}

func TestParseMeldetDefekteEingaben(t *testing.T) {
	cases := map[string]string{
		"nicht geschlossene Flow-Liste": "roots: [/srv/a\n",
		"Tabulator in der Einrückung":   "roots:\n\t- /srv/a\n",
		"weder Schlüssel noch Eintrag":  "roots\nsources: []\n",
	}
	for name, document := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(document)); err == nil {
				t.Fatalf("erwartet wurde ein Fehler, keiner kam")
			}
		})
	}
}

func TestGetIstNilFest(t *testing.T) {
	root, err := Parse([]byte("a: 1\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if node := root.Get("b", "c", "d"); node != nil {
		t.Errorf("Get über einen fehlenden Pfad = %+v", node)
	}
	if root.Get("b").Str() != "" || root.Get("b").At() != 0 {
		t.Errorf("Zugriffe auf nil müssen leer bleiben")
	}
	if value, ok := root.Get("a").Int(); !ok || value != 1 {
		t.Errorf("Int() = %d, %v", value, ok)
	}
}

// Ein einzelner Ursachenfehler darf den Fehlertext nicht fluten: gleichlautende
// Hinweise fallen zusammen, mehr als maxReportedProblems verschiedene werden
// gezählt statt aufgezählt. Die Wirkung bleibt: Parse liefert nil.
func TestFehlertextIstBegrenzt(t *testing.T) {
	var document strings.Builder
	document.WriteString("a: 1\n")
	for line := 0; line < 40; line++ {
		document.WriteString("    verwaist: x\n")
	}
	root, err := Parse([]byte(document.String()))
	if err == nil {
		t.Fatal("erwartet wurde ein Fehler, keiner kam")
	}
	if root != nil {
		t.Errorf("bei einem strukturellen Problem darf es keinen Teilbaum geben: %+v", root)
	}
	text := err.Error()
	if got := strings.Count(text, "Zeile "); got != maxReportedProblems {
		t.Errorf("%d Hinweise im Text, erwartet %d: %s", got, maxReportedProblems, text)
	}
	if !strings.Contains(text, "… und 30 weitere Hinweise") {
		t.Errorf("das Abschneiden muss sichtbar sein: %s", text)
	}
	if len(text) > 600 {
		t.Errorf("Fehlertext ist %d Zeichen lang", len(text))
	}
}

func TestGleicheHinweiseFallenZusammen(t *testing.T) {
	_, err := Parse([]byte("a: [x\n"))
	if err == nil {
		t.Fatal("erwartet wurde ein Fehler, keiner kam")
	}
	if got := strings.Count(err.Error(), "Zeile 1:"); got != 1 {
		t.Errorf("Hinweis zu Zeile 1 kommt %d-mal vor: %s", got, err)
	}
	// Wenige Hinweise bleiben vollständig, ohne Zählhinweis.
	_, err = Parse([]byte("a: 1\n  b: 2\n  c: 3\n"))
	if err == nil || strings.Contains(err.Error(), "weitere") || strings.Count(err.Error(), "Zeile ") != 2 {
		t.Errorf("zwei Hinweise müssen vollständig erscheinen: %v", err)
	}
}

// Blockskalare als Listeneintrag — `- |` und `- >` — sind in Compose-Dateien
// üblich (`command:`, `healthcheck.test:`). Ihr Rumpf ist Text: tiefer
// eingerückte Zeilen darin sind keine Struktur und dürfen nicht gegen die
// offenen Ebenen geprüft werden. Dasselbe gilt für `key: |`.
func TestParseLiestBlockskalareAlsListeneintrag(t *testing.T) {
	root, err := Parse([]byte(`services:
  redis:
    image: redis:7-alpine
    command:
      - sh
      - -c
      - |
        if [ -n "$$REDIS_PASSWORD" ]; then
          exec redis-server --requirepass "$$REDIS_PASSWORD"
        fi
        # Ohne Passwort
        exec redis-server
    healthcheck:
      test:
        - CMD-SHELL
        - >-
          redis-cli
            ping
      script: |
          echo a
            echo b
        # Kommentar, weniger tief als der Rumpf
  db:
    image: postgres:15-alpine
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	command := root.Get("services", "redis", "command").List()
	if len(command) != 3 {
		t.Fatalf("command = %d Einträge, erwartet 3", len(command))
	}
	want := "if [ -n \"$$REDIS_PASSWORD\" ]; then\n  exec redis-server --requirepass \"$$REDIS_PASSWORD\"\nfi\n# Ohne Passwort\nexec redis-server"
	if got := command[2].Str(); got != want {
		t.Errorf("Blockskalar = %q, erwartet %q", got, want)
	}
	if command[2].At() != 7 {
		t.Errorf("Zeile des Blockskalars = %d, erwartet 7", command[2].At())
	}
	test := root.Get("services", "redis", "healthcheck", "test").List()
	if len(test) != 2 || test[1].Kind != Scalar || !strings.HasPrefix(test[1].Str(), "redis-cli") {
		t.Errorf("healthcheck.test = %+v", test)
	}
	if got := root.Get("services", "redis", "healthcheck", "script").Str(); got != "echo a\n  echo b" {
		t.Errorf("script = %q", got)
	}
	if got := root.Get("services", "db", "image").Str(); got != "postgres:15-alpine" {
		t.Errorf("db.image = %q", got)
	}
}
