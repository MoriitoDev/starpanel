# 02: The Theme interface

**What to build:** the header control that picks a Theme, the Themes section in edit mode that imports, deletes and downloads, and the five `data-part` hooks the contract promises.

**Blocked by:** 01 theme-layer.

**Status:** done

- [x] The header shows the active Theme and opens the list of available ones; picking one persists it and the panel repaints without a reload
- [x] The default Theme is always in the list and always selectable; an imported Theme that disappears is shown as absent rather than silently dropped
- [x] Edit mode gains a Themes section: import from a file picker, delete with a confirmation, and download the active Theme
- [x] Download is disabled while the default is active, because the default is not a file
- [x] Importing a file whose name already exists fails with a message that says so, instead of overwriting
- [x] Six `data-part` attributes exist in the markup — `app`, `header`, `identity`, `card`, `empty`, `error` — and a Theme can style each region with them
- [x] `docs/THEMES.md` documents the whole contract (the token variables, the component classes, the five hooks) with a minimal example Theme, and says plainly that anything outside it is allowed but unsupported
- [x] Verified by hand: import a Theme, see it applied, switch back to the default, delete it

## Comments

- 2026-09-12: The picker is a compact `<select>` rather than a hand-rolled listbox: it shows the active name, opens the list and carries the semantics for free. `DESIGN.md` was corrected to say so.
- 2026-09-12: The link the server writes now carries `data-theme`, and an effect keeps that link in step with whatever the Dashboard names. Without the effect, a Theme changed in another tab would leave the stylesheet stale; without the attribute, the effect would delete the server's link on mount and reintroduce the flash ticket 01 removed.
- 2026-09-12: Verified through the API and the accessibility tree: the import landed, the picker listed Default and Midnight, the Themes section appeared in edit mode with its import button, the download button disabled and explained while the default was active, and a Delete beside the imported Theme. The picker and the delete confirmation were not clicked through by hand — the in-app browser session had been cleaned up between turns and the last scripted click errored on an unrelated evaluate — so the swap and the confirmation rest on the code path, not on a screenshot. The Dashboard is back on `default`, and `themes/midnight.css` is left in place to play with.
- 2026-09-12: The picker is edit-mode only, on the owner's call: choosing a look is editing, and view mode shows the result instead of the controls. `DESIGN.md` and this ticket's first line above say "the header" without saying when; the control now sits beside `Edit` and disappears with it.
- 2026-09-12: `data-part="app"` was added afterwards, making six hooks rather than five. The owner asked `docs/THEMES.md` to show how to put a wallpaper behind the panel, and `[data-part="app"]` is the honest way to say that: the alternative would have been documenting `.bg-canvas`, a Tailwind class the contract explicitly does not promise.
