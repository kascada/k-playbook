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

// parseRequiredMCPServers liest die Liste zeilenweise, wie parseGHStatus den
// gh-Block und parseLanguages die Sprachliste: Block tools → mcp → required,
// in Fluss- und Blockform. Kein YAML-Parser, damit die Datei beim späteren
// Zurückschreiben eines anderen Blocks unangetastet bleibt.
func parseRequiredMCPServers(content string) ([]string, bool, error) {
	inTools := false
	mcpIndent := -1
	listIndent := -1
	found := false
	names := []string{}

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := lineIndent(line)

		if indent == 0 {
			if found {
				break
			}
			key, _, _ := strings.Cut(trimmed, ":")
			inTools = strings.TrimSpace(key) == "tools"
			mcpIndent = -1
			continue
		}
		if !inTools {
			continue
		}

		// Innerhalb der Blockliste zählen nur die Spiegelstriche; alles
		// andere auf gleicher oder geringerer Tiefe beendet sie.
		if listIndent >= 0 {
			if strings.HasPrefix(trimmed, "- ") || trimmed == "-" {
				if value := cleanMCPServerName(strings.TrimPrefix(trimmed, "-")); value != "" {
					names = append(names, value)
				}
				continue
			}
			if indent <= listIndent {
				break
			}
			continue
		}

		// Zurück auf die Ebene der Tool-Namen: der mcp-Block ist zu Ende.
		if mcpIndent >= 0 && indent <= mcpIndent {
			mcpIndent = -1
		}
		key, value, hasColon := strings.Cut(trimmed, ":")
		if !hasColon {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if mcpIndent < 0 {
			if key == "mcp" && value == "" {
				mcpIndent = indent
			}
			continue
		}
		if key != "required" {
			continue
		}
		found = true
		if value == "" {
			listIndent = indent
			continue
		}
		// Flussform: required: [k-playbook, atlassian]
		for _, item := range strings.Split(strings.Trim(value, "[]"), ",") {
			if cleaned := cleanMCPServerName(item); cleaned != "" {
				names = append(names, cleaned)
			}
		}
		break
	}

	if !found {
		return []string{}, false, nil
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
