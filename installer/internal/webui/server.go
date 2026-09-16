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
	// build ist die Kennung der Binärdatei dieses Prozesses, ebenfalls beim
	// Start festgehalten — und nur so brauchbar: nach `make dev-install` liegt
	// unter demselben Pfad längst ein anderes Binary, und ein Server, der erst
	// auf Nachfrage nachsähe, meldete dessen Kennung statt seiner eigenen. Sie
	// erkennt den Wechsel, den die stillstehende VERSION nicht zeigt.
	build string
	// registration ist die Laufzeitdatei dieses Servers. Nil in Tests, die
	// nur die Routen prüfen.
	registration *guiproc.Registration

	mu sync.Mutex
	// lastRequestAt ist der Zeitpunkt der letzten Anfrage, egal welcher. Der
	// Leerlaufwächter misst daran.
	lastRequestAt time.Time

	// streams wird beim Beenden geschlossen und beendet damit die offenen
	// Ereignisströme des Chats. Ein Strom endet nie von selbst; ohne den Kanal
	// wartete Shutdown auf ihn bis zur Frist. Nil in Tests, die nur die Routen
	// prüfen.
	streams     chan struct{}
	streamsOnce sync.Once

	commandMu sync.Mutex
	// commandRuns hält je Sitzung den letzten Ausgang des abgekoppelten
	// Command-Aufrufs. Ohne ihn entstünde für einen Fehler nach der 202 kein
	// Ereignis und die Seite bliebe dauerhaft auf „Arbeitet" stehen.
	commandRuns map[string]*chatCommandOutcome
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
	build := guiproc.OwnBuild()
	registration, err := guiproc.Register(guiproc.Record{
		Key:       key,
		Addr:      listener.Addr().String(),
		PID:       os.Getpid(),
		Version:   version,
		Build:     build,
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
	state := &serverState{shutdown: stop, version: version, build: build, registration: registration, lastRequestAt: time.Now(), streams: make(chan struct{})}
	server := &http.Server{Handler: routes(state)}
	server.RegisterOnShutdown(state.closeStreams)

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
	// Die Übersicht aller MCP-Server liest nur Dateien. Die Messung eines
	// Servers steht hinter einem POST: sie startet ein Kommando aus den
	// Projektdateien und darf weder ein GET noch ein Seitenaufruf auslösen.
	mux.HandleFunc("GET /api/mcp-servers", mcpServersHandler)
	mux.HandleFunc("GET /api/mcp-servers/{assistant}/{name}", mcpServerDetailHandler)
	mux.HandleFunc("POST /api/mcp-servers/{assistant}/{name}/probe", mcpServerProbeHandler)
	mux.HandleFunc("GET /api/tools", toolsHandler)
	mux.HandleFunc("POST /api/languages", setLanguagesHandler)
	// Die Basis-Werkzeuge haben einen eigenen Endpunkt neben den Security-Tools:
	// dahinter steht kein Skriptaufruf, sondern der PATH-Befund aus dem Kontext.
	mux.HandleFunc("GET /api/base-tools", baseToolsHandler)
	mux.HandleFunc("GET /api/reviews", reviewsHandler)
	mux.HandleFunc("GET /api/gh", ghHandler)
	mux.HandleFunc("POST /api/gh", setGHHandler)
	// Je Karte der Seite /github ein eigener Endpunkt: dahinter steht ein
	// gh-Subprozess mit Netzzugriff, und eine langsame Abfrage soll die
	// übrigen Karten nicht aufhalten. Nur lesend, ohne Cache.
	mux.HandleFunc("GET /api/github/overview", githubOverviewHandler)
	mux.HandleFunc("GET /api/github/pulls", githubPullsHandler)
	mux.HandleFunc("GET /api/github/runs", githubRunsHandler)
	// Das Log eines Laufs ist die teuerste Abfrage und steht deshalb hinter
	// einem eigenen Endpunkt: geholt wird es erst beim Aufklappen eines roten
	// Laufs, nie für alle Läufe beim Laden der Seite.
	mux.HandleFunc("GET /api/github/runs/{id}/failure", githubRunFailureHandler)
	mux.HandleFunc("GET /api/update", updateCheckHandler)
	mux.HandleFunc("POST /api/update", state.applyUpdateHandler)
	mux.HandleFunc("GET /api/remediation", remediationHandler)
	mux.HandleFunc("POST /api/remediation", setRemediationHandler)
	mux.HandleFunc("GET /api/context", contextHandler)
	mux.HandleFunc("GET /api/docs", docsHandler)
	mux.HandleFunc("GET /api/docs/file", docFileHandler)
	// Das Versionsinventar hat eine eigene API neben der Doku: die zeigt die
	// mitgelieferte Doku der Installation, das Inventar ist eine Datei des
	// Projekts. Hinter dem POST steht ausschließlich inventory.Run.
	mux.HandleFunc("GET /api/inventory", inventoryHandler)
	mux.HandleFunc("POST /api/inventory", runInventoryHandler)
	mux.HandleFunc("GET /api/inventory/file", inventoryFileHandler)
	mux.HandleFunc("GET /api/tasks", tasksHandler)
	mux.HandleFunc("GET /api/tasks/done", doneTasksHandler)
	mux.HandleFunc("GET /api/tasks/file", taskFileHandler)
	mux.HandleFunc("GET /api/todos", todosHandler)
	mux.HandleFunc("GET /api/todos/done", doneTodosHandler)
	// Der Chat leitet an den OpenCode-Dienst weiter; welche Endpunkte dort
	// dahinter stehen, weiß allein chat.go. Die POSTs schützt sameOrigin wie
	// alle übrigen.
	mux.HandleFunc("GET /api/chat/status", chatStatusHandler)
	mux.HandleFunc("GET /api/chat/sessions", chatSessionsHandler)
	mux.HandleFunc("POST /api/chat/sessions", chatCreateSessionHandler)
	mux.HandleFunc("GET /api/chat/sessions/{id}", chatSessionHandler)
	mux.HandleFunc("GET /api/chat/sessions/{id}/messages", chatMessagesHandler)
	// Die Kind-Sitzungen braucht die Seite, um eine Rückfrage aus einem
	// Subtask als zu ihr gehörig zu erkennen.
	mux.HandleFunc("GET /api/chat/sessions/{id}/children", chatSessionChildrenHandler)
	mux.HandleFunc("POST /api/chat/sessions/{id}/prompt", chatPromptHandler)
	mux.HandleFunc("POST /api/chat/sessions/{id}/abort", chatAbortHandler)
	// Commands: die Liste ohne die internen Bausteine, das Senden abgekoppelt —
	// OpenCode antwortet darauf erst nach dem ganzen Lauf — und der Ausgang des
	// abgekoppelten Aufrufs, den sonst kein Ereignis meldete.
	mux.HandleFunc("GET /api/chat/commands", chatCommandsHandler)
	// Die Agenten, unter denen die Seite wählen lässt: alles außer Subagenten
	// und versteckten.
	mux.HandleFunc("GET /api/chat/agents", chatAgentsHandler)
	mux.HandleFunc("POST /api/chat/sessions/{id}/command", state.chatCommandHandler)
	mux.HandleFunc("GET /api/chat/sessions/{id}/command-state", state.chatCommandStateHandler)
	mux.HandleFunc("GET /api/chat/permissions", chatPermissionsHandler)
	mux.HandleFunc("POST /api/chat/permissions/{id}/reply", chatPermissionReplyHandler)
	// Rückfragen des Agenten: die Liste gilt über alle Sitzungen, Antwort und
	// Ablehnung brauchen nur die Kennung der Anfrage.
	mux.HandleFunc("GET /api/chat/questions", chatQuestionsHandler)
	mux.HandleFunc("POST /api/chat/questions/{id}/reply", chatQuestionReplyHandler)
	mux.HandleFunc("POST /api/chat/questions/{id}/reject", chatQuestionRejectHandler)
	mux.HandleFunc("GET /api/chat/events", state.chatEventsHandler)
	// Rendert Antworttexte mit Goldmark; fragt OpenCode nicht.
	mux.HandleFunc("POST /api/chat/markdown", chatMarkdownHandler)

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("eingebettete Assets: %v", err))
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("GET /workflows", workflowsPageHandler)
	mux.HandleFunc("GET /workflows/tasks", tasksPageHandler)
	mux.HandleFunc("GET /workflows/reviews", reviewsPageHandler)
	mux.HandleFunc("GET /workflows/todos", todosPageHandler)
	mux.HandleFunc("GET /chat", chatPageHandler)
	mux.HandleFunc("GET /chat/{id}", chatSessionPageHandler)
	mux.HandleFunc("GET /github", githubPageHandler)
	mux.HandleFunc("GET /knowledge", knowledgePageHandler)
	mux.HandleFunc("GET /docs", docsPageHandler)
	mux.HandleFunc("GET /inventory", inventoryPageHandler)
	mux.HandleFunc("GET /mcp", mcpPageHandler)
	mux.HandleFunc("GET /mcp-servers", mcpServersPageHandler)
	mux.HandleFunc("GET /mcp-servers/{assistant}/{name}", mcpServerPageHandler)
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
	// areaChat ist der Bereich des Chats mit dem OpenCode-Dienst. Er steht
	// unter Workflows: auch dort geht es um die tägliche Arbeit, aber im
	// Gespräch statt über Dateien.
	areaChat = "chat"
	// areaKnowledge ist der Bereich der Wissensablage. Er steht über Docs:
	// Docs ist das Nachschlagewerk der Installation, die Wissensablage das,
	// was im Projekt an Wissen zusammenkommt — unter k-playbook-local/ in
	// den drei Zonen inbox/, queue/ und knowledge/, beschrieben in
	// docs/knowledge-layout.md. Die Seite zeigt vorerst das Zielbild; die
	// Zonen selbst listet sie erst, wenn ein /api/knowledge/* sie liefert.
	areaKnowledge = "knowledge"
	// areaGitHub ist der Bereich der GitHub-Ansicht. Eigener Bereich und keine
	// Karte auf der Startseite: die Seite fragt bei jedem Aufruf über gh nach
	// draußen, und das darf weder die Startseite noch das Menü auslösen.
	areaGitHub = "github"
	areaDocs   = "docs"
	// areaInventory ist der Bereich des Versionsinventars. Er steht neben
	// Docs, nicht darin: Docs zeigt die mitgelieferte Doku der Installation,
	// das Inventar ist eine erzeugte Datei des Projekts.
	areaInventory = "inventory"
)

// pageTemplate parst eine Seite zusammen mit den beiden gemeinsamen
// Fragmenten: dem Kopf und der linken Spalte. Die Vorlage trägt den Namen der
// Seitendatei, damit Execute die Seite ausführt und nicht ein Fragment.
//
// hasPrefix braucht die linke Spalte für den Unterpunkt „MCP-Server": der ist
// auf der Übersicht und auf jeder Detailseite darunter aktiv, aria-current
// führt aber nur die Übersicht selbst.
func pageTemplate(name string) *template.Template {
	funcs := template.FuncMap{"hasPrefix": strings.HasPrefix}
	return template.Must(template.New(name).Funcs(funcs).ParseFS(staticFiles, "static/"+name, "static/sidebar.html", "static/hero.html"))
}

var indexTemplate = pageTemplate("index.html")

// workflowsTemplate ist die Übersicht des Bereichs der täglichen Arbeit: was
// die drei Sorten sind, wie viel in jeder liegt und der Weg zu ihrer Seite.
// Die Listen selbst stehen auf den drei Seiten darunter — untereinander auf
// einer Seite waren sie eine Strecke, auf der man scrollte statt zu lesen.
var workflowsTemplate = pageTemplate("workflows.html")

func workflowsPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, workflowsTemplate, areaWorkflows, "/workflows", "Workflows")
}

// tasksTemplate ist die Seite der Tasks: die offenen, die erledigten und der
// gelesene Inhalt.
var tasksTemplate = pageTemplate("tasks.html")

func tasksPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, tasksTemplate, areaWorkflows, "/workflows/tasks", "Tasks")
}

// reviewsTemplate ist die Seite der Reviews: die bisherigen Läufe.
var reviewsTemplate = pageTemplate("reviews.html")

func reviewsPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, reviewsTemplate, areaWorkflows, "/workflows/reviews", "Reviews")
}

// todosTemplate ist die Seite der Todos: die offenen und die abgehakten.
var todosTemplate = pageTemplate("todos.html")

func todosPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, todosTemplate, areaWorkflows, "/workflows/todos", "Todos")
}

// chatTemplate ist die Übersicht des Chats: die Sitzungen und der Zustand des
// OpenCode-Dienstes, an den er weiterleitet.
var chatTemplate = pageTemplate("chat.html")

func chatPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, chatTemplate, areaChat, "/chat", "Chat")
}

// chatSessionTemplate ist die Seite einer einzelnen Sitzung: die Unterhaltung
// steht auf eigener Seite statt unter der Liste. Eine Kennung, die nicht wie
// eine von OpenCode aussieht, ist 404 — die Seite setzt sie in ihre Anfragen
// ein.
var chatSessionTemplate = pageTemplate("chat-session.html")

func chatSessionPageHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !openCodeID.MatchString(id) || !project.Detect().Installed {
		http.NotFound(w, r)
		return
	}
	renderPage(w, chatSessionTemplate, areaChat, "/chat/"+id, "Chat")
}

// knowledgeTemplate ist die Seite der Wissensablage. Vorerst zeigt sie genau
// eine Datei, knowledge-storage.md aus der mitgelieferten Doku; die Auflistung der
// abgelegten Einträge kommt später als weiterer Block darunter. Deshalb schon
// jetzt ein eigener Bereich und keine Karte im Bereich Docs.
var knowledgeTemplate = pageTemplate("knowledge.html")

func knowledgePageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, knowledgeTemplate, areaKnowledge, "/knowledge", "Knowledge")
}

// githubTemplate ist die Seite des GitHub-Stands: Repo und Zugang, Pull
// Requests, CI-Läufe. Eigener Bereich, weil ihre Karten als einzige der
// Oberfläche über das Netz fragen — als Karte auf der Startseite hinge jeder
// Aufruf von / an GitHub. Geschrieben wird nichts: Approve und Merge bleiben
// bei /k-pr-review.
var githubTemplate = pageTemplate("github.html")

func githubPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, githubTemplate, areaGitHub, "/github", "GitHub")
}

// docsTemplate ist die Seite zum Nachschlagen: der Index links im Menü, die
// gelesene Datei rechts.
var docsTemplate = pageTemplate("docs.html")

func docsPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, docsTemplate, areaDocs, "/docs", "Docs")
}

// inventoryTemplate ist die Seite des Versionsinventars: Stand, Anstoß der
// Erhebung, Quellenkonfiguration und die erzeugte Datei. Eigener Bereich mit
// eigenem Eintrag in der linken Spalte — die Datei muss im Bereich selbst
// lesbar sein, und der Aktualisieren-Knopf braucht Verlaufsstatus; beides
// trägt keine Karte auf der Startseite.
var inventoryTemplate = pageTemplate("inventory.html")

func inventoryPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, inventoryTemplate, areaInventory, "/inventory", "Versionsinventar")
}

// mcpTemplate ist die Seite des MCP-Servers. Sie ist eine Detailseite des
// Setup-Blocks und trägt deshalb dessen Bereich im Umschalter.
var mcpTemplate = pageTemplate("mcp.html")

func mcpPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, mcpTemplate, areaSetup, "/mcp", "k-playbook-MCP")
}

// mcpServersTemplate ist die Übersicht aller MCP-Server der drei Assistenten.
// Sie ist neben /mcp die zweite Seite des Setup-Bereichs und hat als einzige
// einen Unterpunkt in der linken Spalte: /mcp bleibt über die Karte und die
// Detailseite des eigenen Servers erreichbar.
var mcpServersTemplate = pageTemplate("mcp-servers.html")

// mcpServersPage ist der Pfad der Übersicht; die Detailseiten liegen darunter.
const mcpServersPage = "/mcp-servers"

func mcpServersPageHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, mcpServersTemplate, areaSetup, mcpServersPage, "MCP-Server")
}

// mcpServerTemplate ist die Detailseite eines einzelnen Servers bei einem
// Assistenten. Sie gibt es nur für Einträge, die in den Projektdateien
// stehen; alles andere ist 404. Beim Laden zeigt sie die Konfiguration, die
// Messung läuft erst auf Knopfdruck über den POST.
var mcpServerTemplate = pageTemplate("mcp-server.html")

func mcpServerPageHandler(w http.ResponseWriter, r *http.Request) {
	environment := project.Detect()
	if !environment.Installed {
		http.NotFound(w, r)
		return
	}
	assistant, name := r.PathValue("assistant"), r.PathValue("name")
	if _, ok := findMCPServer(environment.ProjectDir, assistant, name, mcpServerFileParam(r)); !ok {
		http.NotFound(w, r)
		return
	}
	renderPage(w, mcpServerTemplate, areaSetup, mcpServersPage+"/"+assistant+"/"+name, name)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	renderPage(w, indexTemplate, areaSetup, "/", "k-playbook")
}

// renderPage füllt den gemeinsamen Kopf und gibt die Vorlage aus. area sagt,
// welcher Eintrag des Umschalters markiert wird, page nennt die offene Seite.
// Beides fällt auseinander, sobald ein Bereich mehr als eine Seite hat: /mcp
// trägt den Bereich Setup, ist aber nicht dessen Startseite, und die drei
// Seiten unter /workflows tragen dessen Bereich — nur die offene Seite darf
// aria-current="page" führen. title ist die Überschrift im Kopf und der Name
// des Fensters.
func renderPage(w http.ResponseWriter, tmpl *template.Template, area string, page string, title string) {
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
		// Title steht im Kopf und im Fensternamen. Die Startseite trägt ihn
		// fest im Markup: sie hat als einzige einen eigenen Kopf, weil dort
		// die Pfade IDs für app.js brauchen und die Knöpfe für Update und
		// Dienst daneben stehen.
		Title string
		// Version ist die des Binarys, das diese Seite ausliefert. Sie steht
		// rechts oben im Kopf, weil die Installation daneben einen anderen
		// Stand tragen kann und ein Fenster nach einem Update sonst nicht
		// verrät, welcher Server gerade antwortet.
		Version string
	}{Installed: environment.Installed, Area: area, Page: page, Title: title, Version: displayVersion(guiproc.OwnVersion())}

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
// Die Antwort nennt Schlüssel, Version, Build-Kennung und PID, damit ein
// CLI-Aufruf den Server als seinen eigenen und als denselben Stand erkennt.
// Der Schlüssel wird je Anfrage neu berechnet: nach einem Umschlüsseln durch
// POST /api/config muss schon die nächste Antwort den neuen tragen. Version
// und Kennung dagegen sind die vom Start.
func (state *serverState) healthHandler(w http.ResponseWriter, r *http.Request) {
	key, _ := guiproc.Key()
	writeJSON(w, http.StatusOK, guiproc.Health{
		Status:  "ok",
		Key:     key,
		Version: state.version,
		Build:   state.build,
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

// displayVersion ist die Version für den Seitenkopf. Leer bleibt sie nur bei
// einem Ad-hoc-`go build` ohne Build-Flags; das soll sichtbar sein und nicht
// wie ein fehlendes Element aussehen.
func displayVersion(version string) string {
	if version == "" {
		return "ohne Version"
	}
	return version
}
