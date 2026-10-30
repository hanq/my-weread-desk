# my-weread-desk

微信读书的桌面客户端。扫码登录后同步书架、搜索书籍、按章下载，并用**你自己的排版**阅读；阅读进度与阅读时长会回写到微信读书。

> 非官方工具，仅供个人学习研究，与腾讯 / 微信读书无任何关联。
> 只读取账号本来就有权阅读的内容，不绕过付费或会员限制。
> 使用本工具产生的账号风险由使用者自行承担。

---

## 功能

**书库**：书架同步（在读 / 读完 / 私密 / 公众号）、书城搜索（电子书 / 全文 / 作者 / 公众号）、书籍详情与目录、加入 / 移出书架、公众号文章阅读、阅读统计。

**阅读**：章节按需拉取与解码（EPUB 分片 / TXT 网文 / 公众号 HTML）、本地缓存与离线阅读、全书下载（串行、断点续传、失败重试）、多栏分页翻页、目录跳转、进度同步、阅读时长上报、划线查看。

**排版**：字体族（本机字体枚举）、字号 / 行距 / 字距 / 段距 / 首行缩进、两端对齐、版心宽度、页边距、单双栏、行号、背景 / 纸面 / 字色 / 强调色、6 套预设主题。

**界面**：响应式布局——宽屏完整侧栏，中屏收起为图标栏，窄屏改为标题栏菜单抽屉；书架筛选自动换行，书封尺寸随窗口列宽缩放。

**沉浸**：禅模式（隐藏全部界面、鼠标静止隐藏光标）、无边框窗口、窗体圆角（0–40px，CSS 抗锯齿，随 DPI 缩放）、Alt 拖拽移动、置顶、整体透明度、鼠标穿透、全局快捷键、窗口位置记忆、紧急隐藏。

**伪装**：独立的“伪装”设置分类；打字模式（任意按键逐字显示正文，IME 不打断）；Word / VS Code 文档皮肤；微信 / 钉钉 / 飞书 / 企业微信 IM 聊天皮肤（正文按段落变成聊天消息，输入框预览下一段，回车或“发送”逐段推进）；任务栏与 Alt+Tab 图标随皮肤切换；老板键（Ctrl+Alt+H 隐藏 / 恢复）；可隐藏任务栏按钮；透明无边距悬浮窗口，鼠标进入 A 区显示、进入 B 区隐藏，滚轮 / 左右点击区 / 右键翻页；窗口标题随皮肤切换（纯净 / Word / VS Code / 微信 / 钉钉 / 飞书 / 企业微信）。

---

## 快速开始

环境要求：

- **Go 1.20+**（`go.mod` 固定 `go 1.20`）
- **Node.js 20+**
- **Wails v2.7.1 CLI**：`go install github.com/wailsapp/wails/v2/cmd/wails@v2.7.1`
- Windows 10/11 + WebView2 Runtime

```bash
make dev        # wails dev，热重载
make build      # 产出 build/bin/my-weread-desk.exe
make test       # Go 单元测试
make lint       # gofmt 检查 + go vet + 前端类型检查
```

> 本机 `go` 为 1.20 时可直接构建。CLI v2.14 也能构建 v2.7.1 模块（会提示版本不一致），
> 建议安装匹配的 CLI：`go install github.com/wailsapp/wails/v2/cmd/wails@v2.7.1`。

前端可脱离 Wails 单独预览（使用内置假数据后端，不访问网络）：

```bash
cd frontend && npm install && npm run dev   # http://localhost:5173
```

### 登录

首次启动会展示微信二维码：

1. 用**微信**扫码（不是微信读书 App），在手机上确认授权；
2. 客户端用一次性 `wx_code` 换取 E-Ink 通道令牌；
3. 成功后自动同步书架。

登录状态、书架、章节内容都只保存在本机。

### 数据位置

`%AppData%\my-weread-desk\`

- `settings.json`：设置与主题
- `library.db`：书架 / 目录 / 章节 / 图片 / 进度缓存
- `account/credentials.bin`：凭据（Windows DPAPI 加密）
- `account/device.id`、`fingerprint`、`install.id`：设备身份（退出登录不清除）

---

## 目录结构

```
main.go                     窗口选项、资源路由（/media/*）
internal/
  weread/protocol/          _e 编码、请求签名、分片解码（纯函数 + 金标测试）
  weread/httpx/             HTTP 客户端、cookie jar、错误信封
  weread/device/            E-Ink 设备身份（UA / 版本头 / 设备 ID 前缀）
  weread/auth/              微信扫码登录、设备指纹、凭据加密存储
  weread/eink/              E-Ink 通道：书架、搜索、详情、目录、进度、笔记、统计
  weread/web/               Web 通道：正文分片、样式、图片 tar、公众号、搜索/详情兜底
  weread/session/           凭据与两条通道的会话 / 续期
  content/                  XHTML → Block 模型
  library/                  SQLite 缓存
  platform/                 Win32：置顶、穿透、透明度、嵌入、全局热键、字体枚举、窗口几何恢复
  app/                      Wails 绑定层（认证、书架、阅读、下载、上报）
frontend/src/
  views/                    Login / Shelf / Search / Detail / Reader / Settings / Stats / MP
  reader/                   分页引擎、Block 渲染、主题面板
  skins/                    Word / VS Code / 微信 / 钉钉 / 飞书 / 企业微信伪装
  styles/                   设计变量、应用外壳、阅读器、皮肤
build/                      Wails 打包资源（appicon.png、windows/icon.ico）
docs/research/              协议调研资料（客户端 TS 参考实现）
```

---

## 架构与协议

两条通道互补，登录一次同时获得：

1. **E-Ink 通道**（`i.weread.qq.com`，`vid` + `accessToken` 头）：书架、搜索、详情、目录、进度、笔记、统计。无签名。
2. **Web 通道**（`weread.qq.com`，Cookie）：正文分片、CSS、图片 tar、公众号、阅读时长上报。需要 `_e` 编码与请求签名。

### 登录必须走 E-Ink 微信 SDK 流程

```
GET  i.weread.qq.com/wxticket?nonceStr=weread        → signature + timeStamp
GET  open.weixin.qq.com/connect/sdk/qrconnect        → uuid + 二维码图
GET  long.open.weixin.qq.com/connect/l/qrconnect     → 408 未扫 / 404 已扫 / 405 已确认
POST i.weread.qq.com/login  { code, deviceId, ... }  → accessToken + refreshToken
```

**不能复用网页端登录**（`weread.qq.com/api/auth/getLoginInfo`）：它返回的是网页会话，
accessToken 请求 API 会得到 `-2012`，`web@...` 的 refreshToken 请求 `/login` 会得到 `-2013`。

实测语义（已在 `internal/weread/auth/qrlogin_test.go` 固化）：

- `wx_errcode=408` 是长轮询的「还没人扫码」，必须继续轮询同一个 uuid，**不是**二维码过期；
- `404` 已扫码待确认，`405` 已确认并带一次性 `wx_code`，`402` 过期，`403` 取消；
- 长轮询不得做传输层重试，客户端超时需大于服务端挂起时间；
- **只有用户操作才能申请新二维码**，自动路径不得重新调用 `wxticket` / `qrconnect`；
- 登录与续期必须使用同一套设备身份（deviceId / deviceName / deviceType / 版本头）。

### 网页通道兜底

旧版网页登录留下的凭据（`wr_skey` + `web@...`）无法认证 E-Ink 接口。此时以下能力会自动切到网页通道：

| 能力 | 兜底接口 |
|---|---|
| 书架同步 | `GET /web/shelf/sync` |
| 书城搜索 | `GET /web/search/global` |
| 书籍详情 | `GET /web/book/info` |
| 目录 | `POST /web/book/chapterInfos` |
| 阅读进度 | `GET /web/book/getProgress` |
| 阅读统计 | `GET /readdata/detail` |
| 划线 | `GET /web/book/bookmarklist` |
| 正文 / 图片 / 公众号 | Web 通道（本来就走） |

图片优先取目录里的 `tar` 资源包；资源包缺失、下载失败或其中没有对应图片时，正文里的绝对图片地址会通过 `/media-proxy` 用同一网页会话回源，并缓存到 `media` 表。`app.log` 会记录 `章节资源包`、`章节图片包解析`、`章节图片未命中`、`图片代理失败`。

**需要重新扫码才能使用**：书架写操作（加入 / 移出 / 标记读完 / 私密）、公众号文章列表、全书下载的目录引导。

---

## 快捷键

| 快捷键 | 作用 |
|---|---|
| `←` `→` `↑` `↓` | 上一页 / 下一页 |
| `Space` / `Shift+Space` | 下一页 / 上一页 |
| `PgUp` `PgDn` `Home` `End` | 翻页与跳转 |
| `Ctrl+←` `Ctrl+→` | 上一章 / 下一章 |
| `Esc` | 关闭面板 / 退出禅模式 |
| `Ctrl+Alt+Z` | 禅模式（全局） |
| `Ctrl+Alt+T` | 窗口置顶（全局） |
| `Ctrl+Alt+D` | 打字模式（全局） |
| `Ctrl+Alt+M` | 循环切换皮肤（全局） |
| `Ctrl+Alt+C` | 鼠标穿透开关（全局） |
| `Ctrl+Alt+H` | 紧急隐藏窗口（全局） |
| `Alt` + 拖拽 | 任意位置移动窗口 |

---

## 已知限制

- **托盘未实现**：`closeToTray` 设置存在，但关窗仍会直接退出。
- **皮肤禅模式**：Word/VSCode 的装饰层在禅模式下不会淡出。
- **划线只读**：可以查看云端划线，尚不支持新建 / 删除划线、想法和书签。
- **旧凭据的能力缺口**：写操作 / 公众号列表需重新扫码；统计已走网页通道，若接口拒绝仍需重新扫码。
- **全书下载的目录引导**仍走 E-Ink；旧凭据下会失败（正常从详情页进入不受影响）。
- **主题导入 / 导出**未实现（仅内置主题与自定义调整）。
- **日志**只发到前端控制台，未落盘。

---

## 风险与合规

- 账号风控：默认低频、串行、带抖动；正文按章按需拉取；不内置任何挂机或刷时长功能。
- 登录互踢：设备指纹 `wr_fp` 每台机器独立生成并持久化，退出登录不清除。
- 协议变更：`internal/weread/protocol` 为纯函数并有金标测试；失败时提示「接口格式已变化」而非静默错误。
- 凭据安全：仅保存在本机，Windows 下使用 DPAPI 加密，不上传任何服务器。

协议调研资料见 `docs/research/`，产品与技术规格见 `spec.md`。
