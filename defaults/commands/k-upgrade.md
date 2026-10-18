# k-upgrade

Später holt dieser Command eine neue Version von k-playbook und zieht danach die
Struktur des Projekts nach — etwa lokale Verzeichnisse entfernen, deren Inhalt in die
Collection gezogen ist.

Heute tut er nur eines: Er nennt die installierte Version und sagt, dass noch nichts
nachzuziehen ist.

## Schritt 1 — Version nennen

Rufe `k-playbook context` auf, oder nimm die Ausgabe, wenn sie in dieser Sitzung schon
vorliegt. Entnimm daraus `playbook.dir` und lies dort die Datei `VERSION`. Gib die
Zeile daraus aus, etwa so:

```text
k-playbook v0.16.3 ist installiert. Es ist nichts nachzuziehen.
```

Mehr ist nicht zu tun: kein Download, keine Änderung an Dateien.

## Hinweis

Nach einem Wechsel des Binaries muss die Sitzung neu gestartet werden: Der laufende
MCP-Server behält das alte Binary, bis der Assistent ihn neu startet.
