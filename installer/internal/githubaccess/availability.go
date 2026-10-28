package githubaccess

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/ghconfig"
)

// Die Verfügbarkeit eines Zugangs (docs/github-access.md, Abschnitt
// Availability): ob er auf diesem Rechner versucht werden kann, beurteilt ohne
// Netz und deutlich unter einer Sekunde. Sie ist ein eigener Schritt nach
// Load — Load liest nur die Datei und startet keinen Subprozess —, und sie
// steht in check, show und in der Umgebungsprüfung. Der Test gegen GitHub
// (test.go) ist etwas anderes: der fragt GitHub und läuft nur auf Aufruf.

// State ist der Zustand eines Zugangs. Jeder Zugang hat genau einen.
type State string

const (
	// StateAvailable: Alles, was der Zugang auf diesem Rechner braucht, ist da.
	StateAvailable State = "available"
	// StateUnavailable: Etwas fehlt; Reason sagt in einem Satz, was und was
	// zu tun ist.
	StateUnavailable State = "unavailable"
	// StateDisabled: disabled: true in der Datei. Nicht beurteilt, nie gewählt.
	StateDisabled State = "disabled"
)

// sshResolveTimeout begrenzt `ssh -G`: Der Aufruf liest nur die lokale
// Konfiguration und ist in Millisekunden fertig; die Frist fängt eine
// hängende Konfiguration (etwa ein Match exec) ab, ohne die Prüfung
// spürbar zu verlangsamen.
const sshResolveTimeout = 3 * time.Second

// Judge beurteilt jeden Zugang jedes Dienstes und trägt State und Reason ein.
// Der Inhalt einer Token-Datei wird gelesen, um ihn zu beurteilen, und
// verlässt die Funktion nicht: Kein Feld, keine Meldung trägt ihn. Judge
// verändert die übergebenen Dienste und gibt sie zurück.
func Judge(services Services) Services {
	ghInstalled := false
	var ghAccounts []string
	ghRead := false
	for s := range services {
		for a := range services[s].Access {
			access := &services[s].Access[a]
			if access.Disabled {
				access.State, access.Reason = StateDisabled, ""
				continue
			}
			switch access.Type {
			case "gh":
				if !ghRead {
					ghRead = true
					_, err := exec.LookPath("gh")
					ghInstalled = err == nil
					ghAccounts = ghconfig.Accounts()
				}
				access.State, access.Reason = judgeGH(access.Login, ghInstalled, ghAccounts)
			case "token":
				access.State, access.Reason = judgeTokenFile(access.File)
			case "ssh":
				if access.Key != "" {
					access.State, access.Reason = judgeKeyFile(access.Key)
				} else {
					access.State, access.Reason = judgeSSHHost(access.Host)
				}
			case "public":
				access.State, access.Reason = StateAvailable, ""
			}
		}
	}
	return services
}

// Usable sagt, ob ein Dienst nutzbar ist: mindestens ein Zugang ist
// available. Setzt voraus, dass Judge gelaufen ist.
func Usable(service Service) bool {
	for _, access := range service.Access {
		if access.State == StateAvailable {
			return true
		}
	}
	return false
}

// FirstReason nennt den Grund des ersten Zugangs eines Dienstes ohne
// verfügbaren Zugang — für die Warnung der Umgebungsprüfung. Trägt der erste
// Zugang keinen Grund, weil er deaktiviert ist, sagt der Satz das.
func FirstReason(service Service) string {
	if len(service.Access) == 0 {
		return "kein Zugang eingetragen"
	}
	if reason := service.Access[0].Reason; reason != "" {
		return reason
	}
	return "alle Zugänge sind deaktiviert"
}

// judgeGH: gh liegt im PATH, und login ist einer der Accounts in gh's
// Konfiguration für github.com — aus den Dateien gelesen, nicht beim Server
// gefragt.
func judgeGH(login string, installed bool, accounts []string) (State, string) {
	if !installed {
		return StateUnavailable, "gh liegt nicht im PATH — gh installieren (https://cli.github.com) und mit gh auth login anmelden."
	}
	for _, account := range accounts {
		if account == login {
			return StateAvailable, ""
		}
	}
	if len(accounts) == 0 {
		return StateUnavailable, "Für " + ghconfig.Host + " ist in gh kein Konto hinterlegt — gh auth login ausführen und als " + login + " anmelden."
	}
	return StateUnavailable, "Der Login " + login + " ist in gh für " + ghconfig.Host + " nicht hinterlegt (hinterlegt: " + strings.Join(accounts, ", ") + ") — gh auth login ausführen und als " + login + " anmelden."
}

// judgeTokenFile: eine gewöhnliche Datei mit Modus 0600 und genau einer
// nicht-leeren Zeile. Gelesen wird nur, um das zu beurteilen; der Inhalt
// bleibt hier.
func judgeTokenFile(path string) (State, string) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return StateUnavailable, "Die Token-Datei fehlt — " + path + " anlegen (eine Zeile, Modus 0600) oder den Pfad in file korrigieren."
		}
		return StateUnavailable, "Die Token-Datei ist nicht lesbar: " + describeReadError(err) + " — Rechte von " + path + " prüfen."
	}
	if !info.Mode().IsRegular() {
		return StateUnavailable, "Die Token-Datei ist keine gewöhnliche Datei — unter " + path + " eine Datei mit dem Token anlegen."
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return StateUnavailable, fmt.Sprintf("Die Token-Datei hat den Modus %04o; erlaubt ist nur 0600 — chmod 600 %s ausführen.", perm, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return StateUnavailable, "Die Token-Datei ist nicht lesbar: " + describeReadError(err) + " — Rechte von " + path + " prüfen."
	}
	// Ein abschließender Zeilenumbruch gilt noch als eine Zeile: Editoren
	// hängen ihn an, und das Token davor ist dasselbe.
	content := bytes.TrimSuffix(bytes.TrimSuffix(data, []byte("\n")), []byte("\r"))
	switch {
	case len(bytes.TrimSpace(content)) == 0:
		return StateUnavailable, "Die Token-Datei ist leer — das Token als einzige Zeile in " + path + " eintragen."
	case bytes.ContainsAny(content, "\r\n"):
		return StateUnavailable, "Die Token-Datei enthält mehr als eine Zeile — nur das Token als einzige Zeile in " + path + " lassen."
	}
	return StateAvailable, ""
}

// judgeKeyFile: eine gewöhnliche Datei ohne Gruppen- und Weltrechte — SSH
// weist einen Schlüssel mit solchen Rechten ohnehin ab.
func judgeKeyFile(path string) (State, string) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return StateUnavailable, "Die Schlüsseldatei fehlt — den Pfad in key prüfen oder den Schlüssel nach " + path + " legen."
		}
		return StateUnavailable, "Die Schlüsseldatei ist nicht lesbar: " + describeReadError(err) + " — Rechte von " + path + " prüfen."
	}
	if !info.Mode().IsRegular() {
		return StateUnavailable, "Die Schlüsseldatei ist keine gewöhnliche Datei — den Pfad in key prüfen: " + path + "."
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return StateUnavailable, fmt.Sprintf("Die Schlüsseldatei hat den Modus %04o; SSH verlangt einen ohne Gruppen- und Weltrechte — chmod 600 %s ausführen.", perm, path)
	}
	return StateAvailable, ""
}

// judgeSSHHost: `ssh -G <host>` löst den Alias auf, und sein hostname ist
// github.com. Ein unbekannter Alias löst zu sich selbst auf und scheitert
// damit an dieser Regel; host: github.com besteht sie.
func judgeSSHHost(host string) (State, string) {
	if strings.HasPrefix(host, "-") {
		// Ginge als Option an ssh, nicht als Host.
		return StateUnavailable, "Der Alias " + host + " beginnt mit einem Bindestrich und kann kein SSH-Host sein — den Wert in host korrigieren."
	}
	if _, err := exec.LookPath("ssh"); err != nil {
		return StateUnavailable, "ssh liegt nicht im PATH — OpenSSH installieren."
	}
	ctx, cancel := context.WithTimeout(context.Background(), sshResolveTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "ssh", "-G", host)
	command.WaitDelay = time.Second
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return StateUnavailable, "ssh -G " + host + " hat nicht innerhalb von " + sshResolveTimeout.String() + " geantwortet — die SSH-Konfiguration prüfen."
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		if line, _, found := strings.Cut(detail, "\n"); found {
			detail = line
		}
		return StateUnavailable, "ssh -G " + host + " ist gescheitert: " + detail + " — den Eintrag in ~/.ssh/config prüfen."
	}
	hostname := ""
	for _, line := range strings.Split(stdout.String(), "\n") {
		if key, value, found := strings.Cut(strings.TrimSpace(line), " "); found && key == "hostname" {
			hostname = strings.TrimSpace(value)
			break
		}
	}
	if !strings.EqualFold(hostname, ghconfig.Host) {
		return StateUnavailable, "Der Alias " + host + " löst nicht nach " + ghconfig.Host + " auf (hostname " + hostname + ") — in ~/.ssh/config einen Block Host " + host + " mit HostName " + ghconfig.Host + " anlegen."
	}
	return StateAvailable, ""
}
