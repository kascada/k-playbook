---
title: publish und seine Antworten
description: Modul für /k-docs-code und /k-docs-tools, vom Hauptkontext gelesen, wenn die Dokumente geschrieben sind — k_playbook_docs_publish aufrufen, die Antwort je Dokument melden (created, written, deleted, unchanged, conflict, failed) und mit den Ablehnungen und Fehlern umgehen (invalid_input, hints_changed, conflict, kephalaion_unavailable, outcome_unknown).
---

Rufe `k_playbook_docs_publish` mit `projectDir` = `context.project.dir` und `erzeuger` = dem
Erzeuger des Commands auf (`code` bzw. `libs`; ersatzweise `k-playbook docs publish
<erzeuger> --json`). `allow_hints: true` nur, wenn der Nutzer geänderte, neue oder mit einem
Dokument entfallende `hints` bestätigt hat.

Melde die Antwort je Dokument: `created`, `written`, `deleted`, `unchanged`, `conflict`
oder `failed`, mit Revision; dazu `hintsChanged`, `readme` und ob aufgeräumt wurde
(`cleaned`). `open` nennt die Dokumente, die noch offen sind.

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
