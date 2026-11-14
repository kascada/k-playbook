package project

import (
	"os"
	"path/filepath"
	"testing"
)

// Frühere Versionen haben k-playbook/ den Schreibzugriff entzogen: Das
// Entfernen muss trotzdem gelingen. Freigegeben wird nur das Verzeichnis des
// Wrappers, und wieder gesperrt wird nichts.
func TestRemoveLegacyWrapperEntferntDatei(t *testing.T) {
	projectDir := t.TempDir()
	playbookDir := PlaybookDir(projectDir)
	wrapper := filepath.Join(playbookDir, legacyWrapperTail)
	binDir := filepath.Dir(wrapper)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("Wrapper anlegen: %v", err)
	}
	// Der Schreibschutz, wie ihn frühere Versionen gesetzt haben.
	for _, path := range []string{wrapper, binDir, playbookDir} {
		if err := os.Chmod(path, 0o555); err != nil {
			t.Fatalf("%s sperren: %v", path, err)
		}
	}
	t.Cleanup(func() {
		_ = os.Chmod(playbookDir, 0o755)
		_ = os.Chmod(binDir, 0o755)
	})

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

	if mode := dirMode(t, binDir); mode&0o200 == 0 {
		t.Errorf("Verzeichnis des Wrappers ist wieder gesperrt: %v", mode)
	}
	if mode := dirMode(t, playbookDir); mode&0o200 != 0 {
		t.Errorf("k-playbook/ wurde über das Verzeichnis des Wrappers hinaus freigegeben: %v", mode)
	}
}

// Ohne die Datei ist nichts zu tun: kein Fehler, keine Meldung — und die
// Rechte bleiben, wie sie sind.
func TestRemoveLegacyWrapperOhneDatei(t *testing.T) {
	projectDir := t.TempDir()
	playbookDir := PlaybookDir(projectDir)
	if err := os.MkdirAll(playbookDir, 0o755); err != nil {
		t.Fatalf("Verzeichnis anlegen: %v", err)
	}

	removed, err := RemoveLegacyWrapper(projectDir)
	if err != nil {
		t.Fatalf("RemoveLegacyWrapper: %v", err)
	}
	if removed != "" {
		t.Errorf("removed = %q, erwartet leer", removed)
	}
	if mode := dirMode(t, playbookDir); mode != 0o755 {
		t.Errorf("Rechte von k-playbook/ = %v, erwartet unverändert 0755", mode)
	}
}

// Liegt unter dem Namen k-playbook eine reguläre Datei in der Projektwurzel,
// etwa ein dort abgelegtes Programm, gibt es keinen Wrapper: kein Fehler, keine
// Meldung, und die Datei bleibt, wie sie ist. Das System meldet für den Pfad
// darunter ENOTDIR, nicht „gibt es nicht“.
func TestRemoveLegacyWrapperPlaybookIstDatei(t *testing.T) {
	projectDir := t.TempDir()
	playbookFile := PlaybookDir(projectDir)
	content := []byte("kein Verzeichnis\n")
	if err := os.WriteFile(playbookFile, content, 0o755); err != nil {
		t.Fatalf("Datei anlegen: %v", err)
	}

	removed, err := RemoveLegacyWrapper(projectDir)
	if err != nil {
		t.Fatalf("RemoveLegacyWrapper: %v", err)
	}
	if removed != "" {
		t.Errorf("removed = %q, erwartet leer", removed)
	}

	got, readErr := os.ReadFile(playbookFile)
	if readErr != nil {
		t.Fatalf("Datei lesen: %v", readErr)
	}
	if string(got) != string(content) {
		t.Errorf("Inhalt der Datei = %q, erwartet unverändert %q", got, content)
	}
	if mode := dirMode(t, playbookFile); mode != 0o755 {
		t.Errorf("Rechte der Datei = %v, erwartet unverändert 0755", mode)
	}
}

func dirMode(t *testing.T, dir string) os.FileMode {
	t.Helper()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("%s lesen: %v", dir, err)
	}
	return info.Mode().Perm()
}
