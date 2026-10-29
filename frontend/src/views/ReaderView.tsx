import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties } from 'react';

import { api, startLiveness } from '../api';
import { Icon } from '../icons';
import { navigate, store, useApp } from '../store';
import type { Chapter, ReaderSession, Theme, TypingSettings } from '../types';
import { ReaderCanvas } from '../reader/ReaderCanvas';
import { ThemePanel } from '../reader/ThemePanel';
import { measureBlocks } from '../reader/BlockView';
import { caretPage, useGeometry, usePagination } from '../reader/layout';
import { themeVars } from '../reader/themeVars';
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
  zen: boolean;
  typing: boolean;
  pinned: boolean;
  skin: string;
  themeStyle: CSSProperties;
  onPrev: () => void;
  onNext: () => void;
  onJump: (page: number) => void;
  onSelectChapter: (uid: number) => void;
  onToggleToc: () => void;
  onToggleTheme: () => void;
  onToggleZen: () => void;
  onToggleTyping: () => void;
  onTogglePin: () => void;
  onExit: () => void;
  children: React.ReactNode;
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
  const typingEnabled = Boolean(settings?.typing.enabled);
  const zen = Boolean(settings?.zen);
  const pinned = Boolean(app.window?.alwaysOnTop);

  const [session, setSession] = useState<ReaderSession | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showToc, setShowToc] = useState(false);
  const [showTheme, setShowTheme] = useState(false);
  const [reveal, setReveal] = useState<number | null>(null);
  const [revealByChapter, setRevealByChapter] = useState<Record<number, number>>({});
  const [presets, setPresets] = useState<Theme[]>([]);

  const stageRef = useRef<HTMLDivElement>(null);
  const viewportRef = useRef<HTMLDivElement>(null);
  const trackRef = useRef<HTMLDivElement>(null);
  const caretRef = useRef<HTMLSpanElement>(null);
  const restoredRef = useRef('');
  const prefetched = useRef('');

  const showGutter = Boolean(theme.lineNumbers) || skin === 'vscode';
  const geometry = useGeometry(stageRef, theme, showGutter, session?.chapter.uid ?? -1, skin !== 'none');
  const blocks = useMemo(() => session?.content.blocks ?? [], [session]);
  const metrics = useMemo(() => measureBlocks(blocks), [blocks]);

  const signature = [
    skin,
    session?.chapter.uid ?? 0,
    blocks.length,
    theme.fontSize,
    theme.lineHeight,
    theme.letterSpacing,
    theme.columns,
    theme.pageWidth,
    theme.paragraphSpacing,
    typingEnabled,
  ].join('|');

  const { page, pages, setPage } = usePagination(trackRef, geometry, signature);
  const percent = pages > 1 ? ((page + 1) / pages) * 100 : 100;

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
    if (!typingEnabled) setReveal(null);
    else setReveal((prev) => prev ?? revealByChapter[session?.chapter.uid ?? 0] ?? 0);
  }, [typingEnabled, revealByChapter, session?.chapter.uid]);

  // Restore the saved offset once the page count is known.
  useLayoutEffect(() => {
    if (!session || pages <= 1) return;
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
    if (target > 0) setPage(target);
  }, [session, pages, metrics.total, setPage, skipRestore]);

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

  // ____________________________________________________________ navigation

  const chapterIndex = useMemo(() => {
    if (!session) return 0;
    return session.chapters.findIndex((c) => c.uid === session.chapter.uid);
  }, [session]);

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

  const turn = useCallback(
    (delta: number) => {
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
      setPage(page + delta);
      void api.touchReader().catch(() => undefined);
    },
    [typingEnabled, advanceTyping, geometry, page, pages, setPage, goChapter],
  );

  // Keep the caret's column on screen while typing.
  useLayoutEffect(() => {
    if (!typingEnabled || reveal === null) return;
    const target = caretPage(caretRef.current, trackRef.current, geometry);
    if (target !== null && target !== page) setPage(target);
  }, [reveal, typingEnabled, geometry, page, setPage]);

  // ------------------------------------------------------------ persistence

  const flatText = useMemo(
    () => blocks.map((b) => (b.kind === 'image' || b.kind === 'divider' ? '' : b.text ?? '')).join(''),
    [blocks],
  );

  useEffect(() => {
    if (!session || !settings?.reader.reportProgress) return;
    const total = metrics.total || 1;
    const chapterPercent = reveal !== null ? (reveal / total) * 100 : percent;
    const offset = Math.round((chapterPercent / 100) * total);
    const summary = Array.from(flatText).slice(offset, offset + 20).join('');
    const timer = window.setTimeout(() => {
      void api
        .savePosition({
          bookId,
          chapterUid: session.chapter.uid,
          chapterIndex: session.chapter.index,
          offset,
          chapterPercent,
          summary,
        })
        .catch(() => undefined);
    }, 900);
    return () => window.clearTimeout(timer);
  }, [bookId, session, percent, reveal, metrics.total, flatText, settings?.reader.reportProgress]);

  useEffect(() => {
    if (!session || !settings?.reader.prefetchNext) return;
    const next = session.chapters[chapterIndex + 1];
    if (!next) return;
    const nearEnd = reveal !== null ? reveal >= metrics.total - 200 : page >= pages - 2;
    if (!nearEnd) return;
    const key = `${bookId}:${next.uid}`;
    if (prefetched.current === key) return;
    prefetched.current = key;
    void api.openChapter(bookId, next.uid).catch(() => undefined);
  }, [session, chapterIndex, page, pages, reveal, metrics.total, bookId, settings?.reader.prefetchNext]);

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
          skipToNextBlock();
          return;
        }
      }

      switch (e.key) {
        case 'ArrowLeft':
        case 'ArrowUp':
        case 'PageUp':
          e.preventDefault();
          turn(-1);
          break;
        case 'ArrowRight':
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
          setPage(0);
          break;
        case 'End':
          e.preventDefault();
          setPage(pages - 1);
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
    skipToNextBlock, setPage, pages, zen, showTheme, showToc,
  ]);

  // Wheel paging, throttled so trackpads do not fly through the book.
  useEffect(() => {
    const el = viewportRef.current;
    if (!el) return;
    let last = 0;
    const onWheel = (e: WheelEvent) => {
      const now = Date.now();
      if (now - last < 260 || Math.abs(e.deltaY) < 8) return;
      last = now;
      e.preventDefault();
      turn(e.deltaY > 0 ? 1 : -1);
    };
    el.addEventListener('wheel', onWheel, { passive: false });
    return () => el.removeEventListener('wheel', onWheel);
  }, [turn]);

  // ------------------------------------------------------------ theme editing

  const updateTheme = useCallback((patch: Partial<Theme>) => {
    const current = store.get().settings;
    if (!current) return;
    void api
      .saveSettings({ ...current, theme: { ...current.theme, ...patch, name: current.theme.name || '自定义' } })
      .then((s) => store.set({ settings: s }));
  }, []);

  const applyPreset = useCallback((t: Theme) => {
    const current = store.get().settings;
    if (!current) return;
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

  const togglePin = useCallback(() => {
    void api.setAlwaysOnTop(!pinned).then((w) => store.set({ window: w }));
  }, [pinned]);

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
    marginX: theme.marginX,
    marginY: theme.marginY,
    paper: skin === 'none' ? 'themed' : 'transparent',
  });

  const canvas = (
    <ReaderCanvas
      blocks={blocks}
      theme={theme}
      geometry={geometry}
      page={page}
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
      flush={skin !== 'none'}
    />
  );

  const chromeProps: ReaderChromeProps = {
    bookTitle: session.book.title,
    bookAuthor: session.book.author,
    chapter: session.chapter,
    chapters: session.chapters,
    page,
    pages,
    percent,
    zen,
    typing: typingEnabled,
    pinned,
    skin,
    themeStyle,
    onPrev: () => turn(-1),
    onNext: () => turn(1),
    onJump: (p) => setPage(p),
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

  const body =
    skin === 'word' ? wordSkin(chromeProps) : skin === 'vscode' ? vsCodeSkin(chromeProps) : plainReader(chromeProps);

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
                className={`toc-item level-${Math.min(3, c.level || 1)}${c.uid === session.chapter.uid ? ' current' : ''}`}
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
    </div>
  );
}

// ---------------------------------------------------------------- plain chrome

function plainReader(p: ReaderChromeProps) {
  return (
    <div className={`reader${p.zen ? ' zen' : ''}${p.typing ? ' typing' : ''}`} style={p.themeStyle}>
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

      <div className="reader-bar bottom">
        <button className="icon-btn" onClick={p.onPrev} title="上一页">
          <Icon name="left" />
        </button>
        <div className="reader-progress">
          <input
            type="range"
            min={0}
            max={Math.max(1, p.pages - 1)}
            value={p.page}
            onChange={(e) => p.onJump(Number(e.target.value))}
          />
          <span className="pct">{p.percent.toFixed(1)}%</span>
        </div>
        <span className="page-jump">
          {p.page + 1} / {p.pages}
        </span>
        <button className="icon-btn" onClick={p.onNext} title="下一页">
          <Icon name="right" />
        </button>
      </div>
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
    fallbackFonts: 'SimSun, serif',
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
    pageWidth: 720,
    marginX: 56,
    marginY: 44,
    lineNumbers: false,
    columns: 1,
    headingColor: '#1d1d1d',
    accentColor: '#2f6fed',
    selectionColor: 'rgba(47,111,237,0.22)',
    paragraphStyle: 'indent',
  };
}
