package kephalaion

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// defaultNodeListen ist die Adresse, auf der der Node lauscht, wenn der
// Abschnitt node: keinen Eintrag listen trägt — so legt Kephalaion selbst
// es aus (config.DefaultListenNode dort). kephalaion init schreibt listen nur
// mit --listen; ohne Eintrag ist das also der Normalfall.
const defaultNodeListen = "127.0.0.1:7433"

// Env ist die Kephalaion-Einrichtung dieses Rechners, soweit k-playbook sie
// braucht: wo die config liegt, ob es sie gibt, die Adresse des Nodes und das
// Token-Verzeichnis.
type Env struct {
	// ConfigDir ist das Verzeichnis der config des Users:
	// $XDG_CONFIG_HOME/kephalaion bzw. ~/.config/kephalaion.
	ConfigDir string
	// ConfigPresent sagt, ob ConfigDir/config.yaml existiert.
	ConfigPresent bool
	// Endpoint ist die URL von /mcp am Node, nur gesetzt, wenn die Adresse
	// eine zulässige Loopback-Adresse ist.
	Endpoint string
	// AddressProblem erklärt, warum Endpoint fehlt, obwohl die config da ist.
	AddressProblem string
	// ConfigError ist gesetzt, wenn die config da, aber nicht lesbar ist.
	ConfigError string
}

// DefaultConfigDir liefert das Verzeichnis der config des Users wie
// Kephalaion selbst (config.UserPath dort): $XDG_CONFIG_HOME/kephalaion, wenn
// die Variable einen absoluten Pfad trägt, sonst ~/.config/kephalaion.
func DefaultConfigDir() (string, error) {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" && filepath.IsAbs(v) {
		return filepath.Join(v, "kephalaion"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Heimatverzeichnis unbekannt: %w", err)
	}
	return filepath.Join(home, ".config", "kephalaion"), nil
}

// LoadEnv liest die Kephalaion-Einrichtung unter configDir. Eine fehlende
// config ist kein Fehler, sondern ConfigPresent false.
func LoadEnv(configDir string) Env {
	env := Env{ConfigDir: configDir}
	data, err := os.ReadFile(filepath.Join(configDir, "config.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		return env
	}
	env.ConfigPresent = true
	if err != nil {
		env.ConfigError = "config.yaml nicht lesbar: " + err.Error()
		return env
	}
	root, err := yamllite.Parse(data)
	if err != nil {
		env.ConfigError = "config.yaml ist kein lesbares YAML (" + describeYAMLError(err) + ")"
		return env
	}
	node := root.Get("node")
	if node == nil || node.Kind != yamllite.Mapping {
		env.AddressProblem = "In der config von Kephalaion ist kein Node eingerichtet (Abschnitt node: fehlt)."
		return env
	}
	listen := strings.TrimSpace(node.Get("listen").Str())
	if listen == "" {
		listen = defaultNodeListen
	}
	endpoint, problem := loopbackEndpoint(listen)
	env.Endpoint, env.AddressProblem = endpoint, problem
	return env
}

// loopbackEndpoint bildet aus node.listen die URL von /mcp. Zulässig sind
// 127.0.0.1, [::1] und localhost; localhost wird nicht aufgelöst, sondern als
// 127.0.0.1 verwendet. Eine Wildcard (0.0.0.0, [::], leerer Host) wird zu
// 127.0.0.1 mit demselben Port. Jede andere Adresse ist ein Problem: Ein
// Token geht nie an eine Adresse außerhalb dieses Rechners.
func loopbackEndpoint(listen string) (endpoint string, problem string) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", "node.listen ist keine Adresse der Form host:port."
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Sprintf("node.listen %s: Port ist keine Zahl von 1 bis 65535.", quoteListen(listen))
	}
	switch host {
	case "127.0.0.1", "localhost", "", "0.0.0.0", "::":
		host = "127.0.0.1"
	case "::1":
		host = "[::1]"
	default:
		return "", fmt.Sprintf("node.listen %s ist keine Loopback-Adresse; k-playbook schickt ein Token nur an 127.0.0.1, [::1] oder localhost.", quoteListen(listen))
	}
	return "http://" + host + ":" + strconv.Itoa(n) + "/mcp", ""
}

// listenPattern ist die Form, in der node.listen in einer Meldung zitiert
// wird: Host aus Buchstaben, Ziffern, Punkt, Bindestrich oder eine
// IPv6-Adresse in Klammern, dazu ein Port aus Ziffern. Ein Token
// (keph_…) passt nie, weil der Unterstrich fehlt.
var listenPattern = regexp.MustCompile(`^(\[[0-9A-Fa-f:.]{2,45}\]|[A-Za-z0-9.-]{0,253}):[0-9]{1,5}$`)

// quoteListen zitiert node.listen für eine Meldung nur, wenn der Wert wie
// eine Adresse aussieht; sonst steht dort nur, dass er unzulässig ist. Die
// config ist eine fremde Datei — was dort versehentlich steht, gehört in
// keine Meldung.
func quoteListen(listen string) string {
	if listenPattern.MatchString(listen) {
		return fmt.Sprintf("%q", listen)
	}
	return "(unzulässiger Wert)"
}

// TokensDir ist das Token-Verzeichnis neben der config.
func (e Env) TokensDir() string {
	return filepath.Join(e.ConfigDir, "tokens")
}

// Candidate ist ein Paar aus Hub und Account, für das eine Token-Datei
// tokens/<hub>/<account>.token vorliegt. Der Pfad bleibt unexportiert: Er
// gelangt in keine Antwort und wird nur aus dieser Liste gelesen.
type Candidate struct {
	Hub     string `json:"hub"`
	Account string `json:"account"`
	path    string
}

// Pending ist ein Paar, für das nur eine Datei *.pending vorliegt: ein Token,
// das am Node noch auf seinen Tausch wartet.
type Pending struct {
	Hub     string `json:"hub"`
	Account string `json:"account"`
}

// Candidates listet die Paare aus dem Token-Verzeichnis, nach Hub und Account
// sortiert. Übergangen werden Dateien *.pending (sie stehen in pending, wenn
// es zum Paar keine .token gibt), Namen außerhalb der Namensregel, Symlinks
// und alles, was keine reguläre Datei ist.
func (e Env) Candidates() (candidates []Candidate, pending []Pending) {
	base := e.TokensDir()
	hubs, err := os.ReadDir(base)
	if err != nil {
		return nil, nil
	}
	for _, hub := range hubs {
		if !hub.IsDir() || CheckName("Hub", hub.Name()) != nil {
			continue
		}
		files, err := os.ReadDir(filepath.Join(base, hub.Name()))
		if err != nil {
			continue
		}
		have := map[string]bool{}
		var waiting []string
		for _, file := range files {
			if !file.Type().IsRegular() {
				continue
			}
			name := file.Name()
			if account, ok := strings.CutSuffix(name, ".token"); ok {
				if checkAccountName(account) != nil {
					continue
				}
				have[account] = true
				candidates = append(candidates, Candidate{
					Hub: hub.Name(), Account: account,
					path: filepath.Join(base, hub.Name(), name),
				})
				continue
			}
			if rest, ok := strings.CutSuffix(name, ".pending"); ok {
				account := strings.TrimSuffix(rest, ".token")
				if checkAccountName(account) == nil {
					waiting = append(waiting, account)
				}
			}
		}
		for _, account := range waiting {
			if !have[account] {
				pending = append(pending, Pending{Hub: hub.Name(), Account: account})
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Hub != candidates[j].Hub {
			return candidates[i].Hub < candidates[j].Hub
		}
		return candidates[i].Account < candidates[j].Account
	})
	sort.Slice(pending, func(i, j int) bool {
		if pending[i].Hub != pending[j].Hub {
			return pending[i].Hub < pending[j].Hub
		}
		return pending[i].Account < pending[j].Account
	})
	return candidates, dedupePending(pending)
}

func dedupePending(list []Pending) []Pending {
	out := list[:0]
	for i, p := range list {
		if i > 0 && p == list[i-1] {
			continue
		}
		out = append(out, p)
	}
	return out
}

// FindCandidate liefert den Kandidaten, der genau zu hub und account passt.
// Die Werte werden vorher nach der Namensregel geprüft; ein Paar ohne
// Kandidat liefert ok false. Der Token-Pfad kommt allein aus der Liste.
func (e Env) FindCandidate(hub, account string) (Candidate, bool) {
	if CheckName("Hub", hub) != nil || checkAccountName(account) != nil {
		return Candidate{}, false
	}
	candidates, _ := e.Candidates()
	var found []Candidate
	for _, c := range candidates {
		if c.Hub == hub && c.Account == account {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		return Candidate{}, false
	}
	return found[0], true
}

// tokenPrefix und tokenLength beschreiben die Form eines Tokens (ident.go in
// Kephalaion): keph_ und 32 Bytes base64url ohne Padding, also 43 Zeichen.
const (
	tokenPrefix = "keph_"
	tokenLength = len(tokenPrefix) + 43
)

// errTokenUnusable meldet eine Token-Datei, die sich nicht lesen lässt oder
// kein Token der erwarteten Form enthält. Der Fehler nennt weder Inhalt noch
// Länge.
var errTokenUnusable = errors.New("Token-Datei nicht lesbar oder ohne gültiges Token")

// readToken liest das Token eines Kandidaten. Es wird nur über die Form
// geprüft — keph_ und 43 Zeichen aus dem base64url-Alphabet —, damit kein
// Zeilenumbruch oder Steuerzeichen in einen Header gerät.
func (c Candidate) readToken() (string, error) {
	if c.path == "" {
		return "", errTokenUnusable
	}
	info, err := os.Lstat(c.path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return "", errTokenUnusable
	}
	data, err := os.ReadFile(c.path)
	if err != nil {
		return "", errTokenUnusable
	}
	token := strings.TrimSpace(string(data))
	if len(token) != tokenLength || !strings.HasPrefix(token, tokenPrefix) {
		return "", errTokenUnusable
	}
	for _, r := range token[len(tokenPrefix):] {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return "", errTokenUnusable
		}
	}
	return token, nil
}
