## Verweise

Hier und in allem, was k-playbook liefert, ist ein Verweis ein einziger Pfad mit Präfix. Das
Präfix sagt, wo der Inhalt liegt und wie du ihn liest.

| Präfix | Wo der Inhalt liegt | So liest du ihn |
|---|---|---|
| `project:/<pfad>` | Datei im Projekt, Pfad ab Projektwurzel | direkt von der Platte |
| `k-playbook:/<pfad>` | von k-playbook geliefert | MCP `k_playbook_doc_read` mit dem Verweis als `ref`; ersatzweise `k-playbook read <verweis>` |
| `k-playbook:/instructions/always/` | das einzige Verzeichnis, das k-playbook zu einem Text zusammenbaut | wie `k-playbook:/<pfad>` |
| `keph:/<pfad>` | Collection des Projekts in Kephalaion | Kephalaion `read` mit `collection` = `kephalaion.address` und `name` = Pfad ohne führenden `/` |

Die Felder `kephalaion.address`, `kephalaion.ok` und `kephalaion.user` stehen im Briefing
unter `context` und in der Ausgabe von `k-playbook context`.

Beispiele:

- `project:/k-playbook-local/docs/README.md`
- `k-playbook:/instructions/on-demand/<thema>.md`
- `keph:/manual/<thema>.md`

Regeln:

- **`k-playbook:` liest du nur über k-playbook**, nie selbst aus `k-playbook/` oder
  `k-playbook-local/`. k-playbook entscheidet, ob eine lokale Fassung die mitgelieferte
  ersetzt oder abschaltet. Wer selbst nachsieht, kann die falsche Fassung erwischen.
- **Ein Verweis, der auf `/` endet, meint ein Verzeichnis.** Zusammengebaut wird nur
  `k-playbook:/instructions/always/`: Du bekommst alle Dateien darin als einen Text, in der
  festgelegten Reihenfolge. Jedes andere Verzeichnis, etwa `on-demand/`, liest du einzeln mit
  Dateinamen (`k-playbook:/instructions/on-demand/<thema>.md`); mit `/` am Ende fehlt der
  Dateiname, und k-playbook meldet einen Fehler.
- **Ein Verweis, den k-playbook nicht kennt, ist ein Fehler.** Melde ihn und such die Datei
  nicht auf eigene Faust.
- **Ein `keph:`-Verweis nennt weder Hub noch Collection.** Beides steht rechnerlokal in der
  Konfiguration und kann je Rechner anders sein. Steht `kephalaion.ok` nicht auf `true`, sind
  `keph:`-Verweise nicht erreichbar. Sag das, statt auszuweichen.
- **Die öffentliche Collection `<hub>:k-playbook` sprichst du nicht direkt an.** Ihr Inhalt
  ist das Mitgelieferte und kommt über `k-playbook:`.
- **`~` als Pfadteil steht für dich**, genauer für deinen Kephalaion-User. In
  `keph:/user/~/tasks/` ersetzt du `~` durch `kephalaion.user`, nicht durch
  `kephalaion.account`: Das Konto gilt nur für diesen Rechner, der User ist die Person.
  Nur ein ganzer Pfadteil `~` ist der Platzhalter. Verweise, die k-playbook selbst ausgibt,
  enthalten schon den echten Namen.
