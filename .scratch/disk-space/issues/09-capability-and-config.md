# 09 — Capability and configuration

Status: ready-for-agent

## What

Every Candidate this process may not act on is still shown, marked in `mute`, and carries no tick box; its reason and the concrete way to enable it live in a tooltip on the row. A Category whose tool is absent reports that as its own state in `mute` with what to do about it, the Plugin never fails in a block because one Scan Root is unreadable, and every threshold and path comes from the Dashboard's per-widget `config` — falling back to the documented defaults, in the card, when that config is malformed, of the wrong type, or names a Scan Root that is not on this machine.

## Why

This is D5 and D7's honest-failure half. The panel runs as `starpanel` (uid 10001), not root, so "you cannot touch this" is the normal case, not an exception: without this ticket an untickable Candidate looks like a broken control, a missing `docker` looks like an empty Category, and one unreadable Scan Root takes the whole Plugin down. D5 also fixes the wording — "cannot touch this" is `mute`, and `--danger` stays reserved for things that actually failed.

## Acceptance

- [ ] A Candidate with `capability.ok: false` renders with **no tick box at all** — not a disabled one, not a faded one — painted in `var(--mute)`
- [ ] Its tooltip (hover and keyboard focus) names the failed syscall in words, e.g. `EACCES deleting /var/log/syslog`, plus one concrete remedy sentence; the same words are reachable without a pointer
- [ ] A Category whose tool is absent reports its own state in `mute` with the reason and what to do (`docker` is not on `PATH`, `/var/run/docker.sock` is not mounted or not readable by uid 10001), and the panel installs nothing
- [ ] `manifest.json` declares `requires: [{"command": "docker"}]`, so the Plugin listing and the answer to its import say the same thing before the Widget is ever added (`docs/PLUGINS.md`, `apps/api/server.go:310-313`)
- [ ] One unreadable Scan Root never fails a block: the card and the dialog still render, that root is reported in `mute` with its errno, and the other roots answer normally
- [ ] The effective config is the eight keys [spec §8](../spec.md) fixes — `scanRoots`, `exclude`, `logDays`, `cacheDays`, `tempDays`, `largeFileBytes`, `roots`, `warnPercent` — and each falls back to its documented default when missing
- [ ] Malformed `config`, a key of the wrong type, and a configured Scan Root that does not exist are each reported in one `mute` line in the card in words, and the Plugin keeps working on the defaults
- [ ] `GET <proxy>/config` echoes the effective values, which default each one took, and the `notes[]` the Plugin has for the owner
- [ ] The widget passes its `config` to the backend on every request; the backend remembers nothing between requests
- [ ] Nothing in this ticket installs, downloads, chmods, mounts, escalates or elevates: there is no code path that changes the machine (D5, `docs/PLUGINS.md`: "The panel diagnoses; it never installs")

## Notes

### What this ticket owns, and what it inherits

Ticket 03 builds the index and fixes the wire shape on `GET /summary` and `GET /candidates`: `capability` on a Candidate and on a Category, `selectable` on a Candidate, `tickable` and `reviewOnly` on a Category, ids, fingerprints and `warnings`. This ticket does **not** re-open that shape — it fills it: the per-Candidate `capability` that ticket 03 leaves as `{"ok": true}` until a file Category asks, the Category-level `capability` for `docker` and the socket that tickets 05 and 06 need reported, and the `Effective(config) (Settings, []string)` seam ticket 03 leaves behind so `/summary` and `/candidates` can send the notes to the widget. If the two tickets disagree about a field, spec §3 is the tiebreaker and this ticket is the one that changes.

Ticket 04 renders all of it. The row without a tick box, the `mute` styling and the tooltip are ticket 04's markup; the data those depend on, and the guarantee that a Candidate without Capability is never offered, are decided here.

### Capability: the shape the backend reports

`capability` sits on a Candidate **and** on a Category, with the same shape:

```json
{
  "ok": false,
  "kind": "fs-permission",
  "errno": "EACCES",
  "syscall": "unlink",
  "attempted": "deleting /var/log/syslog",
  "words": "EACCES deleting /var/log/syslog",
  "remedy": "the panel runs as uid 10001 and /var/log is owned by root. Mount the host's /var/log into the panel, or add uid 10001 to the group that owns it."
}
```

`kind` is one of `ok`, `fs-permission`, `tool-missing`, `socket-missing`, `socket-permission`, `not-a-directory`, `not-found`, `read-only-filesystem`. `words` is what the row's tooltip shows as its first line; `remedy` is its second. Both are plain English, and `words` always contains the errno spelled the way a person would say it (`EACCES`, `EROFS`, `ENOENT`) followed by the syscall in words. `ok: true` carries `{"ok": true}` and nothing else. The `errno`, `syscall` and `attempted` fields are additive to spec §3's example, not a change to it: the spec shows the two the UI must have (`words`, `remedy`) and these three are what a person pastes into a search bar when they want the man page.

For a Category, `capability` describes the whole recipe: `tool-missing` with `words: "the docker command is not on PATH"` and `remedy: "install the docker CLI in the image, or run the panel where it exists — the panel does not install anything"`; `socket-missing` with `words: "/var/run/docker.sock is not there"`; `socket-permission` with `words: "EACCES connecting to /var/run/docker.sock"` and the group remedy below.

How capability is established, per Category:

- **File Categories** (`rotated-logs`, `package-caches`, `temp-files`): `syscall.Access(path, syscall.W_OK)` on the Candidate at measure time, plus a check on its parent directory (`W_OK|X_OK`), because deleting needs the directory, not the file. The `unlink` in the shape above is the syscall the answer is predicted from; the actual failed syscall at apply time replaces it when a `Dry Run` or `Remove` really fails.
- **`docker-images` / `docker-volumes`**: a version probe (`docker version --format '{{.Server.Version}}'`) with a 3s timeout decides between `tool-missing`, `socket-missing`, `socket-permission` and `ok`. Never `docker info` and never a retry loop: the panel polls, and a probe that blocks a poll is worse than a stale answer. Probe at most once per 30s and cache the verdict in memory only.
- **`large-files`**: always `ok` for measuring, and always tick-less (D10) — a Candidate not offered for removal is not a Capability question, so its `capability` stays `{"ok": true}` and the widget decides the tick box from the Category's `tickable` flag.

`tickable` is a separate boolean on the Category, and the two are not the same thing:

| Category | `tickable` | Why |
| --- | --- | --- |
| `docker-images` | true | the CLI owns the rules (D6) |
| `docker-volumes` | true, and rendered in `var(--danger)` with a typed confirmation (D11) | a volume is data, not junk |
| `rotated-logs` | true | file Candidates with a fingerprint (D8) |
| `package-caches` | true | file Candidates with a fingerprint |
| `temp-files` | true | file Candidates with a fingerprint |
| `large-files` | **false** | it is big, not dead (D10) |

The widget's rule, in one line: **a tick box exists only when `category.tickable && candidate.selectable && candidate.capability.ok`.** The two negatives are different facts and both are needed — `selectable: false` (and `tickable: false` on the Category) is "this Plugin will never offer to remove it", which is `large-files` and nothing else (D10), and such a row carries no `id` because it is not a Candidate; `capability.ok: false` is "it would, but this process may not", and that row keeps its `id`. `--danger` paints a real failure (an apply that came back non-zero, a backend that is down); `--danger` is never painted for a Candidate the owner may not touch, and never for a Category state.

### The tooltip: reachable, and never colour alone

`title` alone is not enough (a coarse pointer, a screen reader and a keyboard all read differently). Each row is a `button`-less structure with the path in `body` and, when capability is not `ok`, a following line in `mute` at `var(--text-meta)` that is the tooltip text — so the words are always in the DOM. Add `title` as a pointer convenience, an `aria-describedby` pointing at that line, and `tabindex="0"` on the row so focus reaches it. DESIGN.md §7: nothing is communicated by colour alone, and disabled means `mute` text on `surface-soft` — never a dimmed accent, never a faded copy of an enabled control.

The remedy sentences, verbatim, because they are the whole point of the ticket:

- `fs-permission`, file Candidate: `the panel runs as uid 10001 and <dir> is owned by <owner>. Mount that host path into the panel, run the container as that user, or add uid 10001 to the group that owns it.`
- `tool-missing`: `the <tool> command is not on PATH. Install it in the image, or run the panel where it exists — the panel does not install anything.`
- `socket-missing`: `<socket> is not there. Mount the host's <parent> into the panel.`
- `socket-permission`: `EACCES connecting to <socket>. The socket is owned by group <gid> with mode <mode>; add uid 10001 to that group, or point the panel at a socket it may use.`
- `read-only-filesystem`: `<path> is on a read-only mount (<mount>). Remount it read-write, or exclude it from the Scan Roots.`
- `not-a-directory` / `not-found`: one sentence each, in the same register, saying what was asked for and what was found. A Scan Root that does not exist is **not** one of these: it never fails a Candidate, it is reported as `exists: false` in `GET /config` and in the card's `mute` line (below).

D5 spells out the three remedies a person actually has (`mount this host path into the panel`, `run the container as that user`, `add the group`), and every remedy above has to be one of those plus the honest option of removing the path from the config. The docker group case is the one worth spelling out in the README: a socket bind-mount arrives in the container owned by `root:docker` with mode `0660`, so uid 10001 is refused unless the group id matches an entry the container user is in — a `group_add` in `compose` or a matching `--group-add`, never `--privileged` and never `user: root` (ticket 10, D5).

### Configuration: the eight keys, and their defaults

The keys are fixed by [spec §8](../spec.md); this ticket is where the defaults, the validation, the clamping and the notes are implemented, behind the single `Effective(config) (Settings, []string)` seam ticket 03 leaves in place (`Effective(config) Settings` there, with the notes added here). Written in the Dashboard's per-widget `config` JSON (`apps/api/internal/dashboard/dashboard.go:21`, `json.RawMessage`, free-form by design) and documented in the Plugin's own README:

```json
{
  "scanRoots": ["/var/log", "/var/cache", "/tmp", "/var/tmp", "/var/lib/docker", "/var/crash"],
  "exclude": ["/var/log/star-panel"],
  "logDays": 30,
  "cacheDays": 30,
  "tempDays": 7,
  "largeFileBytes": 1073741824,
  "roots": ["/var/log", "/var/cache", "/tmp"],
  "warnPercent": 90
}
```

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `scanRoots` | string[] | `["/var/log", "/var/cache", "/tmp", "/var/tmp", "/var/lib/docker", "/var/crash"]` | the directories the Plugin may look at — a Scan Root is the only part of the machine it sees (CONTEXT.md), and these are the places waste lives (D14, spec §8) |
| `exclude` | string[] | `[]` | glob patterns, matched against the root-relative path, that are never measured and never Candidates |
| `logDays` | int, days | `30` | age that qualifies a rotated log (D9) |
| `cacheDays` | int, days | `30` | age that qualifies a package-cache entry (D9) |
| `tempDays` | int, days | `7` | age that qualifies a temp file (D9) |
| `largeFileBytes` | int, bytes | `1073741824` (1 GB) | size that qualifies a large file (D10) |
| `roots` | string[] | the effective `scanRoots` | what the on-demand full walk covers by default; an entry must itself be inside a Scan Root (ticket 08, D14) |
| `warnPercent` | int, percent | `90` | the fill level at which the card says a Disk is nearly full |

Three deliberate readings of the spec's example, each worth confirming while implementing:

- **`/var/crash` is added to the `scanRoots` default.** D9 and spec §5 both make `/var/crash` part of `temp-files`, and a Category can only emit a Candidate inside a Scan Root (spec §7), so a default that omits it would ship a documented rule that can never fire. It is one more place waste lives, which is what the default is.
- **`roots` defaults to `scanRoots`, not to `["/"]`.** spec §8's example shows `["/"]`, and `["/"]` is a legitimate value an owner may write — but as a *default* it means the on-demand walk may reach the whole Disk while the Plugin's declared scope (and `large-files`, which measures under a Scan Root) stops at six directories. Deriving one from the other keeps the promise in spec §7 that no path outside a declared Scan Root is ever emitted, and adding `/` to `scanRoots` is a one-line edit for an owner who wants the whole machine in scope.
- **`exclude` is a glob against the root-relative path**, and it can only subtract from what a Scan Root already covers. `/var/log/star-panel` is the example because the panel's own logs are the one log directory whose files are certainly not dead.

Validation, in this order, per request:

1. `config` missing, `null`, not an object, or unparseable → every key takes its default, one note: `config could not be read; using the defaults`.
2. A present key of the wrong type (`scanRoots` as a string, `logDays` as `"30"`, `largeFileBytes` as a float with a fraction, `exclude` as an object) → that key takes its default, one note per key: `scanRoots was not a list of paths; using /var/log, /var/cache, /tmp, /var/tmp, /var/lib/docker, /var/crash`.
3. A number out of the range the recipe can mean (`logDays`, `cacheDays`, `tempDays` < 1; `largeFileBytes` < 0; `warnPercent` outside 1–100) → clamped, one note: `tempDays was 0; a temp file has to be at least a day old, so 1 was used`. `tempDays: 0` would otherwise make everything in `/tmp` a Candidate, which is the exact opposite of a conservative default, and a `warnPercent` of 0 would paint every Disk as nearly full.
4. `scanRoots` and `roots` entries that are not absolute, or contain `..`, are dropped with a note. A root that is a symlink is resolved once (`filepath.EvalSymlinks`) and the resolved path is what is reported, so the panel never names two things for one directory.
5. A `roots` entry that is not inside some `scanRoots` entry is dropped with a note — the full walk is explicit, but it still may not reach outside a declared Scan Root (ticket 08, D14, spec §7). The alternative — letting `roots` widen the Plugin's reach beyond `scanRoots` — was rejected: it would make `large-files` and the walk disagree about what the Plugin is allowed to see.
6. A Scan Root that is absolute and clean but **not on this machine** (`os.Stat` → `ENOENT`, `ENOTDIR`) is kept in the effective list and reported with `exists: false`; it is not dropped and it is not a failure. A config written on one machine is read on another, and the honest answer is "that path is not here", not a shorter list. Same for one that exists but cannot be read: `exists: true, readable: false`.

The notes are just strings; the widget renders them. Rendering rule: if `notes.length > 0`, the card shows exactly one extra line at `var(--text-meta)` in `var(--mute)`, `config: <notes joined by " · ">`, with `title` carrying the rest when they do not fit. One line, not a list, and never `--danger`: a config typo is not a failure of the machine, it is the panel saying what it did instead (DESIGN.md §4: in view mode a card shows a title, its body and at most one muted metadata line). Two keys need one extra sentence each in the widget, because a person cannot see them otherwise:

- a Scan Root with `exists: false` → one `mute` line per missing root: `/mnt/data is not on this machine`, capped at two lines so a config full of stale paths cannot push the card's body off the panel (ticket 02 already caps `warnings` this way).
- `warnPercent` is the one key the existing contract suite already exercises with a lowered value (`apps/web/tests/plugins.contract.test.mjs:240-263`, under the old `disk-usage` name — see ticket 10), and the card must still say "nearly full" **in words** when a Disk reaches it.

### `GET <proxy>/config`

The endpoint that makes all of the above inspectable, and the one place an implementer can see which default a key took:

```json
{
  "effective": {
    "scanRoots": ["/var/log", "/var/cache", "/tmp", "/var/tmp", "/var/lib/docker", "/var/crash"],
    "exclude": ["/var/log/star-panel"],
    "logDays": 30, "cacheDays": 30, "tempDays": 7,
    "largeFileBytes": 1073741824, "roots": ["/var/log", "/var/cache", "/tmp"], "warnPercent": 90
  },
  "sources": { "logDays": "config", "cacheDays": "default", "tempDays": "default",
               "largeFileBytes": "default", "roots": "default", "warnPercent": "default",
               "scanRoots": "default", "exclude": "default" },
  "scanRoots": [ { "path": "/var/log", "exists": true, "readable": true },
                 { "path": "/mnt/data", "exists": false, "readable": false,
                   "words": "/mnt/data is not on this machine" } ],
  "categories": [ { "id": "docker-images", "tickable": true, "estimate": true,
                    "capability": { "ok": false, "kind": "tool-missing",
                                    "words": "the docker command is not on PATH",
                                    "remedy": "install the docker CLI in the image, or run the panel where it exists — the panel does not install anything." } } ],
  "notes": ["tempDays was not a number; using 7"],
  "installs": false
}
```

`sources` values are `config`, `default`, `clamped` or `dropped`. `installs` is always `false` and exists so that "the panel never installs anything" is answerable as data and not only as documentation (D5, `docs/PLUGINS.md`).

### How the widget's config reaches the backend

Plainly: **the widget passes it, on every request, and the backend stores nothing.** Core does not inject it and cannot: the proxy in `apps/api/internal/plugins/backend.go:102` is a plain `httputil.NewSingleHostReverseProxy` that forwards the request as the browser sent it — the backend sees the Dashboard's `config` only because the widget puts it in the request. The widget has it in `ctx.config` (`apps/web/src/WidgetCard.svelte:51-56`).

The convention, so every route agrees:

- Reads (`GET <proxy>/summary`, `GET <proxy>/candidates`, `GET <proxy>/config`, `GET <proxy>/journal`) take the config as a query parameter: `?config=` plus `encodeURIComponent(JSON.stringify(ctx.config))`. That is what keeps the common case a plain GET, and it is why `GET summary` can carry an owner's thresholds without core knowing anything about them.
- Writes and anything with a list (`POST <proxy>/plan`, `POST <proxy>/plan/{planId}/dry-run`, `POST <proxy>/plan/{planId}/apply`, `POST <proxy>/walk`) carry it in the JSON body as a `config` field beside the operation's own fields.
- The backend parses and validates the config on every request, derives the effective config, and answers with the `notes` it produced. A read path costs a few `stat`s and one small JSON parse; the hot index is what is cached, not the config.

Rejected alternative, with the reason it was rejected: the backend could remember the config from the first request that carried it. That is wrong here for three concrete reasons — a first request that arrives without a config would silently set defaults for every later one; two Widgets of the same Plugin with different configs would fight over one memory; and the backend is restarted every 2 seconds when it dies (`apps/api/internal/plugins/backend.go:21`), so the memory would be lost exactly when the owner was most likely to be reconfiguring things. Passing the config is also what makes D8's revalidation honest: whether a Candidate may be acted on is decided against the config in hand, not against a copy from an earlier request.

### Traps

- **A long `config` belongs in the body, not the query string.** `ctx.fetch(path, init?)` now takes an `init` ([spec §9.1](../spec.md)), so a POST carries the config in its JSON body; the GETs take `?config=` because that is the shape a read has. Do not squeeze a list of roots into a URL: it is a header-size limit away from a request that fails for a reason nobody can read.
- **A wrong-type key must not be silently coerced.** `"30"` is not `30`, and `logDays: "30"` taking the default with a note is the answer; `json.Unmarshal` into a struct with `int` fields will fail the *whole* object, which is why the parse goes key by key through `map[string]json.RawMessage`.
- **The `notes` array is the contract, not a log.** It travels in the answer to the request that produced it, so the card can say what happened without the owner opening a console.
- **`exclude` never widens reach.** A pattern can only remove paths from inside a Scan Root; an entry that is itself outside every Scan Root is dropped with a note rather than becoming a new place to look.
- **Do not report a whole Category as failed for one Candidate.** A Category with three unreadable Candidates has three `mute` rows and a normal state; the Category-level `capability` is for the tool and the socket, which are all-or-nothing.
- **`/var/lib/docker` is measured and never edited** (D6): it is a Scan Root for measuring, its paths are never Candidates, and no `exclude`-shaped entry in the docs may suggest otherwise. If the owner's config omits it from `exclude`, the Plugin still refuses to make its contents file Candidates.
- **`Empty` is a state too.** A Category with `capability.ok: true` and no Candidates renders its empty state in `mute` ("nothing qualifies right now"), which is a different sentence from the tool-missing state. Both are designed states (DESIGN.md §3: every state is designed).
- **The `docker` requirement is declared, not probed away.** `requires: [{"command": "docker"}]` in `manifest.json` is what makes the Plugin listing carry the Phosphor warning in `danger` with the reason underneath (`apps/api/server.go:310-313`); the Category state above is the same fact reported where the Candidate list is, and the two must not contradict each other.
