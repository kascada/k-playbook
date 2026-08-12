---
description: Fast read-only health overview for the current project, backed by the k-playbook context output.
argument-hint: [full|reviews|json|strict]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep, TodoWrite]
---

# k-status

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.


Führe den Status-Check für das aktuelle Projekt aus.

Lies dafür `<playbook.dir>/commands/_details/k-status.md` und befolge diese Anleitung mit `$ARGUMENTS`. Gib nicht den Inhalt der Detail-Datei aus.
