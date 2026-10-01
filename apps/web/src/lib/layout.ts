// The grid's arithmetic, in one place.
//
// This module exists because a drag has to predict what CSS will do. `app.css`
// owns the truth — the column template, the breakpoint gaps and the row height
// (DESIGN.md §6) — and everything here mirrors it: the two breakpoints, the
// bounds of a Span, and the conversion from a pointer's travel in pixels to a
// number of columns and rows. Nothing here reads a Token by name, because the
// caller measures the DOM and hands the numbers over (ADR-0004, one source for
// the values; DESIGN.md §8, the arithmetic mirrors the CSS and the tests below
// fail when the two drift).

/** A Widget's size on the grid: columns wide, rows tall, 1–12 each. */
export interface Span {
  w: number;
  h: number;
}

/**
 * What a resize needs to know about the grid it is happening in. The caller
 * measures it where it is drawn, so a Theme that repaints the gap or the row
 * height changes the drag without changing this file.
 */
export interface GridMetrics {
  /** Columns in the grid at this width: 1, 6 or 12. */
  columns: number;
  /** The gap between columns, in pixels. */
  gapPx: number;
  /** One row, in pixels. */
  rowHeightPx: number;
  /** The width of one column, in pixels, gap excluded. */
  columnPx: number;
}

/**
 * Which edges the owner grabbed. The names are the edges, not the axes: a
 * pointer on the right edge is resizing the width even though it moves along x.
 */
export type ResizeAxis = "width" | "height" | "both";

/** The grid's smallest and largest Span, in columns and rows alike. */
export const MIN_SPAN = 1;
export const MAX_SPAN = 12;

/** DESIGN.md §6: one column under 640, six to 1023, twelve from 1024. */
export const NARROW_BREAKPOINT = 640;
export const WIDE_BREAKPOINT = 1024;

const NARROW_COLUMNS = 1;
const MEDIUM_COLUMNS = 6;
const WIDE_COLUMNS = 12;

/**
 * The column count a viewport width gets. It takes the width rather than
 * reading `window`, so the breakpoints are testable without a browser — the
 * drag and the tests go through one function.
 *
 * The comparison is `>=` at each breakpoint because that is what the CSS says
 * (`@media (min-width: 640px)`), and a drag that disagrees with the layout by
 * one pixel is a drag that snaps to the wrong column.
 *
 * A width that is not a number is the narrowest grid, because a drag that
 * cannot measure anything must not offer the owner a resize it cannot honour;
 * an infinite one is simply wider than every breakpoint.
 */
export function columnsForWidth(viewportWidth: number): number {
  if (Number.isNaN(viewportWidth)) return NARROW_COLUMNS;
  if (viewportWidth >= WIDE_BREAKPOINT) return WIDE_COLUMNS;
  if (viewportWidth >= NARROW_BREAKPOINT) return MEDIUM_COLUMNS;
  return NARROW_COLUMNS;
}

/**
 * The Span a drag lands on.
 *
 * `from` is the Span the gesture started at — not the Widget's current one, so
 * a pointer that wanders back to where it began returns the card to where it
 * began instead of accumulating rounding. `travel` is how far the pointer has
 * moved in pixels, `metrics` is the grid measured at the start of the gesture,
 * and `axis` says which edges were grabbed: a right edge never changes `h`, and
 * a bottom edge never changes `w`.
 *
 * A row is a minimum (DESIGN.md §6), so this grows a card and never clips one:
 * the result is clamped to at least one cell and at most the grid's width.
 */
export function resizeSpan(
  from: Span,
  travel: { dxPx: number; dyPx: number },
  metrics: GridMetrics,
  axis: ResizeAxis
): Span {
  const vertical = axis === "height" || axis === "both";
  const horizontal = axis === "width" || axis === "both";

  const w = horizontal
    ? clampSpan(from.w + wholeCells(travel.dxPx, metrics.columnPx), metrics.columns)
    : clampSpan(from.w, metrics.columns);
  const h = vertical
    ? clampSpan(from.h + wholeCells(travel.dyPx, metrics.rowHeightPx), MAX_SPAN)
    : clampSpan(from.h, MAX_SPAN);

  return { w, h };
}

/**
 * How many cells a travel of `pixels` is worth. A cell counts once the pointer
 * has crossed most of the way into the next one, so a drag that stops halfway
 * across a gap keeps the Span it started with: rounding to the nearest whole
 * cell is what makes the gesture feel like it has detents rather than a
 * hair-trigger.
 */
function wholeCells(pixels: number, cellPx: number): number {
  if (!Number.isFinite(pixels) || !Number.isFinite(cellPx) || cellPx <= 0) return 0;
  return Math.round(pixels / cellPx);
}

/** A Span never leaves the grid: at least one cell, at most the grid's width. */
export function clampSpan(value: number, maximum: number): number {
  const ceiling = Math.max(MIN_SPAN, Math.min(MAX_SPAN, Math.trunc(maximum)));
  if (!Number.isFinite(value)) return MIN_SPAN;
  return Math.min(ceiling, Math.max(MIN_SPAN, Math.trunc(value)));
}
