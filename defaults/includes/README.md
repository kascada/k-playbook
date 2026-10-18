# includes

Textbausteine, die k-playbook vor der Ausgabe in Dokumente einsetzt — etwa ein
Absatz, der in mehreren Commands wortgleich steht, einmal hier und von dort
eingesetzt.

- `includes/` ist eine Sorte der Verweisauflösung (Task 093): `k-playbook:/includes/<name>.md`
  wird aufgelöst wie `instructions/` und `commands/`, mitgeliefert aus `defaults/includes/`,
  eine gleichnamige Datei in `k-playbook-local/includes/` ersetzt den Baustein, eine leere
  schaltet ihn ab. Die Sorte erscheint in keinem Katalog, keinem Briefing und keiner Hilfe:
  Bausteine liest keine Sitzung für sich, sie werden eingesetzt.
- Eingesetzt wird über eine Include-Zeile im einbindenden Dokument — ohne führenden
  Leerraum, genau ein Argument, sonst nichts auf der Zeile:

  ```markdown
  include: k-playbook:/includes/<name>.md
  ```

  Ein Baustein hier ist aus einem Command oder einer Anweisung nur über diesen absoluten
  Verweis erreichbar; ein relativer Pfad zählt ab dem Verzeichnis des einbindenden
  Dokuments und führt nicht aus dessen Sorte heraus. Syntax, Rekursion (Tiefe 10),
  Zyklusprüfung, Fehlercodes und der Parameter zum Abschalten (`k-playbook read --raw`,
  `k_playbook_doc_read` mit `includes: false`): `docs/verweisschema.md`, Abschnitt „Includes“.
- Das Verzeichnis liegt in `defaults/`, damit es mit dem Release gepackt wird und auf jedem
  Hub unter `<hub>:k-playbook/defaults/includes/` ankommt. Einen Baustein trägt es weiterhin
  nicht: Module, die nur Commands einbinden, liegen als relative Includes in
  `defaults/commands/_shared/` (seit Task 094: `context.md`, `gh.md`, `base-tools.md`) — dem
  Namensraum, den `rules/command-authoring.md` für Command-Module festlegt; ein relatives
  Include bleibt in der Sorte `commands` und nimmt an deren Überlagerung teil. Hierher kommt
  erst ein Baustein, den mehr als eine Sorte nutzt, etwa später ein Skill-Text. Feste Testfälle
  für den Mechanismus liegen in `defaults/testdata/includes/`.
