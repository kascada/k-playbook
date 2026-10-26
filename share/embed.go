// Package share trägt die Dateien, die das Binary nach
// ~/.local/share/k-playbook/ entpackt: was als Datei auf der Platte liegen muss
// und deshalb nicht über Kephalaion kommen kann. Das sind Commands und Skills
// für die Assistenten und scripts/ für das Binary selbst — Matrizen und
// Kataloge, die es liest, Skripte, die es mit bash startet, und
// Konfigurationen, die es Scannern als Pfad mitgibt.
//
// Ein eigenes Modul, weil go:embed nur unterhalb des eigenen Moduls greift:
// installer/ bindet es über eine replace-Zeile ein, und jeder gewöhnliche
// `go build` dort nimmt den Inhalt mit — ohne Kopierschritt, den ein Build am
// Makefile vorbei vergessen könnte.
package share

import "embed"

// Files ist der eingebettete Inhalt.
//
// all: ist nötig, weil die Module unter commands/ mit _ beginnen (_shared/,
// _docs/ …); ohne den Zusatz ließe go:embed sie stillschweigend aus.
//
//go:embed all:commands all:skills all:scripts
var Files embed.FS

// Roots sind die obersten Verzeichnisse in Files. Entpackt und ersetzt wird
// je Eintrag als Ganzes; was daneben im Zielverzeichnis liegt, bleibt
// unberührt.
var Roots = []string{"commands", "skills", "scripts"}
