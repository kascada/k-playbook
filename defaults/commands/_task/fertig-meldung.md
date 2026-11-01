---
title: Fertig-Meldung
description: Modul für /k-task-run und /k-task-refine, eingebunden als include — wann der Command sich auf dem Rechner des Nutzers meldet (vor jeder Rückfrage, am Ende), was in title, body und sound steht, und dass die Antwort nicht ausgewertet wird.
---

`/k-task-run` und `/k-task-refine` laufen lange, und wer sie startet, schaut danach oft
nicht mehr auf den Bildschirm. Der Command meldet sich deshalb auf dem Rechner des Nutzers,
wenn er fertig ist oder etwas von ihm braucht — mit dem Werkzeug
`k_playbook_notify` des MCP-Servers `k-playbook`, `projectDir` ist `context.project.dir`.

### Wann

- **Vor jeder Rückfrage an den Nutzer**, unmittelbar bevor die Frage gestellt wird — nicht
  danach, und nicht erst, wenn die Antwort fehlt.
- **Am Ende des Commands**, vor der abschließenden Zusammenfassung, auch nach einem
  Abbruch.
- **Nicht bei Zwischenständen**: nicht je Etappe, nicht je Sub-Agent, nicht je Runde.

### Was

| Feld | Inhalt |
|---|---|
| `title` | `<projekt>: <task> – <was>` — `<projekt>` ist der letzte Pfadteil von `context.project.dir`; `<task>` der Dateistamm des Task-Dokuments (der letzte Pfadteil seines Verweises ohne `.md`), bei mehreren `<n> Tasks`; `<was>` ist `fertig`, `Rückfrage: <worum es geht, drei Worte>` oder `abgebrochen` |
| `body` | was passiert ist, ein bis zwei Sätze: Status, Review-Zahlen, der offene Punkt oder die Frage — höchstens 200 Zeichen, der Rest wird abgeschnitten |
| `sound` | `true` |

Beispiele:

```
title: omni-gw: 014-setup-tts – fertig
body:  Ausgeführt, Review: 0 kritisch, 2 wichtig. Task liegt in done/.

title: omni-gw: 015-integrate-tts – Rückfrage: kritische Befunde
body:  Review meldet 2 kritische Befunde. Beheben lassen, trotzdem abschließen oder anhalten?

title: k-playbook: 3 Tasks – fertig
body:  Review-Log an 3 Tasks angehängt, 7 Issues gefixt, 1 offen.
```

### Die Antwort

Die Antwort wird nicht ausgewertet und nicht dem Nutzer gezeigt. `ok: false` mit
`error.code: no_channel` heißt nur, dass dieser Rechner keinen Weg zum Nutzer hat — etwa ein
Devcontainer; weiter wie geplant, ohne Hinweis und ohne Wiederholung. Dasselbe gilt, wenn
das Werkzeug fehlt, weil ein älteres Programm läuft. Die Meldung ersetzt keine Ausgabe:
Frage und Zusammenfassung stehen weiterhin vollständig im Chat.
