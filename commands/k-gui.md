---
description: Start the local k-playbook GUI.
allowed-tools: [Bash, Read]
---

# k-gui

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.
Alle Pfade dieses Commands stammen aus dieser Ausgabe; die `K-PLAYBOOK.yaml` wird
nicht selbst gelesen.

Die Ersteinrichtung eines Projekts läuft nicht über diesen Command, sondern über
`k-playbook/bin/k-playbook` in der Shell. Erst deren dritter Schritt verlinkt die
Commands beim Assistenten — wer `/k-gui` aufrufen kann, hat also bereits eine
Installation. Ein fehlgeschlagener Context-Aufruf ist deshalb hier wie überall ein
Abbruchgrund.


Starte die lokale k-playbook Oberfläche.

Welches Projekt gemeint ist, leitet das Programm aus dem Arbeitsverzeichnis ab,
nicht aus seinem eigenen Ort. Deshalb tut es dasselbe, egal ob es host-weit oder
aus der Installation im Projekt gestartet wird.

Dieser Command nutzt bewusst nicht `make`.

## Ablauf

Der Wrapper liegt unter `<playbook.dir>/bin/k-playbook` — derselbe, den der erste
Schritt schon für `context` aufgerufen hat. Es ist nichts zu suchen und nichts zu
raten; starte ihn ohne Argument im aktuellen Projekt:

```bash
<playbook.dir>/bin/k-playbook
```

## Hinweise

- Die GUI läuft im Vordergrund, bis sie über den Browser-Button `Schließen`, Browser-Tab-Schließen, Heartbeat-Timeout oder `Ctrl+C` beendet wird.
- Die Oberfläche gibt die lokale URL aus, falls der Browser nicht automatisch startet.
- Dieser Command verändert keine Projektdateien.
- Der Start hält nebenbei die host-weite Kopie unter `~/.local/share/k-playbook/installation` aktuell. Nach dem ersten Mal genügt in jedem Projekt ein bloßes `k-playbook`.
