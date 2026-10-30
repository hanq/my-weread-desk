# 书架分组 / RAG 分块 / 有声书可达性 — 评估与落地记录（2026-10-30）

本轮完成三件事：把书架分组从服务端接通到 UI；把 RAG 分块与检索做成可测量的库；
用真实账号探测有声书数据到底能不能取。以下结论均来自本机实测，不是推测。

---

## 0. 结论摘要

| 事项 | 状态 | 关键结论 |
|---|---|---|
| 书架分组 | ✅ 已实现 | `/shelf/sync` 本来就返回 `archive[]`，此前完全没解析 |
| 有声书元数据 | ✅ 已接通 | `albums[]` 此前被静默丢弃，现已进入书架 |
| RAG 分块 + BM25 | ✅ 已实现 | 43 块样本可用，15k 字分块 521µs、建索引 7ms |
| 有声书播放/文字稿 | ⚠️ 未验证 | 账号书架无专辑，E-Ink 搜索为空，缺一个真实 albumId |
| 电子书 TTS | ✅ 已确认可用 | `/book/album` 返回 `playTts` 与 10 个音色 |

---

## 1. 书架分组

### 1.1 此前断开的位置

服务端一直有分组数据，客户端三处断点：

```
/shelf/sync 响应
  books[]        → 已解析
  mp             → 已解析
  albums[]       → 解析了结构体但从未使用（有声书被静默丢弃）
  archive[]      → 结构体里根本没有这个字段
```

`internal/weread/types.go` 的 `Book` 没有分组成员字段，`book` 表也没有 group 列，
`ShelfView.tsx` 只有排序与过滤。三层都没有分组概念。

### 1.2 实测数据（真实账号）

```
书架: 839 项，分组: 16 个
  分组 "小学生"            books=5
  分组 "小说"              books=111
  分组 "视觉开发&AI"       books=34
  分组 "系统&硬件&运维"    books=28
  ...
```

`archive[]` 的子字段确认为 `archiveId` / `name` / `bookIds`，与推断一致。

### 1.3 设计决策：云端镜像 + 本地覆盖

分组有两个来源，混在一起会互相覆盖，所以用 `book.group_source` 区分：

| source | 含义 | 同步时的行为 |
|---|---|---|
| `cloud` | 来自账号的 archive | 每次同步按云端重建 |
| `local` | 用户在本应用内移动 | **不被云端覆盖** |

`ReplaceShelf` 的写法：

```sql
group_id = CASE WHEN book.group_source='local'
                THEN book.group_id ELSE excluded.group_id END
```

这样用户在应用里把书拖进自建分组后，下次同步不会被打回云端分组。

`DeleteGroup` 把 `group_source` 重置为空而不是 `local`，于是云端分组会在下次同步时
恢复（数据没被本地永久删除），本地分组则彻底消失。这是刻意的取舍：
**本地删除云端分组不应变成一次破坏性的账号操作。**

### 1.4 为什么不回写云端

`/shelf/archive`、`/shelf/deleteArchive` 是写接口。桌面客户端频繁调用容易被判定为
自动化，与现有的设备指纹约束冲突。因此：

- 读取：云端分组全量同步
- 写入：只在本地生效
- UI 上的重命名对云端分组也只改本地显示

### 1.5 改动清单

```
internal/weread/types.go          Book 增加 GroupID/GroupName/IsAudio/TrackCount；新增 ShelfGroup
internal/weread/eink/client.go    shelfWire 增加 archive/archives；新增 ShelfSnapshot
internal/library/store.go         schema 5：book 新列 + shelf_group 表 + 幂等 ALTER 迁移
internal/library/repo.go          分组读写、SetBookGroup、RenameGroup、DeleteGroup
internal/app/library.go           Groups/CreateGroup/RenameGroup/DeleteGroup/SetBookGroup
frontend/src/views/ShelfView.tsx  分组栏、移动菜单、双击重命名、右键删除、有声标识
frontend/src/styles/app.css       .shelf-groups / .group-chip / .audio-badge
```

### 1.6 迁移注意

SQLite 没有 `ADD COLUMN IF NOT EXISTS`，`CREATE TABLE IF NOT EXISTS` 也不会给旧表加列。
用 `PRAGMA table_info` 逐列检查后 `ALTER TABLE`。

**踩到的真实坑**：`CREATE INDEX ... ON book(group_id)` 必须放在 ALTER TABLE 之后，
否则旧库启动时直接报 `no such column: group_id`。测试 `TestMigrationAddsGroupColumns`
专门覆盖这条路径。

---

## 2. RAG 分块与检索

### 2.1 分块单元

分块直接消费已有的章节缓存，不重新请求任何接口：

```
chapter_content.blocks_json ([]weread.Block)
  → 过滤 image / divider / 推广行
  → 按段落累积到 ~500 字，边界处重叠 1 块
  → Chunk{ blockStart, blockEnd, chapterUid, chapterIdx, ... }
```

`blockStart` / `blockEnd` 是引用能跳回阅读器的唯一依据：答案里的「第 14 章 blocks 9..14」
可以直接定位到段落，而不是给一句无法核对的引文。

尾部不足 `MinChars` 的碎块会合并进上一块而不是丢弃——丢弃会造成检索覆盖的缺口。

### 2.2 中文 BM25：bigram 而非分词

中文没有空格，用字符二元组代替分词：

```
"林黛玉" → ["林黛", "黛玉"]
"Go 1.20 与 SQLite" → ["go", "1", "20", "与", "sqlite"]
```

代价是索引项变多，收益是**零词典、零模型文件**，且对专名、术语、原句命中最强——
而问答与仿写连续性需要的恰好是这类事实型检索。

### 2.3 实测

本机缓存里只有 9 章正文（58MB 的库主要是图片 BLOB），样本偏小：

```
书籍: DeepSeek高效数据分析 (3300206052)
正文: 7 章, 15274 字
分块: 43 块, 平均 408 字, 最短 86, 最长 490, 耗时 521µs
索引: 43 块, 建索引 7.3ms
```

检索质量（真实查询，非构造）：

| 查询 | 命中 | 第 1 名 | 分数 |
|---|---|---|---|
| 数据清洗 | 3 | 第14章 2.3 读取Web在线数据 | 5.852 |
| 可视化 | 2 | 第13章 2.2 读取数据库数据 | 5.502 |
| Python | 3 | 第9章 1.4 在线开发环境 | 1.104 |
| 缺失值 | 3 | 第12章 2.1 读取本地离线数据 | 5.538 |

按线性外推，30 万字 ≈ 730 块，建索引约 120ms，单次查询仍在毫秒级。
**这个规模不需要向量数据库，也不需要有状态的索引服务。**

### 2.4 探针

```bash
go run ./cmd/rag_probe -list                 # 看哪些书真正缓存了正文
go run ./cmd/rag_probe -q "数据清洗" -q "缺失值"
```

只读打开 `library.db`，不触发迁移、不写任何数据。

### 2.5 尚未接入的部分

分块与索引是**库**，还没有接到 UI。缺的三块：

1. `rag_index` / `rag_chunk` 表与增量索引（下载完成后触发）
2. `internal/ai` provider + 单轮带引用问答
3. 前端引用条与 `chapterUid + blockIndex` 回跳

按性价比，先做 1+2 的纯 BM25 版本即可，embedding 留到出现真实失败样本再评估。

---

## 3. 有声书可达性

### 3.1 探测方法

```bash
go run ./cmd/audio_probe                    # 从书架找有声书
go run ./cmd/audio_probe -album <albumId>   # 直接指定专辑
go run ./cmd/audio_probe -book <bookId>     # 电子书 → 关联专辑/TTS
go run ./cmd/audio_probe -search 三体 -scope all
```

只输出 JSON 形状与长度，不打印 token、cookie 或正文。

### 3.2 实测结果

| 端点 | 结果 |
|---|---|
| `GET /shelf/sync` | 200，`archive[]` 16 组，`albums[]` 0 条 |
| `GET /book/album?bookId=695233` | 200，只有 `bookInfo` + `playTts` + `ttsVoices[10]`，**无 albumId** |
| `GET /store/search?scope=14` | 0 条（E-Ink 商店索引对该账号为空） |
| `GET /store/search?scope=0` | 0 条 |
| 网页 `/web/search` | 30 条（说明通道本身可用，是 E-Ink 索引的问题） |
| `GET /track/text` | 未触发（没有 albumId） |
| `GET /audio/geturl` | 未触发（没有 albumId） |

### 3.3 结论

**有声书能否进 RAG，仍未证伪，但已经可以用一次命令验证。**
当前阻塞点不是权限，而是这个账号书架里没有有声书，且 E-Ink 搜索给不出专辑 ID。

补验方式（把任意一张有声专辑加入书架后）：

```bash
go run ./cmd/audio_probe            # 自动取第一张专辑
```

判定标准：

- `/track/text` 返回 200 且有非空文本字段 → 有声书可进 RAG
- `/audio/geturl` 返回 200 → 播放链路值得继续验证防盗链与格式
- 两者都失败 → 有声书只做元数据展示

### 3.4 不可行的路线

本地 ASR 在这个项目里不可行，不是性能问题而是构建约束：

```
whisper.cpp      → 需要 cgo
本项目 sqlite    → modernc.org/sqlite，纯 Go，无 cgo
Go 1.20          → 不能为了 ASR 升级工具链
```

所以有声书进 RAG **只有** `/track/text` 这一条入口。

### 3.5 更划算的替代：电子书 TTS

`/book/album` 的返回里有：

```
playTts: true
defaultVoiceType: <n>
ttsVoices: [{ name, male, voicetype, model, supportWav, ... } × 10]
```

这说明官方 TTS 是可用能力。与接有声书专辑对比：

| | 有声书专辑 | 电子书 TTS |
|---|---|---|
| 覆盖 | 仅购买了音频版的书 | 所有已缓存正文的书 |
| 接口风险 | 高（取流未验证） | 低（元数据已确认） |
| 实现量 | 5+ 天 + 不确定性 | 2–3 天 |
| 进度回传 | `/album/listenreport` | 复用现有 `SaveProgress` |

如果目标是「这个应用能听书」，TTS 朗读的性价比明显更高。

---

## 4. 验证

```bash
gofmt -l internal cmd            # 无输出
go build ./...                   # 通过
go test ./internal/...           # 全绿
cd frontend && npx tsc --noEmit  # 通过
cd frontend && npm run build     # 通过
```

新增测试：

- `internal/rag/chunk_test.go`：覆盖块边界、重叠、非文本块过滤、碎块合并、Seq 分配
- `internal/rag/bm25_test.go`：bigram 分词、排序正确性、空查询、邻居扩展
- `internal/library/group_test.go`：分组往返、本地覆盖云端、删除清理、重命名、旧库迁移

---

## 5. 建议的下一步

1. **索引落库 + 单轮带引用问答**（P0，3–5 天）：`rag_index`/`rag_chunk` + `internal/ai`
   provider，先把「问这本书 + 点引用回跳」跑通。
2. **有声书补验**（0.5 天）：把任意有声专辑加入书架，跑一次 `audio_probe`，结论即可定型。
3. **电子书 TTS 朗读**（2–3 天）：如果第 2 步判定为否，这是听书需求的低成本解。
4. **embedding 混合召回**：等有真实失败查询再上，不要先做。

分组的作用不只是 UI——它是跨书检索的天然作用域。
`rag.Query` 里预留 `BookIDs`，分组一接通，「我这批推理小说里谁最先死」就变成一次带
scope 的过滤，而不是全库检索。
