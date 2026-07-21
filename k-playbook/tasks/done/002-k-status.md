# Task 002 — /k-status

Erstellt einen neuen Slash-Command `/k-status`, der `K-PLAYBOOK.MD` schnell ausliest, die registrierten Playbook-Bausteine prüft und dem User einen kompakten Health-Überblick mit nächsten Aktionen gibt.

## Intent

`/k-status` soll ein schneller, read-only Lagecheck für ein Projekt mit k-playbook sein: zuerst Überblick, dann konkrete Warnungen und direkt handlungsfähige nächste Commands.

- Der Command bleibt im Standardmodus schnell und führt keine schweren Scans, Installationen oder Reparaturen aus.
- `K-PLAYBOOK.MD` ist die zentrale Quelle; alle Pfade und CodeQL-Entscheidungen werden daraus gelesen und validiert.
- CodeQL wird nur per schneller Preflight-Abfrage geprüft (`K-PLAYBOOK.MD`, Workflow-/DB-Pfade, bei aktivem/geplantem CodeQL zusätzlich `codeql version`), aber nicht analysiert.
- Die Ausgabe ist erweiterbar in klar getrennten Sektionen, damit später weitere Checks ergänzt werden können.
- Der Command ist read-only: keine Dateien anlegen, keine Config ändern, keine Datenbanken erzeugen.

## Referenzen

- `K-PLAYBOOK.MD` — aktuelle Pointer-/Config-Datei mit Pfaden und CodeQL-Block
- `commands/_shared/path-resolution.md` — bestehende Pfadauflösung für Commands
- `commands/k-setup-codeql.md` — Format und Semantik des CodeQL-Managed-Blocks
- `commands/k-install-codeql.md` — lokale CodeQL-Preflight-Signale und erwartete Artefakte
- `commands/k-task-create.md` — Stil und Struktur bestehender Command-Spezifikationen

## Ziel

Eine neue Command-Datei `commands/k-status.md` anlegen, die einen kompakten, schnellen Statusbericht für das aktuelle Projekt definiert.

Der Command soll standardmäßig ungefähr diese Fragen beantworten:

1. Gibt es `K-PLAYBOOK.MD`, und sind die managed Blöcke plausibel?
2. Welche Playbook-Pfade sind registriert, welche existieren, welche fehlen?
3. Gibt es offene Tasks oder TODO-Einträge?
4. Gibt es Review-Dateien, Review-Log und fällige Reviews?
5. Gibt es Enforcement-Regeln, und ist der Pfad nutzbar?
6. Ist CodeQL laut Config aktiv/geplant, und passen Workflow/DB/CLI grob dazu?
7. Wie ist der Git-Zustand des Projekts?
8. Sind Docs-Indizes vorhanden?
9. Welche nächste Aktion ist am wahrscheinlichsten sinnvoll?

## Kontext

- `K-PLAYBOOK.MD` ist keine User-Doku, sondern eine Pointer-/Config-Datei. `/k-status` darf sie anzeigen bzw. zusammenfassen, soll sie aber nicht verändern.
- Der vorhandene CodeQL-Block unterscheidet `github` und `local-database`; beide können `true`, `false` oder `planned` sein.
- `/k-status` soll bewusst nicht mit `/k-install-codeql`, `/k-setup-codeql`, `/k-review` oder `/k-run` konkurrieren. Er soll nur erkennen und auf diese Commands verweisen.
- Der Nutzen entsteht aus Geschwindigkeit: lieber viele kleine, sichere Existenz-/Metadatenchecks als wenige teure Tiefenanalysen.

## Zu bauen

**Neue Datei:**

- `commands/k-status.md`

**Command-Frontmatter:**

- `description`: beschreibt den schnellen read-only Health-Check für `K-PLAYBOOK.MD`, Pfade, Tasks, Reviews, Enforcement, CodeQL, Git und Docs.
- `argument-hint`: `[full|codeql|reviews|json|strict]`
- `allowed-tools`: mindestens `Read`, `Bash`, `Glob`, `Grep`, `TodoWrite`; kein `Write`/`Edit`, weil der Command read-only bleiben soll.

**Argumente / Modi:**

- Kein Argument: kompakte Standardausgabe.
- `full`: zusätzlich den Inhalt von `K-PLAYBOOK.MD` ausgeben oder vollständig zusammenfassen.
- `codeql`: nur CodeQL-Sektion prüfen und ausgeben.
- `reviews`: nur Review-Sektion prüfen und ausgeben.
- `json`: maschinenlesbare Ausgabe als best-effort JSON beschreiben. Falls die konkrete Umsetzung ohne Script zu aufwendig ist, zunächst als optionalen Erweiterungspunkt dokumentieren.
- `strict`: gleiche Checks, aber Warnungen klarer als fehlgeschlagene Health-Gates markieren. Keine Änderungen am Dateisystem.

**Sektionen:**

1. `playbook`
   - `TARGET_DIR` bestimmen wie in `commands/_shared/path-resolution.md`.
   - Prüfen, ob `<TARGET_DIR>/K-PLAYBOOK.MD` existiert.
   - Managed Marker erkennen:
     - `k-setup:managed:begin/end`
     - `k-setup-codeql:managed:begin/end`
   - `repo:` und `setup-run:` ausgeben, falls vorhanden.

2. `paths`
   - Aus `## Pfade` mindestens diese Keys lesen: `base`, `tasks`, `todo`, `checks`, `reviews`, `guidelines`, `enforcement`, `docs`.
   - Relative Pfade gegen `TARGET_DIR` auflösen.
   - Pfade außerhalb von `TARGET_DIR`, absolute Pfade und Symlinks nur auflösen und klar kennzeichnen; keine Traversal-/Symlink-Folgen mit schweren Scans ausführen.
   - Pro Eintrag Status bestimmen: `OK` existiert, `WARN` unset/`-`, `FAIL` gesetzt aber fehlt.
   - Datei- vs. Verzeichniswerte beachten: `todo` ist eine Datei, die übrigen Standardwerte sind Verzeichnisse.

3. `tasks`
   - Wenn `tasks:` existiert: direkt darin liegende `.md`-Dateien mit führender Nummer zählen.
   - `done/` ignorieren, aber optional Anzahl erledigter Tasks nennen.
   - Nächste offene Task-Datei nach Nummer anzeigen.

4. `todo`
   - Wenn `todo:` existiert: grob offene Markdown-Checkboxen zählen (`- [ ]`).
   - Wenn keine Checkboxen vorhanden sind: Datei nur als vorhanden melden, nicht als Fehler.

5. `reviews`
   - Wenn `reviews:` existiert: `review-*.md` zählen.
   - `log.md` und `known-decisions.md` prüfen.
   - Best-effort fällige Reviews aus Frontmatter `interval-weeks` plus Log-Datum ableiten; wenn zu aufwendig, zunächst nur fehlende Log-/Decision-Dateien und Anzahl Reviews melden.

6. `enforcement`
   - Wenn `enforcement:` existiert: `.md`-Regeldateien zählen und kurze Liste anzeigen.
   - Leer gesetzter oder fehlender Enforcement-Pfad ist `WARN`, nicht `FAIL`.

7. `codeql`
   - CodeQL-Block aus `K-PLAYBOOK.MD` parsen:
     - `enabled`
     - `github`
     - `workflow`
     - `local-database`
     - `database`
     - `languages`
     - `queries`
     - `setup-run`
   - Wenn `enabled: false`: kurz melden und keine tiefe Prüfung machen.
   - Wenn `github: true|planned`: prüfen, ob `workflow:` gesetzt ist und die Datei existiert.
   - Wenn `local-database: true|planned`: prüfen, ob `database:` gesetzt ist und der Pfad existiert.
   - `codeql version` nur versuchen, wenn `enabled`, `github` oder `local-database` auf `true` oder `planned` steht; Fehler kurz als `fehlt` melden.
   - Keine Datenbank erzeugen, keine Analyse ausführen, keine SARIF-Dateien hochladen.

8. `git`
   - Prüfen, ob `TARGET_DIR` ein Git-Worktree ist.
   - Branch, clean/dirty, Anzahl geänderter/untracked Dateien anzeigen.
   - Keine Diffs ausgeben.

9. `docs`
   - `docs:` prüfen.
   - `docs/README.md` und `docs/libs/README.md` erkennen.
   - Bei fehlenden Docs nur Empfehlungen geben (`/k-code2docs`, `/k-tools-scan`).

10. `recommendations`
     - Aus den Befunden maximal 3 nächste Aktionen ableiten.
     - Bei mehreren `FAIL`/`WARN` priorisieren: zuerst fehlendes `K-PLAYBOOK.MD`, dann fehlende Pflichtpfade, dann CodeQL, Tasks, Reviews, Docs.
     - Beispiele:
      - `K-PLAYBOOK.MD` fehlt → `/k-setup`
      - CodeQL aktiv/geplant, Workflow fehlt → `/k-setup-codeql`
      - lokale CodeQL-DB geplant/aktiv, DB fehlt → `/k-install-codeql`
      - offene Tasks → `/k-run`
      - Reviews vorhanden/fällig → `/k-review`
      - Docs fehlen → `/k-code2docs`

**Ausgabeformat:**

Kompakt, scanbar, ohne lange Prosa. Beispiel:

```text
/k-status
────────────────────────
Projekt:       /path/to/project
K-PLAYBOOK:    OK (setup-run 2026-07-20)
Pfade:         OK 7 / WARN 1 / FAIL 0
Tasks:         3 offen, nächste: 002-k-status.md
TODO:          OK, 4 offen
Reviews:       WARN, 2 vorhanden, known-decisions fehlt
Enforcement:   OK, 1 Regel
CodeQL:        WARN, enabled=true, github=true workflow fehlt
Git:           WARN, dirty (4 geändert, 1 untracked)
Docs:          WARN, docs/README.md fehlt

Nächste Aktionen:
1. /k-setup-codeql
2. /k-run
3. /k-code2docs
```

**Statusregeln:**

- `OK`: vorhanden und plausibel.
- `WARN`: optional, geplant, leer oder unvollständig, aber nicht zwingend kaputt.
- `FAIL`: gesetzter Pflicht-/Config-Pfad fehlt, Marker sind widersprüchlich, oder `K-PLAYBOOK.MD` fehlt im normalen Modus.

**Nicht Teil dieses Tasks:**

- Ein separates Shell-/Python-Script implementieren.
- Automatische Reparaturen oder `--fix`.
- CodeQL-Datenbanken erstellen oder analysieren.
- Tests, Builds oder Projektchecks ausführen, sofern sie nicht später explizit konfiguriert werden.
- Git-Diffs anzeigen oder Änderungen committen.

---
## Review-Log (2026-07-21)

**Pfad:** k-playbook/tasks/002-k-status.md
**Intent:** —
**Runden:** 1

### Diskussion
Der Critic sah keine grundlegend falsche Richtung, aber mehrere FEHLEND-Punkte, die die spätere Ausführung uneinheitlich machen konnten. Der Editor hat die blockierungsnahen Punkte zu Argument-Hint, Pfadbehandlung, CodeQL-CLI-Check und Empfehlungspriorität minimal präzisiert; reine WARNUNG-Designpunkte wurden gemäß Moderator-Policy nicht geändert.

### Moderator-Entscheidungen
- WARNUNGs wurden nicht an den Editor gegeben, weil sie Designschärfungen betreffen und die Ausführung nicht blockieren.
- Die FEHLEND-Punkte 2, 5, 7 und 9 wurden als ausführungskritisch genug bewertet und gezielt behoben.

### Intent-Alignment
Yes — Die Task-Spezifikation deckt den Intent ab: schneller read-only Check, `K-PLAYBOOK.MD` als zentrale Quelle, nur CodeQL-Preflight ohne Analyse, klare erweiterbare Sektionen, konkrete Warnungen und priorisierte nächste Commands.

### Geänderte Dateien
- `002-k-status.md`: Argument-Hint mit Modi synchronisiert, Pfadbehandlung ergänzt, `codeql version`-Regel präzisiert und Empfehlungspriorität definiert (FEHLEND-02, FEHLEND-05, FEHLEND-07, FEHLEND-09)

### Offen (nicht gefixt)
- WARNUNG-01: `TodoWrite` im read-only Command bleibt eine Designentscheidung; nicht blockierend.
- WARNUNG-03: `json` bleibt als best-effort/Erweiterungspunkt beschrieben; nicht blockierend.
- WARNUNG-04: `full` bleibt flexibel beschrieben; nicht blockierend.
- WARNUNG-06: `strict` bleibt auf Ausgabe-/Health-Gate-Markierung beschränkt; keine Exit-Semantik erforderlich.
- WARNUNG-08: Review-Fälligkeit bleibt best-effort mit Fallback; nicht blockierend.
- WARNUNG-10: Allgemeine Statusregeln bleiben bestehen, sektionale Ausnahmen sind bereits teilweise spezifiziert.

## Ausführung

**Status:** Erfolgreich ausgeführt  
**Datum:** 2026-07-21  
**Zusammenfassung:** Neue Slash-Command-Spezifikation `/k-status` in `commands/k-status.md` angelegt. Der Command beschreibt einen read-only Health-Check für `K-PLAYBOOK.MD`, Pfade, Tasks, TODO, Reviews, Enforcement, CodeQL, Git und Docs inklusive Modi und priorisierten Empfehlungen.

**Intent-Alignment:** Ja - Die Spezifikation deckt den schnellen read-only Standardmodus, K-PLAYBOOK.MD als Quelle, CodeQL nur als Preflight, klare erweiterbare Sektionen und konkrete nächste Commands ab.
