---
description: Detect project tools, libraries and stacks, let the user pick worthwhile entries, and write pitfall-focused references under k-playbook-local/docs/libs/. Thin wrapper around the _docs/tools module.
argument-hint: [scope-dir]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, WebFetch, TodoWrite]
---

# k-docs-tools

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.

Erzeugt Library- und Tool-Steckbriefe mit Fokus auf Pitfalls und Idiome. Der eigentliche
Ablauf liegt im nachladbaren Modul `k-playbook/commands/_docs/tools.md`, damit `/k-docs`
denselben Produzenten anbieten und ausführen kann.

Produces:
- `k-playbook-local/docs/libs/<name>.md` — eine Pitfall-Datei je ausgewähltem Tool.
- `k-playbook-local/docs/libs/README.md` — kurzer Erklärtext des Verzeichnisses, nur wenn
  die Datei fehlt.

## Schritt 1 — Modul anwenden

Wende `k-playbook/commands/_docs/tools.md` an und führe den dort beschriebenen Ablauf mit
denselben `$ARGUMENTS` aus.

Wenn das Modul fehlt, abbrechen und sagen, dass die k-playbook-Installation unvollständig
oder veraltet ist. Keine Ersatzlogik nachbauen.

## Schritt 2 — Abschluss

Der Abschluss kommt aus dem Modul. Folge-Command: **`/k-docs-index`** — nimmt die
geschriebenen Dateien in den einzigen Index `k-playbook-local/docs/README.md` auf.

## Fehlerfälle

- Modul `k-playbook/commands/_docs/tools.md` fehlt → abbrechen und `/k-gui` oder Update der
  Installation nennen.

## Anti-Muster (nicht tun)

- **Ablauf duplizieren.** Änderungen gehören in `_docs/tools.md`, nicht in diesen Wrapper.
