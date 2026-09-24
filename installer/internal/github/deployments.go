package github

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Die Abfragen hier dienen der Seite /branches: Default-Branch, Environments und
// das letzte Deployment je Environment. Wie alles in diesem Paket nur lesend.

// deploymentPageSize ist die Zahl der Deployments, die der erste Aufruf holt.
// Ein Environment, das darin nicht vorkommt, bekommt einen eigenen Aufruf mit
// environment=<name> — sonst verdrängte ein häufig ausgerolltes dev das seltene
// prod aus der Liste.
const deploymentPageSize = 100

// RepoDefault löst `owner/name` und den Default-Branch in einem Aufruf auf.
func (c *Client) RepoDefault(ctx context.Context) (string, string, Result) {
	raw, err := c.gh(ctx, "repo", "view", "--json", "nameWithOwner,defaultBranchRef")
	if err != nil {
		return "", "", Classify(err)
	}
	var view repoView
	if json.Unmarshal(raw, &view) != nil || view.NameWithOwner == "" {
		return "", "", Result{State: StateError, Message: Explain(StateError) + " Die Antwort von gh war kein lesbares JSON."}
	}
	return view.NameWithOwner, view.DefaultBranchRef.Name, Result{State: StateOK}
}

// Deployment ist ein Eintrag aus GET /repos/{o}/{r}/deployments, auf das
// gekürzt, was die Seite braucht.
type Deployment struct {
	ID          int64  `json:"id"`
	Environment string `json:"environment"`
	// Ref ist, was ausgerollt wurde: ein Branch, ein Tag oder ein SHA. Sein
	// Namensmuster ist je Projekt verschieden und wird nicht gedeutet; allgemein
	// auswertbar ist nur SHA.
	Ref       string `json:"ref"`
	SHA       string `json:"sha"`
	CreatedAt string `json:"createdAt"`
	Creator   string `json:"creator"`
}

type deploymentEntry struct {
	ID          int64  `json:"id"`
	Environment string `json:"environment"`
	Ref         string `json:"ref"`
	SHA         string `json:"sha"`
	CreatedAt   string `json:"created_at"`
	Creator     *struct {
		Login string `json:"login"`
	} `json:"creator"`
}

type environmentsPayload struct {
	Environments []struct {
		Name string `json:"name"`
	} `json:"environments"`
}

// FetchEnvironments liest die Namen der GitHub-Environments.
func (c *Client) FetchEnvironments(ctx context.Context, repo string) ([]string, Result) {
	if !ValidRepo(repo) {
		return []string{}, Fail(StateNoRemote)
	}
	raw, err := c.gh(ctx, "api", "repos/"+repo+"/environments?per_page=100")
	if err != nil {
		return []string{}, Classify(err)
	}
	return ParseEnvironments(raw)
}

// ParseEnvironments wertet die Antwort der Environments-API aus.
func ParseEnvironments(raw []byte) ([]string, Result) {
	var payload environmentsPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return []string{}, Result{State: StateError, Message: Explain(StateError) + " Die Antwort von gh war kein lesbares JSON."}
	}
	names := []string{}
	for _, environment := range payload.Environments {
		if name := strings.TrimSpace(environment.Name); name != "" {
			names = append(names, name)
		}
	}
	return names, Result{State: StateOK}
}

// FetchLatestDeployments liefert je Environment das jüngste Deployment.
// environments sind die bekannten Namen: fehlt einer in der ersten Seite, wird
// er gezielt nachgefragt.
func (c *Client) FetchLatestDeployments(ctx context.Context, repo string, environments []string) ([]Deployment, Result) {
	if !ValidRepo(repo) {
		return []Deployment{}, Fail(StateNoRemote)
	}
	raw, err := c.gh(ctx, "api", "repos/"+repo+"/deployments?per_page="+strconv.Itoa(deploymentPageSize))
	if err != nil {
		return []Deployment{}, Classify(err)
	}
	latest, result := LatestDeployments(raw)
	if result.State != StateOK {
		return []Deployment{}, result
	}

	seen := map[string]bool{}
	for _, deployment := range latest {
		seen[deployment.Environment] = true
	}
	for _, name := range environments {
		if seen[name] {
			continue
		}
		raw, err := c.gh(ctx, "api", "repos/"+repo+"/deployments?per_page=1&environment="+url.QueryEscape(name))
		if err != nil {
			return latest, Classify(err)
		}
		more, result := LatestDeployments(raw)
		if result.State != StateOK {
			return latest, result
		}
		latest = append(latest, more...)
	}
	sortDeployments(latest)
	return latest, Result{State: StateOK}
}

// LatestDeployments wertet eine Seite der Deployments-API aus und behält je
// Environment das jüngste. Sortiert wird selbst, nach created_at: auf die
// Reihenfolge der Antwort verlässt sich die Auswertung nicht.
func LatestDeployments(raw []byte) ([]Deployment, Result) {
	var entries []deploymentEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return []Deployment{}, Result{State: StateError, Message: Explain(StateError) + " Die Antwort von gh war kein lesbares JSON."}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].CreatedAt > entries[j].CreatedAt })

	latest := []Deployment{}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.Environment == "" || seen[entry.Environment] {
			continue
		}
		seen[entry.Environment] = true
		deployment := Deployment{
			ID:          entry.ID,
			Environment: entry.Environment,
			Ref:         entry.Ref,
			SHA:         entry.SHA,
			CreatedAt:   entry.CreatedAt,
		}
		if entry.Creator != nil {
			deployment.Creator = entry.Creator.Login
		}
		latest = append(latest, deployment)
	}
	sortDeployments(latest)
	return latest, Result{State: StateOK}
}

func sortDeployments(deployments []Deployment) {
	sort.SliceStable(deployments, func(i, j int) bool { return deployments[i].Environment < deployments[j].Environment })
}
