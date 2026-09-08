---
description: "Listet, ergänzt, ändert, hakt ab und löscht die Todos des Projekts. Ohne Argument listen; done/delete/edit/reopen steuern einen Eintrag, alles andere ist ein neuer Eintrag."
argument-hint: [text | done <id|stichwort> | delete <id|stichwort> | edit <id> <text> | reopen <id>]
# model: github-copilot/gpt-5.5
allowed-tools: [Bash, k_playbook_todo_list, k_playbook_todo_add, k_playbook_todo_update, k_playbook_todo_delete]
---

# k-todo

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe; die
`K-PLAYBOOK.yaml` wird nicht selbst gelesen.

Die Todos liegen in `k-playbook-local/data/todos.json`. Diese Datei gehört Go.
**Sie wird in diesem Command nie gelesen und nie geschrieben** — weder mit Read
noch mit Write oder Edit. Jede Auskunft und jede Änderung läuft über den
Zugriffsweg aus Schritt 1.

## Schritt 1 — Zugriffsweg wählen

In dieser Reihenfolge, der erste verfügbare gewinnt:

1. **MCP-Werkzeuge.** Bietet der Client `k_playbook_todo_list`,
   `k_playbook_todo_add`, `k_playbook_todo_update` und `k_playbook_todo_delete`
   an, werden sie benutzt. `projectDir` ist `project.dir` aus dem Kontext.
2. **Subkommando.** Bietet er sie nicht an, wird `k-playbook todo …` über Bash
   aufgerufen. Das ist der Normalfall und keine Notlösung: der MCP-Server wird
   pro Projekt registriert und kann fehlen, das Binary ist mit der Installation
   da. Die Ausgabe ist JSON auf stdout.

Beide Wege rufen dieselbe Fachlogik auf und geben dieselben Felder zurück:
`id`, `text`, `created`, `done` (leer heißt offen), `doneMigrated`, `origin`.

## Schritt 2 — Argumente deuten

| Eingabe | Aktion |
|---|---|
| kein Argument | listen |
| `done <id\|stichwort>` | abhaken |
| `delete <id\|stichwort>` | löschen |
| `edit <id> <text>` | Text ändern |
| `reopen <id>` | wieder öffnen |
| alles andere | neuer Eintrag mit genau diesem Text |

**Listen.** `k_playbook_todo_list` bzw. `k-playbook todo list`. Ausgegeben wird
je Zeile Kennung, Datum und Text, etwa `#3 (2026-08-25) pip-audit einbauen`.
Die erledigten kommen über `includeDone` bzw. `--done` dazu, wenn der Nutzer
danach fragt.

**Anlegen.** `k_playbook_todo_add` bzw. `k-playbook todo add "<text>"`. Der Text
geht unverändert durch — nicht umformulieren, nicht kürzen.

**Abhaken, ändern, wieder öffnen.** `k_playbook_todo_update` bzw.
`k-playbook todo update <id> --done` / `--text "<text>"` / `--reopen`.

**Löschen.** `k_playbook_todo_delete` bzw. `k-playbook todo delete <id>`.
Löschen ist der Fall für Fehleingaben, nicht für Erledigtes — was getan ist,
wird abgehakt und bleibt stehen.

**Stichwort statt Kennung.** Steht kein `<id>`, sondern ein Stichwort: erst die
Liste holen, dann die Treffer zeigen und bestätigen lassen, danach aufrufen.
Bei mehreren Treffern zur Auswahl stellen. Ohne Bestätigung wird nichts
geschrieben — die unscharfe Zuordnung ist Sache des Modells, aber nicht ohne
Rückfrage.

## Schritt 3 — Nebenmeldungen weitergeben

- `migrated: <anzahl>` — dieser Aufruf hat eine vorhandene `TODO.md` nach
  `data/todos.json` übersetzt und danach entfernt. Einmal nennen, dann normal
  weiterarbeiten.
- `hint` — eine `TODO.md` liegt noch neben der JSON-Datei. Kein Fehler: gelesen
  und geschrieben wird die JSON-Datei. Den Hinweis samt genanntem Ausweg
  weitergeben und **nicht** von Hand aufräumen.

## Fehlerfälle

- `k-playbook todo` meldet `unbekanntes Kommando` → die Installation ist älter
  als dieser Command. Melden, Update oder `/k-gui` nennen, und die Verwaltung
  **nicht** von Hand nachbauen. Derselbe Fall wie bei `k-playbook inventory`.
- Kein Projekt gefunden → `/k-gui` nennen, nichts anlegen.
- Unbekannte Kennung → melden und die Liste zeigen, damit der Nutzer wählen kann.

## Anti-Muster (nicht tun)

- **`data/todos.json` von Hand lesen oder schreiben.** Auch nicht „nur kurz
  nachsehen": es gäbe dann zwei Auslegungen des Formats, und die zweite ginge in
  keine Prüfung ein. Einzige Ausnahme im ganzen Projekt ist die Auflösung eines
  Merge-Konflikts, und die macht ein Mensch.
- **Eine `TODO.md` schreiben oder anlegen.** Die Markdown-Ablage ist Geschichte;
  sie wird beim ersten Zugriff übersetzt.
- **Erledigtes löschen.** Abhaken behält den Eintrag, Löschen wirft ihn weg.
- **Bei einem Stichwort ungefragt schreiben.** Erst Treffer zeigen, dann handeln.
