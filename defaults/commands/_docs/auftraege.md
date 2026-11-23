---
title: Doku-Aufträge lesen und abschließen
description: Modul für /k-docs-code und /k-docs-tools, vom Hauptkontext gelesen im ersten Schritt — die offenen Doku-Aufträge des Erzeugers mit k-playbook docs orders lesen und bewerten lassen (Lauf, roh oder zerlegt, wartend, zusammengelegt, nichts zu tun), die Rückmeldungen von k_playbook_doc_done beim Abschließen und was bei einer älteren Installation gilt.
---

Nach einer Codeänderung reiht eine Sitzung den Nachzug als Auftrag in `keph:/queue/docs/`
ein, statt den Erzeuger sofort laufen zu lassen; Kopf, Reifegrade und Lebenslauf stehen in
`k-playbook:/instructions/on-demand/doku-auftraege.md`. Ein Erzeuger arbeitet die Aufträge
seines `erzeuger` ab: `/k-docs-code` die mit `erzeuger: code`, `/k-docs-tools` die mit
`erzeuger: tools` — dessen Verzeichnis heißt `libs`, der Auftrag `tools`. Bei `code`
steuern die Aufträge den Lauf, bei `tools` begründen sie ihn.

**Lesen und bewerten lässt sie das Programm.** Rufe `k_playbook_docs_orders` mit `projectDir`
= `context.project.dir` und `erzeuger` = `code` bzw. `tools` auf (ersatzweise
`k-playbook docs orders <code|tools> --json`). Die Wartezeit, das Zusammenlegen abhängiger
Aufträge und die Wahl des Laufs rechnet das Programm; sie stehen nirgends sonst und werden
nicht nachgerechnet. Die Antwort trägt:

- `run` — `full` (ein roher Auftrag mit `pfade: .` nimmt alle auf, auch die wartenden),
  `partial` (nur die reifen rohen), `resume` (nur zerlegte, ein früherer Lauf ist
  unterbrochen) oder `none`; `waitMinutes` ist die Wartezeit.
- `orders` — je Auftrag `ref`, `erzeuger`, `pfade`, `anlass`, `entstanden`, `updated`, bei
  einem zerlegten `dokument` und `aktion`, bei einem wartenden `waiting` mit
  `remainingMinutes`.
- Für `code` dazu `rawOrders` (die rohen nach dem Zusammenlegen, je mit den Verweisen
  `refs` der Aufträge, die darin aufgegangen sind, den vereinigten `pfade`, den gesammelten
  `anlaesse` und dem ältesten `entstanden`), `splitOrders` (die offenen zerlegten) und
  `waiting` (die wartenden mit `remainingMinutes`).
- `nothingToDo` mit `reason` — bei `code`: es gibt nichts abzuarbeiten, der Lauf endet; bei
  `tools` nur: es gibt keine offenen Aufträge, der Lauf kann trotzdem laufen.
- `problems` — Aufträge, die das Programm nicht lesen konnte; nennen, nicht raten.

Bei `tools` ist jeder offene Auftrag Teil von `orders`; eine Wartezeit gibt es nicht.

**Rückmeldungen beim Abschließen.** Abgeschlossen wird je Auftrag mit `k_playbook_doc_done`
(ersatzweise `k-playbook doc done <ref> --json`); sie liegen danach unter
`keph:/queue/docs/done/`.

- `wrong_dir` oder `not_found` heißt, ein anderer Lauf hat den Auftrag schon abgeschlossen —
  nennen, nicht wiederholen.
- `outcome_unknown` heißt, das Verschieben kann angekommen sein: nicht wiederholen, sondern
  mit `k_playbook_doc_list` (`kind: docs`, `done: true`) nachsehen.
- Kennt ein Werkzeug die Sorte `docs` nicht (`invalid_input`), ist die Installation älter
  als der Command: melden, `/k-upgrade` nennen; der Lauf selbst bleibt gültig.

**Ältere Installation.** Ist `k_playbook_docs_orders` unbekannt (`invalid_input` für das
Werkzeug) oder meldet `k-playbook docs` `unbekannter Unterbefehl` oder `unbekanntes
Kommando`, ist die Installation älter als der Command: melden, `/k-upgrade` nennen, die
Aufträge nicht von Hand lesen, bewerten oder zusammenlegen und nichts ersetzen.

**Abschluss.** Die Zusammenfassung nennt die abgeschlossenen Aufträge mit Verweis und die,
die offen geblieben sind, mit Grund.
