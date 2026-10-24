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

Die Umgebung: `health` ist die Umgebungsprüfung, im Klartext der Block `Umgebung:` gleich nach
diesem Text. Nenne jeden Eintrag in deiner ersten Antwort (Stufe, Bereich, Satz). Einen Fehler
nennst du, bevor du mit der Arbeit beginnst, und bietest die Abhilfe aus `hint` an; bei einer
Warnung arbeitest du weiter. Nichts davon behebst du still — das tut der Nutzer oder beauftragt
es ausdrücklich. Bei `Umgebung: keine Probleme` schweigst du; `k_playbook_health` prüft neu.

Nicht selbst lesen: `K-PLAYBOOK.yaml` und die Katalogverzeichnisse unter `k-playbook/` und
`k-playbook-local/`. Ihr Inhalt kommt mit dem Briefing.

Ablegen: Jeder Ort leitet sich aus `local.dir` ab. Was wohin gehört, steht in
`k-playbook:/guidelines/betrieb.md`. Ein fehlendes Verzeichnis wird erfragt, nicht still angelegt.

Sub-Agenten: Ein Sub-Agent holt das Briefing nicht von selbst, auch wenn er diese Anweisungen
sieht. Wer einen startet, schreibt an den Anfang seines Auftrags: Führe zuerst `k-playbook briefing`
aus und lies die Ausgabe. Das entfällt nur, wenn der Auftrag reines Nachschlagen ist und nichts
schreibt, bewertet oder ablegt.

Der Abschnitt `context` ist der Rest von `k-playbook context` unter denselben Namen. Alles Übrige
zu den Feldern: `k-playbook:/instructions/on-demand/briefing-felder.md`.

<!-- NACHZIEHEN: Die context.-Namen wandern. Bekommt ein Feld seinen eigenen Platz im Briefing (docs/knowledge-aktuell.md, Tabelle „Context field → future place“), sind die Namen hier umzustellen. -->
