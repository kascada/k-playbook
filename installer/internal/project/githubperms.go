package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tailscale/hujson"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// Die Berechtigungen der Clients für GitHub (Task 190). Die Regel „GitHub über
// run“ (defaults/instructions/on-demand/github.md) steht als Text in den
// Anweisungen; was ein Client ohne Rückfrage durchlässt, entscheidet seine
// Berechtigungsliste. Hat das Projekt einen GitHub-Dienst, schreibt k-playbook
// deshalb Einträge, die nacktes gh und die Netzbefehle von git sperren und den
// Weg über `k-playbook github run` freigeben:
//
//   - gesperrt (deny): `gh *` und die Netzbefehle von git (clone, fetch, pull,
//     push, ls-remote, remote update), auch in der Form `git -C <pfad> …`;
//   - erlaubt (allow): `k-playbook github run <dienst> -- gh …` und
//     `-- git fetch|pull|clone|ls-remote|remote update …`;
//   - mit Rückfrage (ask): `run … --write …` und eine feste Liste
//     schreibender gh-Unterbefehle hinter `run * --`.
//
// Den Dienst schreiben die Muster nicht fest (`run *`): Ein eingebettetes Repo
// in einem Unterverzeichnis kann einen anderen Dienst haben.
//
// Die Ablage hat der Nutzer in Task 190 entschieden: Claude Code liest die
// Einträge aus .claude/settings.local.json, OpenCode aus
// .opencode/opencode.jsonc — beide nicht von git erfasst, weil die Zuordnung
// zu einem Dienst in der persönlichen Zugangsdatei steht und ein
// Teammitglied ohne sie mit nacktem gh arbeitet. Fehlt der Ausschluss aus git,
// trägt k-playbook die Datei in .git/info/exclude ein. Eine von git erfasste
// Datei bleibt unberührt.
//
// Wie die Clients die Muster anwenden, ist gemessen (Task 190, Etappe 1):
// Claude Code wertet deny vor ask vor allow, unabhängig von Datei und
// Reihenfolge, und fragt ohne passende Regel; OpenCode V2 nimmt die zuletzt
// passende Regel über alle Konfigurationsdateien, liest .opencode/opencode.jsonc
// zuletzt und erlaubt ohne passende Regel. Deshalb trägt nur die
// OpenCode-Fassung das breite ask `k-playbook github run *` vor den allow-Regeln
// — in Claude Code schlüge es jedes allow —, und die eigenen OpenCode-Einträge
// stehen am Ende des Objekts permission.bash.
//
// Bekannte Lücken, bewusst hingenommen und in github.md genannt: `*` trifft
// auch Text in Anführungszeichen (`run x -- git commit -m "a -- git fetch"`
// läuft durch); in OpenCode laufen Umwege wie `env gh`, `bash -c` oder ein
// absoluter Pfad ohne eigene Regel durch; schreibende Befehle hinter `run`
// fragen nur, soweit die Liste sie nennt oder `--write` dasteht. Und
// `git -C * fetch*` sperrt auch eine lokale Zeile, in der das Wort hinter -C
// irgendwo vorkommt.
//
// Eigentum: Berechtigungen haben keinen Schlüssel wie der MCP-Eintrag. Ein
// Eintrag in einer der beiden Zieldateien gehört k-playbook, wenn er Muster und
// Wirkung genau einer der eigenen Regeln trägt. Entfernt wird nur, wenn
// mindestens ein Eintrag mit `k-playbook github run` darunter ist — sonst
// stammt etwa ein einzelnes `gh *` vom Nutzer selbst. Ein vom Nutzer gleich
// geschriebener Eintrag in einer Zieldatei, neben den eigenen, wird beim
// Wegfall des Dienstes mit entfernt; das ist der Preis dafür, ohne Ledger und
// ohne Kommentare auszukommen, die Claude Code beim Zurückschreiben der Datei
// verlöre.

// gitHubRunPrefix ist der Anfang jeder Zeile, die über run läuft.
const gitHubRunPrefix = "k-playbook github run"

// Die Wirkungen einer Regel, so wie beide Clients sie schreiben.
const (
	permAllow = "allow"
	permAsk   = "ask"
	permDeny  = "deny"
)

// gitHubNetworkCommands sind die Netzbefehle von git, die nackt gesperrt sind.
var gitHubNetworkCommands = []string{"clone", "fetch", "pull", "push", "ls-remote", "remote update"}

// gitHubRunReadCommands sind die Netzbefehle von git, die hinter run ohne
// Rückfrage laufen. push fehlt: Es schreibt und läuft mit --write, das fragt.
var gitHubRunReadCommands = []string{"fetch", "pull", "clone", "ls-remote", "remote update"}

// gitHubWritingGHCommands sind die schreibenden gh-Unterbefehle, die hinter
// `run * --` auch ohne --write fragen (Task 190, Entscheidung Punkt 4).
var gitHubWritingGHCommands = []string{"pr merge", "pr create", "pr comment", "pr review", "pr close", "release", "repo"}

// gitHubPermissionRule ist eine Regel: das Muster der Befehlszeile und seine
// Wirkung. Bei Claude Code steht das Muster in der Datei als Bash(<muster>).
type gitHubPermissionRule struct {
	Pattern string
	Effect  string
}

// gitHubPermissionRules liefert die eigenen Regeln eines Clients in der
// Reihenfolge, in der sie geschrieben werden. Für Claude Code zählt die
// Reihenfolge nicht; für OpenCode gewinnt die zuletzt passende Regel.
func gitHubPermissionRules(client GitHubPermissionsClient) []gitHubPermissionRule {
	var rules []gitHubPermissionRule
	add := func(effect string, patterns ...string) {
		for _, pattern := range patterns {
			rules = append(rules, gitHubPermissionRule{Pattern: pattern, Effect: effect})
		}
	}

	add(permDeny, "gh *")
	for _, command := range gitHubNetworkCommands {
		add(permDeny, "git "+command+"*", "git -C * "+command+"*")
	}
	if client == GitHubPermissionsOpenCode {
		// Vor den allow-Regeln: Jede andere Zeile hinter run fragt. In
		// Claude Code fragt sie ohnehin, und ein breites ask schlüge dort
		// jedes allow.
		add(permAsk, gitHubRunPrefix+" *")
	}
	add(permAllow, gitHubRunPrefix+" * -- gh *")
	for _, command := range gitHubRunReadCommands {
		add(permAllow, gitHubRunPrefix+" * -- git "+command+"*")
	}
	add(permAsk, gitHubRunPrefix+" * --write *", gitHubRunPrefix+" --write *")
	for _, command := range gitHubWritingGHCommands {
		add(permAsk, gitHubRunPrefix+" * -- gh "+command+"*")
	}
	add(permAsk, gitHubRunPrefix+" * -- gh api*-X*", gitHubRunPrefix+" * -- gh api*--method*")
	return rules
}

// GitHubPermissionsClient benennt den Client einer Zieldatei.
type GitHubPermissionsClient string

const (
	// GitHubPermissionsClaude: Claude Code, permissions.allow|ask|deny als
	// Listen von Bash(<muster>).
	GitHubPermissionsClaude GitHubPermissionsClient = "claude"
	// GitHubPermissionsOpenCode: OpenCode, permission.bash als Objekt
	// Muster → allow|ask|deny.
	GitHubPermissionsOpenCode GitHubPermissionsClient = "opencode"
)

// GitHubPermissionsTarget ist die Datei, in die k-playbook die Berechtigungen
// eines Clients schreibt. Path ist relativ zur Projektwurzel, mit /.
type GitHubPermissionsTarget struct {
	Path      string                  `json:"path"`
	Assistant string                  `json:"assistant"`
	Client    GitHubPermissionsClient `json:"client"`
}

// GitHubPermissionsTargets sind die beiden Zieldateien (Task 190,
// Entscheidung Punkt 1 und 2).
func GitHubPermissionsTargets() []GitHubPermissionsTarget {
	return []GitHubPermissionsTarget{
		{Path: ".claude/settings.local.json", Assistant: "Claude Code", Client: GitHubPermissionsClaude},
		{Path: ".opencode/opencode.jsonc", Assistant: "OpenCode", Client: GitHubPermissionsOpenCode},
	}
}

// gitHubPermissionsProjectFiles sind die Dateien des Projekts, in denen ein
// Client außerdem Berechtigungen liest — geprüft auf Widersprüche, nie
// geschrieben. Für OpenCode in der Lesereihenfolge, die Zieldatei kommt danach.
func gitHubPermissionsProjectFiles(client GitHubPermissionsClient) []string {
	if client == GitHubPermissionsClaude {
		return []string{".claude/settings.json"}
	}
	return []string{opencodeConfigJSON, opencodeConfigJSONC, ".opencode/opencode.json"}
}

// GitHubPermissionsState ist der Zustand einer Zieldatei.
type GitHubPermissionsState string

const (
	// GitHubPermissionsOK: Das Projekt hat einen Dienst, und die eigenen
	// Einträge stehen vollständig (bei OpenCode am Ende, in Reihenfolge).
	GitHubPermissionsOK GitHubPermissionsState = "ok"
	// GitHubPermissionsMissing: Das Projekt hat einen Dienst, eigene Einträge
	// fehlen oder stehen nicht am richtigen Platz. Ein Lauf schreibt sie.
	GitHubPermissionsMissing GitHubPermissionsState = "missing"
	// GitHubPermissionsStale: Dem Projekt ist kein Dienst (mehr) zugeordnet,
	// eigene Einträge stehen noch. Ein Lauf entfernt sie.
	GitHubPermissionsStale GitHubPermissionsState = "stale"
	// GitHubPermissionsNone: kein Dienst, keine eigenen Einträge.
	GitHubPermissionsNone GitHubPermissionsState = "none"
	// GitHubPermissionsTracked: Die Datei ist von git erfasst (oder das ließ
	// sich nicht klären); k-playbook schreibt nicht hinein. Das ist ein
	// bewusst ungeschriebener Stand, keine Warnung wert.
	GitHubPermissionsTracked GitHubPermissionsState = "tracked"
	// GitHubPermissionsSymlink: Der Weg zur Datei führt über einen Link; sie
	// bleibt unberührt wie bei MCP.
	GitHubPermissionsSymlink GitHubPermissionsState = "symlink"
	// GitHubPermissionsUnreadable: Die Datei ist kein JSON-Objekt oder trägt
	// die Berechtigungen in einer Form, in die sich nichts eintragen lässt.
	GitHubPermissionsUnreadable GitHubPermissionsState = "unreadable"
	// GitHubPermissionsUnknown: Ob das Projekt einen Dienst hat, ließ sich
	// nicht klären (Zugangsdatei mit Fehlern, Ort unbekannt). Dann wird weder
	// geschrieben noch entfernt.
	GitHubPermissionsUnknown GitHubPermissionsState = "unknown"
)

// GitHubPermissionsStatus ist der geprüfte Zustand einer Zieldatei.
type GitHubPermissionsStatus struct {
	GitHubPermissionsTarget
	State  GitHubPermissionsState `json:"state"`
	Detail string                 `json:"detail,omitempty"`
	// Unexcluded: Die Datei trägt die eigenen Einträge, ist aber weder von
	// git erfasst noch ausgenommen. Ein Lauf trägt sie in .git/info/exclude
	// ein.
	Unexcluded bool `json:"unexcluded,omitempty"`
}

// GitHubPermissionConflict ist ein Eintrag des Projekts, der der Regel
// widerspricht: Für die Beispielzeile Line sagen die Einträge des Projekts
// (ohne die von k-playbook) Effect, gewollt ist Want.
type GitHubPermissionConflict struct {
	Assistant string `json:"assistant"`
	Path      string `json:"path"`
	// Rule ist der Eintrag, wie er in der Datei steht: Bash(<muster>) bei
	// Claude Code, das Muster bei OpenCode.
	Rule   string `json:"rule"`
	Effect string `json:"effect"`
	Line   string `json:"line"`
	Want   string `json:"want"`
	// Effective: Der Widerspruch wirkt auch mit den Einträgen von k-playbook,
	// so wie sie gerade stehen. Sonst überstimmen sie ihn.
	Effective bool `json:"effective"`
}

// GitHubPermissionsReport ist der Ausgang von Prüfung oder Schreiben.
type GitHubPermissionsReport struct {
	// Services sind die Dienste, die das Repo des Projekts nennen.
	Services []string `json:"services"`
	// Known: Ob das Projekt einen Dienst hat, ist geklärt. Sonst nennt
	// Reason den Grund, und nichts wird geschrieben oder entfernt.
	Known     bool                       `json:"known"`
	Reason    string                     `json:"reason,omitempty"`
	Statuses  []GitHubPermissionsStatus  `json:"statuses"`
	Conflicts []GitHubPermissionConflict `json:"conflicts"`
	// Written, Removed und Excluded nennen, was ein Lauf geschrieben hat:
	// Dateien mit neuen oder nachgezogenen Einträgen, Dateien, aus denen die
	// eigenen Einträge entfernt wurden, und Dateien, die in
	// .git/info/exclude eingetragen wurden.
	Written  []string `json:"written,omitempty"`
	Removed  []string `json:"removed,omitempty"`
	Excluded []string `json:"excluded,omitempty"`
	// Error sagt, was beim Schreiben scheiterte.
	Error string `json:"error,omitempty"`
}

// gitHubPermissionServices bestimmt die Dienste des Projekts; austauschbar,
// damit Tests ohne Zugangsdatei und Remote auskommen.
var gitHubPermissionServices = resolveGitHubPermissionServices

// resolveGitHubPermissionServices liest die Dienste, deren repo das Repo unter
// project.repo_root ist — derselbe Ort wie in der Oberfläche
// (githubClientRoute in webui/github.go, RepoRootDir), ohne die Beurteilung
// der Zugänge. Ist die Konfiguration nicht lesbar, gilt das Hauptverzeichnis.
// Ohne Zugangsdatei, ohne GitHub-Remote oder ohne passenden Dienst ist die
// Liste leer. Ein Fehler heißt: nicht geklärt.
func resolveGitHubPermissionServices(projectDir string) ([]string, error) {
	path, err := githubaccess.Path()
	if err != nil {
		return nil, fmt.Errorf("der Ort der GitHub-Zugangsdatei ist unbekannt: %w", err)
	}
	result := githubaccess.Load(path)
	if !result.Present {
		return nil, nil
	}
	if result.HasErrors() {
		return nil, fmt.Errorf("die GitHub-Zugangsdatei %s hat Fehler; k-playbook github check nennt sie", path)
	}

	ctx, cancel := context.WithTimeout(context.Background(), githubContextTimeout)
	defer cancel()
	gitDir := projectDir
	if config, err := ReadConfig(projectDir); err == nil {
		gitDir = RepoRootDir(projectDir, config)
	}
	repo := githubRemoteRepo(projectRemoteURL(ctx, gitDir), result.Services)
	if repo == "" {
		return nil, nil
	}
	var names []string
	for _, service := range result.Services {
		if strings.EqualFold(service.Repo, repo) {
			names = append(names, service.Name)
		}
	}
	return names, nil
}

// CheckGitHubPermissions prüft die Berechtigungen, ohne etwas zu schreiben.
func CheckGitHubPermissions(projectDir string) GitHubPermissionsReport {
	services, err := gitHubPermissionServices(projectDir)
	return checkGitHubPermissions(projectDir, services, err)
}

// ApplyGitHubPermissions zieht die Berechtigungen nach — derselbe Umfang für
// alle drei Einstiege (Knopf „Einrichten“, Projektschritt von
// `k-playbook upgrade`, Startpflege): fehlende eigene Einträge schreiben,
// eigene Einträge entfernen, wenn Dienst oder Zugangsdatei weggefallen sind,
// und die Zieldatei aus git ausnehmen. Eine von git erfasste Datei, eine
// Datei hinter einem Link und eine unlesbare Datei bleiben unberührt;
// Widersprüche des Projekts werden gemeldet, nie geändert. Der Bericht
// beschreibt den Zustand nach dem Lauf.
func ApplyGitHubPermissions(projectDir string) GitHubPermissionsReport {
	services, serviceErr := gitHubPermissionServices(projectDir)
	before := checkGitHubPermissions(projectDir, services, serviceErr)

	var written, removed, excluded []string
	var failures []error
	for _, status := range before.Statuses {
		switch status.State {
		case GitHubPermissionsMissing:
			if err := writeGitHubPermissions(projectDir, status.GitHubPermissionsTarget); err != nil {
				failures = append(failures, err)
				continue
			}
			written = append(written, status.Path)
			if gitExcludeNeeded(projectDir, status.Path) {
				if err := addGitExclude(projectDir, status.Path); err != nil {
					failures = append(failures, err)
				} else {
					excluded = append(excluded, status.Path)
				}
			}
		case GitHubPermissionsOK:
			if status.Unexcluded {
				if err := addGitExclude(projectDir, status.Path); err != nil {
					failures = append(failures, err)
				} else {
					excluded = append(excluded, status.Path)
				}
			}
		case GitHubPermissionsStale:
			if err := removeGitHubPermissions(projectDir, status.GitHubPermissionsTarget); err != nil {
				failures = append(failures, err)
				continue
			}
			removed = append(removed, status.Path)
		}
	}

	after := checkGitHubPermissions(projectDir, services, serviceErr)
	after.Written, after.Removed, after.Excluded = written, removed, excluded
	if err := errors.Join(failures...); err != nil {
		after.Error = err.Error()
	}
	return after
}

func checkGitHubPermissions(projectDir string, services []string, serviceErr error) GitHubPermissionsReport {
	report := GitHubPermissionsReport{
		Services:  append([]string{}, services...),
		Known:     serviceErr == nil,
		Conflicts: []GitHubPermissionConflict{},
	}
	if serviceErr != nil {
		report.Reason = serviceErr.Error()
	}
	want := report.Known && len(services) > 0
	for _, target := range GitHubPermissionsTargets() {
		report.Statuses = append(report.Statuses, checkGitHubPermissionsTarget(projectDir, target, report.Known, want, report.Reason))
	}
	if want {
		report.Conflicts = gitHubPermissionConflicts(projectDir, services)
	}
	return report
}

func checkGitHubPermissionsTarget(projectDir string, target GitHubPermissionsTarget, known bool, want bool, reason string) GitHubPermissionsStatus {
	status := GitHubPermissionsStatus{GitHubPermissionsTarget: target}
	if !known {
		status.State = GitHubPermissionsUnknown
		status.Detail = reason
		return status
	}
	path := filepath.FromSlash(target.Path)
	if mcpTargetViaSymlink(projectDir, path) {
		status.State = GitHubPermissionsSymlink
		status.Detail = "der Weg zur Datei führt über einen Link; k-playbook schreibt nicht hinein"
		return status
	}
	file, err := readPermissionFile(filepath.Join(projectDir, path), target.Client, true)
	if err != nil {
		status.State = GitHubPermissionsUnreadable
		status.Detail = err.Error()
		return status
	}
	own := file.ownState(target.Client)

	if want {
		switch {
		case own.complete:
			status.State = GitHubPermissionsOK
			status.Unexcluded = file.exists && gitExcludeNeeded(projectDir, target.Path)
		case mcpTargetTracked(projectDir, path):
			// Auch eine im Arbeitsbaum gelöschte Datei: ls-files sieht sie
			// im Index, und neu angelegt erschiene sie als Änderung einer
			// erfassten Datei.
			status.State = GitHubPermissionsTracked
			status.Detail = "von git erfasst; k-playbook schreibt die Berechtigungen nur in eine nicht erfasste Datei"
		default:
			status.State = GitHubPermissionsMissing
			if !file.exists {
				status.Detail = "die Datei fehlt"
			} else {
				status.Detail = "eigene Einträge fehlen oder stehen nicht am Ende"
			}
		}
		return status
	}

	switch {
	case !own.signature:
		status.State = GitHubPermissionsNone
	case mcpTargetTracked(projectDir, path):
		status.State = GitHubPermissionsTracked
		status.Detail = "trägt Einträge von k-playbook ohne Dienst, ist aber von git erfasst; k-playbook entfernt sie nicht"
	default:
		status.State = GitHubPermissionsStale
		status.Detail = "Einträge von k-playbook, aber dem Projekt ist kein Dienst zugeordnet"
	}
	return status
}

// permissionEntry ist ein Eintrag einer gelesenen Datei: bei Claude Code die
// Liste (allow, ask, deny) und der rohe Text Bash(…), bei OpenCode das Muster
// mit seiner Wirkung, in Dateireihenfolge.
type permissionEntry struct {
	Effect string
	Raw    string
	// Own: Eintrag aus der Zieldatei, der genau einer eigenen Regel gleicht.
	Own bool
}

// permissionFile ist eine gelesene Datei mit Berechtigungen.
type permissionFile struct {
	path    string
	exists  bool
	raw     []byte
	entries []permissionEntry
	// claudeLists nennt bei Claude Code die vorhandenen Listen samt ihren
	// Einträgen in Dateireihenfolge; Nicht-Zeichenketten stehen als "".
	claudeLists map[string][]string
	// permissionPresent: das Objekt permissions (Claude Code) bzw.
	// permission (OpenCode) steht da; bashPresent: bei OpenCode
	// permission.bash.
	permissionPresent bool
	bashPresent       bool
}

// claudeListNames sind die drei Listen von Claude Code in der Reihenfolge, in
// der sie geschrieben werden.
var claudeListNames = []string{permDeny, permAllow, permAsk}

// readPermissionFile liest eine Datei. target sagt, ob es die Zieldatei von
// k-playbook ist: Nur dort zählen Einträge als eigene, und nur dort ist eine
// Form, in die sich nichts eintragen lässt, ein Fehler — in einer Datei des
// Projekts zählt sie als Regel für alles (OpenCode: permission oder
// permission.bash als Zeichenkette).
func readPermissionFile(path string, client GitHubPermissionsClient, target bool) (permissionFile, error) {
	file := permissionFile{path: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return file, nil
		}
		return file, fmt.Errorf("nicht lesbar: %w", err)
	}
	file.exists, file.raw = true, raw

	value, err := hujson.Parse(raw)
	if err != nil {
		return file, fmt.Errorf("kein gültiges JSON: %w", err)
	}
	standard := value.Clone()
	standard.Standardize()
	if _, ok := standard.Value.(*hujson.Object); !ok {
		return file, errors.New("kein JSON-Objekt")
	}

	own := map[string]bool{}
	if target {
		for _, rule := range gitHubPermissionRules(client) {
			own[rule.Effect+"\x00"+permissionRaw(client, rule.Pattern)] = true
		}
	}

	if client == GitHubPermissionsClaude {
		return file, readClaudePermissions(&file, &standard, own)
	}
	return file, readOpenCodePermissions(&file, &standard, own, target)
}

func readClaudePermissions(file *permissionFile, standard *hujson.Value, own map[string]bool) error {
	file.claudeLists = map[string][]string{}
	permissions := standard.Find("/permissions")
	if permissions == nil {
		return nil
	}
	if _, ok := permissions.Value.(*hujson.Object); !ok {
		return errors.New("permissions ist kein Objekt")
	}
	file.permissionPresent = true
	for _, list := range claudeListNames {
		found := permissions.Find("/" + list)
		if found == nil {
			continue
		}
		array, ok := found.Value.(*hujson.Array)
		if !ok {
			return fmt.Errorf("permissions.%s ist keine Liste", list)
		}
		values := make([]string, 0, len(array.Elements))
		for _, element := range array.Elements {
			text := ""
			if literal, ok := element.Value.(hujson.Literal); ok && literal.Kind() == '"' {
				text = literal.String()
			}
			values = append(values, text)
			if text != "" {
				file.entries = append(file.entries, permissionEntry{Effect: list, Raw: text, Own: own[list+"\x00"+text]})
			}
		}
		file.claudeLists[list] = values
	}
	return nil
}

func readOpenCodePermissions(file *permissionFile, standard *hujson.Value, own map[string]bool, target bool) error {
	permission := standard.Find("/permission")
	if permission == nil {
		return nil
	}
	file.permissionPresent = true
	if literal, ok := permission.Value.(hujson.Literal); ok && literal.Kind() == '"' {
		if target {
			return errors.New("permission ist eine Zeichenkette, kein Objekt")
		}
		file.entries = append(file.entries, permissionEntry{Effect: literal.String(), Raw: "*"})
		return nil
	}
	if _, ok := permission.Value.(*hujson.Object); !ok {
		return errors.New("permission ist kein Objekt")
	}
	bash := permission.Find("/bash")
	if bash == nil {
		return nil
	}
	file.bashPresent = true
	if literal, ok := bash.Value.(hujson.Literal); ok && literal.Kind() == '"' {
		if target {
			return errors.New("permission.bash ist eine Zeichenkette, kein Objekt")
		}
		file.entries = append(file.entries, permissionEntry{Effect: literal.String(), Raw: "*"})
		return nil
	}
	object, ok := bash.Value.(*hujson.Object)
	if !ok {
		return errors.New("permission.bash ist kein Objekt")
	}
	for _, member := range object.Members {
		name, ok := member.Name.Value.(hujson.Literal)
		if !ok {
			continue
		}
		effect := ""
		if literal, ok := member.Value.Value.(hujson.Literal); ok && literal.Kind() == '"' {
			effect = literal.String()
		}
		pattern := name.String()
		file.entries = append(file.entries, permissionEntry{Effect: effect, Raw: pattern, Own: own[effect+"\x00"+pattern]})
	}
	return nil
}

// permissionRaw ist der Text eines Musters in der Datei des Clients.
func permissionRaw(client GitHubPermissionsClient, pattern string) string {
	if client == GitHubPermissionsClaude {
		return "Bash(" + pattern + ")"
	}
	return pattern
}

// ownPermissionState fasst zusammen, was von den eigenen Regeln in der
// Zieldatei steht.
type ownPermissionState struct {
	// complete: alle eigenen Regeln stehen; bei OpenCode zusätzlich als
	// letzte Einträge von permission.bash, in Reihenfolge.
	complete bool
	// signature: mindestens ein eigener Eintrag mit `k-playbook github run`
	// steht da — erst dann gelten die eigenen Einträge als von k-playbook
	// geschrieben und werden beim Wegfall des Dienstes entfernt.
	signature bool
}

func (file permissionFile) ownState(client GitHubPermissionsClient) ownPermissionState {
	state := ownPermissionState{}
	for _, entry := range file.entries {
		if entry.Own && strings.Contains(entry.Raw, gitHubRunPrefix) {
			state.signature = true
		}
	}

	rules := gitHubPermissionRules(client)
	if client == GitHubPermissionsClaude {
		present := map[string]bool{}
		for _, entry := range file.entries {
			if entry.Own {
				present[entry.Effect+"\x00"+entry.Raw] = true
			}
		}
		state.complete = true
		for _, rule := range rules {
			if !present[rule.Effect+"\x00"+permissionRaw(client, rule.Pattern)] {
				state.complete = false
			}
		}
		return state
	}

	if len(file.entries) < len(rules) {
		return state
	}
	tail := file.entries[len(file.entries)-len(rules):]
	state.complete = true
	for i, rule := range rules {
		if tail[i].Raw != rule.Pattern || tail[i].Effect != rule.Effect {
			state.complete = false
		}
	}
	return state
}

// writeGitHubPermissions trägt die eigenen Regeln ein. Wie bei MCP wird nur
// gepatcht: fremde Schlüssel und Einträge bleiben, Kommentare auch; der
// abschließende Format() rückt die Datei einheitlich ein. Bei OpenCode werden
// vorhandene eigene Einträge zuerst entfernt und dann alle am Ende des Objekts
// permission.bash angefügt — dort gewinnen sie gegen jeden Eintrag davor.
func writeGitHubPermissions(projectDir string, target GitHubPermissionsTarget) error {
	path := filepath.Join(projectDir, filepath.FromSlash(target.Path))
	file, err := readPermissionFile(path, target.Client, true)
	if err != nil {
		return fmt.Errorf("%s: %w", target.Path, err)
	}
	raw := file.raw
	if !file.exists {
		// Mit Zeilenumbruch, damit Format() die Objekte aufklappt.
		raw = []byte("{\n}\n")
	}

	var ops []jsonPatchOp
	if target.Client == GitHubPermissionsClaude {
		ops, err = claudeWriteOps(file)
	} else {
		ops, err = openCodeWriteOps(file)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", target.Path, err)
	}
	return patchPermissionFile(path, target.Path, raw, ops)
}

// removeGitHubPermissions entfernt die eigenen Einträge und lässt alles andere
// stehen, auch die dann leeren Listen oder Objekte.
func removeGitHubPermissions(projectDir string, target GitHubPermissionsTarget) error {
	path := filepath.Join(projectDir, filepath.FromSlash(target.Path))
	file, err := readPermissionFile(path, target.Client, true)
	if err != nil {
		return fmt.Errorf("%s: %w", target.Path, err)
	}
	if !file.exists {
		return nil
	}

	var ops []jsonPatchOp
	if target.Client == GitHubPermissionsClaude {
		ops = claudeRemoveOps(file)
	} else {
		ops = openCodeRemoveOps(file, false)
	}
	if len(ops) == 0 {
		return nil
	}
	return patchPermissionFile(path, target.Path, file.raw, ops)
}

func claudeWriteOps(file permissionFile) ([]jsonPatchOp, error) {
	var ops []jsonPatchOp
	if !file.permissionPresent {
		ops = append(ops, jsonPatchOp{Op: "add", Path: "/permissions", Value: json.RawMessage("{\n}")})
	}
	present := map[string]bool{}
	for _, entry := range file.entries {
		present[entry.Effect+"\x00"+entry.Raw] = true
	}
	for _, list := range claudeListNames {
		var missing []string
		for _, rule := range gitHubPermissionRules(GitHubPermissionsClaude) {
			raw := permissionRaw(GitHubPermissionsClaude, rule.Pattern)
			if rule.Effect == list && !present[list+"\x00"+raw] {
				missing = append(missing, raw)
			}
		}
		if len(missing) == 0 {
			continue
		}
		if _, ok := file.claudeLists[list]; !ok {
			ops = append(ops, jsonPatchOp{Op: "add", Path: "/permissions/" + list, Value: json.RawMessage("[\n]")})
		}
		for _, raw := range missing {
			value, err := json.Marshal(raw)
			if err != nil {
				return nil, err
			}
			ops = append(ops, jsonPatchOp{Op: "add", Path: "/permissions/" + list + "/-", Value: value})
		}
	}
	return ops, nil
}

func claudeRemoveOps(file permissionFile) []jsonPatchOp {
	own := map[string]bool{}
	for _, rule := range gitHubPermissionRules(GitHubPermissionsClaude) {
		own[rule.Effect+"\x00"+permissionRaw(GitHubPermissionsClaude, rule.Pattern)] = true
	}
	var ops []jsonPatchOp
	for _, list := range claudeListNames {
		values := file.claudeLists[list]
		// Von hinten, damit die Indizes der übrigen stimmen.
		for i := len(values) - 1; i >= 0; i-- {
			if values[i] != "" && own[list+"\x00"+values[i]] {
				ops = append(ops, jsonPatchOp{Op: "remove", Path: fmt.Sprintf("/permissions/%s/%d", list, i)})
			}
		}
	}
	return ops
}

func openCodeWriteOps(file permissionFile) ([]jsonPatchOp, error) {
	var ops []jsonPatchOp
	if !file.exists {
		schema, _ := json.Marshal(opencodeSchemaURL)
		ops = append(ops, jsonPatchOp{Op: "add", Path: "/" + jsonPointerEscape(opencodeSchemaKey), Value: schema})
	}
	if !file.permissionPresent {
		ops = append(ops, jsonPatchOp{Op: "add", Path: "/permission", Value: json.RawMessage("{\n}")})
	}
	if !file.bashPresent {
		ops = append(ops, jsonPatchOp{Op: "add", Path: "/permission/bash", Value: json.RawMessage("{\n}")})
	}
	ops = append(ops, openCodeRemoveOps(file, true)...)
	for _, rule := range gitHubPermissionRules(GitHubPermissionsOpenCode) {
		value, err := json.Marshal(rule.Effect)
		if err != nil {
			return nil, err
		}
		ops = append(ops, jsonPatchOp{Op: "add", Path: "/permission/bash/" + jsonPointerEscape(rule.Pattern), Value: value})
	}
	return ops, nil
}

// openCodeRemoveOps entfernt jeden eigenen Eintrag, je Vorkommen einmal. Mit
// sameName auch Einträge, die ein eigenes Muster mit anderer Wirkung tragen:
// Beim Schreiben ersetzte das Anfügen sie sonst an ihrem Platz (ein Objekt
// hat jeden Schlüssel einmal), und die eigenen Einträge stünden nicht
// geschlossen am Ende. Beim Entfernen ohne Dienst bleiben sie stehen — sie
// gehören dem Projekt.
func openCodeRemoveOps(file permissionFile, sameName bool) []jsonPatchOp {
	ownPatterns := map[string]bool{}
	for _, rule := range gitHubPermissionRules(GitHubPermissionsOpenCode) {
		ownPatterns[rule.Pattern] = true
	}
	var ops []jsonPatchOp
	for _, entry := range file.entries {
		if entry.Own || (sameName && ownPatterns[entry.Raw]) {
			ops = append(ops, jsonPatchOp{Op: "remove", Path: "/permission/bash/" + jsonPointerEscape(entry.Raw)})
		}
	}
	return ops
}

// jsonPointerEscape maskiert einen Schlüssel für einen JSON Pointer (RFC 6901).
func jsonPointerEscape(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

// patchPermissionFile wendet die Patch-Folge auf den Rohtext an und schreibt
// die Datei, das Verzeichnis bei Bedarf mit.
func patchPermissionFile(path string, display string, raw []byte, ops []jsonPatchOp) error {
	if len(ops) == 0 {
		return nil
	}
	value, err := hujson.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", display, err)
	}
	patch, err := json.Marshal(ops)
	if err != nil {
		return fmt.Errorf("%s: %w", display, err)
	}
	if err := value.Patch(patch); err != nil {
		return fmt.Errorf("%s: %w", display, err)
	}
	value.Format()
	packed := value.Pack()
	if len(packed) == 0 || packed[len(packed)-1] != '\n' {
		packed = append(packed, '\n')
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", filepath.Dir(display), err)
	}
	if err := os.WriteFile(path, packed, 0o644); err != nil {
		return fmt.Errorf("%s schreiben: %w", display, err)
	}
	return nil
}
