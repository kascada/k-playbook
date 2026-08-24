package sarifconvert

import (
	"encoding/json"
	"strings"
	"testing"
)

// realPipAuditAusgabe ist echte Ausgabe von pip-audit 2.10.0 (Katalog-Aufruf
// `--format json --progress-spinner off -r <datei>`), gekürzt auf drei der
// aufgelösten Pakete — inhaltlich unverändert.
//
// Sie deckt die drei Fälle ab, die der Konverter unterscheiden muss: ein Paket
// mit einer Schwachstelle, eines mit mehreren, und eines, das geprüft und
// sauber ist (leere vulns-Liste, kein Result). Die Beschreibung von
// PYSEC-2019-133 ist mit 351 Zeichen länger als pipAuditMessageLimit und
// belegt damit zugleich die Kürzung.
const realPipAuditAusgabe = `{"dependencies": [{"name": "requests", "version": "2.19.0", "vulns": [{"id": "PYSEC-2018-28", "fix_versions": ["2.20.0"], "aliases": ["GHSA-x84v-xcm2-53pg", "CVE-2018-18074"], "description": "The Requests package before 2.20.0 for Python sends an HTTP Authorization header to an http URI upon receiving a same-hostname https-to-http redirect, which makes it easier for remote attackers to discover credentials by sniffing the network."}]}, {"name": "urllib3", "version": "1.23", "vulns": [{"id": "PYSEC-2019-133", "fix_versions": ["1.24.2"], "aliases": ["CVE-2019-11324", "GHSA-mh33-7rrq-662w"], "description": "The urllib3 library before 1.24.2 for Python mishandles certain cases where the desired set of CA certificates is different from the OS store of CA certificates, which results in SSL connections succeeding in situations where a verification failure is the correct outcome. This is related to use of the ssl_context, ca_certs, or ca_certs_dir argument."}, {"id": "PYSEC-2019-132", "fix_versions": ["1.24.3"], "aliases": ["GHSA-r64q-w8jr-g9qp", "CVE-2019-11236"], "description": "In the urllib3 library through 1.24.1 for Python, CRLF injection is possible if the attacker controls the request parameter."}]}, {"name": "chardet", "version": "3.0.4", "vulns": []}], "fixes": []}`

// pipAuditKonvertiert liest die Ausgabe des Konverters zurück — als dieselbe
// Struktur, mit der er sie geschrieben hat.
func pipAuditKonvertiert(t *testing.T, raw string, manifest string) sarifLog {
	t.Helper()
	data, err := PipAudit([]byte(raw), manifest)
	if err != nil {
		t.Fatalf("PipAudit: %v", err)
	}
	var document sarifLog
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("Ergebnis ist kein lesbares JSON: %v", err)
	}
	if document.Version != "2.1.0" {
		t.Errorf("version = %q, erwartet 2.1.0", document.Version)
	}
	if len(document.Runs) != 1 {
		t.Fatalf("runs = %+v, erwartet genau einen Run", document.Runs)
	}
	if document.Runs[0].Tool.Driver.Name != "pip-audit" {
		t.Errorf("driver.name = %q, erwartet pip-audit", document.Runs[0].Tool.Driver.Name)
	}
	return document
}

// Ein Result je Schwachstelle, nicht je Paket: ein Paket mit mehreren Lücken
// ist mehrfach zu beheben. Das saubere Paket ergibt keins.
func TestPipAuditEinResultJeSchwachstelle(t *testing.T) {
	document := pipAuditKonvertiert(t, realPipAuditAusgabe, "requirements.txt")
	results := document.Runs[0].Results

	if len(results) != 3 {
		t.Fatalf("%d Results, erwartet 3 (1 für requests, 2 für urllib3, keins für chardet): %+v", len(results), results)
	}
	for _, result := range results {
		if strings.Contains(result.Message.Text, "chardet") {
			t.Errorf("chardet hat ein Result bekommen, obwohl es keine Schwachstelle trägt: %q", result.Message.Text)
		}
	}

	// Bevorzugt die CVE-Alias, gleich an welcher Stelle sie steht: unter ihr
	// steht dieselbe Schwachstelle bei den anderen Werkzeugen des Laufs.
	ids := []string{}
	for _, result := range results {
		ids = append(ids, result.RuleID)
	}
	want := "CVE-2018-18074,CVE-2019-11324,CVE-2019-11236"
	if strings.Join(ids, ",") != want {
		t.Errorf("ruleIds = %v, erwartet %s", ids, want)
	}

	// Je Kennung eine Regel, keine doppelte.
	if len(document.Runs[0].Tool.Driver.Rules) != 3 {
		t.Errorf("rules = %+v, erwartet drei", document.Runs[0].Tool.Driver.Rules)
	}
}

// Die Property-Konvention ist der Hebel, über den merge.extractDependency die
// Abhängigkeit erkennt, ohne pip-audit zu kennen.
func TestPipAuditSetztPropertyKonvention(t *testing.T) {
	document := pipAuditKonvertiert(t, realPipAuditAusgabe, "dienste/requirements-dev.txt")
	result := document.Runs[0].Results[0]

	for key, want := range map[string]string{
		"package":     "requests",
		"version":     "2.19.0",
		"manifest":    "dienste/requirements-dev.txt",
		"id":          "PYSEC-2018-28",
		"fixVersions": "2.20.0",
	} {
		if result.Properties[key] != want {
			t.Errorf("properties[%s] = %q, erwartet %q", key, result.Properties[key], want)
		}
	}
	if !strings.Contains(result.Properties["aliases"], "GHSA-x84v-xcm2-53pg") {
		t.Errorf("properties[aliases] = %q, erwartet die GHSA-Alias", result.Properties["aliases"])
	}

	// pip-audit liefert keine Schwere. Ein gesetztes level oder eine
	// severity-Property gälte in deriveSeverity als Angabe des Werkzeugs —
	// eine, die es nie gemacht hat.
	if result.Level != "" {
		t.Errorf("level = %q, erwartet leer — pip-audit liefert keine Schwere", result.Level)
	}
	for _, key := range []string{"severity", "impact", "priority", "security-severity"} {
		if _, gesetzt := result.Properties[key]; gesetzt {
			t.Errorf("properties[%s] ist gesetzt, obwohl pip-audit keine Schwere liefert", key)
		}
	}

	// Die Fundstelle ist das geprüfte Manifest — ohne Zeile, die es nicht gibt.
	if len(result.Locations) != 1 {
		t.Fatalf("locations = %+v, erwartet genau eine", result.Locations)
	}
	location := result.Locations[0].PhysicalLocation
	if location.ArtifactLocation.URI != "dienste/requirements-dev.txt" {
		t.Errorf("artifactLocation.uri = %q, erwartet das geprüfte Manifest", location.ArtifactLocation.URI)
	}
	if location.Region != nil {
		t.Errorf("region = %+v, erwartet keine — pip-audit nennt keine Zeile", location.Region)
	}
}

// Ohne bekanntes Manifest bleibt die Angabe weg, statt falsch zu sein.
func TestPipAuditOhneManifestLaesstAngabeWeg(t *testing.T) {
	document := pipAuditKonvertiert(t, realPipAuditAusgabe, "")
	result := document.Runs[0].Results[0]

	if _, gesetzt := result.Properties["manifest"]; gesetzt {
		t.Errorf("properties[manifest] = %q, erwartet gar keine Angabe", result.Properties["manifest"])
	}
	if len(result.Locations) != 0 {
		t.Errorf("locations = %+v, erwartet keine", result.Locations)
	}
	if result.Properties["package"] != "requests" {
		t.Errorf("properties[package] = %q — Paket und Version stehen auch ohne Manifest fest", result.Properties["package"])
	}
}

// Der Text muss allein stehen können: Paket, Version, Kennung, gekürzte
// Beschreibung, Fix-Version.
func TestPipAuditMeldungTraegtPaketUndFix(t *testing.T) {
	document := pipAuditKonvertiert(t, realPipAuditAusgabe, "requirements.txt")
	text := document.Runs[0].Results[0].Message.Text

	for _, teil := range []string{"requests 2.19.0", "CVE-2018-18074", "behoben in 2.20.0"} {
		if !strings.Contains(text, teil) {
			t.Errorf("message.text = %q, erwartet %q darin", text, teil)
		}
	}
}

// pip-audits Beschreibungen sind mehrere hundert Zeichen Markdown-Freitext.
// Ungekürzt trüge das jede Ergebnisansicht mit.
func TestPipAuditKuerztLangeBeschreibung(t *testing.T) {
	document := pipAuditKonvertiert(t, realPipAuditAusgabe, "requirements.txt")

	// Das zweite Result ist PYSEC-2019-133 mit 351 Zeichen Beschreibung.
	text := document.Runs[0].Results[1].Message.Text
	if !strings.Contains(text, "…") {
		t.Errorf("message.text = %q, erwartet die Kürzungsmarke", text)
	}
	if länge := len([]rune(text)); länge > pipAuditMessageLimit+120 {
		t.Errorf("message.text ist %d Zeichen lang — die Kürzung greift nicht", länge)
	}

	// Das dritte Result (PYSEC-2019-132, 124 Zeichen) bleibt vollständig.
	kurz := document.Runs[0].Results[2].Message.Text
	if !strings.Contains(kurz, "CRLF injection is possible if the attacker controls the request parameter.") {
		t.Errorf("message.text = %q, erwartet die vollständige kurze Beschreibung", kurz)
	}
}

// Ohne CVE-Alias bleibt pip-audits eigene Kennung stehen, und ohne jede
// Kennung wird der Fund nicht verworfen: ein Fund ohne Kennung ist immer noch
// ein Fund.
func TestPipAuditKennungOhneCVE(t *testing.T) {
	raw := `{"dependencies": [{"name": "beispiel", "version": "1.0", "vulns": [` +
		`{"id": "PYSEC-2020-1", "fix_versions": [], "aliases": ["GHSA-aaaa-bbbb-cccc"], "description": "ohne CVE"},` +
		`{"id": "", "fix_versions": [], "aliases": [], "description": "ohne jede Kennung"}]}], "fixes": []}`

	document := pipAuditKonvertiert(t, raw, "requirements.txt")
	results := document.Runs[0].Results
	if len(results) != 2 {
		t.Fatalf("%d Results, erwartet 2: %+v", len(results), results)
	}
	if results[0].RuleID != "PYSEC-2020-1" {
		t.Errorf("ruleId = %q, erwartet PYSEC-2020-1", results[0].RuleID)
	}
	if results[1].RuleID != pipAuditUnknownRule {
		t.Errorf("ruleId = %q, erwartet %s", results[1].RuleID, pipAuditUnknownRule)
	}
}

// Ein Lauf ohne Fund schreibt trotzdem ein vollständiges Dokument — und ergibt
// valides SARIF mit leerer Ergebnisliste.
func TestPipAuditOhneFundLiefertLeeresSARIF(t *testing.T) {
	document := pipAuditKonvertiert(t, `{"dependencies": [{"name": "chardet", "version": "3.0.4", "vulns": []}], "fixes": []}`, "requirements.txt")

	if len(document.Runs[0].Results) != 0 {
		t.Errorf("results = %+v, erwartet leer", document.Runs[0].Results)
	}
}

// Anders als bei trufflehog ist leere Eingabe hier ein Fehler: pip-audit
// schreibt auch ohne Fund ein Dokument. Nichts zu bekommen heißt, dass der
// Lauf nicht zustande kam — ein leeres SARIF sähe wie ein sauberer Scan aus.
func TestPipAuditLeereOderKaputteEingabeIstFehler(t *testing.T) {
	fälle := map[string][]byte{
		"nil":             nil,
		"leer":            {},
		"kein JSON":       []byte("Traceback (most recent call last):"),
		"halbes Dokument": []byte(`{"dependencies": [`),
	}
	for name, raw := range fälle {
		if _, err := PipAudit(raw, "requirements.txt"); err == nil {
			t.Errorf("%s wurde als Ergebnis angenommen", name)
		}
	}
}
