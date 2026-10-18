# includes

Textbausteine, die k-playbook künftig vor der Ausgabe in Dokumente einsetzt — etwa
ein Absatz, der in mehreren Commands wortgleich steht, einmal hier und von dort
eingesetzt.

- Syntax und Mechanismus — wie ein Dokument einen Baustein nennt, wie k-playbook ihn
  bei der Auflösung eines Verweises einsetzt und wo die Rekursion endet — kommen mit
  einem eigenen Task.
- Bis dahin wird hier nichts gelesen und nichts aufgelöst: `includes/` ist keine Sorte
  der Verweisauflösung, und `k-playbook:/includes/<name>.md` ist `unsupported`.
- Das Verzeichnis liegt in `defaults/`, damit es mit dem Release gepackt wird und
  auf jedem Hub unter `<hub>:k-playbook/defaults/includes/` ankommt, sobald es Inhalt
  trägt. Feste Testfälle für den Mechanismus sind für `defaults/testdata/` geplant.
