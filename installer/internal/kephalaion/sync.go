package kephalaion

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// SyncTimeout begrenzt den Abgleich eines Hubs (kephalaion node sync). Der
// Abgleich holt nur, was sich seit dem letzten geändert hat; eine Minute
// reicht auch nach Tagen ohne Abgleich. Austauschbar für Tests.
var SyncTimeout = time.Minute

// Höchstmengen der Ausgabe von kephalaion node sync, die SyncHub liest.
const (
	maxSyncOutput = 64 << 10
	maxSyncStderr = 8 << 10
)

// SyncError ist der Fehler von SyncHub: ein Satz ohne Token, der den Grund
// nennt und, wo es einen gibt, den Weg von Hand.
type SyncError struct {
	Message string
}

func (e *SyncError) Error() string { return e.Message }

// SyncHub gleicht die Replica dieses Nodes für einen Hub ab: kephalaion node
// sync <hub>, das Binary an den Orten von FindBinary. Gebraucht, wenn ein
// Leser sich auf eine frische Replica verlassen muss — etwa die Vorprüfung
// der Befund-Queue (project.CheckFindings), die sonst übersähe, was ein
// anderer Rechner gerade angelegt hat. Ein Fehler ist ein *SyncError: kein
// Binary, Exit ≠ 0 (die erste Zeile von stderr als Grund, sonst die letzte
// nicht leere Zeile von stdout), Zeitgrenze, Abbruch.
func SyncHub(ctx context.Context, hub string) error {
	// Der Hub wird ein Argument; die Namensregel schließt einen führenden
	// Bindestrich aus, also auch eine eingeschleuste Option.
	if err := CheckName("Hub", hub); err != nil {
		return &SyncError{Message: "Abgleich nicht möglich: " + err.Error() + "."}
	}
	path, ok := FindBinary()
	if !ok {
		return &SyncError{Message: "Abgleich nicht möglich: kephalaion ist nicht installiert. Abhilfe: " + InstallCommand + "."}
	}
	call := "kephalaion node sync " + hub

	runCtx, cancel := context.WithTimeout(ctx, SyncTimeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, path, "node", "sync", hub)
	stdout := &cappedBuffer{limit: maxSyncOutput}
	stderr := &cappedBuffer{limit: maxSyncStderr}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	// Ein Kind, das die Ausgabe offen hält, hält Wait sonst über die
	// Zeitgrenze hinaus fest.
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		return &SyncError{Message: fmt.Sprintf(
			"Der Abgleich (%s) hat nicht innerhalb von %s geendet. Abhilfe: %s von Hand aufrufen.", call, SyncTimeout, call)}
	case runCtx.Err() != nil:
		return &SyncError{Message: fmt.Sprintf("Der Abgleich (%s) wurde abgebrochen.", call)}
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return &SyncError{Message: fmt.Sprintf(
			"kephalaion unter %s lässt sich nicht starten (%s). Abhilfe: neu installieren mit %s.",
			path, sanitize(err.Error()), InstallCommand)}
	}
	reason := firstLine(stderr.String())
	if reason == "" {
		reason = lastLine(stdout.String())
	}
	how := "Exit " + strconv.Itoa(exitErr.ExitCode())
	if exitErr.ExitCode() < 0 {
		how = "beendet von außen"
	}
	return &SyncError{Message: fmt.Sprintf("Der Abgleich (%s) ist gescheitert (%s). Abhilfe: %s von Hand aufrufen und den Grund beheben.",
		call, withReason(how, reason), call)}
}

// lastLine ist die letzte nicht leere Zeile, bereinigt.
func lastLine(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return sanitize(lines[i])
		}
	}
	return ""
}
