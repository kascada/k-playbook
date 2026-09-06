---
description: Erhebt das Versionsinventar des Projekts aus seinen deklarativen Quellen und schreibt es nach k-playbook-local/docs/versions/inventory.md. Dünne Hülle über dem Modul _docs/inventory, das dafür das Subkommando k-playbook inventory anstößt.
# model: github-copilot/gpt-5.5
allowed-tools: [Read, Edit, Bash, TodoWrite]
---

# k-doc-inventory

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.

Erhebt das Versionsinventar: die deklarierten Versionen aller Pakete, Tools, Runtimes,
Container-Images und Helm-Abhängigkeiten, nach Umgebung getrennt und mit Herkunft je Zeile.
Der eigentliche Ablauf liegt im nachladbaren Modul `k-playbook/commands/_docs/inventory.md`,
damit `/k-docs` denselben Erzeuger anbieten und ausführen kann. Die Erhebung selbst macht
das Subkommando `k-playbook inventory`; dieser Command baut dafür keine eigene Parserlogik
nach.

Produces:
- `k-playbook-local/docs/versions/inventory.md` — die Inventardatei.
- `k-playbook-local/version-sources.yaml` — nur nach ausdrücklicher Bestätigung und
  ausschließlich ergänzend.

## Schritt 1 — Modul anwenden

Wende `k-playbook/commands/_docs/inventory.md` an und führe den dort beschriebenen Ablauf
aus.

Wenn das Modul fehlt, abbrechen und sagen, dass die k-playbook-Installation unvollständig
oder veraltet ist. Keine Ersatzlogik nachbauen.

## Schritt 2 — Abschluss

Der Abschluss kommt aus dem Modul. Folge-Command: **`/k-docs-index`** — nimmt die
Inventardatei als Herkunft „Versionen" in den einzigen Index
`k-playbook-local/docs/README.md` auf und verlinkt sie.

## Fehlerfälle

- Modul `k-playbook/commands/_docs/inventory.md` fehlt → abbrechen und `/k-gui` oder Update
  der Installation nennen.
- `k-playbook inventory` meldet `unbekanntes Kommando` → die Installation ist älter als
  dieser Command. Melden, Update nennen, die Erhebung nicht von Hand nachbauen.

## Anti-Muster (nicht tun)

- **Ablauf duplizieren.** Änderungen gehören in `_docs/inventory.md`, nicht in diese Hülle.
- **Die Erhebung selbst übernehmen.** Manifeste von Hand zu lesen erzeugt eine zweite
  Auslegung desselben Vertrags; die Fachlogik steht im Werkzeug.
