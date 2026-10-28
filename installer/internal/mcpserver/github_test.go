package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// githubTestSetup gibt dem Test eine Zugangsdatei mit zwei Diensten, einen
// PATH mit Fake-ssh und -git und einen Tester gegen einen httptest.Server.
// Der Dienst docs hat einen public-Zugang (git ls-remote gelingt), der Dienst
// backend einen fehlenden Schlüssel (skipped) und ein deaktiviertes Token.
func githubTestSetup(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	scripts := map[string]string{
		"ssh": "#!/bin/sh\nif [ \"$1\" = \"-G\" ]; then echo \"hostname $2\"; exit 0; fi\nexit 2\n",
		"git": "#!/bin/sh\necho \"abc\tHEAD\"\n",
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("HOME", t.TempDir())

	path := filepath.Join(t.TempDir(), githubaccess.FileName)
	t.Setenv(githubaccess.EnvConfig, path)
	if err := os.WriteFile(path, []byte("services:\n  docs:\n    repo: acme/docs\n    access:\n      - type: public\n  backend:\n    repo: acme/backend\n    access:\n      - type: ssh\n        key: ~/.ssh/fehlt\n      - type: token\n        file: ~/x.token\n        disabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	t.Cleanup(server.Close)
	before := newGitHubTester
	newGitHubTester = func() *githubaccess.Tester {
		return &githubaccess.Tester{APIBase: server.URL, Timeout: 500 * time.Millisecond}
	}
	t.Cleanup(func() { newGitHubTester = before })
	return path
}

func callGitHubTest(t *testing.T, projectDir string, service string) (githubTestEnvelope, map[string]any, *mcp.CallToolResult) {
	t.Helper()
	result, _, err := githubTestToolHandler(context.Background(), nil, githubTestInput{ProjectDir: projectDir, Service: service})
	if err != nil {
		t.Fatalf("github_test: %v", err)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Inhalt ist kein Text: %#v", result.Content[0])
	}
	envelope := githubTestEnvelope{}
	if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v — %s", err, text.Text)
	}
	fields := map[string]any{}
	if err := json.Unmarshal([]byte(text.Text), &fields); err != nil {
		t.Fatal(err)
	}
	return envelope, fields, result
}

// Das Werkzeug liefert den Report von test --json unter test, im üblichen
// Umschlag; mit service nur diesen Dienst.
func TestGitHubTestWerkzeugLiefertDenReport(t *testing.T) {
	path := githubTestSetup(t)
	root := newDocProject(t)

	envelope, fields, result := callGitHubTest(t, root, "")
	if result.IsError || !envelope.OK || envelope.Tool != githubTestTool || envelope.ProjectDir != root || envelope.Test == nil {
		t.Fatalf("Umschlag: %#v", envelope)
	}
	report := *envelope.Test
	if !report.OK || report.File != path || len(report.Services) != 2 || report.Services[0].Name != "docs" {
		t.Errorf("Report: %+v", report)
	}
	if got := report.Services[0].Access[0]; got.Result != githubaccess.ResultOK || got.Identity != githubaccess.IdentityAnonymous || got.Read != githubaccess.AnswerYes {
		t.Errorf("public: %+v", got)
	}
	backend := report.Services[1].Access
	if len(backend) != 2 || backend[0].Result != githubaccess.ResultSkipped || !strings.Contains(backend[0].Message, "Schlüsseldatei fehlt") || backend[1].Result != githubaccess.ResultSkipped || backend[1].Message != "deaktiviert" {
		t.Errorf("backend: %+v", backend)
	}
	if _, embedded := fields["services"]; embedded {
		t.Errorf("Report liegt eingebettet statt unter test: %v", fields)
	}
	if _, present := fields["error"]; present {
		t.Errorf("error steht in einer guten Antwort: %v", fields)
	}

	envelope, _, result = callGitHubTest(t, root, "backend")
	if result.IsError || envelope.Test == nil || len(envelope.Test.Services) != 1 || envelope.Test.Services[0].Name != "backend" || !envelope.Test.OK {
		t.Errorf("mit service: %#v", envelope)
	}
}

// Ein unbekannter Dienst und eine defekte Datei sind Fehler im Umschlag
// ohne Report; ohne projectDir wie bei jedem Werkzeug.
func TestGitHubTestWerkzeugFehler(t *testing.T) {
	path := githubTestSetup(t)
	root := newDocProject(t)

	envelope, _, result := callGitHubTest(t, root, "nope")
	if !result.IsError || envelope.OK || envelope.Test != nil || envelope.Error == nil || envelope.Error.Code != githubErrUnknownService || !strings.Contains(envelope.Error.Message, "docs, backend") {
		t.Errorf("unbekannter Dienst: %#v", envelope)
	}

	if err := os.WriteFile(path, []byte("services:\n  docs:\n    access:\n      - type: public\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	envelope, _, result = callGitHubTest(t, root, "")
	if !result.IsError || envelope.Test != nil || envelope.Error == nil || envelope.Error.Code != githubErrConfig || !strings.Contains(envelope.Error.Message, "services.docs: Dem Dienst fehlt repo.") {
		t.Errorf("defekte Datei: %#v", envelope)
	}

	envelope, _, result = callGitHubTest(t, "", "")
	if !result.IsError || envelope.Error == nil || envelope.Error.Code != "invalid_input" {
		t.Errorf("ohne projectDir: %#v", envelope)
	}
	envelope, _, result = callGitHubTest(t, t.TempDir(), "")
	if !result.IsError || envelope.Error == nil || envelope.Error.Code != "project_not_found" {
		t.Errorf("ohne Projekt: %#v", envelope)
	}
}
