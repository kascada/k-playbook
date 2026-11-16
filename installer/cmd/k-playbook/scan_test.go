package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review"
)

func zahl(value int) *int { return &value }

// Die Kandidatenzahl steht dort, wo sie etwas bedeutet: bei 0 Befunden trennt
// sie „nichts zu prüfen" von „nichts geprüft". Bei Befunden ist sie Rauschen.
func TestDescribeJobNenntKandidatenNurBeiNullBefunden(t *testing.T) {
	fälle := []struct {
		name string
		job  review.JobStatus
		want string
	}{
		{
			name: "0 Befunde mit Zählung",
			job: review.JobStatus{
				State: review.StateDone, Findings: zahl(0), Candidates: zahl(12),
				SARIF: "raw/ruff.sarif",
			},
			want: "fertig, 0 Befunde bei 12 Kandidaten → raw/ruff.sarif",
		},
		{
			name: "0 Befunde ohne Zählung",
			job: review.JobStatus{
				State: review.StateDone, Findings: zahl(0), SARIF: "raw/trivy-config.sarif",
			},
			want: "fertig, 0 Befunde → raw/trivy-config.sarif",
		},
		{
			name: "Befunde: die Zahl sagt schon, dass geprüft wurde",
			job: review.JobStatus{
				State: review.StateDone, Findings: zahl(154), Candidates: zahl(200),
				SARIF: "raw/gosec.sarif",
			},
			want: "fertig, 154 Befunde → raw/gosec.sarif",
		},
	}

	for _, fall := range fälle {
		if got := describeJob(fall.job); got != fall.want {
			t.Errorf("%s: %q, erwartet %q", fall.name, got, fall.want)
		}
	}
}

// Ein übersprungener Job trägt seinen Grund, keine Zahlen.
func TestDescribeJobUebersprungen(t *testing.T) {
	got := describeJob(review.JobStatus{State: review.StateSkipped, Reason: "Sprache nicht gewählt"})
	if !strings.Contains(got, "Sprache nicht gewählt") {
		t.Errorf("Grund fehlt: %q", got)
	}
}

// Ohne den Clone <projekt>/k-playbook/ verweigert scan nicht mehr mit „die
// Installation … fehlt“ (Task 154). Geprüft wird der Teil hinter der Pause:
// runScan als Ganzes endet zurzeit immer an review.Paused und bestünde auch
// mit der Sperre. Das Ergebnisverzeichnis kommt über die Naht
// review.ResultsDirOverride, die nur Tests setzen.
func TestScanRunOhneCloneVerlangtKeineInstallation(t *testing.T) {
	root := t.TempDir()
	mustWriteMCPFile(t, project.ConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n")
	if _, err := os.Stat(filepath.Join(root, project.PlaybookDirName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s/ darf es im Testprojekt nicht geben: %v", project.PlaybookDirName, err)
	}
	t.Chdir(root)
	review.ResultsDirOverride = filepath.Join(root, "ergebnisse")
	t.Cleanup(func() { review.ResultsDirOverride = "" })

	err := scanRun([]string{"kein-lauf"})
	if err == nil {
		t.Fatal("ein fehlender Lauf wurde nicht gemeldet")
	}
	got := err.Error()
	if strings.Contains(got, "Installation unter") {
		t.Errorf("Fehler = %q, erwartet keine Sperre wegen der Installation", got)
	}
	if !strings.Contains(got, "kein-lauf ist nicht lesbar") {
		t.Errorf("Fehler = %q, erwartet, dass scan bis zum Lesen des Laufs kommt", got)
	}
}

// Ohne die Naht kennt scan keinen Ort der Review-Ergebnisse: Der Teil hinter
// der Pause endet mit review.ErrResultsDirUndecided, bevor er einen Lauf
// liest.
func TestScanRunOhneOrtMeldetUnentschieden(t *testing.T) {
	root := t.TempDir()
	mustWriteMCPFile(t, project.ConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n")
	t.Chdir(root)

	if err := scanRun([]string{"kein-lauf"}); !errors.Is(err, review.ErrResultsDirUndecided) {
		t.Errorf("Fehler = %v, erwartet %v", err, review.ErrResultsDirUndecided)
	}
}

// Die Pause aus Task 147 bleibt: scan als Ganzes antwortet mit der Sperre.
func TestRunScanBleibtPausiert(t *testing.T) {
	err := runScan([]string{"kein-lauf"})
	if !errors.Is(err, review.ErrPaused) {
		t.Errorf("Fehler = %v, erwartet die Pause der Review-Wege", err)
	}
}
