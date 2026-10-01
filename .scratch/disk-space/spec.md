# Disk space and cleanup — spec

A **Plugin** for Star Panel that shows Disk usage in full detail and finds dead files,
recommends them, and removes the ones the owner ticks — with the owner supervising
every removal.

The vocabulary is [CONTEXT.md](../../CONTEXT.md) and it is not decoration: a
**Candidate** is something the Plugin measured, a **Selection** is a subset of that, and
a **Plan** is answerable as a **Dry Run** before it is ever applied. Those three
sentences are the whole safety story.

Every decision below was settled in a grilling session and is recorded, with its
rejected alternatives, in [DESIGN-NOTES.md](DESIGN-NOTES.md). Two of them are
architectural and live as ADRs: [ADR-0009](../../docs/adr/0009-disk-plugin-is-an-optional-folder-plugin.md)
(why a destructive tool is an optional folder Plugin) and
[ADR-0010](../../docs/adr/0010-every-plan-is-a-dry-run-first.md) (why the button never
removes on the first press).

## 1. What it is

One Plugin folder, `apps/api/plugins/disk-space/`, dropped into `plugins/` or imported
as a ZIP like any other. It declares **one Widget**, which the owner adds to their
Dashboard like any other, and that Widget's card carries the Disk bars, the total
Reclaimable and a `Clean…` button. The button opens a native `<dialog>` with everything
else: every Disk, every Category, every Candidate, and the two-step removal.

It is **not** a core built-in. It is optional, removable and replaceable — and because
of that, it does not get core's trust, so the safety of what it removes has to live
inside it.

## 2. Shape

```
apps/api/plugins/disk-space/
├── manifest.json          name, version, one widget, requires: docker
├── README.md              what it needs, the mounts, the user it must run as
├── widget.js              the card and the <dialog>, vanilla ESM
├── .archiveignore         state/ and dist/: what a Download must not carry
├── build.ps1              builds the linux/amd64 binary and the ZIP
├── backend                the compiled Go binary (linux/amd64), not in git
├── dist/                  the ZIP, not in git, never inside the ZIP
├── state/                 the journal, written at runtime, never in the ZIP
└── src/                   the Go source: package main, its own go.mod
    ├── go.mod             module star-panel/plugins/disk-space
    ├── main.go            the server, the port handshake, the routes
    ├── scan.go            the hot index over the Scan Roots
    ├── walk.go            the bounded, cancellable full walk
    ├── disk.go            statfs and the Disk listing
    ├── config.go          the eight keys, validated and defaulted
    ├── category.go        the Category registry, with one file per Category
    ├── plan.go            Plan, Dry Run, revalidation, apply
    └── journal.go         the append-only removal record
```

**The source folder is `src/`, not `backend/`, and that is forced.** The binary the
Manifest runs is `./backend` ([docs/PLUGINS.md:56](../../docs/PLUGINS.md#L56) uses exactly
that spelling, and `command[0]` must contain a separator for `exec` to resolve it inside
the folder rather than in `PATH`), so the file `backend` sits at the Plugin root — and a
file and a directory cannot share a name in one folder. `go build -C src -o ../backend .`
is the build, and the manifest keeps `backend.command: ["./backend"]`.

The backend is a supervised subprocess like any other Plugin backend: core allocates a
free port, passes it as `{port}` in the argv and as `STAR_PANEL_PORT`, and runs it with
the Plugin folder as its working directory. It listens on `127.0.0.1` only and answers
paths relative to `/api/v1/plugins/disk-space/proxy/`. It is built with the **standard
library only** — no Docker SDK, no CLI framework, no ORM, no database, no
`golang.org/x/sys` — and `CGO_ENABLED=0`, which is what makes one static binary run on
`alpine` ([ADR-0005](../../docs/adr/0005-go-backend-modules.md) sets the tone for core and
this Plugin follows it).

## 3. The HTTP surface

All paths are the backend's own; the panel reaches them under
`/api/v1/plugins/disk-space/proxy/`. Errors use the frozen `{"error": "..."}` shape that
core already speaks ([backend.go:112](../../apps/api/internal/plugins/backend.go#L112)).

Two conventions apply to every route and are not repeated below. **The widget sends its
own config with each request**, because core carries none to a backend: the reads take
`?config=` plus URL-encoded JSON, and also accept a `POST` with `{"config": …}` in the body
for a config too big for a query string; every mutation is a `POST` with the config in its
body ([§8](#8-capability-and-configuration)). And **every mutation is a `POST`** — never a
`GET` — with the widget verifying the method it actually used, because a silently ignored
`init` argument would turn an apply into a GET.

### `GET /health`

What core's restart loop and the widget's cold-start handling use to tell "backend
starting" from "backend broken".

```json
{ "status": "ok", "indexAge": "42s", "docker": "ok" }
```

### `GET /summary`

What the card polls. Cheap by construction: `statfs` per Disk plus the hot index, never
a walk.

```json
{
  "disks": [
    { "name": "/", "label": "root", "fs": "ext4", "mount": "/",
      "totalBytes": 500107862016,
      "usedBytes": 412316860416, "freeBytes": 87791001600, "usedPercent": 82.4 }
  ],
  "reclaimableBytes": 12884901888,
  "reclaimableIsEstimate": true,
  "categories": [
    { "id": "docker-images",  "candidates": 12, "bytes": 8589934592,
      "capability": { "ok": true }, "estimate": true },
    { "id": "docker-volumes", "candidates": 2,  "bytes": 2147483648,
      "capability": { "ok": false, "kind": "socket-permission", "errno": "EACCES",
                      "attempted": "connecting to /var/run/docker.sock",
                      "words": "EACCES connecting to /var/run/docker.sock",
                      "remedy": "add uid 10001 to the group that owns the socket" },
      "estimate": true }
  ],
  "indexAge": "42s",
  "warnings": ["/var/lib/docker is not mounted in this panel"]
}
```

`reclaimableIsEstimate` is always `true` when any contributing Category is a command
Category: Docker's `RECLAIMABLE` sums to more than the disk because layers are shared,
and the card is not allowed to present that as a promise (D6, D12).

**`name`, `label`, `fs` and `mount` are four different things and all four are
reported.** `name` is what a person calls it and what the existing contract test asserts
on (`C:`, `/`), `label` is the volume label when the platform has one, `fs` is the
filesystem, and `mount` is where it is mounted. On Windows `name` is the drive letter and
the source is `win32`; on Linux `name` is the mount point and the source is `procfs` or
`statfs`. The card renders `name · label · fs` and a footer naming the source and the
count, because that is what the suite at
[`apps/web/tests/plugins.contract.test.mjs`](../../apps/web/tests/plugins.contract.test.mjs)
already demands (see §9).

### `GET /candidates`

What the dialog fetches when it opens. Grouped by Category, every Candidate with what
it takes to decide and what it takes to remove.

```json
{
  "configWarning": "",
  "measuredAt": "2026-01-14T09:12:00Z",
  "categories": [
    {
      "id": "rotated-logs",
      "title": "Rotated logs",
      "rule": "/var/log *.log, *.gz, *.1-*.9 older than 30 days",
      "bytes": 1288490188,
      "estimate": false,
      "tickable": true,
      "capability": { "ok": true },
      "candidates": [
        { "id": "cl_9f2c1a4b7d3e", "path": "/var/log/nginx/access.log.3.gz",
          "bytes": 104857600, "reason": "rotated 41 days ago",
          "selectable": true, "capability": { "ok": true },
          "fingerprint": { "size": 104857600, "mtime": "2026-05-02T04:11:07Z", "inode": 917534 } }
      ]
    },
    {
      "id": "large-files",
      "title": "Large files",
      "rule": "over 1 GB under a Scan Root",
      "reviewOnly": true,
      "tickable": false,
      "bytes": 38654705664,
      "candidates": [
        { "id": null, "path": "/srv/backups/db-2026-01.dump",
          "bytes": 38654705664, "reason": "big, and only you know if it is dead",
          "selectable": false, "capability": { "ok": true },
          "note": "big, and not dead — this one is yours to judge" }
      ]
    }
  ]
}
```

The fields that carry the rules:

- `tickable: false` on a Category, and `selectable: false` on a Candidate — no tick box,
  ever. `large-files` is the whole of this in v1 (D10), and `reviewOnly: true` on its
  Category says so in one word.
- `capability.ok: false` — the process may not act on it; the row renders in `mute` with
  `words` and `remedy` in its tooltip and **no tick box** (D5). The object is the same
  shape on a Category and on a Candidate, so the dialog has one rule: *a tick box exists
  only when `category.tickable && candidate.selectable && candidate.capability.ok`*.
- `fingerprint` — what the apply step revalidates against (D8). Absent for command
  Candidates, because a Docker image cannot be fingerprinted.
- An **observation** is a row with `selectable: false` and **no `id`**: it was measured,
  it is shown, and it is not a Candidate (D10). A Candidate has an `id`, always.

An `id` is stable while the machine is unchanged, because the widget holds ids in a
Selection and a random id would invalidate it on every re-measure. A file Candidate's is
`"cl_" + hex(sha256(category + "\x00" + path))[:12]`; a command Candidate's is the argv it
stands for, `docker-images:dangling` or `docker-volumes:all`, which is stable for free and
readable in a journal. A Candidate whose `id` the backend cannot find in its own measured
set is not a Candidate — that is the rule that makes a `POST` body harmless (D2).

`configWarning` is the one string the card renders when the config was not usable, and
`measuredAt` is when the index behind this answer was built; a Docker figure older than
two index TTLs carries its age in words beside it.

### `POST /plan`

Body: `{ "selection": ["cl_9f2c1a4b7d3e", "docker-images:dangling"] }` — a **Selection**,
which is a subset of the Recommendation. A Selection with an unknown or non-selectable id
is refused whole (`400`), never partially honoured.

Answers a **Plan**:

```json
{
  "planId": "pln_3f9c2a41",
  "bytes": 1331691520,
  "estimate": false,
  "dryRunAt": "",
  "appliedAt": "",
  "steps": [
    { "entryId": "e1", "candidateId": "cl_9f2c1a4b7d3e",
      "path": "/var/log/nginx/access.log.3.gz",
      "bytes": 104857600, "kind": "file",
      "command": ["rm", "-f", "--", "/var/log/nginx/access.log.3.gz"] },
    { "entryId": "e2", "candidateId": "docker-images:all", "path": null,
      "bytes": 8589934592, "kind": "command", "estimate": true,
      "command": ["docker", "system", "prune", "-f"] }
  ]
}
```

`command` is an argv array and never a string: no shell, ever, so a filename with a space,
a quote or a leading `-` cannot become anything but one argument. The Dry Run prints that
array joined for reading, and the apply step executes exactly the array — the printed line
and the run line are the same bytes.

### `POST /plan/{planId}/dry-run` and `POST /plan/{planId}/apply`

The same Plan, two modes. `dry-run` answers it as a **Dry Run**: for a file, it revalidates
and reports; for a command, it runs the command's own dry-run form when there is one, and
otherwise reports the command without running it. `apply` **refuses with `409`** — body
`{"error": …, "code": "dry-run-required"}` — unless *this same Plan* has already been
answered by a dry run. The engine enforces it; the UI only reflects it (ADR-0010). An
apply on a consumed Plan is `409`; on an expired one (10 minutes) it is `410`.

`dry-run` answers:

```json
{
  "planId": "pln_3f9c2a41",
  "dryRun": true,
  "steps": [
    { "entryId": "e1", "outcome": "wouldRemove", "bytes": 104857600,
      "command": ["rm", "-f", "--", "/var/log/nginx/access.log.3.gz"] },
    { "entryId": "e2", "outcome": "wouldRun", "command": ["docker", "system", "prune", "-f"],
      "output": "Deleted Images:\nuntagged: sha256:…\n\nTotal reclaimed space: 8.0GB" }
  ]
}
```

`apply` answers one entry per Candidate, and **never fails whole because one entry did**:

```json
{
  "planId": "pln_3f9c2a41",
  "dryRun": false,
  "freedBytes": 1331691520,
  "reportedBytes": 8589934592,
  "steps": [
    { "entryId": "e1", "outcome": "removed", "bytes": 104857600 },
    { "entryId": "e2", "outcome": "removed", "bytes": 8589934592,
      "output": "Total reclaimed space: 7.4GB" }
  ],
  "skipped": [
    { "entryId": "e3", "outcome": "changed",
      "reason": "size changed from 12.4 MB to 88.1 MB since it was measured" }
  ]
}
```

Outcomes: `removed`, `wouldRemove`, `wouldRun`, `changed`, `gone`, `denied`, `failed`,
`overran`. Every one of them is shown in words; none of them is a silent success.

### `POST /walk`

The full walk (D14), streamed. Ticket 08 settles the shape and this is the summary of it:
`POST /walk` takes `{ "roots": [...], "crossFilesystems": false, "maxFiles": 200000,
"maxMillis": 300000, "config": {...} }` and answers `200
Content-Type: text/event-stream` — one JSON object per event, a blank line after each, a
`: keepalive` comment every 15s — with the events `progress` (`filesSeen`, `bytesSeen`,
`dirsSeen`, `elapsedMs`, `current`), `result` (the per-directory tree, the biggest files
with no tick box, the `unreadable` rows), `error`, and `done`. **A `POST` that streams is
deliberate**: it is a measurement, not a resource, and the answer is the walk. Closing the
stream cancels it — the backend stops its workers on `r.Context().Done()` — so an owner
who closes the dialog is not leaving a walk running behind it. One walk runs at a time and
a second start is `409`. An interrupted walk reports its cap or its cancellation in words,
never as a complete measurement, and nothing reaches the card from here.

### `GET /journal?limit=50`

The append-only record of every removal, newest first, for the dialog's History pane.

## 4. The card and the dialog

**The card** is framed by the Dashboard: title, border, surface and padding belong to
it, and the Widget paints inside. It shows one row per Disk — `name · label · fs`, a bar,
and `usedPercent% · free` in tabular figures — then the Reclaimable total with its
estimate caveat in `mute`, then the `Clean…` button (the one accent on the surface). It
ends with one `mute` footer line naming the source and the count (`2 volumes · source:
win32`), and any `warnings` as at most one more `mute` line, because `DESIGN.md` §4 gives
a card a title, a body and at most one muted metadata line. It marks a Disk "nearly full"
**in words** once `usedPercent` reaches `warnPercent` (default 90) and paints that row in
`var(--danger)`, because `DESIGN.md` §3 forbids communicating anything by colour alone.
It polls `/summary` on the Dashboard's `pollSeconds`. On a cold backend (the 3s
`startWaitTimeout` after a restart) it shows `mute` placeholder text, never a spinner and
never an error flash. A failed poll says `Could not read the disks` and names the status
that failed — that sentence and that shape are already asserted by the suite (§9).

**The dialog** is a `<dialog>` the Widget creates and opens with `showModal()`. A
native dialog is the only overlay that survives an ancestor with a `transform`, which
`svelte-dnd-action` leaves on the grid item while arranging (D4); it also brings focus
trapping, `Esc`, `::backdrop` and inertness without a line of our own. It paints
`var(--surface)`, `var(--radius-lg)` and the one shadow the system allows, and every
other value is a Token.

Its sections, top to bottom:

1. **Disks** — every Disk, with a bar, its filesystem, and used/free in tabular
   figures.
2. **Recommendation** — the total and the `Review` button. This is what builds the
   Selection.
3. **Categories** — a collapsible section each, headed by its title, its rule in
   `mute` and its bytes. Inside, one row per Candidate: tick box, path (in `font-mono`
   at `meta`), size, and reason. A row without Capability is `mute`, has no tick box,
   and carries `capability.words` and `capability.remedy` as a tooltip. `docker-volumes` renders in `var(--danger)`
   with the words that a volume is data and not junk. `large-files` renders as a
   "review this yourself" list with no tick boxes at all.
4. **The Plan** — after `Review`, the Dry Run's output: what would go, what command
   would run, and what came back. Only then does `Remove` appear.
5. **History** — the journal, newest first.

Rules it obeys because everything in `apps/web` does: no literal colour, no second
accent, at most one shadow, nothing animates on its own, every state is designed
(empty, loading, error), nothing is communicated by colour alone, and the whole thing
is reachable by keyboard.

## 5. Categories

A Category is a recipe the Plugin ships, never a rule the owner writes. The owner
adjusts thresholds and exclusions in the Widget's `config`; they do not add Categories.

| Category | Rule | Acts by |
| --- | --- | --- |
| `docker-images` | dangling images, stopped containers, unused networks, build cache | `docker system prune -f`, `docker image prune -f` |
| `docker-volumes` | volumes no container references | `docker volume prune -f` — separate Category, `danger` (D11) |
| `rotated-logs` | `/var/log` `*.log`, `*.gz`, `*.1`..`*.9` older than `logDays` | `rm` per file, revalidated |
| `rotated-logs` | systemd journal older than `logDays` | `journalctl --vacuum-time=<logDays>d` |
| `package-caches` | apt archives, pip, npm, pnpm, go build, cargo older than `cacheDays` | `rm` per file/dir, revalidated |
| `temp-files` | `/tmp`, `/var/tmp` older than `tempDays`; `/var/crash` | `rm` per file, revalidated |
| `large-files` | over `largeFileBytes` under a Scan Root | **nothing** — recommendation only (D10) |

Defaults: `logDays` 30, `cacheDays` 30, `tempDays` 7, `largeFileBytes` 1073741824.

Measured and never a Candidate: `/home`, `/etc`, `/usr`, `/var/lib/docker`, and
anything holding a database. Nothing inside `/var/lib/docker` is ever a file
Candidate — Docker's store is not ours to edit (D6).

**Old kernels are out of scope in v1.** `apt autoremove --purge` can remove the kernel
the machine booted from, and that is disproportionate power for a tick box on a server
reached over SSH. If they surface at all, they surface as large files.

## 6. Scanning

**The hot index** covers the declared Scan Roots only — the few places waste lives
(`/var/lib/docker` when mounted, `/var/log`, `/var/cache`, `/tmp`, `/var/tmp`). It is
built on the first request and refreshed every 10 minutes or when the config changes,
and it is what makes `/summary` and opening the dialog cheap.

**The full walk** is the expensive path and it is always explicit: asked for, streamed,
cancellable, capped at 200k files or 5 minutes. It never crosses filesystem boundaries
by default, skips symlinks, skips pseudo-filesystems, counts a hardlinked file once,
and survives `EACCES` on any subtree without failing as a whole.

**No history.** No stored scan-to-scan deltas, no "grew 2 GB since yesterday" in v1.

## 7. Safety

- **Everything is a Dry Run first**, enforced by the engine: `remove` is `409` on a
  Plan that was not previewed (ADR-0010).
- **A Plan only ever removes Candidates it measured.** The apply step looks each id up
  in its own index; an id it does not know is refused. Nothing in a request body is
  ever treated as a path.
- **Every file Candidate is fingerprinted** (size, `mtime`, inode) and revalidated at
  apply time. What changed is skipped and reported, not removed, and the Plan is not
  aborted wholesale.
- **Command Categories cannot be revalidated**, so their apply step reports what the
  CLI actually reclaimed against what the Dry Run predicted, and under-delivery is
  reported, never hidden.
- **`os.Remove` for files, `os.RemoveAll` only for a directory the Plugin itself
  enumerated as a Candidate** — never a pattern, never a glob, never a recursive walk
  at removal time.
- **The journal** in `state/journal.jsonl`: one append-only record per removal with the
  Candidate, the action, the result and the bytes. Never inside the ZIP (ticket 10).
- **Refused in code, not by convention**: `/`, `/home`, `/etc`, `/usr`, `/boot`,
  `/var/lib/docker` and any path that is not inside a declared Scan Root are refused as
  a Candidate, exercised by a test.

## 8. Capability and configuration

The panel runs as `starpanel` (uid 10001) and is not root, and the design never assumes
otherwise. A Candidate the process may not act on is shown in `mute` with a tooltip
carrying the failed syscall in words and the concrete way to change it — mount the host
path into the panel, run the container as that user, add the group. A Category whose
tool is missing (`docker` not on `PATH`, socket not mounted) says so in `mute`, and the
panel never installs anything. One unreadable Scan Root never fails the Plugin.

`--privileged` and `user: root` are rejected as the documented path: they buy
convenience with the whole host.

Configuration is per-Widget, in the Dashboard's `config` JSON, which already reaches
both the widget and the backend:

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

`scanRoots` defaults to the places waste actually lives (D14), not to `/`: a default that
points the hot index at every Disk is a default nobody intended. It includes `/var/crash`
because `temp-files` claims that rule, and a Category can only emit a Candidate inside a
declared Scan Root — a default that omitted it would ship a documented rule that can
never fire. `roots` defaults to the effective `scanRoots` rather than to `["/"]`, for the
same reason: the on-demand walk may not reach beyond what the Plugin declared. An owner
who wants the whole machine in scope adds `/` to `scanRoots`, which is one line.

`warnPercent` defaults to 90 and is the one pre-existing key — the contract suite already
exercises an owner lowering it to 50 (§9.2). With eight keys, `logDays`, `cacheDays`,
`tempDays`, `largeFileBytes` and `warnPercent` have numeric floors that are validated and
clamped with a note rather than silently honoured: a `tempDays` of 0 would make everything
in `/tmp` a Candidate, which is the exact opposite of a conservative default.

**The config travels with the request, not through core.** Core hands a backend nothing
but `STAR_PANEL_PORT` and `STAR_PANEL_PLUGIN`
([backend.go:186](../../apps/api/internal/plugins/backend.go#L186)) and the proxy forwards
the request as the browser sent it, so the only channel a Plugin has for its own
configuration is the request the widget makes. Reads (`/summary`, `/candidates`, `/config`,
`/journal`) accept it as `?config=` plus URL-encoded JSON, and also as a `POST` body for a
config too big for a query string; every mutation (`/plan`, its `dry-run` and `apply`, and
`/walk`) is a `POST` with it in the JSON body. The backend validates it on every request,
uses the effective values, and answers with the notes it produced. It stores nothing
between requests, which is also what keeps a restart from losing an owner's settings.

Malformed config falls back to the defaults and says so in the card, never with a `4xx`:
an owner who typed `"thirty"` into `logDays` gets the default and one `mute` line naming
it. `GET /config` echoes the effective values, which default each one took, and the notes,
so an implementer never has to guess.

### `GET /config`

The endpoint that makes the above inspectable: the effective values, which key took a
default, which was clamped or dropped, which Scan Roots exist on this machine, each
Category's `tickable` and `capability`, and the notes the owner should read. `installs`
is always `false`, so "the panel never installs anything" is answerable as data.

## 9. What core has to change

**Nothing, to work as designed** — but four things in core stand between this design and
a Plugin that actually runs, and three of them are pre-existing defects that turn out to
be load-bearing here. They are listed because they are deliverables, not surprises.

- **A built-in needs a folder.** `Backends.Handler` asks the registry first
  ([backend.go:80](../../apps/api/internal/plugins/backend.go#L80)) and the registry only
  knows folders ([registry.go:75](../../apps/api/internal/plugins/registry.go#L75)), so
  the `system-stats` built-in in `main.go` is currently unreachable — no
  `plugins/system-stats/` folder exists. That is a separate, pre-existing bug (§9.4); this
  Plugin avoids it by being a folder Plugin.

The Plugin declares `requires: [{"command": "docker"}, {"command": "./backend"}]`. The
panel already reports a missing requirement in its listing and in the answer to an
import, and the Docker Categories report their own state on top of that.

### 9.1 `ctx.fetch` now accepts a method and a body

Today it is `fetch: (path: string) => Promise<Response>`
([types.ts:67](../../apps/web/src/types.ts#L67),
[WidgetCard.svelte:54](../../apps/web/src/WidgetCard.svelte#L54)), which makes the widget
contract read-only: a Plugin can poll but cannot POST, and `POST /plan` — the entire Dry
Run guarantee — would be unreachable through the documented seam. The change is
`fetch: (path: string, init?: RequestInit) => Promise<Response>`, passing `init` straight
to `fetch`, and it is already made. It widens `WidgetContext`, the contract every Plugin
widget is written against: backwards compatible, and what any future Plugin with an action
in it needs.

**The trap that comes with it**: JavaScript ignores an extra argument. A widget that calls
`ctx.fetch("plan", { method: "POST", body })` against an *older core* silently issues a
`GET` — and a `GET /plan` that built a Plan, or worse a `GET` that applied one, is the
worst possible failure of this design. So the widget does not assume the second parameter
exists: it probes, and if the core in front of it is old it builds its own URL and calls
`fetch` directly, then verifies the method it actually used went through. Ticket 02 owns
the probe; ticket 07 owns never turning an apply into a GET.

### 9.2 The contract suite is already written for this Plugin

`apps/web/tests/plugins.contract.test.mjs` tests a Plugin called `disk-usage` — a manifest
with one widget and a backend binary, a widget that lists every volume as `name · label ·
fs` with `usedPercent% · free`, a `nearly full` state written in words and painted
`var(--danger)`, a `warnPercent` config key, the sentence `Could not read the disks` plus
the status when the backend fails, and `No volumes found` on an empty machine. **Those
assertions are the contract, not a suggestion**, and ticket 02 implements them. The folder
name in this design is `disk-space`, so `pnpm test` fails until ticket 10 renames the
cases and reconciles them with the richer answer `/summary` gives. Do not satisfy it by
creating a second folder. And one case in that suite is exactly wrong for this Plugin and
must be re-pointed rather than obeyed: it asserts the backend binary is in `requires` by
its bare name, where this design puts it in `backend.command` as `./backend` and checks it
at discovery instead (§9.3).

### 9.3 The exec bit: a ZIP-imported Plugin cannot start today

This is the defect that decides whether D13 works at all. Two things are wrong today:

- `writeEntry` unpacks **every** file `0o644` ([import.go:223](../../apps/api/internal/plugins/import.go#L223)),
  so importing a ZIP whose folder contains a compiled `backend` produces a file the kernel
  will not execute.
- `Entry.Archive()` writes every entry with `writer.Create`
  ([import.go:109](../../apps/api/internal/plugins/import.go#L109)), which stores mode
  `0644` in the ZIP — so a **Download** of a working Plugin produces an archive that
  cannot be imported back into a working one.

And the change that makes a broken binary visible instead of silent:

- `Entry.Error` gains a check: when `backend.command[0]` names a file inside the folder
  and that file has no execute bit, the entry is an **error** the listing renders, not a
  healthy Plugin. This is deliberately not a `requires` entry: `{"command": "./backend"}`
  would report the Plugin's own file as a missing *requirement* on a machine where the only
  problem is a ZIP that lost a mode bit. A requirement is what the machine must provide;
  the Plugin's own binary is what the Plugin must ship, and shipping it broken is a
  discovery error.

The three add up to a Plugin that looks installed and healthy and answers nothing:
core logs `backend exited (permission denied); restarting in 2s` forever and the card says
the backend is unavailable. `docs/PLUGINS.md:56` already promises that "a path like
`./backend` points at a binary inside your folder, so the machine needs nothing" — today
that promise is false through the import path.

The fix is in core and it is small, and ticket 01 carries it because "the Plugin installs
and runs" is ticket 01's whole point. A test that round-trips a ZIP with an executable file
in it is the test that keeps it true.

### 9.4 A Download must not carry the Plugin's state

`Entry.Archive()` zips every regular file under the folder
([import.go:93](../../apps/api/internal/plugins/import.go#L93)), so the removal journal in
`state/` would ride along in a Download and grow without bound. **The Plugin declares what
does not belong in an archive, and core honours it**: `Archive()` reads
`<folder>/.archiveignore` when it exists — one folder-relative path or `dir/` prefix per
line, `#` for comments — skips those entries, never writes the ignore file itself, and
behaves exactly as today when the file is absent, so `hello-widget`, `echo` and
`system-stats-custom` do not change. Ticket 10 owns the implementation and the test on
both sides of the seam; the alternative (hard-coding a folder name in core) would be core
knowing one Plugin's business.

### 9.5 The `system-stats` built-in that cannot be reached

Verified while writing this spec, and worth its own ticket somewhere else: `main.go`
registers `"system-stats": newStatsHandler(...)` as a built-in
([main.go:72](../../apps/api/main.go#L72)), but there is no `plugins/system-stats/`
folder in the repo — `apps/api/plugins/` holds `echo`, `hello-widget` and
`system-stats-custom` — and `Backends.Handler` asks the registry before it asks the
built-ins ([backend.go:80](../../apps/api/internal/plugins/backend.go#L80)), while
`Registry.Find` only ever returns folders ([registry.go:75](../../apps/api/internal/plugins/registry.go#L75)).
So a deployed panel answers `404 plugin not found: system-stats` for the built-in, and
the default Dashboard — which ships a `system-stats` Widget
([dashboard.go:130](../../apps/api/internal/dashboard/dashboard.go#L130)) — renders a
failing card on a fresh install. The tests do not catch it because each one writes the
folder it needs into a temporary directory
([stats_boundary_test.go:16](../../apps/api/stats_boundary_test.go#L16)), which is exactly
the artefact production is missing.

It is out of scope here and it is not a reason to change the seam: this Plugin is a folder
Plugin precisely so it does not inherit the problem. The fix is a
`plugins/system-stats/manifest.json` plus a `widget.js`, or dropping the built-in in
favour of the `system-stats-custom` shape that already exists.

## 10. What ships, in what order

Ticket 01 is a **vertical slice**: the Plugin installs, the backend answers real numbers
for one Disk, and the card paints one bar. The catalog, the dialog, the Dry Run and
removal follow. An installable Plugin that measures one Disk correctly beats a perfect
engine nobody can look at.

| # | Ticket | Delivers |
| --- | --- | --- |
| 01 | plugin-skeleton | folder, manifest, backend, `/summary` for every Disk, installable |
| 02 | widget-card | the card: bars, Reclaimable, `Clean…`, cold-start handling |
| 03 | hot-index | Scan Roots, the index, refresh, Category registration |
| 04 | detail-dialog | the `<dialog>`, Disks, Categories, Candidates, Capability in `mute` |
| 05 | docker-category | `docker-images`, `docker-volumes`, CLI measuring and acting |
| 06 | file-categories | `rotated-logs`, `package-caches`, `temp-files`, `large-files` |
| 07 | plan-dry-run-removal | Plan, Dry Run, revalidation, apply, `409` |
| 08 | full-walk-and-detail | the explicit walk, streamed, capped, cancellable |
| 09 | capability-and-config | honest failures, tooltips, `config` handling |
| 10 | package-and-document | `linux/amd64` binary, ZIP, README, mounts, compose |

## 11. Done means

- The Plugin installs from its ZIP into a running panel and its card appears without a
  restart.
- `/summary` answers real numbers on a Linux host and does not lie about Docker's
  estimate.
- A Docker image prune is **reviewed** before it runs, and the panel shows the command
  it will run and what the Dry Run returned.
- A file that changed between measuring and removing is **skipped and named**, not
  removed and not silently ignored.
- A Candidate without Capability is visible, explained, and untickable.
- `curl`ing `POST /plan/{id}/apply` without the `dry-run` before it gets
  `409 dry-run-required` — the guarantee is in the engine, not in the markup.
- The ZIP's binary runs as uid 10001 and the Plugin never claims root.
- `pnpm test` passes with the contract suite's cases renamed to `disk-space` and every
  assertion it already made still made: the volume line, `nearly full` in words,
  `warnPercent`, `Could not read the disks` with its status, and `No volumes found`.
- `pnpm check` and `go test ./...` in `apps/api` still pass — the `ctx.fetch` widening is
  backwards compatible, and nothing in core learned about disk cleanup.
