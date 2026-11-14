package project

import (
	"context"
)

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
