package yamllite

import "testing"

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
