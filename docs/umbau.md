# Umbau: projektlokale Installation

Arbeitsdatei für die Dauer der Umstellung. Sie hält fest, was besprochen und festgelegt
ist — nicht, was angedacht wurde. Wenn alles umgestellt ist, wird der bleibende Teil in
die reguläre Doku eingearbeitet und diese Datei gelöscht.

Stand: 2026-08-10, Branch `feat/project-local-install`.

## Arbeitsteilung: Entwicklungsrepo vs. Installation

**`~/dev/k-playbook` ist das Entwicklungsrepo — keine Installation.** Hier entstehen und
werden per git bereitgestellt: die Skills, Commands, Checks, Reviews und Regeln, der
Installer und die Doku.

**Die tatsächliche Installation sieht anders aus.** Referenzprojekt zum Testen und
Anpassen ist `/home/kleist/dev/Aiva/kascada/`. Dort wird jede Umstellung gegen eine
echte, gewachsene Installation geprüft, nicht gegen ein frisch angelegtes
Beispielprojekt.

## Vorgaben

- Je Projekt eine eigene Installation.
- Die lokalen Einstellungen überschreiben die Vorgaben.
- Die Installation muss ohne Go möglich sein. Ein eigener Build muss trotzdem möglich
  sein und dasselbe Ergebnis liefern.
- `dist/` muss die Binaries mitliefern.
- Mehrere Zielplattformen gleichzeitig: macOS für den Host und Linux für den
  DevContainer. Ein Apple-Nutzer braucht beide.

## Festgelegtes Modell

- `<projekt>/k-playbook/` entsteht per `git clone` des Entwicklungsrepos. Der Clone
  bringt auch Quellcode und Docs mit; das Entwicklungsrepo wird passend dafür
  strukturiert.
- Das Werkzeug heißt `k-playbook`, nicht mehr `k-playbook-installer`. Es ist nicht nur
  der Installer, sondern soll künftig weitere Aufgaben übernehmen; die Aufgabe steckt im
  Subkommando.
- `bin/` enthält ausschließlich den Wrapper, versioniert im Repo, damit direkt nach dem
  Clone ein Einstiegspunkt vorhanden ist. `dist/` enthält ausschließlich die
  Plattform-Binaries. Es gibt nur ein Build-Target, damit ein eigener Build und die
  Auslieferung dasselbe Ergebnis liefern.
- Der Wrapper `bin/k-playbook` ruft die zur Plattform passende Version aus `dist/` auf.
- Nach dem Clone wird darin `make install` aufgerufen. Das ruft `bin/k-playbook install`.
- `install` legt das Parallelverzeichnis `k-playbook-local/` an.

## Verzeichnisaufteilung und Anker

Die `K-PLAYBOOK.yaml` liegt im **Hauptverzeichnis**, nicht in der Installation:

```text
<projekt>/                 beliebig benannt
├── K-PLAYBOOK.yaml        der Anker
├── k-playbook/            Installation, vollstaendig ersetzbar
└── k-playbook-local/      projekteigen
```

Weil `k-playbook/` damit nichts Projekteigenes mehr enthält, ist es komplett updatebar —
auch per `rm -rf` und neuem Clone.

**Das Entwicklungsrepo wird wie jede andere Installation behandelt.** Es gibt keinen
Sonderfall und keine Erkennungsheuristik. Für `~/dev/k-playbook` heißt das:
`~/dev/k-playbook/K-PLAYBOOK.yaml` ist der Anker, die Installation liegt unter
`~/dev/k-playbook/k-playbook/`. Dass die dort installierte Version eine andere ist als der
Arbeitsstand daneben, wird bewusst in Kauf genommen.

**Das Playbook-Verzeichnis heißt immer `k-playbook`.** Skills und Commands verwenden
durchgängig `<projekt>/k-playbook/`. Wie das Projektverzeichnis heißt, spielt keine Rolle;
dass es hier ebenfalls `k-playbook` heißt, ist Zufall und darf kein Kriterium sein.

**Das Projekt-Repo steht in der Config**, nicht in einer Ableitung aus dem Dateisystem. Es
kann das übergeordnete Verzeichnis sein oder — etwa im DevContainer — ein paralleles.

## Mechanismus: Anker finden

Gilt gleichermaßen für die LLM und für das Go-Programm.

1. Wurde ein Verzeichnis übergeben, gilt dieses; geprüft wird `<arg>/K-PLAYBOOK.yaml`.
2. Sonst ab `realpath(CWD)` aufwärts, ein Kandidat je Ebene: `<dir>/K-PLAYBOOK.yaml`.
3. Fund: `PROJECT_DIR = <dir>`, `PLAYBOOK_DIR = <dir>/k-playbook`.
4. Grenze der Aufwärtssuche sind `$HOME` und `/`, jeweils einschließlich.
5. Nichts gefunden: melden, dass keine Installation vorliegt. Nicht raten, nichts anlegen.

Die Aufwärtssuche darf **nicht** am Git-Worktree-Root abbrechen. `<projekt>/k-playbook/`
ist ein eigener Clone und damit ein eigener Worktree; wer von dort aus sucht, käme sonst
nie an die Config eine Ebene darüber.

## Anlegen, wenn nichts gefunden wird

Nach `git clone` existiert noch keine Config — die Suche schlägt also fehl. Statt zu raten,
schlägt das Werkzeug den Ort vor und lässt ihn bestätigen.

Das Hauptverzeichnis muss dabei nicht geraten werden: der Aufruf erfolgt über
`A/k-playbook/bin/`, das Binary liegt in `A/k-playbook/dist/`. Zwei Ebenen darüber liegt
`A`. Geprüft wird, dass das Zwischenverzeichnis wirklich `k-playbook` heißt; sonst fällt
der Vorschlag auf das Arbeitsverzeichnis zurück und wird als unsicher gekennzeichnet.

Die `.git`-Suche beantwortet nicht, wo `A` liegt, sondern was in `project.repo_root`
gehört:

| Situation | Hauptverzeichnis | `repo_root` |
|---|---|---|
| `A/.git` vorhanden | `A` | `.` |
| `A/G/.git`, `A` selbst ohne `.git` | `A` | `G` |
| mehrere Kandidaten unter `A` | `A` | leer, der Nutzer wählt |

Geschrieben wird ausschließlich auf Bestätigung. Eine vorhandene `K-PLAYBOOK.yaml` wird
nie überschrieben — sie gehört dem Projekt und kann Werte tragen, die das Werkzeug nicht
kennt.

## Verzeichnisstruktur eines Projekts

```text
projekt/
├── K-PLAYBOOK.yaml       der Anker; sein Ort bestimmt das Hauptverzeichnis
├── AGENTS.md             Instruktionen, eine Quelle für alle Assistenten
├── CLAUDE.md             Symlink auf AGENTS.md
├── .claude/
│   ├── commands  ──┐     Symlink
│   └── skills      │     Symlink; OpenCode liest hier mit
├── .opencode/      │
│   └── commands  ──┤     Symlink
├── .cursor/        │
│   └── commands  ──┤     Symlink
├── k-playbook/   ←─┘     die Installation, vollständig ersetzbar
│   ├── commands/ skills/ rules/ reviews/ checks/
│   ├── bin/ dist/
│   └── installer/ docs/
└── k-playbook-local/     projekteigen, committed
    ├── rules/            Overlay zu k-playbook/rules/
    ├── reviews/          Overlay zu k-playbook/reviews/
    ├── checks/           Overlay zu k-playbook/checks/
    ├── results/          Review-Ergebnisse
    ├── guidelines/
    ├── commands/         projekteigene Slash-Commands
    ├── tasks/done/
    ├── priv/             Inhalt gitignored, Verzeichnis versioniert
    └── TODO.md
```

Jedes Verzeichnis unter `k-playbook-local/` trägt eine `README.md` mit seinem Zweck —
auch weil Git leere Verzeichnisse nicht speichert und sie sonst nach einem Clone des
Projekts fehlen würden.

**Assistenten.** Verlinkt wird für Claude Code, OpenCode und Cursor. Skills stehen nur
einmal unter `.claude/skills`: OpenCode durchsucht dieses Verzeichnis mit, Cursor kennt
kein Skill-Konzept. `CLAUDE.md` ist ein Symlink auf `AGENTS.md`, weil Claude Code
ausschließlich `CLAUDE.md` liest und OpenCode `AGENTS.md` bevorzugt — so landet jede
Änderung in beiden. Fehlt `AGENTS.md`, wird nichts angelegt; die Datei gehört dem Projekt.

## Stand

Das Werkzeug führt durch drei Schritte: Konfiguration anlegen, projekteigene Struktur
anlegen, Assistenten verlinken. Der bisherige Go-Code liegt unter `installer/_old/` als
Nachschlagewerk — von der Go-Toolchain ignoriert und nicht baubar.

## Nachzuziehen

Sammelstelle für alles, was der neuen Struktur noch folgen muss.

**Ergebnisse liegen jetzt unter `k-playbook-local/results/`**, vorher unter
`<paths.reviews>/results/`. Umzustellen: `/k-results` sowie die Review-Rezepte, die dorthin
schreiben — `review-secret-scanning`, `review-codeql-security`, `review-k-check-security`,
`review-dependabot-alerts`, `review-dependency-cve`, `review-iac-container`, `review-tech`.

**Der projektlokale Regelordner heißt `rules/`**, nicht mehr `enforcement/`. Umzustellen:
`commands/_shared/overlay-resolution.md` (beschreibt die Asymmetrie als gewollt),
`rules/README.md`, der Skill `enforcement` und der Command `k-enforcement`.

**Pfade in Commands und Skills** zeigen noch auf das alte Layout: `k-playbook/docs/`,
`k-playbook/tasks/`, `_dist/`. Betroffen sind unter anderem `k-code2docs`, `k-pr-review`,
`k-setup-codeql`, `k-install-codeql` sowie die Skills `ai-session-memory` und
`overlay-repo-analyse`, die `k-playbook/docs/` fest verdrahten.

**`commands/_shared/path-resolution.md`** beschreibt noch `_dist/` als Installation und
die Config im k-playbook-Verzeichnis. Der Anker liegt jetzt im Hauptverzeichnis.

**`checks/README.md` und `bin/k-check`** setzen den Env-Kontrakt `K_PLAYBOOK_DIST` und
leiten das Verzeichnis aus der eigenen Lage ab.

**`scripts/install-security-tools.sh` findet seine Tool-Matrix nicht mehr.** Der Default
zeigt auf `$PLAYBOOK_REPO/global/security-tools.tsv`; ein `global/` gibt es seit dem
Payload-Umzug nicht, die Datei liegt im Repo-Root. Ebenso verweist
`commands/k-install-security-tools.md` auf `<DIST_DIR>/security-tools.tsv` und im Text auf
`global/security-tools.tsv`.

**`make install` und `scripts/install-installer.sh`** folgen dem alten Modell und suchen
ein `k-playbook-installer`-Binary.

**Der Abschnitt „Altes Modell" in `README.md`** wird geleert, sobald die Punkte darüber
abgearbeitet sind.
