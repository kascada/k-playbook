---
name: ks-enforcement
description: Use when code, docs, tests, tasks, reviews, checks, or implementation work may need k-playbook rules applied. Loads global rules from the k-playbook repo plus project-local rules from K-PLAYBOOK.MD `enforcement:` and makes the agent check them during work. Trigger keywords - "Enforcement", "Regeln berücksichtigen", "Checks durchgehen", "Code und Docs", "k-enforcement", "k-playbook/enforcement".
---

# Skill: Enforcement

**Kurzfassung.** Sorgt dafür, dass globale und projektlokale Enforcement-Regeln während der Arbeit aktiv berücksichtigt werden, nicht erst in einem späteren Review.

## Wann anwenden

Immer wenn eine Arbeit Regeln verletzen könnte, insbesondere bei:

- Code-Änderungen.
- Doc-Änderungen.
- Test-, Check-, Review- oder Remediation-Arbeiten.
- Änderungen an Tasks, Guidelines, `K-PLAYBOOK.MD`, `AGENTS.md` oder OpenCode-Konfiguration.
- User-Aussagen wie „Enforcement", „Checks durchgehen", „Regeln berücksichtigen" oder „achte darauf".

## Regelquellen

Der Skill lädt zwei Regel-Ebenen:

1. **Global:** `<PLAYBOOK_REPO>/global/rules/*.md`
2. **Projektlokal:** der in `<TARGET_DIR>/K-PLAYBOOK.MD` registrierte `enforcement:`-Pfad

`PLAYBOOK_REPO` und `TARGET_DIR` werden nach derselben Logik bestimmt wie in `commands/_shared/path-resolution.md`.

Wenn `K-PLAYBOOK.MD` fehlt oder `enforcement:` leer ist, gelten nur die globalen Regeln. Wenn ein eingetragener projektlokaler Pfad fehlt, den User kurz darauf hinweisen und mit den globalen Regeln fortfahren, sofern die Arbeit dadurch nicht blockiert ist.

## Arbeitsweise

1. Vor relevanten Änderungen die Enforcement-Dateien laden.
2. Regeln als laufende Constraints behandeln, nicht nur als Abschluss-Check.
3. Bei jeder Code-Änderung besonders die Regel `docs-sync.md` beachten: betroffene Docs prüfen, anpassen oder bei Unsicherheit fragen.
4. Am Ende kurz berichten, welche Enforcement-Regeln relevant waren und ob offene Punkte bleiben.

## Expliziter Check

Für einen nachgelagerten, expliziten Check denselben Ablauf über den Command ausführen:

`/k-enforcement`

Details und Prüfschritte stehen in:

`~/dev/k-playbook/ks-enforcement/PLAYBOOK.md`
