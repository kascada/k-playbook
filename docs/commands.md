# k-playbook Commands

Diese Datei ist der kompakte Index der aktuellen k-playbook-Commands. Detailablaeufe stehen in eigenen Themenseiten; `commands.md` soll keine langen Ablaufbeschreibungen duplizieren.

Globale Regeln, Review-Rezepte und Checks liegen in diesem Repo unter `global/`. Projektlokale Regeln, Reviews, Checks, Tasks und Docs liegen im jeweiligen Projekt unter den in `K-PLAYBOOK.yaml` konfigurierten `paths.*`; die konventionellen Werte zeigen auf `k-playbook/...`.

## Detailseiten

| Thema | Detailseite |
|---|---|
| PR-Review | [`k-pr-review.md`](./k-pr-review.md) |
| Review-Rezepte | [`k-review.md`](./k-review.md) |
| Results-Summary | [`k-results.md`](./k-results.md) |
| Remediation | [`k-remediation.md`](./k-remediation.md) |
| Task-Flow | [`k-task-flow.md`](./k-task-flow.md) |
| Review-/Results-/Remediation-Artefakte | [`reviews-and-results.md`](./reviews-and-results.md) |
| Installation und DevContainer | [`multi-project-installation.md`](./multi-project-installation.md), [`installation.md`](./installation.md) |

## Kurzuebersicht

Aktueller Slash-Command-Bestand unter `commands/`: neue Dateien werden erst sichtbar, nachdem die Installer-GUI die OpenCode-/Claude-Registrierung aktualisiert hat und der jeweilige Assistant neu gestartet wurde.

| Command | Zweck | Detail |
|---|---|---|
| **Projekt und GUI** | | |
| `/k-gui` | lokale k-playbook Installer-GUI starten | GUI registriert Commands/Skills, verwaltet Projekte, DevContainer-Integration und Projektstruktur |
| `/k-status` | read-only Health-Check fuer Projekt und host-lokale Assistant-Registrierung | nutzt Installer-Status, repariert nichts |
| **Tool-Installation** | | |
| `/k-install-security-tools` | host-lokale Security-Review-Tools aus `global/security-tools.tsv` installieren oder pruefen | spezielle `k-install-*`-Commands werden separat dokumentiert |
| `/k-install-codeql` | lokale CodeQL CLI installieren/pruefen, optional lokale DBs analysieren | aendert `K-PLAYBOOK.yaml` nicht |
| **Docs** | | |
| `/k-code2docs` | semantische Projekt-Doku erzeugen und fuer AI-Sessions registrieren | schreibt unter `paths.docs`, plus `AGENTS.md` und `opencode.json` |
| `/k-tools-scan` | Library-/Tool-Doku nach `/k-code2docs` ergaenzen | schreibt unter `<paths.docs>/libs/` und aktualisiert den Docs-Index |
| **Code-Review** | | |
| `/k-pr-review` | GitHub-PRs laden, bewerten und optional approven, mergen oder lokal validieren | [`k-pr-review.md`](./k-pr-review.md) |
| `/k-review` | globale oder projektlokale Review-Rezepte ausfuehren | [`k-review.md`](./k-review.md) |
| `/k-results` | vorhandene Review-Results projektweit priorisieren | [`k-results.md`](./k-results.md) |
| `/k-remediation` | Review-Findings planen, gruppieren und in Tasks oder Fixes ueberfuehren | [`k-remediation.md`](./k-remediation.md) |
| **Task-Flow** | | |
| `/k-task-create` | strukturierte Task-Datei aus Gespraechskontext erzeugen | [`k-task-flow.md`](./k-task-flow.md) |
| `/k-review-loop` | Task-/Instruktionsdateien vor Ausfuehrung per Critic/Editor-Dialog pruefen | [`k-task-flow.md`](./k-task-flow.md) |
| `/k-run` | Task-Dateien sequenziell ausfuehren | [`k-task-flow.md`](./k-task-flow.md) |
| `/k-todo` | Projekt-TODO anzeigen oder Eintrag ergaenzen | nutzt `paths.todo` |
| **Checks und Hilfen** | | |
| `/k-enforcement` | expliziter Check gegen globale und projektlokale Regeln | read-only Bericht; Fixes nur nach expliziter User-Freigabe |
| `/k-test-check` | Tests ausfuehren, Fehlerursachen diagnostizieren und Fixes erst nach Rueckfrage angehen | startet bewusst Tests, nicht nur Statuschecks |
| `/k-verlauf` | alte AI-Verlaeufe durchsuchen | liest Claude-JSONL bzw. OpenCode-Logs read-only |
| `/k-vscode-project-color` | VS-Code-Fensterfarbe/-Titel pro Projekt setzen | schreibt oder merged `.vscode/settings.json` |

## Review-Flow

Die Code-Review-Familie ist bewusst gestuft:

1. `/k-pr-review` bewertet einen konkreten Pull Request und bleibt standardmaessig read-only.
2. `/k-review <name>` fuehrt ein Review-Rezept aus und erzeugt je nach Rezept interaktive Aenderungsvorschlaege oder Report-Artefakte.
3. `/k-results` priorisiert vorhandene Result-Familien zu einer projektweiten Summary.
4. `/k-remediation <result>` plant die Abarbeitung der Findings.

Wenn `/k-remediation` Tasks erzeugt, gehoeren sie in den normalen Task-Flow: zuerst `/k-review-loop`, danach `/k-run`. Remediation soll keine groesseren Task-Inhalte direkt aus dem Chat heraus abarbeiten, wenn die Projekt-Policy Task/Branch/PR vorsieht.

## Task-Flow

Der Standardablauf fuer geplante Arbeit ist:

```text
/k-task-create
/k-review-loop
/k-run
```

Tasks koennen direkt aus dem Gespraech entstehen oder von `/k-remediation` erzeugt werden. In beiden Faellen gilt: Task-Dateien werden vor der Ausfuehrung mit `/k-review-loop` gegengeprueft und danach mit `/k-run` sequenziell ausgefuehrt.

## Installer-GUI

Die Installer-GUI ist der normale Weg fuer Host-Registrierung und Projekt-Onboarding.

Start direkt:

```text
k-playbook-installer
```

Start aus OpenCode:

```text
/k-gui
```

Die GUI prueft und repariert den Pfadvertrag `~/dev/k-playbook`, registriert OpenCode- und Claude-Commands/Skills, verwaltet die lokale Projektliste, erzeugt `K-PLAYBOOK.yaml`, vervollstaendigt die konfigurierte Projektstruktur und verwaltet die DevContainer-Integration.

## Status und Smoke

`/k-status` ist read-only. Neben Projektstatus aus `K-PLAYBOOK.yaml` prueft der Command auch die host-lokale OpenCode-Registrierung: Command-Symlinks, verwaiste k-playbook-Symlinks und `skills.paths`.

Fuer maschinenlesbaren Projektstatus nutzt `/k-status json` bevorzugt das Installer-Binary im aktuellen Projekt:

```bash
k-playbook-installer status
```

Aufwendigere oder externe Pruefungen laufen explizit ueber den Installer-Smoke-Test, nicht ueber `/k-status`:

```bash
k-playbook-installer smoke [path]
k-playbook-installer smoke --all
```

## global/bin/k-check

`global/bin/k-check` ist kein Slash-Command, sondern ein wiederverwendbarer CLI-Entry-Point fuer globale und projektlokale Checks.

Typische Nutzung aus einem Projekt-Root:

```bash
~/dev/k-playbook/global/bin/k-check --mode changed
~/dev/k-playbook/global/bin/k-check --mode baseline
~/dev/k-playbook/global/bin/k-check --config-root /path/to/project --mode changed
```

Der Runner liest `K-PLAYBOOK.yaml`, fuehrt `.sh`-Checks aus `global/checks/` und projektlokale `.sh`-Checks aus `k-playbook/checks/` aus. Die stabile Check-Schnittstelle ist `.sh`; einzelne Checks duerfen Python oder andere Tools intern verwenden, muessen aber am Ende genau eine Statuszeile `K_CHECK_STATUS=ok|skip|fail` und optional `K_CHECK_REASON=<text>` schreiben.
