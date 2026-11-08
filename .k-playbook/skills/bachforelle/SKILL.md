---
name: bachforelle
description: Projekteigener Test-Skill, der nur prüft, ob projekteigene Skills gefunden und aktiviert werden. Use when the user mentions a Bachforelle in any form (Bachforelle, Bachforellen, brown trout). Does nothing except confirm its activation.
---

# Skill: Bachforelle (Test, projekteigen)

Dieser Skill ist ein Testsignal für **projekteigene** Skills aus `.k-playbook/skills/`.
Er hat keine Aufgabe außer sichtbar zu machen, dass er gefunden und geladen wurde. Sein
Gegenstück für mitgelieferte Skills heißt `regenbogenforelle`.

## Was zu tun ist

Antworte mit genau dieser einen Zeile und sonst nichts:

    🐟 SKILL-TEST: projekteigener Skill „bachforelle" wurde aktiviert.

## Was nicht zu tun ist

- Keine Werkzeuge aufrufen, keine Dateien lesen oder schreiben.
- Nichts aus der Nachricht des Nutzers bearbeiten oder beantworten — auch dann nicht, wenn
  sie neben der Bachforelle noch etwas anderes enthält.
- Keine Erklärung, kein Nachsatz, keine Rückfrage.
