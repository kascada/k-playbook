package github

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// ClosedPullRequestLimit ist die Zahl der zuletzt geschlossenen und gemergten
// PRs, die mitkommen. Fest und nicht einstellbar: die Ansicht beantwortet
// Fragen („wohin ging das zuletzt?"), sie baut GitHubs Liste nicht nach. Wer
// weiter zurück will, geht auf github.com.
const ClosedPullRequestLimit = 20

// openPullRequestLimit deckelt die offenen. Ein Repo mit mehr als 100 offenen
// PRs wäre auf dieser Seite ohnehin nicht mehr lesbar.
const openPullRequestLimit = 100

// PullRequest ist ein PR, so wie ihn die Karte zeigt.
type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Author string `json:"author"`
	// State ist open, draft, merged oder closed. Entwurf ist ein eigener
	// Zustand und kein Zusatz zu „offen": er sagt, dass noch niemand
	// hinsehen soll.
	State string `json:"state"`
	URL   string `json:"url"`
	// Head und Base sind Quell- und Ziel-Branch. Beide stehen immer da, auch
	// wenn das Ziel der Default-Branch ist: „wohin geht dieser PR" ist die
	// Frage, die die Liste beantwortet.
	Head string `json:"head"`
	Base string `json:"base"`
	// Fork meldet eine Quelle außerhalb des Repos, ForkOwner nennt sie.
	Fork      bool   `json:"fork"`
	ForkOwner string `json:"forkOwner"`
	// NonDefaultBase meldet ein Ziel außerhalb des Default-Branchs. Das ist
	// keine Störung, aber es fällt leicht durch — deshalb eine eigene Marke.
	NonDefaultBase bool `json:"nonDefaultBase"`
	// Dependabot meldet einen PR des Bots. Er wird anders gelesen als einer
	// von Hand.
	Dependabot     bool     `json:"dependabot"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
	MergedAt       string   `json:"mergedAt"`
	Additions      int      `json:"additions"`
	Deletions      int      `json:"deletions"`
	ChangedFiles   int      `json:"changedFiles"`
	ReviewDecision string   `json:"reviewDecision"`
	Checks         string   `json:"checks"`
	Mergeable      string   `json:"mergeable"`
	Labels         []string `json:"labels"`
	// Command ist der Einstieg zum Kopieren. Die Ansicht schreibt nichts nach
	// GitHub; Approve und Merge laufen über /k-pr-review.
	Command string `json:"command"`
}

// Pulls ist die Antwort der PR-Karte: offene oben, zuletzt geschlossene
// darunter, getrennt in der Antwort statt in einem Feld „state" vermischt.
type Pulls struct {
	Result
	DefaultBranch string        `json:"defaultBranch"`
	Open          []PullRequest `json:"open"`
	Closed        []PullRequest `json:"closed"`
	// ClosedLimit sagt, wie weit „zuletzt geschlossen" reicht. Ohne die Zahl
	// sähe eine gedeckelte Liste aus wie eine vollständige.
	ClosedLimit int `json:"closedLimit"`
}

// pullsQuery holt offene und geschlossene PRs in **einer** Abfrage. Eine
// GraphQL-Abfrage statt vieler `gh pr view`: jeder PR brauchte sonst einen
// eigenen Netzaufruf, und die Karte wartete auf den langsamsten.
const pullsQuery = `query($owner: String!, $name: String!, $open: Int!, $closed: Int!) {
  repository(owner: $owner, name: $name) {
    defaultBranchRef { name }
    open: pullRequests(states: OPEN, first: $open, orderBy: {field: UPDATED_AT, direction: DESC}) {
      nodes { ...pullRequestFields }
    }
    closed: pullRequests(states: [CLOSED, MERGED], first: $closed, orderBy: {field: UPDATED_AT, direction: DESC}) {
      nodes { ...pullRequestFields }
    }
  }
}
fragment pullRequestFields on PullRequest {
  number title url state isDraft isCrossRepository
  createdAt updatedAt mergedAt
  additions deletions changedFiles mergeable reviewDecision
  headRefName baseRefName
  author { login }
  headRepositoryOwner { login }
  labels(first: 20) { nodes { name } }
  commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
}`

type pullsPayload struct {
	Errors []graphQLError `json:"errors"`
	Data   struct {
		Repository struct {
			DefaultBranchRef struct {
				Name string `json:"name"`
			} `json:"defaultBranchRef"`
			Open   pullNodes `json:"open"`
			Closed pullNodes `json:"closed"`
		} `json:"repository"`
	} `json:"data"`
}

type pullNodes struct {
	Nodes []pullNode `json:"nodes"`
}

type pullNode struct {
	Number            int    `json:"number"`
	Title             string `json:"title"`
	URL               string `json:"url"`
	State             string `json:"state"`
	IsDraft           bool   `json:"isDraft"`
	IsCrossRepository bool   `json:"isCrossRepository"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
	MergedAt          string `json:"mergedAt"`
	Additions         int    `json:"additions"`
	Deletions         int    `json:"deletions"`
	ChangedFiles      int    `json:"changedFiles"`
	Mergeable         string `json:"mergeable"`
	ReviewDecision    string `json:"reviewDecision"`
	HeadRefName       string `json:"headRefName"`
	BaseRefName       string `json:"baseRefName"`
	Author            *struct {
		Login string `json:"login"`
	} `json:"author"`
	HeadRepositoryOwner *struct {
		Login string `json:"login"`
	} `json:"headRepositoryOwner"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					State string `json:"state"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

// FetchPulls liest die PR-Liste. repo ist `owner/name`, wie es die Kopfzeile
// aufgelöst hat — die Ansicht parst die Remote-URL nicht selbst.
func (c *Client) FetchPulls(ctx context.Context, repo string) Pulls {
	owner, name, ok := splitRepo(repo)
	if !ok {
		return Pulls{Result: Fail(StateNoRemote), Open: []PullRequest{}, Closed: []PullRequest{}, ClosedLimit: ClosedPullRequestLimit}
	}

	raw, err := c.gh(ctx, "api", "graphql",
		"-f", "query="+pullsQuery,
		"-F", "owner="+owner,
		"-F", "name="+name,
		"-F", "open="+strconv.Itoa(openPullRequestLimit),
		"-F", "closed="+strconv.Itoa(ClosedPullRequestLimit),
	)
	if err != nil {
		// Auch ein Teilfehler — ein einzelnes Feld FORBIDDEN, der Rest mit
		// Daten — endet bei gh mit Exit 1. Die Daten werden bewusst nicht
		// gezeigt: ein verweigertes Feld sähe in der Karte aus wie ein leeres
		// („keine Checks", „kein Review"), und das wäre eine Behauptung, die
		// die Seite nicht belegen kann.
		result := classifyGraphQL(raw, err)
		return Pulls{Result: result, Open: []PullRequest{}, Closed: []PullRequest{}, ClosedLimit: ClosedPullRequestLimit}
	}
	return ParsePulls(raw)
}

// graphQLError ist ein Eintrag im Feld `errors` einer GraphQL-Antwort.
type graphQLError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// classifyGraphQL ordnet einen gescheiterten `gh api graphql` ein.
//
// Der Typ des Fehlers steht nur im Rumpf auf stdout: auf stderr schreibt gh
// allein die Meldung (`gh: <message>`), den Typ liest es gar nicht. Deshalb
// zuerst der Rumpf — der Typ hängt nicht am Wortlaut, und vom Rate-Limit sind
// mindestens zwei Wortlaute bekannt. Was der Typ nicht eindeutig sagt, geht an
// Classify: FORBIDDEN deckt vom fehlenden Recht am Repo bis zur SAML-Freigabe
// Verschiedenes ab, und dort sagt die Meldung selbst mehr als ein fester Satz.
func classifyGraphQL(raw []byte, err error) Result {
	// Frist und Abbruch stehen am Kontext und gehen jedem Rumpf vor.
	if contextEnded(err) {
		return Classify(err)
	}
	var body struct {
		Errors []graphQLError `json:"errors"`
	}
	if json.Unmarshal(raw, &body) == nil {
		for _, entry := range body.Errors {
			switch entry.Type {
			case "RATE_LIMITED":
				return Fail(StateRateLimited)
			case "NOT_FOUND":
				return Fail(StateNoAccess)
			}
		}
	}
	return Classify(err)
}

// ParsePulls wertet die GraphQL-Antwort aus. Eigene Funktion, damit die Tests
// sie mit Fixture-JSON füttern können.
func ParsePulls(raw []byte) Pulls {
	pulls := Pulls{
		Result:      Result{State: StateOK},
		Open:        []PullRequest{},
		Closed:      []PullRequest{},
		ClosedLimit: ClosedPullRequestLimit,
	}

	var payload pullsPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Pulls{
			Result:      Result{State: StateError, Message: Explain(StateError) + " Die Antwort von gh war kein lesbares JSON."},
			Open:        []PullRequest{},
			Closed:      []PullRequest{},
			ClosedLimit: ClosedPullRequestLimit,
		}
	}

	// Absicherung: gh endet bei einem Feld `errors` mit Exit 1 und kommt gar
	// nicht hierher. Kommt eine solche Antwort doch an, wird sie nicht zu
	// „keine Pull Requests".
	if len(payload.Errors) > 0 {
		messages := make([]string, 0, len(payload.Errors))
		for _, entry := range payload.Errors {
			messages = append(messages, entry.Message)
		}
		return Pulls{
			Result:      classifyGraphQL(raw, errors.New(strings.Join(messages, "\n"))),
			Open:        []PullRequest{},
			Closed:      []PullRequest{},
			ClosedLimit: ClosedPullRequestLimit,
		}
	}

	repository := payload.Data.Repository
	pulls.DefaultBranch = repository.DefaultBranchRef.Name
	for _, node := range repository.Open.Nodes {
		pulls.Open = append(pulls.Open, convertPull(node, pulls.DefaultBranch))
	}
	for _, node := range repository.Closed.Nodes {
		pulls.Closed = append(pulls.Closed, convertPull(node, pulls.DefaultBranch))
	}
	return pulls
}

func convertPull(node pullNode, defaultBranch string) PullRequest {
	pull := PullRequest{
		Number:         node.Number,
		Title:          node.Title,
		URL:            node.URL,
		State:          pullState(node),
		Head:           node.HeadRefName,
		Base:           node.BaseRefName,
		Fork:           node.IsCrossRepository,
		CreatedAt:      node.CreatedAt,
		UpdatedAt:      node.UpdatedAt,
		MergedAt:       node.MergedAt,
		Additions:      node.Additions,
		Deletions:      node.Deletions,
		ChangedFiles:   node.ChangedFiles,
		ReviewDecision: node.ReviewDecision,
		Mergeable:      node.Mergeable,
		Labels:         []string{},
		Command:        "/k-pr-review " + strconv.Itoa(node.Number),
	}
	if node.Author != nil {
		pull.Author = node.Author.Login
	}
	if node.HeadRepositoryOwner != nil && node.IsCrossRepository {
		pull.ForkOwner = node.HeadRepositoryOwner.Login
	}
	// Ohne bekannten Default-Branch wird nichts behauptet: eine Marke „Ziel ist
	// nicht der Default-Branch" wäre dann geraten.
	pull.NonDefaultBase = defaultBranch != "" && node.BaseRefName != "" && node.BaseRefName != defaultBranch
	pull.Dependabot = isDependabot(pull.Author)
	for _, label := range node.Labels.Nodes {
		pull.Labels = append(pull.Labels, label.Name)
	}
	if len(node.Commits.Nodes) > 0 {
		if rollup := node.Commits.Nodes[0].Commit.StatusCheckRollup; rollup != nil {
			pull.Checks = rollup.State
		}
	}
	return pull
}

// pullState macht aus GraphQL-Zustand und isDraft ein Wort. Ein Entwurf ist
// zwar OPEN, aber er will nicht gelesen werden — das ist ein eigener Zustand.
func pullState(node pullNode) string {
	switch strings.ToUpper(node.State) {
	case "MERGED":
		return "merged"
	case "CLOSED":
		return "closed"
	default:
		if node.IsDraft {
			return "draft"
		}
		return "open"
	}
}

// isDependabot erkennt den Bot an seinem Anmeldenamen. GitHub schreibt ihn je
// nach Feld unterschiedlich.
func isDependabot(author string) bool {
	name := strings.ToLower(strings.TrimSpace(author))
	name = strings.TrimPrefix(name, "app/")
	return name == "dependabot" || name == "dependabot[bot]" || name == "dependabot-preview[bot]"
}

// splitRepo zerlegt `owner/name`.
func splitRepo(repo string) (string, string, bool) {
	owner, name, found := strings.Cut(strings.TrimSpace(repo), "/")
	if !found || owner == "" || name == "" {
		return "", "", false
	}
	return owner, name, true
}

// repoPattern begrenzt, was als `owner/name` in einen gh-Aufruf gehen darf.
// Der Wert kommt aus einem Abfrageparameter der Seite; ungeprüft stünde dort
// beliebiger Text in einem Subprozess-Argument.
var repoPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}/[A-Za-z0-9._-]{1,100}$`)

// ValidRepo meldet, ob der Wert die Form `owner/name` hat.
func ValidRepo(repo string) bool {
	return repoPattern.MatchString(strings.TrimSpace(repo))
}

// Repo löst `owner/name` über gh auf. Die Ansicht parst die Remote-URL nicht
// selbst: das Remote kann ein SSH-Alias sein, den allein die SSH-Konfiguration
// auflöst — gh kennt ihn, ein eigener Parser nicht.
func (c *Client) Repo(ctx context.Context) (string, Result) {
	raw, err := c.gh(ctx, "repo", "view", "--json", "nameWithOwner")
	if err != nil {
		return "", Classify(err)
	}
	var view repoView
	if json.Unmarshal(raw, &view) != nil || view.NameWithOwner == "" {
		return "", Fail(StateNoRemote)
	}
	return view.NameWithOwner, Result{State: StateOK}
}
