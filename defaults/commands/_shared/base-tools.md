---
title: Vor dem Aufruf eines Basis-Werkzeugs
description: Die Regel vor dem Aufruf eines Basis-Werkzeugs — context.baseTools aus dem Briefing prüfen, ein fehlendes Werkzeug benennen, nie selbst installieren. Nur für Commands, die Basis-Werkzeuge rufen.
---

**Prüfe `context.baseTools` aus dem Briefing, bevor du ein Basis-Werkzeug aufrufst.** Es
nennt die Werkzeuge, die k-playbook selbst aufruft — `bash`, `git`, `curl` oder `wget`,
`tar`, `python3`, `rg`. Sag nur etwas, wenn `context.baseTools.missing` nicht leer ist; fehlt
nichts, sag nichts. Nenne für jeden Eintrag, den ein Command in diesem Lauf tatsächlich
braucht, das Werkzeug, was `role` als Preis nennt, und `context.baseTools.installCommand` als
den Weg, es zu bekommen. Warne nicht allgemein und nenne keine Werkzeuge, die dieser Lauf
nicht berührt. Ein fehlendes Basis-Werkzeug **warnt, es bricht den Lauf nicht ab** — das ist
der Unterschied zu `gh.ready`, das ein PR-Review beendet. Wo es einen Ersatzweg gibt, nimm
ihn und sag, dass du es getan hast. Installiere nie selbst: Der Befehl wird genannt, nie
ausgeführt. Gemessen wird nur die Anwesenheit im `PATH`; eine Shell-Funktion oder ein Alias
ist unsichtbar — unter Claude Code kann `rg` als fehlend gemeldet werden, obwohl es
funktioniert; das ist bekannt und hingenommen. Ist `context.baseTools.present` falsch, konnte
die Matrix nicht gelesen werden; sag dann nichts zu Basis-Werkzeugen, statt zu raten.
