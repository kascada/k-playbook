---
title: Sub-Agenten starten
description: Baustein für /k-task-run und /k-task-refine und ihre Module, eingebunden als include — welcher Sub-Agent-Typ je Client, womit sein Auftrag beginnt, wie er in Claude Code die MCP-Werkzeuge lädt und auf welchen Wegen er Verweise liest.
---

**Sub-Agenten.** Ein Sub-Agent ist ein General-Purpose-Sub-Agent (OpenCode: `general`, Claude
Code: `general-purpose`). Er kennt das Gespräch nicht und holt das Briefing nicht von selbst;
der Auftrag nennt ihm, was er braucht, und Texte als Verweis, nicht als Kopie.

- Der Auftrag beginnt mit: Führe zuerst `k-playbook briefing` aus und lies die Ausgabe. Das
  entfällt nur bei reinem Nachschlagen, das nichts schreibt, bewertet oder ablegt.
- In Claude Code sind die Werkzeuge `mcp__k-playbook__*` im Sub-Agenten oft verzögert; er lädt
  sie vor dem ersten Aufruf mit der Werkzeugsuche.
- **Lesewege für Verweise:** `k-playbook:` und `keph:` mit `k_playbook_doc_read` (`ref` = der
  Verweis, `projectDir` = `project.dir` des Auftraggebers), ersatzweise in der Shell
  `k-playbook read <verweis>`; `project:/<pfad>` direkt von der Platte, ab `projectDir`.
- Der Auftrag nennt `projectDir` immer, auch wenn die Ausführungswurzel woanders liegt.
