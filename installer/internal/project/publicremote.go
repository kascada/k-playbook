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

// remotePublicURL liefert die HTTPS-Adresse für remoteName, wenn dessen
// eingetragene Adresse eine SSH-Form des offiziellen Repos ist. Gelesen wird
// der rohe Eintrag, nicht `git remote get-url`: das wendet insteadOf an.
func remotePublicURL(ctx context.Context, dir string, remoteName string) (string, bool) {
	url, err := GitOutput(ctx, dir, "config", "--get", "remote."+remoteName+".url")
	if err != nil {
		return "", false
	}
	return publicFetchURL(url)
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
