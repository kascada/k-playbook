---
title: publish und seine Antworten
description: Modul für /k-docs-code und /k-docs-tools, vom Hauptkontext gelesen, wenn die Dokumente geschrieben sind — vorher der Prüfmodus (check), dann k_playbook_docs_publish aufrufen, die Antwort je Dokument melden (created, written, deleted, unchanged, conflict, failed), mit den Ablehnungen und Fehlern umgehen (invalid_input, hints_changed, conflict, kephalaion_unavailable, outcome_unknown) und mit einer Abweichung des Codes seit dem checkout (stand).
---

**Prüfmodus zuerst.** Vor dem ersten `publish` eines Laufs und nach jeder Wiederaufnahme
(weitermachen nach `existing`) rufst du `k_playbook_docs_publish` mit `check: true` auf
(ersatzweise `k-playbook docs publish <erzeuger> --check --json`). Er schreibt nichts und
nennt je Dokument den geplanten Vorgang oder `conflict`. Bevor etwas veröffentlicht ist:

- `problems` — die Ablehnungen, die `publish` als `invalid_input` meldete: beheben, erneut
  prüfen.
- `hintsChanged` — wie `hints_changed` unten vorlegen.
- `conflict` — wie unten vorlegen. Die Vorhersage schließt einen Konflikt beim `publish`
  nicht aus.
- `readme` — `missing`, `differs` oder `unchanged`; `publish` schreibt sie selbst.

**Stand.** Prüfmodus und `publish` vergleichen den Code mit dem beim letzten checkout
(`stand`). Bei `stand.changed` hältst du `stand.files` gegen die Dokumente, die du in diesem
Lauf geschrieben hast: Hast du eine genannte Datei für eines davon gelesen, legst du die
Abweichung dem Nutzer vor dem nächsten `publish` vor — weiter, oder die betroffenen
Dokumente neu schreiben. Sonst nennt der Bericht sie in einer Zeile. Die genannten Dateien
behältst du für den Rest des Laufs, auch nach einem erneuten checkout, und hältst sie gegen
jedes Dokument, das du danach schreibst. `stand.incomplete` nennt der Bericht mit seinem
Grund, `compared: false` mit `stand.message`. Eine Abweichung verhindert `publish` nicht.

Dann rufe `k_playbook_docs_publish` mit `projectDir` = `project.dir` und `erzeuger` = dem
Erzeuger des Commands auf (`code` bzw. `libs`; ersatzweise `k-playbook docs publish
<erzeuger> --json`). `allow_hints: true` nur, wenn der Nutzer geänderte, neue oder mit einem
Dokument entfallende `hints` bestätigt hat.

Melde die Antwort je Dokument: `created`, `written`, `deleted`, `unchanged`, `conflict`
oder `failed`, mit Revision; dazu `hintsChanged`, `readme`, `stand` wie oben und ob
aufgeräumt wurde (`cleaned`). `open` nennt die Dokumente, die noch offen sind.

- `invalid_input` — eine Ablehnung im Arbeitsverzeichnis (fehlendes Frontmatter, falscher
  Name, `README.md`, andere Datei als `.md`). Nichts wurde geschrieben. Die Stellen
  beheben und `publish` erneut aufrufen.
- `hints_changed` — `hints` weichen vom checkout ab, oder ein Dokument mit `hints` wird
  gelöscht oder neu angelegt. Nichts wurde geschrieben. Dem Nutzer die genannten Dokumente
  vorlegen; erst nach Bestätigung mit `allow_hints: true` erneut aufrufen, sonst die
  `hints` zurücksetzen.
- `conflict` — das Dokument wurde seit dem checkout in Kephalaion geändert, angelegt oder
  gelöscht; die eigene Fassung wurde nicht geschrieben. **Vorlegen, nicht still
  nachziehen.** Der Nutzer entscheidet: eigene Änderung verwerfen, oder die eigene Fassung
  sichern, mit `k_playbook_docs_checkout` und `discard: true` neu holen — was schon
  veröffentlicht ist, kommt dabei mit —, die Änderung auf den neuen Stand übertragen und
  erneut veröffentlichen.
- `failed` mit `kephalaion_unavailable` — beim Hub ist nichts angekommen. Später erneut
  `publish`; es schreibt dann nur noch den Rest.
- `failed` mit `outcome_unknown` — der Vorgang kann angekommen sein. **Nicht** noch einmal
  veröffentlichen, sondern mit Kephalaion `read` nachsehen und dem Nutzer berichten.

Ein vollständiger Abgleich räumt das Arbeitsverzeichnis (`cleaned` ist wahr). Solange etwas
offen ist (`open`), bleiben Arbeitsverzeichnis und Merkdatei liegen; ein zweites `publish`
schreibt dann nur noch den Rest.
