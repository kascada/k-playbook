package procgroup

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// endingSignals sind die Signale, mit denen ein Aufrufer endet: Strg-C im
// Terminal und das übliche Beenden von außen.
var endingSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}

// NotifyContext installiert den Handler für SIGINT und SIGTERM und liefert
// einen Kontext, der mit dem Signal endet; der Aufrufer gibt ihn an die
// Abrufe weiter, die Run erreichen. stop nimmt den Handler zurück und beendet
// den Kontext — mit defer, wenn der Aufruf vorbei ist.
//
// Beim ersten Signal:
//
//  1. Run startet keine Gruppe mehr und kehrt nicht mehr zurück.
//  2. Läuft eine Gruppe aus Run, endet der Kontext, jede laufende Gruppe
//     bekommt SIGKILL, und der Handler wartet, bis Run sie eingesammelt hat —
//     höchstens WaitDelay.
//  3. Das Programm endet durch dasselbe Signal wie ohne Handler: Der Handler
//     wird zurückgenommen und das Signal erneut ausgelöst. Wer das Programm
//     gestartet hat, sieht das Ende durch das Signal, eine Shell also 130
//     bzw. 143.
//
// Ohne laufende Gruppe entfällt Schritt 2, auch das Ende des Kontexts: Das
// Programm endet dann sofort, so wie ohne Handler. Ein abgebrochener Kontext
// ließe einen Aufrufer sonst noch einen Fehler melden oder den nächsten
// Schritt beginnen, bevor das Signal ihn beendet. Ein Weiterlaufen mit
// Ersatzmeldung gibt es nicht.
//
// Ein Signal, das beim Start des Programms ignoriert war (etwa SIGINT für
// einen Hintergrundauftrag einer nicht-interaktiven Shell), bleibt ignoriert:
// Für dieses Signal wird kein Handler installiert.
//
// Wer ein Signal anders behandelt — der Dienst der Oberfläche fährt bei
// SIGINT und SIGTERM geordnet herunter —, ruft NotifyContext nicht auf.
func NotifyContext(parent context.Context) (ctx context.Context, stop context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	wanted := []os.Signal{}
	for _, sig := range endingSignals {
		if !signal.Ignored(sig) {
			wanted = append(wanted, sig)
		}
	}
	if len(wanted) == 0 {
		return ctx, cancel
	}

	received := make(chan os.Signal, 1)
	stopped := make(chan struct{})
	signal.Notify(received, wanted...)
	go func() {
		select {
		case sig := <-received:
			endBySignal(sig, cancel)
		case <-stopped:
		}
	}()
	return ctx, func() {
		signal.Stop(received)
		select {
		case <-stopped:
		default:
			close(stopped)
		}
		cancel()
	}
}

// HoldIfEnding hält den Aufrufer an, wenn ein Signal das Programm gerade
// beendet (NotifyContext). main ruft es, bevor es selbst einen Fehler meldet
// oder endet: So endet das Programm durch das Signal und nicht mit einem
// eigenen Exit-Code, auch wenn der Aufruf im selben Augenblick zurückkehrt.
func HoldIfEnding() {
	groups.mu.Lock()
	ending := groups.ending
	groups.mu.Unlock()
	if ending {
		holdForever()
	}
}

// endBySignal ist der Handler: erst die Gruppen, dann das Programm.
func endBySignal(sig os.Signal, cancel context.CancelFunc) {
	groups.mu.Lock()
	groups.ending = true
	pids := make([]int, 0, len(groups.running))
	for pid := range groups.running {
		pids = append(pids, pid)
	}
	var drained chan struct{}
	if len(pids) > 0 {
		drained = make(chan struct{})
		groups.drained = drained
	}
	groups.mu.Unlock()

	if len(pids) > 0 {
		cancel()
		for _, pid := range pids {
			_ = killGroup(pid)
		}
		select {
		case <-drained:
		case <-time.After(WaitDelay):
		}
	}

	signal.Reset(sig)
	number := sig.(syscall.Signal)
	_ = syscall.Kill(os.Getpid(), number)
	// Das erneut ausgelöste Signal beendet das Programm, ohne dass diese Zeile
	// zum Zug kommt. Nur wenn es blockiert wäre: Ende mit dem Code, den eine
	// Shell dafür zeigt.
	time.Sleep(time.Second)
	os.Exit(128 + int(number))
}
