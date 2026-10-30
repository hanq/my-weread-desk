# AI 化功能方向与路线（2026-10-30）

> 单一入口。详细设计与决策见文末索引；本文只记录**方向、状态、依赖与下一步**。

## 0. 一句话

把书变成可检索、可穿越、可改写的世界：

- **RAG** 负责事实（原文段落 + 引用回跳）
- **故事图谱** 负责结构（场景、事件、人物、主线锚点）
- **世界线** 负责选择（分叉、续写、收束）
- **世界表** 负责约束（资源、道具、权限、任务）

四层共用同一套 `block_start/block_end` 锚点，所以任何生成内容都能回到原文。

---

## 1. 已实现

| 能力 | 说明 | 位置 |
|---|---|---|
| 书架分组 | cloud 镜像 + local 覆盖，云端分组不回写 | `internal/library/repo.go`、`ShelfView.tsx` |
| 有声书元数据 | `albums[]` 进书架，`IsAudio` 标记 | `internal/weread/eink/client.go` |
| RAG 分块 | 章节感知、500 字、重叠 1 块、保留 block 区间 | `internal/rag/chunk.go` |
| BM25 检索 | 中文单字 + 二元组，无外部依赖 | `internal/rag/bm25.go` |
| 索引持久化 | schema 6：`rag_index` / `rag_chunk` | `internal/library/rag.go` |
| 助手问答 | OpenAI 兼容流式、引用列表、可取消 | `internal/ai`、`internal/app/rag.go`、`AssistantView.tsx` |
| 故事时间线 | 章节 = 场景，`story_job` 可恢复作业 | `internal/app/story.go`、`TraversalView.tsx` |
| 主线锚点 | `canon_anchor` + 引力/偏移/施压阶梯 | `internal/story/anchor.go` |
| 世界线 | 分叉、逐幕生成、宪法校验、跳跃收敛 | `internal/app/branch.go`、`internal/story/branch.go` |
| 意图编译 | 规则预检 + LLM 分类 + 冲突替代方案 | `internal/story/intent.go` |
| 幕内选择 | 每幕 2–3 个具体选项 | `internal/story/choices.go` |
| 补丁校验 | 容量上限、空条目、重复死亡软警告 | `internal/story/patch.go` |
| 世界表 | 属性/道具/背包，引擎侧钳制与衰减 | `internal/story/world.go`、`internal/library/world.go`、`internal/app/world.go` |
| 世界线包 | 导出/导入、BookKey 匹配、不可信输入上限 | `internal/story/package.go` |

当前 schema：**11**。

---

## 2. 未实现（按优先级）

| 优先级 | 事项 | 依赖 | 为什么现在没做 |
|---|---|---|---|
| P1 | **任务系统**：主线=锚点、支线=子剧情、日常=时间成本 | 世界表 + `story_event` | 需要任务与锚点的映射规则，先验证约束是否有效 |
| P2 | **职业/技能**接入意图编译 | P1 + 人物名册 | 需要「谁有什么权限」的稳定来源 |
| P3 | 属性/道具单项编辑 UI | 世界表 | API 已就绪，缺编辑面板 |
| P4 | GitHub 发布/浏览 | 世界线包 | 需要 PAT、gist 索引、内容审核与合规评估 |
| P5 | 滚动摘要 | 世界线 | 现在只存最后一幕摘要，长线会丢早期上下文 |
| P6 | 语义级补丁校验 | 人物名册 | 结构校验挡不住「把主角写死」 |
| P7 | 段落级引用跳转 | ReaderView | 目前引用只跳到章节，未滚到 `blockStart` |
| P8 | embedding 混合召回 | 真实失败样本 | 先 BM25 收集「明明书里有却搜不到」的 case |

---

## 3. 分期路线

| 阶段 | 内容 | 状态 |
|---|---|---|
| RAG P0 | 分块 + BM25 + 引用回跳 + 问答 | ✅ |
| RAG P1 | 索引持久化 + 助手页 | ✅ |
| 穿越 P0 | 章节=场景 + 时间线 + 可恢复作业 | ✅ |
| 穿越 P1 | canon 锚点 + 引力/偏移 | ✅ |
| 平行世界 P0 | 分叉 + 生成 + 校验 + 收束 | ✅ |
| 平行世界 P1 | 意图编译 + 冲突替代 + 幕内选择 | ✅ |
| 世界表 P0 | 属性/道具/背包 + 引擎侧数值 | ✅ |
| 世界表 P1 | 任务系统 | ⬜ 下一步 |
| 世界表 P2 | 职业/技能 | ⬜ |
| 分发 P1 | GitHub 发布/浏览 | ⬜ 最后 |

---

## 4. 关键决策（摘要）

完整 23 条见 `ai-narrative-decisions-2026-10-30.md`，这里只列影响方向的：

1. **不接通用 memory MCP 做书籍 RAG**：它存摘要与实体，不存原文段落，无法回跳。
2. **不用 codebase-memory-mcp 索引小说**：无 AST，范畴错误；只借「图建模」思路。
3. **不上 `sqlite-vec` / `onnx`**：项目锁定 Go 1.20 + 纯 Go sqlite，两者都要 cgo。
4. **先 BM25，embedding 后置**：事实型查询（人名、术语、原句）BM25 更强。
5. **平行世界 = 局部反事实 + 跳跃收敛**：整本重写一致性必崩，且要等十几分钟。
6. **引力只拉「结果」不拉「路径」**：直接改角色动机是最典型的 OOC。
7. **memory.md 分区 + 引擎哈希**：AI 改写用户规则是世界线腐化的头号原因。
8. **世界表从原著派生，最多 6 项属性**：通用 RPG 模板会毁掉文学质感。
9. **不做等级/经验**：一旦有等级，玩家优化等级而不是故事；成长体现为权限与信息。
10. **分发包不含原文**：只发 AI 生成的幕与幕号引用，避免分发小说副本。

---

## 5. 已知边界

- 时间线只覆盖**已缓存章节**；未下载的章节不入图。
- 引用跳转是**章节级**，未滚动到 `blockStart`。
- 意图编译的 `needs_setup` 只是给目标加前缀，没有真的插入铺垫幕。
- 状态补丁是**结构校验**，挡不住「结构合法但叙事荒谬」的变更。
- 世界表派生只看**前 60 幕**，长书后段可能漏道具。
- `Decay` 的语义是「每幕消耗」，不是真实时钟。
- 世界线包只做**剪贴板导入导出**，没有接 GitHub API。
- 世界线宪法（memory.md）导入后**没有沙箱**，恶意规则只能靠用户自行确认。
- `wails build` 需要重跑才能生成新绑定（`DeriveWorldSheet` 等）。

---

## 6. 下一步（建议）

**世界表 P1：任务系统**

1. `world_quest` 表：`kind`（main/side/daily）、`goal`、`deadline_seq`、`requires`、`rewards`、`anchor_id`
2. **主线任务必须映射 `canon_anchor`**，不另造主线——否则任务与引力会互相打架
3. 支线从 `story_event` 中不在主脊上的事件抽取
4. 日常只做时间成本（消耗属性/推进 decay），不做打卡奖励
5. 任务完成判定：引擎 + 一次性 LLM 判定
6. 前端：任务日志 + 当前目标提示

验收标准：

- 主线任务与锚点一一对应，没有孤儿锚点
- 日常任务不阻塞主线
- 任务完成判定与 `State` 变更可复现

---

## 7. 文档索引

| 文档 | 内容 |
|---|---|
| `ai-rag-group-audio-assessment-2026-10-30.md` | 分组、RAG、有声书可达性（实测） |
| `narrative-traversal-design-2026-10-30.md` | 文字穿越四层、剧情游戏边界、多媒体成本 |
| `parallel-world-design-2026-10-30.md` | 世界意志、意图编译、memory.md、GitHub 分发 |
| `ai-narrative-decisions-2026-10-30.md` | 决策与放弃项、未决问题 |
| 本文 | 方向、状态、依赖、下一步 |

---

## 8. 代码地图

```
internal/rag/          分块、BM25、检索缓存
internal/ai/           OpenAI 兼容 provider、引用问答
internal/story/        宪法、锚点、引力、意图、选择、世界表、分发包
internal/library/      SQLite（schema 11）：rag_*、story_*、canon_anchor、world_*
internal/app/          rag.go、story.go、branch.go、world.go（Wails 绑定）
frontend/src/views/    AssistantView、TraversalView、BranchView
```
