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

// EnvConfigVar ist die Umgebungsvariable, mit der Kephalaion den Ort der
// config ausdrücklich angibt (config.EnvConfig dort). Ist sie gesetzt, gilt
// nur sie.
const EnvConfigVar = "KEPHALAION_CONFIG"

// SystemConfig ist der feste Ort der globalen config von Kephalaion
// (config.SystemConfig dort).
const SystemConfig = "/etc/kephalaion/config.yaml"

// SystemConfigPath ist der Ort, an dem die Suche die globale config erwartet:
// SystemConfig. Tests lenken ihn in ein temporäres Verzeichnis, damit sie das
// echte /etc/kephalaion nicht sehen.
var SystemConfigPath = SystemConfig

// ConfigSource sagt, woher der Ort der geltenden config kommt — wie
// config.Source in Kephalaion, ohne --config.
type ConfigSource string

// Die Quellen, in der Reihenfolge der Suche.
const (
	// SourceEnv: KEPHALAION_CONFIG.
	SourceEnv ConfigSource = "env"
	// SourceUser: die config des Users — weil es sie gibt, oder als Ort, an
	// dem kephalaion init sie anlegt, wenn es weder sie noch die globale gibt.
	SourceUser ConfigSource = "user"
	// SourceSystem: die globale config, weil es die des Users nicht gibt.
	SourceSystem ConfigSource = "system"
)

// Env ist die Kephalaion-Einrichtung dieses Rechners, soweit k-playbook sie
// braucht: welche config gilt und woher, ob es sie gibt, die Adresse des
// Nodes und das Token-Verzeichnis. Die config kann global sein; die Tokens
// liegen immer beim User (tokens.go in Kephalaion).
type Env struct {
	// UserDir ist das Verzeichnis der config des Users:
	// $XDG_CONFIG_HOME/kephalaion bzw. ~/.config/kephalaion. Darin liegt das
	// Token-Verzeichnis, auch wenn die globale config gilt. Leer, wenn das
	// Heimatverzeichnis unbekannt ist.
	UserDir string
	// ConfigPath ist die config, die gilt, als absoluter Pfad. Gibt es keine,
	// ist es der Ort beim User, an dem kephalaion init sie anlegt.
	ConfigPath string
	// ConfigSource sagt, woher ConfigPath kommt.
	ConfigSource ConfigSource
	// ConfigPresent sagt, ob es die geltende config gibt.
	ConfigPresent bool
	// Endpoint ist die URL von /mcp am Node, nur gesetzt, wenn die Adresse
	// eine zulässige Loopback-Adresse ist.
	Endpoint string
	// AddressProblem erklärt, warum Endpoint fehlt, obwohl die config da ist.
	AddressProblem string
	// ConfigError ist gesetzt, wenn die config da, aber nicht lesbar ist; der
	// Grund ohne Pfad und ohne Inhalt der Datei.
	ConfigError string
}

// DefaultConfigDir liefert das Verzeichnis der config des Users wie
// Kephalaion selbst (config.UserPath dort): $XDG_CONFIG_HOME/kephalaion, wenn
// die Variable einen absoluten Pfad trägt, sonst ~/.config/kephalaion. Darin
// liegt das Token-Verzeichnis, gleich welche config gilt.
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

// DefaultEnv ermittelt die Kephalaion-Einrichtung dieses Rechners in der
// Reihenfolge, in der Kephalaion ohne --config seine config sucht
// (config.Locate dort): KEPHALAION_CONFIG, absolut gemacht > die config des
// Users, wenn es sie gibt > die globale (/etc/kephalaion/config.yaml), wenn
// es sie gibt > der Ort beim User. Das Token-Verzeichnis bleibt in jedem Fall
// das des Users. Eine fehlende config ist kein Fehler, sondern ConfigPresent
// false.
func DefaultEnv() Env {
	userDir, err := DefaultConfigDir()
	if err != nil {
		userDir = ""
	}
	return locateEnv(userDir)
}

// locateEnv sucht die config wie DefaultEnv, mit userDir als Verzeichnis des
// Users (leer, wenn es unbekannt ist), und liest sie.
func locateEnv(userDir string) Env {
	env := Env{UserDir: userDir}
	userPath := ""
	if userDir != "" {
		userPath = filepath.Join(userDir, "config.yaml")
	}
	switch {
	case os.Getenv(EnvConfigVar) != "":
		value := os.Getenv(EnvConfigVar)
		path, err := filepath.Abs(value)
		if err != nil {
			path = filepath.Clean(value)
		}
		env.ConfigPath, env.ConfigSource = path, SourceEnv
	case userPath != "" && userFileExists(userPath):
		env.ConfigPath, env.ConfigSource = userPath, SourceUser
	case fileExists(SystemConfigPath):
		env.ConfigPath, env.ConfigSource = filepath.Clean(SystemConfigPath), SourceSystem
	default:
		// Keine config: Erwartet wird sie beim User. Gelesen wird nichts —
		// auch dann nicht, wenn die config des Users nur unzugänglich ist.
		env.ConfigPath, env.ConfigSource = userPath, SourceUser
		return env
	}
	loadConfig(&env)
	return env
}

// fileExists sagt, ob es path gibt. Lässt sich das nicht feststellen, etwa
// mangels Rechten, zählt die Datei als vorhanden: Dann meldet das Lesen den
// Fehler, statt dass still eine andere config gilt (wie in Kephalaion).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

// userFileExists sagt, ob es die config des Users gibt. Anders als bei der
// globalen zählt eine, auf die der Aufrufer nicht zugreifen darf, als nicht
// vorhanden: Sie gehört einem anderen User — etwa unter sudo mit dem HOME des
// Aufrufers (wie in Kephalaion).
func userFileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, fs.ErrPermission)
}

// LoadEnv liest die Kephalaion-Einrichtung, deren config beim User unter
// userDir liegt — ohne Suche, für Aufrufer, die den Ort schon kennen, etwa
// Tests. Eine fehlende config ist kein Fehler, sondern ConfigPresent false.
func LoadEnv(userDir string) Env {
	env := Env{UserDir: userDir, ConfigPath: filepath.Join(userDir, "config.yaml"), ConfigSource: SourceUser}
	loadConfig(&env)
	return env
}

// loadConfig liest die config unter env.ConfigPath und setzt ConfigPresent,
// ConfigError, Endpoint und AddressProblem.
func loadConfig(env *Env) {
	data, err := os.ReadFile(env.ConfigPath)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	env.ConfigPresent = true
	if err != nil {
		env.ConfigError = readFailure(err)
		return
	}
	root, err := yamllite.Parse(data)
	if err != nil {
		env.ConfigError = "kein lesbares YAML (" + describeYAMLError(err) + ")"
		return
	}
	node := root.Get("node")
	if node == nil || node.Kind != yamllite.Mapping {
		env.AddressProblem = "In der config von Kephalaion ist kein Node eingerichtet (Abschnitt node: fehlt)."
		return
	}
	listen := strings.TrimSpace(node.Get("listen").Str())
	if listen == "" {
		listen = defaultNodeListen
	}
	endpoint, problem := loopbackEndpoint(listen)
	env.Endpoint, env.AddressProblem = endpoint, problem
}

// readFailure beschreibt einen Lesefehler ohne den Pfad, den die Meldung
// ohnehin nennt. Fehlt das Leserecht, sagt es das in Worten: Das ist der
// häufige Fall bei einer globalen config.
func readFailure(err error) string {
	if errors.Is(err, fs.ErrPermission) {
		return "Leserecht für diesen User fehlt"
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return "Lesefehler (" + pathErr.Err.Error() + ")"
	}
	return "Lesefehler (" + err.Error() + ")"
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

// TokensDir ist das Token-Verzeichnis neben der config des Users — auch
// wenn die globale config gilt. Leer, wenn das Heimatverzeichnis unbekannt
// ist.
func (e Env) TokensDir() string {
	if e.UserDir == "" {
		return ""
	}
	return filepath.Join(e.UserDir, "tokens")
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
	if base == "" {
		return nil, nil
	}
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
