package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// RemoveLegacyWrapper entfernt die Wrapper-Datei des abgelösten Modells aus
// der Installation und meldet ihren Pfad zurück; ist sie nicht da, bleibt der
// Rückgabewert leer.
//
// Das alte Modell legte den Wrapper unter <projekt>/k-playbook/bin/k-playbook
// ab. Das Quell-Repo kennt die Datei nicht mehr, und .gitignore deckt sie
// nicht ab — sie bleibt sonst als zusätzliche Datei im Clone liegen.
//
// Die Installation ist read-only; sie wird für den Löschvorgang kurz
// beschreibbar gemacht und danach per defer wieder geschützt.
func RemoveLegacyWrapper(projectDir string) (removed string, err error) {
	path := filepath.Join(PlaybookDir(projectDir), legacyWrapperTail)
	if _, statErr := os.Lstat(path); statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return "", nil
		}
		return "", statErr
	}

	if writableErr := setInstallationWritable(projectDir); writableErr != nil {
		return "", fmt.Errorf("Installation beschreibbar machen: %w", writableErr)
	}
	defer keepInstallationReadOnly(projectDir, &err)

	if removeErr := os.Remove(path); removeErr != nil {
		return "", removeErr
	}
	return path, nil
}
