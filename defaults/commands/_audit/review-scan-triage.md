---
title: review-scan-triage
description: Triage-Modul der Review-Familie — verdichtet die Gruppen aus review-input.json zu Bündeln mit Priorität P1–P3 und Kategorie S/T/K/F/A/X und schreibt die Ergebnisdatei review-triage.md als Abhakliste, eine Tabelle je Bündel nach Priorität mit Spalte „erledigt“, darunter jeder Fund im Detail; Teil des Bausteins Zusammenfassen für /k-review und später /k-audit.
---

# review-scan-triage

Dieses Modul ist kein Review-Katalog-Rezept. Es ist der zweite Teil des Bausteins
Zusammenfassen (`k-playbook:/commands/_review-run/zusammenfassen.md`): Aus dem Beleg
`review-input.json` entsteht die Ergebnisdatei `review-triage.md`. Ein eigenständiges
Review (`/k-review`) wendet es über diesen Baustein an; `/k-audit` soll es nach seinem Umbau
über denselben Baustein anwenden. Beide liefern damit dieselbe Ergebnisform.

Der Ort `_audit/` ist Herkunft, nicht Zuständigkeit: an ihm hängen die Konstante
`scanTriageModule`, der Eintragsname `scan-triage` im Audit-Lauf und projekteigene
Fassungen unter `.k-playbook/commands/_audit/`.

## Eingaben

Der aufrufende Baustein nennt:

- `RUN_DIR` — den Ordner mit `review-input.json`: das Arbeitsverzeichnis des Reviews, im
  Audit den Laufordner. Verwende ausschließlich diesen Ordner.
- den Kopf der Ergebnisdatei: Rezept (im Audit die Einträge des Laufs), Scope in Worten
  und Datum. Wer nicht `/k-audit` ist, ruft auf dem Weg von `/k-review`; der Kopf hängt vom
  Weg ab (Abschnitt „Ausgabeformat“).

Lies aus `RUN_DIR`:

- `review-input.json` als vollständigen Beleg. Was darin steht, beschreibt
  `k-playbook:/commands/_review-run/review-input-contract.md`; Feldnamen und die Bildung
  der stabilen Gruppen-IDs werden hier nicht wiederholt.
- `review-input.md` als kompakte Ansicht, falls vorhanden.

Woran Scanner- und Rezept-Belege zu unterscheiden sind, steht im Vertrag unter
`Herkunft der Belege`. Was die Unterscheidung für die Bewertung bedeutet, steht unten in
`Bewertung`.

Suche keine `known-decisions.md` und führe kein eigenes Matching aus. Wo die Deckung
entschieden ist, steht sie im Beleg; wo sie fehlt, sagt der Kopf der Ergebnisdatei das in
einem Satz (Abschnitt „Bekannte Entscheidungen“).

## Schreibweg

Schreibe genau eine Datei: `RUN_DIR/review-triage.md`. Schreibe nicht nach `raw/`, nicht
nach `run.json`, nicht nach `entries/*.json` und nicht nach `review-input.*`; ändere keine
SARIF-Datei.

Im Audit-Lauf meldet `/k-audit` danach den Eintrag `scan-triage` mit
`k_playbook_review_write_ai_entry` als `done` mit `result: review-triage.md`. Ein
eigenständiges Review hat keinen Eintrag; dort entfällt dieser Schritt.

## Bewertung

Verdichte die Gruppen aus `review-input.json` zu Bündeln. Ein Bündel ist eine Ursache und
eine Arbeit.

- Ziehe Gruppen nach gemeinsamer Root-Cause zusammen, zum Beispiel ein einzelnes
  Dependency-Upgrade, ein altes Code-Verzeichnis oder ein wiederkehrendes
  Pfadvalidierungsproblem.
- Vergib pro Bündel eine Priorität `P1`, `P2` oder `P3`.
- Vergib pro Bündel eine Kategorie `S`, `T`, `K`, `F`, `A` oder `X`.
- Begründe kurz, warum die Gruppen zusammengehören und warum die Priorität passt.
- Nenne jede stabile Gruppen-ID des Bündels wörtlich aus `review-input.json`.
- **Kein Fund fällt still heraus.** Jede Gruppe steht in genau einem Bündel oder im
  Restabschnitt `Nicht gebündelt`, und mit ihr jeder ihrer Funde. Im Beleg gehört jedes
  Finding zu genau einer Gruppe; gruppenlose Findings gibt es nicht.
- Trägt eine Gruppe `coveredByKnownDecision`, gehört sie in kein offenes Bündel, sondern in
  den Restabschnitt, mit der Decision-ID als Grund. Eine Gruppe mit `partialCoverage: true`
  bleibt offen; nenne die Teildeckung aus `knownDecisionCoverage`. Solche Marker trägt
  heute nur der Merge eines Audit-Laufs.

KI-Evidence gewichten:

- KI-Evidence ist eine heuristische Quelle. Derselbe Auftrag findet je Lauf eine andere
  Menge; Funde verschwinden und kommen wieder.
- Belege einen KI-Fund am Code, bevor du ihn priorisierst. Ohne diese Sichtung gehört er
  nach `P3` oder in `Nicht gebündelt` und nicht in ein `P1`-Bündel.
- Die Menge hebt die Priorität nicht: fünfzig Funde eines Rezepts sind fünfzig Funde
  einer Heuristik und kein `P1`. Umgekehrt senkt eine kleine Menge sie auch nicht.
- KI-Gruppen werden je Rule-ID und Datei gebündelt, ohne Zeile und ohne Meldung; der
  Repräsentant der Gruppe nennt deshalb nur **eine** Stelle, die Länge von `findingIds`
  die Zahl der Instanzen. Nenne alle Instanzen im Detailteil: eine Datei mit zwölf
  Instanzen ist eine andere Arbeit als eine mit einer.
- `severity` aus KI-Evidence kommt aus dem `level` im SARIF, das das Rezept je Rule-ID
  festlegt: `error` und `note` gelten unverändert (`severitySource: native`), `warning`
  und `none` laufen weiter über CVSS, Tool-Metadaten und das Severity-Mapping. Ein
  `warning` ist damit die schwächste Aussage des Rezepts und kein Urteil. Fehlen
  `derivedSeverity` und `severitySource`, gilt `level` unmittelbar.

Priorisierung:

- `P1`: Sicherheits- oder Betriebsrisiko mit naheliegendem Ausnutzungspfad,
  exponiertem Codepfad oder eindeutig blockierendem Befund.
- `P2`: echter Befund oder starkes Risiko, aber mit begrenzter Exposition,
  größerem Kontextbedarf oder gebündeltem technischen Fix.
- `P3`: Rauschen, Legacy-/Archivcode, niedrige Exposition, kosmetische oder
  vorbereitende Arbeit.

Kategorien:

- `S`: Security-relevante Code- oder Konfigurationsänderung.
- `T`: Tests oder Verifikation.
- `K`: Klärung oder Kontextentscheidung.
- `F`: fachliche Produktentscheidung.
- `A`: Akzeptanz oder bewusste Ausnahme.
- `X`: Ausschluss, Altlast oder nicht umzusetzende Restmenge.

## Bekannte Entscheidungen

Der Merge eines eigenständigen Reviews gleicht keine bekannten Entscheidungen ab; sein Beleg
nennt das unter `knownDecisions.warnings`. Dann sagt die Kopfzeile `Bekannte Entscheidungen:`
in einem Satz, dass keine berücksichtigt wurden, und dieses Modul führt kein eigenes Matching
nach.

## Ausgabeformat

`review-triage.md` ist selbsttragend: Wer sie liest, braucht `review-input.json` nicht. Sie hat
diese Teile in dieser Reihenfolge:

```markdown
---
title: "Review-Ergebnis <rezept> <datum>: <Scope in Worten>"
---

# Review-Ergebnis <rezept> <datum>

Rezept: `<rezept>` (im Audit: die Einträge des Laufs)
Scope: <Scope in Worten>
Erzeugt: <RFC3339-Zeitstempel>
Arbeitsverzeichnis: `<RUN_DIR>` — vergänglich, enthält nur Fundliste und Beleg (im Audit: nur `<RUN_DIR>`, ohne Zusatz)
Beleg: `review-input.json` — <n> Funde in <m> Gruppen
Bekannte Entscheidungen: <ein Satz>

## Bündel

| erledigt | ID | Priorität | Kategorie | Titel | Funde |
|---|---|---|---|---|---:|
| [ ] | B1 | P1 | S | <Titel> | 3 |
| [ ] | B2 | P2 | T | <Titel> | 1 |
| [ ] | R | – | – | Nicht gebündelt | 2 |

## Details

### B1 — <Titel>

Priorität: P1 · Kategorie: S · Gruppen: `ai-code-4f2a91`, `ai-code-81c3d0`

Begründung: …

Nächster Schritt: …

| Ort | Rule-ID | Level | Meldung |
|---|---|---|---|
| `installer/x.go:12` | `code-logic-error` | error | … |

### R — Nicht gebündelt

| Gruppe | Grund | Ort | Rule-ID | Level | Meldung |
|---|---|---|---|---|---|
```

Regeln dazu:

- **Kopf je Weg.** Auf dem Weg von `/k-review` beginnt die Datei mit dem Frontmatter oben:
  `title` in doppelten Anführungszeichen, Rezept, Datum und Scope in einem Satz, ein `"` im
  Wert wird `\"`. `/k-review` legt die Datei so in Kephalaion ab, und der Text dort überlebt
  das Arbeitsverzeichnis; deshalb sagt die Kopfzeile `Arbeitsverzeichnis:`, dass es
  vergänglich ist und nur Fundliste und Beleg enthält (im Audit: ohne Frontmatter, die Datei
  beginnt mit der Überschrift, und die Kopfzeile `Arbeitsverzeichnis:` nennt nur den
  Laufordner wie bisher).
- **Eine Zeile je Bündel**, sortiert nach Priorität `P1` → `P2` → `P3`, innerhalb einer
  Priorität nach Zahl der Funde absteigend. Die IDs `B1`, `B2`, … vergibst du in dieser
  Reihenfolge. Die Spalte `erledigt` steht für jedes Bündel auf `[ ]`; abgehakt wird sie
  später von Hand (`[x]`), auf dem Weg von `/k-review` im abgelegten Dokument in Kephalaion.
- **Der Restabschnitt** `R — Nicht gebündelt` hat eine eigene Zeile am Ende der Tabelle,
  sobald er mindestens eine Gruppe trägt; ohne solche Gruppen entfallen Zeile und
  Abschnitt. Je Fund eine Zeile mit Gruppen-ID und dem Grund, warum die Gruppe in keinem
  Bündel steht (etwa „am Code nicht belegt“, „gedeckt durch `<decision-id>`“).
- **Jeder Fund im Detail:** je Bündel alle Funde aller seiner Gruppen, je Fund eine Zeile
  mit Ort (`pfad:zeile` aus `location`, ohne Zeile nur `pfad`), Rule-ID, Level und
  Meldung. Kürze keine Meldung auf etwas anderes, als sie sagt; ein `|` darin wird `\|`.
- Die Spalte `Funde` ist die Zahl dieser Zeilen. Die Summe über alle Zeilen der Tabelle ist
  die Zahl der Funde im Beleg; stimmt sie nicht, fehlt ein Fund.
- Übernimm die Gruppen-IDs wörtlich aus `review-input.json` und bilde keine eigenen.
- Ein Beleg ohne Funde ergibt die Kopfzeilen, eine Tabelle ohne Zeilen und den Satz „Keine
  Funde.“ unter `## Details`.

## Abschluss

Melde nach dem Schreiben:

- den Pfad `RUN_DIR/review-triage.md`,
- die Zahl der Bündel je Priorität und die Zahl der Funde im Restabschnitt,
- wie viele Bündel auf KI-Evidence beruhen und ob sie am Code belegt wurden,
- den Satz zu den bekannten Entscheidungen aus dem Kopf,
- bei `/k-audit`: dass `k_playbook_review_write_ai_entry` als nächster Schritt den
  Eintrag `scan-triage` auf `done` mit `result: review-triage.md` setzen muss.
