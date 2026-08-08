---
name: ks-enforcement
description: Use when code, docs, tests, tasks, reviews, checks, or implementation work may need k-playbook rules applied. Loads global rules from the k-playbook repo plus project-local rules from k-playbook/enforcement and makes the agent check them during work. Trigger keywords - "Enforcement", "Regeln berücksichtigen", "Checks durchgehen", "Code und Docs", "k-enforcement", "k-playbook/enforcement".
---

# Skill: Enforcement

**Kurzfassung.** Sorgt dafür, dass globale und projektlokale Enforcement-Regeln während der Arbeit aktiv berücksichtigt werden, nicht erst in einem späteren Review.

## Wann anwenden

Immer wenn eine Arbeit Regeln verletzen könnte, insbesondere bei:

- Code-Änderungen.
- Doc-Änderungen.
- Test-, Check-, Review- oder Remediation-Arbeiten.
- Änderungen an Tasks, Guidelines, `K-PLAYBOOK.yaml`, `AGENTS.md` oder OpenCode-Konfiguration.
- User-Aussagen wie „Enforcement", „Checks durchgehen", „Regeln berücksichtigen" oder „achte darauf".

## Regelquellen

Der Skill lädt zwei Regel-Ebenen und kombiniert sie per Overlay:

1. **Mitgeliefert:** `<DIST_DIR>/rules/*.md` — read-only, wird bei Updates ersetzt.
2. **Projektlokal:** das konfigurierte `paths.enforcement` — Eigentum des Projekts.

`PLAYBOOK_DIR` und `DIST_DIR` werden per Discovery bestimmt, wie in
`<DIST_DIR>/commands/_shared/path-resolution.md` beschrieben. Die Kombination der
beiden Ebenen folgt `<DIST_DIR>/commands/_shared/overlay-resolution.md`: Eine
projektlokale Regel ersetzt die gleichnamige mitgelieferte vollständig, und
`overlay.rules.disabled` schaltet einzelne mitgelieferte Regeln ab.

Wenn `K-PLAYBOOK.yaml` fehlt, ist das Verzeichnis kein k-playbook-Projekt; kurz
darauf hinweisen und ohne Enforcement weiterarbeiten. Wenn das projektlokale
Verzeichnis fehlt, mit den mitgelieferten Regeln fortfahren.

## Arbeitsweise

1. Vor relevanten Änderungen die Enforcement-Dateien laden.
2. Regeln als laufende Constraints behandeln, nicht nur als Abschluss-Check.
3. Bei jeder Code-Änderung besonders die Regel `docs-sync.md` beachten: betroffene Docs prüfen, anpassen oder bei Unsicherheit fragen.
4. Am Ende kurz berichten, welche Enforcement-Regeln relevant waren und ob offene Punkte bleiben.

## Expliziter Check

Für einen nachgelagerten, expliziten Check denselben Ablauf über den Command ausführen:

`/k-enforcement`

Details und Prüfschritte stehen in:

`<DIST_DIR>/skills/enforcement/PLAYBOOK.md`
