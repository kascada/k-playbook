package sharedir

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kascada/k-playbook/share"
)

func TestDirFrom(t *testing.T) {
	home := func() (string, error) { return "/home/u", nil }
	noHome := func() (string, error) { return "", errors.New("kein Home") }
	env := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}

	cases := []struct {
		name   string
		getenv func(string) string
		home   func() (string, error)
		want   string
		fails  bool
	}{
		{name: "XDG_DATA_HOME", getenv: env(map[string]string{"XDG_DATA_HOME": "/data"}), home: home, want: "/data/k-playbook"},
		{name: "relatives XDG_DATA_HOME zählt nicht", getenv: env(map[string]string{"XDG_DATA_HOME": "data"}), home: home, want: "/home/u/.local/share/k-playbook"},
		{name: "Home", getenv: env(nil), home: home, want: "/home/u/.local/share/k-playbook"},
		{name: "nichts bekannt", getenv: env(nil), home: noHome, fails: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := dirFrom(c.getenv, c.home)
			if c.fails {
				if err == nil {
					t.Fatalf("Fehler erwartet, bekommen %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("Verzeichnis = %q, erwartet %q", got, c.want)
			}
		})
	}
}

// targetDir ist ein Zielverzeichnis, das t.TempDir wieder aufräumt — der
// Schreibschutz der Dateien darf das nicht verhindern.
func targetDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "k-playbook")
}

func TestEnsureEntpacktErsetztUndLaesstDanebenStehen(t *testing.T) {
	dir := targetDir(t)
	roots := []string{"commands", "skills"}

	// Was schon im Verzeichnis liegt und nicht zu share/ gehört.
	other := filepath.Join(dir, "security-tools", "venv.txt")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("bleibt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	first := fstest.MapFS{
		"commands/k-eins.md":         {Data: []byte("eins\n")},
		"commands/_shared/modul.md":  {Data: []byte("modul\n")},
		"skills/befunde/SKILL.md":    {Data: []byte("skill\n")},
		"skills/befunde/vorlagen/a":  {Data: []byte("a\n")},
		"anderes/nicht-entpackt.txt": {Data: []byte("nein\n")},
	}
	changed, err := ensure(dir, first, roots, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("erster Lauf hat nicht entpackt")
	}
	assertFile(t, dir, "commands/k-eins.md", "eins\n")
	assertFile(t, dir, "commands/_shared/modul.md", "modul\n")
	assertFile(t, dir, "skills/befunde/vorlagen/a", "a\n")
	assertFile(t, dir, VersionName, "v1.0.0\n")
	assertMissing(t, dir, "anderes")
	assertMode(t, dir, "commands/k-eins.md", fileMode)
	assertMode(t, dir, "commands", dirMode)
	assertMode(t, dir, "skills/befunde/vorlagen", dirMode)

	changed, err = ensure(dir, first, roots, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("zweiter Lauf mit gleichem Inhalt hat entpackt")
	}

	// Gleiche Version, anderer Inhalt — der Fall von `make dev-install`.
	second := fstest.MapFS{
		"commands/k-zwei.md":      {Data: []byte("zwei\n")},
		"skills/befunde/SKILL.md": {Data: []byte("skill neu\n")},
	}
	changed, err = ensure(dir, second, roots, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("geänderter Inhalt bei gleicher Version wurde nicht entpackt")
	}
	assertFile(t, dir, "commands/k-zwei.md", "zwei\n")
	assertFile(t, dir, "skills/befunde/SKILL.md", "skill neu\n")
	assertMissing(t, dir, "commands/k-eins.md")
	assertMissing(t, dir, "commands/_shared")
	assertMissing(t, dir, "skills/befunde/vorlagen")
	assertFile(t, dir, "security-tools/venv.txt", "bleibt\n")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".neu-") {
			t.Fatalf("Zwischenverzeichnis %s ist liegen geblieben", entry.Name())
		}
	}
}

func TestEnsureOhneVersionEntferntVERSION(t *testing.T) {
	dir := targetDir(t)
	roots := []string{"commands"}
	if _, err := ensure(dir, fstest.MapFS{"commands/a.md": {Data: []byte("a\n")}}, roots, "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := ensure(dir, fstest.MapFS{"commands/a.md": {Data: []byte("b\n")}}, roots, ""); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, dir, VersionName)
}

func TestEnsureEntpacktNachAbbruchErneut(t *testing.T) {
	dir := targetDir(t)
	roots := []string{"commands"}
	fsys := fstest.MapFS{"commands/a.md": {Data: []byte("a\n")}}
	if _, err := ensure(dir, fsys, roots, "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	// Ein Lauf, der vor der Prüfsumme abbrach, hinterlässt keine.
	if err := os.Remove(filepath.Join(dir, SumName)); err != nil {
		t.Fatal(err)
	}
	changed, err := ensure(dir, fsys, roots, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("ohne Prüfsumme wurde nicht neu entpackt")
	}
}

func TestSumUnterscheidetPfadUndInhalt(t *testing.T) {
	roots := []string{"r"}
	a, err := Sum(fstest.MapFS{"r/ab": {Data: []byte("c")}}, roots)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Sum(fstest.MapFS{"r/a": {Data: []byte("bc")}}, roots)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("verschiedene Bestände ergeben dieselbe Prüfsumme")
	}
}

// TestShareGleichtDenAltenVerzeichnissen hält share/ und die alten
// Verzeichnisse oben im Repo gleich, solange beide bestehen (Stufe 1 bis 3 von
// Punkt 20 der Arbeitsliste in docs/knowledge-aktuell.md). Geprüft wird das
// eingebettete FS, nicht share/ auf der Platte — so fällt auch auf, wenn das
// Einbetten etwas auslässt.
func TestShareGleichtDenAltenVerzeichnissen(t *testing.T) {
	repo := os.DirFS(filepath.Join("..", "..", ".."))
	for _, root := range share.Roots {
		embedded := listFiles(t, share.Files, root)
		onDisk := listFiles(t, repo, root)
		for name := range onDisk {
			if _, ok := embedded[name]; !ok {
				t.Errorf("%s fehlt in share/ (make share-sync)", name)
			}
		}
		for name, data := range embedded {
			disk, ok := onDisk[name]
			switch {
			case !ok:
				t.Errorf("share/%s gibt es oben im Repo nicht (make share-sync)", name)
			case !bytes.Equal(data, disk):
				t.Errorf("share/%s weicht von %s ab (make share-sync)", name, name)
			}
		}
	}
}

func listFiles(t *testing.T, fsys fs.FS, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := fs.WalkDir(fsys, root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "__pycache__" {
				return fs.SkipDir
			}
			return nil
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		files[name] = data
		return nil
	})
	if err != nil {
		t.Fatalf("%s lesen: %v", root, err)
	}
	return files
}

func assertFile(t *testing.T, dir, rel, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, erwartet %q", rel, data, want)
	}
}

func assertMissing(t *testing.T, dir, rel string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(rel))); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s sollte fehlen (err = %v)", rel, err)
	}
}

func assertMode(t *testing.T, dir, rel string, want fs.FileMode) {
	t.Helper()
	info, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s hat Modus %o, erwartet %o", rel, got, want)
	}
}
