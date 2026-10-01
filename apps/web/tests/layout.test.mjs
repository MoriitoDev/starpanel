// The grid's arithmetic against the stylesheet it mirrors.
//
// `apps/web/src/lib/layout.ts` exists so a drag can predict what CSS will do,
// and the failure mode of a mirror is silent drift: someone moves a breakpoint
// in app.css, the drag keeps snapping to the old one, and no test notices. So
// these cases read app.css itself and assert the numbers agree (DESIGN.md §8).
//
// Run: pnpm test   (from apps/web)

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import {
  MAX_SPAN,
  MIN_SPAN,
  NARROW_BREAKPOINT,
  WIDE_BREAKPOINT,
  clampSpan,
  columnsForWidth,
  resizeSpan
} from "../src/lib/layout.ts";

const css = readFileSync(fileURLToPath(new URL("../src/app.css", import.meta.url)), "utf8");

/**
 * The `@media (min-width: Npx)` block that declares the grid's columns.
 *
 * The same breakpoint appears several times in the stylesheet — the grid, the
 * resize handles, the dialog — so a block is chosen by what it declares rather
 * than merely by where it starts, and its braces are balanced rather than
 * matched lazily. A lazy `[\s\S]*?` from the 640px breakpoint reaches the
 * 1024px block's value, and a test that reads the wrong block passes for the
 * wrong reason.
 */
function gridBlockAt(minWidth) {
  const needle = `@media (min-width: ${minWidth}px)`;
  for (let start = css.indexOf(needle); start >= 0; start = css.indexOf(needle, start + 1)) {
    const open = css.indexOf("{", start);
    let depth = 0;
    for (let i = open; i < css.length; i += 1) {
      if (css[i] === "{") depth += 1;
      else if (css[i] === "}") {
        depth -= 1;
        if (depth === 0) {
          const body = css.slice(open, i + 1);
          if (body.includes("grid-template-columns")) return body;
          break;
        }
      }
    }
  }
  return null;
}

/** The columns a breakpoint declares, or null when it does not change them. */
function columnsAt(minWidth) {
  const block = gridBlockAt(minWidth);
  const match = block && /grid-template-columns:\s*repeat\((\d+),/.exec(block);
  return match ? Number(match[1]) : null;
}

/**
 * The grid gap in force, in pixels: the base `.dashboard-grid` rule, overridden
 * by whichever breakpoints raise it. 640px changes only the columns, so the
 * gap in force there is still the base one — which is exactly the kind of
 * detail a mirror gets wrong when it guesses instead of reading.
 */
function gapAt(width) {
  const base = /\.dashboard-grid\s*\{[^}]*--grid-gap:\s*(\d+)px/.exec(css);
  let gap = base ? Number(base[1]) : null;
  for (const minWidth of [NARROW_BREAKPOINT, WIDE_BREAKPOINT]) {
    if (width < minWidth) continue;
    const block = gridBlockAt(minWidth);
    const match = block && /--grid-gap:\s*(\d+)px/.exec(block);
    if (match) gap = Number(match[1]);
  }
  return gap;
}

test("the breakpoints in the arithmetic are the ones in the stylesheet", () => {
  const breakpoints = [...css.matchAll(/@media \(min-width: (\d+)px\)/g)].map((m) => Number(m[1]));
  assert.ok(
    breakpoints.includes(NARROW_BREAKPOINT),
    `app.css has no @media (min-width: ${NARROW_BREAKPOINT}px): ${[...new Set(breakpoints)].join(", ")}`
  );
  assert.ok(
    breakpoints.includes(WIDE_BREAKPOINT),
    `app.css has no @media (min-width: ${WIDE_BREAKPOINT}px)`
  );
});

test("the column count matches the stylesheet at each breakpoint", () => {
  assert.equal(
    columnsForWidth(WIDE_BREAKPOINT),
    columnsAt(WIDE_BREAKPOINT),
    `at ${WIDE_BREAKPOINT}px the stylesheet draws ${columnsAt(WIDE_BREAKPOINT)} columns`
  );
  assert.equal(
    columnsForWidth(NARROW_BREAKPOINT),
    columnsAt(NARROW_BREAKPOINT),
    `at ${NARROW_BREAKPOINT}px the stylesheet draws ${columnsAt(NARROW_BREAKPOINT)} columns`
  );
  assert.equal(columnsForWidth(NARROW_BREAKPOINT - 1), 1, "one column below the first breakpoint");
});

test("one column at the smallest width, and the bound is the grid's", () => {
  assert.equal(columnsForWidth(320), 1);
  assert.equal(columnsForWidth(639), 1);
  assert.equal(columnsForWidth(1023), 6);
  assert.equal(columnsForWidth(1440), 12);
});

test("a viewport width that is not a number is the narrowest grid", () => {
  assert.equal(columnsForWidth(Number.NaN), 1);
  assert.equal(columnsForWidth(Number.POSITIVE_INFINITY), 12);
});

test("the gap the drag reads is the one in force at that width", () => {
  // The stylesheet starts at 16px and the 1024px breakpoint raises it to 24.
  assert.ok(gapAt(320) > 0, "the base rule declares no --grid-gap");
  assert.equal(gapAt(320), gapAt(639), "nothing between the breakpoints changes the gap");
  assert.notEqual(
    gapAt(320),
    gapAt(1200),
    "the wide breakpoint is expected to change the gap; if it no longer does, this case and the drag's assumption both need revisiting"
  );
});

test("the row height the drag reads is the Token the stylesheet sets", () => {
  const rowHeight = /--row-height: (\d+)px/.exec(css);
  assert.ok(rowHeight, "app.css declares no --row-height");
  assert.ok(Number(rowHeight[1]) > 0, "--row-height must be a positive length");
});

test("a drag grows a card by the cells the pointer crossed", () => {
  const metrics = { columns: 12, gapPx: 24, rowHeightPx: 120, columnPx: 100 };
  // Half a column is not a column; a whole one is.
  assert.deepEqual(resizeSpan({ w: 2, h: 2 }, { dxPx: 40, dyPx: 0 }, metrics, "width"), { w: 2, h: 2 });
  assert.deepEqual(resizeSpan({ w: 2, h: 2 }, { dxPx: 100, dyPx: 0 }, metrics, "width"), { w: 3, h: 2 });
  assert.deepEqual(resizeSpan({ w: 2, h: 2 }, { dxPx: 260, dyPx: 0 }, metrics, "width"), { w: 5, h: 2 });
});

test("a row is a minimum, so a downward drag grows and never clips", () => {
  const metrics = { columns: 12, gapPx: 24, rowHeightPx: 120, columnPx: 100 };
  assert.deepEqual(resizeSpan({ w: 2, h: 1 }, { dxPx: 0, dyPx: 120 }, metrics, "height"), { w: 2, h: 2 });
  assert.deepEqual(resizeSpan({ w: 2, h: 2 }, { dxPx: 0, dyPx: -400 }, metrics, "height"), { w: 2, h: 1 });
});

test("an edge changes only the axis it was grabbed by", () => {
  const metrics = { columns: 12, gapPx: 24, rowHeightPx: 120, columnPx: 100 };
  // A right edge, dragged diagonally: the height must not move.
  assert.deepEqual(resizeSpan({ w: 2, h: 2 }, { dxPx: 200, dyPx: 500 }, metrics, "width"), {
    w: 4,
    h: 2
  });
  // A bottom edge, dragged diagonally: the width must not move.
  assert.deepEqual(resizeSpan({ w: 2, h: 2 }, { dxPx: 200, dyPx: 240 }, metrics, "height"), {
    w: 2,
    h: 4
  });
  // The corner moves both.
  assert.deepEqual(resizeSpan({ w: 2, h: 2 }, { dxPx: 100, dyPx: 120 }, metrics, "both"), {
    w: 3,
    h: 3
  });
});

test("a Span never leaves the grid", () => {
  const twelve = { columns: 12, gapPx: 24, rowHeightPx: 120, columnPx: 100 };
  const six = { columns: 6, gapPx: 16, rowHeightPx: 120, columnPx: 100 };
  // Dragged far past the end: it stops at the grid's width, not at 12 blindly.
  assert.equal(resizeSpan({ w: 5, h: 1 }, { dxPx: 99999, dyPx: 0 }, six, "width").w, 6);
  assert.equal(resizeSpan({ w: 5, h: 1 }, { dxPx: 99999, dyPx: 0 }, twelve, "width").w, 12);
  // Dragged far before the start: one cell, never zero or negative.
  assert.equal(
    resizeSpan({ w: 5, h: 5 }, { dxPx: -99999, dyPx: -99999 }, twelve, "both").w,
    MIN_SPAN
  );
  assert.equal(
    resizeSpan({ w: 5, h: 5 }, { dxPx: -99999, dyPx: -99999 }, twelve, "both").h,
    MIN_SPAN
  );
  assert.equal(clampSpan(99, 12), MAX_SPAN);
  assert.equal(clampSpan(0, 12), MIN_SPAN);
});

test("a grid that was not measured cannot produce a wild Span", () => {
  // A column of zero pixels would divide by zero; the drag must keep the Span
  // rather than jump to the grid's edge.
  const unmeasured = { columns: 12, gapPx: 0, rowHeightPx: 0, columnPx: 0 };
  assert.deepEqual(resizeSpan({ w: 3, h: 2 }, { dxPx: 500, dyPx: 500 }, unmeasured, "both"), {
    w: 3,
    h: 2
  });
});
