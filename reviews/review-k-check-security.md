---
name: review-k-check-security
title: k-check Security Assessment
interval-weeks: 4
scope-hint: k-check Runner-Ergebnisse der mitgelieferten Checks; keine Remediation, keine Produktcode-Änderungen
handoff: /k-remediation
result-family: k-check
audit:
  enabled: false
review:
  enabled: true
---

# Review: k-check Security Assessment

Erzeuge eine kuratierte Security-Bewertung aus `k-check`-Ergebnissen. `k-check` bleibt
Runner/Executor; Review, Priorisierung und Remediation-Handoff gehören in diese
Review-Familie und danach in `/k-remediation`.

## Stellung im Audit-Laufmodell

Dieses Rezept bleibt im Audit-Laufmodell deaktiviert (`audit.enabled: false`). Über
`/k-review k-check-security` bleibt es gezielt auswählbar.

**Geprüft und verworfen: Umstellung auf `audit.mode: evidence`.** Der Check-Lauf selbst
liefe ohne Go-Änderung — ein Evidence-Rezept darf `bin/k-check` aufrufen und sein SARIF
nach `raw/k-check.sarif` schreiben. Am Evidence-Vertrag scheitert es trotzdem, an drei
Stellen:

1. **`audit.ruleIds` ist eine abgeschlossene Liste, die Check-Menge wächst mit der
   Installation.** Jede Rule-ID muss in `audit.ruleIds` stehen; eine unbekannte macht
   das ganze Artefakt ungültig, nicht nur den einzelnen Fund. Die natürliche Rule-ID eines
   k-check-Funds ist sein Check — und welche Checks laufen, bestimmt der mitgelieferte
   Katalog der jeweiligen Installation. Das Rezept müsste jeden davon aufzählen und mit
   jedem neuen Check nachgezogen werden; ein vergessener ließe den ganzen Lauf des
   Eintrags scheitern. Projekteigene Checks, mit denen dieser Punkt zuerst begründet war,
   gibt es seit Task 134 nicht mehr.
2. **Funde ohne Ort fielen heraus.** Ein Evidence-Fund braucht einen Fundort im
   Pfad-Scope; ein Fund ohne Ort liegt nie im Scope und wird verworfen. Die Parser-Regeln
   unten halten ausdrücklich fest, dass nicht jede Ausgabe `path:line: message` ist —
   „Ein Check kann Sammelzeilen oder abweichende Formate liefern". Genau diese Funde
   verschwänden stillschweigend.
3. **`ok`, `skip` und technische `error`-Fälle haben in SARIF keinen Platz.** Dieses
   Rezept verlangt, sie getrennt zu dokumentieren — `skip` mit Reason und Wiedervorlage,
   `error` als blockierte Bewertbarkeit. Ein SARIF trägt Funde, keine Nicht-Funde mit
   Grund. Der Zustandsteil des Check-Laufs ginge verloren.

Der bereits vorgesehene Weg ist ein anderer und größerer: `k-check` gibt heute
Terminaltext aus, seine Parameter stehen als Prosa in diesem Rezept, und
`--metadata-output` schreibt bereits JSON. Ein MCP-Werkzeug, das statt der Rohausgabe
zurückgibt, welche Checks liefen und welche mit Datei und Zeile angeschlagen haben, macht
die Evidence-Frage erst beantwortbar. Das ist Arbeit am Go-Werkzeug und braucht einen
eigenen Task; die Rohausgabe bleibt dabei für `raw/` erhalten.

**Was das für den Weg dieses Rezepts bedeutet.** Ein `/k-review k-check-security`-Lauf
legt einen Family-Ordner außerhalb jedes Laufordners an; sein `review-triage.md` geht
direkt an `/k-remediation`:

```text
/k-remediation k-playbook-local/results/k-check/<datum>/review-triage.md
```

Es gibt dabei **keine familienübergreifende Zusammenführung und keine Dedupe gegen andere
Quellen** mehr. `/k-remediation` nimmt genau eine Ergebnisdatei; ein Secret-Fund, den
daneben auch `gitleaks` im Audit-Lauf gemeldet hat, steht in beiden Ergebnissen einmal.
Das ist die bewusste Folge des Umbaus, kein Fehler: Wer eine Zusammenführung braucht,
bringt seine Belege in den Lauf, statt sie danach zusammenzurechnen.

## Zweck

- Die Ausgaben der mitgelieferten Checks dauerhaft als Review-Artefakte sichern.
- Runner-Rohdaten von kuratierter Bewertung trennen.
- Findings mit stabilen IDs und Status-Lifecycle für spätere Remediation vorbereiten.
- `skip`, `ok`, `fail` und technische `error`-Fälle getrennt dokumentieren.
- Keine Produktcode-Änderungen durch dieses Review.

## Parser-Regeln

`k-check`-Ausgaben werden nach diesen Regeln ausgewertet:

- Check-Abschnitte beginnen mit `== <scope>:<check.sh> ==`, z. B.
  `== dist:check_no_obvious_secrets.sh ==`. `<scope>` ist immer `dist`: `k-check` führt
  nur die mitgelieferten Checks aus.
- Innerhalb eines Abschnitts genau eine `K_CHECK_STATUS=<ok|skip|fail>`-Zeile erfassen.
- Optional `K_CHECK_REASON=<text>` erfassen.
- Alles zwischen Abschnittsheader und Status/Summary als Check-Ausgabe behandeln.
- Die Summary beginnt bei `K_CHECK_SUMMARY` und enthält `config_root`, `target_root`,
  `mode`, `file_source`, `files`, `ok`, `skip`, `fail`, `error`.
- Summary-Zeilen `OK|SKIP|FAIL|ERROR <scope>:<check.sh> reason=...` als
  Check-Gesamtstatus erfassen.
- `ok` und `skip` in `review-triage.md` separat dokumentieren; `skip` mit Reason und
  Wiedervorlage.
- `fail`-Findings nach Check-Familie gruppieren.
- Technische `error`-Fälle sind keine Security-Findings, aber blockieren die Bewertbarkeit
  des betroffenen Checks.

Finding-Zeilen sind check-spezifisch. Die mitgelieferten Checks schreiben typischerweise:

```text
path:line: message
```

Ein Check kann Sammelzeilen oder abweichende Formate liefern. Meldet ein Lauf dieselbe
semantische Stelle mehrfach, eine Finding-ID behalten und die weiteren Meldungen als
Beleg/Notiz deduplizieren statt eine zweite Remediation-ID zu erzeugen.

## Status-Lifecycle

Statuswerte im Remediation-Status von `review-triage.md`:

- `open` - neu oder noch nicht geprüft.
- `confirmed` - validierter echter Befund.
- `context-needed` - ohne weiteren Code-/Betriebskontext nicht belastbar bewertbar.
- `likely-false-positive` - plausibler Fehlalarm; review-relevant, aber nur nach
  expliziter Auswahl remediation-relevant.
- `accepted` - bewusst akzeptiertes Restrisiko oder bekannte Entscheidung.
- `fixed` - behoben und verifiziert.

Remediation-relevant sind `open`, `confirmed` und `context-needed`. `accepted` und
`fixed` sind Endzustände und dürfen nicht automatisch in neue Fix-Tasks überführt werden.

## Stabile Finding-IDs

Einmal vergebene IDs dürfen bei Re-Runs, Statusänderungen oder Remediation nicht
umbenannt werden.

Schema für neue k-check-IDs:

`kcheck-<area>-NNN`

Beispiele:

- `kcheck-logging-003`
- `kcheck-secrets-001`
- `kcheck-user-scope-014`

Neue IDs werden nur für neue semantische Findings vergeben. Wiedergefundene oder
deduplizierte Findings behalten die bestehende ID. Importierte Tool-IDs anderer Familien
dürfen ihr natives Präfix behalten.

## Priorisierung

- P1: echte Secrets, produktive Credentials, raw Provider-/Upstream-Responses in Logs.
- P1/P2: user-facing Authz-/Ownership-Findings.
- P2: sensitive Logging, Operational-Event-Kontraktverletzungen.
- P3: Legacy-Baseline, Test-Fixtures, wahrscheinliche Heuristik-False-Positives.

## Handoff

Nach Abschluss verweist `/k-review` auf `review-triage.md` im Family-Ordner. Remediation
ist ausdrücklich nicht Teil dieses Reviews.
