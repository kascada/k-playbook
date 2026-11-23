package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Task 178: ein Werkzeug, das einen Task in einem Schritt prüft, nummeriert
// und schreibt. Der Aufbau, den es prüft, steht in
// defaults/instructions/on-demand/tasks.md („Aufbau“); der Code erzwingt den
// mechanischen Teil, das Urteil (Kurzname, Intent, Zuschnitt, Etappen) bleibt
// beim Command.

// TaskKindName ist die Sorte der Tasks in der Sorten-Tabelle DocKinds.
const TaskKindName = "tasks"

// taskNumberPlaceholder steht im Trockenlauf anstelle der Nummer.
const taskNumberPlaceholder = "<NNN>"

// TaskCreateOptions sind die Eingaben von CreateTask.
type TaskCreateOptions struct {
	// Title ist der Titel ohne „Task <n> —“.
	Title string
	// Name ist der Kurzname; k-playbook normalisiert ihn.
	Name string
	// Body ist der Rumpf in Markdown, ohne Frontmatter und ohne H1.
	Body string
	// DryRun liefert das Ergebnis, ohne etwas anzulegen.
	DryRun bool
	// NoIntent hebt die Pflicht zu einem Abschnitt Intent auf.
	NoIntent bool
}

// TaskCreated ist die Antwort von CreateTask.
type TaskCreated struct {
	// Ref ist der keph:-Verweis mit dem echten User; im Trockenlauf leer.
	Ref string `json:"ref,omitempty"`
	// Number ist die vergebene Nummer; im Trockenlauf 0.
	Number int `json:"number,omitempty"`
	// Name ist der Dateiname; im Trockenlauf mit <NNN> anstelle der Nummer.
	Name     string `json:"name"`
	Revision int64  `json:"revision,omitempty"`
	// Warnings sind die Hinweise, die das Anlegen nicht verhindern.
	Warnings []string `json:"warnings"`
	DryRun   bool     `json:"dryRun,omitempty"`
	// Rendered ist im Trockenlauf der Text, der später geschrieben wird,
	// mit <NNN> anstelle der Nummer.
	Rendered string `json:"rendered,omitempty"`
}

// taskRefChecker prüft, ob das Ziel eines Verweises (project: oder keph:)
// existiert.
type taskRefChecker struct {
	projectDir string
	// docs ist der Zugang zur Collection; nil, wenn Kephalaion nicht bereit
	// ist (nur im Trockenlauf).
	docs *UserDocs
}

// CreateTask prüft, baut und legt einen Task an — der Weg für das MCP-Werkzeug
// k_playbook_task_create und `k-playbook task create`. Ist Kephalaion nicht
// bereit, scheitert das Anlegen mit kephalaion_unavailable; der Trockenlauf
// läuft ohne Nummer und ohne Prüfung der keph:-Verweise, mit einem Hinweis.
func CreateTask(ctx context.Context, projectDir string, opts TaskCreateOptions) (TaskCreated, error) {
	return CreateTaskVia(ctx, projectDir, opts, OpenUserDocs)
}

// CreateTaskVia ist CreateTask mit einem austauschbaren Zugang; die Hüllen
// geben ihren eigenen mit, den ihre Tests ersetzen.
func CreateTaskVia(ctx context.Context, projectDir string, opts TaskCreateOptions,
	open func(projectDir string) (*UserDocs, error)) (TaskCreated, error) {
	docs, err := open(projectDir)
	if err != nil {
		if opts.DryRun && DocRefErrorCode(err) == DocCodeKephalaionUnavailable {
			return createTaskWith(ctx, nil, projectDir, opts)
		}
		return TaskCreated{}, err
	}
	return createTaskWith(ctx, docs, projectDir, opts)
}

// createTaskWith ist CreateTask mit geöffnetem Zugang; docs ist nil nur ohne
// Anbindung im Trockenlauf.
func createTaskWith(ctx context.Context, docs *UserDocs, projectDir string, opts TaskCreateOptions) (TaskCreated, error) {
	name, err := NormalizeDocName(opts.Name)
	if err != nil {
		return TaskCreated{}, err
	}
	title := strings.TrimSpace(opts.Title)
	body := strings.TrimSpace(strings.ReplaceAll(opts.Body, "\r\n", "\n"))

	checker := taskRefChecker{projectDir: projectDir, docs: docs}
	violations, warnings := checkTask(ctx, title, body, opts.NoIntent, checker)
	if docs == nil {
		warnings = append(warnings, "Kephalaion ist nicht bereit: die keph:-Verweise und ein doppelter Kurzname bleiben ungeprüft")
	} else {
		warnings = append(warnings, duplicateTaskName(ctx, docs, name)...)
	}
	if len(violations) > 0 {
		return TaskCreated{}, docRefErrorf(DocCodeInvalidInput, "Task nicht angelegt, %d Verstoß/Verstöße:\n- %s",
			len(violations), strings.Join(violations, "\n- "))
	}

	if opts.DryRun {
		return TaskCreated{
			Name:     taskNumberPlaceholder + "-" + name + ".md",
			Warnings: warnings,
			DryRun:   true,
			Rendered: composeTask(taskNumberPlaceholder, title, body),
		}, nil
	}

	kind, ok := docs.anyKind(TaskKindName)
	if !ok {
		return TaskCreated{}, fmt.Errorf("Sorten-Tabelle ohne %s", TaskKindName)
	}
	created, doc, err := docs.createNumberedWith(ctx, kind, kind.dirFor(docs.user), name,
		func(number int) string { return composeTask(fmt.Sprintf("%03d", number), title, body) }, kephError)
	if err != nil {
		return TaskCreated{}, err
	}
	return TaskCreated{Ref: created.Ref, Number: created.Number, Name: created.Name, Revision: doc.Revision, Warnings: warnings}, nil
}

// composeTask baut den Text eines Tasks: Frontmatter, Überschrift, Rumpf; er
// endet mit genau einem Zeilenumbruch. number ist die dreistellige Nummer
// oder der Platzhalter.
func composeTask(number, title, body string) string {
	return "---\ntitle: " + quoteYAMLTitle(title) + "\n---\n\n# Task " + number + " — " + title + "\n\n" + body + "\n"
}

// quoteYAMLTitle setzt einen Wert in doppelte Anführungszeichen: ein \ wird
// \\, ein " wird \". So liest jeder YAML-Leser genau diesen Text, auch bei
// „: “ oder „ #“.
func quoteYAMLTitle(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

// duplicateTaskName warnt, wenn schon ein eigener offener Task denselben
// Kurznamen trägt.
func duplicateTaskName(ctx context.Context, docs *UserDocs, name string) []string {
	kind, ok := docs.anyKind(TaskKindName)
	if !ok {
		return nil
	}
	entries, matches, err := docs.listKind(ctx, kind, docs.user, false)
	if err != nil {
		return []string{"die vorhandenen Tasks ließen sich nicht lesen, ein doppelter Kurzname bleibt ungeprüft: " + err.Error()}
	}
	var warnings []string
	for i, entry := range entries {
		if strings.Contains(entry.Name, "/done/") {
			continue
		}
		file := matches[i].file
		if _, ok := docNumber(file); !ok {
			continue
		}
		existing := strings.TrimSuffix(file[strings.Index(file, "-")+1:], ".md")
		if existing == name {
			warnings = append(warnings, fmt.Sprintf("es gibt schon einen eigenen offenen Task mit dem Kurznamen %q: %s", name, file))
		}
	}
	return warnings
}

var (
	taskTitleNumber = regexp.MustCompile(`^(?i)task\s+\d+\s*[—–-]`)
	taskRelLink     = regexp.MustCompile(`\]\((\.{1,2}/[^)\s]*)\)`)
	taskQueueRef    = regexp.MustCompile(`keph:/queue/([^\s` + "`" + `'"<>()\[\]…]*)`)
	taskEtappe      = regexp.MustCompile(`^###\s+Etappe\s+(\d+)\b`)
	taskRefPrefixes = []string{"project:/", "keph:/", "k-playbook:/", "http://", "https://"}
	taskStdTools    = map[string]bool{"Read": true, "Write": true, "Edit": true, "Bash": true, "Glob": true, "Grep": true}
	taskSections    = []string{"Intent", "Referenzen", "Tools", "Ziel", "Kontext", "Zu bauen"}
)

// taskSection ist ein Abschnitt `## …` des Rumpfs.
type taskSection struct {
	name  string
	lines []string
}

// splitTaskBody zerlegt den Rumpf: die Zeilen vor dem ersten `## `-Abschnitt
// und die Abschnitte. Zeilen in Codeblöcken zählen nie als Überschrift.
// outside sind alle Zeilen außerhalb von Codeblöcken, in Reihenfolge.
func splitTaskBody(body string) (intro []string, sections []taskSection, outside []string) {
	fence := ""
	for _, line := range strings.Split(body, "\n") {
		if marker, ok := taskFenceMarker(line); ok {
			if fence == "" {
				fence = marker
			} else if strings.HasPrefix(marker, fence) {
				fence = ""
			}
			addTaskLine(&intro, &sections, line)
			continue
		}
		if fence == "" {
			outside = append(outside, line)
			if strings.HasPrefix(line, "## ") {
				sections = append(sections, taskSection{name: strings.TrimSpace(line[3:])})
				continue
			}
		}
		addTaskLine(&intro, &sections, line)
	}
	return intro, sections, outside
}

func addTaskLine(intro *[]string, sections *[]taskSection, line string) {
	if len(*sections) == 0 {
		*intro = append(*intro, line)
		return
	}
	last := &(*sections)[len(*sections)-1]
	last.lines = append(last.lines, line)
}

// taskFenceMarker erkennt den Beginn oder das Ende eines Codeblocks und
// liefert die Zeichenfolge des Zauns (mindestens drei ` oder ~).
func taskFenceMarker(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || trimmed == "" || (trimmed[0] != '`' && trimmed[0] != '~') {
		return "", false
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == trimmed[0] {
		n++
	}
	if n < 3 {
		return "", false
	}
	return trimmed[:n], true
}

func hasContent(lines []string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

// listItemToken liefert bei einer Listenzeile das erste Wort: den Inhalt der
// ersten Backticks, wenn die Zeile damit beginnt, sonst das erste Wort.
func listItemToken(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "* ") {
		return "", false
	}
	rest := strings.TrimSpace(trimmed[2:])
	if rest == "" {
		return "", true
	}
	if rest[0] == '`' {
		if end := strings.Index(rest[1:], "`"); end >= 0 {
			return rest[1 : 1+end], true
		}
	}
	if fields := strings.Fields(rest); len(fields) > 0 {
		return fields[0], true
	}
	return "", true
}

// checkTask prüft Titel und Rumpf. violations verhindern das Anlegen,
// warnings nicht.
func checkTask(ctx context.Context, title, body string, noIntent bool, checker taskRefChecker) (violations, warnings []string) {
	switch {
	case title == "":
		violations = append(violations, "title ist leer")
	case strings.ContainsAny(title, "\r\n"):
		violations = append(violations, "title muss eine Zeile sein")
	case taskTitleNumber.MatchString(title):
		violations = append(violations, "title enthält „Task <n> —“: das setzt das Werkzeug selbst, der Titel steht ohne")
	}

	if body == "" {
		return append(violations, "body ist leer"), warnings
	}
	first := strings.Split(body, "\n")[0]
	if strings.TrimSpace(first) == "---" {
		violations = append(violations, "body beginnt mit Frontmatter: das Frontmatter baut das Werkzeug")
	}

	intro, sections, outside := splitTaskBody(body)
	for _, line := range outside {
		if strings.HasPrefix(line, "# ") {
			violations = append(violations, "body enthält eine Überschrift der ersten Ebene (`# …`): die Überschrift „# Task <NNN> — <Titel>“ setzt das Werkzeug")
			break
		}
	}
	// Einleitungssatz: nach der Überschrift, vor dem ersten Abschnitt, steht
	// Text, der keine Überschrift ist.
	introText := false
	for _, line := range intro {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			introText = true
			break
		}
	}
	if !introText {
		violations = append(violations, "es fehlt der Einleitungssatz: ein Satz, was der Task erreicht, steht vor dem ersten Abschnitt")
	}

	// Reihenfolge und Leere der bekannten Abschnitte.
	lastIndex := -1
	seen := map[string]bool{}
	for _, section := range sections {
		index := -1
		for i, known := range taskSections {
			if known == section.name {
				index = i
			}
		}
		if index < 0 {
			continue
		}
		if seen[section.name] {
			violations = append(violations, fmt.Sprintf("der Abschnitt „%s“ steht mehrmals", section.name))
			continue
		}
		seen[section.name] = true
		if index < lastIndex {
			violations = append(violations, fmt.Sprintf("der Abschnitt „%s“ steht an der falschen Stelle; die Reihenfolge ist %s",
				section.name, strings.Join(taskSections, ", ")))
		} else {
			lastIndex = index
		}
		if !hasContent(section.lines) {
			violations = append(violations, fmt.Sprintf("der Abschnitt „%s“ ist leer; ein leerer Abschnitt entfällt", section.name))
		}
	}
	if !seen["Intent"] && !noIntent {
		violations = append(violations, "es fehlt „## Intent“ (noIntent: true hebt die Pflicht auf, etwa bei rein mechanischer Arbeit oder wenn der Task in einer Reihe nicht der letzte ist)")
	}

	for _, section := range sections {
		switch section.name {
		case "Referenzen":
			for _, line := range section.lines {
				token, isItem := listItemToken(line)
				if !isItem {
					continue
				}
				if !hasRefPrefix(token) {
					violations = append(violations, fmt.Sprintf("Referenzen: „%s“ ist kein Verweis mit Präfix (project:/, keph:/, k-playbook:/); ein nackter Pfad wird nicht geraten", token))
					continue
				}
				warnings = append(warnings, checker.check(ctx, token)...)
			}
		case "Tools":
			for _, line := range section.lines {
				if token, isItem := listItemToken(line); isItem && taskStdTools[token] {
					violations = append(violations, fmt.Sprintf("Tools: %s ist ein Standardwerkzeug und gehört nicht in die Liste", token))
				}
			}
		}
	}

	outsideText := strings.Join(outside, "\n")
	for _, m := range taskRelLink.FindAllStringSubmatch(outsideText, -1) {
		violations = append(violations, fmt.Sprintf("relativer Link %q: Verweise haben ein Präfix (project:/…, keph:/…, k-playbook:/…)", m[1]))
	}
	for _, m := range taskQueueRef.FindAllStringSubmatch(outsideText, -1) {
		rest := strings.Trim(m[1], "/")
		if strings.Count(rest, "/") == 0 && !strings.HasSuffix(m[1], ".md") {
			continue // nur die Ortsangabe eines Verzeichnisses
		}
		violations = append(violations, fmt.Sprintf("Verweis auf keph:/queue/%s: Auf nichts unter keph:/queue/ wird verwiesen, die Queue wird geleert; "+
			"den Inhalt in eigenen Worten hineinschreiben (k-playbook:/instructions/always/010-verweise.md)", m[1]))
	}

	// Etappen.
	var etappen []int
	for _, line := range outside {
		if m := taskEtappe.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			etappen = append(etappen, n)
		}
	}
	for i, n := range etappen {
		if n != i+1 {
			violations = append(violations, fmt.Sprintf("die Etappen sind nicht fortlaufend nummeriert: an Stelle %d steht Etappe %d", i+1, n))
			break
		}
	}
	if len(etappen) > 0 && (len(etappen) < 4 || len(etappen) > 8) {
		warnings = append(warnings, fmt.Sprintf("der Task hat %d Etappen; vier bis acht sind ein sinnvoller Rahmen", len(etappen)))
	}
	return violations, warnings
}

func hasRefPrefix(token string) bool {
	for _, prefix := range taskRefPrefixes {
		if strings.HasPrefix(token, prefix) {
			return true
		}
	}
	return false
}

// check prüft, ob das Ziel eines project:- oder keph:-Verweises existiert;
// das Ergebnis ist eine Warnung, nie ein Fehler. Platzhalter (<…>, *, …) und
// andere Präfixe bleiben ungeprüft.
func (c taskRefChecker) check(ctx context.Context, token string) []string {
	token = strings.TrimRight(token, ".,;:")
	if strings.ContainsAny(token, "<>*…{}") {
		return nil
	}
	switch {
	case strings.HasPrefix(token, "project:/"):
		rel := strings.TrimPrefix(token, "project:/")
		if rel == "" || strings.Contains(rel, "..") {
			return nil
		}
		if _, err := os.Stat(filepath.Join(c.projectDir, filepath.FromSlash(rel))); err != nil {
			return []string{fmt.Sprintf("%s gibt es im Projekt nicht", token)}
		}
	case strings.HasPrefix(token, "keph:/"):
		if c.docs == nil {
			return nil
		}
		segments := strings.Split(strings.TrimPrefix(token, "keph:/"), "/")
		for i, segment := range segments {
			if segment == docUserSegment {
				segments[i] = c.docs.user
			}
		}
		name := strings.TrimRight(strings.Join(segments, "/"), "/")
		if name == "" {
			return nil
		}
		read, err := c.docs.store.Read(ctx, name)
		if err != nil {
			return []string{fmt.Sprintf("%s ließ sich nicht prüfen: %v", token, err)}
		}
		if read.Kind == kephalaion.ReadKindNone {
			return []string{fmt.Sprintf("%s gibt es in der Collection nicht", token)}
		}
	}
	return nil
}
