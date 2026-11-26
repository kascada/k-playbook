package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project/defaultstest"
	"github.com/kascada/k-playbook/installer/internal/review/workdir"
)

const workdirCodeRecipe = "---\ntitle: Code-Review\nhandoff: /k-remediation\naudit:\n  enabled: true\n  mode: evidence\n  ruleIds:\n    - code-logic-error\n  scope:\n    paths:\n      - \"**/*.go\"\nreview:\n  enabled: true\n---\n# Code\n"

const workdirCodeSARIF = `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"code"}},"results":[` +
	`{"ruleId":"code-logic-error","level":"error","message":{"text":"Falsche Bedingung."},` +
	`"locations":[{"physicalLocation":{"artifactLocation":{"uri":"main.go"},"region":{"startLine":1}}}]}]}]}`

// Der Weg eines eigenständigen Reviews über die beiden offenen Werkzeuge:
// Arbeitsverzeichnis anlegen, Fundliste hineinlegen, prüfen, zusammenfassen.
// Die Sperre der Audit-Werkzeuge berührt ihn nicht.
func TestReviewWorkdirUndMergeDir(t *testing.T) {
	root := newReviewProject(t)
	mustWriteFile(t, filepath.Join(root, defaultstest.DefaultsDir, "reviews", "review-code.md"), workdirCodeRecipe)
	temp := t.TempDir()
	before := workdir.TempRoot
	workdir.TempRoot = temp
	t.Cleanup(func() { workdir.TempRoot = before })
	session := reviewSession(t)

	result := callReviewTool(t, session, reviewToolWorkdir, reviewWorkdirInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Recipe: "code"})
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("workdir: %+v", envelope.Error)
	}
	data := envelope.Data.(map[string]any)
	dir, _ := data["dir"].(string)
	if filepath.Dir(dir) != temp || data["created"] != true || data["default"] != true {
		t.Fatalf("workdir = %v", data)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("Verzeichnis %s: %v %v", dir, info, err)
	}

	mustWriteFile(t, filepath.Join(dir, "code.sarif"), workdirCodeSARIF)
	result = callReviewTool(t, session, reviewToolMergeDir, reviewMergeDirInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Dir: dir, Check: true})
	if envelope := decodeReviewEnvelope(t, result); !envelope.OK || envelope.Data.(map[string]any)["checkOnly"] != true {
		t.Fatalf("check: %+v", envelope)
	}
	if _, err := os.Stat(filepath.Join(dir, "review-input.json")); !os.IsNotExist(err) {
		t.Fatalf("check hat geschrieben: %v", err)
	}

	result = callReviewTool(t, session, reviewToolMergeDir, reviewMergeDirInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Dir: dir})
	envelope = decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("merge_dir: %+v", envelope.Error)
	}
	data = envelope.Data.(map[string]any)
	if data["findings"] != float64(1) || data["groups"] != float64(1) {
		t.Errorf("merge_dir = %v", data)
	}
	if len(envelope.Warnings) != 1 || !strings.Contains(envelope.Warnings[0], "nicht berücksichtigt") {
		t.Errorf("Warnungen = %v", envelope.Warnings)
	}
	for _, name := range []string{"review-input.json", "review-input.md"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("%s: %v %v", name, info, err)
		}
	}
}

// Eine Fundliste mit einer Rule-ID außerhalb von audit.ruleIds weist der
// Merge mit sarif_invalid ab und schreibt nichts; ein relativer Pfad ist
// invalid_input.
func TestReviewMergeDirWeistAb(t *testing.T) {
	root := newReviewProject(t)
	mustWriteFile(t, filepath.Join(root, defaultstest.DefaultsDir, "reviews", "review-code.md"), workdirCodeRecipe)
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "code.sarif"), strings.Replace(workdirCodeSARIF, "code-logic-error", "code-erfunden", 1))
	session := reviewSession(t)

	result := callReviewTool(t, session, reviewToolMergeDir, reviewMergeDirInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Dir: dir})
	assertToolErrorCode(t, result, "sarif_invalid")
	if _, err := os.Stat(filepath.Join(dir, "review-input.json")); !os.IsNotExist(err) {
		t.Errorf("trotz ungültiger Fundliste geschrieben: %v", err)
	}
	// Eine Perspektive erhebt keine Fundliste.
	mustWriteFile(t, filepath.Join(dir, "code.sarif"), workdirCodeSARIF)
	mustWriteFile(t, filepath.Join(dir, "tech.sarif"), strings.Replace(workdirCodeSARIF, `"name":"code"`, `"name":"tech"`, 1))
	result = callReviewTool(t, session, reviewToolMergeDir, reviewMergeDirInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Dir: dir})
	assertToolErrorCode(t, result, "sarif_invalid")

	result = callReviewTool(t, session, reviewToolMergeDir, reviewMergeDirInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Dir: "relativ"})
	assertToolErrorCode(t, result, "invalid_input")
	result = callReviewTool(t, session, reviewToolWorkdir, reviewWorkdirInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Recipe: "code", Dir: "relativ"})
	assertToolErrorCode(t, result, "invalid_input")
}

func callReviewTool(t *testing.T, session *mcp.ClientSession, name string, arguments any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatalf("%s: CallTool: %v", name, err)
	}
	return result
}
