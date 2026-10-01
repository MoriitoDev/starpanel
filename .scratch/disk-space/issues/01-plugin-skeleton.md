# 01 — Plugin skeleton: installable, and honest about one Disk

Status: done

## What

`apps/api/plugins/disk-space/` exists as a Plugin: a `manifest.json`, a Go `backend`
that core starts as a supervised subprocess, and a `widget.js` the Dashboard can load.
`GET /health` answers, and `GET /summary` answers real numbers for every Disk on the
machine — mount point, filesystem, total, used, free, used percent — measured with
`statfs`. Dropping the folder into `plugins/` is enough to see it in the panel's Plugin
list and add its Widget.

No Category finds anything yet. The Plugin must be **installable and correct** before it
is useful.

## Why

This is the vertical slice ([spec §10](../spec.md)): an installable Plugin that measures
one Disk correctly beats a perfect engine nobody can look at. It also proves the three
things every later ticket depends on — the subprocess seam, the port handshake, and the
poll path from card to backend — while there is nothing destructive in the code.

## Acceptance

- [ ] `plugins/disk-space/manifest.json` declares `name: "disk-space"`, a version, one
      Widget (`id: "disk-space"`, `module: "widget.js"`) and
      `requires: [{"command": "docker"}]`, and the folder name equals `name`
- [ ] `GET /api/v1/plugins` lists `disk-space` with no `problem` field on a machine that
      has `docker`, and with the `needs docker, …` problem on one that does not
- [ ] The backend binds `127.0.0.1:$STAR_PANEL_PORT` only, and answers
      `/api/v1/plugins/disk-space/proxy/health` with `{"status":"ok",…}`
- [ ] `GET /summary` answers one entry per mounted filesystem with `name`, `label`, `fs`,
      `mount`, real `totalBytes`/`usedBytes`/`freeBytes`, and `usedPercent` consistent
      with them
- [ ] `statfs` accounts for the root reserve: `freeBytes` comes from `Bavail`, not `Bfree`
- [ ] A `statfs` failure on one Disk omits that Disk and adds a `warnings` entry; it never
      fails the whole answer
- [ ] The answer carries `reclaimableBytes: 0`, `reclaimableIsEstimate: true`, `indexAge`
      and the six v1 Category ids with `candidates: 0`
- [ ] The widget loads, paints the mount points and their percentages, and its cleanup
      function stops the poll timer
- [ ] The backend rebuilds and restarts cleanly when core kills it: a request arriving
      during the 3s startup window is retried, and the card never shows an error for it
- [ ] No external Go dependency: `go.mod` in the Plugin folder with no `require` lines
      that need the network
- [ ] `widget.js` contains no literal colour, no utility class and no second accent
- [ ] **Core: a Plugin that arrives as a ZIP is executable.** `writeEntry` preserves the
      archive entry's permission bits instead of forcing `0o644`
      ([import.go:223](../../apps/api/internal/plugins/import.go#L223)), masked so an
      archive can never deliver setuid, setgid or a world-writable file
- [ ] **Core: a Download can be imported back.** `Entry.Archive()` writes a header
      carrying the file's mode instead of `writer.Create`'s `0644`
      ([import.go:109](../../apps/api/internal/plugins/import.go#L109)), so a round trip
      through Download and Import keeps the binary executable
- [ ] **Core: `Entry.Error` catches a non-executable backend.** An entry whose
      `backend.command[0]` names a file inside the folder checks the execute bit and
      reports `backend ./backend is not executable` in the Plugin listing, instead of
      discovering it as a `permission denied` restart loop after the Widget is added
- [ ] **The test that keeps it true**: a `plugins.Import` / `Entry.Archive` round trip with
      an executable file in the folder leaves it executable, on both sides of the seam
      (`import_test.go` and the HTTP test in `plugins_io_test.go`)

## Notes

**The source is `src/`, and it is its own Go module.**
`apps/api/plugins/disk-space/src/go.mod`, module `star-panel/plugins/disk-space`, built with
`go build -C src -o ../backend .`. Two reasons that shape is forced:

- **`src/` and not `backend/`**: the Manifest runs `./backend`, and `command[0]` must
  contain a separator or `exec` looks the name up in `PATH` instead of the Plugin folder —
  so the file `backend` sits at the Plugin root, and a file and a directory cannot share a
  name in one folder. `../backend` from `src/` is where the binary lands.
- **Its own module**: the build never touches core's module at `apps/api/go.mod`, so
  `go build ./...` from `apps/api` does not compile it and removing the folder removes the
  feature with nothing left behind (D2, ADR-0009).

**Three lines of core are between this and a Plugin that runs.** They are small, they are
in `apps/api/internal/plugins/`, and this ticket is where they belong because "the Plugin
installs and runs" is this ticket's entire point. The defect is worth stating plainly: a
compiled Plugin binary **cannot survive Import or Download today**. `writeEntry` unpacks
every file `0o644` ([import.go:223](../../apps/api/internal/plugins/import.go#L223)),
`Entry.Archive()` writes every entry with `writer.Create`
([import.go:109](../../apps/api/internal/plugins/import.go#L109)) — also `0644` — and
`reachable` decides a path-with-separator requirement on `os.Stat` alone
([registry.go:41](../../apps/api/internal/plugins/registry.go#L41)). So:

- `writeEntry` writes `file.Mode().Perm()` with `0o111` and the setuid/setgid/sticky bits
  the importer must never grant. The mask is the point: an archive may grant execute and
  nothing more, so `perm := file.Mode().Perm() &^ 0o6000 &^ os.ModeSticky` (or a flat
  `& 0o755` if you prefer one number) is the shape, not a faithful copy of whatever a
  stranger's ZIP declared.
- `Archive()` builds a `*zip.FileHeader` from the `fs.FileInfo` and sets
  `header.SetMode(info.Mode())` rather than using `writer.Create`.
- **`Entry.Error` gains a third check**, next to "the Manifest does not parse": when
  `backend.command[0]` names a file inside the Plugin folder and that file has no execute
  bit for anyone, the entry is an **error**, not a healthy Plugin with a missing
  requirement. `registry.go` already has an `Err` field on `Entry` and the listing already
  renders it, so this is a few lines and it surfaces the exact symptom that otherwise costs
  an hour: core logging `backend exited (permission denied); restarting in 2s` forever while
  the Plugins section looks fine.

The rejected alternative is worth stating because it is the obvious one: declaring
`requires: [{"command": "./backend"}]` and making `reachable` check the execute bit. It
works, and it is wrong, because it reports the Plugin's own file as a missing *requirement*
on a machine where the only problem is a ZIP that lost a mode bit — the listing says "needs
./backend, which is not on this machine" about a file that is right there. A requirement
is something the machine must provide; the Plugin's own binary is something the Plugin must
ship, and shipping it broken is a discovery error.

`docs/PLUGINS.md:56` already promises that a path like `./backend` lets a Plugin bring its
own binary; without these three lines that promise is false on the import path, and the
symptom is a card that says the backend is unavailable while the folder looks perfect.

**Why this matters for the whole design, not just this ticket**: D13 ships the Plugin as a
ZIP, and ADR-0009 makes it a folder Plugin precisely so it can be installed and moved
without touching core. A binary that loses its execute bit on import breaks both at once.

Layout to create:

```
plugins/disk-space/
├── manifest.json
├── widget.js
├── src/                (Go source, package main, its own module)
│   ├── go.mod
│   ├── main.go
│   └── disk.go
└── .gitignore          backend, dist/, state/
```

**The port handshake.** Core runs the command with the Plugin folder as its working
directory, replaces `{port}` in any argument, and exports `STAR_PANEL_PORT`
([backend.go:184](../../apps/api/internal/plugins/backend.go#L184)). So
`"command": ["./backend"]` is enough — read `STAR_PANEL_PORT`, listen on
`127.0.0.1:` + it, and log the address. Do not hardcode 8080 or any port; a second panel
instance on the same machine must not collide. `./backend` is resolved through
`entry.Dir`, and `requires` uses the same reachability rule
([registry.go:39](../../apps/api/internal/plugins/registry.go#L39)) — a path with a
separator is looked for inside the Plugin folder, so no `requires` entry is needed for
the binary itself, only for `docker`.

**Supervision is not your problem, but its latency is.** Core restarts a dead backend
every 2 seconds and waits up to `startWaitTimeout` (3s) for it to listen; a call that
arrives while it is starting gets an error from core, not from you. The widget must
treat that as "starting", not as failure: on a failed `GET /summary` during the first
seconds, show `mute` placeholder text and retry with a short backoff. Never a spinner
([DESIGN.md:123](../../DESIGN.md#L123)) and never `Widget failed:` for a cold start.

**Disk listing.** Read `/proc/mounts`, keep the real filesystems, and skip the ones that
are not Disks a person cares about: `tmpfs`, `devtmpfs`, `proc`, `sysfs`, `cgroup`,
`cgroup2`, `overlay` on `/`, `squashfs`, `ramfs`, `autofs`, `securityfs`, `debugfs`,
`tracefs`, `mqueue`, `hugetlbfs`, `configfs`, `fusectl`, `binfmt_misc`, `nsfs`. Do not
skip a `tmpfs` mounted at `/tmp` if the owner declared it a Scan Root — measure what is
declared, and only filter the pseudo-filesystems nobody configured. Deduplicate by
`(mountpoint, device)`, because `/proc/mounts` repeats a device under several paths.

`syscall.Statfs` on the mount path gives `Blocks`, `Bfree`, `Bavail` and `Bsize`. Report
`usedBytes = (Blocks - Bfree) * Bsize` and `freeBytes = Bavail * Bsize` — the difference
between `Bfree` and `Bavail` is the root reserve, and calling `Bfree` "free" tells the
owner they have space they cannot use.

**Four fields, four different things.** `name` is what a person calls the volume and what
the existing contract suite asserts on (`C:`, `/`); `label` is the volume label when the
platform has one (empty on Linux); `fs` is the filesystem (`ext4`, `NTFS`); `mount` is
where it is mounted. On Linux `name` is the mount point and `source` is `procfs` or
`statfs`; a Windows build answers `win32` with drive letters as `name`. The WidgetCard
renders `name · label · fs`, so collapsing these four into one field breaks the contract
suite (see [spec §9.2](../spec.md)).

**Warn, in words, and let the owner move the line.** The card marks a Disk "nearly full"
at `warnPercent`, default 90, and paints that row `var(--danger)` — with the words, never
the colour alone. `warnPercent` is a config key the suite already exercises, so read it
from the config with the default and do not hardcode 90 anywhere.

**Shape stability matters more than the fields.** Answer the exact field names
`spec §3` gives for `/summary`, including `reclaimableBytes`,
`reclaimableIsEstimate`, `categories`, `indexAge` and `warnings`. In this ticket
`reclaimableBytes` is `0` and `categories` is the full v1 list with `candidates: 0` —
all six ids from [spec §5](../spec.md), registered in one place so tickets 05 and 06 only
add scanners and never touch the wire shape. A field added later is a field every client
already ignores; a field renamed later breaks the widget in the field.

**Errors.** `{"error": "..."}` with the right status, the same shape core speaks
([backend.go:112](../../apps/api/internal/plugins/backend.go#L112)). The widget reads
`error` and shows it in words, in `--danger`, inside the card — cap the length so a
`syscall.Errno` with a mile of path does not blow the card's layout.

**The card in this ticket is a placeholder.** It paints the Disks and one muted line
saying the detail view is not built yet; ticket 02 gives it the real body and ticket 04
the `<dialog>`. Do not invent layout here that ticket 02 will throw away — read
[DESIGN.md §4](../../DESIGN.md#L101)'s Card and §5, and keep the toolbar row (the
`Clean…` button) as the seam 02 fills.

**Tests.** Unit-test the `/proc/mounts` parsing and the pseudo-filesystem filter against
a fixture, not against the live machine — the suite must pass on a developer's Windows
box where `/proc` does not exist. Keep the Linux-only code behind a `//go:build linux`
file with a stub sibling, the way `apps/api/stats_linux.go` and `stats_notlinux.go` do
it, so the package compiles everywhere even though only `linux/amd64` ships.

## Comments

- 2026-10-01: Done. `apps/api/plugins/disk-space/` exists with `manifest.json`,
  `widget.js`, `src/` (its own module, `star-panel/plugins/disk-space`) and `build.ps1`.
  `src/disk.go` is the portable half — `/proc/mounts` parsing with the kernel's octal
  escapes, the pseudo-filesystem denylist with `overlay` deliberately kept, one Disk per
  device, and the arithmetic where `freeBytes` comes from `Bavail` — and `src/statfs_linux.go`
  is the only platform-specific call, with `src/statfs_other.go` refusing to invent numbers
  on a platform that has no `/proc`. Eight fixture-driven tests in `src/disk_test.go`.
- 2026-10-01: One decision the ticket did not make: `Disks` is measured through an injected
  `read`/`statfs` pair, so the parsing is tested on Windows and only the syscall is Linux.
  That is what let the whole suite run on this machine.
- 2026-10-01: The three core fixes are in, with `permissions_test.go` beside them.
  `unpackedMode` is a pure function of an `fs.FileMode` so the decision is pinned on every
  platform (`TestUnpackedModeGrantsExecuteAndNothingMore`); the round trip is asserted on
  Unix (`TestImportKeepsAnArchivesExecuteBit`, `TestArchiveRoundTripKeepsTheBinaryRunnable`).
  The discovery check became `Manifest.executableBackend` rather than a `reachable` change,
  because a requirement is what the *machine* must provide and the Plugin's own binary is
  what the *Plugin* must ship.
- 2026-10-01: Two things learned by running it rather than reasoning about it. Core's
  `Import` could not create the plugins folder if it did not exist yet, which made an
  import into a fresh deployment answer `400`; `Import` now creates it, with a test. And
  `executableBackend` skips its check entirely on Windows, where `chmod` only moves the
  read-only flag: without that guard the import is refused on the one platform where the
  question has no answer.
- 2026-10-01: **Not verified here**: that core restarts a dead backend cleanly and that a
  request during the 3s startup window is retried. The Plugin's binary is ELF, so on this
  Windows machine the panel answers `502 plugin backend unavailable` for its proxy and the
  restart loop never reaches a healthy state. The widget handles that path in code and the
  proxy's behaviour is core's existing, tested one, but the end-to-end run belongs on the
  Linux panel the Plugin is built for.
