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

// veralteterAnstoss ist der Anstoßblock der ersten abgelösten Fassung, wie ihn
// Bestandsprojekte aus der Zeit des Wrapper-Modells tragen.
const veralteterAnstoss = `# AGENTS.md

<!-- k-playbook:anstoss -->
## k-playbook

Für dieses Projekt gilt k-playbook. Rufe zu Beginn

    k-playbook/bin/k-playbook context

auf und lies die Dateien aus ` + "`instructions`" + ` in der angegebenen Reihenfolge,
bevor du arbeitest. Die Ausgabe nennt außerdem die aufgelösten Verzeichnisse und
die effektiven Kataloge für Regeln, Reviews und Checks.
`

// veralteterKontextAnstoss ist die zweite abgelöste Fassung: der Block, den das
// Binary bis zum Briefing auslieferte — `k-playbook context` ohne Wrapper-Pfad.
const veralteterKontextAnstoss = `# AGENTS.md

<!-- k-playbook:anstoss -->
## k-playbook

Für dieses Projekt gilt k-playbook. Rufe zu Beginn

    k-playbook context

auf und lies die Dateien aus ` + "`instructions`" + ` in der angegebenen Reihenfolge,
bevor du arbeitest. Die Ausgabe nennt außerdem die aufgelösten Verzeichnisse und
die effektiven Kataloge für Regeln, Reviews und Checks.
`

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

	if state := CheckRootInstructions(root); !state.OK() || state.HasOutdatedAnstoss {
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

// Die zweite abgelöste Fassung — `k-playbook context` ohne Wrapper-Pfad — wird
// genauso ersetzt wie die erste. Hinter ihr steht Prosa ohne Überschrift: nur
// die bekannte Gestalt bestimmt die Grenze, und sie muss stimmen.
func TestApplyRootInstructionsErsetztKontextFassung(t *testing.T) {
	root := t.TempDir()
	eigen := veralteterKontextAnstoss + `
Diese Zeilen gehören dem Projekt und stehen ohne Überschrift dahinter.
`
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}
	if state := CheckRootInstructions(root); !state.HasOutdatedAnstoss {
		t.Errorf("die Kontext-Fassung gilt nicht als veraltet: %+v", state)
	}

	if _, err := ApplyRootInstructions(root); err != nil {
		t.Fatalf("ApplyRootInstructions: %v", err)
	}

	content := readInstructions(t, root)
	if hasLegacyInstructionsCommand(content) {
		t.Errorf("der abgelöste Aufruf steht noch drin:\n%s", content)
	}
	for _, erwartet := range []string{anstossAufruf, "Diese Zeilen gehören dem Projekt und stehen ohne Überschrift dahinter."} {
		if !strings.Contains(content, erwartet) {
			t.Errorf("%q fehlt:\n%s", erwartet, content)
		}
	}
	if strings.Count(content, instructionsMarker) != 1 {
		t.Errorf("der Anstoß steht mehrfach:\n%s", content)
	}

	vorher := content
	if _, err := ApplyRootInstructions(root); err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if readInstructions(t, root) != vorher {
		t.Error("der zweite Lauf hat die Datei verändert")
	}
}

// Auch bei der zweiten Fassung wird keine Grenze geraten: umformuliert und
// ohne Folgeabschnitt bleibt sie stehen, und der Hinweis nennt den neuen
// Aufruf, nicht den alten.
func TestKontextFassungMitUnbestimmbaremEndeBleibtMitHinweis(t *testing.T) {
	root := t.TempDir()
	eigen := strings.Replace(veralteterKontextAnstoss,
		"Für dieses Projekt gilt k-playbook. Rufe zu Beginn",
		"Für dieses Projekt gilt k-playbook, bei uns umformuliert. Rufe zu Beginn",
		1) + `
Diese Zeilen gehören dem Projekt und stehen ohne Überschrift dahinter.
`
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}

	repaired, err := RepairRootInstructions(root)
	if err == nil {
		t.Fatal("das unbestimmbare Blockende wurde nicht gemeldet")
	}
	if repaired.Refreshed {
		t.Error("es wurde trotzdem geschrieben")
	}
	if !strings.Contains(err.Error(), "k-playbook briefing") || strings.Contains(err.Error(), "k-playbook context") {
		t.Errorf("der Hinweis nennt nicht den neuen Aufruf: %v", err)
	}
	if readInstructions(t, root) != eigen {
		t.Error("die Datei wurde verändert")
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
	// Die Vorlage ist der Anstoß und sonst nichts: Session-Memory- und
	// Befunde-Block kommen mit dem Briefing.
	if content != rootInstructionsTemplate() {
		t.Errorf("die Datei ist nicht die Vorlage:\n%s", content)
	}
	if hasObsoleteRootBlocks(content) {
		t.Errorf("die Vorlage trägt noch einen abgelösten Zusatzblock:\n%s", content)
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
	if hasObsoleteRootBlocks(string(content)) {
		t.Errorf("ein abgelöster Zusatzblock wurde angehängt:\n%s", content)
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
	if hasObsoleteRootBlocks(string(second)) {
		t.Errorf("ein abgelöster Zusatzblock wurde angehängt:\n%s", second)
	}
}

// Der Anstoß eines Bestandsprojekts nennt noch den abgelösten Wrapper. Weil der
// Marker dasteht, hätte der frühere Ablauf nichts getan — und die Datei zeigte
// dauerhaft auf einen Pfad, den Etappe 4 löscht. Der Git-Update-Weg erreicht
// sie nicht: sie liegt im Hauptverzeichnis, nicht im Clone.
func TestApplyRootInstructionsErsetztVeraltetenBlock(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(veralteterAnstoss), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}

	if _, err := ApplyRootInstructions(root); err != nil {
		t.Fatalf("ApplyRootInstructions: %v", err)
	}

	content := readInstructions(t, root)
	if hasLegacyInstructionsCommand(content) {
		t.Errorf("der abgelöste Wrapper steht noch drin:\n%s", content)
	}
	if !strings.Contains(content, anstossAufruf) {
		t.Errorf("der aktuelle Aufruf fehlt:\n%s", content)
	}
	if strings.Count(content, instructionsMarker) != 1 {
		t.Errorf("der Anstoß steht mehrfach:\n%s", content)
	}
	if hasObsoleteRootBlocks(content) {
		t.Errorf("ein abgelöster Zusatzblock wurde angehängt:\n%s", content)
	}
}

// Ersetzt wird der Block, nicht die Datei: was ein Projekt davor und dahinter
// geschrieben hat, bleibt stehen. Der Session-Memory-Block dazwischen ist
// abgelöst und geht; die Überschrift des Projekts dahinter bleibt.
func TestErsetzenLaesstProjektinhaltStehen(t *testing.T) {
	root := t.TempDir()
	eigen := veralteterAnstoss + "\n" + sessionMemoryBlock() + `
## Unsere eigenen Regeln

Die hier gehören uns.
`
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}

	if _, err := ApplyRootInstructions(root); err != nil {
		t.Fatalf("ApplyRootInstructions: %v", err)
	}

	content := readInstructions(t, root)
	for _, erwartet := range []string{
		"## Unsere eigenen Regeln",
		"Die hier gehören uns.",
		anstossAufruf,
	} {
		if !strings.Contains(content, erwartet) {
			t.Errorf("%q fehlt:\n%s", erwartet, content)
		}
	}
	if hasLegacyInstructionsCommand(content) {
		t.Errorf("der abgelöste Wrapper steht noch drin:\n%s", content)
	}
	if hasObsoleteRootBlocks(content) {
		t.Errorf("ein abgelöster Zusatzblock steht noch drin:\n%s", content)
	}
	if erwartet := rootInstructionsTemplate() + "\n## Unsere eigenen Regeln\n\nDie hier gehören uns.\n"; content != erwartet {
		t.Errorf("Absätze stimmen nicht:\n%q\nerwartet:\n%q", content, erwartet)
	}
}

// Hinter dem Block steht Prosa ohne eigene Überschrift und ohne
// HTML-Kommentar: nichts grenzt sie vom Anstoß ab. Früher reichte der Block
// deshalb bis zum Dateiende, und die Prosa verschwand beim Ersetzen — still,
// denn gemeldet wurde nur „korrigiert".
func TestErsetzenLaesstProsaOhneUeberschriftStehen(t *testing.T) {
	root := t.TempDir()
	eigen := veralteterAnstoss + `
Diese Zeilen gehören dem Projekt. Sie tragen weder eine Überschrift noch einen
HTML-Kommentar, der sie vom Anstoß trennen würde.
`
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}

	repaired, err := RepairRootInstructions(root)
	if err != nil {
		t.Fatalf("RepairRootInstructions: %v", err)
	}
	if !repaired.Refreshed {
		t.Fatal("der veraltete Block wurde nicht ersetzt")
	}

	content := readInstructions(t, root)
	for _, erwartet := range []string{
		"Diese Zeilen gehören dem Projekt.",
		"HTML-Kommentar, der sie vom Anstoß trennen würde.",
		anstossAufruf,
	} {
		if !strings.Contains(content, erwartet) {
			t.Errorf("%q fehlt:\n%s", erwartet, content)
		}
	}
	if hasLegacyInstructionsCommand(content) {
		t.Errorf("der abgelöste Wrapper steht noch drin:\n%s", content)
	}
}

// Ein von Hand umformulierter Block, hinter dem Prosa ohne Überschrift steht:
// weder die bekannte Gestalt noch ein Folgeabschnitt grenzt ihn ab. Dann wird
// die Grenze nicht geraten — die Datei bleibt, wie sie ist, und der Aufrufer
// bekommt einen Hinweis statt eines stillen Verlusts.
func TestErsetzenVerweigertUnbestimmbaresBlockende(t *testing.T) {
	root := t.TempDir()
	eigen := strings.Replace(veralteterAnstoss,
		"Für dieses Projekt gilt k-playbook. Rufe zu Beginn",
		"Für dieses Projekt gilt k-playbook, bei uns umformuliert. Rufe zu Beginn",
		1) + `
Diese Zeilen gehören dem Projekt und stehen ohne Überschrift dahinter.
`
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}

	repaired, err := RepairRootInstructions(root)
	if err == nil {
		t.Fatal("das unbestimmbare Blockende wurde nicht gemeldet")
	}
	if repaired.Refreshed {
		t.Error("es wurde trotzdem geschrieben")
	}
	if readInstructions(t, root) != eigen {
		t.Error("die Datei wurde verändert")
	}

	// Gemeldet bleibt der Block trotzdem als veraltet: er ist es ja. Dass ein
	// Lauf ihn nicht ersetzt, steht daneben.
	if state := CheckRootInstructions(root); !state.HasOutdatedAnstoss || state.AnstossRefreshable || state.Hint == "" {
		t.Errorf("der veraltete Block wird nicht als veraltet, aber nicht ersetzbar gemeldet: %+v", state)
	}
}

// Der Auffangweg beim Start ist eng: er repariert einen veralteten Block und
// rührt sonst nichts an.
func TestRepairRootInstructionsIstEngUndIdempotent(t *testing.T) {
	t.Run("veralteter Block wird ersetzt", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(veralteterAnstoss), 0o644); err != nil {
			t.Fatalf("AGENTS.md anlegen: %v", err)
		}

		repaired, err := RepairRootInstructions(root)
		if err != nil {
			t.Fatalf("RepairRootInstructions: %v", err)
		}
		if !repaired.Refreshed {
			t.Fatal("der veraltete Block wurde nicht gemeldet")
		}

		content := readInstructions(t, root)
		if hasLegacyInstructionsCommand(content) {
			t.Errorf("der abgelöste Wrapper steht noch drin:\n%s", content)
		}
		// Eng: ergänzt wird nichts. Der Session-Memory-Block gehört zum
		// Einrichten, nicht zum Auffangweg.
		if strings.Contains(content, sessionMemoryMarker) {
			t.Errorf("der Auffangweg hat etwas ergänzt:\n%s", content)
		}
		if strings.Contains(content, findingsMarker) {
			t.Errorf("der Auffangweg hat den Befunde-Block ergänzt:\n%s", content)
		}

		vorher := content
		repaired, err = RepairRootInstructions(root)
		if err != nil {
			t.Fatalf("zweiter Lauf: %v", err)
		}
		if repaired.Refreshed {
			t.Error("der zweite Lauf hat erneut geschrieben")
		}
		if readInstructions(t, root) != vorher {
			t.Error("der zweite Lauf hat die Datei verändert")
		}
	})

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
		if repaired.Refreshed {
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
		if repaired.Refreshed {
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
	if repaired.Refreshed {
		t.Error("eine neue Datei wurde als ersetzt gemeldet")
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
	if repaired.Created || repaired.Refreshed {
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
			if repaired.Created || repaired.Refreshed {
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

// Der häufigste Fall in der Praxis: ein Bestandsprojekt trägt den
// `k-playbook context`-Anstoß und beide Zusatzblöcke — genau die Vorlage, wie
// das Binary sie bis zum Briefing schrieb —, dazwischen und dahinter eigenen
// Text. Früher reichte dieser Test den Befunde-Block nach; seit dem Briefing
// werden beide Blöcke entfernt, denn ihr Inhalt kommt als
// `instructions/always/100-…` und `200-…`.
//
// Erwartet wird: der Anstoß ist die Briefing-Fassung, beide Blöcke sind samt
// Marker weg, kein doppelter Absatz bleibt, der Text des Projekts steht
// unverändert da, und der zweite Lauf schreibt nichts mehr.
func TestZusatzbloeckeWerdenBestandsprojektEntfernt(t *testing.T) {
	root := t.TempDir()
	bestand := veralteterKontextAnstoss + "\n" + sessionMemoryBlock() +
		"\n## Dazwischen\n\nEigener Text zwischen den Blöcken.\n\n" + findingsBlock() +
		"\n## Unsere eigenen Regeln\n\nDie hier gehören uns.\n"
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(bestand), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}
	if state := CheckRootInstructions(root); !state.HasOutdatedAnstoss || !state.AnstossRefreshable ||
		len(state.ObsoleteBlocks) != 2 || len(state.RemovableBlocks) != 2 || state.Hint != "" {
		t.Errorf("die Prüfung meldet den Bestand nicht: %+v", state)
	}

	state, err := ApplyRootInstructions(root)
	if err != nil {
		t.Fatalf("ApplyRootInstructions: %v", err)
	}
	if state.NeedsRefresh() {
		t.Errorf("nach dem Lauf ist noch etwas nachzuziehen: %+v", state)
	}

	content := readInstructions(t, root)
	erwartet := rootInstructionsTemplate() +
		"\n## Dazwischen\n\nEigener Text zwischen den Blöcken.\n" +
		"\n## Unsere eigenen Regeln\n\nDie hier gehören uns.\n"
	if content != erwartet {
		t.Errorf("Datei nach dem Lauf:\n%q\nerwartet:\n%q", content, erwartet)
	}

	// Der zweite Lauf hat nichts mehr zu tun.
	if _, err := ApplyRootInstructions(root); err != nil {
		t.Fatalf("zweiter Lauf: %v", err)
	}
	if readInstructions(t, root) != content {
		t.Error("der zweite Lauf hat die Datei verändert")
	}
}

// Ein lokal abgewandelter Zusatzblock — Marker da, Text anders — ohne
// Folgeabschnitt hat keine Grenze. Er bleibt stehen, der Lauf meldet es, und
// die Prüfung trägt den Hinweis.
func TestAbgewandelterZusatzblockBleibtMitHinweis(t *testing.T) {
	root := t.TempDir()
	eigen := rootInstructionsTemplate() + "\n" + findingsMarker + `
## Befunde festhalten

Bei uns umformuliert, und dahinter steht Prosa ohne Überschrift.
`
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}

	state, err := ApplyRootInstructions(root)
	if err == nil {
		t.Fatal("der abgewandelte Block wurde nicht gemeldet")
	}
	if !strings.Contains(err.Error(), "Befunde-Block") {
		t.Errorf("der Hinweis nennt den Block nicht: %v", err)
	}
	if readInstructions(t, root) != eigen {
		t.Error("die Datei wurde verändert")
	}
	if len(state.ObsoleteBlocks) != 1 || state.ObsoleteBlocks[0] != "Befunde-Block" || state.Hint == "" {
		t.Errorf("die Prüfung trägt den Block oder den Hinweis nicht: %+v", state)
	}
	// Ein Lauf entfernte ihn nicht — das sagt die Prüfung getrennt vom Bestand.
	if len(state.RemovableBlocks) != 0 || state.AnstossRefreshable {
		t.Errorf("die Prüfung verspricht, was ein Lauf nicht täte: %+v", state)
	}

	// Der Start sagt dasselbe und schreibt ebenfalls nicht.
	repaired, err := RepairRootInstructions(root)
	if err == nil || repaired.Changed() {
		t.Errorf("der Start hat geschrieben oder nicht gemeldet: %+v, %v", repaired, err)
	}
}

// Eine Datei, die nur die Zusatzblöcke trägt, aber keinen Anstoßmarker: das
// Einrichten hängt den Anstoß an und entfernt die Blöcke in einem Lauf; der
// Start fasst eine Datei ohne Anstoß nicht an und meldet nur.
func TestZusatzbloeckeOhneAnstossmarker(t *testing.T) {
	eigen := "# Unser Projekt\n\nEigene Regeln.\n\n" + sessionMemoryBlock() + "\n" + findingsBlock()

	t.Run("Start meldet nur", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
			t.Fatalf("AGENTS.md anlegen: %v", err)
		}

		repaired, err := RepairRootInstructions(root)
		if err != nil {
			t.Fatalf("RepairRootInstructions: %v", err)
		}
		if repaired.Changed() || !repaired.BlocksWithoutAnstoss {
			t.Errorf("der Start hat geschrieben oder nicht gemeldet: %+v", repaired)
		}
		if readInstructions(t, root) != eigen {
			t.Error("der Start hat die Datei verändert")
		}
	})

	t.Run("Einrichten hängt an und entfernt", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
			t.Fatalf("AGENTS.md anlegen: %v", err)
		}

		if _, err := ApplyRootInstructions(root); err != nil {
			t.Fatalf("ApplyRootInstructions: %v", err)
		}
		content := readInstructions(t, root)
		if erwartet := "# Unser Projekt\n\nEigene Regeln.\n\n" + instructionsBlock(); content != erwartet {
			t.Errorf("Datei nach dem Lauf:\n%q\nerwartet:\n%q", content, erwartet)
		}
	})
}

// Ein abgewandelter Zusatzblock mit Prosa ohne Überschrift dahinter in einer
// Datei ohne Anstoßmarker: Der Anstoß wird angehängt, der Block bleibt samt
// Prosa stehen, der Hinweis kommt — und die Prüfung sagt genau das. Der in
// diesem Lauf angehängte Anstoßmarker darf nie zur Grenze des Blocks werden;
// sonst verschwände die Prosa mit dem Block.
func TestAbgewandelterZusatzblockOhneAnstossmarkerBehaeltProsa(t *testing.T) {
	eigen := "# Unser Projekt\n\n" + findingsMarker + `
## Befunde festhalten

Bei uns umformuliert, und dahinter steht Prosa ohne Überschrift.
`
	erwartet := strings.TrimRight(eigen, "\n") + "\n\n" + instructionsBlock()

	t.Run("Prüfung meldet Anhängen und stehenden Block", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
			t.Fatalf("AGENTS.md anlegen: %v", err)
		}

		state := CheckRootInstructions(root)
		if state.HasMarker || state.HasOutdatedAnstoss || state.AnstossRefreshable {
			t.Errorf("die Prüfung sieht einen Anstoß, den es nicht gibt: %+v", state)
		}
		if len(state.ObsoleteBlocks) != 1 || state.ObsoleteBlocks[0] != "Befunde-Block" {
			t.Errorf("die Prüfung trägt den Block nicht: %+v", state)
		}
		if len(state.RemovableBlocks) != 0 || !strings.Contains(state.Hint, "Befunde-Block") {
			t.Errorf("die Prüfung verspricht Entfernen oder nennt den Block nicht im Hinweis: %+v", state)
		}
	})

	t.Run("Einrichten hängt an und lässt Block samt Prosa stehen", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
			t.Fatalf("AGENTS.md anlegen: %v", err)
		}
		vorher := CheckRootInstructions(root)

		state, err := ApplyRootInstructions(root)
		if err == nil || !strings.Contains(err.Error(), "Befunde-Block") {
			t.Errorf("der Lauf meldet den stehenden Block nicht: %v", err)
		}
		content := readInstructions(t, root)
		if content != erwartet {
			t.Errorf("Datei nach dem Lauf:\n%q\nerwartet:\n%q", content, erwartet)
		}
		if !strings.Contains(content, "Bei uns umformuliert, und dahinter steht Prosa ohne Überschrift.") {
			t.Error("die Prosa hinter dem Block ist verloren")
		}
		if !strings.Contains(content, anstossAufruf) {
			t.Error("der Anstoß fehlt")
		}

		// Der Lauf tat genau das, was die Prüfung vorher ankündigte: Anstoß
		// angehängt, nichts entfernt, der Hinweis bleibt.
		if !state.HasMarker || len(state.ObsoleteBlocks) != 1 || len(state.RemovableBlocks) != 0 || state.Hint == "" {
			t.Errorf("Zustand nach dem Lauf: %+v", state)
		}
		if len(vorher.RemovableBlocks) != 0 || vorher.Hint != state.Hint {
			t.Errorf("Prüfung und Lauf sagen Verschiedenes: vorher %+v, nachher %+v", vorher, state)
		}

		// Der zweite Lauf ändert nichts mehr und meldet weiter: Der jetzt
		// angehängte Anstoß ist für den Block keine Grenze.
		if _, err := ApplyRootInstructions(root); err == nil {
			t.Error("der zweite Lauf meldet den stehenden Block nicht mehr")
		}
		if readInstructions(t, root) != erwartet {
			t.Error("der zweite Lauf hat die Datei verändert")
		}

		// Der Start sieht den Marker und repariert — den Block samt Prosa
		// fasst auch er nicht an.
		repaired, err := RepairRootInstructions(root)
		if err == nil || repaired.Changed() {
			t.Errorf("der Start hat nach dem Lauf geschrieben oder nicht gemeldet: %+v, %v", repaired, err)
		}
		if readInstructions(t, root) != erwartet {
			t.Error("der Start hat die Datei nach dem Lauf verändert")
		}
	})

	t.Run("Start meldet nur", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
			t.Fatalf("AGENTS.md anlegen: %v", err)
		}

		repaired, err := RepairRootInstructions(root)
		if err != nil || repaired.Changed() || !repaired.BlocksWithoutAnstoss {
			t.Errorf("der Start hat geschrieben oder nicht gemeldet: %+v, %v", repaired, err)
		}
		if readInstructions(t, root) != eigen {
			t.Error("der Start hat die Datei verändert")
		}
	})
}

// Eine Datei, die nur die Zusatzblöcke trägt und sonst nichts: nach dem
// Entfernen bleibt nichts, wovor der Anstoß stünde — er beginnt die Datei,
// ohne leeren Absatz davor.
func TestNurZusatzbloeckeWerdenZumAnstoss(t *testing.T) {
	root := t.TempDir()
	eigen := sessionMemoryBlock() + "\n" + findingsBlock()
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(eigen), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}

	if _, err := ApplyRootInstructions(root); err != nil {
		t.Fatalf("ApplyRootInstructions: %v", err)
	}
	if content := readInstructions(t, root); content != instructionsBlock() {
		t.Errorf("Datei nach dem Lauf:\n%q\nerwartet:\n%q", content, instructionsBlock())
	}
}

// Der Start entfernt die Zusatzblöcke genauso wie das Einrichten — sie sind
// Inhalt, den k-playbook selbst geschrieben hat — und meldet, was er entfernt
// hat. Aus der vollständigen alten Vorlage wird so die neue.
func TestRepairRootInstructionsEntferntZusatzbloecke(t *testing.T) {
	root := t.TempDir()
	alteVorlage := veralteterAnstoss + "\n" + sessionMemoryBlock() + "\n" + findingsBlock()
	if err := os.WriteFile(filepath.Join(root, RootInstructionsFile), []byte(alteVorlage), 0o644); err != nil {
		t.Fatalf("AGENTS.md anlegen: %v", err)
	}

	repaired, err := RepairRootInstructions(root)
	if err != nil {
		t.Fatalf("RepairRootInstructions: %v", err)
	}
	if !repaired.Refreshed || len(repaired.RemovedBlocks) != 2 {
		t.Errorf("der Start meldet nicht, was er getan hat: %+v", repaired)
	}
	if content := readInstructions(t, root); content != rootInstructionsTemplate() {
		t.Errorf("aus der alten Vorlage wurde nicht die neue:\n%q", content)
	}

	repaired, err = RepairRootInstructions(root)
	if err != nil || repaired.Changed() {
		t.Errorf("der zweite Lauf hat geschrieben: %+v, %v", repaired, err)
	}
}
