import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties } from 'react';

import { api, startLiveness } from '../api';
import { Icon } from '../icons';
import { navigate, store, useApp } from '../store';
import type { Block, Chapter, ReaderSession, Theme, TypingSettings } from '../types';
import { ReaderCanvas } from '../reader/ReaderCanvas';
import { ChatTranscript } from '../reader/ChatTranscript';
import { ImageLightbox } from '../reader/ImageLightbox';
import { ThemePanel } from '../reader/ThemePanel';
import { measureBlocks } from '../reader/BlockView';
import { caretPage, pageForOffset, readAnchorOffset, resolveColumns, scrollTopForOffset, useGeometry, usePagination, withColumns } from '../reader/layout';
import { themeVars } from '../reader/themeVars';
import { ChatSkin } from '../skins/ChatSkin';
import { isChatSkin } from '../skins/registry';
import { vsCodeSkin } from '../skins/VSCodeSkin';
import { wordSkin } from '../skins/WordSkin';

export interface ReaderChromeProps {
  bookTitle: string;
  bookAuthor: string;
  chapter: Chapter;
  chapters: Chapter[];
  page: number;
  pages: number;
  percent: number;
  mode: 'page' | 'scroll';
  zen: boolean;
  typing: boolean;
  pinned: boolean;
  skin: string;
  floating: boolean;
  autoHideBars: boolean;
  pageTurnAnimation: boolean;
  showStatusBar: boolean;
  idle: boolean;
  themeStyle: CSSProperties;
  onPrev: () => void;
  onNext: () => void;
  onJump: (page: number) => void;
  onSeek: (fraction: number) => void;
  onSelectChapter: (uid: number) => void;
  onToggleToc: () => void;
  onToggleTheme: () => void;
  onToggleZen: () => void;
  onToggleTyping: () => void;
  onTogglePin: () => void;
  onExit: () => void;
  children: React.ReactNode;
  chat?: {
    transcript: React.ReactNode;
    next: string;
    contact: string;
    nickname: string;
    avatar: string;
    sendOnEnter: boolean;
    onSend: () => void;
  };
}

interface ScrollSegment {
  session: ReaderSession;
  blocks: Block[];
  metrics: ReturnType<typeof measureBlocks>;
}

function makeScrollSegment(session: ReaderSession): ScrollSegment {
  const blocks = session.content.blocks ?? [];
  return { session, blocks, metrics: measureBlocks(blocks) };
}

export function ReaderView({
  bookId,
  chapterUid,
  skipRestore = false,
}: {
  bookId: string;
  chapterUid: number;
  skipRestore?: boolean;
}) {
  const app = useApp();
  const settings = app.settings;
  const theme = settings?.theme ?? fallbackTheme();
  const skin = settings?.skin ?? 'none';
  const chatMode = isChatSkin(skin);
  const floating = Boolean(settings?.disguise?.floating) && skin === 'none';
  const typingEnabled = Boolean(settings?.typing.enabled);
  const zen = Boolean(settings?.zen);
  const pinned = Boolean(app.window?.alwaysOnTop);
  const mode: 'page' | 'scroll' = settings?.reader.pageMode === 'scroll' ? 'scroll' : 'page';
  const multiScroll = mode === 'scroll' && !chatMode;
  const multiSegments = multiScroll || chatMode;
  const pageEffect = settings?.reader.pageTurnEffect || 'slide';
  const wheelTurn = settings?.reader.wheelTurn !== false;
  const clickTurn = settings?.reader.clickTurn !== false;
  const showScrollbar = settings?.reader.showScrollbar !== false;
  const smoothScroll = settings?.reader.smoothScroll !== false;
  const pageAnimated = pageEffect !== 'none' && settings?.reader.pageTurnAnimation !== false;
  const [idle, setIdle] = useState(false);
  const [chatScroll, setChatScroll] = useState(0);
  const [scrollPercent, setScrollPercent] = useState(0);
  const [pageFading, setPageFading] = useState(false);

  // Zen mode hides the cursor and the chrome after three seconds without input.
  // Moving the mouse brings the bars back without leaving zen.
  useEffect(() => {
    if (!zen) {
      setIdle(false);
      return;
    }
    let timer = 0;
    const wake = () => {
      setIdle(false);
      window.clearTimeout(timer);
      timer = window.setTimeout(() => setIdle(true), 3000);
    };
    wake();
    window.addEventListener('mousemove', wake);
    window.addEventListener('mousedown', wake);
    window.addEventListener('wheel', wake, { passive: true });
    window.addEventListener('keydown', wake);
    return () => {
      window.clearTimeout(timer);
      window.removeEventListener('mousemove', wake);
      window.removeEventListener('mousedown', wake);
      window.removeEventListener('wheel', wake);
      window.removeEventListener('keydown', wake);
    };
  }, [zen]);

  const [session, setSession] = useState<ReaderSession | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showToc, setShowToc] = useState(false);
  const [showTheme, setShowTheme] = useState(false);
  const [reveal, setReveal] = useState<number | null>(null);
  const [revealByChapter, setRevealByChapter] = useState<Record<number, number>>({});
  const [presets, setPresets] = useState<Theme[]>([]);
  const [scrollSegments, setScrollSegments] = useState<ScrollSegment[]>([]);

  const stageRef = useRef<HTMLDivElement>(null);
  const viewportRef = useRef<HTMLDivElement>(null);
  const trackRef = useRef<HTMLDivElement>(null);
  const caretRef = useRef<HTMLSpanElement>(null);
  const restoredRef = useRef('');
  const chatRestoredRef = useRef('');
  const scrollRestoredRef = useRef('');
  const prefetched = useRef('');
  const prevModeRef = useRef(mode);
  const scrollSegmentsRef = useRef<ScrollSegment[]>([]);
  const scrollGenerationRef = useRef(0);
  const loadingAdjacentRef = useRef(false);
  const pendingPrependRef = useRef<{ height: number; top: number; added: number } | null>(null);
  const userScrolledRef = useRef(false);
  const lastScrollTopRef = useRef(0);

  const showGutter = mode === 'page' && (Boolean(theme.lineNumbers) || skin === 'vscode');
  const baseGeometry = useGeometry(
    stageRef,
    theme,
    showGutter,
    session?.chapter.uid ?? -1,
    skin !== 'none',
    skin === 'vscode' ? { x: 6, y: 8 } : undefined,
    floating,
  );
  const geometry = useMemo(() => {
    if (mode !== 'scroll') return withColumns(baseGeometry, resolveColumns(theme, baseGeometry.pageWidth));
    // A full-width column is hard to read on a desktop window. When the theme
    // leaves the measure automatic, cap the scroll column at a book-like width.
    const width = theme.pageWidth > 0 ? baseGeometry.pageWidth : Math.min(baseGeometry.pageWidth, 920);
    const pad = baseGeometry.pad;
    return {
      ...baseGeometry,
      pageWidth: width,
      paperWidth: width + pad * 2,
      gap: 0,
      colWidth: width,
      perPage: 1,
      step: width,
    };
  }, [baseGeometry, mode, theme.pageWidth, theme.columns]);
  const blocks = useMemo(() => {
    if (multiSegments && scrollSegments.length > 0) {
      return scrollSegments.flatMap((seg, index) => [
        ...(index > 0 ? [{ kind: 'chapter', text: seg.session.chapter.title } as Block] : []),
        ...seg.blocks,
      ]);
    }
    return session?.content.blocks ?? [];
  }, [multiSegments, scrollSegments, session]);
  const metrics = useMemo(() => measureBlocks(blocks), [blocks]);

  const signature = [
    skin,
    mode,
    session?.chapter.uid ?? 0,
    blocks.length,
    theme.fontSize,
    theme.lineHeight,
    theme.letterSpacing,
    theme.columns,
    theme.pageWidth,
    theme.paragraphSpacing,
    typingEnabled,
    geometry.perPage,
  ].join('|');

  const pagination = usePagination(trackRef, geometry, signature);
  const page = mode === 'scroll' ? 0 : pagination.page;
  const pages = mode === 'scroll' ? 1 : pagination.pages;
  const setPage = pagination.setPage;
  const [anchorOffset, setAnchorOffset] = useState<number | null>(null);

  const segmentOffsets = useMemo(() => {
    let acc = 0;
    return scrollSegments.map((seg, index) => {
      const start = acc;
      acc += seg.metrics.total;
      return { ...seg, index, start, end: acc };
    });
  }, [scrollSegments]);

  const segmentAnchor = chatMode
    ? typingEnabled && reveal !== null
      ? reveal
      : Math.round(chatScroll * metrics.total)
    : typingEnabled && reveal !== null
      ? reveal
      : anchorOffset;

  const activeSegment = useMemo(() => {
    if (!multiSegments || segmentOffsets.length === 0) return null;
    const offset = segmentAnchor ?? 0;
    return segmentOffsets.find((seg) => offset < seg.end) ?? segmentOffsets[segmentOffsets.length - 1];
  }, [multiSegments, segmentOffsets, segmentAnchor]);

  const displayChapter = activeSegment?.session.chapter ?? session?.chapter ?? null;
  const activeOffset =
    activeSegment && segmentAnchor !== null
      ? Math.max(0, Math.min(activeSegment.metrics.total, segmentAnchor - activeSegment.start))
      : null;

  // Progress follows the text at the top of the page, not the page index, so a
  // re-layout does not move the percentage even when the page count changes.
  const percent = chatMode
    ? typingEnabled && reveal !== null && metrics.total > 0
      ? (Math.min(reveal, metrics.total) / metrics.total) * 100
      : chatScroll * 100
    : mode === 'scroll'
      ? activeSegment && activeOffset !== null && activeSegment.metrics.total > 0
        ? (activeOffset / activeSegment.metrics.total) * 100
        : scrollPercent
      : pages > 1 && anchorOffset !== null && metrics.total > 0
        ? (anchorOffset / metrics.total) * 100
        : pages > 1
          ? ((page + 1) / pages) * 100
          : 100;

  useEffect(() => {
    if (mode !== 'page' || pageEffect !== 'fade' || !pageAnimated) {
      setPageFading(false);
      return;
    }
    setPageFading(true);
    const timer = window.setTimeout(() => setPageFading(false), 200);
    return () => window.clearTimeout(timer);
  }, [page, mode, pageEffect, pageAnimated]);

  // Only a user page move re-anchors the reading position. Re-layout restores
  // keep the existing anchor, otherwise repeated resizes would creep backwards
  // to the start of whichever page the anchor was restored to.
  const recordAnchorRef = useRef(true);
  const goToPage = useCallback(
    (p: number) => {
      recordAnchorRef.current = true;
      setPage(p);
    },
    [setPage],
  );

  useEffect(() => {
    api
      .presetThemes()
      .then(setPresets)
      .catch(() => undefined);
    return startLiveness();
  }, []);

  // ------------------------------------------------------------ chapter load

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError('');
    restoredRef.current = '';
    chatRestoredRef.current = '';
    scrollRestoredRef.current = '';
    setScrollPercent(0);
    api
      .openChapter(bookId, chapterUid)
      .then((s) => {
        if (cancelled) return;
        setSession(s);
        setReveal((prev) => (typingEnabled ? revealByChapter[s.chapter.uid] ?? 0 : null));
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(String((e as Error).message || e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bookId, chapterUid]);

  useEffect(() => {
    scrollGenerationRef.current += 1;
    loadingAdjacentRef.current = false;
    pendingPrependRef.current = null;
    userScrolledRef.current = false;
    lastScrollTopRef.current = 0;
    if (!session || !multiSegments) {
      scrollSegmentsRef.current = [];
      setScrollSegments([]);
      return;
    }
    const initial = [makeScrollSegment(session)];
    scrollSegmentsRef.current = initial;
    setScrollSegments(initial);
  }, [session, multiSegments]);

  useEffect(() => {
    scrollSegmentsRef.current = scrollSegments;
  }, [scrollSegments]);

  useEffect(() => {
    if (!typingEnabled) setReveal(null);
    else setReveal((prev) => prev ?? revealByChapter[session?.chapter.uid ?? 0] ?? 0);
  }, [typingEnabled, revealByChapter, session?.chapter.uid]);

  // Restore the saved offset once the page count is known.
  useLayoutEffect(() => {
    if (!session || pages <= 1 || mode !== 'page') return;
    if (skipRestore) {
      restoredRef.current = `${session.chapter.uid}`;
      return;
    }
    const key = `${session.chapter.uid}`;
    if (restoredRef.current === key) return;
    restoredRef.current = key;
    const progress = session.progress;
    if (!progress || progress.chapterUid !== session.chapter.uid) return;
    const total = metrics.total || 1;
    const offset = Math.max(0, Math.min(total, progress.offset));
    const target = Math.floor((offset / total) * pages);
    if (target > 0) goToPage(target);
  }, [session, mode, pages, metrics.total, goToPage, skipRestore]);

  // IM skins keep the whole chapter in a vertical transcript, so the saved
  // position is restored as a rune offset instead of a page index.
  useLayoutEffect(() => {
    if (!chatMode || !typingEnabled || !session) return;
    const key = `chat:${session.chapter.uid}`;
    if (chatRestoredRef.current === key) return;
    chatRestoredRef.current = key;
    const progress = session.progress;
    if (progress && progress.chapterUid === session.chapter.uid && progress.offset > 0) {
      setReveal(Math.max(0, Math.min(metrics.total, progress.offset)));
    } else {
      // Open a fresh chat with the first paragraph already sent, otherwise the
      // transcript would be empty until the first keystroke.
      const first = metrics.metrics.find((m) => m.length > 0);
      setReveal(first ? first.start + first.length : 0);
    }
  }, [chatMode, typingEnabled, session, metrics.total]);

  // ------------------------------------------------------------ typing

  const advanceTyping = useCallback(
    (delta: number) => {
      setReveal((prev) => {
        const current = prev ?? 0;
        const next = Math.max(0, Math.min(metrics.total, current + delta));
        if (session) setRevealByChapter((m) => ({ ...m, [session.chapter.uid]: next }));
        return next;
      });
      void api.touchReader().catch(() => undefined);
    },
    [metrics.total, session],
  );

  const skipToNextBlock = useCallback(() => {
    setReveal((prev) => {
      const current = prev ?? 0;
      const next = metrics.metrics.find((m) => m.length > 0 && m.start > current);
      const value = next ? next.start : metrics.total;
      if (session) setRevealByChapter((m) => ({ ...m, [session.chapter.uid]: value }));
      return value;
    });
  }, [metrics, session]);

  // The chat compose bar previews the paragraph that has not been sent yet.
  const chatNext = useMemo(() => {
    if (!chatMode) return '';
    const current = typingEnabled ? reveal ?? 0 : 0;
    for (let i = 0; i < blocks.length; i += 1) {
      const metric = metrics.metrics[i];
      if (metric.length === 0) continue;
      if (current < metric.start + metric.length) {
        const chars = Array.from(blocks[i].text ?? '');
        const shown = Math.max(0, Math.min(chars.length, current - metric.start));
        return chars.slice(shown).join('').slice(0, 260) || '…';
      }
    }
    return '—';
  }, [chatMode, typingEnabled, reveal, blocks, metrics]);

  // Sending in the IM skins reveals the whole current paragraph at once.
  const sendParagraph = useCallback(() => {
    if (!session) return;
    const send = () =>
      setReveal((prev) => {
        const current = prev ?? 0;
        const block = metrics.metrics.find((m) => m.length > 0 && current < m.start + m.length);
        const value = block ? block.start + block.length : metrics.total;
        setRevealByChapter((map) => ({ ...map, [session.chapter.uid]: value }));
        void api.touchReader().catch(() => undefined);
        return value;
      });
    if (!typingEnabled) {
      void api.setTyping(true).then((w) => {
        const current = store.get().settings;
        store.set({ window: w, settings: current ? { ...current, typing: { ...current.typing, enabled: true } } : null });
        send();
      });
      return;
    }
    send();
  }, [typingEnabled, metrics, session]);

  // ____________________________________________________________ navigation

  const chapterIndex = useMemo(() => {
    if (!session) return 0;
    const uid = displayChapter?.uid ?? session.chapter.uid;
    return session.chapters.findIndex((c) => c.uid === uid);
  }, [session, displayChapter]);

  const goChapter = useCallback(
    (delta: number) => {
      if (!session) return;
      const next = session.chapters[chapterIndex + delta];
      if (!next) return;
      void api.reportNow(bookId).catch(() => undefined);
      navigate({ name: 'reader', bookId, chapterUid: next.uid });
    },
    [session, chapterIndex, bookId],
  );

  const loadAdjacent = useCallback(
    async (delta: number): Promise<boolean> => {
      if (!multiSegments || !session || loadingAdjacentRef.current) return false;
      const loaded = scrollSegmentsRef.current;
      if (loaded.length === 0) return false;
      const firstUid = loaded[0].session.chapter.uid;
      const lastUid = loaded[loaded.length - 1].session.chapter.uid;
      const firstIndex = session.chapters.findIndex((c) => c.uid === firstUid);
      const lastIndex = session.chapters.findIndex((c) => c.uid === lastUid);
      const targetIndex = delta > 0 ? lastIndex + 1 : firstIndex - 1;
      const target = session.chapters[targetIndex];
      if (!target || loaded.some((seg) => seg.session.chapter.uid === target.uid)) return false;

      const generation = scrollGenerationRef.current;
      loadingAdjacentRef.current = true;
      try {
        const next = await api.openChapter(bookId, target.uid);
        if (generation !== scrollGenerationRef.current) return false;
        const seg = makeScrollSegment(next);
        if (delta > 0) {
          const nextList = [...scrollSegmentsRef.current, seg];
          scrollSegmentsRef.current = nextList;
          setScrollSegments(nextList);
        } else {
          const viewport = viewportRef.current;
          pendingPrependRef.current = {
            height: viewport?.scrollHeight ?? 0,
            top: viewport?.scrollTop ?? 0,
            added: seg.metrics.total,
          };
          const nextList = [seg, ...scrollSegmentsRef.current];
          scrollSegmentsRef.current = nextList;
          setScrollSegments(nextList);
        }
        return true;
      } catch {
        // Adjacent chapter loading is best-effort; keep the current content.
        return false;
      } finally {
        if (generation === scrollGenerationRef.current) loadingAdjacentRef.current = false;
      }
    },
    [bookId, multiSegments, session],
  );

  const turn = useCallback(
    (delta: number) => {
      if (chatMode) return;
      if (mode === 'scroll') {
        const viewport = viewportRef.current;
        if (!viewport) return;
        const max = viewport.scrollHeight - viewport.clientHeight;
        const step = Math.max(80, Math.round(viewport.clientHeight * 0.85));
        if (delta > 0 && viewport.scrollTop >= max - 2) {
          if (multiScroll) void loadAdjacent(1);
          else goChapter(1);
          return;
        }
        if (delta < 0 && viewport.scrollTop <= 2) {
          if (multiScroll) void loadAdjacent(-1);
          else goChapter(-1);
          return;
        }
        viewport.scrollBy({ top: delta * step, behavior: smoothScroll ? 'smooth' : 'auto' });
        void api.touchReader().catch(() => undefined);
        return;
      }
      if (typingEnabled && delta > 0) {
        const caret = caretPage(caretRef.current, trackRef.current, geometry);
        if (caret !== null && page >= caret) {
          // Typing cannot run ahead of the caret: reveal the next character.
          advanceTyping(1);
          return;
        }
      }
      if (delta > 0 && page >= pages - 1) {
        goChapter(1);
        return;
      }
      if (delta < 0 && page <= 0) {
        goChapter(-1);
        return;
      }
      goToPage(page + delta);
      void api.touchReader().catch(() => undefined);
    },
    [chatMode, mode, smoothScroll, typingEnabled, advanceTyping, geometry, page, pages, goToPage, goChapter, multiScroll, loadAdjacent],
  );

  // Keep the caret's column on screen while typing.
  useLayoutEffect(() => {
    if (mode !== 'page' || !typingEnabled || reveal === null) return;
    const target = caretPage(caretRef.current, trackRef.current, geometry);
    if (target !== null && target !== page) goToPage(target);
  }, [mode, reveal, typingEnabled, geometry, page, goToPage]);

  // In scroll mode the caret is already in the vertical flow; nudge the
  // container only when it would otherwise leave the visible slice.
  useLayoutEffect(() => {
    if (mode !== 'scroll' || !typingEnabled || reveal === null) return;
    const caret = caretRef.current;
    const viewport = viewportRef.current;
    if (!caret || !viewport) return;
    const c = caret.getBoundingClientRect();
    const v = viewport.getBoundingClientRect();
    if (c.top < v.top + 24 || c.bottom > v.bottom - 24) {
      viewport.scrollTop += c.top - v.top - v.height * 0.35;
    }
  }, [mode, typingEnabled, reveal]);

  // Preserve the reading position across re-layouts. The anchor is the rune
  // offset at the top of the page; after a window resize or a font, margin or
  // column change the page holding that offset becomes the current page, so the
  // text on screen does not jump even though the page number may change.
  const layoutKey = [
    skin,
    mode,
    theme.fontFamily,
    theme.fallbackFonts,
    theme.fontSize,
    theme.lineHeight,
    theme.letterSpacing,
    theme.paragraphSpacing,
    theme.textIndent,
    theme.textAlign,
    theme.columns,
    theme.pageWidth,
    theme.marginX,
    theme.marginY,
    showGutter ? 'gutter' : 'plain',
    geometry.pageWidth,
    geometry.height,
    geometry.pad,
    geometry.gap,
    geometry.colWidth,
    blocks.length,
  ].join('|');
  const anchorRef = useRef<number | null>(null);
  const layoutKeyRef = useRef<string | null>(null);
  const anchorChapterRef = useRef<number | null>(null);
  const anchorTimer = useRef<number | null>(null);

  // Keep the reading position stable when a previous chapter is prepended:
  // the browser preserves scrollTop, so add the newly inserted height.
  useLayoutEffect(() => {
    const pending = pendingPrependRef.current;
    if (!pending) return;
    pendingPrependRef.current = null;
    // Prepending shifts every global rune offset. Keep the caret on the same
    // text when typing is active.
    if (typingEnabled && pending.added > 0) {
      setReveal((prev) => (prev == null ? prev : prev + pending.added));
    }
    const viewport = viewportRef.current;
    if (!viewport) return;
    const delta = viewport.scrollHeight - pending.height;
    if (delta !== 0) viewport.scrollTop = pending.top + delta;
    const offset = readAnchorOffset(viewport, trackRef.current);
    if (offset !== null) {
      anchorRef.current = offset;
      setAnchorOffset(offset);
      recordAnchorRef.current = false;
    }
  }, [scrollSegments, typingEnabled]);

  const scrollToOffset = useCallback(
    (offset: number, smooth = smoothScroll) => {
      const viewport = viewportRef.current;
      const track = trackRef.current;
      if (!viewport || !track) return;
      const top = scrollTopForOffset(viewport, track, offset);
      if (top == null) return;
      viewport.scrollTo({ top, behavior: smooth ? 'smooth' : 'auto' });
    },
    [smoothScroll],
  );

  const handleScroll = useCallback(() => {
    const viewport = viewportRef.current;
    if (!viewport) return;
    const max = viewport.scrollHeight - viewport.clientHeight;
    const top = viewport.scrollTop;
    const goingUp = top < lastScrollTopRef.current;
    lastScrollTopRef.current = top;
    setScrollPercent(max > 0 ? Math.min(100, Math.max(0, (top / max) * 100)) : 0);
    const offset = readAnchorOffset(viewport, trackRef.current);
    if (offset !== null) {
      anchorRef.current = offset;
      setAnchorOffset(offset);
      recordAnchorRef.current = false;
    }
    if (multiScroll && userScrolledRef.current && viewport.scrollHeight > viewport.clientHeight) {
      if (!goingUp && top + viewport.clientHeight >= viewport.scrollHeight - 480) {
        void loadAdjacent(1);
      }
      if (goingUp && top <= 480) {
        void loadAdjacent(-1);
      }
    }
  }, [multiScroll, loadAdjacent]);

  useEffect(() => {
    if (mode !== 'scroll') return;
    const viewport = viewportRef.current;
    if (!viewport) return;
    let raf = 0;
    const onScroll = () => {
      if (raf) return;
      raf = requestAnimationFrame(() => {
        raf = 0;
        handleScroll();
      });
    };
    const markUser = () => {
      userScrolledRef.current = true;
    };
    const onWheel = (event: WheelEvent) => {
      userScrolledRef.current = true;
      if (!multiScroll) return;
      const max = viewport.scrollHeight - viewport.clientHeight;
      if (event.deltaY > 0 && (max <= 0 || viewport.scrollTop >= max - 2)) {
        void loadAdjacent(1);
      }
      if (event.deltaY < 0 && (max <= 0 || viewport.scrollTop <= 2)) {
        void loadAdjacent(-1);
      }
    };
    viewport.addEventListener('scroll', onScroll, { passive: true });
    viewport.addEventListener('wheel', onWheel, { passive: true });
    viewport.addEventListener('pointerdown', markUser, { passive: true });
    viewport.addEventListener('touchmove', markUser, { passive: true });
    onScroll();
    return () => {
      viewport.removeEventListener('scroll', onScroll);
      viewport.removeEventListener('wheel', onWheel);
      viewport.removeEventListener('pointerdown', markUser);
      viewport.removeEventListener('touchmove', markUser);
      if (raf) cancelAnimationFrame(raf);
    };
  }, [mode, multiScroll, session, handleScroll, loadAdjacent]);

  useLayoutEffect(() => {
    if (!session || mode !== 'scroll') return;
    if (skipRestore) {
      scrollRestoredRef.current = `${session.chapter.uid}`;
      return;
    }
    const key = `${session.chapter.uid}`;
    if (scrollRestoredRef.current === key) return;
    scrollRestoredRef.current = key;
    const progress = session.progress;
    if (!progress || progress.chapterUid !== session.chapter.uid || progress.offset <= 0) return;
    const offset = Math.max(0, Math.min(metrics.total, progress.offset));
    const timer = window.setTimeout(() => scrollToOffset(offset, false), 80);
    return () => window.clearTimeout(timer);
  }, [session, mode, metrics.total, scrollToOffset, skipRestore]);

  useLayoutEffect(() => {
    if (prevModeRef.current === mode) return;
    prevModeRef.current = mode;
    const offset = anchorRef.current ?? anchorOffset;
    if (offset == null) return;
    if (mode === 'scroll') {
      const id = requestAnimationFrame(() => scrollToOffset(offset, false));
      return () => cancelAnimationFrame(id);
    }
    const target = pageForOffset(trackRef.current, geometry.step, offset);
    if (target != null) goToPage(target);
  }, [mode, anchorOffset, geometry.step, goToPage, scrollToOffset]);

  useEffect(
    () => () => {
      if (anchorTimer.current != null) window.clearTimeout(anchorTimer.current);
    },
    [],
  );

  useLayoutEffect(() => {
    if (!session) return;
    if (multiScroll) {
      layoutKeyRef.current = layoutKey;
      return;
    }
    const chapter = session.chapter.uid;
    if (anchorChapterRef.current !== chapter) {
      anchorChapterRef.current = chapter;
      anchorRef.current = null;
      setAnchorOffset(null);
      layoutKeyRef.current = layoutKey;
      recordAnchorRef.current = true;
    } else if (layoutKeyRef.current !== layoutKey) {
      layoutKeyRef.current = layoutKey;
      const anchor = anchorRef.current;
      if (anchor !== null) {
        if (mode === 'scroll') {
          const id = requestAnimationFrame(() => scrollToOffset(anchor, false));
          return () => cancelAnimationFrame(id);
        }
        const target = pageForOffset(trackRef.current, geometry.step, anchor);
        if (target !== null && target !== page) {
          setPage(target);
          return;
        }
      }
    }
    if (mode === 'scroll') return;
    // The page slide is animated, so the top-left text can only be sampled once
    // the transform has settled; sampling mid-flight records the outgoing page
    // and the anchor walks backwards on every further re-layout.
    if (anchorTimer.current != null) window.clearTimeout(anchorTimer.current);
    anchorTimer.current = window.setTimeout(() => {
      anchorTimer.current = null;
      if (!recordAnchorRef.current) return;
      const offset = readAnchorOffset(viewportRef.current, trackRef.current);
      if (offset === null) return;
      anchorRef.current = offset;
      setAnchorOffset(offset);
      recordAnchorRef.current = false;
    }, 260);
  }, [layoutKey, session, mode, page, pages, geometry.step, setPage, scrollToOffset, multiScroll]);

  // ------------------------------------------------------------ persistence

  const saveBlocks = activeSegment?.blocks ?? blocks;
  const flatText = useMemo(
    () => saveBlocks.map((b) => (b.kind === 'image' || b.kind === 'divider' || b.kind === 'chapter' ? '' : b.text ?? '')).join(''),
    [saveBlocks],
  );

  useEffect(() => {
    if (!session || !settings?.reader.reportProgress) return;
    const saveChapter = activeSegment?.session.chapter ?? session.chapter;
    const total = (activeSegment?.metrics.total ?? metrics.total) || 1;
    const localOffset = activeSegment && activeOffset !== null ? activeOffset : null;
    const offset = activeSegment
      ? localOffset ?? 0
      : reveal !== null
        ? Math.max(0, Math.min(total, reveal))
        : Math.round((percent / 100) * total);
    const chapterPercent = (offset / total) * 100;
    const summary = Array.from(flatText).slice(offset, offset + 20).join('');
    const timer = window.setTimeout(() => {
      void api
        .savePosition({
          bookId,
          chapterUid: saveChapter.uid,
          chapterIndex: saveChapter.index,
          offset,
          chapterPercent,
          summary,
        })
        .catch(() => undefined);
    }, 900);
    return () => window.clearTimeout(timer);
  }, [bookId, session, activeSegment, activeOffset, percent, reveal, metrics.total, flatText, settings?.reader.reportProgress]);

  // Typewriter mode should continue into the next chapter instead of stopping
  // at the end of the loaded segment.
  useEffect(() => {
    if (!multiSegments || !typingEnabled || reveal === null || !session) return;
    const last = scrollSegments[scrollSegments.length - 1];
    if (!last) return;
    const lastIndex = session.chapters.findIndex((c) => c.uid === last.session.chapter.uid);
    if (lastIndex < 0 || lastIndex >= session.chapters.length - 1) return;
    if (metrics.total > 0 && reveal >= metrics.total - 120) {
      void loadAdjacent(1);
    }
  }, [multiSegments, typingEnabled, reveal, metrics.total, scrollSegments, session, loadAdjacent]);

  useEffect(() => {
    if (!session || !settings?.reader.prefetchNext) return;
    const next = session.chapters[chapterIndex + 1];
    if (!next) return;
    const nearEnd = reveal !== null
      ? reveal >= metrics.total - 200
      : mode === 'scroll'
        ? scrollPercent >= 88
        : page >= pages - 2;
    if (!nearEnd) return;
    const key = `${bookId}:${next.uid}`;
    if (prefetched.current === key) return;
    prefetched.current = key;
    void api.openChapter(bookId, next.uid).catch(() => undefined);
  }, [session, chapterIndex, mode, page, pages, reveal, metrics.total, scrollPercent, bookId, settings?.reader.prefetchNext]);

  useEffect(() => {
    return () => {
      void api.reportNow(bookId).catch(() => undefined);
    };
  }, [bookId]);

  // ------------------------------------------------------------ keyboard

  useEffect(() => {
    if (!session) return;
    const handler = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) return;
      // IME composition fires keydown for every keystroke before the character
      // is committed; advancing on those would type pinyin letters instead of
      // the Chinese text.
      if (e.isComposing || e.keyCode === 229) return;

      if (e.ctrlKey && !e.altKey && !e.metaKey) {
        if (e.key === 'ArrowLeft') {
          e.preventDefault();
          goChapter(-1);
        } else if (e.key === 'ArrowRight') {
          e.preventDefault();
          goChapter(1);
        }
        return;
      }
      if (e.altKey || e.metaKey) return;

      if (typingEnabled) {
        if (e.key.length === 1) {
          e.preventDefault();
          advanceTyping(1);
          return;
        }
        if (e.key === 'Backspace') {
          e.preventDefault();
          advanceTyping(-1);
          return;
        }
        if (e.key === 'Enter' && settings?.typing.skipOnEnter) {
          e.preventDefault();
          if (chatMode) sendParagraph();
          else skipToNextBlock();
          return;
        }
      }

      switch (e.key) {
        case 'ArrowLeft':
          e.preventDefault();
          if (mode === 'scroll') goChapter(-1);
          else turn(-1);
          break;
        case 'ArrowRight':
          e.preventDefault();
          if (mode === 'scroll') goChapter(1);
          else turn(1);
          break;
        case 'ArrowUp':
        case 'PageUp':
          e.preventDefault();
          turn(-1);
          break;
        case 'ArrowDown':
        case 'PageDown':
          e.preventDefault();
          turn(1);
          break;
        case ' ':
          e.preventDefault();
          turn(e.shiftKey ? -1 : 1);
          break;
        case 'Home':
          e.preventDefault();
          if (mode === 'scroll') viewportRef.current?.scrollTo({ top: 0, behavior: 'auto' });
          else goToPage(0);
          break;
        case 'End':
          e.preventDefault();
          if (mode === 'scroll') {
            const el = viewportRef.current;
            el?.scrollTo({ top: el.scrollHeight, behavior: 'auto' });
          } else {
            goToPage(pages - 1);
          }
          break;
        case 'Escape':
          if (showTheme) setShowTheme(false);
          else if (showToc) setShowToc(false);
          else if (zen) void api.setZen(false).then((w) => store.set({ window: w }));
          break;
        default:
          break;
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [
    session, typingEnabled, settings?.typing.skipOnEnter, turn, goChapter, advanceTyping,
    skipToNextBlock, chatMode, sendParagraph, goToPage, pages, mode, zen, showTheme, showToc,
  ]);

  // Page-mode wheel handling. A classic wheel notch turns immediately; a
  // precision touchpad emits a stream of small pixel deltas plus inertia, so
  // deltas are accumulated and only one page is turned per gesture. Ctrl+wheel
  // is pinch-to-zoom and is left to WebView2.
  useEffect(() => {
    if (mode !== 'page' || !wheelTurn) return;
    const el = viewportRef.current;
    if (!el) return;
    let acc = 0;
    let lastEvent = 0;
    let lastTurn = 0;
    let locked = false;
    const onWheel = (e: WheelEvent) => {
      if (e.ctrlKey) return;
      const now = performance.now();
      const dx = e.deltaX;
      const dy = e.deltaY;
      if (Math.abs(dy) < 1 && Math.abs(dx) < 1) return;
      // Horizontal two-finger pans are swallowed so the webview cannot scroll
      // sideways or trigger back/forward navigation.
      if (Math.abs(dx) > Math.abs(dy) * 1.2) {
        e.preventDefault();
        return;
      }
      e.preventDefault();
      // A gap in the event stream marks the end of a touchpad gesture. Inertia
      // events keep lastEvent fresh, so the locked gesture cannot page twice.
      if (now - lastEvent > 160) {
        acc = 0;
        locked = false;
      }
      lastEvent = now;
      const notched = e.deltaMode !== 0 || Math.abs(dy) >= 40;
      if (notched) {
        if (now - lastTurn < 320) return;
        lastTurn = now;
        acc = 0;
        locked = true;
        turn(dy > 0 ? 1 : -1);
        return;
      }
      if (locked) return;
      acc += dy;
      if (Math.abs(acc) < 48) return;
      if (now - lastTurn < 320) return;
      lastTurn = now;
      const dir = acc > 0 ? 1 : -1;
      acc = 0;
      locked = true;
      turn(dir);
    };
    el.addEventListener('wheel', onWheel, { passive: false });
    return () => el.removeEventListener('wheel', onWheel);
  }, [mode, wheelTurn, turn]);

  // ------------------------------------------------------------ theme editing

  const themeTimer = useRef<number | null>(null);
  const pendingTheme = useRef<Theme | null>(null);

  const flushTheme = useCallback(() => {
    const next = pendingTheme.current;
    pendingTheme.current = null;
    if (!next) return;
    const current = store.get().settings;
    if (!current) return;
    void api
      .saveSettings({ ...current, theme: next })
      .then((s) => store.set({ settings: s }))
      .catch(() => undefined);
  }, []);

  useEffect(
    () => () => {
      if (themeTimer.current != null) {
        window.clearTimeout(themeTimer.current);
        themeTimer.current = null;
        flushTheme();
      }
    },
    [flushTheme],
  );

  const updateTheme = useCallback(
    (patch: Partial<Theme>) => {
      const current = store.get().settings;
      if (!current) return;
      const theme = { ...current.theme, ...patch, name: current.theme.name || '自定义' };
      // Local-first: sliders repaint immediately and the disk write is debounced
      // so dragging the size slider does not issue one full settings save per px.
      store.set({ settings: { ...current, theme } });
      pendingTheme.current = theme;
      if (themeTimer.current != null) window.clearTimeout(themeTimer.current);
      themeTimer.current = window.setTimeout(() => {
        themeTimer.current = null;
        flushTheme();
      }, 300);
    },
    [flushTheme],
  );

  const applyPreset = useCallback((t: Theme) => {
    const current = store.get().settings;
    if (!current) return;
    if (themeTimer.current != null) {
      window.clearTimeout(themeTimer.current);
      themeTimer.current = null;
    }
    pendingTheme.current = null;
    store.set({ settings: { ...current, theme: { ...t } } });
    void api.saveSettings({ ...current, theme: { ...t } }).then((s) => store.set({ settings: s }));
  }, []);

  const toggleZen = useCallback(() => {
    void api.setZen(!zen).then((w) => {
      const s = store.get().settings;
      store.set({ window: w, settings: s ? { ...s, zen: w.zen } : null });
    });
  }, [zen]);

  const toggleTyping = useCallback(() => {
    void api.setTyping(!typingEnabled).then((w) => {
      const s = store.get().settings;
      store.set({ window: w, settings: s ? { ...s, typing: { ...s.typing, enabled: w.typing } } : null });
      if (w.typing) setReveal((prev) => prev ?? revealByChapter[session?.chapter.uid ?? 0] ?? 0);
      else setReveal(null);
    });
  }, [typingEnabled, revealByChapter, session?.chapter.uid]);

  // IM mode is the only place where typing is also a scroll boundary. A real
  // scroll should stop the typewriter and reveal the full chapter, which also
  // clears the compose preview in ChatSkin.
  const cancelTyping = useCallback(() => {
    if (!typingEnabled) return;
    void api
      .setTyping(false)
      .then((w) => {
        const s = store.get().settings;
        store.set({ window: w, settings: s ? { ...s, typing: { ...s.typing, enabled: false } } : null });
        setReveal(null);
      })
      .catch(() => undefined);
  }, [typingEnabled]);

  const togglePin = useCallback(() => {
    void api.setAlwaysOnTop(!pinned).then((w) => store.set({ window: w }));
  }, [pinned]);

  const seek = useCallback(
    (fraction: number) => {
      const f = Math.max(0, Math.min(1, fraction));
      if (mode === 'scroll') {
        const start = activeSegment?.start ?? 0;
        const total = activeSegment?.metrics.total ?? metrics.total;
        scrollToOffset(start + Math.round(f * total), smoothScroll);
        return;
      }
      if (f >= 1) {
        goToPage(pages - 1);
        return;
      }
      const target = pageForOffset(trackRef.current, geometry.step, Math.round(f * metrics.total));
      goToPage(target ?? 0);
    },
    [mode, activeSegment, metrics.total, scrollToOffset, smoothScroll, pages, geometry.step, goToPage],
  );

  if (loading && !session) {
    return (
      <div className="reader">
        <div className="empty">
          <div className="spinner" />
          <span>正在打开章节…</span>
        </div>
      </div>
    );
  }

  if (error && !session) {
    return (
      <div className="reader">
        <div className="reader-bar top">
          <button className="icon-btn" onClick={() => navigate({ name: 'detail', bookId })} title="返回书籍">
            <Icon name="left" />
          </button>
          <span className="chapter-title">打开章节失败</span>
        </div>
        <div className="empty">
          <p style={{ maxWidth: 460 }}>{error}</p>
          <div className="row">
            <button className="btn btn-primary" onClick={() => navigate({ name: 'reader', bookId, chapterUid })}>
              重试
            </button>
            <button className="btn btn-ghost" onClick={() => navigate({ name: 'detail', bookId })}>
              返回详情
            </button>
          </div>
        </div>
      </div>
    );
  }

  if (!session) return null;

  const themeStyle = themeVars(theme, {
    pageWidth: geometry.pageWidth,
    paperWidth: geometry.paperWidth,
    pad: geometry.pad,
    gap: geometry.gap,
    colWidth: geometry.colWidth,
    columns: geometry.perPage,
    // Keep inline images inside the visible column so a short window never
    // clips the bottom half of an illustration.
    imgMax: Math.max(120, Math.round((geometry.height - geometry.pad * 2) * 0.9)),
    marginX: theme.marginX,
    marginY: theme.marginY,
    paper: skin === 'none' ? 'themed' : 'transparent',
  });

  const stagePadding = floating
    ? '0px'
    : skin === 'vscode'
      ? '8px 6px 0 0'
      : skin === 'word'
        ? '0px'
        : `${theme.marginY}px ${theme.marginX}px`;

  const lastLoadedChapterIndex =
    scrollSegments.length > 0
      ? session.chapters.findIndex((c) => c.uid === scrollSegments[scrollSegments.length - 1].session.chapter.uid)
      : -1;

  const transcript = chatMode ? (
    <ChatTranscript
      blocks={blocks}
      metrics={metrics.metrics}
      reveal={typingEnabled ? reveal ?? 0 : null}
      caretRef={caretRef}
      cursorShape={settings?.typing.cursorShape ?? 'bar'}
      faintUnread={Boolean(settings?.typing.faintUnread)}
      nickname={settings?.disguise?.nickname || '我'}
      avatar={settings?.disguise?.avatar || ''}
      onScrollProgress={(fraction) => setChatScroll(fraction)}
      onCancelTyping={cancelTyping}
      hasNextChapter={lastLoadedChapterIndex >= 0 && lastLoadedChapterIndex < session.chapters.length - 1}
      onNextChapter={() => { void loadAdjacent(1); }}
      onLoadNext={() => { void loadAdjacent(1); }}
      onLoadPrevious={() => loadAdjacent(-1)}
    />
  ) : null;

  const canvas = chatMode ? (
    transcript
  ) : (
    <ReaderCanvas
      blocks={blocks}
      theme={theme}
      geometry={geometry}
      page={page}
      mode={mode}
      showScrollbar={showScrollbar}
      clickTurn={clickTurn}
      animated={pageAnimated}
      fading={pageFading}
      gutter={showGutter}
      typing={(settings?.typing ?? emptyTyping()) as TypingSettings}
      reveal={typingEnabled ? reveal ?? 0 : null}
      caretRef={caretRef}
      stageRef={stageRef}
      viewportRef={viewportRef}
      trackRef={trackRef}
      onPrev={() => turn(-1)}
      onNext={() => turn(1)}
      transparentPaper={skin !== 'none'}
      forceNoGutter={skin === 'word'}
      stagePadding={stagePadding}
    />
  );

  const chromeProps: ReaderChromeProps = {
    bookTitle: session.book.title,
    bookAuthor: session.book.author,
    chapter: displayChapter ?? session.chapter,
    chapters: session.chapters,
    page,
    pages,
    percent,
    mode,
    zen,
    typing: typingEnabled,
    pinned,
    skin,
    floating,
    autoHideBars: Boolean(settings?.reader.autoHideBars),
    pageTurnAnimation: pageAnimated,
    showStatusBar: settings?.reader.showStatusBar !== false,
    idle,
    chat: chatMode
      ? {
          transcript,
          next: chatNext,
          contact: settings?.disguise?.contactName || '文件传输助手',
          nickname: settings?.disguise?.nickname || '我',
          avatar: settings?.disguise?.avatar || '',
          sendOnEnter: settings?.disguise?.sendOnEnter !== false,
          onSend: sendParagraph,
        }
      : undefined,
    themeStyle,
    onPrev: () => turn(-1),
    onNext: () => turn(1),
    onJump: (p) => goToPage(p),
    onSeek: seek,
    onSelectChapter: (uid) => {
      setShowToc(false);
      void api.reportNow(bookId).catch(() => undefined);
      navigate({ name: 'reader', bookId, chapterUid: uid });
    },
    onToggleToc: () => setShowToc((v) => !v),
    onToggleTheme: () => setShowTheme((v) => !v),
    onToggleZen: toggleZen,
    onToggleTyping: toggleTyping,
    onTogglePin: togglePin,
    onExit: () => navigate({ name: 'detail', bookId }),
    children: canvas,
  };

  const body = chatMode ? (
    <ChatSkin brand={skin as 'wechat' | 'dingtalk' | 'feishu' | 'wecom'} {...chromeProps} />
  ) : skin === 'word' ? (
    wordSkin(chromeProps)
  ) : skin === 'vscode' ? (
    vsCodeSkin(chromeProps)
  ) : (
    plainReader(chromeProps)
  );

  return (
    <div style={{ height: '100%', position: 'relative', ...themeStyle }}>
      {body}
      {showToc ? (
        <div className="toc-panel">
          <header>
            <Icon name="list" size={16} />
            <span className="grow truncate">目录 · {session.chapters.length} 章</span>
            <button className="icon-btn" onClick={() => setShowToc(false)} aria-label="关闭目录">
              ×
            </button>
          </header>
          <div className="toc-scroll">
            {session.chapters.map((c) => (
              <button
                key={c.uid}
                className={`toc-item level-${Math.min(3, c.level || 1)}${c.uid === (displayChapter?.uid ?? session.chapter.uid) ? ' current' : ''}`}
                onClick={() => chromeProps.onSelectChapter(c.uid)}
              >
                <span className="idx">{c.index}</span>
                <span className="truncate">{c.title || '未命名章节'}</span>
              </button>
            ))}
          </div>
        </div>
      ) : null}
      {showTheme ? (
        <ThemePanel
          theme={theme}
          presets={presets}
          onChange={updateTheme}
          onPreset={applyPreset}
          onClose={() => setShowTheme(false)}
        />
      ) : null}
      {typingEnabled ? (
        <div className="typing-badge">
          打字模式 · {reveal ?? 0}/{metrics.total}
        </div>
      ) : null}
      <ImageLightbox />
    </div>
  );
}

// ---------------------------------------------------------------- plain chrome

function plainReader(p: ReaderChromeProps) {
  return (
    <div
      className={`reader${p.zen ? ' zen' : ''}${p.typing ? ' typing' : ''}${p.idle ? ' idle' : ''}${
        p.zen && !p.idle ? ' show-chrome' : ''
      }${p.autoHideBars ? ' auto-hide' : ''}${p.pageTurnAnimation ? '' : ' no-anim'}${p.floating ? ' floating' : ''}${
        p.mode === 'scroll' ? ' scroll-mode' : ''
      }`}
      style={p.themeStyle}
      onContextMenu={
        p.floating
          ? (e) => {
              e.preventDefault();
              p.onNext();
            }
          : undefined
      }
    >
      <div className="reader-bar top">
        <button className="icon-btn" onClick={p.onExit} title="返回书籍">
          <Icon name="left" />
        </button>
        <span className="chapter-title">{p.chapter.title || p.bookTitle}</span>
        <span className="spacer" />
        <button className="icon-btn" onClick={p.onToggleToc} title="目录">
          <Icon name="list" />
        </button>
        <button className="icon-btn" onClick={p.onToggleTheme} title="外观">
          <Icon name="palette" />
        </button>
        <button className={`icon-btn${p.typing ? ' active' : ''}`} onClick={p.onToggleTyping} title="打字模式 (Ctrl+Alt+D)">
          <Icon name="keyboard" />
        </button>
        <button className={`icon-btn${p.pinned ? ' active' : ''}`} onClick={p.onTogglePin} title="置顶 (Ctrl+Alt+T)">
          <Icon name={p.pinned ? 'pin' : 'unpin'} />
        </button>
        <button className="icon-btn" onClick={p.onToggleZen} title="禅模式 (Ctrl+Alt+Z)">
          <Icon name={p.zen ? 'exitZen' : 'zen'} />
        </button>
      </div>

      {p.children}

      {p.showStatusBar ? (
        <div className="reader-bar bottom">
          <button className="icon-btn" onClick={p.onPrev} title="上一页">
            <Icon name="left" />
          </button>
          <div className="reader-progress">
            <input
              type="range"
              min={0}
              max={p.mode === 'scroll' ? 1000 : Math.max(1, p.pages - 1)}
              value={p.mode === 'scroll' ? Math.round(p.percent * 10) : p.page}
              onChange={(e) =>
                p.mode === 'scroll'
                  ? p.onSeek(Number(e.target.value) / 1000)
                  : p.onJump(Number(e.target.value))
              }
            />
            <span className="pct">{p.percent.toFixed(1)}%</span>
          </div>
          {p.mode === 'page' ? (
            <span className="page-jump">
              {p.page + 1} / {p.pages}
            </span>
          ) : null}
          <button className="icon-btn" onClick={p.onNext} title="下一页">
            <Icon name="right" />
          </button>
        </div>
      ) : null}
    </div>
  );
}

function emptyTyping(): TypingSettings {
  return {
    enabled: false,
    hideUnread: true,
    faintUnread: false,
    sound: false,
    cursorShape: 'bar',
    skipOnEnter: true,
    advanceOnAny: true,
  };
}

function fallbackTheme(): Theme {
  return {
    name: '纸白',
    fontFamily: 'Source Han Serif SC',
    fallbackFonts: 'Noto Serif SC, Songti SC, SimSun, serif',
    fontSize: 19,
    lineHeight: 1.85,
    letterSpacing: 0.02,
    paragraphSpacing: 12,
    textIndent: 2,
    textColor: '#2b2b2b',
    background: '#f5f2ea',
    surface: '#fffdf8',
    surfaceOpacity: 1,
    textAlign: 'justify',
    pageWidth: 0,
    marginX: 12,
    marginY: 12,
    lineNumbers: false,
    columns: 0,
    headingColor: '#1d1d1d',
    accentColor: '#2f6fed',
    selectionColor: 'rgba(47,111,237,0.22)',
    paragraphStyle: 'indent',
  };
}
