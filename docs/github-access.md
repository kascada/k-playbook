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

**Status (2026-11-05):** the choice of `run` is readable without running it (Task 118): the field
`selected` per service in `check`, `show` and `test --json`, `selectedOk` in `test --json`, and
the matching lines in their text (*The subcommand*, *JSON*). Both come from `Select` in
`choose.go`, which applies `Choose` and nothing else.

**Status (2026-11-06):** the file is created only on request (Task 121): `k-playbook github
init` and the button *Anlegen* on the interface page write the template; no read does —
neither the briefing, the health check, `check`, `show`, `test`, `run`, the MCP tool nor the
page. A missing file is no problem: `present: false` and a hint to `init` (*When the file is
missing*).

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
| Verbs | `check`, `show` (step 1), `test` (step 2), `run` (step 3), `init` (Task 121) |
| Health area | `github` (see Task 098, the environment check in the briefing) |
| MCP tool | `k_playbook_github_test` (step 2); contract in `mcp.md` |
| Interface page | *GitHub-Zugänge*, `/github-access` (step 2) |

The override exists for tests and for containers that mount the file somewhere else. It is
the only second place to look: when the variable is set, the default location is ignored,
including for `init`.

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

The file is **optional**. Whoever does not use the GitHub access file notices nothing of it, and
**no read creates it**: not the briefing, not the health check, not `check`, `show`, `test` or
`run`, not `k_playbook_github_test`, not the interface page. A missing file is not a problem:

- the health check lists nothing under `github`;
- the briefing line reads `GitHub: keine Zugangsdatei` (*In the briefing*);
- `check`, `show` and `test` print the hint *keine Zugangsdatei — `k-playbook github init` legt
  sie an* and exit `0`; with `--json` the object carries `"present": false` and that sentence
  as `hint`, and `problems` stays empty — there is no level below `error` for it;
- `run` ends with `125` and names `k-playbook github init`;
- `k_playbook_github_test` answers `ok: true` with `test.present: false` and the hint, no
  error code;
- the interface page shows *Keine Zugangsdatei* and a button *Anlegen*.

"Missing" means nothing is at the path, not even a link. A path that is occupied without
being a readable file — a symbolic link whose target does not exist — is not missing: the
reader tells the two apart without creating anything (`os.Lstat`) and reports the link as an
`error` that names it and its target; `present` is `true` then.

The file is created by **`k-playbook github init [--json]`**, or by the button *Anlegen* on the
interface page, which calls the same code (`githubaccess.Init`). An AI session that wants the
file calls `init`; it does not write the YAML itself, so template, modes and the container
case stay in one place. `init`:

1. creates the directory (`~/.config/k-playbook`, mode `0700`) if it does not exist;
2. writes a template, mode `0600`, containing `services: {}` and a commented example of every
   access type, so the person can fill it in without opening this page;
3. prints `<path>: angelegt (Verzeichnis 0700, Datei 0600) — trage deine Dienste ein; …` and
   exits `0`.

When the file is already there, `init` writes nothing, whatever it contains, prints `<path>:
schon vorhanden, nichts geändert` and exits `0`. `init --json` prints `{file, created,
problems}`; `created` says whether this call wrote the template. `k-playbook upgrade` does not
create the file.

An **empty file** — `services: {}`, or only comments — is not a problem. A machine may have no
GitHub work at all, or none that goes through this file yet. `check` says so in one line and
exits `0`; the health check lists nothing. Only a file that *says* something wrong is a problem.

`init` creates the template only at the default location or at the path of the override. It
is never created inside a project. If the directory cannot be created or written — a
read-only mount in a container is the expected case — `init` reports an `error` that names the
path to create on the host instead, and exits `1`; the button answers with the same hint. If
the path is occupied without being a readable file — a symbolic link whose target does not
exist, or a directory — `init` creates nothing either: the problem names what is there and
asks to remove it (or, for a link, to create its target).

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
envelope (`ok`, `tool`, `projectDir`, `error`). Without a file it answers `ok: true` with
`test.present: false` and the hint to `k-playbook github init`, tests nothing and creates
nothing; there is no MCP tool for `init` — a session calls the CLI. It exists so that a session that just failed
to push can find out which access works instead of guessing — a read-only question with a
network cost the session asks for knowingly. The full tool contract lives in `mcp.md`.

### Interface page

The interface gets a page *GitHub-Zugänge* at `/github-access`, next to the project pages but
about the person, not the project: it shows the file path, the state of `check` (problems, or
one line), and one block per service with the repository and the accesses in order, each with
type, what the file says, and its availability. A button *Alle prüfen* runs `test` for all
services, a button per service runs it for one; the results fill in identity, read, write,
expiry and the message of a failure. Editing stays in the editor, the page says where the
file is. The page writes the file only when it is missing and the button *Anlegen* is pressed
— the only time the button is shown; it calls the same code as `k-playbook github init`, and in
a read-only container it shows the same hint for the host. Endpoints: `GET
/api/github-access` returns the object of `check --json` and writes nothing (without a file:
`present: false`, `hint`); `POST /api/github-access/init` returns the object of `init --json`,
`409` when the template cannot be created; `POST /api/github-access/test` with an optional
`service` returns the object of `test --json`. Like every other `GET` of the interface it
carries `available` and `message` when the file cannot be read at all. Both pass the objects through unchanged, so
`selected` and `selectedOk` (*JSON*) are in them; the page does not show them yet.

## The subcommand

```
k-playbook github init [--json]
k-playbook github check [--json]
k-playbook github show [<service>] [--json]
k-playbook github test [<service>] [--json]
k-playbook github run <service> [--write] [--use <type>] [--skip <type>]… [--quiet] -- <command> [<arg>…]
```

`run` has its own section, *Running a command under a service*, including its exit codes;
`init` is described under *When the file is missing*; what follows here is about `check`,
`show` and `test`.

`check` reads the file and prints the problems; without problems it prints one line that
names the file, the number of services and the number of services without an available
access, followed per service by one line per access that is `unavailable`
(`services.backend.access[0]: nicht verfügbar — <reason>`) and the choice of `run` for that
service (`backend run: kein Zugang`, see below). `show` prints the configuration as read, one
service per block, or one service when named; it prints what the file says, resolved paths
included, each access with its availability, then the choice of `run`, and nothing the file
does not contain. Neither writes anything; without a file both print the hint to `init`, as
described above.
`test` runs *Testing an access* for all services or the named one and prints one block per
service with one line per access and its result, then the choice of `run` with the result of
each chosen access.

The choice of `run` is the field `selected` of the JSON (below) in the parts of `run`'s own line
(*Output and exit codes*): one line for `run` without options, and a second one only when
`--write` chooses differently. In `show` and `test` the lines are indented under the service, in
`check` the service name stands in front:

```
playbook: example/k-playbook
  1. ssh  host: github-example  [verfügbar]
  2. public  [verfügbar]
  run: git über ssh host github-example, api: keiner

offen: acme/offen
  1. public  [verfügbar]
  2. ssh  key: /home/alice/.ssh/id_offen  [verfügbar]
  run: git über public, api: keiner
  mit --write: git über ssh key ~/.ssh/id_offen, api: keiner

website: acme/website
  1. gh  login: alice  [verfügbar]
  run: git über gh alice, api über gh

backend: acme-labs/backend
  1. ssh  key: /home/alice/.ssh/id_ed25519_acme_labs  [nicht verfügbar — …]
  2. token  file: /home/alice/.config/github-tokens/acme-labs.token  [deaktiviert]
  run: kein Zugang
```

A service with only `public` reads `run: git über public, api: keiner` and `mit --write: kein
Zugang`. `check` prints the same parts as `website run: git über gh alice, api über gh` and
`offen mit --write: …`. `test` adds the result of each chosen access to its channel —
`run: git über ssh host github-example → fehlgeschlagen, api: keiner` — with the words of its
access lines (`ok`, `fehlgeschlagen`, `übersprungen`); the choice itself is the one before the
test (see `selectedOk` below).

`show <service>` with a service that is not in the file is a usage error: one message that
names the known services, exit `2`, no JSON object even with `--json`. `show` on a file with
at least one `error` prints the problems exactly like `check` — in plain text the same lines,
with `--json` the object of `check --json` (with `problems` and `services` as `{}`) — and
exits `1`: there is no configuration to show, only the problems.

Exit codes: `0` no problems, `1` at least one problem, `2` usage error. A missing file is no
problem and exits `0` — also with a service named, because the first step is then `init`, not
another name. An `unavailable` access does not change the exit code of
`check` or `show` either — it is a state, reported as a warning by the health check, not a
problem of the file. For `test`: `0` when every tested access is `ok` (skipped ones do not
count), `1` when at least one `failed`, `2` usage error, including an unknown service.

### JSON

`--json` is for readers that must not parse text: an AI session and the interface. `check
--json` prints this, here for the file of the *Format* section without problems:

```json
{
  "file": "/home/alice/.config/k-playbook/github.yaml",
  "present": true,
  "services": {
    "website": {
      "repo": "acme/website",
      "access": [
        { "type": "gh", "login": "alice", "state": "available" }
      ],
      "selected": {
        "read":  { "git": 0, "api": 0 },
        "write": { "git": 0, "api": 0 }
      }
    },
    "backend": {
      "repo": "acme-labs/backend",
      "access": [
        { "type": "ssh", "key": "/home/alice/.ssh/id_ed25519_acme_labs", "state": "unavailable",
          "reason": "Die Schlüsseldatei fehlt." },
        { "type": "token", "file": "/home/alice/.config/github-tokens/acme-labs.token", "disabled": true,
          "state": "disabled" }
      ],
      "selected": {
        "read":  { "git": null, "api": null },
        "write": { "git": null, "api": null }
      }
    },
    "playbook": {
      "repo": "example/k-playbook",
      "access": [
        { "type": "ssh", "host": "github-example", "state": "available" },
        { "type": "public", "state": "available" }
      ],
      "selected": {
        "read":  { "git": 0, "api": null },
        "write": { "git": 0, "api": null }
      }
    }
  },
  "problems": []
}
```

`state` and `reason` are the availability of step 2; a binary of step 1 does not print them.
`reason` is present only for `unavailable`. `selected` is the choice of `run` for the service
(Task 118); a binary before it does not print it, and a reader that does not know it can ignore
it.

And this for a file with one problem — configuration or problems, never both:

```json
{
  "file": "/home/alice/.config/k-playbook/github.yaml",
  "present": true,
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
- `present` says whether something is at that path. Without a file it is `false`, `services`
  is `{}`, `problems` is `[]`, and `hint` carries *keine Zugangsdatei — k-playbook github init
  legt sie an*; `hint` is absent otherwise:

  ```json
  {
    "file": "/home/alice/.config/k-playbook/github.yaml",
    "present": false,
    "hint": "keine Zugangsdatei — k-playbook github init legt sie an",
    "services": {},
    "problems": []
  }
  ```

- `services` mirrors the file: the same keys, `~` expanded, no comments, `disabled` only
  where it is `true`. Services are in file order; the order of `access` is the precedence.
  As soon as one problem has level `error`, `services` is `{}`.
- `selected` is, per service, the access `run` would choose, per channel, as the index into
  that service's `access`: `read` is `run` without options, `write` is `run --write`, each with
  `git` and `api`; `null` means the channel has no access. It comes from the same rule as `run`
  (*Choosing the accesses*): the same function, applied to the availability after `state` was
  judged, so it can never differ from what `run` does. For `read` and `write` separately: both
  channels are `null` exactly when `run` in that form chooses no access and ends with „kein
  verwendbarer Zugang für <service>“ and exit `125`. The two may differ: a service with only
  `public` has `git` in `read`, and `write` is `null` in both channels. Other failures that end
  `run` at its start — the token cannot be read, the environment cannot be built, the command is
  not found — are not predicted by `selected`. Combinations with `--use` and `--skip` are not
  listed; a script that uses them applies them itself. `selected` is present wherever the
  availability was judged: `check`, `show`, `test`, `GET /api/github-access`; with a problem of
  level `error`, `services` is `{}` and there is no `selected`.
- `problems` has `level` (always `error`; the reader has no other level), `path` (where in the file, `services.<name>` or
  `services.<name>.access[<index>]`, `file` for the file as a whole), `message` and `hint`.
  An entry of the health check has `level`, `area`, `message` and `hint` but no `path`; the
  health source `github` therefore puts the path in front of the message
  (`services.backend.access[1]: Der Pfad in file ist relativ.`) and sets `area` to `github`.
  Messages and hints are German, like every other text k-playbook shows to people.

`show --json` prints the same object without `problems`; with a service named, `services`
holds only that one. Without a file it is `{file, present: false, hint, services: {}}`. Text and JSON carry the same information — nothing is only in one of
them.

`test --json` prints the file, the services tested and one result per access:

```json
{
  "file": "/home/alice/.config/k-playbook/github.yaml",
  "present": true,
  "services": {
    "backend": {
      "repo": "acme-labs/backend",
      "access": [
        { "type": "ssh", "key": "/home/alice/.ssh/id_ed25519_acme_labs",
          "result": "ok", "identity": "alice", "read": "yes", "write": "unknown", "expires": "none" },
        { "type": "token", "file": "/home/alice/.config/github-tokens/acme-labs.token", "disabled": true,
          "result": "skipped", "message": "deaktiviert" }
      ],
      "selected":   { "read": { "git": 0, "api": null }, "write": { "git": 0, "api": null } },
      "selectedOk": { "read": { "git": true, "api": null }, "write": { "git": true, "api": null } }
    },
    "playbook": {
      "repo": "example/k-playbook",
      "access": [
        { "type": "ssh", "host": "github-example",
          "result": "failed", "message": "git@github.com: Permission denied (publickey)." },
        { "type": "public", "result": "ok", "identity": "anonymous", "read": "yes", "write": "no", "expires": "none" }
      ],
      "selected":   { "read": { "git": 0, "api": null }, "write": { "git": 0, "api": null } },
      "selectedOk": { "read": { "git": false, "api": null }, "write": { "git": false, "api": null } }
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
- `present` is `false` only without a file: then nothing was tested, `services` is `{}`, `ok`
  is `true`, and `hint` carries the sentence about `init`, as in `check --json`.
- `selected` is the same as in `check --json`: `test` does not choose anew. It is taken from the
  availability before the test, without the network, never from the results — in the example
  `playbook` keeps the `ssh` access although it `failed` and `public` is `ok`, because that is
  what `run` would take.
- `selectedOk` has the form of `selected` and says per channel whether the chosen access passed
  the test: `true` for `ok`, `false` for `failed` or `skipped`, `null` where `selected` is
  `null`. It is only in `test --json`.

The health check (Task 098) calls the same reader and lists its problems under the area
`github`: structural problems as `error`, a service without an available access as `warning`
(see *Availability*). An empty file produces nothing, and neither does a missing one — the
health check never creates it. A session therefore sees a broken or unusable file in its
briefing, and nothing when it does not use the file.

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

A script or a session that wants to know the choice without running anything reads `selected`
from `check --json`, `show --json` or `test --json` (*JSON*): the same function computes it, for
`run` without options and for `run --write`.

There is **no automatic fallback**. `run` starts the command exactly once. If it fails, its exit
code is returned and nothing else happens: `run` sees only an exit code and cannot tell a
rejected credential from a failing command, and repeating `gh pr create` or `gh pr merge` with
another access could create or merge twice. A caller that wants another access says so —
`--skip ssh`, `--use token` — and an AI session asks `k_playbook_github_test` first.

### What is set up

Everything is set up in the environment of the command only. No file is written — when the
file is missing, `run` ends with `125` and names `k-playbook github init` (see *When the file is
missing*) —, no git configuration outside that process changes, `gh auth switch` is never called. Existing
`GIT_CONFIG_COUNT` entries of the caller are kept; `run` appends its own after them.

Every access chosen for `git` has a target form, the URL git uses for the repository under
`run`; every form of the repository is rewritten to it (see the rules below). The accesses
differ only in the target form and in their own setup:

| Chosen for `git` | Target form | Own setup |
|---|---|---|
| `token`, `gh` | `https://github.com/<owner>/<repo>` | Credential helpers of the machine are cleared for that URL, and a credential helper for it answers with the token. Clearing them keeps git from handing the token to a helper of the machine (`store` after a successful login), which would keep it outside the process. |
| `ssh` with `key` | `git@github.com:<owner>/<repo>` | `GIT_SSH_COMMAND="ssh -i <key> -o IdentitiesOnly=yes"`. |
| `ssh` with `host` | `git@<host>:<owner>/<repo>` | None; SSH takes the key from the alias. |
| `public` | `https://github.com/<owner>/<repo>` | Credential helpers are cleared for that URL, so no other credential of the machine is used. |

| Chosen for `api` | Setup |
|---|---|
| `token`, `gh` | `GH_TOKEN` and `GITHUB_TOKEN` are set to the token. |
| none | `GH_TOKEN` and `GITHUB_TOKEN` are removed from the child's environment, so the command does not quietly use a token the caller happened to have. |

Rules:

- **Only this repository.** Rewrites and the credential helper are bound to the URLs of the
  service's repository: `https://github.com/<owner>/<repo>`, `git@github.com:<owner>/<repo>`,
  `ssh://git@github.com/<owner>/<repo>`, each with and without `.git`, and the `git@<host>:` form
  of every `host` alias the service itself lists, the aliases of disabled accesses included.
  Other repositories in the same command — submodules, a second clone — are left as the
  machine has them. git matches `insteadOf` by prefix; the form without `.git` therefore also
  catches a repository whose name only starts with the same text (`acme/web` and
  `acme/website`). That is accepted: a command run under one service rarely touches such a
  neighbour, and the form with `.git` is exact.
- **Every form goes to the target form.** For every access, each form listed under *Only this
  repository* is rewritten to the target form of the chosen access. Which forms are rewritten
  does not depend on the type; only the target form does. A script or a clone that writes the
  URL in another form — `ssh://`, or the alias of another `host` access of the same service —
  therefore still goes through the access the line on standard error names.
- **The target form is pinned.** The target form is also rewritten to itself, with and without
  `.git`, so a rewrite of the machine cannot take it elsewhere. git applies the longest matching
  prefix, and the repository's own URL is longer than a rule for all of `github.com`, such as
  `url."git@github.com:".insteadOf https://github.com/`.
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
| `125` | `run` itself failed: usage error, no file (the message names `k-playbook github init`), unknown service (the message names the known services), a file with problems, no usable access in either channel, the token could not be read |
| `126` | the command was found but cannot be executed |
| `127` | the command was not found |

Every failure of `run` itself is one German message on standard error with what to do, like the
problems of *Reading the file*; `k-playbook github check` names the reason per access.

## In the briefing

The briefing (`k_playbook_briefing`, `k-playbook briefing`) and `k-playbook context` carry the
field `context.github` (task 119), so that a session does not have to guess which service
reaches GitHub for the repository it works in. Same source, same choice as `show`: `Path`,
`Load` (which never writes), `Judge` with `selected`. There is no rule of its own.

```json
"github": {
  "file": "/home/alice/.config/k-playbook/github.yaml",
  "present": true,
  "problems": [],
  "remote": "git@github-example:acme-labs/backend.git",
  "repo": "acme-labs/backend",
  "projectServices": ["backend"],
  "services": { "backend": { "repo": "acme-labs/backend", "access": [ … ], "selected": { … } } }
}
```

- `file`, `present`: the path after *Names* and whether the file is there. The briefing
  creates nothing — neither the field nor the environment check that runs before it (*When the
  file is missing*). Without a file the field shows `present: false` and the line reads
  `GitHub: keine Zugangsdatei`. A path occupied by something else — a symbolic link whose
  target does not exist, a directory where the file belongs, or a file where its parent
  directory belongs — is not "missing": the field reports `present: true` with an `error`, and
  the line says that the file has errors.
- `problems`: the problems of *Reading the file*, in the form of `check --json`. With an
  `error`, `services` is `{}` and `projectServices` `[]`.
- `remote`, `repo`: the remote that counts is the upstream remote of the current branch,
  otherwise `origin`, read raw from `remote.<name>.url` — `git remote get-url` would apply
  `insteadOf`, including the one of `run`. git looks for the repository from the directory of
  the call (the working directory, or `projectDir` of the MCP tool), so a repository of its own
  below the project gets its own remote. `repo` is `<owner>/<repo>` for the forms `run`
  redirects: `https://github.com/…`, `git@github.com:…`, `ssh://git@github.com/…` and
  `git@<alias>:…`, each with or without `.git`; an alias counts when a service of the file
  names it as `host` (no subprocess) or, otherwise, when `ssh -G <alias>` resolves to
  github.com. Any other host or form leaves both empty. `remote` carries no credentials: in a
  form with a scheme (`https://`, `ssh://`) everything before the `@` is dropped; the scp form
  stays as it is, it cannot carry a password.
- `projectServices`: the services whose `repo` equals `repo`, case-insensitively, in file order
  — usually one.
- `services`: the same object as `show --json`, with `state`, `reason` and `selected`.

No network, no token value; the field writes nothing. The cost beyond `context.gh` is `Judge` (gh's
configuration, file checks, `ssh -G` per `host`, 3 s limit each), three local `git` calls (four when the
upstream remote carries no URL and `origin` is read as well) and at most one more `ssh -G` for a remote alias the file does not name. That is why `ContextForDir`
fills the field and `BuildContext` does not: the interface and the review selection over MCP do
not pay for it.

In the plain text of `k-playbook briefing` the field is one line right after `Projektdoku:`,
built from the same parts as the `run:` lines of `show`, for example
`GitHub: dieses Repo = Dienst backend — run: git über ssh host github-example, api über gh alice`;
otherwise `GitHub: kein Dienst für acme-labs/foo (Dienste: …)`,
`GitHub: kein GitHub-Remote erkannt (Dienste: …)`
or, with several matching services, all of them; without a file
`GitHub: keine Zugangsdatei`, and with an `error` in the file
`GitHub: die Zugangsdatei hat Fehler — k-playbook github check nennt sie`. The rest behind the
separator shows `github`
without `services`. The field is the state when the briefing was loaded; `k-playbook github
show` gives the current one.

## Relation to other pages

- `local-github-ssh.md` describes one machine's SSH aliases and deploy keys by hand. That
  machine's services are in its `github.yaml` since step 1; the page is absorbed into this
  one once `test` can tell whether those aliases work.
- `mcp.md` holds the contracts of `k_playbook_health` and `k_playbook_github_test`; this page
  names the tool and says what it is for.

## Outlook

Not part of the contract yet; each will be specified here before it is built.

- **Choosing an access for a caller**: answered by `selected` (Task 118) — no verb of its own.
  `check`, `show` and `test` name per service the access `run` would choose, with and without
  `--write`; `test` adds whether it works (`selectedOk`). Requested from OMNI on 2026-11-02.
- **Reverse lookup**: from a repository's remote to the service that names it — for the
  briefing answered by `context.github.projectServices` (task 119, *In the briefing*); a verb
  of its own is not specified.
- **Named operations** with project rules, as CLI and MCP — the way AI sessions reach GitHub,
  since `run` is not offered to them: first the reading ones (fetch with status, list of pull
  requests), then pushing a feature branch and a draft pull request, each with approval, merging
  last.
- **A shared, versioned part** for projects where every person uses the same repositories
  with different accounts.
- **Containers**: `~/.config/k-playbook` mounted read-only; what `init` does there is
  described above, the mount itself is the project's business.
