package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Die Matrizen der Wegetests führen je genau einen Eintrag. So steht fest,
// welcher Fall der Rangfolge geprüft wird, und der Test hängt nicht an der
// ausgelieferten Matrix.
const (
	githubEntryMatrix = "name\tgroup\trole\tguarded\tmethods\tapt_package\tgithub_repo\tasset_ref\tasset_pattern\n" +
		"k-test-rg\t-\tTestrolle\tnein\tapt,github\tripgrep\tBurntSushi/ripgrep\tripgrep\t^{tool}-{version}-{arch_raw}-{vendor_os}\\.tar\\.gz$\n"
	aptOnlyEntryMatrix = "name\tgroup\trole\tguarded\tmethods\tapt_package\tgithub_repo\tasset_ref\tasset_pattern\n" +
		"k-test-git\t-\tTestrolle\tnein\tapt\tgit\t-\t-\t-\n"
	noneEntryMatrix = "name\tgroup\trole\tguarded\tmethods\tapt_package\tgithub_repo\tasset_ref\tasset_pattern\n" +
		"k-test-docker\t-\tTestrolle\tnein\tnone\t-\t-\t-\t-\n"
	groupEntryMatrix = "name\tgroup\trole\tguarded\tmethods\tapt_package\tgithub_repo\tasset_ref\tasset_pattern\n" +
		"curl\tdownload\tDownload\tja\tapt\tcurl\t-\t-\t-\n" +
		"wget\tdownload\tDownload-Rückfall\tja\tapt\twget\t-\t-\t-\n"
)

func baseScript(t *testing.T) string {
	t.Helper()
	return scriptPath(t, "install-base-tools.sh")
}

// minimalPath baut ein PATH-Verzeichnis mit genau den Programmen, die das
// Skript braucht. Damit lässt sich die Anwesenheit von apt-get steuern — der
// Unterschied zwischen Fall 3 und Fall 4 der Rangfolge — ohne den Host zu
// verändern.
func minimalPath(t *testing.T, withAptGet bool) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"dirname", "basename", "id", "uname", "stat", "getent", "cut", "mkdir"} {
		source, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s nicht gefunden: %v", name, err)
		}
		if err := os.Symlink(source, filepath.Join(dir, name)); err != nil {
			t.Fatalf("Symlink für %s: %v", name, err)
		}
	}
	if withAptGet {
		// Ein Platzhalter, der nie aufgerufen wird: geprüft wird nur die
		// Anwesenheit. Ausgeführt würde er allenfalls als root, und dann nur
		// unter --dry-run.
		fake := filepath.Join(dir, "apt-get")
		if err := os.WriteFile(fake, []byte("#!/bin/sh\necho \"apt-get $*\"\n"), 0o755); err != nil {
			t.Fatalf("apt-get-Platzhalter: %v", err)
		}
	}
	return dir
}

// runBase ruft das Basis-Skript mit einer Matrix und einem gesetzten PATH auf.
func runBase(t *testing.T, matrix string, path string, args ...string) result {
	t.Helper()
	return runScript(t, baseScript(t), []string{
		"K_BASE_TOOLS_MATRIX=" + writeMatrix(t, matrix),
		"PATH=" + path,
	}, args...)
}

// TestBasePreflightMeldetFehlendeWerkzeuge prüft den lesenden Weg gegen die
// ausgelieferte Matrix: sie muss sauber parsen und einen Zustand liefern.
func TestBasePreflightMeldetFehlendeWerkzeuge(t *testing.T) {
	got := runScript(t, baseScript(t), nil, "--preflight")
	if got.code != 0 {
		t.Fatalf("Preflight brach ab:\n%s", got.all())
	}
	for _, want := range []string{"Basis-Werkzeuge - Preflight", "bash", "git", "curl", "wget", "tar", "python3", "rg"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("Preflight nennt %q nicht:\n%s", want, got.stdout)
		}
	}
}

// TestBaseGruppeGiltAlsVorhanden belegt die curl/wget-Regel: die Gruppe gilt
// als vorhanden, sobald eines ihrer Mitglieder da ist. Ohne das meldete der
// Befund auf einem Host mit curl dauerhaft wget als fehlend.
func TestBaseGruppeGiltAlsVorhanden(t *testing.T) {
	// Ein PATH mit curl, aber ohne wget. Auf dem Entwicklungshost liegen beide;
	// die Regel zeigt sich erst, wenn genau eines fehlt.
	path := minimalPath(t, false)
	source, err := exec.LookPath("curl")
	if err != nil {
		t.Skipf("curl fehlt auf diesem Host: %v", err)
	}
	if err := os.Symlink(source, filepath.Join(path, "curl")); err != nil {
		t.Fatalf("Symlink für curl: %v", err)
	}

	got := runBase(t, groupEntryMatrix, path, "--json")
	if got.code != 0 {
		t.Fatalf("--json brach ab:\n%s", got.all())
	}
	if !strings.Contains(got.stdout, `"missing": 0`) {
		t.Errorf("wget wurde trotz vorhandenem curl als fehlend gezählt:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, `"name": "wget", "group": "download", "methods": "apt", "guarded": "ja", "status": "group-ok"`) {
		t.Errorf("wget steht nicht als über die Gruppe gedeckt in der Ausgabe:\n%s", got.stdout)
	}
}

// TestBaseFall2UserLokalerWeg ist der Grund, aus dem der Rückfall existiert:
// Ubuntu-Host, apt vorhanden, kein root. Installiert wird trotzdem user-lokal,
// und der systemweite Befehl steht als Hinweis daneben.
func TestBaseFall2UserLokalerWeg(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("läuft als root: mit apt-get greift dann Fall 1. Ungetestet bleibt hier der Nicht-Root-Zweig.")
	}
	got := runBase(t, githubEntryMatrix, minimalPath(t, true), "--install", "--yes", "--dry-run")
	if got.code != 0 {
		t.Fatalf("Fall 2 endete mit %d:\n%s", got.code, got.all())
	}
	for _, want := range []string{
		"user-lokal aus dem Release von BurntSushi/ripgrep",
		"sudo apt-get install -y ripgrep",
		"Installiert wird trotzdem user-lokal",
	} {
		if !strings.Contains(got.all(), want) {
			t.Errorf("Fall 2 nennt %q nicht:\n%s", want, got.all())
		}
	}
}

// TestBaseFall2OhneAptOhneHinweis: ohne apt auf dem Host entfällt die
// Hinweiszeile — ein sudo-apt-Befehl auf einem System ohne apt wäre falsch.
func TestBaseFall2OhneAptOhneHinweis(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("läuft als root; siehe TestBaseFall2UserLokalerWeg")
	}
	got := runBase(t, githubEntryMatrix, minimalPath(t, false), "--install", "--yes", "--dry-run")
	if got.code != 0 {
		t.Fatalf("Fall 2 ohne apt endete mit %d:\n%s", got.code, got.all())
	}
	if strings.Contains(got.all(), "sudo apt-get") {
		t.Errorf("auf einem Host ohne apt wurde trotzdem ein apt-Befehl genannt:\n%s", got.all())
	}
}

// TestBaseFall3AptOnlyMitApt: der ausgegebene Befehl ist das Ergebnis, nicht
// ein Zwischenschritt. Es wird nichts user-lokal geschrieben, und der eigene
// Rückgabewert trennt den Fall vom Fehlschlag.
func TestBaseFall3AptOnlyMitApt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("läuft als root: mit apt-get greift dann Fall 1. Ungetestet bleibt hier der Nicht-Root-Zweig.")
	}
	got := runBase(t, aptOnlyEntryMatrix, minimalPath(t, true), "--install", "--yes", "--dry-run")
	if got.code != 3 {
		t.Fatalf("Fall 3 endete mit %d, erwartet 3:\n%s", got.code, got.all())
	}
	for _, want := range []string{"kein user-lokaler Weg", "sudo apt-get install -y git"} {
		if !strings.Contains(got.all(), want) {
			t.Errorf("Fall 3 nennt %q nicht:\n%s", want, got.all())
		}
	}
}

// TestBaseFall4AptOnlyOhneApt: Alpine, RHEL, macOS. Werkzeug und Grund werden
// benannt, statt dass ein sudo-apt-Befehl auf einem System ohne apt erscheint.
func TestBaseFall4AptOnlyOhneApt(t *testing.T) {
	got := runBase(t, aptOnlyEntryMatrix, minimalPath(t, false), "--install", "--yes", "--dry-run")
	if got.code != 3 {
		t.Fatalf("Fall 4 endete mit %d, erwartet 3:\n%s", got.code, got.all())
	}
	if !strings.Contains(got.all(), "apt-only, aber es gibt kein") {
		t.Errorf("Fall 4 benennt den Grund nicht:\n%s", got.all())
	}
	if strings.Contains(got.all(), "sudo apt-get") {
		t.Errorf("auf einem Host ohne apt wurde ein apt-Befehl genannt:\n%s", got.all())
	}
}

// TestBaseMethodeNone endet mit demselben unterscheidenden Rückgabewert wie
// die Fälle 3 und 4.
func TestBaseMethodeNone(t *testing.T) {
	got := runBase(t, noneEntryMatrix, minimalPath(t, true), "--install", "--yes", "--dry-run")
	if got.code != 3 {
		t.Fatalf("Methode none endete mit %d, erwartet 3:\n%s", got.code, got.all())
	}
	if !strings.Contains(got.all(), "kein Installationsweg vorgesehen") {
		t.Errorf("Methode none benennt den Grund nicht:\n%s", got.all())
	}
}

// TestBaseRueckgabewertTrenntVomFehlschlag: ein echter Fehler — hier eine
// unbekannte Option — endet mit 1 und nicht mit dem Wert für „kein Weg".
func TestBaseRueckgabewertTrenntVomFehlschlag(t *testing.T) {
	got := runBase(t, aptOnlyEntryMatrix, minimalPath(t, true), "--gibt-es-nicht")
	if got.code != 1 {
		t.Fatalf("echter Fehlschlag endete mit %d, erwartet 1:\n%s", got.code, got.all())
	}
}

// TestBaseGuardJeEintrag prüft den Guard an der Stelle, an der er im
// Basis-Skript steht: je Eintrag, unmittelbar vor dem Schreiben, und nur für
// den user-lokalen Weg.
func TestBaseGuardJeEintrag(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("läuft als root: /usr/local/bin gehört dann der effektiven UID. Ungetestet bleibt der Abbruch unter root mit fremdem Ziel.")
	}
	if _, err := os.Stat("/usr/local/bin"); err != nil {
		t.Skip("/usr/local/bin gibt es auf diesem Host nicht")
	}

	t.Run("schreibender Lauf auf fremdes Ziel bricht ab", func(t *testing.T) {
		got := runBase(t, githubEntryMatrix, minimalPath(t, false), "--install", "--yes", "--bin-dir", "/usr/local/bin")
		if got.code == 0 {
			t.Fatalf("der Guard hat nicht gegriffen:\n%s", got.all())
		}
		if !strings.Contains(got.stderr, "gehört nicht dem ausführenden Benutzer") {
			t.Errorf("es war ein anderer Abbruch:\n%s", got.all())
		}
	})

	t.Run("dry-run auf dasselbe Ziel läuft durch", func(t *testing.T) {
		got := runBase(t, githubEntryMatrix, minimalPath(t, false), "--install", "--yes", "--dry-run", "--bin-dir", "/usr/local/bin")
		if got.code != 0 {
			t.Fatalf("der Trockenlauf wurde abgewiesen:\n%s", got.all())
		}
	})

	t.Run("apt-only-Eintrag wird vom Guard nicht berührt", func(t *testing.T) {
		// Fall 3 schreibt nichts user-lokal, also darf das fremde Ziel dort
		// keine Rolle spielen: erwartet wird der Rückgabewert für „kein Weg",
		// nicht der Abbruch des Guards.
		got := runBase(t, aptOnlyEntryMatrix, minimalPath(t, true), "--install", "--yes", "--bin-dir", "/usr/local/bin")
		if got.code != 3 {
			t.Fatalf("apt-only endete mit %d, erwartet 3:\n%s", got.code, got.all())
		}
		if strings.Contains(got.stderr, "gehört nicht dem ausführenden Benutzer") {
			t.Errorf("der Guard griff auf einem Weg, der nichts user-lokal schreibt:\n%s", got.all())
		}
	})
}

// TestBaseVenvGuard hält rules/tool-install-scope.md ein: ein aktives
// Projekt-venv bricht die Installation ab, den Preflight aber nicht.
func TestBaseVenvGuard(t *testing.T) {
	env := []string{
		"K_BASE_TOOLS_MATRIX=" + writeMatrix(t, aptOnlyEntryMatrix),
		"VIRTUAL_ENV=/pfad/zu/.venv",
	}

	install := runScript(t, baseScript(t), env, "--install", "--yes", "--dry-run")
	if install.code == 0 {
		t.Errorf("--install lief mit aktivem venv durch:\n%s", install.all())
	}

	preflight := runScript(t, baseScript(t), env, "--preflight")
	if preflight.code != 0 {
		t.Fatalf("--preflight brach mit aktivem venv ab:\n%s", preflight.all())
	}
	if !strings.Contains(preflight.stdout, "aktives Projekt-venv") {
		t.Errorf("der Messkontext wurde nicht gekennzeichnet:\n%s", preflight.stdout)
	}
}

// TestBaseRootFall1 ist der apt-Weg. Er braucht die effektive UID 0.
//
// UNGETESTET AUF EINEM ENTWICKLERHOST: sudo verlangt hier ein Passwort, und CI
// führt keine Tests aus. Übersprungen bleibt damit Fall 1 der Rangfolge — root
// mit vorhandenem apt-get — samt der Zusage, dass der Eigentümer-Guard dort
// nicht gilt, weil das Ziel systemweit ist und nicht an $HOME hängt. Im
// Image-Build läuft er als root mit.
func TestBaseRootFall1(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("braucht die effektive UID 0. Ungetestet bleibt Fall 1 der Rangfolge: root mit apt-get installiert über apt, und der Eigentümer-Guard gilt dort nicht.")
	}

	got := runBase(t, aptOnlyEntryMatrix, minimalPath(t, true), "--install", "--yes", "--dry-run")
	if got.code != 0 {
		t.Fatalf("Fall 1 endete mit %d:\n%s", got.code, got.all())
	}
	if !strings.Contains(got.all(), "apt-get") {
		t.Errorf("Fall 1 ging nicht über apt:\n%s", got.all())
	}
	if strings.Contains(got.all(), "kein user-lokaler Weg") {
		t.Errorf("Fall 1 fiel in Fall 3 durch:\n%s", got.all())
	}
}
