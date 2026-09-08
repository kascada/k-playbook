# Local GitHub SSH configuration

This note documents the local SSH/GitHub configuration on the Kleist development host. It is host-specific and not a general k-playbook installation contract.

## Starting point

`gh` and the regular `github.com` SSH key are configured for a different GitHub account. Therefore, do not blindly use `git@github.com:...` for repositories under `kascada`; use the appropriate SSH host alias from `~/.ssh/config` instead.

## k-playbook

The development repository at `~/dev/k-playbook` uses this remote. The same alias applies to every clone into a target project because every project clones the same repository:

```text
git@github-kamranbycloud:kascada/k-playbook.git
```

The `github-kamranbycloud` alias points to:

```text
~/.ssh/id_ed25519_kamranbycloud
```

This key authenticates with GitHub as the deploy key for `kascada/k-playbook`. It is repository-specific and cannot be reused for other repositories.

## KamranApps

A separate deploy key was created for `kascada/KamranApps`:

```text
~/.ssh/id_ed25519_kamranapps
```

The corresponding SSH config entry is:

```sshconfig
Host github-kamranapps
  HostName github.com
  User git
  IdentityFile ~/.ssh/id_ed25519_kamranapps
  IdentitiesOnly yes
```

Clone URL:

```bash
git clone git@github-kamranapps:kascada/KamranApps.git ~/dev/KamranApps
```

If the clone or `git ls-remote` fails with `Repository not found`, first check whether the public key from `~/.ssh/id_ed25519_kamranapps.pub` is registered as a deploy key in the `kascada/KamranApps` repository.
