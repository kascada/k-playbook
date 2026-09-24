package branches

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/github"
)

// Quellen einer Umgebungszuordnung.
const (
	// SourceConfig: festgelegt in git.environments.
	SourceConfig = "config"
	// SourceDeployment: vorgeschlagen aus dem letzten GitHub-Deployment.
	SourceDeployment = "deployment"
	// SourceName: vorgeschlagen aus dem Branch-Namen.
	SourceName = "name"
)

// Environment ist eine Umgebung mit ihrem Branch.
//
// Welcher Branch eine Umgebung bedient, steht in den seltensten Projekten
// verlässlich an einer Stelle: in omni-gw nur in CI-Ausdrücken, und dort
// widersprüchlich. Maßgeblich ist deshalb die Festlegung in git.environments.
// Ohne sie gibt es nur einen Vorschlag — mit gh aus dem letzten Deployment, sonst
// aus dem Namen —, und der ist als Vorschlag gekennzeichnet.
type Environment struct {
	Name string `json:"name"`
	// Branch ist die wirksame Zuordnung: festgelegt, sonst der Vorschlag.
	Branch    string `json:"branch"`
	Source    string `json:"source"`
	Suggested bool   `json:"suggested"`

	// Configured ist der Branch aus git.environments, leer ohne Festlegung.
	Configured string `json:"configured"`
	// NameSuggestion ist der Branch, den der Name nahelegt.
	NameSuggestion string `json:"nameSuggestion"`
	// DeploymentSuggestion ist der Branch, aus dem das letzte Deployment
	// stammt, soweit das bestimmbar ist.
	DeploymentSuggestion string `json:"deploymentSuggestion"`

	// OnGitHub meldet ein GitHub-Environment dieses Namens.
	OnGitHub   bool             `json:"onGitHub"`
	Deployment *DeploymentState `json:"deployment,omitempty"`
	Findings   []Finding        `json:"findings"`
	Message    string           `json:"message,omitempty"`
}

// DeploymentState ist das letzte Deployment einer Umgebung, gegen das Repo
// aufgelöst.
type DeploymentState struct {
	github.Deployment
	// Resolved meldet, ob der Commit lokal vorhanden ist.
	Resolved bool   `json:"resolved"`
	Subject  string `json:"subject,omitempty"`
	Date     string `json:"date,omitempty"`
	// InBranch ist contained, not-contained oder unknown — bezogen auf den
	// wirksamen Branch der Umgebung.
	InBranch string `json:"inBranch"`
	// Ahead ist die Zahl der Commits, die der Branch über das Deployment
	// hinaus trägt.
	Ahead   int    `json:"ahead"`
	Message string `json:"message,omitempty"`
}

// Finding ist ein Befund zu einer Umgebung: eine Abweichung, kein Fehler.
type Finding struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// environmentGroup ordnet Umgebungs- und Branch-Namen einander zu. Die
// Reihenfolge der Branch-Namen entscheidet, welcher gewinnt, wenn mehrere da
// sind.
type environmentGroup struct {
	key      string
	envNames []string
	branches []string
}

var environmentGroups = []environmentGroup{
	{key: "dev", envNames: []string{"dev", "develop", "development"}, branches: []string{"dev", "develop", "development"}},
	{key: "stage", envNames: []string{"stage", "staging"}, branches: []string{"stage", "staging"}},
	{key: "prod", envNames: []string{"prod", "production"}, branches: []string{"prod", "production", "main", "master"}},
}

func groupOfEnvironment(name string) (environmentGroup, bool) {
	lower := strings.ToLower(name)
	for _, group := range environmentGroups {
		for _, candidate := range group.envNames {
			if candidate == lower {
				return group, true
			}
		}
	}
	return environmentGroup{}, false
}

// nameSuggestion ist der Branch, den der Name der Umgebung nahelegt. Unter
// main und master gewinnt der Default-Branch.
func (st *state) nameSuggestion(environment string) string {
	group, ok := groupOfEnvironment(environment)
	if !ok {
		return ""
	}
	found := []string{}
	for _, candidate := range group.branches {
		for _, name := range st.branchNames() {
			if strings.EqualFold(name, candidate) {
				found = append(found, name)
			}
		}
	}
	for _, name := range found {
		if name == st.def.Name {
			return name
		}
	}
	if len(found) > 0 {
		return found[0]
	}
	return ""
}

func (st *state) branchNames() []string {
	names := make([]string, 0, len(st.locals)+len(st.remoteRef))
	for name := range st.locals {
		names = append(names, name)
	}
	for name := range st.remoteRef {
		if _, local := st.locals[name]; !local {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// environments bestimmt die Umgebungen: festgelegte in Dateireihenfolge, dann
// die GitHub-Environments, dann die Namensgruppen, für die ein Branch da ist.
func (st *state) environments(ctx context.Context) []Environment {
	gh := st.options.GitHub
	list := []Environment{}
	index := map[string]int{}
	add := func(name string) *Environment {
		key := strings.ToLower(name)
		if position, ok := index[key]; ok {
			return &list[position]
		}
		index[key] = len(list)
		list = append(list, Environment{Name: name, Findings: []Finding{}})
		return &list[len(list)-1]
	}

	for _, configured := range st.options.Settings.Environments {
		add(configured.Name).Configured = configured.Branch
	}
	// Gezählt wird, was tatsächlich angelegt ist, nicht die Zahl der
	// Festlegungen: zwei Schreibweisen desselben Namens ergeben einen Eintrag.
	// Der Parser weist solche Dubletten ab; eine zu große Zahl schnitte unten
	// über das Ende der Liste hinaus.
	configured := len(list)
	if gh.Enabled {
		for _, name := range gh.Environments {
			add(name).OnGitHub = true
		}
	}
	for _, group := range environmentGroups {
		covered := false
		for _, env := range list {
			if g, ok := groupOfEnvironment(env.Name); ok && g.key == group.key {
				covered = true
				break
			}
		}
		if !covered && st.nameSuggestion(group.key) != "" {
			add(group.key)
		}
	}

	// Festgelegte Umgebungen behalten die Reihenfolge der Datei; alle übrigen
	// folgen als dev, stage, prod und danach alphabetisch.
	rest := list[configured:]
	sort.SliceStable(rest, func(i, j int) bool { return canonicalRank(rest[i].Name) < canonicalRank(rest[j].Name) })

	deployments := map[string]github.Deployment{}
	if gh.Enabled {
		for _, deployment := range gh.Deployments {
			deployments[strings.ToLower(deployment.Environment)] = deployment
		}
	}

	for position := range list {
		env := &list[position]
		env.NameSuggestion = st.nameSuggestion(env.Name)
		deployment, hasDeployment := deployments[strings.ToLower(env.Name)]
		if hasDeployment {
			env.DeploymentSuggestion = st.deploymentBranch(ctx, deployment, env.NameSuggestion)
		}

		switch {
		case env.Configured != "":
			env.Branch, env.Source = env.Configured, SourceConfig
			if !st.exists(env.Configured) {
				env.Findings = append(env.Findings, Finding{Kind: "missing-branch", Message: "Der festgelegte Branch " + env.Configured + " ist weder lokal noch als Remote-Tracking-Branch vorhanden."})
			}
		case env.DeploymentSuggestion != "":
			env.Branch, env.Source, env.Suggested = env.DeploymentSuggestion, SourceDeployment, true
		case env.NameSuggestion != "":
			env.Branch, env.Source, env.Suggested = env.NameSuggestion, SourceName, true
		default:
			env.Message = "Keine Zuordnung: nichts festgelegt, und weder Name noch Deployment legen einen Branch nahe. Festlegen lässt sie sich in git.environments."
		}

		if hasDeployment {
			env.Deployment = st.resolveDeployment(ctx, deployment, env.Branch)
		}
		st.addFindings(env)
	}
	return list
}

// canonicalRank sortiert dev, stage, prod nach vorn, den Rest alphabetisch
// dahinter.
func canonicalRank(name string) string {
	if group, ok := groupOfEnvironment(name); ok {
		for position, candidate := range environmentGroups {
			if candidate.key == group.key {
				return strconv.Itoa(position) + name
			}
		}
	}
	return "9" + strings.ToLower(name)
}

// deploymentBranch bestimmt, aus welchem Branch ein Deployment stammt.
//
// Nennt Ref einen vorhandenen Branch, ist es dieser. Sonst wird nicht das
// Namensmuster gedeutet — in omni-gw ist Ref ein Tag „<kurz-sha>-<branch>", in
// anderen Projekten etwas anderes —, sondern der SHA: unter den langlebigen
// Kandidaten gewinnt der, der ihn enthält und am wenigsten darüber hinaus
// trägt. Ein Arbeitsbranch zählt nicht mit: frisch abgezweigt enthielte er den
// Stand ebenso.
//
// Enthält der Branch, den der Name der Umgebung nahelegt, den Stand, gewinnt er
// vor dem geringsten Abstand. Ein älteres Deployment steckt meist in mehreren
// langlebigen Branches, und der mit dem geringsten Abstand ist dann nur der, in
// den zuletzt gemergt wurde: in omni-gw lag der stage-Stand e904713 in stage
// (227 Commits dahinter) und in master (14) — master wäre ein falscher Befund.
func (st *state) deploymentBranch(ctx context.Context, deployment github.Deployment, preferred string) string {
	if deployment.Ref != "" && st.exists(deployment.Ref) {
		return deployment.Ref
	}
	if deployment.SHA == "" || !validSHA(deployment.SHA) {
		return ""
	}
	if preferred != "" {
		if ref := st.preferredRef(preferred); ref != "" {
			if _, err := st.repo.read(ctx, "merge-base", "--is-ancestor", deployment.SHA, ref); err == nil {
				return preferred
			}
		}
	}
	best, bestCount := "", -1
	for _, candidate := range st.longLivedCandidates() {
		ref := st.preferredRef(candidate)
		if ref == "" {
			continue
		}
		if _, err := st.repo.read(ctx, "merge-base", "--is-ancestor", deployment.SHA, ref); err != nil {
			continue
		}
		out, err := st.repo.readString(ctx, "rev-list", "--count", deployment.SHA+".."+ref)
		if err != nil {
			continue
		}
		count, err := strconv.Atoi(strings.TrimSpace(out))
		if err != nil {
			continue
		}
		if bestCount < 0 || count < bestCount {
			best, bestCount = candidate, count
		}
	}
	return best
}

// longLivedCandidates sind die Branches, die als Quelle eines Deployments in
// Frage kommen: Default-Branch, festgelegte und nach Namen passende.
func (st *state) longLivedCandidates() []string {
	seen := map[string]bool{}
	candidates := []string{}
	add := func(name string) {
		if name != "" && !seen[name] && st.exists(name) {
			seen[name] = true
			candidates = append(candidates, name)
		}
	}
	add(st.def.Name)
	for _, env := range st.options.Settings.Environments {
		add(env.Branch)
	}
	for _, group := range environmentGroups {
		for _, candidate := range group.branches {
			for _, name := range st.branchNames() {
				if strings.EqualFold(name, candidate) {
					add(name)
				}
			}
		}
	}
	return candidates
}

func validSHA(sha string) bool {
	if len(sha) < 7 || len(sha) > 64 {
		return false
	}
	for _, r := range sha {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}

// resolveDeployment löst den SHA eines Deployments im Repo auf und misst ihn
// gegen den wirksamen Branch der Umgebung.
func (st *state) resolveDeployment(ctx context.Context, deployment github.Deployment, branch string) *DeploymentState {
	result := &DeploymentState{Deployment: deployment, InBranch: FieldUnknown}
	if !validSHA(deployment.SHA) {
		result.Message = "Das Deployment nennt keinen lesbaren SHA."
		return result
	}
	out, err := st.repo.read(ctx, "log", "-1", "--format=%s%x00%cI", deployment.SHA+"^{commit}", "--")
	if err != nil {
		result.Message = "Der Commit " + shortSHA(deployment.SHA) + " ist lokal nicht vorhanden; ein Fetch holt ihn."
		return result
	}
	result.Resolved = true
	subject, date, _ := strings.Cut(strings.TrimRight(string(out), "\n"), "\x00")
	result.Subject, result.Date = subject, date

	if branch == "" {
		result.Message = "Ohne zugeordneten Branch lässt sich der Stand mit keinem Branch vergleichen."
		return result
	}
	ref := st.preferredRef(branch)
	if ref == "" {
		result.Message = "Der Branch " + branch + " ist nicht vorhanden."
		return result
	}
	if _, err := st.repo.read(ctx, "merge-base", "--is-ancestor", deployment.SHA, ref); err != nil {
		if exitCode(err) == 1 {
			result.InBranch = "not-contained"
			result.Message = "Der ausgerollte Stand ist nicht in " + branch + " enthalten."
			return result
		}
		result.Message = describeError("Der Vergleich mit "+branch, err)
		return result
	}
	count, err := st.repo.readString(ctx, "rev-list", "--count", deployment.SHA+".."+ref)
	if err != nil {
		result.Message = describeError("Die Zahl der Commits seit dem Deployment", err)
		return result
	}
	result.Ahead, _ = strconv.Atoi(strings.TrimSpace(count))
	result.InBranch = "contained"
	if result.Ahead == 0 {
		result.Message = branch + " steht auf dem ausgerollten Stand."
	} else {
		result.Message = branch + " ist " + commits(result.Ahead) + " weiter als das Deployment."
	}
	return result
}

// addFindings hält Abweichungen fest. Sie sind Befunde, keine Fehler: in omni-gw
// läuft ein Stand von stage in prod, und das ist dort so gewollt oder nicht —
// entscheiden kann das die Seite nicht.
func (st *state) addFindings(env *Environment) {
	if env.DeploymentSuggestion != "" && env.Configured != "" && env.DeploymentSuggestion != env.Configured {
		env.Findings = append(env.Findings, Finding{
			Kind:    "deployment-differs",
			Message: "Festgelegt ist " + env.Configured + ", das letzte Deployment nach " + env.Name + " stammt aber aus " + env.DeploymentSuggestion + ".",
		})
	}
	if env.Configured == "" && env.DeploymentSuggestion != "" && env.NameSuggestion != "" && env.DeploymentSuggestion != env.NameSuggestion {
		env.Findings = append(env.Findings, Finding{
			Kind:    "name-differs",
			Message: "Der Name legt " + env.NameSuggestion + " nahe, das letzte Deployment nach " + env.Name + " stammt aber aus " + env.DeploymentSuggestion + ".",
		})
	}
	if env.Deployment != nil && env.Deployment.InBranch == "not-contained" && env.Configured != "" {
		env.Findings = append(env.Findings, Finding{
			Kind:    "deployment-not-in-branch",
			Message: "Der ausgerollte Stand " + shortSHA(env.Deployment.SHA) + " ist nicht im festgelegten Branch " + env.Configured + " enthalten.",
		})
	}
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// commits nennt eine Zahl von Commits mit passender Endung.
func commits(count int) string {
	if count == 1 {
		return "1 Commit"
	}
	return strconv.Itoa(count) + " Commits"
}
