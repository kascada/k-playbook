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

Der Installer ist ein host-weites Binary, kein Bestandteil der projektlokalen
Installation unter `_dist/`. Er wird deshalb ueber den `PATH` aufgeloest, nicht
ueber einen Pfad im Projekt.

Dieser Command nutzt bewusst nicht `make`.

## Ablauf

1. Loese `INSTALLER_BIN` auf. Nimm den ersten ausfuehrbaren Treffer:

   - `k-playbook-installer` aus dem `PATH`.
   - `~/.local/bin/k-playbook-installer`.

2. Wenn kein Kandidat ausfuehrbar ist, brich mit einem klaren Hinweis ab:

```text
Installer-Binary nicht gefunden.
Installiere es einmalig pro Host, danach ist es fuer alle Projekte verfuegbar.
```

3. Wenn das Binary vorhanden ist, starte es im aktuellen Projekt:

```bash
"$INSTALLER_BIN"
```

## Hinweise

- Die GUI laeuft im Vordergrund, bis sie ueber den Browser-Button `Schliessen`, Browser-Tab-Schliessen, Heartbeat-Timeout oder `Ctrl+C` beendet wird.
- Der Installer gibt die lokale URL aus, falls der Browser nicht automatisch startet.
- Dieser Command veraendert keine Projektdateien.
