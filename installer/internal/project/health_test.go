package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kascada/k-playbook/installer/internal/githubaccess"
	"github.com/kascada/k-playbook/installer/internal/githubaccess/accesstest"
	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// healthProject legt ein Projekt an, dessen Umgebung in Ordnung ist: gh nicht
// vorausgesetzt, Matrix der Basis-Werkzeuge mit einem Werkzeug, das da ist,
// Quellenkonfiguration gültig, Anbindung ok. Jeder Test verdirbt davon genau
// eine Quelle.
func healthProject(t *testing.T) string {
	t.Helper()
	root := newContextProject(t)
	scriptDir := filepath.Join(shipped(t), "scripts")
	mkdirAll(t, scriptDir)
	write(t, filepath.Join(scriptDir, "base-tools.tsv"), baseMatrixHeader+"k-da\t-\tIst da\tja\tapt\tpaket\t-\t-\t-\n")
	write(t, filepath.Join(LocalDir(root), VersionSourcesFileName), "schema_version: 1\nroots: []\nsources: []\n")
	t.Setenv("PATH", fakeBinDir(t, filepath.Join(t.TempDir(), "aufrufe"), "k-da"))
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	accesstest.Empty(t)
	stubKephalaionCheck(t, kephalaion.Status{Configured: true, State: kephalaion.StateOK, OK: true, Message: "Angemeldet."})
	return root
}

func TestHealthOhneProblemeIstLeer(t *testing.T) {
	root := healthProject(t)

	report := health.Check(context.Background(), root)
	if !report.OK || len(report.Problems) != 0 {
		t.Errorf("Report: %+v", report)
	}
}

func TestHealthKephalaionNichtEingetragenWarnt(t *testing.T) {
	root := healthProject(t)
	stubKephalaionCheck(t, kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen: In K-PLAYBOOK.local.yaml fehlt der Block kephalaion:."})

	problems := healthKephalaion(context.Background(), root)
	if len(problems) != 1 {
		t.Fatalf("Problems = %+v", problems)
	}
	got := problems[0]
	if got.Level != health.LevelWarning || got.Area != health.AreaKephalaion || !strings.HasPrefix(got.Message, "Keine Collection eingetragen") || !strings.Contains(got.Hint, "k-playbook gui") {
		t.Errorf("Eintrag: %+v", got)
	}
	if report := health.Check(context.Background(), root); !report.OK || len(report.Problems) != 1 {
		t.Errorf("Warnung lässt OK nicht wahr: %+v", report)
	}
}

// Eingetragen, aber nicht ok, ist ein Fehler — in jedem Zustand außer ok und
// not-configured. Der Satz ist die Message der Prüfung; ein Token steht nie
// darin.
func TestHealthKephalaionEingetragenAberNichtOKIstFehler(t *testing.T) {
	root := healthProject(t)
	for _, state := range []kephalaion.State{
		kephalaion.StateInvalidEntry, kephalaion.StateNoKephalaion, kephalaion.StateNodeAddress,
		kephalaion.StateTokenMissing, kephalaion.StateNodeUnreachable, kephalaion.StateHubUnknown,
		kephalaion.StateHubNeverSynced, kephalaion.StateLoginInvalid, kephalaion.StateLoginMissing,
		kephalaion.StateReplicaUnreadable, kephalaion.StateCollectionUnreadable,
	} {
		stubKephalaionCheck(t, kephalaion.Status{Configured: true, State: state, Message: "Satz zu " + string(state) + "."})
		problems := healthKephalaion(context.Background(), root)
		if len(problems) != 1 {
			t.Fatalf("%s: Problems = %+v", state, problems)
		}
		got := problems[0]
		if got.Level != health.LevelError || got.Area != health.AreaKephalaion || got.Message != "Satz zu "+string(state)+"." || got.Hint == "" {
			t.Errorf("%s: %+v", state, got)
		}
		if report := health.Check(context.Background(), root); report.OK {
			t.Errorf("%s: OK trotz Fehler", state)
		}
	}
}

func TestHealthGHVorausgesetztAberNichtNutzbarWarnt(t *testing.T) {
	root := healthProject(t)
	write(t, ConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n\ntools:\n  gh:\n    status: enabled\n")

	// PATH ohne gh, keine Anmeldung.
	problems := healthGH(context.Background(), root)
	if len(problems) != 1 || problems[0].Level != health.LevelWarning || problems[0].Area != health.AreaGH ||
		!strings.Contains(problems[0].Message, "nicht im PATH") || !strings.Contains(problems[0].Hint, "gh installieren") {
		t.Errorf("ohne gh: %+v", problems)
	}

	// gh im PATH, aber kein Konto.
	t.Setenv("PATH", fakeBinDir(t, filepath.Join(t.TempDir(), "aufrufe"), "k-da", "gh"))
	problems = healthGH(context.Background(), root)
	if len(problems) != 1 || problems[0].Level != health.LevelWarning || !strings.Contains(problems[0].Message, "kein Konto") || !strings.Contains(problems[0].Hint, "gh auth login") {
		t.Errorf("ohne Konto: %+v", problems)
	}

	// Angemeldet über die Umgebung: nutzbar, nichts zu melden.
	t.Setenv("GH_TOKEN", "gho_geheim")
	if problems = healthGH(context.Background(), root); len(problems) != 0 {
		t.Errorf("mit Anmeldung: %+v", problems)
	}
	for _, problem := range health.Check(context.Background(), root).Problems {
		if strings.Contains(problem.Message+problem.Hint, "gho_geheim") {
			t.Errorf("Eintrag nennt den Token: %+v", problem)
		}
	}
}

// Nutzt das Projekt gh nicht oder hat es nicht entschieden, ist eine fehlende
// CLI kein Thema.
func TestHealthGHNichtVorausgesetztSchweigt(t *testing.T) {
	root := healthProject(t)
	for _, status := range []string{"unknown", "disabled"} {
		write(t, ConfigPath(root), "schema_version: 3\n\nproject:\n  repo_root: .\n  vcs: git\n\ntools:\n  gh:\n    status: "+status+"\n")
		if problems := healthGH(context.Background(), root); len(problems) != 0 {
			t.Errorf("%s: %+v", status, problems)
		}
	}
}

// Ein unbekannter Wert in tools.gh.status ist ein Fehler. Der Satz nennt
// Datei, Feld und die erlaubten Werte, aber nie den Wert aus der Datei: Der
// steht nur in der Kontextausgabe.
func TestHealthGHUnbekannterWertIstFehler(t *testing.T) {
	root := healthProject(t)
	write(t, ConfigPath(root), "schema_version: 3\n\ntools:\n  gh:\n    status: geheim-xyz\n")

	problems := healthGH(context.Background(), root)
	if len(problems) != 1 || problems[0].Level != health.LevelError || problems[0].Area != health.AreaGH {
		t.Fatalf("Problems = %+v", problems)
	}
	problem := problems[0]
	for _, want := range []string{ConfigFileName, "tools.gh.status", "unknown", "enabled", "disabled"} {
		if !strings.Contains(problem.Message, want) {
			t.Errorf("Message ohne %q: %q", want, problem.Message)
		}
	}
	if strings.Contains(problem.Message+problem.Hint, "geheim-xyz") {
		t.Errorf("der Wert aus der Datei steht im Problem: %+v", problem)
	}
	if !strings.Contains(problem.Hint, "tools.gh.status") {
		t.Errorf("Hint ohne Abhilfe: %q", problem.Hint)
	}
}

// Fehlt K-PLAYBOOK.yaml, ist das derselbe Bereich mit eigenem Satz; auch er
// zitiert nichts aus dem Dateisystem außer dem Dateinamen.
func TestHealthGHKonfigurationNichtLesbarIstFehler(t *testing.T) {
	root := healthProject(t)
	if err := os.Remove(ConfigPath(root)); err != nil {
		t.Fatal(err)
	}

	problems := healthGH(context.Background(), root)
	if len(problems) != 1 || problems[0].Level != health.LevelError || problems[0].Area != health.AreaGH ||
		!strings.Contains(problems[0].Message, "nicht lesbar") || !strings.Contains(problems[0].Message, ConfigFileName) ||
		strings.Contains(problems[0].Message, root) {
		t.Errorf("Problems = %+v", problems)
	}
}

func TestHealthBaseToolsJeFehlendemEineWarnung(t *testing.T) {
	root := healthProject(t)
	write(t, filepath.Join(shipped(t), "scripts", "base-tools.tsv"), baseMatrixHeader+
		"k-da\t-\tIst da\tja\tapt\tpaket\t-\t-\t-\n"+
		"k-weg\t-\tVolltextsuche\tnein\tapt,github\tpaket-weg\tbeispiel/weg\tweg\t^{tool}$\n"+
		"k-fort\t-\tArchive\tnein\tapt\tpaket-fort\t-\t-\t-\n")

	problems := healthBaseTools(context.Background(), root)
	if len(problems) != 2 {
		t.Fatalf("Problems = %+v", problems)
	}
	for _, got := range problems {
		if got.Level != health.LevelWarning || got.Area != health.AreaBaseTools || !strings.Contains(got.Hint, "install-base-tools.sh") {
			t.Errorf("Eintrag: %+v", got)
		}
	}
	report := health.Check(context.Background(), root)
	if !report.OK || len(report.Problems) != 2 {
		t.Errorf("Report: %+v", report)
	}
	// Sortiert nach Satz, also k-fort vor k-weg.
	if !strings.Contains(report.Problems[0].Message, "k-fort") || !strings.Contains(report.Problems[1].Message, "k-weg") {
		t.Errorf("Reihenfolge: %+v", report.Problems)
	}
}

// Die Quellenkonfiguration ist der Testfall der Messung: eine Datei, die da,
// aber nicht lesbar ist. Der Satz nennt den Pfad, nicht den Inhalt der
// Datei.
func TestHealthVersionSourcesUnlesbarIstFehler(t *testing.T) {
	root := healthProject(t)
	path := filepath.Join(LocalDir(root), VersionSourcesFileName)
	write(t, path, ": ungültig\n")

	problems := healthVersionSources(context.Background(), root)
	if len(problems) != 1 {
		t.Fatalf("Problems = %+v", problems)
	}
	got := problems[0]
	if got.Level != health.LevelError || got.Area != health.AreaVersionSources || !strings.Contains(got.Message, path) || got.Hint == "" {
		t.Errorf("Eintrag: %+v", got)
	}
	if strings.Contains(got.Message, "ungültig") || strings.Contains(got.Hint, "ungültig") {
		t.Errorf("Eintrag zitiert die Datei: %+v", got)
	}
	report := health.Check(context.Background(), root)
	if report.OK || len(report.Problems) != 1 {
		t.Errorf("Report: %+v", report)
	}

	// Fehlt die Datei, gelten die Standardquellen: kein Problem.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if problems = healthVersionSources(context.Background(), root); len(problems) != 0 {
		t.Errorf("ohne Datei: %+v", problems)
	}
}

// Alle Quellen auf einmal: der Report trägt sie sortiert nach Bereich.
func TestHealthCheckSammeltAlleQuellen(t *testing.T) {
	root := healthProject(t)
	stubKephalaionCheck(t, kephalaion.Status{State: kephalaion.StateNotConfigured, Message: "Keine Collection eingetragen."})
	write(t, ConfigPath(root), "schema_version: 3\n\ntools:\n  gh:\n    status: enabled\n")
	write(t, filepath.Join(shipped(t), "scripts", "base-tools.tsv"), baseMatrixHeader+"k-weg\t-\tFehlt\tnein\tapt\tpaket\t-\t-\t-\n")
	write(t, filepath.Join(LocalDir(root), VersionSourcesFileName), ": ungültig\n")

	report := health.Check(context.Background(), root)
	if report.OK {
		t.Error("OK trotz Fehler")
	}
	var areas []string
	for _, problem := range report.Problems {
		areas = append(areas, problem.Area)
	}
	want := []string{health.AreaBaseTools, health.AreaGH, health.AreaKephalaion, health.AreaVersionSources}
	if strings.Join(areas, ",") != strings.Join(want, ",") {
		t.Errorf("Bereiche %v, erwartet %v", areas, want)
	}
}

// Die Quelle github trägt die Probleme der persönlichen Zugangsdatei ein: der
// Pfad in der Datei vorn im Satz, Stufe wie beim Leser, Bereich github. Eine
// leere Datei ergibt nichts, eine fehlende einmal eine Warnung (die Vorlage),
// eine defekte je Problem einen Fehler — nie ein Wert aus der Datei.
func TestHealthGitHubTraegtDieProblemeDerZugangsdatei(t *testing.T) {
	root := healthProject(t)

	// Leer: nichts.
	if problems := healthGitHub(context.Background(), root); len(problems) != 0 {
		t.Errorf("leere Datei: %+v", problems)
	}

	// Fehlend: einmal die Warnung der Vorlage, danach nichts; OK bleibt wahr.
	missing := filepath.Join(t.TempDir(), githubaccess.FileName)
	t.Setenv(githubaccess.EnvConfig, missing)
	report := health.Check(context.Background(), root)
	if !report.OK || len(report.Problems) != 1 {
		t.Fatalf("fehlende Datei: %+v", report)
	}
	got := report.Problems[0]
	if got.Level != health.LevelWarning || got.Area != health.AreaGitHub || !strings.HasPrefix(got.Message, "file: Die Datei wurde gerade angelegt") || !strings.Contains(got.Hint, missing) {
		t.Errorf("Vorlage: %+v", got)
	}
	if report := health.Check(context.Background(), root); !report.OK || len(report.Problems) != 0 {
		t.Errorf("zweiter Aufruf: %+v", report)
	}

	// Defekt: je Problem ein Fehler mit Pfad, OK falsch.
	write(t, missing, "services:\n  backend:\n    repo: acme-labs/backend\n    access:\n      - type: ssh\n        key: ~/.ssh/id\n      - type: token\n        file: relativ.token\n  web:\n    access:\n      - type: public\n")
	report = health.Check(context.Background(), root)
	if report.OK || len(report.Problems) != 2 {
		t.Fatalf("defekte Datei: %+v", report)
	}
	// health sortiert nach Satz; der Pfad steht vorn.
	if report.Problems[0].Message != "services.backend.access[1]: Der Pfad in file ist relativ." || report.Problems[0].Hint != "Absoluten Pfad oder ~/… eintragen." ||
		report.Problems[1].Message != "services.web: Dem Dienst fehlt repo." {
		t.Errorf("Einträge: %+v", report.Problems)
	}
	for _, problem := range report.Problems {
		if problem.Level != health.LevelError || problem.Area != health.AreaGitHub || strings.Contains(problem.Message, "relativ.token") {
			t.Errorf("Eintrag: %+v", problem)
		}
	}
}

// Liest sich die Datei, beurteilt die Quelle die Verfügbarkeit: ein Dienst
// ohne verfügbaren Zugang ist eine Warnung mit dem Grund des ersten Zugangs,
// ein nutzbarer Dienst schweigt. GitHub wird dafür nicht gefragt; hier gibt
// es weder gh noch ssh im PATH, und public braucht nichts.
func TestHealthGitHubWarntBeiDienstOhneVerfuegbarenZugang(t *testing.T) {
	root := healthProject(t)
	// Das Basis-Werkzeug der Matrix bleibt im PATH, gh und ssh fehlen.
	t.Setenv("PATH", fakeBinDir(t, filepath.Join(t.TempDir(), "aufrufe"), "k-da"))
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	path := filepath.Join(t.TempDir(), githubaccess.FileName)
	t.Setenv(githubaccess.EnvConfig, path)
	write(t, path, "services:\n  website:\n    repo: acme/website\n    access:\n      - type: gh\n        login: alice\n  docs:\n    repo: acme/docs\n    access:\n      - type: gh\n        login: alice\n      - type: public\n")

	report := health.Check(context.Background(), root)
	if !report.OK || len(report.Problems) != 1 {
		t.Fatalf("Report: %+v", report)
	}
	got := report.Problems[0]
	if got.Level != health.LevelWarning || got.Area != health.AreaGitHub ||
		!strings.HasPrefix(got.Message, "services.website: kein verfügbarer Zugang — gh liegt nicht im PATH") ||
		!strings.Contains(got.Hint, path) {
		t.Errorf("Eintrag: %+v", got)
	}
}
