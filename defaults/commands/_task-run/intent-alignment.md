---
title: Intent-Alignment-Check
description: Modul für /k-task-run, vom Hauptkontext gelesen in Schritt 6, wenn alle Tasks nach done/ gewandert sind und der letzte Task einen Intent trägt — Zusammenfassungen sammeln, den Critic (intent-check.md) starten, das Ergebnis an den letzten Task anhängen und bei Abweichung warnen.
---

# Intent-Alignment-Check

Dieses Modul liest der Hauptkontext von `/k-task-run` in Schritt 6. Er hat den Intent des
letzten Tasks (wörtlich, aus Schritt 2), `TASK_REF_DONE` und `REVISION` (die `revision` aus der
Antwort von `k_playbook_doc_done`).

Dieser Schritt liest keinen Task. Den Intent hat der Hauptkontext aus Schritt 2; was
ausgeführt wurde, sind die Zusammenfassungen aus den Rückgaben der Sub-Agenten in diesem
Lauf. Für einen Task, der nach 5a (a) ohne Rückgabe in diesem Lauf abgeschlossen wurde, holt
der Hauptkontext die erste Zeile `^\*\*Zusammenfassung:\*\*` nach seiner Kennzeile mit
`k_playbook_doc_grep` (unter `TASK_REF_DONE`).

Der Hauptkontext löst Verweise im Intent nicht auf; der Critic liest sie. Starte einen
General-Purpose-Sub-Agenten (OpenCode: `general`, Claude Code: `general-purpose`) als Critic.
Er liest seinen Auftrag selbst: `k-playbook:/commands/_task-run/intent-check.md`. Du übergibst
ihm den Verweis, `projectDir` (`project.dir`), den Intent wörtlich und die
Zusammenfassungen der Sub-Agenten, je Task eine. Seine Rückgabe ist die Zeile
`| Alignment | Begründung |` (Ja, Teilweise oder Nein).

Das Ergebnis mit `k_playbook_doc_append` an den letzten Task anhängen, jetzt unter
`TASK_REF_DONE`, mit `since_revision` = der `revision` aus der Antwort von
`k_playbook_doc_done`:

```text
**Intent-Alignment:** <Ja / Teilweise / Nein> - <Begründung>
```

Ist das Alignment **nicht Ja**: vor der abschließenden Zusammenfassung eine klare Warnung:

```
WARNUNG: Intent nicht vollständig erreicht: <Begründung>
```

Ohne Intent entfällt dieser Schritt still.
