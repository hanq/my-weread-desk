// Wire types mirroring the Go binding payloads. Keep field names identical to
// the JSON tags on the Go side.

export interface Book {
  id: string;
  title: string;
  author: string;
  cover: string;
  category: string;
  intro: string;
  rating: number;
  ratingCount: number;
  wordCount: number;
  finished: boolean;
  secret: boolean;
  isMp: boolean;
  /** Audiobook album: it has tracks, not chapters, so the reader cannot open it. */
  isAudio?: boolean;
  trackCount?: number;
  /** Shelf folder. Absent means ungrouped. */
  groupId?: string;
  groupName?: string;
  lastRead: number;
  publisher?: string;
  publishAt?: string;
  isbn?: string;
  price?: number;
  paid: boolean;
  updateAt: number;
}

export interface ShelfGroup {
  id: string;
  name: string;
  /** cloud = mirrored from the account, local = created in this app. */
  source: 'cloud' | 'local' | string;
  bookIds?: string[];
  sort: number;
}

export interface Chapter {
  uid: number;
  index: number;
  title: string;
  level: number;
  wordCount: number;
  paid: boolean;
  price?: number;
  tar?: string;
  format?: string;
}

export interface Block {
  kind: 'heading' | 'para' | 'quote' | 'image' | 'divider' | 'pre' | string;
  level?: number;
  text?: string;
  src?: string;
  alt?: string;
  class?: string;
  href?: string;
}

export interface ChapterContent {
  bookId: string;
  chapterUid: number;
  title: string;
  format: string;
  blocks: Block[];
  css?: string;
  fetchedAt: number;
  images?: string[];
}

export interface Progress {
  bookId: string;
  chapterUid: number;
  chapterIndex: number;
  offset: number;
  percent: number;
  updateTime: number;
  isStart?: boolean;
  finished?: boolean;
}

export interface SearchResult {
  total: number;
  hasMore: boolean;
  sid: string;
  books: Book[];
  query: string;
}

export interface Note {
  id: string;
  bookId: string;
  chapterUid: number;
  chapterName: string;
  text: string;
  content: string;
  range: string;
  color: number;
  createTime: number;
  type: number;
}

export interface Account {
  alias: string;
  vid: string;
  name: string;
  avatar: string;
  hasApiKey: boolean;
  updatedAt: number;
}

export interface AuthState {
  loggedIn: boolean;
  account?: Account;
  hasApiKey: boolean;
}

export interface QRCode {
  uid: string;
  payload: string;
  image: string;
  expires: number;
}

export interface LoginPoll {
  pending: boolean;
  needOtp: boolean;
  done: boolean;
  /** The session can no longer be polled; wait for an explicit refresh. */
  expired: boolean;
  message: string;
  account?: Account;
}

export interface BookDetail {
  book: Book;
  chapters: Chapter[];
  progress?: Progress;
  cachedCount: number;
  chapterCount: number;
  inShelf: boolean;
  notes: Note[];
  offline: boolean;
}

export interface ReaderSession {
  book: Book;
  chapter: Chapter;
  chapters: Chapter[];
  content: ChapterContent;
  progress?: Progress;
  fromCache: boolean;
}

export interface Theme {
  name: string;
  fontFamily: string;
  fallbackFonts: string;
  fontSize: number;
  lineHeight: number;
  letterSpacing: number;
  paragraphSpacing: number;
  textIndent: number;
  textColor: string;
  background: string;
  surface: string;
  surfaceOpacity: number;
  textAlign: 'left' | 'justify' | 'center' | string;
  pageWidth: number;
  marginX: number;
  marginY: number;
  lineNumbers: boolean;
  columns: number;
  headingColor: string;
  accentColor: string;
  selectionColor: string;
  paragraphStyle: string;
}

export interface WindowSettings {
  x: number;
  y: number;
  w: number;
  h: number;
  alwaysOnTop: boolean;
  opacity: number;
  clickThrough: boolean;
  frameless: boolean;
  maximised: boolean;
  hideOnBlur: boolean;
  remembered: boolean;
  radius: number;
}

export interface TypingSettings {
  enabled: boolean;
  hideUnread: boolean;
  faintUnread: boolean;
  sound: boolean;
  cursorShape: 'bar' | 'block' | 'underline' | string;
  skipOnEnter: boolean;
  advanceOnAny: boolean;
}

export interface ReaderSettings {
  pageTurnAnimation: boolean;
  pageMode: 'page' | 'scroll' | string;
  pageTurnEffect: 'slide' | 'fade' | 'none' | string;
  wheelTurn: boolean;
  clickTurn: boolean;
  showScrollbar: boolean;
  smoothScroll: boolean;
  autoHideBars: boolean;
  showStatusBar: boolean;
  reportProgress: boolean;
  reportReadTime: boolean;
  reportInterval: number;
  confirmExit: boolean;
  prefetchNext: boolean;
}

export interface AISettings {
  enabled: boolean;
  baseUrl: string;
  apiKey: string;
  model: string;
  temperature: number;
  maxTokens: number;
  consent: boolean;
}

export interface RAGStatus {
  bookId: string;
  status: string;
  indexed: boolean;
  chunkCount: number;
  charCount: number;
  error: string;
  updatedAt: number;
}

export interface AISource {
  index: number;
  bookId: string;
  chapterUid: number;
  chapterIdx: number;
  chapterTitle: string;
  blockStart: number;
  blockEnd: number;
  excerpt: string;
}

export interface AskEvent {
  requestId: string;
  delta?: string;
  text?: string;
  sources?: AISource[];
  model?: string;
  error?: string;
}

export interface Settings {
  version: number;
  theme: Theme;
  customThemes: Theme[];
  skin: 'none' | 'word' | 'vscode' | string;
  zen: boolean;
  typing: TypingSettings;
  window: WindowSettings;
  reader: ReaderSettings;
  ai: AISettings;
  disguise: {
    taskbarIcon: boolean;
    hideTaskbar: boolean;
    contactName: string;
    nickname: string;
    avatar: string;
    sendOnEnter: boolean;
    floating: boolean;
    hoverReveal: boolean;
    showZone: { x: number; y: number; w: number; h: number };
    hideZone: { x: number; y: number; w: number; h: number };
    migrated: boolean;
  };
  hotkeys: Record<string, string>;
  windowTitle: string;
  hideTray: boolean;
  closeToTray: boolean;
  acceptedTerms: boolean;
  lastBookId: string;
}

export interface WindowState {
  alwaysOnTop: boolean;
  clickThrough: boolean;
  opacity: number;
  zen: boolean;
  skin: string;
  typing: boolean;
  title: string;
  width: number;
  height: number;
}

export interface CursorInfo {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface DownloadState {
  bookId: string;
  running: boolean;
  total: number;
  done: number;
  failed: number;
  current: string;
  errors: string[];
  message: string;
}

export interface Position {
  bookId: string;
  chapterUid: number;
  chapterIndex: number;
  offset: number;
  chapterPercent: number;
  summary: string;
}

export interface ReadStats {
  mode: string;
  baseTime: number;
  totalReadTime: number;
  dailyReadTimes: number[];
  readDays: number;
  dayAverageReadTime: number;
  compare: number;
  readLongest: { bookId: string; title: string; author: string; seconds: number; date: number }[];
  preferCategory: { name: string; seconds: number; ratio: number }[];
  rank: number;
}

export interface DeviceReadDay {
  date: string;
  seconds: number;
}

/** Local per-installation reading time. WeRead only exposes account totals. */
export interface DeviceReadStats {
  client: string;
  deviceName: string;
  deviceId: string;
  totalSeconds: number;
  todaySeconds: number;
  daily: DeviceReadDay[];
}

export interface MPArticle {
  reviewId: string;
  title: string;
  cover: string;
  createTime: number;
  account: string;
}
