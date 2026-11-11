package mcpserver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/project/defaultstest"
	"github.com/kascada/k-playbook/installer/internal/review"
)

// newSelectionProject ist newReviewProject mit den ausgelieferten Rezepten
// aus defaults/reviews/ des Repos als mitgelieferter Ebene (über kephtest,
// siehe TestMain) und drei projekteigenen Rezepten: ein eigenes, eines, das
// ein mitgeliefertes ersetzt, und ein leeres, das eines abschaltet. Zurück
// kommen das Projekt und die Quelldateien der ausgelieferten Rezepte.
func newSelectionProject(t *testing.T) (string, []string) {
	t.Helper()
	sources, err := filepath.Glob(filepath.Join("..", "..", "..", "defaults", "reviews", "review-*.md"))
	if err != nil {
		t.Fatalf("defaults/reviews/ lesen: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("kein Rezept unter defaults/reviews/ an der Repo-Wurzel")
	}
	root := newReviewProject(t)
	reviewsDir := filepath.Join(root, "k-playbook", "defaults", "reviews")
	if err := os.RemoveAll(reviewsDir); err != nil {
		t.Fatalf("Vorlagen entfernen: %v", err)
	}
	for _, source := range sources {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("%s lesen: %v", source, err)
		}
		mustWriteFile(t, filepath.Join(reviewsDir, filepath.Base(source)), string(data))
	}
	localDir := filepath.Join(root, defaultstest.CollectionDir, project.ProjectLayerDirName, "reviews")
	mustWriteFile(t, filepath.Join(localDir, "review-eigen.md"), "---\ntitle: Eigenes Rezept\naudit:\n  enabled: true\n---\n# Überschrift\n")
	mustWriteFile(t, filepath.Join(localDir, "review-tech.md"), "---\naudit:\n  enabled: true\nreview:\n  enabled: true\n---\n# Lokale Technik\n")
	mustWriteFile(t, filepath.Join(localDir, "review-sast.md"), "")
	return root, sources
}

type selectionCandidate struct {
	Name       string
	Mode       review.Mode
	Title      string
	RecipePath string
}

func aiSelectionCandidates(base reviewSelectionBase) []selectionCandidate {
	out := []selectionCandidate{}
	for _, candidate := range base.Candidates {
		if candidate.Kind != review.KindAI || candidate.Name == scanTriageEntry {
			continue
		}
		out = append(out, selectionCandidate{Name: candidate.Name, Mode: candidate.Mode, Title: candidate.Title, RecipePath: candidate.RecipePath})
	}
	return out
}

// Die Auswahl liefert je Rezept Name, Modus, Titel und Rezeptpfad: jedes
// Rezept mit dem wirksamen Verweis k-playbook:/reviews/…, auch ein
// projekteigenes oder überschreibendes aus der Collection des Projekts
// (Task 143). Erwartet wird, was das
// Rezept selbst sagt — gelesen über die Quelldatei auf der Platte, nicht über
// den Weg der Auswahl.
func TestAuswahlKandidatenDerRezepte(t *testing.T) {
	root, sources := newSelectionProject(t)

	want := []selectionCandidate{
		{Name: "eigen", Mode: review.ModePerspective, Title: "Eigenes Rezept", RecipePath: "k-playbook:/reviews/review-eigen.md"},
		{Name: "tech", Mode: review.ModePerspective, Title: "Lokale Technik", RecipePath: "k-playbook:/reviews/review-tech.md"},
	}
	for _, source := range sources {
		name := filepath.Base(source)
		key := strings.TrimSuffix(strings.TrimPrefix(name, "review-"), ".md")
		if key == "tech" || key == "sast" {
			continue
		}
		metadata, err := readAIRecipeMetadata(root, key, source)
		if err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		if !metadata.Enabled {
			continue
		}
		want = append(want, selectionCandidate{Name: key, Mode: metadata.Mode, Title: metadata.Title, RecipePath: "k-playbook:/reviews/" + name})
	}
	sort.Slice(want, func(i, j int) bool { return want[i].Name < want[j].Name })
	// Ohne diese Schranke wäre der Test auch dann grün, wenn die Auswahl
	// keine mitgelieferten Rezepte mehr läse.
	if len(want) < 5 {
		t.Fatalf("zu wenige erwartete Kandidaten: %+v", want)
	}

	env, toolErr := resolveReviewEnvironment(root)
	if toolErr.Code != "" {
		t.Fatalf("Umgebung: %#v", toolErr)
	}
	base, toolErr := buildSelectionBase(env)
	if toolErr.Code != "" {
		t.Fatalf("Auswahl: %#v", toolErr)
	}
	got := aiSelectionCandidates(base)
	if len(got) != len(want) {
		t.Fatalf("Kandidaten:\n got  %+v\n want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Kandidat %d:\n got  %+v\n want %+v", i, got[i], want[i])
		}
	}
}

// readAIRecipeMetadata liest ein Rezept über seinen RecipePath aus dem
// Katalog — auch ein projekteigenes und ein überschreibendes aus der
// Collection des Projekts, deren Path der Verweis k-playbook:/reviews/… ist.
func TestRezeptAngabenUeberRecipePath(t *testing.T) {
	root, _ := newSelectionProject(t)
	for key, title := range map[string]string{"eigen": "Eigenes Rezept", "tech": "Lokale Technik"} {
		metadata, err := readAIRecipeMetadata(root, key, "k-playbook:/reviews/review-"+key+".md")
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if !metadata.Enabled || metadata.Title != title {
			t.Errorf("%s: %+v, erwartet aktiv mit Titel %q", key, metadata, title)
		}
	}
}

// Ohne mitgelieferte Ebene gibt es keine Auswahl: der Fehler ist
// kephalaion_unavailable, nicht read_failed, und kommt, bevor ein Rezept
// gelesen wird — in review_status (available) wie in review_create.
func TestAuswahlOhneMitgelieferteEbene(t *testing.T) {
	root, _ := newSelectionProject(t)
	original := project.OpenDefaultsStore
	project.OpenDefaultsStore = func(string) (project.DefaultsStore, string, error) {
		return nil, "vm:k-playbook", errors.New("Kephalaion ist für dieses Projekt nicht bereit")
	}
	t.Cleanup(func() { project.OpenDefaultsStore = original })

	result, _, err := reviewStatusTool(context.Background(), nil, reviewStatusInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Mode: "available"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	assertToolErrorCode(t, result, project.DocCodeKephalaionUnavailable)

	result, _, err = reviewCreateTool(context.Background(), nil, reviewCreateInput{reviewBaseInput: reviewBaseInput{ProjectDir: root}, Day: "2026-08-19", DryRun: true})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	assertToolErrorCode(t, result, project.DocCodeKephalaionUnavailable)
	if envelope := decodeReviewEnvelope(t, result); !strings.Contains(envelope.Error.Message, "Review-Katalog ist nicht lesbar") {
		t.Errorf("Meldung: %q", envelope.Error.Message)
	}
}

// Eine Auswahl öffnet die mitgelieferte Ebene einmal — für den Katalog —
// und liest die mitgelieferten Rezepte von dort, statt je Rezept neue
// Ebenen und damit einen neuen Zugang zum Node aufzubauen.
func TestAuswahlOeffnetMitgelieferteEbeneEinmal(t *testing.T) {
	root, _ := newSelectionProject(t)
	var mu sync.Mutex
	opens := 0
	original := project.OpenDefaultsStore
	project.OpenDefaultsStore = func(projectDir string) (project.DefaultsStore, string, error) {
		mu.Lock()
		opens++
		mu.Unlock()
		return original(projectDir)
	}
	t.Cleanup(func() { project.OpenDefaultsStore = original })

	env, toolErr := resolveReviewEnvironment(root)
	if toolErr.Code != "" {
		t.Fatalf("Umgebung: %#v", toolErr)
	}
	base, toolErr := buildSelectionBase(env)
	if toolErr.Code != "" {
		t.Fatalf("Auswahl: %#v", toolErr)
	}
	dist := 0
	for _, candidate := range base.Candidates {
		if strings.HasPrefix(candidate.RecipePath, project.DocSchemeKPlaybook+":/") {
			dist++
		}
	}
	if dist == 0 {
		t.Fatal("kein mitgeliefertes Rezept in der Auswahl")
	}
	if opens != 1 {
		t.Errorf("OpenDefaultsStore je Auswahl = %d, erwartet 1 (bei %d mitgelieferten Rezepten)", opens, dist)
	}
}
