package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tailscale/hujson"
)

// stubGitHubPermissionServices ersetzt die Bestimmung der Dienste: services
// sind die Dienste des Projekts, err macht die Frage unbeantwortet.
func stubGitHubPermissionServices(t *testing.T, services []string, err error) {
	t.Helper()
	previous := gitHubPermissionServices
	gitHubPermissionServices = func(string) ([]string, error) { return services, err }
	t.Cleanup(func() { gitHubPermissionServices = previous })
}

// permissionsRepo baut ein Projekt in einem echten Repository. Die globale
// Ausschlussdatei des Rechners bleibt außen vor (XDG_CONFIG_HOME,
// GIT_CONFIG_GLOBAL): auf dem Entwicklungsrechner nimmt sie
// .claude/settings.local.json schon aus.
func permissionsRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	root := t.TempDir()
	gitInit(t, root)
	writeVCSConfig(t, root, "git")
	gitRun(t, root, "config", "user.email", "test@example.invalid")
	gitRun(t, root, "config", "user.name", "Test")
	return root
}

func claudeLocal(root string) string   { return filepath.Join(root, ".claude", "settings.local.json") }
func openCodeLocal(root string) string { return filepath.Join(root, ".opencode", "opencode.jsonc") }

// openCodeBashKeys liest die Schlüssel von permission.bash in Dateireihenfolge.
func openCodeBashKeys(t *testing.T, path string) []string {
	t.Helper()
	file, err := readPermissionFile(path, GitHubPermissionsOpenCode, false)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	keys := []string{}
	for _, entry := range file.entries {
		keys = append(keys, entry.Raw+"="+entry.Effect)
	}
	return keys
}

func claudeList(t *testing.T, path string, list string) []string {
	t.Helper()
	permissions, _ := readJSONC(t, path)["permissions"].(map[string]any)
	values := []string{}
	raw, _ := permissions[list].([]any)
	for _, value := range raw {
		values = append(values, value.(string))
	}
	return values
}

func statusOf(t *testing.T, report GitHubPermissionsReport, client GitHubPermissionsClient) GitHubPermissionsStatus {
	t.Helper()
	for _, status := range report.Statuses {
		if status.Client == client {
			return status
		}
	}
	t.Fatalf("kein Zustand für %s", client)
	return GitHubPermissionsStatus{}
}

// Die Muster je Schema, wie sie geschrieben werden. Wie die Clients sie auf
// Zeilen anwenden, belegen die Messung in Task 190 und die Abnahme.
func TestGitHubPermissionRulesJeSchema(t *testing.T) {
	deny := []string{
		"gh *",
		"git clone*", "git -C * clone*",
		"git fetch*", "git -C * fetch*",
		"git pull*", "git -C * pull*",
		"git push*", "git -C * push*",
		"git ls-remote*", "git -C * ls-remote*",
		"git remote update*", "git -C * remote update*",
	}
	allow := []string{
		"k-playbook github run * -- gh *",
		"k-playbook github run * -- git fetch*",
		"k-playbook github run * -- git pull*",
		"k-playbook github run * -- git clone*",
		"k-playbook github run * -- git ls-remote*",
		"k-playbook github run * -- git remote update*",
	}
	ask := []string{
		"k-playbook github run * --write *",
		"k-playbook github run --write *",
		"k-playbook github run * -- gh pr merge*",
		"k-playbook github run * -- gh pr create*",
		"k-playbook github run * -- gh pr comment*",
		"k-playbook github run * -- gh pr review*",
		"k-playbook github run * -- gh pr close*",
		"k-playbook github run * -- gh release*",
		"k-playbook github run * -- gh repo*",
		"k-playbook github run * -- gh api*-X*",
		"k-playbook github run * -- gh api*--method*",
	}

	byEffect := func(client GitHubPermissionsClient) map[string][]string {
		result := map[string][]string{}
		for _, rule := range gitHubPermissionRules(client) {
			result[rule.Effect] = append(result[rule.Effect], rule.Pattern)
		}
		return result
	}

	claude := byEffect(GitHubPermissionsClaude)
	if !reflect.DeepEqual(claude[permDeny], deny) || !reflect.DeepEqual(claude[permAllow], allow) || !reflect.DeepEqual(claude[permAsk], ask) {
		t.Errorf("Claude-Muster weichen ab:\n%v", claude)
	}

	// OpenCode: dazu das breite ask vor allen allow, weil ohne Regel erlaubt
	// ist; die zuletzt passende Regel gewinnt.
	opencode := gitHubPermissionRules(GitHubPermissionsOpenCode)
	wantOrder := []string{}
	for _, pattern := range deny {
		wantOrder = append(wantOrder, pattern+"=deny")
	}
	wantOrder = append(wantOrder, "k-playbook github run *=ask")
	for _, pattern := range allow {
		wantOrder = append(wantOrder, pattern+"=allow")
	}
	for _, pattern := range ask {
		wantOrder = append(wantOrder, pattern+"=ask")
	}
	got := []string{}
	for _, rule := range opencode {
		got = append(got, rule.Pattern+"="+rule.Effect)
	}
	if !reflect.DeepEqual(got, wantOrder) {
		t.Errorf("OpenCode-Reihenfolge weicht ab:\n%s", strings.Join(got, "\n"))
	}
}

// Die eigenen Regeln, ausgewertet nach der gemessenen Logik des Clients,
// geben jeder Beispielzeile die gewollte Wirkung.
func TestGitHubPermissionRulesTreffenBeispielzeilen(t *testing.T) {
	probes := gitHubPermissionProbes([]string{"k-playbook-dev"})
	for _, client := range []GitHubPermissionsClient{GitHubPermissionsClaude, GitHubPermissionsOpenCode} {
		var sources []permissionSource
		for _, rule := range gitHubPermissionRules(client) {
			sources = append(sources, permissionSource{
				permissionEntry: permissionEntry{Effect: rule.Effect, Raw: permissionRaw(client, rule.Pattern)},
				Pattern:         rule.Pattern,
			})
		}
		for _, probe := range probes {
			got, _ := evaluatePermissions(client, sources, probe.Line)
			if got == "" {
				got = clientDefault(client)
			}
			if got != probe.Want {
				t.Errorf("%s: %q → %s, gewollt %s", client, probe.Line, got, probe.Want)
			}
		}
	}
}

// Die gemessenen Lücken stehen fest: * trifft Text in Anführungszeichen, und
// in OpenCode läuft env gh ohne eigene Regel durch.
func TestGitHubPermissionRulesBekannteLuecken(t *testing.T) {
	var claude, opencode []permissionSource
	for _, rule := range gitHubPermissionRules(GitHubPermissionsClaude) {
		claude = append(claude, permissionSource{permissionEntry: permissionEntry{Effect: rule.Effect}, Pattern: rule.Pattern})
	}
	for _, rule := range gitHubPermissionRules(GitHubPermissionsOpenCode) {
		opencode = append(opencode, permissionSource{permissionEntry: permissionEntry{Effect: rule.Effect}, Pattern: rule.Pattern})
	}
	line := `k-playbook github run x -- git commit -m "a -- git fetch"`
	if got, _ := evaluatePermissions(GitHubPermissionsClaude, claude, line); got != permAllow {
		t.Errorf("Claude: %q → %q, gemessen allow", line, got)
	}
	if got, _ := evaluatePermissions(GitHubPermissionsOpenCode, opencode, line); got != permAllow {
		t.Errorf("OpenCode: %q → %q, gemessen allow", line, got)
	}
	if got, _ := evaluatePermissions(GitHubPermissionsOpenCode, opencode, "env gh pr list"); got != "" {
		t.Errorf("OpenCode: env gh → %q, gemessen ohne Regel", got)
	}
}

func TestWildcardMatch(t *testing.T) {
	cases := []struct {
		pattern, text string
		want          bool
	}{
		{"gh *", "gh pr list", true},
		{"gh *", "ghx", false},
		{"git -C * fetch*", "git -C sub fetch origin", true},
		{"k-playbook github run * -- gh *", "k-playbook github run svc --write -- gh pr list", true},
		{"k-playbook github run * -- gh *", "k-playbook github run svc -- git fetch", false},
		{"a?c", "abc", true},
		{"*", "", true},
	}
	for _, c := range cases {
		if got := wildcardMatch(c.pattern, c.text); got != c.want {
			t.Errorf("wildcardMatch(%q, %q) = %v", c.pattern, c.text, got)
		}
	}
	// " *" am Ende trifft auch die Zeile ohne Argumente.
	if !(permissionSource{Pattern: "gh *"}).matches("gh") {
		t.Error("gh * trifft gh nicht")
	}
	// Die ältere Form von Claude Code: Präfix als ganzes Wort.
	source := permissionSource{Pattern: "gh pr", prefix: true}
	if !source.matches("gh pr list") || source.matches("gh prx") {
		t.Error("Präfixform falsch ausgewertet")
	}
}

// Beide Dateien entstehen neu, mit den eigenen Einträgen, aus git
// ausgenommen; ein zweiter Lauf schreibt nichts.
func TestApplyGitHubPermissionsLegtBeideDateienAn(t *testing.T) {
	root := permissionsRepo(t)
	stubGitHubPermissionServices(t, []string{"k-playbook-dev"}, nil)

	report := ApplyGitHubPermissions(root)
	if report.Error != "" {
		t.Fatalf("Fehler: %s", report.Error)
	}
	if !reflect.DeepEqual(report.Written, []string{".claude/settings.local.json", ".opencode/opencode.jsonc"}) {
		t.Errorf("geschrieben = %v", report.Written)
	}
	if !reflect.DeepEqual(report.Excluded, []string{".claude/settings.local.json", ".opencode/opencode.jsonc"}) {
		t.Errorf("ausgenommen = %v", report.Excluded)
	}
	for _, status := range report.Statuses {
		if status.State != GitHubPermissionsOK || status.Unexcluded {
			t.Errorf("%s: %s (unexcluded %v), erwartet ok", status.Path, status.State, status.Unexcluded)
		}
	}

	if deny := claudeList(t, claudeLocal(root), "deny"); !slices.Contains(deny, "Bash(gh *)") || !slices.Contains(deny, "Bash(git -C * push*)") {
		t.Errorf("deny = %v", deny)
	}
	if allow := claudeList(t, claudeLocal(root), "allow"); !slices.Contains(allow, "Bash(k-playbook github run * -- gh *)") {
		t.Errorf("allow = %v", allow)
	}
	if ask := claudeList(t, claudeLocal(root), "ask"); slices.Contains(ask, "Bash(k-playbook github run *)") {
		t.Errorf("das breite ask gehört nicht zu Claude Code: %v", ask)
	}

	opencode := readJSONC(t, openCodeLocal(root))
	if opencode[opencodeSchemaKey] != opencodeSchemaURL {
		t.Errorf("$schema fehlt: %v", opencode)
	}
	keys := openCodeBashKeys(t, openCodeLocal(root))
	if len(keys) != len(gitHubPermissionRules(GitHubPermissionsOpenCode)) || keys[0] != "gh *=deny" || keys[len(keys)-1] != "k-playbook github run * -- gh api*--method*=ask" {
		t.Errorf("permission.bash = %v", keys)
	}

	exclude := readRaw(t, filepath.Join(root, ".git", "info", "exclude"))
	for _, line := range []string{"/.claude/settings.local.json\n", "/.opencode/opencode.jsonc\n"} {
		if !strings.Contains(exclude, line) {
			t.Errorf(".git/info/exclude ohne %q:\n%s", line, exclude)
		}
	}

	claudeBefore, openCodeBefore := readRaw(t, claudeLocal(root)), readRaw(t, openCodeLocal(root))
	again := ApplyGitHubPermissions(root)
	if len(again.Written)+len(again.Removed)+len(again.Excluded) != 0 {
		t.Errorf("zweiter Lauf schrieb: %+v", again)
	}
	if readRaw(t, claudeLocal(root)) != claudeBefore || readRaw(t, openCodeLocal(root)) != openCodeBefore {
		t.Error("zweiter Lauf veränderte eine Datei")
	}
	if len(again.Lines()) != 0 {
		t.Errorf("Meldungen ohne Anlass: %v", again.Lines())
	}
}

// Fremde Einträge, Schlüssel und Kommentare bleiben; bei OpenCode stehen die
// eigenen Einträge danach geschlossen am Ende.
func TestApplyGitHubPermissionsErhaeltFremdes(t *testing.T) {
	root := permissionsRepo(t)
	stubGitHubPermissionServices(t, []string{"k-playbook-dev"}, nil)
	writeFile(t, claudeLocal(root), `{"permissions":{"allow":["Bash(npm test)"]},"env":{"A":"1"}}`+"\n")
	writeFile(t, openCodeLocal(root), `{
  // eigene Notiz
  "model": "x",
  "permission": {
    "bash": {
      "gh *": "deny",
      "git status*": "allow",
      "k-playbook github run *": "allow",
    },
  },
}
`)

	report := ApplyGitHubPermissions(root)
	if report.Error != "" {
		t.Fatalf("Fehler: %s", report.Error)
	}
	if allow := claudeList(t, claudeLocal(root), "allow"); allow[0] != "Bash(npm test)" {
		t.Errorf("fremder allow-Eintrag verloren: %v", allow)
	}
	if readJSONC(t, claudeLocal(root))["env"] == nil {
		t.Error("fremder Schlüssel env verloren")
	}

	raw := readRaw(t, openCodeLocal(root))
	if !strings.Contains(raw, "// eigene Notiz") || readJSONC(t, openCodeLocal(root))["model"] != "x" {
		t.Errorf("Kommentar oder fremder Schlüssel verloren:\n%s", raw)
	}
	keys := openCodeBashKeys(t, openCodeLocal(root))
	if keys[0] != "git status*=allow" {
		t.Errorf("fremder Eintrag nicht vorn: %v", keys)
	}
	if statusOf(t, report, GitHubPermissionsOpenCode).State != GitHubPermissionsOK {
		t.Errorf("OpenCode nach dem Lauf nicht ok: %v", keys)
	}
	if slices.Contains(keys, "k-playbook github run *=allow") {
		t.Errorf("das gleichnamige pauschale allow blieb stehen: %v", keys)
	}
}

// Ohne Dienst schreibt k-playbook nichts.
func TestApplyGitHubPermissionsOhneDienst(t *testing.T) {
	root := permissionsRepo(t)
	stubGitHubPermissionServices(t, nil, nil)

	report := ApplyGitHubPermissions(root)
	if len(report.Written)+len(report.Excluded) != 0 {
		t.Errorf("geschrieben ohne Dienst: %+v", report)
	}
	for _, path := range []string{claudeLocal(root), openCodeLocal(root)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s angelegt", path)
		}
	}
	for _, status := range report.Statuses {
		if status.State != GitHubPermissionsNone {
			t.Errorf("%s: %s", status.Path, status.State)
		}
	}
}

// Fallen Dienst oder Zugangsdatei weg, entfernt k-playbook seine eigenen
// Einträge — und nur die. Ist die Frage unbeantwortet, bleibt alles stehen.
func TestApplyGitHubPermissionsEntferntBeimWegfall(t *testing.T) {
	root := permissionsRepo(t)
	stubGitHubPermissionServices(t, []string{"k-playbook-dev"}, nil)
	writeFile(t, claudeLocal(root), `{"permissions":{"allow":["Bash(npm test)"]}}`+"\n")
	writeFile(t, openCodeLocal(root), `{"permission":{"bash":{"git status*":"allow"}}}`+"\n")
	ApplyGitHubPermissions(root)

	// Zugangsdatei fehlerhaft: unbeantwortet, nichts entfernt.
	stubGitHubPermissionServices(t, nil, errors.New("die GitHub-Zugangsdatei hat Fehler"))
	unknown := ApplyGitHubPermissions(root)
	if len(unknown.Removed) != 0 || statusOf(t, unknown, GitHubPermissionsClaude).State != GitHubPermissionsUnknown {
		t.Errorf("unbeantwortet, trotzdem entfernt: %+v", unknown)
	}

	// Dienst bzw. Zugangsdatei weg.
	stubGitHubPermissionServices(t, nil, nil)
	check := CheckGitHubPermissions(root)
	if statusOf(t, check, GitHubPermissionsClaude).State != GitHubPermissionsStale {
		t.Errorf("Prüfung meldet %s, erwartet stale", statusOf(t, check, GitHubPermissionsClaude).State)
	}
	report := ApplyGitHubPermissions(root)
	if !reflect.DeepEqual(report.Removed, []string{".claude/settings.local.json", ".opencode/opencode.jsonc"}) {
		t.Errorf("entfernt = %v", report.Removed)
	}
	if allow := claudeList(t, claudeLocal(root), "allow"); !reflect.DeepEqual(allow, []string{"Bash(npm test)"}) {
		t.Errorf("allow danach = %v", allow)
	}
	if deny := claudeList(t, claudeLocal(root), "deny"); len(deny) != 0 {
		t.Errorf("deny danach = %v", deny)
	}
	if keys := openCodeBashKeys(t, openCodeLocal(root)); !reflect.DeepEqual(keys, []string{"git status*=allow"}) {
		t.Errorf("permission.bash danach = %v", keys)
	}
	for _, status := range report.Statuses {
		if status.State != GitHubPermissionsNone {
			t.Errorf("%s: %s", status.Path, status.State)
		}
	}
}

// Ein einzelnes gh * des Nutzers ohne Einträge über run gilt nicht als von
// k-playbook geschrieben und bleibt ohne Dienst stehen.
func TestApplyGitHubPermissionsLaesstEigenesDenyDesNutzers(t *testing.T) {
	root := permissionsRepo(t)
	stubGitHubPermissionServices(t, nil, nil)
	content := `{"permissions":{"deny":["Bash(gh *)"]}}` + "\n"
	writeFile(t, claudeLocal(root), content)

	report := ApplyGitHubPermissions(root)
	if len(report.Removed) != 0 || readRaw(t, claudeLocal(root)) != content {
		t.Errorf("Eintrag des Nutzers entfernt: %+v", report)
	}
}

// Eine von git erfasste Zieldatei bleibt unberührt und ist keine Warnung wert.
func TestApplyGitHubPermissionsLaesstErfassteDatei(t *testing.T) {
	root := permissionsRepo(t)
	stubGitHubPermissionServices(t, []string{"k-playbook-dev"}, nil)
	content := `{"permissions":{}}` + "\n"
	writeFile(t, claudeLocal(root), content)
	gitRun(t, root, "add", "-f", ".claude/settings.local.json")
	gitRun(t, root, "commit", "-qm", "erfasst")

	report := ApplyGitHubPermissions(root)
	if statusOf(t, report, GitHubPermissionsClaude).State != GitHubPermissionsTracked {
		t.Errorf("Zustand %s, erwartet tracked", statusOf(t, report, GitHubPermissionsClaude).State)
	}
	if readRaw(t, claudeLocal(root)) != content {
		t.Error("erfasste Datei verändert")
	}
	if slices.Contains(report.Written, ".claude/settings.local.json") {
		t.Errorf("geschrieben = %v", report.Written)
	}
}

// Eine von git erfasste Zieldatei bleibt unberührt, auch wenn sie im
// Arbeitsbaum fehlt (Task 196, V2): git fragt die Prüfung unabhängig davon, ob
// die Datei da ist.
func TestApplyGitHubPermissionsLaesstGeloeschteErfassteDatei(t *testing.T) {
	root := permissionsRepo(t)
	stubGitHubPermissionServices(t, []string{"k-playbook-dev"}, nil)
	writeFile(t, claudeLocal(root), `{"permissions":{}}`+"\n")
	gitRun(t, root, "add", "-f", ".claude/settings.local.json")
	gitRun(t, root, "commit", "-qm", "erfasst")
	if err := os.Remove(claudeLocal(root)); err != nil {
		t.Fatal(err)
	}

	report := ApplyGitHubPermissions(root)
	if state := statusOf(t, report, GitHubPermissionsClaude).State; state != GitHubPermissionsTracked {
		t.Errorf("Zustand %s, erwartet tracked", state)
	}
	if _, err := os.Stat(claudeLocal(root)); !os.IsNotExist(err) {
		t.Errorf("gelöschte erfasste Datei neu angelegt: %v", err)
	}
	if slices.Contains(report.Written, ".claude/settings.local.json") {
		t.Errorf("geschrieben = %v", report.Written)
	}
}

// Eine Form, in die sich nichts eintragen lässt, wird nicht angefasst.
func TestApplyGitHubPermissionsLaesstUnlesbareDatei(t *testing.T) {
	root := permissionsRepo(t)
	stubGitHubPermissionServices(t, []string{"k-playbook-dev"}, nil)
	content := `{"permissions":["x"]}` + "\n"
	writeFile(t, claudeLocal(root), content)
	openContent := `{"permission":"allow"}` + "\n"
	writeFile(t, openCodeLocal(root), openContent)

	report := ApplyGitHubPermissions(root)
	if statusOf(t, report, GitHubPermissionsClaude).State != GitHubPermissionsUnreadable ||
		statusOf(t, report, GitHubPermissionsOpenCode).State != GitHubPermissionsUnreadable {
		t.Errorf("Zustände %+v", report.Statuses)
	}
	if readRaw(t, claudeLocal(root)) != content || readRaw(t, openCodeLocal(root)) != openContent {
		t.Error("unlesbare Datei verändert")
	}
}

// Liegt das Projekt unterhalb der Wurzel des Repositorys, nennt der Eintrag in
// .git/info/exclude den Pfad ab der Wurzel.
func TestApplyGitHubPermissionsAusschlussImUnterverzeichnis(t *testing.T) {
	repo := permissionsRepo(t)
	root := filepath.Join(repo, "sub")
	writeVCSConfig(t, root, "git")
	stubGitHubPermissionServices(t, []string{"k-playbook-dev"}, nil)

	report := ApplyGitHubPermissions(root)
	if report.Error != "" {
		t.Fatalf("Fehler: %s", report.Error)
	}
	exclude := readRaw(t, filepath.Join(repo, ".git", "info", "exclude"))
	if !strings.Contains(exclude, "/sub/.claude/settings.local.json\n") {
		t.Errorf(".git/info/exclude:\n%s", exclude)
	}
	gitRun(t, root, "check-ignore", "-q", ".claude/settings.local.json")
}

// Widersprüche des Projekts werden gemeldet, mit Datei, Eintrag und
// Beispielzeile — auch die, die die eigenen Einträge überstimmen.
func TestGitHubPermissionConflicts(t *testing.T) {
	root := permissionsRepo(t)
	stubGitHubPermissionServices(t, []string{"k-playbook-dev"}, nil)
	writeFile(t, filepath.Join(root, ".claude", "settings.json"), `{"permissions":{"allow":["Bash(gh pr:*)","Bash(k-playbook github run*)"],"deny":["Bash(k-playbook github run * -- gh issue*)"]}}`+"\n")
	writeFile(t, filepath.Join(root, "opencode.json"), `{"permission":{"bash":{"gh pr*":"allow","git fetch*":"allow","k-playbook github run*":"allow"}}}`+"\n")

	report := ApplyGitHubPermissions(root)
	got := map[string]GitHubPermissionConflict{}
	for _, conflict := range report.Conflicts {
		got[conflict.Path+" "+conflict.Rule] = conflict
	}
	want := map[string]bool{
		".claude/settings.json Bash(gh pr:*)":                              false,
		".claude/settings.json Bash(k-playbook github run*)":               true,
		".claude/settings.json Bash(k-playbook github run * -- gh issue*)": true,
		"opencode.json gh pr*":                                             false,
		"opencode.json git fetch*":                                         false,
		"opencode.json k-playbook github run*":                             false,
	}
	for key, effective := range want {
		conflict, ok := got[key]
		if !ok {
			t.Errorf("Widerspruch %s fehlt; gemeldet: %v", key, report.Conflicts)
			continue
		}
		if conflict.Effective != effective {
			t.Errorf("%s: wirksam %v, erwartet %v (%s)", key, conflict.Effective, effective, conflict.Line)
		}
	}
	if len(got) != len(want) {
		t.Errorf("gemeldet %d Widersprüche, erwartet %d: %v", len(got), len(want), report.Conflicts)
	}
	lines := strings.Join(report.Lines(), "\n")
	if !strings.Contains(lines, "Vorschlag: den Eintrag entfernen") || !strings.Contains(lines, "„Bash(k-playbook github run*)“") {
		t.Errorf("Meldungen:\n%s", lines)
	}
}

// Das Muster aus Omni, von Hand gebaut, widerspricht nicht: Was es zu wenig
// hat, ergänzt k-playbook, und gleiche Einträge zählen nicht als Widerspruch.
func TestGitHubPermissionConflictsOmniMuster(t *testing.T) {
	root := permissionsRepo(t)
	stubGitHubPermissionServices(t, []string{"OMNIWrapper"}, nil)
	writeFile(t, filepath.Join(root, "opencode.json"), `{"permission":{"bash":{
		"git *": "deny", "git status*": "allow", "git remote*": "allow", "git remote update*": "deny",
		"git -C * status*": "allow",
		"gh *": "deny",
		"k-playbook github run *": "ask",
		"k-playbook github run * -- git fetch*": "allow",
		"k-playbook github run * -- git pull*": "allow",
		"k-playbook github run * -- git clone*": "allow",
		"k-playbook github run * -- git ls-remote*": "allow",
		"k-playbook github run * -- gh *": "allow",
		"k-playbook github run * --write *": "ask"}}}`+"\n")

	report := CheckGitHubPermissions(root)
	if len(report.Conflicts) != 0 {
		t.Errorf("Widersprüche im Omni-Muster: %v", report.Conflicts)
	}
}

// Die Zieldatei als Vorlage: hujson liest, was geschrieben wurde.
func TestGitHubPermissionsGeschriebenIstJWCC(t *testing.T) {
	root := permissionsRepo(t)
	stubGitHubPermissionServices(t, []string{"k-playbook-dev"}, nil)
	ApplyGitHubPermissions(root)
	for _, path := range []string{claudeLocal(root), openCodeLocal(root)} {
		if _, err := hujson.Parse([]byte(readRaw(t, path))); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}

// Die Umgebungsprüfung warnt über fehlende Einträge und Widersprüche, nicht
// über eine erfasste Datei und nicht ohne Dienst.
func TestHealthGitHubPermissions(t *testing.T) {
	root := permissionsRepo(t)
	stubGitHubPermissionServices(t, nil, nil)
	if problems := healthGitHubPermissions(context.Background(), root); len(problems) != 0 {
		t.Errorf("ohne Dienst: %v", problems)
	}

	stubGitHubPermissionServices(t, []string{"k-playbook-dev"}, nil)
	problems := healthGitHubPermissions(context.Background(), root)
	if len(problems) != 2 || !strings.Contains(problems[0].Message, "fehlen") {
		t.Errorf("fehlende Einträge: %v", problems)
	}

	ApplyGitHubPermissions(root)
	if problems := healthGitHubPermissions(context.Background(), root); len(problems) != 0 {
		t.Errorf("nach dem Schreiben: %v", problems)
	}

	writeFile(t, filepath.Join(root, "opencode.json"), `{"permission":{"bash":{"gh pr*":"allow"}}}`+"\n")
	problems = healthGitHubPermissions(context.Background(), root)
	if len(problems) != 1 || !strings.Contains(problems[0].Message, "opencode.json") || !strings.Contains(problems[0].Message, "überstimmen") {
		t.Errorf("Widerspruch: %v", problems)
	}

	// Ohne Dienst stehen die Einträge noch: Warnung.
	stubGitHubPermissionServices(t, nil, nil)
	problems = healthGitHubPermissions(context.Background(), root)
	if len(problems) != 2 || !strings.Contains(problems[0].Message, "kein Dienst") {
		t.Errorf("ohne Dienst mit Einträgen: %v", problems)
	}
}

// Eine erfasste Zieldatei ist ein bewusst ungeschriebener Stand.
func TestHealthGitHubPermissionsErfassteDatei(t *testing.T) {
	root := permissionsRepo(t)
	stubGitHubPermissionServices(t, []string{"k-playbook-dev"}, nil)
	for _, path := range []string{claudeLocal(root), openCodeLocal(root)} {
		writeFile(t, path, "{}\n")
	}
	gitRun(t, root, "add", "-f", ".claude/settings.local.json", ".opencode/opencode.jsonc")
	gitRun(t, root, "commit", "-qm", "erfasst")
	if problems := healthGitHubPermissions(context.Background(), root); len(problems) != 0 {
		t.Errorf("erfasste Dateien: %v", problems)
	}
}

// Die Berechtigungen bestimmen den Dienst auf demselben Weg wie die
// Oberfläche (Task 196, V3): aus dem Remote des Repos unter project.repo_root.
// Nachgestellt ist der Fall, in dem das Hauptverzeichnis in keinem Repo liegt
// und repo_root auf ein Unterverzeichnis mit eigenem Repo zeigt.
func TestGitHubPermissionServicesAusRepoRoot(t *testing.T) {
	githubFakes(t)
	githubConfig(t, githubTwoServices)
	projectDir := t.TempDir()
	repo := filepath.Join(projectDir, "sub")
	run(t, projectDir, "git", "init", "-q", "-b", "main", repo)
	run(t, repo, "git", "remote", "add", "origin", "https://github.com/kascada/k-playbook.git")
	writeFile(t, ConfigPath(projectDir), "schema_version: "+SchemaVersion+"\n\nproject:\n  repo_root: sub\n  vcs: git\n")

	services, err := resolveGitHubPermissionServices(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(services, []string{"k-playbook"}) {
		t.Errorf("Dienste %v, erwartet [k-playbook] aus dem Repo unter repo_root", services)
	}
}
