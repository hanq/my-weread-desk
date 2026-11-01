import type { BookKind } from './types';

// The same labels the Go classifier uses in errors and AIStatus. Keep the
// order stable: it is also the order shown in the book detail selector.
export const BOOK_KIND_OPTIONS: { value: BookKind; label: string }[] = [
  { value: 'fiction', label: '小说' },
  { value: 'nonfiction', label: '非虚构' },
  { value: 'textbook', label: '教材' },
  { value: 'periodical', label: '期刊' },
  { value: 'medical', label: '医学' },
  { value: 'imaging', label: '影像' },
  { value: 'other', label: '其他' },
];

export function bookKindLabel(kind?: string): string {
  return BOOK_KIND_OPTIONS.find((option) => option.value === kind)?.label ?? '其他';
}

/** Story traversal is only meaningful for fiction. */
export function canTraverse(kind?: string): boolean {
  return kind === 'fiction';
}

// WeRead inserts cover/copyright/recommendation pages before the body. They
// are part of the catalogue but not book content, and many are image-only or
// empty. Skip them when choosing where to start reading.
const FRONT_MATTER_MARKERS = ['封面', '版权信息', '文前', '扉页', '书名页', '出版说明', '内容提要'];

export function isFrontMatterChapter(title?: string): boolean {
  const t = (title ?? '').trim();
  if (!t) return false;
  if (t === '目录') return true;
  if (FRONT_MATTER_MARKERS.some((marker) => t.includes(marker))) return true;
  if (t.startsWith('推荐')) {
    const rest = t.slice(2);
    if (rest === '' || rest.startsWith('序') || rest.startsWith('语')) return true;
    return /^[一二三四五六七八九十]/.test(rest);
  }
  return false;
}

export function firstReadableChapter<T extends { title?: string }>(chapters: T[]): T | undefined {
  return chapters.find((chapter) => !isFrontMatterChapter(chapter.title)) ?? chapters[0];
}
