---
description: Generate semantic project documentation from code under k-playbook-local/docs/code/. Defaults to the current directory, or uses [target-dir] if given. Thin wrapper around the _docs/code module.
argument-hint: [target-dir]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, TodoWrite]
---

# k-docs-code

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.

Erzeugt semantische Projekt-Doku aus dem Code. Der eigentliche Ablauf liegt im
nachladbaren Modul `k-playbook/commands/_docs/code.md`, damit `/k-docs` denselben
Produzenten anbieten und ausführen kann.

Produces:
- `k-playbook-local/docs/code/<NN>-<slug>.md` — one file per coherent topic.

## Schritt 1 — Modul anwenden

Wende `k-playbook/commands/_docs/code.md` an und führe den dort beschriebenen Ablauf mit
denselben `$ARGUMENTS` aus.

Wenn das Modul fehlt, abbrechen und sagen, dass die k-playbook-Installation unvollständig
oder veraltet ist. Keine Ersatzlogik nachbauen.

## Schritt 2 — Abschluss

Der Abschluss kommt aus dem Modul. Folge-Command: **`/k-docs-index`** — baut aus allen
Herkünften den einzigen Index `k-playbook-local/docs/README.md` und registriert die Docs in
`AGENTS.md` und `opencode.json`.

## Fehlerfälle

- Modul `k-playbook/commands/_docs/code.md` fehlt → abbrechen und `/k-gui` oder Update der
  Installation nennen.

## Anti-Muster (nicht tun)

- **Ablauf duplizieren.** Änderungen gehören in `_docs/code.md`, nicht in diesen Wrapper.
