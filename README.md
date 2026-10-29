# my-weread-desk

微信读书的桌面客户端。扫码登录后同步书架、搜索书籍、按章下载，并用**你自己的排版**阅读；进度与阅读时长回写到微信读书。

非官方工具，仅供个人学习研究，与腾讯 / 微信读书无任何关联。只读取账号本就有权阅读的内容，不绕过付费或会员限制。

---

## 能力

**书库**：书架同步（分组 / 读完 / 私密）、书城搜索（电子书 / 全文 / 作者 / 公众号）、书籍详情与目录、加入移出书架、公众号文章阅读、阅读统计。

**阅读**：章节按需拉取与解码（EPUB 分片 / TXT 网文 / 公众号 HTML）、本地缓存与离线阅读、全书下载（串行、断点续传、失败重试）、多列分页翻页、目录跳转、进度同步、阅读时长上报、划线与想法查看。

**排版**：字体族（本机字体枚举）、字号 / 行距 / 字距 / 段距 / 首行缩进、两端对齐、版心宽度、页边距、单双栏、行号、背景 / 纸面 / 字色 / 强调色、6 套预设主题。

**沉浸**：禅模式（隐藏全部界面、鼠标静止隐藏光标）、无边框窗口、Alt 拖拽移动、置顶、整体透明度、鼠标穿透、全局快捷键、窗口位置记忆、紧急隐藏。

**伪装**：打字模式（任意按键逐字显示正文，IME 不打断）、Word 皮肤（Ribbon / 标尺 / A4 纸面 / 状态栏）、VS Code 皮肤（活动栏 / 资源管理器 / 标签页 / 行号 / 状态栏 / 小地图）、窗口标题伪装。

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

## 开发

前置：Go 1.25+、Node 20+、Wails v2 CLI（`go install github.com/wailsapp/wails/v2/cmd/wails@latest`）。

```bash
make dev        # wails dev，热重载
make build      # 产出 build/bin/my-weread-desk.exe
make test       # Go 单元测试
make lint       # gofmt 检查 + go vet + 前端类型检查
```

前端可以脱离 Wails 单独预览（使用内置的假数据后端，不会访问网络）：

```bash
cd frontend && npm install && npm run dev   # http://localhost:5173
```

### 目录结构

```
main.go                     窗口选项、资源路由（/media/*）
internal/
  weread/protocol/          _e 编码、请求签名、分片解码（纯函数 + 单测）
  weread/httpx/             HTTP 客户端、cookie jar、错误信封
  weread/auth/              扫码登录、设备指纹、凭据加密存储
  weread/eink/              E-Ink 通道：书架、搜索、目录、进度、笔记、统计
  weread/web/               Web 通道：正文分片、样式、图片 tar、公众号、时长上报
  weread/session/           凭据与两条通道的会话/续期
  content/                  XHTML → Block 模型
  library/                  SQLite 缓存
  platform/                 Win32：置顶、穿透、嵌入、全局热键、字体枚举
  app/                      Wails 绑定层（认证、书库、阅读、下载、上报）
frontend/src/
  views/                    Login / Shelf / Search / Detail / Reader / Settings / Stats / MP
  reader/                   分页引擎、Block 渲染、主题面板
  skins/                    Word / VS Code 伪装
  styles/                   设计变量、应用外壳、阅读器、皮肤
```

### 数据位置

`%AppData%\my-weread-desk\`

- `settings.json` 设置与主题
- `library.db` 书架 / 目录 / 章节 / 图片 / 进度缓存
- `account/credentials.bin` 凭据（Windows DPAPI 加密），`device.id`、`fingerprint`

---

## 协议要点

两条通道互补，登录一次同时获得：

1. **E-Ink 通道**（`i.weread.qq.com`，`vid` + `accessToken` 头）：书架、搜索、详情、目录、进度、笔记、统计。无签名。
2. **Web 通道**（`weread.qq.com`，Cookie）：正文分片、CSS、图片 tar、公众号、阅读时长上报。需要 `_e` 编码与请求签名。

扫码登录的实测语义（容易踩坑，已在 `internal/weread/auth/qrlogin_test.go` 固定）：

- `getLoginInfo` 不带 `otp` 时服务端挂起约 55s，然后返回 `LOGIN_TIMEOUT` —— 这是**「还没人扫码」**，必须用同一个 uid 继续轮询，**不是**二维码过期。
- 空 `otp=` 会被立即拒绝，必须省略该参数。
- 客户端超时必须大于 55s（本项目 75s），且该请求**不得重试**。
- **只有用户操作才能申请新二维码**；任何自动路径都不得重新调用 `getLoginUid`，否则会变成请求风暴并触发风控。
- 连续请求失败按 3s → 6s → 12s → 20s 退避，4 次后停止轮询，等待用户手动刷新。

完整协议调研见 `docs/research/`，产品与技术规格见 `spec.md`，开发计划见 `docs/development-plan-2026-10-29.md`。

---

## 风险与合规

- 账号风控：默认低频、串行、带抖动；正文按章按需拉取；不内置任何挂机或刷时长功能。
- 登录互踢：设备指纹 `wr_fp` 每台机器独立生成并持久化，退出登录不清除。
- 协议变更：`internal/weread/protocol` 为纯函数并有单测；失败时提示「接口格式已变化」而非静默错误。
- 使用本工具产生的账号风险由使用者自行承担。
