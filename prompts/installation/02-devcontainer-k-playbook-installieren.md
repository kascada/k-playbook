# Prompt: DevContainer Fuer k-playbook Vorbereiten

Du arbeitest auf dem Host, nicht im DevContainer.

Lies zuerst `~/dev/k-playbook/docs/installation.md`, insbesondere Abschnitt `DevContainer-Pfadvertrag`.

Frage den User, in welchem Zielprojekt der DevContainer fuer k-playbook vorbereitet werden soll. Akzeptiere einen absoluten Pfad oder einen Pfad relativ zum aktuellen Arbeitsverzeichnis.

Fuehre danach aus:

1. Pruefe, ob das Zielprojekt existiert.
2. Pruefe, ob `<zielprojekt>/.devcontainer/devcontainer.json` existiert. Wenn nicht, stoppe und erklaere, dass zuerst ein DevContainer im Zielprojekt angelegt werden muss.
3. Fuehre `~/dev/k-playbook/scripts/install-devcontainer-k-playbook.sh <zielprojekt>` aus.
4. Pruefe, ob `<zielprojekt>/.devcontainer/setup-k-playbook.sh` existiert.
5. Pruefe, ob `<zielprojekt>/.devcontainer/devcontainer.json` jetzt den k-playbook-Bind-Mount und die Setup-Hooks enthaelt.

Beachte:

- Veraendere keine anderen Projektdateien.
- Starte keinen DevContainer-Rebuild selbst, wenn der User das nicht explizit verlangt.
- Wenn das Zielprojekt uncommitted Aenderungen hat, fahre fort, aber mache deutlich, welche Dateien durch das Script geaendert wurden.

Berichte am Ende knapp, welche Dateien angepasst wurden und dass der User den DevContainer jetzt neu bauen oder neu starten soll.
