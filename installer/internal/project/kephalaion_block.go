package project

import (
	"fmt"
	"os"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Der Block kephalaion: in .k-playbook/K-PLAYBOOK.yaml (Task 138) trägt die
// Anbindung des Projekts an seine Collection: den Hub-Alias und die
// Collection. Beide sind für alle Nutzer eines Projekts gleich und deshalb
// versioniert; Kephalaion verteilt den Hub-Alias über den setup code, damit
// er auf jedem Rechner gleich lautet. Der Account gehört zum Rechner und
// kommt von Kephalaion (kephalaion node client) — ein account: im Block wird
// nicht gelesen, die Gesundheitsprüfung warnt davor.
//
//	kephalaion:
//	  hub: vm
//	  collection: k-playbook-dev

// KephalaionBlockName ist der Block in K-PLAYBOOK.yaml mit der Anbindung.
const KephalaionBlockName = "kephalaion"

// ConfigKephalaion ist der Block kephalaion: aus K-PLAYBOOK.yaml.
type ConfigKephalaion struct {
	// Present sagt, ob der Block auf oberster Ebene steht.
	Present    bool   `json:"present,omitempty"`
	Hub        string `json:"hub,omitempty"`
	Collection string `json:"collection,omitempty"`
	// AccountSet sagt, ob im Block ein account: steht. Gelesen wird der Wert
	// nicht; der Account kommt von Kephalaion.
	AccountSet bool `json:"accountSet,omitempty"`
}

// Binding ist die Anbindung für das Paket kephalaion: Hub und Collection.
// Ohne beide gilt sie als nicht eingetragen.
func (c ConfigKephalaion) Binding() kephalaion.Binding {
	return kephalaion.Binding{Hub: c.Hub, Collection: c.Collection}
}

// ConfigDisplayName ist der Ort der Konfiguration, wie eine Meldung ihn
// nennt: relativ zum Hauptverzeichnis.
const ConfigDisplayName = ProjectDirName + "/" + ConfigFileName

// parseKephalaionBlock liest den Block kephalaion: zeilenweise: hub,
// collection und — nur als Vorhandensein — account, jeweils als Kind des
// Blocks (die Einrückung der ersten Zeile darunter). Tiefere Zeilen,
// Kommentare und unbekannte Schlüssel werden übergangen. Ein Wert verliert
// Anführungszeichen und einen Kommentar dahinter.
func parseKephalaionBlock(content string) ConfigKephalaion {
	block := ConfigKephalaion{}
	inBlock := false
	childIndent := -1
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if isTopLevelYAMLLine(line) {
			inBlock = isBlockStart(line, KephalaionBlockName)
			if inBlock {
				block.Present = true
			}
			childIndent = -1
			continue
		}
		if !inBlock {
			continue
		}
		indent := lineIndent(line)
		if childIndent < 0 {
			childIndent = indent
		}
		if indent != childIndent {
			continue
		}
		key, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "hub":
			block.Hub = configScalar(value)
		case "collection":
			block.Collection = configScalar(value)
		case "account":
			block.AccountSet = true
		}
	}
	return block
}

// configScalar nimmt einen Wert in einer Zeile: ohne Kommentar dahinter und
// ohne umschließende Anführungszeichen.
func configScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') {
		if end := strings.IndexByte(value[1:], value[0]); end >= 0 {
			return strings.TrimSpace(value[1 : end+1])
		}
	}
	if strings.HasPrefix(value, "#") {
		return ""
	}
	for _, marker := range []string{" #", "\t#"} {
		if index := strings.Index(value, marker); index >= 0 {
			value = value[:index]
		}
	}
	return strings.TrimSpace(value)
}

// KephalaionCommitHint ist der Satz nach jedem Eintragen: Die Datei gehört
// zum Repo des Projekts.
const KephalaionCommitHint = ConfigDisplayName + " ist versioniert: Die Änderung will committet werden."

// kephalaionBlockComment steht im neu angelegten Block unter kephalaion:,
// eingerückt — wie bei docs:, damit es zum Block gehört.
const kephalaionBlockComment = `  # Anbindung an die Collection dieses Projekts in Kephalaion: der Alias des
  # Hubs (auf jedem Rechner gleich) und die Collection. Der Account gehört
  # zum Rechner und kommt von Kephalaion (kephalaion node client).
`

// KephalaionBlockResult ist die Antwort von SetKephalaionBinding.
type KephalaionBlockResult struct {
	// Config ist der Pfad der Konfiguration.
	Config string `json:"config"`
	// Previous ist der Block vorher.
	Previous ConfigKephalaion `json:"previous"`
	// Changed sagt, ob die Datei geschrieben wurde; ohne Änderung bleibt sie
	// byte-gleich.
	Changed bool `json:"changed"`
}

// SetKephalaionBinding trägt hub und collection in den Block kephalaion: von
// K-PLAYBOOK.yaml ein — die Fachlogik hinter dem Einrichtungsschritt der
// Oberfläche und dem Umzug in k-playbook upgrade. Zeilenweise wie
// AddDocsExclude: Fehlt der Block, wird er samt Kommentar angehängt; steht er
// da, werden nur die Zeilen hub: und collection: ersetzt (ein Kommentar
// dahinter bleibt) oder, wenn eine fehlt, hinter der letzten eingerückten
// Zeile des Blocks eingefügt. Jede andere Zeile — andere Blöcke, Kommentare,
// ein account: — bleibt byte-gleich; ein Account wird nie geschrieben. Eine
// Form, die sich so nicht sicher ändern lässt (Block mehrfach, Wert in der
// Kopfzeile, hub: oder collection: mehrfach), ist ein Fehler, und nichts
// wird geschrieben.
//
// Ob die Anbindung trägt, prüft der Aufrufer vorher (kephalaion.CheckBinding);
// SetKephalaionBinding prüft nur die Namen.
func SetKephalaionBinding(projectDir, hub, collection string) (KephalaionBlockResult, error) {
	file := ConfigPath(projectDir)
	result := KephalaionBlockResult{Config: file}
	if err := kephalaion.ValidateBinding(kephalaion.Binding{Hub: hub, Collection: collection}); err != nil {
		return result, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return result, err
	}
	content := string(data)
	result.Previous = parseKephalaionBlock(content)
	updated, err := setKephalaionBlock(content, hub, collection)
	if err != nil {
		return result, err
	}
	if updated == content {
		return result, nil
	}
	if err := os.WriteFile(file, []byte(updated), 0o644); err != nil {
		return result, err
	}
	result.Changed = true
	return result, nil
}

// kephalaionBlockUnsupported ist der Fehler für eine Form, die
// SetKephalaionBinding nicht sicher ändern kann.
func kephalaionBlockUnsupported(line int, reason string) error {
	return fmt.Errorf("%s, Zeile %d: %s — nichts eingetragen; den Block kephalaion: von Hand auf je eine Zeile "+
		"hub: und collection: umstellen, dann erneut eintragen", ConfigDisplayName, line+1, reason)
}

// setKephalaionBlock setzt hub und collection im Text der Konfiguration, wie
// SetKephalaionBinding es beschreibt.
func setKephalaionBlock(content, hub, collection string) (string, error) {
	lines := strings.Split(content, "\n")
	start := -1
	for index, line := range lines {
		if !isTopLevelYAMLLine(line) {
			continue
		}
		key, value, _ := strings.Cut(strings.TrimSpace(strings.TrimSuffix(line, "\r")), ":")
		if strings.TrimSpace(key) != KephalaionBlockName {
			continue
		}
		if start >= 0 {
			return "", kephalaionBlockUnsupported(index, "kephalaion: steht mehrfach auf oberster Ebene")
		}
		if value = strings.TrimSpace(value); value != "" && !strings.HasPrefix(value, "#") {
			return "", kephalaionBlockUnsupported(index, "kephalaion: trägt einen Wert in der Kopfzeile")
		}
		start = index
	}
	if start < 0 {
		block := "kephalaion:\n" + kephalaionBlockComment + "  hub: " + hub + "\n  collection: " + collection + "\n"
		return replaceTopLevelBlock(content, KephalaionBlockName, block), nil
	}

	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if isTopLevelYAMLLine(lines[index]) {
			end = index
			break
		}
	}
	childIndent := -1
	lastIndented := start
	found := map[string]int{}
	for index := start + 1; index < end; index++ {
		line := strings.TrimSuffix(lines[index], "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && lineIndent(line) > 0 {
			lastIndented = index
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := lineIndent(line)
		if childIndent < 0 {
			childIndent = indent
		}
		if indent != childIndent {
			continue
		}
		key, _, ok := strings.Cut(trimmed, ":")
		key = strings.TrimSpace(key)
		if !ok || (key != "hub" && key != "collection") {
			continue
		}
		if _, twice := found[key]; twice {
			return "", kephalaionBlockUnsupported(index, key+": steht mehrfach im Block kephalaion:")
		}
		found[key] = index
	}
	if childIndent < 0 {
		childIndent = 2
	}

	eol := ""
	if strings.HasSuffix(lines[start], "\r") {
		eol = "\r"
	}
	var missing []string
	for _, entry := range []struct{ key, value string }{{"hub", hub}, {"collection", collection}} {
		index, ok := found[entry.key]
		if !ok {
			missing = append(missing, strings.Repeat(" ", childIndent)+entry.key+": "+entry.value+eol)
			continue
		}
		lines[index] = replaceScalarLine(lines[index], entry.key, entry.value)
	}
	if len(missing) > 0 {
		at := lastIndented + 1
		result := append([]string{}, lines[:at]...)
		result = append(result, missing...)
		lines = append(result, lines[at:]...)
	}
	return strings.Join(lines, "\n"), nil
}

// replaceScalarLine setzt in einer Zeile `<einrückung><key>: <wert>` den Wert;
// ein Kommentar dahinter — auch hinter einem Wert in Anführungszeichen — und
// ein CR am Ende bleiben. Ist der Wert schon derselbe, bleibt die Zeile
// byte-gleich.
func replaceScalarLine(line, key, value string) string {
	body, crlf := strings.CutSuffix(line, "\r")
	_, rest, _ := strings.Cut(body, ":")
	if configScalar(rest) == value {
		return line
	}
	_, comment := splitValueComment(rest)
	updated := body[:lineIndent(body)] + key + ": " + value + comment
	if crlf {
		updated += "\r"
	}
	return updated
}

// splitValueComment teilt, was in einer Zeile hinter `<key>:` steht, in den
// Wert und einen Kommentar dahinter; der Kommentar trägt den Abstand davor,
// damit er beim Ersetzen bleibt, wie er war. Ein Kommentar beginnt mit # nach
// Leerraum, aber nicht innerhalb eines Werts in Anführungszeichen: In "…"
// maskiert ein Backslash das nächste Zeichen, in '…' stehen zwei Apostrophe
// für eines. Schließt das Anführungszeichen nicht in derselben Zeile, gibt es
// keinen Kommentar.
func splitValueComment(rest string) (value, comment string) {
	from := len(rest) - len(strings.TrimLeft(rest, " \t"))
	if from < len(rest) && (rest[from] == '"' || rest[from] == '\'') {
		end := closingQuote(rest, from)
		if end < 0 {
			return rest, ""
		}
		from = end + 1
	}
	for index := from; index < len(rest); index++ {
		if rest[index] != '#' || index == 0 || (rest[index-1] != ' ' && rest[index-1] != '\t') {
			continue
		}
		for index > 0 && (rest[index-1] == ' ' || rest[index-1] == '\t') {
			index--
		}
		return rest[:index], rest[index:]
	}
	return rest, ""
}

// closingQuote liefert die Stelle des Anführungszeichens, das den Wert ab
// open schließt, oder -1.
func closingQuote(text string, open int) int {
	quote := text[open]
	for index := open + 1; index < len(text); index++ {
		switch {
		case quote == '"' && text[index] == '\\':
			index++
		case text[index] != quote:
		case quote == '\'' && index+1 < len(text) && text[index+1] == '\'':
			index++
		default:
			return index
		}
	}
	return -1
}
