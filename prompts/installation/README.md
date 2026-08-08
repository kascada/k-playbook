# Installations-Prompts (veraltet)

> **Diese Prompts beschreiben das alte zentrale Installationsmodell** mit Basisinstallation
> unter `~/dev/k-playbook`, globaler OpenCode-/Claude-Registrierung und DevContainer-Bind-Mount.
>
> Dieses Modell ist abgeloest. k-playbook wird heute pro Projekt in ein Unterverzeichnis
> installiert; die Payload steckt im Installer-Binary.

Der aktuelle Weg ist kurz genug, um keinen Prompt zu brauchen:

```bash
cd /pfad/zum/projekt
k-playbook-installer init
```

Bestehende Projekte:

```bash
k-playbook-installer migrate --dry-run
k-playbook-installer migrate
```

Siehe [`../../README.md`](../../README.md) und
[`../../docs/k-playbook-format.md`](../../docs/k-playbook-format.md).

Die Dateien in diesem Verzeichnis bleiben vorerst als Referenz fuer den Umbau der GUI und
der DevContainer-Integration liegen, die beide noch auf dem alten Modell stehen. Sie sind
nicht mehr auszufuehren.
