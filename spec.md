# my-weread-desk 产品与技术规格（spec.md）

> 版本：v0.1 · 2026-10-29
> 目标平台：Windows 10/11 x64（主），macOS/Linux 保留可编译路径
> 技术栈：Go 1.25 + Wails v2.14 + React 18 + TypeScript + Vite
> 协议调研依据：`docs/research/weread-ecosystem-research-2026-10-29.md`

---

## 1. 产品定位

一个**桌面端的微信读书客户端**：扫码登录后可直接浏览书架、搜索书籍、下载章节、在本地渲染阅读，并把进度与阅读时长同步回微信读书。

与 KOReader 插件的差异在于：面向桌面（键鼠、多窗口、大屏），核心价值是**可控的排版**与**可控的窗口行为**——禅模式、主题定制、打字伪装、Word/VSCode 皮肤、透明置顶、窗口嵌入。

与 Electron 内嵌网页方案的差异在于：不加载官方 Web 页面，自建渲染层，因此可以做行号、分页、伪装与低占用。

### 1.1 非目标（明确不做）

- 不绕过付费/会员：只能阅读账号已有权益的章节。
- 不提供批量导出全库的默认入口（单本手动导出可保留为高级功能）。
- 不做社交、评论发布、书城运营位。
- 不做音频/有声书播放（协议存在但成本高，列为后续）。

### 1.2 合规声明

应用首启展示免责声明：非官方工具，与腾讯/微信读书无关联；仅供个人学习研究；账号风险自负。默认请求频率保守（串行 + 抖动），不内置任何「刷时长」自动化。

---

## 2. 功能范围

### 2.1 账号

| 编号 | 功能 | 优先级 |
|---|---|---|
| A1 | 微信扫码登录（二维码 + 轮询 + 4 位验证码兜底） | P0 |
| A2 | 设备指纹 `wr_fp` 生成与持久化（防互踢） | P0 |
| A3 | 凭据加密落盘（DPAPI / 文件权限） | P0 |
| A4 | 令牌自动续期（Web renewal + E-Ink `/login`） | P0 |
| A5 | 多账号切换 | P2 |
| A6 | 退出登录（保留设备指纹） | P0 |

### 2.2 书库

| 编号 | 功能 | 优先级 |
|---|---|---|
| B1 | 书架同步（分页、分组、私密标记、读完状态） | P0 |
| B2 | 搜索（书名/作者/全文，分 scope） | P0 |
| B3 | 书籍详情（简介、评分、字数、目录） | P0 |
| B4 | 加入/移出书架、标记读完/在读、置顶、设为私密 | P1 |
| B5 | 推荐 / 相似书籍 | P2 |
| B6 | 公众号订阅与文章列表 | P1 |
| B7 | 本地缓存索引（离线可见书架与已下载章节） | P0 |

### 2.3 阅读

| 编号 | 功能 | 优先级 |
|---|---|---|
| C1 | 章节按需拉取 + 解码 + 本地缓存 | P0 |
| C2 | 全书下载（后台队列、进度、断点续传） | P1 |
| C3 | 标准化 Block 渲染（段落/标题/引用/图片/脚注） | P0 |
| C4 | 分页阅读（CSS 多列 + 平移），键盘/滚轮/点击翻页 | P0 |
| C5 | 目录跳转、进度条、章节内百分比 | P0 |
| C6 | 阅读进度云端同步（读时上报 + 进入时拉取） | P0 |
| C7 | 阅读时长上报（真实前台时长，30s 节流） | P1 |
| C8 | 划线 / 想法查看 | P1 |
| C9 | 划线 / 想法写入 | P2 |
| C10 | 阅读统计面板（周/月/年/总） | P2 |
| C11 | 全文搜索（书内） | P2 |

### 2.4 排版与主题

| 编号 | 功能 | 优先级 |
|---|---|---|
| D1 | 字体族（系统字体枚举 + 内置思源宋/黑 + 自定义文件） | P0 |
| D2 | 字号、行高、字距、段距、首行缩进、两端对齐 | P0 |
| D3 | 背景（纯色 / 渐变 / 图片 / 透明）与文字色、不透明度 | P0 |
| D4 | 页边距、版心宽度、分栏（单/双栏） | P1 |
| D5 | 行号（连续行号 / 每页行号 / 关闭） | P1 |
| D6 | 预设主题（纸白、羊皮、夜间、护眼绿、透明） | P0 |
| D7 | 主题导入/导出（JSON） | P2 |

### 2.5 沉浸与窗口

| 编号 | 功能 | 优先级 |
|---|---|---|
| E1 | 禅模式：隐藏全部 UI，仅剩正文，鼠标静止 3s 后隐藏光标 | P0 |
| E2 | 无边框窗口、可拖拽任意空白处、可缩放 | P0 |
| E3 | 置顶（toggle + 全局快捷键） | P0 |
| E4 | 窗口透明度（整体 30%–100%） | P1 |
| E5 | 鼠标穿透（点击穿透到下层窗口） | P1 |
| E6 | 记住位置/尺寸/置顶/主题 | P0 |
| E7 | 全局快捷键（可配置） | P1 |
| E8 | 窗口嵌入（SetParent 到指定窗口，实验特性） | P3 |
| E9 | 托盘图标 + 最小化到托盘 | P1 |

### 2.6 伪装

| 编号 | 功能 | 优先级 |
|---|---|---|
| F1 | 打字模式：任意按键 → 逐字显示正文（按到哪里显示哪里） | P0 |
| F2 | Word 皮肤：Ribbon、标尺、A4 纸面、状态栏、光标 | P1 |
| F3 | VSCode 皮肤：活动栏、资源管理器、标签页、行号、状态栏、小地图 | P1 |
| F4 | 皮肤内沉浸：皮肤装饰在禅模式下淡出 | P1 |
| F5 | 一键紧急隐藏（`Ctrl+Alt+H` 立即最小化/切换为空窗口） | P0 |
| F6 | 窗口标题伪装（可自定义标题与图标） | P1 |

### 2.7 福利（实验）

| 编号 | 功能 | 优先级 |
|---|---|---|
| G1 | 阅读挑战天数/时长展示（只读） | P2 |
| G2 | 每日签到 / 领无限卡（`/act/recvinfinite` 等） | P3（需线上验证） |

---

## 3. 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│ Wails WebView (React + TS)                                  │
│  ├─ 登录页  ├─ 书架  ├─ 阅读器  ├─ 设置  ├─ 伪装皮肤层        │
│  └─ 窗口控制条 / 全局快捷键事件订阅                          │
└──────────────┬──────────────────────────────────────────────┘
               │ Wails Bindings (JSON-RPC)
┌──────────────┴──────────────────────────────────────────────┐
│ internal/app  ── 用例编排（Auth / Shelf / Reader / Sync）    │
├─────────────────────────────────────────────────────────────┤
│ internal/weread                                             │
│  ├─ auth   扫码登录、设备指纹、凭据、令牌续期                │
│  ├─ eink   E-Ink 通道（vid/accessToken）                    │
│  ├─ web    Web 通道（Cookie）：正文/样式/图片/公众号/上报     │
│  ├─ skill  官方 Skill 网关（可选）                           │
│  └─ protocol  _e / sign / shard decode / appId 纯函数        │
├─────────────────────────────────────────────────────────────┤
│ internal/library  SQLite 缓存：书架/目录/章节/进度/设置       │
├─────────────────────────────────────────────────────────────┤
│ internal/content  XHTML → Block 模型（去噪、图片改写、脚注）  │
├─────────────────────────────────────────────────────────────┤
│ internal/platform Win32：置顶/穿透/嵌入/热键/托盘/字体枚举    │
└─────────────────────────────────────────────────────────────┘
```

### 3.1 分层约束

1. `protocol` 只依赖标准库，纯函数，可单测。
2. `eink`/`web`/`skill` 实现同一组接口，业务层不感知通道差异。
3. `app` 层不直接发 HTTP，只调用 `weread` 的领域方法。
4. 前端不解析 XHTML，只消费 `Block[]`；所有样式由 CSS 变量驱动。
5. `platform` 层的 Windows 专有代码用 `//go:build windows` 隔离，其他平台提供 no-op 实现。

### 3.2 通道接口

```go
type MetaSource interface {
    Shelf(ctx context.Context) ([]Book, error)
    Search(ctx context.Context, q string, scope int, offset, count int) (*SearchResult, error)
    BookInfo(ctx context.Context, bookID string) (*BookDetail, error)
    Chapters(ctx context.Context, bookID string) ([]Chapter, error)
    Progress(ctx context.Context, bookID string) (*Progress, error)
    Notes(ctx context.Context, bookID string) ([]Note, error)
}

type ContentSource interface {
    OpenBook(ctx context.Context, bookID string) (*ReaderState, error)   // psvts/pclts/token
    Chapter(ctx context.Context, bookID string, ch ChapterRef) (*ChapterContent, error)
    Images(ctx context.Context, bookID string, ch ChapterRef) (map[string][]byte, error)
    ReportRead(ctx context.Context, r ReadReport) error
}

type AccountSource interface {
    QRLogin(ctx context.Context) (QRCode, error)         // uid + 图片
    PollLogin(ctx context.Context, uid, otp string) (*Credentials, error)
    Renew(ctx context.Context) error
    Logout() error
}
```

---

## 4. 数据模型

### 4.1 Go 领域模型

```go
type Book struct {
    ID        string   `json:"id"`
    Title     string   `json:"title"`
    Author    string   `json:"author"`
    Cover     string   `json:"cover"`
    Category  string   `json:"category"`
    Intro     string   `json:"intro"`
    Rating    float64  `json:"rating"`
    WordCount int      `json:"wordCount"`
    Finished  bool     `json:"finished"`     // 已读完
    Secret    bool     `json:"secret"`
    IsMP      bool     `json:"isMp"`         // 公众号
    LastRead  int64    `json:"lastRead"`
    ChapterN  int      `json:"chapterCount"`
    Cached    bool     `json:"cached"`       // 是否已缓存章节
}

type Chapter struct {
    UID      int64  `json:"uid"`
    Index    int    `json:"index"`
    Title    string `json:"title"`
    Level    int    `json:"level"`
    WordCount int   `json:"wordCount"`
    Paid     bool   `json:"paid"`
    Anchors  []Anchor `json:"anchors,omitempty"`
}

type Block struct {
    Kind  string   `json:"kind"`  // heading|para|quote|image|divider|footnote|pre
    Level int      `json:"level,omitempty"`
    Text  string   `json:"text,omitempty"`
    Src   string   `json:"src,omitempty"`   // 本地媒体 URL
    Alt   string   `json:"alt,omitempty"`
    Class string   `json:"class,omitempty"` // 原始 class 提示（斜体/加粗/居中）
}

type ChapterContent struct {
    BookID   string  `json:"bookId"`
    ChapterUID int64 `json:"chapterUid"`
    Title    string  `json:"title"`
    Blocks   []Block `json:"blocks"`
    Format   string  `json:"format"` // epub|txt|mp
    FetchedAt int64  `json:"fetchedAt"`
}
```

### 4.2 SQLite 表

```sql
CREATE TABLE account (
  alias TEXT PRIMARY KEY, vid TEXT, name TEXT, avatar TEXT,
  device_id TEXT, fingerprint TEXT,
  access_token BLOB, refresh_token BLOB,
  cookies BLOB, api_key BLOB, updated_at INTEGER
);

CREATE TABLE book (
  id TEXT PRIMARY KEY, title TEXT, author TEXT, cover TEXT, category TEXT,
  intro TEXT, rating REAL, word_count INTEGER, finished INTEGER, secret INTEGER,
  is_mp INTEGER, last_read INTEGER, shelf_order INTEGER, raw_json TEXT,
  updated_at INTEGER
);

CREATE TABLE chapter (
  book_id TEXT, uid INTEGER, idx INTEGER, title TEXT, level INTEGER,
  word_count INTEGER, paid INTEGER, tar TEXT, format TEXT,
  PRIMARY KEY (book_id, uid)
);

CREATE TABLE chapter_content (
  book_id TEXT, uid INTEGER, title TEXT, blocks_json TEXT,
  format TEXT, fetched_at INTEGER,
  PRIMARY KEY (book_id, uid)
);

CREATE TABLE progress (
  book_id TEXT PRIMARY KEY, chapter_uid INTEGER, chapter_idx INTEGER,
  offset INTEGER, percent REAL, updated_at INTEGER, dirty INTEGER
);

CREATE TABLE setting (key TEXT PRIMARY KEY, value TEXT);
CREATE TABLE media (book_id TEXT, name TEXT, mime TEXT, bytes BLOB,
                    PRIMARY KEY (book_id, name));
```

凭据字段用 DPAPI（Windows）或 AES-GCM + 本地密钥加密。

---

## 5. UI 结构与交互

### 5.1 页面树

```
App
├─ BootGate            首启声明 / 加载
├─ LoginView           扫码登录（二维码 / 验证码 / 状态机）
├─ Shell
│  ├─ TitleBar          自定义无边框标题栏（- 置顶 / 禅 / 设置 / 最小化 / 关闭）
│  ├─ Sidebar           书架 | 搜索 | 公众号 | 统计 | 设置
│  ├─ ContentRouter
│  │  ├─ ShelfView      网格/列表切换、分组、筛选
│  │  ├─ SearchView     搜索框 + 结果
│  │  ├─ BookDetailView 详情 + 目录 + 开始阅读/下载
│  │  ├─ ReaderView     阅读器（核心）
│  │  ├─ MPView         公众号文章列表
│  │  ├─ StatsView      阅读统计
│  │  └─ SettingsView   外观/窗口/快捷键/账号/高级
│  └─ StatusBar         进度、缓存、同步状态、通道指示
└─ SkinLayer            伪装皮肤（Word / VSCode）覆盖层
```

### 5.2 阅读器布局

```
┌───────────────────────────────────────────────────────────┐
│ ReaderTopBar   ← 章节名 · 进度 · 目录/主题/窗口按钮        │  可自动隐藏
├───────────────────────────────────────────────────────────┤
│ ┌─ gutter ─┬───────────────────────────────────────────┐  │
│ │  001     │  ╭─ 正文页面（多列分页） ─╮                │  │
│ │  002     │  │  　　第一章               │              │  │
│ │  003     │  │  　　正文段落……           │              │  │
│ │  ...     │  ╰──────────────────────────╯              │  │
│ └──────────┴───────────────────────────────────────────┘  │
├───────────────────────────────────────────────────────────┤
│ ReaderBottomBar  进度条 · 百分比 · 上一章/下一章 · 计时     │  可自动隐藏
└───────────────────────────────────────────────────────────┘
```

**分页实现**：正文容器 `columns: <pageWidth>px; column-gap: <gap>px; height: <viewportH>`，
平移 `translateX(-page * (pageWidth + gap))`。总页数 `ceil(scrollWidth / (pageWidth+gap))`。
翻页动画 120ms `transform` 过渡（可在设置中关闭）。

**行号**：左侧独立 gutter，行数 = `floor(viewportH / lineHeight)`，页码 `p` 显示
`p*linesPerPage + 1 .. (p+1)*linesPerPage`。段落间距会带来 ±2 行误差，属可接受装饰。

### 5.3 皮肤层

皮肤层是**渲染器的另一种外观**，不是独立渲染管线：同一份 `Block[]` 由不同 CSS 变量与装饰组件呈现。

| 元素 | Word 皮肤 | VSCode 皮肤 |
|---|---|---|
| 顶部 | Ribbon（文件/开始/插入/布局/审阅/视图）+ 标题栏 | 菜单栏 + 标签页 + 面包屑 |
| 左侧 | — | 活动栏（图标）+ 资源管理器（章节目录树） |
| 行号 | 可选页面行号 | 代码行号（必开） |
| 背景 | `#f3f2f1` 工作区 + 白纸 + 阴影 | `#1e1e1e` |
| 字体 | 宋体/等线 | 等宽（Cascadia/Consolas） |
| 状态栏 | 字数 / 页码 / 缩放 / 语言 | 分支 / 错误 / 行列 / UTF-8 / 语言 |
| 光标 | 竖线闪烁 | 块状光标 |

### 5.4 打字模式（核心交互）

- 开关：`Ctrl+Alt+D` 或工具栏。
- 行为：把正文切成「已显示 / 待显示」两段，仅响应用户**按键事件**推进光标一位；
  任意键（含字母、数字、符号、空格）都推进，不校验内容。
- 退格删除一位；回车跳过到下一段；方向键/PageUp/PageDown 仍然翻页（不推进光标）。
- 输入法组合（IME）不推进，避免中文输入法弹出候选框；用 `compositionstart/end` 屏蔽。
- 光标位置以字符索引记录（含标点），持久化到当前章节。
- 视觉：光标为插入符 + 呼吸动画；已输入文字正常色，未输入文字隐藏（按皮肤可设为极淡）。
- 打字音效（可选，默认关闭）。

### 5.5 快捷键

| 快捷键 | 作用 |
|---|---|
| `←` `→` | 上一页 / 下一页 |
| `↑` `↓` | 上一页 / 下一页 |
| `Space` | 下一页（`Shift+Space` 上一页） |
| `PgUp` `PgDn` | 上一页 / 下一页 |
| `Home` `End` | 章首 / 章末 |
| `Ctrl+←` `Ctrl+→` | 上一章 / 下一章 |
| `Ctrl+Alt+Z` | 禅模式 |
| `Ctrl+Alt+T` | 置顶切换（全局） |
| `Ctrl+Alt+D` | 打字模式 |
| `Ctrl+Alt+H` | 紧急隐藏（全局） |
| `Ctrl+Alt+M` | 切换皮肤（无 / Word / VSCode） |
| `Ctrl+,` | 设置 |
| `Esc` | 退出禅 / 关闭弹层 |

全局快捷键用 Win32 `RegisterHotKey` 实现（窗口未聚焦时也生效）；窗口内快捷键由前端处理。

### 5.6 窗口行为

- 无边框 + `WebviewIsTransparent` + `WindowIsTranslucent`。
- 拖拽：标题栏区域 `--wails-draggable:drag`；禅模式下**整窗拖拽**（按住 `Alt` 拖动）。
- 缩放：Win32 `WM_NCHITTEST` 返回边框命中，或使用 8 个自绘把手。
- 鼠标穿透：`WS_EX_TRANSPARENT | WS_EX_LAYERED`，用全局快捷键恢复。
- 置顶：`SetWindowPos(HWND_TOPMOST)`，与 Wails 的 `WindowSetAlwaysOnTop` 保持一致。
- 窗口嵌入（实验）：`SetParent(ourHWND, targetHWND)` + 修改 `WS_CHILD`，可逆。

---

## 6. 关键流程

### 6.1 首次启动

```
BootGate → 无凭据 → LoginView
  → 生成/加载 wr_fp
  → GET /r/weread-skills（带 wr_fp）
  → GET /api/auth/getLoginUid → uid
  → 渲染二维码（uid → 微信扫码串）
  → 轮询 getLoginInfo（同一 uid，长轮询 ~55s，客户端超时 75s，不重试）
  → succeed → 写 cookies + accessToken/refreshToken (+ 可选 apiKey)
  → 拉 /api/userInfo 展示昵称
  → 进入 Shell
```

登录状态机（实测语义，详见 `docs/research/weread-ecosystem-research-2026-10-29.md` 第 8 节）：

| 服务端返回 | 处理 |
|---|---|
| `succeed:true` | 完成登录 |
| `logicCode:""` 或 `LOGIN_TIMEOUT` | **继续轮询同一个 uid**（这是「还没人扫码」，不是二维码过期） |
| `NEED_OTP` / `OTP_NOT_MATCH` / `OTP_EXPIRED` | 请求验证码；**暂停轮询**，每次提交只发一次请求 |
| `LOGIN_CANCEL` / 未知值 / 会话超过 5 分钟 | 停止轮询，等用户手动刷新 |
| 空响应体 / 非 200 | 视为 pending |
| 传输错误 / 非 JSON | 报错并按 3s→6s→12s→20s 退避，4 次后停止轮询 |

硬性约束：

1. **只有用户操作可以申请新二维码**；任何自动路径都不得重新调用 `getLoginUid`（否则变成请求风暴并触发风控）。
2. 同一时刻只允许一个在途轮询请求。
3. `getLoginInfo` 不得做传输层重试；空 `otp` 参数必须省略。

### 6.2 打开一本书

```
点击书 → BookDetailView
  → E-Ink BookInfo + Chapters（写库）
  → E-Ink Progress（对比本地，取较新）
  → ReaderView.Open(bookId, chapterUid)
      → 若 chapter_content 命中且新鲜 → 直接渲染
      → 否则 Web.OpenBook（拿 psvts/pclts/token）
      → 若 psvts 缺失 → 请求 /web/reader/{_e(bookId)}k{_e(chapterUid)}
      → Web.Chapter（探测 epub/txt，拉分片，解码，转 Block）
      → 图片：tar → media 表 → /media/{bookId}/{name}
      → 写 chapter_content
  → 渲染 + 恢复本地 offset
```

### 6.3 阅读中同步

```
每 30s（且窗口可见、最近 60s 内有翻页/按键）:
  → 计算 chapterUid / offset / percent / summary
  → Web.ReportRead（带当前 psvts/pclts/token）
  → 成功：更新本地 progress(dirty=0)
  → 失败(-2012)：Renew → 重试一次；仍失败标记 dirty，下次再试
切换章节 / 关闭窗口时立即上报一次
```

### 6.4 全书下载

```
Queue(book) → Chapters() → 跳过已缓存
  → 逐章 Chapter()（串行，200–500ms 抖动）
  → 失败重试 2 次 → 记录失败章节
  → 完成：book.cached = 1
（UI 显示 n/m + 失败项，可续传）
```

---

## 7. 目录结构

```
my-weread-desk/
├─ wails.json
├─ main.go
├─ app.go                      # Wails App 结构与生命周期
├─ go.mod
├─ Makefile
├─ spec.md
├─ docs/
│  ├─ development-plan-2026-10-29.md
│  └─ research/
├─ internal/
│  ├─ weread/
│  │  ├─ protocol/             # _e / sign / shard / appId（纯函数 + 测试）
│  │  ├─ auth/                 # 二维码登录、凭据、续期
│  │  ├─ eink/                 # E-Ink 通道
│  │  ├─ web/                  # Web 通道
│  │  ├─ types.go
│  │  └─ errors.go
│  ├─ library/                 # SQLite 仓储
│  ├─ content/                 # XHTML → Block
│  ├─ platform/                # win32 / noop
│  ├─ app/                     # 用例：AuthService / ShelfService / ReaderService / WindowService
│  └─ config/                  # 设置项与默认值
├─ frontend/
│  ├─ index.html
│  ├─ src/
│  │  ├─ main.tsx
│  │  ├─ App.tsx
│  │  ├─ bindings/             # 由 wails 生成 + 手写封装
│  │  ├─ state/                # zustand store
│  │  ├─ api/                  # 对 bindings 的类型化封装
│  │  ├─ views/                # Login / Shelf / Search / Detail / Reader / Settings / Stats / MP
│  │  ├─ reader/               # 分页引擎、Block 渲染、行号、打字模式
│  │  ├─ skins/                # word / vscode 皮肤
│  │  ├─ components/
│  │  └─ styles/               # 主题变量、字体、皮肤
│  └─ wailsjs/                 # 自动生成
└─ build/                      # 图标、清单
```

---

## 8. 验收标准

### 8.1 P0

1. 扫码登录成功率高，重启后免登录；`wr_fp` 持久化，不与网页端点互踢。
2. 书架 ≥ 200 本时首屏 < 1.5s（本地缓存优先，后台刷新）。
3. 打开一本书到首屏可读 < 2s（未缓存，单章）；已缓存 < 200ms。
4. 分页不截断文字：任何字号/窗口尺寸下，页内文字完整、无横向滚动条。
5. 键盘 `←/→/↑/↓/Space/PgUp/PgDn` 均可翻页，且不触发页面滚动。
6. 禅模式进入/退出 < 100ms，退出后恢复原 UI 状态。
7. 主题修改即时生效（< 16ms 一帧），并持久化。
8. 打字模式任意按键逐字显示，退格可回退，IME 不中断。
9. 置顶、拖拽、位置记忆在重启后一致。
10. `Ctrl+Alt+H` 立即隐藏窗口（< 200ms）。

### 8.2 P1

11. 全书下载可断点续传，失败章节可单独重试。
12. 阅读时长上报不污染进度（offset 与 UI 一致）。
13. Word/VSCode 皮肤视觉完整（含状态栏、行号、目录树），可一键切换。
14. 行号与分页页码一致（±2 行内）。
15. 托盘常驻，关闭到托盘不退出。

---

## 9. 风险与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| 协议变更（算法/字段） | 正文拉取失败 | protocol 层纯函数 + 单测样例；错误提示指向「协议可能已更新」 |
| Cookie 过期被顶 | 阅读中断 | 自动续期 → 失败引导重新扫码；脏进度本地保留 |
| 大量章节请求触发风控 | 封号 | 串行 + 抖动 + 单本上限 + 明确的手动触发 |
| 透明窗口在部分显卡异常 | 黑底/闪烁 | 设置项可关闭透明，退回不透明背景 |
| 无边框窗口缩放难用 | 体验差 | 提供 8 向把手 + 快捷键缩放 + 最大化按钮 |
| 字体缺失 | 排版跳变 | 内置两套中文字体；缺字回退链 `serif → system-ui` |
| SQLite 并发写 | 锁冲突 | 单写连接 + WAL + 事务批处理 |

---

## 10. 里程碑映射

详细任务拆分见 `docs/development-plan-2026-10-29.md`。概览：

| 里程碑 | 内容 | 状态 |
|---|---|---|
| M0 | 项目骨架、构建脚本、窗口基础 | 待办 |
| M1 | 协议层（`_e`/签名/分片）+ 单测 | 待办 |
| M2 | 扫码登录 + 凭据 + 书架/搜索（E-Ink） | 待办 |
| M3 | 正文拉取 + Block 转换 + 缓存 | 待办 |
| M4 | 阅读器（分页、主题、禅模式、快捷键） | 待办 |
| M5 | 窗口行为（置顶/透明/穿透/热键/托盘） | 待办 |
| M6 | 伪装（打字模式 + Word/VSCode 皮肤） | 待办 |
| M7 | 同步（进度/时长）、下载队列、统计 | 待办 |
| M8 | 打磨、打包、安装器 | 待办 |
