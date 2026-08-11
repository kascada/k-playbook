---
name: ks-enforcement
description: Use when code, docs, tests, tasks, reviews, checks, or implementation work may need k-playbook rules applied. Loads the effective rule catalog - shipped plus project-local - and makes the agent check it during work. Trigger keywords - "Enforcement", "Regeln berücksichtigen", "Checks durchgehen", "Code und Docs", "k-enforcement".
---

# Skill: Enforcement

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Skills stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


**Kurzfassung.** Sorgt dafür, dass globale und projektlokale Enforcement-Regeln während der Arbeit aktiv berücksichtigt werden, nicht erst in einem späteren Review.

## Wann anwenden

Immer wenn eine Arbeit Regeln verletzen könnte, insbesondere bei:

- Code-Änderungen.
- Doc-Änderungen.
- Test-, Check-, Review- oder Remediation-Arbeiten.
- Änderungen an Tasks, Guidelines, `K-PLAYBOOK.yaml`, `AGENTS.md` oder OpenCode-Konfiguration.
- User-Aussagen wie „Enforcement", „Checks durchgehen", „Regeln berücksichtigen" oder „achte darauf".

## Regelquelle

Die effektive Regelmenge steht in `catalogs.rules` der Context-Ausgabe. Sie führt
die mitgelieferten Regeln aus `<playbook.dir>/rules/` und die projekteigenen aus
`<local.dir>/rules/` bereits zusammen: eine projekteigene Regel ersetzt die
gleichnamige mitgelieferte vollständig, eine leere gleichnamige Datei schaltet sie
ab und ist als `disabled` markiert.

Wenn der Context-Aufruf fehlschlägt, ist das Verzeichnis kein k-playbook-Projekt;
kurz darauf hinweisen und ohne Enforcement weiterarbeiten.

## Arbeitsweise

1. Vor relevanten Änderungen die Enforcement-Dateien laden.
2. Regeln als laufende Constraints behandeln, nicht nur als Abschluss-Check.
3. Bei jeder Code-Änderung besonders die Regel `docs-sync.md` beachten: betroffene Docs prüfen, anpassen oder bei Unsicherheit fragen.
4. Am Ende kurz berichten, welche Enforcement-Regeln relevant waren und ob offene Punkte bleiben.

## Expliziter Check

Für einen nachgelagerten, expliziten Check denselben Ablauf über den Command ausführen:

`/k-enforcement`

Details und Prüfschritte stehen in:

`<playbook.dir>/skills/enforcement/PLAYBOOK.md`
