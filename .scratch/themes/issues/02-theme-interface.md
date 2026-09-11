# 02: The Theme interface

**What to build:** the header control that picks a Theme, the Themes section in edit mode that imports, deletes and downloads, and the five `data-part` hooks the contract promises.

**Blocked by:** 01 theme-layer.

**Status:** ready-for-agent

- [ ] The header shows the active Theme and opens the list of available ones; picking one persists it and the panel repaints without a reload
- [ ] The default Theme is always in the list and always selectable; an imported Theme that disappears is shown as absent rather than silently dropped
- [ ] Edit mode gains a Themes section: import from a file picker, delete with a confirmation, and download the active Theme
- [ ] Download is disabled while the default is active, because the default is not a file
- [ ] Importing a file whose name already exists fails with a message that says so, instead of overwriting
- [ ] Five `data-part` attributes exist in the markup — `header`, `identity`, `card`, `empty`, `error` — and a Theme can style each region with them
- [ ] `docs/THEMES.md` documents the whole contract (the token variables, the component classes, the five hooks) with a minimal example Theme, and says plainly that anything outside it is allowed but unsupported
- [ ] Verified by hand: import a Theme, see it applied, switch back to the default, delete it
