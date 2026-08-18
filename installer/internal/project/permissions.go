package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// SetInstallationReadOnly entzieht der Installation die Schreibrechte. Die
// Installation ist ein Vendor-Clone: geändert wird sie nur während eines
// Updates oder beim gezielten Einspielen des Entwicklungsstands.
func SetInstallationReadOnly(projectDir string) error {
	return chmodInstallation(projectDir, func(mode fs.FileMode) fs.FileMode {
		return mode &^ 0o222
	})
}

// setInstallationWritable macht die Installation für den Eigentümer temporär
// beschreibbar. Das Gegenstück muss immer per defer laufen.
func setInstallationWritable(projectDir string) error {
	return chmodInstallation(projectDir, func(mode fs.FileMode) fs.FileMode {
		return mode | 0o200
	})
}

func chmodInstallation(projectDir string, transform func(fs.FileMode) fs.FileMode) error {
	dir := PlaybookDir(projectDir)
	if !isDir(dir) {
		return nil
	}

	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		current := info.Mode().Perm()
		wanted := transform(current)
		if wanted == current {
			return nil
		}
		return os.Chmod(path, wanted)
	})
}

func keepInstallationReadOnly(projectDir string, errp *error) {
	if protectErr := SetInstallationReadOnly(projectDir); protectErr != nil {
		if *errp != nil {
			*errp = errors.Join(*errp, fmtReadOnlyError(protectErr))
			return
		}
		*errp = fmtReadOnlyError(protectErr)
	}
}

func fmtReadOnlyError(err error) error {
	return fmt.Errorf("Installation konnte nicht wieder read-only gesetzt werden: %w", err)
}
