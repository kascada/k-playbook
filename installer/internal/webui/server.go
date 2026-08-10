// Package webui stellt die lokale Browser-Oberflaeche von k-playbook bereit.
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
	// Der Client meldet sich regelmaessig. Bleibt er laenger als
	// clientHeartbeatTimeout aus, ist das Browserfenster weg und der Server
	// wird nicht mehr gebraucht.
	clientHeartbeatTimeout = 5 * time.Second
	// Nach einer ausdruecklichen Abmeldung wird kurz gewartet, damit ein
	// Reload den Server nicht abraeumt.
	clientGoneShutdownDelay = 3 * time.Second
	clientMonitorInterval   = 2 * time.Second
	shutdownTimeout         = 5 * time.Second
	// Verzoegerung, damit die Antwort auf /api/shutdown noch rausgeht,
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
// abmeldet, verschwindet oder Ctrl+C gedrueckt wird.
func Run() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("GUI-Port oeffnen: %w", err)
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

// announce gibt die URL aus und oeffnet den Browser, sofern das hier
// ueberhaupt sinnvoll ist.
func announce(url string) {
	fmt.Printf("k-playbook: %s\n", url)

	if marker, inside := containerMarker(); inside {
		fmt.Printf("Container erkannt (%s), der Browser wird nicht geoeffnet.\n", marker)
		fmt.Println("Obige URL im Browser auf dem Host eintragen; im DevContainer muss der Port weitergeleitet sein.")
	} else if err := openBrowser(url); err != nil {
		fmt.Printf("Browser konnte nicht automatisch geoeffnet werden: %v\n", err)
		fmt.Println("Obige URL bitte manuell im Browser eintragen.")
	}

	fmt.Println("Zum Beenden Ctrl+C druecken.")
}

func routes(state *serverState) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", state.healthHandler)
	mux.HandleFunc("POST /api/client-gone", state.clientGoneHandler)
	mux.HandleFunc("POST /api/shutdown", state.shutdownHandler)
	mux.HandleFunc("GET /api/config", configHandler)
	mux.HandleFunc("POST /api/config", createConfigHandler)
	mux.HandleFunc("GET /api/assistant", assistantHandler)
	mux.HandleFunc("POST /api/assistant", applyAssistantHandler)

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("eingebettete Assets: %v", err))
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("GET /", indexHandler)

	return mux
}

var indexTemplate = template.Must(template.ParseFS(staticFiles, "static/index.html"))

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	environment := project.Detect()
	data := struct {
		Mode      string
		ModeLabel string
		Path      string
	}{}

	if environment.Installed {
		data.Mode = "project"
		data.ModeLabel = "Projekt"
		data.Path = project.DisplayPath(environment.ProjectDir)
	} else {
		data.Mode = "none"
		data.ModeLabel = "Nicht installiert"
		data.Path = project.DisplayPath(environment.SearchedFrom)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexTemplate.Execute(w, data); err != nil {
		// Die Antwort laeuft bereits; mehr als protokollieren geht nicht.
		fmt.Fprintf(os.Stderr, "Startseite rendern: %v\n", err)
	}
}

// healthHandler dient dem Client als Lebenszeichen in beide Richtungen:
// schlaegt der Aufruf fehl, weiss der Client, dass der Server weg ist;
// bleibt er aus, weiss der Server, dass der Client weg ist.
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
