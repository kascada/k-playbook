package webui

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Im Container darf der Chat nur einen OpenCode-Dienst im Container selbst
// benutzen. Ein Dienst des Hosts arbeitet mit dessen Dateisystem, Zugangsdaten
// und Freigaben; der Container wäre als Grenze wertlos.
//
// Eine Loopback-Adresse beweist das nicht: mit --network host ist 127.0.0.1 der
// Host. Geprüft wird deshalb, ob ein Prozess, den dieser Container sieht, den
// lauschenden Socket besitzt. /proc/net/tcp zeigt die Sockets des
// Netzwerk-Namensraums — bei --network host auch die des Hosts —, die
// fd-Verweise unter /proc/<pid>/fd dagegen nur Prozesse des eigenen
// PID-Namensraums.

// chatInContainer meldet, ob der Server in einem Container läuft. Eine
// Variable, damit Tests beide Fälle prüfen können.
var chatInContainer = func() bool {
	_, ok := containerMarker()
	return ok
}

// procRoot ist die Wurzel des proc-Dateisystems; Tests setzen ein
// nachgebautes.
var procRoot = "/proc"

// port liefert den Port der Adresse, ohne Angabe den des Schemas.
func (target openCodeTarget) port() string {
	address, err := url.Parse(target.baseURL)
	if err != nil {
		return ""
	}
	if port := address.Port(); port != "" {
		return port
	}
	if address.Scheme == "https" {
		return "443"
	}
	return "80"
}

// containerGuard liefert eine Meldung, wenn der Dienst unter target hier nicht
// benutzt werden darf, sonst "". Außerhalb eines Containers gibt es keine
// Einschränkung.
func (target openCodeTarget) containerGuard() string {
	if !chatInContainer() {
		return ""
	}
	const rule = "Im Container nutzt der Chat nur einen OpenCode-Dienst im Container selbst"

	address, err := url.Parse(target.baseURL)
	if err != nil || address.Hostname() == "" {
		return fmt.Sprintf("%s; die Adresse %s ist nicht lesbar.", rule, target.baseURL)
	}
	if host := address.Hostname(); host != "localhost" {
		if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
			return fmt.Sprintf("%s; %s zeigt nach außen.", rule, target.baseURL)
		}
	}
	port, err := strconv.Atoi(target.port())
	if err != nil {
		return fmt.Sprintf("%s; der Port in %s ist nicht lesbar.", rule, target.baseURL)
	}
	owned, err := listenerOwnedLocally(port)
	if err != nil {
		return fmt.Sprintf("%s; ob Port %d einem Prozess dieses Containers gehört, ließ sich nicht prüfen: %v.", rule, port, err)
	}
	if !owned {
		return fmt.Sprintf("%s; auf Port %d lauscht kein Prozess dieses Containers.", rule, port)
	}
	return ""
}

// listenerOwnedLocally meldet, ob ein sichtbarer Prozess einen lauschenden
// TCP-Socket auf port besitzt.
func listenerOwnedLocally(port int) (bool, error) {
	suffix := fmt.Sprintf(":%04X", port)
	inodes := map[string]bool{}
	readable := false
	for _, name := range []string{"tcp", "tcp6"} {
		data, err := os.ReadFile(filepath.Join(procRoot, "net", name))
		if err != nil {
			continue
		}
		readable = true
		for _, line := range strings.Split(string(data), "\n")[1:] {
			// Spalten: Index, lokale Adresse, entfernte Adresse, Zustand, …,
			// Inode an zehnter Stelle. Zustand 0A ist LISTEN.
			fields := strings.Fields(line)
			if len(fields) < 10 || fields[3] != "0A" || !strings.HasSuffix(fields[1], suffix) {
				continue
			}
			inodes[fields[9]] = true
		}
	}
	if !readable {
		return false, fmt.Errorf("%s nicht lesbar", filepath.Join(procRoot, "net", "tcp"))
	}
	if len(inodes) == 0 {
		return false, nil
	}

	// Fremde Prozesse ohne Leserecht fallen still heraus; sie können den
	// eigenen Dienst ohnehin nicht sein.
	links, err := filepath.Glob(filepath.Join(procRoot, "[0-9]*", "fd", "*"))
	if err != nil {
		return false, err
	}
	for _, link := range links {
		target, err := os.Readlink(link)
		if err != nil || !strings.HasPrefix(target, "socket:[") {
			continue
		}
		if inodes[strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")] {
			return true, nil
		}
	}
	return false, nil
}

// chatHint sagt, wie ein fehlender Dienst entsteht. Nur Text und Befehle:
// ausgeführt wird nichts, installiert wird bewusst im Terminal.
type chatHint struct {
	Text     string   `json:"text"`
	Commands []string `json:"commands,omitempty"`
}

// openCodeInstallHint unterscheidet, ob OpenCode installiert ist. Die Befehle
// nennen den Port der konfigurierten Adresse, damit ein gestarteter Dienst dort
// auch gefunden wird.
func openCodeInstallHint(target openCodeTarget) *chatHint {
	serve := "opencode serve --port " + target.port()
	where := "hier"
	if chatInContainer() {
		where = "in diesem Container"
	}
	if path := openCodeBinary(); path != "" {
		return &chatHint{
			Text:     fmt.Sprintf("OpenCode ist %s installiert (%s), aber unter %s läuft kein Dienst. Starten:", where, path, target.baseURL),
			Commands: []string{serve},
		}
	}
	text := fmt.Sprintf("OpenCode ist %s nicht installiert. Installieren und danach den Dienst starten:", where)
	if chatInContainer() {
		text += " Beides im Container — ein Dienst des Hosts wird nicht verwendet."
	}
	return &chatHint{
		Text:     text,
		Commands: []string{"curl -fsSL https://opencode.ai/install | bash", serve},
	}
}

// openCodeBinary sucht opencode im PATH und dort, wohin das offizielle
// Installationsskript schreibt. Leer, wenn es nirgends liegt.
func openCodeBinary() string {
	if path, err := exec.LookPath("opencode"); err == nil {
		return path
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".opencode", "bin", "opencode")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}
