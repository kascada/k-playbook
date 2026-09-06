package mcpserver

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// text zieht den Text aller Inhaltsblöcke zusammen.
func text(result *mcp.CallToolResult) string {
	var parts []string
	for _, content := range result.Content {
		if block, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// handler ist die Attrappe hinter der Middleware: eine Antwort mit einem Block.
func handler(result mcp.Result, err error) mcp.MethodHandler {
	return func(context.Context, string, mcp.Request) (mcp.Result, error) {
		return result, err
	}
}

func answer() *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "die Antwort"}}}
}

// Der Hinweis hängt nur an, wenn unter dem Pfad nachweislich eine andere Datei
// liegt — und er ersetzt die Antwort nicht, er kommt dazu.
func TestStaleBinaryNoticeHaengtNurBeiWechselAn(t *testing.T) {
	tests := []struct {
		name    string
		started string
		current string
		want    bool
	}{
		{name: "unverändert", started: "a", current: "a"},
		{name: "gewechselt", started: "a", current: "b", want: true},
		{name: "ohne Kennung beim Start", started: "", current: "b"},
		{name: "jetzt nicht ermittelbar", started: "a", current: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			middleware := staleBinaryNotice(test.started, func() string { return test.current })
			result, err := middleware(handler(answer(), nil))(context.Background(), "tools/call", nil)
			if err != nil {
				t.Fatalf("Fehler: %v", err)
			}
			got := text(result.(*mcp.CallToolResult))
			if !strings.HasPrefix(got, "die Antwort") {
				t.Errorf("die Antwort selbst fehlt oder steht nicht vorn: %q", got)
			}
			if strings.Contains(got, staleNotice) != test.want {
				t.Errorf("Hinweis vorhanden = %v, erwartet %v", !test.want, test.want)
			}
		})
	}
}

// Andere Methoden und Protokollfehler bleiben unangetastet: ein Hinweis gehört
// in eine Werkzeugantwort, nicht in ein tools/list oder einen Fehlerpfad.
func TestStaleBinaryNoticeLaesstAnderesInRuhe(t *testing.T) {
	middleware := staleBinaryNotice("a", func() string { return "b" })

	result, err := middleware(handler(answer(), nil))(context.Background(), "tools/list", nil)
	if err != nil {
		t.Fatalf("Fehler: %v", err)
	}
	if got := text(result.(*mcp.CallToolResult)); strings.Contains(got, staleNotice) {
		t.Error("tools/list trägt den Hinweis")
	}

	broken := errors.New("kaputt")
	if _, err := middleware(handler(nil, broken))(context.Background(), "tools/call", nil); !errors.Is(err, broken) {
		t.Errorf("Fehler = %v, erwartet %v", err, broken)
	}
}
