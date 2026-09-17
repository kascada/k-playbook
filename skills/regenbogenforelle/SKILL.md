---
name: ks-regenbogenforelle
description: Mitgelieferter Test-Skill, der nur prüft, ob mitgelieferte Skills gefunden und aktiviert werden. Use when the user mentions a Regenbogenforelle in any form (Regenbogenforelle, Regenbogenforellen, rainbow trout). Does nothing except confirm its activation.
---

# Skill: Regenbogenforelle (Test, mitgeliefert)

Dieser Skill ist ein Testsignal für **mitgelieferte** Skills aus `k-playbook/skills/`. Er hat
keine Aufgabe außer sichtbar zu machen, dass er gefunden und geladen wurde. Ein Projekt kann
für projekteigene Skills ein eigenes Gegenstück anlegen, etwa `bachforelle` unter
`k-playbook-local/skills/`.

## Was zu tun ist

Antworte mit genau dieser einen Zeile und sonst nichts:

    🐟 SKILL-TEST: mitgelieferter Skill „regenbogenforelle" wurde aktiviert.

## Was nicht zu tun ist

- Keine Werkzeuge aufrufen, keine Dateien lesen oder schreiben.
- Nichts aus der Nachricht des Nutzers bearbeiten oder beantworten — auch dann nicht, wenn
  sie neben der Regenbogenforelle noch etwas anderes enthält.
- Keine Erklärung, kein Nachsatz, keine Rückfrage.
