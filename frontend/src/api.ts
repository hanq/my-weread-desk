// Thin typed façade over the Wails bindings and runtime. Everything funnels
// through here so the rest of the UI never touches `window.go` directly and
// so running the frontend in a plain browser fails with one clear message.

import type {
  AuthState, Book, BookDetail, Chapter, DownloadState, LoginPoll, MPArticle,
  Note, Position, Progress, QRCode, ReadStats, ReaderSession, SearchResult,
  Settings, Theme, WindowState,
} from './types';

type AnyFn = (...args: unknown[]) => Promise<unknown>;

interface Bindings {
  [name: string]: AnyFn;
}

function bindings(): Bindings {
  const g = (window as any).go;
  const app = g?.app?.App;
  if (!app) {
    throw new Error('未连接到本地服务：请通过桌面应用启动（wails dev / wails build）。');
  }
  return app as Bindings;
}

async function call<T>(name: string, ...args: unknown[]): Promise<T> {
  const fn = bindings()[name];
  if (!fn) throw new Error(`绑定方法不存在：${name}`);
  return (await fn(...args)) as T;
}

export const api = {
  // settings & window
  getSettings: () => call<Settings>('GetSettings'),
  saveSettings: (s: Settings) => call<Settings>('SaveSettings', s),
  updateSetting: (key: string, value: unknown) => call<Settings>('UpdateSetting', key, value),
  presetThemes: () => call<Theme[]>('PresetThemes'),
  resetSettings: () => call<Settings>('ResetSettings'),
  acceptTerms: () => call<void>('AcceptTerms'),
  fontFamilies: () => call<string[]>('FontFamilies'),
  dataDir: () => call<string>('DataDir'),
  openDataDir: () => call<void>('OpenDataDir'),
  appVersion: () => call<string>('AppVersion'),

  getWindowState: () => call<WindowState>('GetWindowState'),
  setAlwaysOnTop: (on: boolean) => call<WindowState>('SetAlwaysOnTop', on),
  setOpacity: (alpha: number) => call<WindowState>('SetOpacity', alpha),
  setClickThrough: (on: boolean) => call<WindowState>('SetClickThrough', on),
  setWindowTitle: (title: string) => call<void>('SetWindowTitle', title),
  setZen: (on: boolean) => call<WindowState>('SetZen', on),
  setSkin: (skin: string) => call<WindowState>('SetSkin', skin),
  setTyping: (on: boolean) => call<WindowState>('SetTyping', on),
  hideWindow: () => call<void>('HideWindow'),
  showWindow: () => call<void>('ShowWindow'),
  minimiseWindow: () => call<void>('MinimiseWindow'),
  closeWindow: () => call<void>('CloseWindow'),
  maximiseWindow: () => call<void>('MaximiseWindow'),
  registerHotkeys: () => call<void>('RegisterHotkeys'),

  // auth
  getAuthState: () => call<AuthState>('GetAuthState'),
  startLogin: () => call<QRCode>('StartLogin'),
  pollLogin: (uid: string, otp: string) => call<LoginPoll>('PollLogin', uid, otp),
  logout: () => call<void>('Logout'),
  renewSession: () => call<void>('RenewSession'),

  // library
  books: () => call<Book[]>('Books'),
  syncShelf: () => call<Book[]>('SyncShelf'),
  search: (keyword: string, scope: string, page: number) =>
    call<SearchResult>('Search', keyword, scope, page),
  bookDetail: (bookId: string) => call<BookDetail>('BookDetail', bookId),
  chapters: (bookId: string) => call<Chapter[]>('Chapters', bookId),
  mutateShelf: (bookId: string, action: string) =>
    call<void>('MutateShelf', { bookId, action }),
  notes: (bookId: string) => call<Note[]>('Notes', bookId),
  stats: (mode: string, baseTime: number) => call<ReadStats>('Stats', mode, baseTime),
  mpArticles: (accountId: string, offset: number) =>
    call<MPArticle[]>('MPArticles', accountId, offset),
  openMPArticle: (reviewId: string) => call<any>('OpenMPArticle', reviewId),
  cachedChapterCount: (bookId: string) =>
    call<{ cached: number; total: number }>('CachedChapterCount', bookId),
  clearChapterCache: (bookId: string) => call<void>('ClearChapterCache', bookId),

  // reader
  openChapter: (bookId: string, chapterUid: number) =>
    call<ReaderSession>('OpenChapter', bookId, chapterUid),
  savePosition: (position: Position) => call<void>('SavePosition', position),
  storedProgress: (bookId: string) => call<Progress | null>('StoredProgress', bookId),
  syncProgress: (bookId: string) => call<Progress>('SyncProgress', bookId),
  reportNow: (bookId: string) => call<void>('ReportNow', bookId),
  touchReader: () => call<void>('TouchReader'),

  // downloads
  startDownload: (bookId: string) => call<DownloadState>('StartDownload', bookId),
  downloadState: (bookId: string) => call<DownloadState>('GetDownloadState', bookId),
  cancelDownload: () => call<void>('CancelDownload'),
};

// ---------------------------------------------------------------- runtime

type Handler = (...args: any[]) => void;

export const events = {
  on(name: string, handler: Handler): () => void {
    const rt = (window as any).runtime;
    if (!rt?.EventsOn) return () => {};
    return rt.EventsOn(name, handler);
  },
  emit(name: string, ...args: unknown[]) {
    (window as any).runtime?.EventsEmit?.(name, ...args);
  },
};

export const runtime = {
  windowGetSize(): Promise<[number, number]> {
    return (window as any).runtime?.WindowGetSize?.() ?? Promise.resolve([0, 0]);
  },
  windowSetSize(w: number, h: number) {
    (window as any).runtime?.WindowSetSize?.(w, h);
  },
  windowGetPosition(): Promise<[number, number]> {
    return (window as any).runtime?.WindowGetPosition?.() ?? Promise.resolve([0, 0]);
  },
  windowSetPosition(x: number, y: number) {
    (window as any).runtime?.WindowSetPosition?.(x, y);
  },
  openURL(url: string) {
    (window as any).runtime?.BrowserOpenURL?.(url);
  },
  quit() {
    (window as any).runtime?.Quit?.();
  },
};

// startLiveness keeps the "is the user actually reading" clock honest.
export function startLiveness(): () => void {
  const bump = () => {
    void api.touchReader().catch(() => {});
  };
  // A long page can be read for minutes without input, so the reader is
  // considered active while the window has focus.
  const tick = () => {
    if (document.hasFocus()) bump();
  };
  const id = window.setInterval(tick, 45_000);
  window.addEventListener('focus', bump);
  window.addEventListener('keydown', bump);
  window.addEventListener('mousedown', bump);
  return () => {
    window.clearInterval(id);
    window.removeEventListener('focus', bump);
    window.removeEventListener('keydown', bump);
    window.removeEventListener('mousedown', bump);
  };
}
