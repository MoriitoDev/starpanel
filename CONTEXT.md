# Star Panel

Self-hosted homelab dashboard that stays lightweight by serving a static frontend from a single Go binary.

## Language

**Dashboard**:
The user's configured set of widgets in an ordered list.
_Avoid_: home page, board

**Widget**:
A single card on the dashboard rendering one plugin's output.
_Avoid_: card, tile, stat, gadget

**Plugin**:
An installable unit providing one or more widgets, with optional backend logic.
_Avoid_: stat, source, app, integration, addon

**Manifest**:
A plugin's declared name, version, widgets, and backend contract.
_Avoid_: config, descriptor

**Theme**:
The Dashboard's stored colors: the active mode plus one value per color Token for each Palette.
_Avoid_: skin, style, custom CSS, color scheme

**Palette**:
The Token values that render while one mode (light or dark) is active.
_Avoid_: color set, scheme, theme (a Palette is part of a Theme, never the whole)

**Token**:
A named design value — a color, a radius, a type step — that both the Dashboard chrome and Plugin widgets style with.
_Avoid_: variable, CSS var, constant, style setting
