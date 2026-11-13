package mcpserver

import (
	"context"
	"crypto/sha256"
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

// pausedReviewCalls sind die vier gesperrten Werkzeuge mit Argumenten, die dem
// Schema genügen und ohne die Sperre schrieben: create legte den Lauf eines
// neuen Tages an, scan führte mockscan aus, merge schriebe review-input.*, und
// write_ai_entry setzte den Eintrag tech auf done.
func pausedReviewCalls(root string) []*mcp.CallToolParams {
	base := reviewBaseInput{ProjectDir: root}
	return []*mcp.CallToolParams{
		{Name: reviewToolCreate, Arguments: reviewCreateInput{reviewBaseInput: base, Day: "2026-08-20"}},
		{Name: reviewToolScan, Arguments: reviewScanInput{reviewBaseInput: base, Run: "2026-08-19"}},
		{Name: reviewToolMerge, Arguments: reviewMergeInput{reviewBaseInput: base, Run: "2026-08-19"}},
		{Name: reviewToolWriteAIEntry, Arguments: reviewWriteAIEntryInput{
			reviewBaseInput: base, Run: "2026-08-19", Entry: "tech", State: review.StateDone, Result: "review-tech.md",
		}},
	}
}

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

// Die vier schreibenden Review-Werkzeuge antworten mit reviews_paused und dem
// Satz mit dem Grund, als Fehler markiert — und an einem vorhandenen Lauf
// ändert sich nichts.
func TestGesperrteReviewWerkzeugeAntwortenMitSperre(t *testing.T) {
	root := newReviewProject(t)
	runDir := mustCreateRun(t, root, []review.Entry{{Name: "mockscan", Kind: review.KindTool}, aiEntry("tech")})
	mustWriteFile(t, filepath.Join(runDir, "review-tech.md"), "# Ergebnis\n")
	session := reviewSession(t)

	before := treeSnapshot(t, root)
	for _, params := range pausedReviewCalls(root) {
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
	assertSameTree(t, "vorhandener Lauf", before, treeSnapshot(t, root))
}

// In einem Projekt ohne k-playbook-local/ entsteht durch die gesperrten
// Werkzeuge nichts: kein Verzeichnis, kein results/. Die Sperre greift vor dem
// Auflösen des Projekts — auch ein projectDir ohne Projekt, ein leeres oder
// ein fehlendes bekommt sie.
func TestGesperrteReviewWerkzeugeLegenNichtsAn(t *testing.T) {
	root := newReviewProject(t)
	if err := os.RemoveAll(project.LocalDir(root)); err != nil {
		t.Fatalf("%s entfernen: %v", project.LocalDirName, err)
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
	assertSameTree(t, "ohne "+project.LocalDirName, before, treeSnapshot(t, root))
	if _, err := os.Stat(project.LocalDir(root)); !os.IsNotExist(err) {
		t.Errorf("%s ist entstanden (Stat: %v)", project.LocalDirName, err)
	}
}

// Die Werkzeugliste nennt die Sperre vorn in der Beschreibung der vier
// gesperrten Werkzeuge; status bleibt offen, liest und legt nichts an.
func TestReviewWerkzeuglisteNenntDieSperre(t *testing.T) {
	root := newReviewProject(t)
	if err := os.RemoveAll(project.LocalDir(root)); err != nil {
		t.Fatalf("%s entfernen: %v", project.LocalDirName, err)
	}
	session := reviewSession(t)

	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	paused := map[string]bool{reviewToolCreate: true, reviewToolScan: true, reviewToolMerge: true, reviewToolWriteAIEntry: true}
	seen := 0
	for _, tool := range listed.Tools {
		starts := strings.HasPrefix(tool.Description, "Vorübergehend nicht verfügbar")
		switch {
		case paused[tool.Name]:
			seen++
			if !starts || !strings.Contains(tool.Description, review.PausedCode) {
				t.Errorf("%s: Beschreibung nennt die Sperre nicht vorn: %q", tool.Name, tool.Description)
			}
		case tool.Name == reviewToolStatus:
			seen++
			if starts {
				t.Errorf("%s gilt als gesperrt: %q", tool.Name, tool.Description)
			}
		}
	}
	if seen != 5 {
		t.Errorf("Werkzeugliste nennt %d von 5 Review-Werkzeugen", seen)
	}

	before := treeSnapshot(t, root)
	for _, input := range []reviewStatusInput{
		{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Mode: "available"},
		{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Run: "2026-08-19"},
	} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: reviewToolStatus, Arguments: input})
		if err != nil {
			t.Fatalf("status: CallTool: %v", err)
		}
		if envelope := decodeReviewEnvelope(t, result); envelope.Error != nil && envelope.Error.Code == review.PausedCode {
			t.Errorf("status ist gesperrt: %+v", envelope.Error)
		}
	}
	assertSameTree(t, "status ohne "+project.LocalDirName, before, treeSnapshot(t, root))
}
