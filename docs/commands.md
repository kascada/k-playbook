# Commands

Kompakter Index der Slash-Commands. Detailabläufe stehen in eigenen Themenseiten; diese
Seite dupliziert sie nicht.

Mitgelieferte Commands, Skills, Regeln, Review-Rezepte und Checks liegen unter
`k-playbook/`. Unter `k-playbook-local/` liegt das Projekteigene — dieselben fünf Sorten
plus Tasks und Ergebnisse. Beide Orte ergeben sich aus der Lage der `K-PLAYBOOK.yaml`; es
gibt keine konfigurierten Pfade mehr.

Ein projekteigener Command mit dem Namen eines mitgelieferten **ersetzt** diesen; ein
leerer schaltet ihn ab. Diese Seite listet die mitgelieferten — was in einem konkreten
Projekt tatsächlich gilt, zeigt die Oberfläche im Assistenten-Block.

## Detailseiten

| Thema | Detailseite |
|---|---|
| PR-Review | [`pr-review.md`](./pr-review.md) |
| Code-Review-Flow | [`code-review.md`](./code-review.md) |
| Task-Flow | [`task-flow.md`](./task-flow.md) |
| Review-, Results- und Remediation-Artefakte | [`reviews-and-results.md`](./reviews-and-results.md) |
| Installation | [`installation.md`](./installation.md) |
| Projektkonfiguration | [`k-playbook-format.md`](./k-playbook-format.md) |

## Übersicht

Neue Commands werden erst sichtbar, nachdem die Verlinkung steht und der Assistent neu
gestartet wurde.

| Command | Zweck | Detail |
|---|---|---|
| **Projekt** | | |
| `/k-gui` | Oberfläche starten | führt durch Konfiguration, projekteigene Struktur und Assistenten-Verlinkung |
| **Docs** | | |
| `/k-docs` | Docs-Bestand prüfen und mögliche Aktionen anbieten | read-only Status; kann zu Code-, Tool-, Extract- oder Index-Aktion dispatchen |
| `/k-docs-code` | semantische Projekt-Doku aus dem Code erzeugen | schreibt je Thema eine Datei nach `k-playbook-local/docs/code/`; dorthin schreibt auch der Skill `ks-overlay-repo-analyse` |
| `/k-docs-tools` | Library-/Tool-Doku ergänzen | erzeugt je ausgewähltem Tool eine Pitfall-Datei unter `k-playbook-local/docs/libs/` |
| `/k-docs-extract` | Rohmaterial aus `k-playbook-local/material/` zu Doku verdichten | schreibt je Thema eine Datei nach `k-playbook-local/docs/extracted/`, mit Quelle und Konfidenz |
| `/k-docs-index` | den einen Docs-Index bauen und die Docs für AI-Sessions registrieren | schreibt `k-playbook-local/docs/README.md`, dazu `AGENTS.md` und `opencode.json` |
| **Code-Review** | | |
| `/k-pr-review` | GitHub-PRs laden, bewerten und optional approven, mergen oder lokal validieren | [`pr-review.md`](./pr-review.md) |
| `/k-review` | Review-Rezepte ausführen | [`code-review.md`](./code-review.md) |
| `/k-audit` | vollständigen Audit-Sweep über MCP anlegen oder fortsetzen und nach dem Merge triagieren | [`review-runs.md`](./review-runs.md) |
| `/k-results` | vorhandene Ergebnisse projektweit priorisieren | [`code-review.md`](./code-review.md) |
| `/k-remediation` | Findings bündeln und in Tasks oder Fixes überführen | [`code-review.md`](./code-review.md) |
| **Task-Flow** | | |
| `/k-task-create` | Task-Datei aus dem Gesprächskontext erzeugen | [`task-flow.md`](./task-flow.md) |
| `/k-task-refine` | Task-Dateien vor der Ausführung per Critic/Editor-Dialog härten | [`task-flow.md`](./task-flow.md) |
| `/k-run` | Task-Dateien sequenziell ausführen | [`task-flow.md`](./task-flow.md) |
| `/k-todo` | `k-playbook-local/TODO.md` anzeigen oder ergänzen | |
| **Hilfen** | | |
| `/k-enforcement` | expliziter Check gegen die effektive Regelmenge | read-only Bericht; Fixes nur nach Freigabe |
| `/k-test-check` | Tests ausführen und Fehlerursachen diagnostizieren | startet bewusst Tests, nicht nur Statuschecks |
| `/k-verlauf` | alte AI-Verläufe durchsuchen | read-only |
| `/k-vscode-project-color` | VS-Code-Fensterfarbe und -Titel pro Projekt setzen | schreibt `.vscode/settings.json` |

Einen Command `/k-install-security-tools` gibt es nicht mehr. Status und
Installationsbefehl kommen aus der Oberfläche, alles Weitere kann
`k-playbook/scripts/install-security-tools.sh` selbst — siehe
[`installation.md`](./installation.md#security-tools).

## Review-Flow

Die Code-Review-Familie ist bewusst gestuft:

| Anlass | Command | Ergebnis |
|---|---|---|
| vollständiger Sicherheits-Sweep über passende Werkzeuge und Audit-Rezepte | `/k-audit` | `k-playbook-local/results/YYYY-MM-DD/review-input.json`, `review-input.md`, `review-triage.md` |
| gezieltes einzelnes Review-Rezept, interaktiv oder als Report | `/k-review <name>` | interaktive Änderungsvorschläge oder `<family>/YYYY-MM-DD/review-input.json` und `review-triage.md` |
| Task-/Instruction-Datei vor Ausführung härten | `/k-task-refine [path]` | Review-Log direkt in der geprüften Task-/Instruction-Datei |

1. `/k-pr-review` bewertet einen konkreten Pull Request und bleibt standardmäßig read-only.
2. `/k-review <name>` führt ein Rezept aus und erzeugt je nach Rezept interaktive
   Änderungsvorschläge oder `review-triage.md` als Report-Handoff.
3. `/k-audit` orchestriert das Laufmodell: Lauf anlegen oder fortsetzen, Scanner starten,
   Evidence-Rezepte vor dem Merge ausführen, Merge starten, Perspektiven danach führen und
   `review-triage.md` schreiben.
4. `/k-results` priorisiert vorhandene Ergebnisfamilien zu einer projektweiten Summary.
5. `/k-remediation <result>` plant die Abarbeitung der Findings.

Wenn `/k-remediation` Tasks erzeugt, gehören sie in den normalen Task-Flow: erst
`/k-task-refine`, dann `/k-run`.

## Task-Flow

```text
/k-task-create
/k-task-refine
/k-run
```

Tasks entstehen direkt aus dem Gespräch oder aus `/k-remediation`. In beiden Fällen
werden sie vor der Ausführung gegengeprüft.

## Wo Commands ihre Ziele finden

Kein Command liest oder rät einen Pfad. Alles leitet sich aus dem Ort der
`K-PLAYBOOK.yaml` ab:

| Command | schreibt nach |
|---|---|
| `/k-task-create`, `/k-run` | `k-playbook-local/tasks/`, erledigt nach `tasks/done/` |
| `/k-todo` | `k-playbook-local/TODO.md` |
| `/k-review`, `/k-results` | `k-playbook-local/results/` |
| `/k-docs-code`, Skill `ks-overlay-repo-analyse` | `k-playbook-local/docs/code/` |
| `/k-docs-tools` | `k-playbook-local/docs/libs/` |
| `/k-docs-extract` | `k-playbook-local/docs/extracted/` |
| `/k-docs-index` | `k-playbook-local/docs/README.md`, dazu `AGENTS.md` und `opencode.json` im Hauptverzeichnis |

Gelesen wird zusätzlich aus `k-playbook/` — Regeln, Rezepte, Checks und Skripte.
Geschrieben wird dorthin nie.

## Der aufgelöste Arbeitsstand

Kein Command rechnet selbst aus, was gilt. Das macht das Werkzeug:

```bash
k-playbook/bin/k-playbook context
```

Die JSON-Ausgabe nennt die aufgelösten Verzeichnisse, die Instruktionsdateien in
Lesereihenfolge, die Remediation-Policy, die Guidelines und die drei Kataloge —
mitgeliefert und projekteigen bereits zusammengeführt:

```json
{
  "instructions": [
    "/projekt/k-playbook/k-playbook.md",
    "/projekt/k-playbook-local/k-playbook.md"
  ],
  "catalogs": {
    "rules": [
      { "name": "review-authoring.md",   "key": "review-authoring",  "origin": "dist" },
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

Der Aufruf steht am Anfang jedes Commands, aber nur einmal je Sitzung. Die Ausgabe
ändert sich während der Arbeit nicht und ist für jeden Command dieselbe, also
verwenden nachfolgende Commands die vorhandene weiter — auch die Dateien aus
`instructions` werden nur einmal gelesen. Neu geholt wird sie, wenn die
`K-PLAYBOOK.yaml` geschrieben wurde, sich der Bestand an Regeln, Reviews, Checks oder
Guidelines geändert hat oder die Arbeit in ein anderes Projekt gewechselt ist.

`/k-review`, `/k-enforcement` und `k-check` arbeiten auf dieser Menge und weisen sie vor
der Arbeit aus. Die Regeln im Detail stehen in
[`k-playbook-format.md`](./k-playbook-format.md#mitgeliefertes-und-projekteigenes-zusammenfassen).

## k-check

`k-playbook/bin/k-check` ist kein Slash-Command, sondern ein CLI-Runner für die
effektive Check-Menge:

```bash
k-playbook/bin/k-check --mode changed
k-playbook/bin/k-check --mode baseline
```

Die stabile Check-Schnittstelle ist `.sh`. Ein Check darf Python oder anderes intern
verwenden, muss aber genau eine Statuszeile `K_CHECK_STATUS=ok|skip|fail` und optional
`K_CHECK_REASON=<text>` schreiben. Details stehen in
[`../checks/README.md`](../checks/README.md).
