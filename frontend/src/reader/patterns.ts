import type { CSSProperties } from 'react';

// Chinese paper patterns rendered as CSS masks. The mask only carries shape;
// the reader paints it with the theme ink, so every pattern follows the current
// paper colour instead of shipping a fixed image.
export interface ReaderPattern {
  id: string;
  label: string;
  hint: string;
  /** A CSS `url("data:image/svg+xml,...")` mask. */
  mask: string;
  /** Tile size in CSS pixels. */
  size: string;
}

export const DEFAULT_PATTERN_OPACITY = 0.14;

function mask(width: number, height: number, body: string): string {
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" ` +
    `viewBox="0 0 ${width} ${height}" fill="none" stroke="black" stroke-width="1.1" ` +
    `stroke-linecap="round" stroke-linejoin="round">${body}</svg>`;
  return `url("data:image/svg+xml,${encodeURIComponent(svg)}")`;
}

export const READER_PATTERNS: ReaderPattern[] = [
  {
    id: 'hui',
    label: '回纹',
    hint: '回环不断，吉庆绵长',
    mask: mask(48, 48, '<path d="M6 42V6h36v36H18V18h12v12"/>'),
    size: '48px 48px',
  },
  {
    id: 'yun',
    label: '云纹',
    hint: '云气舒卷，如意随心',
    mask: mask(
      64,
      48,
      '<path d="M3 33c0-7 5-12 12-12 2-6 10-7 13-2 3-5 11-4 13 2 6 0 11 5 11 12 0 7-5 12-12 12H14C7 45 3 40 3 33z"/>' +
        '<path d="M18 33c0-3 2-5 5-5s5 2 5 5"/><path d="M41 33c0-3 2-5 5-5s5 2 5 5"/>',
    ),
    size: '64px 48px',
  },
  {
    id: 'wave',
    label: '水波',
    hint: '水波不兴，清雅平和',
    mask: mask(
      64,
      26,
      '<path d="M0 20c8 0 8-10 16-10s8 10 16 10 8-10 16-10 8 10 16 10"/>' +
        '<path d="M0 6c8 0 8-10 16-10s8 10 16 10 8-10 16-10 8 10 16 10" opacity=".55"/>',
    ),
    size: '64px 26px',
  },
  {
    id: 'ice',
    label: '冰裂',
    hint: '冰裂纹瓷，疏朗有致',
    mask: mask(
      96,
      96,
      '<path d="M4 6l20 20-8 24 26 14-8 26"/>' +
        '<path d="M44 2l8 24 26 6-10 28 22 20"/>' +
        '<path d="M6 62l20-8 16 20 26-6"/>' +
        '<path d="M66 0l-4 26 16 18"/>' +
        '<path d="M0 38l16 4 4 18"/>',
    ),
    size: '96px 96px',
  },
  {
    id: 'turtle',
    label: '龟背',
    hint: '龟背延年，六合安稳',
    mask: mask(
      54,
      48,
      '<path d="M14 2h26l13 22-13 22H14L1 24z"/>' +
        '<path d="M41 26h26L80 48 67 70H41L28 48z" opacity=".7"/>',
    ),
    size: '54px 48px',
  },
  {
    id: 'lozenge',
    label: '菱格',
    hint: '菱花映窗，秩序井然',
    mask: mask(
      32,
      32,
      '<path d="M16 1L31 16 16 31 1 16z"/><path d="M16 9L23 16 16 23 9 16z" opacity=".55"/>',
    ),
    size: '32px 32px',
  },
  {
    id: 'scale',
    label: '鱼鳞',
    hint: '鱼鳞叠翠，岁岁有余',
    mask: mask(
      48,
      24,
      '<path d="M0 21a12 12 0 0 1 24 0 12 12 0 0 1 24 0"/>' +
        '<path d="M-12 9a12 12 0 0 1 24 0 12 12 0 0 1 24 0 12 12 0 0 1 24 0" opacity=".6"/>',
    ),
    size: '48px 24px',
  },
  {
    id: 'fangsheng',
    label: '方胜',
    hint: '方胜同心，连绵相扣',
    mask: mask(
      48,
      32,
      '<path d="M12 2l14 14-14 14L-2 16z"/><path d="M30 2l14 14-14 14L16 16z" opacity=".7"/>',
    ),
    size: '48px 32px',
  },
  {
    id: 'vine',
    label: '缠枝',
    hint: '缠枝不断，生生不息',
    mask: mask(
      80,
      80,
      '<path d="M4 76C14 54 6 42 22 30S50 26 54 8"/>' +
        '<path d="M22 30c10-2 16 2 18 10"/>' +
        '<path d="M34 46c-8 4-12 10-12 18"/>' +
        '<path d="M54 8c10 4 12 12 10 20"/>' +
        '<ellipse cx="30" cy="23" rx="4" ry="2" fill="black" stroke="none" transform="rotate(-30 30 23)"/>' +
        '<ellipse cx="52" cy="38" rx="4" ry="2" fill="black" stroke="none" transform="rotate(30 52 38)"/>',
    ),
    size: '80px 80px',
  },
  {
    id: 'grass',
    label: '卷草',
    hint: '卷草回春，舒展自如',
    mask: mask(
      64,
      64,
      '<path d="M6 58C6 34 18 22 32 24s18 10 18 22-10 16-18 16-12-6-12-12 4-10 8-10 8 4 8 8"/>' +
        '<path d="M48 6c-2 10-8 16-16 18" opacity=".7"/>',
    ),
    size: '64px 64px',
  },
  {
    id: 'ruyi',
    label: '如意',
    hint: '如意顺遂，云头轻卷',
    mask: mask(
      64,
      32,
      '<path d="M2 26c0-8 6-12 12-12 2-6 10-6 12 0 2-6 10-6 12 0 6 0 12 4 12 12"/>' +
        '<path d="M14 26c0-3 2-5 5-5s5 2 5 5M40 26c0-3 2-5 5-5s5 2 5 5" opacity=".7"/>',
    ),
    size: '64px 32px',
  },
  {
    id: 'mat',
    label: '席纹',
    hint: '席纹朴素，编织成章',
    mask: mask(
      24,
      24,
      '<path d="M0 0l24 24M24 0L0 24" opacity=".8"/><path d="M0 12h24M12 0v24" opacity=".35"/>',
    ),
    size: '24px 24px',
  },
  {
    id: 'coin',
    label: '钱纹',
    hint: '钱纹团圆，交叠成锦',
    mask: mask(
      48,
      48,
      '<circle cx="24" cy="24" r="15"/><rect x="19" y="19" width="10" height="10"/>' +
        '<circle cx="0" cy="0" r="15" opacity=".5"/><circle cx="48" cy="0" r="15" opacity=".5"/>' +
        '<circle cx="0" cy="48" r="15" opacity=".5"/><circle cx="48" cy="48" r="15" opacity=".5"/>',
    ),
    size: '48px 48px',
  },
  {
    id: 'plum',
    label: '梅花',
    hint: '梅花清供，暗香疏影',
    mask: mask(
      48,
      48,
      '<circle cx="24" cy="12" r="5"/><circle cx="35" cy="20" r="5"/>' +
        '<circle cx="31" cy="33" r="5"/><circle cx="17" cy="33" r="5"/>' +
        '<circle cx="13" cy="20" r="5"/><circle cx="24" cy="24" r="2" fill="black" stroke="none"/>',
    ),
    size: '48px 48px',
  },
];

export function readerPattern(id?: string): ReaderPattern | undefined {
  if (!id) return undefined;
  return READER_PATTERNS.find((pattern) => pattern.id === id);
}

export function patternMaskStyle(pattern: ReaderPattern): CSSProperties {
  return {
    WebkitMaskImage: pattern.mask,
    maskImage: pattern.mask,
    WebkitMaskSize: pattern.size,
    maskSize: pattern.size,
    WebkitMaskRepeat: 'repeat',
    maskRepeat: 'repeat',
    WebkitMaskPosition: 'center',
    maskPosition: 'center',
  };
}
