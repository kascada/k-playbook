package webui

import (
	"context"
	"net/http"
	"os"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/vscodeext"
)

// vscodeHandler antwortet mit dem Zustand der VS-Code-Erweiterung: die
// eingebettete Fassung, die Installationen aus extensions.json, die CLI, die
// ein Versuch jetzt nähme, und der letzte selbsttätige Nachzug dieses
// Dienstes. Rein lesend — installiert wird beim Start des Dienstes, nicht auf
// einen Seitenaufruf hin.
func vscodeHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, vscodeext.CurrentStatus())
}

// careForVSCodeExtension zieht die Erweiterung im Hintergrund nach.
//
// Im Hintergrund, weil der Start nicht warten darf: eine Installation dauert
// rund eine Sekunde, in einer trägen Umgebung länger. Beim Start des
// Dienstes, nicht im Aufruf: der Aufruf läuft jedes Mal, der Dienst nur beim
// ersten Start und nach einem Programmwechsel — und genau dann ist etwas
// nachzuziehen. Das ist der einzige Auslöser (Todo #24 klärt weitere).
//
// Die Sperre liegt im Laufzeitverzeichnis des Dienstes: mehrere Projekte
// können gleichzeitig starten und würden sonst gleichzeitig in dasselbe
// Erweiterungsverzeichnis schreiben.
func careForVSCodeExtension(ctx context.Context) {
	// Nur aus einem gestempelten Programm. Ohne Version ist dies ein
	// Ad-hoc-Build — ein `go build` ohne Build-Flags oder das Test-Binary
	// dieses Pakets, das sich für `TestSpawnServerUndStop` selbst im
	// Servermodus startet. Ein solcher Prozess greift nicht in das VS Code
	// des Rechners ein: ein Testlauf, der dort wirklich installiert, ist
	// genau der Fehler, den „Tests nie gegen das echte code" verbietet. Der
	// reguläre Weg ist davon nicht betroffen — auch `make dev-install`
	// stempelt die Version.
	if guiproc.OwnVersion() == "" {
		return
	}
	dir, err := guiproc.RuntimeDir()
	if err != nil {
		// Ohne Laufzeitverzeichnis gibt es auch keine Laufzeitdatei; dann ist
		// dieser Prozess ohnehin kein regulärer Dienst.
		return
	}
	// Nach stdout: der Aufruf gibt dem abgekoppelten Dienst die Logdatei als
	// stdout und stderr vor, eine Zeile landet also dort.
	go vscodeext.EnsureInstalled(ctx, dir, os.Stdout)
}
