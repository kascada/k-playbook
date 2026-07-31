---
description: Start the local k-playbook installer GUI from the installed binary at ~/.local/bin/k-playbook-installer.
allowed-tools: [Bash, Read]
---

# k-gui

Starte die lokale k-playbook Installer-GUI ueber das installierte Binary.

Dieser Command nutzt bewusst nicht `make`, sondern ruft direkt auf:

```bash
~/.local/bin/k-playbook-installer
```

## Ablauf

1. Pruefe, ob `~/.local/bin/k-playbook-installer` existiert und ausfuehrbar ist.
2. Wenn das Binary fehlt oder nicht ausfuehrbar ist, brich mit einem klaren Hinweis ab:

```text
Installer-Binary fehlt. Installiere es zuerst im k-playbook-Repo mit:
make install
```

3. Wenn das Binary vorhanden ist, starte es direkt:

```bash
~/.local/bin/k-playbook-installer
```

## Hinweise

- Die GUI laeuft im Vordergrund, bis sie ueber den Browser-Button `Schliessen`, Browser-Tab-Schliessen, Heartbeat-Timeout oder `Ctrl+C` beendet wird.
- Der Installer gibt die lokale URL aus, falls der Browser nicht automatisch startet.
- Dieser Command veraendert keine Projektdateien.
