# AI 能力开发计划（2026-11）

> 依据：`docs/ai-roadmap-2026-10-30.md`、上一轮评估结论、`docs/design/ai-ui/README.md`。
> 原则：先地基、再体验；每个阶段都必须能独立构建、测试和回退。

## 阶段总览

| 阶段 | 内容 | 状态 |
|---|---|---|
| Phase 0 | 书籍类型 + AI 准备门禁 + 类型化入口 | ✅ 已完成 |
| Phase 1 | 统一 AI 设置：供应商、模型、提示词、用量 | 待开始 |
| Phase 2 | AI 书房总览：一个入口、准备向导、问答 | 待开始 |
| Phase 3 | 沉浸穿越：场景索引、角色视角、反应选项 | 待开始 |
| Phase 4 | 改写世界线：选择承诺、memory v2、回溯 | 待开始 |
| Phase 5 | 教材 / 期刊 / 医学 / 影像能力 | 待开始 |

---

## Phase 0：地基

### 0.1 书籍类型 `BookKind`

- [x] 新增类型：`fiction` / `nonfiction` / `textbook` / `periodical` / `medical` / `imaging` / `other`
- [x] 基于分类、书名、简介做规则识别
- [x] 存储：`book_ai(book_id, kind, kind_source, updated_at)`
- [x] 用户可在书籍详情更正类型，`kind_source=user`
- [x] 书架同步不覆盖用户更正

验收：

- 小说、教材、医学、期刊、影像、非虚构、未知都能给出稳定类型。
- 分类不确定时默认 `other`，不会误开穿越。
- 用户更正后，后续书架同步保持用户选择。

### 0.2 AI 准备状态

- [x] `AIStatus(bookId)` 返回三段状态：`text` / `scenes` / `world`
- [x] 能力判断：`ask` / `immerse` / `rewrite`
- [x] 文本索引校验 `chunker_version`，版本不符标记 `stale`
- [x] 剧情索引取 `story_job` + `story_scene`
- [x] 世界设定取 `canon_anchor` + `world_stat` / `world_item`

验收：

- 未建文本索引，问答返回可执行提示，而不是空检索。
- 未建剧情索引，沉浸/穿越不可用。
- 未建世界设定，改写不可用。
- 索引过期后能力自动降级。

### 0.3 后端硬门禁

- [x] `AskBook` 要求文本索引
- [x] `BuildStoryGraph` 要求小说 + 文本索引
- [x] `StartBranch` / `ContinueBranch` 要求小说 + 文本 + 剧情 + 世界
- [x] `SuggestAnchors` / `DeriveWorldSheet` 要求小说 + 文本 + 剧情
- [x] `ImportBranch` 要求小说 + 文本 + 剧情
- [x] 错误信息带下一步动作

验收：

- 直接调用绑定也不能绕过 UI。
- 非虚构书籍无法生成时间线或世界线。
- 错误信息能告诉用户缺哪一步、去哪里补。

### 0.4 前端类型化入口

- [x] `Book` 类型增加 `kind` / `kindSource`
- [x] 书籍详情显示类型，并提供更正入口
- [x] 右键菜单只在小说上显示「故事穿越」
- [x] 穿越页只列出小说；没有小说时给出解释
- [x] 开发 mock 支持新绑定

验收：

- 非小说不会出现穿越入口。
- 用户可以自行把误判的书标记为小说。
- 560×380 下类型选择可用。

### 0.5 测试

- [x] 分类器单测
- [x] `book_ai` 存储单测
- [x] `AIStatus` 能力门禁单测
- [x] `go test ./internal/...`
- [x] `npm run build`

验证结果：

- `gofmt -l internal`：无输出
- `go build ./...`：通过
- `go test ./internal/library ./internal/app`：通过
- `npm run build`：通过

## Phase 1：统一 AI 设置

- [ ] 设置页「AI 服务」：供应商预设、地址、Key、默认/快速模型
- [ ] 测试连接
- [ ] 检索与生成参数
- [ ] 提示词注册表：版本、必填变量、恢复默认
- [ ] 数据与安全开关
- [ ] 用量与预算

验收：

- 只有一个 AI 配置入口。
- 提示词缺失必填变量时不能保存。
- 连接测试不发送正文。

---

## Phase 2：AI 书房

- [ ] 路由 `{ name: 'ai', tab: 'ask' | 'immerse' | 'world', bookId? }`
- [ ] 侧栏合并为一个「AI 书房」
- [ ] 本书准备卡：三步状态 + 继续/暂停
- [ ] 能力卡：可用 / 需准备 / 类型不适用
- [ ] 问答接入 `AIStatus`，引用可回跳段落
- [ ] 旧 `assistant` / `traversal` 路由别名

验收：

- 未配置、未索引、类型不适用都有明确下一步。
- 一个入口内可以切换问答/沉浸/改写。

---

## Phase 3：沉浸穿越

- [ ] 幕级场景切分（一章多幕）
- [ ] `immersion_session` 会话
- [ ] 角色卡与视角切换
- [ ] 场景播放器 + 反应选项
- [ ] 防剧透与实体白名单校验
- [ ] 回溯到本幕开头

验收：

- 不出现当前幕之后的事实。
- 不改变原著结果。
- 每次演绎可回到原文 block。
- 生成中显示「正在生成主角体验…」。

---

## Phase 4：改写世界线

- [ ] `Choice` 增加 `intent` / `effects` / `hard`
- [ ] `branch_choice` 承诺账本
- [ ] `branch_event` 事件账本
- [ ] memory v2：USER / INVARIANTS / WORLD / LEDGER / SUMMARY / ENGINE
- [ ] 选择先落账再生成
- [ ] 生成后语义校验
- [ ] 回溯与另存世界线
- [ ] 锚点冲突前置提示

验收：

- 用户选择必须落实；未落实明确失败。
- 死亡/道具/关系变化可复现。
- memory 的 USER 区不可被模型修改。
- 回溯前说明会删除几幕。

---

## Phase 5：其他书籍类型

- [ ] 教材：脑图 → 自测卷 → 知识地图
- [ ] 期刊：跨期速览、议题追踪
- [ ] 医学：循证问答（严格引用、免责声明）
- [ ] 影像：图注检索；多模态评估后再做

---

## 当前迭代

Phase 0 已完成。下一步 Phase 1：统一 AI 设置入口。
