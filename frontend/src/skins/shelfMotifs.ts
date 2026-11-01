// Interface carriers. A motif is not a disguise skin: it drives the shelf and
// the app chrome, while the reading canvas keeps its own independent theme.
import { useSyncExternalStore } from 'react';

export type ShelfMotif = 'paper' | 'bamboo' | 'jade' | 'orb';

export interface ShelfMotifOption {
  id: ShelfMotif;
  /** Short name shown on the dial knob and in tooltips. */
  label: string;
  /** Single-glyph mark used by the half-circle switcher. */
  glyph: string;
  hint: string;
}

export const SHELF_MOTIFS: ShelfMotifOption[] = [
  { id: 'paper', label: '纸页', glyph: '纸', hint: '素纸书脊，安静克制' },
  { id: 'bamboo', label: '竹简', glyph: '竹', hint: '韦编竹牍，竖排书名' },
  { id: 'jade', label: '玉简', glyph: '玉', hint: '青玉书版，温润通透' },
  { id: 'orb', label: '圆珠', glyph: '珠', hint: '环形封面，腕表蜂窝' },
];

export const DEFAULT_SHELF_MOTIF: ShelfMotif = 'paper';

export function isShelfMotif(value: unknown): value is ShelfMotif {
  return SHELF_MOTIFS.some((option) => option.id === value);
}

export function shelfMotifLabel(motif: ShelfMotif): string {
  return SHELF_MOTIFS.find((option) => option.id === motif)?.label ?? '纸页';
}

// The shelf, settings and shell read this module-level store so changing the
// carrier in one place updates every surface, including the title bar and sidebar.
const MOTIF_KEY = 'my-weread-desk:shelf-motif';

function readMotif(): ShelfMotif {
  if (typeof window === 'undefined') return DEFAULT_SHELF_MOTIF;
  try {
    const raw = window.localStorage.getItem(MOTIF_KEY);
    if (isShelfMotif(raw)) return raw;
  } catch {
    /* Storage can be unavailable; fall back to the default. */
  }
  return DEFAULT_SHELF_MOTIF;
}

let currentMotif = readMotif();
const listeners = new Set<() => void>();

export function getShelfMotif(): ShelfMotif {
  return currentMotif;
}

export function setShelfMotif(next: ShelfMotif) {
  if (next === currentMotif) return;
  currentMotif = next;
  try {
    window.localStorage.setItem(MOTIF_KEY, next);
  } catch {
    /* The choice still applies for this session. */
  }
  listeners.forEach((listener) => listener());
}

function subscribeMotif(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/** Live interface carrier, shared by the shelf dial, Settings and the app shell. */
export function useShelfMotif(): ShelfMotif {
  return useSyncExternalStore(subscribeMotif, getShelfMotif, () => DEFAULT_SHELF_MOTIF);
}
