package pathnorm

import "testing"

// TestNormalizeNormiertZielfrei ist der gemeinsame Tabellentest der einen
// Normierung. Er deckt die Eingaben **beider** bisherigen Fassungen ab — der
// aus `review/merge` und der aus `knowndecisions` —, weil die Funktion jetzt
// beide bedient.
//
// Die vier Zeilen mit dem Kommentar „früher Divergenz" sind die Stellen, an
// denen die alte `knowndecisions`-Kopie ein anderes Ergebnis lieferte: sie
// schnitt `file://` per TrimPrefix ab und ließ deshalb den Rechnernamen als
// erstes Segment stehen, und sie löste weder `..` noch `./` mitten im Pfad noch
// doppelte Slashes auf. Wo `merge` zwei Funde zusammenzog, traf eine Decision
// mit `pathGlob` danach nur einen von beiden. Die Divergenz ist damit
// verschwunden, nicht begründet.
func TestNormalizeNormiertZielfrei(t *testing.T) {
	cases := map[string]string{
		"requirements.txt":                      "requirements.txt",
		"/requirements.txt":                     "requirements.txt",
		"./requirements.txt":                    "requirements.txt",
		"file:///home/x/requirements.txt":       "home/x/requirements.txt",
		"file://host/pfad/requirements.txt":     "pfad/requirements.txt", // früher Divergenz: "host/pfad/requirements.txt"
		"FILE:///Home/X/Requirements.txt":       "home/x/requirements.txt",
		"file://requirements.txt":               "requirements.txt",
		`src\pkg\App.go`:                        "src/pkg/app.go",
		"src//pkg/./app.go":                     "src/pkg/app.go", // früher Divergenz: "src//pkg/./app.go"
		"src/pkg/../app.go":                     "src/app.go",     // früher Divergenz: "src/pkg/../app.go"
		"../a/b.txt":                            "../a/b.txt",
		"/../a.txt":                             "a.txt", // früher Divergenz: "../a.txt"
		"":                                      "",
		"//":                                    "",
		"/abs/projekt/tmp/requirements.txt":     "abs/projekt/tmp/requirements.txt",
		"file:///abs/projekt/requirements.txt/": "abs/projekt/requirements.txt",
		"_old/**":                               "_old/**",
		"**/*.py":                               "**/*.py",
	}
	for input, want := range cases {
		if got := Normalize(input); got != want {
			t.Errorf("Normalize(%q) = %q, erwartet %q", input, got, want)
		}
	}
}

// TestNormalizeLaesstGlobMusterHeil sichert die Nutzung in knowndecisions ab:
// dort läuft nicht nur der Fundort, sondern auch das Muster durch dieselbe
// Funktion. Ein Muster darf dabei weder Segmente verlieren noch welche dazu
// bekommen, sonst matcht ein bestehender Eintrag danach etwas anderes.
func TestNormalizeLaesstGlobMusterHeil(t *testing.T) {
	patterns := []string{"_old/**", "**/*.py", "src/**/*.go", "*.md", "**"}
	for _, pattern := range patterns {
		if got := Normalize(pattern); got != pattern {
			t.Errorf("Normalize(%q) = %q — Muster verändert", pattern, got)
		}
	}
}
