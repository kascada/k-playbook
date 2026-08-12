# Globale Regeln

Dieses Verzeichnis enthaelt projektuebergreifende Regeln fuer k-playbook.

Globale Regeln liegen hier im Repo. Projektlokale Regeln liegen im jeweiligen Zielprojekt unter `<projekt>/k-playbook/enforcement/`.

## Dateien

- `review-authoring.md` - Regeln fuer neue oder geaenderte Review-Rezepte.
- `docs-sync.md` - Regel: Code- und Doku-Aenderungen synchron halten.
- `tool-install-scope.md` - Regel: `/k-install*`, host-lokale Tools und Projekt-venvs trennen.

## Globale Checks

Wiederverwendbare Checks liegen unter `../checks/` und werden ueber `../bin/k-check` ausgefuehrt. Projektlokale Checks bleiben im jeweiligen Zielprojekt unter `<projekt>/k-playbook/checks/`.

Globale Checks duerfen keine projektspezifischen Begriffe, Modellnamen oder Runtime-Dateien enthalten. Solche Regeln bleiben projektlokal; Test-Fixtures duerfen projektspezifische Begriffe nur als explizite Negativbeispiele markieren.

## Zusammenfuehrung

Commands laden zuerst die globalen Regeln aus diesem Verzeichnis und danach projektlokale Regeln aus `<projekt>/k-playbook/enforcement/`, sofern dort Regeldateien liegen.

Projektlokale Regeln ergaenzen die globalen Regeln. Sie ersetzen globale Regeln nur, wenn sie das explizit sagen.
