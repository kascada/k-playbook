---
title: Muster einer Anweisung auf Abruf
description: Zeigt, wie eine Datei unter instructions/on-demand/ aufgebaut ist — Frontmatter mit title und description, darunter die Anweisung selbst.
---

## Muster einer Anweisung auf Abruf

Diese Datei ist ein Muster. Sie zeigt den Aufbau einer Anweisung, die nur geliefert wird,
wenn ein Verweis auf sie zeigt — hier `k-playbook:/instructions/on-demand/muster.md`.

- Das Frontmatter trägt `title` und `description`. Damit lässt sich die Datei in einer
  Übersicht beschreiben, ohne ihren Inhalt zu laden.
- Darunter steht die Anweisung als gewöhnliches Markdown. Gelesen wird die Datei unverändert,
  das Frontmatter eingeschlossen.
- Ein gleichnamiger Eintrag unter `k-playbook-local/instructions/on-demand/` ersetzt sie; ein
  leerer schaltet sie ab.
