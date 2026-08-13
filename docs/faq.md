# FAQ

## Wann rufe ich `/k-gui` auf?

`/k-gui` startet die Oberfläche. Sinnvoll ist das:

- nach dem Klonen von k-playbook in ein Projekt, für die drei Einrichtungsschritte.
- nach einem `git pull`, wenn neue Commands oder Skills dazugekommen sind und die
  Verlinkung nachgezogen werden soll.
- wenn Teile der projekteigenen Struktur fehlen oder Symlinks kaputt sind.

Wenn du nur wissen willst, ob alles stimmt, öffne die Oberfläche und schau, ohne einen
der Schritte zu bestätigen — geschrieben wird erst nach Bestätigung.

## Muss ich `/k-gui` in einem bestimmten Verzeichnis aufrufen?

Am besten im Projekt, das du meinst. Das Werkzeug sucht ab dem Arbeitsverzeichnis
aufwärts nach `K-PLAYBOOK.yaml` und nimmt den ersten Fund als Hauptverzeichnis.

Findet es nichts — etwa direkt nach dem Clone — dann rät es nicht, sondern schlägt
einen Ort vor und lässt ihn bestätigen. Der stärkste Hinweis ist dabei das
Git-Repository, in dem der Aufruf stattfindet: wer das Werkzeug startet, steht in aller
Regel in dem Projekt, das er meint.

## Ich habe mehrere Projekte. Muss ich k-playbook mehrfach installieren?

Ja, und das ist Absicht. Jedes Projekt bekommt seinen eigenen Clone unter
`<projekt>/k-playbook/`.

Der Vorteil: Projekte können unterschiedliche Stände tragen. Ein Projekt, das gerade
nicht angefasst wird, bleibt auf seinem Stand, und ein Update in einem anderen Projekt
ändert daran nichts. Es gibt keine zentrale Installation, die alle gleichzeitig
betrifft, und keinen Hostpfad, der auf allen Rechnern stimmen muss.

## Das Verzeichnis muss `k-playbook` heißen?

Ja. Commands und Skills sprechen es so an. Wie das Projektverzeichnis darüber heißt,
spielt dagegen keine Rolle.

```bash
git clone git@github.com:kascada/k-playbook.git k-playbook
```

Das Argument hinter der URL bestimmt den Namen.

## Warum liegt `K-PLAYBOOK.yaml` nicht in `k-playbook/`?

Weil `k-playbook/` bei jedem Update vollständig ersetzt wird. Alles, was dem Projekt
gehört, muss daneben liegen — sonst wäre es nach dem nächsten `git pull` weg.

Deshalb liegen `K-PLAYBOOK.yaml` und `k-playbook-local/` im Hauptverzeichnis:

```text
projekt/
├── K-PLAYBOOK.yaml        der Anker
├── k-playbook/            ersetzbar
└── k-playbook-local/      projekteigen
```

## Wo stehen die Pfade für Tasks, Reviews und Ergebnisse?

Nirgends — sie ergeben sich aus dem Ort der `K-PLAYBOOK.yaml`. Tasks liegen unter
`k-playbook-local/tasks/`, Ergebnisse unter `k-playbook-local/results/`, mitgelieferte
Regeln unter `k-playbook/rules/`.

Frühere Versionen hatten dafür einen `paths:`-Block mit neun Schlüsseln. Der ist
entfallen: die Struktur ist fest, und ein Schlüssel mit immer demselben Wert wäre nur
eine Fehlerquelle. Die vollständige Zuordnung steht in
[`k-playbook-format.md`](./k-playbook-format.md).

## Wie ändere ich eine mitgelieferte Regel?

Gar nicht — `k-playbook/` wird beim Update ersetzt. Stattdessen legst du eine Datei mit
demselben Namen unter `k-playbook-local/` an:

```text
k-playbook/rules/docs-sync.md          wird dann nicht mehr gelesen
k-playbook-local/rules/docs-sync.md    gilt allein
```

Die lokale Datei ersetzt die mitgelieferte **vollständig**. Sie muss die Regel also
ganz enthalten; einzelne Abschnitte werden nicht aus dem Original übernommen. Der Preis
davon ist, dass spätere Verbesserungen am Original diese Kopie nicht mehr erreichen.

Dasselbe gilt für `reviews/` und `checks/`.

## Wie schalte ich eine mitgelieferte Regel ab?

Mit einer **leeren** Datei desselben Namens. Da die lokale Datei die mitgelieferte
vollständig ersetzt, bleibt bei einer leeren nichts übrig.

„Leer" heißt: nichts außer Leerzeilen und Kommentaren. Damit kann die Datei ihren
eigenen Grund tragen:

```bash
# k-playbook-local/checks/check_django_baseline.sh
# Abgeschaltet: dieses Projekt nutzt kein Django.
```

Bei `rules` und `reviews` bleibt der Eintrag im Katalog sichtbar, sein Inhalt sagt dann,
dass er abgeschaltet ist. Ein Check fällt ganz aus dem Katalog — ein leeres Skript liefe
mit Exit 0 durch und sähe aus wie ein bestandener Check.

Eine Liste in der Konfiguration gibt es dafür nicht.

## Woher weiß ich, was am Ende gilt?

```bash
k-playbook/bin/k-playbook context
```

Gibt den aufgelösten Arbeitsstand als JSON aus: Verzeichnisse, Instruktionsdateien in
Lesereihenfolge, Remediation-Policy, Guidelines und die drei Kataloge — mitgeliefert und
projekteigen bereits zusammengeführt, mit Herkunft je Eintrag und markierten
Abschaltungen.

Die Oberfläche zeigt dasselbe lesbar aufbereitet, im Block `Aufgelöster Kontext`.

## Was ist `k-playbook.md`?

Die Instruktionsdatei — was ein Assistent vor der Arbeit lesen soll. Es gibt sie zweimal:

| Datei | Gilt für | Beim Update |
|---|---|---|
| `k-playbook/k-playbook.md` | jedes Projekt, das k-playbook nutzt | wird ersetzt |
| `k-playbook-local/k-playbook.md` | nur dieses Projekt | bleibt |

Gelesen wird in dieser Reihenfolge. In die projekteigene Ebene gehört, was nur hier
gilt: Aufbau und Besonderheiten des Projekts, Konventionen, wiederkehrende Abläufe.
Allgemeine k-playbook-Regeln nicht — die stehen in der mitgelieferten Ebene und werden
bei jedem Update aktualisiert.

Sie heißt bewusst nicht `AGENTS.md`: diesen Namen lesen die Assistenten von sich aus, und
er ist dem Hauptverzeichnis vorbehalten. Dort steht nur ein kurzer Anstoß, der auf
`k-playbook context` verweist.

## Kann ich eigene Slash-Commands hinzufügen?

Ja. `k-playbook-local/commands/` nimmt sie auf, `k-playbook-local/skills/` die eigenen
Skills. Es gilt dieselbe Regel wie bei Regeln und Reviews: gleicher Name ersetzt den
mitgelieferten, ein leerer Eintrag schaltet ihn ab.

```text
k-playbook-local/commands/k-eigen.md    neu, nur in diesem Projekt
k-playbook-local/commands/k-todo.md     ersetzt den mitgelieferten
k-playbook-local/skills/mein-skill/SKILL.md
```

Danach die Oberfläche starten (`/k-gui`) und im Assistenten-Block auf **Einrichten**
drücken — der neue Command wird nicht von selbst registriert. Anschließend den
Assistenten neu starten, der erfasst Commands beim Start.

Bei Commands zählt der Pfad ab `commands/`: eine lokale `commands/_shared/x.md` ersetzt
genau diese Datei, der Rest des Namensraums bleibt mitgeliefert. Ein Skill wird dagegen
als Ganzes ersetzt — `SKILL.md` und Beiwerk müssen zueinander passen.

## Warum sind `.claude/commands/` und `.opencode/commands/` voller Symlinks?

Weil die Einträge aus zwei Quellen kommen. Ein Verzeichnis-Symlink zeigt auf genau eine —
damit käme entweder nur `k-playbook/` oder nur `k-playbook-local/` an. Deshalb ist das
Ziel ein echtes Verzeichnis mit je einem Link pro Command bzw. Skill, der auf die Fassung
zeigt, die nach der Overlay-Regel gilt.

Ältere Installationen haben dort noch einen einzelnen Verzeichnis-Symlink. Die
Oberfläche erkennt ihn und baut ihn beim Einrichten um.

Eine **echte Datei**, die du selbst dort abgelegt hast, wird nie ersetzt. Sie gewinnt, und
die Oberfläche weist sie als projekteigen aus.

## Darf beim Installieren von Security-Tools ein venv aktiv sein?

Nein. Sonst wird ein Tool aus dem Projekt-venv fälschlich als host-global vorhanden
erkannt. Wenn `VIRTUAL_ENV` gesetzt ist:

```bash
deactivate
```

Auch `.venv/bin`, `venv/bin` oder `env/bin` im `PATH` sind nicht erlaubt. Python-CLI-Tools
gehören in `pipx` oder in ein dediziertes k-playbook-Tool-venv unter
`~/.local/share/k-playbook/`, nicht in `<projekt>/.venv`.

## Wie installiere ich fehlende Security-Tools?

Die Oberfläche zeigt den Status read-only. Installiert wird über das Skript:

```bash
k-playbook/scripts/install-security-tools.sh --install missing
```

Ohne `--yes` zeigt es den Plan und fragt. `--help` erklärt die Methoden `auto`,
`native`, `docker`, `pipx` und `venv`. Es schreibt keine Projektdateien und startet keine
Scans.

Einen eigenen `/k-install-security-tools`-Command gibt es nicht mehr — er hätte das
Skript nur in Prosa gedoppelt und wäre bei jeder Skriptänderung nachzuziehen gewesen.

## Brauche ich Go?

Nein. `bin/k-playbook` ist ein Wrapper, der das zur Plattform passende Binary aus `dist/`
startet; die Binaries liegen fertig im Repo, für macOS und Linux.

Go brauchst du nur, wenn du am Werkzeug selbst arbeitest:

```bash
make dist
make gui
```
