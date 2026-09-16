package inventory

import (
	"strings"
	"testing"
)

// Unter camelCase-Schlüsseln mit der Endung `Image` gelten dieselben beiden
// Formen wie unter `image`.
func TestHelmValuesErkenntImageSchluesselMitEndungImage(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	result := collectFiles(t, map[string]string{
		"helm/values.yaml": "playwrightMcp:\n" +
			"  gatewayImage:\n" +
			"    repository: registry.example.io/km/omni-gw\n" +
			"    tag: \"\"\n" +
			"    digest: \"\"\n" +
			"  browserImage:\n" +
			"    repository: registry.example.io/km/omni-playwright-mcp\n" +
			"    tag: \"0.0.79-omni.2\"\n" +
			"  pinnedImage:\n" +
			"    repository: registry.example.io/km/pinned\n" +
			"    tag: \"1.0.0\"\n" +
			"    digest: \"" + digest + "\"\n" +
			"  sidecarImage: busybox:1.36\n",
	})

	gateway := find(t, result, "container/registry.example.io/km/omni-gw", "helm/values.yaml")
	if gateway.Pin != PinFloating || gateway.Version != "" || gateway.SourceLine != 3 ||
		gateway.SourceKey != "playwrightMcp.gatewayImage.repository" || gateway.Context != EnvDeployment {
		t.Errorf("gatewayImage mit leerem Tag = %+v", gateway)
	}
	browser := find(t, result, "container/registry.example.io/km/omni-playwright-mcp", "helm/values.yaml")
	if browser.Pin != PinExact || browser.Version != "0.0.79-omni.2" || browser.SourceLine != 7 {
		t.Errorf("browserImage = %+v", browser)
	}
	pinned := find(t, result, "container/registry.example.io/km/pinned", "helm/values.yaml")
	if pinned.Pin != PinDigest || pinned.Digest != digest || pinned.Version != "1.0.0" {
		t.Errorf("pinnedImage mit Digest = %+v", pinned)
	}
	sidecar := find(t, result, "container/busybox", "helm/values.yaml")
	if sidecar.Pin != PinExact || sidecar.Version != "1.36" || sidecar.SourceKey != "playwrightMcp.sidecarImage" {
		t.Errorf("sidecarImage als String = %+v", sidecar)
	}
	if len(result.Entries) != 4 {
		t.Errorf("Einträge = %+v, erwartet 4", result.Entries)
	}
}

// Gegenbeispiele: keine Image-Referenz ohne passenden Schlüssel, und ein
// beliebiger `tag:` ohne Konfiguration ergibt keine Zeile.
func TestHelmValuesRaetKeineAnderenSchluessel(t *testing.T) {
	result := collectFiles(t, map[string]string{
		"helm/values.yaml": "imagePullSecrets:\n" +
			"  - name: registry\n" +
			"IMAGE:\n  repository: nope/upper\n  tag: \"1\"\n" +
			"myimage:\n  repository: nope/lower\n  tag: \"1\"\n" +
			"chart:\n  repository: https://charts.example.io\n  version: 1.2.3\n" +
			"helm-chart-generic:\n" +
			"  redis:\n" +
			"    standalone:\n" +
			"      tag: v7.4.11\n" +
			"    repository: registry.example.io\n",
	})

	if len(result.Entries) != 0 {
		t.Errorf("erwartet keine Zeile, gefunden: %+v", result.Entries)
	}
	if len(result.Notes) != 0 {
		t.Errorf("erwartet keine Hinweise: %+v", result.Notes)
	}
}

func TestIsImageKey(t *testing.T) {
	for key, want := range map[string]bool{
		"image": true, "gatewayImage": true, "browserImage": true, "base2Image": true,
		"Image": false, "IMAGE": false, "myimage": false, "imagePullSecrets": false,
		"image_tag": false, "_Image": false, "images": false,
	} {
		if got := isImageKey(key); got != want {
			t.Errorf("isImageKey(%q) = %v, erwartet %v", key, got, want)
		}
	}
}
