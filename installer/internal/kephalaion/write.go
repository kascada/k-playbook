package kephalaion

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// IgnoreLine ist der Eintrag, den WriteBinding in k-playbook-local/.gitignore
// ergänzt, wenn Git die Datei sonst nicht ignorierte.
const IgnoreLine = "/" + LocalFileName

// gitTimeout begrenzt jeden Git-Aufruf beim Schreiben.
const gitTimeout = 5 * time.Second

// ErrTracked meldet, dass LocalFileName schon im Git-Index steht. Dann wird
// nicht geschrieben: Die Datei würde sonst mitversioniert.
var ErrTracked = errors.New(LocalFileName + " steht im Git-Index und würde mitversioniert; " +
	"erst mit git rm --cached " + LocalFileName + " aus dem Index nehmen (die Datei bleibt liegen)")

// localHeader ist der Kopf einer neu angelegten Datei.
const localHeader = `# k-playbook — Einstellungen nur für diesen Rechner
#
# Ergänzt K-PLAYBOOK.yaml um das, was auf jedem Rechner anders ist.
# Die Datei wird nicht versioniert (k-playbook-local/.gitignore); ein
# anderer Rechner mit demselben Repo hat seine eigene.
#
# Kein Geheimnis hier: das Token bleibt in Kephalaions eigener Datei
# ~/.config/kephalaion/tokens/<hub>/<account>.token.
`

// bindingBlock ist der Block kephalaion: mit seinen Kommentaren.
func bindingBlock(b Binding) string {
	return fmt.Sprintf(`kephalaion:
  # Alias des Hub-Eintrags am Node dieses Rechners (kephalaion status).
  hub: %s
  # Collection dieses Projekts am Hub; Adresse für Clients: %s.
  collection: %s
  # Account dieses Rechners; ein Account gehört auf genau einen Rechner.
  account: %s
`, b.Hub, b.Address(), b.Collection, b.Account)
}

// WriteBinding schreibt den Block kephalaion: nach LocalFileName unter
// localDir. Ersetzt wird nur dieser Block; Kopfkommentar und übrige Blöcke
// bleiben, wie sie sind. Fehlt die Datei, entsteht sie mit Kopf.
//
// Vorher stellt WriteBinding sicher, dass Git die Datei ignoriert — gemessen
// mit git check-ignore, nicht an einer bestimmten Zeile. Nur wenn nicht, wird
// IgnoreLine in localDir/.gitignore ergänzt (die Datei entsteht bei Bedarf);
// das ist die einzige Schreibstelle außerhalb von LocalFileName. Ohne Git
// oder ohne Git-Repo entfällt der Schritt; scheitert Git aus einem anderen
// Grund, wird nichts geschrieben. Steht die Datei schon im Index, wird nichts
// geschrieben: ErrTracked.
//
// WriteBinding prüft nur die Namen. Ob die Anbindung trägt, prüft der
// Aufrufer vorher mit CheckBinding — eingetragen wird nur nach bestandenem
// Test.
func WriteBinding(localDir string, b Binding) error {
	if err := ValidateBinding(b); err != nil {
		return err
	}
	info, err := os.Stat(localDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("Verzeichnis %s fehlt", localDir)
	}
	if err := ensureIgnored(localDir); err != nil {
		return err
	}

	path := LocalFilePath(localDir)
	mode := fs.FileMode(0o644)
	content := ""
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		content = string(data)
		if st, err := os.Stat(path); err == nil {
			mode = st.Mode().Perm()
		}
	case errors.Is(err, fs.ErrNotExist):
		content = localHeader
	default:
		return fmt.Errorf("%s nicht lesbar: %w", LocalFileName, err)
	}
	updated := replaceBlock(content, BlockName, bindingBlock(b))
	return writeAtomic(path, []byte(updated), mode)
}

// writeAtomic schreibt über eine temporäre Datei im selben Verzeichnis und
// benennt sie um: Eine halb geschriebene Datei bleibt nie zurück.
func writeAtomic(path string, data []byte, mode fs.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// replaceBlock ersetzt einen Block auf oberster Ebene: von der Zeile
// "<name>:" bis vor die nächste Zeile, die in Spalte 0 beginnt — auch ein
// Kommentar dort gehört schon zum Folgenden. Leerzeilen am Ende des Blocks
// bleiben stehen. Fehlt der Block, wird er mit einer Leerzeile angehängt.
func replaceBlock(content, name, block string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		if line == name+":" || strings.HasPrefix(line, name+": ") || strings.HasPrefix(line, name+":\t") ||
			strings.HasPrefix(line, name+":#") {
			start = i
			break
		}
	}
	blockLines := strings.Split(strings.TrimRight(block, "\n"), "\n")
	if start == -1 {
		trimmed := strings.TrimRight(content, "\n")
		if trimmed == "" {
			return strings.Join(blockLines, "\n") + "\n"
		}
		return trimmed + "\n\n" + strings.Join(blockLines, "\n") + "\n"
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if line != "" && line[0] != ' ' && line[0] != '\t' {
			end = i
			break
		}
	}
	for end > start+1 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	result := append([]string{}, lines[:start]...)
	result = append(result, blockLines...)
	result = append(result, lines[end:]...)
	return strings.TrimRight(strings.Join(result, "\n"), "\n") + "\n"
}

// git ruft git in dir auf und liefert Exit-Code und Ausgabe. -1 heißt: git
// ließ sich nicht starten.
func git(dir string, args ...string) (int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), string(out)
	}
	return -1, string(out)
}

// ensureIgnored stellt sicher, dass Git LocalFileName unter localDir
// ignoriert, siehe WriteBinding.
func ensureIgnored(localDir string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}
	inRepo, err := insideWorkTree(localDir)
	if err != nil || !inRepo {
		return err
	}
	switch code, out := git(localDir, "ls-files", "--error-unmatch", "--", LocalFileName); code {
	case 0:
		return ErrTracked
	case 1:
	default:
		return fmt.Errorf("git ls-files gescheitert (%s); nichts geschrieben", oneLine(out))
	}
	ignored, err := isIgnored(localDir)
	if err != nil || ignored {
		return err
	}
	if err := appendIgnoreLine(filepath.Join(localDir, ".gitignore")); err != nil {
		return fmt.Errorf(".gitignore nicht ergänzt: %w", err)
	}
	ignored, err = isIgnored(localDir)
	if err != nil {
		return err
	}
	if !ignored {
		return fmt.Errorf("Git ignoriert %s trotz %s in .gitignore nicht (eine Ausnahme mit ! hebt den Eintrag auf); nichts geschrieben", LocalFileName, IgnoreLine)
	}
	return nil
}

// insideWorkTree fragt git rev-parse, ob localDir in einem Arbeitsverzeichnis
// liegt. „Kein Repo“ gilt nur, wenn Git das eindeutig sagt: Antwort false
// oder „not a git repository“. Jeder andere Fehlschlag — etwa ein Repo, das
// Git wegen safe.directory ablehnt („dubious ownership“), oder eine
// Zeitüberschreitung — ist ein Fehler: Sonst entstünde LocalFileName ohne
// Ignore-Eintrag in einem Repo, das Git nur gerade nicht lesen mag.
func insideWorkTree(localDir string) (bool, error) {
	code, out := git(localDir, "rev-parse", "--is-inside-work-tree")
	switch {
	case code == 0 && strings.TrimSpace(out) == "true":
		return true, nil
	case code == 0 && strings.TrimSpace(out) == "false":
		return false, nil
	case code != 0 && strings.Contains(out, "not a git repository"):
		return false, nil
	case strings.Contains(out, "dubious ownership"):
		return false, fmt.Errorf("Git lehnt das Repo ab, weil es einem anderen Benutzer gehört (safe.directory); "+
			"ob Git %s ignoriert, lässt sich so nicht prüfen, nichts geschrieben (%s)", LocalFileName, oneLine(out))
	default:
		return false, fmt.Errorf("git rev-parse gescheitert (%s); ob Git %s ignoriert, lässt sich nicht prüfen, nichts geschrieben",
			oneLine(out), LocalFileName)
	}
}

// oneLine fasst eine Ausgabe von Git für eine Meldung auf eine Zeile
// zusammen; ohne Ausgabe steht dort „ohne Ausgabe“.
func oneLine(out string) string {
	text := strings.Join(strings.Fields(out), " ")
	if text == "" {
		return "ohne Ausgabe"
	}
	return text
}

// isIgnored fragt git check-ignore: 0 ignoriert, 1 nicht, alles andere ist
// ein Fehler.
func isIgnored(localDir string) (bool, error) {
	code, out := git(localDir, "check-ignore", "-q", "--", LocalFileName)
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("git check-ignore gescheitert: %s", strings.TrimSpace(out))
	}
}

// appendIgnoreLine hängt IgnoreLine samt Kommentar an eine .gitignore an
// oder legt sie damit an.
func appendIgnoreLine(path string) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	content := string(data)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	content += "# Einstellungen nur für diesen Rechner, etwa der eigene Kephalaion-Account\n" + IgnoreLine + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}
