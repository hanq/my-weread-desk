import type { CSSProperties } from 'react';

import type { Theme } from '../types';
import { DEFAULT_PATTERN_OPACITY, readerPattern } from './patterns';

export function fontStack(theme: Theme): string {
  const families = [theme.fontFamily, theme.fallbackFonts]
    .flatMap((f) => (f || '').split(','))
    .map((f) => f.trim())
    .filter(Boolean)
    .map((f) => (f.includes(' ') && !f.startsWith('"') ? `"${f}"` : f));
  if (!families.some((f) => /serif|monospace|sans-serif/i.test(f))) {
    families.push('serif');
  }
  return families.join(', ');
}

export interface ThemeVarOptions {
  pageWidth: number;
  paperWidth?: number;
  pad?: number;
  gap: number;
  colWidth: number;
  /** Columns per page. Forced via column-count so a narrow measure cannot
   * silently collapse a two-column layout into one column. */
  columns: number;
  /** Maximum height of inline images; defaults to a font-based guess. */
  imgMax?: number;
  /** Skins provide their own surface, so the paper keeps its own background. */
  paper?: 'themed' | 'transparent';
  marginX: number;
  marginY: number;
}

export function themeVars(theme: Theme, opts: ThemeVarOptions): CSSProperties {
  const surface = theme.surface || 'transparent';
  const isTransparent = theme.background === 'transparent' || theme.surface === 'transparent';
  const pattern = readerPattern(theme.pattern);
  const patternOpacity = Math.min(0.4, Math.max(0, theme.patternOpacity ?? DEFAULT_PATTERN_OPACITY));
  return {
    '--r-bg': theme.background || 'transparent',
    '--r-surface': opts.paper === 'transparent' ? 'transparent' : surface,
    '--r-ink': theme.textColor,
    '--r-heading': theme.headingColor || theme.textColor,
    '--r-accent': theme.accentColor || '#4d84ff',
    '--r-selection': theme.selectionColor || 'rgba(77,132,255,0.25)',
    '--r-font': fontStack(theme),
    '--r-size': `${theme.fontSize}px`,
    '--r-lh': String(theme.lineHeight),
    '--r-ls': `${theme.letterSpacing}em`,
    '--r-para': `${theme.paragraphSpacing}px`,
    '--r-indent': `${theme.textIndent}em`,
    '--r-align': theme.textAlign || 'justify',
    '--r-page-w': `${Math.round(opts.paperWidth ?? opts.pageWidth)}px`,
    '--r-text-w': `${Math.round(opts.pageWidth)}px`,
    '--r-col-w': `${Math.round(opts.colWidth)}px`,
    '--r-cols': String(opts.columns || 1),
    '--r-gap': `${Math.round(opts.gap)}px`,
    '--r-margin-x': `${Math.round(opts.marginX)}px`,
    '--r-margin-y': `${Math.round(opts.marginY)}px`,
    '--r-img-max': `${Math.round(opts.imgMax ?? theme.fontSize * theme.lineHeight * 14)}px`,
    '--r-paper-pad': `${Math.round(opts.pad ?? 0)}px`,
    '--r-paper-shadow': isTransparent ? 'none' : 'var(--shadow-paper)',
    '--r-paper-radius': isTransparent ? '0px' : '6px',
    '--r-pattern-mask': pattern?.mask ?? 'none',
    '--r-pattern-size': pattern?.size ?? 'auto',
    '--r-pattern-opacity': String(patternOpacity),
  } as CSSProperties;
}

export function nestedVars(theme: Theme, extra: Record<string, string>): CSSProperties {
  return { ...theme, ...extra } as unknown as CSSProperties;
}
