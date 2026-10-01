# 03 — The hot index and the wire the dialog reads

Status: ready-for-agent

## What

The backend's Scan Root index: the declared roots are walked on the first request,
refreshed every 10 minutes or when the config changes, and the result is what
`GET /summary` counts and `GET /candidates` lists. All six v1 Categories are registered
and reachable by id, and `GET /candidates` answers the full shape `spec §3` fixes —
grouped by Category, every Candidate with its `id`, `selectable`, `capability` and
`reason`. Tickets 05 and 06 fill the Categories with real findings; this ticket builds
the contract they fill and the cheap index that makes the dialog open instantly.

## Why

Two things are load-bearing here and neither is visible in a screenshot. First, the
shape: `id`, `selectable`, `capability`, `reason` and `fingerprint` are what the dialog,
the Plan and the apply step all key off, so they are decided once, in one place, before
three tickets depend on them. Second, the cost: the owner opens this dialog while
looking at a slow disk, and a dialog that walks 2 TB before showing anything is a dialog
nobody opens twice ([spec §6](../spec.md), D14).

## Acceptance

- [ ] `GET /candidates` answers the shape in [spec §3](../spec.md): `categories[]`, each
      with `id`, `title`, `rule`, `bytes`, `estimate` and `candidates[]`
- [ ] A Candidate carries `id`, `path` (null for a command Candidate), `bytes`, `reason`,
      `selectable`, `capability`, and a `fingerprint` of `size`, `mtime` and `inode` for a
      file Candidate
- [ ] `id` is `cl_` plus twelve hex of a SHA-256 over the Category and the absolute path
      for a file Candidate, and the stable synthetic `docker-images:dangling` /
      `docker-volumes:all` form for a command Candidate; it is stable across two calls
      while the index has not been rebuilt
- [ ] An **observation** (a `large-files` row) carries no `id` and `selectable: false` —
      it was measured and shown, and it is not a Candidate
- [ ] `GET /summary`'s `categories[]` counts and bytes come from the same index as
      `GET /candidates` — one build, two readers, no double walk
- [ ] The index is built at most once per 10 minutes; ten concurrent first requests
      trigger one walk, not ten
- [ ] A Scan Root that does not exist, or that the process may not read, is skipped with
      a `warnings` entry and does not stop the index
- [ ] `/summary` answers from the hot index without touching the filesystem beyond
      `statfs`, and answers in well under a second on a machine with a 2 TB root
- [ ] All six Categories from [spec §5](../spec.md) are registered with their title and
      their human-readable `rule`, and a Category that has no scanner yet reports zero
      Candidates rather than disappearing
- [ ] A Candidate whose path cannot be represented as absolute, or that resolves outside
      every declared Scan Root, is never emitted — with a test for each
- [ ] `docker-images` and `docker-volumes` appear with `estimate: true`; every file
      Category appears with `estimate: false`
- [ ] A Category carries `tickable`, and a Candidate carries `capability` as an object
      (`{"ok": true}` or `{"ok": false, "kind", "errno", "attempted", "words", "remedy"}`)
      — never a bare string, because the dialog and ticket 09 both read the fields
- [ ] `large-files` is registered with `tickable: false` and its rows carry
      `selectable: false` and no `id`

## Notes

**Where the config comes from.** The widget passes its `config` with each request; the
backend does not remember it ([spec §8](../spec.md)). So the index has to be keyed on
the config it was built with: keep a hash of the effective settings and rebuild when the
hash changes, otherwise a `/candidates?roots=…` call silently answers another root's
findings. In this ticket the config arrives as query parameters on the GETs; ticket 09
owns the defaults, the validation and the fallback when the `config` object is garbage,
so read it defensively now and leave a single `Effective(config) Settings` seam for 09
to complete.

**The index belongs behind one type.** Something like:

```go
type Index struct {
    mu        sync.Mutex
    built     time.Time
    key       string
    settings  Settings
    entries   map[CategoryID][]Candidate
    warnings  []string
}

func (i *Index) Get(ctx context.Context, s Settings) (Snapshot, error)
```

`Get` returns the cached Snapshot when the key matches and the build is younger than 10
minutes, and otherwise builds once — holding the lock across the build is acceptable and
simpler than a singleflight, but then a *second* caller must not start a build it did
not win: re-check the timestamp under the lock and return the fresh snapshot. Do not
walk with the lock held in a way that blocks `/health`.

A `Snapshot` is immutable once returned. Readers never see a half-built index, and the
next build never mutates the previous one.

**Bound the walk even in the index.** A declared root is small by contract, but `/` is
a legal thing for someone to write in a config file. Cap the index walk — depth and
total entries — and add a `warning` when it hits the cap, rather than discovering the
problem as a panel that stops answering. The full walk is ticket 08 and is explicit;
nothing in this ticket reaches for it.

**The Categories as a registry, not a switch.** One `Scanner` interface, one slice of
registered Categories, and the index walking them in a fixed order:

```go
type Scanner interface {
    ID() CategoryID
    Title() string
    Rule() string
    Scan(ctx context.Context, s Settings) ([]Candidate, error)
}
```

Tickets 05 and 06 add a `Scanner` each and touch nothing else. Order is stable and
explicit — `docker-images`, `docker-volumes`, `rotated-logs`, `package-caches`,
`temp-files`, `large-files` — because the dialog renders in the order it receives and a
list that reshuffles between polls is unusable.

**`reason` is for a person.** "rotated 41 days ago", "no container references it",
"big, and only you know if it is dead". Not `mtime < now-30d`. The dialog shows it
verbatim next to the path and it is the only justification the owner gets, so it is a
sentence and it says *why this one qualifies*.

**`selectable: false` versus `capability.ok: false` are different facts.** The first
means "this Plugin will never offer to remove it" (`large-files`, D10) — and an
observation carries no `id` at all, because it is not a Candidate. The second means
"it would, but this process cannot" (D5), and that row keeps its `id`: it *was* measured.
Both end up without a tick box, and the dialog shows them differently — the first is a
review list, the second is a row that explains how to enable it. Do not collapse them into
one field.

**The walk itself.** This index is not the place to build the fast walker — ticket 08 owns
the bounded workers, the caps, cancellation and hardlink de-duplication, so use the
simplest correct traversal here (`filepath.WalkDir` is fine) and keep it in one
`walk(root)` helper so 08 replaces a body rather than forking one. What this ticket must
get right: `Lstat` rather than `Stat` when deciding whether a path is a symlink, never
follow a symlink out of a root, compare `filepath.Clean`ed absolute paths with
`filepath.Rel` before concluding a path is inside a root, and never emit a path you have
not proven is inside one.

**Tests.** Fixture-driven, not machine-driven: build a temporary tree with a
`t.TempDir()` and point a fake root at it, then assert the ids, the
`bytes`, the reasons and the cap behaviour. Two calls in a row must produce identical
ids; a rebuilt index after a config change must not. Add one test that a root of `/`
hits the cap and warns instead of running away.
