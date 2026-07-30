# AGENTS.md - k-playbook

Diese Datei wird von OpenCode als Projekt-Kontext geladen.

## Projektrolle

Dieses Repo ist die globale k-playbook-Basisinstallation. Es enthaelt die wiederverwendbaren Commands, Skills, Playbooks, globalen Regeln, Reviews, Checks und Installationsskripte fuer Zielprojekte.

Es ist selbst kein normales Zielprojekt mit projektlokaler `K-PLAYBOOK.MD`-Konfiguration.

## Wichtige Memory-Regel

Die k-playbook-Regeln gelten in diesem Repo nicht automatisch direkt als projektlokale Zielprojekt-Regeln. `global/rules/` ist hier Teil des Produkts bzw. der Basisinstallation.

Wende Enforcement-Regeln in diesem Repo nur dann als aktive Arbeitsregel an, wenn der Nutzer das ausdruecklich verlangt, z. B. mit `/k-enforcement`, "Regeln beruecksichtigen" oder einem konkreten Review-/Check-Auftrag.

Bei Arbeiten an Commands, Regeln, Reviews, Checks, Skills oder Docs gelten die dokumentierten Produktvertraege dieses Repos. Unterscheide dabei zwischen dem Pflegen der Regeldefinitionen und dem Anwenden dieser Regeln auf ein Zielprojekt.

## Docs Zuerst

Fuer strukturelle Fragen zuerst `docs/README.md` lesen. Wichtige Einstiegspunkte sind:

- `docs/handbuch.md` fuer Zweck, Konzepte und Standardablaeufe.
- `docs/commands.md` fuer Slash-Command-Zustaendigkeiten.
- `docs/installation.md` fuer Host-Installation und OpenCode-Setup.
- `installer/docs/architecture.md` fuer Arbeiten am Go-Installer, Browser-GUI, Web-API, Docs-Viewer und Git-Pull-Flow.
- `docs/reviews-and-results.md` fuer Review-, Results- und Remediation-Flows.

Bei konkreten Code- oder Doku-Aenderungen danach gezielt die betroffenen Dateien lesen.
