# AGENTS.md（项目级）

> 本文件是 my-weread-desk 的项目级约定，叠加在全局 `~/.pi/agent/AGENTS.md` 之上。
> 通用规则（删除安全、图标、命名、注释、提交身份）仍然适用，这里只补充本项目特有的约束。

## 项目概览

微信读书桌面客户端（Wails v2 + Go 1.26 + React 19 / Vite 7 / TypeScript）。

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

`go.mod` 要求 **go 1.26.0**。本机 PATH 里的 `go` 可能是旧版本（如 1.20 / 1.25），此时：

- 普通构建：`GOTOOLCHAIN=auto go build ./...`（让 Go 自动下载对应工具链）；
- `wails build`：它直接调用 PATH 中的 `go`，需要把 1.26 的 `GOROOT` 与 `bin` 放到 PATH 前面，例如：

```powershell
$tc = 'C:\Users\jhq\.g\versions\1.26.5'
$env:GOROOT = $tc
$env:PATH = "$tc\bin;$env:PATH"
$env:GOTOOLCHAIN = 'local'
wails build
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
| 划线 | `web.Notes` |

兜底放在 `internal/app` 层（先 E-Ink，失败再 Web），不要在 `web` 包里反向调用 E-Ink。
已知仍只走 E-Ink、需要重新扫码的能力：书架写操作、统计、公众号列表、全书下载的目录引导。

### 6. 图标与品牌资源

- 界面功能图标禁止 emoji，使用线性 SVG（`stroke="currentColor"`，24×24，`stroke-width=2`）或 iconfont / iconpark。
- 应用图标：`build/appicon.png`（1024×1024）、`build/windows/icon.ico`（16–256 多尺寸）、
  `frontend/public/favicon.png`、`frontend/public/weread-icon.png`。
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
