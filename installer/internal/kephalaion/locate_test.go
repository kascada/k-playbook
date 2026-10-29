package kephalaion

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Die config jeder Quelle trägt einen eigenen Port; am Endpunkt ist zu
// sehen, welche gilt.
const (
	userConfig   = "node:\n  listen: 127.0.0.1:7501\n"
	systemConfig = "node:\n  listen: 127.0.0.1:7502\n"
	envConfig    = "node:\n  listen: 127.0.0.1:7503\n"

	userEndpoint   = "http://127.0.0.1:7501/mcp"
	systemEndpoint = "http://127.0.0.1:7502/mcp"
	envEndpoint    = "http://127.0.0.1:7503/mcp"
)

// locateSetup lenkt alles, was die Suche liest, in temporäre Verzeichnisse:
// XDG_CONFIG_HOME, KEPHALAION_CONFIG (leer) und den Ort der globalen config.
// Das echte /etc/kephalaion und ein gesetztes KEPHALAION_CONFIG des Rechners
// sieht der Test nicht. Im Verzeichnis des Users liegt ein Token für
// vm/kamran-wsl; die config des Users und die globale legt der Test selbst an.
func locateSetup(t *testing.T) (userDir, systemPath string) {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv(EnvConfigVar, "")
	original := SystemConfigPath
	SystemConfigPath = filepath.Join(t.TempDir(), "etc", "kephalaion", "config.yaml")
	t.Cleanup(func() { SystemConfigPath = original })
	userDir = filepath.Join(xdg, "kephalaion")
	writeFile(t, filepath.Join(userDir, "tokens", "vm", "kamran-wsl.token"), testToken+"\n")
	return userDir, SystemConfigPath
}

// assertUserTokens prüft, dass die Tokens aus dem Verzeichnis des Users
// kommen, gleich welche config gilt.
func assertUserTokens(t *testing.T, env Env, userDir string) {
	t.Helper()
	if env.UserDir != userDir || env.TokensDir() != filepath.Join(userDir, "tokens") {
		t.Fatalf("Token-Verzeichnis %q (User %q), erwartet unter %q", env.TokensDir(), env.UserDir, userDir)
	}
	if _, ok := env.FindCandidate("vm", "kamran-wsl"); !ok {
		t.Fatal("Token des Users nicht gefunden")
	}
}

func TestDefaultEnvOnlyUser(t *testing.T) {
	userDir, _ := locateSetup(t)
	writeFile(t, filepath.Join(userDir, "config.yaml"), userConfig)
	env := DefaultEnv()
	if env.ConfigSource != SourceUser || env.ConfigPath != filepath.Join(userDir, "config.yaml") || !env.ConfigPresent {
		t.Fatalf("nur User: %+v", env)
	}
	if env.Endpoint != userEndpoint || env.ConfigProblem() != "" {
		t.Fatalf("nur User: %+v", env)
	}
	assertUserTokens(t, env, userDir)
}

func TestDefaultEnvOnlySystem(t *testing.T) {
	userDir, systemPath := locateSetup(t)
	writeFile(t, systemPath, systemConfig)
	env := DefaultEnv()
	if env.ConfigSource != SourceSystem || env.ConfigPath != systemPath || !env.ConfigPresent {
		t.Fatalf("nur global: %+v", env)
	}
	if env.Endpoint != systemEndpoint || env.ConfigProblem() != "" {
		t.Fatalf("nur global: %+v", env)
	}
	assertUserTokens(t, env, userDir)
}

// Gibt es beide, gilt die des Users. Den Fehler „zwei Arten auf einem
// Rechner“ meldet kephalaion status, nicht k-playbook.
func TestDefaultEnvBothUserWins(t *testing.T) {
	userDir, systemPath := locateSetup(t)
	writeFile(t, filepath.Join(userDir, "config.yaml"), userConfig)
	writeFile(t, systemPath, systemConfig)
	env := DefaultEnv()
	if env.ConfigSource != SourceUser || env.Endpoint != userEndpoint || env.ConfigProblem() != "" {
		t.Fatalf("beide: %+v", env)
	}
	assertUserTokens(t, env, userDir)
}

func TestDefaultEnvNone(t *testing.T) {
	userDir, systemPath := locateSetup(t)
	env := DefaultEnv()
	if env.ConfigPresent || env.ConfigSource != SourceUser || env.ConfigPath != filepath.Join(userDir, "config.yaml") {
		t.Fatalf("keine: %+v", env)
	}
	if env.Endpoint != "" {
		t.Fatalf("keine config, aber Endpunkt: %+v", env)
	}
	problem := env.ConfigProblem()
	if !strings.Contains(problem, filepath.Join(userDir, "config.yaml")) || !strings.Contains(problem, systemPath) {
		t.Fatalf("Meldung nennt nicht beide gesuchten Orte: %q", problem)
	}
	assertUserTokens(t, env, userDir)

	status := CheckBinding(context.Background(), goodBinding, env)
	if status.State != StateNoKephalaion || status.Message != problem {
		t.Fatalf("Zustand: %+v", status)
	}
}

// KEPHALAION_CONFIG gewinnt vor beiden; ein relativer Pfad wird absolut.
func TestDefaultEnvFromEnvVariable(t *testing.T) {
	userDir, systemPath := locateSetup(t)
	writeFile(t, filepath.Join(userDir, "config.yaml"), userConfig)
	writeFile(t, systemPath, systemConfig)
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "sub", "eigene.yaml"), envConfig)
	t.Chdir(work)
	t.Setenv(EnvConfigVar, filepath.Join("sub", "eigene.yaml"))

	env := DefaultEnv()
	want, err := filepath.Abs(filepath.Join("sub", "eigene.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if env.ConfigSource != SourceEnv || env.ConfigPath != want || !filepath.IsAbs(env.ConfigPath) {
		t.Fatalf("KEPHALAION_CONFIG: %+v, erwartet %q", env, want)
	}
	if env.Endpoint != envEndpoint || env.ConfigProblem() != "" {
		t.Fatalf("KEPHALAION_CONFIG: %+v", env)
	}
	assertUserTokens(t, env, userDir)
}

// Zeigt KEPHALAION_CONFIG auf eine fehlende Datei, nennt die Meldung nur
// diesen Pfad und die Quelle — die übrigen Orte gelten dann nicht.
func TestDefaultEnvEnvVariableMissingFile(t *testing.T) {
	userDir, systemPath := locateSetup(t)
	writeFile(t, filepath.Join(userDir, "config.yaml"), userConfig)
	writeFile(t, systemPath, systemConfig)
	missing := filepath.Join(t.TempDir(), "fehlt.yaml")
	t.Setenv(EnvConfigVar, missing)

	env := DefaultEnv()
	if env.ConfigPresent || env.ConfigSource != SourceEnv || env.ConfigPath != missing || env.Endpoint != "" {
		t.Fatalf("fehlende Datei: %+v", env)
	}
	problem := env.ConfigProblem()
	if !strings.Contains(problem, missing) || !strings.Contains(problem, EnvConfigVar) {
		t.Fatalf("Meldung: %q", problem)
	}
	if strings.Contains(problem, userDir) || strings.Contains(problem, systemPath) {
		t.Fatalf("Meldung nennt Orte, die nicht gelten: %q", problem)
	}
	assertUserTokens(t, env, userDir)
	if status := CheckBinding(context.Background(), goodBinding, env); status.State != StateNoKephalaion {
		t.Fatalf("Zustand: %+v", status)
	}
}

// Eine globale config ohne Leserecht gilt als vorhanden: Die Meldung nennt
// ihren Pfad und das fehlende Recht, statt still eine andere zu nehmen.
func TestDefaultEnvSystemWithoutReadRight(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("als root greift die Dateiberechtigung nicht")
	}
	userDir, systemPath := locateSetup(t)
	writeFile(t, systemPath, systemConfig)
	if err := os.Chmod(systemPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(systemPath, 0o600) })

	env := DefaultEnv()
	if env.ConfigSource != SourceSystem || env.ConfigPath != systemPath || !env.ConfigPresent || env.Endpoint != "" {
		t.Fatalf("global ohne Leserecht: %+v", env)
	}
	assertUserTokens(t, env, userDir)
	status := CheckBinding(context.Background(), goodBinding, env)
	if status.State != StateNoKephalaion {
		t.Fatalf("Zustand: %+v", status)
	}
	if !strings.Contains(status.Message, systemPath) || !strings.Contains(status.Message, "Leserecht für diesen User fehlt") {
		t.Fatalf("Meldung: %q", status.Message)
	}
}

// Auf die config des Users kein Zugriff (das Verzeichnis gehört etwa einem
// anderen User): Sie zählt als nicht vorhanden. Gilt dann die globale, kommt
// der Endpunkt aus ihr; gibt es keine, ist Kephalaion nicht eingerichtet.
func TestDefaultEnvUserConfigWithoutAccess(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("als root greift die Dateiberechtigung nicht")
	}
	userDir, systemPath := locateSetup(t)
	writeFile(t, filepath.Join(userDir, "config.yaml"), userConfig)
	if err := os.Chmod(userDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(userDir, 0o755) })

	env := DefaultEnv()
	if env.ConfigPresent || env.ConfigSource != SourceUser || env.ConfigError != "" {
		t.Fatalf("User ohne Zugriff, keine globale: %+v", env)
	}
	if env.TokensDir() != filepath.Join(userDir, "tokens") {
		t.Fatalf("Token-Verzeichnis: %q", env.TokensDir())
	}

	writeFile(t, systemPath, systemConfig)
	env = DefaultEnv()
	if env.ConfigSource != SourceSystem || env.Endpoint != systemEndpoint || env.ConfigProblem() != "" {
		t.Fatalf("User ohne Zugriff, globale da: %+v", env)
	}
	if env.TokensDir() != filepath.Join(userDir, "tokens") {
		t.Fatalf("Token-Verzeichnis: %q", env.TokensDir())
	}
}

// Fehlt das Token, nennt die Meldung das Token-Verzeichnis des Users, auch
// wenn die globale config gilt.
func TestTokenMissingNamesUserTokensDir(t *testing.T) {
	userDir, systemPath := locateSetup(t)
	writeFile(t, systemPath, systemConfig)
	env := DefaultEnv()
	status := CheckBinding(context.Background(), Binding{Hub: "vm", Collection: "k-playbook", Account: "bob"}, env)
	if status.State != StateTokenMissing {
		t.Fatalf("Zustand: %+v", status)
	}
	if !strings.Contains(status.Message, filepath.Join(userDir, "tokens")) || strings.Contains(status.Message, filepath.Dir(systemPath)) {
		t.Fatalf("Meldung: %q", status.Message)
	}
}

// Ohne Heimatverzeichnis gibt es kein Token-Verzeichnis; gelesen wird dann
// auch kein relatives tokens/ im Arbeitsverzeichnis.
func TestEnvWithoutUserDirReadsNoRelativeTokens(t *testing.T) {
	_, systemPath := locateSetup(t)
	writeFile(t, systemPath, systemConfig)
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "tokens", "vm", "kamran-wsl.token"), testToken+"\n")
	t.Chdir(work)

	env := locateEnv("")
	if env.ConfigSource != SourceSystem || env.Endpoint != systemEndpoint {
		t.Fatalf("ohne Heimatverzeichnis: %+v", env)
	}
	if env.TokensDir() != "" {
		t.Fatalf("Token-Verzeichnis: %q", env.TokensDir())
	}
	if candidates, _ := env.Candidates(); len(candidates) != 0 {
		t.Fatalf("relatives tokens/ gelesen: %+v", candidates)
	}
	if status := CheckBinding(context.Background(), goodBinding, env); status.State != StateTokenMissing {
		t.Fatalf("Zustand: %+v", status)
	}

	if err := os.Remove(systemPath); err != nil {
		t.Fatal(err)
	}
	none := locateEnv("")
	if none.ConfigPresent || !strings.Contains(none.ConfigProblem(), systemPath) {
		t.Fatalf("ohne Heimatverzeichnis, keine config: %+v %q", none, none.ConfigProblem())
	}
}
