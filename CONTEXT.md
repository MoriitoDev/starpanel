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
