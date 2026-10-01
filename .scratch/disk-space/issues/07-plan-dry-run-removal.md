# 07 — Plan, Dry Run and removal

Status: ready-for-agent

## What

Build the engine that answers a Plan in two modes over the same Plan: measure (the Dry Run) and apply. Apply without a prior Dry Run of that same Plan is a refused request — the backend answers `409`, in the engine, not in the UI (D7, [ADR-0010](../../../docs/adr/0010-every-plan-is-a-dry-run-first.md)). Apply revalidates every file Candidate's fingerprint, removes the unchanged ones, skips the changed ones and says so in words, reports the delta for the command Categories that cannot be revalidated at all (D8), refuses anything that is not a Candidate the Plugin itself measured (D2), and appends one record per removal to a removal journal.

## Why

This is the whole guarantee of [ADR-0010](../../../docs/adr/0010-every-plan-is-a-dry-run-first.md) and the reason the Plugin is allowed to delete anything at all. Without it the Plugin is a browser with a `Remove` button; with it written anywhere but the engine — a disabled button, a hidden route — the guarantee is a UI preference, and the next widget, script or `curl` proves it was never a guarantee. D2 makes the same point from the other side: after the Plugin is a folder, core never sees the Candidate list, so the only thing standing between a request and a person's file is the check in this ticket.

## Acceptance

- [ ] `POST /plan/{id}/apply` on a Plan that has never been answered as a Dry Run answers `409` with `{"error": …, "code": "dry-run-required"}`, proved by an HTTP test that never opens the widget
- [ ] A Plan is immutable once built, a Dry Run never rewrites a fingerprint, and a changed Selection is a new `planId` with its own mandatory Dry Run
- [ ] Apply revalidates every `kind: "file"` and `kind: "dir"` entry against the fingerprint the Plan carries, removes the unchanged ones with the entry's own argv, skips the changed and the gone ones, and does not abort the rest of the Plan
- [ ] The apply response contains the sentence "3 of 12 changed; they were not touched." for a Plan of twelve with three changed Candidates, with the singular form for one
- [ ] A request body can name only `candidateId`s — there is no field for a path — an unknown id is a `400` with `code: "not-a-candidate"`, and an unknown JSON field in the body is a `400` rather than a silently ignored path
- [ ] Command entries report `estimateBytes` from the Dry Run against `reclaimedBytes` from the CLI's own output, and under-delivery is stated in words, never rounded away or hidden
- [ ] A `docker-volumes` Plan cannot be built without the exact phrase the backend supplied in the Category's `confirmPhrase`, and the refusal is a `409` from the engine
- [ ] No request can produce a second apply of the same Plan: `409` with `code: "already-applied"`, and a concurrent apply gets `409` with `code: "apply-in-flight"`
- [ ] A Dry Run performs no removal and no prune — a test asserts the fixture tree is unchanged and that the fake `docker` script was never called with a prune subcommand
- [ ] Apply is sequential, ordered deterministically, and takes one `statfs` reading per affected Disk before the first removal and one after the last; the response reports `disks[].freedBytes` per Disk and says in words when it disagrees with the sum of the removed Candidates' sizes
- [ ] Every entry the apply step settles appends exactly one JSON object to `journal.jsonl`, `fsync`ed before the response, with its result; a journal that cannot be written is a `warnings` entry, never a swallowed failure and never a reason to stop removing
- [ ] The widget shows a `--mute` "starting…" state across a backend restart, retries once, and reaches for `--danger` only when the retry fails too

## Notes

### Files this ticket owns

Under `apps/api/plugins/disk-space/src/`:

- `plan.go` — Plan construction from a Selection, `candidateId` resolution against the measurement snapshot, coalescing, ordering, the immutable Plan record and its bounded store.
- `dryrun.go` — measure mode.
- `apply.go` — revalidation, removal, the `statfs` readings, the response.
- `journal.go` — the append-only record.
- `plan_test.go`, `apply_test.go`, `journal_test.go`.

Ticket 05 owns `docker.go`, ticket 06 owns `config.go`, `catalog.go`, `walk.go` and the four Category files; tickets 01–04 own `main.go`, the endpoints' registration, `widget.js` and `manifest.json`.

### The two modes over one Plan

```
GET  /summary                 → the Disks and the Reclaimable total (tickets 01–04)
GET  /candidates              → the Categories and their Candidates (tickets 01–06)
POST /plan                    → 201, builds the Plan and stores it
POST /plan/{planId}/dry-run   → 200, measure mode, returns and stores the report
POST /plan/{planId}/apply     → 200, apply mode, requires the Dry Run above
GET  /journal?limit=50        → the tail of the removal journal
```

Every path above is reached through the Plugin's proxy: a request to `/api/v1/plugins/disk-space/proxy/plan` arrives as `/plan` (D2, `docs/PLUGINS.md`). The two reads take the widget's config as `?config=` (and also accept a `POST` body when it does not fit a query string, so a `POST /candidates` is legitimate); `/plan`, `/plan/{id}/dry-run` and `/plan/{id}/apply` are `POST`s carrying the config in the body, because core does not carry it (ticket 06, "Config").

**The Plan** (`POST /plan` body and `201` answer):

```json
{
  "planId": "pln_3f9c2a41",
  "state": "built",
  "builtAt": "2026-01-14T09:12:04Z",
  "dryRunAt": "",
  "appliedAt": "",
  "configHash": "sha256:6b1f…",
  "entries": [
    {
      "entryId": "e1",
      "candidateId": "cl_9f2c1a4b7d3e",
      "category": "rotated-logs",
      "kind": "file",
      "label": "/var/log/syslog.2.gz",
      "path": "/var/log/syslog.2.gz",
      "command": ["rm", "-f", "--", "/var/log/syslog.2.gz"],
      "bytes": 1179648,
      "fingerprint": { "size": 1179648, "mtime": "2025-12-01T03:04:05.123456789Z", "inode": "348192" }
    },
    {
      "entryId": "e2",
      "candidateId": "docker-images:all",
      "category": "docker-images",
      "kind": "command",
      "label": "everything Docker no longer needs",
      "command": ["docker", "system", "prune", "-f"],
      "estimateBytes": 4294967296,
      "estimateSource": "docker system df"
    }
  ],
  "entryCount": 2,
  "selectionBytes": 4296146944,
  "estimateBytes": 4294967296,
  "notes": [
    "all four docker-images Candidates are selected, so one docker system prune -f runs instead of four commands: it removes exactly stopped containers, unused networks, dangling images and unused build cache."
  ]
}
```

Request:

```json
{
  "config": { "scanRoots": ["/var/log"], "logDays": 30 },
  "selection": [
    { "category": "rotated-logs", "candidateId": "cl_9f2c1a4b7d3e" },
    { "category": "docker-images", "candidateId": "docker-images:dangling" },
    { "category": "docker-images", "candidateId": "docker-images:containers" },
    { "category": "docker-images", "candidateId": "docker-images:networks" },
    { "category": "docker-images", "candidateId": "docker-images:build-cache" }
  ],
  "confirmation": ""
}
```

- **There is no field for a path, and there must never be one.** A request names what the Plugin measured; the engine copies `path`, `command`, `bytes` and `fingerprint` out of its own snapshot. The decoder is `json.Decoder` with `DisallowUnknownFields()`, so a body carrying `"path": "/etc/passwd"` is a `400` with `code: "unknown-field"` instead of a field that is ignored today and honoured by somebody's refactor tomorrow. That, plus resolving every id against the snapshot, is what "validated against the Candidate set the Plugin itself measured" means in code (D2).
- **The snapshot is the Catalog the Plugin last measured**, kept in memory per Category with the same TTL as ticket 05's. If an id is not in it: `400`, `{"error": "candidate cl_deadbeef is not in the current measurement; only Candidates the Plugin measured can be planned", "code": "not-a-candidate"}`. If the `category` and the id disagree: `400`, `code: "category-mismatch"`. An empty or missing `selection`: `400`, `code: "empty-selection"`. Duplicate ids are de-duplicated, not an error.
- **The Plan is immutable.** Once built, `entries` never change: not by a second `POST /plan`, not by a Dry Run. This is what makes "the same Plan" in D7 a thing rather than a phrase, and it is why a Dry Run compares against the fingerprint the Plan carries instead of re-baselining it. To re-baseline a Candidate that has changed, the owner asks for a new measurement — which is a new Plan with a new `planId` and its own mandatory Dry Run.
- **Plans live in memory only.** D14 forbids state on disk, and there is a second reason: core restarts a dead backend every two seconds (`restartBackoff` in `apps/api/internal/plugins/backend.go`) and the first call after a restart waits up to `startWaitTimeout` (3s). So a restart between the Dry Run and the apply loses the Plan. Apply then answers `404` with `code: "plan-not-found"`, the widget says "the Plugin restarted; review again" in `--mute`, and the owner presses `Review` once more. An unknown id is **never** treated as "no Dry Run needed": the default for a missing Plan is refusal, always. Keep the store bounded — the last 8 Plans, 30-minute idle expiry — so a long-running panel does not accumulate Plans for Candidates that no longer exist.
- **Coalescing, and why it is not an optimisation.** Ticket 05 gives `docker-images` four disjoint Candidates. When a Selection contains **all four**, the Plan carries one entry whose command is `docker system prune -f`; otherwise it carries one entry per ticked Candidate, in the order stopped containers → unused networks → dangling images → build cache. Two reasons, and both are honesty rather than speed: the four dedicated prunes are exactly what `docker system prune -f` does (its own warning list is stopped containers, networks not used, dangling images, unused build cache), so the union's estimate is the union's estimate and not a sum of four overlapping ones; and running them one after another makes each command's delta depend on the one before it, since pruning a stopped container can make an image dangling. The same rule applies to `docker-volumes`, whose single bulk Candidate **is** the coalesced form of every unused volume. Nothing else coalesces, and the rule is stated in the Plan's `notes` when it fires, so the Dry Run shows the command that will actually run — which is the requirement D7 puts on the Dry Run in the first place.
- **Ordering** is deterministic: by the Catalog's Category order, then by `path` ascending, then by `entryId`. The same Selection on the same measurement produces the same Plan byte for byte, which is what makes the tests readable.

### The Dry Run (measure mode)

```json
{
  "planId": "pln_3f9c2a41",
  "mode": "dry-run",
  "finishedAt": "2026-01-14T09:12:05Z",
  "entries": [
    { "entryId": "e1", "candidateId": "cl_9f2c1a4b7d3e", "category": "rotated-logs",
      "outcome": "would-remove", "label": "/var/log/syslog.2.gz",
      "command": ["rm", "-f", "--", "/var/log/syslog.2.gz"], "bytes": 1179648, "detail": "" },
    { "entryId": "e2", "candidateId": "docker-images:all", "category": "docker-images",
      "outcome": "would-run", "label": "everything Docker no longer needs",
      "command": ["docker", "system", "prune", "-f"], "estimateBytes": 4294967296, "detail": "" }
  ],
  "wouldRemoveCount": 1,
  "wouldRunCount": 1,
  "changedCount": 0,
  "goneCount": 0,
  "wouldFreeBytes": 1179648,
  "estimateBytes": 4294967296,
  "notes": [
    "Docker reports each image's reclaimable size on its own; layers are shared between images, so the sum is larger than what a prune actually frees."
  ]
}
```

- **A Dry Run mutates nothing.** For a file entry it is one `stat` per Candidate and a comparison against the Plan's fingerprint, classified `would-remove`, `changed` or `already-gone`. It does not open, touch, `utime` or read the file, and it does not run a walk. Its only side effect is storing the report on the Plan and setting `dryRunAt`.
- For a command entry it performs the **read half only** — the `docker system df` (and `docker system df -v`, for the volumes Category) snapshot — and prints the argv it *would* run. It never invokes a `… prune` subcommand. The test that proves it is a fake `docker` script that appends every invocation to a log file and exits 1 on any prune subcommand: after a Dry Run the log holds only `system df`.
- `wouldFreeBytes` counts distinct inodes once and excludes anything `frees: "memory"` or `frees: "none"` (ticket 06), so the Dry Run's file figure and the apply's are computed by one function and cannot drift apart.
- The Dry Run may be requested more than once; it is a measurement, so repeating it is harmless. It still never rewrites a fingerprint.
- **`dryRunStaleAfter`.** D8 accepts that a command Category cannot be revalidated — "the daemon may see something else in between" — and that is an argument about seconds. A Plan with at least one `kind: "command"` entry whose Dry Run is older than `15 * time.Minute` answers `409` with `code: "dry-run-stale"` and `{"error": "this Plan's Dry Run is 4 hours old and Docker cannot be revalidated; review it again"}`. File entries are exempt, because their fingerprints *are* the revalidation. This is the one rule this ticket adds on top of D7/D8, and it is there because the alternative is a Plan that takes a fifteen-minute-old preview as a promise about a daemon that has been building images since.

### Apply (apply mode)

```json
{
  "planId": "pln_3f9c2a41",
  "mode": "apply",
  "finishedAt": "2026-01-14T09:12:07Z",
  "entries": [
    { "entryId": "e1", "outcome": "removed", "command": ["rm", "-f", "--", "/var/log/syslog.2.gz"],
      "bytes": 1179648, "detail": "" },
    { "entryId": "e2", "outcome": "skipped-changed", "command": ["rm", "-f", "--", "/var/log/syslog.3.gz"],
      "bytes": 0, "detail": "mtime 2026-01-14T08:55:02Z, was 2025-12-01T03:04:05Z; size 134217728, was 1179648" },
    { "entryId": "e3", "outcome": "ran", "command": ["docker", "system", "prune", "-f"],
      "estimateBytes": 4294967296, "reclaimedBytes": 3221225472,
      "detail": "Total reclaimed space: 3.221GB" }
  ],
  "entryCount": 12,
  "removedCount": 9,
  "skippedChangedCount": 3,
  "skippedGoneCount": 0,
  "refusedCount": 0,
  "failedCount": 0,
  "removedBytes": 5368709120,
  "reclaimedBytes": 8590131200,
  "estimateBytes": 9663676416,
  "disks": [
    { "disk": "/var", "beforeFreeBytes": 21474836480, "afterFreeBytes": 26731753472, "freedBytes": 5256926992 }
  ],
  "summary": "9 of 12 removed; 3 of 12 changed; they were not touched.",
  "warnings": ["Docker reclaimed 3.0 GB of the 4.1 GB its estimate promised."]
}
```

*(The `entries` array above is abbreviated to three of the twelve the counts describe.)*

- **Sequential and single-flight.** One entry at a time, in the Plan's order, under a `sync.Mutex` held for the whole run. A second apply for the same `planId` while the first is running is `409` `apply-in-flight`; a second apply after it finished is `409` `already-applied`. Both are cheap and both close a real hole: the Go HTTP server serves concurrently, so "apply was called once" is an assumption nothing else enforces.
- **Revalidation, and what a skip means.** For `kind: "file"`, re-`Lstat` and compare all three fields of the fingerprint. For `kind: "dir"`, compare `inode` and `mtime` (ticket 06 says why the recursive size is not re-measured). Unchanged → run `command`. Different → `skipped-changed`, with `detail` naming the field that moved and both values, in words an owner can read. `ENOENT` → `skipped-gone`, which is not an error: the Dry Run said it was there and the world moved. **The Plan is not aborted.** A skipped Candidate is a good failure — the alternative, aborting a twelve-file Plan because one log was written to, trains the owner to stop using the Dry Run at all.
- The sentence, exactly, in `summary`: `"%d of %d changed; they were not touched."` joined with `"%d of %d removed."`; with one changed Candidate it is `"1 of 12 changed; it was not touched."`. The phrase matters more than it looks: it is the only thing that tells a person their file is still there. Build it from `changedCount` and `entryCount`, never from the abbreviated `entries` the response happens to carry.
- A removal that fails (`EACCES`, `EROFS`, a CLI exit code) is `outcome: "failed"`, `detail` carries the syscall and the path in words — `EACCES unlinking /var/log/syslog.2.gz: uid 10001 cannot write to /var/log` — and the run continues. `failedCount > 0` makes the widget render those rows in `--danger` (D5: real failures get `--danger`, "you may not touch this" is `--mute`), and the rest of the summary stays in `--ink` and `--mute`.
- `refusedCount` is for a command the CLI itself refuses: `docker volume rm`-style in-use races, or a Docker CLI that answers "volume is in use". It is reported in words and is not a failure of the panel — it is the CLI's rule working, which is exactly what D6 wants.

### The delta for command Categories

`kind: "command"` entries cannot be revalidated at all, so the Dry Run's estimate is the only preview and the apply step's job is to say what actually happened:

- `reclaimedBytes` comes from, in order: the CLI's own summary line in stdout — `Total reclaimed space: 3.221GB` — parsed with the **decimal** units Docker formats with; else the difference of the matching row's `Reclaimable` in a `docker system df` taken immediately after the command against one taken immediately before; else `null` with `"reclaimedKnown": false`. Never `0`, which would read as "it freed nothing" when the truth is "we could not tell".
- Under-delivery is the expected case and is reported as a sentence, not a percentage: "Docker reclaimed 3.0 GB of the 4.1 GB its estimate promised." Over-delivery is reported too, because it is the same honesty running the other way ("it reclaimed 1.2 GB more than its estimate: layers shared with images it did not remove were freed as well"). Both come from one comparison and both go in `warnings` as well as in the entry's `detail`, so an owner who reads only the top of the response still sees it.
- `estimateBytes` for the whole response is the sum of the Plan's command estimates plus `wouldFreeBytes` from the file entries; `reclaimedBytes` sums the same way. The two are shown side by side and never merged into one figure, because one is what the Plugin removed and the other is what the tools said they removed — D12's point, at the level of a Plan.

### `Reclaimable`, and the one number that is not a sum

- Before the first removal the apply step takes `statfs` on each Disk the Plan's file entries touch, and after the last one it takes them again. `disks[].freedBytes` is the difference, and it is the most defensible answer to "what did this Plan actually free" that exists without walking anything: it is the filesystem's own account, so it is right about open handles, shared extents and filesystem overhead by construction.
- `removedBytes` (the sum of the removed Candidates' measured sizes) is reported **next to it**, never instead of it. When the two differ by more than 5%, one sentence says so in words: "the filesystem reports 1.2 GB freed; the removed files measured 2.0 GB — some of it was still open, or shared with a link elsewhere." That is D12 written as a line of output rather than as a philosophy.
- The `statfs` reading is a measurement of the Disk, not an attribution to this Plan, and the note says so: another process writing during the run moves it too. Say it in the response's `notes` rather than quietly presenting correlation as cause.
- Docker entries contribute no Disk: with `DOCKER_HOST` set the bytes are not even on this machine, and the reclaimed figure is reported without a Disk for the same reason (ticket 05).

### The removal journal

`apps/api/plugins/disk-space/journal.jsonl`, beside the backend binary, in the Plugin folder that is also its working directory (`docs/PLUGINS.md`: `command` runs with the Plugin's folder as cwd). One JSON object per line, appended, `fsync`ed before the response is written:

```json
{"at":"2026-01-14T09:12:07.123456789Z","planId":"pln_3f9c2a41","entryId":"e1","candidateId":"cl_9f2c1a4b7d3e","category":"rotated-logs","kind":"file","path":"/var/log/syslog.2.gz","command":["rm","-f","--","/var/log/syslog.2.gz"],"outcome":"removed","bytes":1179648,"error":""}
```

- **One record per removal is the requirement; one record per settled entry is what it does.** Skips and failures get a line too, because "why is my file still there" is the question the journal exists to answer, and a record that only holds successes answers it by silence. `outcome` is one of `removed`, `skipped-changed`, `skipped-gone`, `refused`, `failed`, `ran`.
- Written as each entry settles, not once at the end, and `Sync()`ed immediately: core kills and restarts this process on its own schedule (`restartBackoff` in `backend.go`), and a journal that is buffered until the end of a run is a journal that is empty for exactly the run that was interrupted.
- Opened `O_APPEND|O_CREATE|O_WRONLY`, one `write` of the full line plus `\n`, by a single writer under the apply mutex. Never rewritten, never truncated, never rotated: an audit record that deletes itself is not one. It grows by roughly 300 bytes per removal, so ten thousand removals is about 3 MB — state that in the README rather than adding a rotation nobody asked for (D14 keeps history out of the model, and this is not history: it is the only record that a removal was the one somebody ticked).
- If the append fails — a read-only Plugin folder, a full disk — the entry's result is still returned, and `warnings` carries "the removal journal could not be written (EACCES opening /app/plugins/disk-space/journal.jsonl): this removal is not recorded". It never blocks a removal, and it is never swallowed: a silent audit failure is worse than no audit at all.
- `GET /journal?limit=50` returns the last N parsed lines, newest last, with a `truncated` flag when the file holds more. A line that does not parse is skipped and counted in `unreadable`, so one corrupt line does not lose the file.

### Errors, exactly

Every non-2xx body carries `error` (a sentence in words) and `code` (a stable token), matching the shape core already uses for its own failures in `backend.go`'s `writeError`.

| Status | `code` | When |
| --- | --- | --- |
| 400 | `unknown-field` | the body names anything the decoder does not know — including a path |
| 400 | `empty-selection` | no Candidate ids |
| 400 | `not-a-candidate` | an id that is not in the current measurement |
| 400 | `category-mismatch` | the id exists in a different Category |
| 400 | `config-too-large` | a body over 64 KiB |
| 404 | `plan-not-found` | an unknown or expired `planId` |
| 409 | `dry-run-required` | apply on a Plan with no `dryRunAt` |
| 409 | `dry-run-stale` | apply on a command Plan whose Dry Run is older than 15 minutes |
| 409 | `confirmation-required` | a `docker-volumes` Selection with no or a wrong `confirmation` |
| 409 | `already-applied` | a second apply of an applied Plan |
| 409 | `apply-in-flight` | an apply already running for that `planId` |

`409` is for a Plan whose state does not allow the request; `400` is for a request that is malformed. Use them that way, because the widget distinguishes them: a `409` is shown with the words and a way forward ("Review again"), a `400` is a bug in the widget and is shown in `--danger`.

### The widget's side

- `Remove` appears only after `Review` has answered with both a Plan and its Dry Run report (D7), and the two calls happen in that order behind one gesture: `POST /plan`, then `POST /plan/{id}/dry-run`. Between them the dialog shows one `var(--mute)` line — "reviewing…" — and never a spinner (`DESIGN.md` §4: loading is `mute` placeholder text).
- The widget builds its own `api(path, init)` against `/api/v1/plugins/disk-space/proxy/` and does not route a mutation through `ctx.fetch`. `ctx.fetch` now accepts an `init` ([types.ts](../../apps/web/src/types.ts), [WidgetCard.svelte](../../apps/web/src/WidgetCard.svelte)), so this is not a workaround for a missing feature: JavaScript ignores an extra argument, so against a core that was not rebuilt the second argument is dropped and an apply would silently become a `GET`. `api()` is what makes a state-changing request impossible to downgrade, and it verifies the method that actually went out. An apply that can silently become a GET is not a thing to leave in the codebase, whatever the current core does.
- **Across a backend restart, `--mute`, never `--danger`.** Core answers `502` with `{"error":"plugin backend unavailable: …"}` while the subprocess is starting (`backend.go`), and the Plugin is restarted two seconds after any death with a first call waiting up to three seconds. So a `502` on `plan`, `dry-run` or `apply` means: show "the disk-space backend is starting…" in `--mute`, retry that one call once after `2s`, and only if the retry also fails render the failure in `--danger`. A `danger` flash on every plugin restart is how a working panel trains its owner to ignore `danger`.
- The dialog is ticket 04's native `<dialog>` (D4) and carries `var(--popover-shadow)`; everything inside reads its colours from `var(--ink)`, `var(--body)`, `var(--mute)`, `var(--danger)`, `var(--border-soft)` and `var(--surface-soft)` only. Numbers — byte figures, counts, ages, the `freedBytes` in the Disk line — carry `font-variant-numeric: tabular-nums`. No literal colour, no second accent, no shadow of the widget's own.
- `Remove` is a normal button. It is **not** a `danger`-filled one: `DESIGN.md` reserves `danger` for state, and a destructive action is not a state. The `docker-volumes` Category's *label* is `--danger` (D11); the button in the dialog stays the panel's standard control.
- The typed confirmation (D11) takes the phrase from the response — `categories[].confirmPhrase`, `"remove volumes"` — never a string in the widget, so the engine and the UI cannot disagree about what was typed. The input's label carries the Category's warning sentence, the confirm control is disabled (`--mute` text on `--surface-soft`) until the phrase matches, and the disabled state is courtesy only: the engine is what refuses, and the test that proves it posts a `docker-volumes` Selection with an empty `confirmation` and asserts the `409`.
- After an apply the dialog shows `summary` verbatim in `--ink`, each skipped row's `detail` in `--mute`, each failed row's in `--danger`, and `warnings` as a `--mute` list under a hairline. Nothing is summarised away: if the response says 3 of 12 changed, the three are in the list.

### Tests

`apps/api/plugins/disk-space/src/` is our own Go module, so its handler is an `httptest` server and no core change is needed to test any of this:

- **The guarantee**: `POST /plan` then `POST /plan/{id}/apply` with no Dry Run → `409 dry-run-required`. This test must not go through the widget, the dialog or any DOM: it is the statement that the rule is in the engine.
- **Immutability**: build a Plan, run a Dry Run, mutate the fixture file, run a second Dry Run, then apply → the mutated entry is `skipped-changed` and the Plan's stored fingerprint is byte-identical to the one it was built with.
- **The sentence**: a twelve-entry Plan with three mutated files produces `summary == "9 of 12 removed; 3 of 12 changed; they were not touched."`, and a one-changed variant produces `"1 of 12 changed; it was not touched."`.
- **No path in**: `{"path": "/etc/passwd"}` in the body → `400 unknown-field`; an unknown `candidateId` → `400 not-a-candidate`.
- **Dry Run mutates nothing**: a fixture tree with recorded mtimes, inodes and contents, plus a fake `docker` script logging its argv — after a Dry Run the tree is bit-identical and the log holds only `system df`.
- **Coalescing**: a Selection with all four `docker-images` Candidates produces one `docker system prune -f` entry; three of four produces three entries and no `system prune`.
- **The delta**: a fake `docker` that prints `Total reclaimed space: 3221225472B` against an `estimateBytes` of 4294967296 produces the under-delivery sentence with both figures in words.
- **The journal**: one line per settled entry, each line parses, `outcome` and `error` are present, and a fixture with an unwritable folder produces a `warnings` entry while the removal result is still returned.
- **Docker's argv guard**: walk every `command` array in every built Plan's entries and fail on the substring `--volumes` or the prefix `/var/lib/docker`.

### Traps

1. A Dry Run that re-baselines a fingerprint is the bug that makes the whole guarantee decorative. The fingerprint belongs to the Plan.
2. `409` from the engine, never from a disabled button. The widget's disabled `Remove` is a courtesy and the test must not go through it.
3. An unknown `planId` is a refusal, not a fresh start — the safe default for a missing Plan is `404`, always.
4. `fsync` per journal line. Without it, the interrupted run is the one run with no record.
5. `exec.CommandContext` kills the direct child only. `rm` is short-lived and safe; the Docker prunes are not — give them `Setpgid` and kill the group on timeout, or a cancelled apply keeps deleting after the response (ticket 05 has the same trap from the measurement side).
6. The plan store must be bounded and expired. An in-memory map keyed by `planId` with no eviction is a leak whose entries are Candidate lists holding file paths.
7. `reclaimedBytes` is `null` with `reclaimedKnown: false` when there is no source. `0` is a claim, and it is the wrong one.
8. The skip decision is taken from the Plan's fingerprint and the *current* `stat`, never from the Dry Run's reading: the Dry Run is a report, not a new baseline.
9. One `statfs` per affected Disk, taken once before the first removal and once after the last — not per entry. The per-entry version measures other processes' writes as if they were this Plan's removals.
10. The apply mutex is held for the whole run, and `already-applied` is set from `appliedAt`, not from the lock: a lock alone would let two sequential applies of one Plan both succeed.
