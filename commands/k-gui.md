---
description: Start the local k-playbook installer GUI from the canonical repo-local launcher.
allowed-tools: [Bash, Read]
---

# k-gui

Starte die lokale k-playbook Installer-GUI ueber den kanonischen repo-lokalen Launcher.

Dieser Command nutzt bewusst nicht `make`, sondern ruft direkt auf:

```bash
~/dev/k-playbook/bin/k-playbook-installer
```

## Ablauf

1. Pruefe, ob `~/dev/k-playbook/bin/k-playbook-installer` existiert und ausfuehrbar ist.
2. Wenn das Binary fehlt oder nicht ausfuehrbar ist, brich mit einem klaren Hinweis ab:

```text
Installer-Binary fehlt. Installiere es zuerst im k-playbook-Repo mit:
make install
```

3. Wenn das Binary vorhanden ist, starte es direkt:

```bash
~/dev/k-playbook/bin/k-playbook-installer
```

## Hinweise

- Die GUI laeuft im Vordergrund, bis sie ueber den Browser-Button `Schliessen`, Browser-Tab-Schliessen, Heartbeat-Timeout oder `Ctrl+C` beendet wird.
- Der Installer gibt die lokale URL aus, falls der Browser nicht automatisch startet.
- Dieser Command veraendert keine Projektdateien.
