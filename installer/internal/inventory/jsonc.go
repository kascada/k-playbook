package inventory

import (
	"encoding/json"

	"github.com/tailscale/hujson"
)

// standardizeJSONC macht aus JSON mit Kommentaren und nachlaufenden Kommata
// gültiges JSON. DevContainer-Dateien sind so geschrieben, und dieselbe
// Bibliothek liest im Werkzeug bereits die Assistenten-Konfiguration.
func standardizeJSONC(data []byte) ([]byte, error) {
	return hujson.Standardize(data)
}

func unmarshalJSON(data []byte, target any) error {
	return json.Unmarshal(data, target)
}
