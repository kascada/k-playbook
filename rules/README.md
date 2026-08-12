# Globale Regeln

Dieses Verzeichnis enthält projektübergreifende Regeln für k-playbook.

Globale Regeln liegen hier im Repo. Projektlokale Regeln liegen im jeweiligen Zielprojekt unter `<projekt>/k-playbook/enforcement/`.

## Dateien

- `review-authoring.md` - Regeln für neue oder geänderte Review-Rezepte.
- `docs-sync.md` - Regel: Code- und Doku-Änderungen synchron halten.
- `tool-install-scope.md` - Regel: `/k-install*`, host-lokale Tools und Projekt-venvs trennen.

## Globale Checks

Wiederverwendbare Checks liegen unter `../checks/` und werden über `../bin/k-check` ausgeführt. Projektlokale Checks bleiben im jeweiligen Zielprojekt unter `<projekt>/k-playbook/checks/`.

Globale Checks dürfen keine projektspezifischen Begriffe, Modellnamen oder Runtime-Dateien enthalten. Solche Regeln bleiben projektlokal; Test-Fixtures dürfen projektspezifische Begriffe nur als explizite Negativbeispiele markieren.

## Zusammenführung

Commands laden zuerst die globalen Regeln aus diesem Verzeichnis und danach projektlokale Regeln aus `<projekt>/k-playbook/enforcement/`, sofern dort Regeldateien liegen.

Projektlokale Regeln ergänzen die globalen Regeln. Sie ersetzen globale Regeln nur, wenn sie das explizit sagen.
