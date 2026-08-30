# k-playbook

Diese Datei gilt in jedem Projekt, das k-playbook nutzt. Sie wird mitgeliefert und
bei jedem Update ersetzt. Was nur für ein einzelnes Projekt gilt, steht in
`k-playbook-local/k-playbook.md` und wird nach dieser Datei gelesen.

## Zwei Verzeichnisse, zwei Eigentümer

```text
<projekt>/
├── K-PLAYBOOK.yaml       der Anker; sein Ort bestimmt das Hauptverzeichnis
├── k-playbook/           die Installation
└── k-playbook-local/     alles Projekteigene
```

**In `k-playbook/` wird nie geschrieben.** Das Verzeichnis ist ein Clone und wird bei
jedem Update vollständig ersetzt — eine Änderung dort ist beim nächsten `git pull`
verloren. Wer eine mitgelieferte Datei anpassen will, legt eine gleichnamige in
`k-playbook-local/` an.

Alles, was das Projekt selbst hervorbringt, gehört nach `k-playbook-local/`: eigene
Regeln, Reviews, Checks, Commands und Skills, dazu Ergebnisse, Tasks, Notizen.

## Überlagern und Abschalten

Für `rules`, `reviews`, `checks`, `commands` und `skills` gilt: Ein gleichnamiger
Eintrag in `k-playbook-local/` **ersetzt** den mitgelieferten vollständig. Der
mitgelieferte wird dann gar nicht gelesen; es werden auch keine einzelnen Abschnitte
übernommen.

Ein **leerer** lokaler Eintrag — nichts außer Leerzeilen und Kommentaren — schaltet den
mitgelieferten ab. So kann die Datei ihren eigenen Grund tragen:

```bash
# Abgeschaltet: dieses Projekt nutzt kein Django.
```

Bei `rules` und `reviews` bleibt der Eintrag sichtbar, sein Inhalt sagt dann, dass er
abgeschaltet ist. Ein Check fällt ganz aus dem Katalog: ein leeres Skript würde mit
Exit 0 durchlaufen und wie ein bestandener Check aussehen. Ein abgeschalteter Command
oder Skill wird nicht mehr beim Assistenten registriert.

Die Vergleichseinheit ist der Name:

| Sorte | Einheit | Name |
|---|---|---|
| `rules`, `reviews` | eine `*.md`-Datei | Dateiname |
| `checks` | ein `*.sh`-Skript | Dateiname |
| `commands` | eine `*.md`-Datei | Pfad ab `commands/`, z. B. `_shared/context.md` |
| `skills` | ein Verzeichnis mit `SKILL.md` | Verzeichnisname |

Commands werden bis in die Namensräume hinein verglichen: eine lokale
`commands/_shared/context.md` ersetzt genau diese Datei, der Rest des
Namensraums bleibt mitgeliefert. Ein Skill wird dagegen als Ganzes ersetzt — `SKILL.md`,
`PLAYBOOK.md` und Vorlagen müssen zueinander passen.

Neue oder geänderte Commands und Skills werden nicht von selbst wirksam: sie müssen
beim Assistenten registriert werden. Die Oberfläche (`k-playbook` ohne Argument)
vergleicht den Katalog mit dem Registrierten und richtet die Verlinkung ein.

## Wo was liegt

Verlass dich nicht auf Annahmen über Pfade. `k-playbook/bin/k-playbook context`
liefert die aufgelösten Verzeichnisse und die effektiven Kataloge — mitgeliefert und
projekteigen bereits zusammengeführt, abgeschaltete Einträge markiert.

## Wie gearbeitet wird

Eine Umsetzung, die über einen trivialen Einzelschritt hinausgeht, bleibt nicht als
Plan-Text im Gespräch stehen, wird nicht als Plan-Datei abgelegt und nicht einfach
begonnen — sie wird eine Task: `/k-task-create` anbieten oder nutzen, danach
`/k-task-refine`, dann `/k-task-run`. Das betrifft, wo das Ergebnis einer Planung landet,
nicht wie es entsteht; im Plan-Modus des Assistenten zu entwerfen bleibt richtig. Für
Befunde aus Reviews gilt zusätzlich der nächste Abschnitt.

Die mitgelieferten Commands im Überblick — Details in `k-playbook/docs/commands.md`:

- **Projekt** — `/k-gui` startet die Oberfläche.
- **Docs** — `/k-docs` prüft den Bestand und verzweigt; `/k-docs-code`,
  `/k-docs-tools` und `/k-docs-extract` erzeugen Doku, `/k-docs-index` baut den Index.
- **Review** — `/k-review` führt ein einzelnes Rezept aus, `/k-audit` einen
  vollständigen Sweep, `/k-pr-review` bewertet einen Pull Request, `/k-remediation`
  überführt Befunde in Tasks oder Fixes.
- **Task-Flow** — `/k-task-create`, `/k-task-refine`, `/k-task-run`; `/k-todo` pflegt
  `k-playbook-local/TODO.md`.
- **Hilfen** — `/k-enforcement` prüft gegen die effektive Regelmenge, `/k-test-check`
  führt Tests aus und diagnostiziert Fehler, `/k-verlauf` durchsucht alte AI-Verläufe,
  `/k-vscode-project-color` setzt Fensterfarbe und Titel.

Was in einem konkreten Projekt davon gilt, steht im Katalog aus `context` — ein
projekteigener Command ersetzt den gleichnamigen mitgelieferten oder schaltet ihn ab.

Die Oberfläche startet ohne Argument:

```bash
k-playbook/bin/k-playbook
```

Das ist der Weg, den ein Nutzer selbst geht: Konfiguration, Status der projekteigenen
Struktur, Assistenten-Verlinkung und weitere Hilfe stehen dort. `/k-gui` startet
dieselbe Oberfläche aus dem Assistenten heraus — der Weg, auf dem die KI sie von sich
aus anbieten und bei Bedarf öffnen kann, wenn sie das für sinnvoll hält.

## Umgang mit Befunden

`remediation.mode` in der `K-PLAYBOOK.yaml` legt fest, wie Befunde aus Reviews
abgearbeitet werden:

- `task-branch-pr` — keine direkten Fixes; jedes bestätigte Bündel wird eine Task mit
  Branch- und PR-Hinweis.
- `task-first` — Tasks sind der Standard; direkte Fixes nur nach ausdrücklicher
  Freigabe einzelner kleiner Bündel.
- `direct-allowed` — kleine, sichere Befunde dürfen nach Code-Sichtung direkt behoben
  werden.

Steht nichts in der Datei, gilt `task-first`.
