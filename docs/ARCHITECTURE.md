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
│   ├── tool.go tool_logs.go tool_view.go tool_brief.go   # Agent 工具面：查询/上下文（见 §5）
│   ├── tool_check.go tool_patch.go tool_refresh.go polish.go
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
│   └── *_register.go          # 子包命令的唯一接线点（RegisterCommand(... NewCommand())）
├── tools/                     # 独立开发/分析二进制（非主 CLI，见 tools/README.md）
│   ├── check_novel/ rpg-check/ rpg-write/ simulate/ visualize/
│   └── story2rpg/ validate-outline/ deep-analyze/ extract-*/ analyze-outline/
├── internal/
│   ├── agents/                # Agent 层 + 两套技能集
│   │   ├── agent_base.go      # BaseAgent：prompt 组装 / 调用 / 记账
│   │   ├── agent_json.go      # JSON 解析与确定性修复
│   │   ├── agent_evidence.go  # SDK 工具证据校验
│   │   ├── agent_logging.go   # prompt/response 落盘与 usage 记账
│   │   ├── setup_agent.go compose_agent.go craft_agent.go
│   │   ├── write_agent.go recap_agent.go translate_agent.go draft_agent.go
│   │   ├── chapter_analysis_agent.go chapter_to_dsl_agent.go
│   │   ├── skill_loader.go    # 加载并 embed internal/agents/skills
│   │   └── skills/*/SKILL.md  # 38 个 chat 路径阶段 prompt
│   ├── agentruntime/          # Agent SDK 进程运行时
│   │   ├── process_runtime.go # 通过 stdin/stdout 调用 runner
│   │   ├── config.go settings.go types.go
│   │   ├── runner_embed.go    # embed runner 与 SDK workflow skills
│   │   ├── runners/claude_runner.py     # Claude Agent SDK 适配进程
│   │   └── skills/*/SKILL.md            # 17 个 SDK workflow prompt
│   ├── llm/                   # LLM 集成
│   │   ├── config.go          # llm_config.json + provider 解析
│   │   └── client.go          # OpenAI 兼容客户端 + RuntimeClient + 重试
│   ├── models/                # 持久化数据模型（唯一事实源）
│   ├── project/               # 项目定位与状态加载
│   │                          #   project.go: FindRoot/LoadConfig/LoadStorySetup/LoadOutline/LoadAllElements/AllChapters/KnownCharacterNames
│   │                          #   chapters.go: 章节/草稿 IO + SaveFinalChapter + SelectChapters + VolumesForSelection
│   │                          #   chapter_markdown.go: 章节标题/正文规范化 + 叙事单元计数
│   ├── continuity/            # 章节连续性：上下文加载、离线 recap、转场/角色出场自动修复
│   ├── reviewstore/           # 卷级评审 JSON 读写 + 改进建议块格式化
│   ├── quality/               # setup/outline 确定性质量门（无 LLM）
│   ├── writersdl/             # 章节→RPG-DSL 生成、批缓存、信号增强
│   ├── rpgllm/                # LLM 驱动的 RPG 抽取适配（让 rpg 引擎不依赖 llm）
│   └── architecture/          # 分层依赖约束测试（internal 包导入方向）
│   ├── rpg/                   # RPG 引擎（GameWorld / 模拟 / 基准）
│   │   ├── dsl/               # DSL：AST/解析/校验/转换/模拟/合并/hook
│   │   └── benchmark/         # 基准与跨章检测
│   ├── logic/                 # 状态矩阵、连续性、ID、依赖、风格
│   │   └── continuity/{character,transition,recap}/
│   ├── logger/ usage/ utils/
├── webui/                     # 独立 module：本地 Web 服务 + 前端
├── docs/                      # 技术文档
├── configs/ examples/ scripts/
└── build.ps1 build.bat Makefile
```

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

**命令子包约定**：自包含的命令组可拆成 `cmd/<group>/` 子包（当前有 `config`、`translate`、`initcmd`、`export`）。子包导出 `NewCommand() *cobra.Command`，由 `cmd/<group>_register.go` 统一接线到 `RegisterCommand`。共享的项目定位/加载逻辑放在 [`internal/project`](../internal/project/project.go)，不再散落在 `cmd` 里。新增命令若只依赖 `internal/*`，优先放进子包。

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

每个创作阶段是一个 Agent，统一继承 [`internal/agents.BaseAgent`](../internal/agents/agent_base.go)：

- 从 `SKILL.md` 加载 system prompt（支持 `{{language}}` 等模板变量）。
- 把强类型 input 转成 markdown，追加 output JSON schema 要求。
- 调用后端、解析 JSON（含确定性修复与一次 JSON 修复重试）。
- 记录 usage 到 `logs/usage.jsonl`，并落调试用 prompt/response 文件。

### 4.1 两条执行路径

`BaseAgent` 在 `invokeAIWithRuntimeResult` 中二选一：

1. **Chat 路径**：`internal/llm.OpenAIClient`，标准 OpenAI 兼容 `/chat/completions`。用于非 SDK 的调用（例如 JSON 修复），以及未要求 SDK 的简单阶段。
2. **Agent SDK 路径**：`internal/agentruntime.ProcessRuntime`，通过 stdin/stdout 调用 [`claude_runner.py`](../internal/agentruntime/runners/claude_runner.py)（Claude Agent SDK）。`setup/compose/craft/write/recap/translate` 均以 `RequireSDK: true` 要求该路径。

运行时选择见 `resolveRuntime`：显式传入 > client 实现的 `Runtime()` > 按 provider（`claude`/`claude_sdk`/`agent`）加载 [`~/.novelgen/agent_config.json`](../internal/agentruntime/config.go)。

### 4.2 两套技能集（用途不同，不是重复）

两套技能集**用途不同、名称不重叠**，都通过 `go:embed` 打进二进制：

| 路径 | 数量 | 用途 |
| --- | --- | --- |
| [`internal/agents/skills/`](../internal/agents/skills/) | 38 | chat 路径的阶段 prompt（`setup-gen`、`compose-review`、`craft-characters` …） |
| [`internal/agentruntime/skills/`](../internal/agentruntime/skills/) | 17 | SDK workflow prompt（`write-chapter-workflow`、`outline-review-workflow`、`novel-tools-core` …） |

SDK skills 会在运行时物化到 `~/.novelgen/agents/skills/`；chat skills 直接读源码树旁的 `skills/` 或内嵌副本。

> 仓库里另有 `.agents/skills/` 与 `.trae/skills/`：那是给外部编辑器/助手（Trae 等）用的开发辅助技能，与运行时无关。

---

## 5. Agent 工具面（`novelgen tool`）

SDK Agent **不能直接读写项目文件**。它们只能通过 CLI 的 `tool` 子命令访问项目事实（见 [`novel-tools-core`](../internal/agentruntime/skills/novel-tools-core/SKILL.md)）：

| 命令 | 作用 |
| --- | --- |
| `tool query <story-setup\|outline\|craft\|chapter\|context\|logs>` | 只读查询，带 `--view index\|brief\|full`、`--limit`、`--fields` 预算控制 |
| `tool check <quality\|simulation\|all\|schema>` | 确定性校验 |
| `tool patch <outline\|craft\|setup\|recap\|chapter>` | 打补丁，**默认 dry-run** |
| `tool patch-buffer append/show/clear` | 大正文分块暂存 |
| `tool refresh [chapter-dsl]` | 重建派生数据 |

这保证了 AGENTS.md 的硬规则：**AI 输出不得直接成为项目状态**。Agent 只能提交 patch，由 [`cmd/tool_patch.go`](../cmd/tool_patch.go)（落盘）与 [`cmd/tool_check.go`](../cmd/tool_check.go)（校验）负责 merge、保留 ID、normalize、schema/quality/simulation 校验，失败即拒绝，`--apply` 才落盘。

---

## 6. 领域模型与 RPG/DSL

### 6.1 模型层次（务必区分）

| 层 | 位置 | 角色 |
| --- | --- | --- |
| **持久化模型（唯一事实源）** | [`internal/models`](../internal/models/) | `StorySetup` / `Outline` / `Elements` / `Recap` / `StateMatrix` 等，落盘为 `story/**` 的 JSON |
| **RPG 引擎模型** | [`internal/rpg`](../internal/rpg/) | `GameWorld`/`Character`/`Item`/`Map`/`Quest`… 由 models 派生，用于模拟与基准 |
| **DSL AST** | [`internal/rpg/dsl`](../internal/rpg/dsl/) | `ast.go`/`parser.go`/`validator.go`/`converter.go`/`simulator.go`/`merger.go`/`hook*.go` |

方向约定：**models 是真相，RPG GameWorld 与 DSL AST 都是派生视图**。转换入口：

- `models → GameWorld`：`internal/rpg/shared_models.go`、`novelgen_adapter.go`
- `models → DSL AST`：`internal/rpg/dsl/model_adapter.go`
- `DSL AST → GameWorld`：`internal/rpg/dsl/converter.go`

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

- [`internal/logic/state_matrix.go`](../internal/logic/state_matrix.go) 的 `StateMatrixManager` 是**活跃**的状态折叠引擎，会把本章事件与 DSL `state_delta` 折进 `models.StateMatrix`。
- [`internal/logic/chapter_continuity.go`](../internal/logic/chapter_continuity.go) 的 `ChapterContinuityBuilder` 在其上生成写作面向的连续性快照。
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
└── logs/{prompts,responses,agent-live,usage.jsonl,write_contexts}/
```

---

## 8. Web UI（`webui` module）

- Gin HTTP 服务 + WebSocket（[`webui/main.go`](../webui/main.go)），前端为 React + Vite（`webui/frontend/`），由 `frontend/dist` 静态托管。
- 通过 `exec` 调用 `novelgen` 二进制执行任务，**同时**直接 `import novelgen/internal/models` 读取项目数据。
- 本机访问加固在 [`webui/security.go`](../webui/security.go)。

**集成边界（由 `webui/boundary_test.go` 强制）**：读操作 in-process 用 `novelgen/internal/models`（只读透传端点保持原始 JSON）；写与长任务只走 CLI `exec`；webui 不得 import `novelgen/cmd/**` 或 `models` 之外的 `internal/*`。

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
2. **`cmd` 仍是单包**（约 2.7 万行）。已把 `tool.go` 拆成 `tool.logs/view/brief`，`tool_check_patch.go` 拆成 `tool_check.go`/`tool_patch.go`，`BaseAgent` 拆成 `agent_base/json/evidence/logging`；但都还在同一个 Go package 内，尚未拆成子包（存在共享全局状态，需要更大的重构）。
3. **依赖方向未强制**：`agents → rpg/logic`、`logic → rpg/dsl`、`rpg → llm`、`llm → agentruntime`。缺少不可穿透的分层约束。
4. **两套执行路径**：chat 与 Agent SDK 并存，provider 判定分散在多处。
5. **并发已统一（基本完成）**：craft / recap / draft / polish / write / compose 的章节与批次循环已全部改用 [`internal/workerpool`](../internal/workerpool/workerpool.go) 的 `Map`（结果按输入顺序返回，去掉了各处手写的 `WaitGroup+Mutex+channel`）。仅 write 的 best-of 生成/评分子流程仍是自定义 `WaitGroup`，因为它不是“按项独立”的扇出。
6. **webui 有两条集成路径**：既 `exec` 调用 CLI，又直接 `import novelgen/internal/models` 读数据，边界不清晰。
7. **`cmd` 正在按组拆子包（进行中）**：共享的项目定位/加载、章节/草稿 IO、章节正文规范化抽到 [`internal/project`](../internal/project/project.go)；连续性/自动修复抽到 [`internal/continuity`](../internal/continuity/continuity.go)；卷级评审 IO 抽到 [`internal/reviewstore`](../internal/reviewstore/reviewstore.go)。已拆出的子包有 `config`/`translate`/`initcmd`/`export`/`recap`/`analyzecmd`/`simulatedsl`/`rpgcmd`/`craftcmd`/`draftcmd`（`NewCommand()` + `*_register.go` 接线）。剩余命令（setup/compose/write/project/rpg-dsl/tool/polish 等）仍留在 `cmd` 包内，下一步要下沉的簇：① quality-gate 簇（`quality_gate.go` + `runOutlineValidatorOnModel`）→ `internal/quality`；② 工具面 `queryLogs`/`toolResponse`/`toolCheckResult` → `internal/toolplane`；③ `write_rpg_dsl.go` 批处理 helper；④ write/polish 的生成与修复 helper（`generateImprovedChapter`/`resolveAgentAppliedChapterContent`/`extractAndSaveRecapWithGate`/`emitChapterRPGDSLForProject` 等）。

---

## 11. 相关文档

- [README.md](../README.md) — 命令参考
- [STAGE_CONTRACTS.md](STAGE_CONTRACTS.md) — 各阶段数据契约（改动阶段行为前必读）
- [RPG_DSL_SPEC.md](RPG_DSL_SPEC.md) / [DSL_RPG_INTEGRATION_SPEC.md](DSL_RPG_INTEGRATION_SPEC.md)
- [RPG_WRITE_USAGE.md](RPG_WRITE_USAGE.md) / [DSL_INCREMENTAL_WORKFLOW.md](DSL_INCREMENTAL_WORKFLOW.md)
- [tools/README.md](../tools/README.md) — 独立工具的清单
