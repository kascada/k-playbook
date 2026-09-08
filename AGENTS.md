# AGENTS.md

<!-- k-playbook:anstoss -->
## k-playbook

Für dieses Projekt gilt k-playbook. Rufe zu Beginn

    k-playbook context

auf und lies die Dateien aus `instructions` in der angegebenen Reihenfolge,
bevor du arbeitest. Die Ausgabe nennt außerdem die aufgelösten Verzeichnisse und
die effektiven Kataloge für Regeln, Reviews und Checks.

<!-- k-playbook:session-memory -->
## Projektwissen zuerst

Die autoritative Projektdokumentation beginnt bei
`k-playbook-local/docs/README.md`. Lies diesen Index zuerst, bevor du den
Code analysierst. Erst wenn die Dokumentation fehlt, nicht passt oder ein
konkreter Fix den aktuellen Code verlangt, ist eine Code-Recherche nötig.

<!-- k-playbook:befunde -->
## Befunde festhalten

Wenn du Code analysierst, einer Ursache nachgehst oder eine These durch einen
Test prüfst, ist die Arbeit erst abgeschlossen, wenn der Befund geschrieben ist.
Er gehört nach `k-playbook-local/material/befunde/`, eine Datei je Thema; neue
Erkenntnisse werden angehängt. Nenne in deiner Antwort die geschriebene Datei.

Knapp festhalten, sobald etwas belegt ist. Ausführlicher, sobald ein Problem
erkannt und gelöst wurde — dann auch, welche Wege du ausgeschlossen hast. Jeder
Eintrag nennt Befund, Beleg (`pfad:zeile` oder Testausgabe) und Sicherheit
(bestaetigt, unbestaetigt oder widerlegt). Das vollständige Format steht in
`k-playbook/rules/befunde.md`.

Nicht festgehalten werden Zwischenschritte und alles, was der Code selbst sagt.

Am Ende einer Sitzung schließt **/k-danke** die Arbeit ab: er legt die Befunde
vor, befördert Bestätigtes in die Dokumentation und prüft, ob die Doku
nachzuziehen ist.

