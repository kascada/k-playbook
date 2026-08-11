---
description: Start the local k-playbook installer GUI.
allowed-tools: [Bash, Read]
---

# k-gui

## Erster Schritt

Fuehre zuerst `commands/_shared/context.md` aus: rufe
`k-playbook/bin/k-playbook context` auf und lies die Dateien aus `instructions`.
Alle Pfade und Kataloge dieses Commands stammen aus dieser Ausgabe.


Starte die lokale k-playbook Installer-GUI.

Welches Projekt gemeint ist, leitet das Programm aus dem Arbeitsverzeichnis ab,
nicht aus seinem eigenen Ort. Deshalb tut es dasselbe, egal ob es host-weit oder
aus der Installation im Projekt gestartet wird.

Dieser Command nutzt bewusst nicht `make`.

## Ablauf

1. Loese `INSTALLER_BIN` auf. Nimm den ersten ausfuehrbaren Treffer:

   - `k-playbook` aus dem `PATH`.
   - `~/.local/bin/k-playbook`.
   - `k-playbook/bin/k-playbook` im Projekt.

2. Wenn kein Kandidat ausfuehrbar ist, brich mit einem klaren Hinweis ab:

```text
k-playbook nicht gefunden.
Erwartet wird die Installation unter k-playbook/ im Projekt.
```

3. Wenn das Binary vorhanden ist, starte es im aktuellen Projekt:

```bash
"$INSTALLER_BIN"
```

## Hinweise

- Die GUI laeuft im Vordergrund, bis sie ueber den Browser-Button `Schliessen`, Browser-Tab-Schliessen, Heartbeat-Timeout oder `Ctrl+C` beendet wird.
- Der Installer gibt die lokale URL aus, falls der Browser nicht automatisch startet.
- Dieser Command veraendert keine Projektdateien.
- Der Start haelt nebenbei die host-weite Kopie unter `~/.local/share/k-playbook/installation` aktuell. Nach dem ersten Mal genuegt in jedem Projekt ein blosses `k-playbook`.
