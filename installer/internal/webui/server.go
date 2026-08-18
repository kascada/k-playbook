// Package webui stellt die lokale Browser-Oberfläche von k-playbook bereit.
package webui

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/kascada/k-playbook/installer/internal/project"
)

//go:embed static
var staticFiles embed.FS

const (
	// Der Client meldet sich regelmäßig. Bleibt er länger als
	// clientHeartbeatTimeout aus, ist das Browserfenster weg und der Server
	// wird nicht mehr gebraucht.
	clientHeartbeatTimeout = 5 * time.Second
	// Nach einer ausdrücklichen Abmeldung wird kurz gewartet, damit ein
	// Reload den Server nicht abräumt.
	clientGoneShutdownDelay = 3 * time.Second
	clientMonitorInterval   = 2 * time.Second
	shutdownTimeout         = 5 * time.Second
	// Verzögerung, damit die Antwort auf /api/shutdown noch rausgeht,
	// bevor der Server zumacht.
	shutdownResponseDelay = 150 * time.Millisecond
)

type serverState struct {
	shutdown func()

	mu             sync.Mutex
	lastClientSeen time.Time
	clientGoneAt   time.Time
}

// Run startet den lokalen Server und blockiert, bis der Client sich
// abmeldet, verschwindet oder Ctrl+C gedrückt wird.
func Run() error {
	protectProjectInstallation()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("GUI-Port öffnen: %w", err)
	}

	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	state := &serverState{shutdown: stop}
	server := &http.Server{Handler: routes(state)}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve(listener)
	}()
	go state.monitorClient(ctx)

	announce("http://" + listener.Addr().String() + "/")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)

	select {
	case <-interrupt:
		fmt.Println()
	case <-ctx.Done():
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("GUI-Server: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("GUI-Server stoppen: %w", err)
	}
	return nil
}

func protectProjectInstallation() {
	environment := project.Detect()
	if !environment.Installed || !environment.PlaybookPresent {
		return
	}
	if err := project.SetInstallationReadOnly(environment.ProjectDir); err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: Installation konnte nicht read-only gesetzt werden: %v\n", err)
	}
}

// announce gibt die URL aus und öffnet den Browser, sofern das hier
// überhaupt sinnvoll ist.
func announce(url string) {
	fmt.Printf("k-playbook: %s\n", url)

	marker, inContainer := containerMarker()
	openers := browserOpeners()
	if inContainer {
		// Im Container zählt allein ein ausdrücklich gesetzter $BROWSER: dort
		// steht ein Helfer, der die URL an den Host durchreicht — so richtet es
		// der DevContainer von VS Code ein. Die geratenen Kandidaten leisten das
		// nicht und können schaden: in schlanken Images zeigen x-www-browser und
		// sensible-browser gern auf einen Terminal-Browser, der dann das
		// Terminal übernimmt.
		openers = envOpeners()
	}

	if len(openers) == 0 {
		fmt.Printf("Container erkannt (%s), der Browser wird nicht geöffnet.\n", marker)
		fmt.Println("Obige URL im Browser auf dem Host eintragen; im DevContainer muss der Port weitergeleitet sein.")
	} else if err := openBrowser(url, openers); err != nil {
		fmt.Printf("Browser konnte nicht automatisch geöffnet werden: %v\n", err)
		fmt.Println("Obige URL bitte manuell im Browser eintragen.")
	}

	fmt.Println("Zum Beenden Ctrl+C drücken.")
}

func routes(state *serverState) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", state.healthHandler)
	mux.HandleFunc("POST /api/client-gone", state.clientGoneHandler)
	mux.HandleFunc("POST /api/shutdown", state.shutdownHandler)
	mux.HandleFunc("GET /api/path", hostPathHandler)
	mux.HandleFunc("GET /api/config", configHandler)
	mux.HandleFunc("POST /api/config", createConfigHandler)
	mux.HandleFunc("POST /api/config/reset", resetConfigHandler)
	mux.HandleFunc("GET /api/local", localHandler)
	mux.HandleFunc("POST /api/local", createLocalHandler)
	mux.HandleFunc("GET /api/local/private", localPrivateHandler)
	mux.HandleFunc("POST /api/local/private", setLocalPrivateHandler)
	mux.HandleFunc("GET /api/assistant", assistantHandler)
	mux.HandleFunc("POST /api/assistant", applyAssistantHandler)
	mux.HandleFunc("GET /api/mcp", mcpHandler)
	mux.HandleFunc("POST /api/mcp", applyMCPHandler)
	// Eigener Endpunkt, weil dahinter ein Subprozess steht: nur die Seite /mcp
	// fragt ihn, die Startseite bliebe sonst daran hängen.
	mux.HandleFunc("GET /api/mcp/tools", mcpToolsHandler)
	mux.HandleFunc("GET /api/tools", toolsHandler)
	mux.HandleFunc("POST /api/languages", setLanguagesHandler)
	mux.HandleFunc("GET /api/reviews", reviewsHandler)
	mux.HandleFunc("POST /api/reviews", createRunHandler)
	mux.HandleFunc("GET /api/gh", ghHandler)
	mux.HandleFunc("POST /api/gh", setGHHandler)
	mux.HandleFunc("GET /api/update", updateCheckHandler)
	mux.HandleFunc("POST /api/update", applyUpdateHandler)
	mux.HandleFunc("POST /api/update/discard", discardDevSyncHandler)
	mux.HandleFunc("GET /api/remediation", remediationHandler)
	mux.HandleFunc("POST /api/remediation", setRemediationHandler)
	mux.HandleFunc("GET /api/context", contextHandler)
	mux.HandleFunc("GET /api/docs", docsHandler)
	mux.HandleFunc("GET /api/docs/file", docFileHandler)
	mux.HandleFunc("GET /api/workflows", workflowsHandler)
	mux.HandleFunc("GET /api/tasks", tasksHandler)
	mux.HandleFunc("GET /api/tasks/file", taskFileHandler)

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("eingebettete Assets: %v", err))
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("GET /reviews", reviewsPageHandler)
	mux.HandleFunc("GET /tasks", tasksPageHandler)
	mux.HandleFunc("GET /mcp", mcpPageHandler)
	mux.HandleFunc("GET /", indexHandler)

	return mux
}

var indexTemplate = template.Must(template.ParseFS(staticFiles, "static/index.html"))

// reviewsTemplate ist die zweite Seite. Sie teilt den Kopf mit der Startseite,
// deshalb dieselben Vorlagendaten.
var reviewsTemplate = template.Must(template.ParseFS(staticFiles, "static/reviews.html"))

func reviewsPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, reviewsTemplate)
}

// tasksTemplate ist die Seite der Tasks, ebenfalls mit dem gemeinsamen Kopf.
var tasksTemplate = template.Must(template.ParseFS(staticFiles, "static/tasks.html"))

func tasksPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, tasksTemplate)
}

// mcpTemplate ist die Seite des MCP-Servers, ebenfalls mit dem gemeinsamen Kopf.
var mcpTemplate = template.Must(template.ParseFS(staticFiles, "static/mcp.html"))

func mcpPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, mcpTemplate)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	renderPage(w, indexTemplate)
}

// renderPage füllt den gemeinsamen Kopf und gibt die Vorlage aus.
func renderPage(w http.ResponseWriter, tmpl *template.Template) {
	environment := project.Detect()
	data := struct {
		Mode        string
		ModeLabel   string
		Path        string
		PlaybookDir string
		Installed   bool
	}{Installed: environment.Installed}

	if environment.Installed {
		data.Mode = "project"
		data.ModeLabel = "Projekt"
		data.Path = project.DisplayPath(environment.ProjectDir)
		// Aus diesem Verzeichnis kommen Skripte, Regeln, Reviews und Checks. Es
		// ist ein eigener Clone und kann einen anderen Stand tragen als das
		// Binary — deshalb gehört es in den Kopf und nicht hinter einen Klick.
		data.PlaybookDir = project.DisplayPath(environment.PlaybookDir)
	} else {
		data.Mode = "none"
		// Als Beschriftung eines Pfades gelesen, nicht als Zustandsmarke: der
		// Pfad daneben ist der Ort, ab dem gesucht wurde.
		data.ModeLabel = "Nicht installiert, gesucht ab"
		data.Path = project.DisplayPath(environment.SearchedFrom)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		// Die Antwort läuft bereits; mehr als protokollieren geht nicht.
		fmt.Fprintf(os.Stderr, "Seite rendern: %v\n", err)
	}
}

// healthHandler dient dem Client als Lebenszeichen in beide Richtungen:
// schlägt der Aufruf fehl, weiß der Client, dass der Server weg ist;
// bleibt er aus, weiß der Server, dass der Client weg ist.
func (state *serverState) healthHandler(w http.ResponseWriter, r *http.Request) {
	state.noteClientSeen()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (state *serverState) clientGoneHandler(w http.ResponseWriter, r *http.Request) {
	state.noteClientGone()
	w.WriteHeader(http.StatusNoContent)
}

func (state *serverState) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "shutting_down"})
	go func() {
		time.Sleep(shutdownResponseDelay)
		state.shutdown()
	}()
}

func (state *serverState) noteClientSeen() {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.lastClientSeen = time.Now()
	state.clientGoneAt = time.Time{}
}

func (state *serverState) noteClientGone() {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.clientGoneAt = time.Now()
}

func (state *serverState) monitorClient(ctx context.Context) {
	ticker := time.NewTicker(clientMonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if state.shouldShutdownForMissingClient(now) {
				state.shutdown()
				return
			}
		}
	}
}

// shouldShutdownForMissingClient meldet, ob der Client weg ist. Solange sich
// noch nie einer gemeldet hat, bleibt der Server stehen: der Browser kann
// noch unterwegs sein oder die URL wird von Hand eingetragen.
func (state *serverState) shouldShutdownForMissingClient(now time.Time) bool {
	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.clientGoneAt.IsZero() && now.Sub(state.clientGoneAt) >= clientGoneShutdownDelay {
		return true
	}
	return !state.lastClientSeen.IsZero() && now.Sub(state.lastClientSeen) >= clientHeartbeatTimeout
}
