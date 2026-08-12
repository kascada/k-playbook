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
