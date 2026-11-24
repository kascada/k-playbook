package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// anstossAufruf ist die Zeile, an der der aktuelle Anstoß erkennbar ist: das
// installierte k-playbook unter seinem Namen, eingerückt als Codeblock, mit dem
// Briefing als dem einen Aufruf.
const anstossAufruf = "\n    k-playbook briefing\n"

// repoRootInstructions liest die AGENTS.md des Quell-Repos: die Zielgestalt
// der Vorlage, seit 2026-10-18 von Hand auf das Briefing umgestellt.
func repoRootInstructions(t *testing.T) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("..", "..", "..", RootInstructionsFile))
	if err != nil {
		t.Skipf("AGENTS.md des Quell-Repos nicht lesbar: %v", err)
	}
	return string(content)
}

// Die Vorlage ist genau die AGENTS.md des Quell-Repos. Läuft eine der beiden
// davon, merkt es dieser Test — die Datei im Repo ist die Fixture.
func TestVorlageIstDieAgentsDatei(t *testing.T) {
	if repo := repoRootInstructions(t); repo != rootInstructionsTemplate() {
		t.Errorf("die Vorlage ist nicht die AGENTS.md des Repos:\n%q\nRepo:\n%q", rootInstructionsTemplate(), repo)
	}
}

// Die Briefing-Fassung ist aktuell: weder Einrichten noch Start fassen sie an.
func TestBriefingFassungBleibtUnveraendert(t *testing.T) {
	root := t.TempDir()
	repo := repoRootInstructions(t)
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(repo), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}

	if state := CheckRootInstructions(root); !state.OK() {
		t.Errorf("die Briefing-Fassung gilt nicht als aktuell: %+v", state)
	}
	repaired, err := RepairRootInstructions(root)
	if err != nil {
		t.Fatalf("RepairRootInstructions: %v", err)
	}
	if repaired.Changed() {
		t.Errorf("der Start hat geschrieben: %+v", repaired)
	}
	if _, err := ApplyRootInstructions(root); err != nil {
		t.Fatalf("ApplyRootInstructions: %v", err)
	}
	if readInstructions(t, root) != repo {
		t.Errorf("die Briefing-Fassung wurde verändert:\n%s", readInstructions(t, root))
	}
}

func TestApplyRootInstructionsLegtDateiAn(t *testing.T) {
	root := t.TempDir()

	state, err := ApplyRootInstructions(root)
	if err != nil {
		t.Fatalf("ApplyRootInstructions: %v", err)
	}
	if !state.OK() {
		t.Fatalf("nicht eingerichtet: %+v", state)
	}

	content := readInstructions(t, root)
	if !strings.Contains(content, anstossAufruf) {
		t.Errorf("Anstoß fehlt:\n%s", content)
	}
	// Die Vorlage ist der Anstoß und sonst nichts.
	if content != rootInstructionsTemplate() {
		t.Errorf("die Datei ist nicht die Vorlage:\n%s", content)
	}
}

// Eine vorhandene Datei gehört dem Projekt: der Anstoß wird angehängt.
func TestApplyRootInstructionsErgaenztVorhandene(t *testing.T) {
	root := t.TempDir()
	eigen := "# Unser Projekt\n\nHier stehen unsere eigenen Regeln.\n"
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}

	if _, err := ApplyRootInstructions(root); err != nil {
		t.Fatalf("ApplyRootInstructions: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(root, RootInstructionsFile))
	if err != nil {
		t.Fatalf("AGENTS.md lesen: %v", err)
	}
	if !strings.Contains(string(content), "Hier stehen unsere eigenen Regeln.") {
		t.Errorf("vorhandener Text verloren:\n%s", content)
	}
	if !strings.Contains(string(content), anstossAufruf) {
		t.Errorf("Anstoß fehlt:\n%s", content)
	}
}

// Ein vorhandener Anstoßblock gehört dem Projekt, auch in anderer Fassung:
// Einrichten und Start lassen ihn stehen.
func TestVorhandenerAnstossBleibtInJederFassung(t *testing.T) {
	root := t.TempDir()
	eigen := "# AGENTS.md\n\n" + instructionsMarker + "\n## k-playbook\n\nBei uns umformuliert.\n\nProsa ohne Überschrift.\n"
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}

	if state := CheckRootInstructions(root); !state.OK() {
		t.Errorf("ein Block mit Marker gilt nicht als eingerichtet: %+v", state)
	}
	if _, err := ApplyRootInstructions(root); err != nil {
		t.Fatalf("ApplyRootInstructions: %v", err)
	}
	if repaired, err := RepairRootInstructions(root); err != nil || repaired.Changed() {
		t.Errorf("RepairRootInstructions = %+v, %v", repaired, err)
	}
	if readInstructions(t, root) != eigen {
		t.Errorf("der Block wurde verändert:\n%s", readInstructions(t, root))
	}
}

// Ein zweiter Lauf darf den Block nicht doppeln.
func TestApplyRootInstructionsIstIdempotent(t *testing.T) {
	root := t.TempDir()

	if _, err := ApplyRootInstructions(root); err != nil {
		t.Fatalf("erster Lauf: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(root, RootInstructionsFile))
	if err != nil {
		t.Fatalf("AGENTS.md lesen: %v", err)
	}

	if _, err := ApplyRootInstructions(root); err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(root, RootInstructionsFile))
	if err != nil {
		t.Fatalf("AGENTS.md lesen: %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("zweiter Lauf hat die Datei verändert:\n%s", second)
	}
	if strings.Count(string(second), instructionsMarker) != 1 {
		t.Errorf("Anstoß steht mehrfach:\n%s", second)
	}
}

// Der Auffangweg beim Start ist eng: eine vorhandene Datei rührt er nicht an.
func TestRepairRootInstructionsIstEngUndIdempotent(t *testing.T) {
	t.Run("aktueller Block bleibt unangetastet", func(t *testing.T) {
		root := t.TempDir()
		if _, err := ApplyRootInstructions(root); err != nil {
			t.Fatalf("ApplyRootInstructions: %v", err)
		}
		vorher := readInstructions(t, root)

		repaired, err := RepairRootInstructions(root)
		if err != nil {
			t.Fatalf("RepairRootInstructions: %v", err)
		}
		if repaired.Changed() {
			t.Error("ein aktueller Block wurde geschrieben")
		}
		if readInstructions(t, root) != vorher {
			t.Error("ein aktueller Block wurde verändert")
		}
	})

	t.Run("fremder Text bleibt liegen", func(t *testing.T) {
		root := t.TempDir()
		eigen := "# Unser Projekt\n\nHier steht kein Anstoß.\n"
		if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
			t.Fatalf("AGENTS.md anlegen: %v", err)
		}

		repaired, err := RepairRootInstructions(root)
		if err != nil {
			t.Fatalf("RepairRootInstructions: %v", err)
		}
		if repaired.Changed() {
			t.Error("ohne Anstoß wurde geschrieben")
		}
		if readInstructions(t, root) != eigen {
			t.Error("eine fremde Datei wurde verändert")
		}
	})

	t.Run("fremder Text wird nicht angelegt gemeldet", func(t *testing.T) {
		root := t.TempDir()
		eigen := "# Unser Projekt\n\nHier steht kein Anstoß.\n"
		if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
			t.Fatalf("AGENTS.md anlegen: %v", err)
		}

		repaired, err := RepairRootInstructions(root)
		if err != nil {
			t.Fatalf("RepairRootInstructions: %v", err)
		}
		if repaired.Created {
			t.Error("eine vorhandene Datei wurde als angelegt gemeldet")
		}
		if readInstructions(t, root) != eigen {
			t.Error("eine fremde Datei wurde verändert")
		}
	})
}

// Der zweite Fall des Auffangwegs: fehlt AGENTS.md ganz, entsteht sie aus der
// Vorlage — nur die Datei. opencode.json bleibt unangetastet, und der zweite
// Lauf hat nichts mehr zu tun.
func TestRepairRootInstructionsLegtFehlendeDateiAusVorlageAn(t *testing.T) {
	root := t.TempDir()
	opencode := filepath.Join(root, "opencode.json")
	vorher := `{"$schema":"https://opencode.ai/config.json"}` + "\n"
	if err := os.WriteFile(opencode, []byte(vorher), 0o644); err != nil {
		t.Fatalf("opencode.json anlegen: %v", err)
	}

	repaired, err := RepairRootInstructions(root)
	if err != nil {
		t.Fatalf("RepairRootInstructions: %v", err)
	}
	if !repaired.Created {
		t.Fatal("die fehlende Datei wurde nicht angelegt")
	}
	if readInstructions(t, root) != rootInstructionsTemplate() {
		t.Errorf("die Datei entspricht nicht der Vorlage:\n%s", readInstructions(t, root))
	}
	if state := CheckRootInstructions(root); !state.OK() {
		t.Errorf("die angelegte Datei gilt nicht als eingerichtet: %+v", state)
	}

	nachher, err := os.ReadFile(opencode)
	if err != nil {
		t.Fatalf("opencode.json lesen: %v", err)
	}
	if string(nachher) != vorher {
		t.Errorf("der Auffangweg hat opencode.json verändert:\n%s", nachher)
	}

	erst := readInstructions(t, root)
	repaired, err = RepairRootInstructions(root)
	if err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if repaired.Changed() {
		t.Errorf("der zweite Lauf hat erneut geschrieben: %+v", repaired)
	}
	if readInstructions(t, root) != erst {
		t.Error("der zweite Lauf hat die Datei verändert")
	}
}

// Was die Fallmatrix nicht als glatten Fall kennt, legt der Auffangweg nicht
// an: ein Konflikt, eine echte CLAUDE.md, deren Umbenennung dem Knopf gehört,
// und ein Rest-Link an AGENTS.md, dem WriteFile an sein totes Ziel folgte.
func TestRepairRootInstructionsLegtBeiKonfliktNichtsAn(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
	}{
		{
			name: "CLAUDE.md zeigt auf ein fremdes Ziel (Zeile 4)",
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "fremd.md"), "# fremd\n")
				if err := os.Symlink("fremd.md", filepath.Join(root, ClaudeInstructionsFile)); err != nil {
					t.Fatalf("Symlink anlegen: %v", err)
				}
			},
		},
		{
			name: "echte CLAUDE.md wartet auf die Umbenennung (Zeile 10)",
			setup: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ClaudeInstructionsFile), "# Unser Projekt\n")
			},
		},
		{
			name: "AGENTS.md ist ein Rest-Link (Zeile 7)",
			setup: func(t *testing.T, root string) {
				if err := os.Symlink("weg.md", filepath.Join(root, RootInstructionsFile)); err != nil {
					t.Fatalf("Symlink anlegen: %v", err)
				}
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			test.setup(t, root)

			repaired, err := RepairRootInstructions(root)
			if err != nil {
				t.Fatalf("RepairRootInstructions: %v", err)
			}
			if repaired.Changed() {
				t.Errorf("der Auffangweg hat geschrieben: %+v", repaired)
			}
			if info, err := os.Lstat(filepath.Join(root, RootInstructionsFile)); err == nil && info.Mode().IsRegular() {
				t.Error("der Auffangweg hat AGENTS.md angelegt")
			}
			if pathExists(filepath.Join(root, "weg.md")) {
				t.Error("der Auffangweg ist dem Rest-Link an sein totes Ziel gefolgt")
			}
		})
	}
}

func readInstructions(t *testing.T, root string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(root, RootInstructionsFile))
	if err != nil {
		t.Fatalf("AGENTS.md lesen: %v", err)
	}
	return string(content)
}

// Eine leere Datei: es gibt nichts, wovor der Anstoß stünde — er beginnt die
// Datei, ohne leeren Absatz davor.
func TestLeereDateiWirdZumAnstoss(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte("\n"), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}

	if _, err := ApplyRootInstructions(root); err != nil {
		t.Fatalf("ApplyRootInstructions: %v", err)
	}
	if content := readInstructions(t, root); content != instructionsBlock() {
		t.Errorf("Datei nach dem Lauf:\n%q\nerwartet:\n%q", content, instructionsBlock())
	}
}
