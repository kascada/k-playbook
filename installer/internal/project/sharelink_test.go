package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func readlink(t *testing.T, path string) string {
	t.Helper()
	destination, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return destination
}

// Mitgelieferte Einträge zeigen relativ durch .k-playbook/share, der Link
// selbst absolut auf das mitgelieferte Verzeichnis, und .gitignore nimmt nur
// ihn aus.
func TestApplyLinksVerlinktDurchShare(t *testing.T) {
	root := newProject(t)
	writeFile(t, filepath.Join(root, LocalDirName, "commands", "k-eigen.md"), "eigen\n")

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if !LinksOK(statuses) {
		t.Fatalf("nicht eingerichtet: %+v", statuses)
	}

	if got := readlink(t, ShareLink(root)); got != shipped(t) {
		t.Errorf("share zeigt auf %q, erwartet %q", got, shipped(t))
	}
	cases := map[string]string{
		filepath.Join(".claude", "commands", "k-test.md"):             "../../.k-playbook/share/commands/k-test.md",
		filepath.Join(".claude", "commands", "_shared", "geteilt.md"): "../../../.k-playbook/share/commands/_shared/geteilt.md",
		filepath.Join(".opencode", "commands", "k-test.md"):           "../../.k-playbook/share/commands/k-test.md",
		filepath.Join(".claude", "skills", "beispiel"):                "../../.k-playbook/share/skills/beispiel",
		filepath.Join(".claude", "commands", "k-eigen.md"):            "../../k-playbook-local/commands/k-eigen.md",
	}
	for rel, want := range cases {
		if got := readlink(t, filepath.Join(root, rel)); got != filepath.FromSlash(want) {
			t.Errorf("%s zeigt auf %q, erwartet %q", rel, got, want)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, ".claude", "commands", "k-test.md"))
	if err != nil || string(data) != "test\n" {
		t.Errorf("k-test.md durch den Link: %q, %v", data, err)
	}
	if !shareIgnored(root) {
		t.Error(".k-playbook/.gitignore nimmt share nicht aus")
	}
}

// Die Links aus der Zeit vor Stufe 2 zeigen in den Clone. Sie gehören uns:
// was noch im Katalog steht, wird umgesetzt, was nicht, verschwindet.
func TestHealLinksSetztCloneLinksUm(t *testing.T) {
	root := newProject(t)
	writeFile(t, filepath.Join(root, PlaybookDirName, "commands", "k-test.md"), "alt\n")
	writeFile(t, filepath.Join(root, PlaybookDirName, "commands", "k-weg.md"), "alt\n")
	commands := filepath.Join(root, ".claude", "commands")
	if err := os.MkdirAll(commands, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"k-test.md", "k-weg.md"} {
		if err := os.Symlink(filepath.Join("..", "..", PlaybookDirName, "commands", name), filepath.Join(commands, name)); err != nil {
			t.Fatal(err)
		}
	}

	status := statusFor(t, CheckLinks(root), claudeCommands())
	if !slices.Contains(status.Wrong, "k-test.md") || !slices.Contains(status.Stale, "k-weg.md") {
		t.Fatalf("Wrong = %v, Stale = %v", status.Wrong, status.Stale)
	}

	repair := HealLinks(root)
	if !repair.Applied || repair.Error != "" {
		t.Fatalf("Heilung: %+v", repair)
	}
	if got := readlink(t, filepath.Join(commands, "k-test.md")); !strings.Contains(got, ".k-playbook") {
		t.Errorf("k-test.md zeigt weiter auf %q", got)
	}
	if _, err := os.Lstat(filepath.Join(commands, "k-weg.md")); !os.IsNotExist(err) {
		t.Errorf("k-weg.md ist noch da (%v)", err)
	}
}

// Ein Link auf ein anderes Home — Host und DevContainer teilen das Projekt —
// wird auf das eigene umgesetzt; der Rest der .gitignore bleibt.
func TestHealLinksSetztFremdenShareUm(t *testing.T) {
	root := newProject(t)
	if _, err := ApplyLinks(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(ShareLink(root)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/home/anderer/.local/share/k-playbook", ShareLink(root)); err != nil {
		t.Fatal(err)
	}
	ignore := filepath.Join(root, ProjectDirName, ".gitignore")
	if err := os.WriteFile(ignore, []byte("/eigenes"), 0o644); err != nil {
		t.Fatal(err)
	}

	status := statusFor(t, CheckLinks(root), shareLinkRel())
	if status.State != StateStale {
		t.Fatalf("State = %q (%s), erwartet %q", status.State, status.Detail, StateStale)
	}
	if repair := HealLinks(root); !repair.Applied || repair.Error != "" {
		t.Fatalf("Heilung: %+v", repair)
	}
	if got := readlink(t, ShareLink(root)); got != shipped(t) {
		t.Errorf("share zeigt auf %q", got)
	}
	data, err := os.ReadFile(ignore)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "/eigenes\n/share\n" {
		t.Errorf(".gitignore = %q", data)
	}
}

// Ein echtes Verzeichnis an der Stelle des Links gehört dem Projekt.
func TestShareLinkBlockiertDurchVerzeichnis(t *testing.T) {
	root := newProject(t)
	if err := os.MkdirAll(ShareLink(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyLinks(root); err != nil {
		t.Fatal(err)
	}
	status := statusFor(t, CheckLinks(root), shareLinkRel())
	if status.State != StateBlocked {
		t.Fatalf("State = %q, erwartet %q", status.State, StateBlocked)
	}
	if info, err := os.Lstat(ShareLink(root)); err != nil || !info.IsDir() {
		t.Errorf("Verzeichnis angefasst: %v", err)
	}
}

// Ohne mitgeliefertes Verzeichnis gibt es nichts, worauf der Link zeigen
// könnte; projekteigene Einträge kommen trotzdem an.
func TestShareLinkOhneMitgeliefertes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, LocalDirName, "commands", "k-eigen.md"), "eigen\n")

	statuses, err := ApplyLinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if status := statusFor(t, statuses, shareLinkRel()); status.State != StateNoSource {
		t.Errorf("State = %q, erwartet %q", status.State, StateNoSource)
	}
	if _, err := os.Lstat(ShareLink(root)); !os.IsNotExist(err) {
		t.Errorf("Link angelegt (%v)", err)
	}
	if got := readlink(t, filepath.Join(root, ".claude", "commands", "k-eigen.md")); !strings.Contains(got, LocalDirName) {
		t.Errorf("k-eigen.md zeigt auf %q", got)
	}
}
