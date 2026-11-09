---
title: Doku nachziehen
---

## Doku nachziehen

Hat die Änderung Code berührt, ziehst du Handdoku, `README.md` und `AGENTS.md` im selben
Arbeitsgang nach. Für die erzeugte Doku (`keph:/docs/generated/code/`,
`keph:/docs/generated/libs/`) reihst du einen Auftrag ein, wie
`k-playbook:/instructions/on-demand/doku-auftraege.md` es beschreibt — einen je Sitzung und
Erzeuger, nicht wenn es diese Doku noch nicht gibt. Ist unklar, welche Doku betroffen ist,
suchst du danach, bevor du die Änderung abschließt, und fragst im Zweifel den Nutzer. Dein
Abschluss nennt den Verweis auf den Auftrag oder sagt, warum nichts eingereiht wurde; ist
bewusst keine Doku-Änderung nötig, sagt er kurz, warum.

Den ausdrücklichen Prüfbericht über den Doku-Nachzug, samt `k-check` über die geänderten
Dateien, liefert `/k-enforcement`.
