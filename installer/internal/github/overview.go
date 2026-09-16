package github

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
)

// Overview ist die Kopfzeile der Ansicht: welches Repo, welches Konto, welches
// Recht, wie steht die CI auf dem Default-Branch und wie weit ist der
// Arbeitsstand seit dem letzten Tag.
type Overview struct {
	Result
	// Repo ist `owner/name`, wie gh es aufgelöst hat — nicht aus der
	// Remote-URL geparst. Das Remote kann ein SSH-Alias sein, den nur die
	// SSH-Konfiguration auflöst.
	Repo          string `json:"repo"`
	URL           string `json:"url"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"defaultBranch"`
	// Account ist das Konto, mit dem gh die API anspricht (`gh api user`).
	Account string `json:"account"`
	// Permission ist `viewerPermission`: READ, TRIAGE, WRITE, MAINTAIN, ADMIN.
	// Leserecht ist nicht Schreibrecht — „angemeldet" allein sagt nichts
	// darüber, ob über gh gemergt oder approved werden kann.
	Permission string `json:"permission"`
	// RemoteURL ist die URL von `git remote get-url origin`, unverändert.
	RemoteURL string `json:"remoteUrl"`
	// SSHAlias ist der Hostname der Remote-URL, wenn er nicht github.com ist.
	// Dann läuft `git push` über einen Eintrag der SSH-Konfiguration mit
	// eigenem Schlüssel.
	SSHAlias string `json:"sshAlias"`
	// AliasHint ist der Satz dazu. Er nennt den Alias und **kein Konto**: der
	// Aliasname ist kein Kontoname, und welches Konto hinter dem Schlüssel
	// steht, wäre nur über einen weiteren Netzaufruf zu erfahren. Den macht
	// diese Seite nicht.
	AliasHint string `json:"aliasHint"`
	// Workflows ist der letzte Lauf je Workflow auf dem Default-Branch.
	Workflows []WorkflowState `json:"workflows"`
	// CI fasst die Workflows zu einem Wort zusammen: ok, warn, error oder
	// leer, wenn es keine Läufe gibt.
	CI string `json:"ci"`
	// LastTag ist der letzte Tag im Arbeitsstand, CommitsSinceTag die Zahl der
	// Commits darüber hinaus. Beides kommt lokal aus git, nicht aus der API.
	LastTag         string `json:"lastTag"`
	LastTagDate     string `json:"lastTagDate"`
	CommitsSinceTag int    `json:"commitsSinceTag"`
	// TagNote sagt, warum Tag-Angaben fehlen, statt sie leer zu lassen.
	TagNote string `json:"tagNote"`
}

// WorkflowState ist der letzte Lauf eines Workflows.
type WorkflowState struct {
	Name       string `json:"name"`
	RunID      int64  `json:"runId"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Branch     string `json:"branch"`
	Event      string `json:"event"`
	Title      string `json:"title"`
	CreatedAt  string `json:"createdAt"`
	URL        string `json:"url"`
}

type repoView struct {
	NameWithOwner    string `json:"nameWithOwner"`
	URL              string `json:"url"`
	IsPrivate        bool   `json:"isPrivate"`
	ViewerPermission string `json:"viewerPermission"`
	DefaultBranchRef struct {
		Name string `json:"name"`
	} `json:"defaultBranchRef"`
}

type apiUser struct {
	Login string `json:"login"`
}

// FetchOverview liest die Kopfzeile. Drei gh-Aufrufe und vier git-Aufrufe; die
// git-Aufrufe kosten kein Netz.
//
// Scheitert der erste Aufruf, ist die Antwort der eingeordnete Zustand und
// sonst nichts: ohne Repo gibt es keine Kopfzeile. Scheitern spätere Aufrufe,
// bleibt der Zustand ok und das einzelne Feld leer — ein fehlender Tag macht
// die Repo-Angabe nicht falsch.
// failedOverview ist die Kopfzeile ohne Daten. Sie hält die Zusage des Pakets,
// dass eine Liste leer ist und nicht null: die Oberfläche soll über einen
// Fehlerpfad nicht anders stolpern als über einen leeren Erfolg.
func failedOverview(result Result) Overview {
	return Overview{Result: result, Workflows: []WorkflowState{}}
}

func (c *Client) FetchOverview(ctx context.Context) Overview {
	overview := Overview{Result: Result{State: StateOK}, Workflows: []WorkflowState{}}

	raw, err := c.gh(ctx, "repo", "view", "--json", "nameWithOwner,defaultBranchRef,viewerPermission,url,isPrivate")
	if err != nil {
		return failedOverview(Classify(err))
	}
	var view repoView
	if err := json.Unmarshal(raw, &view); err != nil {
		return failedOverview(Result{State: StateError, Message: Explain(StateError) + " Die Antwort von gh war kein lesbares JSON."})
	}
	overview.Repo = view.NameWithOwner
	overview.URL = view.URL
	overview.Private = view.IsPrivate
	overview.Permission = view.ViewerPermission
	overview.DefaultBranch = view.DefaultBranchRef.Name

	if raw, err := c.gh(ctx, "api", "user"); err == nil {
		var user apiUser
		if json.Unmarshal(raw, &user) == nil {
			overview.Account = user.Login
		}
	} else if result := Classify(err); result.State == StateBadCredentials {
		// `gh repo view` kann aus dem Cache antworten, `gh api user` nicht.
		// Ein abgewiesener Token gehört deshalb hier noch gemeldet.
		return failedOverview(result)
	}

	overview.readRemote(ctx, c)
	overview.readTag(ctx, c)
	overview.readWorkflows(ctx, c)
	return overview
}

// readRemote liest die Remote-URL und erkennt einen SSH-Alias. Ohne Netz: der
// Alias wird am Hostnamen erkannt, nicht aufgelöst, und es gibt kein `ssh -T`.
func (o *Overview) readRemote(ctx context.Context, c *Client) {
	raw, err := c.git(ctx, "remote", "get-url", "origin")
	if err != nil {
		return
	}
	o.RemoteURL = strings.TrimSpace(string(raw))
	if alias := SSHAliasOf(o.RemoteURL); alias != "" {
		o.SSHAlias = alias
		o.AliasHint = AliasHint(alias)
	}
}

// githubHosts sind die Hostnamen, hinter denen wirklich github.com steht. Alles
// andere in einer SSH-Remote-URL ist ein Eintrag der SSH-Konfiguration.
var githubHosts = map[string]bool{
	"github.com":     true,
	"ssh.github.com": true,
	"www.github.com": true,
}

// SSHAliasOf liefert den Hostnamen einer Remote-URL, wenn er nicht github.com
// ist — sonst den leeren String. Erkannt werden `git@host:owner/repo.git`,
// `ssh://git@host/owner/repo.git` und die https-Form (die nie ein Alias ist).
func SSHAliasOf(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	host := ""
	switch {
	case strings.HasPrefix(remote, "ssh://"):
		rest := strings.TrimPrefix(remote, "ssh://")
		if at := strings.Index(rest, "@"); at >= 0 {
			rest = rest[at+1:]
		}
		host = rest
		if slash := strings.IndexAny(host, "/:"); slash >= 0 {
			host = host[:slash]
		}
	case strings.HasPrefix(remote, "http://"), strings.HasPrefix(remote, "https://"):
		return ""
	default:
		// scp-Form: [user@]host:pfad
		colon := strings.Index(remote, ":")
		if colon < 0 {
			return ""
		}
		host = remote[:colon]
		if at := strings.Index(host, "@"); at >= 0 {
			host = host[at+1:]
		}
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || githubHosts[host] {
		return ""
	}
	return host
}

// AliasHint ist der Satz zum SSH-Alias. Er nennt **keinen Kontonamen**: der
// Alias ist ein Eintrag in ~/.ssh/config, und welches Konto zu seinem Schlüssel
// gehört, steht nirgends im Arbeitsstand. Es zu erfahren, hieße `ssh -T` zu
// rufen — ein weiterer Netzaufruf, den diese Seite nicht macht.
// Der Satz trägt keine Auszeichnung: er geht als Text in die Seite, und
// Backticks stünden dort als Backticks.
func AliasHint(alias string) string {
	return "Der Befehl git push läuft über den SSH-Alias „" + alias + "“ (eigener Schlüssel) und damit nicht über das gh-Konto. " +
		"Was gh hier liest, sagt deshalb nichts darüber, unter welchem Konto gepusht wird."
}

func (o *Overview) readTag(ctx context.Context, c *Client) {
	raw, err := c.git(ctx, "describe", "--tags", "--abbrev=0")
	if err != nil {
		o.TagNote = "Kein Tag im Arbeitsstand gefunden."
		return
	}
	o.LastTag = strings.TrimSpace(string(raw))
	if o.LastTag == "" {
		o.TagNote = "Kein Tag im Arbeitsstand gefunden."
		return
	}
	if raw, err := c.git(ctx, "rev-list", o.LastTag+"..HEAD", "--count"); err == nil {
		if count, convErr := strconv.Atoi(strings.TrimSpace(string(raw))); convErr == nil {
			o.CommitsSinceTag = count
		}
	}
	if raw, err := c.git(ctx, "log", "-1", "--format=%cI", o.LastTag); err == nil {
		o.LastTagDate = strings.TrimSpace(string(raw))
	}
}

type runListEntry struct {
	DatabaseID   int64  `json:"databaseId"`
	WorkflowName string `json:"workflowName"`
	Conclusion   string `json:"conclusion"`
	Status       string `json:"status"`
	HeadBranch   string `json:"headBranch"`
	Event        string `json:"event"`
	DisplayTitle string `json:"displayTitle"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	StartedAt    string `json:"startedAt"`
	URL          string `json:"url"`
}

const runListFields = "databaseId,workflowName,conclusion,status,headBranch,event,displayTitle,createdAt,updatedAt,startedAt,url"

// readWorkflows holt die letzten Läufe auf dem Default-Branch und behält je
// Workflow den jüngsten. `gh run list` gibt sie bereits absteigend zurück.
func (o *Overview) readWorkflows(ctx context.Context, c *Client) {
	o.Workflows = []WorkflowState{}
	if o.DefaultBranch == "" {
		return
	}
	raw, err := c.gh(ctx, "run", "list", "--branch", o.DefaultBranch, "--limit", "40", "--json", runListFields)
	if err != nil {
		return
	}
	var entries []runListEntry
	if json.Unmarshal(raw, &entries) != nil {
		return
	}

	seen := map[string]bool{}
	for _, entry := range entries {
		if seen[entry.WorkflowName] {
			continue
		}
		seen[entry.WorkflowName] = true
		o.Workflows = append(o.Workflows, WorkflowState{
			Name:       entry.WorkflowName,
			RunID:      entry.DatabaseID,
			Status:     entry.Status,
			Conclusion: entry.Conclusion,
			Branch:     entry.HeadBranch,
			Event:      entry.Event,
			Title:      entry.DisplayTitle,
			CreatedAt:  entry.CreatedAt,
			URL:        entry.URL,
		})
	}
	o.CI = summarizeCI(o.Workflows)
}

// summarizeCI fasst die Workflows zu einem Wort zusammen: ein einziger roter
// Lauf macht die CI rot, ein noch laufender macht sie gelb.
func summarizeCI(workflows []WorkflowState) string {
	if len(workflows) == 0 {
		return ""
	}
	state := "ok"
	for _, workflow := range workflows {
		switch {
		case workflow.Conclusion == "failure", workflow.Conclusion == "timed_out", workflow.Conclusion == "startup_failure":
			return "error"
		case workflow.Status != "completed", workflow.Conclusion == "cancelled", workflow.Conclusion == "action_required":
			state = "warn"
		}
	}
	return state
}
