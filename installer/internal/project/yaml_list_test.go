package project

import (
	"strings"
	"testing"
)

// parseYAMLList ist der eine Listenleser für project.languages und
// tools.mcp.required. Geprüft wird vor allem, dass der Pfad genau trifft.
func TestParseYAMLList(t *testing.T) {
	cases := []struct {
		name    string
		content string
		path    []string
		want    []string
		found   bool
	}{
		{
			name:    "Blockform mit Kommentar und Anführungszeichen",
			content: "project:\n  languages:\n    # Sprachen\n    - python\n    - \"go\"\n  vcs: git\n",
			path:    []string{"project", "languages"},
			want:    []string{"python", `"go"`},
			found:   true,
		},
		{
			name:    "Blockform auf Höhe des Schlüssels",
			content: "tools:\n  mcp:\n    required:\n    - a\n    - b\n  gh:\n    status: enabled\n",
			path:    []string{"tools", "mcp", "required"},
			want:    []string{"a", "b"},
			found:   true,
		},
		{
			name:    "Flussform",
			content: "tools:\n  mcp:\n    required: [a, b]\n",
			path:    []string{"tools", "mcp", "required"},
			want:    []string{"a", "b"},
			found:   true,
		},
		{
			name:    "leere Flussform ist gefunden",
			content: "project:\n  languages: []\n",
			path:    []string{"project", "languages"},
			want:    []string{},
			found:   true,
		},
		{
			name:    "tiefer liegender gleichnamiger Schlüssel zählt nicht",
			content: "tools:\n  mcp:\n    servers:\n      x:\n        required: true\n",
			path:    []string{"tools", "mcp", "required"},
			want:    []string{},
			found:   false,
		},
		{
			name:    "Schlüssel unter fremdem Elternteil zählt nicht",
			content: "project:\n  other:\n    languages: [x]\n",
			path:    []string{"project", "languages"},
			want:    []string{},
			found:   false,
		},
		{
			name:    "Schlüssel in einem Listeneintrag zählt nicht",
			content: "tools:\n  mcp:\n    - name: x\n      required: [y]\n",
			path:    []string{"tools", "mcp", "required"},
			want:    []string{},
			found:   false,
		},
		{
			name:    "ein neuer Block auf oberster Ebene beendet die Liste",
			content: "project:\n  languages:\n    - go\ntools:\n  - kein-eintrag\n",
			path:    []string{"project", "languages"},
			want:    []string{"go"},
			found:   true,
		},
		{
			name:    "Liste bis zum Dateiende",
			content: "tools:\n  mcp:\n    required:\n      - a",
			path:    []string{"tools", "mcp", "required"},
			want:    []string{"a"},
			found:   true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			items, found := parseYAMLList(testCase.content, testCase.path...)
			if found != testCase.found {
				t.Errorf("found = %t, erwartet %t", found, testCase.found)
			}
			if items == nil {
				t.Error("items ist nil, erwartet eine leere Liste")
			}
			if strings.Join(items, ",") != strings.Join(testCase.want, ",") {
				t.Errorf("items = %v, erwartet %v", items, testCase.want)
			}
		})
	}
}
