package inventory

import (
	"strings"
	"testing"
)

func TestClassifyPinTrenntFloatingVonUnknown(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"1.2.3", PinExact},
		{"v1.8.0", PinExact},
		{"==1.2.3", PinExact},
		{"1.2.3-rc1", PinExact},
		{"^1", PinRange},
		{">=1,<2", PinRange},
		{"~1.2", PinRange},
		{"1.2.x", PinRange},
		{"", PinFloating},
		{"*", PinFloating},
		{"${IMAGE_TAG}", PinUnknown},
		{"irgendwas", PinUnknown},
		{"sha256:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", PinDigest},
		{"8d9ed9ac5c53483de85588cdf95a591a75ab9f55", PinDigest},
		{"8d9ed9a", PinUnknown},
	}
	for _, testCase := range cases {
		if got := classifyPin(testCase.value); got != testCase.want {
			t.Errorf("classifyPin(%q) = %q, erwartet %q", testCase.value, got, testCase.want)
		}
	}
}

// versionNormalized entsteht nur bei exact: ein normalisierter Bereich wäre
// eine Interpretation, und das Inventar interpretiert nicht.
func TestNormalizeVersionNurBeiExact(t *testing.T) {
	if got := normalizeVersion("==1.2.3", PinExact); got != "1.2.3" {
		t.Errorf("normalizeVersion exact = %q", got)
	}
	if got := normalizeVersion("v1.8.0", PinExact); got != "1.8.0" {
		t.Errorf("führendes v muss fallen: %q", got)
	}
	for _, pin := range []string{PinRange, PinFloating, PinDigest, PinLocal, PinUnknown} {
		if got := normalizeVersion(">=1,<2", pin); got != "" {
			t.Errorf("normalizeVersion(%s) = %q, erwartet leer", pin, got)
		}
	}
}

func TestNormalizeNameFolgtPEP503NurInPython(t *testing.T) {
	if got := normalizeName(EcoPython, "Zope.Interface_x"); got != "zope-interface-x" {
		t.Errorf("Python-Name = %q", got)
	}
	if got := normalizeName(EcoNode, "@Scope/Paket"); got != "@scope/paket" {
		t.Errorf("Node-Scope muss erhalten bleiben: %q", got)
	}
	if got := normalizeName(EcoGo, "github.com/spf13/cobra"); got != "github.com/spf13/cobra" {
		t.Errorf("Go-Modulpfad muss vollständig bleiben: %q", got)
	}
	if groupKey(EcoPython, "redis") == groupKey(EcoContainer, "redis") {
		t.Errorf("der Gruppenschlüssel muss ökosystemlokal sein")
	}
}

func TestParseImageErgaenztKeineRegistry(t *testing.T) {
	digest := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	name, version, pin, gotDigest := imageEntry("redis:7.2.4@" + digest)
	if name != "redis" || version != "7.2.4" || pin != PinDigest || gotDigest != digest {
		t.Errorf("Digest-Referenz = %q %q %q %q", name, version, pin, gotDigest)
	}
	if name, _, _, _ := imageEntry("redis:7.2"); name != "redis" {
		t.Errorf("ein fehlendes docker.io/library darf nicht ergänzt werden: %q", name)
	}
	if _, _, pin, _ := imageEntry("nginx"); pin != PinFloating {
		t.Errorf("ohne Tag = %q, erwartet floating", pin)
	}
	if _, _, pin, _ := imageEntry("nginx:latest"); pin != PinFloating {
		t.Errorf("latest = %q, erwartet floating", pin)
	}
	if _, _, pin, _ := imageEntry("app:${TAG}"); pin != PinUnknown {
		t.Errorf("nicht auflösbare Variable = %q, erwartet unknown", pin)
	}
	// Ein Registry-Port darf nicht als Tag gelesen werden.
	if name, version, _, _ := imageEntry("registry.example:5000/app:1.2"); name != "registry.example:5000/app" || version != "1.2" {
		t.Errorf("Port-Referenz = %q %q", name, version)
	}
}

func TestParseRequirementHaeltExtrasUndMarkerInDerVersion(t *testing.T) {
	entry, ok := parseRequirement("httpx[http2]>=0.27 ; python_version >= '3.11'")
	if !ok {
		t.Fatal("Anforderung wurde nicht gelesen")
	}
	if entry.Name != "httpx" {
		t.Errorf("Name = %q — Extras gehören nicht in den Namen", entry.Name)
	}
	if entry.Version != "[http2]>=0.27 ; python_version >= '3.11'" {
		t.Errorf("Version = %q — Extras und Marker bleiben erhalten", entry.Version)
	}
	if entry.Pin != PinRange {
		t.Errorf("Pin = %q", entry.Pin)
	}

	if entry, _ := parseRequirement("-e ./libs/kern"); entry.Pin != PinLocal || entry.Name != "kern" {
		t.Errorf("Editable = %+v", entry)
	}
	if _, ok := parseRequirement("-e ."); ok {
		t.Errorf("`-e .` ist das Projekt selbst und kein Gegenstand")
	}
	if _, ok := parseRequirement("--index-url https://example.invalid"); ok {
		t.Errorf("Optionen sind keine Versionsaussagen")
	}
	if entry, _ := parseRequirement("paket @ git+https://example.invalid/x.git"); entry.Pin != PinLocal {
		t.Errorf("direkte Referenz = %+v", entry)
	}
}

// Zu jedem `unknown` gehört ein Grund. Die Parser setzen ihn, wo sie ihn genauer
// kennen; für alle übrigen Fälle liefert ihn diese Funktion, damit keine
// Inventarzeile eine Einordnung ohne Begründung trägt.
func TestUnknownReasonNenntDenGrund(t *testing.T) {
	fälle := map[string]string{
		"":                             "keine deutbare",
		"${{ env.GO_VERSION }}":        "nicht auflösbar",
		"$IMAGE_TAG":                   "nicht auflösbar",
		"8d9ed9ac5c53483":              "Kurz-SHA",
		"irgendein-freier-text @ hier": "nicht deutbar",
	}
	for version, want := range fälle {
		if got := unknownReason(version); !strings.Contains(got, want) {
			t.Errorf("unknownReason(%q) = %q, erwartet einen Hinweis auf %q", version, got, want)
		}
	}
}
