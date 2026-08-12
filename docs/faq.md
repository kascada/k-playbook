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

Ja. `k-playbook-local/commands/` nimmt sie auf, `k-playbook-local/skills/` die eigenen
Skills. Es gilt dieselbe Regel wie bei Regeln und Reviews: gleicher Name ersetzt den
mitgelieferten, ein leerer Eintrag schaltet ihn ab.

```text
k-playbook-local/commands/k-eigen.md    neu, nur in diesem Projekt
k-playbook-local/commands/k-todo.md     ersetzt den mitgelieferten
k-playbook-local/skills/mein-skill/SKILL.md
```

Danach die Oberflaeche starten (`/k-gui`) und im Assistenten-Block auf **Einrichten**
druecken — der neue Command wird nicht von selbst registriert. Anschliessend den
Assistenten neu starten, der erfasst Commands beim Start.

Bei Commands zaehlt der Pfad ab `commands/`: eine lokale `commands/_shared/x.md` ersetzt
genau diese Datei, der Rest des Namensraums bleibt mitgeliefert. Ein Skill wird dagegen
als Ganzes ersetzt — `SKILL.md` und Beiwerk muessen zueinander passen.

## Warum sind `.claude/commands/` und `.opencode/commands/` voller Symlinks?

Weil die Eintraege aus zwei Quellen kommen. Ein Verzeichnis-Symlink zeigt auf genau eine —
damit kaeme entweder nur `k-playbook/` oder nur `k-playbook-local/` an. Deshalb ist das
Ziel ein echtes Verzeichnis mit je einem Link pro Command bzw. Skill, der auf die Fassung
zeigt, die nach der Overlay-Regel gilt.

Aeltere Installationen haben dort noch einen einzelnen Verzeichnis-Symlink. Die
Oberflaeche erkennt ihn und baut ihn beim Einrichten um.

Eine **echte Datei**, die du selbst dort abgelegt hast, wird nie ersetzt. Sie gewinnt, und
die Oberflaeche weist sie als projekteigen aus.

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

## Was ist mit CodeQL?

Der Zweig ist vollstaendig entfallen: die Commands `/k-setup-codeql` und
`/k-install-codeql`, das Skript `install-codeql-local.sh`, die Regel `rules/codeql.md`,
das Rezept `review-codeql-security.md`, der `codeql`-Modus von `/k-status` und der Block
`tools.codeql`.

Er passte nicht mehr zur Struktur: er schrieb `tools.codeql` direkt in die
`K-PLAYBOOK.yaml` statt ueber die `context`-Ausgabe zu gehen, und er legte CLI,
Datenbanken und SARIF unter `k-playbook/` ab — also in dem Verzeichnis, das jedes Update
ersetzt. Ein Umbau haette die CLI-Beschaffung an `tools.gh` haengen, das Scannen in ein
eigenes Subkommando verlagern, der Datenbank einen Ort samt Aktualitaetspruefung geben und
ihren Zustand in `context` melden muessen. Solange die uebrigen Scan-Familien noch nicht
umgestellt sind, steht dieser Aufwand in keinem Verhaeltnis zum Nutzen.

Ein spaeterer Wiedereinstieg ist ueber die Git-Historie moeglich.

## Brauche ich Go?

Nein. `bin/k-playbook` ist ein Wrapper, der das zur Plattform passende Binary aus `dist/`
startet; die Binaries liegen fertig im Repo, fuer macOS und Linux.

Go brauchst du nur, wenn du am Werkzeug selbst arbeitest:

```bash
make dist
make gui
```
