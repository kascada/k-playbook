package githubaccess

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Level ist die Stufe eines Problems; die Namen sind die der Umgebungsprüfung.
type Level string

const (
	// LevelError: Die Datei sagt etwas Falsches; es gibt dann keine Konfiguration.
	LevelError Level = "error"
	// LevelWarning: nur die einmalige Meldung der frisch angelegten Vorlage.
	LevelWarning Level = "warning"
)

// PathFile ist der Pfad eines Problems, das die Datei als Ganzes betrifft.
const PathFile = "file"

// Problem ist ein Eintrag der Problemliste: wo in der Datei (file,
// services.<name> oder services.<name>.access[<index>]), was nicht stimmt, und
// was zu tun ist. Die Zeile dient nur der Sortierung nach Dateiposition.
type Problem struct {
	Level   Level  `json:"level"`
	Path    string `json:"path"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
	line    int
}

// Access ist ein Zugang eines Dienstes, wie die Datei ihn nennt: type und die
// Schlüssel seines Typs, Pfade mit aufgelöstem ~. Disabled steht im JSON nur,
// wenn es wahr ist.
type Access struct {
	Type     string `json:"type"`
	Login    string `json:"login,omitempty"`
	File     string `json:"file,omitempty"`
	Key      string `json:"key,omitempty"`
	Host     string `json:"host,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Service ist ein Dienst: Name, Repository und die Zugänge in Vorrangreihenfolge.
type Service struct {
	Name   string   `json:"-"`
	Repo   string   `json:"repo"`
	Access []Access `json:"access"`
}

// Services sind die Dienste in Dateireihenfolge. Im JSON eine Abbildung von
// Name auf Dienst in dieser Reihenfolge, leer als {}.
type Services []Service

// MarshalJSON schreibt die Dienste als Objekt in Dateireihenfolge — eine
// Go-Map verlöre sie.
func (s Services) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, service := range s {
		if i > 0 {
			b.WriteByte(',')
		}
		name, err := json.Marshal(service.Name)
		if err != nil {
			return nil, err
		}
		body, err := json.Marshal(service)
		if err != nil {
			return nil, err
		}
		b.Write(name)
		b.WriteByte(':')
		b.Write(body)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// Lookup findet einen Dienst nach Namen.
func (s Services) Lookup(name string) (Service, bool) {
	for _, service := range s {
		if service.Name == name {
			return service, true
		}
	}
	return Service{}, false
}

// Names nennt die Dienstnamen in Dateireihenfolge.
func (s Services) Names() []string {
	names := make([]string, 0, len(s))
	for _, service := range s {
		names = append(names, service.Name)
	}
	return names
}

// Result ist das Ergebnis von Load: die Konfiguration oder die Probleme.
// Sobald ein error vorliegt, ist Services leer; die einmalige warning der
// frisch angelegten Vorlage zählt dabei nicht. Problems ist nie nil.
type Result struct {
	File     string    `json:"file"`
	Created  bool      `json:"created"`
	Services Services  `json:"services"`
	Problems []Problem `json:"problems"`
}

// HasErrors sagt, ob mindestens ein Problem die Stufe error trägt — dann gibt
// es keine Konfiguration, und check endet mit 1.
func (r Result) HasErrors() bool {
	for _, problem := range r.Problems {
		if problem.Level == LevelError {
			return true
		}
	}
	return false
}

// Load liest die Datei unter path und prüft sie vollständig. Fehlt sie, wird
// einmal die Vorlage angelegt (Created, eine warning). Alles, was die Datei
// selbst sagt, wird geprüft; nichts, worauf sie zeigt, wird gelesen.
func Load(path string) Result {
	result := Result{File: path, Problems: []Problem{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		created, problem := createTemplate(path)
		result.Created = created
		if problem != nil {
			result.Problems = append(result.Problems, *problem)
			if problem.Level == LevelError {
				return finish(result)
			}
		}
		// Genau ein zweites Lesen, keine Rekursion: Jetzt steht die Vorlage
		// da, oder jemand anderes hat die Datei zwischen Lesen und Anlegen
		// geschrieben. Fehlt sie dann noch immer, ist der Pfad belegt, ohne
		// eine lesbare Datei zu sein — ein symbolischer Link, dessen Ziel
		// fehlt: ReadFile sieht nichts, O_EXCL sieht etwas. Ein erneutes
		// Load fände denselben Zustand und liefe endlos.
		data, err = os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			result.Problems = append(result.Problems, occupiedPathProblem(path))
			return finish(result)
		}
	}
	if err != nil {
		result.Problems = append(result.Problems, Problem{
			Level: LevelError, Path: PathFile,
			Message: "Die Datei ist nicht lesbar: " + describeReadError(err) + ".",
			Hint:    "Rechte und Art von " + path + " prüfen: eine gewöhnliche Datei, die dir gehört, mit Modus 0600.",
		})
		return finish(result)
	}
	root, err := yamllite.Parse(data)
	if err != nil {
		result.Problems = append(result.Problems, Problem{
			Level: LevelError, Path: PathFile,
			Message: "Die Datei ist kein YAML, das k-playbook liest" + describeYAMLError(err) + ".",
			Hint:    "Einrückung und Schreibweise in " + path + " prüfen; das Muster steht in docs/github-access.md.",
		})
		return finish(result)
	}
	services, problems := validate(root)
	result.Services = services
	result.Problems = append(result.Problems, problems...)
	return finish(result)
}

// finish sortiert die Probleme nach Dateiposition und räumt bei einem error
// die Konfiguration weg: Konfiguration oder Probleme, nie beides.
func finish(result Result) Result {
	sort.SliceStable(result.Problems, func(i, j int) bool {
		return result.Problems[i].line < result.Problems[j].line
	})
	if result.HasErrors() {
		result.Services = nil
	}
	if result.Problems == nil {
		result.Problems = []Problem{}
	}
	return result
}

// occupiedPathProblem beschreibt einen Pfad, den ReadFile als fehlend meldet,
// obwohl die Vorlage dort nicht anzulegen war: Der erwartete Fall ist ein
// symbolischer Link, dessen Ziel fehlt. Die Meldung nennt den Link und sein
// Ziel; ohne Abhilfe könnte Load die Vorlage nie anlegen.
func occupiedPathProblem(path string) Problem {
	if info, err := os.Lstat(path); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		target, readErr := os.Readlink(path)
		if readErr != nil || target == "" {
			target = "(unbekannt)"
		}
		return Problem{Level: LevelError, Path: PathFile,
			Message: "Die Datei fehlt und lässt sich nicht anlegen: Der Pfad ist ein symbolischer Link, dessen Ziel " + target + " es nicht gibt.",
			Hint:    "Den Link " + path + " entfernen oder sein Ziel als Datei mit dem Inhalt services: {} anlegen (Modus 0600); das Format steht in docs/github-access.md."}
	}
	return Problem{Level: LevelError, Path: PathFile,
		Message: "Die Datei fehlt und lässt sich nicht anlegen: Der Pfad ist belegt, aber nicht lesbar.",
		Hint:    "Prüfen, was unter " + path + " liegt, und es entfernen; danach legt k-playbook die Vorlage an."}
}

// describeReadError nennt den Grund eines Lesefehlers ohne den Pfad zu
// wiederholen.
func describeReadError(err error) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "keine Leserechte"
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		if strings.Contains(pathErr.Err.Error(), "is a directory") {
			return "ein Verzeichnis statt einer Datei"
		}
		return pathErr.Err.Error()
	}
	return err.Error()
}

// yamlLinePattern findet die Zeilennummern im Fehlertext von yamllite.
var yamlLinePattern = regexp.MustCompile(`Zeile (\d+)`)

// describeYAMLError nennt die Zeilen, die yamllite beanstandet, und sonst
// nichts: Der Fehlertext von yamllite zitiert die Zeile, und die Datei ist
// eine fremde — was dort versehentlich steht, gehört in keine Meldung.
func describeYAMLError(err error) string {
	matches := yamlLinePattern.FindAllStringSubmatch(err.Error(), -1)
	if len(matches) == 0 {
		return ""
	}
	lines := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		if !seen[match[1]] {
			seen[match[1]] = true
			lines = append(lines, match[1])
		}
	}
	if len(lines) == 1 {
		return " (Zeile " + lines[0] + ")"
	}
	return " (Zeilen " + strings.Join(lines, ", ") + ")"
}

var (
	// namePattern ist die Namensregel für Dienste: Buchstaben, Ziffern, - und _.
	namePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	// repoPattern ist <owner>/<repo> auf github.com: der Owner wie ein
	// GitHub-Login, der Name wie ein Repository-Name.
	repoPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?/[A-Za-z0-9_.-]+$`)
)

// Die Typen eines Zugangs und ihre erlaubten Schlüssel neben type und disabled.
var accessKeys = map[string][]string{
	"gh":     {"login"},
	"token":  {"file"},
	"ssh":    {"key", "host"},
	"public": {},
}

const accessTypes = "gh, token, ssh und public"

// validate prüft den gelesenen Baum und sammelt alle Probleme.
func validate(root *yamllite.Node) (Services, []Problem) {
	var problems []Problem
	fileProblem := func(line int, message, hint string) {
		problems = append(problems, Problem{Level: LevelError, Path: PathFile, Message: message, Hint: hint, line: line})
	}
	if root.Kind != yamllite.Mapping {
		fileProblem(root.At(), "Die Datei ist auf oberster Ebene keine Abbildung mit dem Schlüssel services.",
			"Die Datei mit services: beginnen; das Muster steht in docs/github-access.md.")
		return nil, problems
	}
	for _, key := range root.MapKeys() {
		if key != "services" {
			fileProblem(root.Get(key).At(), "Der Schlüssel "+key+" auf oberster Ebene ist nicht vorgesehen; erlaubt ist nur services.",
				"Den Schlüssel entfernen oder seinen Inhalt unter services: einordnen.")
		}
	}
	for _, duplicate := range root.Duplicates {
		fileProblem(duplicate.Line, "Der Schlüssel "+duplicate.Key+" kommt auf oberster Ebene mehr als einmal vor.",
			"Die Einträge zu einem zusammenführen.")
	}

	services := root.Get("services")
	switch {
	case services == nil, services.Kind == yamllite.Scalar && strings.TrimSpace(services.Value) == "":
		// Kein services oder ein leeres services: — nichts eingetragen, kein Problem.
		return Services{}, problems
	case services.Kind != yamllite.Mapping:
		fileProblem(services.At(), "services ist keine Abbildung von Dienstnamen auf Dienste.",
			"Unter services: je Dienst einen Namen mit repo und access eintragen; services: {} heißt: noch nichts eingetragen.")
		return nil, problems
	}
	for _, duplicate := range services.Duplicates {
		problems = append(problems, Problem{
			Level: LevelError, Path: "services." + duplicate.Key,
			Message: "Der Dienst " + duplicate.Key + " ist mehr als einmal eingetragen; die Namen müssen eindeutig sein.",
			Hint:    "Einen der Einträge umbenennen oder entfernen.",
			line:    duplicate.Line,
		})
	}
	result := Services{}
	for _, name := range services.MapKeys() {
		service, serviceProblems := validateService(name, services.Get(name))
		problems = append(problems, serviceProblems...)
		result = append(result, service)
	}
	return result, problems
}

// validateService prüft einen Dienst: Name, repo, access und keine fremden
// Schlüssel.
func validateService(name string, node *yamllite.Node) (Service, []Problem) {
	path := "services." + name
	service := Service{Name: name, Access: []Access{}}
	var problems []Problem
	problem := func(line int, message, hint string) {
		problems = append(problems, Problem{Level: LevelError, Path: path, Message: message, Hint: hint, line: line})
	}
	if !namePattern.MatchString(name) {
		problem(node.At(), "Der Dienstname verstößt gegen die Namensregel; erlaubt sind Buchstaben, Ziffern, - und _.",
			"Den Dienst umbenennen.")
	}
	if node.Kind != yamllite.Mapping {
		problem(node.At(), "Der Dienst ist keine Abbildung mit repo und access.",
			"Unter dem Namen repo: <owner>/<repo> und access: mit den Zugängen eintragen.")
		return service, problems
	}
	for _, key := range node.MapKeys() {
		if key != "repo" && key != "access" {
			problem(node.Get(key).At(), "Der Schlüssel "+key+" gehört nicht zu einem Dienst; erlaubt sind repo und access.",
				"Den Schlüssel entfernen.")
		}
	}

	repo := node.Get("repo")
	switch {
	case repo == nil:
		problem(node.At(), "Dem Dienst fehlt repo.", "repo: <owner>/<repo> eintragen.")
	case repo.Kind != yamllite.Scalar || !repoPattern.MatchString(strings.TrimSpace(repo.Value)) || strings.HasSuffix(repo.Value, ".git"):
		problem(repo.At(), "repo ist nicht in der Form <owner>/<repo>.",
			"Nur <owner>/<repo> auf github.com eintragen, ohne URL und ohne .git.")
	default:
		service.Repo = strings.TrimSpace(repo.Value)
	}

	access := node.Get("access")
	switch {
	case access == nil:
		problem(node.At(), "Dem Dienst fehlt access.", "access: mit mindestens einem Zugang anlegen, etwa - type: gh mit login.")
	case access.Kind == yamllite.Scalar && strings.TrimSpace(access.Value) == "", access.Kind == yamllite.Sequence && len(access.Items) == 0:
		problem(access.At(), "access ist leer; mindestens ein Zugang ist nötig.", "Unter access: einen Zugang eintragen, etwa - type: gh mit login.")
	case access.Kind != yamllite.Sequence:
		problem(access.At(), "access ist keine Liste von Zugängen.", "Die Zugänge als Liste schreiben: je Eintrag - type: … mit seinen Schlüsseln.")
	default:
		for index, item := range access.List() {
			entry, accessProblems := validateAccess(fmt.Sprintf("%s.access[%d]", path, index), item)
			problems = append(problems, accessProblems...)
			service.Access = append(service.Access, entry)
		}
	}
	return service, problems
}

// validateAccess prüft einen Zugang: type, die Schlüssel seines Typs, die
// Pfade und disabled.
func validateAccess(path string, node *yamllite.Node) (Access, []Problem) {
	var access Access
	var problems []Problem
	problem := func(line int, message, hint string) {
		problems = append(problems, Problem{Level: LevelError, Path: path, Message: message, Hint: hint, line: line})
	}
	if node.Kind != yamllite.Mapping {
		problem(node.At(), "Der Zugang ist keine Abbildung mit type.", "Jeden Zugang als - type: … mit seinen Schlüsseln schreiben.")
		return access, problems
	}

	scalar := func(key string) (string, bool) {
		value := node.Get(key)
		if value == nil || value.Kind != yamllite.Scalar || strings.TrimSpace(value.Value) == "" {
			return "", false
		}
		return strings.TrimSpace(value.Value), true
	}

	if disabled := node.Get("disabled"); disabled != nil {
		switch value, _ := scalar("disabled"); value {
		case "true":
			access.Disabled = true
		case "false":
		default:
			problem(disabled.At(), "disabled hat einen anderen Wert als true oder false.", "disabled: true setzen oder den Schlüssel entfernen.")
		}
	}

	typ, ok := scalar("type")
	if !ok {
		problem(node.At(), "Dem Zugang fehlt type.", "type: eintragen; erlaubt sind "+accessTypes+".")
		return access, problems
	}
	access.Type = typ
	allowed, known := accessKeys[typ]
	if !known {
		problem(node.Get("type").At(), "type "+typ+" ist unbekannt; erlaubt sind "+accessTypes+".", "Einen der vier Typen eintragen.")
		return access, problems
	}
	for _, key := range node.MapKeys() {
		if key == "type" || key == "disabled" || contains(allowed, key) {
			continue
		}
		hint := "Den Schlüssel entfernen; ein Zugang vom Typ " + typ + " kennt nur type und disabled."
		if len(allowed) > 0 {
			hint = "Den Schlüssel entfernen; ein Zugang vom Typ " + typ + " kennt neben type und disabled nur " + strings.Join(allowed, " und ") + "."
		}
		problem(node.Get(key).At(), "Der Schlüssel "+key+" gehört nicht zu einem Zugang vom Typ "+typ+".", hint)
	}

	switch typ {
	case "gh":
		if login, ok := scalar("login"); ok {
			access.Login = login
		} else {
			problem(node.At(), "Dem Zugang vom Typ gh fehlt login.", "login: <gh-Login> eintragen, wie gh auth status ihn nennt.")
		}
	case "token":
		if file, ok := scalar("file"); ok {
			access.File = checkPath(path, "file", file, node.Get("file").At(), &problems)
		} else {
			problem(node.At(), "Dem Zugang vom Typ token fehlt file.", "file: <pfad der token-datei> eintragen, absolut oder mit ~/.")
		}
	case "ssh":
		key, hasKey := scalar("key")
		host, hasHost := scalar("host")
		switch {
		case hasKey && hasHost:
			problem(node.Get("host").At(), "Der Zugang vom Typ ssh nennt key und host; erlaubt ist genau eines.",
				"key (Pfad des Schlüssels) oder host (Alias aus ~/.ssh/config) behalten, das andere entfernen.")
		case hasKey:
			access.Key = checkPath(path, "key", key, node.Get("key").At(), &problems)
		case hasHost:
			access.Host = host
		default:
			problem(node.At(), "Dem Zugang vom Typ ssh fehlt key oder host.",
				"key: <pfad des privaten schlüssels> oder host: <alias aus ~/.ssh/config> eintragen.")
		}
	}
	return access, problems
}

// checkPath löst ~ auf und verlangt einen absoluten Pfad; ein relativer wäre
// je Aufrufer eine andere Datei. Liefert den aufgelösten Pfad, bei einem
// Problem den Wert unverändert.
func checkPath(problemPath, key, value string, line int, problems *[]Problem) string {
	expanded, err := expandHome(value)
	if err != nil {
		*problems = append(*problems, Problem{Level: LevelError, Path: problemPath,
			Message: "Der Pfad in " + key + " beginnt mit ~, aber das Heimatverzeichnis ist unbekannt.",
			Hint:    "HOME setzen oder den Pfad absolut eintragen.", line: line})
		return value
	}
	if !filepath.IsAbs(expanded) {
		*problems = append(*problems, Problem{Level: LevelError, Path: problemPath,
			Message: "Der Pfad in " + key + " ist relativ.",
			Hint:    "Absoluten Pfad oder ~/… eintragen.", line: line})
		return value
	}
	return filepath.Clean(expanded)
}

// expandHome ersetzt ein führendes ~ oder ~/ durch das Heimatverzeichnis.
// ~name für andere Nutzer wird nicht aufgelöst und bleibt ein relativer Pfad.
func expandHome(value string) (string, error) {
	if value != "~" && !strings.HasPrefix(value, "~/") {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(value, "~")), nil
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
