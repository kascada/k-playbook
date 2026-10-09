package vscodeext

import (
	"errors"
	"os"
	"testing"
)

// EnvNoInstall sperrt jeden Aufruf einer VS-Code-CLI, solange die Variable
// gesetzt und nicht leer ist. In einem Testlauf setzt sie das init() unten
// von selbst.
//
// **Warum eine Marke in der Umgebung und nicht ein sauber gehaltener PATH.**
// Ein Testlauf startet echte, gestempelte Programme als Dienste
// (internal/webui/restart_test.go und update_program_test.go über
// guiproc.Spawn), und ein Dienst pflegt beim Start die Erweiterung. Jede
// Abdichtung, die am einzelnen Test hängt — eigenes HOME, eigener PATH —,
// gilt nur dort, wo jemand daran gedacht hat: `isolateHome` lässt
// `/usr/local/bin:/usr/bin:/bin` im PATH stehen, weil die Tests dort git und
// sh finden müssen, und damit rief ein Testlauf ein `code` der Distribution
// wirklich mit `--install-extension … --force`. Dass das grün blieb, lag
// allein daran, dass auf dieser Maschine kein `code` in `/usr/bin` liegt. Der
// nächste neue Test, der einen Dienst startet, träte wieder hinein.
//
// Die Umgebung dagegen erbt jedes Kind von seinem Starter — guiproc.Spawn
// gibt `os.Environ()` weiter —, und zwar beliebig tief. Eine einzige Marke
// deckt deshalb den Testprozess und alles ab, was er startet, auch ein erst
// im Testlauf gebautes Programm.
//
// Die Variable ist zugleich der ausdrückliche Weg für einen Benutzer, der den
// selbsttätigen Nachzug nicht will; `docs/vscode.md` nennt sie.
const EnvNoInstall = "K_PLAYBOOK_NO_VSCODE_INSTALL"

// ErrNoInstall ist die Antwort von Install, solange die Marke steht. Sie
// nennt die Variable: ein Benutzer, der sie selbst gesetzt hat, soll an der
// Meldung erkennen, woran es liegt.
var ErrNoInstall = errors.New("nicht installiert: " + EnvNoInstall + " ist gesetzt")

// init setzt die Marke in jedem Test-Binary.
//
// testing.Testing() steht zur Linkzeit fest — der go-Befehl stempelt die
// Marke in jedes mit `go test` gebaute Binary — und ist deshalb schon hier in
// init() verlässlich. Genau diese Verlässlichkeit fehlte der früher
// verworfenen Prüfung „läuft unter go test"; was dort nicht trug, war die
// Vererbung an Kindprozesse, und die übernimmt die Umgebungsvariable.
//
// In init() und nicht erst beim ersten Aufruf: die Marke muss stehen, bevor
// ein Testlauf den ersten Prozess startet.
//
// Ein Test, der den Nachzug selbst prüft, hebt sie ausdrücklich auf
// (`t.Setenv(EnvNoInstall, "")`) — und dann ausschließlich mit einer Fake-CLI
// in isoliertem HOME.
func init() {
	if testing.Testing() {
		_ = os.Setenv(EnvNoInstall, "1")
	}
}

// installBlocked sagt, ob ein Aufruf der CLI gesperrt ist.
func installBlocked() bool {
	return os.Getenv(EnvNoInstall) != ""
}
