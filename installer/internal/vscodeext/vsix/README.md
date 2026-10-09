# Eingecheckte VSIX

`k-playbook-workspace-tools.vsix` ist die gebaute VS-Code-Erweiterung aus
`installer/vscode/`. Sie liegt hier eingecheckt und wird per `go:embed` in das
Programm genommen.

Warum eingecheckt und nicht bei jedem Build erzeugt: vsce schreibt Bauzeit und
die mtimes der Quelldateien in das Zip. Eine bei jedem Build neu erzeugte VSIX
hätte andere Bytes, und die Release-CI prüft die Binaries bitgleich gegen
`SHA256SUMS`. Node ist deshalb ein Entwickler-Schritt und weder für `make dist`
noch für ein Release nötig.

Neu bauen nach jeder Änderung unter `installer/vscode/`:

```bash
make vscode-vsix
```

Die neue Datei gehört in denselben Commit wie die Quelländerung. `make test`
wird sonst rot: `TestVSIXPasstZurQuelle` vergleicht beides.

Diese Datei hält das Verzeichnis außerdem nicht leer — `go:embed vsix` würde
sich an einem leeren Verzeichnis nicht übersetzen lassen.
