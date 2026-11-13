---
description: Start the local k-playbook GUI.
# model: github-copilot/gpt-5.5
allowed-tools: [Bash, Read, mcp__k-playbook__k_playbook_briefing]
---

# k-gui

## Erster Schritt

Liegt das Briefing in dieser Sitzung vor, nimm es. Sonst rufe `k_playbook_briefing` mit
`projectDir` = Arbeitsverzeichnis, ersatzweise `k-playbook briefing --json`. Ist k-playbook
nicht installiert, brich ab und melde es. Die Regeln zum Briefing stehen in seinem Text der
immer geltenden Anweisungen (`documents.always.content`).

Die Ersteinrichtung eines Projekts läuft nicht über diesen Command, sondern über
`k-playbook` in der Shell. Erst deren dritter Schritt verlinkt die
Commands beim Assistenten — wer `/k-gui` aufrufen kann, hat also bereits eine
Installation. Ein fehlgeschlagenes Briefing ist deshalb hier wie überall ein
Abbruchgrund.


Starte die lokale k-playbook Oberfläche.

Welches Projekt gemeint ist, leitet das Programm aus dem Arbeitsverzeichnis ab,
nicht aus seinem eigenen Ort. Ein Aufruf ist deshalb überall derselbe; jedes
Projekt bekommt seinen eigenen Hintergrunddienst.

Dieser Command nutzt bewusst nicht `make`.

## Schritt 1 — Oberfläche starten

`k-playbook` ist einmal je Host oder DevContainer installiert — dasselbe Kommando,
das der erste Schritt schon für das Briefing aufgerufen hat. Es ist nichts zu suchen und
nichts zu raten; starte es ohne Argument im aktuellen Projekt:

```bash
k-playbook
```

- Der Server läuft als Hintergrunddienst je Projekt; der Aufruf kehrt zurück, sobald der Browser offen ist. Ein zweiter Aufruf im selben Projekt startet nichts Neues, sondern öffnet nur den Browser.
- Beendet wird über den Knopf `Dienst beenden` in der Oberfläche oder `k-playbook stop`; ohne jede Anfrage beendet sich der Server nach 60 Minuten von selbst. Das Schließen des Browserfensters beendet ihn nicht.
- Die Oberfläche gibt die lokale URL aus, falls der Browser nicht automatisch startet.
- Dieser Command löscht keine Projektdateien. Der Start zieht nebenbei eine veraltete MCP-Registrierung und einen veralteten Anstoßblock in `AGENTS.md` nach und meldet, was er getan hat.
- Ist das laufende Programm älter als die `VERSION` der Installation, sagt das eine Zeile im Terminal, und die Oberfläche bietet „Programm aktualisieren" an. Erst der Klick installiert über `k-playbook/bin/install` und startet den Dienst daraus neu — die Seite landet dabei von selbst beim neuen Dienst, unter neuer Adresse. Der Start selbst lädt nie etwas.

Folge-Command: keiner. Mit dem geöffneten Browser ist der Command abgeschlossen;
alles Weitere geschieht in der Oberfläche.
