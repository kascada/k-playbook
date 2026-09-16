package github

import (
	"context"
	"encoding/json"
	"errors"
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
	// Notes sagt je Feld, ob es gelesen wurde — und wenn nicht, warum. Ein
	// leeres Feld allein sähe bei einem Fehler aus wie „nichts da".
	Notes OverviewNotes `json:"notes"`
}

// FieldState unterscheidet, warum ein Feld der Kopfzeile leer ist.
type FieldState string

const (
	// FieldOK: das Feld ist gelesen.
	FieldOK FieldState = "ok"
	// FieldNone: gelesen, und es gibt nichts — kein Tag, kein Remote, keine
	// Läufe. Ein gültiger Leerzustand.
	FieldNone FieldState = "none"
	// FieldUnreadable: die Abfrage ist gescheitert. Über das Feld ist nichts
	// bekannt, auch nicht, dass es leer wäre.
	FieldUnreadable FieldState = "unreadable"
	// FieldTimeout: die Frist ist abgelaufen, bevor das Feld an der Reihe war
	// oder während es gelesen wurde. Die Felder davor können gelesen sein.
	FieldTimeout FieldState = "timeout"
)

// FieldNote ist der Hinweis zu einem Feld. Message ist bei FieldOK leer.
type FieldNote struct {
	State   FieldState `json:"state"`
	Message string     `json:"message"`
}

// OverviewNotes sind die Hinweise der Felder, die nach dem ersten Aufruf
// einzeln gelesen werden. Repo, Default-Branch und Recht brauchen keinen: sie
// kommen aus dem ersten Aufruf, und scheitert der, ist die ganze Kopfzeile ein
// Zustand.
type OverviewNotes struct {
	Account   FieldNote `json:"account"`
	Remote    FieldNote `json:"remote"`
	Tag       FieldNote `json:"tag"`
	Workflows FieldNote `json:"workflows"`
}

// fieldFailure macht aus einem gescheiterten Aufruf den Hinweis zu einem Feld.
// Die Frist wird am Kontext erkannt wie in Classify; alles andere ist „nicht
// lesbar" samt dem Satz, den Classify dazu hat.
func fieldFailure(subject string, err error) FieldNote {
	result := Classify(err)
	if result.State == StateTimeout {
		return FieldNote{State: FieldTimeout, Message: subject + " nicht gelesen: die Frist der Anfrage war abgelaufen."}
	}
	return FieldNote{State: FieldUnreadable, Message: subject + " nicht lesbar. " + result.Message}
}

// contextEnded meldet, ob ein Aufruf an Frist oder Abbruch gescheitert ist.
// Dann sagt sein Fehlertext nichts mehr darüber, ob es das Feld gibt.
func contextEnded(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// unreadableJSON ist der Hinweis zu einer Antwort, die kein lesbares JSON war.
func unreadableJSON(subject string) FieldNote {
	return FieldNote{State: FieldUnreadable, Message: subject + " nicht lesbar. Die Antwort von gh war kein lesbares JSON."}
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

// failedOverview ist die Kopfzeile ohne Daten. Sie hält die Zusage des Pakets,
// dass eine Liste leer ist und nicht null: die Oberfläche soll über einen
// Fehlerpfad nicht anders stolpern als über einen leeren Erfolg.
func failedOverview(result Result) Overview {
	return Overview{Result: result, Workflows: []WorkflowState{}}
}

// FetchOverview liest die Kopfzeile. Drei gh-Aufrufe und vier git-Aufrufe; die
// git-Aufrufe kosten kein Netz.
//
// Scheitert der erste Aufruf, ist die Antwort der eingeordnete Zustand und
// sonst nichts: ohne Repo gibt es keine Kopfzeile. Scheitern spätere Aufrufe,
// bleibt der Zustand ok, das einzelne Feld leer, und sein Hinweis in Notes sagt
// warum — ein fehlender Tag macht die Repo-Angabe nicht falsch, ein nicht
// lesbarer Tag ist aber auch kein fehlender. Alle Aufrufe teilen sich den
// Kontext und damit das Budget der Anfrage: ist es verbraucht, melden die
// übrigen Felder FieldTimeout.
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
		if json.Unmarshal(raw, &user) == nil && user.Login != "" {
			overview.Account = user.Login
			overview.Notes.Account = FieldNote{State: FieldOK}
		} else {
			overview.Notes.Account = unreadableJSON("Das gh-Konto")
		}
	} else if result := Classify(err); result.State == StateBadCredentials {
		// `gh repo view` kann aus dem Cache antworten, `gh api user` nicht.
		// Ein abgewiesener Token gehört deshalb hier noch gemeldet.
		return failedOverview(result)
	} else {
		overview.Notes.Account = fieldFailure("Das gh-Konto", err)
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
		// „error: No such remote 'origin'" ist die Antwort, dass es keins gibt
		// (gemessen an git 2.53); jeder andere Fehler sagt darüber nichts.
		if !contextEnded(err) && strings.Contains(strings.ToLower(errorText(err)), "no such remote") {
			o.Notes.Remote = FieldNote{State: FieldNone, Message: "Kein Remote origin eingetragen."}
			return
		}
		o.Notes.Remote = fieldFailure("Das Git-Remote", err)
		return
	}
	o.RemoteURL = strings.TrimSpace(string(raw))
	if o.RemoteURL == "" {
		o.Notes.Remote = FieldNote{State: FieldNone, Message: "Kein Remote origin eingetragen."}
		return
	}
	o.Notes.Remote = FieldNote{State: FieldOK}
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

// noTagNote ist der Leerzustand des Tags: gelesen, und es gibt keinen.
var noTagNote = FieldNote{State: FieldNone, Message: "Kein Tag im Arbeitsstand gefunden."}

// readTag liest den letzten Tag, die Commits seitdem und sein Datum. Ein Hinweis
// für alle drei: sie stehen in der Seite in einer Zeile. Scheitert erst die
// Zählung oder das Datum, bleibt der Tag stehen und der Hinweis sagt, was
// fehlt — „0 Commits seitdem" wäre sonst eine Behauptung.
func (o *Overview) readTag(ctx context.Context, c *Client) {
	raw, err := c.git(ctx, "describe", "--tags", "--abbrev=0")
	if err != nil {
		// „fatal: No names found, cannot describe anything." ist die Antwort,
		// dass es keinen Tag gibt (gemessen an git 2.53). Alles andere — kein
		// Repo, Frist, ein kaputter Arbeitsstand — sagt darüber nichts.
		if !contextEnded(err) && strings.Contains(strings.ToLower(errorText(err)), "no names found") {
			o.Notes.Tag = noTagNote
			return
		}
		o.Notes.Tag = fieldFailure("Der letzte Tag", err)
		return
	}
	o.LastTag = strings.TrimSpace(string(raw))
	if o.LastTag == "" {
		o.Notes.Tag = noTagNote
		return
	}

	raw, err = c.git(ctx, "rev-list", o.LastTag+"..HEAD", "--count")
	if err != nil {
		o.Notes.Tag = fieldFailure("Die Zahl der Commits seit dem Tag", err)
		return
	}
	count, convErr := strconv.Atoi(strings.TrimSpace(string(raw)))
	if convErr != nil {
		o.Notes.Tag = FieldNote{State: FieldUnreadable, Message: "Die Zahl der Commits seit dem Tag nicht lesbar. git hat keine Zahl geliefert."}
		return
	}
	o.CommitsSinceTag = count

	raw, err = c.git(ctx, "log", "-1", "--format=%cI", o.LastTag)
	if err != nil {
		o.Notes.Tag = fieldFailure("Das Datum des Tags", err)
		return
	}
	o.LastTagDate = strings.TrimSpace(string(raw))
	o.Notes.Tag = FieldNote{State: FieldOK}
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
		// Ein Repo ohne Default-Branch hat noch keinen Commit, also auch
		// keinen Lauf darauf.
		o.Notes.Workflows = FieldNote{State: FieldNone, Message: "Das Repo hat noch keinen Default-Branch und damit keine Läufe darauf."}
		return
	}
	raw, err := c.gh(ctx, "run", "list", "--branch", o.DefaultBranch, "--limit", "40", "--json", runListFields)
	if err != nil {
		o.Notes.Workflows = fieldFailure("Die Läufe auf dem Default-Branch", err)
		return
	}
	var entries []runListEntry
	if json.Unmarshal(raw, &entries) != nil {
		o.Notes.Workflows = unreadableJSON("Die Läufe auf dem Default-Branch")
		return
	}
	if len(entries) == 0 {
		o.Notes.Workflows = FieldNote{State: FieldNone, Message: "Noch kein Lauf auf dem Default-Branch."}
		return
	}
	o.Notes.Workflows = FieldNote{State: FieldOK}

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
