---
title: Die Workbench
---

## Die Workbench

Ein Vorhaben, das länger als eine Sitzung dauert, bekommt ein Thema in deiner Workbench,
`keph:/user/~/workbench/`. Das Thema sagt, wo das Vorhaben steht, was offen ist und was als
Nächstes kommt, damit die nächste Sitzung dort weitermacht, statt den Stand neu zu erarbeiten.

Abgrenzung:

- Ein Task ist ein Auftrag, den eine Sitzung ausführt.
- Ein Befund ist belegt und gilt auch dann noch, wenn das Vorhaben vorbei ist.
- Ein Todo ist eine Zeile.
- Ein Workbench-Thema beschreibt ein laufendes Vorhaben und darf verschwinden, wenn es fertig
  ist. Es wird nicht indiziert und nicht in die Wissensablage übernommen.

Wann:

- Das Briefing nennt deine aktiven Themen unter `work.workbench`. Passt eines zu deiner
  Arbeit, lies es, bevor du anfängst.
- Lege ein Thema an, sobald du einen Plan mit offenen Punkten hast, den du nicht in dieser
  Sitzung abarbeitest.
- Führe das Thema nach, sobald ein Punkt erledigt ist oder neu hinzukommt, nicht erst am Ende
  der Sitzung.
- Ist das Vorhaben fertig, schließe das Thema mit `k_playbook_doc_done`. Was davon bleiben
  soll, ist vorher als Befund oder Task geschrieben.

Wie:

- Anlegen mit `k_playbook_doc_create`, Sorte `workbench`; die Antwort nennt den Verweis.
  Inhalt schreiben mit Kephalaion `write`, lesen mit `k_playbook_doc_read`.
- Die Datei zeigt den aktuellen Stand, nicht die Geschichte: Du überschreibst sie ganz, mit
  `base_revision` aus dem letzten `read`, damit keine andere Sitzung still überschrieben wird.
- Frontmatter: `title` ist Pflicht, das Briefing zeigt ihn. Der erste Absatz sagt in zwei
  Sätzen, wo das Vorhaben steht. Danach frei und kurz: Stand, offene Punkte, nächster Schritt.
- Nenne in deiner Antwort das Thema, das du angelegt oder nachgeführt hast.
