package guiproc

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ExitTimeout ist die Wartegrenze auf das Ende eines Servers, dem ein
// Shutdown geschickt wurde: deutlich über den 5 Sekunden, die der Server sich
// selbst zum Herunterfahren gibt — sonst gälte ein regulär endender Server
// als „lebt ohne Antwort".
const ExitTimeout = 10 * time.Second

// exitPollInterval ist der Takt, in dem WaitForExit nachsieht.
const exitPollInterval = 100 * time.Millisecond

// RequestShutdown schickt POST /api/shutdown an einen Server. Die Antwort
// kommt vor dem eigentlichen Ende; ob es eintritt, sagt WaitForExit.
func RequestShutdown(addr string) error {
	client := &http.Client{Timeout: healthTimeout}
	response, err := client.Post("http://"+addr+"/api/shutdown", "application/json", strings.NewReader(""))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("/api/shutdown: Status %d", response.StatusCode)
	}
	return nil
}

// WaitForExit wartet, bis die Laufzeitdatei unter path weg oder der Prozess
// pid tot ist, höchstens timeout lang. Beides gilt als Ende: die Datei
// verschwindet nach dem Schließen des Listeners, die PID nach dem Prozess.
func WaitForExit(path string, pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if _, ok, err := Read(path); err == nil && !ok {
			return true
		}
		if !ProcessAlive(pid) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(exitPollInterval)
	}
}

// ProcessAlive meldet, ob die PID vergeben ist. Allein sagt das nicht, ob es
// noch derselbe Prozess ist — dafür gibt es IdentityMatches.
func ProcessAlive(pid int) bool {
	return processAlive(pid)
}
