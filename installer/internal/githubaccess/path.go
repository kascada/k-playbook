// Package githubaccess liest und prüft die persönliche GitHub-Zugangsdatei
// ~/.config/k-playbook/github.yaml nach dem Vertrag docs/github-access.md
// (Workbench 002, Schritt 1). Die Datei nennt je Dienst ein Repository und
// die Zugänge in Vorrangreihenfolge — ein gh-Login, eine Token-Datei, ein
// SSH-Schlüssel oder -Alias, oder gar keinen —, aber nie ein Geheimnis: Sie
// sagt, wo ein Zugang liegt, nicht was er ist. Dieses Paket liest nur die
// Datei selbst; die Dateien, auf die sie zeigt, fasst es nicht an, und ob ein
// Zugang verfügbar ist, prüft es nicht (das kommt in einem späteren Schritt).
//
// Abgrenzung zu internal/github: Das fragt über gh die GitHub-API für die
// Seite /branches und /k-pr-review; hier geht es allein um die lokale Datei.
//
// Load liefert die Konfiguration oder die Probleme, nie beides und nie nur
// das erste. Fehlt die Datei, legt Load einmal eine Vorlage an und meldet das
// als warning; das ist die einzige Nebenwirkung des Pakets. Das Paket hängt
// nur von der Standardbibliothek und yamllite ab, damit project es für die
// Umgebungsprüfung einbinden kann.
package githubaccess

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// EnvConfig ist der Override auf den Pfad der Datei, für Tests und für
	// Container, die die Datei an anderer Stelle einhängen. Ist er gesetzt,
	// gilt nur er — auch für das Anlegen der Vorlage.
	EnvConfig = "K_PLAYBOOK_GITHUB_CONFIG"
	// ConfigDirName ist das Verzeichnis unter $XDG_CONFIG_HOME bzw. ~/.config.
	ConfigDirName = "k-playbook"
	// FileName ist der Name der Datei.
	FileName = "github.yaml"
)

// Path nennt den Ort der Datei: der Override EnvConfig, wie er gesetzt ist;
// sonst $XDG_CONFIG_HOME/k-playbook/github.yaml, wenn die Variable einen
// absoluten Pfad trägt; sonst ~/.config/k-playbook/github.yaml — dieselbe
// XDG-Regel wie kephalaion.DefaultConfigDir.
func Path() (string, error) {
	if v := os.Getenv(EnvConfig); v != "" {
		return v, nil
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" && filepath.IsAbs(v) {
		return filepath.Join(v, ConfigDirName, FileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Heimatverzeichnis unbekannt: %w", err)
	}
	return filepath.Join(home, ".config", ConfigDirName, FileName), nil
}
