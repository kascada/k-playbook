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

## Stand

Das Werkzeug ist auf ein leeres Gerüst zurückgesetzt: `bin/k-playbook` startet die lokale
GUI und zeigt eine leere Seite mit Titel. Der bisherige Go-Code liegt unter
`installer/_old/` als Nachschlagewerk — von der Go-Toolchain ignoriert und nicht baubar.

## Offen

- `make install` und `scripts/install-installer.sh` folgen noch dem alten Modell und
  suchen ein `k-playbook-installer`-Binary.
