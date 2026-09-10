# 03: Theme v2, semantic tokens

**What to build:** the Theme reduced to fourteen semantic colors plus a `light | dark | auto` mode, across Go, the stored document and the frontend types — with stored v1 palettes falling back to the new defaults.

**Blocked by:** 01 design-system-doc.

**Status:** done

- [x] `Palette` carries exactly the fourteen `DESIGN.md` tokens and `Theme` carries `mode` plus one Palette per mode; `auto` is a valid mode
- [x] The light and dark defaults match `DESIGN.md` value for value
- [x] `Normalize`, `Validate` and `Migrate` cover the new shape, and the legacy machinery (`expandOld`, `hasOld`, the three-color aliases, the flat-theme fields) is deleted
- [x] A v1 document loads as the new defaults without an error, and `apps/api/data/dashboard.json` is rewritten to the new shape
- [x] Go tests updated: default palettes, rejection of a missing token, `auto` accepted, and a v1 document migrating to defaults
- [x] `apps/web/src/types.ts` matches the new shape and its `DESIGN-posthog.md` comment points at `DESIGN.md`

## Comments

- 2026-09-10: Tests were written first and went red for the right reasons, including one finding worth keeping: the v1 and v2 token sets share six names (`canvas`, `ink`, `body`, `mute`, `surfaceSoft`, `focusRing`), so filling a stored palette token by token *kept* v1 colours under v2 names and never reached the defaults. Migration is now all-or-nothing — a Palette missing any token is replaced wholesale (`Palette.healed`) — which is what the spec's "resets existing documents" meant. The spec wording was corrected in the same commit.
- 2026-09-10: New tests cover three legacy document shapes: the first persist build's flat theme, the pre-design three-colour palettes, and the full v1 29-token palette. All three load as the defaults with the stored `mode` preserved, and `go vet` is clean.
- 2026-09-10: Transient compile gap, which the ticket split implies: between this ticket and 04 the frontend still referenced the removed token names, so `pnpm check` failed until 04 landed. They are one change in practice.
