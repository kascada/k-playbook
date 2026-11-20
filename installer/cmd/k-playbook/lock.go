package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/procgroup"
	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/worklock"
)

// lockUsage ist die Hilfe des Unterkommandos.
const lockUsage = `k-playbook lock <aktion> [--token <marke>] [--seconds <n>] [--force] [--expect <nummer>]
                [--purpose <text>] [--session <kennung>] [--dir <verzeichnis>] [--json]

Die Sperre des Arbeitsbaums dieses Projekts: Zu einer Zeit ändert höchstens ein
Lauf von /k-task-run den Arbeitsbaum. Dieselbe Mitte wie das MCP-Werkzeug
k_playbook_lock.

Aktionen:
  status    wer hält die Sperre, oder frei
  release   gibt frei — mit --token die eigene Marke; ohne Marke nur mit --force,
            und das nur, wenn der Nutzer es ausdrücklich will (etwa nach einer
            Sitzung, die ohne Freigabe endete). --expect <nummer> gibt dann nur
            frei, solange der Halter noch die Marke mit dieser Nummer hat.
  acquire   nimmt die Sperre und nennt die Marke, oder den Halter
  wait      wie acquire, wartet aber, solange belegt, höchstens --seconds
            (Standard 90, höchstens 100)
  check     prüft die eigene Marke (--token): gehalten, oder verloren mit Grund

Über die CLI genommen kennt die Sperre keinen langlebigen Halter-Prozess und
gilt deshalb nie als sicher verwaist.

Exit-Code 0 bei genommen, gehalten, freigegeben und status; 3 bei belegt oder
verloren; 1 bei einem Fehler. Die Sperrdatei liegt unter
.k-playbook/work/lock/ im Projekt und hält sich selbst aus Git heraus.
`

// lockExitNotYours ist der Exit-Code für belegt und verloren: kein Fehler,
// aber die Sperre gehört nicht dem Aufrufer.
const lockExitNotYours = 3

// Frist von wait, gleich der des MCP-Werkzeugs: unter den 120 Sekunden, nach
// denen ein Bash-Aufruf in beiden Clients abgebrochen oder in den Hintergrund
// geschoben wird.
const (
	lockCLIWaitDefault  = 90 * time.Second
	lockCLIWaitMax      = 100 * time.Second
	lockCLIWaitInterval = time.Second
)

func runLock(out io.Writer, args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(out, lockUsage)
		if len(args) == 0 {
			return errors.New("lock braucht eine Aktion")
		}
		return nil
	}
	action := args[0]
	flags := flag.NewFlagSet("lock", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	token := flags.String("token", "", "die eigene Marke")
	seconds := flags.Int("seconds", 0, "Frist von wait in Sekunden")
	force := flags.Bool("force", false, "ohne Marke freigeben")
	expect := flags.Int64("expect", 0, "nur freigeben, solange diese Nummer hält")
	purpose := flags.String("purpose", "", "Wozu, im Klartext")
	session := flags.String("session", "", "Sitzungskennung aus dem Briefing")
	dir := flags.String("dir", "", "Verzeichnis, ab dem nach dem Projekt gesucht wird")
	asJSON := flags.Bool("json", false, "Ergebnis als JSON")
	if err := flags.Parse(args[1:]); err != nil {
		fmt.Fprint(os.Stderr, lockUsage)
		return err
	}
	if rest := flags.Args(); len(rest) > 0 {
		fmt.Fprint(os.Stderr, lockUsage)
		return fmt.Errorf("lock erwartet keine weiteren Argumente: %s", strings.Join(rest, " "))
	}

	start := *dir
	if start == "" {
		workdir, err := os.Getwd()
		if err != nil {
			return err
		}
		start = workdir
	}
	environment := project.DetectFrom(start)
	if !environment.Installed {
		return fmt.Errorf("kein k-playbook-Projekt gefunden — gesucht ab %s aufwärts nach %s", start, project.ConfigFileName)
	}

	lock := worklock.New(environment.ProjectDir)
	holder := worklock.Holder{Client: "cli", Session: strings.TrimSpace(*session), Purpose: strings.TrimSpace(*purpose)}
	var result worklock.Result
	var err error
	switch action {
	case "status":
		result, err = lock.Status()
	case "release":
		result, err = lock.Release(*token, *force, *expect)
	case "acquire":
		result, err = lock.Acquire(*token, holder)
	case "wait":
		timeout := lockCLIWaitDefault
		if *seconds != 0 {
			timeout = time.Duration(*seconds) * time.Second
			if *seconds < 0 || timeout > lockCLIWaitMax {
				return fmt.Errorf("--seconds muss zwischen 1 und %d liegen", int(lockCLIWaitMax/time.Second))
			}
		}
		ctx, stop := procgroup.NotifyContext(context.Background())
		defer stop()
		result, err = lock.Wait(ctx, *token, holder, timeout, lockCLIWaitInterval)
	case "check":
		result, err = lock.Check(*token, holder)
	default:
		fmt.Fprint(os.Stderr, lockUsage)
		return fmt.Errorf("unbekannte Aktion: %s", action)
	}
	if err != nil {
		return err
	}

	if *asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return err
		}
	} else if _, err := fmt.Fprint(out, describeLock(result)); err != nil {
		return err
	}
	if result.State == worklock.StateBusy || result.State == worklock.StateLost {
		return exitError{code: lockExitNotYours}
	}
	return nil
}

// describeLock ist die Antwort im Klartext.
func describeLock(result worklock.Result) string {
	var text strings.Builder
	switch result.State {
	case worklock.StateAcquired:
		if result.Reentrant {
			fmt.Fprintf(&text, "Sperre weiter gehalten (erneut genommen), Marke %s\n", result.Token)
		} else {
			fmt.Fprintf(&text, "Sperre genommen, Marke %s\n", result.Token)
		}
		if result.TakenOver != nil {
			fmt.Fprintf(&text, "Übernommen von einem verwaisten Halter: %s\n", describeHolder(*result.TakenOver))
		}
	case worklock.StateBusy:
		if result.WaitedSeconds > 0 {
			fmt.Fprintf(&text, "Noch belegt nach %d s\n", result.WaitedSeconds)
		} else {
			text.WriteString("Belegt\n")
		}
	case worklock.StateHeld:
		fmt.Fprintf(&text, "Gehalten, Marke %s\n", result.Token)
	case worklock.StateLost:
		fmt.Fprintf(&text, "Verloren — %s\n", describeReason(result.Reason))
		if result.Holder == nil {
			text.WriteString("Die Sperre ist frei.\n")
		}
	case worklock.StateReleased:
		fmt.Fprintf(&text, "Freigegeben (Marke Nr. %d)\n", result.Number)
	case worklock.StateFree:
		text.WriteString("Frei\n")
	}
	if result.Holder != nil && result.State != worklock.StateReleased {
		fmt.Fprintf(&text, "Halter: %s\n", describeHolder(*result.Holder))
	}
	fmt.Fprintf(&text, "Sperrdatei: %s\n", result.Path)
	return text.String()
}

func describeHolder(view worklock.HolderView) string {
	parts := []string{fmt.Sprintf("Marke Nr. %d", view.Number)}
	if view.Client != "" {
		parts = append(parts, view.Client)
	}
	if view.Session != "" {
		parts = append(parts, "Sitzung "+view.Session)
	}
	if view.ClientSession != "" {
		parts = append(parts, view.ClientSession)
	}
	if view.Purpose != "" {
		parts = append(parts, "wozu: "+view.Purpose)
	}
	if !view.Since.IsZero() {
		parts = append(parts, "seit "+view.Since.Local().Format("2006-01-02 15:04:05"))
	}
	verwaist := map[string]string{worklock.OrphanedNo: "nein", worklock.OrphanedYes: "ja", worklock.OrphanedUncertain: "unsicher"}[view.Orphaned]
	parts = append(parts, "verwaist: "+verwaist+" ("+view.OrphanedWhy+")")
	return strings.Join(parts, ", ")
}

func describeReason(reason string) string {
	switch reason {
	case worklock.ReasonReleased:
		return "vom Halter freigegeben"
	case worklock.ReasonForced:
		return "vom Nutzer freigegeben (force)"
	case worklock.ReasonTakenOver:
		return "als verwaist übernommen"
	case worklock.ReasonFileMissing:
		return "die Sperrdatei fehlte"
	default:
		return "Grund unbekannt"
	}
}
