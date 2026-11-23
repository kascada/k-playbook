---
title: Ohne Task-Refine
description: Modul für /k-task-run, vom Hauptkontext gelesen in Schritt 3, wenn mindestens ein Task kein Review-Log trägt (und nicht der Modus refine ihn prüft) — die Rückfrage trotzdem ausführen, abbrechen oder zuerst reviewen, und warum sie eine Rückfrage und kein Abbruch ist.
---

# Ohne Task-Refine

Dieses Modul liest der Hauptkontext von `/k-task-run` in Schritt 3, wenn mindestens ein
gesammelter Task kein `## Review-Log` trägt — im Modus `refine` nur für die Tasks, die dort auf
die Rückfrage zurückfielen, mit dem Grund je Task. Er hat die betroffenen Tasks.

Die Fertig-Meldung (`Rückfrage: ohne Task-Refine`) ist schon gesendet. Nenne die betroffenen
Tasks in **einer** Nachricht und frage:

```
Ohne Task-Refine:
  - 014-setup-tts.md
  - 015-integrate-tts.md

Diese Tasks wurden nicht mit /k-task-refine gegengelesen.
Trotzdem ausführen? (ja / nein / zuerst reviewen)
```

- `ja` — weiter mit Schritt 4.
- `nein` — abbrechen, nichts ausführen.
- `zuerst reviewen` — abbrechen und wörtlich `/k-task-refine <TASK_REF>` je Task nennen,
  danach `/k-task-run` erneut starten. Den Task-Refine-Command nicht selbst aufrufen.

Ein fehlender Block heißt: nie durch Task-Refine gegangen. Das bleibt eine Rückfrage und kein
Abbruch: Ein von Hand geschriebener Task kann sachlich in Ordnung sein — das entscheidet der
Nutzer.
