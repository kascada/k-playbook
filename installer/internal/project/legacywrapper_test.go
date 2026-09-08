package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Die Datei liegt im Clone und die Installation ist read-only: das Entfernen
// muss trotzdem gelingen, und der Schutz muss danach wieder stehen.
func TestRemoveLegacyWrapperEntferntDatei(t *testing.T) {
	projectDir := t.TempDir()
	t.Cleanup(func() {
		_ = setInstallationWritable(projectDir)
	})
	wrapper := filepath.Join(PlaybookDir(projectDir), legacyWrapperTail)
	if err := os.MkdirAll(filepath.Dir(wrapper), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("Wrapper anlegen: %v", err)
	}
	if err := SetInstallationReadOnly(projectDir); err != nil {
		t.Fatalf("Installation schützen: %v", err)
	}

	removed, err := RemoveLegacyWrapper(projectDir)
	if err != nil {
		t.Fatalf("RemoveLegacyWrapper: %v", err)
	}
	if removed != wrapper {
		t.Errorf("removed = %q, erwartet %q", removed, wrapper)
	}
	if _, statErr := os.Lstat(wrapper); statErr == nil {
		t.Error("Wrapper liegt noch da")
	}

	info, err := os.Stat(PlaybookDir(projectDir))
	if err != nil {
		t.Fatalf("Installation lesen: %v", err)
	}
	if info.Mode().Perm()&0o222 != 0 {
		t.Errorf("Installation ist nach dem Entfernen beschreibbar: %v", info.Mode().Perm())
	}
}

// Ohne die Datei ist nichts zu tun: kein Fehler, keine Meldung — und die
// Installation wird auch nicht angefasst.
func TestRemoveLegacyWrapperOhneDatei(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.MkdirAll(PlaybookDir(projectDir), 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}

	removed, err := RemoveLegacyWrapper(projectDir)
	if err != nil {
		t.Fatalf("RemoveLegacyWrapper: %v", err)
	}
	if removed != "" {
		t.Errorf("removed = %q, erwartet leer", removed)
	}

	var mode fs.FileMode
	info, statErr := os.Stat(PlaybookDir(projectDir))
	if statErr != nil {
		t.Fatalf("Installation lesen: %v", statErr)
	}
	mode = info.Mode().Perm()
	if mode&0o200 == 0 {
		t.Errorf("Installation wurde ohne Anlass read-only gesetzt: %v", mode)
	}
}
