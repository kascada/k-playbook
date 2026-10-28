package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// githubAccessSetup gibt dem Test eine Zugangsdatei mit zwei Diensten, einen
// PATH mit Fake-ssh (löst github-example auf und grüßt als Deploy Key) und
// Fake-git (ls-remote gelingt), eine leere gh-Konfiguration und einen Tester
// gegen einen httptest.Server, der jedes Token abweist. Der Dienst playbook
// ist nutzbar (Alias und public), backend nicht (fehlender Schlüssel,
// deaktiviertes Token).
func githubAccessSetup(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	scripts := map[string]string{
		"ssh": "#!/bin/sh\nif [ \"$1\" = \"-G\" ]; then case \"$2\" in github-example) echo \"hostname github.com\" ;; *) echo \"hostname $2\" ;; esac; exit 0; fi\n" +
			"echo \"Hi example/k-playbook! You've successfully authenticated, but GitHub does not provide shell access.\" >&2; exit 1\n",
		"git": "#!/bin/sh\necho \"abc\tHEAD\"\n",
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GH_CONFIG_DIR", t.TempDir())

	path := filepath.Join(t.TempDir(), githubaccess.FileName)
	t.Setenv(githubaccess.EnvConfig, path)
	if err := os.WriteFile(path, []byte("services:\n  playbook:\n    repo: example/k-playbook\n    access:\n      - type: ssh\n        host: github-example\n      - type: public\n  backend:\n    repo: acme/backend\n    access:\n      - type: ssh\n        key: ~/.ssh/fehlt\n      - type: token\n        file: ~/x.token\n        disabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	t.Cleanup(server.Close)
	before := newGitHubAccessTester
	newGitHubAccessTester = func() *githubaccess.Tester {
		return &githubaccess.Tester{APIBase: server.URL, Timeout: 500 * time.Millisecond}
	}
	t.Cleanup(func() { newGitHubAccessTester = before })
	return path
}

func githubAccessGet(t *testing.T) (int, githubAccessResponse, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	routes(&serverState{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/github-access", nil))
	var response githubAccessResponse
	if recorder.Code == http.StatusOK {
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("Antwort nicht lesbar: %v — %s", err, recorder.Body.String())
		}
	}
	return recorder.Code, response, recorder.Body.String()
}

func githubAccessPost(t *testing.T, body string) (int, string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/github-access/test", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	routes(&serverState{}).ServeHTTP(recorder, request)
	return recorder.Code, recorder.Body.String()
}

// GET liefert das Objekt von check --json mit der Verfügbarkeit je Zugang,
// dazu available; die Dienste in Dateireihenfolge, ohne Netz.
func TestGitHubAccessLiefertCheckMitVerfuegbarkeit(t *testing.T) {
	path := githubAccessSetup(t)

	code, response, body := githubAccessGet(t)
	if code != http.StatusOK || !response.Available || response.File != path || response.Created || len(response.Problems) != 0 {
		t.Fatalf("Antwort: %d %s", code, body)
	}
	if len(response.Services) != 2 || response.Services[0].Name != "playbook" || response.Services[1].Name != "backend" {
		t.Fatalf("Dienste: %+v", response.Services)
	}
	playbook := response.Services[0].Access
	if playbook[0].State != githubaccess.StateAvailable || playbook[1].State != githubaccess.StateAvailable {
		t.Errorf("playbook: %+v", playbook)
	}
	backend := response.Services[1].Access
	if backend[0].State != githubaccess.StateUnavailable || !strings.Contains(backend[0].Reason, "Schlüsseldatei fehlt") || backend[1].State != githubaccess.StateDisabled {
		t.Errorf("backend: %+v", backend)
	}
	for _, want := range []string{`"available":true`, `"file":"` + path + `"`, `"created":false`, `"problems":[]`, `"state":"available"`, `"state":"unavailable"`, `"reason":"`, `"state":"disabled"`} {
		if !strings.Contains(body, want) {
			t.Errorf("Antwort enthält %s nicht: %s", want, body)
		}
	}
}

// Eine Datei, die sich nicht lesen lässt — hier ein Verzeichnis unter dem
// Pfad —, ist available false mit message, wie bei jedem GET der
// Oberfläche; problems trägt trotzdem, was der Leser sagt. Eine Datei mit
// einem Fehler im Inhalt bleibt available: die Seite zeigt die Probleme.
func TestGitHubAccessBeiNichtLesbarerDatei(t *testing.T) {
	githubAccessSetup(t)
	t.Setenv(githubaccess.EnvConfig, t.TempDir())

	code, response, body := githubAccessGet(t)
	if code != http.StatusOK || response.Available || !strings.Contains(response.Message, "nicht lesbar") || len(response.Problems) != 1 || response.Problems[0].Path != githubaccess.PathFile {
		t.Errorf("Verzeichnis: %d %s", code, body)
	}
	if !strings.Contains(body, `"services":{}`) {
		t.Errorf("services fehlt oder ist nicht leer: %s", body)
	}

	path := filepath.Join(t.TempDir(), githubaccess.FileName)
	t.Setenv(githubaccess.EnvConfig, path)
	if err := os.WriteFile(path, []byte("services:\n  web:\n    access:\n      - type: public\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, response, body = githubAccessGet(t)
	if code != http.StatusOK || !response.Available || response.Message != "" || len(response.Problems) != 1 || response.Problems[0].Path != "services.web" || len(response.Services) != 0 {
		t.Errorf("Inhaltsfehler: %d %s", code, body)
	}
}

// POST mit service testet nur diesen Dienst und antwortet mit dem Objekt von
// test --json; ohne Rumpf alle Dienste. Nicht verfügbare und deaktivierte
// Zugänge sind skipped.
func TestGitHubAccessTestLiefertDenReport(t *testing.T) {
	path := githubAccessSetup(t)

	code, body := githubAccessPost(t, `{"service":"playbook"}`)
	if code != http.StatusOK {
		t.Fatalf("Status = %d: %s", code, body)
	}
	var report githubaccess.Report
	if err := json.Unmarshal([]byte(body), &report); err != nil {
		t.Fatalf("Report nicht lesbar: %v — %s", err, body)
	}
	if !report.OK || report.File != path || len(report.Services) != 1 || report.Services[0].Name != "playbook" {
		t.Fatalf("Report: %+v", report)
	}
	access := report.Services[0].Access
	if access[0].Result != githubaccess.ResultOK || access[0].Identity != "example/k-playbook" || access[0].Read != githubaccess.AnswerYes || access[0].Write != githubaccess.AnswerUnknown ||
		access[1].Result != githubaccess.ResultOK || access[1].Identity != githubaccess.IdentityAnonymous {
		t.Errorf("Zugänge: %+v", access)
	}
	for _, want := range []string{`"file":"` + path + `"`, `"ok":true`, `"result":"ok"`, `"expires":"none"`} {
		if !strings.Contains(body, want) {
			t.Errorf("Antwort enthält %s nicht: %s", want, body)
		}
	}

	code, body = githubAccessPost(t, "")
	if err := json.Unmarshal([]byte(body), &report); err != nil || code != http.StatusOK {
		t.Fatalf("ohne Rumpf: %d %v %s", code, err, body)
	}
	if len(report.Services) != 2 || !report.OK {
		t.Fatalf("alle Dienste: %+v", report)
	}
	backend := report.Services[1].Access
	if backend[0].Result != githubaccess.ResultSkipped || !strings.HasPrefix(backend[0].Message, "nicht verfügbar — ") || backend[1].Result != githubaccess.ResultSkipped || backend[1].Message != "deaktiviert" {
		t.Errorf("backend: %+v", backend)
	}
}

// Ein unbekannter Dienst ist eine Fehlerantwort, die die bekannten nennt, ohne
// Testergebnisse — wie Exit 2 der CLI; eine defekte Datei ihre Probleme; ein
// unlesbarer Rumpf 400.
func TestGitHubAccessTestFehler(t *testing.T) {
	path := githubAccessSetup(t)

	code, body := githubAccessPost(t, `{"service":"nope"}`)
	var failure githubAccessTestError
	if err := json.Unmarshal([]byte(body), &failure); err != nil {
		t.Fatalf("Fehler nicht lesbar: %v — %s", err, body)
	}
	if code != http.StatusBadRequest || !strings.Contains(failure.Message, "Unbekannter Dienst nope") || strings.Join(failure.Known, ",") != "playbook,backend" || strings.Contains(body, `"result"`) {
		t.Errorf("unbekannter Dienst: %d %s", code, body)
	}

	if code, body := githubAccessPost(t, `{"service":`); code != http.StatusBadRequest || !strings.Contains(body, "nicht lesbar") {
		t.Errorf("kaputter Rumpf: %d %s", code, body)
	}

	if err := os.WriteFile(path, []byte("services:\n  web:\n    access:\n      - type: public\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, body = githubAccessPost(t, "")
	if err := json.Unmarshal([]byte(body), &failure); err != nil {
		t.Fatal(err)
	}
	if code != http.StatusConflict || len(failure.Problems) != 1 || failure.Problems[0].Path != "services.web" || strings.Contains(body, `"result"`) {
		t.Errorf("defekte Datei: %d %s", code, body)
	}
}

// Die Seite selbst lädt, trägt beide Karten und bindet ihr Skript ein.
func TestGitHubAccessSeite(t *testing.T) {
	status, body := getPage(t, githubAccessPage)
	if status != http.StatusOK {
		t.Fatalf("Status = %d", status)
	}
	for _, want := range []string{`id="access-file-card"`, `id="access-services-card"`, `id="access-test-all"`, `/static/github-access.js`, `<h1>GitHub-Zugänge</h1>`} {
		if !strings.Contains(body, want) {
			t.Errorf("Seite enthält %s nicht", want)
		}
	}
}
