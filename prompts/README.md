# k-playbook Prompts

Dieses Verzeichnis enthaelt kopierbare Arbeitsauftraege fuer AI-Assistenten. Sie sind keine Slash-Commands und werden nicht nach OpenCode verlinkt.

Prompts sind fuer Situationen gedacht, in denen k-playbook noch nicht voll registriert ist oder ein Ablauf bewusst in einfachen Schritten gestartet werden soll.

## Installation

Die vereinfachte Multi-Project-Installation ist in [`../docs/multi-project-installation.md`](../docs/multi-project-installation.md) beschrieben.

Prompts in empfohlener Reihenfolge:

1. [`installation/01-host-opencode-registrieren.md`](./installation/01-host-opencode-registrieren.md) - Host-OpenCode nach dem Clone registrieren.
2. [`installation/03A-projekt-ohne-devcontainer-setup.md`](./installation/03A-projekt-ohne-devcontainer-setup.md) - Zielprojekt ohne DevContainer projektlokal einrichten.
3. [`installation/02-devcontainer-k-playbook-installieren.md`](./installation/02-devcontainer-k-playbook-installieren.md) - Zielprojekt auf dem Host fuer DevContainer vorbereiten.
4. [`installation/03B-devcontainer-projekt-setup.md`](./installation/03B-devcontainer-projekt-setup.md) - Im DevContainer k-playbook pruefen und projektlokal einrichten.
5. [`installation/04-smoke-test-neues-system.md`](./installation/04-smoke-test-neues-system.md) - Installation in einer Wegwerf-Umgebung wie auf einem neuen System testen.

Reproduzierbarer Testdurchlauf:

- [`installation/RUNBOOK-smoke-test-neues-system.md`](./installation/RUNBOOK-smoke-test-neues-system.md) - Testumgebung neu initialisieren und deterministisch pruefen.

## Nutzung

Oeffne die jeweilige Datei und gib den Inhalt als Auftrag an den AI-Assistenten. Der Assistent soll die referenzierte Doku lesen, Rueckfragen stellen, wenn Zielpfade fehlen, und danach die beschriebenen Schritte ausfuehren und pruefen.
