# my-weread-desk 开发计划（2026-10-29）

> 配套文档：`spec.md`、`docs/research/weread-ecosystem-research-2026-10-29.md`
> 原则：每个里程碑结束时应用**可运行**；协议层先写单测再写调用方；前端先跑通再抛光。

---

## 里程碑总览

| # | 里程碑 | 交付物 | 验收 |
|---|---|---|---|
| M0 | 骨架 | Wails 工程可 `wails dev` 启动，无边框透明窗口 | 窗口能开、能拖、能关 |
| M1 | 协议内核 | `internal/weread/protocol` + 单测 | 与 Python 参考实现逐字节一致 |
| M2 | 登录与书库 | 扫码登录、书架、搜索、详情 | 真实账号可登录并看到书架 |
| M3 | 正文 | 章节拉取、解码、Block 转换、缓存 | 打开一本书可读 |
| M4 | 阅读器 | 分页、主题、禅模式、快捷键 | spec 8.1 的 4/5/6/7 通过 |
| M5 | 窗口 | 置顶、透明、穿透、热键、托盘 | spec 8.1 的 8/9/10 通过 |
| M6 | 伪装 | 打字模式、Word/VSCode 皮肤 | 皮肤可切换，打字逐字显示 |
| M7 | 同步与批量 | 进度/时长上报、下载队列、统计 | spec 8.2 的 11/12 通过 |
| M8 | 发布 | 打包、图标、安装器、首启声明 | 生成可分发 exe |

---

## M0 · 骨架（半天）

- [x] `wails init -t react-ts` 初始化，`wails.json` 配置
- [x] 目录分层：`internal/{weread,library,content,platform,app,config}`
- [x] 无边框 + 透明窗口配置；自定义标题栏与拖拽区
- [x] 开发脚本：`make dev` / `make build`（Windows 用 `task`/`make` 均可）
- [x] `.gitignore`（`node_modules`、`build/bin`、`*.db`、凭据目录）
- [x] 首启免责声明弹层

## M1 · 协议内核（1 天）

- [x] `protocol.E()`：`_e` 编码，含数字/非数字分支与补长
- [x] `protocol.Sign()`：排序查询 + 自定义哈希
- [x] `protocol.EncodeQuery()`：JS 布尔拼写、`encodeURIComponent` 语义
- [x] `protocol.DecodeShards()`：MD5 校验、去首字符、swap 还原、base64url、UTF-8 修复
- [x] `protocol.WebAppID()`、`protocol.ReadPayload()`、`protocol.ContentParams()`
- [x] `protocol.SHA256Hex`、`protocol.MD5Hex`
- [x] 单测：用调研文档中的样例向量断言
      `_e("43208843") = c9c321c07293508bc9c79df`、`_e("2") = c81322c012c81e728d9d180`、
      `_e("119") = 07e323f027707e1cd7dc674`
- [x] 黄金样本：一段真实分片（可 mock）解码后 XHTML 首尾一致

## M2 · 登录与书库（1.5 天）

- [x] 设备指纹生成（4 随机字节 → 无符号十进制）与持久化
- [x] `auth.QRLogin`：`/r/weread-skills` → `getLoginUid` → 二维码 → `getLoginInfo` 轮询
- [x] 二维码渲染：后端生成 PNG data URL，前端直接展示
- [x] 4 位验证码输入状态机
- [x] **登录协议实测修正（2026-10-29）**：`LOGIN_TIMEOUT` 实为「长轮询窗口结束、尚无人扫码」而非二维码过期；
      空 `otp=` 必须省略；客户端超时 75s > 服务端 ~55s 挂起且不重试；
      **禁止任何自动路径重新申请二维码**，改为单飞轮询 + 指数退避 + 4 次失败即停止；
      桩服务器回归测试见 `internal/weread/auth/qrlogin_test.go`
- [x] 凭据落盘（Windows DPAPI / 其他平台 AES-GCM+本地 key）
- [x] `eink.Client`：统一请求头、`-2012` 处理、令牌续期
- [x] `/login` 令牌续期（sha256 签名）
- [x] 书架 `/shelf/sync` + 分页；搜索 `/store/search`；详情 `/book/info`、`/book/chapterInfos`
- [x] 仓库层：`book` / `chapter` 表写入与查询
- [x] 前端：LoginView（二维码 + 状态）、ShelfView（网格、分组、右键菜单）、SearchView

## M3 · 正文（1.5 天）

- [x] `web.Client`：Cookie 注入、`/web/login/renewal`、`Referer`/`Origin`
- [x] `web.ReaderState`：抓 `/web/reader/{hash}` 解析 `__INITIAL_STATE__`
- [x] 格式探测：`e_0` 首字符判定 epub / txt
- [x] 正文拉取：`e_0/e_1/e_3`、`e_2`(st=1)、`t_0/t_1`
- [x] `tar` 图片下载 → `media` 表 → `/media/{bookId}/{name}` 资源路由
- [x] `content.Parse`：XHTML → `Block[]`（标题/段落/引用/图片/脚注/分页符）
- [x] TXT → 段落块
- [x] 公众号：`/web/mp/articles` + `/web/mp/content` → Block
- [x] 缓存：`chapter_content` 表，含 `fetched_at` 与失效策略
- [x] 前端：BookDetailView（目录、开始阅读、下载）

## M4 · 阅读器（2 天）

- [x] 分页引擎：CSS columns + `translateX`，窗口 resize 重算
- [x] 页码、章节进度、全书进度
- [x] 键盘：方向键/空格/PgUp/PgDn/Home/End/Ctrl+方向
- [x] 鼠标：点击左右 1/3 翻页、滚轮翻页、拖拽选中不误翻
- [x] 主题系统：CSS 变量 + 预设 + 自定义（字体/字号/行高/字距/段距/背景/字色/透明度）
- [x] 字体枚举（Win32 `EnumFontFamiliesEx`，其他平台回退）与内置字体
- [x] 行号 gutter
- [x] 禅模式（UI 淡出 + 光标隐藏 + 整窗拖拽）
- [x] 阅读进度持久化 + 恢复定位
- [x] 目录抽屉、进度条跳转

## M5 · 窗口（1 天）

- [x] 无边框标题栏、8 向缩放把手
- [x] 置顶（应用内 + 全局快捷键）
- [x] 整体透明度（`SetLayeredWindowAttributes` 或 WebView 背景 alpha）
- [x] 鼠标穿透（`WS_EX_TRANSPARENT`）+ 恢复热键
- [x] 全局热键注册（`RegisterHotKey` + 消息循环）
- [x] 托盘图标（`systray`）+ 关闭到托盘
- [x] 位置/尺寸记忆
- [x] 紧急隐藏
- [ ] 窗口嵌入（`SetParent`，实验，标记 v1.1）

## M6 · 伪装（1.5 天）

- [x] 打字模式状态机（字符索引、退格、翻页不受影响、IME 屏蔽）
- [x] 打字光标 + 已/未显示样式
- [x] Word 皮肤（Ribbon、标尺、白纸、状态栏）
- [x] VSCode 皮肤（活动栏、资源管理器、标签页、行号、状态栏）
- [x] 皮肤切换与持久化
- [x] 窗口标题/图标伪装
- [x] 皮肤 × 主题兼容（皮肤内仍可用禅模式）

## M7 · 同步与批量（1.5 天）

- [x] `/web/book/read` 上报：payload 组装、30s 节流、可见性判断
- [x] `-2012` → 续期 → 重试一次；失败标记 dirty
- [x] 进入书籍时拉取云端进度并与本地比较
- [x] 下载队列（串行、抖动、进度事件、失败重试、断点续传）
- [x] 阅读统计视图（`/readdata/detail`）
- [x] 划线/想法查看
- [ ] 划线/想法写入（P2）
- [ ] 签到/领奖（P3，实验开关）

## M8 · 发布（0.5 天）

- [ ] 应用图标（iconfont 风格线性 SVG → ico）
- [ ] `wails build -platform windows/amd64 -nsis`
- [ ] 版本信息、关于页、更新检查（可选）
- [ ] 文档：README、使用说明、FAQ（被顶号/风控/协议变更）

---

## 依赖与工作量

| 依赖 | 用途 | 备注 |
|---|---|---|
| `modernc.org/sqlite` | 纯 Go SQLite | 避免 cgo 交叉编译问题 |
| `golang.org/x/net/html` | XHTML 解析 | 宽容解析器 |
| `golang.org/x/sys/windows` | Win32 调用 | 置顶/穿透/热键/字体 |
| `golang.org/x/text` | 编码与 UTF-8 修复辅助 | 可选 |
| `github.com/skip2/go-qrcode` | 后端出二维码图 | 或前端 `qrcode` 二选一 |
| `github.com/getlantern/systray` | 托盘 | Wails 无内置托盘 |

前端：`react`、`react-dom`、`vite`、`typescript`、`zustand`、`qrcode`（或后端出图）、无 UI 框架（自绘皮肤需要完全控制）。

预计总量：**约 10 个工作日**（单人），M0–M4 占 60%。

---

## 实施顺序建议

1. **先协议后 UI**：M1 的单测是后续一切的地基，样例向量错一位就全线失败。
2. **先真机后 mock**：M2 完成当天用真实账号验证一次登录与书架，避免后期返工。
3. **渲染层与协议层解耦**：M3 输出的 `Block[]` 一旦定稿，M4/M6 可并行开发。
4. **窗口能力放 M5 而非最后**：无边框/透明会影响布局尺寸计算，早做早稳定。
5. **伪装放 M6**：它依赖 M4 的渲染管线和 M5 的窗口能力，前置会重复劳动。

---

## 已知待验证项

| 项 | 风险 | 验证方式 |
|---|---|---|
| `getLoginInfo` 长轮询语义 | 误判为二维码过期 → 自动重发二维码 → 风控 | **已实测修正**（2026-10-29），见第 8 节调研结论 |
| `e_2` 样式分片是否所有书都有 | 部分书无 CSS | 探测失败时使用内置默认 CSS |
| TXT 书的图片 | TXT 无 `tar` | 忽略，正文纯文本 |
| 公众号文章正文提取选择器 | DOM 变动 | 多选择器回退 + 提取失败保留原文 |
| `/act/recvinfinite` 签到语义 | 可能已下线 | 实验开关 + 只读探测，不默认开启 |
| 透明度 + 点击穿透组合 | Windows 上 `WS_EX_LAYERED` 与 WebView2 冲突 | 形态 A（仅透明）/ 形态 B（穿透）分开测试 |
