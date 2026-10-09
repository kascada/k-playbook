# k-playbook Workspace Tools

Sammelpaket für Workspace-Aktionen von k-playbook in VS Code. Die Erweiterung läuft
im Workspace-Kontext (`extensionKind: ["workspace"]`), hat keine Abhängigkeiten, keinen
Webview und keinen eigenen Server. Sie ist unabhängig von jeder anderen Erweiterung.

Installiert wird sie von k-playbook selbst: die VSIX reist im Programm-Binary mit und
wird beim Start der Oberfläche eingespielt und nachgezogen. Wer die Oberfläche nie
startet, ruft `k-playbook vscode install`. Einzelheiten in `docs/vscode.md` im Repo.

## Befehle

| Befehl | Titel | Tastenkürzel |
|---|---|---|
| `kPlaybook.openCode` | k-playbook: OpenCode im neuen Tab | `ctrl+alt+shift+o`, mac `cmd+alt+shift+o` |

Der Befehl öffnet ein Terminal **im Editor-Bereich** — einen Tab neben den Dateien,
nicht das Panel unten — und startet darin das Anhäng-Programm von OpenCode. Das
Arbeitsverzeichnis ist der gewählte Workspace-Ordner; bei mehreren Ordnern fragt eine
Auswahl, bei einem wird nicht gefragt. Neben der Befehlspalette und dem Tastenkürzel
liegt der Befehl als Knopf in der Titelzeile des Editors.

## Einstellungen

Beide Einstellungen haben `scope: "machine"`: kein Arbeitsbereich darf den ausgeführten
Pfad umbiegen.

| Einstellung | Standard | Bedeutung |
|---|---|---|
| `kPlaybook.openCode.executable` | `opencode-attach` | Das zu startende Programm. Ein Name ohne `/` wird im PATH gesucht, ein Wert mit `/` als Pfad genommen. |
| `kPlaybook.openCode.args` | `[]` | Zusätzliche Argumente. Der Ordner wird nicht als Argument übergeben, sondern als Arbeitsverzeichnis. |

Der Programmpfad wird im Extension-Host aufgelöst und dem Terminal absolut übergeben.
Der Grund steht in `lib/executable.js`: der Extension-Host trägt den PATH der
Login-Shell, der Prozess, der die Terminals startet, nicht. Fehlt das Programm, sagt
das eine Meldung mit dem Knopf „Einstellung öffnen“ — nicht nur das Terminal.

## Eine neue Aktion hinzufügen

1. `actions/<name>.js` anlegen, das `{ id, run }` exportiert. `id` folgt dem Schema
   `kPlaybook.<aktion>`.
2. Das Modul in der Liste `actions` in `extension.js` nennen.
3. In `package.json` unter `contributes.commands` eintragen, Kategorie `k-playbook`,
   dazu nach Bedarf `menus` und `keybindings`.
4. Einen Test unter `test/` ergänzen; gelaufen wird mit `make vscode-test`.

Am Bestand ist dafür nichts zu ändern: `extension.js` registriert, was in der Liste
steht, und sonst nichts.

## Tests

```bash
make vscode-test     # node --test installer/vscode/test/
```

Die Tests laufen mit `node:test` ohne Abhängigkeiten gegen ein nachgebildetes
`vscode`-Modul. Gegen ein echtes VS Code wird nicht getestet.
