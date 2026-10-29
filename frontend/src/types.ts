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
  lastRead: number;
  publisher?: string;
  publishAt?: string;
  isbn?: string;
  price?: number;
  paid: boolean;
  updateAt: number;
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
  autoHideBars: boolean;
  showStatusBar: boolean;
  reportProgress: boolean;
  reportReadTime: boolean;
  reportInterval: number;
  confirmExit: boolean;
  prefetchNext: boolean;
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

export interface MPArticle {
  reviewId: string;
  title: string;
  cover: string;
  createTime: number;
  account: string;
}
