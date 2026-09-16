package inventory

import (
	"encoding/json"
	"strings"
	"testing"
)

// Die Erwartungen folgen dem npm-Paket `semver` ohne Optionen
// (`semver.satisfies(version, range)`).
func TestNPMSatisfies(t *testing.T) {
	cases := []struct {
		declaration string
		version     string
		want        bool
	}{
		// exakte Angaben
		{"1.2.3", "1.2.3", true},
		{"1.2.3", "1.2.4", false},
		{"=1.2.3", "1.2.3", true},
		{"v1.2.3", "1.2.3", true},
		{"1.2.3+build.5", "1.2.3", true},
		{"1.2.3-beta.1", "1.2.3-beta.1", true},
		{"1.2.3-beta.1", "1.2.3", false},

		// Platzhalter und Teilangaben
		{"*", "3.17.1", true},
		{"", "3.17.1", true},
		{"x", "0.0.1", true},
		{"X", "10.0.0", true},
		{"1", "1.9.9", true},
		{"1", "2.0.0", false},
		{"1.x", "1.0.0", true},
		{"1.2", "1.2.9", true},
		{"1.2", "1.3.0", false},
		{"1.2.x", "1.2.0", true},
		{"1.2.*", "1.3.0", false},

		// Caret
		{"^3.17.1", "3.17.1", true},
		{"^3.17.1", "3.99.0", true},
		{"^3.17.1", "3.17.0", false},
		{"^3.17.1", "4.0.0", false},
		{"^3.14.8", "3.17.1", true},
		{"^0.2.3", "0.2.9", true},
		{"^0.2.3", "0.3.0", false},
		{"^0.0.3", "0.0.3", true},
		{"^0.0.3", "0.0.4", false},
		{"^0.x", "0.9.0", true},
		{"^0.x", "1.0.0", false},
		{"^0.0.x", "0.0.9", true},
		{"^0.0.x", "0.1.0", false},
		{"^0", "0.5.0", true},
		{"^1.2", "1.9.0", true},
		{"^0.2", "0.3.0", false},
		{"^1.2.3-beta.2", "1.2.3-beta.3", true},
		{"^1.2.3-beta.2", "1.2.3-beta.1", false},
		{"^1.2.3-beta.2", "1.5.0", true},

		// Tilde
		{"~1.2.3", "1.2.9", true},
		{"~1.2.3", "1.3.0", false},
		{"~1.2", "1.2.0", true},
		{"~1", "1.9.0", true},
		{"~1", "2.0.0", false},
		{"~>1.2.3", "1.2.4", true},
		{"~0.2.3", "0.3.0", false},

		// Vergleiche und Verknüpfungen
		{">=3", "3.17.1", true},
		{">=3", "2.9.9", false},
		{"> 1.2.3", "1.2.4", true},
		{">1.2", "1.2.9", false},
		{">1.2", "1.3.0", true},
		{">1", "2.0.0", true},
		{"<2", "1.9.9", true},
		{"<2", "2.0.0", false},
		{"<=1.2", "1.2.9", true},
		{"<=1.2", "1.3.0", false},
		{">=1.2.3 <2.0.0", "1.9.0", true},
		{">=1.2.3 <2.0.0", "2.0.0", false},
		{">= 1.2.3  < 2", "1.2.3", true},
		{"^1.0.0 || ^2.0.0", "2.1.0", true},
		{"^1.0.0 || ^2.0.0", "3.0.0", false},
		{"1.2.3 - 2.3.4", "2.3.4", true},
		{"1.2.3 - 2.3.4", "2.3.5", false},
		{"1.2 - 2", "2.9.0", true},
		{"1.2 - 2", "1.1.9", false},

		// Vorabversionen nach npm
		{"*", "1.0.0-beta", false},
		{"^1.2.3", "1.5.0-beta", false},
		{"^1.2.3", "2.0.0-beta", false},
		{">=1.0.0-0", "1.0.0-beta", true},
		{">=1.0.0-0 <2.0.0", "1.5.0-beta", false},
		{"<2.0.0", "2.0.0-beta", false},
	}
	for _, tc := range cases {
		satisfied, checkable := npmSatisfies(tc.declaration, tc.version)
		if !checkable {
			t.Errorf("%q gegen %s: nicht prüfbar, erwartet prüfbar", tc.declaration, tc.version)
			continue
		}
		if satisfied != tc.want {
			t.Errorf("%q gegen %s = %v, erwartet %v", tc.declaration, tc.version, satisfied, tc.want)
		}
	}
}

// Was keine sicher lesbare Angabe ist, wird nicht geprüft, sondern als nicht
// prüfbar gemeldet.
func TestNPMNichtPruefbareFormen(t *testing.T) {
	for _, declaration := range []string{
		"npm:string-width@^4.2.0",
		"workspace:*",
		"workspace:^1.0.0",
		"link:../lib",
		"file:../lib",
		"git+https://github.com/owner/repo.git#v1.0.0",
		"git://github.com/owner/repo.git",
		"github:owner/repo",
		"owner/repo",
		"owner/repo#semver:^1.0.0",
		"https://example.io/package.tgz",
		"latest",
		"next",
		"beta",
		"1.2.3beta",
		">=1.2.3,<2",
		"^^1.2.3",
		"1.2.3.4",
		">=",
		"1.x-beta",
	} {
		if _, checkable := npmSatisfies(declaration, "1.2.3"); checkable {
			t.Errorf("%q gilt als prüfbar, erwartet nicht prüfbar", declaration)
		}
	}
	for _, version := range []string{"", "1.2", "latest", "file:../lib", "1.2.3.4"} {
		if _, checkable := npmSatisfies("^1.0.0", version); checkable {
			t.Errorf("Lock-Version %q gilt als prüfbar", version)
		}
	}
}

func TestCompareSemverVorabkennungen(t *testing.T) {
	ordered := []string{"1.0.0-0", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "2.0.0"}
	for index := 1; index < len(ordered); index++ {
		left, _ := parseSemver(ordered[index-1])
		right, _ := parseSemver(ordered[index])
		if compareSemver(left, right) >= 0 || compareSemver(right, left) <= 0 {
			t.Errorf("%s muss vor %s stehen", ordered[index-1], ordered[index])
		}
	}
}

// Jede Zeile aus package-lock.json kennt ihr package.json im selben
// Verzeichnis; Zeilen aus yarn.lock und pnpm-lock.yaml bleiben ohne
// Verknüpfung, und die Verknüpfung steht nicht im JSON.
func TestLockZeilenKennenIhrManifest(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"package.json":                       `{"dependencies":{"alpinejs":"^3.14.8"}}`,
		"package-lock.json":                  `{"packages":{"":{"dependencies":{"alpinejs":"^3.14.8"}},"node_modules/alpinejs":{"version":"3.14.8"}}}`,
		"theme/static_src/package.json":      `{"devDependencies":{"alpinejs":"^3.17.1"}}`,
		"theme/static_src/package-lock.json": `{"packages":{"":{"devDependencies":{"alpinejs":"^3.17.1"}},"node_modules/alpinejs":{"version":"3.17.1"}}}`,
		"yarn/package.json":                  `{"dependencies":{"a":"^1"}}`,
		"yarn/yarn.lock":                     "a@^1:\n  version \"1.0.0\"\n",
		"pnpm/pnpm-lock.yaml":                "importers:\n  .:\n    dependencies:\n      b:\n        specifier: ^2\n        version: 2.0.0\n",
	})

	for file, manifest := range map[string]string{
		"package-lock.json":                  "package.json",
		"theme/static_src/package-lock.json": "theme/static_src/package.json",
		"yarn/yarn.lock":                     "",
		"pnpm/pnpm-lock.yaml":                "",
		"package.json":                       "",
	} {
		entries := entriesFrom(result, file)
		if len(entries) == 0 {
			t.Fatalf("%s: keine Zeilen", file)
		}
		for _, entry := range entries {
			if entry.Manifest != manifest {
				t.Errorf("%s: Manifest %q, erwartet %q", file, entry.Manifest, manifest)
			}
		}
	}

	data, err := json.Marshal(result.Entries)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(data)), "manifest") {
		t.Errorf("die Verknüpfung gehört nicht ins JSON: %s", data)
	}
}
