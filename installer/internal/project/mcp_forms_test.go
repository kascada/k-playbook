package project

import (
	"os"
	"path/filepath"
	"testing"
)

// InstalledCommandPath nimmt das, was bin/install hinlegt: ~/.local/bin/k-playbook.
func TestInstalledCommandPathFindetDasInstallierteBinary(t *testing.T) {
	installed := installTestBinary(t)

	got, err := InstalledCommandPath()
	if err != nil {
		t.Fatalf("InstalledCommandPath: %v", err)
	}
	if got != installed {
		t.Errorf("Pfad = %q, erwartet %q", got, installed)
	}

	command, args, err := MCPCommand()
	if err != nil {
		t.Fatalf("MCPCommand: %v", err)
	}
	if !filepath.IsAbs(command) {
		t.Errorf("Kommando = %q, erwartet einen absoluten Pfad", command)
	}
	if command != installed {
		t.Errorf("Kommando = %q, erwartet %q", command, installed)
	}
	if len(args) != 1 || args[0] != MCPSubcommand {
		t.Errorf("Argumente = %v, erwartet [%s]", args, MCPSubcommand)
	}
}

// Eine nicht ausführbare Datei ist keine Installation: dann gibt es kein
// Kommando, und geschrieben wird nichts.
func TestInstalledCommandPathOhneInstallation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, err := InstalledCommandPath(); err == nil {
		t.Fatal("ohne installiertes Binary wurde ein Pfad gemeldet")
	}
	if _, _, err := MCPCommand(); err == nil {
		t.Fatal("ohne installiertes Binary wurde ein Kommando gemeldet")
	}
}

// Der Vertrag in einer Tabelle: was als aktuell gilt, was im engen Sinn
// veraltet ist und was beides nicht ist.
func TestMengeDerAkzeptiertenEintragsformen(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		accepted bool
		outdated bool
	}{
		{
			name:     "aufgelöster absoluter Pfad",
			command:  "/home/wer/.local/bin/k-playbook",
			accepted: true,
		},
		{
			// Der Kern des Ping-Pong-Schutzes: Host und DevContainer teilen
			// sich dieselbe Datei, aber nicht dasselbe HOME.
			name:     "absoluter Pfad eines fremden HOME",
			command:  "/Users/wer/.local/bin/k-playbook",
			accepted: true,
		},
		{
			name:     "absoluter Pfad außerhalb von ~/.local/bin",
			command:  "/opt/k-playbook/k-playbook",
			accepted: true,
		},
		{
			name:     "alter Wrapper, relativ",
			command:  "k-playbook/bin/k-playbook",
			outdated: true,
		},
		{
			name:     "alter Wrapper, mit ./ davor",
			command:  "./k-playbook/bin/k-playbook",
			outdated: true,
		},
		{
			name:     "alter Wrapper ohne Installationsverzeichnis",
			command:  "bin/k-playbook",
			outdated: true,
		},
		{
			name:     "alter Wrapper, absolut",
			command:  "/home/wer/projekt/k-playbook/bin/k-playbook",
			outdated: true,
		},
		{
			// Die Falle: ~/.local/bin/k-playbook endet ebenfalls auf
			// bin/k-playbook. Als veraltet zählte es endlos neu geschrieben.
			name:     "installiertes Binary ist kein alter Wrapper",
			command:  "/home/wer/.local/bin/k-playbook",
			accepted: true,
		},
		{
			// Die portable Form: die eincheckbare Registrierung dieses
			// Quell-Repos. Sie trägt kein $HOME und gilt deshalb auf dem Host
			// wie im DevContainer.
			name:     "bloßer Kommandoname",
			command:  "k-playbook",
			accepted: true,
		},
		{
			// Nur der Name selbst, nicht irgendein Pfad, der darauf endet:
			// sonst wäre auch ein relativer Projektpfad plötzlich „aktuell".
			name:    "Kommandoname mit ./ davor",
			command: "./k-playbook",
		},
		{
			name:    "fremdes Kommando",
			command: "/opt/fremd/dienst",
		},
		{
			name:    "leer",
			command: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isAcceptedMCPCommand(test.command); got != test.accepted {
				t.Errorf("isAcceptedMCPCommand(%q) = %v, erwartet %v", test.command, got, test.accepted)
			}
			if got := isOutdatedMCPCommand(test.command); got != test.outdated {
				t.Errorf("isOutdatedMCPCommand(%q) = %v, erwartet %v", test.command, got, test.outdated)
			}
			if test.accepted && test.outdated {
				t.Fatal("akzeptiert und veraltet schließen einander aus")
			}
		})
	}
}

// Ein Eintrag mit zusätzlichen Schlüsseln — etwa einem von Hand gesetzten
// timeout — bleibt gültig. Der frühere Vergleich auf strukturelle Gleichheit
// hätte ihn bei jedem Lauf weggeschrieben.
func TestAkzeptierteFormVertraegtZusaetzlicheSchluessel(t *testing.T) {
	entry := map[string]any{
		"type":    "local",
		"command": []any{"/home/wer/.local/bin/k-playbook", "mcp"},
		"enabled": true,
		"timeout": float64(90000),
	}
	if !mcpEntryUpToDate(MCPSchemaOpenCode, entry) {
		t.Error("Eintrag mit timeout gilt nicht als aktuell")
	}
}

// Als aktuell gilt eine Menge von Formen — auch die eines fremden HOME. Ein
// ausdrückliches Einrichten schreibt eine solche Datei deshalb nicht um.
func TestApplyMCPLaesstAkzeptierteFormUnveraendert(t *testing.T) {
	root := newMCPProject(t)
	fremd := `{"mcpServers":{"` + MCPServerKey + `":{"command":"/Users/fremd/.local/bin/k-playbook","args":["mcp"]}}}` + "\n"
	writeFile(t, filepath.Join(root, ".mcp.json"), fremd)

	if status := mcpStatusFor(t, CheckMCP(root), ".mcp.json"); !status.OK() {
		t.Fatalf("fremder absoluter Pfad gilt nicht als aktuell: %+v", status)
	}

	if _, err := ApplyMCP(root); err != nil {
		t.Fatalf("ApplyMCP: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatalf(".mcp.json lesen: %v", err)
	}
	if string(raw) != fremd {
		t.Errorf("akzeptierte Form wurde überschrieben:\n%s", raw)
	}
}

// Der eng definierte Fall: der alte Wrapper wird als veraltet gemeldet.
func TestCheckMCPMeldetDenAltenWrapperAlsVeraltet(t *testing.T) {
	root := newMCPProject(t)
	writeFile(t, filepath.Join(root, ".mcp.json"),
		`{"mcpServers":{"`+MCPServerKey+`":{"command":"k-playbook/bin/k-playbook","args":["mcp"]}}}`+"\n")

	status := mcpStatusFor(t, CheckMCP(root), ".mcp.json")
	if status.State != MCPStateOutdated {
		t.Fatalf("State = %q, erwartet %q", status.State, MCPStateOutdated)
	}
}

// Die Auto-Korrektur greift genau einmal und genau dort: der Wrapper-Eintrag
// wird ersetzt, die akzeptierte Form daneben bleibt Byte für Byte stehen.
func TestRepairMCPKorrigiertNurVeralteteEintraege(t *testing.T) {
	installed := installTestBinary(t)
	root := t.TempDir()

	veraltet := filepath.Join(root, ".mcp.json")
	writeFile(t, veraltet,
		`{"mcpServers":{"`+MCPServerKey+`":{"command":"k-playbook/bin/k-playbook","args":["mcp"]}}}`+"\n")

	portabel := `{"mcpServers":{"` + MCPServerKey + `":{"command":"/Users/fremd/.local/bin/k-playbook","args":["mcp"]}}}` + "\n"
	writeFile(t, filepath.Join(root, ".cursor", "mcp.json"), portabel)

	repaired, err := RepairMCP(root)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 1 || repaired[0] != ".mcp.json" {
		t.Fatalf("korrigiert = %v, erwartet nur .mcp.json", repaired)
	}

	entry := readJSON(t, veraltet)["mcpServers"].(map[string]any)[MCPServerKey].(map[string]any)
	if entry["command"] != installed {
		t.Errorf("command = %v, erwartet %q", entry["command"], installed)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".cursor", "mcp.json"))
	if err != nil {
		t.Fatalf(".cursor/mcp.json lesen: %v", err)
	}
	if string(raw) != portabel {
		t.Errorf("akzeptierte Form wurde von der Auto-Korrektur angefasst:\n%s", raw)
	}

	// opencode.json fehlte und wurde nicht angelegt: die Auto-Korrektur
	// repariert, sie richtet nicht ein.
	if _, err := os.Stat(filepath.Join(root, "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("opencode.json wurde von der Auto-Korrektur angelegt: %v", err)
	}
}

// Idempotenz: der zweite Lauf meldet nichts und fasst nichts an. Ohne das
// machte jeder Start die getrackten MCP-Dateien eines Projekts dreckig.
func TestRepairMCPIstIdempotent(t *testing.T) {
	root := newMCPProject(t)
	writeFile(t, filepath.Join(root, ".mcp.json"),
		`{"mcpServers":{"`+MCPServerKey+`":{"command":"k-playbook/bin/k-playbook","args":["mcp"]}}}`+"\n")

	if _, err := RepairMCP(root); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatalf(".mcp.json lesen: %v", err)
	}

	repaired, err := RepairMCP(root)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("zweiter Lauf hat erneut korrigiert: %v", repaired)
	}

	second, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatalf(".mcp.json lesen: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("zweiter Lauf hat die Datei verändert:\n%s\n%s", first, second)
	}
}

// Ein Projekt ohne MCP-Dateien bleibt eines: die Auto-Korrektur legt nichts an.
func TestRepairMCPLegtNichtsAn(t *testing.T) {
	root := newMCPProject(t)

	repaired, err := RepairMCP(root)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("korrigiert = %v, erwartet nichts", repaired)
	}
	for _, target := range MCPTargets(root) {
		if _, err := os.Stat(filepath.Join(root, target.Path)); !os.IsNotExist(err) {
			t.Errorf("%s wurde angelegt", target.Path)
		}
	}
}

// Ohne installiertes k-playbook gibt es keinen Pfad, der sich eintragen ließe.
// Dann korrigiert auch die Auto-Korrektur nichts.
func TestRepairMCPOhneInstalliertesBinary(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()

	veraltet := `{"mcpServers":{"` + MCPServerKey + `":{"command":"k-playbook/bin/k-playbook","args":["mcp"]}}}` + "\n"
	writeFile(t, filepath.Join(root, ".mcp.json"), veraltet)

	repaired, err := RepairMCP(root)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("korrigiert = %v, erwartet nichts", repaired)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil {
		t.Fatalf(".mcp.json lesen: %v", err)
	}
	if string(raw) != veraltet {
		t.Errorf("ohne installiertes Binary wurde geschrieben:\n%s", raw)
	}
}

// Die portable Form in allen drei Dateien: der Zustand dieses Quell-Repos ab
// Etappe 3. Beide selbsttätigen Wege — Clone-Update und Start — müssen daran
// vorbeigehen, sonst schriebe die eigene Automatik bei jedem Lauf einen
// absoluten Home-Pfad in getrackte Dateien und der Release-Preflight scheiterte
// an ihr.
func TestPortableFormBleibtVonSelbstUnangetastet(t *testing.T) {
	installTestBinary(t)
	root := newMCPProject(t)

	portabel := map[string]string{
		".mcp.json":        `{"mcpServers":{"` + MCPServerKey + `":{"command":"` + InstalledCommandName + `","args":["mcp"]}}}` + "\n",
		".cursor/mcp.json": `{"mcpServers":{"` + MCPServerKey + `":{"command":"` + InstalledCommandName + `","args":["mcp"]}}}` + "\n",
		"opencode.json": `{"$schema":"https://opencode.ai/config.json",` +
			`"instructions":["` + RootInstructionsFile + `"],` +
			`"references":{"docs":{"path":"./docs"}},` +
			`"mcp":{"` + MCPServerKey + `":{"type":"local","command":["` + InstalledCommandName + `","mcp"],"enabled":true}}}` + "\n",
	}
	for name, content := range portabel {
		writeFile(t, filepath.Join(root, filepath.FromSlash(name)), content)
	}

	for _, status := range CheckMCP(root) {
		if !status.OK() {
			t.Errorf("%s gilt nicht als aktuell: %s (%s)", status.Path, status.State, status.Detail)
		}
	}

	repaired, err := RepairMCP(root)
	if err != nil {
		t.Fatalf("RepairMCP: %v", err)
	}
	if len(repaired) != 0 {
		t.Errorf("die Auto-Korrektur hat geschrieben: %v", repaired)
	}

	if _, err := ApplyMCP(root); err != nil {
		t.Fatalf("ApplyMCP: %v", err)
	}

	for name, content := range portabel {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("%s lesen: %v", name, err)
		}
		if string(raw) != content {
			t.Errorf("%s wurde verändert:\n%s", name, raw)
		}
	}
}
