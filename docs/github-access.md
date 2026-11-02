# GitHub access

The contract for the personal GitHub access file: where it lives, what it contains, how
k-playbook reads and checks it, and what happens when it is missing.

This page is the **single** definition. The reader, the subcommand, the check, and the tests
refer to it and do not redefine any of it. What is not stated here is not defined.

**Status (2026-10-28):** step 1 is implemented (Task 099): the reader
(`installer/internal/githubaccess`), the template, `k-playbook github check` and `show`, and
the health area `github`. Step 2 is implemented (Task 102): *availability* of an access
without network (`availability.go`, part of `check`, `show` and the health check), the *test*
against GitHub (`test.go`; `k-playbook github test`, `k_playbook_github_test`, the interface
page `/github-access`), see the two sections of those names. The gh configuration is read by
the shared package `internal/ghconfig`, which `project` and `githubaccess` both use.

**Status (2026-11-02):** step 3 is implemented (Task 110): *Running a command under a
service*, `k-playbook github run` — the choice per channel (`choose.go`), the environment of
the child (`runenv.go`) and the verb (`cmd/k-playbook/githubrun.go`). Later steps — named
operations among them — are listed under *Outlook* and are not part of the contract until they
are specified here.

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
| Verbs | `check`, `show` (step 1), `test` (step 2), `run` (step 3) |
| Health area | `github` (see Task 098, the environment check in the briefing) |
| MCP tool | `k_playbook_github_test` (step 2); contract in `mcp.md` |
| Interface page | *GitHub-Zugänge*, `/github-access` (step 2) |

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
| `access` | yes, at least one entry | The accesses in order of precedence. The first entry that is available is used; see *Availability*, and for `run` the two channels under *Running a command under a service*. |

Unknown keys are an error. Two services may name the same repository; that is how one
person keeps a writing and a read-only service for the same repo apart, if they want to.

### An access

Every access has a `type`. Which other keys are allowed depends on it; a missing required
key or a key that does not belong to the type is an error.

| `type` | Keys | Meaning |
|---|---|---|
| `gh` | `login` (required) | The token of this login in the local `gh` configuration (`gh auth token -u <login>`). |
| `token` | `file` (required) | A token read from this file: a fine-grained or classic personal access token, one line, no trailing newline. The file itself must be mode `0600`. |
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
GitHub, not by this file; `test` reports it where it can (see *Testing an access*).

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
- `disabled` with a value other than `true` or `false`.
- A key the format does not know anywhere.

What the reader does **not** check: whether a `gh` login is known to `gh`, whether a token
file or key exists, whether an SSH alias is configured. Those are *availability*, the next
section, and they are states of an access, not problems of the file. Whether the repository
exists and what the access may do there is only known to GitHub; that is *Testing an access*.

## Availability

Availability says whether an access can be tried on this machine, judged **without the
network** and in well under a second. It is computed by `check` and `show`, shown in both,
and feeds the health check. Every access has exactly one state:

| State | Meaning |
|---|---|
| `available` | Everything the access needs on this machine is in place. |
| `unavailable` | Something is missing; `reason` says what, in one sentence, with what to do. |
| `disabled` | `disabled: true` in the file. Not judged, never chosen. |

What "in place" means per type:

| `type` | available when |
|---|---|
| `gh` | `gh` is installed and `login` is one of the accounts in gh's configuration for github.com — read from gh's files the way `context.gh` already does, not asked of the server. |
| `token` | `file` is a regular file, mode `0600` (no group or world bits), and contains exactly one non-empty line. k-playbook reads the file only to judge that; the content never leaves the process and never appears in any output, log or error. |
| `ssh` with `key` | `key` is a regular file whose mode has no group or world bits — SSH refuses such a key anyway ("UNPROTECTED PRIVATE KEY FILE"). |
| `ssh` with `host` | `ssh -G <host>` resolves the alias and its `hostname` is `github.com`. An unknown alias resolves to itself and therefore fails this rule; `host: github.com` passes it. |
| `public` | always. |

A service is **usable** when at least one access is `available`. A service whose accesses are
all `unavailable` or `disabled` is reported by the health check as a `warning` under `github`
(`services.<name>: kein verfügbarer Zugang — <reason of the first access>`), because the
file still parses but nothing in it can be used. The health check does not go further than
this: it never contacts GitHub.

The ssh-agent is deliberately not part of availability: a key with a passphrase that is not
in the agent is *available* here and fails in `test`, with SSH's own message. Judging the agent
without the network would mean listing its keys, and that differs between hosts and containers
in ways this step does not want to encode.

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
on the host instead. If the path is occupied without being a readable file — a symbolic link
whose target does not exist — the template cannot be created either: the problem says so,
names the link and its target, and asks to remove the link or create the target. After the
template step the reader reads the file at most once more; it never retries beyond that.

## Testing an access

`test` asks GitHub whether an access works. It is the only part of this page that uses the
network, and it runs **only on explicit request**: the verb, the MCP tool, the button on the
interface page. Neither the briefing nor the health check nor `check` ever trigger it. A test
of one access is several requests and can take seconds; a machine with many services, or a
container that several people start, must not pay that on every session. A project that
wants a full test at container start calls `k-playbook github test` from its own start script
— that is the project's decision, not k-playbook's.

`test` covers every `available` access of the named service, or of all services. `disabled`
and `unavailable` accesses are reported as `skipped` with their reason and are not contacted.
Each tested access ends in one of:

| Result | Meaning |
|---|---|
| `ok` | GitHub accepted the credential and the repository is reachable with it. |
| `failed` | GitHub rejected the credential, or the repository is not reachable with it; `message` carries GitHub's answer (status code and GitHub's own text, or SSH's stderr). |
| `skipped` | Not tested: `disabled`, `unavailable`, or an earlier step for the same access already failed. |

What is asked, per type:

| `type` | Identity | Read | Write | Expiry |
|---|---|---|---|---|
| `gh`, `token` | `GET /user` with the token as bearer → `login` | `GET /repos/<owner>/<repo>` → `200` yes; `404` no (also for a fine-grained token the organisation has not approved); `401` the token is invalid; `403` with a SAML/SSO text → the organisation requires authorisation | `permissions.push` of the same answer → `yes`/`no` | header `github-authentication-token-expiration` → date; absent → `none` |
| `ssh` | `ssh -T git@github.com` (`-i <key> -o IdentitiesOnly=yes` for `key`, `git@<host>` for `host`), `BatchMode=yes` → the name in "Hi <name>!": a user, or `<owner>/<repo>` for a deploy key | `git ls-remote <ssh url> HEAD` → yes/no | `unknown` — write over SSH cannot be told without a push, and `test` never pushes | `none` |
| `public` | `anonymous` | `git ls-remote https://github.com/<owner>/<repo>.git HEAD` without credentials → yes/no | `no` | `none` |

Rules:

- Every request carries a timeout of ten seconds; a timeout is `failed` with that word.
- An expiry within fourteen days is a `warning` on the result (`läuft in 9 Tagen ab`); an
  expiry in the past is `failed`.
- The token is passed to `gh` and to the HTTP client in memory and to nothing else. No
  command line carries it, no message contains it, not even a prefix. SSH's stderr is passed
  through as GitHub and SSH print it; it never contains a credential.
- `test` reads; it never pushes, never creates anything at GitHub, never changes the file.

### MCP tool

`k_playbook_github_test` takes `projectDir` (as every tool) and an optional `service`, runs
the same test and returns the object of `test --json` under `test`, wrapped in the usual
envelope (`ok`, `tool`, `projectDir`, `error`). It exists so that a session that just failed
to push can find out which access works instead of guessing — a read-only question with a
network cost the session asks for knowingly. The full tool contract lives in `mcp.md`.

### Interface page

The interface gets a page *GitHub-Zugänge* at `/github-access`, next to the project pages but
about the person, not the project: it shows the file path, the state of `check` (problems, or
one line), and one block per service with the repository and the accesses in order, each with
type, what the file says, and its availability. A button *Alle prüfen* runs `test` for all
services, a button per service runs it for one; the results fill in identity, read, write,
expiry and the message of a failure. Nothing on the page writes the file: editing stays in the
editor, the page says where the file is. Endpoints: `GET /api/github-access` returns the
object of `check --json`; `POST /api/github-access/test` with an optional `service` returns
the object of `test --json`. Like every other `GET` of the interface it carries `available`
and `message` when the file cannot be read at all.

## The subcommand

```
k-playbook github check [--json]
k-playbook github show [<service>] [--json]
k-playbook github test [<service>] [--json]
k-playbook github run <service> [--write] [--use <type>] [--skip <type>]… [--quiet] -- <command> [<arg>…]
```

`run` has its own section, *Running a command under a service*, including its exit codes;
what follows here is about `check`, `show` and `test`.

`check` reads the file and prints the problems; without problems it prints one line that
names the file, the number of services and the number of services without an available
access, followed by one line per access that is `unavailable` (`services.backend.access[0]:
nicht verfügbar — <reason>`). `show` prints the configuration as read, one service per
block, or one service when named; it prints what the file says, resolved paths included, each
access with its availability, and nothing the file does not contain. Both create the template
when the file is missing, as described above. `test` runs *Testing an access* for all services
or the named one and prints one block per service with one line per access and its result.

`show <service>` with a service that is not in the file is a usage error: one message that
names the known services, exit `2`, no JSON object even with `--json`. `show` on a file with
at least one `error` prints the problems exactly like `check` — in plain text the same lines,
with `--json` the object of `check --json` (with `problems` and `services` as `{}`) — and
exits `1`: there is no configuration to show, only the problems.

Exit codes: `0` no problems, `1` at least one problem, `2` usage error. The one-time
warning about a freshly created template does not change the exit code: a created file is a
state to resolve, not a broken one. An `unavailable` access does not change the exit code of
`check` or `show` either — it is a state, reported as a warning by the health check, not a
problem of the file. For `test`: `0` when every tested access is `ok` (skipped ones do not
count), `1` when at least one `failed`, `2` usage error, including an unknown service.

### JSON

`--json` is for readers that must not parse text: an AI session and the interface. `check
--json` prints this, here for the file of the *Format* section without problems:

```json
{
  "file": "/home/alice/.config/k-playbook/github.yaml",
  "created": false,
  "services": {
    "website": {
      "repo": "acme/website",
      "access": [
        { "type": "gh", "login": "alice", "state": "available" }
      ]
    },
    "backend": {
      "repo": "acme-labs/backend",
      "access": [
        { "type": "ssh", "key": "/home/alice/.ssh/id_ed25519_acme_labs", "state": "unavailable",
          "reason": "Die Schlüsseldatei fehlt." },
        { "type": "token", "file": "/home/alice/.config/github-tokens/acme-labs.token", "disabled": true,
          "state": "disabled" }
      ]
    },
    "playbook": {
      "repo": "example/k-playbook",
      "access": [
        { "type": "ssh", "host": "github-example", "state": "available" },
        { "type": "public", "state": "available" }
      ]
    }
  },
  "problems": []
}
```

`state` and `reason` are the availability of step 2; a binary of step 1 does not print them.
`reason` is present only for `unavailable`.

And this for a file with one problem — configuration or problems, never both:

```json
{
  "file": "/home/alice/.config/k-playbook/github.yaml",
  "created": false,
  "services": {},
  "problems": [
    {
      "level": "error",
      "path": "services.backend.access[1]",
      "message": "Der Pfad in file ist relativ.",
      "hint": "Absoluten Pfad oder ~/… eintragen."
    }
  ]
}
```

- `file` is the path that was read, after the override and `XDG_CONFIG_HOME` were applied.
- `created` says whether this call wrote the template.
- `services` mirrors the file: the same keys, `~` expanded, no comments, `disabled` only
  where it is `true`. Services are in file order; the order of `access` is the precedence.
  As soon as one problem has level `error`, `services` is `{}`; the one-time `warning` of a
  freshly created template leaves it as it is (empty, because the template is empty).
- `problems` has `level` (`error`|`warning`), `path` (where in the file, `services.<name>` or
  `services.<name>.access[<index>]`, `file` for the file as a whole), `message` and `hint`.
  An entry of the health check has `level`, `area`, `message` and `hint` but no `path`; the
  health source `github` therefore puts the path in front of the message
  (`services.backend.access[1]: Der Pfad in file ist relativ.`) and sets `area` to `github`.
  Messages and hints are German, like every other text k-playbook shows to people.

`show --json` prints the same object without `problems`; with a service named, `services`
holds only that one. Text and JSON carry the same information — nothing is only in one of
them.

`test --json` prints the file, the services tested and one result per access:

```json
{
  "file": "/home/alice/.config/k-playbook/github.yaml",
  "services": {
    "backend": {
      "repo": "acme-labs/backend",
      "access": [
        { "type": "ssh", "key": "/home/alice/.ssh/id_ed25519_acme_labs",
          "result": "ok", "identity": "alice", "read": "yes", "write": "unknown", "expires": "none" },
        { "type": "token", "file": "/home/alice/.config/github-tokens/acme-labs.token", "disabled": true,
          "result": "skipped", "message": "deaktiviert" }
      ]
    },
    "playbook": {
      "repo": "example/k-playbook",
      "access": [
        { "type": "ssh", "host": "github-example",
          "result": "failed", "message": "git@github.com: Permission denied (publickey)." },
        { "type": "public", "result": "ok", "identity": "anonymous", "read": "yes", "write": "no", "expires": "none" }
      ]
    }
  },
  "ok": false
}
```

- `result` is `ok`, `failed` or `skipped`; `message` is present for `failed` and `skipped`.
- `identity` is the login GitHub reported, `<owner>/<repo>` for a deploy key, `anonymous`
  for `public`; absent when nothing was reached.
- `read` and `write` are `yes`, `no` or `unknown`; `expires` is a date (`YYYY-MM-DD`) or
  `none`; `warning` is present when the expiry is within fourteen days.
- `ok` is true when no tested access `failed`.

The health check (Task 098) calls the same reader and lists its problems under the area
`github`: structural problems as `error`, the one-time notice about a freshly created template
as `warning`, a service without an available access as `warning` (see *Availability*). An
empty file produces nothing. A session therefore sees a broken or unusable file in its
briefing, and is told once when the file has just come into being.

## Running a command under a service

`run` starts one command with the access of a service set up for that process, and for
nothing else. It is how scripts and Make targets reach a repository without knowing which
access applies: they name the service and keep their own logic — which branch, which lease,
which merge rule — in their own code. `run` replaces the hand-made setup that scripts carry
today (`gh auth token -u`, `GH_TOKEN`, a credential helper, `insteadOf`).

```
k-playbook github run OMNI-GW -- git push origin feature/x
k-playbook github run OMNI-GW -- gh pr create --base development --draft --fill
k-playbook github run chapter-mcps -- gh release download v1.2.0 -p '*.tar.gz'
```

`run` needs no k-playbook project; the service is the only context. It is a command-line verb
only: there is no MCP tool and no button for it, because running an arbitrary command is not
something an AI session or the interface should be offered. AI sessions get named operations
instead (*Outlook*).

### Choosing the accesses

The choice is made **without the network**, from the availability of every access (see
*Availability*), and in file order: the order of `access` is the precedence, and each person
decides it in their own file. `run` chooses two things, because a command may need two:

| Channel | Used for | Chosen access |
|---|---|---|
| `git` | git's transport: clone, fetch, pull, push, ls-remote | the first `available` access of any type |
| `api` | `gh` and anything else that reads `GH_TOKEN`/`GITHUB_TOKEN` | the first `available` access of type `token` or `gh` |

`make omni-pr` is the reason: it pushes and then opens a pull request. With `ssh` first in the
file the push goes over SSH and the pull request still gets a token, if the service has one.
When the `git` channel chooses a `token` or `gh` access, it is the same access as the `api`
channel — the first available one is also the first available one with a token — so a command
never sees two different tokens.

Options narrow the choice; they apply to both channels:

- `--write` skips `public`. Without it, `public` may be chosen for `git` like any other access.
- `--use <type>` considers only accesses of that type (`gh`, `token`, `ssh`, `public`).
- `--skip <type>` leaves accesses of that type out; it may be given more than once.

A channel without a usable access stays empty; that alone is not an error, because the command
may not need it. Only when **both** channels are empty does `run` refuse to start the command.

There is **no automatic fallback**. `run` starts the command exactly once. If it fails, its exit
code is returned and nothing else happens: `run` sees only an exit code and cannot tell a
rejected credential from a failing command, and repeating `gh pr create` or `gh pr merge` with
another access could create or merge twice. A caller that wants another access says so —
`--skip ssh`, `--use token` — and an AI session asks `k_playbook_github_test` first.

### What is set up

Everything is set up in the environment of the command only. No file is written — except
the template, which `run` creates like every other read when the file is missing (see *When the
file is missing*; `run` then ends with `125`, because the template names no service) —, no git
configuration outside that process changes, `gh auth switch` is never called. Existing
`GIT_CONFIG_COUNT` entries of the caller are kept; `run` appends its own after them.

| Chosen for `git` | Setup |
|---|---|
| `token`, `gh` | SSH forms of the repository URL are rewritten to `https://github.com/<owner>/<repo>`; credential helpers of the machine are cleared for that URL, and a credential helper for it answers with the token. Clearing them keeps git from handing the token to a helper of the machine (`store` after a successful login), which would keep it outside the process. |
| `ssh` with `key` | HTTPS forms are rewritten to `git@github.com:<owner>/<repo>`; `GIT_SSH_COMMAND="ssh -i <key> -o IdentitiesOnly=yes"`. |
| `ssh` with `host` | HTTPS forms and `git@github.com:` forms are rewritten to `git@<host>:<owner>/<repo>`. |
| `public` | SSH forms are rewritten to HTTPS; credential helpers are cleared for that URL, so no other credential of the machine is used. |

| Chosen for `api` | Setup |
|---|---|
| `token`, `gh` | `GH_TOKEN` and `GITHUB_TOKEN` are set to the token. |
| none | `GH_TOKEN` and `GITHUB_TOKEN` are removed from the child's environment, so the command does not quietly use a token the caller happened to have. |

Rules:

- **Only this repository.** Rewrites and the credential helper are bound to the URLs of the
  service's repository: `https://github.com/<owner>/<repo>`, `git@github.com:<owner>/<repo>`,
  `ssh://git@github.com/<owner>/<repo>`, each with and without `.git`, and the `git@<host>:` form
  of every `host` alias the service itself lists. Other repositories in the same command —
  submodules, a second clone — are left as the machine has them. git matches `insteadOf` by
  prefix; the form without `.git` therefore also catches a repository whose name only starts
  with the same text (`acme/web` and `acme/website`). That is accepted: a command run under one
  service rarely touches such a neighbour, and the form with `.git` is exact.
- **The target form is pinned.** The form the table rewrites to is also rewritten to itself,
  with and without `.git`, so a rewrite of the machine cannot take it elsewhere. git applies
  the longest matching prefix, and the repository's own URL is longer than a rule for all of
  `github.com`, such as `url."git@github.com:".insteadOf https://github.com/`.
- **Fetch and push.** Every rewrite, the pinned target form included, is set up for fetch and
  push (`insteadOf` and `pushInsteadOf`), so a push rewrite of the machine cannot take the push
  elsewhere. git looks for the push URL under `pushInsteadOf` first; an `insteadOf` alone would
  lose to a `pushInsteadOf` of the machine.
- **The token.** For a `gh` access the token is read with `gh auth token -u <login>` when
  `run` starts; for a `token` access from the file. It is in the child's environment —
  necessarily, `gh` and git's credential helper read it there — and therefore visible to every
  process the command starts. It is never on a command line, never in a git configuration
  value (the helper reads it from the environment), and never in any output of `run`.
- **No prompts.** `GIT_TERMINAL_PROMPT=0` is set, so a rejected credential ends the command
  instead of waiting for a password nobody types.
- Standard input, output and error, the working directory and every other variable are passed
  through unchanged.

### Output and exit codes

Before starting the command, `run` writes one line to standard error that names the service and
both choices, for example `github run OMNI-GW: git über token ~/.config/github-tokens/koelnmesse-IT.token, api über token`
or `… git über ssh host github.com-km, api über gh kkl_kmde`; an empty channel reads `api: keiner`.
`--quiet` suppresses the line. Standard output belongs to the command alone.

The line is `github run <service>: <git part>, <api part>`. A channel names its access as
`über <type> <what the file says>`:

| Access | Part |
|---|---|
| `gh` | `über gh <login>` |
| `token` | `über token <file>` |
| `ssh` with `key` | `über ssh key <key>` |
| `ssh` with `host` | `über ssh host <host>` |
| `public` | `über public` |
| none | `git: keiner` or `api: keiner` |

Paths under the home directory are shown with `~`. The api part names only the type — `api über
token`, `api über gh` — when it is the same access as the git part, which is always so when git
uses a `token` or `gh` access; otherwise it names login or path like the git part (`api über gh
kkl_kmde` after `git über ssh host github.com-km`).

The exit code is the command's. `run`'s own failures use the codes `env` uses, so they do not
collide with the command's:

| Code | Meaning |
|---|---|
| the command's | the command ran. `run` replaces itself with the command, so a signal reaches the command directly and once, and a command ended by a signal ends `run` by the same signal, which a shell reports as `128 + <signal>` |
| `125` | `run` itself failed: usage error, unknown service (the message names the known services), a file with problems, no usable access in either channel, the token could not be read |
| `126` | the command was found but cannot be executed |
| `127` | the command was not found |

Every failure of `run` itself is one German message on standard error with what to do, like the
problems of *Reading the file*; `k-playbook github check` names the reason per access.

## Relation to other pages

- `local-github-ssh.md` describes one machine's SSH aliases and deploy keys by hand. That
  machine's services are in its `github.yaml` since step 1; the page is absorbed into this
  one once `test` can tell whether those aliases work.
- `mcp.md` holds the contracts of `k_playbook_health` and `k_playbook_github_test`; this page
  names the tool and says what it is for.

## Outlook

Not part of the contract yet; each will be specified here before it is built.

- **Choosing an access for a caller**: a verb and an MCP tool that name, per service, the
  accesses that work right now in file order — without the network by default, with `--test`
  against GitHub. Requested from OMNI on 2026-11-02; what a script gets (the first, all, or a
  choice) and that it gets the way to a credential, never its value, are still to be specified.
- **Reverse lookup**: from a repository's remote to the service that names it.
- **Named operations** with project rules, as CLI and MCP — the way AI sessions reach GitHub,
  since `run` is not offered to them: first the reading ones (fetch with status, list of pull
  requests), then pushing a feature branch and a draft pull request, each with approval, merging
  last.
- **A shared, versioned part** for projects where every person uses the same repositories
  with different accounts.
- **Containers**: `~/.config/k-playbook` mounted read-only; what the template step does there
  is described above, the mount itself is the project's business.
