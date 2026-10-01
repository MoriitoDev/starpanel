# 04 — The dialog

Status: ready-for-agent

## What

The detail interface the card's `Clean…` button opens: a native `<dialog>` with a
`showModal()` call, painting every Disk, every Category and every Candidate, with a tick
box on the ones the owner may remove and no tick box on the ones they may not. It owns
the Selection, it keeps the two counters honest (selected bytes against Reclaimable), and
it ends with the `Review` seam that ticket 07 fills with the Plan and the Dry Run.

## Why

This is the "todo lujo de detalles" the whole Plugin exists for, and it is also the only
place where a person can be told the truth about what they are about to delete. Two
rules are decided here and nowhere else: what a Candidate row looks like when it cannot
be acted on, and that a big file is shown without a tick box. Both are design decisions
with data-loss consequences.

## Acceptance

- [ ] `Clean…` opens a real `<dialog>` by `showModal()`, and not an overlay: the card
      opens it and `Esc` closes it
- [ ] The dialog paints `var(--surface)`, `var(--radius-lg)` and the one permitted shadow;
      the outside dims with `::backdrop`; no other shadow, no gradient, no second accent
- [ ] Focus moves into the dialog on open and returns to `Clean…` on close; the dialog is
      labelled by its own heading
- [ ] **Disks** renders every Disk with its bar, filesystem type and used/free in
      tabular figures
- [ ] **Categories** renders one collapsible section per Category, headed by title, rule
      in `mute` and bytes; sections start collapsed except the one with the largest
      Reclaimable
- [ ] A Candidate row shows tick box, path (`font-mono`, `meta`), size and `reason`
- [ ] A tick box exists **only** when `category.tickable && candidate.selectable &&
      candidate.capability.ok`; the rule is written once and used everywhere
- [ ] A row with `selectable: false` (a `large-files` observation, which arrives with no
      `id` at all) has **no** tick box and renders in `mute`; the section states in words
      that these are big and only the owner knows whether they are dead
- [ ] A row with `capability.ok: false` has **no** tick box, renders in `mute` (never
      `--danger`), and exposes `capability.words` and `capability.remedy` as a tooltip
      reachable by keyboard, with the words also in the DOM
- [ ] Docker's number is shown as an estimate in `mute` wherever it appears, including the
      Category header and any total it contributes to, with the words that shared layers
      make it overlap; `docker-volumes` renders in `--danger` with the words that a volume
      is data and not junk
- [ ] A running total of the Selection — items and bytes — is visible while ticking, and
      `Review` is disabled while the Selection is empty
- [ ] The dialog has designed empty, loading and error states: nothing qualifies, a fetch
      in flight, and a backend that failed each render as their own surface
- [ ] The table is reachable and operable by keyboard alone, and nothing is communicated
      by colour alone
- [ ] The dialog never fetches `/walk`, and opening it never starts a full tree walk
- [ ] Opening the dialog issues one `/candidates` request, not one per Category

## Notes

**Why a `<dialog>` and not a fixed overlay.** `svelte-dnd-action` leaves a `transform` on
the grid item while the owner arranges cards ([App.svelte:686](../../apps/web/src/App.svelte#L686)),
and a transformed ancestor becomes the containing block of a `position: fixed`
descendant — so an overlay would be trapped inside the card, and a wide dialog would be
clipped to a card's width. The browser's top layer is immune to that, and it brings focus
trapping, `Esc`, `::backdrop` and inertness for free (D4). Use `showModal()`, not
`show()`: only the modal form gives the top layer and the focus trap.

**The dialog lives in the Widget's DOM.** Do not mount it on `document.body` and do not
ask core for a route. It is created in `render(el, ctx)`, appended to `el` or replaced
into it, and removed by the cleanup function along with the poll timer. A dialog left
open when the Dashboard re-renders a card is a regression: the cleanup function closes
it.

**Section 5 is a seam, not a placeholder.** `Review` builds the Selection and this ticket
stops there — the Plan, the Dry Run output and `Remove` are ticket 07, and the History
pane needs `/journal` from ticket 07's journal. Leave the section rendering `mute`
explanatory text, and leave the code path that calls `POST /plan` behind one function so
07 replaces its body rather than untangling it.

**Sizes read at a glance, and one of them is a lie.** Every byte count is formatted the
same way everywhere in the dialog, with `font-variant-numeric: tabular-nums`, and the
formatting is one function — a dialog that says `8.0 GB` in one row and `8192 MB` three
rows down makes the owner do arithmetic. Docker's number is never formatted as if it
were exact: it carries its estimate marker wherever it appears, including the Category
header and the totals it contributes to.

**The tooltip on a denied row.** `capability.words` says the failed syscall in words and
`capability.remedy` says what to change — "EACCES deleting /var/log/syslog; mount the
host's /var/log into this container". It must be reachable without a pointer: reveal on
focus as well as hover, and give the row an accessible description, not just `title`.
`DESIGN.md` §7: the detail is reachable without a pointer, and focus is never removed. The
same shape arrives on a Category, so one component renders both.

**Do not let the dialog move under the owner.** Polling continues while the dialog is
open — the Dashboard owns the card's timer — so re-rendering must not collapse a section
the owner expanded, uncheck a tick, or reorder the rows. Keep the expanded/checked state
in the dialog's own memory, keyed by the stable `id` from ticket 03, and reconcile rather
than rebuild.

**Sizing.** The dialog is at most the content width of the panel (`DESIGN.md` §6, 1200px
max) and never taller than the viewport; its body scrolls, its header and footer do not.
Inside it, `font-mono` at `meta` is the one place a long path is allowed to wrap rather
than truncate, because a path the owner cannot read is a path they cannot check.

**The card must not be resizable into a broken dialog.** A Span can be one column wide
(`DESIGN.md` §6) and the dialog still has to be usable: it is laid out by the viewport,
not by the card. Test it with the card at `w: 1` and with the panel under 640px.
