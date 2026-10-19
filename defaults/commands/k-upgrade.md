# k-upgrade

Zieht Programm und Projekt auf den aktuellen Stand von k-playbook: das Binary, wenn das
laufende älter ist als die Installation, danach `AGENTS.md` (Anstoß, abgelöste Blöcke),
`CLAUDE.md`, die Verlinkung der Commands und Skills und die MCP-Einträge. Das tut der
Unterbefehl `k-playbook upgrade`; dieser Command ruft ihn und gibt seine Meldung wieder,
mehr nicht. Er nimmt kein Argument.

## Schritt 1 — Briefing

include: _shared/erster-schritt.md

## Schritt 2 — Nachziehen

Führe in der Shell aus:

    k-playbook upgrade

Gib die Ausgabe unverändert wieder — jede Zeile, auch Hinweise und Fehlermeldungen wie
„Download/Install nicht möglich“. Fasse nichts zusammen, lass nichts weg und führe nichts
anderes aus: keine weiteren Befehle, keine Änderungen an Dateien. Was ein Lauf nicht von
selbst nachziehen konnte, steht in der Ausgabe als Hinweis; nenne ihn, behebe ihn nicht.

## Schritt 3 — Schluss

Steht in der Ausgabe, dass das Programm gewechselt hat und die Sitzung neu zu starten ist,
schließe mit genau diesem Hinweis: Der laufende MCP-Server behält das alte Binary, bis der
Assistent ihn neu startet. Sonst endet der Command mit der Ausgabe.
