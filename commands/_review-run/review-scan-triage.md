# review-scan-triage

Dieses Modul ist kein Review-Katalog-Rezept. Es wird ausschließlich von
`/k-review-run` eingebunden und beschreibt den AI-Eintrag `scan-triage` eines
Review-Laufs. Der Eintrag gehört nicht zu `catalogs.reviews`; er entsteht aus dem
effektiven Command-Namensraum `commands/_review-run/review-scan-triage.md`.

## Eingaben

Der aufrufende Command liefert den Laufstatus aus `k_playbook_review_status`.
Verwende daraus ausschließlich den gemeldeten Laufordner als `RUN_DIR`.

Lies aus `RUN_DIR`:

- `review-input.json` als vollständigen Audit-Beleg mit Provenienz, Belegen und
  stabilen Gruppen-IDs sowie `knownDecisions` und `coveredByKnownDecision`.
- `review-input.md` als kompakte Ansicht.

Suche keine `known-decisions.md` und führe kein eigenes Matching aus. Die Deckung ist
bereits im Merge-Artefakt entschieden: Nutze ausschließlich `review-input.json`, dort
`groups[].coveredByKnownDecision`, `groups[].partialCoverage`,
`groups[].knownDecisionCoverage` und den Metablock `knownDecisions`.

## Schreibweg

Schreibe genau eine Datei direkt:

- `RUN_DIR/review-triage.md`

`RUN_DIR` ist der Laufordner aus dem MCP-Status des aktuellen Laufs. Schreibe nicht
nach `raw/`, nicht nach `run.json`, nicht nach `entries/*.json` und nicht nach
`review-input.*`.

Nach erfolgreichem Schreiben ruft `/k-review-run`
`k_playbook_review_write_ai_entry` auf und setzt den Eintrag `scan-triage` auf
`done` mit `result: review-triage.md`. Dieses MCP-Werkzeug schreibt nur den
Eintragszustand, nicht den Markdown-Inhalt.

## Bewertung

Verdichte die Gruppen aus `review-input.json` zu Bewertungs-Bündeln.

- Ziehe Gruppen nach gemeinsamer Root-Cause zusammen, zum Beispiel ein einzelnes
  Dependency-Upgrade, ein altes Code-Verzeichnis oder ein wiederkehrendes
  Pfadvalidierungsproblem.
- Vergib pro Bündel eine Priorität `P1`, `P2` oder `P3`.
- Vergib pro Bündel eine Kategorie `S`, `T`, `K`, `F`, `A` oder `X`, wie in
  `/k-remediation` verwendet.
- Begründe kurz, warum die Gruppen zusammengehören und warum die Priorität passt.
- Verlinke jede betroffene stabile Gruppen-ID aus `review-input.json`.
- Markiere Gruppen mit `coveredByKnownDecision` ausdrücklich als gedeckt; entferne sie
  nicht stillschweigend und lege sie nicht in offene Bündel.
- Gruppen mit `partialCoverage: true` bleiben offen sichtbar; nenne die Teildeckung aus
  `knownDecisionCoverage`.
- Liste Gruppen ohne sinnvolle Bündel-Zuordnung im Abschnitt `Nicht gebündelt`.

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

## Ausgabeformat

`review-triage.md` hat diese Abschnitte in dieser Reihenfolge:

```markdown
# Review-Triage <lauf>

Erzeugt: <RFC3339-Zeitstempel>
Lauf: `<RUN_DIR_DISPLAY>`
Quelle: `review-input.json`
Known-Decisions: <Kurzstatus aus `knownDecisions`, keine eigene Suche>

## Bündel

| ID | Priorität | Kategorie | Kurzbegründung | Gruppen |
|---|---|---|---|---|

## Bündel-Details

### <Bündel-ID> — <Titel>

Begründung: ...

Betroffene Belege: ...

Nächster Schritt: ...

## Nicht gebündelt

| Priorität | Kategorie | Kurzbegründung | Gruppen |
|---|---|---|---|

## Deckung aus known-decisions

| Decision-ID | Kategorie | Vollständig gedeckte Gruppen | Teilgedeckte Gruppen | Ablaufdatum | Applied | Hinweis |
|---|---|---:|---:|---|---|---|
```

Stabile Gruppen-IDs werden als Links in den Laufbeleg geschrieben, zum Beispiel
`` `scan-gosec-b94401` `` oder, wenn ein Anker vorhanden ist,
`[scan-gosec-b94401](review-input.md#...)`.

## Abschluss

Melde nach dem Schreiben:

- den Pfad `RUN_DIR/review-triage.md`,
- die Anzahl der Bündel,
- Restgruppen im Abschnitt `Nicht gebündelt`,
- ob `known-decisions.md` verwendet wurde,
- wie viele Gruppen durch `knownDecisions` aus `review-input.json` vollständig oder
  teilweise gedeckt sind,
- dass `k_playbook_review_write_ai_entry` als nächster Schritt den Eintrag
  `scan-triage` auf `done` mit `result: review-triage.md` setzen muss.
