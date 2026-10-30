# AGENTS.md（项目级）

> 本文件是 my-weread-desk 的项目级约定，叠加在全局 `~/.pi/agent/AGENTS.md` 之上。
> 通用规则（删除安全、图标、命名、注释、提交身份）仍然适用，这里只补充本项目特有的约束。

## 项目概览

微信读书桌面客户端（Wails v2.7.1 + Go 1.20 + React 19 / Vite 7 / TypeScript）。

- 后端：`internal/`，Wails 绑定层在 `internal/app`，协议实现分 `eink` / `web` 两条通道。
- 前端：`frontend/src`，视图在 `views/`，分页引擎在 `reader/`，伪装皮肤在 `skins/`。
- 打包：`build/`（`appicon.png`、`windows/icon.ico`），产物在 `build/bin`（已 gitignore）。
- 协议调研资料：`docs/research/`；产品规格：`spec.md`。

## 构建与测试

```bash
make dev        # wails dev
make build      # wails build → build/bin/my-weread-desk.exe
make test       # go test ./...
make lint       # gofmt + go vet + tsc --noEmit
```

### Go 工具链注意

`go.mod` 要求 **go 1.20**。Wails 固定在 **v2.7.1**（v2.9+ 要求 Go 1.21，v2.14 要求 1.25），
SQLite 驱动为 v1.30.1，x/net 与 x/sys 也锁定在同期版本。升级其中任何一个都要同步提高
`go` 指令，否则 Go 1.20 无法构建。

本机 PATH 中的 `go` 为 1.20 时可以直接构建：

```bash
go build ./...
go test ./internal/...
```

`wails build` 会调用 PATH 里的 `go`。实测 CLI v2.14 仍能构建 v2.7.1 模块，但会打印版本不一致警告；
要消除警告并保证绑定模板一致，可安装匹配的 CLI：

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.7.1
```

`go vet` 在 `internal/platform/fonts_windows.go` 有一条既有的 `unsafe.Pointer` 警告，属于已知问题，不要顺手改掉。

## 架构红线

### 1. 登录必须走 E-Ink 微信 SDK 流程

```
wxticket → open.weixin.qq.com/connect/sdk/qrconnect
        → long.open.weixin.qq.com/connect/l/qrconnect
        → POST i.weread.qq.com/login { code }
```

**禁止**改回网页端登录（`weread.qq.com/api/auth/getLoginInfo`）：网页登录拿到的是 `wr_skey` +
`web@...` refresh token，API 会返回 `-2012`，`/login` 会返回 `-2013`。
相关逻辑与状态机在 `internal/weread/auth/qrlogin.go`，测试在 `qrlogin_test.go`，改动必须保持测试通过。

### 2. 设备身份必须统一

登录、续期、请求头都使用 `internal/weread/device` 里的同一套 UA / 版本头 / deviceId / deviceName / deviceType。
身份不一致会让服务端把令牌判为盗用（同样是 `-2013`）。

### 3. 窗口位置不要直接用 Wails 的 WindowSetPosition

Wails v2 的 `WindowSetPosition(x, y)` 在 Windows 上是**相对当前显示器工作区原点**的偏移，
而 `WindowGetPosition()` 返回的是**绝对虚拟桌面坐标**。多屏下直接混用会把窗口推到屏幕外。

恢复窗口几何必须走 `platform.RestoreBounds`（绝对坐标 + 多屏校验 + 越界回主屏居中），
相关代码在 `internal/platform/geometry.go`、`window_windows.go`，入口在 `internal/app/api.go`。
`SaveSettings` / `UpdateSetting` 只允许应用窗口标志（置顶 / 透明度 / 穿透 / 热键），**不得重设几何**。

### 4. 服务端字段解析一律用 Flex 类型

微信读书的字段类型不稳定（例如 `reader.pclts` 会返回数字）。解析响应时使用
`weread.FlexString` / `FlexInt` / `FlexFloat`，不要写死 `string` / `int`，否则会报
「接口格式已变化」。`parseInitialState` 是典型例子。

### 5. 网页兜底矩阵

旧版网页凭据下，E-Ink 接口全部不可用。以下能力必须保留网页通道兜底（`internal/weread/web`）：

| 能力 | 方法 |
|---|---|
| 书架 | `web.Shelf` |
| 搜索 | `web.Search` |
| 详情 | `web.BookInfo` |
| 目录 | `web.Catalog` |
| 进度 | `web.Progress` |
| 统计 | `web.Stats` |
| 划线 | `web.Notes` |

兜底放在 `internal/app` 层（先 E-Ink，失败再 Web），不要在 `web` 包里反向调用 E-Ink。
已知仍只走 E-Ink、需要重新扫码的能力：书架写操作、公众号列表、全书下载的目录引导。
统计已通过 `web.Stats` 调用 `/readdata/detail` 走网页通道；若网页接口也拒绝，再引导重新扫码。
章节图片优先取 `tar`；资源包缺失/失败时通过 `/media-proxy` 按原图地址走同一 Cookie 会话回源，只允许 `*.weread.qq.com` 与 `mmbiz.qpic.cn`，并缓存到 `media`。

### 6. 图标与品牌资源

- 界面功能图标禁止 emoji，使用线性 SVG（`stroke="currentColor"`，24×24，`stroke-width=2`）或 iconfont / iconpark。
- 应用图标：`build/appicon.png`（1024×1024）、`build/windows/icon.ico`（16–256 多尺寸）、
  `frontend/public/favicon.png`、`frontend/public/weread-icon.png`。
- 应用图标必须带圆角：圆角半径约为边长的 22.37%，四角在图标外缘处为完全透明（alpha=0，
  由 4× 超采样遮罩做抗锯齿），不要用四角不透明的整块方形图。
  四个位置需保持同一套圆角遮罩：`appicon.png` 1024 基准，其余尺寸由它等比缩放并单独重新套遮罩。
- 更换图标后需要重新 `wails build` 才会嵌入 exe。

### 7. 命名与注释

- 新建文件名用 snake_case（`device_config.go`、`net_remote.go`），既有文件保持原名。
- 代码注释必须自包含，禁止引用 `spec.md`、`docs/...` 等文档路径；约束和原因直接写在注释里。
- Go 代码保持 `gofmt` 干净；前端保持 `tsc --noEmit` 通过。

### 8. 敏感数据

凭据、设备指纹、SQLite 缓存、`settings.json` 一律不提交（见 `.gitignore`）。
新增本地文件时确认已被忽略，且不要写入任何日志。

### 9. 提交身份

推送到 GitHub 等公共平台时，作者/提交者统一为：

- `user.name = hanq`
- `user.email = hanq@live.cn`

仓库级配置：`git config --local user.name hanq && git config --local user.email hanq@live.cn`。

### 10. 伪装皮肤与任务栏图标

前端皮肤统一由 `frontend/src/skins/registry.ts` 的 `SKIN_OPTIONS` 注册
（`none` / `word` / `vscode` / `wechat` / `dingtalk` / `feishu` / `wecom`）。新增皮肤必须同时：

1. 在 `registry.ts` 增加条目；
2. 在 `internal/app/api.go` 的 `SetSkin` 白名单、`internal/app/disguise.go` 的
   `skinTitles` / `skinIcons` 中登记；
3. 提供 `internal/app/icons/<skin>.ico`。启动与切换皮肤时 `applyDisguise` 会把图标
   写入数据目录，并通过 `platform.SetWindowIcon` 更新任务栏与 Alt+Tab 图标。

IM 皮肤复用 `ReaderView` 的 `reveal` 与 `BlockView`，由 `ChatTranscript` 把段落渲染成
消息；不要在皮肤内另建文本渲染管线，否则打字模式、进度上报和锚点恢复都会失效。

窗口标题由 `resolveWindowTitle` 按皮肤归一化：内置标题随皮肤切换，用户自定义标题保留；
启动、切换皮肤和保存设置时都会同步到原生窗口。

### 11. 老板键、任务栏与悬停区域

- 老板键 `panic`（默认 `Ctrl+Alt+H`）由 Go 侧直接处理为“显示 / 隐藏切换”，不经过前端；窗口隐藏时仍然有效。
- `Disguise.HideTaskbar` 通过 `platform.SetTaskbarHidden` 设置 `WS_EX_TOOLWINDOW`，把窗口从任务栏与 Alt+Tab 移除；隐藏后仍可用老板键或悬停显示区恢复。
- `Disguise.HoverReveal` 由 `internal/app/hover.go` 每 120ms 轮询 `CursorPosition`，进入 `ShowZone` 显示、进入 `HideZone` 隐藏；坐标是物理屏幕像素，用“取光标”按钮拾取。
- `Disguise.Floating` 让阅读器使用透明、零边距布局（`useGeometry` 的 floating 分支 + `.reader.floating`），窗口背景同步为透明；只对“纯净”皮肤生效。
- `Window.Radius` 只作为 CSS 圆角半径（`--window-radius`）使用，由 `.app-root` / `.window-frame` 的 `border-radius` + `overflow: hidden` 绘制；不要再用 `CreateRoundRectRgn` / `SetWindowRgn` 裁剪原生窗口，二进制区域无法抗锯齿，在高 DPI 下会出现颗粒状锯齿。`platform.SetWindowRadius` 仅负责清除旧版本残留的 native region；圆角大于 0 时窗口背景强制透明，避免四角露出底色。

### 12. 响应式布局

`frontend/src/styles/responsive.css` 定义壳层断点：>1080 完整侧栏；761–1080 为 64px 图标栏；≤760 改为标题栏菜单抽屉（`.shell.nav-open` + `.nav-backdrop`）。≤900 书架工具栏换行；≤680/560 收紧间距与书封最小宽度；书封列宽用 `clamp()` 随窗口缩放。

新增页面必须保证在 560×380 下可操作：工具栏里的分段控件用 `.seg`（`flex: none` + 按钮 `white-space: nowrap`），不要给它们设置可收缩宽度；抽屉导航在路由切换或点击背景时关闭。

## 验证清单

改完代码至少执行：

```bash
gofmt -l internal
go build ./...
go test ./internal/...
```

涉及前端改动时补 `cd frontend && npm run build`；涉及打包资源时执行一次 `wails build`。

## 常见坑

- `frontend/dist/.gitkeep` 必须保留：`//go:embed all:frontend/dist` 在全新克隆下依赖该目录存在。
- `frontend/wailsjs`、`build/bin`、`frontend/dist/*` 是生成物，不要提交。
- `wails build` 会重新生成绑定文件并编译前端；它调用 PATH 里的 `go`，注意上面的工具链问题。
- 登录页的 OTP UI 是历史遗留，新流程不会用到验证码，不要据此改后端。
- 书架排序在前端完成（最近阅读 / 加入书架 / 书名）；后端 `Books()` 只提供默认顺序，不要在前端依赖服务端返回顺序。
