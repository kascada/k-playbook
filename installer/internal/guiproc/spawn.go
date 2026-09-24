package guiproc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ServeEnv ist die Marke, mit der der Aufruf sein eigenes Binary als Server
// startet. Gesetzt heißt Servermodus: nur der Server, keine Wirt-Pflege,
// keine Ausgabe außer ins Log.
const ServeEnv = "K_PLAYBOOK_SERVE"

// ServeMode meldet, ob dieser Prozess der abgekoppelte Server ist.
func ServeMode() bool {
	return os.Getenv(ServeEnv) == "1"
}

// StartupTimeout ist die Wartegrenze des Aufrufs auf das erste /api/health
// des eben gestarteten Servers.
const StartupTimeout = 10 * time.Second

const startupPollInterval = 100 * time.Millisecond

var (
	// ErrChildExited: der gestartete Server hat sich beendet, bevor er
	// antwortete — etwa, weil ein gleichzeitiger Start die Laufzeitdatei
	// zuerst geschrieben hat.
	ErrChildExited = errors.New("der Server hat sich vorzeitig beendet")
	// ErrStartupTimeout: der gestartete Server hat innerhalb der Wartegrenze
	// nicht geantwortet.
	ErrStartupTimeout = errors.New("der Server hat nicht rechtzeitig geantwortet")
)

// Child ist der abgekoppelte Serverprozess aus Sicht des Aufrufs.
type Child struct {
	Process *os.Process
	// Exited liefert das Ergebnis von Wait, sobald der Prozess endet.
	Exited <-chan error
	// LogPath ist die Datei, in die stdout und stderr des Kindes gehen.
	LogPath string
}

// HostCareEnv ist die zweite Marke neben ServeEnv: der Server pflegt vor dem
// Start den Wirt wie ein argumentloser Aufruf — MCP-Registrierung,
// Schreibschutz, Aufräumen von Altlasten —, öffnet aber keinen Browser.
//
// Gesetzt wird sie beim Neustart nach einer Programmaktualisierung. Dort gibt
// es keinen argumentlosen Aufruf, der die Pflege sonst übernähme, und das neue
// Programm soll sie mit seinem eigenen Code ausführen, nicht mit dem des alten
// Dienstes. Ein Programm, das die Marke nicht kennt, übergeht sie.
const HostCareEnv = "K_PLAYBOOK_HOST_CARE"

// HostCareMode meldet, ob dieser Server vor dem Start den Wirt pflegen soll.
func HostCareMode() bool {
	return os.Getenv(HostCareEnv) == "1"
}

// Spawn startet exe als abgekoppelten Server: eigene Sitzung (Setsid),
// unverändertes Arbeitsverzeichnis, stdin aus /dev/null, stdout und stderr in
// die Logdatei, die bei jedem Start neu beginnt. extraEnv kommt zur Umgebung
// dieses Prozesses und der Servermarke hinzu.
//
// Das Arbeitsverzeichnis bleibt bewusst: alle Handler leiten das Projekt
// daraus ab. Das chdir("/") klassischer Daemonisierung wäre hier genau der
// Fehler, der alles bricht.
//
// exe ist ein Parameter und nicht fest os.Executable(): der argumentlose
// Aufruf startet sich selbst, der Neustart nach einer Programmaktualisierung
// dagegen das Installationsziel. Das laufende Programm kann woanders liegen
// als ~/.local/bin/k-playbook, und gestartet werden soll die Datei, die auch
// der nächste Aufruf von `k-playbook` findet.
func Spawn(exe string, logPath string, extraEnv ...string) (*Child, error) {
	// O_TRUNC: jeder Start beginnt das Log neu. O_APPEND dazu, weil bei zwei
	// gleichzeitigen Starts beide Kinder dieselbe Datei halten — so
	// überschreibt der Verlierer nicht die Zeilen des Gewinners.
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("Logdatei anlegen: %w", err)
	}
	// Das Kind bekommt eigene Deskriptoren; die hier gehen nach dem Start zu.
	defer logFile.Close()
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return nil, err
	}
	defer devNull.Close()

	cmd := exec.Command(exe)
	cmd.Env = append(append(os.Environ(), ServeEnv+"=1"), extraEnv...)
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = detachAttr()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("Server starten: %w", err)
	}

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	return &Child{Process: cmd.Process, Exited: exited, LogPath: logPath}, nil
}

// Await wartet, bis das Kind seine Laufzeitdatei unter file geschrieben hat
// und /api/health mit key antwortet. Beides muss vom Kind selbst kommen —
// Datei mit seiner PID, Antwort mit seiner PID —, sonst zählte die Datei
// eines gleichzeitigen Starts. Endet das Kind vorher, ErrChildExited; läuft
// timeout ab, ErrStartupTimeout.
func (c *Child) Await(key string, file string, timeout time.Duration, probe func(addr string) (Health, error)) (Record, error) {
	deadline := time.Now().Add(timeout)
	for {
		select {
		case err := <-c.Exited:
			if err != nil {
				return Record{}, fmt.Errorf("%w: %v", ErrChildExited, err)
			}
			return Record{}, ErrChildExited
		default:
		}

		record, ok, err := Read(file)
		if err == nil && ok && record.PID == c.Process.Pid {
			if health, err := probe(record.Addr); err == nil && health.Key == key && health.PID == record.PID {
				return record, nil
			}
		}

		if time.Now().After(deadline) {
			return Record{}, ErrStartupTimeout
		}
		time.Sleep(startupPollInterval)
	}
}

// Log liest, was das Kind bisher geschrieben hat — für die Fehlermeldung.
func (c *Child) Log() string {
	data, err := os.ReadFile(c.LogPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Terminate schickt dem Kind SIGTERM; es räumt seine Laufzeitdatei dann
// selbst weg.
func (c *Child) Terminate() {
	_ = Terminate(c.Process.Pid)
}
