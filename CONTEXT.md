# Star Panel

Self-hosted homelab dashboard that stays lightweight by serving a static frontend from a single Go binary.

## Language

**Dashboard**:
The user's configured set of widgets in an ordered list.
_Avoid_: home page, board

**Widget**:
A single card on the dashboard rendering one plugin's output.
_Avoid_: card, tile, stat, gadget

**Span**:
How much room a widget takes on the dashboard's grid: columns wide and rows tall, chosen by the owner. Widgets flow in order, so a Span says how big a card is, never where it sits.
_Avoid_: size, width and height, dimensions, position

**Plugin**:
An installable unit providing one or more widgets, with optional backend logic.
_Avoid_: stat, source, app, integration, addon

**Manifest**:
A plugin's declared name, version, widgets, and backend contract.
_Avoid_: config, descriptor

**Theme**:
A named stylesheet the owner imports to repaint the Dashboard's visuals; the default Theme ships inside the binary.
_Avoid_: skin, style, color scheme, mode

**Palette**:
The fourteen baseline Token values the default Theme ships, which an imported Theme overrides as it likes.
_Avoid_: color set, scheme, theme (a Palette is what a Theme paints with, never the whole)

**Token**:
A named design value — a color, a radius, a type step — that both the Dashboard chrome and Plugin widgets style with.
_Avoid_: variable, CSS var, constant, style setting

**Disk**:
A mounted filesystem with its own capacity, named by its mount point. A machine has several; none of them is "the disk".
_Avoid_: drive, volume, partition, mount

**Scan Root**:
A directory a Plugin was told to walk, and the only part of the machine it looks at.
_Avoid_: path, target, scope, location

**Category**:
A class of removable waste a Plugin knows how to find and the rules it finds it by — images Docker no longer needs, rotated logs, package caches.
_Avoid_: type, kind, group, bucket, rule

**Candidate**:
One thing a Plugin measured and offers to remove, with its size and the reason it qualifies. A Candidate is always found by the Plugin; a path someone typed is not one.
_Avoid_: item, file, junk, target, result

**Recommendation**:
The Candidate set a Plugin puts in front of its owner as worth removing, and the space it would free.
_Avoid_: suggestion, advice, plan

**Selection**:
The Candidates the owner ticked. A Selection is always a subset of a Recommendation: the owner chooses among what was measured, never among what was typed.
_Avoid_: filter, choice, batch

**Plan**:
The removals about to run, each naming its Candidate and the exact action for it. Every Plan is answerable as a Dry Run first.
_Avoid_: job, queue, task, action list

**Dry Run**:
Answering a Plan without performing it: what would be removed, what would be run, and what came back.
_Avoid_: simulation, pretest, check

**Reclaimable**:
The bytes a Plan would actually free, which is not the bytes its Candidates occupy: shared layers, open handles and filesystem overhead make "used" an overestimate and sometimes an underestimate.
_Avoid_: freeable, savable, wasted

**Capability**:
Whether this process may act on a Candidate at all — deleting a file needs permission on it, and pruning Docker needs its socket. A Candidate without the Capability is shown and never offered.
_Avoid_: permission, privilege, access, rights
