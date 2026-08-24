package sarifconvert

import (
	"encoding/json"
	"strings"
	"testing"
)

// realLogZeile ist eine echte Log-Zeile von trufflehog 3.97.0 (im
// tatsächlichen Aufrufpfad läuft sie über stderr, nie über das stdout, das
// dieser Konverter liest — siehe Task 024, Abschnitt „Kontext"). Sie steht
// hier trotzdem mit im NDJSON, um die defensive Verwerfung zu prüfen.
const realLogZeile = `{"level":"info-0","ts":"2026-08-24T14:40:27+02:00","logger":"trufflehog","msg":"finished scanning","chunks":2,"bytes":145,"verified_secrets":0,"unverified_secrets":1}`

// realFundZeileOhneRedacted ist eine echte Fund-Zeile von trufflehog 3.97.0
// (Katalog-Aufruf `git file://{target} --json --no-update`, Detektor
// SlackWebhook, gegen ein Testrepo mit einem Fake-Webhook als Secret). Der
// SlackWebhook-Detektor liefert kein Redacted — ein realer Fall, den der
// Konverter über einen eigenen Platzhalter auffangen muss.
//
// Der Secret-Wert selbst ist gegenüber der Originalausgabe entschärft
// (hooks.slack.invalid statt des echten Hosts): GitHubs Push Protection
// blockt sonst jeden Push, der diese Datei anfasst. Für den Konverter ist
// der Wert ohnehin opak — geprüft wird nur, dass er nirgends durchschlägt.
const realFundZeileOhneRedacted = `{"SourceMetadata":{"Data":{"Git":{"commit":"cc31cbf991892f3ff02416f39b4dffa9421f22a6","file":"secret.txt","email":"test <test@test.com>","repository":"file:///tmp/thtest3","timestamp":"2026-08-24 12:40:24 +0000","line":1}}},"SourceID":1,"SourceType":16,"SourceName":"trufflehog - git","DetectorType":30,"DetectorName":"SlackWebhook","DetectorDescription":"Slack webhooks are used to send messages from external sources into Slack channels. If compromised, they can be used to send unauthorized messages.","DecoderName":"PLAIN","Verified":false,"VerificationFromCache":false,"Raw":"https://hooks.slack.invalid/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX","RawV2":"","Redacted":"","ExtraData":{"rotation_guide":"https://howtorotate.com/docs/tutorials/slack-webhook/"},"StructuredData":null,"SecretParts":{"key":"https://hooks.slack.invalid/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX"}}`

// realFundZeileMitRedacted ist dieselbe Herkunft, Detektor PrivateKey gegen
// ein Testrepo mit einem generierten RSA-Schlüssel. Verified ist hier von
// false auf true umgestellt, um die level-Ableitung zu prüfen. Der
// Schlüsselkörper in Raw und SecretParts ist wie oben durch einen
// synthetischen Sentinel ersetzt; Redacted bleibt der echte Präfix und damit
// weiterhin ein Präfix von Raw — die Struktur, auf die es dem Konverter
// ankommt, ist unverändert.
const realFundZeileMitRedacted = `{"SourceMetadata":{"Data":{"Git":{"commit":"d52827538234dd8c9e1af54ac96c70f24338837c","file":"key.pem","email":"test <test@test.com>","repository":"file:///tmp/thtest5","timestamp":"2026-08-24 12:41:26 +0000","line":1}}},"SourceID":1,"SourceType":16,"SourceName":"trufflehog - git","DetectorType":15,"DetectorName":"PrivateKey","DetectorDescription":"Private keys are used for securely connecting and authenticating to various systems and services. Exposure of private keys can lead to unauthorized access and data breaches.","DecoderName":"PLAIN","Verified":true,"VerificationFromCache":false,"Raw":"-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwNURTESTDATENKEINECHTERSCHLUESSEL\n-----END PRIVATE KEY-----\n","RawV2":"","Redacted":"-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcw","ExtraData":null,"StructuredData":null,"SecretParts":{"token":"-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwNURTESTDATENKEINECHTERSCHLUESSEL\n-----END PRIVATE KEY-----\n"}}`

func TestTruffleHogLeereEingabeLiefertValidesLeeresSARIF(t *testing.T) {
	data, err := TruffleHog(nil)
	if err != nil {
		t.Fatalf("TruffleHog(nil): %v", err)
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
	if document.Runs[0].Tool.Driver.Name != "trufflehog" {
		t.Errorf("driver.name = %q, erwartet trufflehog", document.Runs[0].Tool.Driver.Name)
	}
	if len(document.Runs[0].Results) != 0 {
		t.Errorf("results = %+v, erwartet leer", document.Runs[0].Results)
	}

	// Auch mit 0 Byte statt nil muss dasselbe gelten — der Normalfall bei 0
	// Funden im echten Aufrufpfad (siehe execute.go, runJob).
	data, err = TruffleHog([]byte{})
	if err != nil {
		t.Fatalf("TruffleHog([]byte{}): %v", err)
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("Ergebnis ist kein lesbares JSON: %v", err)
	}
	if len(document.Runs) != 1 || len(document.Runs[0].Results) != 0 {
		t.Errorf("runs = %+v, erwartet einen leeren Run", document.Runs)
	}
}

// Log-Zeile und Fund-Zeile zusammen: die Log-Zeile wird verworfen, die
// Fund-Zeile korrekt übersetzt. Der Detektor liefert kein Redacted — der
// Platzhalter tritt an dessen Stelle, das Rohsecret (Raw) landet nirgends im
// Ergebnis.
func TestTruffleHogFundZeileOhneRedactedWirdSecretfreiUebersetzt(t *testing.T) {
	input := strings.Join([]string{realLogZeile, realFundZeileOhneRedacted}, "\n")

	data, err := TruffleHog([]byte(input))
	if err != nil {
		t.Fatalf("TruffleHog: %v", err)
	}

	raw := string(data)
	if strings.Contains(raw, "hooks.slack.invalid/services/T00000000") {
		t.Fatalf("Ergebnis enthält das Rohsecret (Raw): %s", raw)
	}

	var document sarifLog
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("Ergebnis ist kein lesbares JSON: %v", err)
	}
	if len(document.Runs) != 1 || len(document.Runs[0].Results) != 1 {
		t.Fatalf("results = %+v, erwartet genau einen Befund (Log-Zeile verworfen)", document.Runs)
	}
	rules := document.Runs[0].Tool.Driver.Rules
	if len(rules) != 1 || rules[0].ID != "SlackWebhook" {
		t.Fatalf("rules = %+v, erwartet genau eine Rule SlackWebhook", rules)
	}

	result := document.Runs[0].Results[0]
	if result.RuleID != "SlackWebhook" {
		t.Errorf("ruleId = %q, erwartet SlackWebhook", result.RuleID)
	}
	if result.Level != "warning" {
		t.Errorf("level = %q, erwartet warning (unverifiziert)", result.Level)
	}
	if result.Message.Text == "" || strings.Contains(result.Message.Text, "hooks.slack.invalid") {
		t.Errorf("message.text = %q, erwartet einen secretfreien Platzhalter", result.Message.Text)
	}
	if len(result.Locations) != 1 {
		t.Fatalf("locations = %+v, erwartet genau eine Location", result.Locations)
	}
	location := result.Locations[0].PhysicalLocation
	if location.ArtifactLocation.URI != "secret.txt" {
		t.Errorf("uri = %q, erwartet secret.txt", location.ArtifactLocation.URI)
	}
	if location.Region == nil || location.Region.StartLine != 1 {
		t.Errorf("region = %+v, erwartet startLine 1", location.Region)
	}
}

// Ein verifizierter Fund mit gesetztem Redacted: level wird error, und
// message.text ist die gekürzte Fassung — nicht Raw oder RawV2, obwohl beide
// in der Eingabe stehen.
func TestTruffleHogVerifizierterFundWirdError(t *testing.T) {
	data, err := TruffleHog([]byte(realFundZeileMitRedacted))
	if err != nil {
		t.Fatalf("TruffleHog: %v", err)
	}

	raw := string(data)
	if strings.Contains(raw, "NURTESTDATENKEINECHTERSCHLUESSEL") {
		t.Fatalf("Ergebnis enthält Rohmaterial aus Raw/RawV2, das nicht in Redacted steht: %s", raw)
	}

	var document sarifLog
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("Ergebnis ist kein lesbares JSON: %v", err)
	}
	if len(document.Runs) != 1 || len(document.Runs[0].Results) != 1 {
		t.Fatalf("results = %+v, erwartet genau einen Befund", document.Runs)
	}
	result := document.Runs[0].Results[0]
	if result.RuleID != "PrivateKey" {
		t.Errorf("ruleId = %q, erwartet PrivateKey", result.RuleID)
	}
	if result.Level != "error" {
		t.Errorf("level = %q, erwartet error (verifiziert)", result.Level)
	}
	if !strings.HasPrefix(result.Message.Text, "-----BEGIN PRIVATE KEY-----") {
		t.Errorf("message.text = %q, erwartet den Redacted-Wert", result.Message.Text)
	}
	if len(result.Locations) != 1 || result.Locations[0].PhysicalLocation.ArtifactLocation.URI != "key.pem" {
		t.Errorf("locations = %+v, erwartet key.pem", result.Locations)
	}
}
