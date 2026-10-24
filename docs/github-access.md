# GitHub access

The contract for the personal GitHub access file: where it lives, what it contains, how
k-playbook reads and checks it, and what happens when it is missing.

This page is the **single** definition. The reader, the subcommand, the check, and the tests
refer to it and do not redefine any of it. What is not stated here is not defined.

**Status (2026-10-24):** the format is agreed; nothing of it is implemented yet. This page
is the contract the first task builds against. Later steps — availability of an access,
running a command under a service, named operations — are listed under *Outlook* and are not
part of the contract until they are specified here.

## What the file is, and is not

A person works with several GitHub repositories, often under several GitHub accounts, and
reaches each one in a different way: a `gh` login, a token in a file, an SSH key or a
deploy key, or no login at all for a public repository. Scripts, Make targets and AI
sessions should not have to know any of that. They name a **service** — a short name the
person chose for a repository — and k-playbook resolves which access applies.

The file is **personal**: one per person, for every project on the machine. It is not part
of any project and not versioned. k-playbook manages it, but its content is not about
k-playbook; the same file serves a project's Makefile as well as an AI session.

The file contains **no secrets**. It only refers to where a credential is: a `gh` login
name, the path of a token file, the path of a key. An AI session may read it. The token
files and keys it points to are a different matter and are never read into any output.

## Names

These names are fixed and used unchanged everywhere.

| What | Name |
|---|---|
| File | `$XDG_CONFIG_HOME/k-playbook/github.yaml`, or `~/.config/k-playbook/github.yaml` when `XDG_CONFIG_HOME` is unset or not an absolute path |
| Override | environment variable `K_PLAYBOOK_GITHUB_CONFIG=<path>`; the path is used as given |
| Subcommand | `k-playbook github <verb>` |
| Verbs in this step | `check`, `show` |
| Health area | `github` (see Task 098, the environment check in the briefing) |

The override exists for tests and for containers that mount the file somewhere else. It is
the only second place to look: when the variable is set, the default location is ignored,
including for creating the template.

## Format

YAML, one top-level key `services`. Each service names a repository and an ordered list of
accesses.

```yaml
services:
  website:
    repo: acme/website
    access:
      - type: gh
        login: alice

  backend:
    repo: acme-labs/backend
    access:
      - type: ssh
        key: ~/.ssh/id_ed25519_acme_labs
      - type: token
        file: ~/.config/github-tokens/acme-labs.token
        disabled: true          # issued, waiting for the organisation's approval

  playbook:
    repo: example/k-playbook
    access:
      - type: ssh
        host: github-example    # alias in ~/.ssh/config, a deploy key for this repository
      - type: public
```

### `services`

A mapping from service name to service. The name is chosen by the person. It is the
argument every caller passes, so it should be short and stable. Allowed are letters, digits,
`-` and `_`; names are case-sensitive and must be unique. An empty mapping (`services: {}`)
is valid and means: nothing configured yet.

### A service

| Key | Required | Meaning |
|---|---|---|
| `repo` | yes | `<owner>/<repo>` on github.com. No URL, no `.git`. The host is always github.com; that includes every organisation on GitHub Enterprise Cloud, managed users included, because they live on github.com. Only GitHub Enterprise **Server** — a company's own installation under its own host name — is out of scope, in line with `gh` support elsewhere in k-playbook (`GHHost`). |
| `access` | yes, at least one entry | The accesses in order of precedence. The first entry that is available is used. Availability is not part of this step; see *Outlook*. |

Unknown keys are an error. Two services may name the same repository; that is how one
person keeps a writing and a read-only service for the same repo apart, if they want to.

### An access

Every access has a `type`. Which other keys are allowed depends on it; a missing required
key or a key that does not belong to the type is an error.

| `type` | Keys | Meaning |
|---|---|---|
| `gh` | `login` (required) | The token of this login in the local `gh` configuration (`gh auth token -u <login>`). |
| `token` | `file` (required) | A token read from this file: a fine-grained or classic personal access token, one line, no trailing newline. The file itself is expected to be mode `0600`; the file is not read in this step. |
| `ssh` | exactly one of `key` or `host` | Which key SSH uses for this repository; see below. Deploy keys are configured this way, which is why an SSH access belongs to a service, not to an account. |
| `public` | none | No login. The repository can be read anonymously. |

Every access may carry `disabled: true`. A disabled access stays in the file and is shown,
but it is never chosen. It marks an access that exists but does not work yet, such as a
token waiting for an organisation's approval.

#### `ssh`: `key` or `host`

SSH decides which key to offer from two places, and the access names exactly one of them:

- **`key`** is the path of a private key. k-playbook will run git with
  `GIT_SSH_COMMAND="ssh -i <key> -o IdentitiesOnly=yes"` against the plain URL
  `git@github.com:<owner>/<repo>`. Nothing in `~/.ssh/config` is needed; `IdentitiesOnly`
  keeps the agent from offering other keys first, which GitHub would otherwise accept as a
  different user.
- **`host`** is a `Host` alias from `~/.ssh/config`, such as `github-example`. k-playbook will
  use the URL `git@<host>:<owner>/<repo>` and leave the key choice to SSH, which reads
  `IdentityFile` and the rest from the alias. This is how deploy keys are usually set up by
  hand today. Whoever wants the default key of the machine, deliberately, writes
  `host: github.com`.

Both at once would be two statements about the same key that can contradict each other, and
k-playbook would have to decide which one wins. Neither would mean "whatever SSH picks" — the
unstated default this file exists to replace.

#### Paths

Paths may start with `~`, which is expanded to the home directory. Otherwise they must be
absolute. A relative path such as `tokens/acme.token` would be resolved against the working
directory of whoever calls k-playbook — a Makefile in one project, a shell in another, an MCP
server in a third — and would name a different file each time. The reader rejects it instead
of guessing.

### Reading versus writing

There is no `rights` field. Every access except `public` is assumed to allow writing, within
what GitHub grants it. `public` is the only read-only access: an operation that writes skips
it. Whether a token or a key actually has write rights on the repository is decided by
GitHub, not by this file; the check in a later step will report it.

## Reading the file

The reader returns the whole configuration or a list of problems — never a mix, never only
the first problem. A problem names the service and access it belongs to (`services.backend.
access[1]`), says what is wrong in one sentence, and says what to do. The list is sorted by
position in the file.

Problems in this step, all detected from the file alone:

- The file exists but is not readable (permissions, directory instead of file).
- The content is not YAML the reader understands.
- A top-level key other than `services`.
- A service name that violates the name rule.
- A service without `repo`, or with a `repo` that is not `<owner>/<repo>`.
- A service without `access`, or with an empty list.
- An access without `type`, or with an unknown `type`.
- An access with a missing required key, with a key that does not belong to its type, or
  with both `key` and `host`.
- A relative path.
- A key the format does not know anywhere.

What the reader does **not** check in this step: whether a `gh` login is known to `gh`,
whether a token file or key exists, whether an SSH alias is configured, whether the repository
exists. Those are *availability* and come later.

## When the file is missing

A missing file is a state the person has to resolve, not a crash. On the first read that
finds no file, k-playbook:

1. creates the directory (`~/.config/k-playbook`, mode `0700`) if it does not exist;
2. writes a template, mode `0600`, containing `services: {}` and a commented example of every
   access type, so the person can fill it in without opening this page;
3. reports, once, as a `warning`: *The file was just created and is empty — add your
   services.* The call that created the template is the one that carries the warning; a
   later call finds an existing, empty file and reports nothing.

An **empty file** — `services: {}`, or only comments — is not a problem. A machine may have no
GitHub work at all, or none that goes through this file yet. `check` says so in one line and
exits `0`; the health check lists nothing. Only a file that *says* something wrong is a problem.

The template is created only at the default location or at the path of the override. It is
never created inside a project. If the directory cannot be created or written — a read-only
mount in a container is the expected case — the problem says so and names the path to create
on the host instead.

## The subcommand

```
k-playbook github check [--json]
k-playbook github show [<service>] [--json]
```

`check` reads the file and prints the problems; without problems it prints one line that
names the file and the number of services. `show` prints the configuration as read, one
service per block, or one service when named; it prints what the file says, resolved paths
included, and nothing the file does not contain. Both create the template when the file is
missing, as described above.

Exit codes: `0` no problems, `1` at least one problem, `2` usage error. `--json` prints the
configuration together with the `problems` list; the shape of the JSON is the shape of this
page's tables and is fixed by the tests.

The health check (Task 098) calls the same reader and lists its problems under the area
`github`: structural problems as `error`, the one-time notice about a freshly created template
as `warning`. An empty file produces nothing. A session therefore sees a broken file in its
briefing, and is told once when the file has just come into being.

## Relation to other pages

- `local-github-ssh.md` describes one machine's SSH aliases and deploy keys by hand. Once
  that machine's services are in `github.yaml`, the page is absorbed into this one.
- `mcp.md` gets the `k_playbook_health` contract; this page does not define MCP tools.

## Outlook

Not part of the contract yet; each will be specified here before it is built.

- **Availability** per access: `gh` logged in as that login, token file present and `0600`,
  key present or alias configured, agent reachable. Then `check` also reports what is
  configured but unusable, and the first *available* access becomes meaningful.
- **Running under a service**: `k-playbook github run <service> [--write] -- <command>`,
  which sets up the chosen access for that one process (token through a credential helper,
  `GIT_SSH_COMMAND`, `insteadOf` between SSH and HTTPS) and never prints the credential.
- **Reverse lookup**: from a repository's remote to the service that names it.
- **Named operations** with project rules, as CLI and MCP.
- **A shared, versioned part** for projects where every person uses the same repositories
  with different accounts.
- **Containers**: `~/.config/k-playbook` mounted read-only; what the template step does there
  is described above, the mount itself is the project's business.
