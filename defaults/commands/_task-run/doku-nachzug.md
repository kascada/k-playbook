---
title: Doku-Nachzug nach dem Lauf
description: Modul für /k-task-run, vom Hauptkontext gelesen in Schritt 7, wenn der Lauf Code oder ein Manifest geändert hat — welche Pfade als Code zählen, wie je Erzeuger ein Doku-Auftrag eingereiht wird (Verweis auf doku-auftraege.md), und was bei unbekannten Pfaden oder ohne Kephalaion gilt.
---

# Doku-Nachzug nach dem Lauf

Dieses Modul liest der Hauptkontext von `/k-task-run` in Schritt 7, wenn `CHANGED_PATHS`
nicht leer ist oder ein Task „Pfade unbekannt“ trägt.

Vor der Zusammenfassung prüft der Hauptkontext, ob der Lauf Code geändert
hat, und reiht dann den Nachzug der erzeugten Doku ein — er, nicht der
Ausführungs-Sub-Agent, denn die Sub-Agenten laden das Briefing nicht. Maßgeblich sind der
Doku-Nachzug in `k-playbook:/instructions/always/220-regeln-anwenden.md` und
`k-playbook:/instructions/on-demand/doku-auftraege.md`; dieser Text wiederholt sie nicht.

- Grundlage ist `CHANGED_PATHS` aus 5f, Schritt 6 — nur Namen, kein Diff. Code für `code` ist
  Quelltext unter `project.repoRoot` außerhalb von `.k-playbook/`, versteckten Pfaden und
  `environment.docs.exclude` — das holt er an dieser Stelle über den Abruf `environment`
  (`k_playbook_briefing` mit `part: "environment"`, ersatzweise `k-playbook briefing --part environment`), es steht nicht
  im Briefing; ein geändertes Manifest oder Lockfile betrifft `tools`.
- Je betroffenem Erzeuger ruft er `k_playbook_docs_order_add` auf (ersatzweise `k-playbook
  docs order add <code|tools> --pfad <pfad>… --anlass "<satz>" --sitzung <session> --json`):
  `projectDir` = `project.dir`, `erzeuger` `code` bzw. `tools`, `pfade` aus
  `CHANGED_PATHS`, so fein wie möglich, `anlass` ein Satz mit den Tasknummern, `sitzung` = die
  `session` seines Briefings. „Kein Doppel“ und „Wann nicht eingereiht wird“ rechnet das
  Programm nach `doku-auftraege.md`; die Antwort sagt `status`: `created` oder `extended`
  (eingereiht), `covered` (ein Auftrag deckt schon, mit `ref`), `skipped` (`no_generated_docs`:
  nichts nachzuziehen; `kephalaion_unavailable`: nichts eingereiht, der Nachzug bleibt offen).
  Das geschieht ohne Rückfrage: Ein Auftrag ändert keine Doku, er merkt den Nachzug vor. Ist
  das Werkzeug unbekannt (`invalid_input`) oder meldet `k-playbook docs` `unbekannter
  Unterbefehl`, ist die Installation älter: melden, `/k-upgrade` nennen, nichts von Hand
  anlegen.
- Ändert der Lauf ein Manifest, ist das Inventar sofort nachzuziehen; das tut dieser Command
  nicht, die Zeile „Doku“ nennt `/k-doc-inventory`.
- Ohne Kephalaion oder bei einem Task mit „Pfade unbekannt“ wird für ihn nichts eingereiht;
  die Zeile „Doku“ nennt den Nachzug als offen und `/k-enforcement`.
