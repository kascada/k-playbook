package project

import "testing"

// Die Muster im Stil einer .gitignore (Task 179): verankert und
// unverankert, nur Verzeichnisse, **, Klassen, Verneinung.
func TestIgnoreVerdict(t *testing.T) {
	cases := []struct {
		patterns []string
		path     string
		dir      bool
		want     bool
	}{
		{[]string{"dist/"}, "dist", true, true},
		{[]string{"dist/"}, "dist", false, false},
		{[]string{"dist/"}, "a/dist/x.js", false, true},
		{[]string{"/dist/"}, "a/dist/x.js", false, false},
		{[]string{"/dist/"}, "dist/x.js", false, true},
		{[]string{"*.log"}, "a/b/x.log", false, true},
		{[]string{"*.log", "!keep.log"}, "keep.log", false, false},
		{[]string{"*.log", "!keep.log"}, "a.log", false, true},
		{[]string{"docs/api"}, "docs/api/x.md", false, true},
		{[]string{"docs/api"}, "other/docs/api/x.md", false, false},
		{[]string{"**/*.min.js"}, "x.min.js", false, true},
		{[]string{"**/*.min.js"}, "a/b/x.min.js", false, true},
		{[]string{"a/**/c"}, "a/c", false, true},
		{[]string{"a/**/c"}, "a/b/b/c", false, true},
		{[]string{"a/**"}, "a/b", false, true},
		{[]string{"file?.txt"}, "file1.txt", false, true},
		{[]string{"file?.txt"}, "file12.txt", false, false},
		{[]string{"[ab].txt"}, "b.txt", false, true},
		{[]string{"[!ab].txt"}, "b.txt", false, false},
		{[]string{"# Kommentar", ""}, "x", false, false},
		{[]string{"\\#datei"}, "#datei", false, true},
		{[]string{"*.bundle.*"}, "x.bundle.js", false, true},
	}
	for _, c := range cases {
		rules := compileIgnoreRules(c.patterns, false)
		if _, got := ignoreVerdict(rules, c.path, c.dir); got != c.want {
			t.Errorf("%v auf %s (Verzeichnis %v): %v, erwartet %v", c.patterns, c.path, c.dir, got, c.want)
		}
	}

	// Ohne Verankerung greift ein Muster mit Schrägstrich in der Mitte auf
	// jeder Ebene (tests/fixtures/ der festen Ausschlüsse).
	rules := compileIgnoreRules([]string{"tests/fixtures/"}, true)
	if _, got := ignoreVerdict(rules, "pkg/tests/fixtures/a.json", false); !got {
		t.Error("tests/fixtures/ greift nicht auf tieferer Ebene")
	}
	if rule, _ := ignoreVerdict(rules, "tests/fixtures", true); rule.source != "tests/fixtures/" {
		t.Errorf("Quelle der Regel: %q", rule.source)
	}
}
