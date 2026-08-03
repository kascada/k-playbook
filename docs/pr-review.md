# /k-pr-review

Kurzer Guide fuer den PR-Check-Flow.

`/k-pr-review` dient dazu, einen Pull Request strukturiert zu bewerten, ihn vorab mit den passenden Checks zu testen und danach je nach Lage entweder zu approven, zu mergen oder beim Anlegen eines lokalen Branches fuer weitergehende Tests zu helfen.

## Zweck

`/k-pr-review` soll einen GitHub-PR fuer das in `K-PLAYBOOK.yaml` konfigurierte Repo laden, knapp bewerten und danach eine sinnvolle Folgeaktion anbieten: Approval, expliziter Merge oder ein lokaler Validierungs-Branch fuer weitergehende Tests.

Perspektivisch sollte der Flow ausserdem eine Schnittstelle zu Jira oder Confluence bekommen, damit Bewertungen, Entscheidungen, Testergebnisse und Merge-Hinweise strukturiert ausserhalb des Chats abgelegt werden koennen.

## Aufrufe

- `/k-pr-review`
- `/k-pr-review 454`
- `/k-pr-review #454`
- `/k-pr-review https://github.com/<owner>/<repo>/pull/454`
- `/k-pr-review quick`
- `/k-pr-review 454 standard`
- `/k-pr-review #454 deep`

## Phasen

1. Repo und PR aufloesen
2. PR-Ueberblick zeigen
3. Bewertung in `quick`, `standard` oder `deep`
4. Empfehlung ableiten
5. Folgeaktion ausfuehren

## Mini-Schaubild

```text
/k-pr-review [selector] [mode]
        |
        v
PR laden -> Ueberblick -> Bewertung -> Empfehlung
                                 |
              +------------------+------------------+
              |                  |                  |
              v                  v                  v
         direkt approven    direkt mergen   branch erstellen und weiter testen
              |                  |                  |
              v                  v                  v
      GitHub-Approval      GitHub-Merge     lokaler Pruef-Branch + Tests
              \                  |                  /
               \                 |                 /
                +----------------+----------------+
                                 |
                                 v
                 Abschluss + Repo-Sauberkeitspruefung
```

## Bewertungsmodi

- `quick`: GitHub-Signale, Diff-Scope, Enforcement-Einschaetzung
- `standard`: `quick` plus `k-check --mode changed`
- `deep`: `standard` plus kleinste sinnvolle lokale Zusatzvalidierung

## Folgeaktionen

- `direkt annehmen`
  - Approval auf GitHub
  - optional auf ausdrueckliche Anfrage auch Merge
- `branch erstellen und weiter testen`
  - lokaler Validierungs-Branch vom PR-Head
  - erweiterter Testlauf
  - danach zurueck zum urspruenglichen PR
  - optional approven oder mergen
  - lokalen Pruef-Branch wieder aufraeumen
- `nichts weiter`

## Wichtige Regeln

- Kein automatischer Merge ohne ausdrueckliche User-Anfrage
- PR-Kommentar- und Approval-Texte immer per `--body-file`, nie als fragiler Inline-Mehrzeiler
- Lokale Pruef-Branches sind nicht merge-relevant
- Merge-relevant bleibt immer der urspruengliche PR
- Am Ende immer `git status --short --branch` pruefen

## Bereits praktisch getestet

Der Flow wurde bereits praktisch durchgespielt:

1. Offene PRs listen und PR auswaehlen
2. `standard`-Bewertung fuer echte PRs (`#441`, `#442`, `#454`)
3. Approval-Fall mit sauberem Self-Approval-Fehler
4. Branch-Flow fuer `#454`
5. Lokalen Validierungs-Branch anlegen
6. Erweiterte Tests auf dem Pruef-Branch ausfuehren
7. PR `#454` mergen
8. Lokale Branches aufraeumen und Repo-Zustand pruefen

## Offene Grenze

Wenn GitHub wegen Branch-Protection, fehlenden Rechten oder Self-Approval blockiert, soll der Command das klar melden und keinen stillen Workaround versuchen.

Eine sinnvolle naechste Erweiterung ist die Ablage der PR-Ergebnisse in Jira oder Confluence, z. B. als verlinkte Review-Notiz, Entscheidungsprotokoll oder Testzusammenfassung.
