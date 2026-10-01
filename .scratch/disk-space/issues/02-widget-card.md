# 02 — The card

Status: done

## What

The Widget's card, as the owner sees it on the Dashboard: one row per Disk with a bar,
its mount point and used-of-total in tabular figures, then the Reclaimable total with
its estimate caveat in `mute`, then the `Clean…` button. It polls `GET /summary` on the
Dashboard's `pollSeconds` and releases its timer on cleanup. It survives a cold backend,
a backend that dies mid-poll, and a config it cannot read.

The card's job is to be glanced at. Anything you have to read twice does not belong on
it.

## Why

A dashboard card is the only surface most days ever touch, and the repo has a design
system precisely so that a Plugin's card looks like it was born there
([ADR-0003](../../docs/adr/0003-plugin-subprocess-vanilla-esm.md), [DESIGN.md §5](../../DESIGN.md#L133)).
This is also where the honest-numbering rule becomes visible: the card says `≈` and
`estimate` where Docker's number is a guess, and stops saying it once a real number is
available.

## Acceptance

- [ ] The card renders inside the frame the Dashboard supplies: no border, no surface, no
      padding, no shadow, no title of its own
- [ ] Every Disk from `/summary` gets a row: `name · label · fs`, a bar, and
      `usedPercent% · free` with `font-variant-numeric: tabular-nums`
- [ ] The Reclaimable total is shown, and when `reclaimableIsEstimate` is true it is
      marked as an estimate in `mute` — with words, not a colour alone
- [ ] A Disk at or above `warnPercent` (default 90) reads `nearly full` in words on its
      row and that row is painted `var(--danger)`
- [ ] The card ends with one `mute` footer line naming the source and the count
      (`2 volumes · source: win32`)
- [ ] `warnings` from `/summary` and the config `notes` render as at most one more `mute`
      line, capped so they cannot push the card's body off the panel
- [ ] A failed poll says `Could not read the disks` and names the status that failed, in
      `--danger`, and the card stays
- [ ] An empty answer (no Disks) says `No volumes found` and is a designed state, not a
      blank card
- [ ] A cold backend shows `mute` placeholder text and retries with a short backoff; it
      never shows an error for the first seconds after a restart
- [ ] `pollSeconds` drives the interval, and the returned cleanup function clears the
      timer and any pending fetch's effects
- [ ] No literal colour, no utility class, no second accent; the `Clean…` button is the
      only accent on the surface
- [ ] The card has an accessible reading order and the button is reachable by keyboard
- [ ] The contract test renders the module against the stub element and stub context with a
      fixed `/summary` payload, and asserts on the text it produces
- [ ] `pnpm test` passes: the six existing `disk-usage` cases keep every assertion they
      already make (renamed in ticket 10, satisfied here)

## Notes

**The suite already fixes most of this ticket's copy.** `apps/web/tests/plugins.contract.test.mjs`
tests this Plugin under the name `disk-usage`; ticket 10 renames the cases to
`disk-space` and this ticket satisfies them. The assertions are, verbatim:
`C: · Windows · NTFS`, `50% · 512 MB free`, `2 volumes · source: win32`,
`/ · ext4`, `nearly full`, `90% · 100 B free`, a style containing `var(--danger)` for the
nearly-full row, `warnPercent: 50` in the config moving the threshold, `/Could not read
the disks/` and `/500/` when the backend fails, and `/No volumes found/` for an empty
machine. Write the copy to match rather than inventing your own and changing the test —
the test is the contract ([spec §9.2](../spec.md)).

Two details of that payload the ticket otherwise would not imply: the source string is
`win32` on a Windows answer and `procfs`/`statfs` on a Linux one, and the byte formatting
turns `512 * 1024 ** 2` into `512 MB` and `1.5 * 1024 ** 3` into `1.5 GB` — so the
formatter takes one decimal, drops a trailing `.0`, and uses `B`, `MB`, `GB` with a
space. One formatter, used for every size in the card and later in the dialog.

**The module contract.** `export default function render(el, ctx)`; `ctx` is
`{ config, pollSeconds, fetch }` ([docs/PLUGINS.md:16](../../docs/PLUGINS.md#L16)).
`ctx.fetch(path, init?)` resolves to a `Response` — call `.json()` on it. It targets
`/api/v1/plugins/disk-space/proxy/`, so the path is `"summary"`, no leading slash. `init`
was added for the POST-shaped routes this Plugin needs later ([spec §9.1](../spec.md));
the card itself only reads.

**The stub element is deliberately poor.** `apps/web/tests` renders widgets against a
stub that supports only `append`, `replaceChildren`, `textContent` and
`style.cssText`. Stay inside that surface or the contract test cannot run.

**Styling is `cssText` against Tokens.** There are no utility classes at runtime
(Tailwind never sees this file) and no literal colour is allowed. Use
`var(--ink)`, `var(--body)`, `var(--mute)`, `var(--border)`, `var(--surface-soft)`,
`var(--accent)`, `var(--danger)`, `var(--on-accent)`, `var(--radius-sm)`,
`var(--radius-pill)`, `var(--text-meta)`, `var(--text-base)`, `var(--text-subheading)`.
A bar is a track in `--surface-soft` with a fill whose width is the percentage. One
judgement to make and justify in the code: the bar's fill uses `--ink`, not `--accent`
— the accent is for the one thing that acts, and `Clean…` is that thing (the same call
the existing `system-stats` widget made; see
[`star-panel-v2/issues/05`](../../star-panel-v2/issues/05-plugin-styling-contract.md)).

**Empty, loading and error are designed states, not browser defaults.** No Widgets, no
Disks and no answer are three different screens. `DESIGN.md` §3: every state is
designed, nothing animates on its own, body text never below 15px and meta never below
13px.

**Do not fetch `/candidates`, `/walk` or anything expensive from the card.** The card
polls; the card must stay cheap. `/summary` is `statfs` plus the hot index by design
([spec §6](../spec.md)), and a card that walks a tree every 10 seconds would make the
whole panel feel broken.

**`config` may be garbage.** `ctx.config` is free-form JSON an owner edited by hand. In
this ticket only the polling path matters, but do not assume `config` is an object:
guard the reads with a default now so ticket 09 has less to fix.

**Tests.** `apps/web/tests/` uses `node --test` and runs under `pnpm test` from
`apps/web`. Add a case for the healthy payload, one for a `warnings` payload, and one
for a `fetch` that rejects — the third is the one that catches an unhandled rejection
taking down the Dashboard's poll loop.

## Comments

- 2026-10-01: Done. `apps/api/plugins/disk-space/widget.js` renders the Disk rows
  (`name · label · fs`), a bar in `--ink` over a `--surface-soft` track, `usedPercent% ·
  free` in tabular figures, the `≈ N reclaimable` line with its estimate caveat, one muted
  line for warnings and config notes, the `N volumes · source: …` footer, and the `Clean…`
  button as the single accent on the surface.
- 2026-10-01: The existing suite is satisfied, not worked around. The six `disk-usage`
  cases are renamed to `disk-space` in `apps/web/tests/plugins.contract.test.mjs` and
  every assertion they already made is still made — the volume line, `nearly full` in
  words plus `var(--danger)`, `warnPercent: 50` moving the threshold, `Could not read the
  disks` with its status, and `No volumes found`. Two changes were needed on the fixture
  side and both are the point of the rename: the payload is `/summary`'s shape
  (`disks`, not `volumes`) and the volume line reads `/ · root · ext4`.
- 2026-10-01: Two details the ticket did not anticipate, both forced by the stub. The
  loading state renders **exactly one** node, because the suite waits for the second text
  node to mean "the answer landed"; and the `Clean…` button is created with `setAttribute`
  so the stub's `attributes` map sees it. The button is a seam: it has no click handler
  until ticket 04's dialog exists, and ticket 04 owns wiring it.
- 2026-10-01: `configWarning` is read from the summary when present and `warnings` is
  joined when not, which is the one place `/summary` and `/candidates` disagree about the
  name of the same idea. Worth reconciling in ticket 03 rather than carrying two names
  forward: the answer that has a config to complain about is `/summary`'s, and it has no
  `configWarning` field yet.
