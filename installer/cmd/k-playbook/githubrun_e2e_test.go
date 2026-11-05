package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// runCanary ist das Token, das das falsche gh ausgibt: erkennbar, damit
// jeder Test prüfen kann, dass es in keiner Ausgabe von run steht.
const runCanary = "gho_KANARIENVOGEL_run_0123456789abcdef"

const githubRunFile = `services:
  website:
    repo: acme/website
    access:
      - type: gh
        login: alice
  backend:
    repo: acme-labs/backend
    access:
      - type: ssh
        host: github-example
      - type: gh
        login: alice
  nurssh:
    repo: acme/nurssh
    access:
      - type: ssh
        host: github-example
  aus:
    repo: acme/aus
    access:
      - type: gh
        login: alice
        disabled: true
  kaputt:
    repo: acme/kaputt
    access:
      - type: gh
        login: broken
  offen:
    repo: acme/offen
    access:
      - type: public
`

// githubRunSetup ist die Umgebung eines run-Prozesses im Test: eigenes
// HOME mit XDG_CONFIG_HOME, eine gh-Konfiguration mit alice und broken, ein
// falsches gh, das seine Aufrufe protokolliert und für alice das Token
// ausgibt (für broken gibt es das Token aus und scheitert), ein falsches
// ssh, das nur -G beantwortet, und das echte git, sofern vorhanden.
type githubRunSetup struct {
	home   string
	config string
	bin    string
	ghLog  string
	work   string
	env    []string
}

func newGitHubRunSetup(t *testing.T, content string) githubRunSetup {
	t.Helper()
	s := githubRunSetup{home: t.TempDir(), bin: t.TempDir(), work: t.TempDir()}
	s.config = filepath.Join(t.TempDir(), githubaccess.FileName)
	s.ghLog = filepath.Join(t.TempDir(), "gh.log")
	if content != "" {
		if err := os.WriteFile(s.config, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ghDir := t.TempDir()
	hosts := "github.com:\n    user: alice\n    oauth_token: gho_x\n    users:\n        alice:\n            oauth_token: gho_x\n        broken:\n            oauth_token: gho_y\n"
	if err := os.WriteFile(filepath.Join(ghDir, "hosts.yml"), []byte(hosts), 0o600); err != nil {
		t.Fatal(err)
	}
	gh := "#!/bin/sh\necho \"$*\" >> " + s.ghLog + "\n" +
		"if [ \"$1 $2\" = \"auth token\" ]; then\n" +
		"  case \"$6\" in alice) echo " + runCanary + "; exit 0 ;; broken) echo " + runCanary + "; echo \"gh: kaputt " + runCanary + "\" >&2; exit 1 ;; esac\n" +
		"fi\nexit 0\n"
	ssh := "#!/bin/sh\nif [ \"$1\" = -G ]; then case \"$2\" in github-example|github.com) echo \"hostname github.com\" ;; *) echo \"hostname $2\" ;; esac; exit 0; fi\nexit 255\n"
	for name, body := range map[string]string{"gh": gh, "ssh": ssh} {
		if err := os.WriteFile(filepath.Join(s.bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if git, err := exec.LookPath("git"); err == nil {
		if err := os.Symlink(git, filepath.Join(s.bin, "git")); err != nil {
			t.Fatal(err)
		}
	}
	s.env = []string{
		"PATH=" + s.bin + ":/usr/bin:/bin",
		"HOME=" + s.home,
		"XDG_CONFIG_HOME=" + filepath.Join(s.home, ".config"),
		"GH_CONFIG_DIR=" + ghDir,
		"GIT_CONFIG_NOSYSTEM=1",
		githubaccess.EnvConfig + "=" + s.config,
		"LANG=C",
	}
	return s
}

// command baut den Aufruf des Test-Binarys als k-playbook github run args.
func (s githubRunSetup) command(t *testing.T, extraEnv []string, args ...string) *exec.Cmd {
	t.Helper()
	return s.commandArgs(t, extraEnv, append([]string{"github", "run"}, args...)...)
}

// commandArgs baut den Aufruf des Test-Binarys als k-playbook mit args, in
// derselben Umgebung wie run — etwa für github show.
func (s githubRunSetup) commandArgs(t *testing.T, extraEnv []string, args ...string) *exec.Cmd {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(append(append([]string{}, s.env...), extraEnv...), mainArgsEnv+"="+string(raw))
	cmd.Dir = s.work
	return cmd
}

// run führt `k-playbook github run args…` aus und liefert stdout, stderr
// und Exit-Code. Kein Ergebnis darf das Token tragen.
func (s githubRunSetup) run(t *testing.T, stdin string, args ...string) (string, string, int) {
	t.Helper()
	cmd := s.command(t, nil, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("Prozess: %v", err)
	}
	for name, text := range map[string]string{"stdout": stdout.String(), "stderr": stderr.String()} {
		if strings.Contains(text, runCanary[:12]) {
			t.Errorf("Token auf %s von run: %q", name, strings.ReplaceAll(text, runCanary, "[token]"))
		}
	}
	return stdout.String(), stderr.String(), code
}

// snapshot hält Pfad und Prüfsumme jeder Datei unter dir fest.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = fmt.Sprintf("%x", sha256.Sum256(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestGitHubRunReichtExitCodeUndStdoutDurch(t *testing.T) {
	s := newGitHubRunSetup(t, githubRunFile)

	stdout, stderr, code := s.run(t, "", "website", "--", "sh", "-c", "echo aus dem Befehl; exit 3")
	if code != 3 {
		t.Errorf("Exit-Code %d, erwartet 3", code)
	}
	if stdout != "aus dem Befehl\n" {
		t.Errorf("stdout %q", stdout)
	}
	if stderr != "github run website: git über gh alice, api über gh\n" {
		t.Errorf("stderr %q", stderr)
	}

	// stdin und Arbeitsverzeichnis kommen beim Befehl an.
	stdout, _, code = s.run(t, "hallo\n", "website", "--", "sh", "-c", "cat; pwd")
	if code != 0 || stdout != "hallo\n"+s.work+"\n" {
		t.Errorf("stdin/pwd: %d %q", code, stdout)
	}

	// Der Befehl sieht das Token in GH_TOKEN und GITHUB_TOKEN und die
	// Einrichtung für git; ein Argument trägt es nicht.
	tokenFile := filepath.Join(t.TempDir(), "erwartet")
	if err := os.WriteFile(tokenFile, []byte(runCanary), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `t=$(cat "$1"); [ "$GH_TOKEN" = "$t" ] && [ "$GITHUB_TOKEN" = "$t" ] && [ "$GIT_TERMINAL_PROMPT" = 0 ] && [ -n "$GIT_CONFIG_COUNT" ]`
	if _, stderr, code := s.run(t, "", "website", "--", "sh", "-c", script, "_", tokenFile); code != 0 {
		t.Errorf("Umgebung des Befehls: Exit-Code %d, stderr %q", code, stderr)
	}
}

func TestGitHubRunWahlUndQuiet(t *testing.T) {
	s := newGitHubRunSetup(t, githubRunFile)
	cases := []struct {
		args []string
		line string
	}{
		{[]string{"backend"}, "github run backend: git über ssh host github-example, api über gh alice"},
		{[]string{"backend", "--skip", "ssh"}, "github run backend: git über gh alice, api über gh"},
		{[]string{"backend", "--use=ssh"}, "github run backend: git über ssh host github-example, api: keiner"},
		{[]string{"nurssh"}, "github run nurssh: git über ssh host github-example, api: keiner"},
		{[]string{"offen"}, "github run offen: git über public, api: keiner"},
	}
	for _, c := range cases {
		args := append(append([]string{}, c.args...), "--", "true")
		_, stderr, code := s.run(t, "", args...)
		if code != 0 || stderr != c.line+"\n" {
			t.Errorf("%v: %d %q, erwartet %q", c.args, code, stderr, c.line)
		}
	}
	// Ohne api-Kanal sieht der Befehl kein geerbtes Token.
	cmd := s.command(t, []string{"GH_TOKEN=geerbt", "GITHUB_TOKEN=geerbt"}, "nurssh", "--quiet", "--", "sh", "-c", `[ -z "${GH_TOKEN+x}" ] && [ -z "${GITHUB_TOKEN+x}" ]`)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil || stderr.Len() != 0 {
		t.Errorf("geerbtes Token oder Ausgabe trotz --quiet: %v %q", err, stderr.String())
	}
}

func TestGitHubRunEigeneFehlerEndenMit125(t *testing.T) {
	s := newGitHubRunSetup(t, githubRunFile)
	cases := []struct {
		name  string
		args  []string
		parts []string
	}{
		{"unbekannter Dienst", []string{"nichts", "--", "true"}, []string{"unbekannter Dienst nichts", "bekannt: website, backend, nurssh, aus, kaputt, offen"}},
		{"ohne --", []string{"website", "true"}, []string{"zweiter Dienst true", "Aufruf: k-playbook github run"}},
		{"ohne -- und Befehl", []string{"website"}, []string{"-- vor dem Befehl fehlt"}},
		{"ohne Befehl", []string{"website", "--"}, []string{"nach -- fehlt der Befehl"}},
		{"ohne Dienst", []string{"--", "true"}, []string{"der Dienst fehlt"}},
		{"unbekannte Option", []string{"website", "--schnell", "--", "true"}, []string{"unbekannte Option --schnell"}},
		{"unbekannter Typ", []string{"website", "--use", "https", "--", "true"}, []string{"unbekannter Typ https bei --use", "gh, token, ssh und public"}},
		{"--skip ohne Typ", []string{"website", "--skip", "--", "true"}, []string{"--skip braucht einen Typ"}},
		{"zwei --use", []string{"website", "--use", "gh", "--use", "ssh", "--", "true"}, []string{"--use ist schon gh"}},
		{"kein Zugang", []string{"aus", "--", "true"}, []string{"kein verwendbarer Zugang für aus", "access[0] gh: deaktiviert"}},
		{"--write mit nur public", []string{"offen", "--write", "--", "true"}, []string{"kein verwendbarer Zugang für offen", "durch --write ausgeschlossen"}},
		{"--use gh ohne gh", []string{"nurssh", "--use", "gh", "--", "true"}, []string{"durch --use gh ausgeschlossen"}},
		{"gh auth token scheitert", []string{"kaputt", "--", "true"}, []string{"das Token für kaputt ist nicht zu bekommen", "gh auth token für broken", "gh: kaputt [token]"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stdout, stderr, code := s.run(t, "", c.args...)
			if code != 125 {
				t.Errorf("Exit-Code %d, erwartet 125; stderr %q", code, stderr)
			}
			if stdout != "" {
				t.Errorf("stdout %q", stdout)
			}
			if strings.Count(stderr, "\n") != 1 || !strings.HasPrefix(stderr, "Fehler: github run: ") {
				t.Errorf("keine einzelne Meldung: %q", stderr)
			}
			for _, part := range c.parts {
				if !strings.Contains(stderr, part) {
					t.Errorf("Meldung ohne %q: %q", part, stderr)
				}
			}
		})
	}

	// --help ist kein Fehler.
	if stdout, _, code := s.run(t, "", "--help"); code != 0 || !strings.Contains(stdout, "k-playbook github run <dienst>") {
		t.Errorf("--help: %d %q", code, stdout)
	}
}

func TestGitHubRunDefekteUndFehlendeDatei(t *testing.T) {
	s := newGitHubRunSetup(t, githubBroken)
	_, stderr, code := s.run(t, "", "website", "--", "true")
	if code != 125 || !strings.Contains(stderr, s.config+" hat 1 Problem, zuerst services.backend.access[1]: Der Pfad in file ist relativ.") || !strings.Contains(stderr, "k-playbook github check") {
		t.Errorf("defekte Datei: %d %q", code, stderr)
	}

	// Fehlt die Datei, legt run wie jedes Lesen die Vorlage an und endet
	// mit 125, weil sie keinen Dienst nennt.
	s = newGitHubRunSetup(t, "")
	_, stderr, code = s.run(t, "", "website", "--", "true")
	if code != 125 || !strings.Contains(stderr, "unbekannter Dienst website: "+s.config+" wurde eben als Vorlage angelegt") {
		t.Errorf("fehlende Datei: %d %q", code, stderr)
	}
	if info, err := os.Stat(s.config); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("Vorlage nicht angelegt: %v", err)
	}
}

func TestGitHubRunBefehlNichtGefundenOderNichtAusfuehrbar(t *testing.T) {
	s := newGitHubRunSetup(t, githubRunFile)
	if _, stderr, code := s.run(t, "", "website", "--", "gibt-es-nicht"); code != 127 || !strings.Contains(stderr, "Befehl gibt-es-nicht nicht gefunden") {
		t.Errorf("nicht gefunden: %d %q", code, stderr)
	}
	if _, stderr, code := s.run(t, "", "website", "--", "./fehlt.sh"); code != 127 || !strings.Contains(stderr, "./fehlt.sh nicht gefunden") {
		t.Errorf("Pfad nicht gefunden: %d %q", code, stderr)
	}
	// Ein leerer Name (`-- "$CMD"` mit leerem CMD) ist nicht gefunden wie
	// bei env, nicht ein PATH-Verzeichnis, das nicht ausführbar wäre.
	if _, stderr, code := s.run(t, "", "website", "--", "", "arg"); code != 127 || stderr != "Fehler: github run: der Befehl ist leer — nach -- einen Befehl nennen\n" {
		t.Errorf("leerer Befehl: %d %q", code, stderr)
	}
	if err := os.WriteFile(filepath.Join(s.work, "skript.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := s.run(t, "", "website", "--", "./skript.sh"); code != 126 || !strings.Contains(stderr, "./skript.sh ist nicht ausführbar") {
		t.Errorf("nicht ausführbar per Pfad: %d %q", code, stderr)
	}
	if err := os.WriteFile(filepath.Join(s.bin, "ohne-x"), []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := s.run(t, "", "website", "--", "ohne-x"); code != 126 || !strings.Contains(stderr, "ohne-x ist nicht ausführbar") {
		t.Errorf("nicht ausführbar im PATH: %d %q", code, stderr)
	}
	if _, stderr, code := s.run(t, "", "website", "--", s.work); code != 126 {
		t.Errorf("Verzeichnis: %d %q", code, stderr)
	}
}

// Außerhalb des Befehls ändert sich nichts: die globale git-Konfiguration
// samt .git-credentials unter HOME, die Zugangsdatei byte-gleich, und gh
// wurde nur nach dem Token gefragt, nie umgeschaltet. Der Befehl selbst
// meldet sich dabei mit dem Token bei git an (credential fill und approve
// mit dem store-Helper des Rechners).
func TestGitHubRunAendertNichtsAusserhalb(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git liegt nicht im PATH")
	}
	s := newGitHubRunSetup(t, githubRunFile)
	gitConfig := exec.Command("git", "config", "--global", "credential.helper", "store")
	gitConfig.Env = s.env
	if out, err := gitConfig.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(s.home, ".git-credentials"), []byte("https://gespeichert:STOREDPW@github.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	homeBefore := snapshot(t, s.home)
	fileBefore, err := os.ReadFile(s.config)
	if err != nil {
		t.Fatal(err)
	}

	script := `set -e
printf 'url=https://github.com/acme/website.git\n\n' | git credential fill > fill.out
grep -q "^password=$GH_TOKEN\$" fill.out
printf 'url=https://github.com/acme/website.git\nusername=x-access-token\npassword=%s\n\n' "$GH_TOKEN" | git credential approve
rm fill.out
[ "$(git ls-remote --get-url git@github.com:acme/website.git)" = https://github.com/acme/website.git ]`
	if _, stderr, code := s.run(t, "", "website", "--quiet", "--", "sh", "-c", script); code != 0 {
		t.Fatalf("Befehl: Exit-Code %d, stderr %q", code, stderr)
	}
	if _, stderr, code := s.run(t, "", "backend", "--skip", "ssh", "--", "true"); code != 0 {
		t.Fatalf("backend: %d %q", code, stderr)
	}

	homeAfter := snapshot(t, s.home)
	if fmt.Sprint(homeBefore) != fmt.Sprint(homeAfter) {
		t.Errorf("HOME verändert:\nvorher %v\nnachher %v", homeBefore, homeAfter)
	}
	if fileAfter, _ := os.ReadFile(s.config); !bytes.Equal(fileBefore, fileAfter) {
		t.Errorf("github.yaml verändert")
	}
	log, _ := os.ReadFile(s.ghLog)
	for _, line := range strings.Split(strings.TrimSpace(string(log)), "\n") {
		if line != "auth token --hostname github.com --user alice" {
			t.Errorf("unerwarteter gh-Aufruf: %q", line)
		}
	}
	if strings.Contains(string(log), "switch") {
		t.Errorf("gh auth switch aufgerufen")
	}
}

// waitForMarker wartet, bis der Befehl die Datei marker angelegt hat.
func waitForMarker(t *testing.T, cmd *exec.Cmd, marker string, stderr *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			t.Fatalf("der Befehl ist nicht gestartet; stderr %q", stderr.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// waitWithin wartet höchstens zehn Sekunden auf das Ende von cmd und
// liefert den Code, den eine Shell als Aufrufer sähe: den Exit-Code, nach
// einem Ende durch Signal 128 + Signal.
func waitWithin(t *testing.T, cmd *exec.Cmd, what string) int {
	t.Helper()
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case err := <-waited:
		if err == nil {
			return 0
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("Prozess: %v", err)
		}
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		return exitErr.ExitCode()
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("run ist nach %s nicht beendet", what)
	}
	return -1
}

// SIGTERM an run erreicht den Befehl: run hat sich durch ihn ersetzt, der
// Aufrufer sieht das Ende durch SIGTERM, eine Shell also 128 + 15.
func TestGitHubRunGibtSIGTERMWeiter(t *testing.T) {
	s := newGitHubRunSetup(t, githubRunFile)
	marker := filepath.Join(t.TempDir(), "bereit")
	cmd := s.command(t, nil, "website", "--quiet", "--", "sh", "-c", `echo bereit > "$1"; exec sleep 30`, "_", marker)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitForMarker(t, cmd, marker, &stderr)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if code := waitWithin(t, cmd, "SIGTERM"); code != 128+15 {
		t.Errorf("Ende mit %d, erwartet 143; stderr %q", code, stderr.String())
	}
}

// Ein Strg-C ist ein SIGINT an die ganze Prozessgruppe des Terminals. Es
// erreicht den Befehl genau einmal — run gibt nichts weiter, es ist der
// Befehl. Der Befehl zählt jedes SIGINT in einer Datei; sein sleep läuft im
// Hintergrund und ignoriert SIGINT deshalb, wie POSIX es für eine
// nicht-interaktive Shell vorsieht.
func TestGitHubRunSIGINTAnDieGruppeKommtEinmalAn(t *testing.T) {
	s := newGitHubRunSetup(t, githubRunFile)
	dir := t.TempDir()
	marker, log := filepath.Join(dir, "bereit"), filepath.Join(dir, "sigint.log")
	script := `trap 'echo INT >> "$2"' INT
sleep 30 &
echo bereit > "$1"
while :; do wait; done`
	cmd := s.command(t, nil, "website", "--quiet", "--", "sh", "-c", script, "_", marker, log)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	group := cmd.Process.Pid
	defer func() { _ = syscall.Kill(-group, syscall.SIGKILL) }()
	waitForMarker(t, cmd, marker, &stderr)
	if err := syscall.Kill(-group, syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	// Ein zweites, weitergegebenes SIGINT käme in dieser Zeit an.
	time.Sleep(500 * time.Millisecond)
	if err := syscall.Kill(-group, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if code := waitWithin(t, cmd, "SIGTERM"); code != 128+15 {
		t.Errorf("Ende mit %d, erwartet 143; stderr %q", code, stderr.String())
	}
	data, _ := os.ReadFile(log)
	if got := strings.Count(string(data), "INT\n"); got != 1 {
		t.Errorf("SIGINT %d-mal beim Befehl angekommen, erwartet einmal", got)
	}
}

// run wählt wie selected: Je Dienst der Fixture nennt die Zeile von run
// ohne Optionen und mit --write dieselben Zugänge wie selected.read bzw.
// selected.write aus github show --json in derselben Umgebung. Endet run
// mit „kein verwendbarer Zugang“, sind dort beide Kanäle null, und
// umgekehrt; ein anderer Fehler von run (kaputt: Token nicht zu bekommen)
// lässt selected gesetzt.
func TestGitHubRunWaehltWieSelected(t *testing.T) {
	s := newGitHubRunSetup(t, githubRunFile)
	cmd := s.commandArgs(t, nil, "github", "show", "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("show --json: %v %s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), runCanary[:12]) {
		t.Fatalf("Token in show --json")
	}
	views := githubSelectionViews(t, stdout.String(), false)
	if len(views) != 6 {
		t.Fatalf("Dienste: %+v", views)
	}

	seen := map[string]bool{}
	for _, view := range views {
		for _, write := range []bool{false, true} {
			index, args, mode := view.selected.Read, []string{view.name, "--", "true"}, "read"
			if write {
				index, args, mode = view.selected.Write, []string{view.name, "--write", "--", "true"}, "write"
			}
			_, stderr, code := s.run(t, "", args...)
			noAccess := strings.Contains(stderr, "kein verwendbarer Zugang für "+view.name)
			switch {
			case index.Empty() != noAccess:
				t.Errorf("%s %s: selected %s, run %d %q", view.name, mode, describeChannelIndex(index), code, stderr)
			case noAccess:
				if code != githubRunExitFailed {
					t.Errorf("%s %s: Exit %d", view.name, mode, code)
				}
				seen[view.name+" "+mode+" leer"] = true
			case code == 0:
				if want := "github run " + view.name + ": " + githubExpectedChoice(view.accesses, index, s.home, false) + "\n"; stderr != want {
					t.Errorf("%s %s: run %q, selected %q", view.name, mode, stderr, want)
				}
			case code == githubRunExitFailed && strings.Contains(stderr, "das Token für "+view.name+" ist nicht zu bekommen"):
				seen[view.name+" "+mode+" token"] = true
			default:
				t.Errorf("%s %s: unerwartet %d %q", view.name, mode, code, stderr)
			}
		}
	}
	for _, want := range []string{"aus read leer", "aus write leer", "offen write leer", "kaputt read token", "kaputt write token"} {
		if !seen[want] {
			t.Errorf("Fall %s nicht gesehen: %v", want, seen)
		}
	}
}
