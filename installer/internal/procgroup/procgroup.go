// Package procgroup startet einen externen Prozess in einer eigenen
// Prozessgruppe und beendet die ganze Gruppe, wenn der Aufruf vorzeitig endet
// (Task 152): durch eine Frist, einen abgebrochenen Kontext oder ein Signal an
// den Aufrufer.
//
// Ohne eigene Gruppe beendet exec.CommandContext nur den direkten Kindprozess.
// Was der startet — beim install.sh von Kephalaion etwa der Download über
// curl —, läuft weiter, und WaitDelay gibt nur die Pipe frei, an der es hängt.
// Mit eigener Gruppe trifft das Beenden alle Prozesse, die die Gruppe nicht
// selbst verlassen haben (setsid, eigenes setpgid).
//
// Die eigene Gruppe hat eine Kehrseite: Strg-C im Terminal ist ein SIGINT an
// die Vordergrundgruppe, und zu der gehört der Prozess nicht mehr. Damit ein
// Signal an den Aufrufer die Gruppe trotzdem erreicht, installiert der
// Aufrufer NotifyContext (signal.go). Läuft dann eine Gruppe aus Run, beendet
// ein SIGINT oder SIGTERM zuerst die Gruppe und danach das Programm, wie es
// das Signal ohne Handler beendet hätte.
//
// Nur Linux und macOS: Andere Plattformen baut die CI nicht
// (.github/workflows/release.yml).
//
// Nicht abgedeckt ist SIGKILL an den Aufrufer: Dagegen hilft nur Pdeathsig,
// und das gibt es nur unter Linux und nur für das direkte Kind.
package procgroup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// WaitDelay ist die Wartezeit von Wait auf die Pipes, nachdem die Gruppe
// beendet wurde. Beendet ist damit jeder Prozess der Gruppe, und die Pipes
// schließen sofort; die Zeit zählt nur, wenn ein Enkel die Gruppe verlassen
// hat und die Ausgabe offen hält. Eine Variable, damit Tests sie verlängern
// können.
var WaitDelay = 5 * time.Second

// Command bereitet name mit args für Run vor: eigene Prozessgruppe, und endet
// ctx (Frist oder Abbruch), bekommt die ganze Gruppe SIGKILL statt nur der
// direkte Kindprozess.
//
// SIGKILL wie beim Standard von exec.CommandContext: Wer beim Abbruch eine
// Gnadenfrist braucht (kephalaion upgrade, program.KephalaionStopGrace),
// nimmt einen eigenen Weg.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killGroup(cmd.Process.Pid) }
	cmd.WaitDelay = WaitDelay
	return cmd
}

// Run startet cmd aus Command, wartet auf sein Ende und gibt den Fehler von
// Wait zurück.
//
// Solange cmd läuft, ist seine Gruppe angemeldet: Ein SIGINT oder SIGTERM an
// den Aufrufer beendet sie auch dann, wenn ctx nicht der Kontext aus
// NotifyContext ist. Hat ein solches Signal das Programm erreicht, kehrt Run
// nicht mehr zurück, sondern wartet, bis das Signal das Programm beendet —
// der Aufrufer läuft nach dem Signal nicht weiter und meldet nichts.
//
// Endet der Prozess von selbst, bleibt seine Gruppe stehen: Ein Enkel, den er
// absichtlich im Hintergrund weiterlaufen lässt, ist seine Sache.
func Run(cmd *exec.Cmd) error {
	groups.mu.Lock()
	if groups.ending {
		groups.mu.Unlock()
		holdForever()
	}
	// Start und Anmelden unter derselben Sperre: Ein Signal dazwischen sähe
	// sonst eine laufende Gruppe nicht.
	if err := cmd.Start(); err != nil {
		groups.mu.Unlock()
		return err
	}
	pid := cmd.Process.Pid
	groups.running[pid] = struct{}{}
	groups.mu.Unlock()

	err := cmd.Wait()

	groups.mu.Lock()
	delete(groups.running, pid)
	ending := groups.ending
	if ending && len(groups.running) == 0 && groups.drained != nil {
		close(groups.drained)
		groups.drained = nil
	}
	groups.mu.Unlock()
	if ending {
		holdForever()
	}
	return err
}

// groups sind die laufenden Gruppen aus Run, nach der Prozess-ID ihres
// Anführers, die zugleich die Gruppen-ID ist.
var groups = struct {
	mu      sync.Mutex
	running map[int]struct{}
	// ending: ein Signal beendet das Programm; Run startet nichts mehr und
	// kehrt nicht mehr zurück.
	ending bool
	// drained wird geschlossen, sobald nach dem Signal keine Gruppe mehr
	// läuft.
	drained chan struct{}
}{running: map[int]struct{}{}}

// killGroup schickt SIGKILL an die ganze Gruppe pgid. Gibt es keinen Prozess
// der Gruppe mehr, ist das kein Fehler, sondern os.ErrProcessDone — wie bei
// Process.Kill, dem Standard von exec.CommandContext.
func killGroup(pgid int) error {
	err := syscall.Kill(-pgid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

// holdForever hält die aufrufende Goroutine an, bis das Programm endet. Ein
// Timer statt select{}: Warten alle Goroutinen ohne Timer, meldete die
// Laufzeit einen Deadlock und endete mit eigenem Exit-Code statt durch das
// Signal.
func holdForever() {
	for {
		time.Sleep(time.Hour)
	}
}
