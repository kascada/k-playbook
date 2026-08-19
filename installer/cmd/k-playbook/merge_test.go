package main

import "testing"

func TestFormatBuildVersion(t *testing.T) {
	fälle := []struct {
		name     string
		version  string
		settings map[string]string
		want     string
	}{
		{name: "Release", version: "v1.2.3", settings: map[string]string{"vcs.revision": "abcdef1234567890"}, want: "v1.2.3"},
		{name: "Lokal", settings: map[string]string{"vcs.revision": "abcdef1234567890"}, want: "abcdef1"},
		{name: "Dirty", settings: map[string]string{"vcs.revision": "abcdef1234567890", "vcs.modified": "true"}, want: "abcdef1-dirty"},
		{name: "Go run", version: "(devel)", settings: map[string]string{"vcs.revision": "abcdef1234567890"}, want: "(devel)+abcdef1"},
		{name: "Leer", settings: map[string]string{}, want: "unknown"},
	}
	for _, fall := range fälle {
		if got := formatBuildVersion(fall.version, fall.settings); got != fall.want {
			t.Errorf("%s: %q, erwartet %q", fall.name, got, fall.want)
		}
	}
}
