# k-playbook Handbuch

## Zweck

k-playbook ist ein eigener, versionierter Werkzeugkasten fuer AI-Assistant-Arbeit in Entwicklungsprojekten. Er buendelt wiederkehrende Ablaeufe als Slash-Commands, Skills/Playbooks, globale Regeln, Review-Rezepte und Checks.

Das Ziel ist nicht, ein moeglichst grosses Framework zu bauen. Das Ziel ist, haeufige Arbeitsschritte kontrolliert, nachvollziehbar und wiederholbar zu machen:

- Projekte einrichten und ihre lokalen Playbook-Pfade registrieren.
- Docs als erste Quelle fuer AI-Sessions verankern.
- Tasks strukturiert erstellen und ausfuehren.
- Reviews, Scannergebnisse und Remediation auditierbar ablegen.
- Globale und projektlokale Regeln waehrend der Arbeit beachten.
- Host-Installation und Projekt-Setup sauber trennen.

## Grundmodell

k-playbook trennt globale Bausteine von projektlokalen Daten.

| Ebene | Ort | Inhalt |
|---|---|---|
| Global | `~/dev/k-playbook/` | Commands, Skills, globale Regeln, globale Reviews, globale Checks. |
| Projektlokal | Zielprojekt | `K-PLAYBOOK.MD`, Tasks, TODOs, Checks, Reviews, Docs, Enforcement-Regeln. |

`K-PLAYBOOK.MD` ist die zentrale Pointer-Datei im Projekt. Commands sollen keine Projektpfade raten, sondern diese Datei lesen.

`~/dev/k-playbook` ist dabei der verbindliche logische Pfad zum globalen Basis-Repo. Auf normalen Hosts liegt dort entweder das echte Repo oder ein Symlink auf den echten Klon. In Devcontainern liegt dort typischerweise ein Symlink auf den gemounteten Repo-Pfad, z. B. `/home/vscode/dev/k-playbook -> /workspaces/k-playbook`. Dadurch bleibt der `repo:`-Eintrag in `K-PLAYBOOK.MD` auf Host und Container identisch.

Das Basis-Repo wird zur Laufzeit gebraucht: OpenCode-Command-Eintraege sind Symlinks auf `commands/k-*.md`, Skills werden ueber `skills.paths` aus dem Repo geladen, und Commands/Skills nutzen den `repo:`-Rueckverweis fuer globale Regeln, Shared-Module und Skripte.

Typische projektlokale Pfade:

```markdown
- base:        .
- tasks:       ./tasks
- todo:        ./TODO.md
- checks:      ./checks
- reviews:     ./reviews
- guidelines:  ./guidelines
- enforcement: ./enforcement
- docs:        ./docs
```

## Installation Und Setup

Auf jedem Host wird k-playbook einmal global fuer OpenCode registriert:

```text
/k-install
/k-install-security-tools --install missing
```

In jedem Zielprojekt wird danach die lokale Konfiguration angelegt:

```text
/k-setup
```

Optional pro Projekt:

```text
/k-setup-codeql
```

Details stehen in [`installation.md`](./installation.md), [`multi-project-installation.md`](./multi-project-installation.md), [`commands.md`](./commands.md) und der kurzen [`FAQ`](./faq.md).

## Bausteine

### Commands

Commands liegen unter `commands/k-*.md` und werden in OpenCode als Slash-Commands genutzt.

Kurzuebersicht der wichtigsten Commands nach Arbeitsphase:

| Command | Scope | Projekt-Konfig | Artefakte / Host |
|---------|-------|----------------|------------------|
| **Install** | | | |
| `/k-install` | k-playbook auf diesem Host fuer OpenCode registrieren und Security-Tool-Preflight zeigen | keine Aenderung | OpenCode-Symlinks, ggf. Skill-Pfad, nur Tool-Status |
| `/k-install-security-tools` | host-lokale Security-Review-Tools installieren/pruefen | keine Aenderung | `gitleaks`, `trufflehog`, `pip-audit`, `trivy`, `syft`, `grype` oder Docker-Images |
| `/k-install-codeql` | lokale CodeQL CLI installieren/pruefen, optional lokale DBs analysieren | keine Aenderung an `K-PLAYBOOK.MD` | optional `codeql-cli/`, `databases/`, `results/` |
| `/k-setup` | k-playbook in einem Projekt konfigurieren | schreibt `K-PLAYBOOK.MD` und gewaehlte Playbook-Pfade | keine Host-Aenderung |
| `/k-setup-codeql` | CodeQL-Entscheidung im Projekt registrieren | schreibt CodeQL-Block in `K-PLAYBOOK.MD` | optional CLI-only Artefakt unter `codeql-cli/` |
| `/k-code2docs` | semantische Projekt-Doku erzeugen und fuer AI-Sessions registrieren | liest `docs:` | schreibt `<docs>/*.md`, `<docs>/README.md`, `AGENTS.md`, `opencode.json` |
| `/k-tools-scan` | Library-/Tool-Doku nach `/k-code2docs` ergaenzen | liest `docs:` | schreibt `<docs>/libs/*.md`, `libs/README.md`, aktualisiert Hauptindex |
| `/k-status` | read-only Health-Check fuer Projekt und host-lokale OpenCode-Registrierung | keine Aenderung | prueft u. a. Command-Symlinks und `skills.paths` |
| **Code-Review** | | | |
| `/k-review` | globale oder projektlokale Review-Rezepte ausfuehren | liest `reviews:` und `known-decisions.md` | interaktive Aenderungen oder Report-Artefakte unter `<reviews>/results/<family>/YYYY-MM-DD/` |
| `/k-results` | vorhandene Review-Results projektweit priorisieren | liest `reviews:` und optional `tasks:` | schreibt `<reviews>/results/summary-YYYY-MM-DD.md` |
| `/k-remediation` | Review-Findings planen, gruppieren und abarbeiten | liest `reviews:`, `tasks:` und Remediation-Policy | erzeugt Tasks, aktualisiert Findings/Assessment oder macht freigegebene direkte Fixes |
| **Task-Flow** | | | |
| `/k-task-create` | strukturierte Task-Datei aus Gespraechskontext erzeugen | liest `tasks:` | schreibt `<tasks>/<NNN>-<slug>.md` nach Bestaetigung |
| `/k-review-loop` | Task-/Instruktionsdateien vor Ausfuehrung per Critic/Editor-Dialog pruefen | liest optional `tasks:` | Moderator schreibt akzeptierte Task-Edits und Review-Log |
| `/k-run` | Task-Dateien sequenziell ausfuehren | liest `tasks:` und `K-PLAYBOOK.MD`-Kontext | delegiert an Subagenten, schreibt Ausfuehrungsnotiz, verschiebt erfolgreiche Tasks nach `done/` |
| **Nuetzliches** | | | |
| `/k-verlauf` | alte AI-Verlaeufe durchsuchen | keine Projektdatei noetig | liest Claude-JSONL bzw. OpenCode-Logs read-only |
| `/k-vscode-project-color` | VS-Code-Fensterfarbe/-Titel pro Projekt setzen | keine `K-PLAYBOOK.MD`-Pflicht | schreibt/merged `.vscode/settings.json` |
| **Weitere** | | | |
| `/k-todo` | Projekt-TODO anzeigen oder Eintrag ergaenzen | liest `todo:` | schreibt/ergaenzt `TODO.md` bzw. den registrierten Todo-Pfad |
| `/k-enforcement` | expliziter Check gegen globale und projektlokale Regeln | liest `enforcement:` und `docs:` | read-only Bericht; Fixes nur nach expliziter User-Freigabe |
| `/k-test-check` | Tests ausfuehren und Fehlerursachen diagnostizieren | keine eigene Pfad-Konfig | startet Tests, macht Diagnose, fragt vor Fixes |

Details zu allen Commands stehen in [`commands.md`](./commands.md). Fuer die Reihenfolge bei mehreren Zielprojekten und DevContainern siehe [`multi-project-installation.md`](./multi-project-installation.md).

### Skills Und Playbooks

Skills liegen unter `ks-<thema>/`. Jede Skill-Struktur hat mindestens:

```text
ks-<thema>/
├── SKILL.md
└── PLAYBOOK.md
```

`SKILL.md` beschreibt, wann OpenCode den Skill automatisch laden soll. `PLAYBOOK.md` enthaelt die eigentliche Anleitung, Checklisten und Details.

Aktuelle Skills:

| Skill | Zweck |
|---|---|
| `ks-ai-session-memory` | Docs fuer spaetere AI-Sessions als autoritative Quelle verankern. |
| `ks-enforcement` | Globale und projektlokale Regeln waehrend der Arbeit anwenden. |
| `ks-overlay-repo-analyse` | Docker-Overlay-Repos gegen ihre Base analysieren und dokumentieren. |

### Globale Regeln

Globale Regeln liegen unter `global/rules/`. Sie gelten projektuebergreifend und koennen durch projektlokale Regeln aus dem `enforcement:`-Pfad ergaenzt werden.

Wichtige Regeln:

- `docs-sync.md`: Code- und Dokumentationsaenderungen synchron halten.
- `review-authoring.md`: Review-Rezepte knapp und spezifisch halten.
- `codeql.md`: CodeQL-Setup, CLI und lokale Analysen sauber trennen.
- `tool-install-scope.md`: `/k-install*`, host-lokale Tools und Projekt-venvs trennen.

### Globale Checks

`global/bin/k-check` fuehrt globale und projektlokale `.sh`-Checks aus. Der Runner kann im `changed`- oder `baseline`-Modus laufen.

```bash
~/dev/k-playbook/global/bin/k-check --mode changed
~/dev/k-playbook/global/bin/k-check --mode baseline
```

Fuer auditierbare Review-Laeufe koennen Raw-Ausgabe und Metadaten dauerhaft in `reviews/results/k-check/<datum>/` geschrieben werden.

Details: [`../global/checks/README.md`](../global/checks/README.md).

## Standardablaeufe

### Neuer Host

1. Repo nach `~/dev/k-playbook` klonen.
2. OpenCode starten.
3. Im k-playbook-Repo `/k-install` ausfuehren.
4. Falls ein Python-venv aktiv ist: `deactivate` ausfuehren.
5. Fehlende Security-Tools mit `/k-install-security-tools --install missing` installieren.
6. OpenCode neu starten.

### Neues Projekt

1. Im Projektroot `/k-setup` ausfuehren.
2. Benoetigte Bausteine aktivieren, z. B. `tasks`, `todo`, `reviews`, `docs`, `enforcement`.
3. Optional `/k-setup-codeql` ausfuehren.
4. Mit `/k-status` pruefen, ob Pfade und Grundstruktur plausibel sind.

### Zielprojekt Im Devcontainer

1. Auf dem Host `~/dev/k-playbook/scripts/install-devcontainer-k-playbook.sh <projekt-root>` ausfuehren.
2. Das Script schreibt `.devcontainer/setup-k-playbook.sh`, ergaenzt den Mount nach `/workspaces/k-playbook` und registriert `postCreateCommand`/`postStartCommand`.
3. DevContainer neu bauen oder neu starten und OpenCode im Container neu starten.
4. Im Zielprojekt `/k-status` ausfuehren; die Sections `playbook`, `opencode` und `devcontainer` muessen plausibel sein.

### Docs-First Aufsetzen

1. `docs:` in `/k-setup` aktivieren.
2. `/k-code2docs` nutzen oder das Playbook `ks-ai-session-memory` manuell anwenden.
3. Sicherstellen, dass `<docs>/README.md` im in `K-PLAYBOOK.MD` registrierten `docs:`-Pfad einen Index hat.
4. Sicherstellen, dass `AGENTS.md` und `opencode.json` die Docs als autoritative Quelle registrieren.
5. OpenCode neu starten.

Neue von `/k-code2docs` und `/k-tools-scan` erzeugte Doc-Dateien nutzen leichtgewichtig OKF-kompatibles YAML-Frontmatter (`type`, `title`, `description`, `tags`, `status`, `generated`). Der Hauptindex bleibt bewusst `<docs>/README.md`; es wird kein OKF-`index.md` als Ersatz erzwungen.

### Review Und Remediation

1. Review ausfuehren: `/k-review <name>`.
2. Resultate unter `<reviews>/results/<family>/<date>/` ablegen.
3. Projektweit priorisieren: `/k-results`.
4. Remediation planen: `/k-remediation <summary-oder-assessment>`.
5. Erzeugte Tasks mit `/k-run` umsetzen.

Result-Familien sollen mindestens diese Artefakte haben:

```text
assessment.md
findings.md
raw/
run-metadata.json
```

## Betriebsregeln

- `/k-install*` ist host-global und schreibt keine Projektdateien.
- `/k-install` wird bevorzugt im k-playbook-Repo ausgefuehrt; aus Projekten ist es erlaubt, nutzt aber immer den festen Pfad `~/dev/k-playbook`.
- `/k-setup*` ist projektlokal und installiert keine host-globalen Tools.
- `K-PLAYBOOK.MD` ist Konfiguration, keine User-Dokumentation.
- `repo:` in `K-PLAYBOOK.MD` ist fest `~/dev/k-playbook`; andere physische Repo-Orte werden ueber Symlinks abgebildet.
- Projektpfade werden nicht geraten, sondern aus `K-PLAYBOOK.MD` gelesen.
- Review-Rohdaten und Run-Metadaten sind auditierbar und werden nicht still ueberschrieben.
- Projektwissen gehoert in `docs/`; AI-Sessions sollen Docs zuerst konsultieren.
- Neue oder geaenderte Commands muessen nach dem Pull auf jedem Host mit `/k-install` sichtbar gemacht werden.

## Konventionen

- User-facing Namen beginnen mit `k-`.
- Slash-Commands heissen `commands/k-<thema>.md`.
- Skills heissen `ks-<thema>/`.
- Templates liegen in `ks-<thema>/vorlagen/`.
- Skill-spezifische Skripte liegen in `ks-<thema>/scripts/`.
- Globale Check-Entry-Points liegen unter `global/bin/`.
- Globale Checks liegen als `.sh`-Schnittstelle unter `global/checks/`.

## Weiterfuehrende Docs

- Installation: [`installation.md`](./installation.md)
- Multi-Project-Installation: [`multi-project-installation.md`](./multi-project-installation.md)
- Commands: [`commands.md`](./commands.md)
- Reviews und Results: [`reviews-and-results.md`](./reviews-and-results.md)
- Check-Runner: [`../global/checks/README.md`](../global/checks/README.md)
- Regeln: [`../global/rules/README.md`](../global/rules/README.md)
