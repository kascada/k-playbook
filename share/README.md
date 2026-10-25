# share/

Was das Binary mitbringt und nach `~/.local/share/k-playbook/` entpackt
(`$XDG_DATA_HOME/k-playbook/`, wenn gesetzt) — einmal je Rechner, für alle
Projekte. Hier liegt, was die Assistenten als Datei auf der Platte brauchen:
die Commands und die Skills. Alles, was über MCP gelesen werden kann, gehört
nach `defaults/` und damit nach Kephalaion.

| im Repo | landet in |
|---|---|
| `defaults/` | `<hub>:k-playbook/defaults/` (Kephalaion) |
| `share/` | `~/.local/share/k-playbook/` (Platte) |

Eingebettet werden `commands/` und `skills/` (`embed.go`). Das Binary vergleicht
bei jedem Aufruf die Prüfsumme des eingebetteten Inhalts mit der Datei
`.inhalt-sha256` im Zielverzeichnis und entpackt nur bei Abweichung. Die Dateien
dort sind schreibgeschützt.

**Übergang (Stufe 1, Punkt 20 der Arbeitsliste in
[`docs/knowledge-aktuell.md`](../docs/knowledge-aktuell.md)):** `commands/` und
`skills/` liegen noch zusätzlich oben im Repo, und die Projekte verlinken weiter
dorthin. Bis die alten Verzeichnisse wegfallen, wird jede Änderung an beiden
Stellen gemacht; `make share-sync` kopiert oben nach hier, und ein Test im
Installer schlägt fehl, sobald beide auseinanderlaufen. Eine Änderung hier ändert
das Binary und verlangt damit einen VERSION-Bump.
