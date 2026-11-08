package project

import (
	"os"
	"reflect"
	"testing"
)

// legacyTemplateConfig ist die Vorlage, die CreateConfig vor Task 128 schrieb:
// der auskommentierte Block docs: direkt über tools:, darunter gh als einziger
// Block samt Kommentar.
const legacyTemplateConfig = `# k-playbook

schema_version: 3

project:
  repo_root: .
  vcs: git

remediation:
  mode: task-first

# docs:
#   exclude:
#     - docs/
tools:
  gh:
    # Soll die GitHub CLI genutzt werden? unknown, enabled oder disabled.
    # Ob gh auf diesem Rechner liegt, steht bewusst nicht hier: das ist ein
    # Host-Befund und gehört in die Kontextausgabe.
    status: unknown
`

// Das Einrichten entfernt den alten Block tools.gh samt Kommentar, lässt
// tools.mcp mit seinem Kommentar stehen und meldet das Entfernen in einem
// eigenen Feld. Der zweite Lauf findet nichts mehr und meldet nichts.
func TestApplyAssistantSetupEntferntAltenGHBlock(t *testing.T) {
	root := newProject(t)
	writeFile(t, ConfigPath(root), `schema_version: 3

tools:
  # Pflichtliste der MCP-Server.
  mcp:
    required: [k-playbook]
  gh:
    # Soll die GitHub CLI genutzt werden?
    status: disabled

# Umschalten der Branches.
git:
  switch: unknown
`)

	setup, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatalf("ApplyAssistantSetup: %v", err)
	}
	if !reflect.DeepEqual(setup.ConfigBlocksRemoved, []string{LegacyGHBlockName}) {
		t.Errorf("ConfigBlocksRemoved = %v, erwartet [%s]", setup.ConfigBlocksRemoved, LegacyGHBlockName)
	}
	want := `schema_version: 3

tools:
  # Pflichtliste der MCP-Server.
  mcp:
    required: [k-playbook]

# Umschalten der Branches.
git:
  switch: unknown
`
	if got := readFile(t, ConfigPath(root)); got != want {
		t.Errorf("K-PLAYBOOK.yaml nach dem Einrichten:\n%s\nerwartet:\n%s", got, want)
	}

	second, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if len(second.ConfigBlocksRemoved) != 0 {
		t.Errorf("zweiter Lauf meldet %v", second.ConfigBlocksRemoved)
	}
	if got := readFile(t, ConfigPath(root)); got != want {
		t.Errorf("zweiter Lauf ändert die Datei:\n%s", got)
	}
}

// Bleibt unter tools: nichts, fällt auch tools:. Der auskommentierte Block
// docs: direkt darüber gehört nicht dazu und bleibt.
func TestApplyAssistantSetupEntferntLeeresTools(t *testing.T) {
	root := newProject(t)
	writeFile(t, ConfigPath(root), legacyTemplateConfig)

	setup, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatalf("ApplyAssistantSetup: %v", err)
	}
	if !reflect.DeepEqual(setup.ConfigBlocksRemoved, []string{LegacyGHBlockName}) {
		t.Errorf("ConfigBlocksRemoved = %v", setup.ConfigBlocksRemoved)
	}
	want := `# k-playbook

schema_version: 3

project:
  repo_root: .
  vcs: git

remediation:
  mode: task-first

# docs:
#   exclude:
#     - docs/
`
	if got := readFile(t, ConfigPath(root)); got != want {
		t.Errorf("K-PLAYBOOK.yaml nach dem Einrichten:\n%s\nerwartet:\n%s", got, want)
	}
	if _, err := BuildContext(root); err != nil {
		t.Errorf("BuildContext nach dem Entfernen: %v", err)
	}
}

// Ohne Block tools.gh bleibt die Datei unberührt — Inhalt und Zeitstempel —,
// und das Feld bleibt leer.
func TestApplyAssistantSetupOhneGHBlockUnberuehrt(t *testing.T) {
	root := newProject(t)
	content := "schema_version: 3\n\ntools:\n  mcp:\n    required: []\n"
	writeFile(t, ConfigPath(root), content)
	before, err := os.Stat(ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}

	setup, err := ApplyAssistantSetup(root)
	if err != nil {
		t.Fatalf("ApplyAssistantSetup: %v", err)
	}
	if setup.ConfigBlocksRemoved != nil {
		t.Errorf("ConfigBlocksRemoved = %v, erwartet leer", setup.ConfigBlocksRemoved)
	}
	after, err := os.Stat(ConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, ConfigPath(root)); got != content || !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("Datei verändert:\n%s", got)
	}
	if LegacyGHBlockPresent(root) {
		t.Error("LegacyGHBlockPresent ohne Block")
	}
}

func TestRemoveLegacyGHBlockGrenzen(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
		removed bool
	}{
		{
			name:    "ohne tools",
			content: "schema_version: 3\n",
			want:    "schema_version: 3\n",
		},
		{
			name:    "gh tiefer unter einem anderen Tool zählt nicht",
			content: "tools:\n  beispiel:\n    gh:\n      status: enabled\n",
			want:    "tools:\n  beispiel:\n    gh:\n      status: enabled\n",
		},
		{
			name:    "gh als erster Block, Nachbar mit Kommentar bleibt",
			content: "tools:\n  gh:\n    status: enabled\n  # zu mcp\n  mcp:\n    required: []\n",
			want:    "tools:\n  # zu mcp\n  mcp:\n    required: []\n",
			removed: true,
		},
		{
			name:    "Kommentar über gh fällt mit",
			content: "tools:\n  mcp:\n    required: []\n  # GitHub CLI\n  gh:\n    status: enabled\n",
			want:    "tools:\n  mcp:\n    required: []\n",
			removed: true,
		},
		{
			name:    "tools in der Mitte fällt ganz, der Kommentar des Nachbarn bleibt",
			content: "schema_version: 3\n\ntools:\n  gh:\n    status: vielleicht\n\n# zu git\ngit:\n  switch: unknown\n",
			want:    "schema_version: 3\n\n# zu git\ngit:\n  switch: unknown\n",
			removed: true,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, removed := removeLegacyGHBlock(testCase.content)
			if removed != testCase.removed || got != testCase.want {
				t.Errorf("removed = %t, Inhalt:\n%q\nerwartet %t:\n%q", removed, got, testCase.removed, testCase.want)
			}
		})
	}
}
