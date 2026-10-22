# Novelgen 架构文档

> 状态: 活跃开发中
> 本文描述的是**当前代码**的实际结构。若代码与本文不一致，以代码为准，并同时更新本文。

---

## 1. 项目概述

Novelgen 是一个 AI 辅助小说创作的命令行工具，覆盖从创意到成书的完整工作流：

```
init → setup → compose → craft → write → export
                              ↘ recap / rpg-dsl / simulate-dsl
```

核心特点：

- **结构化创作**：部 → 卷 → 章 的三级大纲体系。
- **AI 驱动**：每个阶段由专门的 Agent 执行，产物是强类型 JSON。
- **RPG/DSL 验证**：把故事转成可执行的 DSL/游戏数据，用确定性规则检查一致性与战力。
- **双执行后端**：同一个 Agent 既可以走 OpenAI 兼容的 chat 接口，也可以走 Claude Agent SDK 进程（默认用于主要创作阶段）。

---

## 2. 仓库形态

仓库包含**两个 Go module**，由根目录的 [`go.work`](../go.work) 组成 workspace：

| Module | 位置 | 产物 |
| --- | --- | --- |
| `novelgen` | 仓库根 | `bin/novelgen.exe`（主 CLI）+ `tools/` 下的独立分析工具 |
| `novelgen/webui` | [`webui/`](../webui/main.go) | 本地 Web 服务，前端在 `webui/frontend/`（React + Vite） |

```bash
go test ./...      # 根 module（含 cmd/、internal/、tools/、examples/）
cd webui && go test ./...   # webui module（根目录的 ./... 不会跨到它）
make test-all      # 一次跑两个 module（根 Makefile）
```

注意：根目录的 `./...` **不会**跨到 webui module；两个 module 需分别测试。CI 也是分 job 测试的。

### 2.1 目录结构

```
novelgen/
├── main.go                    # 唯一入口：注册命令并以可取消 context 运行
├── cmd/                       # 主 CLI（package cmd）+ 已拆出的命令子包
│   ├── root.go                # 根命令 + 进程级 context
│   ├── registry.go            # init()-based 命令自注册
│   ├── setup.go compose.go craft.go draft.go write.go
│   ├── recap_cmd.go project.go rpg.go rpg_dsl.go simulate_dsl.go
│   ├── tool_register.go       # 仅接线 internal/toolplane.NewCommand()（实现见 §5）
│   ├── polish.go
│   ├── config/                # package config，     config.NewCommand()
│   ├── translate/             # package translate，  translate.NewCommand()
│   ├── initcmd/               # package initcmd，    initcmd.NewCommand()
│   ├── export/                # package export，     export.NewCommand()
│   ├── recap/                 # package recap，      recap.NewCommand()
│   ├── analyzecmd/            # analyze-ai-rpg / analyze-chapter-ai
│   ├── simulatedsl/           # package simulatedsl，simulatedsl.NewCommand()
│   ├── rpgcmd/                # package rpgcmd，     rpgcmd.NewCommand()
│   ├── craftcmd/              # package craftcmd，   craftcmd.NewCommand()
│   ├── draftcmd/              # package draftcmd，   draftcmd.NewCommand()
│   ├── composecmd/ setupcmd/  # 大纲 / 设定命令（NewCommand()）
│   ├── writecmd/ polishcmd/   # 写作 / 打磨命令（NewCommand()）
│   ├── projectcmd/            # 项目管理命令（NewCommand()）
│   └── *_register.go          # 子包命令的唯一接线点（RegisterCommand(... NewCommand())）
├── tools/                     # 独立开发/分析二进制（非主 CLI，见 tools/README.md）
│   ├── check_novel/ rpg-check/ rpg-write/ simulate/ visualize/
│   └── story2rpg/ validate-outline/ deep-analyze/ extract-*/ analyze-outline/
├── internal/
│   ├── contract/              # L0 纯契约：落盘模型、解析/规范化、命令串
│   │   ├── story/             # StorySetup/Outline/Elements/Recap/Review/State
│   │   └── toolcmd/           # `novelgen tool ...` 命令串 builder
│   ├── platform/              # L1 技术设施
│   │   ├── store/             # Root/FS/布局/原子写/快照（唯一知道磁盘布局）
│   │   ├── llm/               # OpenAI 兼容客户端 + 配置
│   │   ├── agentruntime/      # Claude Agent SDK 进程运行时 + workflow skills
│   │   └── logger/ utils/ envutil/ usage/ workerpool/ cli/
│   ├── fiction/               # L2 创作域
│   │   ├── rpg/               # RPG 引擎：GameWorld/模拟/benchmark；dsl/；llm/（LLM 抽取）；project/（项目加载适配）
│   │   ├── domain/            # project/state(logic)/continuity/quality/review/craft
│   │   ├── check/             # 确定性 check 引擎 + issue navigation
│   │   ├── mutation/          # MutationService：canonical 写入/merge/normalize/checkpoint
│   │   ├── agents/            # Agent 阶段：base/setup/compose/craft/write/recap/...
│   │   ├── derive/            # 章节→RPG-DSL 生成/批缓存/信号增强
│   │   ├── refresh/           # 派生 DSL 刷新
│   │   └── pipeline/          # writepipe：write/polish 共享流程
│   ├── toolplane/             # L3 Agent 工具面适配器：CLI/JSON/query/context/brief/view
│   ├── app/                   # L4 组合根：Session
│   └── architecture/          # 分层 + 模块边界测试
├── webui/                     # 独立 module：本地 Web 服务 + 前端
├── docs/                      # 技术文档
├── configs/ examples/ scripts/
└── build.ps1 build.bat Makefile
```

模块依赖只能向内：`contract ← platform ← fiction ← toolplane ← app`。`fiction` 内部顺序为
`rpg < domain < check < mutation < agents < derive < refresh < pipeline`，由
[`internal/architecture/modules_test.go`](../internal/architecture/modules_test.go) 强制。

---

## 3. 主 CLI

入口 [`main.go`](../main.go)：

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
cmd.RegisterAllCommands()   // 触发各命令文件 init() 中的注册
cmd.ExecuteContext(ctx)     // 长流程可在安全点响应取消
```

命令通过 [`cmd/registry.go`](../cmd/registry.go) 的 `RegisterCommand` 在 `init()` 中自注册，最终挂到 [`cmd/root.go`](../cmd/root.go) 的 `rootCmd`。所有命令都在同一个 `cmd` 包内。

**命令子包约定**：自包含的命令组可拆成 `cmd/<group>/` 子包（当前有 `config`、`translate`、`initcmd`、`export`）。子包导出 `NewCommand() *cobra.Command`，由 `cmd/<group>_register.go` 统一接线到 `RegisterCommand`。共享的项目定位/加载逻辑放在 [`internal/fiction/domain/project`](../internal/fiction/domain/project/project.go)，不再散落在 `cmd` 里。新增命令若只依赖 `internal/*`，优先放进子包。

主要命令分组：

| 分组 | 命令 |
| --- | --- |
| 创作流水线 | `init` `setup` `compose` `craft` `write` `export` |
| 辅助阶段 | `recap` `translate` `draft`（已废弃，legacy） |
| RPG / DSL | `rpg` `rpg-dsl` `simulate-dsl` |
| 质量与修复 | `quality-gate` `continuity` `polish` `fix` |
| 项目与配置 | `project` `config` |
| **Agent 工具面** | `tool`（见 §5） |

`tools/` 下还有 14 个独立分析二进制，它们**不属于**主 CLI，`build.ps1`/`Makefile` 不产出它们。

---

## 4. Agent 层

每个创作阶段是一个 Agent，统一继承 [`internal/fiction/agents/base.BaseAgent`](../internal/fiction/agents/base/agent_base.go)：

- 从 `SKILL.md` 加载 system prompt（支持 `{{language}}` 等模板变量）。
- 把强类型 input 转成 markdown，追加 output JSON schema 要求。
- 调用后端、解析 JSON（含确定性修复与一次 JSON 修复重试）。
- 记录 usage 到 `logs/usage.jsonl`，并落调试用 prompt/response 文件。

### 4.1 两条执行路径

`BaseAgent` 在 `invokeAIWithRuntimeResult` 中二选一：

1. **Chat 路径**：`internal/platform/llm.OpenAIClient`，标准 OpenAI 兼容 `/chat/completions`。用于非 SDK 的调用（例如 JSON 修复），以及未要求 SDK 的简单阶段。
2. **Agent SDK 路径**：`internal/platform/agentruntime.ProcessRuntime`，通过 stdin/stdout 调用 [`claude_runner.py`](../internal/platform/agentruntime/runners/claude_runner.py)（Claude Agent SDK）。`setup/compose/craft/write/recap/translate` 均以 `RequireSDK: true` 要求该路径。

运行时选择见 `resolveRuntime`：显式传入 > client 实现的 `Runtime()` > 按 provider（`claude`/`claude_sdk`/`agent`）加载 [`~/.novelgen/agent_config.json`](../internal/platform/agentruntime/config.go)。

命名注意：`XxxWithAgentSDK` 方法表示“使用带项目工具的 workflow prompt”，**不**表示
“换一个 runtime”。在 Claude provider 下，普通 `Xxx` 方法也可能已经走 SDK runtime；区别在
prompt/tool allowlist 与写入方式。后续新增阶段应优先把这种差异表达为显式的 mode/options，
而不是继续增加 `WithAgentSDK` 方法对。

### 4.2 两套技能集（用途不同，不是重复）

两套技能集**用途不同、名称不重叠**，都通过 `go:embed` 打进二进制：

| 路径 | 数量 | 用途 |
| --- | --- | --- |
| [`internal/fiction/agents/base/skills/`](../internal/fiction/agents/base/skills/) | 38 | chat 路径的阶段 prompt（`setup-gen`、`compose-review`、`craft-characters` …） |
| [`internal/platform/agentruntime/skills/`](../internal/platform/agentruntime/skills/) | 22 | SDK workflow prompt（`goal-agent-core`、`write-chapter-workflow`、`outline-review-workflow`、`novel-tools-core` …） |

SDK skills 会在运行时物化到 `~/.novelgen/agents/skills/`；chat skills 直接读源码树旁的 `skills/` 或内嵌副本。

> 仓库里另有 `.agents/skills/` 与 `.trae/skills/`：那是给外部编辑器/助手（Trae 等）用的开发辅助技能，与运行时无关。

### 4.3 Goal Agent（目标级自主执行）

`novelgen agent run` 不再按固定 stage 方法编排一次调用，而是把 goal、skill
catalog、工具白名单和权限交给 Agent SDK：

- **Skill catalog**：`ListEmbeddedSkills()` 只注入 name/description；模型通过
  `novelgen tool skill read <name>` 按需加载完整 `SKILL.md`。
- **Tool families**：只读模式开放 `tool skill/query/check`；`--write` 才增加
  `tool patch`、`patch-buffer` 和 `refresh`。工具仍然是结构化项目入口，不是
  任意 shell。
- **Typed tools**：支持 in-process MCP 的 SDK 会暴露
  `skill_list`/`skill_read`/`project_query`/`project_check`/`project_patch`/
  `project_refresh`，以及长正文分块的 `patch_buffer_*` 和可见 todo 的
  `plan_update`。它们内部仍调用 `novelgen tool`，因此不绕过 mutation/check；
  不支持 MCP 的 SDK 自动退回 Bash allowlist。patch buffer 的版本号参与 dry-run
  指纹，dry-run 后继续 append 会让 apply 失效。
- **Session resume**：runner 从 `ResultMessage.session_id` 取回 session id；
  `agent run --resume <id>` 把它传给 SDK 的 `resume`，继续同一段对话上下文。
- **模型决定过程**：读哪些事实、加载哪个 skill、是否 patch、修几轮，由模型
  在 goal 内决定。
- **Go 决定完成边界**：`RunAgentGoal` 校验最终报告；声称改动但未观察到
  `patch --apply`、apply 后没有 follow-up check、或完全没有使用项目工具，
  都会被拒绝。usage 继续写入 `logs/usage.jsonl`。

这是与 `--agent-sdk` stage 模式并行的另一层抽象：stage 模式适合可重复和窄范围
的 pipeline；goal agent 适合跨对象、需要自主探索和组合 skill 的任务。二者共享
同一套 Agent runtime、project tools、mutation/check 与持久化边界。

---

## 5. Agent 工具面（`novelgen tool`）

SDK Agent **不能直接读写项目文件**。它们只能通过 CLI 的 `tool` 子命令访问项目事实（见 [`novel-tools-core`](../internal/platform/agentruntime/skills/novel-tools-core/SKILL.md)）：

| 命令 | 作用 |
| --- | --- |
| `tool skill list/read` | 发现技能目录并按需加载一个 `SKILL.md` |
| `tool query <story-setup\|outline\|craft\|chapter\|context\|logs>` | 只读查询，带 `--view index\|brief\|full`、`--limit`、`--fields` 预算控制 |
| `tool check <quality\|simulation\|all\|schema>` | 确定性校验 |
| `tool patch <outline\|craft\|setup\|recap\|chapter>` | 打补丁，**默认 dry-run** |
| `tool patch-buffer append/show/clear` | 大正文分块暂存 |
| `tool refresh [chapter-dsl]` | 重建派生数据 |

这保证了 AGENTS.md 的硬规则：**AI 输出不得直接成为项目状态**。Agent 只能提交 patch，由 [`internal/fiction/mutation`](../internal/fiction/mutation/mutation.go)（merge、保留 ID、normalize、落盘与 checkpoint）与 [`internal/fiction/check`](../internal/fiction/check/check.go)（schema/quality/simulation 校验）负责，失败即拒绝，`--apply` 才落盘。

### 5.1 共享 Mutation Service

Agent 产物写入现在有且只有一个归属：`internal/fiction/mutation.MutationService`
（[`mutation.go`](../internal/fiction/mutation/mutation.go)）。`novelgen tool patch` 是它的
CLI/进程适配器；Chat/typed-JSON 阶段和写作流水线直接调用它的 typed 方法：

- `ReplaceSetup` / `UpsertCraftElements` / `ReplaceChapter(Object)` /
  `ReplaceRecap` / `ReplaceDraft` 用于完整对象或批次提交。
- `ReplaceOutline` 用于 compose 的完整大纲提交；`ReplaceOutlineRaw` 用于
  scoped Agent SDK 改进，避免把未选中卷也重新 normalize。
- `CommitOutlineRaw` 用于层级生成过程中对 canonical `outline.json` 的增量
  progress 快照：只做锁 + 原子 JSON 写，不 normalize、不跑 check、不重写 md。
- `Patch` 语义通过 `MutationOptions.Mode` 区分 `merge` 与 `replace`。CLI 路径
  始终校验；命令层已经跑过自己的 quality/repair loop 时以 `Validate:false`
  提交，仍然经过 normalize、checkpoint 和 store 原子写。
- 读-改-写由 `store.Root.WithMutationLock` 串行化，避免并发 worker 在
  craft/outline 上丢失更新。跨进程写入仍需要后端提供 CAS 或项目锁。

canonical 产物（setup/outline/craft/chapter/recap/draft）的命令层直写由
[`internal/architecture/mutation_ownership_test.go`](../internal/architecture/mutation_ownership_test.go)
守卫；派生 markdown、日志、评审报告、导出文件、compose 的中间 progress
快照不属于这条 mutation 边界。

---

## 6. 领域模型与 RPG/DSL

### 6.1 模型层次（务必区分）

| 层 | 位置 | 角色 |
| --- | --- | --- |
| **持久化模型（唯一事实源）** | [`internal/contract/story`](../internal/contract/story/) | `StorySetup` / `Outline` / `Elements` / `Recap` / `StateMatrix` 等，落盘为 `story/**` 的 JSON |
| **RPG 引擎模型** | [`internal/fiction/rpg`](../internal/fiction/rpg/) | `GameWorld`/`Character`/`Item`/`Map`/`Quest`… 由 models 派生，用于模拟与基准 |
| **DSL AST** | [`internal/fiction/rpg/dsl`](../internal/fiction/rpg/dsl/) | `ast.go`/`parser.go`/`validator.go`/`converter.go`/`simulator.go`/`merger.go`/`hook*.go` |

方向约定：**models 是真相，RPG GameWorld 与 DSL AST 都是派生视图**。转换入口：

- `models → GameWorld`：`internal/fiction/rpg/novelgen_adapter.go`（`NovelgenProject.ConvertToRPG`）、`internal/fiction/rpg/story_adapter.go`（`ConvertOutlineToRPG`）
- `GameWorld → models`（导出回写）：`internal/fiction/rpg/shared_models.go`（`SharedModelConverter.Export*`）
- `models → DSL AST`：`internal/fiction/rpg/dsl/model_adapter.go`
- `DSL AST → GameWorld`：`internal/fiction/rpg/dsl/converter.go`

### 6.2 DSL 流程

```
AI 分析章节/大纲
  → story/rpg/{01_outline,02_craft,03_systems,04_chapters}.rpg
  → merger（分阶段合并）
  → parser → AST
  → validator（引用/类型/完整性）
  → converter → GameWorld
  → simulator（问题检测）
  → 约束系统 → 反馈给写作 Agent
```

### 6.3 连续性 / 状态

- [`internal/fiction/domain/logic/state_matrix.go`](../internal/fiction/domain/logic/state_matrix.go) 的 `StateMatrixManager` 是**活跃**的状态折叠引擎，会把本章事件与 DSL `state_delta` 折进 `models.StateMatrix`。
- [`internal/fiction/domain/logic/chapter_continuity.go`](../internal/fiction/domain/logic/chapter_continuity.go) 的 `ChapterContinuityBuilder` 在其上生成写作面向的连续性快照。
- `models.StateMatrix` 是数据，`logic.StateMatrixManager` 是计算——二者是类型与逻辑的分工，不是重复实现。

---

## 7. 项目数据布局（`novelgen init` 生成）

```
<book>/
├── novel.json                     # 项目配置
├── llm_config.json                # 项目级 LLM 配置（可缺省，落到全局）
├── story/
│   ├── setup/story_setup.json|md
│   ├── compose/outline.json
│   ├── craft/{characters,locations,items,organizations}.json
│   ├── recaps/<chapter_id>.json
│   ├── reviews/
│   └── rpg/*.rpg
├── drafts/C{n}.md                 # legacy（write 直接产出最终稿）
├── chapters/chapter-{n}.md
├── logs/{prompts,responses,agent-live,write_contexts,draft_contexts}.md 日志流
│        ai_calls/<id>.json（每次 AI 调用一条结构化记录：model/tokens/两个 prompt/response）
│        usage.jsonl（成本账本）、clone_manifest.json
└── story/{setup,compose,craft,recaps}/checkpoints/   # patch 前快照
```

**这份布局只由 [`internal/platform/store`](../internal/platform/store/paths.go) 描述，并由
[`internal/architecture/layout_test.go`](../internal/architecture/layout_test.go) 强制**。四条守卫测试覆盖四类漂移：

1. **目录与文件名**：根模块非测试 Go 文件里出现 `story/...`、`chapters/...`、`drafts/...`、`logs/...`、
   分阶段 `0N_*.rpg` 文件名，或 `novel.json`/`story_setup.json`/`outline.json`/四个 craft 文件名/
   `usage.jsonl`/`clone_manifest.json` 这些**文件名**字面量就直接失败（`llm_config.json` 例外：同一个名字也是
   用户全局配置 `~/.novelgen/llm_config.json`，不归 store 所有）。
2. **IO 归属**（`TestProjectIOGoesThroughStore`）：把 store 给出的路径再喂给 `os.ReadFile/WriteFile/OpenFile/
   Stat/ReadDir/MkdirAll/Remove/Rename` 会失败——项目状态只能走 `Root.Read/Write/Append/FS()`。允许清单
   （`projectIOAllowlist`）**故意为空**：操作主机路径的代码（全局配置、agent home、用户的输出路径、
   `project clone` 的整树复制）根本不会把 store 路径递给 `os.*`，规则也就看不见它们；将来真需要例外时才加条目，
   且一旦该条目不再命中，测试会要求删掉它。
3. **环境根**（`TestAmbientProjectRootUseIsPinned`）：层表里第 5 层及以上的包调用 `store.Default()`/
   `project.Default()`，或调用 `project.LoadConfig/LoadSetup/.../FindRoot` 这类内部会落到 ambient root 的
   便利函数，都会失败，除非该文件在允许清单里（只剩 5 处：`internal/platform/llm/{config,client}.go`、
   `internal/fiction/agents/base/{agent_base,agent_logging}.go`、`internal/fiction/derive/write_rpg_dsl.go`）。新代码要显式接收
   `*store.Root`；允许清单条目一旦不再命中同样会失败（防止清单腐化）。
4. **webui 平行副本**（`TestWebUILayoutMatchesStore`）：webui 不能 import 父模块（§8），它手抄的项目相对
   路径若与 store 产出的路径前缀不符即失败，把"改布局要两边一起改"从口头约定变成会失败的测试；`logs/<流名>`
   的第一级还要落在 `store.LogStreamNames()` 声明的集合里，所以改名不会留下一个读空目录的页面。
   跨模块的**记录格式**（不是路径）由 `internal/architecture/schema_test.go` 盯住：它逐个比较
   `internal/fiction/agents/base.aiCallRecord` 与 `webui.aiCallLogFile` 的 JSON 标签，任一边改名都会失败。

任何需要项目内路径的代码都通过 `*store.Root`（或它上面的
[`internal/fiction/domain/project.Book`](../internal/fiction/domain/project/project.go)）取得；已持有目录字符串的调用方用 `store.At(dir)`
换到同一个 Root，不再自己拼路径。磁盘写走 `Root.Write`/`WriteText`/`WriteJSON` 的"同目录临时文件 + rename"
原子替换（`FS` 另提供 `WriteFileExclusive`/`OpenAppend` 供日志抢名与流式写入），因此中断的
`outline.json`/`story_setup.json`/章节正文/派生 `04_chapters.rpg` 写入不会留下半截文件——包括
`tool patch setup|outline --apply` 这两条 agent 写入路径（它们此前用裸 `os.WriteFile`）。

**后端注入与契约**：项目级入口由 [`internal/platform/store/backend.go`](../internal/platform/store/backend.go) 的
`Backend` 接口负责。默认实现是 `HostBackend`（OSFS）；测试或宿主程序可以用 `store.SetBackend` 安装
MemFS/远程实现，CLI 的 `--dir` 解析、`store.Default()` 和 `project.CommandDir()` 都走
`OpenProject`/`LocateProject`，不需要改命令行代码。`FS` 仍是文件语义接口（`Rename`、独占创建、
`OpenAppend`、`Stat.ModTime`），所以对象存储/数据库后端必须显式实现这些语义；`TestFSContract` 用同一组
断言跑 OSFS 和 MemFS，覆盖覆盖写、独占创建、追加、目录列举与替换式 rename。

项目内部管道已改为显式传递 `*store.Root`：`writersdl.EmitChapterRPGDSLForProject`、toolplane 的
chapter check/refresh、`internal/fiction/domain/logic` 的 state matrix/continuity、`internal/fiction/rpg` 的
`LoadNovelgenProjectAt` 都直接使用调用方传入的 root。字符串 root 的导出函数只保留在命令边缘和兼容测试中，
并标注为 host-directory 入口；新增 pipeline 代码不要再用 `root.Dir()` 重新构造 `store.At`。

快照（checkpoint/backup）与日志流都用"秒级时间戳 + `WriteFileExclusive` 抢名（冲突时加数字后缀）"，
同一秒内的两次 patch 不会互相覆盖；`checkpoints/`、`backups/` 只增不删，删掉它们不影响 CLI 读取，
但它们是唯一的改前状态记录，所以 CLI 不会自动清理。

章节正文的候选文件名（`chapters/chapter-<id>.md` → 兼容的数字名 → `chapters/<id>.md`）由
`store.Root.ChapterCandidates` 单点定义，写入流水线、导出、DSL 转换与 agent 工具面共用同一份；
分阶段 DSL 文件名由 `store.RPGPhaseFiles()`／`store.RPGPhaseChaptersFile` 等常量给出。

项目根的解析顺序（[`cmd/root.go`](../cmd/root.go) 的 `PersistentPreRunE`）：

1. `--dir <目录>`（显式指定，目录必须存在）；
2. `NOVELGEN_PROJECT_DIR` 环境变量；
3. 从工作目录向上查找 `novel.json`。

解析结果同时安装到命令上下文（`store.WithRoot`）与进程默认根（`store.SetDefault`），所以
`compose`/`setup`/`craft`/`translate`/`tool` 等命令在子目录里运行、或用 `--dir` 指向别处时行为一致。
`init` 是唯一的例外：它用同一套目标目录，但允许那里还没有 `novel.json`。

---

## 8. Web UI（`webui` module）

- Gin HTTP 服务 + WebSocket（[`webui/main.go`](../webui/main.go)），前端为 React + Vite（`webui/frontend/`），由 `frontend/dist` 静态托管。
- 只读端点通过 `exec` 调用 `novelgen storage projects/stat/read/list`，编辑器保存调用 `novelgen storage write`；这些命令内部走 `store.Backend`，所以 webui 不需要知道 OSFS/MemFS/远程实现。**派生文件与长任务仍通过 `exec` 调用 `novelgen` 二进制**（例如保存大纲后调用 `novelgen compose normalize --force` 重写 `outline.md`）。AI 调用查看器（`AICallsViewer`）读两类来源：CLI 默认写的 `logs/prompts`↔`logs/responses` markdown 配对，以及每次调用一条的 `logs/ai_calls/<id>.json` 结构化记录（由 `internal/fiction/agents` 写，字段含 model/tokens/耗时；两边的 JSON 契约由 `internal/architecture` 的 schema 测试盯住）。
- webui **不 import 父 module 的任何包**，连 `internal/contract/story` 也没有：所有项目内容访问都经过 `novelgen storage ...` 的 JSON 协议。它仍自己声明项目相对路径（约 50 处），所以这部分布局知识与 `internal/platform/store` 是**并行维护**的；`TestWebUILayoutMatchesStore` 现在同时检查 `filepath.Join` 链和带分隔符的路径字面量。章节 id 目前由文件名派生（`chapters/<id>.md` → id 去掉 `.md`），所以对外的 id 是文件名而不是大纲里的章节 ID。
- 本机访问加固在 [`webui/security.go`](../webui/security.go)。

**集成边界（由 `webui/boundary_test.go` 强制）**：webui 不得 import `novelgen/cmd/**` 或 `novelgen/internal/**`；项目内容读写走 `novelgen storage` CLI 命令，派生文件与长任务走 CLI `exec`。二进制位置可用 `NOVELGEN_BIN` 覆盖。

---

## 9. 配置

| 配置 | 位置 |
| --- | --- |
| 项目 | `<book>/novel.json` |
| LLM | `<book>/llm_config.json` 或 `~/.novelgen/llm_config.json` |
| Agent 运行时 | `~/.novelgen/agent_config.json`（缺失时回退 `~/.claude/settings.json`） |
| Agent 资产 | `~/.novelgen/agents/{runners,skills,settings}` |

行为调参通过 `NOVELGEN_*` 环境变量（timeout / max-turns / retry / 日志开关等）。

---

## 10. 已知架构债（待收敛）

1. **文档需持续同步**：本文已重写以匹配现状，请随代码同步维护。
2. **依赖方向与布局归属已强制**：[`internal/architecture`](../internal/architecture/layers_test.go) 的测试同时约束五件事——① internal 包只能向下依赖（层表 `layer`）；② internal 包不得依赖 `novelgen/cmd/**`；③ `cmd/<group>/` 之间不得互相 import，且命令子包只能由根 `cmd` 包的 `*_register.go` 接线；④ 项目布局字面量只准出现在 `internal/platform/store`（[`layout_test.go`](../internal/architecture/layout_test.go)）；⑤ 模块图 `contract ← platform ← fiction ← toolplane ← app` 以及 `fiction` 内的 `rpg < domain < check < mutation < agents < derive < refresh < pipeline` 顺序由 [`modules_test.go`](../internal/architecture/modules_test.go) 强制。新增 internal 包必须同时在层表登记并通过模块检查。`internal/platform/logger` 因需要写 `logs/` 被放在 store 同一层（第 2 层），其余工具包仍在第 1 层。
3. **`cmd` 已按组拆完**：`config`/`translate`/`initcmd`/`export`/`recap`/`analyzecmd`/`simulatedsl`/`rpgcmd`/`craftcmd`/`draftcmd`/`setupcmd`/`composecmd`/`writecmd`/`polishcmd`/`projectcmd`/`rpgdsl`/`tool` 都是独立子包；根 `cmd` 包只保留 `root.go`、`registry.go` 与接线文件。
4. **双执行路径的脚手架已部分收敛**：Agent SDK 调用的公共形状（Bash-only 工具面、`dontAsk` 权限、`RequireSDK`）与 allowlist 去重规则由 [`internal/fiction/agents/base/agent_sdk_params.go`](../internal/fiction/agents/base/agent_sdk_params.go) 统一；章节 check/refresh 命令串（原先手拼二十余处）由 [`internal/contract/toolcmd`](../internal/contract/toolcmd/commands.go) 统一，toolplane 的导航输出与 agents 的指令共用同一构造器；非 ASCII patch 规则与"唯一允许写入"子句抽到 [`base/agent_sdk_instructions.go`](../internal/fiction/agents/base/agent_sdk_instructions.go)；provider 名称与"是否走 Agent SDK"的判定收敛到 [`internal/contract/story/providers.go`](../internal/contract/story/providers.go) 的 `ProviderXxx` 常量与 `IsAgentSDKProvider`（agents / llm / init / setup 均复用它）。仍各自实现的是各阶段的 tool 命令清单与 patch/check 文案主体、review 结果转换（`toModelReviewResult`），是下一步的收敛目标。
5. **并发已统一（基本完成）**：craft / recap / draft / polish / write / compose 的章节与批次循环已全部改用 [`internal/platform/workerpool`](../internal/platform/workerpool/workerpool.go) 的 `Map`（结果按输入顺序返回，去掉了各处手写的 `WaitGroup+Mutex+channel`）。仅 write 的 best-of 生成/评分子流程仍是自定义 `WaitGroup`，因为它不是“按项独立”的扇出。
6. **webui 已收敛为单一集成路径**：项目内容通过 `novelgen storage` 命令读取或写回，派生文件与长任务通过 `exec` 调用 CLI；webui 不 import 父 module 的任何包（`boundary_test.go` 强制）。为支持这条路径，`compose normalize` 新增 `--force`（无规范化改动时也重写派生文件），二进制可用 `NOVELGEN_BIN` 指定。
7. **write/polish 共享逻辑已下沉**：章节上下文加载、生成/改进、post-save 检查与 recap 门禁从 `cmd/writecmd` 抽到 [`internal/fiction/pipeline`](../internal/fiction/pipeline/)，`polishcmd` 不再依赖 `writecmd`。包内按 `context.go` / `improve.go` / `postsave.go` / `recap.go` 分文件。
8. **`internal/fiction/rpg/dsl` 有尚未接线的 Phase 2 组件**：`hook*.go`、`evaluator.go`、`functions_enhanced.go`、`converter.go`、`errors_enhanced.go`、`logger.go`、`parser_enhanced.go` 目前没有被生产代码调用（仅有 placeholder 测试覆盖），但它们是 [RPG_DSL_SPEC.md](RPG_DSL_SPEC.md) 里 Phase 2 的既定交付物。**接入前不要把它们当作可用能力**；若决定不做，应连同文档一起删除。
9. **`internal/fiction/rpg` 引擎仍有一批 CLI 未使用的方法**（如 `GameWorld.RemoveItemFromPlayer`、`MapManager.MoveEntity`、`QuestManager.FailQuest`）：引擎本身是文档化的分层组件，这些方法属于游戏运行时 API 面。这里选择保留而非删除，接入新玩法时再评估。（真正无人使用的重复实现已清理：`SharedModelConverter` 的 models→GameWorld 导入半边和孤立的 `SkillInstance` 类型。）
10. **测试已归位到被测包**：原先堆在 `cmd/` 的 7k+ 行测试已迁到 `internal/toolplane`（151 个用例）、`internal/fiction/domain/quality`、`internal/fiction/pipeline`、`internal/fiction/derive`、`internal/fiction/domain/project`、`cmd/writecmd`，`cmd/` 只保留命令面（`command_surface_test.go`）与文档一致性（`agent_tools_docs_test.go`）两类测试。跨包共享的测试夹具抽到 `internal/contract/testfixtures`；`internal/fiction/domain/continuity` 及其 `logic/continuity/{character,transition}` 子包补上离线 recap、转场/角色启发式与 fix helper 的用例；`internal/fiction/derive` 补上批 manifest/缓存失效、章节 ID 归一、合并去重、issue 过滤与分阶段文件名推断（`batch_test.go`）；`cmd/export`、`cmd/initcmd`、`internal/fiction/rpg/llm` 也补了包内用例。
11. **`internal/contract/story` 无法复用 `internal/platform/utils`**（层规则里 models 是第 0 层），因此 `outline_normalizer.go` 保留了自己的 `compactStrings`。这是刻意的分层结果，不是遗漏。
12. **工具与内部流水线已统一**：`tools/check_novel` 原先自带一份章节→DSL 批处理/缓存的旧副本，现在改为调用 [`writersdl.EmitWriteRPGDSL`](../internal/fiction/derive/emit.go)（新增的共享入口，`EmitChapterRPGDSLForProject` 也复用它），该工具从 1113 行降到 597 行，并自动获得 manifest 章节校验与战斗信号增强。同一轮还把 `dsl.ApplySimulationDefaults`（simulatedsl 与 check_novel 各一份）和 `models.ReviewSuggestionPriorityRank`（quality 与 agents 各一份）收敛为单一实现。
13. **仍存的少量平行实现**（有意保留）：各 `tools/*` 自带的 `printHelp`/`printSummary` 等打印函数；`models.Event` 与 `rpg.StoryEvent` 各自的旧格式事件适配（服务于两套不同的模型，需同步维护）；`toolplane.reviewPriorityRank` 与 models 版本签名不同（前者区分未知优先级）。
14. **静态检查基线**：根模块与 webui 的 `go vet ./...` 与 `staticcheck ./...` 均为干净，唯一剩余的是第 8 条里 DSL Phase-2 脚手架的 4 个 `U1000`。新增告警请当场处理，别让基线退化——这一轮就是靠它翻出 `draft improve` 丢弃生成稿、`fmt.Errorf` 动态格式串、JSON 提取被字符串内 `}` 截断等问题。
15. **小说 storage 已收敛**：新增 [`internal/platform/store`](../internal/platform/store/)，磁盘布局、原子写、检查点/备份、类型化产物读写都只此一处；`internal/fiction/domain/project` 退化为它上面的门面，`cmd/root.go` 负责解析一次根（`--dir` / `NOVELGEN_PROJECT_DIR` / 向上查找）。之前"同一份布局散在 47 个文件里、`compose` 用 cwd 相对路径而 `write` 用向上查找"的不一致已消除——`compose check` 现在在子目录与 `--dir` 两种场景下都可用（原实现要求 cwd 就是项目根）。`internal/contract/story` 现在只保留 `Parse*`/`MarshalForSave` 与**路径入参**的便利读写（不含布局知识），`FindProjectRoot` 移入 store。测试不再靠 `os.Chdir` 定位项目：改为向命令上下文注入 root（`bindProject`）或安装进程默认根（`useProjectRoot`），全仓只剩 `cmd/rpgdsl` 一处 chdir，因为 `rpg-dsl` 的契约本身就是"在含 `books/` 的工作区里运行"，那不是项目根、store 无法表达。

   **后续一轮把残留的平行实现也并回 store**（此前本节曾写成"已消除"，实际还有二十余处）：`internal/toolplane` 全部改用 `*store.Root`（含章节正文读取、craft 读写与 checkpoint），`internal/fiction/derive`（含 `04_chapters.rpg`、批缓存与 manifest 改走原子写）、`internal/fiction/domain/logic/state_matrix.go`、`internal/platform/logger`、`internal/platform/agentruntime`、`internal/fiction/agents`、`cmd/{export,projectcmd,rpgdsl,writecmd}`、`tools/check_novel`、`internal/fiction/rpg/{benchmark,dsl}` 都改为向 store 取路径。这一轮还修掉了三处真实分歧：① 工具面自带一份章节候选文件名（缺数字名回退、多认 `drafts/`），与 write/export 读取的可能是不同文件；② `tool patch chapter --apply` 与 `rpg-dsl` 导出直接 `os.WriteFile`，绕过原子替换；③ `RunToolChapterCheckWithTargetWords` 走 `project.LoadFinalChapterContent`（进程默认根），注入 root 的调用会读到别的项目。剩下的"平行维护"只有 webui（§8 的边界所致）。
16. **RPG 侧的工作区假设已消除（本轮完成）**：`rpg.LoadNovelgenProject` 以前签名是 `(工作区根, 书名)` 并硬编码 `<工作区>/books/<书名>/story/...`，导致任何不在 `books/<name>` 下的项目（包括 `--dir` 指定的）都加载失败；现在它只接受**项目目录**，路径由 `store` 给出，书名取 `novel.json` 的 `name`（缺失才退回目录名）。调用方（`cmd/rpgcmd`、`cmd/rpgdsl`、`internal/fiction/agents/chapter/chapter_to_dsl_adapter.go`、`tools/novelgen-rpg`、`tools/rpg-write`）都只传目录；`internal/fiction/rpg/dsl` 的 setup 查找同样改用 store。`cmd/rpgcmd` 仍保留 `<workspace>/output/rpg_data.json` 这份"顺手再存一份"的行为，但只在项目确实位于 `books/<name>` 下时才写，不再往凭空推导的路径写文件。RPG 侧现在也容忍 BOM。
17. **仍是两套寻址约定**：项目命令与 `rpg`/`rpg-dsl`/`tools/*` 都按给定目录工作（`rpg-dsl`/`tools/*` 仍以 `<workspace>/books/<name>` 作为**默认**目录，字面量收敛到 `store.BooksDirName`/`store.BookDir`），webui 则自己拼只读路径（见 §8）。`logger` 现在既支持 `SetProjectDir`（host 边界）也支持 `SetProjectRoot`；`cmd/writecmd` 用后者把 `book.Root` 绑定给文件日志，MemFS 项目不会再把日志写到宿主磁盘。`logger.ProjectDir()` 只是"当前项目目录"的兼容缓存；agents/llm 的 agent-home 与 live-log 仍按"logger → `store.Default()` → cwd"的顺序解析（`agentLogRoot`/`agentWorkspaceRoot`/`runtimeWorkspaceRoot`），这是外部进程边界。

18. **storage 抽象的两半（本轮收口）**：布局所有权这一半此前已成立，IO 这一半只做了一半——`FS`/`MemFS` 除了 store 自己的测试以外没有任何使用者，因为项目状态的读写仍在各处走 `os.*`。本轮按 §7 的四条守卫把两半对齐：原子写覆盖 `tool patch setup|outline --apply`（此前走 `models.*.Save`/`project.SavePartialOutline` 的裸 `os.WriteFile`）、派生 markdown、compose 报告与 improve progress；`FS` 补上 `WriteFileExclusive` 与 `OpenAppend`，于是快照/agent 日志/运行日志改用 O_EXCL 抢名（同一秒不再互相覆盖，agent 日志的手写 64 次重试循环删除），writer 自己的 temp+rename 实现并回 store；`MemFS` 补上修改时间与独占创建，三处"比较新旧"的判断（章节 vs DSL、recap vs 章节、章节 vs recap）在内存后端下行为一致。同时把"读不到"从"不存在"里分出来：`LoadChapterContent`/`LoadDraftContent`/`LoadAllDrafts`/`LoadSetupAt` 返回错误，只有 `fs.ErrNotExist` 才表示"没有"，`WriteJSONMerging` 不再把读失败当成空文件合并（那会悄悄丢掉其余键），`BackupFile`/`loadCraftFile` 同理；确实需要"没有就算了"的上下文构造点（continuity/writepipe 的上下文加载、write 流水线的启发式检查）显式忽略错误或记日志。

   同一轮还把 seam 从"存在"推进到"被证明"：`internal/toolplane`（`TestToolPlaneRunsOnAnInMemoryProject`）与
   `internal/fiction/derive`（`TestEmitWriteRPGDSLRunsOnAnInMemoryProject`）各有一个端到端用例，把 setup/craft patch、
   patch buffer、批次缓存与合并全部跑在 `store.MemFS` 上——这两个用例就是当初"内存后端没人能用"的反证，也各自
   逼出了两个真漏点（`RunToolPatchBuffer` 从命令上下文解析项目、`SaveWriteRPGDSLBatchToCache` 写缓存时绕过传入的 FS）。
   同时把隐式环境根从库层摘掉：`internal/fiction/pipeline`（章节上下文、post-save check、章节正文读写）、
   `internal/fiction/domain/review`（卷级 review）、`internal/fiction/domain/continuity`（前后章上下文、离线 recap 落盘）现在都要求调用方传入
   `*project.Book`，`cmd/writecmd`、`cmd/draftcmd`、`cmd/polishcmd` 在命令入口解析一次并向下传。

19. **仍未收口的部分（清楚记着）**：
    - **`logs/ai_calls` 的生产者已补上（本轮）**：`internal/fiction/agents/base.recordAICall` 在每次 AI 调用结束时（成功与失败都算）按 webui 既有的 `aiCallLogFile` schema 写一条 JSON，开关与 markdown prompt/response 日志共用 `NOVELGEN_AGENT_LOGS`；schema 由 `internal/architecture/schema_test.go` 与 webui 读取端逐字段对齐，`logs/ai_calls` 也已进入 `store.LogStreamNames()`，所以 webui 侧的路径与流名都被守卫覆盖。代价是每次调用多落一份 prompt+response（与 markdown 日志重复），关掉日志开关即可完全避免。
    - 日志流名字本身仍有开放面：webui 只列出它知道的流（现在都在 `store.LogStreamNames()` 里），但流内**文件**的命名约定（`<agent>_<timestamp>.md`、`<id>.json`）没有跨模块校验，只有 store 负责保证唯一性。
    - agent home（`~/.novelgen/agents`、Claude settings、runner 脚手架）不是项目布局，仍直接走 `os.*`；`internal/platform/agentruntime` 与 `internal/platform/llm` 也仍在环境根上取工作区（第 3 条守卫的允许清单）。
    - `FS` 只覆盖"整文件 + 独占创建 + 追加句柄"，没有权限/符号链接语义：`project clone` 这种整树复制、以及 agent-home 操作天生在 seam 之外。`Root.Walk` 提供递归列目录（日志面在用）。

20. **storage seam 可替换性（本轮）**：新增 `store.Backend` / `SetBackend` / `OpenProject` /
    `LocateProject`，CLI 根解析与 `store.Default` 都经过这层；默认仍是 `HostBackend`，因此现有行为不变。
    `writersdl`、toolplane check/refresh、`logic` 的 state matrix/continuity、`rpg.LoadNovelgenProjectAt`
    不再把 `*store.Root` 降级成 `Dir()` 再重建 OSFS；`writepipe` 的 post-save check、离线 recap 落盘、
    章节上下文都有 MemFS 端到端用例（并断言 ambient 项目没有被触碰）。`Root.Stat`/`ExistsErr` 保留
    "缺失 vs 读取失败"的区别，`ValidateKeyPart` 拒绝会逃出项目的 chapter/draft/recap key。
    `TestFSContract` 用同一组断言覆盖 OSFS 与 MemFS。webui 的读取/编辑器保存已改走
    `novelgen storage ...` 命令，不再直接 `os.ReadFile` 项目内容；项目发现、CLI `exec` 的工作目录与
    Agent SDK runner 仍需要宿主路径，这是外部进程边界。对象存储后端还必须自行实现
    `Rename`/`OpenAppend`/`ModTime` 以及 `WorkspaceLister`，并让 CLI 子进程指向同一个 backend。

21. **命令级组合根与 canonical 编辑器写入（本轮）**：新增 `internal/app.Session`，由它解析一次
    `*store.Root`、加载 `novel.json`、按需加载 setup/outline，并构造共享的 LLM client 与 agent；
    compose/write/craft/draft/polish/recap/translate/export/rpg/analyze 的入口已改为使用它，避免各命令重复
    `LoadConfig`/`LoadSetup`/`LoadOutline`/`LoadOrCreateConfig`/`CreateClient`。`ComposeAgent`
    现在由 `Session.NewComposeAgent` 强制绑定显式 root，checkpoint/report 不再走
    `agentWorkspaceRoot()`。

    新增 `novelgen storage replace`：编辑器保存 canonical 产物（setup/outline/craft/chapter/
    recap）时，不再直接 `storage write`，而是走同一个 `MutationService`，因此 schema 校验、
    checkpoint、normalize 与派生 markdown 仍然生效。webui 只对笔记/报告等非 canonical 文件使用
    `storage write`。`storage replace` 的 craft 路径使用整文件替换语义（被删除的条目会真正删除），
    与 `UpsertCraftElements` 的保留式合并区分开。

    另外把两处字符串分类收敛为 typed error：章节长度超限使用
    `agents.ErrAgentSDKChapterTooLong`，runner 进程失败使用 `agentruntime.RunError`
    （`IsTransient`），compose 的 transient retry 不再依赖 stderr 文案。`internal/architecture`
    新增了间接 ambient root 与 skill 引用存在性守卫。

    原 4.4k 行的 `compose_agent.go` 已按 concern 拆成 core / sdk / patch / loop /
    generate / prompt 六个文件，并由 `TestComposeAgentStaysSplitByConcern` 固定文件归属与
    单文件上限；原 2.8k 行的 `cmd/composecmd/compose.go` 也按 gen / improve / review /
    pipeline / state / support 拆开，由 `TestComposeCommandStaysSplitByWorkflow` 守卫。
    `craft` 的四种元素生成/改进循环也合并为泛型 batch runner。

    `store.Root.WithMutationLock` 的锁改为按项目目录注册，因此两个 `store.Open`/`At`
    句柄指向同一项目时也会串行；`agentruntime.Config.NewRuntimeWithResolver` 允许显式传入
    provider resolver，不再只能依赖 `internal/platform/llm` 的 init 全局注册。

    `BaseAgent` 现在也持有显式项目 root：SDK 的 `workspace_root`、agent prompt/response 日志、
    AI-call 记录与 usage 都优先使用绑定 root；`Session.New*Agent` 与命令层的直接构造点会统一
    调用 `SetProjectRoot`。只有未绑定的旧工具/测试才回退到 ambient 解析。

    runner 现在在结果 JSON 中返回结构化的 `tool_events`（permission / PreToolUse /
    PostToolUse）。Go 的 tool evidence 校验优先用 `agentruntime.SummarizeToolEvents` 计算
    query/check/patch 与 denied 计数，live JSONL 只作为旧 runner 的兼容回退，不再要求
    Go 从人类可读日志里反推工具行为。

---

## 11. 相关文档

- [README.md](../README.md) — 命令参考
- [STAGE_CONTRACTS.md](STAGE_CONTRACTS.md) — 各阶段数据契约（改动阶段行为前必读）
- [RPG_DSL_SPEC.md](RPG_DSL_SPEC.md) / [DSL_RPG_INTEGRATION_SPEC.md](DSL_RPG_INTEGRATION_SPEC.md)
- [RPG_WRITE_USAGE.md](RPG_WRITE_USAGE.md) / [DSL_INCREMENTAL_WORKFLOW.md](DSL_INCREMENTAL_WORKFLOW.md)
- [tools/README.md](../tools/README.md) — 独立工具的清单
