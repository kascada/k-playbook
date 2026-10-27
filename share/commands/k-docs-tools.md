---
description: Vorübergehend nicht verfügbar (Umbau auf keph:/docs/) — Detect project tools, libraries and stacks, let the user pick worthwhile entries, and write pitfall-focused references under k-playbook-local/docs/libs/. Thin wrapper around the _docs/tools module.
argument-hint: [scope-dir]
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Write, Edit, Bash, Glob, Grep, WebFetch, TodoWrite]
---

# k-docs-tools

## Vorübergehend nicht verfügbar

Dieser Command wird auf die Projektdoku in Kephalaion (`keph:/docs/`) umgebaut und steht bis
dahin nicht zur Verfügung. Führe nichts von dem aus, was darunter steht — nichts lesen,
nichts schreiben, nichts anbieten. Sag dem Nutzer in einem Satz, dass `/k-docs-tools`
vorübergehend nicht verfügbar ist, und nenne, was es schon gibt:

- das Versionsinventar: `/k-doc-inventory`, Ergebnis unter
  `keph:/docs/generated/versions/inventory.md`;
- die Projektdoku: `keph:/docs/`, Einstieg im Briefing;
- den Umzug alter Doku von der Platte: `/k-upgrade`, Umstellung 3.2.

Der Text darunter bleibt als Vorlage für den Umbau stehen.

## Erster Schritt

Wende `.claude/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.

Erzeugt Library- und Tool-Steckbriefe mit Fokus auf Pitfalls und Idiome. Der eigentliche
Ablauf liegt im nachladbaren Modul `.claude/commands/_docs/tools.md`, damit `/k-docs`
denselben Produzenten anbieten und ausführen kann.

Produces:
- `k-playbook-local/docs/libs/<name>.md` — eine Pitfall-Datei je ausgewähltem Tool.
- `k-playbook-local/docs/libs/README.md` — kurzer Erklärtext des Verzeichnisses, nur wenn
  die Datei fehlt.

## Schritt 1 — Modul anwenden

Wende `.claude/commands/_docs/tools.md` an und führe den dort beschriebenen Ablauf mit
denselben `$ARGUMENTS` aus.

Wenn das Modul fehlt, abbrechen und sagen, dass die k-playbook-Installation unvollständig
oder veraltet ist. Keine Ersatzlogik nachbauen.

## Schritt 2 — Abschluss

Der Abschluss kommt aus dem Modul. Folge-Command: **`/k-docs-index`** — nimmt die
geschriebenen Dateien in den einzigen Index `k-playbook-local/docs/README.md` auf.

## Fehlerfälle

- Modul `.claude/commands/_docs/tools.md` fehlt → abbrechen und `/k-gui` oder Update der
  Installation nennen.

## Anti-Muster (nicht tun)

- **Ablauf duplizieren.** Änderungen gehören in `_docs/tools.md`, nicht in diesen Wrapper.
