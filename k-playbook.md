# k-playbook

Diese Datei gilt in jedem Projekt, das k-playbook nutzt. Sie wird mitgeliefert und
bei jedem Update ersetzt. Was nur fuer ein einzelnes Projekt gilt, steht in
`k-playbook-local/k-playbook.md` und wird nach dieser Datei gelesen.

## Zwei Verzeichnisse, zwei Eigentuemer

```text
<projekt>/
├── K-PLAYBOOK.yaml       der Anker; sein Ort bestimmt das Hauptverzeichnis
├── k-playbook/           die Installation
└── k-playbook-local/     alles Projekteigene
```

**In `k-playbook/` wird nie geschrieben.** Das Verzeichnis ist ein Clone und wird bei
jedem Update vollstaendig ersetzt — eine Aenderung dort ist beim naechsten `git pull`
verloren. Wer eine mitgelieferte Datei anpassen will, legt eine gleichnamige in
`k-playbook-local/` an.

Alles, was das Projekt selbst hervorbringt, gehoert nach `k-playbook-local/`: eigene
Regeln, Reviews und Checks, Ergebnisse, Tasks, Notizen.

## Ueberlagern und Abschalten

Fuer `rules`, `reviews` und `checks` gilt: Eine gleichnamige Datei in
`k-playbook-local/` **ersetzt** die mitgelieferte vollstaendig. Die mitgelieferte wird
dann gar nicht gelesen; es werden auch keine einzelnen Abschnitte uebernommen.

Eine **leere** lokale Datei — nichts ausser Leerzeilen und Kommentaren — schaltet den
mitgelieferten Eintrag ab. So kann die Datei ihren eigenen Grund tragen:

```bash
# Abgeschaltet: dieses Projekt nutzt kein Django.
```

Bei `rules` und `reviews` bleibt der Eintrag sichtbar, sein Inhalt sagt dann, dass er
abgeschaltet ist. Ein Check faellt ganz aus dem Katalog: ein leeres Skript wuerde mit
Exit 0 durchlaufen und wie ein bestandener Check aussehen.

## Wo was liegt

Verlass dich nicht auf Annahmen ueber Pfade. `k-playbook/bin/k-playbook context`
liefert die aufgeloesten Verzeichnisse und die effektiven Kataloge — mitgeliefert und
projekteigen bereits zusammengefuehrt, abgeschaltete Eintraege markiert.

## Umgang mit Befunden

`remediation.mode` in der `K-PLAYBOOK.yaml` legt fest, wie Befunde aus Reviews
abgearbeitet werden:

- `task-branch-pr` — keine direkten Fixes; jedes bestaetigte Buendel wird eine Task mit
  Branch- und PR-Hinweis.
- `task-first` — Tasks sind der Standard; direkte Fixes nur nach ausdruecklicher
  Freigabe einzelner kleiner Buendel.
- `direct-allowed` — kleine, sichere Befunde duerfen nach Code-Sichtung direkt behoben
  werden.

Steht nichts in der Datei, gilt `task-first`.
