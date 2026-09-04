---
description: Start the local k-playbook GUI.
allowed-tools: [Bash, Read]
---

# k-gui

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook context` auf und lies die
Dateien aus `instructions`.
Alle Pfade dieses Commands stammen aus dieser Ausgabe; die `K-PLAYBOOK.yaml` wird
nicht selbst gelesen.

Die Ersteinrichtung eines Projekts läuft nicht über diesen Command, sondern über
`k-playbook` in der Shell. Erst deren dritter Schritt verlinkt die
Commands beim Assistenten — wer `/k-gui` aufrufen kann, hat also bereits eine
Installation. Ein fehlgeschlagener Context-Aufruf ist deshalb hier wie überall ein
Abbruchgrund.


Starte die lokale k-playbook Oberfläche.

Welches Projekt gemeint ist, leitet das Programm aus dem Arbeitsverzeichnis ab,
nicht aus seinem eigenen Ort. Ein Aufruf ist deshalb überall derselbe; jedes
Projekt bekommt seinen eigenen Hintergrunddienst.

Dieser Command nutzt bewusst nicht `make`.

## Ablauf

`k-playbook` ist einmal je Host oder DevContainer installiert — dasselbe Kommando,
das der erste Schritt schon für `context` aufgerufen hat. Es ist nichts zu suchen und
nichts zu raten; starte es ohne Argument im aktuellen Projekt:

```bash
k-playbook
```

## Hinweise

- Der Server läuft als Hintergrunddienst je Projekt; der Aufruf kehrt zurück, sobald der Browser offen ist. Ein zweiter Aufruf im selben Projekt startet nichts Neues, sondern öffnet nur den Browser.
- Beendet wird über den Knopf `Dienst beenden` in der Oberfläche oder `k-playbook stop`; ohne jede Anfrage beendet sich der Server nach 60 Minuten von selbst. Das Schließen des Browserfensters beendet ihn nicht.
- Die Oberfläche gibt die lokale URL aus, falls der Browser nicht automatisch startet.
- Dieser Command löscht keine Projektdateien. Der Start zieht nebenbei eine veraltete MCP-Registrierung und einen veralteten Anstoßblock in `AGENTS.md` nach und meldet, was er getan hat.
