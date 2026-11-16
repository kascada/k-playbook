package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review"
)

// pausedReviewCalls sind die fünf gesperrten Werkzeuge mit Argumenten, die dem
// Schema genügen und ohne die Sperre läsen oder schrieben: status läse die
// Auswahlbasis und einen Lauf, create legte den Lauf eines neuen Tages an,
// scan führte mockscan aus, merge schriebe review-input.*, und write_ai_entry
// setzte den Eintrag tech auf done.
func pausedReviewCalls(root string) []*mcp.CallToolParams {
	base := reviewBaseInput{ProjectDir: root}
	return []*mcp.CallToolParams{
		{Name: reviewToolStatus, Arguments: reviewStatusInput{reviewBaseInput: base, Mode: "available"}},
		{Name: reviewToolStatus, Arguments: reviewStatusInput{reviewBaseInput: base, Run: "2026-08-19"}},
		{Name: reviewToolCreate, Arguments: reviewCreateInput{reviewBaseInput: base, Day: "2026-08-20"}},
		{Name: reviewToolScan, Arguments: reviewScanInput{reviewBaseInput: base, Run: "2026-08-19"}},
		{Name: reviewToolMerge, Arguments: reviewMergeInput{reviewBaseInput: base, Run: "2026-08-19"}},
		{Name: reviewToolWriteAIEntry, Arguments: reviewWriteAIEntryInput{
			reviewBaseInput: base, Run: "2026-08-19", Entry: "tech", State: review.StateDone, Result: "review-tech.md",
		}},
	}
}

// pausedReviewTools sind die Namen der fünf gesperrten Werkzeuge.
var pausedReviewTools = []string{reviewToolStatus, reviewToolCreate, reviewToolScan, reviewToolMerge, reviewToolWriteAIEntry}

// reviewSession verbindet einen Client mit einem Server, der die
// Review-Werkzeuge so anmeldet wie der echte: über addReviewTools.
func reviewSession(t *testing.T) *mcp.ClientSession {
	t.Helper()

	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	addReviewTools(server)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("Server verbinden: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("Client verbinden: %v", err)
	}
	t.Cleanup(func() { clientSession.Close() })
	return clientSession
}

// treeSnapshot hält jeden Pfad unter root fest: Verzeichnisse mit ihrem
// Modus, Dateien dazu mit der Prüfsumme ihres Inhalts.
func treeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()

	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		state := info.Mode().String()
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			state += fmt.Sprintf(" %x", sha256.Sum256(data))
		}
		snapshot[relative] = state
		return nil
	})
	if err != nil {
		t.Fatalf("%s durchlaufen: %v", root, err)
	}
	return snapshot
}

func assertSameTree(t *testing.T, label string, before map[string]string, after map[string]string) {
	t.Helper()

	for path, state := range after {
		if old, ok := before[path]; !ok {
			t.Errorf("%s: neu entstanden: %s", label, path)
		} else if old != state {
			t.Errorf("%s: verändert: %s", label, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			t.Errorf("%s: verschwunden: %s", label, path)
		}
	}
}

// Alle fünf Review-Werkzeuge antworten mit reviews_paused und dem Satz mit
// dem Grund, als Fehler markiert — und an einem vorhandenen Lauf ändert sich
// nichts. Der Umschlag nennt nur das übergebene projectDir.
func TestGesperrteReviewWerkzeugeAntwortenMitSperre(t *testing.T) {
	root := newReviewProject(t)
	runDir := mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, aiEntry("tech")})
	mustWriteFile(t, filepath.Join(runDir, "review-tech.md"), "# Ergebnis\n")
	session := reviewSession(t)

	before := treeSnapshot(t, root)
	called := map[string]bool{}
	for _, params := range pausedReviewCalls(root) {
		called[params.Name] = true
		result, err := session.CallTool(context.Background(), params)
		if err != nil {
			t.Fatalf("%s: CallTool: %v", params.Name, err)
		}
		if !result.IsError {
			t.Errorf("%s: Ergebnis nicht als Fehler markiert", params.Name)
		}
		envelope := decodeReviewEnvelope(t, result)
		if envelope.OK || envelope.Error == nil {
			t.Errorf("%s: keine Sperre: %+v", params.Name, envelope)
			continue
		}
		if envelope.Error.Code != review.PausedCode || envelope.Error.Message != review.PausedMessage {
			t.Errorf("%s: Fehler = %+v", params.Name, envelope.Error)
		}
		if envelope.Tool != params.Name || envelope.Project.InputDir != root || envelope.Project.Root != "" {
			t.Errorf("%s: Umschlag = %+v", params.Name, envelope)
		}
	}
	for _, name := range pausedReviewTools {
		if !called[name] {
			t.Errorf("%s wurde nicht geprüft", name)
		}
	}
	assertSameTree(t, "vorhandener Lauf", before, treeSnapshot(t, root))
}

// Durch die gesperrten Werkzeuge entsteht nichts, auch kein
// Ergebnisverzeichnis. Die Sperre greift vor dem Auflösen des Projekts — auch
// ein projectDir ohne Projekt, ein leeres oder ein fehlendes bekommt sie.
func TestGesperrteReviewWerkzeugeLegenNichtsAn(t *testing.T) {
	root := newReviewProject(t)
	if err := os.RemoveAll(reviewResultsDir(root)); err != nil {
		t.Fatalf("Ergebnisverzeichnis entfernen: %v", err)
	}
	session := reviewSession(t)

	before := treeSnapshot(t, root)
	for _, dir := range []string{root, filepath.Join(root, "gibt-es-nicht"), ""} {
		for _, params := range pausedReviewCalls(dir) {
			result, err := session.CallTool(context.Background(), params)
			if err != nil {
				t.Fatalf("%s: CallTool: %v", params.Name, err)
			}
			assertToolErrorCode(t, result, review.PausedCode)
		}
	}
	assertSameTree(t, "ohne Ergebnisverzeichnis", before, treeSnapshot(t, root))
}

// Die Werkzeugliste nennt die Sperre vorn in der Beschreibung aller fünf
// Review-Werkzeuge; ein offenes Statuswerkzeug gibt es nicht.
func TestReviewWerkzeuglisteNenntDieSperre(t *testing.T) {
	session := reviewSession(t)

	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	paused := map[string]bool{}
	for _, name := range pausedReviewTools {
		paused[name] = true
	}
	seen := 0
	for _, tool := range listed.Tools {
		if !paused[tool.Name] {
			continue
		}
		seen++
		if !strings.HasPrefix(tool.Description, "Vorübergehend nicht verfügbar") || !strings.Contains(tool.Description, review.PausedCode) {
			t.Errorf("%s: Beschreibung nennt die Sperre nicht vorn: %q", tool.Name, tool.Description)
		}
	}
	if seen != 5 {
		t.Errorf("Werkzeugliste nennt %d von 5 Review-Werkzeugen", seen)
	}
}

// Ohne den Clone <projekt>/k-playbook/ antwortet der Handler hinter
// k_playbook_review_status wie sonst, statt mit „k-playbook-Installation
// fehlt“; der Umschlag trägt weder playbookDir noch localDir oder
// reviewRunsDir. Über die Werkzeugliste antworten alle fünf Werkzeuge auch
// dort mit reviews_paused (Task 154).
func TestReviewWerkzeugeOhneClone(t *testing.T) {
	root := newReviewProject(t)
	if err := os.RemoveAll(filepath.Join(root, project.PlaybookDirName)); err != nil {
		t.Fatalf("%s entfernen: %v", project.PlaybookDirName, err)
	}

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Mode: "available"})
	if err != nil {
		t.Fatalf("%s: %v", reviewToolStatus, err)
	}
	envelope := decodeReviewEnvelope(t, result)
	if !envelope.OK {
		t.Fatalf("%s ohne Clone: %+v", reviewToolStatus, envelope.Error)
	}
	if envelope.Project.Root != root {
		t.Errorf("Projekt = %+v, erwartet root %s", envelope.Project, root)
	}
	for _, content := range result.Content {
		text, ok := content.(*mcp.TextContent)
		if !ok {
			continue
		}
		var raw struct {
			Project map[string]any `json:"project"`
		}
		if err := json.Unmarshal([]byte(text.Text), &raw); err != nil {
			t.Fatalf("Antwort nicht lesbar: %v", err)
		}
		for _, key := range []string{"playbookDir", "localDir", "reviewRunsDir"} {
			if _, found := raw.Project[key]; found {
				t.Errorf("Umschlag trägt noch %s: %v", key, raw.Project)
			}
		}
	}

	session := reviewSession(t)
	for _, params := range pausedReviewCalls(root) {
		result, err := session.CallTool(context.Background(), params)
		if err != nil {
			t.Fatalf("%s: CallTool: %v", params.Name, err)
		}
		assertToolErrorCode(t, result, review.PausedCode)
	}
}

// Ohne die Naht kennt das Programm keinen Ort der Review-Ergebnisse: Der
// Handler hinter der Sperre endet beim Auflösen der Umgebung mit
// results_dir_undecided und dem Satz von review.ErrResultsDirUndecided, und
// im Projekt entsteht nichts.
func TestReviewHandlerOhneOrtMeldetUnentschieden(t *testing.T) {
	root := newReviewProject(t)
	setReviewResultsDir(t, "")

	before := treeSnapshot(t, root)
	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Mode: "available"})
	if err != nil {
		t.Fatalf("%s: %v", reviewToolStatus, err)
	}
	assertToolErrorCode(t, result, resultsDirUndecidedCode)
	if envelope := decodeReviewEnvelope(t, result); envelope.Error == nil || envelope.Error.Message != review.ErrResultsDirUndecided.Error() {
		t.Errorf("Fehler = %+v", envelope.Error)
	}
	assertSameTree(t, "ohne Ort", before, treeSnapshot(t, root))
}
