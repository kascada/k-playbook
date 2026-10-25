## Befunde festhalten

Wenn du Code analysierst, einer Ursache nachgehst oder eine These durch einen Test prüfst,
ist die Arbeit erst abgeschlossen, wenn der Befund geschrieben ist. Jeder Befund ist ein
eigenes Dokument in `keph:/queue/findings/`, angelegt mit `k_playbook_doc_create`, Sorte
`findings`. Nenne in deiner Antwort den Verweis. Wie ein Befund aussieht und was beim Schreiben
gilt, steht in `k-playbook:/instructions/on-demand/befunde.md`; lies es vor dem ersten Befund
einer Sitzung.

Ein Befund wartet darauf, in die Wissensablage übernommen zu werden, und verschwindet danach.
Verweise deshalb nicht auf ihn. Der Stand eines laufenden Vorhabens, also Plan, offene Punkte
und Fortschritt, ist kein Befund; er gehört in die Workbench (Abschnitt „Die Workbench“).

Halte knapp fest, sobald etwas belegt ist. Ausführlicher wird es, sobald ein Problem erkannt
und gelöst ist; dann gehört auch dazu, welche Wege du ausgeschlossen hast. Jeder Befund nennt
Befund, Beleg (`pfad:zeile` oder Testausgabe) und Sicherheit (bestaetigt, unbestaetigt oder
widerlegt). Ist Kephalaion nicht erreichbar, sag das und gib den Befund vollständig in deiner
Antwort wieder.

Nicht festgehalten werden Zwischenschritte und alles, was der Code selbst sagt.

Am Ende einer Sitzung schließt **/k-danke** die Arbeit ab. Er legt die Befunde der Sitzung
vor, trägt Fehlendes nach, fragt, was aus ihnen werden soll, und prüft, ob die Doku
nachzuziehen ist.
