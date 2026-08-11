---
description: Fast read-only health overview for the current project, backed by k-playbook-installer status JSON.
argument-hint: [full|codeql|reviews|json|strict]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Bash, Glob, Grep, TodoWrite]
---

# k-status

## Erster Schritt

Fuehre zuerst `commands/_shared/context.md` aus: rufe
`k-playbook/bin/k-playbook context` auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe.


Fuehre den Status-Check fuer das aktuelle Projekt aus.

Lies dafuer zuerst `commands/_details/k-status.md` aus dem k-playbook-Repo und befolge diese Anleitung mit `$ARGUMENTS`. Gib nicht den Inhalt der Detail-Datei aus.
