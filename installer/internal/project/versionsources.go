package project

import (
	"context"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/versionsources"
)

// Die Quellenkonfiguration des Versionsinventars liegt seit Task 144 in der
// Collection des Projekts, im Dokument versionsources.DocRef
// (keph:/k-playbook/inventory.md). Gelesen wird sie allein von
// internal/versionsources; hier wird nur der Zugang geöffnet und der Zustand
// für den Teilabruf environment des Briefings, die Gesundheitsprüfung und die
// Oberfläche aufbereitet.

// Die Zustände von versionSources im Teilabruf environment.
const (
	// VersionSourcesPresent: Das Dokument liegt und ist lesbar.
	VersionSourcesPresent = "present"
	// VersionSourcesMissing: Das Dokument fehlt; es gelten die
	// Standardquellen. Kein Fehler.
	VersionSourcesMissing = "missing"
	// VersionSourcesInvalid: Das Dokument liegt, ist aber nicht lesbar, von
	// fremder Fassung oder trägt roots. Der Erhebungslauf bräche ab.
	VersionSourcesInvalid = "invalid"
	// VersionSourcesUnavailable: Kephalaion ist nicht erreichbar; über das
	// Dokument ist nichts bekannt. Kein Rückfall auf die Platte und keiner
	// auf die Standardquellen.
	VersionSourcesUnavailable = "unavailable"
)

// VersionSources ist der Zustand der Quellenkonfiguration im Teilabruf
// environment des Briefings. Der Vertrag dazu steht in defaults/docs/handbook/version-inventory.md,
// Abschnitt „Source Configuration"; die Feldnamen sind die YAML-Schlüssel
// des Frontmatters, in camelCase wie überall sonst in dieser Ausgabe.
//
// Hier steht nur das Schema. Gelesen wird das Dokument von
// internal/versionsources — der einzigen Implementierung, die auch der
// Sammler des Inventars benutzt. Zwei Leser derselben Konfiguration wären
// zwei Auslegungen derselben Vertrauensgrenze.
type VersionSources struct {
	// Ref ist der Ort des Dokuments als keph:-Verweis — auch wenn es fehlt
	// oder nicht erreichbar ist, damit klar ist, wo es hingehört.
	Ref string `json:"ref"`
	// State ist present, missing, invalid oder unavailable (die Konstanten
	// VersionSources…).
	State string `json:"state"`
	// Present sagt, ob das Dokument da ist. Fehlt es, ist das kein Fehler: es
	// gelten die Standardquellen unterhalb der Projektwurzel. Bei
	// unavailable ist es false, ohne dass das etwas über das Dokument sagt.
	Present bool `json:"present"`
	// SchemaVersion ist die vom Dokument deklarierte Fassung; 0, solange
	// nichts gelesen wurde.
	SchemaVersion int `json:"schemaVersion,omitempty"`
	// Sources sind die konfigurierten Zusatzquellen. Sie ergänzen die
	// Standarderkennung, sie ersetzen sie nicht.
	Sources []VersionSource `json:"sources,omitempty"`
	// Exclude sind die Muster, die von der Standarderkennung übergangen werden.
	// Sie wirken nur dort: was unter Sources ausdrücklich steht, bleibt gelesen.
	Exclude []string `json:"exclude,omitempty"`
	// HelmValues sind die konfigurierten Helm-Werte. Wie bei Sources stehen
	// auch abgelehnte Einträge darin.
	HelmValues []VersionHelmValue `json:"helmValues,omitempty"`
	// Error ist gesetzt bei invalid und unavailable: der Grund. Der Teilabruf
	// environment bricht deswegen nicht ab: Commands holen ihn, wo sie die
	// Angaben brauchen, und eine defekte oder unerreichbare
	// Zusatzkonfiguration darf nicht jeden Command lahmlegen. Der Erhebungslauf des Inventars bricht
	// sehr wohl ab — so steht es im Vertrag.
	Error string `json:"error,omitempty"`
}

// VersionSource ist ein Eintrag aus sources:. Die Felder heißen wie die
// YAML-Schlüssel; die gültigen Werte von Kind und Env stehen im Vertrag.
type VersionSource struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Env  string `json:"env"`
	Note string `json:"note,omitempty"`
	// Optional: true heißt, dass eine fehlende Datei kein Hinweis ist.
	Optional bool `json:"optional,omitempty"`
}

// VersionHelmValue ist ein Eintrag aus helm_values:. Die Felder heißen wie die
// YAML-Schlüssel.
type VersionHelmValue struct {
	Path string `json:"path"`
	Key  string `json:"key"`
	Item string `json:"item"`
}

// OpenVersionSourcesReader öffnet den Zugang zur Collection des Projekts für
// die Quellenkonfiguration — derselbe Weg wie OpenInventoryStore, mit einem
// Status, den der Aufrufer schon geprüft hat. Ein Fehler trägt
// DocCodeKephalaionUnavailable. Austauschbar, damit Tests den Node nicht
// fragen.
var OpenVersionSourcesReader = func(projectDir string, status kephalaion.Status) (versionsources.Reader, error) {
	client, _, err := openKephClientStatus(projectDir, "", status)
	if err != nil {
		return nil, err
	}
	return inventoryStore{client: client}, nil
}

// LoadVersionSources liest die Quellenkonfiguration des Projekts: Status
// prüfen, Zugang öffnen, versionsources.Load. Ist Kephalaion nicht bereit,
// ist der Fehler ein versionsources.UnavailableError mit dem Grund der
// Anbindung; opened sagt dann false. Das Ergebnis ist dasselbe, das der
// Erhebungslauf liest.
func LoadVersionSources(ctx context.Context, projectDir string) (config versionsources.Config, opened bool, err error) {
	return loadVersionSources(ctx, projectDir, KephalaionStatus(projectDir))
}

func loadVersionSources(ctx context.Context, projectDir string, status kephalaion.Status) (versionsources.Config, bool, error) {
	reader, err := OpenVersionSourcesReader(projectDir, status)
	if err != nil {
		return versionsources.Config{Ref: versionsources.DocRef}, false, &versionsources.UnavailableError{Err: err}
	}
	config, err := versionsources.Load(ctx, reader)
	return config, true, err
}

// versionSourcesWithStatus löst den Zustand der Quellenkonfiguration auf, mit
// dem Status der Anbindung, den der Aufrufer (BuildBriefingEnvironment) schon
// hat.
//
// Ein defektes oder unerreichbares Dokument bricht den Teilabruf environment
// nicht ab: Commands holen ihn, wo sie die Angaben brauchen, und eine defekte
// Zusatzkonfiguration darf nicht jeden Command lahmlegen. Sichtbar bleibt der
// Zustand trotzdem — dafür sind state und error da. Der Erhebungslauf des
// Inventars bricht bei demselben Dokument sehr wohl ab; das ist die Trennung
// zwischen Auskunft geben und Erheben, nicht ein zweites Verhalten desselben
// Lesers.
func versionSourcesWithStatus(projectDir string, status kephalaion.Status) *VersionSources {
	config, _, err := loadVersionSources(context.Background(), projectDir, status)
	return VersionSourcesFrom(config, err)
}

// VersionSourcesFrom macht aus dem Ergebnis von versionsources.Load den
// Zustand des Teilabrufs environment — für Aufrufer, die den Zugang selbst schon
// offen haben, etwa die Oberfläche über OpenInventoryStore.
func VersionSourcesFrom(config versionsources.Config, err error) *VersionSources {
	state := &VersionSources{Ref: versionsources.DocRef, Present: config.Present}
	switch {
	case err != nil && versionsources.IsUnavailable(err):
		state.State = VersionSourcesUnavailable
		state.Present = false
		state.Error = err.Error()
		return state
	case err != nil:
		state.State = VersionSourcesInvalid
		state.Error = err.Error()
		return state
	case !config.Present:
		state.State = VersionSourcesMissing
		return state
	}
	state.State = VersionSourcesPresent
	state.SchemaVersion = config.SchemaVersion
	state.Exclude = config.Exclude
	// Ausgegeben werden die Einträge so, wie sie im Dokument stehen — auch
	// die, die der Sammler wegen unbekanntem kind oder env oder absolutem Pfad
	// ablehnt. Sie hier wegzulassen hieße, das Dokument anders darzustellen
	// als es ist; die Ablehnung selbst führt der Erhebungslauf sichtbar in der
	// Inventardatei.
	for _, source := range config.Sources {
		state.Sources = append(state.Sources, VersionSource{
			Path:     source.Path,
			Kind:     source.Kind,
			Env:      source.Env,
			Note:     source.Note,
			Optional: source.Optional,
		})
	}
	for _, value := range config.HelmValues {
		state.HelmValues = append(state.HelmValues, VersionHelmValue{
			Path: value.Path,
			Key:  value.Key,
			Item: value.Item,
		})
	}
	return state
}
