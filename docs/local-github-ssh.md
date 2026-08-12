# Lokale GitHub-SSH-Konfiguration

Diese Notiz dokumentiert die lokale SSH-/GitHub-Konfiguration auf dem Kleist-Dev-Host. Sie ist host-spezifisch und kein allgemeiner k-playbook-Installationsvertrag.

## Ausgangslage

`gh` und der normale `github.com`-SSH-Key sind für einen anderen GitHub-Account eingerichtet. Für Repos unter `kascada` deshalb nicht blind `git@github.com:...` verwenden, sondern den passenden SSH-Host-Alias aus `~/.ssh/config`.

## k-playbook

Das Entwicklungs-Repo unter `~/dev/k-playbook` nutzt diesen Remote. Derselbe Alias gilt
für jeden Clone in ein Zielprojekt, denn jedes Projekt klont dasselbe Repo:

```text
git@github-kamranbycloud:kascada/k-playbook.git
```

Der Alias `github-kamranbycloud` zeigt auf:

```text
~/.ssh/id_ed25519_kamranbycloud
```

Dieser Key authentifiziert bei GitHub als Deploy-Key für `kascada/k-playbook`. Er ist repo-spezifisch und kann nicht für weitere Repos wiederverwendet werden.

## KamranApps

Für `kascada/KamranApps` wurde ein separater Deploy-Key angelegt:

```text
~/.ssh/id_ed25519_kamranapps
```

Der passende SSH-Config-Eintrag ist:

```sshconfig
Host github-kamranapps
  HostName github.com
  User git
  IdentityFile ~/.ssh/id_ed25519_kamranapps
  IdentitiesOnly yes
```

Clone-URL:

```bash
git clone git@github-kamranapps:kascada/KamranApps.git ~/dev/KamranApps
```

Wenn der Clone oder `git ls-remote` mit `Repository not found` fehlschlägt, zuerst prüfen, ob der Public Key aus `~/.ssh/id_ed25519_kamranapps.pub` als Deploy-Key im Repo `kascada/KamranApps` hinterlegt ist.
