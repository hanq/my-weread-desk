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
  const marginX = flush ? 0 : Math.max(0, theme.marginX);
  const marginY = flush ? 0 : Math.max(0, theme.marginY);
  const availOuter = Math.max(240, box.w - marginX * 2 - (gutter ? GUTTER_WIDTH : 0));
  // The paper keeps a visual margin around the text so the columns never touch
  // the sheet edge, which would look like clipped text.
  const pad = clamp(Math.round(availOuter * 0.055), 14, 54);
  const avail = Math.max(240, availOuter - pad * 2);
  const pageWidth = Math.max(240, theme.pageWidth > 0 ? Math.min(theme.pageWidth, avail) : avail);
  const columns = clamp(Math.round(theme.columns || 1), 1, 2);
  const gap = Math.max(20, fontSize * 2.4);
  const colWidth = Math.max(120, (pageWidth - (columns - 1) * gap) / columns);
  const height = Math.max(120, box.h - marginY * 2);
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

  const remeasure = useCallback(() => {
    const el = trackRef.current;
    if (!el) return;
    const { step, perPage, gap } = geometry;
    const raw = (el.scrollWidth + gap) / step;
    const total = Math.max(1, Math.round(raw));
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

  const setPage = useCallback(
    (p: number) => {
      setPageState((prev) => {
        const next = clamp(Math.round(p), 0, Math.max(0, pages - 1));
        return next === prev ? prev : next;
      });
    },
    [pages],
  );

  useEffect(() => {
    setPageState((prev) => clamp(prev, 0, Math.max(0, pages - 1)));
  }, [pages]);

  return { page, pages, setPage, remeasure };
}

/** Reads the page that currently contains the typing caret. */
export function caretPage(
  caret: HTMLElement | null,
  track: HTMLElement | null,
  geometry: Geometry,
): number | null {
  if (!caret || !track) return null;
  const c = caret.getBoundingClientRect();
  const t = track.getBoundingClientRect();
  const column = Math.floor((c.left - t.left) / geometry.step);
  if (!Number.isFinite(column) || column < 0) return null;
  return Math.floor(column / geometry.perPage);
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
