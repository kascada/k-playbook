---
title: review-input-contract
description: Belegvertrag der Review-Familie — das Schema von review-input.json, das allein der Go-Merge schreibt (im Audit-Lauf und auf das Arbeitsverzeichnis eines eigenständigen Reviews), die Bildung der stabilen Gruppen-IDs und die Herkunft der Belege; die einzige Stelle, an der das Schema beschrieben wird.
---

# review-input-contract

Dieses Modul ist kein Review-Katalog-Rezept. Es beschreibt `review-input.json` — den
Belegvertrag, auf dem die Triage arbeitet — und ist die einzige Stelle, an der dieses
Schema beschrieben wird. Commands, Rezepte und Doku verweisen darauf, statt es zu
wiederholen.

Gelesen wird es von der Triage (`k-playbook:/commands/_audit/review-scan-triage.md`), die
der Baustein Zusammenfassen (`k-playbook:/commands/_review-run/zusammenfassen.md`)
anwendet — in `/k-review` und, nach seinem Umbau, in `/k-audit`. Geschrieben wird die Datei
nie von einem Agenten, sondern immer vom Go-Merge. Das Modul liegt deshalb im
Familien-Namensraum `commands/_review-run/` und nicht im Modulverzeichnis eines einzelnen
Commands.

Die Wahrheit über das Schema ist der Merge-Code unter
`installer/internal/review/merge/` — `merge.go`, `write.go`, `dedupe.go`, `stable.go`.
Was hier steht, ist gegen diesen Code geprüft. Weicht der Code ab, gilt der Code, und
dieser Vertrag wird nachgezogen; nicht umgekehrt.

## Ein Weg: der Go-Merge

`review-input.json` entsteht immer im Go-Merge. Er dedupliziert, leitet die Schwere ab und
vergibt die stabilen Gruppen-IDs. Zwei Einstiege führen dorthin:

- **Merge auf ein Arbeitsverzeichnis** — `k_playbook_review_merge_dir` (CLI
  `k-playbook review merge <verzeichnis>`). Er liest jede Datei `<eintrag>.sarif`
  unmittelbar im Verzeichnis, prüft sie gegen den Vertrag des Rezepts mit diesem
  Schlüssel und schreibt die Datei daneben. Ohne Lauf: kein `run.json`, keine
  `entries/`, und bekannte Entscheidungen gleicht er nicht ab. Das ist der Weg von
  `/k-review`.
- **Merge eines Audit-Laufs** — `k_playbook_review_merge` liest `run.json`,
  `entries/*.json` und die SARIF-Dateien unter `raw/` und gleicht `known-decisions.md`
  ab. Er ist gesperrt, bis `/k-audit` umgebaut ist.

Einen von Hand geschriebenen Beleg gibt es nicht. Ein Feld, das der Merge nicht kennt,
steht nicht im Vertrag.

## Felder

Das Gerüst, das jeder Merge schreibt. Die weiteren Felder unten kommen dazu.

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
| `run.name` | Name des Laufs. Im Audit das Laufdatum, im Arbeitsverzeichnis dessen Name. |
| `run.dir` | Ordner der Datei, relativ zur Projektwurzel; ein Arbeitsverzeichnis außerhalb des Projekts steht absolut. |
| `run.selectedEntries[]` | Die Einträge, die zu diesem Beleg beigetragen haben, mit `name`, `kind` (`tool` oder `ai`), `state` und — bei `kind: ai` — `mode` (`evidence` oder `perspective`). `mode` beschreibt die Rolle **in diesem Beleg**: `evidence`, wenn der Eintrag die Funde erhoben hat, `perspective`, wenn er einen fertigen Beleg bewertet. Im Audit kommt der Wert aus `audit.mode` des Rezepts; fehlt er dort, normalisiert `review.NormalizeMode` ihn zu `perspective`. Im Arbeitsverzeichnis steht je SARIF-Datei ein Eintrag, `kind: ai` mit `mode: evidence`: Er hat die Funde selbst erhoben. |
| `findings[].id` | In der Datei eindeutig. Der Merge bildet ihn als `<sarif>:<runIndex>:<resultIndex>`. |
| `findings[].evidence.tool` | Bei Scanner-Belegen der Werkzeugname (`gosec`), bei Rezept-Belegen der Eintragsname (`tech`). Es gibt kein zweites Feld für die Quelle: `source` existiert nicht. |
| `findings[].evidence.job` | Der Scan-Job. Bei einem Rezept-Beleg heißt er wie der Eintrag. |
| `findings[].ruleId` | Die Rule-ID. Bei Evidence-Rezepten aus deren `audit.ruleIds`. |
| `findings[].level` | Der SARIF-Level. Er ist die einzige Wertung, die ein Rezept vergibt. |
| `findings[].message` | Die Meldung des Fundes. |
| `findings[].location` | `uri` als projektrelativer Pfad, `startLine` und `startColumn` optional. |
| `findings[].dependency` | Nur bei Dependency-Befunden: `package`, `version`, `manifest`, `ids`. Fehlt sonst. Drei weitere Felder kommen dazu. `keyIds` sind die Kennungen, die den Fund *benennen* (`ruleId` und benannte Alias-Felder) — nur sie gehen in den harten Dedupe-Schlüssel und in die stabile Gruppen-ID. Sie sind kein bloßes Schlüsseldetail, sondern die Antwort auf „worum geht dieser Fund?" gegenüber `ids`, das auch nennt, was das Advisory nur *erwähnt*: wer zwei Belege inhaltlich zusammenfasst, weil sie eine Kennung teilen, muss die beiden Mengen auseinanderhalten können. In **beiden** Mengen stehen dieselben Kennungsformen: `CVE-…`, `GHSA-…`, `OSV-…`, `PYSEC-…` und `GO-…` — letztere die Kennung der Go Vulnerability Database mit mindestens vierstelliger Nummer (`GO-2026-5024`), unter der grype seine Go-Funde ausschließlich führt. Was in keiner dieser Formen steht, taucht in keiner der beiden Mengen auf; ein Alias wie `SNYK-…`, das nur im Advisory-Text vorkommt, wird bewusst nicht gelesen. Der Kreis der Formen ist Teil des Vertrags, nicht Implementierungsdetail: eine Form mehr kann einen Fund erstmals zum Dependency-Befund machen und über `keyIds` Klasse und Präfix seiner Gruppe setzen. `package` und `version` tragen dort ausschließlich **strukturiert** gelesene Werte, also eine benannte Property oder einen purl. Was nur im Fließtext des Werkzeugs steht, steht in `textPackage` / `textVersion` und ist ausdrücklich nur für die Anzeige — es bildet keinen harten Schlüssel und rechtfertigt kein Zusammenlegen. Fehlen `package` und `version`, ist der Fund nicht schlechter belegt; sein Werkzeug nennt sie nur nicht als Feld (osv-scanner, trivy) — dann sind `textPackage` / `textVersion` die einzige Auskunft darüber, welches Paket gemeint ist, und ohne sie müsste jeder Leser die Meldung noch einmal selbst zerlegen. Genau deshalb stehen sie im Beleg. |
| `groups[].displayId` | Laufende Anzeige-ID `G001`, `G002`, … Sie ist nicht stabil und taugt nicht als Referenz über Läufe hinweg. |
| `groups[].stableId` | Die stabile Gruppen-ID. Sie ist die Referenz, die in `review-triage.md` und in `known-decisions.md` steht. |
| `groups[].title`, `ruleId`, `level`, `location` | Der Repräsentant der Gruppe. |
| `groups[].findingIds` | Die IDs der Findings dieser Gruppe. Ihre Anzahl ist die Zahl der Instanzen. |
| `groups[].evidence[]` | Mindestens ein Eintrag, gleiche Form wie `findings[].evidence`. |

Zwei Regeln über den Feldern:

- **Jedes Finding gehört zu genau einer Gruppe.** Im Merge ergibt sich das aus dem
  Union-Find über die harten Schlüssel: jeder Fund wird einem Wurzelknoten und damit
  einer Gruppe zugeordnet, auch wenn er allein steht. Ein Feld für gruppenlose Findings
  gibt es deshalb nicht.
- **Gruppen bilden, nicht Findings verwerfen.** Eine Gruppe fasst zusammen; sie löscht
  nichts. Alle Belege und alle Finding-IDs bleiben erhalten.

## Weitere Felder

Diese Felder schreibt der Merge ebenfalls. Einige bleiben auf dem Weg über ein
Arbeitsverzeichnis leer, weil es dort keinen Lauf gibt; die Spalte rechts sagt, was dann
gilt. Die Triage prüft ihr Vorhandensein und arbeitet ohne sie weiter, statt sie zu
erfinden.

| Feld | Inhalt | Fehlt es oder ist es leer, gilt |
|---|---|---|
| `schemaVersion` | Version des Merge-Schemas, derzeit `1`. | Kein Merge-Artefakt; keine Aussage über die Schemafassung. |
| `kPlaybookVersion` | Version des Werkzeugs, das den Merge gerechnet hat. | Keine Werkzeug-Provenienz; die Herkunft steht in `run` und `evidence`. |
| `run.created`, `run.state`, `run.derivedState`, `run.languages` | Zustand und Sprachen des Audit-Laufs aus `run.json`. | Kein Audit-Lauf. Im Arbeitsverzeichnis steht der Zustand auf `done`, `created` und `languages` bleiben leer. |
| `run.selectedEntries[].recipeKey`, `recipePath`, `recipeOrigin`, `title`, `resultRequired`, `defaultResult`, `scope` | Der beim Laufstart eingefrorene Rezept-Snapshot aus `run.json`. | Es gibt keinen Snapshot; im Arbeitsverzeichnis steht nur `recipeKey`. Der Vertrag des Rezepts wurde beim Merge frisch aus dem Katalog gelesen. |
| `entries[]` | Eintrags- und Job-Status aus `entries/*.json`, mit SARIF-Pfaden und Fundzahlen. | Im Arbeitsverzeichnis je SARIF-Datei ein Eintrag `done` mit einem Job; `reason` nennt verworfene Funde außerhalb von `audit.scope.paths`. |
| `findings[].derivedSeverity`, `severitySource` | Abgeleitete Schwere aus SARIF-Level, CVSS, Tool-Metadaten und `scripts/severity.tsv`. | `level` gilt unmittelbar und ist die einzige Wertung. Keine eigene Ableitung improvisieren. |
| `findings[].ruleName`, `ruleDescription` | Regeltexte aus dem SARIF-Regelkatalog. | Nur `ruleId` und `message` beschreiben den Fund. |
| `findings[].locations[]` | Alle Orte eines Fundes, nicht nur der erste. | Es gilt `location`. |
| `findings[].fingerprints`, `partialFingerprints` | Werkzeug-Fingerprints aus dem SARIF. | Kein Fingerprint-Abgleich möglich. |
| `evidence.sarif`, `evidence.runIndex`, `evidence.resultIndex` | Rückverweis in die SARIF-Rohdatei — im Audit unter `raw/`, im Arbeitsverzeichnis der Dateiname daneben. | Kein Rückverweis. `tool` und `job` bleiben. |
| `groups[].stableKey` | Der Klartext-Schlüssel, aus dem der Digest der stabilen ID gebildet wurde. | — (der Merge schreibt ihn immer) |
| `groups[].dedupeRules`, `possibleDuplicates` | Welche Regel zusammengefasst hat und welche Gruppen unsichere Dubletten sind. | Keine Dedupe-Regel hat gegriffen. Die Bündelung ist Sache der Triage. |
| `groups[].derivedSeverity`, `severitySource` | Wie beim Finding, für den Repräsentanten. | Wie beim Finding: `level` gilt unmittelbar. |
| `groups[].dependency` | Dependency-Angaben des Repräsentanten. `ids` und `keyIds` sind die Vereinigung über die Findings der Gruppe, die dieselbe Dependency beschreiben; `package`, `version`, `manifest` und die Freitextfelder bleiben die des Repräsentanten und lassen sich nicht vereinigen. | Steht bei den Findings, sofern die Quelle sie kennt. |
| `knownDecisions` | Geladene Quellen, Decisions mit `applied` / `notAppliedReason` / `expired` und Warnungen. | Im Arbeitsverzeichnis gibt es kein Matching: keine Quellen, keine Decisions, und `warnings` sagt in einem Satz, dass keine bekannten Entscheidungen berücksichtigt wurden. |
| `findings[].coveredByKnownDecision`, `groups[].coveredByKnownDecision`, `groups[].partialCoverage`, `groups[].knownDecisionCoverage` | Ergebnis des Matchings je Fund und je Gruppe, mit `matchedBy`. | Keine Deckungsmarker im Beleg. Die Ergebnisdatei sagt in einem Satz, dass keine bekannten Entscheidungen berücksichtigt wurden. |

## Stabile Gruppen-IDs

`groups[].stableId` ist die einzige Gruppen-Referenz, die über Läufe hinweg trägt.
`displayId` wechselt mit der Reihenfolge und wird nie als Referenz benutzt.

Gebildet wird sie in `merge/stable.go`, auf beiden Einstiegen gleich. Der Digest ist ohne
Merge-Code nicht reproduzierbar; deshalb bildet kein Agent eine ID selbst. Präfix aus der
Klasse der Gruppe, dahinter ein gekürzter SHA256-Digest über normalisierte Attribute der
Funde:

- `ai-<eintrag>-<digest>` — alle Funde der Gruppe stammen aus einem Evidence-Rezept.
  Der Schlüssel besteht nur aus Eintragsname, Rule-ID und normalisiertem Pfad; Zeile,
  Meldung und Fingerprints bleiben draußen, damit die ID über Läufe hinweg hält.
- `scan-cve-<kennung>-<digest>` — die Gruppe trägt eine Dependency-Kennung.
  `<kennung>` ist die alphabetisch erste aus der **engen** Kennungsmenge der Gruppe:
  aus den Kennungen, die den Fund selbst benennen (`ruleId` und benannte Alias-Felder,
  siehe `findings[].dependency.keyIds`), nicht aus denen, die im Advisory-Text nebenbei
  vorkommen. Nennt ein Werkzeug seine einzige Kennung nur im Freitext, fällt die Bildung
  auf die breite Menge `ids` zurück. Dieselbe enge Menge steht auch in der
  `dependencies`-Zeile des Schlüssels. Das Segment `cve` im Präfix ist ein fester
  Bestandteil des Namens, keine Aussage über die Form: `<kennung>` trägt jede
  Kennungsform, die `keyIds` kennt — eine Go-Gruppe heißt deshalb
  `scan-cve-go-2026-5024-<digest>`.
- `scan-<werkzeug>-<digest>` — alles Übrige; `<werkzeug>` ist `multi`, wenn mehrere
  Werkzeuge beteiligt sind.

**Pfade im Schlüssel sind eingefroren normiert.** Überall, wo ein Pfad in den Schlüssel
oder in die Klassenentscheidung eingeht — die Zeilen `locations`, `dependencies` und
`paths` —, läuft er über eine eigene, festgeschriebene Normierung und **nicht** über die
der Gruppierung: Backslashes zu `/`, `file://` samt Authority weg, `.`- und
`..`-Segmente sowie doppelte Slashes aufgelöst, führendes `/` weg, Kleinschreibung. Die
Trennung ist Absicht: die Dedupe-Normierung darf besser werden, ohne die IDs zu bewegen.
Ändert sich dagegen die eingefrorene Fassung, verschieben sich Stable-IDs — Begründung
und Folgen für `known-decisions.md` stehen in `k-playbook:/docs/handbook/review-runs.md`.

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

Im Arbeitsverzeichnis eines eigenständigen Reviews gibt es nur Rezept-Belege: Alle Gruppen
tragen dort das Präfix `ai-<eintrag>-`, und `run.selectedEntries` hat je SARIF-Datei einen
Eintrag.

## Was der Vertrag nicht kennt

Drei Felder standen in Prosa, ohne je geschrieben zu werden. Sie sind gestrichen; kein
Command und keine Doku führt sie weiter:

- **Top-Level `scope`** mit `scope.type` und `scope.family`. Das `Result`-Struct kennt
  es nicht. Es wäre auch redundant: `run.name` und `run.dir` tragen dieselbe Auskunft.
- **`ungroupedFindings`.** Der Merge kann sie nicht erzeugen — das Union-Find gibt jedem
  Fund eine Gruppe, das Feld wäre strukturell immer leer. Der Abschnitt
  `Nicht gebündelt` in `review-triage.md` speist sich deshalb aus den **Gruppen**, die die
  Triage keinem Bündel zugeordnet hat, nicht aus einem Feld des Belegs.
- **`groups[].id`.** Auf Gruppen gibt es `displayId` und `stableId`, sonst nichts.

Ebenso gibt es kein `evidence[].source`, `evidence[].file`, `evidence[].line` oder
`evidence[].message`. Die Quelle steht in `evidence.tool`, Ort und Meldung am Finding.

## Bestandsartefakte

Bestehende `review-input.json` älterer Audit-Läufe bleiben unverändert lesbar: Felder sind nur hinzugekommen, keines ist entfallen oder umbenannt
worden, und `schemaVersion` bleibt `1`.

**Die stabilen Gruppen-IDs sind dagegen verschoben.** Ein Lauf, der mit dem heutigen
Merge neu zusammengeführt wird, vergibt für dieselben Funde andere `stableId` als eine
ältere Fassung des Werkzeugs. Sechs Änderungen wirken zusammen: die erweiterte
Pfadnormierung, der Dedupe-Schlüssel je Kennung, das aus purl-Quellen nachgeholte Paket
samt der dafür geänderten Gruppenzusammensetzung, die Einengung von Präfix und
`dependencies`-Zeile auf die enge Kennungsmenge, die Zulassungsliste für Fingerprints bei
Dependency-Funden und die zusätzlich erkannte Kennungsform `GO-…`. Gemessen wurde in zwei
Schritten: die ersten vier an einem Messlauf mit 74 Gruppen, von denen am Ende 12 ihre
alte ID trugen; die beiden letzten an einem Messlauf mit 73 Gruppen, von denen 64 ihre
alte ID behielten. Beide Zahlen sind der Stand ihres jeweiligen Laufs, keine Vorhersage
für einen anderen.

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
  unempfindlich. Unempfindlich gegen Kennungen **außerhalb** der engen Menge bleibt sie;
  was **in** die enge Menge fällt, ist damit aber nicht festgeschrieben: eine zusätzlich
  erkannte Kennungsform — zuletzt `GO-…` — erweitert genau diese Menge und verschiebt die
  ID. Was ebenfalls weiterhin verschiebt — eine geänderte Gruppenzusammensetzung —,
  steht in `k-playbook:/docs/handbook/review-runs.md`, dort auch die Begründung der Entscheidung.
