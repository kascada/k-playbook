package main

import (
	"strings"
	"testing"

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
