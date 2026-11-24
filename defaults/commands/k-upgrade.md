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
`scripts/k-upgrade-umbau.sh`. Danach die Umstellungen, die eine Beurteilung brauchen und
deshalb die Sitzung macht, nicht das Programm; zurzeit gibt es keine.
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
folgen: Erst das neue Programm liefert ihren aktuellen Text.

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
dazu jede Warnung. Liegt bei keiner Umstellung ein Anlass vor, steht für Schritt 3 genau eine
Zeile: `Umstellungen: alles erledigt.`

Für alle Umstellungen gilt:

- Gelöscht, verschoben oder überschrieben wird erst nach Bestätigung. Lege je Umstellung
  gebündelt vor, was du vorhast, und warte auf die Antwort.
- Bist du bei einem Eintrag unsicher, sag das in der Vorlage ausdrücklich, statt still zu
  entscheiden.
- Braucht eine Umstellung die Projekt-Collection und steht `kephalaion.ok` nicht auf
  `true`, wird sie übersprungen; sag das und nenne den Grund aus `kephalaion.message`.

Zurzeit gibt es keine Umstellung: Schritt 3 meldet `Umstellungen: alles erledigt.`

## Schritt 4 — Schluss

Nenne nur die Umstellungen, die etwas getan haben, bei denen etwas liegen geblieben ist oder die
eine Warnung tragen, je eine Zeile; eine Umstellung ohne Anlass erscheint nicht. Trifft das auf
keine zu, steht hier nur `Umstellungen: alles erledigt.`, dazu Hinweise aus Schritt 2.
