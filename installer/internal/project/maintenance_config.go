package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Der Block maintenance: in K-PLAYBOOK.yaml (Task 161) steuert den
// Wartungslauf `k-playbook maintain`: das Tagesprogramm und die Sicherung der
// Projektdoku aus Kephalaion in einen Ordner des Repositorys.
//
//	maintenance:
//	  daily:
//	    - docs-backup
//	  backup:
//	    enabled: false
//	    source: docs
//	    target: keph-backup/docs
//	    git:
//	      commit: false
//
// Gelesen und ergänzt wird zeilenweise wie bei docs: und kephalaion:, ohne
// YAML-Bibliothek: Kommentare, Reihenfolge und fremde Schlüssel bleiben
// erhalten. Hub und Collection kommen aus dem Block kephalaion:, eine zweite
// Adresse gibt es nicht.

// MaintenanceBlockName ist der Block in K-PLAYBOOK.yaml.
const MaintenanceBlockName = "maintenance"

// MaintenanceBackupMarker ist die Markierung im Sicherungsziel: Sie sagt, dass
// der Ordner dem Export gehört. Der erste Lauf legt sie in einem fehlenden
// oder leeren Ziel an; jeder weitere verlangt sie, und --exclude schützt sie
// vor --delete.
const MaintenanceBackupMarker = ".k-playbook-backup"

// Vorgaben des Blocks: Sicherung aus, Quelle docs, Ziel keph-backup/docs,
// kein Commit.
const (
	DefaultMaintenanceBackupSource = "docs"
	DefaultMaintenanceBackupTarget = "keph-backup/docs"
)

// localLayerDirName ist das Verzeichnis für Projekteigenes neben
// .k-playbook/. Das Programm liest es nicht mehr selbst; als Sicherungsziel ist
// es trotzdem tabu, weil dort Dateien von Hand liegen können.
const localLayerDirName = "k-playbook-local"

// ConfigMaintenance ist der gelesene Block maintenance:. Fehlende Felder tragen
// ihre Vorgabe; Problems nennt, was nicht gelesen werden konnte.
type ConfigMaintenance struct {
	// Present sagt, ob der Block auf oberster Ebene steht.
	Present bool `json:"present,omitempty"`
	// Daily ist das Tagesprogramm. DailySet sagt, ob das Feld dasteht; ohne
	// Feld gilt DefaultMaintenanceDaily.
	Daily    []string `json:"daily,omitempty"`
	DailySet bool     `json:"dailySet,omitempty"`
	// BackupEnabled ist maintenance.backup.enabled; ohne Feld false.
	BackupEnabled bool   `json:"backupEnabled,omitempty"`
	BackupSource  string `json:"backupSource,omitempty"`
	// BackupTarget ist das Ziel, wie es in der Datei steht (relativ zum
	// Hauptverzeichnis), ohne Feld die Vorgabe.
	BackupTarget string `json:"backupTarget,omitempty"`
	// GitCommit ist maintenance.backup.git.commit; ohne Feld false.
	GitCommit bool `json:"gitCommit,omitempty"`
	// TargetProblem sagt, dass das Ziel selbst nicht gelesen werden konnte;
	// dann gibt es keinen Ausschluss für den Code-Scan.
	TargetProblem bool `json:"targetProblem,omitempty"`
	// Problems sind die Stellen, die nicht gelesen werden konnten, je mit
	// Zeile und Grund. Ein Wartungslauf beginnt dann nicht.
	Problems []string `json:"problems,omitempty"`
}

// EffectiveDaily ist das Tagesprogramm: das Feld daily, ohne Feld die Vorgabe.
func (m ConfigMaintenance) EffectiveDaily() []string {
	if m.DailySet {
		return append([]string{}, m.Daily...)
	}
	return append([]string{}, DefaultMaintenanceDaily...)
}

// maintNode ist eine Zeile `key:` oder `key: wert` im Block samt allem, was
// tiefer eingerückt darunter steht.
type maintNode struct {
	key string
	// value ist der Wert in derselben Zeile, ohne Kommentar; leer für einen
	// Block darunter.
	value    string
	line     int
	indent   int
	children []*maintNode
	// childIndent ist die Spalte der Kinder, -1 ohne Kinder.
	childIndent int
	items       []maintItem
	itemIndent  int
	// end ist die letzte inhaltliche Zeile des Teilbaums.
	end int
}

// maintItem ist eine Zeile `- wert` einer Blockliste.
type maintItem struct {
	value string
	line  int
}

func (n *maintNode) child(key string) *maintNode {
	for _, child := range n.children {
		if child.key == key {
			return child
		}
	}
	return nil
}

// maintTree ist der zeilenweise gelesene Block.
type maintTree struct {
	root  *maintNode
	lines []string
	// eol ist "\r", wenn die Datei CRLF trägt.
	eol string
	// problems sind Stellen, an denen der Aufbau nicht eindeutig ist; ein
	// Ergänzen schreibt dann nichts.
	problems []string
}

// maintProblem formuliert eine Stelle mit 1-basierter Zeile.
func maintProblem(line int, format string, args ...any) string {
	return fmt.Sprintf("Zeile %d: %s", line+1, fmt.Sprintf(format, args...))
}

// parseMaintenanceTree liest den Block maintenance: als Baum. found ist false,
// wenn der Block fehlt. Was sich nicht eindeutig zuordnen lässt — der Block
// mehrfach, ein Wert in der Kopfzeile, Tabulatoren in der Einrückung,
// Schlüssel mehrfach, Kinder in uneinheitlicher Spalte, Liste und Schlüssel
// nebeneinander —, steht in problems.
func parseMaintenanceTree(content string) (maintTree, bool) {
	tree := maintTree{lines: strings.Split(content, "\n")}
	lines := tree.lines

	start := -1
	for index, line := range lines {
		body := strings.TrimSuffix(line, "\r")
		if !isTopLevelYAMLLine(body) {
			continue
		}
		key, _, _ := strings.Cut(strings.TrimSpace(body), ":")
		if strings.TrimSpace(key) != MaintenanceBlockName {
			continue
		}
		if start >= 0 {
			tree.problems = append(tree.problems, maintProblem(index, "maintenance: steht mehrfach auf oberster Ebene"))
			continue
		}
		start = index
	}
	if start < 0 {
		return tree, false
	}
	if strings.HasSuffix(lines[start], "\r") {
		tree.eol = "\r"
	}

	header := strings.TrimSuffix(lines[start], "\r")
	_, rest, _ := strings.Cut(header, ":")
	headerValue, _ := splitValueComment(rest)
	root := &maintNode{key: MaintenanceBlockName, value: strings.TrimSpace(headerValue), line: start,
		indent: 0, childIndent: -1, itemIndent: -1, end: start}
	tree.root = root
	if root.value != "" {
		tree.problems = append(tree.problems, maintProblem(start, "maintenance: trägt einen Wert in der Kopfzeile (%s)", root.value))
	}

	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if isTopLevelYAMLLine(strings.TrimSuffix(lines[index], "\r")) {
			end = index
			break
		}
	}

	stack := []*maintNode{root}
	for index := start + 1; index < end; index++ {
		raw := strings.TrimSuffix(lines[index], "\r")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lead := raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
		if strings.Contains(lead, "\t") {
			tree.problems = append(tree.problems, maintProblem(index, "Tabulator in der Einrückung"))
			continue
		}
		indent := len(lead)
		isItem := isYAMLSeqItem(trimmed)

		for len(stack) > 1 {
			top := stack[len(stack)-1]
			// Eine Liste darf in derselben Spalte stehen wie ihr Schlüssel.
			if indent > top.indent || (isItem && indent == top.indent && top.value == "" && len(top.children) == 0) {
				break
			}
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		for _, open := range stack {
			open.end = index
		}

		if isItem {
			switch {
			case len(parent.children) > 0:
				tree.problems = append(tree.problems, maintProblem(index, "%s: Listeneintrag neben Schlüsseln", parent.key))
				continue
			case parent.itemIndent >= 0 && indent != parent.itemIndent:
				tree.problems = append(tree.problems, maintProblem(index, "%s: die Einträge stehen nicht in einer Spalte", parent.key))
				continue
			}
			parent.itemIndent = indent
			parent.items = append(parent.items, maintItem{value: docsExcludeItem(strings.TrimPrefix(trimmed, "-")), line: index})
			continue
		}

		key, rest, found := strings.Cut(trimmed, ":")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			tree.problems = append(tree.problems, maintProblem(index, "weder Schlüssel noch Listeneintrag"))
			continue
		}
		switch {
		case len(parent.items) > 0:
			tree.problems = append(tree.problems, maintProblem(index, "%s: Schlüssel %s neben Listeneinträgen", parent.key, key))
			continue
		case parent.value != "":
			tree.problems = append(tree.problems, maintProblem(index, "%s: Schlüssel %s unter einem Wert in einer Zeile", parent.key, key))
			continue
		case parent.childIndent >= 0 && indent != parent.childIndent:
			tree.problems = append(tree.problems, maintProblem(index, "%s: Schlüssel %s steht nicht in der Spalte der anderen", parent.key, key))
			continue
		case parent.child(key) != nil:
			tree.problems = append(tree.problems, maintProblem(index, "%s: Schlüssel %s steht mehrfach", parent.key, key))
			continue
		}
		parent.childIndent = indent
		value, _ := splitValueComment(rest)
		node := &maintNode{key: key, value: strings.TrimSpace(value), line: index, indent: indent,
			childIndent: -1, itemIndent: -1, end: index}
		parent.children = append(parent.children, node)
		stack = append(stack, node)
	}
	return tree, true
}

// parseMaintenanceBlock liest den Block in ConfigMaintenance. Ohne Block gelten
// die Vorgaben, und Present ist false.
func parseMaintenanceBlock(content string) ConfigMaintenance {
	config := ConfigMaintenance{BackupSource: DefaultMaintenanceBackupSource, BackupTarget: DefaultMaintenanceBackupTarget}
	tree, found := parseMaintenanceTree(content)
	if !found {
		return config
	}
	config.Present = true
	config.Problems = append(config.Problems, tree.problems...)
	root := tree.root
	if len(root.items) > 0 {
		config.Problems = append(config.Problems, maintProblem(root.items[0].line, "maintenance: ist eine Liste, keine Zuordnung"))
	}

	if daily := root.child("daily"); daily != nil {
		config.DailySet = true
		config.Daily = []string{}
		switch {
		case daily.value == "[]":
		case daily.value != "":
			config.Problems = append(config.Problems, maintProblem(daily.line,
				"maintenance.daily: Wert in einer Zeile (%s) wird nicht gelesen; je Schritt eine Zeile `- <schritt>` unter daily:", daily.value))
		case len(daily.children) > 0:
			config.Problems = append(config.Problems, maintProblem(daily.line, "maintenance.daily: ist keine Liste"))
		default:
			for _, item := range daily.items {
				if item.value != "" && !containsString(config.Daily, item.value) {
					config.Daily = append(config.Daily, item.value)
				}
			}
		}
	}

	backup := root.child("backup")
	if backup == nil {
		return config
	}
	if !maintMapping(backup, "maintenance.backup", &config.Problems) {
		return config
	}
	if node := backup.child("enabled"); node != nil {
		config.BackupEnabled = maintBool(node, "maintenance.backup.enabled", &config.Problems)
	}
	if node := backup.child("source"); node != nil {
		if source, ok := maintScalar(node, "maintenance.backup.source", &config.Problems); ok {
			config.BackupSource = source
			if source != DefaultMaintenanceBackupSource {
				config.Problems = append(config.Problems, maintProblem(node.line,
					"maintenance.backup.source: %q wird nicht unterstützt; als Quelle gibt es nur %s", source, DefaultMaintenanceBackupSource))
			}
		}
	}
	if node := backup.child("target"); node != nil {
		target, ok := maintScalar(node, "maintenance.backup.target", &config.Problems)
		switch {
		case !ok:
			config.TargetProblem = true
		case target == "":
			config.TargetProblem = true
			config.Problems = append(config.Problems, maintProblem(node.line, "maintenance.backup.target ist leer"))
		default:
			config.BackupTarget = target
		}
	}
	if git := backup.child("git"); git != nil && maintMapping(git, "maintenance.backup.git", &config.Problems) {
		if node := git.child("commit"); node != nil {
			config.GitCommit = maintBool(node, "maintenance.backup.git.commit", &config.Problems)
		}
	}
	return config
}

// maintMapping prüft, dass ein Knoten eine Zuordnung mit Kindern darunter ist.
func maintMapping(node *maintNode, name string, problems *[]string) bool {
	switch {
	case node.value != "":
		*problems = append(*problems, maintProblem(node.line, "%s: Wert in einer Zeile (%s) wird nicht gelesen; je Feld eine eigene Zeile darunter", name, node.value))
		return false
	case len(node.items) > 0:
		*problems = append(*problems, maintProblem(node.line, "%s: ist eine Liste, keine Zuordnung", name))
		return false
	}
	return true
}

// maintScalar liest einen Wert in einer Zeile, ohne Anführungszeichen.
func maintScalar(node *maintNode, name string, problems *[]string) (string, bool) {
	if len(node.children) > 0 || len(node.items) > 0 {
		*problems = append(*problems, maintProblem(node.line, "%s: erwartet wird ein Wert in derselben Zeile", name))
		return "", false
	}
	return configScalar(node.value), true
}

// maintBool liest true oder false; alles andere ist ein Problem und gilt als
// false — eine Sicherung schaltet sich nicht aus Versehen ein.
func maintBool(node *maintNode, name string, problems *[]string) bool {
	value, ok := maintScalar(node, name, problems)
	if !ok {
		return false
	}
	switch value {
	case "true":
		return true
	case "false":
		return false
	}
	*problems = append(*problems, maintProblem(node.line, "%s: erwartet wird true oder false, nicht %q", name, value))
	return false
}

// maintDefault ist ein Feld des Blocks mit seiner Vorgabe: ein Wert, eine
// Liste oder Kinder.
type maintDefault struct {
	key      string
	scalar   string
	list     []string
	children []maintDefault
}

// maintenanceDefaults sind die Felder, die Setup und Upgrade schreiben, in
// dieser Reihenfolge. Die Sicherung ist aus, der Commit auch.
func maintenanceDefaults() []maintDefault {
	return []maintDefault{
		{key: "daily", list: DefaultMaintenanceDaily},
		{key: "backup", children: []maintDefault{
			{key: "enabled", scalar: "false"},
			{key: "source", scalar: DefaultMaintenanceBackupSource},
			{key: "target", scalar: DefaultMaintenanceBackupTarget},
			{key: "git", children: []maintDefault{
				{key: "commit", scalar: "false"},
			}},
		}},
	}
}

// render schreibt das Feld samt Kindern mit der Einrückung indent; Kinder
// eine Stufe (zwei Leerzeichen) tiefer.
func (d maintDefault) render(indent int, eol string) []string {
	pad := strings.Repeat(" ", indent)
	switch {
	case d.list != nil:
		lines := []string{pad + d.key + ":" + eol}
		for _, item := range d.list {
			lines = append(lines, pad+"  - "+item+eol)
		}
		return lines
	case d.children != nil:
		lines := []string{pad + d.key + ":" + eol}
		for _, child := range d.children {
			lines = append(lines, child.render(indent+2, eol)...)
		}
		return lines
	}
	return []string{pad + d.key + ": " + d.scalar + eol}
}

// maintenanceBlockComment steht im geschriebenen Block unter maintenance:,
// eingerückt, damit er zum Block gehört.
const maintenanceBlockComment = `  # Wartungslauf ` + "`k-playbook maintain`" + `: daily ist das Tagesprogramm ohne --only.
  # backup sichert keph:/docs/ nach target (relativ zum Hauptverzeichnis, ein
  # nur dafür reservierter Ordner im Repository). Gilt für alle Nutzer; bei
  # mehreren Personen bleibt git.commit auf false, und nur die zuständige
  # Person ruft ` + "`k-playbook maintain --commit`" + ` auf.
`

// maintenanceBlock ist der vollständige Block mit den Vorgaben, wie ihn eine
// neue Konfiguration trägt und wie das Upgrade ihn anlegt, wenn er fehlt.
func maintenanceBlock() string {
	var builder strings.Builder
	builder.WriteString(MaintenanceBlockName + ":\n")
	builder.WriteString(maintenanceBlockComment)
	for _, field := range maintenanceDefaults() {
		for _, line := range field.render(2, "") {
			builder.WriteString(line + "\n")
		}
	}
	return builder.String()
}

// MaintenanceBlockResult ist die Antwort von EnsureMaintenanceBlock und
// PlanMaintenanceBlock.
type MaintenanceBlockResult struct {
	// Added nennt die ergänzten Felder als Pfad, etwa maintenance.backup.git;
	// fehlte der Block ganz, steht dort nur maintenance.
	Added []string `json:"added,omitempty"`
	// Problem sagt, warum nichts ergänzt wurde, obwohl etwas fehlt oder der
	// Block nicht eindeutig zuzuordnen ist.
	Problem string `json:"problem,omitempty"`
	// Changed sagt, ob die Datei geschrieben wurde.
	Changed bool `json:"changed,omitempty"`
}

// planMaintenanceBlock ergänzt im Text der Konfiguration, was am Block fehlt,
// und gibt den neuen Text zurück. Bestehende Werte bleiben, auch ein
// explizites enabled: false; fremde Schlüssel und Kommentare bleiben byte-
// gleich. Fehlt der Block, wird er hinten angehängt. Lässt sich der Block
// nicht eindeutig zuordnen, kommt der unveränderte Text mit problem zurück.
func planMaintenanceBlock(content string) (string, []string, string) {
	tree, found := parseMaintenanceTree(content)
	if len(tree.problems) > 0 {
		return content, nil, tree.problems[0]
	}
	if !found {
		return replaceTopLevelBlock(content, MaintenanceBlockName, maintenanceBlock()), []string{MaintenanceBlockName}, ""
	}
	if tree.root.value != "" || len(tree.root.items) > 0 {
		return content, nil, maintProblem(tree.root.line, "maintenance: ist keine Zuordnung mit eigenen Zeilen")
	}

	type insertion struct {
		at     int
		indent int
		lines  []string
	}
	var inserts []insertion
	var added []string
	var problem string

	var walk func(node *maintNode, path string, defaults []maintDefault)
	walk = func(node *maintNode, path string, defaults []maintDefault) {
		var missing []string
		childIndent := node.childIndent
		if childIndent < 0 {
			childIndent = node.indent + 2
		}
		for _, field := range defaults {
			existing := node.child(field.key)
			if existing == nil {
				missing = append(missing, field.render(childIndent, tree.eol)...)
				added = append(added, path+"."+field.key)
				continue
			}
			if field.children == nil {
				// Ein Blatt, das dasteht, bleibt, wie es ist — auch in einer
				// Form, die das Lesen als Problem meldet.
				continue
			}
			if existing.value != "" || len(existing.items) > 0 {
				if problem == "" {
					problem = maintProblem(existing.line, "%s.%s ist keine Zuordnung mit eigenen Zeilen; fehlende Felder lassen sich nicht ergänzen", path, field.key)
				}
				continue
			}
			walk(existing, path+"."+field.key, field.children)
		}
		if len(missing) > 0 {
			inserts = append(inserts, insertion{at: node.end + 1, indent: childIndent, lines: missing})
		}
	}
	walk(tree.root, MaintenanceBlockName, maintenanceDefaults())
	if problem != "" {
		return content, nil, problem
	}
	if len(inserts) == 0 {
		return content, nil, ""
	}

	// Von unten nach oben einsetzen, damit die Indizes stimmen. An derselben
	// Stelle kommt das tiefer Eingerückte zuerst: Es gehört zum Teilbaum, der
	// dort endet.
	lines := tree.lines
	for i := 0; i < len(inserts); i++ {
		for j := i + 1; j < len(inserts); j++ {
			a, b := inserts[i], inserts[j]
			if b.at > a.at || (b.at == a.at && b.indent < a.indent) {
				inserts[i], inserts[j] = inserts[j], inserts[i]
			}
		}
	}
	for _, insert := range inserts {
		result := append([]string{}, lines[:insert.at]...)
		result = append(result, insert.lines...)
		lines = append(result, lines[insert.at:]...)
	}
	return strings.Join(lines, "\n"), added, ""
}

// PlanMaintenanceBlock meldet, was EnsureMaintenanceBlock an K-PLAYBOOK.yaml
// ergänzen würde, ohne zu schreiben — für `k-playbook upgrade --check`.
func PlanMaintenanceBlock(projectDir string) (MaintenanceBlockResult, error) {
	content, ok, err := maintenanceConfigContent(projectDir)
	if err != nil || !ok {
		return MaintenanceBlockResult{}, err
	}
	_, added, problem := planMaintenanceBlock(content)
	return MaintenanceBlockResult{Added: added, Problem: problem}, nil
}

// EnsureMaintenanceBlock ergänzt in K-PLAYBOOK.yaml, was am Block maintenance:
// fehlt, mit den Vorgaben (Sicherung aus). Bestehende Werte werden nie
// überschrieben; wiederholte Aufrufe ändern nichts mehr. Eine fehlende Datei
// oder eine Konfiguration einer anderen schema_version wird nicht angefasst.
func EnsureMaintenanceBlock(projectDir string) (MaintenanceBlockResult, error) {
	content, ok, err := maintenanceConfigContent(projectDir)
	if err != nil || !ok {
		return MaintenanceBlockResult{}, err
	}
	updated, added, problem := planMaintenanceBlock(content)
	result := MaintenanceBlockResult{Added: added, Problem: problem}
	if updated == content {
		return result, nil
	}
	path := ConfigPath(projectDir)
	info, err := os.Stat(path)
	if err != nil {
		return MaintenanceBlockResult{}, err
	}
	if err := os.WriteFile(path, []byte(updated), info.Mode().Perm()); err != nil {
		return MaintenanceBlockResult{}, err
	}
	result.Changed = true
	return result, nil
}

// maintenanceConfigContent liest die Konfiguration für das Ergänzen. ok ist
// false, wenn es keine Datei gibt oder ihre Fassung nicht die dieses
// Werkzeugs ist: Eine fremde Fassung wird nicht nach diesen Regeln
// umgeschrieben.
func maintenanceConfigContent(projectDir string) (string, bool, error) {
	data, err := os.ReadFile(ConfigPath(projectDir))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	config, err := ReadConfig(projectDir)
	if err != nil {
		return "", false, err
	}
	if SchemaState(config) != SchemaOK {
		return "", false, nil
	}
	return string(data), true, nil
}

// DescribeMaintenanceBlock ist der Satz, mit dem `k-playbook upgrade` das
// Ergänzen meldet; would sagt, ob es die Vorschau von --check ist.
func DescribeMaintenanceBlock(result MaintenanceBlockResult, would bool) string {
	if result.Problem != "" {
		return "der Block " + MaintenanceBlockName + " in " + ConfigFileName + " ist nicht eindeutig zuzuordnen und wird nicht ergänzt — " +
			result.Problem + "; von Hand richten (k-playbook:/docs/handbook/k-playbook-format.md, Abschnitt maintenance)."
	}
	if len(result.Added) == 0 {
		return ""
	}
	verb := "ergänzt"
	if would {
		verb = "würde ergänzt"
	}
	what := "Block " + MaintenanceBlockName
	if !(len(result.Added) == 1 && result.Added[0] == MaintenanceBlockName) {
		what = strings.Join(result.Added, ", ")
	}
	return what + " in " + ConfigFileName + " " + verb + " (Doku-Sicherung deaktiviert, Vorgaben; vorhandene Werte bleiben). " +
		ConfigDisplayName + " ist versioniert: Die Änderung will committet werden."
}

// MaintenanceTarget ist das geprüfte Sicherungsziel.
type MaintenanceTarget struct {
	// Configured ist der Wert aus der Datei.
	Configured string `json:"configured"`
	// Path ist der aufgelöste absolute Pfad, Symlinks ausgewertet.
	Path string `json:"path"`
	// RepoRoot ist project.repoRoot, Symlinks ausgewertet.
	RepoRoot string `json:"repoRoot"`
	// Rel ist der Pfad relativ zu RepoRoot, mit /.
	Rel string `json:"rel"`
}

// ResolveMaintenanceTarget löst maintenance.backup.target gegen das
// Hauptverzeichnis auf und lehnt gefährliche Ziele ab: Nach dem Auswerten der
// Symlinks (fehlt das Ziel noch, auf seinem nächsten vorhandenen Vorfahren)
// muss es unterhalb von project.repoRoot liegen. Es darf weder
// project.repoRoot, das Hauptverzeichnis, .git, .k-playbook/ noch
// k-playbook-local/ sein, keinem davon übergeordnet sein und nicht in .git,
// .k-playbook/ oder k-playbook-local/ liegen.
func ResolveMaintenanceTarget(projectDir string, config Config) (MaintenanceTarget, error) {
	configured := config.Maintenance.BackupTarget
	if configured == "" {
		configured = DefaultMaintenanceBackupTarget
	}
	result := MaintenanceTarget{Configured: configured}
	reject := func(reason string) (MaintenanceTarget, error) {
		return result, fmt.Errorf("maintenance.backup.target %q: %s", configured, reason)
	}
	if config.Maintenance.TargetProblem {
		return reject("nicht lesbar")
	}
	slashed := strings.ReplaceAll(configured, `\`, "/")
	if filepath.IsAbs(configured) || strings.HasPrefix(slashed, "/") || windowsDrive.MatchString(configured) ||
		slashed == "~" || strings.HasPrefix(slashed, "~/") {
		return reject("muss relativ zum Hauptverzeichnis angegeben sein")
	}
	if strings.ContainsFunc(configured, isDocsExcludeLineBreaker) {
		return reject("enthält ein Steuerzeichen")
	}

	target, err := evalPathFromAncestor(filepath.Join(projectDir, configured))
	if err != nil {
		return reject(err.Error())
	}
	repoRoot, err := evalPathFromAncestor(RepoRootDir(projectDir, config))
	if err != nil {
		return reject("project.repoRoot nicht auflösbar: " + err.Error())
	}
	main, err := evalPathFromAncestor(projectDir)
	if err != nil {
		return reject("Hauptverzeichnis nicht auflösbar: " + err.Error())
	}
	result.Path, result.RepoRoot = target, repoRoot

	if target == repoRoot {
		return reject("ist project.repoRoot")
	}
	if !pathStrictlyBelow(repoRoot, target) {
		return reject("liegt nach dem Auswerten der Symlinks nicht unterhalb von project.repoRoot (" + target + ")")
	}
	if target == main || pathStrictlyBelow(target, main) {
		return reject("ist das Hauptverzeichnis oder ihm übergeordnet")
	}
	protected := []struct{ path, name string }{
		{filepath.Join(repoRoot, ".git"), ".git"},
		{filepath.Join(main, ".git"), ".git"},
		{filepath.Join(main, ProjectDirName), ProjectDirName + "/"},
		{filepath.Join(main, localLayerDirName), localLayerDirName + "/"},
	}
	for _, entry := range protected {
		resolved, err := evalPathFromAncestor(entry.path)
		if err != nil {
			resolved = entry.path
		}
		for _, candidate := range []string{entry.path, resolved} {
			switch {
			case target == candidate:
				return reject("ist " + entry.name)
			case pathStrictlyBelow(target, candidate):
				return reject("ist " + entry.name + " übergeordnet")
			case pathStrictlyBelow(candidate, target):
				return reject("liegt in " + entry.name)
			}
		}
	}

	rel, err := filepath.Rel(repoRoot, target)
	if err != nil {
		return reject(err.Error())
	}
	result.Rel = filepath.ToSlash(rel)
	return result, nil
}

// evalPathFromAncestor wertet die Symlinks eines Pfads aus. Fehlt er noch, auf
// seinem nächsten vorhandenen Vorfahren; der fehlende Rest kommt unverändert
// dahinter. Ein Symlink, der ins Leere zeigt, ist ein Fehler.
func evalPathFromAncestor(path string) (string, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		path = absolute
	}
	current := path
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", fmt.Errorf("Symlink nicht auflösbar: %s", current)
			}
			rest, err := filepath.Rel(current, path)
			if err != nil {
				return "", err
			}
			return filepath.Clean(filepath.Join(resolved, rest)), nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path, nil
		}
		current = parent
	}
}

// pathStrictlyBelow sagt, ob child echt unterhalb von parent liegt.
func pathStrictlyBelow(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil || rel == "." || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// maintenanceDocsExclude ist der Eintrag für context.docs.exclude: das
// geprüfte Ziel relativ zu project.repoRoot, verankert und als Verzeichnis,
// etwa /sub/keph-backup/docs/. Ohne Block, mit unlesbarem oder abgelehntem
// Ziel gibt es keinen.
func maintenanceDocsExclude(projectDir string, config Config) (string, bool) {
	if !config.Maintenance.Present || projectDir == "" {
		return "", false
	}
	target, err := ResolveMaintenanceTarget(projectDir, config)
	if err != nil || target.Rel == "" {
		return "", false
	}
	return "/" + strings.TrimSuffix(target.Rel, "/") + "/", true
}
