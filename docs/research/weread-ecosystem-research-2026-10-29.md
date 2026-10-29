# 微信读书第三方生态调研（2026-10-29）

> 目标：为桌面端（Windows 优先）微信读书客户端 `my-weread-desk` 选定协议通道、内容获取方案与竞品取舍。
> 调研方式：GitHub Search API + 各仓库 README / 源码 / 官方接口文档交叉验证。
> 结论先行见文末「调研结论」。

---

## 1. 生态总览

按用途可分为 5 类。星数为调研时（2026-10-29）GitHub 数值。

### 1.1 阅读器插件（KOReader / Kindle 生态）——协议实现的最高参考价值

| 项目 | 星 | 语言 | 关键能力 |
|---|---:|---|---|
| `finlater/weread.koplugin` | 998 | Lua | 扫码登录、书架、搜索、章节下载、进度同步、阅读时长上报、阅读统计、划线/想法、书评、公众号文章 |
| `miumiupy98-art/miuread-koreader` | 688 | Lua | 觅阅；双通道（E-Ink 授权 + Web）、精确 `chapter_uid + co` 定位、疑难冲突处理、译文 |
| `leshier/weui.koplugin` | 1 | Lua | We UI 风格 + 书架/搜索/下载 |
| `Exhen/wereadlite.koplugin` | 13 | Lua | 轻量版 |
| `dororo42/weread_K4.koplugin` | 2 | Lua | 面向非触摸 Kindle 4/5 |
| `Mr54233/pickthought.koplugin` | 36 | Lua | 把微信读书想法注入本地书 |
| `MsReverie/Weread_Annotation_Lite.koplugin` | 10 | Lua | 本地 EPUB/TXT 显示云端划线与想法 |

**价值**：`weread.koplugin` 的 `docs/weread-api-reference.md` 是目前公开最完整的协议文档（官方 Skill 网关 + Web 阅读器 + 分片解码 + 图片 tar + 公众号），且源码为纯 HTTP 实现，不依赖浏览器内核，可直接移植到 Go。

### 1.2 全能 SDK / CLI / Agent

| 项目 | 星 | 语言 | 关键能力 |
|---|---:|---|---|
| `teng-lin/weread-omni` | 690 | TypeScript | 40 项操作、E-Ink 2.1.2 全协议目录（184 端点）、扫码登录、TypeScript SDK、JSON CLI |
| `freestylefly/mcp-server-weread` | 584 | JS | MCP server |
| `LiamZJU/kindle-weread-skill` | 13 | Python | Agent skill：越狱 + 装机 + 部署插件 |

**价值**：`weread-omni/docs/endpoints.md` 基于 E-Ink APK 反编译，列出 184 个端点及其参数，是「哪些能力真实存在」的权威清单。其 `src/auth/qrlogin.ts`、`src/profile.ts`、`src/api/mobile.ts` 给出了 E-Ink 通道的登录/续期/请求头/错误码处理的工业级实现。

### 1.3 导出 / 下载

| 项目 | 星 | 语言 | 方案 |
|---|---:|---|---|
| `drunkdream/weread-exporter` | 2359 | Python | **Hook Canvas**：拦截网页 Canvas 绘制文本与样式 → Markdown → EPUB/PDF/MOBI |
| `Algebra-FUN/WeReadScan` | 1005 | Python | 截图 + OCR 思路，已停更（2023-09） |
| `HuanMeng233/weread_downloader` | 283 | Go | Go 实现，README 仅一句「可能会封号」，已停更（2023-12） |
| `nianqianha/WeReadDownloader` | 15 | Python | EPUB 导出、注释保留、断点续传 |
| `whale4113/weread-downloader` | 30 | TS | Web 端下载 |
| `RRRRUDDDD/WeRead-Downloader` | 1 | TS | Markdown + 原生 EPUB |

**价值**：Canvas Hook 方案不需要破解加密分片，但依赖浏览器与页面渲染，无法在无头服务端稳定复现；分片解码方案（1.1）更彻底。`HuanMeng233` 说明 Go 生态已有先例但基本失效，需要重写。

### 1.4 桌面端 / 挂机 / 自动化

| 项目 | 星 | 语言 | 方案 |
|---|---:|---|---|
| `z-z-studio/transparent_reader` | 4 | JS(Electron) | **直接内嵌 weread.qq.com 网页**；透明窗口、无边框、置顶、文字颜色/透明度、自动淡出、窗口缩放、持久化会话 |
| `DoooReyn/WxRead-WebAutoReader` | 459 | JS | 浏览器自动翻页刷时长 |
| `jqknono/weread-challenge-selenium` | 191 | JS | Selenium 自动签到 + 刷时长 |
| `GaviZhao/WeReadGears` | 8 | Python | Docker 挂机；扫码免抓包、签名计算、多账号、Cron、热力图 |
| `fun3588/weread-auto-reader` | 0 | Python | 抓 curl → 定时重放 `/web/book/read`，带 `sg=sha256(ts+rn+token)` |

**价值**：
- `transparent_reader` 证明「透明 + 置顶 + 字号/颜色自定义」这条 UI 路线可行，但其架构（内嵌网页）无法做排版引擎、行号、打字伪装，且网页 DOM 频繁变动会导致功能脆弱。**本项目不采用内嵌网页，改为自建渲染层。**
- `WeReadGears` / `fun3588` 给出了 `/web/book/read` 上报体与签名（`s` 为自定义哈希，`sg` 为 sha256(ts+rn+固定 token)）的验证实现。

### 1.5 数据同步 / 笔记

`malinkang/weread2notion`(2935)、`malinkang/weread2notion-pro`(3416)、`zhaohongxuan/obsidian-weread-plugin`(2287)、`arry-lee/wereader`(958)、`Higurashi-kagome/wereader`(776)：仅笔记/划线同步，使用官方 Skill 或 Web Cookie，无正文下载。

---

## 2. 协议通道对比（结论：双通道）

微信读书存在两套可编程接口，能力互补。

### 2.1 通道 A：官方 Skill 网关（`i.weread.qq.com/api/agent/gateway`）

- 认证：`Authorization: Bearer <API Key>`，Key 由 App「我 → 设置 → 微信读书 Skill → 获取 API Key」生成。
- 请求体：`{api_name, skill_version, ...业务参数平铺}`。
- 能力：搜索、书籍详情、官方目录、进度读取、书架、笔记本/划线、书评、统计、推荐。
- **限制**：官方仅开放 6 项只读能力（书架/搜索/统计/详情/笔记/推荐），**不提供正文**；API Key 需要用户手动开启 Skill。

### 2.2 通道 B：E-Ink 客户端 API（`i.weread.qq.com/<path>`）

- 认证：扫码登录得到 `vid` + `accessToken`（+ `refreshToken` 续期）。
- 请求头：
  ```
  vid: <webLoginVid>
  accessToken: <accessToken>
  baseapi: 30
  appver: 2.1.2.10245900
  basever: 2.1.2.10245900
  osver: 11
  channelId: 900
  User-Agent: WeRead/2.1.2 WRBrand/Onyx wr_eink Dalvik/2.1.0 (Linux; U; Android 11; BOOX Build/onyx)
  ```
- **无需签名**（除 `/login` 续期与 `/book/read`）。
- 覆盖：书架增删、搜索、详情、目录、进度、划线与想法的增删改、书评、阅读统计、公众号、导入。
- 优势：请求简单、字段稳定、限频相对宽松；是 `weread-omni` 40 项操作的主力通道。
- 劣势：**没有正文分片接口**（正文只有 Web 通道有）。

### 2.3 通道 C：Web 阅读器（`weread.qq.com`，Cookie）

- 认证：Cookie `wr_vid` / `wr_skey` / `wr_rt`，以及 `wr_fp`（32 位无符号十进制指纹，用于避免同 UA 多端互踢）。
- 续期：`POST /web/login/renewal`，body `{"rq":"%2Fweb%2Fbook%2Fread","ql":false}`，成功返回 `{"succ":1}` 并下发新 `Set-Cookie`。
- 能力：
  - `GET /web/reader/{_e(bookId)}` → 页面内 `window.__INITIAL_STATE__` 含 `reader.bookInfo`、`psvts`、`pclts`、`token`、`currentChapter`、`progress`。
  - `POST /web/book/chapterInfos` → 带 `tar` 资源包地址的目录。
  - `POST /web/book/chapter/e_0|e_1|e_3`（EPUB 正文）、`e_2`（CSS, `st=1`）、`t_0|t_1`（TXT 网文）。
  - `GET res.weread.qq.com/wrco/tar_<bookId>_<chapterUid>` → 图片 tar。
  - `GET /web/mp/articles?bookId=MP_WXS_xxx` + `GET /web/mp/content?reviewId=xxx` → 公众号文章完整 HTML。
  - `POST /web/book/read` → 阅读时长与进度上报。

### 2.4 通道选择策略（本项目）

```
登录（扫码）── 一次性得到 ──┬── cookies(wr_vid/wr_skey/wr_rt/wr_fp)  → 通道 C
                            └── accessToken / refreshToken            → 通道 B
（可选）Skill API Key                                                 → 通道 A
```

- 元数据/书架/搜索/笔记/统计/进度读写 → **B**（简单稳定）
- 正文/样式/图片/公众号/时长上报 → **C**
- 任一通道失败按「只读可回退，写入不重放」原则处理（避免重复写入）。

---

## 3. 关键算法（已确认可移植）

### 3.1 `_e(value)` 编码（bookHash / chapterHash / `b` / `c` / `pc`）

```
s = str(value); h = md5hex(s); r = h[0:3]
数字串: chunks = [hex(int(9位一段)) ...]; flag = "3"
非数字: chunks = [concat(hex(ord(ch)))]; flag = "4"
r += flag + "2" + h[-2:]
for i, c in chunks: r += hex2(len(c)) + c; 段间插 "g"
if len(r) < 20: r += h[0:20-len(r)]
r += md5hex(r)[0:3]
```
样例：`_e("43208843") = c9c321c07293508bc9c79df`、`_e("2") = c81322c012c81e728d9d180`。

### 3.2 请求签名 `s`

```
q = "&".join(f"{k}={encodeURIComponent(jsString(v))}" for k in sorted(keys if k != "s"))
a = b = 0x15051505
for i from len(q)-1 down to 1 step -2:
    a = (a ^ (ord(q[i]) << ((len(q)-i) % 30))) & 0x7fffffff
    b = (b ^ (ord(q[i-1]) << (i % 30))) & 0x7fffffff
s = hex(a+b)[2:]
```
注意：布尔必须输出 JS 拼写 `false`/`true`（Python 的 `False` 会导致后端返回 `{}`）。

### 3.3 正文分片解码

```
1. 各分片响应形如 "<32位大写MD5><body>"，校验 md5(body).upper() == prefix
2. concatenated = e_0 + e_1 + e_3（TXT: t_0 + t_1）
3. 去掉首字符
4. reverse_swaps：根据 swap_positions 逆序交换字符
   - len < 4 → []
   - len < 11 → [0, 2]
   - n = min(4, (len+9)//10)
   - tmp = 从末尾 n 个字符各取 base4（radix-4）数字串拼接
   - m = len - n - 2; step = len(str(m))
   - 每 step 位取两个数 % m，最多 10 个
5. base64url → base64（去非字母数字，`-`→`+`, `_`→`/`），补齐 `=`
6. 按 latin-1 取字节后做 UTF-8 修复（严格按首字节长度校验续字节）
```
EPUB 正文为此流程输出 XHTML；TXT 为纯文本（`\r\n` 分段、全角空格缩进），需包 `<p>`。

### 3.4 阅读时长上报 `/web/book/read`

```json
{"appId":"wb...h...","b":"_e(bookId)","c":"_e(chapterUid)","ci":2,"co":1543,
 "sm":"当前位置前后20字","pr":0,"rt":30,"ts":1780666397000,"rn":123,
 "sg":"sha256(ts+rn+token)","ct":1780666397,"ps":"<psvts>","pc":"<pclts 或 _e(ct)>","s":"<签名>"}
```
- `appId = "wb" + 前 12 个 UA 分段长度%10 拼接 + "h" + (0x83 滚动哈希)`
- `token` 来自 `__INITIAL_STATE__.reader.token`，缺省可用固定串 `3c5c8717f3daf09iop3423zafeqoi`（社区长期验证）
- 间隔建议 ≥ 30s，且必须携带真实当前位置，否则污染云端进度。

### 3.5 扫码登录

1. `GET https://weread.qq.com/r/weread-skills`（建立会话，携带 `wr_fp`）
2. `GET /api/auth/getLoginUid` → `{uid}`
3. 前端渲染二维码（内容为 uid 对应的微信登录串）
4. 轮询 `GET /api/auth/getLoginInfo?uid=...&otp=`
   - `succeed: true` 后返回 `webLoginVid` / `accessToken` / `refreshToken`
   - 手机端可能要求输入 4 位验证码（otp）
5. 组装 Cookie：`wr_vid=webLoginVid`、`wr_skey=accessToken`、`wr_ql=0`、`wr_rt=urlencode(refreshToken)`
6. （可选）`GET /api/userInfo?userVid=` 取昵称；`GET /api/skills/apikeyGet?only_show=1` 取 Skill Key
7. E-Ink 续期：`POST i.weread.qq.com/login`
   ```json
   {"deviceId":"eink334691225<19位>","deviceName":"BOOX","inBackground":0,"kickType":1,
    "random":123,"refCgi":"","refreshToken":"...","signature":"sha256(ts+deviceId+random)",
    "timestamp":1750000000000,"trackId":"","deviceType":3}
   ```
   → 新 `vid` / `accessToken` / `refreshToken`。

> `wr_fp` 必须每台设备独立生成（4 个 OS 随机字节转十进制），否则两台设备同 UA 登录会互相顶掉（`-2012`）。

---

## 4. 错误码

| 现象 | 含义 | 处置 |
|---|---|---|
| HTTP 200 + `{"errCode":-2012,"errMsg":"登录超时"}` | Cookie/令牌失效或被顶 | 先续期，失败则重新扫码；写操作**不重放** |
| `{"errCode":-2010,"errMsg":"用户不存在"}` | 实际未认证 | 检查 Cookie 域与 `wr_skey` 重复 |
| 正文接口返回 `{}` | 签名错 / `sc≠1` / `e_2` 未用 `st=1` / 无权限 / 会话过期 | 逐项排查 |
| `e_0` 返回以 `{"bookId"` 开头的 JSON | 该书是 TXT 格式 | 改用 `t_0/t_1` |
| 图片空白 | XHTML 仍引用 `res.weread.qq.com` 且未带权限 | 下载 tar 并改写为本地相对路径 |

---

## 5. 合规与风险

- 所有第三方项目均在 README 声明「仅供个人学习研究，不得商用，风险自负」。本项目同样遵循：**只读取当前账号本就有权阅读的内容，不绕过付费/会员限制，不提供批量导出入口作为默认功能**。
- 主要风险：
  1. **账号风控/封禁**：高频请求、异常 UA、同 UA 多设备、刷时长。
  2. **协议变更**：Web 前端算法与 DOM 会变；需把协议层隔离成可替换模块。
  3. **登录互踢**：`wr_fp` 缺失或复用会顶号。
- 缓解：默认低频、串行、带随机抖动；正文按章按需拉取并本地缓存；明确「导出」为手动单本操作；所有网络失败都要有可读的中文提示。

---

## 6. 竞品 UI 取舍

| 方案 | 代表 | 优点 | 缺点 | 本项目 |
|---|---|---|---|---|
| 内嵌官方网页 | `transparent_reader` | 零协议风险、登录/排版/评论全免费获得 | 无法做排版引擎、行号、打字伪装；DOM 变动即失效；内存占用高 | ✗ |
| Canvas Hook 导出 | `weread-exporter` | 导出还原度高 | 需真实浏览器渲染、无法作为阅读器 | ✗（可作为未来的导出备选） |
| 自建协议 + 自建渲染 | `weread.koplugin` | 完全掌控排版/主题/伪装；性能可控；离线可读 | 协议维护成本高；需自研分页 | ✓ |

---

## 7. 调研结论

1. **协议可行**：二维码登录即可同时获得 Web Cookie 与 E-Ink 令牌，两套通道覆盖「元数据 + 正文」全部需求，且已有 Lua/TS 两套可对照的开源实现，算法细节完备（`_e`、签名、分片解码、`tar` 图片、`/web/book/read`）。
2. **不要内嵌网页**：桌面端体验的核心竞争力是排版、禅模式、伪装与窗口行为，内嵌网页无法满足；自建渲染层是唯一可扩展路线。
3. **Go 生态空白**：现有的 Go 下载器已停更且不可用，本项目用 Go 重写协议层具备实际价值。
4. **能力边界**：正文只能走 Web Cookie 通道；E-Ink 通道不能取正文。签到/领奖励类接口（`/act/recvinfinite`、`/pay/balance`、`/exchange`）在公开仓库中缺少可靠验证，应作为「实验特性」单独隔离，不影响主流程。
5. **设计原则**：协议层与渲染层彻底解耦；协议层可替换（Web/E-Ink/Skill 三实现），渲染层只吃标准化 Block 模型。

---

## 8. 扫码登录协议实测修正（2026-10-29，真实抓取）

对 `https://weread.qq.com/api/auth/getLoginInfo` 的原始响应做了单次受控实测（1 次 `getLoginUid` + 1 次 `getLoginInfo`），结论与部分开源实现的处理方式**不一致**，必须以此为准：

### 8.1 长轮询语义

```
GET /api/auth/getLoginInfo?uid=<uid>            # 不携带 otp
→ HTTP 200，服务端挂起约 55s
→ {"succeed":false,"logicCode":"LOGIN_TIMEOUT","canRetry":false}
```

**`LOGIN_TIMEOUT` 是「本轮长轮询窗口结束、尚无人扫码」，不是「二维码过期」。**
二维码会话的有效期是独立的（约 5 分钟）。正确处理是：**用同一个 uid 继续轮询**，而不是重新申请二维码。

若把 `LOGIN_TIMEOUT` 当成过期并自动重新 `getLoginUid`，就会在用户还没扫码时不断生成新二维码 —— 即请求风暴，直接触发风控。这是本项目 v0.1 实现中的真实缺陷。

### 8.2 空 `otp` 参数必须省略

```
GET .../getLoginInfo?uid=<uid>&otp=     → 立即返回 LOGIN_TIMEOUT（视为空验证码尝试）
GET .../getLoginInfo?uid=<uid>          → 正常长轮询挂起约 55s
```

### 8.3 客户端超时必须大于服务端挂起时间

服务端挂起 ~55s，客户端超时若设为 45s 会在服务端应答前中断连接；被截断的响应体会被误报为 JSON 解析失败。本项目取 **75s**（带 `otp` 时 25s），且**禁止对该请求做传输层重试**。

### 8.4 状态机（唯一正确版本）

| `succeed` | `logicCode` | 处理 |
|---|---|---|
| `true` | — | 登录成功，落盘 cookies + accessToken/refreshToken |
| `false` | `""` | pending，继续轮询同一 uid |
| `false` | `LOGIN_TIMEOUT` | **pending，继续轮询同一 uid** |
| `false` | `NEED_OTP` | 请求用户输入 4 位验证码；**暂停轮询**，每次提交只发一次请求 |
| `false` | `OTP_NOT_MATCH` / `OTP_EXPIRED` | 提示重新输入，暂停轮询 |
| `false` | `LOGIN_CANCEL` | 终止轮询，等待用户手动刷新二维码 |
| 其它 | 未知值 | 终止轮询并给出提示 |

其它硬性约束：

1. **只有用户操作才能申请新二维码**（首启或点击「重新获取二维码」）。客户端任何自动路径都不得调用 `getLoginUid`。
2. 同一时刻只允许一个在途轮询请求；每次响应结束后再决定下一次。
3. 传输/解析错误按 3s → 6s → 12s → 20s 退避，连续 4 次失败即**停止轮询**并提示手动刷新。
4. 轮询请求不得重试（`NoRetry`）。
5. 请求头需带 `wr_fp` 设备指纹 cookie 与 `Referer: https://weread.qq.com/r/weread-skills`。

对应回归测试：`internal/weread/auth/qrlogin_test.go`（桩服务器，不打真实接口）。

### 8.5 Cookie 域

`wr_vid` / `wr_skey` / `wr_rt` 需要作为域 cookie（`.weread.qq.com`）保存，因为章节图片来自 `res.weread.qq.com`；而 `wr_fp` 是设备指纹，按域写入即可。实现中 cookie 的 domain 由请求 URL 推导，而非硬编码，否则在非 `weread.qq.com` 的测试端点会被 cookie jar 丢弃。
