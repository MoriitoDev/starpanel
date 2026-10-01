# Disk space plugin — settled design notes

Canonical record of every decision settled in the grilling session, before the spec,
the tickets and the ADRs were written. If a document contradicts this file, this file
is right and the document is wrong.

Vocabulary is fixed in [CONTEXT.md](../../CONTEXT.md): **Disk**, **Scan Root**,
**Category**, **Candidate**, **Recommendation**, **Selection**, **Plan**, **Dry Run**,
**Reclaimable**, **Capability**.

## Goal

A Star Panel **Plugin** that (a) shows disk usage in full detail and (b) finds dead
files, recommends them, and removes the ones the owner ticks — with the owner
supervising every removal.

## D1 — Target machine

The Linux server where Star Panel is deployed. Development happens on Windows, but
Windows is not a promise: nothing in the design depends on it, and no Windows-only
concept (Recycle Bin, VSS, `C:\`) enters the model. This is why only `linux/amd64`
is built (D13).

## D2 — Packaging: a folder Plugin with its own Go binary

`apps/api/plugins/disk-space/` with a `manifest.json`, a `widget.js` and a Go
`backend` binary compiled per platform, run as a **supervised subprocess** by core
([backend.go](../../apps/api/internal/plugins/backend.go)).

Rejected: a core built-in registered in `main.go` like `system-stats`. The Plugin is
**optional** — it must be installable, removable and replaceable without touching
core — and in exchange for that portability it gives up core's trust boundary, which
is the entire subject of [ADR-0009](../../docs/adr/0009-disk-plugin-is-an-optional-folder-plugin.md).

Consequences that follow from this choice and are not optional:

- **The safety of removal lives inside the Plugin**, not in core. Every removal is
  validated against the Candidate set the Plugin itself measured. Core cannot and
  will not mediate, because core never sees the Candidate list.
- A backend subprocess is restarted every 2s if it dies and the first call after a
  restart waits up to 3s (`startWaitTimeout`). The widget must survive that with a
  loading state, never an error flash.
- The Plugin talks to the panel over HTTP on `127.0.0.1:$STAR_PANEL_PORT` with its
  own folder as the working directory, and answers on paths relative to
  `/api/v1/plugins/disk-space/proxy/`.

## D3 — One widget, one card

The manifest declares **one** widget, `disk-space`. The card shows the Disk bars,
the total Reclaimable, and a `Clean…` button. The button opens the detail interface.
No second read-only widget: the owner's answer was "una sola tarjeta".

## D4 — The detail interface is a native `<dialog>`

The card's button calls `showModal()` on a `<dialog>` the widget creates.

Why not an overlay: `svelte-dnd-action` leaves a `transform` on the card's ancestor
while arranging, and a transformed ancestor becomes the containing block of a
`position: fixed` descendant, so an overlay would be trapped inside the card. The
top layer is immune to that, and it also gives focus trapping, `Esc`, `::backdrop`
and inertness for free.

The dialog is the widget's own DOM: still the ESM contract of
[docs/PLUGINS.md](../../docs/PLUGINS.md), no core change, no new route. It carries
the one shadow the design system allows (`--popover-shadow`) and every visual value
comes from the Tokens.

## D5 — Rootless, degrading with Capability

The panel runs as `starpanel` (uid 10001) and is not root. The design never assumes
otherwise:

- A Candidate the process may not act on is shown, marked in `mute`, and **has no
  tick box**. `--danger` is for real failures; "you cannot touch this" is `mute`.
- The reason and how to change it live in a tooltip on that row: the failed syscall
  in words (`EACCES deleting /var/log/syslog`) plus the concrete way to grant it
  (mount this host path into the panel, run the container with that user, add the
  group).
- A Category whose tool is absent (`docker` not on `PATH`, socket not mounted) reports
  that as its own state, in `mute`, and says what to do — the panel never installs
  anything ([docs/PLUGINS.md](../../docs/PLUGINS.md)).
- The whole Plugin never fails in a block because one Scan Root is unreadable.

**Rejected**: `--privileged` / `user: root` as the documented path. It buys
convenience with the whole host.

## D6 — Docker via the CLI, never via `rm`

- Acting: `docker system prune`, `docker image prune`, and (D11) a separate volume
  Category. The CLI owns the rules for what is dangling and what is in use; we do not
  reimplement them.
- Measuring: the CLI's own numbers, shown **as an estimate in `mute`**, with the
  overlap stated in words in the dialog. `docker system df` reports a per-image
  `RECLAIMABLE` that sums to more than the total because layers are shared; presenting
  that as a promise would be a lie.
- `--volumes` is **never** passed to `system prune`. Volumes reach removal only
  through their own Category (D11).
- The `docker` CLI is a declared requirement: `requires: [{"command": "docker"}]`.
- **Nothing inside `/var/lib/docker` is ever a file Candidate.** Docker's own store is
  not ours to edit; it is measured, never ticked.

## D7 — Every Plan is Dry Run first

`Dry Run` is mandatory and has no bypass. The flow is:

1. The owner ticks Candidates in a **Selection** (a subset of what was measured).
2. `Review` builds a **Plan** and runs it in Dry Run: what would be removed, what
   command would run, and what came back.
3. Only then does `Remove` appear, and it applies that same Plan.

Consequences for the engine, which the tickets must respect:

- The engine has a measure mode and an apply mode over the **same Plan**.
- **Apply without a prior Dry Run of that same Plan is an error, not a shortcut.**
  The backend refuses it (409), it is not merely hidden in the UI.
- This is a guarantee, not a UI preference: [ADR-0010](../../docs/adr/0010-every-plan-is-a-dry-run-first.md).

## D8 — Revalidation, and where it cannot reach

Between measuring and removing the world moves. Each file Candidate carries a
fingerprint — size, `mtime`, inode — and the apply step revalidates it:

- Unchanged: removed.
- Changed: **skipped**, reported in words ("3 of 12 changed; they were not touched").
- The Plan is **not** aborted wholesale. A skipped Candidate is a good failure.

**Docker cannot be revalidated.** "Dangling" and `prune` are two separate CLI calls and
the daemon may see something else in between. So for command Categories the Dry Run
output is the only preview there is, and the apply step **reports the delta**: what
the CLI actually reclaimed against what the Dry Run said. Under-delivering is
reported, never hidden.

## D9 — Categories

A **Category** is a recipe, not a user-authored rule. v1 ships a fixed set; the widget's
`config` sets thresholds and exclusions only. Users do not add Categories.

| Category | What qualifies | Acting |
| --- | --- | --- |
| `docker-images` | dangling images, stopped containers, unused networks, build cache | `docker system prune`, `docker image prune` |
| `docker-volumes` | volumes with no container referencing them | `docker volume prune` (D11) |
| `rotated-logs` | `/var/log` `*.log`, `*.gz`, `*.1`..`*.9` older than 30 days; systemd journal older than 30 days | `rm` per file; `journalctl --vacuum-time=30d` |
| `package-caches` | apt archives, pip, npm, pnpm, go build, cargo caches older than 30 days | `rm` per file/dir, revalidated |
| `temp-files` | `/tmp`, `/var/tmp` older than 7 days; `/var/crash` | `rm` per file, revalidated |
| `large-files` | files over 1 GB under the declared Scan Roots | **Nothing.** Recommendation only (D10) |

Thresholds are `config`, with these defaults: 30 days for logs and caches, 7 days for
temps, 1 GB for large files. **Old kernels are out of scope in v1** — `apt autoremove`
can remove the kernel you booted from, and that is disproportionate power for a tick
box on a machine reached over SSH. They surface, if at all, as large files.

`/home`, `/etc`, `/usr`, `/var/lib/docker` and anything holding a database are
**measured only**. They are never Candidates.

## D10 — Large files are never tickable

A file over the threshold under a Scan Root is the one thing this Plugin knows is
*big* but not *dead*. It appears in the dialog under its Category with its size, in
`mute`, with **no tick box**, as a "review this yourself" list. The alternative — a
tick box next to a 40 GB database dump — is how a cleanup tool becomes a data-loss
story.

## D11 — Volumes get their own Category, in `danger`

Agreed after the fact, with the risk accepted explicitly:

- `docker-volumes` is a **separate** Category, never a tick box inside `docker-images`.
- It renders in `--danger`, with the words that a volume is data, not junk.
- Its Plan requires a typed confirmation in addition to the Dry Run, because it is the
  one Category whose Candidates cannot be told apart from something someone needs.
- `system prune` never receives `--volumes` (D6).

## D12 — Reclaimable is shown honestly

`Reclaimable` is bytes a Plan would actually free, and it is not the sum of what the
Candidates occupy. Shared Docker layers, open handles and filesystem overhead make the
two differ in both directions. The card shows a number the Plugin can defend; a Docker
number is labelled as Docker's estimate and shown in `mute`.

## D13 — Build and distribution

- Backend built for **`linux/amd64` only** for now. `GOOS/GOARCH` for other targets is
  left to whoever needs it; the manifest does not pretend otherwise.
- A build script inside the Plugin folder produces the binary and the ZIP that
  **Import a Plugin…** accepts.
- Distribution is a ZIP of `apps/api/plugins/disk-space/` including the compiled
  binary, so the panel needs nothing on the machine but the `docker` CLI.
- The Plugin folder carries a README saying what it needs, the mounts it expects and
  the user it must run as.

## D14 — Scan strategy: hot index over declared roots, full walk on demand

- The backend keeps a **hot index of the declared Scan Roots only** — the few places
  waste lives (`/var/lib/docker`, `/var/log`, `/var/cache`, `/tmp`, `/var/tmp`). It is
  cheap and it makes opening the dialog feel instant.
- Walking a whole Disk in full detail is **explicit**: asked for, streamed, with
  progress, never silently triggered by opening the dialog. `>1 TB` trees take minutes
  and punish I/O.
- The card's numbers come from the hot index plus `statfs`, so the card is cheap to
  poll; the default `pollSeconds` stays conservative.
- **No history.** No stored scan-to-scan deltas, no "grew 2 GB since yesterday" in v1.
  It would mean state on disk to migrate and expire for a number nobody asked for.

## Configuration

Everything per-widget, in the Dashboard's `config` JSON, which the **widget** receives in
`ctx.config`. Core does **not** hand it to the backend: a subprocess gets only
`STAR_PANEL_PORT` and `STAR_PANEL_PLUGIN` (`apps/api/internal/plugins/backend.go`), and
the proxy forwards the browser's request untouched, so the widget sends the config itself
— as a query parameter on the GETs and in the JSON body on the writes. The backend
validates it per request and stores nothing between requests, which is also what keeps a
restart from losing it.

Keys and defaults: `scanRoots` (the places waste lives), `exclude`, `logDays` 30,
`cacheDays` 30, `tempDays` 7, `largeFileBytes` 1 GiB, `roots` (the effective `scanRoots`),
`warnPercent` 90. Unreadable or malformed config falls back to the defaults and says so in
the card, never with a 4xx.

## D15 — Core changes this design depends on

Four, all small, all in `apps/api`, and three of them are pre-existing defects this Plugin
walks into:

1. **`ctx.fetch` must accept a method and a body.** It is
   `(path: string) => Promise<Response>` today, which makes the widget contract read-only
   and `POST /plan` — the Dry Run guarantee — unreachable. Done: it is now
   `(path: string, init?: RequestInit)`. The widget must still probe, because a silently
   ignored second argument would turn an apply into a GET.
2. **The exec bit on Import and Download.** `writeEntry` forces `0o644` and `Archive()`
   uses `writer.Create`, so a compiled `backend` cannot survive either direction, and
   `reachable` accepts it on `os.Stat` alone. Without the fix the Plugin installs,
   reports healthy, and never runs.
3. **A non-executable backend must be visible, not silent.** `requires` is what the
   *machine* must provide and stays `docker` alone; the Plugin's own binary is checked at
   discovery, so a ZIP that lost the mode bit is reported as a broken entry instead of a
   `permission denied` restart loop.
4. **`.archiveignore`**, so a Download does not carry `state/` — the removal journal —
   and the ZIP does not grow with every cleanup.

And one defect to leave alone: `system-stats` is registered as a core built-in with no
`plugins/system-stats/` folder, and `Backends.Handler` asks the registry first, so the
built-in is unreachable in production while the tests pass because they write the folder
themselves. Out of scope, recorded in the spec.

## Delivery

Vertical slice first: ticket 01 is a skeleton that installs, answers real numbers for
one Disk and paints one bar. The catalog, the dialog, the Dry Run and removal follow.
A perfect engine nobody can look at is worse than an installable one that measures one
Disk correctly.
