# 10 — Package and document

Status: in-progress

## What

`apps/api/plugins/disk-space/` becomes a shippable Plugin: a build script inside the folder produces the `linux/amd64` backend binary and the ZIP that **Import a Plugin…** (`POST /api/v1/plugins`) accepts, the folder carries a README saying what it needs, the mounts it expects and the user it must run as, `README.md` and `docker-compose.yml` document the deployment changes, the removal journal is kept out of a download, and the whole thing is verified by hand against DESIGN.md.

## Why

D13 is the reason the Plugin is a folder Plugin at all: installable, removable and replaceable without touching core (D2, ADR-0009) — but only if there is an artefact someone can install and a README that says what it needs. Without the documented mounts and uid, the Plugin measures the container instead of the server, and without the archive decision the removal journal rides along in every download.

## Acceptance

- [ ] `apps/api/plugins/disk-space/build.ps1` (the name and the shape [spec §2](../spec.md) fixes) builds the backend for `linux/amd64` and writes the ZIP an import accepts, and both are reproducible from a clean checkout
- [ ] The built ZIP is a ZIP of the `disk-space` folder and imports through `POST /api/v1/plugins`; the folder name, the Manifest's `name` and every `module` path agree
- [ ] The build refuses to produce a ZIP that would exceed `maxPluginArchiveBytes` (32 MB in `apps/api/server.go:320`) instead of producing one the panel will reject
- [ ] `manifest.json` says `linux/amd64` and only `linux/amd64`: the README's first line and the build script's only target say the same, and nothing in the Manifest or the README promises another platform
- [ ] The Plugin folder's README states the `requires` (`docker`), the four mounts (`/var/lib/docker` read-only and measured only, `/var/log` read-write, `/tmp` read-write, `/var/run/docker.sock`), the user it must run as (uid 10001, not root), and how to run it when the panel is not in Docker
- [ ] `README.md` and `docker-compose.yml` carry the deployment changes where deployment is documented, in the existing three-point warning's register, and the Docker-socket point says plainly that mounting it into a panel with no login is a real decision
- [ ] `--privileged` and `user: root` are rejected in words, with D5's reason (it buys convenience with the whole host), and no example anywhere uses them
- [ ] State is kept out of the archive: a `Download` of the Plugin does not carry the removal journal, and the mechanism is a small, tested change in core rather than a rule the Plugin's author has to remember
- [ ] The ZIP this script produces, **imported and then downloaded again**, still runs: the round trip is exercised end to end, because the exec bit is what ticket 01 fixed in core (`writeEntry` and `Archive`) and this is the ticket that ships the artefact that depends on it
- [ ] The widget renders correctly in **view** mode only, and the manual checklist against DESIGN.md is written down and walked

## Notes

### The build script

`apps/api/plugins/disk-space/build.ps1` — the name and the place [spec §2](../spec.md)'s layout fixes — run from anywhere with PowerShell (the repo's dev machine is Windows; `pwsh` if only PowerShell 7 is on `PATH`):

```powershell
# The one target this Plugin promises (D1, D13): linux/amd64.
$ErrorActionPreference = "Stop"
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"

# The binary sits beside manifest.json and widget.js, named exactly what the
# Manifest runs. It is the Linux build: no extension. The source is src/, because
# the binary is called backend and a file and a directory cannot share a name.
go build -C "$here/src" -trimpath -ldflags "-s -w -X main.version=0.1.0" -o "$here/backend" .

# The ZIP the panel's Import accepts. It is built with System.IO.Compression and
# NOT with Compress-Archive, because Compress-Archive writes no Unix mode into
# the entry and the panel would then unpack a backend it cannot execute. The
# external attributes carry 0o755 in the high 16 bits; that is the whole reason
# this is twenty lines instead of one.
Add-Type -AssemblyName System.IO.Compression.FileSystem
$dist = Join-Path $here "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null
$zip = Join-Path $dist "disk-space.zip"
if (Test-Path $zip) { Remove-Item $zip }

$excluded = @("dist", "state", ".archiveignore")
$archive = [System.IO.Compression.ZipFile]::Open($zip, "Create")
try {
  Get-ChildItem -Path $here -Recurse -File | Where-Object {
    $rel = $_.FullName.Substring($here.Length + 1).Replace("\", "/")
    -not ($excluded | Where-Object { $rel -eq $_ -or $rel.StartsWith("$_/") })
  } | ForEach-Object {
    $rel = $_.FullName.Substring($here.Length + 1).Replace("\", "/")
    $entry = $archive.CreateEntry("disk-space/$rel", [System.IO.Compression.CompressionLevel]::Optimal)
    # 0o755 for the backend, 0o644 for everything else, in the Unix mode field
    # the ZIP format keeps in the high 16 bits of the external attributes.
    $mode = if ($rel -eq "backend") { 0o755 } else { 0o644 }
    $entry.ExternalAttributes = $mode -shl 16
    $input = [System.IO.File]::OpenRead($_.FullName)
    try {
      $output = $entry.Open()
      try { $input.CopyTo($output) } finally { $output.Dispose() }
    } finally { $input.Dispose() }
  }
} finally { $archive.Dispose() }

$size = (Get-Item $zip).Length
if ($size -gt 25165824) {
  throw "disk-space.zip is $size bytes; the panel refuses anything over 32 MiB"
}
Write-Host "wrote $zip ($([math]::Round($size / 1MB, 1)) MiB)"
```

- **`Compress-Archive` is unusable here, and this is the trap in this ticket.** It writes
  no Unix mode into a ZIP entry, so the archive would contain a `backend` the panel
  unpacks non-executable no matter what ticket 01 fixes in `writeEntry` — the same
  `permission denied` restart loop, this time with a correct importer. `System.IO.Compression`
  with `ExternalAttributes = mode -shl 16` is what makes the ZIP carry `0o755` for the
  binary. Verify it rather than trusting it: after building, `unzip -Z -l` (or `tar -tvf`
  on Linux) must show `-rwxr-xr-x` for `disk-space/backend`.
- The same reasoning applies to any other ZIP of this Plugin a person makes by hand on
  Windows: `Compress-Archive`, Explorer's "Send to → Compressed folder" and 7-Zip's ZIP on
  Windows all drop the bit. The Plugin's README says so, and `docs/PLUGINS.md`'s promise
  that a Plugin can bring its own binary deserves a sentence about how the archive has to
  be made.
- The output binary is `apps/api/plugins/disk-space/backend` with **no extension** — the Linux build the Manifest runs, produced from the source in `src/` by ticket 01's `go build -C src -o ../backend .`. `dist/` is where the ZIP goes and it lives inside the folder, so it is excluded above **and** by `.archiveignore`: a ZIP of the folder that contains the previous ZIP is the classic way to walk into 32 MB.
- `CGO_ENABLED=0` is the difference between a static binary that runs on `alpine` and one that fails on `musl`. Nothing here needs cgo: `os.ReadDir`, `os.Lstat`, `syscall.Stat_t`, `syscall.Access` and `syscall.Statfs` are all in the standard library, so the module requires nothing at all (ticket 01's acceptance says the same) and the ZIP stays small.
- The source folder is `src/` and the binary is `backend`, both at the Plugin root, and they cannot share a name: the Manifest runs `./backend`, so that name belongs to the file. `build.ps1` builds from `-C "$here/src"`; nothing else in this ticket moves either way.
- A POSIX `build.sh` next to it is a fine addition for a Linux dev box or CI — `zip -X` preserves the mode natively, so it is genuinely simpler — but it is not the deliverable: **spec §2 names `build.ps1`**, and a second script that drifts is worse than one.
- Verification is part of the ticket. `pwsh apps/api/plugins/disk-space/build.ps1`, then, against a running panel:

  ```powershell
  curl.exe -sS -X POST -H "Content-Type: application/zip" `
    --data-binary "@apps/api/plugins/disk-space/dist/disk-space.zip" `
    http://localhost:8080/api/v1/plugins
  ```

  answers `201` with the Plugin's `PluginInfo`, and `GET /api/v1/plugins/disk-space/archive` answers a ZIP that imports back. Run the second half of that check after the `.archiveignore` change lands, because that is the test that the journal is not in the download. And check the mode in **both** archives — the one built here and the one downloaded back — because that round trip is the only thing standing between this Plugin and a `permission denied` loop.

### Size, and where the 32 MB bites

`maxPluginArchiveBytes = 32 << 20` is the body cap on an import (`apps/api/server.go:328`); a bigger upload is read short and answered `400 {"error": "the plugin archive is too large to import"}`. `maxUnpackedBytes = 64 << 20` is the other half (`apps/api/internal/plugins/import.go:25`), and it counts **file sizes as uncompressed as they are unpacked**, so a ZIP that compresses beautifully does not help. Numbers an implementer should hold: a stripped `CGO_ENABLED=0` Go binary for this Plugin is roughly 9–12 MB and the ZIP roughly 4–6 MB, so both caps are met with a wide margin — which is exactly why the build fails loudly at 24 MB rather than shipping a ZIP that gets refused after the owner has already selected the file. If the binary ever approaches the cap, the answer is `-trimpath`, no new dependency, and no debug symbols — not shipping less of the Plugin.

Where the failure lands, in the owner's words, so the README can say it: the import control says the archive is too large to import, nothing is written (the archive is never a file on disk and the import is staged, `import.go:35-77`), and the fix is to build it here or copy the folder into `plugins/` by hand — `docs/PLUGINS.md` calls the folder copy "the short road".

### The Manifest, in full

`apps/api/plugins/disk-space/manifest.json`:

```json
{
  "name": "disk-space",
  "version": "0.1.0",
  "widgets": [{ "id": "disk-space", "title": "Disk space", "module": "widget.js" }],
  "backend": { "command": ["./backend"] },
  "requires": [{ "command": "docker" }]
}
```

- `command[0]` **must contain a separator**. `reachable` in `apps/api/internal/plugins/registry.go:39-46` looks a bare name up in `PATH`, and `exec.CommandContext` does the same: `"backend"` would be looked for in `PATH`, not in the folder, and the backend would never start. `"./backend"` is the argv core runs with the Plugin folder as `cmd.Dir` (`backend.go:185`), and it is resolved through that folder on both paths. A bare `"backend"` is the single most likely way to get this wrong.
- The port arrives as `STAR_PANEL_PORT` (`backend.go:186-189`), and `{port}` substitution is available for arguments. This Manifest needs neither, so it uses neither; if the backend later wants a flag, `"-addr", "127.0.0.1:{port}"` is the spelling, and the backend must still listen on `127.0.0.1` and never on every interface.
- `requires` names `docker` and nothing else — D6 says the CLI is a declared requirement, and ticket 01 settles that the Plugin's own binary needs no `requires` entry, because `command[0]` already has a separator and `exec` resolves it inside the folder. Adding `{"command": "./backend"}` would be harmless but it would report the Plugin's own file as a missing *requirement*, which is not what the listing is for.
- The version is deliberately left out of `requires`: `minVersion` means parsing, and every tool writes its version differently (`docs/PLUGINS.md`).
- No `backend` env var, no `{port}` in `requires`, no second widget (D3: one widget, one card).

### The Plugin folder's README

`apps/api/plugins/disk-space/README.md`, in the register of `apps/api/plugins/hello-widget/README.md`:

1. **What it is** — one paragraph: the Plugin that shows disk usage and offers to remove dead files, with every removal supervised, and the sentence `Built for linux/amd64 only.` as its own line at the top.
2. **What it needs** — `docker` on `PATH` for the `docker-images` and `docker-volumes` Categories; nothing else at runtime, because the backend is a static binary inside the folder. The panel diagnoses and never installs (`docs/PLUGINS.md`), so this section says what to install and where — including that the shipped `alpine` runtime image has no `docker` in it, so a Docker deployment adds it to the image (`apk add --no-cache docker-cli`) or the two command Categories honestly report `tool-missing`.
3. **The mounts it expects**, as a copy-pasteable table plus a `compose` fragment:

   | Host path | In the panel | Mode | Why |
   | --- | --- | --- | --- |
   | `/var/lib/docker` | `/var/lib/docker` | `ro` | measured for `docker-images`; its contents are never file Candidates (D6) |
   | `/var/log` | `/var/log` | `rw` | `rotated-logs` measures and removes here |
   | `/tmp` | `/tmp` | `rw` | `temp-files`; without the mount the Plugin measures the container's `/tmp`, which is empty and unimportant |
   | `/var/run/docker.sock` | `/var/run/docker.sock` | `ro` | the `docker` CLI's channel; a socket needs `ro`, it cannot be mounted `rw` and does not need to be |

   And the honest sentence under it: `/var/cache` is worth mounting too if the owner wants `package-caches` to mean the host's caches rather than the container's — it is one of the six default Scan Roots (ticket 09), so without the mount that Category measures the container's empty `/var/cache` and reports nothing, which is a true answer to a question nobody asked. Mounting nothing is a valid configuration: the Plugin still measures the container and says so.
4. **The user it must run as** — uid 10001 (`starpanel`), never root (D5). The socket is the concrete case: a bind-mounted `/var/run/docker.sock` arrives owned by `root:docker` with mode `0660`, so uid 10001 is refused with `EACCES` unless the container is given the host's docker group id — `group_add: ["<docker gid>"]` in compose, `--group-add <docker gid>` on the command line — and the Plugin reports exactly that in `mute` (ticket 09's `socket-permission` remedy) rather than asking for more privilege.
5. **How to run it when the panel is not in Docker** — the binary beside `plugins/`, i.e. what `README.md` already describes: `./star-panel -addr :8080` from a folder holding `plugins/disk-space/`; then the Plugin sees the real `/var/log`, `/tmp` and `/var/run/docker.sock` and needs no mounts at all, which is the configuration D5's remedies are written for. Include the one-liner that proves the backend runs: `STAR_PANEL_PORT=18099 ./plugins/disk-space/backend &` then `curl -sS 127.0.0.1:18099/config`.
6. **What it will not do** — five bullets, so nobody expects otherwise: it never installs anything, it never runs as root or `--privileged`, it never edits `/var/lib/docker`, it never removes a volume through `docker system prune` (D6, D11), and it keeps no history of scans (D14).

### Deployment documentation

Two files change, both where deployment is already documented:

- **`README.md`** — the Docker section's list at lines 65–69 already warns about three things; the disk-space Plugin adds a fourth in the same voice, and the socket point there is the one that must stay honest: *"Mounting `/var/run/docker.sock` into a panel with no login is a real decision: anything that can reach the port can prune your Docker."* The other additions are the uid/group sentence, the mounts, and the note that `system-stats`' disk row is the container's view while this Plugin's Scan Roots are whatever was mounted. `README.md` also gains the build line in **Commands**: `pwsh apps/api/plugins/disk-space/build.ps1`.
- **`docker-compose.yml`** — the disk-space block, commented out or as a second service's example so that `docker compose up` without it still works exactly as today:

  ```yaml
  # The disk-space Plugin measures and cleans the host. It is optional: without
  # this block the panel runs exactly as before.
  #   mounts: the store is measured and never edited; the socket is what the
  #   docker CLI needs, and mounting it into a panel with no login is a real
  #   decision. /tmp, /var/log and /var/cache are read-write because removing is
  #   the point. Skip any of them and that Category measures the container
  #   instead, which the Plugin says in mute rather than pretending otherwise.
  #   group_add: the socket is root:docker 0660, so uid 10001 needs that group.
  #   The image needs docker-cli too, or the two command Categories report
  #   "the docker command is not on PATH".
  services:
    star-panel:
      volumes:
        - /var/lib/docker:/var/lib/docker:ro
        - /var/log:/var/log
        - /var/tmp:/var/tmp
        - /var/cache:/var/cache
        - /tmp:/tmp
        - /var/run/docker.sock:/var/run/docker.sock
      group_add:
        - "999"   # the host's docker group id, `getent group docker`
  ```

  The `docker` CLI itself has to be in the image for this Plugin's command Categories: `alpine:3.21` has no `docker`. The change belongs in the `Dockerfile`'s runtime stage (`Dockerfile:26-34`), one line in the `apk add` next to `ca-certificates` (`docker-cli`), with the comment that says why — which is documentation the deployment reads, not a hidden requirement.

### `--privileged` and `user: root`: rejected, with the reason

D5 rejected them and this ticket keeps the rejection visible rather than leaving it to archaeology:

- `--privileged` hands the container every host device and every capability. It would make every removal succeed and would also make the panel's own guarantee meaningless: the Plugin's safety is that removal is validated against Candidates the Plugin measured, and a process that can do anything on the host is not a process where that validation is the thing standing between the owner and their data.
- `user: root` is the same trade in smaller print, and it breaks the premise the whole design is written against: every Capability message in ticket 09 says "run as uid 10001 and fix the permission", and a root panel never sees one.
- Both are rejected in the README's own words, in the same section that says what to do instead (mount the path, add the group). The `docs/PLUGINS.md` sentence is the one to echo: the panel runs without privileges, and a manifest that could name an installer would let a Plugin decide what runs on the server.

### State out of the archive: the decision, and the small core change

The trap, stated in [spec §9](../spec.md) as one of the two things worth knowing: `Entry.Archive()` (`apps/api/internal/plugins/import.go:90-123`) zips **every regular file under the Plugin folder**. The removal journal this Plugin keeps — `state/journal.jsonl`, one append-only record per removal, which is what the dialog's History pane reads and which is *not* the scan history D14 refuses — would therefore ride along in every **Download**, growing without bound and calling itself part of the Plugin.

**Decision: the Plugin declares what does not belong in an archive, and core honours it.**

- New convention, in core, in `Archive()`: before the walk, read `<folder>/.archiveignore` if it exists and is a regular file. Each non-empty, non-`#` line is a path relative to the folder or a `dir/` prefix; a file whose folder-relative slash path equals an entry or starts with a `dir/` entry is skipped, and `.archiveignore` itself is never written into the ZIP. A missing file means today's behaviour, so nothing changes for `hello-widget`, `echo` or `system-stats-custom`.
- The Plugin's `.archiveignore` is then one line, next to its README:

  ```
  # Written at runtime. Not part of the Plugin, and not what a download is for.
  state/
  dist/
  ```

- This is a small, self-contained change in `apps/api/internal/plugins/import.go` (one helper, one call, plus the `d.IsDir()` skip for a listed folder), and it is the honest place for the rule: core owns what an archive of a Plugin contains, the Plugin owns what its own files mean, and a Plugin author adding state does not have to know that a download exists. The alternatives were rejected: a second route or a query parameter on the archive endpoint grows the HTTP surface for a rule about files (and `README.md`'s endpoint list is deliberately short); naming the folder `state` and hard-coding that name in core is core knowing one Plugin's business; keeping the state outside `plugins/` entirely (in `data/`) would put a Plugin's file into core's storage and break D2's "a Plugin is a folder you can move".
- Test it at the seam `apps/api/internal/plugins/import_test.go` already uses: an extended `TestArchiveRoundTripsAPluginFolder` case with a `.archiveignore` naming a folder that holds a file, asserting the archive holds the Manifest and the widget and **not** the ignored file, and that the round trip still imports. `apps/api/plugins_io_test.go:96-123` (the HTTP download test) can assert the same through the endpoint, because "a download does not carry state" is worth one test on each side of the seam.
- Say in the Plugin's README what the journal is and that it is intentionally not in a download: a Plugin copied to another machine starts with an empty journal, which is correct — the journal describes this machine's removals.

### Manual checklist: view mode and DESIGN.md

Validation is by hand, against the design system the widget obeys (DESIGN.md §2 Tokens, §3 Rules, §5 Plugin widgets, §7 Accessibility, §8 notes). Write it as this list, walk it, and record the result in the ticket's `## Comments`:

1. Drop the built folder into `plugins/` with `docker compose up -d --build`, add the Widget in edit mode, then leave edit mode. In view mode the card shows a title, the Disk bars, the total Reclaimable, the `Clean…` button and **at most one** `mute` metadata line — no config controls, no `Edit` affordances, no second button.
2. Every colour in the card and the dialog is a Token: search the widget source for `#`, `rgb(`, `hsl(` and find none; the only shadow anywhere is `--popover-shadow` on the `<dialog>`.
3. One accent per surface: the `Clean…` button (or the single leading action) carries `--accent`; `--success` and `--danger` appear only as state; `--mute` carries every "not available" state. Paint it over a dark imported Theme (drop one into `themes/`, pick it in edit mode) and re-read it: nothing unreadable, nothing literal.
4. Numbers are `font-variant-numeric: tabular-nums` — the Disk figures, the walk's tree sizes, the biggest-file sizes, the walk's progress line.
5. Type: no body text under `var(--text-base)`, no meta under `var(--text-meta)`; headings inside the dialog stay at or below the card's `subheading`.
6. Keyboard only: `Tab` to the card's button, `Enter` opens the dialog, focus is trapped, `Esc` closes and focus returns to the button, the walk control is reachable, and the capability rows' tooltip text is announced (the described-by line exists in the DOM).
7. Coarse pointer (or the browser's touch emulation): every control is at least 44px; under 640px the dialog's tree flattens and nothing overflows horizontally.
8. `prefers-reduced-motion: reduce`: the progress line updates without animating; nothing else moves on its own while the card polls.
9. The card in view mode with a broken config (`"logDays": "thirty"`) shows the one `mute` config line and keeps its numbers; with the backend stopped (`docker compose stop`/kill the subprocess) it shows one `mute` line and recovers within a few seconds when the supervisor restarts it (`backend.go:21`) — never a `--danger` flash for a restart.
10. `pnpm test` and `pnpm check` pass, and `go test ./...` passes from `apps/api`, including the new `.archiveignore` case.

### Traps

- **`pnpm test` currently fails by construction.** `apps/web/tests/plugins.contract.test.mjs` names a `disk-usage` folder (lines 159–190) that DESIGN-NOTES and spec §2 do not call for. Rename those references to `disk-space`, keep the cases that still apply (the backend binary ships with the Plugin, the widget renders one row per Disk and reports a failed backend), and re-point the `warnPercent` case, which ticket 09 keeps as a real key with a default of 90.
- **The `Dockerfile` copies bundled Plugins one by one** (`Dockerfile:31-32`). Adding `disk-space` there is what makes a Docker deployment have it without an import, and it is optional: leaving it out means the owner imports the ZIP, which is the flow this ticket has to keep working either way.
- **The runtime image has no `zip` and no `go`** — the build script runs on the developer's machine or in CI, never in the container.
- **`command[0]` must contain a separator** (`registry.go:39-46`). Written down twice in this ticket because it is the single mistake that makes the Plugin silently never start on a machine where everything else is fine — and `requires` does **not** cover it, since ticket 01 settles that the Manifest declares `docker` only.
- **`Compress-Archive` is not byte-reproducible.** A Go build is; a ZIP carries timestamps and an entry order that follows the filesystem. "Reproducible from a clean checkout" in the acceptance above means the same contents and the same import result, not a matching hash.
- **`linux/amd64` is a promise about the target, not about the build machine**: `GOOS`/`GOARCH` cross-compile from Windows and from an arm64 Mac; `CGO_ENABLED=0` is what makes the result run on `alpine`. On a `linux/arm64` host the binary simply does not run, and the honest answer is a build for that target — which D13 leaves to whoever needs it, and the README says so rather than implying otherwise.
