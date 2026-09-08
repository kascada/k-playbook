package yamllite

import (
	"os"
	"path/filepath"
	"testing"
)

// Die fünf Fälle aus Task 052 — Ziel-Tabelle —, je einzeln prüfbar. Gegen den
// Stand vor der Task müssen die ersten drei fehlschlagen; Merge-Key und
// Kontrolle blieben schon damals grün.
func TestAnkerAmSkalarIstEtikett(t *testing.T) {
	root, err := Parse([]byte("tag: &v \"1.2.3\"\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := root.Get("tag").Str(); got != "1.2.3" {
		t.Errorf("tag = %q, erwartet 1.2.3", got)
	}
	if got := root.Get("tag").At(); got != 1 {
		t.Errorf("Zeile = %d, erwartet 1", got)
	}
}

func TestAnkerVorBlockLiestDenBlock(t *testing.T) {
	root, err := Parse([]byte("b: &block\n  x: 1\n  y: zwei\nc: 3\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	block := root.Get("b")
	if block == nil || block.Kind != Mapping {
		t.Fatalf("b = %+v, erwartet eine Abbildung", block)
	}
	if got := block.Get("y").Str(); got != "zwei" {
		t.Errorf("b.y = %q", got)
	}
	if got := block.Get("y").At(); got != 3 {
		t.Errorf("Zeile von b.y = %d, erwartet 3", got)
	}
	if got := root.Get("c").Str(); got != "3" {
		t.Errorf("c = %q — der Block darf die Folgezeile nicht schlucken", got)
	}
}

func TestAnkerAmListeneintrag(t *testing.T) {
	root, err := Parse([]byte("l:\n  - &item wert\n  - &item\n    k: v\n  - &leer\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	items := root.Get("l").List()
	if len(items) != 3 {
		t.Fatalf("l = %d Einträge, erwartet 3", len(items))
	}
	if got := items[0].Str(); got != "wert" {
		t.Errorf("erster Eintrag = %q, erwartet wert", got)
	}
	if items[0].At() != 2 {
		t.Errorf("Zeile des ersten Eintrags = %d, erwartet 2", items[0].At())
	}
	if items[1].Kind != Mapping || items[1].Get("k").Str() != "v" {
		t.Errorf("zweiter Eintrag = %+v, erwartet Abbildung mit k: v", items[1])
	}
	if items[1].Get("k").At() != 4 {
		t.Errorf("Zeile von k = %d, erwartet 4", items[1].Get("k").At())
	}
	if items[2].Kind != Scalar || items[2].Str() != "" || items[2].At() != 5 {
		t.Errorf("dritter Eintrag = %+v, erwartet leerer Skalar in Zeile 5", items[2])
	}
}

func TestMergeKeyBleibtGewoehnlichesFeld(t *testing.T) {
	root, err := Parse([]byte("<<: *base\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := root.Get("<<").Str(); got != "*base" {
		t.Errorf("<< = %q, erwartet den wörtlichen Wert *base", got)
	}
}

func TestSkalarOhneAnkerBleibtUnveraendert(t *testing.T) {
	root, err := Parse([]byte("tag: \"1.2.3\"\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := root.Get("tag").Str(); got != "1.2.3" {
		t.Errorf("tag = %q", got)
	}
}

func TestAnkerVorListe(t *testing.T) {
	for name, document := range map[string]string{
		"eingerückt": "l: &all\n  - a\n  - b\n",
		"bündig":     "l: &all\n- a\n- b\n",
	} {
		t.Run(name, func(t *testing.T) {
			root, err := Parse([]byte(document))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			items := root.Get("l").List()
			if len(items) != 2 || items[0].Str() != "a" || items[1].Str() != "b" {
				t.Errorf("l = %+v", items)
			}
			if len(items) == 2 && items[1].At() != 3 {
				t.Errorf("Zeile von b = %d, erwartet 3", items[1].At())
			}
		})
	}
}

func TestAnkerAufObersterEbene(t *testing.T) {
	for name, document := range map[string]string{
		"eigene Zeile":    "&root\na: 1\nb: 2\n",
		"am Dokumentkopf": "--- &root\na: 1\nb: 2\n",
	} {
		t.Run(name, func(t *testing.T) {
			root, err := Parse([]byte(document))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got := root.Get("a").Str(); got != "1" {
				t.Errorf("a = %q", got)
			}
			if got := root.Get("b").At(); got != 3 {
				t.Errorf("Zeile von b = %d, erwartet 3", got)
			}
		})
	}
}

// Anker an Schlüsseln und in Flow-Sammlungen liegen außerhalb des Umfangs:
// sie bleiben Rohtext und werden weder erkannt noch gemeldet.
func TestAnkerAnSchluesselUndInFlowBleibenRohtext(t *testing.T) {
	root, err := Parse([]byte("&a key: v\nflow: [&b x, y]\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := root.Get("&a key").Str(); got != "v" {
		t.Errorf("Schlüssel mit Anker = %q, erwartet v unter dem Rohtext-Schlüssel", got)
	}
	if items := root.Get("flow").List(); len(items) != 2 || items[0].Str() != "&b x" {
		t.Errorf("flow = %+v, erwartet Rohtext &b x", items)
	}
}

// Die anonymisierte Belegdatei führt alle Sorten zusammen: Skalar-Anker,
// Block-Anker, Alias, Merge-Key, Anker an Listeneintrag und vor einer Liste.
// Sie ist der dauerhafte Nachweis; die Messung an fremden Daten (Etappe 5)
// bestätigt nur.
func TestBelegdateiMitAllenAnkersorten(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "anchors.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	root, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	checks := []struct {
		path  []string
		value string
		line  int
	}{
		{[]string{"global", "environment"}, "development", 3},
		{[]string{"global", "timeout"}, "30", 4},
		{[]string{"global", "base", "replicas"}, "2", 6},
		{[]string{"global", "base", "logLevel"}, "info", 7},
		{[]string{"global", "overlay", "<<"}, "*base_config", 9},
		{[]string{"global", "overlay", "logLevel"}, "debug", 10},
		{[]string{"app", "image", "repository"}, "registry.example/team/app", 13},
		{[]string{"app", "image", "tag"}, "1.4.2", 14},
		{[]string{"app", "environment"}, "*default_environment", 15},
		{[]string{"worker", "image"}, "*app_image", 27},
		{[]string{"worker", "timeout"}, "*default_timeout", 29},
	}
	for _, check := range checks {
		node := root.Get(check.path...)
		if node.Str() != check.value {
			t.Errorf("%v = %q, erwartet %q", check.path, node.Str(), check.value)
		}
		if node.At() != check.line {
			t.Errorf("Zeile von %v = %d, erwartet %d", check.path, node.At(), check.line)
		}
	}
	sidecars := root.Get("app", "sidecars").List()
	if len(sidecars) != 3 {
		t.Fatalf("sidecars = %d Einträge, erwartet 3", len(sidecars))
	}
	if got := sidecars[0].Get("image").Str(); got != "registry.example/team/app:1.4.2" {
		t.Errorf("sidecars[0].image = %q", got)
	}
	if got := sidecars[0].Get("image").At(); got != 19 {
		t.Errorf("Zeile von sidecars[0].image = %d, erwartet 19", got)
	}
	if got := sidecars[1].Str(); got != "metrics" || sidecars[1].At() != 20 {
		t.Errorf("sidecars[1] = %q in Zeile %d, erwartet metrics in Zeile 20", got, sidecars[1].At())
	}
	if got := sidecars[2].Get("image").Str(); got != "*app_image" {
		t.Errorf("sidecars[2].image = %q, erwartet den wörtlichen Alias", got)
	}
	hosts := root.Get("app", "hosts").List()
	if len(hosts) != 2 || hosts[1].Str() != "b.example" || hosts[1].At() != 25 {
		t.Errorf("hosts = %+v", hosts)
	}
}

// Etappe 6: Aliase sind eine bewusste Grenze. `*name` an einer Wertposition
// ist ein Skalar mit dem wörtlichen Wert, ohne Hinweis und ohne Abbruch; der
// Merge-Key `<<` ist ein gewöhnliches Feld mit diesem Rohwert.
func TestAliasBleibtWoertlicherSkalar(t *testing.T) {
	for name, check := range map[string]struct {
		document string
		key      string
	}{
		"Alias als Wert": {"base: &base app:1.0\nimage: *base\n", "image"},
		"Merge-Key":      {"base: &base\n  a: 1\n<<: *base\n", "<<"},
	} {
		t.Run(name, func(t *testing.T) {
			root, err := Parse([]byte(check.document))
			if err != nil {
				t.Fatalf("ein Alias darf keinen Hinweis und keinen Abbruch auslösen: %v", err)
			}
			node := root.Get(check.key)
			if node == nil || node.Kind != Scalar {
				t.Fatalf("%s = %+v, erwartet ein Skalar", check.key, node)
			}
			if node.Str() != "*base" {
				t.Errorf("%s = %q, erwartet den wörtlichen Wert *base", check.key, node.Str())
			}
			if node.At() != 3 && check.key == "<<" || node.At() != 2 && check.key == "image" {
				t.Errorf("Zeile von %s = %d — der Alias bleibt an seiner eigenen Zeile", check.key, node.At())
			}
		})
	}
}
