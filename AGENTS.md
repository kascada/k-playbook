# AGENTS.md - k-playbook

Diese Datei wird von OpenCode als Projekt-Kontext geladen.

## Projektrolle

Dieses Repo ist das Quell- und Entwickler-Repo fuer k-playbook. Es enthaelt zwei Dinge:

- die **Payload**, die in Zielprojekte installiert wird: Commands, Skills, Regeln, Reviews, Checks, Skripte.
- den **Installer**, der diese Payload in das Unterverzeichnis eines Zielprojekts schreibt.

k-playbook wird pro Projekt in ein Unterverzeichnis installiert, konventionell `<projekt>/k-playbook/`. Die Payload landet dort unter `_dist/` und wird bei jedem Update vollstaendig ersetzt. Es gibt keine zentrale Basisinstallation und keinen festen Hostpfad mehr.

Dieses Repo ist selbst kein Zielprojekt und hat keine eigene `K-PLAYBOOK.yaml`.

## Wichtige Memory-Regel

Die k-playbook-Regeln gelten in diesem Repo nicht automatisch als projektlokale Zielprojekt-Regeln. `global/rules/` ist hier Payload, nicht aktive Arbeitsregel.

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
