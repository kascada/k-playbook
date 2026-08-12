---
description: Start the local k-playbook GUI.
allowed-tools: [Bash, Read]
---

# k-gui

## Erster Schritt

Wende `k-playbook/commands/_shared/context.md` an. Liegt die Ausgabe in dieser
Sitzung schon vor, verwende sie; sonst rufe `k-playbook/bin/k-playbook context`
auf und lies die Dateien aus `instructions`.

Anders als bei allen anderen Commands ist ein Fehlschlag hier kein Abbruchgrund:
`/k-gui` ist genau der Command, mit dem ein Projekt eingerichtet wird, also gibt es
die Konfiguration womöglich noch gar nicht. Melde das kurz und starte die
Oberfläche trotzdem.


Starte die lokale k-playbook Oberfläche.

Welches Projekt gemeint ist, leitet das Programm aus dem Arbeitsverzeichnis ab,
nicht aus seinem eigenen Ort. Deshalb tut es dasselbe, egal ob es host-weit oder
aus der Installation im Projekt gestartet wird.

Dieser Command nutzt bewusst nicht `make`.

## Ablauf

1. Löse `K_PLAYBOOK_BIN` auf. Nimm den ersten ausführbaren Treffer:

   - `k-playbook` aus dem `PATH`.
   - `~/.local/bin/k-playbook`.
   - `k-playbook/bin/k-playbook` im Projekt.

2. Wenn kein Kandidat ausführbar ist, brich mit einem klaren Hinweis ab:

```text
k-playbook nicht gefunden.
Erwartet wird die Installation unter k-playbook/ im Projekt.
```

3. Wenn das Binary vorhanden ist, starte es im aktuellen Projekt:

```bash
"$K_PLAYBOOK_BIN"
```

## Hinweise

- Die GUI läuft im Vordergrund, bis sie über den Browser-Button `Schließen`, Browser-Tab-Schließen, Heartbeat-Timeout oder `Ctrl+C` beendet wird.
- Die Oberfläche gibt die lokale URL aus, falls der Browser nicht automatisch startet.
- Dieser Command verändert keine Projektdateien.
- Der Start hält nebenbei die host-weite Kopie unter `~/.local/share/k-playbook/installation` aktuell. Nach dem ersten Mal genügt in jedem Projekt ein bloßes `k-playbook`.
