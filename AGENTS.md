# AGENTS.md - k-playbook

Diese Datei wird von OpenCode als Projekt-Kontext geladen.

## Projektrolle

Dieses Repo ist das Quell- und Entwickler-Repo fuer k-playbook. Es enthaelt zwei Dinge:

- den **Inhalt**, der in Zielprojekte geklont wird: Commands, Skills, Regeln, Reviews, Checks, Skripte.
- das **Werkzeug** unter `installer/`, das ein Projekt einrichtet.

k-playbook wird pro Projekt geklont, nach `<projekt>/k-playbook/`. Dieses Verzeichnis wird bei jedem Update vollstaendig ersetzt. Daneben liegen `K-PLAYBOOK.yaml` als Anker und `k-playbook-local/` mit allem Projekteigenen. Es gibt keine zentrale Basisinstallation und keinen festen Hostpfad.

Dieses Repo wird wie jede andere Installation behandelt: `~/dev/k-playbook/K-PLAYBOOK.yaml` ist der Anker, die Installation liegt unter `~/dev/k-playbook/k-playbook/`. Dass die dort installierte Version eine andere ist als der Arbeitsstand daneben, wird bewusst in Kauf genommen. Beides ist lokal und gitignored.

## Wichtige Memory-Regel

Die k-playbook-Regeln gelten in diesem Repo nicht automatisch als projekteigene Zielprojekt-Regeln. `rules/` ist hier Inhalt, nicht aktive Arbeitsregel.

Wende Enforcement-Regeln in diesem Repo nur dann als aktive Arbeitsregel an, wenn der Nutzer das ausdruecklich verlangt, z. B. mit `/k-enforcement`, "Regeln beruecksichtigen" oder einem konkreten Review-/Check-Auftrag.

Bei Arbeiten an Commands, Regeln, Reviews, Checks, Skills oder Docs gelten die dokumentierten Produktvertraege dieses Repos. Unterscheide dabei zwischen dem Pflegen der Regeldefinitionen und dem Anwenden dieser Regeln auf ein Zielprojekt.

## Umstellung laeuft

Das Modell wurde auf die projektlokale Installation umgestellt. `docs/umbau.md` ist die Arbeitsdatei dazu: sie haelt fest, was festgelegt ist, was ersatzlos entfallen ist und was in Commands und Skills noch nachzuziehen bleibt. Vor Aenderungen an Commands oder Skills dort nachsehen.

## Docs Zuerst

Fuer strukturelle Fragen zuerst `docs/README.md` lesen. Wichtige Einstiegspunkte sind:

- `docs/k-playbook-format.md` fuer den Kontrakt: Anker, Verzeichnisaufteilung, Overlay-Regeln.
- `docs/handbuch.md` fuer Zweck, Konzepte und Standardablaeufe.
- `docs/commands.md` fuer Slash-Command-Zustaendigkeiten.
- `docs/installation.md` fuer Clone, Einrichtungsschritte und Security-Tools.
- `docs/reviews-and-results.md` fuer Review-, Results- und Remediation-Flows.
- `installer/docs/architecture.md` fuer Arbeiten am Go-Werkzeug, an der Oberflaeche und der Web-API.

Bei konkreten Code- oder Doku-Aenderungen danach gezielt die betroffenen Dateien lesen.
