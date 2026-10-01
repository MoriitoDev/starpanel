# 05 — Docker Categories

Status: ready-for-agent

## What

Add the `docker-images` and `docker-volumes` Categories to the `disk-space` Plugin: they measure Docker's own numbers from `docker system df` and `docker volume ls`, present them as Docker's estimate in `mute` with the overlap said in words, and act only through `docker system prune`, `docker image prune` and `docker volume prune`. Declare the `docker` CLI in the Manifest's `requires` so the panel reports it missing before anyone adds the Widget. Every Candidate carries the exact command line that would run, so the Dry Run can print it.

## Why

Docker's store is the largest single pile of dead bytes on a homelab machine, and the easiest one to lose data with — `rm` inside `/var/lib/docker` corrupts the store and `--volumes` on `system prune` deletes data with one word (D6, D11). Without this ticket the Plugin can only see files and the biggest win stays invisible; done wrongly, the biggest win becomes the worst loss. The Docker Categories are also the only place where Capability is two things rather than one — the CLI and the socket — which is why D5's own state in `mute` has to be built here first.

## Acceptance

- [ ] `manifest.json` declares `"requires": [{"command": "docker"}, {"command": "./backend"}]`, and the Plugins section reports a missing `docker` before the Widget is added
- [ ] `docker-images` produces four Candidates — dangling images, stopped containers, unused networks, build cache — each carrying its own `action` (the exact command line) and Docker's own number for it
- [ ] No Plan ever contains two Candidates that would reclaim the same bytes: a Selection of all four coalesces into one `docker system prune -f`, a partial one runs only the dedicated prunes it needs
- [ ] `--volumes` never appears in any `action` the Plugin can produce, and no `action` can name a path under `/var/lib/docker`; both are pinned by a test that walks every action the scanners register
- [ ] `docker-volumes` is a separate Category rendering in `var(--danger)`, is never a tick box inside `docker-images`, states in words that a volume is data and not junk, and cannot be planned without the typed confirmation phrase the backend itself supplies
- [ ] Its single Candidate's `action` is `docker volume prune -f -a`, and the unused volumes it would take are listed as read-only evidence rows with their sizes, links and drivers
- [ ] Capability is two things — the CLI on `PATH` and the daemon reachable — and each of `command-missing`, `socket-missing`, `socket-permission`, `daemon-timeout`, `daemon-error` answers the spec's `capability` object (`kind`, `errno`, `attempted`, `words`, `remedy`) on the Category, renders in `--mute` with a verbatim reason and a concrete remedy, and offers no tick box
- [ ] Docker's figures render in `--mute` labelled "Docker's estimate", the shared-layer overlap is stated in words in the dialog, and any `/summary` whose `reclaimableBytes` includes a Docker figure sets `reclaimableIsEstimate: true`
- [ ] Rows and figures use `font-variant-numeric: tabular-nums`, no literal colour, and no Token beyond those in `docs/PLUGINS.md`
- [ ] `docker system df` is never run by `GET /summary`: it is measured with the hot index (spec §6) and `/summary` answers the cached figures with `indexAge`
- [ ] A fake `docker` shell script on a temp `PATH` drives the tests: the four Candidates and their actions, the coalescing rule, the volume Category, every capability state, and the refusal to build a `docker-volumes` Plan without the phrase
- [ ] The ZIP that **Import a Plugin…** takes produces a `backend` the panel can execute, which needs core's exec-bit fixes (spec §9.3, ticket 01)

## Notes

### Files this ticket owns

Per the spec's layout (§2) the backend is one `package main` at `apps/api/plugins/disk-space/`, with its own `go.mod` (`module star-panel/plugins/disk-space`, standard library only, `CGO_ENABLED=0`) and its Go sources in `src/` beside the built binary. This ticket adds both Docker scanners to `src/category.go` — the Category registry and the six scanners — and their tests to a `_test.go` beside it.

**The layout collision, and how it is settled.** Spec §2 lists both `backend` (the compiled linux/amd64 binary) and `backend/` (the Go source) in one folder, and no filesystem allows a file and a directory to share a name. The file wins: `./backend` is in §2's manifest line, in §9.3's `backend.command = ["./backend"]`, and already promised by `docs/PLUGINS.md:56`. So the source lives in `apps/api/plugins/disk-space/src/`, the module is `star-panel/plugins/disk-space`, `build.ps1` (ticket 10 owns it) runs `go build -C src -o ../backend .`, and the binary lands at `apps/api/plugins/disk-space/backend` where the Manifest expects it. A nested module is also excluded from `go build ./...` in `apps/api`, which is the point: the Plugin never imports core and core never imports it (D2).

### The Manifest

```json
{
  "name": "disk-space",
  "version": "0.1.0",
  "widgets": [{ "id": "disk-space", "title": "Disk Space", "module": "widget.js" }],
  "backend": { "command": ["./backend"] },
  "requires": [{ "command": "docker" }, { "command": "./backend" }]
}
```

- **No `minVersion`.** `docs/PLUGINS.md` says the panel shows the version you asked for and does not judge the one installed, and `docker --version` prints `Docker version 27.3.1, build ce12230` — a sentence, not a version. A floor we cannot verify is a claim; the runtime capability check is the real answer.
- `requires` is what makes the missing CLI visible **before anyone adds the Widget**: `Entry.Missing()` in `apps/api/internal/plugins/registry.go` resolves a bare name with `exec.LookPath`, so the Plugins listing and the answer to an import say "needs `docker`" with no code from us. Do not duplicate that check in the widget. The `./backend` entry and the exec-bit discovery check are ticket 01's (§9.3).
- The backend reads `STAR_PANEL_PORT` (`backend.go` exports it and substitutes `{port}` in arguments; the command above needs neither).

### The two Categories, measured

Catalog response shape is spec §3's `GET /candidates` — Category `{id, title, rule, bytes, estimate, tickable, capability, reviewOnly?, candidates[]}` and Candidate `{id, path, bytes, reason, selectable, capability, fingerprint?}` — with `action` and `notes` added by this ticket. A file Candidate's `id` is `cl_` plus twelve hex; a Docker Candidate's is the stable argv form `docker-images:dangling`, `docker-images:build-cache`, `docker-volumes:all`, which is readable in a journal and needs no hashing. Requests carry the config: `?config=` on the GET, and a `POST` with `{"config": …}` when it is too big for a query string.

**`docker-images`** — one Candidate per thing the CLI can name, all four disjoint by construction, so any subset reclaims no byte twice:

| Candidate | `id` source | `kind` | `action` | Estimate from |
| --- | --- | --- | --- | --- |
| dangling images | `docker-images` + `docker image prune -f` | `command` | `docker image prune -f` | sum of `.Size` over `docker image ls --filter dangling=true --format '{{json .}}'` |
| stopped containers | `docker-images` + `docker container prune -f` | `command` | `docker container prune -f` | `Containers` `.Reclaimable` in `docker system df` |
| unused networks | `docker-images` + `docker network prune -f` | `command` | `docker network prune -f` | none — a network consumes no disk, so `bytesKnown: false` |
| build cache | `docker-images` + `docker builder prune -f -a` | `command` | `docker builder prune -f -a` | `Build Cache` `.Reclaimable`/`.Size` in `docker system df` |

- A command Candidate has no `path` and no `fingerprint` (spec §3 says so explicitly: a Docker image cannot be fingerprinted), so its `id` is `c-` plus six hex of a SHA-256 over the Category, the empty path and the **`action` string** — the same derivation as a file Candidate, with the fingerprint absent and the command standing in for the path. Report that substitution in a code comment, or the next reader will think the hash is arbitrary.
- A Candidate whose estimate is unknown (`bytesKnown: false`) is still offered — the CLI knows better than we do what it will take — with the words "Docker did not report a size for this" in `mute` instead of a figure.
- The coalesced step ticket 07 builds when all four are selected carries the id `c-` plus the hex of the Category and `docker system prune -f`, and it appears in a Plan and never in `/candidates`. A `POST /plan` naming it is a `400 not-a-candidate`: a caller may select only what the Plugin measured, and the coalescing is the engine's decision (spec §7, D2).
- **Keep the argv in Go and the string on the wire.** Each scanner registers an argv constant (`[]string{"docker","system","prune","-f"}`) and `action` is its display form. The apply step executes the registered argv and **never** `strings.Fields(action)`, because splitting a display string is how a quoting rule gets invented at runtime. A test asserts that the executed argv equals the registered argv for every registered action.

- **Why not `docker image prune -a`.** The `Images` `RECLAIMABLE` column counts every image no container references, *including tagged ones*, while `docker image prune -f` removes only dangling images and reclaims far less. Those are two different numbers, and offering `-a` would delete an image the owner pulled for tomorrow. `-a` is deliberately not offered in v1, and the dialog says so where the gap is visible.
- **Why the build cache gets `-a`.** `docker builder prune -f` removes only *dangling* build cache while `docker system prune -f` removes *unused* build cache, so `-a` is what makes the dedicated Candidate equal to the coalesced one. The build cache includes BuildKit `RUN --mount=type=cache` mounts, which are a build accelerator rather than junk: the Candidate's `reason` must say "pruning this makes the next build slower; it loses no data."
- **Networks** contribute a count and no bytes. Never render `0 B` for them; render the count and "occupies no disk space" in `--mute`. Docker's own reference says network information is not shown by `system df` "because it doesn't consume disk space".

**Measurement commands, exactly:**

```
docker system df --format json
docker image ls --filter dangling=true --no-trunc --format '{{json .}}'
```

- `docker system df --format json` prints **one JSON object per line**, every value a string: `{"Active":"2","Reclaimable":"2.498GB (94%)","Size":"2.631GB","TotalCount":"6","Type":"Images"}`. `Type` is one of `Images`, `Containers`, `Local Volumes`, `Build Cache`.
- `Reclaimable` is a human string. Parse the first whitespace-separated token and its decimal unit suffix (`B`, `kB`, `MB`, `GB`, `TB` — **powers of 1000**, so `2.498GB` is 2 498 000 000) and ignore the trailing `(94%)`. Reformat for display with the Plugin's own binary units and never re-read Docker's string as 1024-based.
- If `--format json` exits non-zero (a CLI older than the `json` directive), retry once without `--format` and parse the default table, adding a Category `notes` entry in words: "sizes were parsed from this Docker CLI's table output". If that also fails, the Category answers a `capability` object with `kind: "daemon-error"` and the CLI's first stderr line as `words`.
- **`--format` has no effect with `-v`** (Docker's reference says so explicitly), which is why per-volume sizes are parsed from the `-v` table and never asked for as JSON.

**`docker-volumes`** is measured with both commands, because each answers a different half:

```
docker volume ls --filter dangling=true --format '{{json .}}'
docker system df -v
```

- `dangling=true` on `docker volume ls` means "not referenced by any Container", which is this Category's qualifying rule. Its keys are `Name`, `Driver`, `Scope`, `Labels`, `Mountpoint`.
- **`Mountpoint` is a path inside `/var/lib/docker/volumes/` and is never a path this Plugin removes.** It goes on the evidence row and nowhere else — the same rule as D6's, met from the other side.
- Sizes come from the `Local Volumes space usage:` table in `docker system df -v`, joined by volume name. That table is human-aligned text (`NAME  LINKS  SIZE`, then a `Total:` line): split the section between `Local Volumes space usage:` and the next blank-line-terminated header, and match the row whose first column equals the name after trimming — not by fixed column offsets, which shift with the longest name. A volume in `volume ls` but absent from the table has never been written to: `bytesKnown: false`, rendered "size unknown", never `0 B`. A volume the daemon holds but never mounted genuinely has no size to report, and `0 B` says "empty", which is a different claim.
- **One Candidate**, `action: "docker volume prune -f -a"`, `path: null`, `bytes` = the sum of the known volume sizes with `bytesKnown: false` when any is unknown, `tickable: true` on the Category, and `capability` per the section below. The individual unused volumes are evidence rows — name, driver, size, links, and any `com.docker.compose.project` / `com.docker.compose.volume` label — in `var(--danger)`, with **no tick box**.
- The Category is all-or-nothing because **the CLI's own shape is all-or-nothing**: `docker volume prune` takes `--filter label=…` and nothing by name, so a partial prune is not expressible without us deciding which volumes are safe — exactly what D6 forbids. That is why D11 gives it the typed confirmation, and the dialog says so in that sentence.
- **`-a` is required, and spec §5's shorthand `docker volume prune -f` is the anonymous-only default.** Without `-a`, only *anonymous* volumes are removed, so every named-but-unused volume this Category just listed would survive the command that claims to remove them. `-a` is what makes "unused" mean unused; the delta report in ticket 07 is where a divergence shows up, and the README and the dialog both name the flag.

### Capability: the CLI and the socket

Probe with the measurement itself and classify the exit and the stderr, in this order, when the hot index is built (spec §6: every 10 minutes or when the config changes; never on the card's poll):

| `kind` | Detected by | `attempted` | `words` | `remedy` |
| --- | --- | --- | --- | --- |
| `command-missing` | `exec.LookPath("docker")` returns `exec.ErrNotFound` | `exec.LookPath("docker")` | "the `docker` command is not on PATH" | "Install the Docker CLI on the machine running Star Panel, or add it to the panel's image and rebuild." |
| `socket-missing` | stderr contains `Cannot connect to the Docker daemon` | `connecting to unix:///var/run/docker.sock` | the CLI's sentence, verbatim | "Mount the socket: `-v /var/run/docker.sock:/var/run/docker.sock`, or set `DOCKER_HOST`." |
| `socket-permission` | stderr contains `permission denied` on the socket | `connecting to unix:///var/run/docker.sock` | the CLI's sentence, verbatim | "Add uid 10001 to the group that owns the socket, or run the panel as a user that may read it." |
| `daemon-timeout` | the deadline expires | `running docker system df` | "the daemon did not answer within 10s" | "`docker system df` traverses every image, container and volume; on a large machine it is slow. Retry, or check the daemon's health." |
| `daemon-error` | anything else | `running docker system df` | the CLI's first stderr line, verbatim | "`docker` answered with something this Plugin could not read; run the command by hand to see it." |

- The object is spec §3's: `{"ok": false, "kind": …, "errno": "EACCES", "attempted": …, "words": …, "remedy": …}`, with `errno` the errno when there is one and omitted when there is not (`command-missing` has none). It goes on the **Category**, so the dialog's one rule — a tick box exists only when `category.tickable && candidate.capability.ok` — disables the whole Category without a special case.
- **Every one of these renders in `--mute`, with a verbatim reason and a concrete remedy, and with no tick box** (D5). `--danger` is used for none of them: `DESIGN.md` §2 says there is no warning hue and a state is fine, not fine, or *not known yet* — and "I cannot reach Docker" is not-yet-known, not a failure of the panel. `danger` is spent only on the `docker-volumes` Category's data warning and on a removal that actually failed (ticket 07).
- `DOCKER_HOST` and `DOCKER_CONTEXT` are inherited and never overridden: a remote daemon is a legitimate answer, the sizes are then *that* daemon's, and the Category `notes` say so in those words. **A Docker Candidate is never attributed to a Disk on this machine**, because with `DOCKER_HOST` set the bytes do not come back here at all; ticket 07's per-Disk readings therefore skip command steps entirely.
- The socket is root-equivalent: whoever holds it can start a privileged container. The Plugin's `README.md` says so plainly, next to the mount line (D5's spirit — the convenience is not free, and the owner decides).

### Cost, caching and the card

Docker's reference warns that `docker system df` "can be resource-intensive … traverses the filesystem of every image, container, and volume". So:

- `docker system df` is run when the **hot index** is built and its parsed figures are stored on the index (spec §6: built on the first request, refreshed every 10 minutes or when the config changes). `GET /summary` reads them from the index and never runs the command — spec §3 requires `/summary` to be "cheap by construction: `statfs` per Disk plus the hot index, never a walk".
- The index carries `indexAge`, which `/summary` and `/health` already report; the card renders it in its `mute` footer, so an old Docker figure is visibly old rather than silently stale.
- Nothing here is written to disk.

### The card, and what `mute` means there

`DESIGN.md` §2 has no warning hue and D12 is explicit: a Docker number is an estimate and says so. Spec §3 resolves it in the answer rather than by omission:

- When any contributing Category answers `estimate: true`, `/summary` sets `reclaimableIsEstimate: true` and the card renders the total with its caveat in `var(--mute)`.
- The overlap sentence, verbatim, sits under the Docker Categories' figures in the dialog: "Docker reports each image's reclaimable size on its own; layers are shared between images, so the sum is larger than what a prune actually frees." It is words, not a footnote marker, because that sentence is what makes the number honest.
- The `Local Volumes` figure stays out of the card's invitation: it appears inside the dialog, in `var(--danger)`, next to the words and the phrase. A `danger` figure on a card the owner glances at is alarm with no action attached. Tickets 01, 02 and 04 own the card's layout; this ticket owns the rule and the two keys.
- The Category's `rule` string, in the spec's own voice: `docker-images` — "dangling images, stopped containers, unused networks, build cache"; `docker-volumes` — "volumes no container references".

### Tokens

Only what `docs/PLUGINS.md` lists: `var(--mute)` for every Docker figure, every `words` and every `remedy`; `var(--danger)` for the `docker-volumes` title, its warning sentence and the phrase prompt's label; `var(--ink)` for counts and the displayed `action`; `var(--border-soft)` under an evidence row; `var(--surface-soft)` behind an `action` line. Type from `var(--text-meta)` and `var(--text-base)` only, with `font-variant-numeric: tabular-nums` on every figure. An `action` line is `font-mono` at `meta` size (spec §4 does the same for a Candidate's path). No literal colour, no second accent, no shadow — the dialog's one shadow is `var(--popover-shadow)` and it belongs to ticket 04's `<dialog>` (D4).

### Traps

1. `docker system df` **without** `--format json` prints aligned columns, and `--format` and `-v` are mutually exclusive. Two parsers, not one.
2. `Reclaimable` strings are decimal and carry a `(NN%)` suffix on three rows and none on `Build Cache` — a parser that assumes the suffix drops the build cache silently.
3. `Images RECLAIMABLE` ≠ what `docker image prune -f` frees. They differ by every tagged image no container references, which is usually most of them.
4. `--volumes` must not exist anywhere in the code as a string that can reach an argv, and neither must `/var/lib/docker`. Write the test that walks every registered action and fails on either substring.
5. `docker volume prune` without `-a` removes anonymous volumes only.
6. Never prompt: `-f` on every prune, and the CLI runs with no TTY (`stdin` `/dev/null`), because core's supervised subprocess has no terminal and a prompt would hang until the deadline — and then `backend.go`'s restart loop would restart the Plugin mid-command.
7. Every CLI call gets its own `context.WithTimeout` — 10s for measurement, 5 minutes for a prune — and `exec.CommandContext` kills only the direct child. Give it `Setpgid` and kill the group on expiry, or a timed-out `docker system prune` keeps deleting behind the panel's back. Ticket 07 reports the timeout as `overran`.
8. Capability is checked when the index is built, not when the Plan is built: a Candidate whose Capability vanished between the two is a `denied` step in ticket 07's apply, reported in words, never an error.
9. A Category's `bytes` is Docker's own sum of its own rows, and `estimate: true` says so. It is not a promise, and `reclaimableIsEstimate` exists because of this Category.
10. If the socket is a TCP `DOCKER_HOST`, a successful `docker system df` still does not mean the bytes are on this machine. Never attribute.
