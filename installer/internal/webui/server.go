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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kascada/k-playbook/installer/internal/guiproc"
	"github.com/kascada/k-playbook/installer/internal/project"
)

//go:embed static
var staticFiles embed.FS

const (
	// Der Server hängt nicht am Browserfenster: er bleibt stehen, bis ihn
	// idleTimeout lang niemand mehr gefragt hat — dann ist er vergessen und
	// räumt sich selbst weg. Ein offenes Fenster fragt regelmäßig /api/health
	// und hält ihn damit aus dem Leerlauf.
	idleTimeout       = 60 * time.Minute
	idleCheckInterval = 30 * time.Second
	shutdownTimeout   = 5 * time.Second
	// Verzögerung, damit die Antwort auf /api/shutdown noch rausgeht,
	// bevor der Server zumacht.
	shutdownResponseDelay = 150 * time.Millisecond
)

type serverState struct {
	shutdown func()
	// version ist die VERSION der Installation, die dieses Binary gewählt hat,
	// beim Start festgehalten. Der Client vergleicht sie mit seiner eigenen
	// und ersetzt einen Server anderer Version.
	version string
	// registration ist die Laufzeitdatei dieses Servers. Nil in Tests, die
	// nur die Routen prüfen.
	registration *guiproc.Registration

	mu sync.Mutex
	// lastRequestAt ist der Zeitpunkt der letzten Anfrage, egal welcher. Der
	// Leerlaufwächter misst daran.
	lastRequestAt time.Time
}

// Serve ist der Servermodus: der abgekoppelte Prozess hinter K_PLAYBOOK_SERVE=1.
// Er behält das Arbeitsverzeichnis seines Starts — daraus leiten alle Handler
// das Projekt ab — und blockiert, bis er per /api/shutdown, SIGINT oder
// SIGTERM beendet wird oder idleTimeout lang niemand mehr fragt.
//
// Wirt-Pflege und Browser gehören dem Aufruf, nicht dem Server: hier liefen
// sie nur beim allerersten Start. Ausgaben gehen ins Log, das der Aufruf als
// stdout und stderr vorgibt.
func Serve() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("GUI-Port öffnen: %w", err)
	}

	// Die Laufzeitdatei entsteht nach dem Binden, damit sie eine Adresse
	// trägt, unter der schon jemand antwortet. Sie geht mit dem Server: ein
	// späterer Aufruf fände sonst eine Datei ohne Prozess.
	key, err := guiproc.Key()
	if err != nil {
		listener.Close()
		return err
	}
	version := guiproc.OwnVersion()
	registration, err := guiproc.Register(guiproc.Record{
		Key:       key,
		Addr:      listener.Addr().String(),
		PID:       os.Getpid(),
		Version:   version,
		StartTime: guiproc.OwnStartTime().Unix(),
	})
	if err != nil {
		listener.Close()
		if errors.Is(err, fs.ErrExist) {
			// Zwei Aufrufe zugleich: der andere Start hat die Datei zuerst
			// geschrieben, dieser Prozess tritt zurück. Der Aufruf liest dann
			// dessen Datei und öffnet dessen Server.
			location, _ := guiproc.Locate(key)
			return fmt.Errorf("für dieses Projekt liegt schon eine Laufzeitdatei, ein anderer Start hat gewonnen: %s", location.File)
		}
		return fmt.Errorf("Laufzeitdatei anlegen: %w", err)
	}
	defer registration.Remove()

	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	// Der Leerlauf zählt ab dem Start: auch ein Server, den nie jemand
	// besucht, soll nicht ewig stehen bleiben.
	state := &serverState{shutdown: stop, version: version, registration: registration, lastRequestAt: time.Now()}
	server := &http.Server{Handler: routes(state)}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve(listener)
	}()
	go state.watchIdle(ctx)

	fmt.Printf("k-playbook: http://%s/\n", listener.Addr().String())
	fmt.Printf("Server für %s (PID %d, Version %q). Beenden mit: k-playbook stop\n", key, os.Getpid(), version)

	// SIGTERM wie SIGINT: `k-playbook stop` greift damit auch bei einem
	// Server, der nicht mehr antwortet, und die Laufzeitdatei verschwindet
	// über das defer auch nach einem kill.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	select {
	case sig := <-interrupt:
		fmt.Printf("Signal %s, der Server beendet sich.\n", sig)
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
	fmt.Println("Server beendet.")
	return nil
}

// Announce gibt die URL aus und öffnet den Browser, sofern das hier
// überhaupt sinnvoll ist. Dieselbe Bewegung für einen frisch gestarteten
// wie für einen wiedergefundenen Server; sie läuft im Aufruf, nicht im
// Server — der hat kein Terminal mehr, in dem ein Browser sinnvoll wäre.
func Announce(url string) {
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
}

func routes(state *serverState) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", state.healthHandler)
	mux.HandleFunc("POST /api/shutdown", state.shutdownHandler)
	mux.HandleFunc("GET /api/path", hostPathHandler)
	mux.HandleFunc("GET /api/config", configHandler)
	mux.HandleFunc("POST /api/config", state.createConfigHandler)
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
	mux.HandleFunc("GET /api/gh", ghHandler)
	mux.HandleFunc("POST /api/gh", setGHHandler)
	mux.HandleFunc("GET /api/update", updateCheckHandler)
	mux.HandleFunc("POST /api/update", state.applyUpdateHandler)
	mux.HandleFunc("POST /api/update/discard", discardDevSyncHandler)
	mux.HandleFunc("GET /api/remediation", remediationHandler)
	mux.HandleFunc("POST /api/remediation", setRemediationHandler)
	mux.HandleFunc("GET /api/context", contextHandler)
	mux.HandleFunc("GET /api/docs", docsHandler)
	mux.HandleFunc("GET /api/docs/file", docFileHandler)
	mux.HandleFunc("GET /api/tasks", tasksHandler)
	mux.HandleFunc("GET /api/tasks/done", doneTasksHandler)
	mux.HandleFunc("GET /api/tasks/file", taskFileHandler)
	mux.HandleFunc("GET /api/todos", todosHandler)
	mux.HandleFunc("GET /api/todos/done", doneTodosHandler)

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("eingebettete Assets: %v", err))
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("GET /workflows", workflowsPageHandler)
	mux.HandleFunc("GET /docs", docsPageHandler)
	mux.HandleFunc("GET /mcp", mcpPageHandler)
	mux.HandleFunc("GET /", indexHandler)

	return sameOrigin(state.noteRequests(mux))
}

// sameOrigin weist schreibende Anfragen fremder Herkunft ab. Der Prozess lebt
// jetzt Stunden statt Minuten, und eine beliebige Seite im Browser des
// Nutzers könnte sonst Endpunkte treffen, hinter denen git pull und
// Schreibvorgänge stehen.
//
// Geprüft wird ein echter Same-Origin-Vergleich: der Host-Anteil von Origin
// gegen den Host-Header, ohne Fixierung auf 127.0.0.1:<port> und ohne
// Loopback-Namensliste — hinter einer Portweiterleitung (VS Code, Codespaces)
// kommt der Host-Header vom Browser unverändert und wäre sonst gerade dort
// abgewiesen. Das Schema bleibt außen vor, weil Codespaces TLS terminiert:
// der Browser schickt ein https-Origin, der Server sieht http. Fehlt Origin —
// curl, das Unterkommando stop —, gilt die Anfrage als eigene Herkunft.
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && !originAllowed(r.Header.Get("Origin"), r.Host) {
			http.Error(w, "Anfrage fremder Herkunft abgewiesen.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// originAllowed meldet, ob origin zum Host der Anfrage passt. Ohne Origin ja.
func originAllowed(origin string, host string) bool {
	if origin == "" {
		return true
	}
	return originHost(origin) == host
}

// originHost schneidet das Schema ab und nimmt den Rest bis zum nächsten "/".
// Ein opakes Origin ("null") bleibt stehen und passt zu keinem Host.
func originHost(origin string) string {
	rest := origin
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	if i := strings.Index(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

// noteRequests hält den Zeitpunkt jeder Anfrage fest, egal welcher: solange
// irgendjemand fragt, ist der Server nicht vergessen.
func (state *serverState) noteRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.noteRequest(time.Now())
		next.ServeHTTP(w, r)
	})
}

// Die Bereiche des Umschalters. Der Wert steht in den Vorlagendaten und
// entscheidet, welcher Eintrag markiert ist.
const (
	areaSetup     = "setup"
	areaWorkflows = "workflows"
	areaDocs      = "docs"
)

// pageTemplate parst eine Seite zusammen mit dem Fragment der linken Spalte.
// Die Seitendatei steht zuerst: ParseFS benennt das Ergebnis nach der ersten
// Datei, und Execute führt damit die Seite aus und nicht das Fragment.
func pageTemplate(name string) *template.Template {
	return template.Must(template.ParseFS(staticFiles, "static/"+name, "static/sidebar.html"))
}

var indexTemplate = pageTemplate("index.html")

// workflowsTemplate ist die Seite der täglichen Arbeit: Reviews, Tasks und
// Todos untereinander.
var workflowsTemplate = pageTemplate("workflows.html")

func workflowsPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, workflowsTemplate, areaWorkflows, "/workflows")
}

// docsTemplate ist die Seite zum Nachschlagen: der Index links im Menü, die
// gelesene Datei rechts.
var docsTemplate = pageTemplate("docs.html")

func docsPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, docsTemplate, areaDocs, "/docs")
}

// mcpTemplate ist die Seite des MCP-Servers. Sie ist eine Detailseite des
// Setup-Blocks und trägt deshalb dessen Bereich im Umschalter.
var mcpTemplate = pageTemplate("mcp.html")

func mcpPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, mcpTemplate, areaSetup, "/mcp")
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	renderPage(w, indexTemplate, areaSetup, "/")
}

// renderPage füllt den gemeinsamen Kopf und gibt die Vorlage aus. area sagt,
// welcher Eintrag des Umschalters markiert wird, page nennt die offene Seite.
// Beides fällt auseinander, sobald ein Bereich mehr als eine Seite hat: /mcp
// trägt den Bereich Setup, ist aber nicht dessen Startseite — und nur die
// offene Seite darf aria-current="page" führen.
func renderPage(w http.ResponseWriter, tmpl *template.Template, area string, page string) {
	environment := project.Detect()
	data := struct {
		Mode        string
		ModeLabel   string
		Path        string
		RepoRoot    string
		PlaybookDir string
		Installed   bool
		Area        string
		Page        string
	}{Installed: environment.Installed, Area: area, Page: page}

	if environment.Installed {
		data.Mode = "project"
		data.ModeLabel = "Projekt"
		data.Path = project.DisplayPath(environment.ProjectDir)
		if config, err := project.ReadConfig(environment.ProjectDir); err == nil {
			data.RepoRoot = project.DisplayPath(project.RepoRootDir(environment.ProjectDir, config))
		}
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

// healthHandler ist das Lebenszeichen des Fensters: solange eines fragt,
// bleibt der Server aus dem Leerlauf. Schlägt der Aufruf fehl, weiß das
// Fenster, dass der Server weg ist.
//
// Die Antwort nennt Schlüssel, Version und PID, damit ein CLI-Aufruf den
// Server als seinen eigenen erkennt. Der Schlüssel wird je Anfrage neu
// berechnet: nach einem Umschlüsseln durch POST /api/config muss schon die
// nächste Antwort den neuen tragen. Die Version dagegen ist die vom Start.
func (state *serverState) healthHandler(w http.ResponseWriter, r *http.Request) {
	key, _ := guiproc.Key()
	writeJSON(w, http.StatusOK, guiproc.Health{
		Status:  "ok",
		Key:     key,
		Version: state.version,
		PID:     os.Getpid(),
	})
}

// rekey zieht die Laufzeitdatei auf den aktuellen Schlüssel nach. Nötig nach
// POST /api/config: die Konfiguration entsteht in einem vom Nutzer gewählten
// Verzeichnis, und das aufgelöste ProjectDir kann sich dadurch ändern. Bei
// gleichem Schlüssel passiert nichts.
func (state *serverState) rekey() {
	if state.registration == nil {
		return
	}
	key, err := guiproc.Key()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: Schlüssel der Laufzeitdatei nicht bestimmbar: %v\n", err)
		return
	}
	if err := state.registration.Rekey(key); err != nil {
		fmt.Fprintf(os.Stderr, "Hinweis: Laufzeitdatei nicht umgeschlüsselt: %v\n", err)
	}
}

// shutdownHandler beendet den Dienst für alle Fenster.
func (state *serverState) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "shutting_down"})
	state.shutdownAfterResponse()
}

// shutdownAfterResponse lässt die laufende Antwort noch hinaus und macht dann
// zu. Ohne die Verzögerung bekäme der Aufrufer einen Verbindungsabbruch statt
// der Bestätigung.
func (state *serverState) shutdownAfterResponse() {
	if state.shutdown == nil {
		return
	}
	go func() {
		time.Sleep(shutdownResponseDelay)
		state.shutdown()
	}()
}

func (state *serverState) noteRequest(now time.Time) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.lastRequestAt = now
}

// watchIdle beendet den Server, wenn idleTimeout lang keine Anfrage kam.
func (state *serverState) watchIdle(ctx context.Context) {
	ticker := time.NewTicker(idleCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if state.idleExceeded(now) {
				fmt.Printf("Seit %s keine Anfrage, der Server beendet sich.\n", idleTimeout)
				state.shutdown()
				return
			}
		}
	}
}

// idleExceeded meldet, ob seit der letzten Anfrage idleTimeout vergangen ist.
// Eigene Funktion mit übergebener Zeit, damit der Wächter ohne Warten prüfbar
// ist.
func (state *serverState) idleExceeded(now time.Time) bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	return !state.lastRequestAt.IsZero() && now.Sub(state.lastRequestAt) >= idleTimeout
}
