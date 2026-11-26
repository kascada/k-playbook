---
title: Baustein Erheben
description: Baustein der Review-Familie, eingebunden als include — ein Rezept auf einen Scope anwenden und seine Funde als SARIF-Fundliste <eintrag>.sarif in das Arbeitsverzeichnis schreiben, nach dem Evidence-Vertrag des Rezepts (audit.mode evidence, audit.ruleIds, audit.scope.paths), und die Fundliste mit k_playbook_review_merge_dir im Prüfmodus prüfen; fasst nie zusammen.
---

Erheben macht aus einem Rezept und einem Scope eine Fundliste: eine SARIF-Datei im
Arbeitsverzeichnis. Es fasst nie zusammen — keine Bündel, keine Priorität, keine Kategorie,
keine Ergebnisdatei. Das ist Sache des Bausteins Zusammenfassen.

Der aufrufende Command nennt:

- `RECIPE_KEY` — den Schlüssel des Rezepts (`code` für `review-code.md`) und den Text des
  Rezepts, den er mit `k_playbook_doc_read` gelesen hat,
- `SCOPE` — die Dateien und Verzeichnisse, die geprüft werden, mit den Ausschlüssen,
- `WORK_DIR` — das Arbeitsverzeichnis, absolut.

**Vertrag des Rezepts.** Eine Fundliste erhebt nur ein Rezept mit `audit.mode: evidence`,
einer Liste `audit.ruleIds` und `audit.scope.paths` im Frontmatter. Fehlt davon etwas, brich
ab und nenne das Rezept und das fehlende Feld; schreibe keine Fundliste.

**Scope.** Geprüft wird `SCOPE`, soweit er in `audit.scope.paths` liegt; innerhalb der Globs
gelten zusätzlich die zentralen Ausschlüsse der Modulsuche (`k-playbook`, `vendor`,
`node_modules`, `testdata`, Punkt-Verzeichnisse) und die Ausschlüsse des Rezepts. Ein Fund
außerhalb von `audit.scope.paths` wird beim Prüfen verworfen und gezählt; melde ihn gar
nicht erst.

**Erheben.** Wende die Kriterien des Rezepts auf den Code im Scope an. Was ein Fund ist,
welche Rule-ID welches `level` trägt und wie viele Funde höchstens gemeldet werden, steht im
Rezept, im Abschnitt „Ergebnisform (SARIF)“ und den Abschnitten davor.

**Fundliste schreiben.** Schreibe genau eine Datei, `WORK_DIR/<RECIPE_KEY>.sarif`, mit dem
Schreibwerkzeug. Form:

```json
{
  "version": "2.1.0",
  "runs": [{
    "tool": { "driver": { "name": "<RECIPE_KEY>" } },
    "results": [{
      "ruleId": "<rule-id aus audit.ruleIds>",
      "level": "error|warning|note",
      "message": { "text": "<ein Satz: was dort steht und welche Folge es hat>" },
      "locations": [{ "physicalLocation": {
        "artifactLocation": { "uri": "<pfad relativ zur Projektwurzel>" },
        "region": { "startLine": 12 }
      }}]
    }]
  }]
}
```

- `tool.driver.name` ist der Schlüssel des Rezepts, nicht der Dateiname des Rezepts.
- Jeder Fund trägt eine `ruleId` aus `audit.ruleIds`, das dort festgelegte `level`, genau
  einen Fundort mit projektrelativem Pfad und Startzeile und eine `message.text` von einem
  Satz.
- Mehrere Instanzen desselben Problems in derselben Datei sind mehrere Funde. Fasse sie
  nicht selbst zusammen: der Merge gruppiert sie je Rule-ID und Datei.
- Eine Fundliste ohne Ergebnisse (`"results": []`) ist gültig: ein leerer Befund ist ein
  Ergebnis.
- Gib die Fundliste nicht in den Chat aus; eine Rohausgabe kann Secrets enthalten.

**Prüfen.** Rufe `k_playbook_review_merge_dir` mit `projectDir` = `project.dir`,
`dir` = `WORK_DIR` und `check: true`, ersatzweise in der Shell
`k-playbook review merge <WORK_DIR> --check`. Die Prüfung schreibt nichts. Sie verlangt:
`tool.driver.name` gleich Schlüssel, jede `ruleId` aus `audit.ruleIds`, einen Fundort je Fund.

- **Gültig:** `sources[]` nennt je Fundliste `findings` und `droppedFindings`. Sind Funde
  verworfen worden, nenne ihre Zahl und `droppedPaths`: das Rezept hat über seinen Scope
  hinausgegriffen.
- **`sarif_invalid`:** `error.details.problems` nennt je Datei den Grund. Ein Fund, der
  keiner Rule-ID der Liste zuzuordnen ist, ist kein Fund dieses Rezepts. Korrigiere die
  Fundliste nach den Kriterien des Rezepts und prüfe erneut — höchstens zweimal; bleibt sie
  ungültig, brich ab und nenne den Grund. Eine Rule-ID wird nie erfunden oder umgedeutet,
  damit die Prüfung durchgeht.
