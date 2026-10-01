# disk-space

Shows how full every Disk is, finds the files worth removing, and removes only
the ones you tick — after showing you what would go.

**Built for `linux/amd64` only.** The backend is a static Go binary inside this
folder, so the machine needs nothing but the `docker` CLI; building it for
another architecture is a `GOOS`/`GOARCH` change in `build.ps1` and nothing else.

## What it needs

- **`docker` on `PATH`** for the `docker-images` and `docker-volumes`
  Categories. Without it those two say so themselves, in words, and the rest of
  the Plugin works: the panel diagnoses and never installs.
- Nothing else at runtime. The binary is in this folder and is a subprocess of
  the panel, supervised like any other Plugin backend.

## The mounts it expects

The Plugin measures and cleans **the host's** waste, and a container sees none of
it unless it is mounted. Point the Widget's `scanRoots` at the mount points
*inside* the panel, not at the host's paths, which do not exist in there.

| Host path | In the panel | Mode | Why |
| --- | --- | --- | --- |
| `/var/lib/docker` | `/var/lib/docker` | `ro` | measured for `docker-images`; its contents are **never** file Candidates (D6) |
| `/var/log` | `/var/log` | `rw` | `rotated-logs` measures and removes here |
| `/tmp` | `/tmp` | `rw` | `temp-files`; without the mount you measure the container's `/tmp`, which is empty and unimportant |
| `/var/run/docker.sock` | `/var/run/docker.sock` | `ro` | the `docker` CLI's channel; a socket cannot be mounted `rw` and does not need to be |

`/var/cache` is worth mounting too if you want `package-caches` to mean the
host's caches rather than the container's.

Mounting nothing is a valid configuration: the Plugin then measures the
container and says so, which is at least honest.

## The user it must run as

The panel's image runs it as **uid 10001 (`starpanel`), never root**, and this
Plugin is written for that: what it cannot touch it shows in `mute` with a
tooltip naming the failed syscall and the way to change it.

The concrete case is the socket. A bind-mounted `/var/run/docker.sock` arrives
owned by `root:docker` with mode `0660`, so uid 10001 is refused with `EACCES`
unless the container is given the host's docker group:

```yaml
group_add:
  - "999"   # the host's docker group id: getent group docker
```

`--privileged` and `user: root` are not the answer. The first hands the container
every host device; the second would break the premise every Capability message
is written against, which is that the panel runs without privileges and tells you
what it cannot reach instead of taking it.

## Running it without Docker

The binary sits beside `manifest.json`, so a panel started from a folder holding
`plugins/disk-space/` needs no mounts at all — it sees the real `/var/log`, `/tmp`
and `/var/run/docker.sock`:

```powershell
./star-panel -addr :8080
```

The backend can also be run by hand, which is the quickest way to see what it
thinks:

```powershell
STAR_PANEL_PORT=18099 ./plugins/disk-space/backend
curl -sS 127.0.0.1:18099/summary
```

## Building it

```powershell
powershell -ExecutionPolicy Bypass -File apps/api/plugins/disk-space/build.ps1
```

That writes `backend` (linux/amd64, static) and `dist/disk-space.zip`, which is
what **Import a Plugin…** takes.

The ZIP is written with `System.IO.Compression` rather than `Compress-Archive`
on purpose: `Compress-Archive` writes no Unix mode into an entry, so the panel
would unpack a `backend` it cannot execute and log `permission denied` on a
restart loop while the Plugins section looked healthy. **A ZIP made by hand on
Windows — Explorer's "Send to → Compressed folder", 7-Zip, `Compress-Archive` —
has the same problem.** Check it before you import one:

```powershell
node apps/api/plugins/disk-space/verify-zip.mjs        # the built ZIP
node apps/api/plugins/disk-space/verify-zip.mjs http://localhost:8080   # and the round trip
```

On Windows the round-trip half cannot be checked: `chmod` there only moves the
read-only flag, so the mode cannot survive a trip through the filesystem. Run
that half against the Linux panel.

## What it will not do

- It never installs anything, and it never asks for more privilege.
- It never edits `/var/lib/docker`: Docker's store is measured and cleaned
  through the CLI, which knows its own format.
- It never removes a volume through `docker system prune`; volumes are their own
  Category, in `danger`, with a typed confirmation.
- It keeps no history of scans: no delta between runs, nothing to migrate.

## How it is put together

```
manifest.json     one widget, one backend
widget.js         the card, and the dialog it opens
src/              the Go source, its own module
backend           the compiled linux/amd64 binary (not committed)
dist/             the ZIP (not committed)
state/            the removal journal, written at runtime (never in the ZIP)
```
