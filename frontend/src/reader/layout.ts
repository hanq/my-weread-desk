import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';

import type { Theme } from '../types';

export interface Geometry {
  /** Width of the text column area. */
  pageWidth: number;
  /** Width of the paper sheet, including its inner padding. */
  paperWidth: number;
  /** Inner padding between the paper edge and the text. */
  pad: number;
  gap: number;
  colWidth: number;
  perPage: number;
  step: number;
  height: number;
  linesPerPage: number;
}

const GUTTER_WIDTH = 44;

/**
 * Minimum text-area width for the automatic two-column layout. Below it each
 * column would be narrower than a comfortable measure, so the reader keeps a
 * single column even though the window is wider than a phone.
 */
export const AUTO_COLUMNS_MIN_WIDTH = 1080;

/**
 * Resolves the column count for the current text area. A theme column value of
 * 0 means automatic: one column on narrow windows, two once the text area is
 * wide enough. A fixed page width (pageWidth > 0) keeps the manual choice, so
 * two 300px columns never appear inside a deliberately narrow measure.
 */
export function resolveColumns(theme: Theme, pageWidth: number): number {
  const manual = clamp(Math.round(theme.columns || 1), 1, 2);
  if (theme.columns !== 0) return manual;
  if (theme.pageWidth > 0) return manual;
  return pageWidth >= AUTO_COLUMNS_MIN_WIDTH ? 2 : 1;
}

/**
 * Recomputes the column geometry for another column count without measuring
 * the stage again. The page step only depends on the page width, so it stays
 * the same; only the column width and the CSS column count change.
 */
export function withColumns(geometry: Geometry, columns: number): Geometry {
  if (columns === geometry.perPage) return geometry;
  const colWidth = Math.max(120, (geometry.pageWidth - (columns - 1) * geometry.gap) / columns);
  return { ...geometry, perPage: columns, colWidth };
}

function clamp(v: number, lo: number, hi: number) {
  return Math.min(hi, Math.max(lo, v));
}

/**
 * Measures the reading stage and derives the column geometry. The page width is
 * clamped to the available space so a wide "pageWidth" never causes a
 * horizontal scrollbar.
 */
export function useGeometry(
  stageRef: React.RefObject<HTMLElement | null>,
  theme: Theme,
  gutter: boolean,
  mountKey: unknown,
  flush = false,
  inset?: { x?: number; y?: number },
  floating = false,
): Geometry {
  const [box, setBox] = useState({ w: 0, h: 0 });
  const observed = useRef<HTMLElement | null>(null);
  const observer = useRef<ResizeObserver | null>(null);

  // The stage element only exists once the chapter is loaded and it is
  // re-created when the skin changes, so the observer is re-attached whenever
  // the underlying DOM node changes. There is intentionally no cleanup on every
  // render: that would disconnect the observer the first time React re-renders.
  useLayoutEffect(() => {
    const el = stageRef.current;
    if (!el) {
      observed.current = null;
      return;
    }
    if (observed.current === el && observer.current) return;
    observer.current?.disconnect();
    observed.current = el;

    const read = () => {
      const next = { w: el.clientWidth, h: el.clientHeight };
      setBox((prev) => (prev.w === next.w && prev.h === next.h ? prev : next));
    };
    read();
    const ro = new ResizeObserver(read);
    ro.observe(el);
    observer.current = ro;
  });

  useEffect(() => () => observer.current?.disconnect(), []);

  const fontSize = Math.max(10, theme.fontSize || 18);
  // Skins render a flush stage: their own chrome already provides the margins,
  // so subtracting the theme margins twice would shrink the text column.
  const marginX = flush || floating ? 0 : Math.max(0, theme.marginX);
  const marginY = flush || floating ? 0 : Math.max(0, theme.marginY);
  // Skins whose stage adds its own padding (VSCode) report it here so the
  // measured column never overflows the padded content box.
  const insetX = flush ? Math.max(0, inset?.x ?? 0) : 0;
  const insetY = flush ? Math.max(0, inset?.y ?? 0) : 0;
  const availOuter = Math.max(240, box.w - marginX * 2 - (gutter ? GUTTER_WIDTH : 0) - insetX);
  // The paper keeps a visual margin around the text so the columns never touch
  // the sheet edge, which would look like clipped text.
  // Keep only a hairline gutter between the sheet and the window; the theme
  // margins are already small by default.
  const pad = floating ? 0 : clamp(Math.round(availOuter * 0.02), 8, 16);
  const avail = floating ? availOuter : Math.max(240, availOuter - pad * 2);
  const pageWidth = floating ? availOuter : Math.max(240, theme.pageWidth > 0 ? Math.min(theme.pageWidth, avail) : avail);
  const columns = clamp(Math.round(theme.columns || 1), 1, 2);
  // Round the gap here as well as in the CSS variable: a fractional gap makes
  // the translate step differ from the real column pitch by a fraction of a
  // pixel per page, which accumulates on long chapters.
  const gap = Math.max(20, Math.round(fontSize * 2.4));
  const colWidth = Math.max(120, (pageWidth - (columns - 1) * gap) / columns);
  const height = Math.max(120, box.h - marginY * 2 - insetY);
  const lineHeightPx = fontSize * (theme.lineHeight || 1.6);
  const linesPerPage = Math.max(1, Math.floor((height - pad * 2) / lineHeightPx));

  return {
    pageWidth,
    paperWidth: pageWidth + pad * 2,
    pad,
    gap,
    colWidth,
    perPage: columns,
    step: pageWidth + gap,
    height,
    linesPerPage,
  };
}

/**
 * Computes the page count from the track's scroll width and keeps the current
 * page inside range. Re-measures on content, geometry, font and image changes.
 */
export function usePagination(
  trackRef: React.RefObject<HTMLElement | null>,
  geometry: Geometry,
  signature: unknown,
): { page: number; pages: number; setPage: (p: number) => void; remeasure: () => void } {
  const [pages, setPages] = useState(1);
  const [page, setPageState] = useState(0);
  // The measured count is also kept in a ref so a caller can clamp a page right
  // after a re-layout, before React has committed the new `pages` state.
  const pagesRef = useRef(1);

  const remeasure = useCallback(() => {
    const el = trackRef.current;
    if (!el) return;
    const { step, gap } = geometry;
    const raw = (el.scrollWidth + gap) / step;
    const total = Math.max(1, Math.round(raw));
    pagesRef.current = total;
    setPages(total);
  }, [trackRef, geometry]);

  useLayoutEffect(() => {
    remeasure();
  }, [remeasure, signature]);

  useEffect(() => {
    const onRelayout = () => remeasure();
    window.addEventListener('reader:relayout', onRelayout);
    window.addEventListener('resize', onRelayout);
    if (document.fonts?.ready) void document.fonts.ready.then(onRelayout);
    return () => {
      window.removeEventListener('reader:relayout', onRelayout);
      window.removeEventListener('resize', onRelayout);
    };
  }, [remeasure]);

  const setPage = useCallback((p: number) => {
    setPageState((prev) => {
      const next = clamp(Math.round(p), 0, Math.max(0, pagesRef.current - 1));
      return next === prev ? prev : next;
    });
  }, []);

  useEffect(() => {
    setPageState((prev) => clamp(prev, 0, Math.max(0, pages - 1)));
  }, [pages]);

  return { page, pages, setPage, remeasure };
}

/**
 * Reads the page that currently contains the typing caret. A page advances by
 * `step` pixels (all columns of one page), so the horizontal offset already
 * yields the page index; dividing by the column count would halve it on
 * two-column pages.
 */
export function caretPage(
  caret: HTMLElement | null,
  track: HTMLElement | null,
  geometry: Geometry,
): number | null {
  if (!caret || !track) return null;
  return pageAt(caret.getBoundingClientRect().left, track, geometry.step);
}

function pageAt(left: number, track: HTMLElement, step: number): number | null {
  const origin = track.getBoundingClientRect().left;
  const page = Math.floor((left - origin) / step);
  if (!Number.isFinite(page) || page < 0) return null;
  return page;
}

/**
 * Rune offset of the first character visible at the page's top-left corner.
 * Comparing this offset across re-layouts keeps the reader on the same line
 * when the window, font size or page geometry changes.
 */
export function readAnchorOffset(
  viewport: HTMLElement | null,
  track: HTMLElement | null,
): number | null {
  if (!viewport || !track) return null;
  const box = viewport.getBoundingClientRect();
  if (box.width <= 0 || box.height <= 0) return null;
  const line = parseFloat(getComputedStyle(track).lineHeight) || 24;
  // Probe inside the first line box, not in the gap above or below it. The
  // leftmost caret of a line is its first character even with a text indent.
  for (const dy of [0.35, 0.6, 0.85]) {
    const y = box.top + Math.min(line * dy, box.height - 1);
    for (const dx of [3, 16, 48, 140]) {
      const range = document.caretRangeFromPoint(box.left + dx, y);
      if (!range || range.startContainer.nodeType !== Node.TEXT_NODE) continue;
      const offset = runeOffset(track, range.startContainer, range.startOffset);
      if (offset !== null) return offset;
    }
  }
  return null;
}

/**
 * Page that holds the given rune offset in the current layout. The offset is
 * resolved back to a text caret, so this works even when the number of columns
 * per page changed between the two layouts.
 */
export function pageForOffset(
  track: HTMLElement | null,
  step: number,
  offset: number,
): number | null {
  if (!track) return null;
  let seen = 0;
  const walker = document.createTreeWalker(track, NodeFilter.SHOW_TEXT);
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    const text = node as Text;
    const runes = Array.from(text.nodeValue ?? '');
    if (offset < seen + runes.length) {
      const local = runes.slice(0, offset - seen).join('').length;
      const range = document.createRange();
      range.setStart(text, Math.min(local, text.length));
      range.collapse(true);
      return pageAt(range.getBoundingClientRect().left, track, step);
    }
    seen += runes.length;
  }
  return null;
}

/**
 * Scroll offset that puts the given rune offset at the top of a vertical
 * viewport. Text nodes are walked so images and blank blocks do not distort
 * the mapping; the caller can apply the result to a scroll container.
 */
export function scrollTopForOffset(
  viewport: HTMLElement | null,
  track: HTMLElement | null,
  offset: number,
): number | null {
  if (!viewport || !track) return null;
  if (offset <= 0) return 0;
  let seen = 0;
  const view = viewport.getBoundingClientRect();
  const walker = document.createTreeWalker(track, NodeFilter.SHOW_TEXT);
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    const text = node as Text;
    const runes = Array.from(text.nodeValue ?? '');
    if (offset < seen + runes.length) {
      const local = runes.slice(0, offset - seen).join('').length;
      const range = document.createRange();
      range.setStart(text, Math.min(local, text.length));
      range.collapse(true);
      const rect = range.getBoundingClientRect();
      return Math.max(0, viewport.scrollTop + rect.top - view.top);
    }
    seen += runes.length;
  }
  return viewport.scrollHeight;
}

function runeOffset(root: HTMLElement, node: Node, offset: number): number | null {
  let seen = 0;
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    const text = n as Text;
    if (text === node) return seen + Array.from((text.nodeValue ?? '').slice(0, offset)).length;
    seen += Array.from(text.nodeValue ?? '').length;
  }
  return null;
}

/** Reveal steps for typing mode: block boundaries are computed by the caller. */
export function useRafCallback(fn: () => void) {
  const raf = useRef<number | null>(null);
  useEffect(() => () => {
    if (raf.current != null) cancelAnimationFrame(raf.current);
  }, []);
  return useCallback(() => {
    if (raf.current != null) return;
    raf.current = requestAnimationFrame(() => {
      raf.current = null;
      fn();
    });
  }, [fn]);
}
