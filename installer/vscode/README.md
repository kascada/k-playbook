# k-playbook Workspace Tools

Sammelpaket für Workspace-Aktionen von k-playbook in VS Code. Die Erweiterung läuft
im Workspace-Kontext (`extensionKind: ["workspace"]`), hat keine Abhängigkeiten, keinen
Webview und keinen eigenen Server. Sie setzt keine andere Erweiterung voraus; das Einbinden
der Kephalaion-Collections wird nur tätig, wenn die Kephalaion-Erweiterung installiert ist.

Installiert wird sie von k-playbook selbst: die VSIX reist im Programm-Binary mit und
wird beim Start der Oberfläche eingespielt und nachgezogen. Wer die Oberfläche nie
startet, ruft `k-playbook vscode install`. Einzelheiten in `defaults/docs/handbook/vscode.md` im Repo.

## Befehle

| Befehl | Titel | Tastenkürzel |
|---|---|---|
| `kPlaybook.initWorkspace` | k-playbook: Kephalaion-Collections einbinden | — (läuft beim Start von selbst) |

### Kephalaion-Collections im Arbeitsbereich

Bei jedem Fensterstart (`activationEvents: ["onStartupFinished"]`) bindet die Erweiterung
die Collection des Projekts als Ordner ein, mit Hub und Collection aus dem Block
`kephalaion:` von `.k-playbook/K-PLAYBOOK.yaml`. Die öffentliche Collection `k-playbook` gehört
nicht dazu: ihr Inhalt kommt über k-playbook selbst. Welche Collection es ist, fragt sie das
Programm: `k-playbook vscode collections --json`, im Projektordner. Die Regel
steht damit nur im Programm.

- **Alles eingebunden:** nichts zu tun.
- **Workspace-Datei offen, etwas fehlt:** Das Fehlende wird hinten angefügt
  (`updateWorkspaceFolders`). VS Code schreibt es selbst in die geöffnete Datei.
- **Nur ein Ordner offen:** Die Workspace-Datei des Ordners wird geöffnet, das Fenster lädt
  dabei neu. Gibt es keine, wird `<ordnername>.code-workspace` angelegt; gibt es genau eine,
  wird sie genommen; bei mehreren die mit dem Ordnernamen, sonst sagt eine Meldung, dass
  eine von Hand zu öffnen ist. Eine vorhandene Datei wird nie überschrieben.

Beim Start bleibt still, was keine Störung ist: kein Projektordner, keine Anbindung, keine
Kephalaion-Erweiterung, kein `k-playbook` im PATH. Ein Fehlschlag kommt als Warnung. Von Hand
aufgerufen meldet der Befehl jeden Ausgang. Eine bewusst entfernte Collection kommt beim
nächsten Start zurück.

## Eine neue Aktion hinzufügen

1. `actions/<name>.js` anlegen, das `{ id, run }` exportiert. `id` folgt dem Schema
   `kPlaybook.<aktion>`.
2. Das Modul in der Liste `actions` in `extension.js` nennen.
3. In `package.json` unter `contributes.commands` eintragen, Kategorie `k-playbook`,
   dazu nach Bedarf `menus` und `keybindings`.
4. Einen Test unter `test/` ergänzen; gelaufen wird mit `make vscode-test`.

Am Bestand ist dafür nichts zu ändern: `extension.js` registriert, was in der Liste
steht, und sonst nichts. Soll eine Aktion bei jedem Fensterstart laufen, exportiert sie
zusätzlich `onStartup`; `activate` ruft sie, ohne auf sie zu warten, und sie darf nie werfen.

## Tests

```bash
make vscode-test     # node --test installer/vscode/test/
```

Die Tests laufen mit `node:test` ohne Abhängigkeiten gegen ein nachgebildetes
`vscode`-Modul. Gegen ein echtes VS Code wird nicht getestet.
