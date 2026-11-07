---
title: Regeln anwenden
---

## Regeln anwenden

Das Briefing nennt unter `Regeln:` die Regeln dieses Projekts (`documents.rules`). Bevor du
eine Änderung abschließt, liest du die Regeln, die deinen Diff betreffen, mit
`k_playbook_doc_read` und wendest sie an — nicht erst am Ende der Sitzung. Welche betroffen
sind, sagt ihre Überschrift; im Zweifel liest du sie. Eine Regel mit der Herkunft `disabled`
gilt nicht.

Hat die Änderung Code berührt, gilt `k-playbook:/rules/docs-sync.md`: Handdoku, `README.md`
und `AGENTS.md` ziehst du im selben Arbeitsgang nach. Für die erzeugte Doku
(`keph:/docs/generated/code/`, `keph:/docs/generated/libs/`) reihst du einen Auftrag ein,
wie `k-playbook:/instructions/on-demand/doku-auftraege.md` es beschreibt — einen je Sitzung
und Erzeuger, nicht wenn es diese Doku noch nicht gibt. Dein Abschluss nennt den Verweis auf
den Auftrag oder sagt, warum nichts eingereiht wurde.

Den ausdrücklichen Prüfbericht über alle Regeln, samt `k-check` über die geänderten Dateien,
liefert `/k-enforcement`.
