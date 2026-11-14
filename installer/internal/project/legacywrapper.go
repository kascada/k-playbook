package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// RemoveLegacyWrapper entfernt die Wrapper-Datei des abgelösten Modells aus
// dem früheren Clone k-playbook/ des Projekts und meldet ihren Pfad zurück; ist
// sie nicht da, bleibt der Rückgabewert leer.
//
// Das alte Modell legte den Wrapper unter <projekt>/k-playbook/bin/k-playbook
// ab. Das Quell-Repo kennt die Datei nicht mehr, und .gitignore deckt sie
// nicht ab — sie bleibt sonst als zusätzliche Datei liegen.
//
// Frühere Versionen haben k-playbook/ den Schreibzugriff entzogen. Deshalb
// gibt der Aufruf vor dem Löschen das Verzeichnis des Wrappers frei, und nur
// dieses; wieder gesperrt wird nichts.
//
// Ist k-playbook oder k-playbook/bin kein Verzeichnis, sondern eine Datei —
// etwa ein in der Projektwurzel abgelegtes Programm —, gibt es den Wrapper
// ebenfalls nicht: kein Fehler, und die Datei bleibt unberührt.
func RemoveLegacyWrapper(projectDir string) (removed string, err error) {
	path := filepath.Join(PlaybookDir(projectDir), legacyWrapperTail)
	if _, statErr := os.Lstat(path); statErr != nil {
		if errors.Is(notExistForFileAsDir(statErr), fs.ErrNotExist) {
			return "", nil
		}
		return "", statErr
	}

	if writableErr := makeOwnerWritable(filepath.Dir(path)); writableErr != nil {
		return "", fmt.Errorf("Verzeichnis des Wrappers beschreibbar machen: %w", writableErr)
	}
	if removeErr := os.Remove(path); removeErr != nil {
		return "", removeErr
	}
	return path, nil
}

// makeOwnerWritable gibt dem Eigentümer das Schreibrecht auf dir, wenn es
// fehlt. Löschen braucht das Recht am Verzeichnis, nicht an der Datei.
func makeOwnerWritable(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	mode := info.Mode().Perm()
	if mode&0o200 != 0 {
		return nil
	}
	return os.Chmod(dir, mode|0o200)
}
