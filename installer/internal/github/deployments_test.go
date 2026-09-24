package github

import (
	"strings"
	"testing"
)

// Die Fixture bildet den gemessenen Aufbau der Deployments-API nach — Felder,
// Ref als Tag „<kurz-sha>-<branch>", mehrere Einträge je Environment —, mit
// erfundenen Werten: gesichert wird die Form, nicht der Inhalt eines fremden
// Repos. Absichtlich nicht nach Datum sortiert: die Auswertung verlässt sich
// nicht auf die Reihenfolge der Antwort.
func TestLatestDeploymentsJeEnvironment(t *testing.T) {
	latest, result := LatestDeployments(readFixture(t, "deployments.json"))
	if result.State != StateOK {
		t.Fatalf("Zustand %q", result.State)
	}
	if len(latest) != 2 {
		t.Fatalf("%d Deployments, erwartet je Environment eines: %+v", len(latest), latest)
	}
	dev, prod := latest[0], latest[1]
	if dev.Environment != "dev" || dev.Ref != "1111111-development" || !strings.HasPrefix(dev.SHA, "1111111") {
		t.Errorf("dev = %+v", dev)
	}
	// Das jüngere prod-Deployment steht in der Fixture hinter dem älteren.
	if prod.Environment != "prod" || prod.Ref != "3333333-stage" || prod.CreatedAt != "2026-09-10T15:13:15Z" || prod.Creator != "deploy-bot" {
		t.Errorf("prod = %+v", prod)
	}
}

func TestParseEnvironments(t *testing.T) {
	names, result := ParseEnvironments([]byte(`{"total_count":2,"environments":[{"name":"dev"},{"name":"prod"}]}`))
	if result.State != StateOK || strings.Join(names, ",") != "dev,prod" {
		t.Errorf("names = %v, result = %+v", names, result)
	}
	if _, result := ParseEnvironments([]byte("kein json")); result.State != StateError {
		t.Errorf("unlesbare Antwort = %+v", result)
	}
}

// Fehlt ein bekanntes Environment in der ersten Seite, wird es gezielt
// nachgefragt; sonst verdrängte ein häufiges dev das seltene prod.
func TestFetchLatestDeploymentsFragtFehlendeNach(t *testing.T) {
	fake := &fakeRunner{answers: []fakeAnswer{
		{prefix: "gh api repos/acme/app/deployments?per_page=100", out: `[{"id":1,"environment":"dev","ref":"main","sha":"aaaaaaa","created_at":"2026-09-02T00:00:00Z"}]`},
		{prefix: "gh api repos/acme/app/deployments?per_page=1&environment=prod", out: `[{"id":2,"environment":"prod","ref":"v1","sha":"bbbbbbb","created_at":"2026-08-01T00:00:00Z"}]`},
	}}
	latest, result := fake.client().FetchLatestDeployments(t.Context(), "acme/app", []string{"dev", "prod"})
	if result.State != StateOK || len(latest) != 2 || latest[1].Environment != "prod" {
		t.Errorf("latest = %+v, result = %+v", latest, result)
	}
	if len(fake.calls) != 2 {
		t.Errorf("Aufrufe = %v", fake.calls)
	}
}

func TestRepoDefault(t *testing.T) {
	fake := &fakeRunner{answers: []fakeAnswer{
		{prefix: "gh repo view --json nameWithOwner,defaultBranchRef", out: `{"nameWithOwner":"acme/app","defaultBranchRef":{"name":"master"}}`},
	}}
	repo, branch, result := fake.client().RepoDefault(t.Context())
	if repo != "acme/app" || branch != "master" || result.State != StateOK {
		t.Errorf("RepoDefault = %q %q %+v", repo, branch, result)
	}
}
