package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Widersprüche und Ausschluss aus git für die Berechtigungen (Task 190).
//
// Ein Widerspruch ist ein Eintrag des Projekts, der für eine Beispielzeile
// etwas anderes sagt als die Regel: nacktes gh oder ein Netzbefehl von git
// erlaubt, eine Zeile hinter run, die fragen soll, erlaubt (etwa
// `k-playbook github run*` pauschal), oder der Weg über run gesperrt bzw. mit
// Rückfrage. Ausgewertet wird mit der Logik des Clients, gemessen in Task 190
// (Etappe 1): Claude Code deny vor ask vor allow über alle Dateien, OpenCode
// die zuletzt passende Regel in Lesereihenfolge. Eine Rückfrage, wo gesperrt
// sein soll, und eine Sperre, wo gefragt werden soll, sind strenger und kein
// Widerspruch.
//
// Gezählt werden zuerst nur die Einträge des Projekts — ohne die von
// k-playbook —, so dass ein Widerspruch auch dann gemeldet wird, wenn die
// eigenen Einträge ihn überstimmen. Ob er trotzdem wirkt, sagt Effective,
// ausgewertet über alle Einträge, wie sie gerade stehen.

// gitHubPermissionProbe ist eine Beispielzeile mit der Wirkung, die sie
// haben soll.
type gitHubPermissionProbe struct {
	Line string
	Want string
}

// gitHubPermissionProbes baut die Beispielzeilen. Zeilen über run gibt es je
// Dienst, weil ein Eintrag des Projekts den Dienst nennen kann.
func gitHubPermissionProbes(services []string) []gitHubPermissionProbe {
	var probes []gitHubPermissionProbe
	add := func(want string, lines ...string) {
		for _, line := range lines {
			probes = append(probes, gitHubPermissionProbe{Line: line, Want: want})
		}
	}
	add(permDeny,
		"gh pr list", "gh api repos/o/r",
		"git fetch", "git fetch origin", "git pull", "git push", "git push origin main",
		"git clone https://github.com/o/r", "git ls-remote origin", "git remote update",
		"git -C sub fetch", "git -C sub push")
	for _, service := range services {
		run := gitHubRunPrefix + " " + service
		add(permAllow,
			run+" -- gh pr list", run+" -- gh issue view 1",
			run+" -- git fetch", run+" -- git fetch origin", run+" -- git pull",
			run+" -- git clone https://github.com/o/r", run+" -- git ls-remote origin", run+" -- git remote update")
		add(permAsk,
			run+" --write -- git push", run+" --write -- gh pr create --fill",
			gitHubRunPrefix+" --write "+service+" -- git push",
			run+" -- gh pr merge 1", run+" -- gh api -X POST repos/o/r/issues",
			run+" -- git push", run+" -- git status", run+" -- git reset --hard", run+" -- ls")
	}
	return probes
}

// permissionSource ist ein Eintrag samt Datei, aus der er stammt.
type permissionSource struct {
	permissionEntry
	Path string
	// Pattern ist das Muster ohne die Hülle Bash(…).
	Pattern string
	// matchAll: Claude Code Bash ohne Muster bzw. OpenCode permission als
	// Zeichenkette.
	matchAll bool
	// prefix: Claude Codes ältere Form Bash(<präfix>:*).
	prefix bool
}

// gitHubPermissionConflicts liest die Dateien eines Projekts und meldet die
// Widersprüche beider Clients. Unlesbare Dateien werden übergangen; die
// Zieldateien meldet die Prüfung ohnehin.
func gitHubPermissionConflicts(projectDir string, services []string) []GitHubPermissionConflict {
	conflicts := []GitHubPermissionConflict{}
	probes := gitHubPermissionProbes(services)
	for _, target := range GitHubPermissionsTargets() {
		var sources []permissionSource
		for _, path := range gitHubPermissionsProjectFiles(target.Client) {
			sources = append(sources, readPermissionSources(projectDir, path, target.Client, false)...)
		}
		sources = append(sources, readPermissionSources(projectDir, target.Path, target.Client, true)...)
		conflicts = append(conflicts, permissionConflicts(target, sources, probes)...)
	}
	return conflicts
}

func readPermissionSources(projectDir string, path string, client GitHubPermissionsClient, target bool) []permissionSource {
	file, err := readPermissionFile(filepath.Join(projectDir, filepath.FromSlash(path)), client, target)
	if err != nil {
		return nil
	}
	sources := make([]permissionSource, 0, len(file.entries))
	for _, entry := range file.entries {
		source := permissionSource{permissionEntry: entry, Path: path, Pattern: entry.Raw}
		if client == GitHubPermissionsClaude {
			pattern, ok := claudeBashPattern(entry.Raw)
			if !ok {
				continue
			}
			source.Pattern = pattern
			switch {
			case pattern == "" || pattern == "*":
				source.matchAll = true
			case strings.HasSuffix(pattern, ":*"):
				source.prefix = true
				source.Pattern = strings.TrimSuffix(pattern, ":*")
			}
		} else if entry.Raw == "*" {
			source.matchAll = true
		}
		sources = append(sources, source)
	}
	return sources
}

// claudeBashPattern liest das Muster aus einem Eintrag von Claude Code: Bash
// allein trifft jede Zeile (leeres Muster), Bash(<muster>) das Muster; jedes
// andere Werkzeug zählt nicht.
func claudeBashPattern(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "Bash" {
		return "", true
	}
	if strings.HasPrefix(raw, "Bash(") && strings.HasSuffix(raw, ")") {
		return strings.TrimSuffix(strings.TrimPrefix(raw, "Bash("), ")"), true
	}
	return "", false
}

// matches meldet, ob der Eintrag die Zeile trifft. `*` passt auf jede Folge,
// auch über Leerzeichen und in Anführungszeichen hinein (gemessen in beiden
// Clients); ein Muster, das auf " *" endet, trifft auch die Zeile ohne
// Argumente. Die ältere Form von Claude Code `<präfix>:*` trifft den Präfix
// als ganzes Wort.
func (source permissionSource) matches(line string) bool {
	switch {
	case source.matchAll:
		return true
	case source.prefix:
		return line == source.Pattern || strings.HasPrefix(line, source.Pattern+" ")
	}
	if wildcardMatch(source.Pattern, line) {
		return true
	}
	if trimmed, ok := strings.CutSuffix(source.Pattern, " *"); ok {
		return wildcardMatch(trimmed, line)
	}
	return false
}

// wildcardMatch vergleicht mit `*` für jede Folge und `?` für ein Zeichen,
// verankert am Anfang und am Ende.
func wildcardMatch(pattern string, text string) bool {
	p, t := []rune(pattern), []rune(text)
	star, mark := -1, 0
	i, j := 0, 0
	for j < len(t) {
		switch {
		case i < len(p) && (p[i] == '?' || p[i] == t[j]):
			i++
			j++
		case i < len(p) && p[i] == '*':
			star, mark = i, j
			i++
		case star >= 0:
			i = star + 1
			mark++
			j = mark
		default:
			return false
		}
	}
	for i < len(p) && p[i] == '*' {
		i++
	}
	return i == len(p)
}

// evaluatePermissions wertet die Einträge für eine Zeile aus, wie der Client
// es tut, und nennt die entscheidenden Einträge. Ohne passenden Eintrag ist
// die Wirkung leer.
func evaluatePermissions(client GitHubPermissionsClient, sources []permissionSource, line string) (string, []permissionSource) {
	if client == GitHubPermissionsOpenCode {
		for i := len(sources) - 1; i >= 0; i-- {
			if sources[i].matches(line) {
				return sources[i].Effect, []permissionSource{sources[i]}
			}
		}
		return "", nil
	}
	byEffect := map[string][]permissionSource{}
	for _, source := range sources {
		if source.matches(line) {
			byEffect[source.Effect] = append(byEffect[source.Effect], source)
		}
	}
	for _, effect := range []string{permDeny, permAsk, permAllow} {
		if len(byEffect[effect]) > 0 {
			return effect, byEffect[effect]
		}
	}
	return "", nil
}

// clientDefault ist die Wirkung ohne passenden Eintrag: Claude Code fragt,
// OpenCode erlaubt.
func clientDefault(client GitHubPermissionsClient) string {
	if client == GitHubPermissionsOpenCode {
		return permAllow
	}
	return permAsk
}

// permissionContradicts meldet, ob got der Wirkung want widerspricht. Strenger
// zählt nicht: ask statt deny und deny statt ask lassen nichts ungefragt
// durch.
func permissionContradicts(want string, got string) bool {
	switch want {
	case permDeny, permAsk:
		return got == permAllow
	case permAllow:
		return got == permDeny || got == permAsk
	}
	return false
}

func permissionConflicts(target GitHubPermissionsTarget, sources []permissionSource, probes []gitHubPermissionProbe) []GitHubPermissionConflict {
	ownRules := map[string]bool{}
	for _, rule := range gitHubPermissionRules(target.Client) {
		ownRules[rule.Effect+"\x00"+rule.Pattern] = true
	}
	project := make([]permissionSource, 0, len(sources))
	for _, source := range sources {
		if !source.Own {
			project = append(project, source)
		}
	}

	var conflicts []GitHubPermissionConflict
	seen := map[string]int{}
	for _, probe := range probes {
		got, deciding := evaluatePermissions(target.Client, project, probe.Line)
		if got == "" || !permissionContradicts(probe.Want, got) {
			continue
		}
		all, _ := evaluatePermissions(target.Client, sources, probe.Line)
		if all == "" {
			all = clientDefault(target.Client)
		}
		effective := permissionContradicts(probe.Want, all)
		for _, source := range deciding {
			// Ein Eintrag, der einer eigenen Regel gleicht, widerspricht ihr
			// nicht; er ist nur nicht die ganze Regel.
			if ownRules[source.Effect+"\x00"+source.Pattern] && !source.prefix && !source.matchAll {
				continue
			}
			conflict := GitHubPermissionConflict{
				Assistant: target.Assistant,
				Path:      source.Path,
				Rule:      source.Raw,
				Effect:    source.Effect,
				Line:      probe.Line,
				Want:      probe.Want,
				Effective: effective,
			}
			// Je Eintrag eine Meldung. Wirkt er an irgendeiner Zeile, gilt
			// er als wirksam, und diese Zeile wird genannt.
			key := source.Path + "\x00" + source.Effect + "\x00" + source.Raw
			if index, ok := seen[key]; ok {
				if effective && !conflicts[index].Effective {
					conflicts[index] = conflict
				}
				continue
			}
			seen[key] = len(conflicts)
			conflicts = append(conflicts, conflict)
		}
	}
	return conflicts
}

// gitExcludeNeeded meldet, ob eine nicht erfasste Zieldatei aus git
// ausgenommen werden muss: project.vcs ist git, die Datei ist nicht erfasst,
// und git check-ignore sagt ausdrücklich „nicht ignoriert“ (Exit 1). Jede
// andere Antwort — kein Repository, git fehlt — heißt: nichts eintragen.
func gitExcludeNeeded(projectDir string, path string) bool {
	config, err := ReadConfig(projectDir)
	if err != nil || config.VCS != "git" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitCallTimeout)
	defer cancel()
	if _, code, _ := runGit(ctx, projectDir, "check-ignore", "-q", "--", path); code != 1 {
		return false
	}
	return !mcpTargetTracked(projectDir, filepath.FromSlash(path))
}

// addGitExclude trägt die Datei in .git/info/exclude ein, als Pfad ab der
// Wurzel des Repositorys mit führendem /. Die Datei kommt von
// `git rev-parse --git-path info/exclude`, damit ein Worktree die des
// Hauptrepositorys trifft.
func addGitExclude(projectDir string, path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitCallTimeout)
	defer cancel()
	excludeFile, code, reason := runGit(ctx, projectDir, "rev-parse", "--git-path", "info/exclude")
	if code != 0 {
		return fmt.Errorf("%s nicht aus git ausgenommen: %s", path, reason)
	}
	prefix, code, reason := runGit(ctx, projectDir, "rev-parse", "--show-prefix")
	if code != 0 {
		return fmt.Errorf("%s nicht aus git ausgenommen: %s", path, reason)
	}
	excludeFile = strings.TrimSpace(excludeFile)
	if !filepath.IsAbs(excludeFile) {
		excludeFile = filepath.Join(projectDir, excludeFile)
	}
	line := "/" + strings.TrimSpace(prefix) + path

	existing, err := os.ReadFile(excludeFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s lesen: %w", excludeFile, err)
	}
	text := string(existing)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += line + "\n"
	if err := os.MkdirAll(filepath.Dir(excludeFile), 0o755); err != nil {
		return fmt.Errorf("%s anlegen: %w", filepath.Dir(excludeFile), err)
	}
	if err := os.WriteFile(excludeFile, []byte(text), 0o644); err != nil {
		return fmt.Errorf("%s schreiben: %w", excludeFile, err)
	}
	return nil
}

// Lines fasst den Bericht für Meldungen in Zeilen, ohne Vorsilbe. Gemeldet
// wird, was geschrieben wurde, was ein Lauf noch täte, was unberührt bleibt,
// und jeder Widerspruch mit Vorschlag. Ohne Dienst und ohne eigene Einträge
// bleibt die Liste leer.
func (r GitHubPermissionsReport) Lines() []string {
	var lines []string
	for _, path := range r.Written {
		lines = append(lines, "GitHub-Berechtigungen in "+path+" eingetragen (Dienst "+strings.Join(r.Services, ", ")+"); der Client liest sie beim nächsten Start.")
	}
	for _, path := range r.Removed {
		lines = append(lines, "GitHub-Berechtigungen von k-playbook aus "+path+" entfernt: dem Projekt ist kein Dienst mehr zugeordnet.")
	}
	for _, path := range r.Excluded {
		lines = append(lines, path+" in .git/info/exclude eingetragen: die Datei trägt persönliche Einträge und bleibt unversioniert.")
	}
	for _, status := range r.Statuses {
		switch status.State {
		case GitHubPermissionsMissing:
			lines = append(lines, "GitHub-Berechtigungen für "+status.Assistant+" fehlen in "+status.Path+" ("+status.Detail+").")
		case GitHubPermissionsStale:
			lines = append(lines, status.Path+": "+status.Detail+".")
		case GitHubPermissionsUnreadable:
			lines = append(lines, "GitHub-Berechtigungen für "+status.Assistant+" nicht geschrieben: "+status.Path+" — "+status.Detail+".")
		case GitHubPermissionsSymlink:
			// Ohne Dienst gibt es dort nichts zu tun.
			if len(r.Services) > 0 {
				lines = append(lines, "GitHub-Berechtigungen für "+status.Assistant+": "+status.Path+" bleibt unberührt — "+status.Detail+".")
			}
		case GitHubPermissionsTracked:
			lines = append(lines, "GitHub-Berechtigungen für "+status.Assistant+": "+status.Path+" bleibt unberührt — "+status.Detail+".")
		}
	}
	for _, conflict := range r.Conflicts {
		lines = append(lines, conflict.Sentence())
	}
	if r.Error != "" {
		lines = append(lines, "GitHub-Berechtigungen nicht vollständig nachgezogen: "+r.Error)
	}
	return lines
}

// Sentence beschreibt einen Widerspruch mit Vorschlag.
func (c GitHubPermissionConflict) Sentence() string {
	verb := map[string]string{permAllow: "erlaubt", permAsk: "fragt bei", permDeny: "sperrt"}[c.Effect]
	if verb == "" {
		verb = "setzt " + c.Effect + " für"
	}
	want := map[string]string{permAllow: "ohne Rückfrage laufen", permAsk: "nachfragen", permDeny: "gesperrt sein"}[c.Want]
	sentence := "Widerspruch in " + c.Path + " (" + c.Assistant + "): " + verb + " „" + c.Rule + "“, aber „" + c.Line + "“ soll " + want + ". Vorschlag: den Eintrag entfernen"
	if c.Want == permAllow {
		sentence += "; den Weg über run erlaubt k-playbook selbst"
	}
	if c.Effective {
		return sentence + "."
	}
	return sentence + " (die Einträge von k-playbook überstimmen ihn derzeit)."
}
