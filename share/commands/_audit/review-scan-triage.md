# review-scan-triage

Dieses Modul ist kein Review-Katalog-Rezept. Es wird von `/k-audit` und vom
`/k-review`-Report-Modus eingebunden und beschreibt die einheitliche Triage auf Basis
von `review-input.json`. Der Audit-Eintrag `scan-triage` gehört nicht zu
`catalogs.reviews`; er entsteht aus dem effektiven Command-Namensraum
`commands/_audit/review-scan-triage.md`.

Der Ort `_audit/` ist Herkunft, nicht Zuständigkeit: an ihm hängen die Konstante
`scanTriageModule`, der Eintragsname `scan-triage` und bestehende projektlokale Overlays.
Das Modul ist nicht audit-exklusiv — beide Wege wenden denselben Wortlaut an.

## Eingaben

Der aufrufende Command liefert entweder den Laufstatus aus `k_playbook_review_status`
(`/k-audit`) oder das Ergebnisverzeichnis des Report-Reviews (`/k-review`). Verwende
daraus ausschließlich den gemeldeten Ordner als `RUN_DIR`.

Lies aus `RUN_DIR`:

- `review-input.json` als vollständigen Beleg. Was darin steht, beschreibt
  `commands/_review-run/review-input-contract.md`. Wende dieses Modul an;
  Feldnamen und die Bildung der stabilen Gruppen-IDs werden hier nicht wiederholt.
- `review-input.md` als kompakte Ansicht, falls vorhanden. Im `/k-review`-Report-Modus
  darf die Markdown-Ansicht fehlen; dann verlinke Gruppen-IDs nur als Code-Spans.

Der Vertrag ist zweigeteilt. Auf den Kern ist Verlass; die merge-only-Felder fehlen im
Report-Weg, und der Vertrag sagt je Feld, was dann gilt. Prüfe ihr Vorhandensein, statt
sie vorauszusetzen, und erfinde keines nach.

Woran Scanner- und Rezept-Belege zu unterscheiden sind, steht im Vertrag unter
`Herkunft der Belege`. Was die Unterscheidung für die Bewertung bedeutet, steht unten in
`Bewertung`.

Suche keine `known-decisions.md` und führe kein eigenes Matching aus. Wo die Deckung
bereits entschieden ist, steht sie im Beleg; wo sie fehlt, hat der aufrufende Command
sie vor der Triage berücksichtigt.

## Schreibweg

Schreibe genau eine Datei direkt:

- `RUN_DIR/review-triage.md`

`RUN_DIR` ist der Laufordner aus dem MCP-Status des aktuellen Audit-Laufs oder das
Family-Date-Verzeichnis eines Report-Reviews. Schreibe nicht nach `raw/`, nicht nach
`run.json`, nicht nach `entries/*.json` und nicht nach `review-input.*`.

Nach erfolgreichem Schreiben ruft `/k-audit`
`k_playbook_review_write_ai_entry` auf und setzt den Eintrag `scan-triage` auf
`done` mit `result: review-triage.md`. Dieses MCP-Werkzeug schreibt nur den
Eintragszustand, nicht den Markdown-Inhalt. Im `/k-review`-Report-Modus gibt es keinen
Audit-Entry; dort entfällt dieser MCP-Schritt.

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
- Trägt der Beleg keine Deckungsmarker, gibt es nichts zu markieren. Das ist kein
  Hinweis darauf, dass nichts gedeckt wäre — der Vertrag sagt, was dann gilt.
- Liste Gruppen, die du keinem Bündel zugeordnet hast, im Abschnitt `Nicht gebündelt`.
  Eine andere Quelle hat der Abschnitt nicht: im Beleg gehört jedes Finding zu genau
  einer Gruppe, gruppenlose Findings gibt es nicht.

KI-Evidence gewichten:

- KI-Evidence ist eine heuristische Quelle. Derselbe Auftrag findet je Lauf eine andere
  Menge; Funde verschwinden und kommen wieder.
- Belege einen KI-Fund am Code, bevor du ihn priorisierst. Ohne diese Sichtung gehört er
  nach `P3` oder in `Nicht gebündelt` und nicht in ein `P1`-Bündel.
- Die Menge hebt die Priorität nicht: fünfzig Funde eines Rezepts sind fünfzig Funde
  einer Heuristik und kein `P1`. Umgekehrt senkt eine kleine Menge sie auch nicht.
- Die Spalte `Befunde` in `review-input.md` — in `review-input.json` die Länge von
  `findingIds` — trägt die Zahl der Instanzen. KI-Gruppen werden je Rule-ID und Datei
  gebündelt, ohne Zeile und ohne Meldung; der Repräsentant nennt deshalb nur **eine**
  Stelle. Lies die Anzahl mit und nenne sie: eine Datei mit zwölf Instanzen ist eine
  andere Arbeit als eine mit einer.
- Eine Decision auf eine solche Gruppe deckt die Datei für diese Rule-ID ganz. Eine
  Abweisung je Instanz gibt es nicht; wer eine einzelne Instanz stehen lassen will, muss
  die Gruppe offen lassen.
- `severity` aus KI-Evidence kommt aus dem `level` im SARIF, das das Rezept je Rule-ID
  festlegt: `error` und `note` gelten unverändert (`severitySource: native`), `warning`
  und `none` laufen weiter über CVSS, Tool-Metadaten und das Severity-Mapping. Ein
  `warning` ist damit die schwächste Aussage des Rezepts und kein Urteil. Fehlen
  `derivedSeverity` und `severitySource`, gilt `level` unmittelbar — dieselbe Lesart
  ohne den abgeleiteten Zwischenschritt.

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

## Deckung aus known-decisions

Trägt der Beleg das Matching, ist die Deckung bereits entschieden und steht vollständig
in `review-input.json`. Wie sie zu lesen ist:

- Eine Decision mit `applied: false` und `notAppliedReason: "kein Finding getroffen"` hat
  in diesem Lauf nichts gedeckt. Der Grund sagt nicht, warum: er steht wortgleich an
  einer Decision, deren Fund verschwunden ist, wie an einer, die noch nie etwas getroffen
  hat. Aus dem Feld allein sind beide Fälle nicht zu unterscheiden.
- Ein nicht wiedergefundener Fund ist **kein Erledigt-Signal**. Bei KI-Evidence ist er
  der Regelfall und nicht die Ausnahme, weil die Quelle heuristisch ist. Trage die
  Decision mit `Applied: nein` und ihrem Grund ein und behaupte nicht, der Befund sei
  behoben.
- Wer wissen will, ob ein Fund verschwunden ist oder nie existierte, vergleicht die
  `review-input.json` des Vorlaufs. Das ist eine Rückfrage an den Nutzer und keine
  Aufgabe dieses Moduls.
- Eine abgelaufene Decision trägt `notAppliedReason: "abgelaufen"`. Sie deckt nichts, und
  ihre Gruppen bleiben offen.

Welches Kriterium getroffen hat, steht je Finding in `coveredByKnownDecision.matchedBy`:

- `stableId` deckt genau eine Gruppe. Für KI-Evidence ist das der vorgesehene Weg: die
  Gruppen-ID entsteht nur aus Eintragsname, Rule-ID und Pfad und bleibt deshalb über Läufe
  hinweg stabil.
- `ruleId+location` deckt eine Regel an einem Pfad. `location` ist ein Pfad-Glob und
  keine Zeile — der Weg deckt also Regel plus Datei, genau die Körnung einer KI-Gruppe,
  und ist damit der brauchbare zweite Weg neben `stableId`.
- `pathGlob` deckt **jeden** Fund an diesem Pfad, auch Scanner-Funde fremder Werkzeuge
  und fremder Regeln. Wo eine Gruppe so gedeckt ist, nenne diese Nebenwirkung im Hinweis:
  eine per `pathGlob` gedeckte Datei ist für alle Werkzeuge gedeckt, nicht nur für das
  gemeinte.

Trägt der Beleg kein Matching, bleibt der Abschnitt trotzdem stehen. Er sagt dann in
einem Satz, dass der Beleg keine Deckungsmarker trägt und wer die bekannten
Entscheidungen stattdessen berücksichtigt hat. Führe kein eigenes Matching nach.

## Ausgabeformat

`review-triage.md` hat diese Abschnitte in dieser Reihenfolge:

```markdown
# Review-Triage <lauf-oder-family-date>

Erzeugt: <RFC3339-Zeitstempel>
Scope: `<run.name>` in `<run.dir>`
Ordner: `<RUN_DIR_DISPLAY>`
Quelle: `review-input.json`
Known-Decisions: <Kurzstatus aus dem Beleg, keine eigene Suche>

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

Die `Scope:`-Zeile kommt aus `run.name` und `run.dir` des Belegs; ein eigenes
Scope-Feld gibt es nicht. Fehlt `RUN_DIR_DISPLAY`, nimm den projektrelativen Pfad des
Ordners.

Stabile Gruppen-IDs werden als Links in den Laufbeleg geschrieben, zum Beispiel
`` `scan-gosec-b94401` ``, `` `ai-tech-4f2a91` `` oder
`` `ai-dependabot-alerts-81` ``, wenn ein Anker vorhanden ist,
`[scan-gosec-b94401](review-input.md#...)`. Übernimm die IDs wörtlich aus
`review-input.json` und bilde keine eigenen.

## Abschluss

Melde nach dem Schreiben:

- den Pfad `RUN_DIR/review-triage.md`,
- die Anzahl der Bündel,
- Restgruppen im Abschnitt `Nicht gebündelt`,
- wie viele Bündel auf KI-Evidence beruhen und ob sie am Code belegt wurden,
- den Known-Decisions-Status aus `review-input.json`, oder dass der Beleg keinen trägt,
- wie viele Gruppen durch `knownDecisions` aus `review-input.json` vollständig oder
  teilweise gedeckt sind,
- bei `/k-audit`: dass `k_playbook_review_write_ai_entry` als nächster Schritt den
  Eintrag `scan-triage` auf `done` mit `result: review-triage.md` setzen muss.
- bei `/k-review`: den Handoff `/k-remediation RUN_DIR/review-triage.md`.
