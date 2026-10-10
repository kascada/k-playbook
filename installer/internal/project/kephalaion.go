package project

import (
	"context"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// KephalaionStatus prüft die Anbindung des Projekts an seine
// Kephalaion-Collection: liest den Block kephalaion: aus
// k-playbook-local/K-PLAYBOOK.local.yaml und fragt den Node dieses Rechners
// über MCP (whoami). Scheitert nie — jeder Fehler ist ein Zustand. Ohne
// Eintrag wird nichts gefragt; mit Eintrag gilt die Zeitgrenze
// kephalaion.DefaultTimeout für die ganze Prüfung.
//
// Die Prüfung steht bewusst nicht in BuildContext: Den rufen auch Werkzeuge,
// die nur den Katalog brauchen (etwa die Review-Auswahl), und die sollen den
// Node nicht fragen. ContextForDir — das Unterkommando context und das
// MCP-Werkzeug — hängt das Ergebnis an.
func KephalaionStatus(projectDir string) kephalaion.Status {
	return kephalaionCheck(projectDir)
}

// kephalaionCheck ist austauschbar, damit Tests den Node nicht fragen.
var kephalaionCheck = func(projectDir string) kephalaion.Status {
	env := kephalaion.Env{}
	if dir, err := kephalaion.DefaultConfigDir(); err == nil {
		env = kephalaion.LoadEnv(dir)
	}
	return kephalaion.Check(context.Background(), LocalDir(projectDir), env)
}
