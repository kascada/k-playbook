# /k-remediation

`/k-remediation` arbeitet Findings aus Review-Ergebnissen strukturiert ab. Der Command plant zuerst sinnvolle Buendel und entscheidet danach anhand der Projekt-Policy, ob Tasks erzeugt oder kleine direkte Fixes erlaubt sind.

## Zweck

`/k-remediation` ist nicht nur ein Fix-Command. Er ist der Planungs- und Handoff-Schritt fuer Findings:

- offene Findings laden.
- `known-decisions.md` beruecksichtigen.
- Findings nach Risiko, Aufwand, Kopplung und Verifikation buendeln.
- Remediation-Policy aus `K-PLAYBOOK.yaml` anzeigen.
- bestaetigte Buendel in Tasks oder freigegebene direkte Fixes ueberfuehren.
- Status und Task-Verweise nachvollziehbar in `findings.md` oder Summary pflegen.

## Aufrufe

```text
/k-remediation
/k-remediation k-playbook/reviews/results/summary-YYYY-MM-DD.md
/k-remediation k-playbook/reviews/results/<family>/<date>/assessment.md
```

## Unterstuetzte Inputs

- Result-Summaries von `/k-results` oder Report-Reviews ohne eigene `result-family`.
- Result-Familien wie `<paths.reviews>/results/<family>/<date>/assessment.md` mit zugehoerigem `findings.md`.
- Legacy-Dateien wie `<paths.reviews>/result-*.md`.

`raw/` und Run-Metadaten bleiben read-only. Sie sind auditierbare Belege und duerfen nicht umgeschrieben werden.

## Remediation-Policy

Die Policy steht in `K-PLAYBOOK.yaml` im Block `remediation:`. Wichtige Felder sind:

- `mode`: `task-branch-pr`, `task-first` oder `direct-allowed`.
- `target`: tatsaechlicher Code-/Git-Root.
- `grouping`: ob Findings vor der Umsetzung gebuendelt werden.
- `quick_wins`: ob einfache wirkungsstarke Buendel hervorgehoben werden.
- `branch_prefix`: empfohlener Prefix fuer Remediation-Branches.
- `pr_required`: ob PR-Handoff erwartet wird.
- `direct_fixes`: ob direkte Fixes ueberhaupt erlaubt sind.

Im Modus `task-branch-pr` erzeugt `/k-remediation` keine direkten Code-Fixes. Bestaetigte Buendel werden als Task-Dateien mit Ausfuehrungskontext geschrieben.

## Task-Handoff

Wenn `/k-remediation` Tasks erzeugt, ist der naechste Schritt nicht direkte Umsetzung im Chat, sondern der normale Task-Flow:

```text
/k-review-loop
/k-run
```

Die erzeugten Tasks sollen Branch-/PR-Hinweise enthalten, wenn die Policy das verlangt. `/k-run` wertet den Abschnitt `## Ausfuehrungskontext` aus und fuehrt vor der Delegation Branch- und Dirty-Worktree-Preflights aus.

## Statuspflege

Bei Result-Familien ist `findings.md` das Arbeitsregister. Typische Statuswerte sind:

- `open`
- `confirmed`
- `context-needed`
- `likely-false-positive`
- `accepted`
- `fixed`

Task-Verweise, Triage-Notizen und Remediation-Logs werden nachvollziehbar dort gepflegt. `assessment.md` darf fuer Handoff-Status oder Zusammenfassung aktualisiert werden, aber nicht als Ersatz fuer das Findings-Register dienen.

## Abgrenzung

- `/k-remediation` startet keine Scanner.
- `/k-remediation` priorisiert nicht projektweit neu; dafuer gibt es `/k-results`.
- Groessere Umsetzung laeuft ueber Tasks, `/k-review-loop` und `/k-run`.
- Direkte Fixes sind nur bei passender Policy und expliziter Freigabe erlaubt.
