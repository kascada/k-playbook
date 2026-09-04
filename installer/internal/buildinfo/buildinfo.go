// Package buildinfo trägt die Werte, die beim Bauen in das Binary gestempelt
// werden.
package buildinfo

// Version wird beim Bauen über -ldflags gesetzt. Leer bleibt sie nur bei einem
// Ad-hoc-`go build`, das die Build-Flags bewusst weglässt.
var Version string
