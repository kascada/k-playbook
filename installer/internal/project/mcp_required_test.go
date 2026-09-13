package project

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRequiredMCPServers(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		want       []string
		configured bool
		wantErr    bool
	}{
		{
			name:       "ohne Block gilt nichts verlangt",
			content:    "schema_version: 3\n\nproject:\n  repo_root: .\n\ntools:\n  gh:\n    status: enabled\n",
			want:       []string{},
			configured: false,
		},
		{
			name: "Blockform",
			content: `tools:
  mcp:
    # Pflichtserver
    required:
      - k-playbook
      - "atlassian"
`,
			want:       []string{"k-playbook", "atlassian"},
			configured: true,
		},
		{
			name:       "Flussform",
			content:    "tools:\n  mcp:\n    required: [k-playbook, atlassian]\n",
			want:       []string{"k-playbook", "atlassian"},
			configured: true,
		},
		{
			name: "gh vor mcp",
			content: `tools:
  gh:
    status: enabled
  mcp:
    required:
      - k-playbook
`,
			want:       []string{"k-playbook"},
			configured: true,
		},
		{
			name: "gh nach mcp beendet die Liste",
			content: `tools:
  mcp:
    required:
      - k-playbook
  gh:
    status: enabled
`,
			want:       []string{"k-playbook"},
			configured: true,
		},
		{
			name:       "leere Liste ist konfiguriert",
			content:    "tools:\n  mcp:\n    required: []\n",
			want:       []string{},
			configured: true,
		},
		{
			name:       "ein required außerhalb von mcp zählt nicht",
			content:    "tools:\n  gh:\n    required: [x]\n",
			want:       []string{},
			configured: false,
		},
		{
			name:       "ein ungültiger Name ist ein Fehler",
			content:    "tools:\n  mcp:\n    required:\n      - k-playbook\n      - ../boese\n",
			configured: true,
			wantErr:    true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			names, configured, err := parseRequiredMCPServers(testCase.content)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("kein Fehler für %q", testCase.content)
				}
				return
			}
			if err != nil {
				t.Fatalf("unerwarteter Fehler: %v", err)
			}
			if strings.Join(names, ",") != strings.Join(testCase.want, ",") {
				t.Errorf("names = %v, erwartet %v", names, testCase.want)
			}
			if names == nil {
				t.Error("names ist nil, erwartet eine leere Liste")
			}
			if configured != testCase.configured {
				t.Errorf("configured = %t, erwartet %t", configured, testCase.configured)
			}
		})
	}
}

// Das Inventar führt Dateien und Pflichtliste zusammen; ein ungültiger
// Pflichtname kommt als Fehler zurück und lässt die Serverliste stehen.
func TestMCPServerInventoryForLiestPflichtliste(t *testing.T) {
	root := t.TempDir()
	write(t, ConfigPath(root), "schema_version: 3\n\ntools:\n  mcp:\n    required: [k-playbook]\n")
	writeFile(t, filepath.Join(root, ".mcp.json"), `{"mcpServers": {"k-playbook": {"command": "k-playbook", "args": ["mcp"]}}}`)

	inventory, err := MCPServerInventoryFor(root)
	if err != nil {
		t.Fatalf("MCPServerInventoryFor: %v", err)
	}
	if !inventory.RequiredConfigured || len(inventory.Required) != 1 {
		t.Errorf("Pflichtliste = %+v", inventory)
	}
	if len(inventory.Missing) != 2 {
		t.Errorf("Lücken = %+v, erwartet OpenCode und Cursor", inventory.Missing)
	}

	write(t, ConfigPath(root), "schema_version: 3\n\ntools:\n  mcp:\n    required: [k playbook]\n")
	inventory, err = MCPServerInventoryFor(root)
	if err == nil {
		t.Fatal("ein ungültiger Pflichtname wurde nicht gemeldet")
	}
	if len(inventory.Servers) != 1 {
		t.Errorf("die Serverliste ging mit dem Fehler verloren: %+v", inventory.Servers)
	}
}
