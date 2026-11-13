package project

import (
	"context"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/buildinfo"
	"github.com/kascada/k-playbook/installer/internal/githubaccess/accesstest"
	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/release/releasetest"
)

// ownVersion stempelt dem Testprozess eine eigene Version.
func ownVersion(t *testing.T, version string) {
	t.Helper()
	before := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = before })
	buildinfo.Version = version
}

// programProblems sind die Einträge der Quelle program im Report.
func programProblems(report health.Report) []health.Problem {
	var problems []health.Problem
	for _, problem := range report.Problems {
		if problem.Area == health.AreaProgram {
			problems = append(problems, problem)
		}
	}
	return problems
}

// Ist das Programm älter als das neueste Release, warnt die Umgebungsprüfung
// mit beiden Versionen und k-playbook upgrade als Abhilfe. Eine Warnung, kein
// Fehler: OK hängt nicht daran.
func TestHealthProgramWarntBeiAelteremProgramm(t *testing.T) {
	source := releasetest.Serve(t)
	source.AddTag("v0.16.1")
	source.AddTag("v0.17.0")
	// Ein Tag mit Zusatz ist kein Release, auch wenn er höher wäre.
	source.AddTag("v0.18.0-rc.1")
	ownVersion(t, "v0.16.1")

	problems := healthProgram(context.Background(), t.TempDir())
	if len(problems) != 1 {
		t.Fatalf("Einträge = %+v, erwartet genau eine Warnung", problems)
	}
	got := problems[0]
	if got.Level != health.LevelWarning || got.Area != health.AreaProgram ||
		got.Message != "Das Programm (v0.16.1) ist älter als das neueste Release (v0.17.0)." {
		t.Errorf("Eintrag = %+v", got)
	}
	if !strings.Contains(got.Hint, "k-playbook upgrade") {
		t.Errorf("Abhilfe nennt k-playbook upgrade nicht: %q", got.Hint)
	}

	// Eingehängt: Der Report trägt den Eintrag, und die Warnung allein lässt
	// OK wahr.
	accesstest.Empty(t)
	report := health.Check(context.Background(), t.TempDir())
	if listed := programProblems(report); len(listed) != 1 || listed[0] != got {
		t.Errorf("health.Check trägt den Eintrag nicht: %+v", report.Problems)
	}
	for _, problem := range report.Problems {
		if problem.Area == health.AreaProgram && problem.Level == health.LevelError {
			t.Errorf("die Quelle program meldet einen Fehler: %+v", problem)
		}
	}
}

// Bei gleicher oder neuerer Version schweigt die Quelle: Ein Build über dem
// neuesten Tag gilt als aktuell.
func TestHealthProgramSchweigtBeiGleicherOderNeuererVersion(t *testing.T) {
	releasetest.Serve(t).AddTag("v0.17.0")
	for _, running := range []string{"v0.17.0", "v0.17.1", "v1.0.0"} {
		ownVersion(t, running)
		if problems := healthProgram(context.Background(), t.TempDir()); len(problems) != 0 {
			t.Errorf("laufend %s: %+v", running, problems)
		}
	}
}

// Ohne eigene Version ruft die Quelle nichts ab — kein Netzaufruf — und
// schweigt. Das ist der Zustand jedes Ad-hoc-Builds und fast aller Tests.
func TestHealthProgramOhneVersionRuftNichtsAb(t *testing.T) {
	source := releasetest.Serve(t)
	source.AddTag("v0.17.0")
	for _, running := range []string{"", "dev", "v0.17"} {
		ownVersion(t, running)
		if problems := healthProgram(context.Background(), t.TempDir()); len(problems) != 0 {
			t.Errorf("laufend %q: %+v", running, problems)
		}
	}
	if requests := source.Requests(); len(requests) != 0 {
		t.Errorf("ohne eigene Version wurde die Release-Quelle gefragt: %v", requests)
	}
}

// Ist der neueste Tag unbekannt — ohne Netz, ohne git, Zeitüberschreitung;
// hier die abgeschaltete Quelle und eine Quelle ohne Release-Tag —, schweigt
// die Quelle: unbekannt ist kein Nachweis.
func TestHealthProgramSchweigtBeiUnbekanntemTag(t *testing.T) {
	ownVersion(t, "v0.16.1")

	releasetest.Off(t)
	if problems := healthProgram(context.Background(), t.TempDir()); len(problems) != 0 {
		t.Errorf("abgeschaltet: %+v", problems)
	}

	releasetest.Serve(t).AddTag("v0.18.0-rc.1")
	if problems := healthProgram(context.Background(), t.TempDir()); len(problems) != 0 {
		t.Errorf("ohne Release-Tag: %+v", problems)
	}
}
