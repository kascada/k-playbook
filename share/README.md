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

Die Projekte verlinken die Einträge durch `.k-playbook/share` (siehe
`installer/docs/architecture.md`, „Der Umweg-Link `.k-playbook/share`“). Ein
Command nennt ein Modul mit dem verlinkten Pfad, etwa
`.claude/commands/_shared/context.md` (`rules/command-authoring.md`).

Hier ist die einzige Fassung; `commands/` und `skills/` oben im Repo gibt es seit
2026-10-25 nicht mehr. Eine Änderung hier ändert das Binary und verlangt damit
einen VERSION-Bump.
