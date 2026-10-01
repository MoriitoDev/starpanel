# 06 — File Categories

Status: ready-for-agent

## What

Add the four file Categories to the `disk-space` Plugin — `rotated-logs`, `package-caches`, `temp-files` and `large-files` — each a recipe over the declared Scan Roots with the D9 defaults (30 days for logs and Caches, 7 days for temps, 1 GB as the large-file floor). Every file Candidate carries the size, `mtime` and inode fingerprint that ticket 07's apply step revalidates, and every Candidate records the exact `rm` argv that would run. `large-files` produces observations only, is never tickable, and exists to be read.

## Why

D9 and D10: this is the part of the Plugin that finds the waste the owner actually accumulates, and the part where a tick box can become a data-loss story if the model is loose. It cannot be written without the fingerprint rule of D8, because between measuring and removing the world moves, and it cannot be written without D14, because walking a `>1 TB` tree on every card poll is a punishable amount of I/O. Without these Categories the Plugin is a Docker viewer.

## Acceptance

- [ ] Four Categories exist with the D9 defaults, and the defaults are the ones that apply when the Widget sends a `config` that is absent, malformed or out of range — never a 4xx, always a `configWarning` the card renders in `--mute`
- [ ] `rotated-logs` offers `/var/log` `*.log`, `*.gz` and `*.1`..`*.9` older than `logDays` as file Candidates with `rm -f -- <path>` argv, and the systemd journal older than `logDays` as one command Candidate with `journalctl --vacuum-time=<logDays>d` (`30d` by default)
- [ ] The journal's Candidate and the `rm` Candidates have separate Capability lines and separate Dry Run output; nothing under `/var/log/journal/` is ever an `rm` Candidate
- [ ] `package-caches` covers the apt archives plus pip, npm, pnpm, go build and cargo caches by recipe, and a cache root under a never-Candidate prefix is listed as skipped in words rather than offered
- [ ] `temp-files` covers `/tmp` and `/var/tmp` older than `tempDays` and `/var/crash`, and a Candidate on a tmpfs is marked as freeing memory rather than Disk
- [ ] `large-files` produces observations under its Category with size and `mtime`, in `--mute`, with no tick box, no `candidateId`, no fingerprint and no `command`, and the Category reports `selectable: false`
- [ ] No Candidate is ever produced under `/home/`, `/etc/`, `/usr/`, `/var/lib/docker/` or the default database directories, whatever the `config` says — the `exclude` entry adds to that set and cannot subtract from it
- [ ] Every file Candidate's `fingerprint` is `{size, mtime, inode}` read from one `stat`, with `inode` serialized as a decimal **string**, and ticket 07 can revalidate it with one `stat` and no walk
- [ ] A file in a sticky directory that this process does not own has no tick box, and its tooltip names the syscall and the uid in words
- [ ] Old kernels are absent from the code and the absence is explained in the ticket's Notes and in the Plugin's `README.md`
- [ ] Tests walk a temp tree fixture: the `*.log`/`*.gz`/`*.N` set and what it excludes, the age boundaries, the never-Candidate prefixes, the tmpfs and hard-link byte arithmetic, and one Candidate's full JSON shape

## Notes

### Files this ticket owns

Under `apps/api/plugins/disk-space/src/` (the Plugin's own Go module — see ticket 05's Notes for the layout, the build script and the `./backend` binary):

- `config.go` — the config keys, their defaults, the decode and the `configWarning`.
- `catalog.go` — the Category and Candidate envelope, `candidateId`, and the shared Capability helpers. Tickets 01–04 create the endpoint; this ticket owns the envelope's fields.
- `walk.go` — the hot index over the Scan Roots, the path→Disk mapping, and the never-Candidate prefixes. **The hot index is not the full walk**: the explicit, streamed, cancellable walk of a whole tree is ticket 08, and this file must not grow it. Keep the traversal behind one `scanRoot(...)` helper so ticket 08 replaces a body instead of forking one.
- `logs.go`, `caches.go`, `temps.go`, `large.go` — one file per Category except the journal, which lives in `logs.go` next to it because they share the `/var/log` walk but nothing else.
- `fingerprint_linux.go` and `fingerprint_other.go` — the inode, behind a build tag.

Ticket 05 owns `docker.go`; ticket 07 owns `plan.go`, `dryrun.go`, `apply.go` and `journal.go`.

### Config

`config` is free-form JSON on the Dashboard's Widget (`apps/api/internal/dashboard/dashboard.go`, `Widget.Config`) and it reaches the **widget** only: `apps/web/src/WidgetCard.svelte` passes it to `render(el, ctx)`, and core starts the backend with nothing but `STAR_PANEL_PORT` and `STAR_PANEL_PLUGIN` (`apps/api/internal/plugins/backend.go`). So the widget re-sends it on every request: the reads (`/summary`, `/candidates`, `/config`, `/journal`) take `?config=` plus URL-encoded JSON **and** also accept a `POST` whose body carries `{"config": {…}}` for a config too big for a query string; every mutation (`/plan`, `/plan/{id}/dry-run`, `/plan/{id}/apply`, `/walk`) is a `POST` with the config in its body. Nothing travels through core, because core has no config to carry. `ctx.fetch` now accepts an `init` ([types.ts](../../apps/web/src/types.ts), [WidgetCard.svelte](../../apps/web/src/WidgetCard.svelte)), but the widget's mutation path still builds its own `api(path, init)` and verifies the method it used: JavaScript ignores an extra argument, so against a core that was not rebuilt a second argument is dropped and an apply would silently become a `GET`.

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `scanRoots` | `[]string` | `["/var/lib/docker", "/var/log", "/var/cache", "/tmp", "/var/tmp"]` | The directories the hot index walks (D14). Absolute paths only. |
| `exclude` | `[]string` | `["/var/lib/mysql", "/var/lib/postgresql", "/var/lib/mongodb", "/var/lib/influxdb", "/var/lib/elasticsearch", "/var/lib/redis"]` | Absolute directory prefixes added to the never-Candidate set. Additive only. |
| `logDays` | int | `30` | Age floor for `rotated-logs`, journal included. |
| `cacheDays` | int | `30` | Age floor for `package-caches`. |
| `tempDays` | int | `7` | Age floor for `/tmp` and `/var/tmp`. |
| `largeFileBytes` | int64 | `1073741824` | The floor for a `large-files` observation. |
| `roots` | `[]string` | `[]` | The on-demand full walk (D14). Belongs to tickets 01–04; this ticket only must not confuse it with `scanRoots`. |

- An absent key, a wrong type, a negative number, `logDays: 0`, or a `scanRoots` entry that is not absolute all fall back **for that key alone** to the default, and the response carries `configWarning: "config: logDays is not a positive integer; using 30. config: scanRoots[1] is not an absolute path; using the default roots."` The card renders that string in `--mute`. A malformed config never fails a request and never crashes a Category (DESIGN-NOTES, Configuration).
- **`exclude` can only add.** The effective never-Candidate set is the union of three things: the four hard-coded prefixes (`/home`, `/etc`, `/usr`, `/var/lib/docker`), the built-in database defaults above, and whatever the request's `exclude` names. A request that sets `exclude` **adds** to the first two and can never remove from them: `{"exclude": []}` does not open `/var/lib/mysql`, and `{"exclude": ["/home"]}` is a no-op. Say it in the Category's notes, because it looks like a bug to anyone who expects config to replace a default — and it is the property that makes D9 hold regardless of what a Dashboard's `config` JSON contains.
- Body cap: `http.MaxBytesReader` at 64 KiB. A config larger than that is a `400` with `{"error": "config is larger than 64 KiB", "code": "config-too-large"}` — the one config case that is an error, because it is not a config, it is an attack on the request parser.

### The envelope

`GET /candidates` answers the Categories — and accepts a `POST` with the same body when the config does not fit a query string. Ticket 05 adds `docker-images` and `docker-volumes` to this same shape.

```json
{
  "configWarning": "",
  "measuredAt": "2026-01-14T09:12:00Z",
  "disks": [
    { "disk": "/", "totalBytes": 107374182400, "freeBytes": 21474836480, "reclaimableBytes": 4831838208 }
  ],
  "reclaimableBytes": 4831838208,
  "dockerEstimateBytes": 0,
  "dockerEstimateLabel": "",
  "categories": [
    {
      "id": "rotated-logs",
      "title": "Rotated logs",
      "state": "ok",
      "stateReason": "",
      "stateFix": "",
      "selectable": true,
      "confirmPhrase": "",
      "measureBytes": 5368709120,
      "reclaimableBytes": 4831838208,
      "estimateBytes": 0,
      "estimateLabel": "",
      "estimateSource": "",
      "measuredAt": "2026-01-14T09:12:00Z",
      "notes": ["30 days or older is the default; set logDays in the Widget's config to change it."],
      "candidates": [ /* Candidate */ ],
      "observations": [],
      "skipped": [
        { "path": "/home/ana/.npm/_cacache", "reason": "/home is never offered for removal" }
      ]
    }
  ]
}
```

**Candidate** — a file, a directory, or (ticket 05) a command:

```json
{
  "id": "cl_9f2c1a4b7d3e",
  "category": "rotated-logs",
  "kind": "file",
  "label": "/var/log/syslog.2.gz",
  "path": "/var/log/syslog.2.gz",
  "bytes": 1179648,
  "bytesKnown": true,
  "reason": "rotated log, last written 41 days ago (limit 30)",
  "command": ["rm", "-f", "--", "/var/log/syslog.2.gz"],
  "fingerprint": {
    "size": 1179648,
    "mtime": "2025-12-01T03:04:05.123456789Z",
    "inode": "348192"
  },
  "capability": { "ok": true },
  "selectable": true,
  "frees": "disk",
  "disk": "/var",
  "links": 1
}
```

- **A file Candidate's `id` is derived, not random**: `"cl_" + hex(sha256(category + "\x00" + path))[:12]` — `id` is spec §3's field name, and a Plan entry carries the same value under `candidateId`. A **command** Candidate does not hash anything: its `id` is the stable argv form the Docker Categories register, `docker-images:dangling` or `docker-volumes:all`, which is unique, readable in a journal and stable for free. It must be stable across two measurements of an unchanged machine, because the widget holds ids in a Selection and a random id would invalidate it on every re-measure — and because it is what makes "the same Plan" in D7/D8 mean something.
- **`inode` is a decimal string.** Inodes exceed 2^53 on XFS and overlayfs, and JSON numbers are IEEE doubles in the widget's `JSON.parse`: an id that round-trips through a double is no longer the inode. `size` stays a number (bytes, far below 2^53).
- **`mtime` is RFC 3339 with nanoseconds**, and ticket 07 compares the *exact* value it measured, not a truncated second. Two writes inside one second are exactly the case the fingerprint exists for.
- The fingerprint is read from **one `stat`** (`os.Lstat` → `syscall.Stat_t` for `Ino`, `Nlink`, `Uid`). `fingerprint_linux.go` carries the linux implementation; `fingerprint_other.go` (same build tag style as `apps/api/stats_linux.go` / `stats_notlinux.go`) returns inode `"0"` with `bytesKnown` untouched, so a developer's Windows build of the panel still compiles and `go vet ./...` stays clean — only `linux/amd64` is a promise (D13).
- **`command` is an argv array and never a string.** No shell, ever: `["rm","-f","--",path]`. A filename with a space, a quote, a newline or a leading `-` cannot become anything but one argument, and `--` stops the leading-dash case for the `rm` we do not shell out for anyway. The Dry Run prints exactly this array, joined for reading with each element shell-quoted, and the apply step executes exactly this array — the printed line and the run line are the same bytes, which is the whole point of recording it.
- `selectable: false` with a **null `id`** means a row is not a Candidate at all; it is an observation (below). A Candidate with `capability.ok == false` keeps its `id` — it was measured — but carries `selectable: false`, and the widget gives it no tick box (D5).

**Observation** — `large-files` only:

```json
{
  "path": "/srv/media/archive.tar",
  "bytes": 42949672960,
  "bytesKnown": true,
  "mtime": "2025-06-02T11:03:00Z",
  "inode": "918273",
  "links": 1,
  "disk": "/srv",
  "reason": "large file: 40.0 GB (floor 1.0 GB)",
  "note": "Big, not dead — review this yourself. Star Panel will not remove this."
}
```

### The walk, and what is measured

- The hot index walks **the declared Scan Roots only** (D14) and is what `GET /candidates` reads. Ticket 03 owns that index and ticket 08 owns the full-Disk walk (`POST /walk`, streamed, explicit); this ticket owns **what a walk is allowed to turn into a Candidate**, which is the filter below. A full walk never reaches Candidate generation at all — it is a measurement, not a Recommendation ([spec §3](../spec.md), ticket 08).
- **Never walked at all:** `/proc`, `/sys`, `/dev`. Walking `/proc` is how a disk tool hangs on a machine with many processes.
- **Never a Candidate, hard-coded, not configurable:** `/home/`, `/etc/`, `/usr/`, `/var/lib/docker/` (D9). Also, by the same reasoning and listed as a default in `exclude` so an owner can see them and move a database they really did put somewhere else: `/var/lib/mysql/`, `/var/lib/postgresql/`, `/var/lib/mongodb/`, `/var/lib/influxdb/`, `/var/lib/elasticsearch/`, `/var/lib/redis/`. They are **measured** — their bytes are in `measureBytes` and in the Disk bars — and never offered.
- Path comparison for these prefixes is done on a `filepath.Clean`ed path with a trailing separator appended to the prefix, so `/home` never matches `/homework`. Symlinks are resolved with `filepath.EvalSymlinks` **before** the check, because `/var/run` → `/run` and an `exclude` that only works on the un-resolved path is a hole. A resolved path that fails `EvalSymlinks` (a dangling link) is skipped, not treated as clean.
- **A symlink is never a Candidate.** Its size is its target's, its removal frees nothing, and following it points outside the Scan Root. It appears in `skipped` with the reason "symbolic link; the file it points at is elsewhere". Directories are walked with `os.Lstat` semantics for entries — `filepath.WalkDir`'s `DirEntry` is used so an entry's type comes from the readdir, not from a follow-up `stat`.
- **Hard links are counted once.** `Nlink > 1` means the bytes may be shared with a path we cannot see; the Candidate is offered (its removal is real — one link goes away) but carries `"frees": "none"` and is excluded from `reclaimableBytes`. Two Candidates that share an inode are counted once in `reclaimableBytes` and the second one is `"frees": "none"`. This is D12's overlap, concretely: `Reclaimable` is not the sum of what the Candidates occupy.
- **The Disk a Candidate is on** comes from the longest mount point in `/proc/self/mounts` that prefixes the resolved path (with a boundary check, so `/var` does not match `/varnish`), and its capacity and free space from `syscall.Statfs`. `st_dev` alone is not enough: two bind mounts of one filesystem have the same `st_dev` and different mount points, and the card names a Disk by its mount point. Resolve the path first — `/tmp` is a symlink to `/private/tmp` on some systems and to nothing special on Linux, but a bind-mounted `/var/log` is exactly the case that breaks an un-resolved prefix match.
- **A tmpfs is not a Disk saving.** If the Candidate's filesystem is tmpfs or ramfs (`f_type == 0x01021994` / `0x858458F6`), the Candidate keeps its tick box — the bytes are still waste — but carries `"frees": "memory"`, is excluded from every `reclaimableBytes`, and its row says in `--mute`: "on a memory filesystem: removing this frees RAM, not Disk." A `/tmp` that is a tmpfs is the common case and counting it as Disk space would be a lie about the number the whole Plugin exists to report.
- Directory sizes are computed during the walk, once, and cached with the Category (TTL as ticket 05's). Nothing re-walks a tree to answer a request.

### `rotated-logs`

| | |
| --- | --- |
| Qualifies | under any Scan Root, a file matching `*.log`, `*.gz`, `*.1` … `*.9`, older than `logDays` (default 30) |
| Acted | `rm -f -- <path>`, one Candidate per file, revalidated (D8) |
| Also | the systemd journal older than `logDays`, as **one** command Candidate: `journalctl --vacuum-time=<logDays>d` (`30d` by default) |

- The pattern set is what keeps the **live** logs out: `/var/log/syslog` and `/var/log/messages` have no suffix at all, and `/var/log/syslog.1` is matched while `/var/log/syslog` is not, so the rotating family is separable from the file being written to right now. `/var/log/auth.log` *does* match `*.log` — its protection is the age floor, and it is correct that a machine whose logger has been dead for 30 days offers its untouched log: nothing is appending to it. `/var/log/lastlog`, `/var/log/wtmp`, `/var/log/btmp` and `/var/log/faillog` match nothing and must stay matching nothing: they are binary accounting files, and a `rm` makes `last`, `who` and login accounting silently wrong. There is a test for exactly that, and a comment on the matcher saying why the pattern is not widened.
- `*.1` … `*.9` are spelled as a suffix test rather than nine patterns: take the tail after the last `.` and accept it when it is exactly one character in `1`–`9`. Do **not** use a `\.[0-9]+$` regex — it accepts `syslog.10` and `syslog.100`, which is a different rule from D9's and one that would grow the Recommendation every time a badly configured `logrotate` ran.
- The age is `mtime` and **only** `mtime`. Never `atime`: `relatime` is the default on every distribution that matters, so `atime` is a value that is sometimes days stale and sometimes discarded, and a log rotated but read yesterday would flip in and out of the Recommendation.
- **The journal is not a file Candidate, and this is the one place in the Plugin where a Category acts on two different kinds of thing.** Both halves live in the same Category and both are in the same Plan, so the two Capability stories and the two Dry Run readings must be kept apart in the code and in the output:

  | | `rm` half | journal half |
  | --- | --- | --- |
  | Capability | write+execute on the **containing directory** (not on the file), plus the sticky-bit owner test below | write access to `/var/log/journal/` **and** a reachable journald — `journalctl` runs as this process and does the vacuuming itself |
  | Missing-Capability words | `EACCES unlinking /var/log/syslog.2.gz: uid 10001 cannot write to /var/log` | `Failed to vacuum journal: Permission denied` (the CLI's own sentence) |
  | Fix | "Run the panel with a group that may write `/var/log`, or mount that directory writable for it." | "Run the panel with the `systemd-journal` group, or mount `/var/log/journal` writable." |
  | Dry Run entry | `outcome: "would-remove"`, the file's bytes, the exact argv | `outcome: "would-run"`, the argv, and the estimate below |
  | Estimate | the file's own size, exact | sum of `*.journal` files under `/var/log/journal/*/` whose `mtime` is older than `logDays` |
  | What can go wrong | the file changed → skipped (ticket 07) | nothing is revalidated at all: the daemon may have vacuumed since, so the apply step reports the delta |

- The journal estimate is honest about what it is: a journal file's `mtime` is its last write, so a file untouched for 30 days holds only entries older than 30 days, and `--vacuum-time` removes whole files whose newest entry is older than the cutoff. The estimate is therefore close to exact for the *files*, but `journalctl --vacuum-time` never splits a file: the estimate is presented as "about", and the apply step's delta is the number that counts. `journalctl --disk-usage` is **not** the estimate — it reports the whole journal, including the last 30 days, and using it would overstate `Reclaimable` by whatever the owner wanted to keep.
- Nothing under `/var/log/journal/` is ever a `rm` Candidate. Removing journal files behind a running journald leaves the daemon writing into unlinked files and the space is not freed until it restarts; the vacuum exists precisely because the journal is a binary store with an owner. Spell that out in the code comment and in the README, because "why not just `rm`" is the first question anyone asks.
- `journalctl` is not declared in `requires`: the same machine that has no journal has no `journalctl`, and a missing `journalctl` is a Category state (`no-cli`-style, in `--mute`, "the systemd journal is not readable on this machine; `journalctl` was not found"), not a reason to refuse to install the Plugin. `docker` is different because D6 makes the CLI a declared requirement of the *Widget's* usefulness.
- Keep the two halves in one Category, not two: D9 lists them together, both are "logs older than 30 days", and splitting them would put two rows in the dialog for one idea. The row for the journal says "systemd journal (vacuumed by `journalctl`, not deleted file by file)".

### `package-caches`

| Recipe | Root | Age | Acting |
| --- | --- | --- | --- |
| apt archives | `/var/cache/apt/archives/**/*.deb` | `cacheDays` | one file Candidate per `.deb`, `rm -f -- <path>` |
| pip, npm, pnpm, go build, cargo | the declared cache roots, below | `cacheDays` | one **directory** Candidate per root, `rm -rf -- <path>` |

- Declared cache roots, in this order, each included only when it exists: `$GOCACHE` (default `$HOME/.cache/go-build`), `$CARGO_HOME/registry` (default `$HOME/.cargo/registry`), `$XDG_CACHE_HOME/{pip,npm,pnpm}` (default `$HOME/.cache/{pip,npm,pnpm}`), `$HOME/.npm/_cacache`, `$PNPM_HOME/store`, `$npm_config_cache`, and these fixed ones that a container deployment is expected to bind a host cache onto: `/var/cache/pip`, `/var/cache/npm`, `/var/cache/pnpm`, `/var/cache/go-build`, `/var/cache/cargo`. `$HOME` comes from the process environment and is not assumed to be `/root`.
- **A cache root is a recipe, not a user rule** (D9): the list is compiled in, and the `config` can only exclude from it, never add to it. Users do not add Categories and they do not add cache roots.
- **The `/home` rule bites here, and the ticket says so out loud.** With D14's default Scan Roots the only recipe that qualifies is `/var/cache/apt/archives`; `~/.cache/go-build`, `~/.npm/_cacache` and `~/.cargo/registry` live under `/home`, and D9 lists `/home` as never a Candidate. So they appear in `skipped` with the reason "/home is never offered for removal", and the README tells the owner the way to have them: bind-mount the host cache under `/var/cache/<tool>` (or set `GOCACHE`/`npm_config_cache` to a path there) so it lies both inside a Scan Root and outside `/home`. Do not quietly widen the rule to `$HOME/.cache` because the recipe looks silly otherwise — D9 is binding, and the dialog saying why is better than the Plugin deciding for itself.
- A recipe whose root does not exist, or exists but is empty, is not a `skipped` row — it is simply absent, with a Category note naming which recipes were found ("caches found: apt archives").
- **`/var/cache/apt/archives/partial/` is never touched** — apt downloads into it — and neither is `lock`. Both must be explicit exclusions in the code, not an accident of the `*.deb` pattern, because `partial/` holds `.deb` files mid-download.
- Removing a cached `.deb` does not uninstall anything: the package stays installed and apt re-downloads the archive if it needs it again. Say it in the Candidate's `reason`, because it is the fear that stops people ticking it: "cached installer; the installed package is not affected".
- **Directory Candidates and their fingerprint.** A directory Candidate is a declared cache root, so its removal cannot lose data — the tool rebuilds it. Its `fingerprint` is `{size, mtime, inode}` of the **directory itself** from one `stat`, not of its tree: `size` is `st_size` of the directory inode, `mtime` is the directory's own, `inode` is the directory's. The recursive total lives in `bytes` and in `reason`, and is **not** re-measured at apply time. Ticket 07 revalidates a directory Candidate on **`inode` and `mtime` together**: a different inode means the path is now a different object and the entry is skipped; a changed `mtime` means something wrote into the cache since the Dry Run and the entry is **also** skipped, reported in words. The reason for the weaker-than-a-file check is that a changed cache is a smaller gain, never a loss — and the reason for not re-walking is D14: an apply step that re-walks a 40 GB Go build cache to save one `stat` is the I/O the whole design exists to avoid.
- Never call the tools' own clean commands — `apt-get clean`, `npm cache clean --force`, `pip cache purge`, `go clean -modcache`, `cargo cache`. They are all-or-nothing, they cannot be revalidated per Candidate (D8), they drag a runtime dependency we do not declare, and each of them changes what it means between versions of the tool. D9 says `rm` per file/dir and revalidated, and that is the rule this Category follows.

### `temp-files`

| Qualifies | Age | Acting |
| --- | --- | --- |
| `/tmp/**` and `/var/tmp/**` | `tempDays` (default 7), by `mtime` | `rm -f -- <path>` per file |
| `/var/crash/**` | no age floor | `rm -f -- <path>` per file |

- `/var/crash` holds crash dumps and `apport` report files; a dump is dead the moment it is written, which is why D9 gives it no threshold. The one guard: a file whose `mtime` is within the last hour is not offered, with the reason "a crash report written in the last hour may still be being written" — a Candidate that vanishes between the Dry Run and the apply is a skipped entry rather than a disaster, but offering a half-written dump is noise.
- **The sticky bit is a real Capability wall on `/tmp`.** `/tmp` is mode `1777`: anyone may write, but a file may only be unlinked by its owner, the directory's owner, or root. This process runs as uid 10001 (D5). So the Capability test for a file Candidate is not only "may I write the directory":

  ```
  parent := filepath.Dir(path)
  access(parent, W_OK|X_OK) must succeed
  if the parent directory has S_ISVTX and file.Uid != euid → no Capability
  ```

  The reason string names both facts in words: `EACCES unlinking /tmp/build.log: /tmp is a sticky directory and its owner is uid 1000, not 10001`. The fix is the concrete one: "Run the panel as that user, or add the group, or leave it — this file is another user's." The row keeps its size and loses its tick box (D5): a Candidate the process may not act on is shown, in `--mute`, without a box.
- The test is `unlink(2)` needs write **and** execute on the containing directory, and execute matters: a directory with write but no execute lets you create names and not remove them. `syscall.Access` with the real uid (not the effective one) is the right check for a non-setuid process, and it is cheap enough for every row.
- Everything above is stdlib `syscall` (`syscall.Stat_t`, `syscall.Statfs`, `syscall.Access`), behind the linux build tag. The Plugin's module adds **no dependency**: the repo's `go.mod` requires nothing, the Plugin is a separate module, and a plugin that drags `golang.org/x/sys` into an offline build of a homelab panel is a cost with no return on a `linux/amd64`-only target (D13).
- A `/tmp` or `/var/tmp` that is a tmpfs produces `frees: "memory"` rows (see the walk, above) — the common case, and the reason this Category must never add to a Disk's `reclaimableBytes` without checking the filesystem type first.
- `/var/tmp` is *not* cleared on reboot by design and holds things that are meant to survive one; the 7-day floor is what makes it safe, and the Category note says "7 days" so the owner can raise it.

### `large-files`

- Every file over `largeFileBytes` under a Scan Root becomes an **observation**: no `candidateId`, no `fingerprint`, no `command`, `selectable: false`, rendered in `--mute` with its size, `mtime` and Disk. D10's words are the row's `note`, and they are the whole design: a file this Plugin knows is *big* but not *dead* is the one thing it must never offer to remove, because the alternative is a tick box next to a 40 GB database dump.
- **The Category has no tick box ever** — not for a row, not for "select all". The dialog's Category header says it in `--mute`: "Recommendation only: Star Panel will not remove these."
- `/var/lib/docker/**` is excluded from observations too, and this is a deliberate reading: those are big *and* owned by the daemon, listing `/var/lib/docker/overlay2/…` as "review this yourself" is advice that leads straight to a corrupted store, and `docker-images` is where that space is explained and acted on (D6).
- `/swapfile`, `/swap.img`, `/dev/*` and anything under a never-walked prefix are excluded, with `/swapfile` named in the code and the README: a swap file is large, is supposed to exist, and deleting it at runtime is a machine-wide event, not a cleanup.
- Two observations pointing at one inode (a hard link) collapse to one row with "1 other link", so the list does not read as if the same 40 GB file were two problems.
- The model's job here is *reading*, so the list is ordered by size descending and the Category note says how many were found and from which floors: "12 files over 1.0 GB". No charts, no history (D14).

### Out of scope: old kernels, and why

`apt autoremove` can remove the kernel you are booted from; its heuristics depend on `apt`'s notion of which images are installed-versus-running, on `/boot` being a separate Disk that fills up first, and on a bootloader update that a containerised panel cannot complete. Putting that behind a tick box on a machine reached over SSH — where the recovery is a serial console or a rescue image — is disproportionate power for the bytes it frees, and the failure mode is not a lost file, it is a machine that does not come back. So no kernel recipe exists in v1, there is no `apt` Candidate that could reach `/boot`, `/boot` itself is not a default Scan Root, and old kernels surface only if an owner declares a root that contains them and a kernel image happens to be over `largeFileBytes` — where it is an observation, in `--mute`, with no tick box, which is exactly the right amount of power. The Plugin's `README.md` says this in one sentence so the omission reads as a decision rather than an oversight.

### Tokens

`var(--mute)` for every `skipped` reason, every `stateReason`/`stateFix`, the `large-files` rows and the Category notes; `var(--ink)` for paths and figures; `var(--body)` for a Category's qualifying sentence; `var(--border-soft)` between rows; `var(--surface-soft)` behind a `command` line or a disabled tick box; `var(--danger)` only where a removal actually failed (ticket 07) and on the `docker-volumes` Category (ticket 05). Type from `var(--text-meta)` and `var(--text-base)` only, `font-variant-numeric: tabular-nums` on every size, age and count. No literal colour, no literal radius, no shadow except the dialog's `var(--popover-shadow)` (D4).

### Traps

1. `--` before every path in every argv, and never a shell. A path is data.
2. `atime` is not a timestamp you may use. `mtime` only.
3. The age comparison is against the walk's single "now", captured once per Catalog: comparing each Candidate to `time.Now()` gives a Recommendation that changes while it is being built.
4. `EvalSymlinks` before the never-Candidate test, or a symlink is a way around `/home`.
5. `os.Lstat` for the fingerprint, `os.Stat` never: a symlink's target has a different inode and a different size, and following it is how a Candidate ends up naming a file it did not measure.
6. The fingerprint's `mtime` is nanosecond-exact, and the comparison in ticket 07 is on the exact value. A second-granularity comparison is the bug that removes a file rewritten within the same second.
7. A `.deb` under `/var/cache/apt/archives/partial/` is a download in flight. Exclude the directory explicitly.
8. Directory Candidates revalidate on inode **and** mtime, and are never re-walked. Files revalidate on all three of size, mtime and inode.
9. `/var/lib/docker` is walked for its size and never for its contents as Candidates, from either direction — file Categories or `large-files` observations.
10. `exclude` adds; it never subtracts. Write the test that sends `{"exclude": ["/home"]}` and asserts `/home` is still refused.
11. A Category with an unreadable root is `state: "partial"` with the `EACCES` path in `stateReason`, and its other Candidates are still offered: the Plugin never fails as a block because one Scan Root is unreadable (D5).
12. The journal's `*.journal` walk is `/var/log/journal/*/*.journal`, not a recursive glob: the leaf directories are machine ids and a recursive walk would also reach archived subdirectories the vacuum has its own opinion about. Both are read for the estimate; only the vacuum removes them.
