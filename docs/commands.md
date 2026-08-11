# Commands

Kompakter Index der Slash-Commands. Detailablaeufe stehen in eigenen Themenseiten; diese
Seite dupliziert sie nicht.

Mitgelieferte Commands, Skills, Regeln, Review-Rezepte und Checks liegen unter
`k-playbook/`. Projekteigene Regeln, Reviews, Checks, Tasks und Ergebnisse liegen unter
`k-playbook-local/`. Beide Orte ergeben sich aus der Lage der `K-PLAYBOOK.yaml`; es gibt
keine konfigurierten Pfade mehr.

## Detailseiten

| Thema | Detailseite |
|---|---|
| PR-Review | [`pr-review.md`](./pr-review.md) |
| Code-Review-Flow | [`code-review.md`](./code-review.md) |
| Task-Flow | [`task-flow.md`](./task-flow.md) |
| Review-, Results- und Remediation-Artefakte | [`reviews-and-results.md`](./reviews-and-results.md) |
| Installation | [`installation.md`](./installation.md) |
| Projektkonfiguration | [`k-playbook-format.md`](./k-playbook-format.md) |

## Uebersicht

Neue Commands werden erst sichtbar, nachdem die Verlinkung steht und der Assistent neu
gestartet wurde.

| Command | Zweck | Detail |
|---|---|---|
| **Projekt** | | |
| `/k-gui` | Oberflaeche starten | fuehrt durch Konfiguration, projekteigene Struktur und Assistenten-Verlinkung |
| `/k-status` | read-only Zustandsbericht fuer das Projekt | repariert nichts |
| **Tools** | | |
| `/k-install-codeql` | CodeQL CLI installieren/pruefen, optional lokale DBs analysieren | aendert `K-PLAYBOOK.yaml` nicht |
| `/k-setup-codeql` | CodeQL-Entscheidung im Projekt festhalten | schreibt nur `tools.codeql` |
| **Docs** | | |
| `/k-code2docs` | semantische Projekt-Doku erzeugen und fuer AI-Sessions registrieren | schreibt nach `k-playbook-local/docs/`, dazu `AGENTS.md` und `opencode.json` |
| `/k-tools-scan` | Library-/Tool-Doku nach `/k-code2docs` ergaenzen | erzeugt je ausgewaehltem Tool eine Pitfall-Datei unter `k-playbook-local/docs/libs/` |
| **Code-Review** | | |
| `/k-pr-review` | GitHub-PRs laden, bewerten und optional approven, mergen oder lokal validieren | [`pr-review.md`](./pr-review.md) |
| `/k-review` | Review-Rezepte ausfuehren | [`code-review.md`](./code-review.md) |
| `/k-results` | vorhandene Ergebnisse projektweit priorisieren | [`code-review.md`](./code-review.md) |
| `/k-remediation` | Findings buendeln und in Tasks oder Fixes ueberfuehren | [`code-review.md`](./code-review.md) |
| **Task-Flow** | | |
| `/k-task-create` | Task-Datei aus dem Gespraechskontext erzeugen | [`task-flow.md`](./task-flow.md) |
| `/k-review-loop` | Task-Dateien vor der Ausfuehrung per Critic/Editor-Dialog pruefen | [`task-flow.md`](./task-flow.md) |
| `/k-run` | Task-Dateien sequenziell ausfuehren | [`task-flow.md`](./task-flow.md) |
| `/k-todo` | `k-playbook-local/TODO.md` anzeigen oder ergaenzen | |
| **Hilfen** | | |
| `/k-enforcement` | expliziter Check gegen die effektive Regelmenge | read-only Bericht; Fixes nur nach Freigabe |
| `/k-test-check` | Tests ausfuehren und Fehlerursachen diagnostizieren | startet bewusst Tests, nicht nur Statuschecks |
| `/k-verlauf` | alte AI-Verlaeufe durchsuchen | read-only |
| `/k-vscode-project-color` | VS-Code-Fensterfarbe und -Titel pro Projekt setzen | schreibt `.vscode/settings.json` |

Einen Command `/k-install-security-tools` gibt es nicht mehr. Status und
Installationsbefehl kommen aus der Oberflaeche, alles Weitere kann
`k-playbook/scripts/install-security-tools.sh` selbst — siehe
[`installation.md`](./installation.md#security-tools).

## Review-Flow

Die Code-Review-Familie ist bewusst gestuft:

1. `/k-pr-review` bewertet einen konkreten Pull Request und bleibt standardmaessig read-only.
2. `/k-review <name>` fuehrt ein Rezept aus und erzeugt je nach Rezept interaktive
   Aenderungsvorschlaege oder Report-Artefakte.
3. `/k-results` priorisiert vorhandene Ergebnisfamilien zu einer projektweiten Summary.
4. `/k-remediation <result>` plant die Abarbeitung der Findings.

Wenn `/k-remediation` Tasks erzeugt, gehoeren sie in den normalen Task-Flow: erst
`/k-review-loop`, dann `/k-run`.

## Task-Flow

```text
/k-task-create
/k-review-loop
/k-run
```

Tasks entstehen direkt aus dem Gespraech oder aus `/k-remediation`. In beiden Faellen
werden sie vor der Ausfuehrung gegengeprueft.

## Wo Commands ihre Ziele finden

Kein Command liest oder raet einen Pfad. Alles leitet sich aus dem Ort der
`K-PLAYBOOK.yaml` ab:

| Command | schreibt nach |
|---|---|
| `/k-task-create`, `/k-run` | `k-playbook-local/tasks/`, erledigt nach `tasks/done/` |
| `/k-todo` | `k-playbook-local/TODO.md` |
| `/k-review`, `/k-results` | `k-playbook-local/results/` |
| `/k-setup-codeql` | `K-PLAYBOOK.yaml`, nur `tools.codeql` |
| `/k-code2docs`, `/k-tools-scan` | `k-playbook-local/docs/`, Tool-Steckbriefe unter `libs/` |

Gelesen wird zusaetzlich aus `k-playbook/` — Regeln, Rezepte, Checks und Skripte.
Geschrieben wird dorthin nie.

## Der aufgeloeste Arbeitsstand

Kein Command rechnet selbst aus, was gilt. Das macht das Werkzeug:

```bash
k-playbook/bin/k-playbook context
```

Die JSON-Ausgabe nennt die aufgeloesten Verzeichnisse, die Instruktionsdateien in
Lesereihenfolge, die Remediation-Policy, die Guidelines und die drei Kataloge —
mitgeliefert und projekteigen bereits zusammengefuehrt:

```json
{
  "instructions": [
    "/projekt/k-playbook/k-playbook.md",
    "/projekt/k-playbook-local/k-playbook.md"
  ],
  "catalogs": {
    "rules": [
      { "name": "codeql.md",             "key": "codeql",            "origin": "dist" },
      { "name": "docs-sync.md",          "key": "docs-sync",         "origin": "override" },
      { "name": "my-api-rules.md",       "key": "my-api-rules",      "origin": "local" },
      { "name": "tool-install-scope.md", "key": "tool-install-scope","origin": "override",
        "disabled": true }
    ]
  }
}
```

`origin` ist `dist`, `local` oder `override`. `disabled` steht dort, wo die projekteigene
Datei leer ist — das ist der Weg, einen mitgelieferten Eintrag abzuschalten.

`/k-review`, `/k-enforcement` und `k-check` arbeiten auf dieser Menge und weisen sie vor
der Arbeit aus. Die Regeln im Detail stehen in
[`k-playbook-format.md`](./k-playbook-format.md#mitgeliefertes-und-projekteigenes-zusammenfassen).

## Status

`/k-status` ist read-only und prueft:

- ob `K-PLAYBOOK.yaml` existiert, gueltig ist und `schema_version: 3` traegt,
- ob `k-playbook-local/` vollstaendig ist,
- ob die Symlinks fuer Claude Code, OpenCode und Cursor stimmen,
- ob `project.repo_root` auf ein Repository zeigt und wie sauber es ist,
- welche Tasks offen sind und ob Security-Tools fehlen.

Der Command repariert nichts. Fuer Reparaturen ist `/k-gui` zustaendig.

## k-check

`k-playbook/bin/k-check` ist kein Slash-Command, sondern ein CLI-Runner fuer die
effektive Check-Menge:

```bash
k-playbook/bin/k-check --mode changed
k-playbook/bin/k-check --mode baseline
```

Die stabile Check-Schnittstelle ist `.sh`. Ein Check darf Python oder anderes intern
verwenden, muss aber genau eine Statuszeile `K_CHECK_STATUS=ok|skip|fail` und optional
`K_CHECK_REASON=<text>` schreiben. Details stehen in
[`../checks/README.md`](../checks/README.md).
