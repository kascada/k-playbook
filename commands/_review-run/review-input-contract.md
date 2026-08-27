# review-input-contract

Dieses Modul ist kein Review-Katalog-Rezept. Es beschreibt `review-input.json` — den
Belegvertrag, auf dem die Triage arbeitet — und ist die einzige Stelle, an der dieses
Schema beschrieben wird. Commands, Rezepte und Doku verweisen darauf, statt es zu
wiederholen.

Eingebunden wird es von den beiden Lauf-Commands:

- `/k-audit` — mittelbar über `commands/_audit/review-scan-triage.md`, das es in
  Schritt 8 wortlaut-treu anwendet. Geschrieben wird die Datei dort nicht vom Command,
  sondern in Schritt 6 vom Go-Merge.
- `/k-review` — unmittelbar in Step 5b, dem Report-Modus. Dort schreibt der Agent die
  Datei von Hand, in den Family-Ordner aus `result-family`.

Es liegt deshalb im Familien-Namensraum `commands/_review-run/` und nicht im
Modulverzeichnis eines einzelnen Commands.

Die Wahrheit über das Schema ist der Merge-Code unter
`installer/internal/review/merge/` — `merge.go`, `write.go`, `dedupe.go`, `stable.go`.
Was hier steht, ist gegen diesen Code geprüft. Weicht der Code ab, gilt der Code, und
dieser Vertrag wird nachgezogen; nicht umgekehrt.

## Zwei Wege, ein Vertrag

`review-input.json` entsteht auf zwei Wegen, und sie können nicht dasselbe leisten:

- **Audit-Weg.** `k_playbook_review_merge` liest `run.json`, `entries/*.json` und die
  SARIF-Dateien unter `raw/` und schreibt die Datei. Nur dort wird dedupliziert, Schwere
  abgeleitet und `known-decisions.md` gematcht.
- **Report-Weg.** `/k-review` führt keine MCP-Werkzeuge; der Merge steht dort nie zur
  Verfügung. Der Agent schreibt die Datei selbst aus dem, was das Rezept erhoben hat.

Der Vertrag ist darum zweigeteilt:

- **Kern** — was beide Wege schulden. Die Triage darf sich darauf verlassen.
- **Merge-only** — was erst im Merge entsteht. Der Report-Weg lässt diese Felder weg
  oder leer; die Triage darf sich auf keines von ihnen verlassen.

Einen report-only-Teil gibt es nicht. Ein Feld, das der Merge nicht kennt, wird entweder
Teil des Kerns — dann zieht der Merge nach — oder es steht nicht im Vertrag. Ein Feld,
das nur der Report-Weg füllte, wäre eine zweite Fassung des Schemas und genau die
Doppelung, gegen die dieses Modul steht.

## Kern

Das Gerüst, das beide Wege schreiben. Weggelassene Felder aus dem merge-only-Teil sind
zulässig und bedeuten nicht, dass die Datei unvollständig ist.

```json
{
  "generated": "<RFC3339-Zeitstempel>",
  "run": {
    "name": "<lauf-oder-family-date>",
    "dir": "<projektrelativer Ordner der Datei>",
    "selectedEntries": [
      { "name": "<eintrag>", "kind": "ai", "state": "done", "mode": "evidence" }
    ]
  },
  "findings": [
    {
      "id": "<in dieser Datei eindeutig>",
      "evidence": { "tool": "<eintrag oder werkzeug>", "job": "<job>" },
      "ruleId": "<rule-id>",
      "level": "error|warning|note|none",
      "message": "<einzeilige Meldung>",
      "location": { "uri": "<pfad>", "startLine": 12 },
      "dependency": {
        "package": "…", "version": "…", "manifest": "…", "ids": ["CVE-…"],
        "keyIds": ["CVE-…"], "textPackage": "…", "textVersion": "…"
      }
    }
  ],
  "groups": [
    {
      "displayId": "G001",
      "stableId": "<siehe Stabile Gruppen-IDs>",
      "title": "<Repräsentant der Gruppe>",
      "ruleId": "<rule-id>",
      "level": "error|warning|note|none",
      "location": { "uri": "<pfad>", "startLine": 12 },
      "findingIds": ["<id>", "<id>"],
      "evidence": [{ "tool": "<eintrag oder werkzeug>", "job": "<job>" }]
    }
  ]
}
```

Was die Felder tragen:

| Feld | Inhalt |
|---|---|
| `generated` | Erzeugungszeit der Datei, RFC3339. |
| `run.name` | Name des Laufs. Im Audit das Laufdatum, im Report-Weg `<result-family>/<YYYY-MM-DD>`. |
| `run.dir` | Ordner der Datei, relativ zur Projektwurzel. |
| `run.selectedEntries[]` | Die Einträge, die zu diesem Beleg beigetragen haben, mit `name`, `kind` (`tool` oder `ai`), `state` und — bei `kind: ai` — `mode` (`evidence` oder `perspective`). `mode` beschreibt die Rolle **in diesem Beleg**: `evidence`, wenn der Eintrag die Funde erhoben hat, `perspective`, wenn er einen fertigen Beleg bewertet. Im Audit kommt der Wert aus `audit.mode` des Rezepts; fehlt er dort, normalisiert `review.NormalizeMode` ihn zu `perspective`. Im Report-Weg gibt es genau einen Eintrag — das ausgeführte Rezept, `kind: ai` —, und er trägt `evidence`, weil er die Funde selbst erhoben hat. Das ist unabhängig davon, was `audit.mode` desselben Rezepts für den Audit-Lauf festlegt; ein Rezept mit `audit.enabled: false` hat gar keines. |
| `findings[].id` | In der Datei eindeutig. Der Merge bildet ihn als `<sarif>:<runIndex>:<resultIndex>`; der Report-Weg nimmt den stabilen Schlüssel der Quelle, z. B. die Alert-Nummer. |
| `findings[].evidence.tool` | Bei Scanner-Belegen der Werkzeugname (`gosec`), bei Rezept-Belegen der Eintragsname (`tech`). Es gibt kein zweites Feld für die Quelle: `source` existiert nicht. |
| `findings[].evidence.job` | Der Scan-Job. Bei einem Rezept-Beleg heißt er wie der Eintrag. |
| `findings[].ruleId` | Die Rule-ID. Bei Evidence-Rezepten aus deren `audit.ruleIds`. |
| `findings[].level` | Der SARIF-Level. Er ist die einzige Wertung, die ein Rezept vergibt. |
| `findings[].message` | Die Meldung des Fundes. |
| `findings[].location` | `uri` als projektrelativer Pfad, `startLine` und `startColumn` optional. |
| `findings[].dependency` | Nur bei Dependency-Befunden: `package`, `version`, `manifest`, `ids`. Fehlt sonst. Drei weitere Felder schreibt **nur** der Merge; der Report-Weg lässt sie weg. `keyIds` sind die Kennungen, die den Fund *benennen* (`ruleId` und benannte Alias-Felder) — nur sie gehen in den harten Dedupe-Schlüssel und in die stabile Gruppen-ID. Sie sind kein bloßes Schlüsseldetail, sondern die Antwort auf „worum geht dieser Fund?" gegenüber `ids`, das auch nennt, was das Advisory nur *erwähnt*: wer zwei Belege inhaltlich zusammenfasst, weil sie eine Kennung teilen, muss die beiden Mengen auseinanderhalten können. `package` und `version` tragen dort ausschließlich **strukturiert** gelesene Werte, also eine benannte Property oder einen purl. Was nur im Fließtext des Werkzeugs steht, steht in `textPackage` / `textVersion` und ist ausdrücklich nur für die Anzeige — es bildet keinen harten Schlüssel und rechtfertigt kein Zusammenlegen. Fehlen `package` und `version`, ist der Fund nicht schlechter belegt; sein Werkzeug nennt sie nur nicht als Feld (osv-scanner, trivy) — dann sind `textPackage` / `textVersion` die einzige Auskunft darüber, welches Paket gemeint ist, und ohne sie müsste jeder Leser die Meldung noch einmal selbst zerlegen. Genau deshalb stehen sie im Beleg und nicht nur im Merge. |
| `groups[].displayId` | Laufende Anzeige-ID `G001`, `G002`, … Sie ist nicht stabil und taugt nicht als Referenz über Läufe hinweg. |
| `groups[].stableId` | Die stabile Gruppen-ID. Sie ist die Referenz, die in `review-triage.md` und in `known-decisions.md` steht. |
| `groups[].title`, `ruleId`, `level`, `location` | Der Repräsentant der Gruppe. |
| `groups[].findingIds` | Die IDs der Findings dieser Gruppe. Ihre Anzahl ist die Zahl der Instanzen. |
| `groups[].evidence[]` | Mindestens ein Eintrag, gleiche Form wie `findings[].evidence`. |

Zwei Regeln über den Feldern:

- **Jedes Finding gehört zu genau einer Gruppe.** Im Merge ergibt sich das aus dem
  Union-Find über die harten Schlüssel: jeder Fund wird einem Wurzelknoten und damit
  einer Gruppe zugeordnet, auch wenn er allein steht. Der Report-Weg hält es genauso —
  ein Fund ohne Gruppe wäre im Beleg unsichtbar. Ein Feld für gruppenlose Findings gibt
  es deshalb nicht.
- **Gruppen bilden, nicht Findings verwerfen.** Eine Gruppe fasst zusammen; sie löscht
  nichts. Alle Belege und alle Finding-IDs bleiben erhalten.

## Merge-only

Diese Felder entstehen erst im Merge. Der Report-Weg lässt sie weg. Die Triage prüft
ihr Vorhandensein und arbeitet ohne sie weiter, statt sie zu erfinden.

| Feld | Inhalt | Fehlt es, gilt |
|---|---|---|
| `schemaVersion` | Version des Merge-Schemas, derzeit `1`. | Kein Merge-Artefakt; keine Aussage über die Schemafassung. |
| `kPlaybookVersion` | Version des Werkzeugs, das den Merge gerechnet hat. | Keine Werkzeug-Provenienz; die Herkunft steht in `run` und `evidence`. |
| `run.created`, `run.state`, `run.derivedState`, `run.languages` | Zustand und Sprachen des Audit-Laufs aus `run.json`. | Kein Audit-Lauf. Der Zustand ist der des einen Report-Rezepts. |
| `run.selectedEntries[].recipeKey`, `recipePath`, `recipeOrigin`, `title`, `resultRequired`, `defaultResult`, `scope` | Der beim Laufstart eingefrorene Rezept-Snapshot aus `run.json`. | Es gibt keinen Snapshot. Der Eintrag ist das Rezept, das gerade läuft. |
| `entries[]` | Eintrags- und Job-Status aus `entries/*.json`, mit SARIF-Pfaden und Fundzahlen. | Keine Eintragsdateien. Was beigetragen hat, steht in `run.selectedEntries` und in `evidence`. |
| `findings[].derivedSeverity`, `severitySource` | Abgeleitete Schwere aus SARIF-Level, CVSS, Tool-Metadaten und `scripts/severity.tsv`. | `level` gilt unmittelbar und ist die einzige Wertung. Keine eigene Ableitung improvisieren. |
| `findings[].ruleName`, `ruleDescription` | Regeltexte aus dem SARIF-Regelkatalog. | Nur `ruleId` und `message` beschreiben den Fund. |
| `findings[].locations[]` | Alle Orte eines Fundes, nicht nur der erste. | Es gilt `location`. |
| `findings[].fingerprints`, `partialFingerprints` | Werkzeug-Fingerprints aus dem SARIF. | Kein Fingerprint-Abgleich möglich. |
| `evidence.sarif`, `evidence.runIndex`, `evidence.resultIndex` | Rückverweis in die SARIF-Rohdatei. | Kein `raw/`-Rückverweis. `tool` und `job` bleiben. |
| `groups[].stableKey` | Der Klartext-Schlüssel, aus dem der Digest der stabilen ID gebildet wurde. | Die stabile ID ist nach der Report-Regel gebildet und nicht aus einem Digest herleitbar. |
| `groups[].dedupeRules`, `possibleDuplicates` | Welche Regel zusammengefasst hat und welche Gruppen unsichere Dubletten sind. | Keine maschinelle Dedupe-Herleitung. Die Bündelung ist Sache der Triage. |
| `groups[].derivedSeverity`, `severitySource` | Wie beim Finding, für den Repräsentanten. | Wie beim Finding: `level` gilt unmittelbar. |
| `groups[].dependency` | Dependency-Angaben des Repräsentanten. `ids` und `keyIds` sind die Vereinigung über die Findings der Gruppe, die dieselbe Dependency beschreiben; `package`, `version`, `manifest` und die Freitextfelder bleiben die des Repräsentanten und lassen sich nicht vereinigen. | Steht bei den Findings, sofern die Quelle sie kennt. |
| `knownDecisions` | Geladene Quellen, Decisions mit `applied` / `notAppliedReason` / `expired` und Warnungen. | Es gab kein zentrales Matching. Im Report-Weg lädt `/k-review` `known-decisions.md` in Step 3 selbst und meldet gedeckte Funde gar nicht erst als Fund. |
| `findings[].coveredByKnownDecision`, `groups[].coveredByKnownDecision`, `groups[].partialCoverage`, `groups[].knownDecisionCoverage` | Ergebnis des Matchings je Fund und je Gruppe, mit `matchedBy`. | Keine Deckungsmarker im Beleg. Der Abschnitt `## Deckung aus known-decisions` bleibt stehen und sagt in einem Satz, dass der Beleg kein Matching trägt und warum. |

## Stabile Gruppen-IDs

`groups[].stableId` ist die einzige Gruppen-Referenz, die über Läufe hinweg trägt.
`displayId` wechselt mit der Reihenfolge und wird nie als Referenz benutzt.

Die **Bildung** ist herkunftsabhängig und bleibt zweigeteilt. Sie wird hier beschrieben,
nicht vereinheitlicht: der Digest des Merge ist ohne Merge-Code nicht reproduzierbar.

**Audit-Weg — `merge/stable.go`.** Präfix aus der Klasse der Gruppe, dahinter ein
gekürzter SHA256-Digest über normalisierte Attribute der Funde:

- `ai-<eintrag>-<digest>` — alle Funde der Gruppe stammen aus einem Evidence-Rezept.
  Der Schlüssel besteht nur aus Eintragsname, Rule-ID und normalisiertem Pfad; Zeile,
  Meldung und Fingerprints bleiben draußen, damit die ID über Läufe hinweg hält.
- `scan-cve-<kennung>-<digest>` — die Gruppe trägt eine Dependency-Kennung.
  `<kennung>` ist die alphabetisch erste aus der **engen** Kennungsmenge der Gruppe:
  aus den Kennungen, die den Fund selbst benennen (`ruleId` und benannte Alias-Felder,
  siehe `findings[].dependency.keyIds`), nicht aus denen, die im Advisory-Text nebenbei
  vorkommen. Nennt ein Werkzeug seine einzige Kennung nur im Freitext, fällt die Bildung
  auf die breite Menge `ids` zurück. Dieselbe enge Menge steht auch in der
  `dependencies`-Zeile des Schlüssels.
- `scan-<werkzeug>-<digest>` — alles Übrige; `<werkzeug>` ist `multi`, wenn mehrere
  Werkzeuge beteiligt sind.

**Pfade im Schlüssel sind eingefroren normiert.** Überall, wo ein Pfad in den Schlüssel
oder in die Klassenentscheidung eingeht — die Zeilen `locations`, `dependencies` und
`paths` —, läuft er über eine eigene, festgeschriebene Normierung und **nicht** über die
der Gruppierung: Backslashes zu `/`, `file://` samt Authority weg, `.`- und
`..`-Segmente sowie doppelte Slashes aufgelöst, führendes `/` weg, Kleinschreibung. Die
Trennung ist Absicht: die Dedupe-Normierung darf besser werden, ohne die IDs zu bewegen.
Ändert sich dagegen die eingefrorene Fassung, verschieben sich Stable-IDs — Begründung
und Folgen für `known-decisions.md` stehen in `docs/review-runs.md`.

Die Einengung auf die enge Menge gilt für Präfix und Digest gemeinsam und ist Absicht:
über die breite Menge bestimmte eine beiläufig im Advisory-Text genannte Fremd-Kennung
das Präfix, sobald sie alphabetisch vorne lag, und **jede** zusätzlich genannte Kennung
verschob die ID, obwohl sich am Befund nichts geändert hatte. Dieselbe Gruppe mit
breiterer Aliasmenge behält jetzt Schlüssel, Präfix und Klasse — solange die zusätzliche
Kennung außerhalb der engen Menge liegt. An der Klasse ändert die Einengung nichts: die
enge Menge ist genau dann leer, wenn auch die breite leer ist.

Der Digest wird auf sechs Zeichen gekürzt und nur so weit verlängert, wie er je Präfix
eindeutig sein muss. Eine Gruppe, in der Scanner- und Rezept-Belege zusammenliegen,
bleibt bewusst `scan-…`: sonst verlöre eine bestehende Scanner-Gruppe ihre ID, sobald ein
Rezept-Fund hinzukommt.

**Report-Weg.** Der Merge steht nicht zur Verfügung, der Digest ist nicht nachzurechnen.
Die ID wird deshalb aus dem gebildet, was die Quelle selbst stabil hergibt:

```text
ai-<eintrag>-<schlüssel>
```

- `<eintrag>` ist der Eintragsname des Rezepts — der Katalog-Schlüssel ohne `review-`.
  Das Präfix ist `ai-`, weil der Beleg aus einem Rezept kommt und nicht aus einem
  Scan-Job. Es sagt die Herkunft an, nicht die Gleichheit mit einer Audit-ID: dieselbe
  Gruppe bekäme im Audit-Lauf einen anderen Digest, und ein Rezept, das dort als
  Perspektive läuft, brächte überhaupt keine eigene Gruppe in den Beleg.
- `<schlüssel>` ist die kleinste Kennung, die die Quelle selbst vergibt und die einen
  Re-Run überlebt: die Alert- oder Ticket-Nummer, die CVE-/GHSA-Kennung, sonst Rule-ID
  und Pfad. Kleinbuchstaben, `a`–`z` und `0`–`9`; alles andere wird zu einem Bindestrich,
  Bindestriche am Rand fallen weg.
- Beispiel: `ai-dependabot-alerts-81` für GitHub-Alert 81.

**Verboten sind laufende Nummern.** `review-<family>-001` und alles in dieser Form ist
keine stabile ID: verschwindet ein Fund, rutschen alle folgenden IDs auf einen anderen
Befund, und jede Decision, die auf eine solche ID zeigt, deckt danach den falschen. Der
Merge erzeugt eine solche ID auch nie.

## Herkunft der Belege

Belege kommen aus zwei Quellen: aus Scannern und aus Rezepten, die im Lauf als
Evidence-Quelle arbeiten. Der Beleg unterscheidet sie ohne eigenes Schemafeld, an drei
Stellen:

- `evidence.tool` trägt bei Rezept-Belegen den Eintragsnamen (z. B. `tech` für
  `review-tech.md`), bei Scanner-Belegen den Werkzeugnamen (z. B. `gosec`).
- Das Präfix der stabilen Gruppen-ID: `ai-<eintrag>-` gegen `scan-<werkzeug>-` oder
  `scan-cve-<kennung>-`.
- `run.selectedEntries[].mode` nennt je Eintrag die Betriebsart `evidence` oder
  `perspective`.

Im Report-Weg gibt es nur eine Quelle — das ausgeführte Rezept. Alle Gruppen tragen dort
das Präfix `ai-<eintrag>-`, und `run.selectedEntries` hat genau einen Eintrag.

## Was der Vertrag nicht kennt

Drei Felder standen in Prosa, ohne je geschrieben zu werden. Sie sind gestrichen; kein
Command und keine Doku führt sie weiter:

- **Top-Level `scope`** mit `scope.type` und `scope.family`. Das `Result`-Struct kennt
  es nicht. Es wäre auch redundant: `run.name` und `run.dir` tragen dieselbe Auskunft,
  und im Report-Weg ist das Family-Date-Verzeichnis der Ordner der Datei selbst.
- **`ungroupedFindings`.** Der Merge kann sie nicht erzeugen — das Union-Find gibt jedem
  Fund eine Gruppe, das Feld wäre strukturell immer leer. Der Abschnitt
  `## Nicht gebündelt` in `review-triage.md` speist sich deshalb aus den **Gruppen**, die
  die Triage keinem Bündel zugeordnet hat, nicht aus einem Feld des Belegs.
- **`groups[].id`.** Auf Gruppen gibt es `displayId` und `stableId`, sonst nichts.

Ebenso gibt es kein `evidence[].source`, `evidence[].file`, `evidence[].line` oder
`evidence[].message`. Die Quelle steht in `evidence.tool`, Ort und Meldung am Finding.

## Bestandsartefakte

Bestehende `review-input.json` unter `k-playbook-local/results/<YYYY-MM-DD>/` bleiben
unverändert lesbar: Felder sind nur hinzugekommen, keines ist entfallen oder umbenannt
worden, und `schemaVersion` bleibt `1`.

**Die stabilen Gruppen-IDs sind dagegen verschoben.** Ein Lauf, der mit dem heutigen
Merge neu zusammengeführt wird, vergibt für dieselben Funde andere `stableId` als eine
ältere Fassung des Werkzeugs. Vier Änderungen wirken zusammen: die erweiterte
Pfadnormierung, der Dedupe-Schlüssel je Kennung, das aus purl-Quellen nachgeholte Paket
samt der dafür geänderten Gruppenzusammensetzung, und die Einengung von Präfix und
`dependencies`-Zeile auf die enge Kennungsmenge. An einem Messlauf mit 74 Gruppen tragen
am Ende 12 ihre alte ID.

Was daraus folgt:

- **Known-Decisions-Kriterien, die auf `stableId` matchen, treffen nicht mehr dieselben
  Gruppen.** Sie sind einmal neu abzuleiten, aus einem mit dieser Fassung erzeugten
  `review-input.json`. Kriterien mit `pathGlob`, `ruleId` oder `fingerprint` sind nicht
  betroffen.
- **Alte und neue Artefakte desselben Laufs sind über `stableId` nicht vergleichbar.**
  Wer zwei Läufe gegeneinander hält, muss beide mit derselben Werkzeugfassung
  zusammengeführt haben.
- Ab dieser Fassung ist die Bildung gegen Änderungen an der Pfadnormierung der
  Gruppierung abgedichtet und gegen zusätzliche Kennungen außerhalb der engen Menge
  unempfindlich. Was weiterhin verschiebt — eine geänderte Gruppenzusammensetzung —,
  steht in `docs/review-runs.md`, dort auch die Begründung der Entscheidung.
