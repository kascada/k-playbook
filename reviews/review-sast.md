---
name: review-sast
title: SAST Assessment
interval-weeks: 4
scope-hint: SAST-Evidence aus semgrep, gosec, ruff und njsscan; keine Code-Änderungen aus diesem Review heraus
handoff: /k-remediation
result-family: sast
audit:
  enabled: true
  title: SAST Assessment
  resultRequired: true
  defaultResult: review-sast.md
  scope:
    tools: [semgrep, gosec, ruff, njsscan]
review:
  enabled: true
---

# Review: SAST Assessment

Bewerte SAST-Belege als fokussierte Perspektive auf `review-input.json`. Dieses Rezept führt
keine eigenen Scanner aus und schreibt genau eine Ergebnisdatei im aktuellen Lauf- oder
Family-Ordner.

## Zweck

- Muster im eigenen Quellcode bewerten, die ein Security-Risiko anzeigen.
- Tool-Severity von projektspezifischer Review-Priorität trennen.
- Erreichbarkeit, Vertrauensgrenze und Schutzmaßnahme für die Triage erfassen.
- Stabile Gruppen-IDs aus `review-input.json` erhalten.
- Keine Code-Änderungen durch dieses Review.

## Eingaben

Lies `review-input.json` aus dem vom aufrufenden Command genannten Ordner. Im
Audit-Laufmodell gilt der im `run.json`-Eintrag gespeicherte Scope:

```yaml
scope:
  tools: [semgrep, gosec, ruff, njsscan]
```

Filtere auf Evidence-Ebene:

- Eine Gruppe gehört zur Perspektive, wenn mindestens eine Evidence dieser Gruppe ein
  `evidence.tool` aus `scope.tools` trägt.
- Die Gruppen-ID bleibt unverändert; nicht neu deduplizieren, splitten oder umnummerieren.
- Bewerte nur scoped Evidence als primären SAST-Befund.
- Evidence anderer Tools bleibt als Kontext sichtbar und wird eindeutig als „außerhalb des
  Scopes" markiert.
- Leere Scope-Ergebnisse sind gültig; schreibe dann einen Report mit Status „keine scoped
  Findings".

## Bewertungskriterien

Tool-Severity allein ist keine Aussage über das Risiko. Die Perspektive bewertet
Erreichbarkeit und Kontext und liefert diese Einordnung an die generische Triage; die
Priorität `P1`–`P3` und die Kategorie vergibt allein die Triage.

Relevanz erhöhend:

- Fremdeingaben erreichen die Stelle über Parsing, Authentifizierung, Netzwerk oder
  Deserialisierung.
- Subprozessaufrufe mit variablen Argumenten, variabler Shell-Zeile oder variablem
  Programmpfad.

Relevanz senkend oder zu kennzeichnen:

- Produktionscode ohne erkennbaren Fremdeingabepfad ist weniger dringlich.
- Muster in Werkzeugen, Skripten, Testhilfen oder Migrationen sind als solche zu
  kennzeichnen.
- Ausdrücklich beabsichtigte Muster sind als beabsichtigt zu kennzeichnen, mit dem Beleg,
  woraus die Absicht hervorgeht.

Berücksichtige außerdem bekannte Entscheidungen aus `known-decisions.md` im Merge-Beleg.

**Mindestangaben je Bewertung.** Jede Bewertung, auch ein wahrscheinlicher Falschalarm,
enthält mindestens:

- Werkzeug und Rule-ID.
- Fundort, also Datei und Stelle.
- Den relevanten Aufruf oder Daten- beziehungsweise Argumentfluss.
- Die maßgebliche Vertrauensgrenze.
- Den Nachweis einer vorhandenen oder fehlenden Schutzmaßnahme.

Als Schutzmaßnahme gilt nur ein konkret benannter Nachweis: eine Validierung, eine sichere
API, eine feste Argumentliste, eine Framework-Garantie oder eine wirksame Konfiguration.
„Nicht erreichbar" oder „beabsichtigt" ohne diese Angaben ist `context-needed` und nicht
`likely-false-positive`.

**Werkzeugtypische Falschalarmkontexte.** Ein Falschalarmkontext ordnet die auslösende
Rule-ID einem konkreten Code- oder Laufzeitkontext zu und nennt dazu den relevanten Aufruf
oder Argumentfluss, eine vorhandene Validierung oder sichere API sowie die maßgebliche
Vertrauensgrenze oder Framework-/Konfigurationseinstellung. Welcher Kontext greift, hängt
am im Lauf vorliegenden Scanner und an seiner Regel; ein statischer Regelkatalog wird
hier nicht geführt.

- `semgrep` — die Regelsätze treffen breit über Sprachen hinweg und melden auch dort, wo
  ein Framework die Eingabe bereits bindet oder maskiert. Belege deshalb je Fund, welche
  Regel ausgelöst hat und ob die Eingabe die genannte Vertrauensgrenze überhaupt
  überschreitet.
- `gosec` — die Regeln über Dateipfade, Subprozesse und Rechte sehen den Aufrufer nicht.
  Belege, woher das Argument stammt und ob es eine feste Liste, eine Konstante oder eine
  geprüfte Eingabe ist.
- `ruff` — die `S`-Regeln aus flake8-bandit sind syntaktisch und bewerten weder Aufrufer
  noch Umgebung. Belege den tatsächlichen Aufruf samt Argumentquelle, nicht nur das
  getroffene Muster.
- `njsscan` — die Muster treffen häufig Bundles, Beispielcode und Framework-Konventionen.
  Belege, ob die Stelle zum eigenen Quellcode gehört und ob die Konfiguration des
  Frameworks die gemeldete Einstellung überhaupt wirksam werden lässt.

Drei Fälle gelten unabhängig vom Werkzeug:

- Ein Subprozessaufruf mit festen Argumenten ist kein Befund, auch wenn die Regel
  anschlägt.
- Ein Pfad aus einer Konfigurationsdatei ist keine Fremdeingabe im Sinne der Regel,
  solange die Datei dem Projekt gehört.
- Muster in generiertem oder gebündeltem Code sind kein Befund am Projekt.

Review-Status im Perspektiven-Report:

- `open` - neu oder noch nicht geprüft.
- `confirmed` - erreichbares Muster mit fehlender oder unwirksamer Schutzmaßnahme.
- `context-needed` - Fluss, Vertrauensgrenze oder Schutzmaßnahme nicht belegt.
- `likely-false-positive` - Regelkontext trifft belegbar nicht zu.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Findings inhaltlich zusammen bewerten, wenn Werkzeug, Rule-ID, Datei und Muster gleich
sind. Die Dedupe-Entscheidung aus `review-input.json` bleibt maßgeblich.

## Was ausdrücklich nicht als Finding zählt

Diese Perspektive bewertet Stellen im eigenen Quellcode. Ein Befund über eine Version oder
ein Paket ist kein SAST-Finding, auch wenn ein SAST-Werkzeug ihn meldet.

- Versions- und Paketbefunde gehören in `review-dependency-cve`.
- Secrets gehören in `review-secret-scanning`.
- IaC-, Container- und Imagekonfiguration gehört in `review-iac-container`.
- Reine Qualitäts- und Wartbarkeitsbefunde, etwa aus `golangci-lint`, gehören in
  `review-tech`.

Solche Gruppen werden mit unveränderter Gruppen-ID, Werkzeug und Rule-ID als „außerhalb des
Scopes" markiert und ohne SAST-Bewertung an die jeweilige Perspektive übergeben. Ist keine
Grenze eindeutig, bleibt die Gruppe mit der Begründung „Zuständigkeit klären" für die
generische Triage sichtbar.

## Perspektiven-Report-Format

Schreibe die im `run.json`-Eintrag genannte Datei, standardmäßig `review-sast.md`, direkt
in den aktuellen Ordner:

```markdown
# SAST Assessment - <lauf-oder-family-date>

Erzeugt: <RFC3339-Zeitstempel>
Quelle: `review-input.json`
Scope-Tools: `semgrep`, `gosec`, `ruff`, `njsscan`
Status: <bewertet | keine scoped Findings | technisch nicht bewertbar>

## Kurzfazit

- Scoped Gruppen: <n>
- Scoped Findings: <n>
- Bestätigt / Kontext nötig / wahrscheinlich Falschalarm: <counts>
- Wichtigster Codepfad: <kurz oder keiner>

## Bewertete SAST-Gruppen

| Gruppen-ID | Status | Werkzeug | Rule-ID | Ort | Bewertung | Belege | Nächster Schritt |
|---|---|---|---|---|---|---|---|

## Sofortige Triage-Reihenfolge

1. ...

## Evidence außerhalb des Scopes

| Gruppen-ID | Werkzeug | Rule-ID | Zuständige Perspektive | Grund |
|---|---|---|---|---|

## Deckung aus known-decisions

| Decision-ID | Betroffene Gruppen | Wirkung |
|---|---|---|

## Handoff

`/k-remediation <aktueller-ordner>/review-triage.md`
```

Die Spalte `Belege` trägt die Mindestangaben — Aufruf oder Daten-/Argumentfluss,
Vertrauensgrenze und Nachweis der Schutzmaßnahme — oder verweist präzise auf die Stelle im
Report, an der sie stehen. Der Report nennt alle betrachteten Gruppen-IDs. Bei Gruppen mit
gemischter Evidence muss klar erkennbar sein, welche Belege den SAST-Scope tragen und
welche nur Kontext sind.

## Handoff

Nach Abschluss verweist das Review auf `review-triage.md` im selben Ordner. Remediation und
Code-Änderungen sind ausdrücklich nicht Teil dieses Reviews.

**Eigenständiger `/k-review`-Lauf nach dem Umbau.** Dieses Rezept läuft im Audit mit; dort
steckt sein Beleg schon im gemeinsamen Merge und eine Aggregation danach wäre doppelt.
Über `review.enabled: true` bleibt es daneben einzeln aufrufbar, und ein solcher Lauf legt
einen eigenen Family-Ordner **außerhalb** jedes Laufordners an:

```text
k-playbook-local/results/sast/<datum>/
```

Sein `review-triage.md` geht direkt an `/k-remediation`:

```text
/k-remediation k-playbook-local/results/sast/<datum>/review-triage.md
```

Es gibt dabei **keine Zusammenführung mit dem Audit-Lauf und keine Dedupe gegen dessen
Befunde**. `/k-remediation` nimmt genau eine Ergebnisdatei; ein Befund, den derselbe Tag
auch im Audit-Lauf trägt, steht dann in beiden Ergebnissen einmal. Wer beide Seiten
zusammen sehen will, nimmt den Audit-Lauf — dort und nur dort sitzt die Zusammenführung.
