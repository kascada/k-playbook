package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// staleNotice hängt an jeder Antwort, sobald unter dem Pfad dieses Prozesses
// ein anderes Binary liegt.
//
// Der Text richtet sich an den Assistenten, nicht an den Nutzer: er ist der
// einzige Leser einer Werkzeugantwort und die einzige Stelle, die das
// weitersagen kann. Deshalb steht die Handlung ausdrücklich darin — ein
// bloßer Zustandsbericht bliebe folgenlos.
const staleNotice = `Hinweis von k-playbook: Dieser MCP-Server läuft aus einem Binary, das ` +
	`inzwischen ersetzt wurde. Die Antwort oben stammt damit aus dem alten Programmstand. ` +
	`Die Inhalte darin — Regeln, Reviews, Checks, Instruktionen — sind aktuell, sie werden ` +
	`bei jedem Aufruf von der Platte gelesen; veraltet ist der Code der Werkzeuge, und ` +
	`Werkzeuge, die seither dazugekommen sind, fehlen dieser Sitzung ganz. Bitte dem Nutzer ` +
	`melden: den Assistenten neu starten, damit der MCP-Server aus dem neuen Binary kommt.`

// staleBinaryNotice meldet einen Binärwechsel in der Antwort, statt den Prozess
// zu beenden.
//
// Warum nicht beenden: bei stdio startet der **Client** den Serverprozess. Ob
// er nach einem Ende neu startet, steht in keiner Spec und ist je Client
// anders — endet der Server bei einem Client, der das nicht tut, hat der
// Assistent für den Rest der Sitzung gar keine k-playbook-Werkzeuge mehr. Das
// wäre schlechter als ein alter Server, der arbeitet. Ein Hinweis kostet
// nichts und wirkt überall.
//
// Warum nicht `syscall.Exec` auf das neue Binary, das die Pipes behielte: das
// neue Prozessabbild erwartete ein `initialize`, das der Client längst
// geschickt hat und nicht wiederholt.
//
// started ist die Kennung beim Start, current liest die von jetzt — dieselbe
// Funktion, zu zwei Zeitpunkten. Fehlt eine von beiden, wird nichts gemeldet:
// eine Kennung, die sich nicht erheben lässt, ist kein Nachweis eines
// Wechsels, und ein Hinweis an jeder Antwort wäre dann dauerhaft falsch.
func staleBinaryNotice(started string, current func() string) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			result, err := next(ctx, method, req)
			if method != "tools/call" || err != nil {
				return result, err
			}
			call, ok := result.(*mcp.CallToolResult)
			if !ok || !binaryChanged(started, current) {
				return result, err
			}
			// Ein eigener Inhaltsblock, kein Eingriff in den bestehenden: die
			// erste Antwort bleibt Zeichen für Zeichen die des Subkommandos,
			// und genau darauf beruht der Vergleich beider Fassaden.
			call.Content = append(call.Content, &mcp.TextContent{Text: staleNotice})
			return call, nil
		}
	}
}

// binaryChanged meldet, ob unter dem Pfad dieses Prozesses inzwischen eine
// andere Datei liegt.
func binaryChanged(started string, current func() string) bool {
	if started == "" {
		return false
	}
	now := current()
	return now != "" && now != started
}
