# FAQ

## Wann rufe ich `/k-gui` auf?

`/k-gui` startet die Oberflaeche. Sinnvoll ist das:

- nach dem Klonen von k-playbook in ein Projekt, fuer die drei Einrichtungsschritte.
- nach einem `git pull`, wenn neue Commands oder Skills dazugekommen sind und die
  Verlinkung nachgezogen werden soll.
- wenn `/k-status` fehlende Teile der projekteigenen Struktur oder kaputte Symlinks meldet.

Wenn du nur wissen willst, ob alles stimmt, nimm `/k-status`. Der Command ist read-only
und repariert nichts.

## Muss ich `/k-gui` in einem bestimmten Verzeichnis aufrufen?

Am besten im Projekt, das du meinst. Das Werkzeug sucht ab dem Arbeitsverzeichnis
aufwaerts nach `K-PLAYBOOK.yaml` und nimmt den ersten Fund als Hauptverzeichnis.

Findet es nichts — etwa direkt nach dem Clone — dann raet es nicht, sondern schlaegt
einen Ort vor und laesst ihn bestaetigen. Der staerkste Hinweis ist dabei das
Git-Repository, in dem der Aufruf stattfindet: wer das Werkzeug startet, steht in aller
Regel in dem Projekt, das er meint.

## Ich habe mehrere Projekte. Muss ich k-playbook mehrfach installieren?

Ja, und das ist Absicht. Jedes Projekt bekommt seinen eigenen Clone unter
`<projekt>/k-playbook/`.

Der Vorteil: Projekte koennen unterschiedliche Staende tragen. Ein Projekt, das gerade
nicht angefasst wird, bleibt auf seinem Stand, und ein Update in einem anderen Projekt
aendert daran nichts. Es gibt keine zentrale Installation, die alle gleichzeitig
betrifft, und keinen Hostpfad, der auf allen Rechnern stimmen muss.

## Das Verzeichnis muss `k-playbook` heissen?

Ja. Commands und Skills sprechen es so an. Wie das Projektverzeichnis darueber heisst,
spielt dagegen keine Rolle.

```bash
git clone git@github.com:kascada/k-playbook.git k-playbook
```

Das Argument hinter der URL bestimmt den Namen.

## Warum liegt `K-PLAYBOOK.yaml` nicht in `k-playbook/`?

Weil `k-playbook/` bei jedem Update vollstaendig ersetzt wird. Alles, was dem Projekt
gehoert, muss daneben liegen — sonst waere es nach dem naechsten `git pull` weg.

Deshalb liegen `K-PLAYBOOK.yaml` und `k-playbook-local/` im Hauptverzeichnis:

```text
projekt/
├── K-PLAYBOOK.yaml        der Anker
├── k-playbook/            ersetzbar
└── k-playbook-local/      projekteigen
```

## Wo stehen die Pfade fuer Tasks, Reviews und Ergebnisse?

Nirgends — sie ergeben sich aus dem Ort der `K-PLAYBOOK.yaml`. Tasks liegen unter
`k-playbook-local/tasks/`, Ergebnisse unter `k-playbook-local/results/`, mitgelieferte
Regeln unter `k-playbook/rules/`.

Fruehere Versionen hatten dafuer einen `paths:`-Block mit neun Schluesseln. Der ist
entfallen: die Struktur ist fest, und ein Schluessel mit immer demselben Wert waere nur
eine Fehlerquelle. Die vollstaendige Zuordnung steht in
[`k-playbook-format.md`](./k-playbook-format.md).

## Wie aendere ich eine mitgelieferte Regel?

Gar nicht — `k-playbook/` wird beim Update ersetzt. Stattdessen legst du eine Datei mit
demselben Namen unter `k-playbook-local/` an:

```text
k-playbook/rules/docs-sync.md          wird dann nicht mehr gelesen
k-playbook-local/rules/docs-sync.md    gilt allein
```

Die lokale Datei ersetzt die mitgelieferte **vollstaendig**. Sie muss die Regel also
ganz enthalten; einzelne Abschnitte werden nicht aus dem Original uebernommen. Der Preis
davon ist, dass spaetere Verbesserungen am Original diese Kopie nicht mehr erreichen.

Dasselbe gilt fuer `reviews/` und `checks/`.

## Wie schalte ich eine mitgelieferte Regel ab?

Mit einer **leeren** Datei desselben Namens. Da die lokale Datei die mitgelieferte
vollstaendig ersetzt, bleibt bei einer leeren nichts uebrig.

„Leer" heisst: nichts ausser Leerzeilen und Kommentaren. Damit kann die Datei ihren
eigenen Grund tragen:

```bash
# k-playbook-local/checks/check_django_baseline.sh
# Abgeschaltet: dieses Projekt nutzt kein Django.
```

Bei `rules` und `reviews` bleibt der Eintrag im Katalog sichtbar, sein Inhalt sagt dann,
dass er abgeschaltet ist. Ein Check faellt ganz aus dem Katalog — ein leeres Skript liefe
mit Exit 0 durch und saehe aus wie ein bestandener Check.

Eine Liste in der Konfiguration gibt es dafuer nicht.

## Woher weiss ich, was am Ende gilt?

```bash
k-playbook/bin/k-playbook context
```

Gibt den aufgeloesten Arbeitsstand als JSON aus: Verzeichnisse, Instruktionsdateien in
Lesereihenfolge, Remediation-Policy, Guidelines und die drei Kataloge — mitgeliefert und
projekteigen bereits zusammengefuehrt, mit Herkunft je Eintrag und markierten
Abschaltungen.

Die Oberflaeche zeigt dasselbe lesbar aufbereitet, im Block `Aufgeloester Kontext`.

## Was ist `k-playbook.md`?

Die Instruktionsdatei — was ein Assistent vor der Arbeit lesen soll. Es gibt sie zweimal:

| Datei | Gilt fuer | Beim Update |
|---|---|---|
| `k-playbook/k-playbook.md` | jedes Projekt, das k-playbook nutzt | wird ersetzt |
| `k-playbook-local/k-playbook.md` | nur dieses Projekt | bleibt |

Gelesen wird in dieser Reihenfolge. In die projekteigene Ebene gehoert, was nur hier
gilt: Aufbau und Besonderheiten des Projekts, Konventionen, wiederkehrende Ablaeufe.
Allgemeine k-playbook-Regeln nicht — die stehen in der mitgelieferten Ebene und werden
bei jedem Update aktualisiert.

Sie heisst bewusst nicht `AGENTS.md`: diesen Namen lesen die Assistenten von sich aus, und
er ist dem Hauptverzeichnis vorbehalten. Dort steht nur ein kurzer Anstoss, der auf
`k-playbook context` verweist.

## Kann ich eigene Slash-Commands hinzufuegen?

Derzeit nicht. Es gibt kein `k-playbook-local/commands/`.

Der Grund ist die Verlinkung: `.claude/commands` ist ein einzelner Symlink auf
`k-playbook/commands`, und ein Symlink kann nur auf eine Quelle zeigen. Gaebe es beide
Verzeichnisse, muesste pro Datei verlinkt und nach jedem Update nachgezogen werden. Fuer
Skills gilt dasselbe.

## Darf beim Installieren von Security-Tools ein venv aktiv sein?

Nein. Sonst wird ein Tool aus dem Projekt-venv faelschlich als host-global vorhanden
erkannt. Wenn `VIRTUAL_ENV` gesetzt ist:

```bash
deactivate
```

Auch `.venv/bin`, `venv/bin` oder `env/bin` im `PATH` sind nicht erlaubt. Python-CLI-Tools
gehoeren in `pipx` oder in ein dediziertes k-playbook-Tool-venv unter
`~/.local/share/k-playbook/`, nicht in `<projekt>/.venv`.

## Wie installiere ich fehlende Security-Tools?

Die Oberflaeche zeigt den Status read-only. Installiert wird ueber das Skript:

```bash
k-playbook/scripts/install-security-tools.sh --install missing
```

Ohne `--yes` zeigt es den Plan und fragt. `--help` erklaert die Methoden `auto`,
`native`, `docker`, `pipx` und `venv`. Es schreibt keine Projektdateien und startet keine
Scans.

Einen eigenen `/k-install-security-tools`-Command gibt es nicht mehr — er haette das
Skript nur in Prosa gedoppelt und waere bei jeder Skriptaenderung nachzuziehen gewesen.

## Brauche ich Go?

Nein. `bin/k-playbook` ist ein Wrapper, der das zur Plattform passende Binary aus `dist/`
startet; die Binaries liegen fertig im Repo, fuer macOS und Linux.

Go brauchst du nur, wenn du am Werkzeug selbst arbeitest:

```bash
make dist
make gui
```
