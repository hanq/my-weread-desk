import type { SyntheticEvent } from 'react';

import type { Book } from './types';

/** Shared default cover for WeChat official-account books without artwork. */
export const MP_DEFAULT_COVER = '/mp-cover.svg';

/** Build an authenticated proxy URL for an externally hosted article image. */
export function proxiedImageUrl(bookId: string, src: string): string {
  if (!bookId || !src) return src;
  if (src.startsWith('/media/') || src.startsWith('/media-proxy?') || src.startsWith('data:')) return src;
  const normalized = src.startsWith('//') ? `https:${src}` : src;
  if (!/^https?:/i.test(normalized)) return src;
  const params = new URLSearchParams({ book: bookId, uid: '0', src: normalized });
  return `/media-proxy?${params.toString()}`;
}

/** The URL to render for a shelf/search/detail cover, including MP fallback. */
export function coverUrl(book: Pick<Book, 'cover' | 'isMp'>): string {
  return book.cover || (book.isMp ? MP_DEFAULT_COVER : '');
}

/**
 * If an MP cover URL is missing or fails (mmbiz URLs often reject no-referrer
 * loads), swap in the bundled default once. The data flag prevents a loop if
 * the default asset itself ever fails.
 */
export function handleCoverError(
  book: Pick<Book, 'isMp'>,
  event: SyntheticEvent<HTMLImageElement>,
): void {
  if (!book.isMp) return;
  const img = event.currentTarget;
  if (img.dataset.mpFallback === '1') return;
  img.dataset.mpFallback = '1';
  img.src = MP_DEFAULT_COVER;
}
