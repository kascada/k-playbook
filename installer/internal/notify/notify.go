// Package notify zeigt dem Nutzer eine Meldung auf dem Rechner, auf dem das
// Programm läuft: ein Popup, auf Wunsch mit Ton. Gedacht für „die KI ist
// fertig“, wenn eine lange Arbeit endet und niemand mehr auf den Bildschirm
// schaut.
//
// Es gibt keinen gemeinsamen Weg zum Nutzer. Jede Plattform hat ihren eigenen,
// und in manchen Umgebungen gibt es gar keinen — ein Devcontainer hat weder
// Display noch Windows-Seite. Das Paket probiert die bekannten Wege in fester
// Reihenfolge und sagt ehrlich, wenn keiner geht; dieselbe Antwort bedient das
// Subkommando `k-playbook notify` und das MCP-Werkzeug k_playbook_notify.
package notify

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/kascada/k-playbook/installer/internal/hostenv"
)

// Request ist, was gemeldet werden soll. Title ist Pflicht, Body darf fehlen,
// Sound verlangt zusätzlich einen Ton.
type Request struct {
	Title string
	Body  string
	Sound bool
}

// Result sagt, was geschehen ist: über welchen Kanal die Meldung ging, ob ein
// Ton gespielt wurde, und — wenn Sound verlangt war, aber nicht ging — warum
// nicht.
type Result struct {
	Channel     string `json:"channel"`
	SoundPlayed bool   `json:"soundPlayed"`
	Reason      string `json:"reason,omitempty"`
}

// Die Kanäle, wie sie in Result.Channel stehen.
const (
	ChannelWindowsToast = "windows-toast" // WSL: Toast und Ton über die Windows-Seite
	ChannelMacOS        = "macos"         // osascript
	ChannelLinuxDesktop = "linux-desktop" // notify-send, Ton über paplay oder canberra
)

// bodyLimit ist die Länge, auf die der Inhalt gekürzt wird. Toasts schneiden
// ohnehin ab, nur ohne es zu sagen.
const bodyLimit = 200

// commandTimeout begrenzt jedes externe Programm. Eine Meldung, die nach zehn
// Sekunden noch nicht da ist, kommt nicht mehr.
const commandTimeout = 10 * time.Second

// ErrEmptyTitle: ohne Titel gibt es nichts zu melden.
var ErrEmptyTitle = errors.New("title fehlt")

// ErrNoChannel ist der Kern jeder NoChannelError; Aufrufer prüfen mit
// errors.Is darauf.
var ErrNoChannel = errors.New("keine Meldung möglich")

// NoChannelError sagt, dass dieser Rechner keinen Weg zum Nutzer hat, und
// warum: der Container-Marker, das fehlende Display, das fehlende Programm.
type NoChannelError struct {
	Reason string
}

func (e *NoChannelError) Error() string { return ErrNoChannel.Error() + ": " + e.Reason }

func (e *NoChannelError) Is(target error) bool { return target == ErrNoChannel }

// Austauschbare Umgebung, damit Tests jeden Kanal ohne Desktop und ohne
// Windows-Seite durchspielen — wie LookPath in vscodeext und chatInContainer
// in webui.
var (
	lookPath        = exec.LookPath
	runCommand      = execCommand
	getenv          = os.Getenv
	goos            = runtime.GOOS
	isWSL           = hostenv.IsWSL
	containerMarker = hostenv.ContainerMarker
)

// execCommand führt ein Programm aus und gibt im Fehlerfall dessen stderr
// zurück: erst die sagt, was schiefging.
func execCommand(ctx context.Context, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, name, args...)
	command.WaitDelay = time.Second
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		text := strings.TrimSpace(stderr.String())
		if text == "" {
			text = err.Error()
		}
		return fmt.Errorf("%s: %s", name, text)
	}
	return nil
}

// Send zeigt die Meldung über den ersten Kanal, der auf diesem Rechner geht.
//
// Der Fehler ist eine *NoChannelError, wenn kein Kanal geht; ErrEmptyTitle
// ohne Titel; sonst der Fehler des Programms, das die Meldung zeigen sollte.
func Send(ctx context.Context, req Request) (Result, error) {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return Result{}, ErrEmptyTitle
	}
	req.Body = truncate(strings.TrimSpace(req.Body), bodyLimit)

	for _, candidate := range channels() {
		if !candidate.available() {
			continue
		}
		return candidate.send(ctx, req)
	}
	return Result{}, &NoChannelError{Reason: noChannelReason()}
}

// channel ist ein Weg zum Nutzer: ob er hier geht, und wie er die Meldung
// zeigt.
type channel struct {
	name      string
	available func() bool
	send      func(ctx context.Context, req Request) (Result, error)
}

// channels liefert die Kanäle in der Reihenfolge, in der sie probiert werden.
// Windows kommt nur über WSL vor: gebaut werden Linux- und macOS-Binaries.
func channels() []channel {
	return []channel{
		{
			name:      ChannelWindowsToast,
			available: func() bool { return isWSL() && installed("powershell.exe") },
			send:      sendWindowsToast,
		},
		{
			name:      ChannelMacOS,
			available: func() bool { return goos == "darwin" && installed("osascript") },
			send:      sendMacOS,
		},
		{
			name:      ChannelLinuxDesktop,
			available: func() bool { return hasDisplay() && installed("notify-send") },
			send:      sendLinuxDesktop,
		},
	}
}

func installed(name string) bool {
	_, err := lookPath(name)
	return err == nil
}

func hasDisplay() bool {
	return strings.TrimSpace(getenv("DISPLAY")) != "" || strings.TrimSpace(getenv("WAYLAND_DISPLAY")) != ""
}

// noChannelReason erklärt, warum kein Kanal ging — in der Reihenfolge, in der
// ein Leser es nachvollziehen kann: erst die Umgebung, dann das Fehlende.
func noChannelReason() string {
	var reasons []string
	if marker, ok := containerMarker(); ok {
		reasons = append(reasons, fmt.Sprintf("Container erkannt (%s)", marker))
	}
	switch {
	case isWSL():
		reasons = append(reasons, "WSL ohne powershell.exe im PATH")
	case goos == "darwin":
		reasons = append(reasons, "osascript nicht gefunden")
	case !hasDisplay():
		reasons = append(reasons, "kein Display (DISPLAY und WAYLAND_DISPLAY leer)")
	default:
		reasons = append(reasons, "notify-send nicht installiert")
	}
	return strings.Join(reasons, "; ")
}

// truncate kürzt text auf limit Zeichen und hängt eine Ellipse an, wenn etwas
// wegfiel.
func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-1]) + "…"
}

// Windows über WSL. Der Toast kommt über die WinRT-Klassen, die PowerShell 5.1
// laden kann; das XML wird über XmlDocument.LoadXml gesetzt, denn
// GetTemplateContent().GetXml() liefert in PowerShell 5.1 eine Zeichenkette
// statt eines Dokuments. Als Absender dient die PowerShell-Verknüpfung des
// Startmenüs: ein Toast braucht eine registrierte AppId, und diese gibt es auf
// jedem Windows. Der Toast selbst bleibt stumm; den Ton spielt, wenn verlangt,
// der SoundPlayer mit einer Datei, die Windows mitbringt — so hängt der Ton
// nicht an den Benachrichtigungseinstellungen von Windows.
//
// Das Skript geht als -EncodedCommand (UTF-16LE, Base64) an PowerShell:
// so bleibt es von der Umschreibung der Argumente durch die WSL-Brücke
// unberührt, Anführungszeichen und Zeilenumbrüche eingeschlossen.
const windowsSoundFile = `C:\Windows\Media\notify.wav`

func windowsScript(req Request) string {
	var script strings.Builder
	script.WriteString("$ErrorActionPreference = 'Stop'\n")
	if req.Sound {
		fmt.Fprintf(&script, "(New-Object Media.SoundPlayer '%s').PlaySync()\n", windowsSoundFile)
	}
	script.WriteString("[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null\n")
	script.WriteString("[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null\n")
	script.WriteString("$xml = New-Object Windows.Data.Xml.Dom.XmlDocument\n")
	fmt.Fprintf(&script, "$xml.LoadXml('%s')\n", toastXML(req))
	script.WriteString("$toast = New-Object Windows.UI.Notifications.ToastNotification $xml\n")
	script.WriteString(`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe').Show($toast)` + "\n")
	return script.String()
}

// toastXML baut das Toast-Dokument. html.EscapeString ersetzt auch das
// einfache Anführungszeichen, sodass das Ergebnis in der einfach
// zitierten PowerShell-Zeichenkette keine Grenze sprengt.
func toastXML(req Request) string {
	return `<toast><visual><binding template="ToastText02"><text id="1">` + html.EscapeString(req.Title) +
		`</text><text id="2">` + html.EscapeString(req.Body) +
		`</text></binding></visual><audio silent="true"/></toast>`
}

// encodePowerShell kodiert ein Skript für -EncodedCommand.
func encodePowerShell(script string) string {
	units := utf16.Encode([]rune(script))
	raw := make([]byte, 0, 2*len(units))
	for _, unit := range units {
		raw = append(raw, byte(unit), byte(unit>>8))
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func sendWindowsToast(ctx context.Context, req Request) (Result, error) {
	args := []string{"-NoProfile", "-NonInteractive", "-EncodedCommand", encodePowerShell(windowsScript(req))}
	if err := runCommand(ctx, "powershell.exe", args...); err != nil {
		return Result{}, err
	}
	return Result{Channel: ChannelWindowsToast, SoundPlayed: req.Sound}, nil
}

// macOS: display notification über osascript, der Ton als Systemklang.
func appleScript(req Request) string {
	script := fmt.Sprintf("display notification %s with title %s", appleString(req.Body), appleString(req.Title))
	if req.Sound {
		script += ` sound name "Glass"`
	}
	return script
}

func appleString(text string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(text) + `"`
}

func sendMacOS(ctx context.Context, req Request) (Result, error) {
	if err := runCommand(ctx, "osascript", "-e", appleScript(req)); err != nil {
		return Result{}, err
	}
	return Result{Channel: ChannelMacOS, SoundPlayed: req.Sound}, nil
}

// Linux-Desktop: notify-send für das Popup; der Ton ist ein zweites Programm,
// und fehlt es, geht die Meldung trotzdem — mit dem Grund im Ergebnis.
const linuxSoundFile = "/usr/share/sounds/freedesktop/stereo/complete.oga"

func sendLinuxDesktop(ctx context.Context, req Request) (Result, error) {
	if err := runCommand(ctx, "notify-send", "--app-name", "k-playbook", "--", req.Title, req.Body); err != nil {
		return Result{}, err
	}
	result := Result{Channel: ChannelLinuxDesktop}
	if !req.Sound {
		return result, nil
	}
	switch {
	case installed("paplay"):
		if err := runCommand(ctx, "paplay", linuxSoundFile); err != nil {
			result.Reason = "Ton nicht gespielt: " + err.Error()
			return result, nil
		}
	case installed("canberra-gtk-play"):
		if err := runCommand(ctx, "canberra-gtk-play", "-i", "complete"); err != nil {
			result.Reason = "Ton nicht gespielt: " + err.Error()
			return result, nil
		}
	default:
		result.Reason = "Ton nicht gespielt: weder paplay noch canberra-gtk-play installiert"
		return result, nil
	}
	result.SoundPlayed = true
	return result, nil
}
