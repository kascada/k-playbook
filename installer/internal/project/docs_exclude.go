package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// Der Block docs: in K-PLAYBOOK.yaml hält fest, was die Docs-Erzeuger beim
// Code-Scan auslassen, zusätzlich zu den festen Ausschlüssen ihres
// Command-Texts. Ein Command liest die Liste aus context.docs.exclude und
// schreibt sie über AddDocsExclude — nie die Datei selbst (Task 116).

// DocsExcludeProblem ist ein Eintrag, der nicht in die Liste kommt, samt Grund.
// Dieselbe Form meldet das Lesen (context.docs.problems) und das Eintragen
// (rejected).
type DocsExcludeProblem struct {
	Pattern string `json:"pattern"`
	Reason  string `json:"reason"`
}

// ContextDocs ist der Abschnitt docs der Kontextausgabe. Beide Listen sind nie
// null: ohne Block ist exclude leer, und es gibt nichts zu deuten.
type ContextDocs struct {
	// Exclude sind die Muster aus docs.exclude, die die Prüfung bestanden haben,
	// in der Reihenfolge der Datei und ohne Doppelte.
	Exclude []string `json:"exclude"`
	// Problems sind die verworfenen Einträge mit Grund.
	Problems []DocsExcludeProblem `json:"problems"`
}

// contextDocs baut den Abschnitt aus der gelesenen Konfiguration.
func contextDocs(config Config) ContextDocs {
	docs := ContextDocs{Exclude: []string{}, Problems: []DocsExcludeProblem{}}
	docs.Exclude = append(docs.Exclude, config.DocsExclude...)
	docs.Problems = append(docs.Problems, config.DocsProblems...)
	return docs
}

// docsBlockComment steht im geschriebenen Block unter docs:, nicht darüber:
// alles über dem Schlüssel liegt außerhalb dessen, was replaceTopLevelBlock
// ersetzt, und sammelte sich mit jedem Eintragen ein weiteres Mal an.
const docsBlockComment = `  # Vom Code-Scan der Docs-Erzeuger ausgenommen, zusätzlich zu den festen
  # Ausschlüssen. Muster wie in .gitignore, relativ zu project.repo_root.
`

// docsBlockExample ist der Block, wie ihn eine neue Konfiguration trägt: ganz
// als Kommentar, mit Beispiel und ohne Einträge. Ohne Block ändert sich nichts.
const docsBlockExample = `# docs:
#   # Vom Code-Scan der Docs-Erzeuger ausgenommen, zusätzlich zu den festen
#   # Ausschlüssen. Muster wie in .gitignore, relativ zu project.repo_root.
#   # Eintragen mit ` + "`k-playbook docs exclude add <muster>…`" + `.
#   exclude:
#     - docs/
#     - vendor/
`

// docsBlock rendert den Block mit den Einträgen in dieser Reihenfolge.
func docsBlock(patterns []string) string {
	var builder strings.Builder
	builder.WriteString("docs:\n")
	builder.WriteString(docsBlockComment)
	builder.WriteString("  exclude:\n")
	for _, pattern := range patterns {
		fmt.Fprintf(&builder, "    - %s\n", docsYAMLScalar(pattern))
	}
	return builder.String()
}

// windowsDrive erkennt einen Pfad wie C:\ oder C:/ — auf Linux kein absoluter
// Pfad für filepath.IsAbs, gemeint ist er trotzdem.
var windowsDrive = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

// checkDocsExcludePattern prüft ein Muster gegen project.repo_root als Basis.
// Zurück kommt der Grund, aus dem es verworfen wird, leer, wenn es gilt.
//
// Ein führender Schrägstrich zählt als absolut, obwohl er in einer .gitignore
// das Muster an die Wurzel bindet: Die Liste nennt Pfade im Projekt, und ein
// Pfad wie /home/… soll nicht still als „home/… ab der Wurzel" durchgehen. Ein
// Muster mit Schrägstrich in der Mitte ist in .gitignore ohnehin gebunden.
// Ein vorangestelltes ! (Wiedereinschluss) wird für die Prüfung abgenommen.
//
// Ein Muster steht in genau einer Zeile: Mit Zeilenumbruch, Wagenrücklauf oder
// einem anderen Steuerzeichen (auch Tabulator und die Unicode-Zeilentrenner)
// wird es verworfen. Roh eingefügt ergäbe ein Zeilenumbruch eine Zeile außerhalb
// des Blocks, und die gelesene Liste wiche von der gemeldeten ab. Beim Lesen
// trifft dieselbe Regel einen Eintrag, der ein solches Zeichen mitten in der
// Zeile oder als Escape in Anführungszeichen trägt.
func checkDocsExcludePattern(pattern string) string {
	if strings.ContainsFunc(pattern, isDocsExcludeLineBreaker) {
		return "enthält einen Zeilenumbruch oder ein anderes Steuerzeichen; ein Muster steht in genau einer Zeile"
	}
	body := strings.TrimPrefix(pattern, "!")
	slashed := strings.ReplaceAll(body, `\`, "/")
	if strings.HasPrefix(slashed, "/") || filepath.IsAbs(body) || windowsDrive.MatchString(body) ||
		slashed == "~" || strings.HasPrefix(slashed, "~/") {
		return "absolut; Muster gelten relativ zu project.repo_root"
	}
	if cleaned := path.Clean(slashed); cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "führt mit .. aus project.repo_root heraus"
	}
	return ""
}

// isDocsExcludeLineBreaker erkennt ein Zeichen, das in einem Muster nicht
// stehen darf: Steuerzeichen (C0, DEL, C1 samt NEL) und die Unicode-Zeilen-
// und Absatztrenner, die manche YAML-Leser als Zeilenende nehmen.
func isDocsExcludeLineBreaker(r rune) bool {
	return unicode.IsControl(r) || r == '\u2028' || r == '\u2029'
}

// parseDocsExclude liest docs.exclude zeilenweise: nur die Blockliste, je
// Zeile `- <muster>`, tiefer eingerückt als exclude: oder — ebenso gültiges
// YAML — in derselben Spalte wie exclude:. Leere Einträge und Kommentare werden
// übersprungen, Doppelte still zusammengefasst. Die Flussform in einer Zeile
// wird nicht gelesen, sondern als Problem gemeldet — still übergangen sähe sie
// aus wie keine Liste. `exclude: []` ist die leere Liste.
//
// Zurück kommen die gültigen Muster und die verworfenen samt Grund.
func parseDocsExclude(content string) (valid []string, problems []DocsExcludeProblem) {
	valid, problems = []string{}, []DocsExcludeProblem{}

	inDocs := false
	childIndent := -1
	listIndent := -1
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if isTopLevelYAMLLine(line) {
			key, _, _ := strings.Cut(trimmed, ":")
			inDocs = strings.TrimSpace(key) == "docs"
			childIndent, listIndent = -1, -1
			continue
		}
		if !inDocs {
			continue
		}
		indent := lineIndent(line)

		if listIndent >= 0 {
			// Ein Eintrag ist tiefer eingerückt als exclude: oder steht als
			// `- …` in derselben Spalte (Liste ohne Einrückung).
			if indent > listIndent || (indent == listIndent && isYAMLSeqItem(trimmed)) {
				if strings.HasPrefix(trimmed, "-") {
					item := docsExcludeItem(strings.TrimPrefix(trimmed, "-"))
					if item == "" {
						continue
					}
					if reason := checkDocsExcludePattern(item); reason != "" {
						problems = append(problems, DocsExcludeProblem{Pattern: item, Reason: reason})
					} else if !containsString(valid, item) {
						valid = append(valid, item)
					}
				}
				continue
			}
			listIndent = -1
		}

		// Kinder von docs: sind die Zeilen mit der Einrückung der ersten.
		if childIndent < 0 {
			childIndent = indent
		}
		if indent != childIndent {
			continue
		}
		key, value, found := strings.Cut(trimmed, ":")
		if !found || strings.TrimSpace(key) != "exclude" {
			continue
		}
		value = docsExcludeValue(value)
		switch {
		case value == "":
			listIndent = indent
		case value == "[]":
		default:
			problems = append(problems, DocsExcludeProblem{
				Pattern: value,
				Reason:  "Wert in einer Zeile wird nicht gelesen; je Muster eine Zeile `- <muster>` unter exclude:",
			})
		}
	}
	return valid, problems
}

// docsExcludeValue ist der Wert hinter `exclude:` ohne Kommentar: leer für die
// Blockliste, `[]` für die leere Flussliste, sonst der Wert in einer Zeile.
func docsExcludeValue(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "#") {
		return ""
	}
	if strings.TrimSpace(strings.SplitN(value, "#", 2)[0]) == "[]" {
		return "[]"
	}
	return value
}

// isYAMLSeqItem erkennt eine Zeile `- …` (oder einen leeren `-`) einer
// Blockliste. Ein Schlüssel wie `-x: y` ist kein Eintrag.
func isYAMLSeqItem(trimmed string) bool {
	return trimmed == "-" || strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "-\t")
}

// docsExcludeItem nimmt einen Listeneintrag ohne Spiegelstrich: Anführungs-
// zeichen ab, ein Kommentar dahinter weg.
func docsExcludeItem(value string) string {
	value = strings.TrimSpace(value)
	switch {
	case strings.HasPrefix(value, `"`):
		var unquoted string
		if err := json.NewDecoder(strings.NewReader(value)).Decode(&unquoted); err == nil {
			return strings.TrimSpace(unquoted)
		}
		return strings.TrimSpace(strings.Trim(value, `"`))
	case strings.HasPrefix(value, "'"):
		var builder strings.Builder
		for index := 1; index < len(value); index++ {
			if value[index] == '\'' {
				if index+1 < len(value) && value[index+1] == '\'' {
					builder.WriteByte('\'')
					index++
					continue
				}
				return strings.TrimSpace(builder.String())
			}
			builder.WriteByte(value[index])
		}
		return strings.TrimSpace(strings.Trim(value, "'"))
	}
	for _, marker := range []string{" #", "\t#"} {
		if index := strings.Index(value, marker); index >= 0 {
			value = value[:index]
		}
	}
	return strings.TrimSpace(value)
}

// DocsExcludeResult ist die Antwort von AddDocsExclude.
type DocsExcludeResult struct {
	// Config ist der Pfad der geänderten (oder unberührten) Datei.
	Config string `json:"config"`
	// Exclude ist die gültige Liste danach.
	Exclude []string `json:"exclude"`
	// Added sind die Muster, die neu dazugekommen sind.
	Added []string `json:"added"`
	// Rejected sind die verworfenen Muster mit Grund; sie stehen nicht in der Datei.
	Rejected []DocsExcludeProblem `json:"rejected"`
	// Changed meldet, ob die Datei geschrieben wurde.
	Changed bool `json:"changed"`
}

// AddDocsExclude trägt Muster in docs.exclude von K-PLAYBOOK.yaml ein und legt
// den Block an, wenn er fehlt — die Fachlogik hinter `k-playbook docs exclude
// add` und dem MCP-Werkzeug k_playbook_docs_exclude_add. Ohne ein nicht leeres
// Muster ist das invalid_input. Jedes Muster wird geprüft wie beim Lesen; ein
// verworfenes wird gemeldet, nicht geschrieben. Ein Muster, das schon in der
// Liste steht, wird übersprungen; ist nichts neu, bleibt die Datei byte-gleich.
//
// Fehlt der Block docs:, hängt replaceTopLevelBlock ihn an. Steht er da, wird
// er nicht neu gerendert: Die neuen Einträge kommen als eigene Zeilen hinter
// den letzten vorhandenen Eintrag der Liste (planDocsExcludeEdit), und jede
// andere Zeile — Nachbarschlüssel wie include:, Kommentare, vorhandene und
// verworfene Einträge — bleibt byte-gleich. Steht docs: oder exclude: in einer
// Form, die sich so nicht sicher ergänzen lässt (etwa die Flussform
// `exclude: [docs/]`), wird nichts geschrieben: Das ist unsupported mit Grund.
func AddDocsExclude(projectDir string, patterns []string) (DocsExcludeResult, error) {
	file := ConfigPath(projectDir)
	result := DocsExcludeResult{Config: file, Added: []string{}, Rejected: []DocsExcludeProblem{}}

	given := false
	for _, pattern := range patterns {
		given = given || strings.TrimSpace(pattern) != ""
	}
	if !given {
		return result, docRefErrorf(DocCodeInvalidInput, "kein Muster angegeben")
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return result, err
	}
	content := string(data)
	existing, _ := parseDocsExclude(content)

	exclude := append([]string{}, existing...)
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if reason := checkDocsExcludePattern(pattern); reason != "" {
			result.Rejected = append(result.Rejected, DocsExcludeProblem{Pattern: pattern, Reason: reason})
			continue
		}
		if containsString(exclude, pattern) {
			continue
		}
		exclude = append(exclude, pattern)
		result.Added = append(result.Added, pattern)
	}
	result.Exclude = exclude
	if len(result.Added) == 0 {
		return result, nil
	}

	updated, err := insertDocsExclude(content, result.Added)
	if err != nil {
		// Nichts geschrieben: Die Antwort darf nicht so tun, als stünden die
		// Muster jetzt in der Liste.
		result.Exclude = existing
		result.Added = []string{}
		return result, err
	}
	if err := os.WriteFile(file, []byte(updated), 0o644); err != nil {
		return result, err
	}
	result.Changed = true
	return result, nil
}

// docsExcludeEdit sagt, wie insertDocsExclude die neuen Einträge in einen
// vorhandenen Block docs: setzt, ohne eine andere Zeile anzufassen.
type docsExcludeEdit struct {
	// at ist der Index der Zeile, vor der eingefügt wird.
	at int
	// header ist die Zeile `exclude:`, wenn der Schlüssel neu angelegt wird.
	header string
	// itemIndent ist die Einrückung der neuen Einträge.
	itemIndent int
	// replace ist der Index einer Zeile, die durch replacement ersetzt wird,
	// sonst -1 — nur für `exclude: []`, das zur Blockliste wird.
	replace     int
	replacement string
}

// insertDocsExclude setzt die Muster in docs.exclude ein. Fehlt der Block
// docs:, legt replaceTopLevelBlock ihn hinten an; sonst wird nur eingefügt.
func insertDocsExclude(content string, added []string) (string, error) {
	lines := strings.Split(content, "\n")
	edit, found, err := planDocsExcludeEdit(lines)
	if err != nil {
		return "", err
	}
	if !found {
		return replaceTopLevelBlock(content, "docs", docsBlock(added)), nil
	}

	if edit.replace >= 0 {
		lines[edit.replace] = edit.replacement
	}
	// Eine Datei mit CRLF bekommt auch in den neuen Zeilen CRLF.
	eol := ""
	if edit.at > 0 && strings.HasSuffix(lines[edit.at-1], "\r") {
		eol = "\r"
	}
	inserted := []string{}
	if edit.header != "" {
		inserted = append(inserted, edit.header+eol)
	}
	for _, pattern := range added {
		inserted = append(inserted, strings.Repeat(" ", edit.itemIndent)+"- "+docsYAMLScalar(pattern)+eol)
	}
	result := append([]string{}, lines[:edit.at]...)
	result = append(result, inserted...)
	result = append(result, lines[edit.at:]...)
	return strings.Join(result, "\n"), nil
}

// docsExcludeUnsupported ist der Fehler für eine Form, die AddDocsExclude
// nicht sicher ergänzen kann. Geschrieben wird dann nichts.
func docsExcludeUnsupported(line int, reason string) error {
	return docRefErrorf(DocCodeUnsupported,
		"docs in K-PLAYBOOK.yaml, Zeile %d: %s — nichts eingetragen; je Muster eine Zeile `- <muster>` unter "+
			"docs.exclude: von Hand umstellen, dann erneut eintragen", line+1, reason)
}

// planDocsExcludeEdit sucht im Block docs: die Stelle für neue Einträge, so
// wie parseDocsExclude den Block liest. found ist false, wenn der Block fehlt.
// Eine Form, bei der ein Einfügen Inhalt verlöre oder ungültiges YAML ergäbe,
// ist ein Fehler, statt geraten zu werden: docs: mit Wert in der Kopfzeile oder
// als Liste, exclude: mehrfach, mit Wert in einer Zeile (Flussform, außer der
// leeren `[]`) oder mit Zeilen unter exclude:, die kein Eintrag `- <muster>`
// in einer gemeinsamen Spalte sind.
func planDocsExcludeEdit(lines []string) (docsExcludeEdit, bool, error) {
	edit := docsExcludeEdit{replace: -1}

	start := -1
	for index, line := range lines {
		if !isTopLevelYAMLLine(line) {
			continue
		}
		key, value, _ := strings.Cut(strings.TrimSpace(line), ":")
		if strings.TrimSpace(key) != "docs" {
			continue
		}
		if start >= 0 {
			return edit, true, docsExcludeUnsupported(index, "docs: steht mehrfach auf oberster Ebene")
		}
		if value = strings.TrimSpace(value); value != "" && !strings.HasPrefix(value, "#") {
			return edit, true, docsExcludeUnsupported(index, fmt.Sprintf("docs: trägt einen Wert in der Kopfzeile (%s)", value))
		}
		start = index
	}
	if start < 0 {
		return edit, false, nil
	}
	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if isTopLevelYAMLLine(lines[index]) {
			end = index
			break
		}
	}

	isContent := func(line string) bool {
		trimmed := strings.TrimSpace(line)
		return trimmed != "" && !strings.HasPrefix(trimmed, "#")
	}

	// Kinder von docs: sind die Zeilen mit der Einrückung der ersten.
	childIndent := -1
	lastIndented := start
	exclude := -1
	for index := start + 1; index < end; index++ {
		line := lines[index]
		if strings.TrimSpace(line) != "" && lineIndent(line) > 0 {
			lastIndented = index
		}
		if !isContent(line) {
			continue
		}
		indent := lineIndent(line)
		trimmed := strings.TrimSpace(line)
		if childIndent < 0 {
			childIndent = indent
			if isYAMLSeqItem(trimmed) {
				return edit, true, docsExcludeUnsupported(index, "docs: ist eine Liste, keine Zuordnung mit exclude:")
			}
		}
		if indent != childIndent {
			continue
		}
		key, _, found := strings.Cut(trimmed, ":")
		if !found || strings.TrimSpace(key) != "exclude" {
			continue
		}
		if exclude >= 0 {
			return edit, true, docsExcludeUnsupported(index, "exclude: steht mehrfach unter docs:")
		}
		exclude = index
	}

	if exclude < 0 {
		// Kein exclude: — hinter die letzte eingerückte Zeile des Blocks.
		// Kommentare ohne Einrückung am Ende gehören zu dem, was folgt.
		if childIndent < 0 {
			childIndent = 2
		}
		edit.at = lastIndented + 1
		edit.header = strings.Repeat(" ", childIndent) + "exclude:"
		edit.itemIndent = childIndent + 2
		return edit, true, nil
	}

	line := lines[exclude]
	colon := strings.Index(line, ":")
	switch value := docsExcludeValue(line[colon+1:]); value {
	case "":
	case "[]":
		// Die leere Flussliste wird zur Blockliste: `[]` fällt weg, ein
		// Kommentar dahinter bleibt stehen.
		body, crlf := strings.CutSuffix(line, "\r")
		rest := body[colon+1:]
		cut := strings.Index(rest, "[]")
		replacement := body[:colon+1]
		if tail := rest[cut+2:]; strings.TrimSpace(tail) != "" {
			replacement += tail
		}
		if crlf {
			replacement += "\r"
		}
		edit.replace, edit.replacement = exclude, replacement
		edit.at = exclude + 1
		edit.itemIndent = childIndent + 2
		return edit, true, nil
	default:
		return edit, true, docsExcludeUnsupported(exclude,
			fmt.Sprintf("exclude: trägt einen Wert in einer Zeile (%s), etwa die Flussform, und wird nicht umgeschrieben", value))
	}

	// Blockliste: Einträge tiefer als exclude: oder in derselben Spalte.
	itemIndent := -1
	last := exclude
	for index := exclude + 1; index < end; index++ {
		line := lines[index]
		if !isContent(line) {
			continue
		}
		indent := lineIndent(line)
		trimmed := strings.TrimSpace(line)
		if indent < childIndent || (indent == childIndent && !isYAMLSeqItem(trimmed)) {
			break
		}
		if !isYAMLSeqItem(trimmed) {
			return edit, true, docsExcludeUnsupported(index, "unter exclude: steht eine Zeile, die kein Eintrag `- <muster>` ist")
		}
		if itemIndent >= 0 && indent != itemIndent {
			return edit, true, docsExcludeUnsupported(index, "die Einträge unter exclude: stehen nicht in einer Spalte")
		}
		itemIndent = indent
		last = index
	}
	if itemIndent < 0 {
		itemIndent = childIndent + 2
	}
	edit.at = last + 1
	edit.itemIndent = itemIndent
	return edit, true, nil
}
