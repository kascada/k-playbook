package kephalaion

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Altbestand (Task 138): Bis zur Auskunft kephalaion node client stand die
// Anbindung rechnerlokal in k-playbook-local/K-PLAYBOOK.local.yaml, Block
// kephalaion: mit Hub, Collection und Account. Kein Programmteil liest die
// Anbindung mehr von dort. Gelesen wird die Datei nur noch, um den Altbestand
// zu erkennen: Die Gesundheitsprüfung warnt, und der Projektschritt von
// k-playbook upgrade zieht Hub und Collection in den Block kephalaion: von
// .k-playbook/K-PLAYBOOK.yaml um.

// LegacyFileName ist die frühere rechnerlokale Datei unter k-playbook-local/.
const LegacyFileName = "K-PLAYBOOK.local.yaml"

// legacyBlock ist der Block in LegacyFileName, der die Anbindung trug.
const legacyBlock = "kephalaion"

// LegacyBinding ist der Inhalt des Blocks kephalaion: aus LegacyFileName.
type LegacyBinding struct {
	Hub        string `json:"hub"`
	Collection string `json:"collection"`
	Account    string `json:"account"`
}

// LegacyFilePath ist der Ort von LegacyFileName unter localDir.
func LegacyFilePath(localDir string) string {
	return filepath.Join(localDir, LegacyFileName)
}

// ReadLegacyBinding liest den Block kephalaion: aus LegacyFileName unter
// localDir.
//
// present ist false, wenn die Datei oder der Block fehlt oder der Block
// keinen der drei Werte trägt. Ein Fehler heißt: Die Datei ist da, lässt sich
// aber nicht lesen oder nicht als YAML verstehen; er zitiert höchstens
// Zeilennummer und Art, nie den Inhalt. Die Namen prüft ReadLegacyBinding
// nicht.
func ReadLegacyBinding(localDir string) (binding LegacyBinding, present bool, err error) {
	data, err := os.ReadFile(LegacyFilePath(localDir))
	if errors.Is(err, fs.ErrNotExist) {
		return LegacyBinding{}, false, nil
	}
	if err != nil {
		return LegacyBinding{}, false, fmt.Errorf("%s nicht lesbar: %w", LegacyFileName, err)
	}
	root, err := yamllite.Parse(data)
	if err != nil {
		return LegacyBinding{}, false, fmt.Errorf("%s ist kein lesbares YAML (%s)", LegacyFileName, describeYAMLError(err))
	}
	block := root.Get(legacyBlock)
	if block == nil {
		return LegacyBinding{}, false, nil
	}
	binding = LegacyBinding{
		Hub:        strings.TrimSpace(block.Get("hub").Str()),
		Collection: strings.TrimSpace(block.Get("collection").Str()),
		Account:    strings.TrimSpace(block.Get("account").Str()),
	}
	if binding == (LegacyBinding{}) {
		return LegacyBinding{}, false, nil
	}
	return binding, true, nil
}
