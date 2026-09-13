package project

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// mcpServerNamePattern begrenzt, was als Pflichtname in die Konfiguration
// darf: der Name ist zugleich Schlüssel in den MCP-Dateien und Pfadsegment
// der Detailseite, also nichts, was dort eine eigene Bedeutung hätte.
var mcpServerNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ValidMCPServerName meldet, ob ein Servername zulässig ist.
func ValidMCPServerName(name string) bool {
	return mcpServerNamePattern.MatchString(name)
}

// MCPRequirements ist der MCP-Teil der Kontextausgabe: die Pflichtliste aus
// tools.mcp.required und ob der Block dastand. Objektform statt bloßer Liste,
// damit weitere MCP-Angaben Platz finden, ohne das Feld umzubenennen.
type MCPRequirements struct {
	Required   []string `json:"required"`
	Configured bool     `json:"configured"`
}

// ReadRequiredMCPServers liest tools.mcp.required. Das zweite Ergebnis meldet,
// ob der Schlüssel dastand — fehlt er, ist die Liste leer und niemand hat
// etwas verlangt.
func ReadRequiredMCPServers(projectDir string) ([]string, bool, error) {
	data, err := os.ReadFile(ConfigPath(projectDir))
	if err != nil {
		return []string{}, false, err
	}
	return parseRequiredMCPServers(string(data))
}

// parseRequiredMCPServers liest tools.mcp.required über parseYAMLList — Fluss-
// und Blockform, genau dieser Pfad — und bereinigt und prüft die Namen.
func parseRequiredMCPServers(content string) ([]string, bool, error) {
	items, found := parseYAMLList(content, "tools", "mcp", "required")
	if !found {
		return []string{}, false, nil
	}
	names := []string{}
	for _, item := range items {
		if cleaned := cleanMCPServerName(item); cleaned != "" {
			names = append(names, cleaned)
		}
	}
	for _, name := range names {
		if !ValidMCPServerName(name) {
			return []string{}, true, fmt.Errorf("tools.mcp.required enthält den unzulässigen Namen %q; erlaubt sind Buchstaben, Ziffern und . _ -", name)
		}
	}
	return names, true, nil
}

func cleanMCPServerName(value string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"'`))
}

// MCPServerInventoryFor liest alle Server und die Pflichtliste eines Projekts
// und führt beides zusammen. Ein unzulässiger Pflichtname ist ein Fehler, wie
// bei den Sprachen: er soll nicht still verschwinden.
func MCPServerInventoryFor(projectRoot string) (MCPServerInventory, error) {
	servers, files := ListMCPServers(projectRoot)
	required, configured, err := ReadRequiredMCPServers(projectRoot)
	if err != nil {
		return newMCPServerInventory(servers, files, nil, configured), err
	}
	return newMCPServerInventory(servers, files, required, configured), nil
}
