package project

import (
	"context"
	"errors"
	"fmt"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Der Altbestand der Anbindung (Task 138): Bis zur Auskunft kephalaion node
// client stand sie rechnerlokal in k-playbook-local/K-PLAYBOOK.local.yaml.
// Kein Programmteil liest sie mehr von dort. Der Projektschritt von
// k-playbook upgrade zieht Hub und Collection in den Block kephalaion: von
// K-PLAYBOOK.yaml um (MigrateKephalaionBinding) — nicht der Text von
// /k-upgrade: Den liefert Kephalaion, und ohne Block ist er nicht lesbar. Die
// alte Datei löscht danach /k-upgrade nach Rückfrage, aber nur, wenn eine
// Zeile dieses Schritts sagt, dass der Block steht.

// LegacyKephalaionDisplay ist die alte Datei, wie eine Meldung sie nennt.
const LegacyKephalaionDisplay = LocalDirName + "/" + kephalaion.LegacyFileName

// KephalaionMigration ist der Umzug der Anbindung aus der alten Datei.
type KephalaionMigration struct {
	// Legacy sagt, ob die alte Datei einen Block kephalaion: trägt; ohne ihn
	// gibt es nichts umzuziehen und keine Meldung.
	Legacy bool `json:"legacy"`
	// Old ist der Block der alten Datei, samt Account.
	Old kephalaion.LegacyBinding `json:"old"`
	// Block ist der Block kephalaion: in K-PLAYBOOK.yaml vor dem Lauf.
	Block ConfigKephalaion `json:"block"`
	// Written: Hub und Collection stehen jetzt im Block (nicht bei --check).
	Written bool `json:"written,omitempty"`
	// WouldWrite: --check; ein Lauf schriebe sie.
	WouldWrite bool `json:"wouldWrite,omitempty"`
	// Differs: der vorhandene Block weicht von der alten Datei ab; er bleibt.
	Differs bool `json:"differs,omitempty"`
	// Account ist der Satz zum Account der alten Datei, wenn Kephalaion für
	// den Hub keinen oder einen anderen wählt; leer sonst.
	Account string `json:"account,omitempty"`
	// Error nennt, was scheiterte; geschrieben ist dann nichts.
	Error string `json:"error,omitempty"`
}

// kephalaionQuery ist die Auskunft für den Account-Abgleich; austauschbar
// für Tests.
var kephalaionQuery = kephalaion.QueryClient

// MigrateKephalaionBinding zieht die Anbindung aus der alten Datei um. Fehlt
// der Block kephalaion: in K-PLAYBOOK.yaml (oder trägt er weder hub noch
// collection) und nennt die alte Datei beide, schreibt es sie ohne Rückfrage
// zeilenweise hinein (SetKephalaionBinding); mit check nur die Meldung, was
// ein Lauf täte. Ein vorhandener Block bleibt unverändert; weicht er ab,
// nennt die Meldung beide Werte. Der Account der alten Datei wird nirgends
// geschrieben. Wählt Kephalaion für den Hub keinen oder einen anderen
// Account, nennt die Meldung ihn samt kephalaion node account default — sonst
// ginge er gerade bei „Wahl fehlt“ verloren, weil dann auch der Text von
// /k-upgrade nicht lesbar ist.
func MigrateKephalaionBinding(projectDir string, check bool) KephalaionMigration {
	migration := KephalaionMigration{}
	old, present, err := kephalaion.ReadLegacyBinding(LocalDir(projectDir))
	if err != nil {
		migration.Legacy = true
		migration.Error = err.Error()
		return migration
	}
	if !present {
		return migration
	}
	migration.Legacy, migration.Old = true, old
	config, err := ReadConfig(projectDir)
	if err != nil {
		migration.Error = ConfigDisplayName + " nicht lesbar: " + err.Error()
		return migration
	}
	migration.Block = config.Kephalaion

	oldBinding := kephalaion.Binding{Hub: old.Hub, Collection: old.Collection}
	switch {
	case config.Kephalaion.Hub == "" && config.Kephalaion.Collection == "":
		if err := kephalaion.ValidateBinding(oldBinding); err != nil {
			migration.Error = LegacyKephalaionDisplay + " nennt keine gültige Anbindung (" + err.Error() + ")"
			break
		}
		if check {
			migration.WouldWrite = true
			break
		}
		if _, err := SetKephalaionBinding(projectDir, old.Hub, old.Collection); err != nil {
			migration.Error = err.Error()
			break
		}
		migration.Written = true
	default:
		migration.Differs = config.Kephalaion.Hub != old.Hub || config.Kephalaion.Collection != old.Collection
	}
	migration.Account = legacyAccountNote(old)
	return migration
}

// legacyAccountNote vergleicht den Account der alten Datei mit der Wahl von
// Kephalaion für ihren Hub.
func legacyAccountNote(old kephalaion.LegacyBinding) string {
	if old.Account == "" || kephalaion.CheckName("Hub", old.Hub) != nil || kephalaion.CheckName("Account", old.Account) != nil {
		return ""
	}
	remedy := fmt.Sprintf("Soll %s gelten: kephalaion node account default %s %s (ist das Token-Verzeichnis nur lesend "+
		"eingebunden, wie im DevContainer, auf dem Host).", old.Account, old.Hub, old.Account)
	chosen := ""
	info, err := kephalaionQuery(context.Background(), old.Hub)
	if err != nil {
		var clientErr *kephalaion.ClientError
		reason := err.Error()
		if errors.As(err, &clientErr) {
			reason = clientErr.Message
		}
		return fmt.Sprintf("Account — die alte Datei nannte für den Hub %s den Account %s; welchen Kephalaion wählt, "+
			"ist nicht zu erfahren: %s %s", old.Hub, old.Account, reason, remedy)
	}
	for _, hub := range info.Hubs {
		if hub.Alias != old.Hub {
			continue
		}
		if hub.Choice != kephalaion.ChoiceMissing && hub.Account != nil {
			chosen = *hub.Account
			if chosen == old.Account {
				return ""
			}
			return fmt.Sprintf("Account — die alte Datei nannte für den Hub %s den Account %s; Kephalaion wählt dort %s (%s). %s",
				old.Hub, old.Account, chosen, choiceLabel(hub.Choice), remedy)
		}
	}
	return fmt.Sprintf("Account — die alte Datei nannte für den Hub %s den Account %s; Kephalaion wählt dort keinen. %s",
		old.Hub, old.Account, remedy)
}

// choiceLabel nennt die Herkunft einer Wahl in Worten.
func choiceLabel(choice kephalaion.Choice) string {
	switch choice {
	case kephalaion.ChoiceOverride:
		return "Überschreibung"
	case kephalaion.ChoiceDefault:
		return "Standard"
	case kephalaion.ChoiceOnly:
		return "einzige Token-Datei"
	}
	return string(choice)
}

// Messages formuliert den Umzug als Zeilen, ohne Präfix. Ohne alte Datei
// keine. Die Zeilen „Anbindung umgezogen“ und „Anbindung — der Block … steht“
// sind der Vertrag mit /k-upgrade: Nur nach einer davon löscht der Text die
// alte Datei.
func (m KephalaionMigration) Messages() []string {
	if !m.Legacy {
		return nil
	}
	var lines []string
	old := fmt.Sprintf("hub %s, collection %s", m.Old.Hub, m.Old.Collection)
	switch {
	case m.Error != "":
		lines = append(lines, "Anbindung nicht umgezogen — "+m.Error+". Abhilfe: Hub und Collection in den Block kephalaion: von "+
			ConfigDisplayName+" eintragen (k-playbook gui, Abschnitt Kephalaion).")
	case m.Written:
		lines = append(lines, fmt.Sprintf("Anbindung umgezogen — %s steht jetzt im Block kephalaion: von %s (aus %s). %s "+
			"%s liest kein Programmteil mehr; /k-upgrade löscht sie nach Rückfrage.",
			old, ConfigDisplayName, LegacyKephalaionDisplay, KephalaionCommitHint, LegacyKephalaionDisplay))
	case m.WouldWrite:
		lines = append(lines, fmt.Sprintf("Anbindung — ein Lauf schriebe %s aus %s in den Block kephalaion: von %s.",
			old, LegacyKephalaionDisplay, ConfigDisplayName))
	case m.Differs:
		lines = append(lines, fmt.Sprintf("Anbindung — der Block kephalaion: von %s steht (hub %s, collection %s) und bleibt; "+
			"%s nennt anderes (%s) und wird nicht mehr gelesen; /k-upgrade löscht sie nach Rückfrage.",
			ConfigDisplayName, m.Block.Hub, m.Block.Collection, LegacyKephalaionDisplay, old))
	default:
		lines = append(lines, fmt.Sprintf("Anbindung — der Block kephalaion: von %s steht (hub %s, collection %s); "+
			"%s liest kein Programmteil mehr; /k-upgrade löscht sie nach Rückfrage.",
			ConfigDisplayName, m.Block.Hub, m.Block.Collection, LegacyKephalaionDisplay))
	}
	if m.Account != "" {
		lines = append(lines, m.Account)
	}
	return lines
}
