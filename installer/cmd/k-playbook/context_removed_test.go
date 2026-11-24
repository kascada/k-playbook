package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Das Unterkommando context ist mit Task 188 entfernt. Wer es noch aufruft —
// etwa eine Sitzung in einem Projekt mit altem Anstoß in AGENTS.md —, liest
// auf stderr in einem Satz, was an seine Stelle tritt, und bekommt Exit-Code 1.
// stdout bleibt leer, die Hilfe erscheint nicht, und im Projekt entsteht
// nichts.
func TestContextKommandoIstEntfernt(t *testing.T) {
	root := newProject(t)
	before := listTree(t, root)

	raw, err := json.Marshal([]string{"context"})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), mainArgsEnv+"="+string(raw))
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	var exit *exec.ExitError
	if !errors.As(runErr, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("Exit = %v, erwartet Exit-Code 1\nstderr:\n%s", runErr, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout ist nicht leer:\n%s", stdout.String())
	}
	want := "Fehler: " + errContextRemoved.Error() + "\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, erwartet %q", stderr.String(), want)
	}
	for _, part := range []string{"context ist entfernt", "k-playbook briefing", "k-playbook briefing --part environment"} {
		if !strings.Contains(stderr.String(), part) {
			t.Errorf("Meldung nennt %q nicht: %q", part, stderr.String())
		}
	}
	if after := listTree(t, root); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("der Aufruf hat %s verändert:\nvorher:  %v\nnachher: %v", root, before, after)
	}

	// Die Hilfe nennt das Kommando nicht mehr.
	if strings.Contains(usageText(t), "\n  context ") {
		t.Error("printUsage nennt context noch")
	}
}

// usageText fängt die Hilfe ab, die printUsage nach stderr schreibt.
func usageText(t *testing.T) string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stderr
	os.Stderr = write
	printUsage()
	os.Stderr = original
	write.Close()
	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(read); err != nil {
		t.Fatal(err)
	}
	read.Close()
	return buffer.String()
}
