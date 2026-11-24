package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Quellstand in der Merkdatei (Task 195): checkout hält Branch, Commit und
// die Prüfsummen der geänderten Dateien fest; checkout bei existing, der
// Prüfmodus und publish vergleichen damit.

// docsStandGit führt git im Projekt aus.
func docsStandGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

// docsStandFixture ist ein Projekt in einem git-Arbeitsbaum mit einem Commit
// und einem Dokument in der Collection.
func docsStandFixture(t *testing.T) (string, *DocsWork) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("kein git im Pfad")
	}
	root, store, work := docsWorkFixture(t)
	store.Put("docs/generated/code/01-a.md", docsDoc("A"))
	writeFile(t, filepath.Join(root, "src", "a.go"), "package a\n")
	docsStandGit(t, root, "init", "-q", "-b", "main")
	docsStandGit(t, root, "add", "-A")
	docsStandGit(t, root, "commit", "-q", "-m", "eins")
	return root, work
}

func docsStandCheckout(t *testing.T, work *DocsWork) {
	t.Helper()
	if _, err := work.Checkout(context.Background(), "code", DocsCheckoutOptions{}); err != nil {
		t.Fatal(err)
	}
}

// existingStand ist der Vergleich, den ein zweiter checkout bei existing
// meldet; dazu prüft er, dass Prüfmodus und publish denselben melden.
func existingStand(t *testing.T, work *DocsWork) *DocsStandCheck {
	t.Helper()
	result, err := work.Checkout(context.Background(), "code", DocsCheckoutOptions{})
	if err != nil || result.State != DocsStateExisting || result.Existing.Stand == nil {
		t.Fatalf("checkout: %+v, %v", result, err)
	}
	check, err := work.Publish(context.Background(), "code", DocsPublishOptions{Check: true})
	if err != nil || !reflect.DeepEqual(check.Stand, result.Existing.Stand) {
		t.Fatalf("Prüfmodus: %+v, %v — checkout: %+v", check.Stand, err, result.Existing.Stand)
	}
	return result.Existing.Stand
}

func TestDocsStandUnveraendert(t *testing.T) {
	root, work := docsStandFixture(t)
	writeFile(t, filepath.Join(root, "src", "a.go"), "package a // lokal\n")
	docsStandCheckout(t, work)

	record, _, _ := readDocsRecord(DocsRecordFile(root, "code"))
	if record.Stand == nil || !record.Stand.Git || record.Stand.Branch != "main" || len(record.Stand.Commit) != 40 {
		t.Fatalf("Merkdatei: %+v", record.Stand)
	}
	var paths []string
	for _, file := range record.Stand.Dirty {
		paths = append(paths, file.Path)
		if file.Hash == "" {
			t.Errorf("ohne Prüfsumme: %+v", file)
		}
	}
	// .k-playbook/.gitignore schreibt checkout selbst; der Stand kommt danach.
	if !reflect.DeepEqual(paths, []string{".k-playbook/.gitignore", "src/a.go"}) {
		t.Errorf("geänderte Dateien: %v", paths)
	}

	stand := existingStand(t, work)
	if !stand.Compared || stand.Changed || len(stand.Files) != 0 || len(stand.Incomplete) != 0 ||
		stand.Recorded.Commit != record.Stand.Commit || stand.Current.Branch != "main" {
		t.Errorf("Vergleich: %+v", stand)
	}
	result, err := work.Publish(context.Background(), "code", DocsPublishOptions{})
	if err != nil || result.Stand == nil || result.Stand.Changed || !result.Cleaned {
		t.Errorf("publish: %+v, %v", result, err)
	}
}

func TestDocsStandNeuerCommit(t *testing.T) {
	root, work := docsStandFixture(t)
	docsStandCheckout(t, work)
	writeFile(t, filepath.Join(root, "src", "b.go"), "package a\n")
	docsStandGit(t, root, "add", "src/b.go")
	docsStandGit(t, root, "commit", "-q", "-m", "zwei")

	stand := existingStand(t, work)
	if !stand.Changed || !reflect.DeepEqual(stand.Files, []string{"src/b.go"}) || stand.Recorded.Commit == stand.Current.Commit {
		t.Errorf("Vergleich: %+v", stand)
	}
	// publish schreibt trotz Abweichung.
	result, err := work.Publish(context.Background(), "code", DocsPublishOptions{})
	if err != nil || !result.Stand.Changed || !result.Cleaned {
		t.Errorf("publish: %+v, %v", result, err)
	}
}

func TestDocsStandDieselbeDateiWeiterGeaendert(t *testing.T) {
	root, work := docsStandFixture(t)
	writeFile(t, filepath.Join(root, "src", "a.go"), "package a // eins\n")
	docsStandCheckout(t, work)
	writeFile(t, filepath.Join(root, "src", "a.go"), "package a // zwei\n")

	stand := existingStand(t, work)
	if !stand.Changed || !reflect.DeepEqual(stand.Files, []string{"src/a.go"}) || stand.Recorded.Commit != stand.Current.Commit {
		t.Errorf("Vergleich: %+v", stand)
	}
}

func TestDocsStandDateiDazugekommen(t *testing.T) {
	root, work := docsStandFixture(t)
	docsStandCheckout(t, work)
	writeFile(t, filepath.Join(root, "src", "neu", "c.go"), "package neu\n")

	stand := existingStand(t, work)
	if !stand.Changed || !reflect.DeepEqual(stand.Files, []string{"src/neu/c.go"}) {
		t.Errorf("Vergleich: %+v", stand)
	}
}

// Mehr als 500 geänderte Dateien: die übrigen ohne Prüfsumme; der Vergleich
// nennt dirty_truncated statt eines stillen changed: false, und eine Datei
// hinter der Grenze gilt nicht als Abweichung.
func TestDocsStandMehrAls500(t *testing.T) {
	root, work := docsStandFixture(t)
	for i := range docsStandMaxDirty + 3 {
		writeFile(t, filepath.Join(root, "viele", fmt.Sprintf("f%04d.txt", i)), "x\n")
	}
	docsStandCheckout(t, work)
	record, _, _ := readDocsRecord(DocsRecordFile(root, "code"))
	if len(record.Stand.Dirty) != docsStandMaxDirty || record.Stand.DirtyTruncated != 4 {
		t.Fatalf("Merkdatei: %d Einträge, %d übrig", len(record.Stand.Dirty), record.Stand.DirtyTruncated)
	}

	stand := existingStand(t, work)
	if stand.Changed || !reflect.DeepEqual(stand.Incomplete, []string{DocsStandDirtyTruncated}) {
		t.Errorf("unverändert: %+v", stand)
	}
	writeFile(t, filepath.Join(root, "viele", "f0001.txt"), "y\n")
	writeFile(t, filepath.Join(root, "zzz.txt"), "hinter der Grenze\n")
	stand = existingStand(t, work)
	if !stand.Changed || !reflect.DeepEqual(stand.Files, []string{"viele/f0001.txt"}) {
		t.Errorf("geändert: %+v", stand)
	}
}

// Ein festgehaltener Commit, der sich nicht mehr auflösen lässt: kein Fehler,
// incomplete commit_unknown, changed wegen des anderen Commits.
func TestDocsStandCommitUnbekannt(t *testing.T) {
	root, work := docsStandFixture(t)
	docsStandCheckout(t, work)
	file := DocsRecordFile(root, "code")
	record, _, _ := readDocsRecord(file)
	record.Stand.Commit = strings.Repeat("d", 40)
	if err := writeDocsRecord(file, record); err != nil {
		t.Fatal(err)
	}

	stand := existingStand(t, work)
	if !stand.Changed || !reflect.DeepEqual(stand.Incomplete, []string{DocsStandCommitUnknown}) || len(stand.Files) != 0 {
		t.Errorf("Vergleich: %+v", stand)
	}
}

// Ohne git: Der Stand trägt git false, es gibt keinen Vergleich und keinen
// Fehler.
func TestDocsStandOhneGit(t *testing.T) {
	root, _, work := docsWorkFixture(t)
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	docsStandCheckout(t, work)
	record, _, _ := readDocsRecord(DocsRecordFile(root, "code"))
	if record.Stand == nil || record.Stand.Git {
		t.Fatalf("Merkdatei: %+v", record.Stand)
	}
	stand := existingStand(t, work)
	if stand.Compared || stand.Changed || !strings.Contains(stand.Message, "kein Stand festgehalten") {
		t.Errorf("Vergleich: %+v", stand)
	}
}

// Eine Merkdatei ohne stand (älterer checkout) bleibt gültig: kein Vergleich,
// kein Fehler, publish geht durch.
func TestDocsStandMerkdateiOhneStand(t *testing.T) {
	root, work := docsStandFixture(t)
	docsStandCheckout(t, work)
	file := DocsRecordFile(root, "code")
	var raw map[string]any
	data, _ := os.ReadFile(file)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw, "stand")
	data, _ = json.Marshal(raw)
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}

	stand := existingStand(t, work)
	if stand.Compared || stand.Changed || !strings.Contains(stand.Message, "kein Stand festgehalten") {
		t.Errorf("Vergleich: %+v", stand)
	}
	result, err := work.Publish(context.Background(), "code", DocsPublishOptions{})
	if err != nil || result.Stand == nil || result.Stand.Compared || !result.Cleaned {
		t.Errorf("publish: %+v, %v", result, err)
	}
}

// git status -z: Umbenennen und Kopieren bringen den alten Pfad als eigenes
// Feld, im Index (X) wie im Arbeitsbaum (Y).
func TestDocsStatusPaths(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   []string
	}{
		{"Index umbenannt", " M b.go\x00R  neu.go\x00alt.go\x00?? a b.txt\x00", []string{"a b.txt", "alt.go", "b.go", "neu.go"}},
		{"Index kopiert", "C  kopie.go\x00original.go\x00", []string{"kopie.go", "original.go"}},
		{"Arbeitsbaum umbenannt", " R neu.txt\x00original_name.txt\x00 M b.go\x00", []string{"b.go", "neu.txt", "original_name.txt"}},
		{"Arbeitsbaum kopiert", " C kopie.txt\x00original_name.txt\x00", []string{"kopie.txt", "original_name.txt"}},
		{"beide umbenannt", "RM neu.go\x00alt.go\x00", []string{"alt.go", "neu.go"}},
	}
	for _, c := range cases {
		if got := docsStatusPaths(c.status); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %q, erwartet %q", c.name, got, c.want)
		}
	}
}

// Umbenennen mit git selbst: im Index (git mv) und im Arbeitsbaum (mv und
// git add -N) stehen alter und neuer Pfad unverkürzt im Stand.
func TestDocsStandUmbenannt(t *testing.T) {
	root, _ := docsStandFixture(t)
	content := strings.Repeat("eine Zeile, die beim Umbenennen gleich bleibt\n", 20)
	writeFile(t, filepath.Join(root, "original_name.txt"), content)
	writeFile(t, filepath.Join(root, "index_alt.txt"), "anders\n"+content)
	docsStandGit(t, root, "add", "-A")
	docsStandGit(t, root, "commit", "-q", "-m", "zwei")

	docsStandGit(t, root, "mv", "index_alt.txt", "index_neu.txt")
	if err := os.Rename(filepath.Join(root, "original_name.txt"), filepath.Join(root, "neu.txt")); err != nil {
		t.Fatal(err)
	}
	docsStandGit(t, root, "add", "-N", "neu.txt")

	stand := readDocsStand(context.Background(), root)
	var got []string
	for _, f := range stand.Dirty {
		got = append(got, f.Path)
	}
	want := []string{"index_alt.txt", "index_neu.txt", "neu.txt", "original_name.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Pfade: %q, erwartet %q", got, want)
	}
}
