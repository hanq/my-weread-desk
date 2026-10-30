// Development-only fake backend. `npm run dev` in a browser (outside Wails)
// installs this so layout and interaction work can be checked without a real
// account. It is imported only when `import.meta.env.DEV` and no bindings
// exist, so production builds never include the path.

import type { Block, Book, Chapter, ReaderSession, Settings, Theme } from './types';

const theme: Theme = {
  name: '纸白',
  fontFamily: 'Source Han Serif SC',
  fallbackFonts: 'Noto Serif SC, Songti SC, SimSun, serif',
  fontSize: 19,
  lineHeight: 1.85,
  letterSpacing: 0.02,
  paragraphSpacing: 12,
  textIndent: 2,
  textColor: '#2b2b2b',
  background: '#e9e5da',
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

const night: Theme = { ...theme, name: '夜间', background: '#14161a', surface: '#1b1e24', textColor: '#c9ced8', headingColor: '#e8ecf3', accentColor: '#6d9bff' };

const settings: Settings = {
  version: 1,
  theme,
  customThemes: [],
  skin: 'none',
  zen: false,
  typing: { enabled: false, hideUnread: true, faintUnread: false, sound: false, cursorShape: 'bar', skipOnEnter: true, advanceOnAny: true },
  window: { x: 80, y: 60, w: 1180, h: 780, alwaysOnTop: false, opacity: 1, clickThrough: false, frameless: true, maximised: false, hideOnBlur: false, remembered: true, radius: 12 },
  reader: { pageTurnAnimation: true, pageMode: 'page', pageTurnEffect: 'slide', wheelTurn: true, clickTurn: true, showScrollbar: true, smoothScroll: true, autoHideBars: true, showStatusBar: true, reportProgress: true, reportReadTime: true, reportInterval: 30, confirmExit: false, prefetchNext: true },
  ai: { enabled: false, baseUrl: '', apiKey: '', model: '', temperature: 0.3, maxTokens: 1200, consent: false },
  disguise: {
    taskbarIcon: true,
    hideTaskbar: false,
    contactName: '文件传输助手',
    nickname: '我',
    avatar: '',
    sendOnEnter: true,
    floating: false,
    hoverReveal: false,
    showZone: { x: 0, y: 120, w: 8, h: 180 },
    hideZone: { x: 0, y: 420, w: 8, h: 180 },
    migrated: true,
  },
  hotkeys: { zen: 'Ctrl+Alt+Z', toggleTop: 'Ctrl+Alt+T', typing: 'Ctrl+Alt+D', panic: 'Ctrl+Alt+H', cycleSkin: 'Ctrl+Alt+M', clickThrough: 'Ctrl+Alt+C' },
  windowTitle: 'Document1 - Word',
  hideTray: false,
  closeToTray: true,
  acceptedTerms: true,
  lastBookId: 'b1',
};

const titles = [
  ['置身事内：中国政府与经济发展', '兰小欢'],
  ['人类简史', '尤瓦尔·赫拉利'],
  ['系统之美', '德内拉·梅多斯'],
  ['被讨厌的勇气', '岸见一郎'],
  ['克拉拉与太阳', '石黑一雄'],
  ['我们仨', '杨绛'],
  ['枪炮、病菌与钢铁', '贾雷德·戴蒙德'],
  ['万历十五年', '黄仁宇'],
  ['娱乐至死', '尼尔·波兹曼'],
  ['中国历代政治得失', '钱穆'],
  ['局外人', '阿尔贝·加缪'],
  ['深度工作', '卡尔·纽波特'],
] as const;

const books: Book[] = titles.map(([title, author], i) => ({
  id: `book-${i + 1}`,
  title,
  author,
  cover: '',
  category: i % 3 === 0 ? '历史' : '社科',
  intro: '这是一本用于界面预览的示例书籍。真实内容来自你的微信读书账号，只会缓存到本机。',
  rating: 8.1 + (i % 9) / 10,
  ratingCount: 1200 + i * 37,
  wordCount: 180000 + i * 12000,
  finished: i === 2,
  secret: false,
  isMp: i === 11,
  lastRead: 1760000000 - i * 86400,
  paid: false,
  updateAt: 1760000000,
}));

const chapter = (i: number): Chapter => ({
  uid: 1000 + i,
  index: i + 1,
  title: i === 0 ? '第一章　地方政府的权力与事务' : `第${i + 1}章　${['财税与政府行为', '政府投融资', '工业化中的政府角色', '城市化与不平衡', '债务与风险', '国内国际失衡'][i % 7] ?? '附录'}`,
  level: 1,
  wordCount: 4800,
  paid: false,
});

const chapters = Array.from({ length: 9 }, (_, i) => chapter(i));

const paragraphs = [
  '要理解中国经济，先要理解地方政府。地方政府不仅是行政主体，也是经济主体；它掌握土地、金融与政策工具，深度参与投资与招商。',
  '本章从地方政府的事权与财权出发，讨论权力如何在纵向与横向之间划分。事权划分有三个原则：公共服务的规模经济、信息复杂性、激励相容。',
  '第一条原则是公共服务的规模经济。国防、外交这类服务覆盖全国，边际成本随人口增加而下降，适合由中央提供；而治安、消防、教育这类服务与本地居民的生活半径高度相关，由地方提供更有效率。',
  '第二条原则是信息复杂性。上级政府很难完全掌握基层的真实情况，信息在层层传递中损耗、变形。因此，把决策权交给拥有信息优势的一方，往往比集中决策更有效。',
  '第三条原则是激励相容。所谓激励相容，是指一方想做的事，正好也是另一方希望它做的事。如果制度设计让地方官员的升迁与本地的经济增长挂钩，那么地方政府就有动力去推动投资与招商。',
  '这三条原则解释了为什么中国的行政体制既不是完全的集权，也不是完全的联邦制，而是一种在集权与分权之间反复调整的混合结构。',
  '理解了这一点，再看土地财政、地方融资平台、招商引资中的补贴竞争，就不再是孤立的怪现象，而是同一套激励结构在不同领域的具体表现。',
  '以土地为例。土地所有权归国家或集体，但使用权可以转让。地方政府通过征地、整理、出让获得收入，再用这些收入投资基础设施。基础设施改善又会推高地价，形成循环。',
  '这个循环在城市化高速推进时非常有效，但也会带来两个后果：一是对土地出让收入的依赖，二是地方债务的累积。当城市化速度放缓，循环的动能减弱，风险就暴露出来。',
  '因此，近年来的改革方向是：厘清事权与支出责任，规范地方政府举债融资机制，逐步降低对土地收入的依赖。这些改革并不容易，因为它们触及既有的激励结构。',
  '把视角拉远一些，经济现象从来不是孤立的。价格、投资、债务、人口流动，都是无数个体在特定制度约束下做出选择的结果。理解约束，比记住结论更重要。',
  '这也是本书想做的事：不是提供一个标准答案，而是提供一套可以反复使用的分析框架。当你下次看到一条新闻时，能问出「谁受益、谁承担成本、约束条件是什么」这三个问题。',
];

const blocks: Block[] = [
  { kind: 'heading', level: 1, text: chapters[0].title },
  ...paragraphs.map((text, i) => ({ kind: 'para' as const, text, class: i === 0 ? 'no-indent' : '' })),
  { kind: 'heading', level: 2, text: '第一节　事权划分的三条原则' },
  ...paragraphs.slice(3).map((text) => ({ kind: 'para' as const, text })),
  { kind: 'quote', text: '理解约束，比记住结论更重要。' },
  ...paragraphs.slice(0, 6).map((text) => ({ kind: 'para' as const, text })),
];

const session = (chapterUid: number): ReaderSession => {
  const idx = Math.max(0, chapters.findIndex((c) => c.uid === chapterUid));
  const ch = chapters[idx] ?? chapters[0];
  return {
    book: books[0],
    chapter: ch,
    chapters,
    content: { bookId: books[0].id, chapterUid: ch.uid, title: ch.title, format: 'epub', blocks, fetchedAt: 1760000000 },
    progress: { bookId: books[0].id, chapterUid: ch.uid, chapterIndex: ch.index, offset: 220, percent: 12, updateTime: 1760000000 },
    fromCache: true,
  };
};

const handlers: Record<string, (...args: any[]) => any> = {
  GetSettings: () => settings,
  SaveSettings: (s: Settings) => Object.assign(settings, s),
  UpdateSetting: (k: string, v: any) => {
    (settings as any)[k] = v;
    return settings;
  },
  PresetThemes: () => [theme, night, { ...theme, name: '羊皮', background: '#efe3c8', surface: '#f7ecd6', textColor: '#4a3b28' }, { ...theme, name: '护眼', background: '#1f2a24', surface: '#243029', textColor: '#b9cdb9' }, { ...theme, name: '透明', background: 'transparent', surface: 'transparent', textColor: '#f2f4f8' }, { ...theme, name: '等宽', fontFamily: 'Cascadia Mono', background: '#1e1e1e', surface: '#252526', textColor: '#d4d4d4', lineNumbers: true }],
  ResetSettings: () => settings,
  AcceptTerms: () => undefined,
  FontFamilies: () => ['Songti SC', 'SimSun', 'Microsoft YaHei', 'KaiTi', 'Cascadia Mono', 'Consolas', 'Georgia'],
  DataDir: () => 'C:/Users/demo/AppData/Roaming/my-weread-desk',
  OpenDataDir: () => undefined,
  ChooseAvatar: () => settings,
  ClearAvatar: () => settings,
  AppVersion: () => '0.1.0-dev',
  GetWindowState: () => ({ alwaysOnTop: false, clickThrough: false, opacity: 1, zen: settings.zen, skin: settings.skin, typing: settings.typing.enabled, title: settings.windowTitle, width: 1180, height: 780 }),
  SetAlwaysOnTop: (on: boolean) => ({ ...(handlers.GetWindowState() as any), alwaysOnTop: on }),
  SetOpacity: (a: number) => ({ ...(handlers.GetWindowState() as any), opacity: a }),
  SetWindowRadius: () => undefined,
  SetClickThrough: (on: boolean) => ({ ...(handlers.GetWindowState() as any), clickThrough: on }),
  SetWindowTitle: () => undefined,
  SetZen: (on: boolean) => {
    settings.zen = on;
    return { ...(handlers.GetWindowState() as any), zen: on };
  },
  SetSkin: (skin: string) => {
    settings.skin = skin;
    return { ...(handlers.GetWindowState() as any), skin };
  },
  SetTyping: (on: boolean) => {
    settings.typing.enabled = on;
    return { ...(handlers.GetWindowState() as any), typing: on };
  },
  CursorPosition: () => ({ x: 640, y: 400, width: 1280, height: 800 }),
  HideWindow: () => undefined,
  ShowWindow: () => undefined,
  MinimiseWindow: () => undefined,
  CloseWindow: () => undefined,
  MaximiseWindow: () => undefined,
  RegisterHotkeys: () => undefined,
  GetAuthState: () => ({ loggedIn: true, account: { alias: 'default', vid: 'demo', name: '演示账号', avatar: '', hasApiKey: true, updatedAt: 1760000000 }, hasApiKey: true }),
  StartLogin: () => ({ uid: 'demo', payload: '', image: '', expires: 0 }),
  PollLogin: () => ({ pending: true, needOtp: false, done: false, message: '开发预览模式' }),
  Logout: () => undefined,
  RenewSession: () => undefined,
  Books: () => books,
  SyncShelf: () => books,
  Search: (kw: string) => ({ total: books.length, hasMore: false, sid: '', query: kw, books: books.filter((b) => b.title.includes(kw) || !kw) }),
  BookDetail: (id: string) => ({ book: books.find((b) => b.id === id) ?? books[0], chapters, progress: { bookId: id, chapterUid: chapters[0].uid, chapterIndex: 1, offset: 220, percent: 12, updateTime: 1760000000 }, cachedCount: 3, chapterCount: chapters.length, inShelf: true, notes: [{ id: 'n1', bookId: id, chapterUid: chapters[0].uid, chapterName: chapters[0].title, text: '理解约束，比记住结论更重要。', content: '', range: '', color: 1, createTime: 1760000000, type: 0 }], offline: false }),
  Chapters: () => chapters,
  MutateShelf: () => undefined,
  Notes: () => [],
  Stats: () => ({ mode: 'weekly', baseTime: 0, totalReadTime: 72300, dailyReadTimes: [3200, 5400, 0, 7300, 4200, 8800, 6100], readDays: 6, dayAverageReadTime: 10320, compare: 0.12, readLongest: books.slice(0, 5).map((b) => ({ bookId: b.id, title: b.title, author: b.author, seconds: 7200, date: 1760000000 })), preferCategory: [{ name: '社科', seconds: 32000, ratio: 0.44 }, { name: '历史', seconds: 21000, ratio: 0.29 }, { name: '文学', seconds: 12000, ratio: 0.17 }], rank: 128 }),
  DeviceStats: () => ({ client: 'MyWeReadDesk', deviceName: 'BOOX', deviceId: '...123456', totalSeconds: 0, todaySeconds: 0, daily: [] }),
  MPArticles: () => [],
  OpenMPArticle: () => ({ blocks: [] }),
  CachedChapterCount: () => ({ cached: 3, total: chapters.length }),
  ClearChapterCache: () => undefined,
  OpenChapter: (_id: string, uid: number) => session(uid),
  SavePosition: () => undefined,
  StoredProgress: () => null,
  SyncProgress: () => null,
  ReportNow: () => undefined,
  TouchReader: () => undefined,
  StartDownload: (id: string) => ({ bookId: id, running: false, total: 9, done: 9, failed: 0, current: '', errors: [], message: '完成' }),
  GetDownloadState: (id: string) => ({ bookId: id, running: false, total: 0, done: 0, failed: 0, current: '', errors: [], message: '' }),
  CancelDownload: () => undefined,
  EmbedWindow: () => undefined,
  UnembedWindow: () => undefined,
  FlushProgress: () => undefined,
};

export function installDevMock() {
  (window as any).go = { app: { App: handlers } };
  (window as any).runtime = {
    EventsOn: () => () => undefined,
    EventsEmit: () => undefined,
    WindowGetSize: () => Promise.resolve([1180, 780]),
    WindowGetPosition: () => Promise.resolve([80, 60]),
    WindowSetSize: () => undefined,
    WindowSetPosition: () => undefined,
    BrowserOpenURL: () => undefined,
    Quit: () => undefined,
  };
  console.info('[dev] mock backend installed');
}
