package project

import (
	"context"
	"strings"
)

// PublicRepoURL ist die anonyme Leseadresse der Installation. Das Repo ist
// öffentlich: über HTTPS braucht der Pull weder Schlüssel noch Token.
const PublicRepoURL = "https://github.com/kascada/k-playbook.git"

// publicRepoSSHForms sind die SSH-Adressen, unter denen die Anleitung das Repo
// früher klonen ließ. Nur sie werden umgestellt: ein Alias-Host aus
// ~/.ssh/config (git@github-<name>:…) oder ein Fork ist eine bewusste Wahl.
var publicRepoSSHForms = []string{
	"git@github.com:kascada/k-playbook",
	"ssh://git@github.com/kascada/k-playbook",
}

// publicFetchURL liefert PublicRepoURL, wenn url eine der SSH-Formen des
// offiziellen Repos auf github.com ist.
func publicFetchURL(url string) (string, bool) {
	trimmed := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(url), "/"), ".git")
	for _, form := range publicRepoSSHForms {
		if strings.EqualFold(trimmed, form) {
			return PublicRepoURL, true
		}
	}
	return "", false
}

// upstreamRemote nennt den Remote, dem der aktuelle Branch folgt; leer ohne
// Branch oder Upstream.
func upstreamRemote(ctx context.Context, dir string) string {
	branch, err := GitOutput(ctx, dir, "branch", "--show-current")
	if err != nil || branch == "" {
		return ""
	}
	remoteName, err := GitOutput(ctx, dir, "config", "--get", "branch."+branch+".remote")
	if err != nil {
		return ""
	}
	return remoteName
}

// remoteRawURL liest die eingetragene Adresse von remoteName, den rohen
// Eintrag remote.<name>.url — nicht `git remote get-url`: das wendet
// insteadOf an. Leer, wenn es den Eintrag nicht gibt.
func remoteRawURL(ctx context.Context, dir string, remoteName string) string {
	url, err := GitOutput(ctx, dir, "config", "--get", "remote."+remoteName+".url")
	if err != nil {
		return ""
	}
	return url
}

// remotePublicURL liefert die HTTPS-Adresse für remoteName, wenn dessen
// eingetragene Adresse (remoteRawURL) eine SSH-Form des offiziellen Repos ist.
func remotePublicURL(ctx context.Context, dir string, remoteName string) (string, bool) {
	url := remoteRawURL(ctx, dir, remoteName)
	if url == "" {
		return "", false
	}
	return publicFetchURL(url)
}

// projectRemoteURL ist der rohe Eintrag des Remotes, das für die Zuordnung
// eines Arbeitsstands zu einem Repo zählt: das Upstream-Remote des aktuellen
// Branches (upstreamRemote), sonst origin — auch dann, wenn der Upstream
// keine Adresse trägt, etwa „.“ für einen lokalen Branch. Leer ohne Repo
// oder ohne ein solches Remote.
func projectRemoteURL(ctx context.Context, dir string) string {
	if remoteName := upstreamRemote(ctx, dir); remoteName != "" {
		if url := remoteRawURL(ctx, dir, remoteName); url != "" {
			return url
		}
	}
	return remoteRawURL(ctx, dir, "origin")
}

// usePublicRemote stellt den Upstream des Clones bleibend auf PublicRepoURL
// um, wenn er eine SSH-Form des offiziellen Repos trägt. Sonst verlangt jeder
// Pull einen SSH-Schlüssel, obwohl das Repo öffentlich ist. Liefert die neue
// Adresse, leer, wenn nichts umzustellen war. Die Installation muss dafür
// beschreibbar sein.
func usePublicRemote(ctx context.Context, dir string) (string, error) {
	remoteName := upstreamRemote(ctx, dir)
	if remoteName == "" {
		return "", nil
	}
	public, ok := remotePublicURL(ctx, dir, remoteName)
	if !ok {
		return "", nil
	}
	if _, err := GitOutput(ctx, dir, "remote", "set-url", remoteName, public); err != nil {
		return "", err
	}
	return public, nil
}
