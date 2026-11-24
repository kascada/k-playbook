# k-upgrade

Zieht Kephalaion, Programm und Projekt auf den aktuellen Stand von k-playbook. Zuerst das, was
das Programm allein kann, in dieser Reihenfolge: Kephalaion — `kephalaion upgrade --auto`
aktualisiert es und startet seinen Node neu, soweit Kephalaion das selbst für gefahrlos hält,
sonst steht in der Ausgabe, was zu tun ist —, dann das Binary, wenn das laufende älter ist als
das neueste Release, dann `AGENTS.md` (Anstoß), `CLAUDE.md`, die Verlinkung der
Commands und Skills, die MCP-Einträge und in einem Projekt mit GitHub-Dienst die Berechtigungen der
Clients, die nacktes `gh` und die Netzbefehle von git sperren und den Weg über
`k-playbook github run` freigeben. Das tut der Unterbefehl `k-playbook upgrade`, und den führt die Hülle
dieses Commands schon beim Aufruf aus — vor dem ersten Schritt, damit das Briefing danach
den neuen Stand sieht und nicht das alte Programm die Verlinkung nach seinen Regeln
nachzieht. Danach, falls das Projekt eines mitbringt, sein eigenes Umbauskript
`scripts/k-upgrade-umbau.sh`. Danach die Umstellung, die eine Beurteilung braucht und deshalb die Sitzung
macht, nicht das Programm: das Löschen eines alten Clones `k-playbook/` samt der Zeilen, die
`k-playbook/` in einer `.gitignore` ausnehmen.
Der Command nimmt kein Argument.

## Schritt 1 — Briefing

include: _shared/erster-schritt.md

## Schritt 2 — Ausgabe des Nachziehens

Die Ausgabe von `k-playbook upgrade` steht in der Hülle dieses Commands, unter „Ausgabe von
`k-playbook upgrade`“. Gib sie unverändert wieder — jede Zeile, auch Hinweise und
Fehlermeldungen wie „Download/Install nicht möglich“. Fasse nichts zusammen und lass nichts
weg. In diesem Schritt führst du nichts anderes aus und änderst keine Datei. Was der Lauf
nicht von selbst nachziehen konnte, steht in der Ausgabe als Hinweis; nenne ihn, behebe ihn
nicht. Das gilt auch für die Zeilen zu den GitHub-Berechtigungen: Ein „Widerspruch in …“ ist
ein Eintrag des Projekts mit Vorschlag; ob er entfernt wird, entscheidet der Nutzer.

Steht dort keine Ausgabe von `k-playbook upgrade` — die Zeile ist leer, nennt nur den Befehl
selbst oder meldet, dass er nicht gefunden oder nicht ausgeführt wurde —, dann hat der
Assistent die Einbettung nicht ausgeführt. Führe dann in der Shell aus:

    K_PLAYBOOK_VIA_COMMAND=1 k-playbook upgrade

und gib diese Ausgabe nach derselben Regel wieder. Nenne dazu in einer Zeile, dass die Hülle
den Aufruf nicht ausgeführt hat. Hülle und Ersatzaufruf setzen `K_PLAYBOOK_VIA_COMMAND=1`,
damit der Hinweis entfällt, den `k-playbook upgrade` im Terminal nach einer neuen Version
gibt — `/k-upgrade` aufzurufen —; hier läuft `/k-upgrade` schon.

**Projekteigener Umbau.** Liegt im Projekt die Datei
`<project.dir>/scripts/k-upgrade-umbau.sh`, führst du sie direkt danach aus, mit
`project.dir` als Arbeitsverzeichnis:

    bash scripts/k-upgrade-umbau.sh

Das Skript gehört dem Projekt und baut um, was nur dieses Projekt betrifft. Es läuft vor
Schritt 3 und ohne Rückfrage; die Bestätigungsregel aus Schritt 3 gilt für das Skript nicht. Gib seine Ausgabe nach derselben
Regel wie oben unverändert wieder. Endet es mit einem Exit-Code ungleich 0, nenne den Code
und frag, ob Schritt 3 trotzdem laufen soll; die Umstellungen auf einem halb umgebauten
Projekt laufen zu lassen, entscheidet der Nutzer. Fehlt die Datei, meldest du nichts dazu.
Schritt 3 stellt seinen Bestand erst danach fest, also nach dem, was das Skript gelöscht
oder verschoben hat.

Steht in der Ausgabe, dass die Sitzung neu zu starten ist — weil das Programm gewechselt hat
oder weil Kephalaion aktualisiert bzw. sein Node neu gestartet wurde —, endet der Command hier
mit genau diesem Hinweis: Der laufende MCP-Server behält das alte Binary, bis der Assistent ihn
neu startet, und die Verbindung zu einem neu gestarteten Node ist danach nicht verlässlich.
Füge hinzu, dass die Umstellungen aus Schritt 3 danach mit einem zweiten Lauf von `/k-upgrade`
folgen; 3.8 setzt voraus, dass Schritt 2 das laufende Programm als neuestes Release meldet,
und das tut nach einem Programmwechsel erst der zweite Lauf.

Läufst du in OpenCode und meldet die Ausgabe „OpenCode: alle MCP-Server des Dienstes neu
geladen“ und außer dem Programmwechsel keinen anderen Grund für einen Neustart, insbesondere
keinen von Kephalaion, ist kein Neustart der Sitzung nötig: Der Reload hat den MCP-Server
dieser Sitzung schon aus dem neuen Programm gestartet. Der Command endet trotzdem hier; statt
des Neustart-Hinweises sagst du, dass der zweite Lauf von `/k-upgrade` in derselben Sitzung
folgen kann. Meldet der Lauf noch einen anderen Neustart-Grund, bleibt es beim Neustart der
Sitzung. Ohne diese Bedingung gilt der Absatz davor, in Claude Code also weiter der Hinweis auf
den Neustart, auch wenn die Zeile „neu geladen“ dasteht: Sie betrifft nur den OpenCode-Dienst
desselben Benutzers.

## Schritt 3 — Umstellungen

Jede Umstellung prüft zuerst ihren Anlass. Liegt er nicht vor, gehst du ohne Meldung zur
nächsten; eine erledigte Umstellung wird so beim nächsten Lauf von selbst übersprungen.
`/k-upgrade` läuft oft, und eine Zeile „nichts zu tun“ je Umstellung sagt dann nichts mehr.
Gemeldet wird nur, was zu tun ist: was eine Umstellung vorlegt, ausführt oder liegen lässt,
dazu jede Warnung, etwa dass sie auf den Binärwechsel wartet. Liegt bei keiner Umstellung ein
Anlass vor, steht für Schritt 3 genau eine Zeile: `Umstellungen: alles erledigt.`

Für alle Umstellungen gilt:

- Gelöscht, verschoben oder überschrieben wird erst nach Bestätigung. Lege je Umstellung
  gebündelt vor, was du vorhast, und warte auf die Antwort. Ausgenommen ist ein Fall, und nur
  er: der Clone `k-playbook/`, den 3.8 löscht, samt der Zeilen, die 3.8 streicht, auch in
  einer versionierten `.gitignore`. 3.8 löscht und ändert ohne Vorlage und ohne auf eine
  Antwort zu warten.
- Bist du bei einem Eintrag unsicher, sag das in der Vorlage ausdrücklich, statt still zu
  entscheiden; in 3.8, das nichts vorlegt, steht es in der Meldung, ohne neue Rückfrage.
- Braucht eine Umstellung die Projekt-Collection und steht `kephalaion.ok` nicht auf
  `true`, wird sie übersprungen; sag das und nenne den Grund aus `kephalaion.message`.

Zurzeit gibt es eine Umstellung, 3.8; sie behält ihre Nummer, weil Programm und Doku auf sie
verweisen.

### 3.8 — Clone `k-playbook/` und die `.gitignore`-Zeilen

Bis v0.16.38 brauchte ein Projekt eine Kopie des k-playbook-Repos von GitHub unter
`<project.dir>/k-playbook`: Das Programm zog sie nach und aktualisierte sich aus ihr.
Heute kommt alles mit dem Programm; es liest den Clone nicht mehr und zieht ihn nicht mehr
nach. Diese Umstellung löscht den Clone ganz, ohne Rückfrage, und streicht danach aus den
`.gitignore`-Dateien die Zeilen, die `k-playbook/` aus Git heraushalten — jede erst, wenn das
Verzeichnis weg ist, damit nichts davon versehentlich in einen Commit gerät. Sie braucht die
Collection nicht.

**Anlass:** `<project.dir>/k-playbook` liegt, als Verzeichnis oder als Symlink, oder
eine `.gitignore` von `project.dir` bis `project.repoRoot` trägt eine der
Zeilen aus d) für ein Verzeichnis, das nicht mehr existiert.

**Vorbedingung:** Schritt 2 hat die Zeile `Programm-Stand: nichts zu tun — …` ausgegeben, und
sie nennt `neuestes Release`. So schreibt sie erst ein Programm ab v0.16.39. Fehlt die Zeile
oder nennt sie stattdessen `Installation`, löschst und änderst du nichts und meldest
`k-playbook/ löschen wartet auf den Binärwechsel`: Ein älteres Programm schreibt die Zeile mit
`Installation`, bekommt diesen Text aber über den Hub zu lesen und braucht den Clone für sein
eigenes Upgrade.

In allen Befehlen setzt du den Pfad in die doppelten Anführungszeichen ein; enthält er selbst
`"`, `$`, `` ` `` oder `\`, steht davor ein `\`.

**a) Prüfen, ob es ein Clone ist.** Liegt `<project.dir>/k-playbook` nicht, gehst du
zu d). Sonst:

```sh
test -L "<project.dir>/k-playbook" && echo SYMLINK
test -d "<project.dir>/k-playbook/.git" && ! test -L "<project.dir>/k-playbook/.git" && echo CLONE
```

Ohne Rückfrage gelöscht wird nur ein echtes Verzeichnis, kein Symlink, in dem direkt ein
Verzeichnis `.git` liegt: Nur dann ist es eine Kopie von GitHub. In jedem anderen Fall löschst
du nichts, weder das Verzeichnis noch einen Link noch sein Ziel, meldest in einer Zeile, warum
es liegen bleibt, und gehst zu d):

- ein Symlink: `k-playbook/ ist ein Symlink, nichts gelöscht`;
- kein `.git` darin, oder `.git` ist eine Datei wie in einem Submodul oder Worktree, oder
  `.git` ist ein Symlink: `k-playbook/ ist kein Clone, nichts gelöscht: <grund>`.

**b) Nennen, was nicht vom Repo kam.** Ein Clone wird ganz gelöscht, auch mit unverfolgten,
geänderten oder ignorierten Dateien und mit eigenen Commits darin. Vorher erhebst du, was
davon verloren geht:

```sh
git -C "<project.dir>/k-playbook" status --porcelain --ignored || echo FEHLGESCHLAGEN
git -C "<project.dir>/k-playbook" rev-list --count HEAD --branches --not --remotes || echo FEHLGESCHLAGEN
```

Jede Zeile des ersten Befehls ist eine Datei oder ein Verzeichnis, das nicht vom Repo kam; der
zweite nennt die Zahl der Commits, die auf keinem Remote liegen. Meldet einer der beiden
Befehle `FEHLGESCHLAGEN` — er endet mit einem anderen Exit-Status als 0, auch weil `git`
fehlt —, löschst du nichts, meldest
`k-playbook/ ließ sich nicht prüfen, nichts gelöscht: <fehler>` — `<fehler>` ist die
Fehlermeldung, gekürzt auf eine Zeile — und gehst zu d).

Gibt der erste Befehl etwas aus oder ist die Zahl größer als 0, nennst du das, bevor du
löschst: jede Zeile des ersten Befehls, auch wenn es viele sind, und die Zahl der Commits. Das
ist eine Meldung, keine Vorlage: Du wartest auf keine Antwort, und eine Rückfrage gibt es auch
dann nicht.

**c) Löschen.** Frühere Versionen haben dem Clone den Schreibzugriff entzogen; deshalb gibst du
ihn zuerst frei und löschst dann das Verzeichnis samt Inhalt im Dateisystem:

```sh
chmod -R u+w -- "<project.dir>/k-playbook"
rm -rf -- "<project.dir>/k-playbook"
```

Liegt das Verzeichnis danach noch, meldest du den Fehlschlag mit der Fehlermeldung; seine
Zeilen in d) bleiben dann stehen. Committet wird nicht.

**d) Die `.gitignore`-Zeilen streichen.** Durchsucht wird jede Datei `.gitignore` in den
Verzeichnissen von `project.dir` aufwärts bis `project.repoRoot`, beide
eingeschlossen. Liegt `project.repoRoot` nicht oberhalb von `project.dir` und
ist es auch nicht dasselbe Verzeichnis, bleibt es bei der `.gitignore` in
`project.dir`. Gemeint sind nur Zeilen, die genau dieses Verzeichnis treffen;
verglichen wird die ganze Zeile, Leerraum am Zeilenende zählt nicht mit:

- `k-playbook`, `/k-playbook`, `k-playbook/`, `/k-playbook/`, `**/k-playbook` und
  `**/k-playbook/`;
- in einer `.gitignore` oberhalb von `project.dir` dazu die Formen mit dem Pfad von
  ihrem Verzeichnis bis zum Projektverzeichnis, etwa `<projektpfad>/k-playbook/`, mit und ohne
  `/` vorn, mit und ohne `/` hinten.

Kommentare und alle anderen Muster bleiben stehen, auch eine Verneinung (`!k-playbook/`), ein
Muster mit Platzhalter (`k-playbook*`) oder ein Pfad darunter (`k-playbook/dist/`).

Je Zeile gilt, nach dem Stand, den c) hinterlassen hat:

- Ihr Verzeichnis ist `<project.dir>/k-playbook`. Existiert es nicht oder nicht mehr,
  gleich wer es gelöscht hat, fällt die Zeile. Steht es noch — auch als Symlink oder weil a) bis c) es
  liegen ließen —, bleibt auch die Zeile, und die Meldung sagt warum.
- Eine Zeile in einer `.gitignore` oberhalb von `project.dir` bleibt auch dann, wenn
  sie unter `project.repoRoot` noch ein anderes vorhandenes Verzeichnis trifft, etwa
  den Clone eines zweiten Projekts; die Meldung nennt die Zeile und dieses Verzeichnis. Eine
  Zeile ohne Pfad und ohne `/` vorn trifft jedes Verzeichnis dieses Namens unterhalb ihrer
  `.gitignore`, eine mit `/` vorn oder mit Pfad nur das eine ab ihrer `.gitignore`. Für die
  erste Form suchst du:

  ```sh
  find "<verzeichnis der .gitignore>" -name .git -prune -o \( -type d -o -type l \) -name "k-playbook" -print
  ```

Die fallenden Zeilen streichst du mit dem Edit-Werkzeug, ohne Vorlage und ohne Rückfrage, auch
in einer versionierten `.gitignore`; sonst änderst du an der Datei nichts, und sie bleibt
liegen, auch wenn sie danach leer ist. Committet wird nicht. Ob eine geänderte `.gitignore`
versioniert ist, sagt `git ls-files --error-unmatch <pfad>` in ihrem Verzeichnis.

**Melden.** 3.8 meldet in Schritt 3 für den Clone genau eine dieser Zeilen, wenn
`<project.dir>/k-playbook` lag oder die Vorbedingung fehlt:

- `k-playbook/ gelöscht` — darunter, eingerückt, jede Zeile aus b), die nicht vom Repo kam, und
  die Zahl der Commits, die auf keinem Remote lagen, wenn sie größer als 0 ist;
- `k-playbook/ ist ein Symlink, nichts gelöscht`;
- `k-playbook/ ist kein Clone, nichts gelöscht: <grund>`;
- `k-playbook/ ließ sich nicht prüfen, nichts gelöscht: <fehler>`;
- `k-playbook/ löschen wartet auf den Binärwechsel`.

Dazu je `.gitignore`, in der eine der Zeilen steht, eine Zeile mit ihrem Pfad ab
`project.repoRoot`: was gestrichen wurde und was bleibt, mit dem Verzeichnis, das die
Zeile hält. Wurde eine versionierte `.gitignore` geändert, folgt eine Zeile: Sie steht jetzt
als geändert in `git status` und gehört in den nächsten Commit des Projekts; andere Checkouts
lassen `/k-upgrade` laufen, bevor sie mit `git add -A` committen — wer diesen Commit holt und
noch einen Clone hat, findet `k-playbook/` sonst als unverfolgtes Verzeichnis im Arbeitsbaum.

```text
Clone k-playbook/ und .gitignore
  k-playbook/ gelöscht — darin lag, was nicht vom Repo kam:
    ?? notizen.md
     M Makefile
    !! dist/
    2 Commits auf keinem Remote
  dienste/erstes/.gitignore: gestrichen /k-playbook/
  .gitignore: bleibt k-playbook/ — trifft noch dienste/zweites/k-playbook
  dienste/erstes/.gitignore ist versioniert und steht als geändert in git status; andere Checkouts lassen /k-upgrade laufen, bevor sie mit git add -A committen
```

Das Beispiel stammt aus einem Projekt in `dienste/erstes/` eines Repos, in dem ein zweites
Projekt seinen Clone noch hat.

Die Umstellung ist wiederholbar: Eine Zeile, die bleibt, streicht der nächste Lauf, sobald ihr
Verzeichnis weg ist.

## Schritt 4 — Schluss

Nenne nur die Umstellungen, die etwas getan haben, bei denen etwas liegen geblieben ist oder die
eine Warnung tragen, je eine Zeile; eine Umstellung ohne Anlass erscheint nicht. Trifft das auf
keine zu, steht hier nur `Umstellungen: alles erledigt.`, dazu Hinweise aus Schritt 2. Für 3.8
die Zeilen aus Schritt 3: `k-playbook/ gelöscht` mit jeder Zeile dessen, was darin nicht vom
Repo kam, und der Zahl der Commits ohne Remote, dass es ein Symlink oder kein Clone ist und
nichts gelöscht wurde, dass es sich nicht prüfen ließ und nichts gelöscht wurde, oder
`k-playbook/ löschen wartet auf den Binärwechsel`; dazu je `.gitignore` die gestrichenen und
die gebliebenen Zeilen mit Grund und, wenn eine versionierte `.gitignore` geändert wurde, die
Zeile zu `git status` und den anderen Checkouts.
