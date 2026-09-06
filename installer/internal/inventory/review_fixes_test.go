package inventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func collectFiles(t *testing.T, files map[string]string) Result {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Collect(Options{ProjectDir: root, SourcesFile: filepath.Join(root, "version-sources.yaml")})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func entriesFrom(result Result, file string) []Entry {
	var entries []Entry
	for _, entry := range result.Entries {
		if entry.SourceFile == file {
			entries = append(entries, entry)
		}
	}
	return entries
}

func names(entries []Entry) map[string]bool {
	got := map[string]bool{}
	for _, entry := range entries {
		got[entry.Name] = true
	}
	return got
}

func requireNames(t *testing.T, entries []Entry, wanted ...string) {
	t.Helper()
	got := names(entries)
	for _, name := range wanted {
		if !got[name] {
			t.Errorf("direkte Abhängigkeit %q fehlt: %+v", name, entries)
		}
	}
	for name := range got {
		if !contains(wanted, name) {
			t.Errorf("unerwartete oder transitive Abhängigkeit %q: %+v", name, entries)
		}
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func noteContains(result Result, fragments ...string) bool {
	for _, note := range result.Notes {
		matches := true
		for _, fragment := range fragments {
			if !strings.Contains(note.Text, fragment) {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

// Jede betroffene Lockfile-Art erhält eigene direkte und transitive Pakete.
// Damit der Test nicht versehentlich auf einen einzelnen Parser zugeschnitten
// ist, leitet die Tabelle die geprüften Lockfiles aus lockManifest ab.
func TestLockfilesFuehrenNurDirekteManifestAbhaengigkeiten(t *testing.T) {
	files := map[string]string{
		"node/package.json":     `{"dependencies":{"node-direct":"^1"}}`,
		"node/yarn.lock":        "node-direct@^1:\n  version \"1.0.0\"\nnode-transitive@^1:\n  version \"2.0.0\"\n",
		"python/pyproject.toml": "[project]\ndependencies = [\"python-direct>=1\"]\n",
		"python/poetry.lock":    "[[package]]\nname = \"python-direct\"\nversion = \"1.0.0\"\n\n[[package]]\nname = \"python-transitive\"\nversion = \"2.0.0\"\n",
		"uv/pyproject.toml":     "[project]\ndependencies = [\"uv-direct>=1\"]\n",
		"uv/uv.lock":            "[[package]]\nname = \"uv-direct\"\nversion = \"1.0.0\"\n\n[[package]]\nname = \"uv-transitive\"\nversion = \"2.0.0\"\n",
		"rust/Cargo.toml":       "[dependencies]\nrust-direct = \"1\"\n",
		"rust/Cargo.lock":       "[[package]]\nname = \"rust-direct\"\nversion = \"1.0.0\"\n\n[[package]]\nname = \"rust-transitive\"\nversion = \"2.0.0\"\n",
		"pip/Pipfile":           "[packages]\npip-direct = \"*\"\n",
		"pip/Pipfile.lock":      `{"default":{"pip-direct":{"version":"==1.0.0"},"pip-transitive":{"version":"==2.0.0"}}}`,
		"php/composer.json":     `{"require":{"vendor/php-direct":"^1"}}`,
		"php/composer.lock":     `{"packages":[{"name":"vendor/php-direct","version":"1.0.0"},{"name":"vendor/php-transitive","version":"2.0.0"}]}`,
		"elixir/mix.exs":        "def project, do: [deps: [{:elixir_direct, \"~> 1.0\"}]]\n",
		"elixir/mix.lock":       `{"elixir_direct": {:hex, :elixir_direct, "1.0.0"}, "elixir_transitive": {:hex, :elixir_transitive, "2.0.0"}}`,
		// A second manifest/lock pair verifies that references are local to a pair.
		"other/package.json": `{"dependencies":{"other-direct":"^1"}}`,
		"other/yarn.lock":    "other-direct@^1:\n  version \"1.0.0\"\nnode-direct@^1:\n  version \"9.9.9\"\n",
	}
	result := collectFiles(t, files)
	cases := map[string][]string{
		"node/yarn.lock":     {"node-direct"},
		"python/poetry.lock": {"python-direct"},
		"uv/uv.lock":         {"uv-direct"},
		"rust/Cargo.lock":    {"rust-direct"},
		"pip/Pipfile.lock":   {"pip-direct"},
		"php/composer.lock":  {"vendor/php-direct"},
		"elixir/mix.lock":    {"elixir_direct"},
		"other/yarn.lock":    {"other-direct"},
	}
	for lock, wanted := range cases {
		t.Run(lock, func(t *testing.T) { requireNames(t, entriesFrom(result, lock), wanted...) })
	}

	for _, lock := range affectedLockfiles() {
		tested := false
		for path := range cases {
			if filepath.Base(path) == lock {
				tested = true
				break
			}
		}
		if !tested {
			t.Errorf("%s hat keinen Direktheits-Test", lock)
		}
	}
}

func affectedLockfiles() []string {
	var locks []string
	for base := range lockManifests {
		locks = append(locks, base)
	}
	return locks
}

func TestWorkspaceLockfilesVereinigenMitgliedsabhaengigkeiten(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"node/package.json":            `{"workspaces":["packages/*"]}`,
		"node/packages/a/package.json": `{"dependencies":{"node-a":"1"}}`,
		"node/packages/b/package.json": `{"devDependencies":{"node-b":"1"}}`,
		"node/yarn.lock":               "node-a@1:\n  version \"1.0.0\"\nnode-b@1:\n  version \"1.0.0\"\nnode-transitive@1:\n  version \"1.0.0\"\n",
		"rust/Cargo.toml":              "[workspace]\nmembers = [\"members/a\"]\n[workspace.dependencies]\nshared = \"1\"\n",
		"rust/members/a/Cargo.toml":    "[dependencies]\nmember = \"1\"\nshared = { workspace = true }\n",
		"rust/Cargo.lock":              "[[package]]\nname = \"member\"\nversion = \"1.0.0\"\n\n[[package]]\nname = \"shared\"\nversion = \"1.0.0\"\n\n[[package]]\nname = \"rust-transitive\"\nversion = \"1.0.0\"\n",
		"uv/pyproject.toml":            "[tool.uv.workspace]\nmembers = [\"members/a\", \"members/b\"]\n",
		"uv/members/a/pyproject.toml":  "[project]\ndependencies = [\"uv-a>=1\"]\n",
		"uv/members/b/pyproject.toml":  "[project]\ndependencies = [\"uv-b>=1\"]\n",
		"uv/uv.lock":                   "[[package]]\nname = \"uv-a\"\nversion = \"1.0.0\"\n\n[[package]]\nname = \"uv-b\"\nversion = \"1.0.0\"\n\n[[package]]\nname = \"uv-transitive\"\nversion = \"1.0.0\"\n",
		"mix/mix.exs":                  "def project, do: [apps_path: \"components\"]\n",
		"mix/components/a/mix.exs":     "def project, do: [deps: [{:mix_a, \"~> 1\"}]]\n",
		"mix/components/b/mix.exs":     "def project, do: [deps: [{:mix_b, \"~> 1\"}]]\n",
		"mix/mix.lock":                 "{\n  \"mix_a\": {:hex, :mix_a, \"1.0.0\"},\n  \"mix_b\": {:hex, :mix_b, \"1.0.0\"},\n  \"mix_transitive\": {:hex, :mix_transitive, \"1.0.0\"}\n}",
	})
	requireNames(t, entriesFrom(result, "node/yarn.lock"), "node-a", "node-b")
	requireNames(t, entriesFrom(result, "rust/Cargo.lock"), "member", "shared")
	requireNames(t, entriesFrom(result, "uv/uv.lock"), "uv-a", "uv-b")
	requireNames(t, entriesFrom(result, "mix/mix.lock"), "mix_a", "mix_b")
}

func TestCargoWorkspaceDependenciesRequireWorkspaceTrue(t *testing.T) {
	direct := map[string]string{}
	root := []byte("[workspace.dependencies]\nshared = \"1\"\nnot-shared = \"1\"\n")
	member := []byte("[dependencies]\nshared = { workspace = true }\nnot-shared = { workspace = false, optional = true }\n")

	addCargoWorkspaceDependencies(direct, root, member)

	if direct["shared"] != "main" {
		t.Errorf("workspace = true muss die Wurzel-Abhängigkeit ergänzen: %+v", direct)
	}
	if _, found := direct["not-shared"]; found {
		t.Errorf("workspace = false darf die Wurzel-Abhängigkeit nicht ergänzen: %+v", direct)
	}
}

func TestWorkspaceFehlerUndFehlendeOderAusgeschlosseneManifesteLeerenLockfile(t *testing.T) {
	cases := []struct {
		name, lock, expected, sources string
		files                         map[string]string
	}{
		{"nicht auflösbares Mitglied", "yarn.lock", "Workspace-Manifest packages/missing/package.json", "", map[string]string{
			"package.json": `{"workspaces":["packages/missing"]}`, "yarn.lock": "direct@1:\n  version \"1.0.0\"\n",
		}},
		{"ausgeschlossenes Mitglied", "yarn.lock", "Ausschlussregel \"packages/private/**\"", "schema_version: 1\nexclude:\n  - packages/private/**\n", map[string]string{
			"package.json": `{"workspaces":["packages/private"]}`, "packages/private/package.json": `{"dependencies":{"direct":"1"}}`, "yarn.lock": "direct@1:\n  version \"1.0.0\"\n",
		}},
		{"unlesbares Mitglied", "Cargo.lock", "ist ein Verzeichnis", "", map[string]string{
			"Cargo.toml": "[workspace]\nmembers = [\"members/bad\"]\n", "members/bad/Cargo.toml/keep": "not a manifest", "Cargo.lock": "[[package]]\nname = \"direct\"\nversion = \"1.0.0\"\n",
		}},
		{"fehlendes Manifest", "yarn.lock", "package.json", "", map[string]string{"yarn.lock": "direct@1:\n  version \"1.0.0\"\n"}},
		{"ausgeschlossenes Manifest", "yarn.lock", "Ausschlussregel \"package.json\"", "schema_version: 1\nexclude:\n  - package.json\n", map[string]string{
			"package.json": `{"dependencies":{"direct":"1"}}`, "yarn.lock": "direct@1:\n  version \"1.0.0\"\n",
		}},
		{"konfiguriertes Lockfile inkludiert Manifest nicht", "yarn.lock", "Ausschlussregel \"package.json\"", "schema_version: 1\nexclude:\n  - package.json\nsources:\n  - path: yarn.lock\n    kind: node\n    env: lokal\n", map[string]string{
			"package.json": `{"dependencies":{"direct":"1"}}`, "yarn.lock": "direct@1:\n  version \"1.0.0\"\n",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{}
			for path, data := range tc.files {
				files[path] = data
			}
			if tc.sources != "" {
				files["version-sources.yaml"] = tc.sources
			}
			result := collectFiles(t, files)
			if entries := entriesFrom(result, tc.lock); len(entries) != 0 {
				t.Fatalf("Lockfile darf nichts beitragen: %+v", entries)
			}
			if !noteContains(result, strings.Fields(tc.expected)...) {
				t.Errorf("sichtbarer Hinweis mit %q fehlt: %+v", tc.expected, result.Notes)
			}
		})
	}
}

func TestDockerfileFlagsStagesUndScopedNPM(t *testing.T) {
	entries, notes := parseFile(fileContext{Display: "Dockerfile", Base: "Dockerfile", Kind: KindDockerfile, Env: EnvDeployment, EnvOrigin: ContextDefault, Data: []byte("FROM --platform=$BUILDPLATFORM golang:1.22 AS build\nFROM build AS final\nRUN npm install @scope/install@1.2.3\nRUN npm i @scope/i@2.3.4\nRUN yarn add @scope/yarn@3.4.5\n")})
	if len(notes) != 0 {
		t.Fatalf("Hinweise = %+v", notes)
	}
	if len(entries) != 5 {
		t.Fatalf("Einträge = %+v", entries)
	}
	if entries[0].Name != "golang" || entries[0].Version != "1.22" {
		t.Errorf("FROM mit Flag = %+v", entries[0])
	}
	if entries[1].Pin != PinLocal || entries[1].Name != "build" {
		t.Errorf("lokale Stage = %+v", entries[1])
	}
	for index, name := range []string{"@scope/install", "@scope/i", "@scope/yarn"} {
		if entries[index+2].Name != name {
			t.Errorf("Scoped NPM-Pin = %+v", entries[index+2])
		}
	}
}

func TestConfiguredSourceNoteWirdMarkdownSicherGerendert(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "requirements.txt"), []byte("redis==1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sources := filepath.Join(root, "sources.yaml")
	if err := os.WriteFile(sources, []byte("schema_version: 1\nsources:\n  - path: requirements.txt\n    kind: python\n    env: lokal\n    note: Produktionsquelle | `geprüft`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Collect(Options{ProjectDir: root, SourcesFile: sources})
	if err != nil {
		t.Fatal(err)
	}
	rendered := Render(result, "2026-09-06T00:00:00Z")
	if !strings.Contains(rendered, "Produktionsquelle \\| `geprüft`") {
		t.Errorf("Note ist nicht Markdown-sicher: %s", rendered)
	}
}

func TestReadStatusVollstaendigUnvollstaendigUndDefekt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inventory.md")
	full := Render(Result{}, "2026-09-06T00:00:00Z")
	cases := []struct {
		name, content string
		problem       bool
	}{
		{"vollständig", full, false},
		{"unvollständig", "---\ntype: Version Inventory\n---\n", true},
		{"syntaktisch defekt", "---\ntype: [\n---\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := ReadStatus(path).Problem != ""; got != tc.problem {
				t.Errorf("Problem = %v, erwartet %v: %+v", got, tc.problem, ReadStatus(path))
			}
		})
	}
}

func TestCorruptInventoryIsRewritten(t *testing.T) {
	options := newRunProject(t)
	if err := os.MkdirAll(filepath.Dir(options.InventoryFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(options.InventoryFile, []byte("---\ntype: [\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, outcome, err := Run(options)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Written || outcome.Problem == "" {
		t.Errorf("defekter Bestand muss sichtbar neu geschrieben werden: %+v", outcome)
	}
	if status := ReadStatus(options.InventoryFile); status.Problem != "" {
		t.Errorf("neu geschriebener Bestand = %q", status.Problem)
	}
}
