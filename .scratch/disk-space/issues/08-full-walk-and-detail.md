# 08 — Full walk and detail view

Status: ready-for-agent

## What

The detail interface opens instantly against the hot index and the Candidate list, and lets the owner ask, explicitly, for a full walk of a Scan Root; that walk is streamed as `text/event-stream` with progress, cancellable by closing the stream, capped and bounded, and it renders as a per-directory tree with totals plus the biggest files under the walked Scan Root. The walk is the expensive on-demand path (D14): it is asked for by the owner, never triggered by opening the dialog, never triggered by the card's poll, and its result never changes what the card shows.

## Why

D14 splits the measuring strategy in two: a cheap hot index over the declared Scan Roots that makes `/summary` and opening the dialog feel instant, and an explicit, streamed walk for the day someone actually wants to know where the space went. Without this ticket the Plugin can only explain waste it already knows how to name, and a `>1 TB` tree has no answer that does not block the panel — a synchronous walk would hold a proxy request for minutes and punish I/O on a machine reached over SSH. [spec §3](../spec.md) and [spec §6](../spec.md) fix the whole contract, including the caps: 200k files or 5 minutes.

## Acceptance

- [ ] `POST /api/v1/plugins/disk-space/proxy/walk` answers `text/event-stream` and the backend sees it as `POST /walk` (the proxy strips the prefix); the events are `progress`, `result`, `error`, `done`, in that grammar
- [ ] A `progress` event carries `scanned`, `bytes`, `current` (plus `workers` and the caps); a `result` event carries the tree, the biggest files, the skip counters and the interruption reason; `done` closes the walk
- [ ] Starting a walk is an explicit control inside the `<dialog>` ("Walk `/var/log` in full — takes minutes"), never called on `showModal()`, on the card render, or from a poll
- [ ] Opening the dialog issues no walk; the first paint comes from `GET <proxy>/summary` and `GET <proxy>/candidates` only (tickets 02, 03), and the walk section starts as an unstarted control
- [ ] Closing the dialog, pressing Cancel, or the widget's cleanup running aborts the fetch, and the backend stops walking on `r.Context().Done()`; no walk outlives the surface that asked for it
- [ ] An interrupted walk is reported as interrupted, in `mute`, with the reason in words and the number reached, and is never rendered as if it finished
- [ ] The tree shows per-directory totals with `font-variant-numeric: tabular-nums`; the biggest files are listed with their size, in `mute`, with **no tick box** (D10)
- [ ] The walker skips symlinks, does not cross filesystem boundaries by default, skips pseudo-filesystems, counts a hardlinked file once, survives `EACCES` on any subtree without failing, and never reports a path outside the walked Scan Root
- [ ] Concurrency is a bounded worker pool over `os.ReadDir`; no code path starts one goroutine per directory, and the queue is bounded so a wide tree cannot grow memory without limit
- [ ] The caps are the spec's: 200,000 files or 5 minutes, whichever comes first; the walk stops walking, still `result`s what it measured, and names the cap it hit
- [ ] No history exists anywhere: a walk writes no file, reports no scan-to-scan delta, and no copy in the dialog says "grew since" anything (D14)
- [ ] A backend restart mid-walk ends the stream and the widget says so in `mute` with a retry control, never `--danger` (core restarts a dead backend every 2s — `backend.go:21`)
- [ ] Every colour, radius and type size in the new markup comes from a Token; no literal colour, no second accent, no shadow other than `--popover-shadow` on the dialog
- [ ] A backend test drives the walk over a `t.TempDir()` fixture tree and asserts the tree, the dedup, the skip counters and each cap, rather than walking the machine it runs on

## Notes

### Where the code lives

- Backend: `apps/api/plugins/disk-space/src/` — the Go source folder, its own module (ticket 01: module `star-panel/plugins/disk-space`, `go build -C src -o ../backend .`). The source folder is `src/` and not `backend/` because `backend` is the **binary** at the Plugin root, and a file and a directory cannot share a name; the walker is `src/walk.go` and the stream writer sits beside the routes in `src/main.go`. This is the "cap the walk" work ticket 03 explicitly leaves here — that index uses the plain traversal and is not the fast walker.
- Widget: `apps/api/plugins/disk-space/widget.js` (the card) plus `detail.js` for the dialog, both framework-free ESM, no bundler, no Svelte (`docs/PLUGINS.md`).
- Core is read for the contract and **not edited**. `Backends.Handler` builds a `httputil.NewSingleHostReverseProxy` (`apps/api/internal/plugins/backend.go:102`) with no response buffering, so an SSE answer streams through as long as the backend flushes.

### The two paths, in this ticket's own words

| | Hot index (`/summary`, `/candidates`) | Full walk (`/walk`) |
| --- | --- | --- |
| When | every card poll, and dialog open | only when the owner presses Walk |
| Cost | a few `ReadDir` over the declared Scan Roots | minutes on a `>1 TB` tree, real I/O |
| Transport | one answer, one request | `text/event-stream`, progress streamed |
| Feeds the card | yes | never |
| Feeds `Reclaimable` | yes, for its Categories | no — a walk measures, it does not recommend |
| Built by | ticket 03, refreshed every 10 minutes or on a config change | this ticket, from scratch, every time |

A walk produces no Candidate and therefore no Selection and no Plan. `Reclaimable` stays a number the Plugin can defend (D12); the walk's total is "what is under this Scan Root", a different number, labelled as one.

### The stream contract

`POST /walk`, body (spec §3, plus the one optional field this ticket adds):

```json
{ "root": "/var/log", "maxDepth": 4, "topFiles": 50, "crossFilesystems": false, "config": {} }
```

- `root` is required and must be inside a declared Scan Root; anything else is `400 {"error": "/etc is not inside a declared Scan Root"}`. One walk is one root: a person who wants two walks asks twice, and the tree's totals stay answerable for the root they named. A root that does not exist is the same `400` with the path in the words.
- `maxDepth` defaults to `4` and is clamped to `12`; `topFiles` defaults to `50` and is clamped to `200`. `crossFilesystems` defaults to `false` and is per-request only — never a config default (D14, spec §6).
- `config` follows ticket 09: the widget passes it in the body because the write path carries it there, and the backend stores nothing between requests.

The answer, `200`, `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `X-Accel-Buffering: no`, one JSON object per event, a blank line after each, and a `: keepalive` comment every 15s so a slow tree does not look like a dead stream:

```
event: progress
data: {"scanned":41230,"bytes":91827364501,"current":"/var/lib/docker/overlay2/9a1f…","workers":4,"maxFiles":200000,"maxMillis":300000}

event: result
data: {"root":"/var/log","complete":false,"truncated":{"kind":"file-cap","limit":200000,"reached":200000,"at":"/var/log/journal","words":"stopped at the 200,000 file cap — the numbers below are a floor, not a total"},"elapsedMs":84000,"scanned":200000,"bytesCounted":91827364501,"skipped":{"symlinks":12,"otherFilesystem":4,"pseudoFilesystem":2,"hardlinks":118,"unreadable":1},"tree":{},"biggest":[],"biggestOverflow":37,"unreadable":[],"filesystem":{"totalBytes":1073741824000,"freeBytes":214748364800,"fs":"ext4"}}

event: done
data: {"ok":true}
```

- `progress` fires at most once a second — never per file and never per directory, because a `>1 TB` tree would spend the whole walk writing to the socket. `workers` and the caps ride along so the widget can show what it is inside.
- `truncated.kind` is one of `file-cap`, `time-cap`, `depth-cap`, `cancelled`, `unreadable-root`, and is absent on an uninterrupted walk. Hitting a cap stops the walking, not the answer: `result` still follows with what was measured, then `done`.
- `tree` is one node per directory, `{ "path", "name", "bytes", "files", "complete", "children": [] }`. `complete: false` means at least one descendant was not enumerated, so `bytes` is a floor. Only directories that were fully enumerated — plus the directories on the path to them — appear: a partial directory's bytes are not invented.
- `biggest` entries are `{ "path", "bytes", "mtime", "directory" }`, at most `topFiles`, with `biggestOverflow` when more qualified. `unreadable` entries are `{ "path", "errno", "words" }`, capped at 50.
- `error` is for a genuine failure (a root that vanished mid-walk, a `ReadDir` that failed with something other than `EACCES`/`ENOENT`) and is followed by `done { "ok": false }`. `EACCES` is never one: it goes to `unreadable`.
- `filesystem` comes from `statfs` on the walked root; it is context, not the sum of the tree. A walk does not add up to a Disk (CONTEXT.md **Disk**, D12).
- There is no `history` field, no `lastScan`, no timestamped snapshot: a walk keeps nothing (D14).

**The widget reads it with `fetch`, not `EventSource`.** The body carries `config` and the browser has to be able to abort, and `EventSource` can do neither — and `ctx.fetch` returns a buffered `Response`, which is the wrong shape for a stream. So build the URL and call `fetch` directly:

```js
const controller = new AbortController();
const res = await fetch("/api/v1/plugins/disk-space/proxy/walk", {
  method: "POST", headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body), signal: controller.signal
});
const reader = res.body.getReader();   // decode, split on "\n\n", parse "event:" / "data:"
```

`controller.abort()` is Cancel; the cleanup function a widget returns (`docs/PLUGINS.md`) calls it, so a card removed or re-rendered mid-walk stops the walk. The backend stops because `r.Context().Done()` fires when the browser closes the connection, and the proxy and `net/http` propagate that on their own.

A walk that was running when the dialog closed is **gone**, not resumed: reopening starts fresh. Nothing is resumed from disk, which is the same decision as "no history".

The 200,000 files and 5 minutes are defaults, not preferences. `maxFiles` is what keeps the `result` answer inside the memory this process is willing to hold; `maxMillis` is the longest walk a person sits in front of. Both are clamped server-side whatever the request says.

### The walker's rules

- **Symlinks are never followed.** `os.ReadDir`'s `DirEntry.Type()&fs.ModeSymlink`, and `Lstat` rather than `Stat` everywhere else; count it into `skipped.symlinks` and descend nowhere. Following one would double-count and would climb out of the Scan Root, which is the one thing a walk may not do.
- **Do not cross a filesystem boundary by default.** `os.Lstat` the child directory and compare `syscall.Stat_t.Dev` with the parent's; a difference means another filesystem, so count it into `skipped.otherFilesystem` and do not descend. `crossFilesystems: true` opts in for one walk only.
- **Pseudo-filesystems are skipped always**, boundary or not: build a denylist once per walk from `/proc/mounts` — `proc`, `sysfs`, `devpts`, `devtmpfs`, `tmpfs` under `/dev`, `cgroup`, `cgroup2`, `securityfs`, `debugfs`, `tracefs`, `mqueue`, `hugetlbfs`, `configfs`, `fusectl`, `binfmt_misc`, `nsfs`, `bpf`, `squashfs`, `ramfs`, `autofs`, `fuse.*`. It is the same list ticket 01 uses for the Disk listing, applied here to descending. They are not storage, and counting them makes the biggest-directory list nonsense.
- **Hardlinks count once.** Keep a `map[[2]uint64]struct{}` keyed by `(dev, ino)` for regular files with `Nlink > 1`; a repeat goes into `skipped.hardlinks` and contributes no bytes and no file. Two paths to one file are one file's worth of space, and counting both inflates a tree someone is about to trust.
- **`EACCES` never fails a walk.** A directory that cannot be listed, or a file that cannot be `Lstat`ed, is appended to `unreadable` with its errno, the subtree is marked `complete: false`, and the pool continues. `EACCES` on the **root itself** is a `result` with `tree` omitted, `truncated.kind: "unreadable-root"` and that one `unreadable` row — the stream still ends with `done { "ok": true }`, because the walk answered the question it was asked.
- **Nothing outside the walked root is ever emitted.** Every path in `tree`, `biggest` and `unreadable` is verified to be inside `root` with `filepath.Rel` before it enters the answer — the same guard `insideFolder` uses in `apps/api/internal/plugins/manifest.go:136`. A `..` in the root, or a root that is a symlink, is resolved once and refused at the door if it lands outside a declared Scan Root.
- **Nothing inside `/var/lib/docker` is ever a file Candidate** (D6). It is a legal place to walk and measure; whether a row gets a tick box is ticket 09's decision, and a walk never creates one.

### Concurrency, progress, cancellation

- One walk per backend process. A second `POST /walk` while one runs answers `409 {"error": "a walk is already running"}` rather than joining it: two owners on one stream is a later problem, and joining would hide whose Cancel it was.
- The pool is `min(4, GOMAXPROCS)` workers over a channel of directories, with a bounded queue (8192 entries) — when it is full the worker blocks, and that backpressure is what keeps memory honest. **Never `go walk(dir)` per directory**: `/var/lib/docker/overlay2` is millions of directories wide, and a goroutine each is an OOM, not parallelism. Blocking on a full queue is also what makes `r.Context().Done()` land within one directory listing instead of one file.
- Progress is a mutex-guarded struct that the event ticker reads once a second; the walk updates counters per directory and never logs per file. The `result` is assembled from the same accumulator once, after the walk stops.
- Cancellation paths, all ending in `r.Context().Done()`: the widget's Cancel button (`controller.abort()`), the dialog closing, the widget's cleanup function, and core shutting the backend down. The stream is simply not written to again; the walker returns at its next listing.
- A walk never touches the hot index's cache and never opens the removal journal (`state/journal.jsonl`, ticket 07). It writes no file at all.

### The detail interface (DESIGN.md §3, §5, §7; docs/PLUGINS.md Token contract)

Ticket 04 owns the dialog's Disks, Categories and Candidate rows. This ticket adds the walk section to it, after the Candidates:

1. **Full walk** — the explicit control, named after the root ("Walk `/var/log` in full"), with one `mute` line under it: `walks the tree in full — minutes on a big tree, and it does not change the card`. While running: Cancel (a ghost button) and one progress line.
2. **The tree** — indentation by depth, hairlines from `border-soft` on top-level separators only, no rules between rows, path in `body`, size in `ink` at `font-variant-numeric: tabular-nums`, file count in `mute` at `var(--text-meta)`. A row is a `<button>` only if it collapses; a leaf row is not interactive and is not focusable.
3. **The biggest files** — the `mute`, untickable review list (D10), one row per file with its size in tabular figures and the sentence `big, and only you know if it is dead`.
4. **Could not read** — the `unreadable` rows, in `mute`, one line each.

The progress line while running: `41230 files · 85.5 GB seen · 41s · /var/lib/docker/overlay2/9a1f…` — `--mute`, at `var(--text-meta)`, one line, truncated in the middle with `…` rather than wrapping, because a long path in this line would push the dialog around once a second. **Nothing animates on its own** (DESIGN.md §3): no spinner, no indeterminate bar; the numbers changing are the progress.

A `complete: false` node carries `≥` before its size and the described-by text `at least — a subtree could not be read`.

The interrupted lines, verbatim, because they are the honest-reporting point of the ticket:

- `file-cap`: `stopped at the 200,000 file cap — the numbers below are a floor, not a total`
- `time-cap`: `stopped at the 5-minute cap — the numbers below are a floor, not a total`
- `depth-cap`: `stopped at depth 4 — deeper directories are included in their parent's total`
- `cancelled`: `you stopped this walk — the numbers below are a floor, not a total`
- `unreadable-root`: `could not read /var/log (EACCES) — nothing was measured here`
- backend restart: `Walk interrupted — the backend restarted before it finished.` plus a `Walk again` control, in `mute`, never `--danger` (nothing on the machine failed; a subprocess restarted)

Sizing: the dialog is `width:min(960px, 100%)`, `max-height:80dvh`, header and footer fixed, **body scrolls** — and the scroll container is the body, not the dialog, because DESIGN.md §7 says the focus ring is never clipped. Under 640px the tree flattens to the current depth ±1 with a breadcrumb row.

### The card must not move

View mode is untouched by this ticket: the card still shows the Disk bars, the total Reclaimable and the `Clean…` button, and a walk changes none of those numbers and adds no line to the card. If a walk is running when the dialog closes it is cancelled, and the card must not gain a spinner, a badge or a second metadata line for a job the owner walked away from (DESIGN.md §4: in view mode a card shows a title, its body and at most one muted metadata line).

### Traps

- **The event grammar is not negotiable, and neither is flushing.** Write the headers before the first `progress`, `Flush()` after every event, and send a `: keepalive` comment when nothing has gone out for 15s — an unflushed `http.ResponseWriter` buffers the whole walk into one delivery at the end, which is exactly the synchronous behaviour D14 exists to prevent. Do not set `Content-Length`, and do not let anything write a JSON error body after the stream has begun: an error after the headers is an `error` event, not a status code.
- **Core's proxy error path answers `{"error": …}`.** If the backend is not listening, core answers `502` with the frozen shape (`backend.go:103-106`) and the widget's reader gets a JSON body, not an event stream. Check `res.ok` before reading the body and show that message in `mute`.
- **The walk does not go through `ctx.fetch`, and the count of reasons is three.** It needs a `POST` with a body, an `AbortController` it owns (closing the dialog must abort the stream), and a response it reads incrementally rather than as a whole `Response` — and `ctx.fetch(path, init?)` hands back a buffered `Response`, which is the opposite of what a stream is for. Build the URL and call `fetch` directly against the same absolute proxy path. See [spec §9.1](../spec.md) for why the widget also verifies the method it used.
- **A cap is not an error, and a `result` is not a promise of completeness.** The widget renders `truncated` before the tree, never after: a floor read as a total is the failure mode this ticket exists to avoid.
- **`os.ReadDir` sorts by filename.** Fine — the tree is sorted by bytes for display — but do not assume the sort survives a cap mid-directory, and do not let a fixture test depend on readdir order for the totals.
- **Do not carry the walk's tree into `/summary` or `/candidates`.** The card's numbers come from the hot index and `statfs` (tickets 02 and 03); mixing a walk result into them would make a poll expensive and a Disk's usage wrong.
- **The contract test file already names a different Plugin.** `apps/web/tests/plugins.contract.test.mjs:159-190` imports `disk-usage/widget.js` and asserts a `disk-usage` folder. `pnpm test` therefore fails until ticket 10 renames it to `disk-space` and re-points its cases (including its `warnPercent` key). Do not fix it by creating a second folder: the name is `disk-space` (D2, D3) and case is folded on a Mac, so the failure is confusing rather than loud.
