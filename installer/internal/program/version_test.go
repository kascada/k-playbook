package program

import "testing"

// Der Vergleich ist semantisch: Zahlen numerisch, Vorabversionen vor der
// Freigabe, und alles Unlesbare ist „unbekannt" statt einer Vermutung.
func TestCompare(t *testing.T) {
	tests := []struct {
		version   string
		reference string
		want      Order
	}{
		{"v0.9.3", "v0.9.4", Older},
		{"v0.9.4", "v0.9.4", Equal},
		{"v0.9.5", "v0.9.4", Newer},
		// Lexikalisch stünde v0.10.0 vor v0.9.4.
		{"v0.9.4", "v0.10.0", Older},
		{"v1.0.0", "v0.99.99", Newer},
		// Ohne „v" ist dieselbe Version gemeint.
		{"0.9.4", "v0.9.4", Equal},
		{" v0.9.4\n", "v0.9.4", Equal},
		// Vorabversionen nach SemVer.
		{"v1.0.0-rc.1", "v1.0.0", Older},
		{"v1.0.0", "v1.0.0-rc.1", Newer},
		{"v1.0.0-rc.2", "v1.0.0-rc.10", Older},
		{"v1.0.0-alpha", "v1.0.0-alpha.1", Older},
		{"v1.0.0-1", "v1.0.0-alpha", Older},
		// Build-Metadaten zählen nicht.
		{"v1.0.0+abc", "v1.0.0", Equal},
		// Nicht lesbar: nie ein Nachweis.
		{"", "v0.9.4", Unknown},
		{"v0.9.4", "", Unknown},
		{"dev", "v0.9.4", Unknown},
		{"v0.9", "v0.9.4", Unknown},
		{"v0.9.x", "v0.9.4", Unknown},
		{"v0.+9.4", "v0.9.4", Unknown},
		{"v0.9.4-", "v0.9.4", Unknown},
	}
	for _, test := range tests {
		if got := Compare(test.version, test.reference); got != test.want {
			t.Errorf("Compare(%q, %q) = %s, erwartet %s", test.version, test.reference, got, test.want)
		}
	}
}
