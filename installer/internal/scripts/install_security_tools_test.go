package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// securityMatrix ist eine Matrix mit genau einem Pflicht-Tool, das auf jedem
// Host vorhanden ist. Damit erreicht `--install missing` den Guard, findet
// danach aber nichts zu installieren — der Lauf endet ohne Netzzugriff und ohne
// Schreibzugriff. Geprüft wird also genau der Guard und nichts sonst.
const securityMatrix = "name\tlanguages\trequired\tinstallable\tinstall_method\tinstall_ref\tasset_pattern\trole\tdocker_image\tversion_args\n" +
	"bash\t*\ttrue\ttrue\tgithub\tbeispiel/bash\t^{tool}$\tInterpreter\t-\t--version\n"

func securityScript(t *testing.T) string {
	t.Helper()
	return scriptPath(t, "install-security-tools.sh")
}

// runSecurityInstall ruft den schreibenden Weg mit einem vorgegebenen Ziel auf.
func runSecurityInstall(t *testing.T, binDir string, extra ...string) result {
	t.Helper()
	matrix := writeMatrix(t, securityMatrix)
	args := append([]string{"--install", "missing", "--bin-dir", binDir}, extra...)
	return runScript(t, securityScript(t), []string{
		"K_SECURITY_TOOLS_MATRIX=" + matrix,
	}, args...)
}

// TestSecurityGuardWeistFremdesZielAb ist der ursprüngliche Fehler: ein
// schreibender Lauf auf ein Ziel, das dem ausführenden Benutzer nicht gehört.
//
// Als normaler Nutzer wird das über /usr/local/bin geprüft — das Verzeichnis
// gehört root, die effektive UID nicht. Als root trifft dieselbe Konstellation
// zu, wenn das Ziel einem anderen Benutzer gehört; siehe
// TestSecurityGuardRootFaelle.
func TestSecurityGuardWeistFremdesZielAb(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("läuft als root: /usr/local/bin gehört dann der effektiven UID. Der Abbruchfall wird in TestSecurityGuardRootFaelle geprüft.")
	}
	if _, err := os.Stat("/usr/local/bin"); err != nil {
		t.Skip("/usr/local/bin gibt es auf diesem Host nicht")
	}

	got := runSecurityInstall(t, "/usr/local/bin")
	if got.code == 0 {
		t.Fatalf("Aufruf lief durch, erwartet war ein Abbruch. Ausgabe:\n%s", got.all())
	}
	for _, want := range []string{
		"gehört nicht dem ausführenden Benutzer",
		"/usr/local/bin",
		"Effektive UID:",
		"Eigentümer:",
		"--bin-dir /usr/local/bin",
	} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("Meldung nennt %q nicht:\n%s", want, got.stderr)
		}
	}
}

// TestSecurityGuardLaesstEigenesZielDurch deckt die Durchlauf-Fälle ab, die
// ohne root prüfbar sind: ein vorhandenes eigenes Ziel und ein noch nicht
// vorhandenes, dessen nächstes vorhandenes Elternverzeichnis dem Benutzer
// gehört. Der zweite Fall ist der erste Lauf, bei dem ~/.local/bin noch fehlt.
func TestSecurityGuardLaesstEigenesZielDurch(t *testing.T) {
	base := t.TempDir()
	cases := map[string]string{
		"vorhandenes eigenes Ziel":             base,
		"noch nicht vorhandenes Ziel":          filepath.Join(base, "gibt", "es", "noch", "nicht"),
		"tiefes Ziel unter eigenem Elternteil": filepath.Join(base, "bin"),
	}

	for name, binDir := range cases {
		t.Run(name, func(t *testing.T) {
			got := runSecurityInstall(t, binDir)
			if got.code != 0 {
				t.Fatalf("Aufruf brach ab, erwartet war ein Durchlauf. Ausgabe:\n%s", got.all())
			}
			if strings.Contains(got.stderr, "gehört nicht dem ausführenden Benutzer") {
				t.Errorf("Guard hat zu Unrecht gegriffen:\n%s", got.stderr)
			}
		})
	}
}

// TestSecurityGuardIgnoriertSudoUser belegt, dass SUDO_USER für sich genommen
// kein Abbruchgrund ist. Das ist der Fall `sudo -u <user> -H …`: eine
// Rechteabgabe, bei der Ziel und effektive UID zusammenpassen.
func TestSecurityGuardIgnoriertSudoUser(t *testing.T) {
	got := runScript(t, securityScript(t), []string{
		"K_SECURITY_TOOLS_MATRIX=" + writeMatrix(t, securityMatrix),
		"SUDO_USER=jemand-anders",
	}, "--install", "missing", "--bin-dir", t.TempDir())

	if got.code != 0 {
		t.Fatalf("SUDO_USER allein hat den Lauf abgebrochen:\n%s", got.all())
	}
}

// TestSecurityGuardLaesstLesendeLaeufeDurch prüft die Ausnahme: --preflight,
// --json und --dry-run schreiben nichts. Ein Abbruch nähme dort gerade die
// Diagnose, mit der man den Fall versteht.
func TestSecurityGuardLaesstLesendeLaeufeDurch(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("läuft als root: /usr/local/bin gehört dann der effektiven UID, der schreibende Lauf bräche gar nicht ab.")
	}
	if _, err := os.Stat("/usr/local/bin"); err != nil {
		t.Skip("/usr/local/bin gibt es auf diesem Host nicht")
	}

	// Erst der Beleg, dass genau diese Konstellation schreibend abbräche.
	writing := runSecurityInstall(t, "/usr/local/bin")
	if writing.code == 0 {
		t.Fatalf("Vorbedingung verfehlt: der schreibende Lauf bricht nicht ab:\n%s", writing.all())
	}

	matrix := writeMatrix(t, securityMatrix)
	env := []string{"K_SECURITY_TOOLS_MATRIX=" + matrix}
	cases := map[string][]string{
		"dry-run":   {"--install", "missing", "--bin-dir", "/usr/local/bin", "--dry-run"},
		"preflight": {"--preflight", "--bin-dir", "/usr/local/bin"},
		"json":      {"--json", "--bin-dir", "/usr/local/bin"},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			got := runScript(t, securityScript(t), env, args...)
			if got.code != 0 {
				t.Fatalf("lesender Lauf brach ab:\n%s", got.all())
			}
			if strings.Contains(got.stderr, "gehört nicht dem ausführenden Benutzer") {
				t.Errorf("Guard hat einen lesenden Lauf abgewiesen:\n%s", got.stderr)
			}
		})
	}
}

// TestSecurityGuardRootFaelle deckt die Konstellationen ab, für die die
// effektive UID 0 sein muss.
//
// UNGETESTET AUF EINEM ENTWICKLERHOST: sudo verlangt hier ein Passwort, und
// CI führt keine Tests aus. Übersprungen bleiben damit die drei Root-Fälle aus
// der Doktrin — root mit root-eigenem Ziel (`sudo -H`, Image-Build), root mit
// ausdrücklich systemweitem Ziel (--bin-dir /usr/local/bin) und root mit einem
// Ziel, das einem anderen Benutzer gehört (der eigentliche Abbruchfall unter
// sudo). Im Image-Build laufen sie als root mit.
func TestSecurityGuardRootFaelle(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("braucht die effektive UID 0. Ungetestet bleiben: root mit root-eigenem Ziel (Image-Build), root mit --bin-dir /usr/local/bin und root mit einem Ziel, das einem anderen Benutzer gehört.")
	}

	t.Run("root mit root-eigenem Ziel läuft durch", func(t *testing.T) {
		got := runSecurityInstall(t, t.TempDir())
		if got.code != 0 {
			t.Fatalf("root mit eigenem Ziel wurde abgewiesen:\n%s", got.all())
		}
	})

	t.Run("root mit systemweitem Ziel läuft durch", func(t *testing.T) {
		if _, err := os.Stat("/usr/local/bin"); err != nil {
			t.Skip("/usr/local/bin gibt es auf diesem Host nicht")
		}
		got := runSecurityInstall(t, "/usr/local/bin")
		if got.code != 0 {
			t.Fatalf("der ausdrücklich erlaubte systemweite Weg wurde abgewiesen:\n%s", got.all())
		}
	})

	t.Run("root mit fremdem Ziel bricht ab", func(t *testing.T) {
		foreign := filepath.Join(t.TempDir(), "fremd")
		if err := os.Mkdir(foreign, 0o755); err != nil {
			t.Fatalf("Verzeichnis anlegen: %v", err)
		}
		// 65534 ist nobody auf den verbreiteten Distributionen.
		if err := os.Chown(foreign, 65534, 65534); err != nil {
			t.Skipf("Eigentümer nicht setzbar: %v", err)
		}
		got := runSecurityInstall(t, foreign)
		if got.code == 0 {
			t.Fatalf("fremdes Ziel lief durch, erwartet war ein Abbruch:\n%s", got.all())
		}
	})
}
