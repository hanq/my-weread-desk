# 文字穿越与剧情游戏 — 架构评估（2026-10-30）

问题：小说类作品能否做「文字穿越」，让读者体验主人公的整条故事线？
再叠加 AI 生图、AI 音乐、AI 视频，能否变成剧情游戏？

先给结论，再拆依据。

---

## 0. 结论

**能做，但必须拆成四层，且四层的成本与风险相差一个数量级。**

| 层 | 内容 | 成本 | 风险 | 建议 |
|---|---|---|---|---|
| L1 故事线图谱 | 章节 → 场景 → 事件 → 因果 | 中（一次性，按书） | 低 | ✅ 先做 |
| L2 穿越浏览 | 时间线 + 场景卡 + POV 重写 | 低 | 中（幻觉标注） | ✅ 紧随 |
| L3 分支剧情 | 决策点 + 状态机 + 收敛 | 高 | 高（毁书感） | ⚠️ 按题材试点 |
| L4 多媒体 | 生图 / 音乐 / 配音 / 视频 | 极高 | 高（一致性、版权） | ⚠️ 只做图+配音 |

**「文字穿越」的最小可用形态是 L1+L2，而且它复用现有 RAG 资产，不是另起炉灶。**
L3 才是「剧情游戏」；L4 里只有「生图 + TTS 配音」值得做，AI 视频不建议。

---

## 1. 为什么 L1 是地基（也是 RAG 的直接增值）

现有资产：`rag_chunk` 已经把书切成带 `block_start/block_end` 的块。
缺的是**语义结构**：谁、在哪、做了什么、为什么。

### 1.1 数据模型

```sql
-- 场景：一个连续的时空单元
CREATE TABLE story_scene (
  id TEXT PRIMARY KEY,
  book_id TEXT NOT NULL,
  seq INTEGER NOT NULL,          -- 全书顺序，时间线排序用
  chapter_uid INTEGER NOT NULL,
  chapter_idx INTEGER NOT NULL,
  block_start INTEGER NOT NULL,  -- 与 rag_chunk 同一套锚点
  block_end INTEGER NOT NULL,
  location TEXT NOT NULL DEFAULT '',
  time_hint TEXT NOT NULL DEFAULT '',   -- 清晨/当夜/三日后
  pov TEXT NOT NULL DEFAULT '',         -- 视角人物
  mood TEXT NOT NULL DEFAULT '',        -- 用于配乐/配色
  summary TEXT NOT NULL DEFAULT ''
);

-- 事件：场景内的动作与转折
CREATE TABLE story_event (
  id TEXT PRIMARY KEY,
  scene_id TEXT NOT NULL,
  seq INTEGER NOT NULL,
  kind TEXT NOT NULL DEFAULT '',  -- action/dialogue/reveal/decision/turn
  summary TEXT NOT NULL DEFAULT '',
  participants_json TEXT NOT NULL DEFAULT '[]',
  cause_ids_json TEXT NOT NULL DEFAULT '[]',
  effect_ids_json TEXT NOT NULL DEFAULT '[]'
);

-- 人物/地点/物件：与仿写共用
CREATE TABLE story_entity (
  id TEXT PRIMARY KEY,
  book_id TEXT NOT NULL,
  kind TEXT NOT NULL,             -- person/place/item/org
  canonical TEXT NOT NULL,
  aliases_json TEXT NOT NULL DEFAULT '[]',
  first_scene_seq INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE story_relation (
  book_id TEXT NOT NULL,
  src TEXT NOT NULL,
  dst TEXT NOT NULL,
  rel TEXT NOT NULL,              -- 师徒/父子/敌对/暗恋
  evidence_scene TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (book_id, src, dst, rel)
);
```

关键点：`story_scene.block_start/block_end` 与 `rag_chunk` 同源。
**任何 AI 生成的内容都能一键跳回原文段落**，这是整个功能可信度的基础。

### 1.2 抽取流水线

```
rag_chunk (700 块/30 万字)
   ↓ 按章节聚合，滑动窗口 3~5 块
   ↓ LLM 抽取：场景边界 + 在场人物 + 事件 + 因果
   ↓ 去重合并（同一场景被窗口切两次）
   ↓ 人物别名归并（"黛玉"/"林妹妹"/"颦儿" → 林黛玉）
story_scene / story_event / story_entity
```

成本估算（30 万字）：

| 项 | 数量 | token | 费用（廉价模型） |
|---|---|---|---|
| 场景抽取 | ~700 次小块调用 | ~40 万 | ¥0.2–1 |
| 别名归并 | 1 次全书实体表 | ~2 万 | 可忽略 |
| 因果补全 | ~150 场景 | ~15 万 | ~¥0.1 |

**按书一次性，可缓存、可重跑、可人工修正。**

### 1.3 顺带解决 RAG 的两个短板

图谱建好后，这些问题不再靠向量碰运气：

- 「主角第一次改变主意在哪一章」→ 查 `story_event where kind='turn'`
- 「他为什么恨父亲」→ 沿 `cause_ids` 回溯
- 「第三章出现的人后来去哪了」→ 查 `story_entity.first_scene_seq` 与出场序列

也就是说，**L1 不只是穿越功能的地基，它同时是 RAG 的第二条检索通道**（结构检索），
与现有 BM25 形成「事实 + 结构」互补。

---

## 2. L2 穿越浏览：产品形态

### 2.1 时间线视图

```
┌ 第一章 ──────────────────────────────┐
│ ● 雨夜投宿        [场景卡]            │
│ │                                     │
│ ● 初见黛玉        ← 点击进入          │
│ │                                     │
│ ● 摔玉            [决策点]            │
└───────────────────────────────────────┘
```

每个节点显示：地点、在场人物、一句话摘要、情绪色。
点击 → 场景卡：

```
┌──────────────────────────────────────────┐
│ 第三回 · 荣国府      黄昏 · 黛玉视角      │
│                                          │
│ 【原文】                                  │
│ 步步留心，时时在意，不肯轻易多说一句话…  │
│                                          │
│ 【穿越演绎 · AI 生成】                    │
│ 我扶着丫鬟的手下车时，先听见了檐下的雨…  │
│                                          │
│ [回到原文] [上一幕] [下一幕]              │
└──────────────────────────────────────────┘
```

### 2.2 一条必须坚持的规则

**原文与 AI 演绎永远并列，且默认展示原文。**

原因有三：

1. POV 重写必然补充原著没有的心理描写，不标注就是伪造。
2. 读者对书有情感预期，改写过头会被认为「毁书」。
3. 版权上，AI 演绎不能替代原文，只能作为阅读辅助。

UI 上用一个显式的切换（`原文 / 演绎`），演绎态加角标。这既是诚实，也是产品差异点。

### 2.3 POV 重写的提示词结构

```
[系统] 你是书中人物「{name}」，用第一人称重述下面这一幕。
[系统] 只允许补充感官与心理，不得新增事件、改变结果、添加原著没有的人物。
[系统] 文风约束：{从 style_profile 读取的句长/对话比/意象}
[原文] {scene 对应的 blocks}
[要求] 300~500 字，结尾停在本幕结束时。
```

复用之前设计的 `style_profile`，所以**风格仿写与穿越演绎共用同一套风格资产**。

---

## 3. L3 剧情游戏：能做，但有硬约束

### 3.1 只有「留白处」可以分支

这是最重要的设计约束。分支不能改写已发生的关键事实：

```
原著骨干（canon spine）：场景 1 → 2 → 3 → … → N
分支点只能挂在「原著没写结果」的位置：
  场景 k 的结尾有一句「他没有回答」
  → 允许读者选择「说」/「不说」
  → 两条支线各生成 1~2 幕
  → 必须收敛回场景 k+1
```

如果允许改写关键事实，后续章节就接不上，读者会立刻感到「这不是这本书」。

### 3.2 状态机

```go
type GameState struct {
    SceneID   string            `json:"sceneId"`
    Flags     map[string]bool   `json:"flags"`     // 已知事实、持有物件
    Affinity  map[string]int    `json:"affinity"`  // 与各 NPC 的关系值
    History   []string          `json:"history"`   // 已走路径
}
```

每幕给出 2~3 个选项：

```go
type Choice struct {
    ID          string
    Text        string
    RejoinScene string // 支线结束后回到的 canon 场景
    Effects     map[string]int
}
```

规则：

- 支线最多 2 幕，必须收敛
- 选项不能引入原著不存在的能力/人物
- 结局只有 canon 结局（可以走不同路径到达）

### 3.3 题材适配度

| 题材 | 适配 | 原因 |
|---|---|---|
| 悬疑/推理 | ★★★★★ | 天然决策点，分支=不同推理路径 |
| 武侠/冒险 | ★★★★☆ | 动作选择明确，容错高 |
| 言情 | ★★★☆☆ | 情感选择多，但读者对角色有固定预期 |
| 严肃文学/意识流 | ★☆☆☆☆ | 无决策点，分支即破坏 |

**建议先选一本悬疑做单本试点，验证「分支是否让人觉得合理」，再决定是否推广。**

### 3.4 最大的失败模式

不是技术，是**写崩**。

读者能容忍 AI 图不够美，但不能容忍「主角做了 OOC 的决定」。
所以分支生成必须：
- 低温度（0.3 以下）
- 强制注入人物设定卡（性格、口癖、当前关系值）
- 生成后做一次一致性检查（是否与 `story_entity` 冲突）

---

## 4. L4 多媒体：成本与一致性

### 4.1 生图

| 项 | 评估 |
|---|---|
| 单张成本 | ¥0.1–0.3 |
| 一本书场景数 | ~100–200 |
| 全量预生成 | **¥20–60 / 本** |
| 角色一致性 | 难。需要角色参考图 + 固定 seed + IP-Adapter，否则每张脸都不同 |
| 内容安全 | 暴力/情色场景需要过滤，否则审核风险 |

**结论：不做全量预生成。按需生成 + 缓存。**

理由：一本书 200 张图，用户可能只看 10 张。全量生成等于为一个可能不用的功能付费，
而且首屏等待时间不可接受。

实现上：
- 场景卡点「生成插图」才调用
- 生成结果存入 `media` 表（复用现有图片缓存），key 用 `scene:<id>` 命名空间
- 每本书固定一个「视觉风格卡」（从 `style_profile` 的 mood/时代推导），保证同一本书画风统一
- 角色一致性优先用「同一 seed + 角色描述模板」，做不到就接受「插画集」而非「分镜」

### 4.2 音乐

| 项 | 评估 |
|---|---|
| 按情绪映射固定曲库 | 成本 0，延迟 0 |
| AI 生成 BGM | 每首 ¥0.5–2，且质量不稳定 |

**结论：用固定曲库。** 准备 8–12 段 royalty-free 循环乐，按 `story_scene.mood` 映射。
生成音乐在这个场景里没有足够收益。

### 4.3 视频

**结论：不建议。**

- 单条 5 秒视频的成本是图的 10–50 倍
- 角色一致性比图更差（跨镜头几乎不可能）
- 生成时间以分钟计，破坏阅读节奏

如果一定要「动起来」，用**图片 + Ken Burns 运镜**（ffmpeg 缩放平移）合成 3 秒动图，
视觉上接近短视频，成本接近 0。

### 4.4 配音

**结论：优先做。**

上一轮的探测已经确认 `/book/album` 返回 `playTts` 与 10 个音色。
穿越演绎的文本可以直接 TTS 朗读，配合逐句高亮，沉浸感提升明显，风险最低。

---

## 5. 与现有代码的衔接

```
internal/rag        已有：chunk + block 锚点 + BM25
internal/story      新增：场景/事件抽取、时间线、状态机
internal/media      已有：图片缓存（复用 media 表，加 scene: 命名空间）
internal/ai         已有：provider + 引用；复用风格卡做 POV 重写
```

新增绑定：

```go
BuildStoryGraph(bookID) (StoryGraphStatus, error)   // 离线抽取，按书一次
StoryTimeline(bookID) ([]Scene, error)
SceneDetail(sceneID) (SceneDetail, error)
ReplayScene(sceneID, mode) (string, error)          // 原文 / 演绎
GenerateSceneArt(sceneID) (string, error)           // 按需生图
StartStoryGame(bookID) (GameState, error)
ChoosePath(stateID, choiceID) (GameState, error)
```

前端新增 `TraversalView`：左侧时间线轨道，右侧场景卡。
复用 `ReaderView` 的 `chapterUid + blockStart` 跳转能力。

---

## 6. 排期建议

| 阶段 | 内容 | 工期 | 依赖 |
|---|---|---|---|
| P0 | `story_scene/story_event` 抽取 + 时间线浏览 | 5–7 天 | 现有 RAG |
| P1 | POV 重写（原文/演绎切换 + 标注） | 3 天 | P0 + style_profile |
| P2 | 结构化检索接入问答（"第一次转折在哪"） | 2 天 | P0 |
| P3 | 单本悬疑试点分支 + 状态机 | 7–10 天 | P0 + P1 |
| P4 | 按需生图 + TTS 配音 | 3–5 天 | P0 |
| — | AI 视频 | 不做 | — |

**P0+P2 是最划算的组合**：它既交付「穿越」的骨架，又把 RAG 从「关键词检索」
升级成「结构 + 文本」双通道。P3 再决定要不要把它做成游戏。

---

## 7. 一句话总结

「文字穿越」不是给书加特效，而是**先让机器读懂故事结构**。
结构一旦建立，时间线浏览、结构化问答、POV 重写、分支游戏、插图配乐都只是同一张图的
不同渲染方式。

所以正确的顺序是：**图谱 → 浏览 → 分支 → 多媒体**，而不是反过来从视频开始。
