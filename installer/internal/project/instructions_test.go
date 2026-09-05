package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// anstossAufruf ist die Zeile, an der der aktuelle Anstoß erkennbar ist: das
// installierte k-playbook unter seinem Namen, eingerückt als Codeblock.
//
// Bewusst mit Einrückung und Zeilenumbrüchen und nicht als bloßes
// "k-playbook context": das steht auch im abgelösten
// "k-playbook/bin/k-playbook context" drin, und die Prüfung liefe grün, während
// der alte Aufruf dastünde.
const anstossAufruf = "\n    k-playbook context\n"

// veralteterAnstoss ist der Anstoßblock, wie ihn Bestandsprojekte tragen.
const veralteterAnstoss = `# AGENTS.md

<!-- k-playbook:anstoss -->
## k-playbook

Für dieses Projekt gilt k-playbook. Rufe zu Beginn

    k-playbook/bin/k-playbook context

auf und lies die Dateien aus ` + "`instructions`" + ` in der angegebenen Reihenfolge,
bevor du arbeitest. Die Ausgabe nennt außerdem die aufgelösten Verzeichnisse und
die effektiven Kataloge für Regeln, Reviews und Checks.
`

func TestApplyRootInstructionsLegtDateiAn(t *testing.T) {
	root := t.TempDir()

	state, err := ApplyRootInstructions(root)
	if err != nil {
		t.Fatalf("ApplyRootInstructions: %v", err)
	}
	if !state.OK() {
		t.Fatalf("nicht eingerichtet: %+v", state)
	}

	content, err := os.ReadFile(filepath.Join(root, RootInstructionsFile))
	if err != nil {
		t.Fatalf("AGENTS.md lesen: %v", err)
	}
	if !strings.Contains(string(content), anstossAufruf) {
		t.Errorf("Anstoß fehlt:\n%s", content)
	}
	if !strings.Contains(string(content), "k-playbook-local/docs/README.md") {
		t.Errorf("Session-Memory-Verweis fehlt:\n%s", content)
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
	if !strings.Contains(string(content), "k-playbook-local/docs/README.md") {
		t.Errorf("Session-Memory-Verweis fehlt:\n%s", content)
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
	if strings.Contains(content, legacyInstructionsCommand) {
		t.Errorf("der abgelöste Wrapper steht noch drin:\n%s", content)
	}
	if !strings.Contains(content, anstossAufruf) {
		t.Errorf("der aktuelle Aufruf fehlt:\n%s", content)
	}
	if strings.Count(content, instructionsMarker) != 1 {
		t.Errorf("der Anstoß steht mehrfach:\n%s", content)
	}
	if !strings.Contains(content, sessionMemoryMarker) {
		t.Errorf("der Session-Memory-Block fehlt:\n%s", content)
	}
}

// Ersetzt wird der Block, nicht die Datei: was ein Projekt davor und dahinter
// geschrieben hat, bleibt stehen.
func TestErsetzenLaesstProjektinhaltStehen(t *testing.T) {
	root := t.TempDir()
	eigen := veralteterAnstoss + `
<!-- k-playbook:session-memory -->
## Projektwissen zuerst

Steht schon da.

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
		"Steht schon da.",
		anstossAufruf,
	} {
		if !strings.Contains(content, erwartet) {
			t.Errorf("%q fehlt:\n%s", erwartet, content)
		}
	}
	if strings.Contains(content, legacyInstructionsCommand) {
		t.Errorf("der abgelöste Wrapper steht noch drin:\n%s", content)
	}
	if strings.Count(content, sessionMemoryMarker) != 1 {
		t.Errorf("der Session-Memory-Block steht mehrfach:\n%s", content)
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
	if !repaired {
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
	if strings.Contains(content, legacyInstructionsCommand) {
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
	if repaired {
		t.Error("es wurde trotzdem geschrieben")
	}
	if readInstructions(t, root) != eigen {
		t.Error("die Datei wurde verändert")
	}

	// Gemeldet bleibt der Block trotzdem als veraltet: er ist es ja.
	if state := CheckRootInstructions(root); !state.HasOutdatedAnstoss {
		t.Errorf("der veraltete Block wird nicht mehr als veraltet gemeldet: %+v", state)
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
		if !repaired {
			t.Fatal("der veraltete Block wurde nicht gemeldet")
		}

		content := readInstructions(t, root)
		if strings.Contains(content, legacyInstructionsCommand) {
			t.Errorf("der abgelöste Wrapper steht noch drin:\n%s", content)
		}
		// Eng: ergänzt wird nichts. Der Session-Memory-Block gehört zum
		// Einrichten, nicht zum Auffangweg.
		if strings.Contains(content, sessionMemoryMarker) {
			t.Errorf("der Auffangweg hat etwas ergänzt:\n%s", content)
		}

		vorher := content
		repaired, err = RepairRootInstructions(root)
		if err != nil {
			t.Fatalf("zweiter Lauf: %v", err)
		}
		if repaired {
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
		if repaired {
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
		if repaired {
			t.Error("ohne Anstoß wurde geschrieben")
		}
		if readInstructions(t, root) != eigen {
			t.Error("eine fremde Datei wurde verändert")
		}
	})

	t.Run("fehlende Datei wird nicht angelegt", func(t *testing.T) {
		root := t.TempDir()

		repaired, err := RepairRootInstructions(root)
		if err != nil {
			t.Fatalf("RepairRootInstructions: %v", err)
		}
		if repaired {
			t.Error("eine fehlende Datei wurde gemeldet")
		}
		if _, err := os.Stat(filepath.Join(root, RootInstructionsFile)); !os.IsNotExist(err) {
			t.Error("der Auffangweg hat AGENTS.md angelegt")
		}
	})
}

func readInstructions(t *testing.T, root string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(root, RootInstructionsFile))
	if err != nil {
		t.Fatalf("AGENTS.md lesen: %v", err)
	}
	return string(content)
}
