---
title: Das Briefing
---

## Das Briefing

Das Briefing ist das Erste, was eine Sitzung liest, und es gilt für die ganze Sitzung: Liegt es
vor, wird es nicht noch einmal geholt. Neu geladen wird es nur, wenn sich das Projekt, die
Konfiguration, der Katalog oder die Installation geändert hat oder `context.now.date` nicht mehr
stimmt — die vollständige Liste steht in `k-playbook:/instructions/on-demand/briefing-felder.md`.
Felder aus zwei Ladungen werden nie gemischt.

Die Felder jeder Sitzung:

- `context.now.date` ist das Datum. Nie raten, nie `date` rufen.
- `context.project.dir`, `context.local.dir` und `context.playbook.dir` sind die drei Wurzeln.
  Alles Projekteigene liegt unter `local.dir`; nach `playbook.dir` wird nie geschrieben.
- `documents.*` sind Verweise, gelesen mit `k_playbook_doc_read`. Einträge mit der Herkunft
  `disabled` werden übersprungen; die Überlagerung ist schon gerechnet.
- `work` ist die eigene offene Arbeit.

Nicht selbst lesen: `K-PLAYBOOK.yaml` und die Katalogverzeichnisse unter `k-playbook/` und
`k-playbook-local/`. Ihr Inhalt kommt mit dem Briefing.

Ablegen: Jeder Ort leitet sich aus `local.dir` ab. Was wohin gehört, steht in
`k-playbook:/guidelines/betrieb.md`. Ein fehlendes Verzeichnis wird erfragt, nicht still angelegt.

Der Abschnitt `context` ist der Rest von `k-playbook context` unter denselben Namen. Alles Übrige
zu den Feldern: `k-playbook:/instructions/on-demand/briefing-felder.md`.

<!-- NACHZIEHEN: Die context.-Namen wandern. Bekommt ein Feld seinen eigenen Platz im Briefing (docs/knowledge-aktuell.md, Tabelle „Context field → future place“), sind die Namen hier umzustellen. -->
