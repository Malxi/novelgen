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
│   ├── composecmd/ setupcmd/  # 大纲 / 设定命令（NewCommand()）
│   ├── writecmd/ polishcmd/   # 写作 / 打磨命令（NewCommand()）
│   ├── projectcmd/            # 项目管理命令（NewCommand()）
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
│   ├── store/                 # 项目存储（唯一知道磁盘布局的地方，见 §7）
│   │                          #   store.go: Root/Open/Locate/Default + 上下文与默认根
│   │                          #   fs.go: FS 接口 + OSFS + 原子写；memfs.go: 内存后端（测试与未来后端）
│   │                          #   paths.go: 全部路径方法；artifacts.go: 类型化读写；snapshots.go: checkpoints/backups
│   ├── project/               # store 之上的项目视图（门面）
│   │                          #   Book: 显式 root 的章节/草稿/加载 API；ForCommand/CommandDir: 从命令取根
│   │                          #   project.go/chapters.go: 兼容的自由函数（走 store.Default()）
│   │                          #   chapter_markdown.go: 章节标题/正文规范化 + 叙事单元计数
│   ├── continuity/            # 章节连续性：上下文加载、离线 recap、转场/角色出场自动修复
│   ├── reviewstore/           # 卷级评审 JSON 读写 + 改进建议块格式化
│   ├── quality/               # setup/outline 确定性质量门（无 LLM）
│   ├── writersdl/             # 章节→RPG-DSL 生成、批缓存、信号增强
│   ├── rpgllm/                # LLM 驱动的 RPG 抽取适配（让 rpg 引擎不依赖 llm）
│   ├── toolplane/             # Agent 工具面：query/check/patch/brief/view/refresh + 类型
│   ├── writepipe/             # write/polish 共享的章节生成·改进·post-save·recap 门禁逻辑
│   ├── testfixtures/          # 仅测试使用的共享夹具（quality/toolplane 等复用）
│   ├── toolcmd/              # `novelgen tool ...` 命令串的规范构造（toolplane 与 agents 共用）
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

- `models → GameWorld`：`internal/rpg/novelgen_adapter.go`（`NovelgenProject.ConvertToRPG`）、`internal/rpg/story_adapter.go`（`ConvertOutlineToRPG`）
- `GameWorld → models`（导出回写）：`internal/rpg/shared_models.go`（`SharedModelConverter.Export*`）
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
├── logs/{prompts,responses,ai_calls,agent-live,write_contexts,usage.jsonl,clone_manifest.json}
└── story/{setup,compose,craft,recaps}/checkpoints/   # patch 前快照
```

**这份布局只由 [`internal/store`](../internal/store/paths.go) 描述。** 任何需要项目内路径的代码都通过
`*store.Root`（或它上面的 [`internal/project.Book`](../internal/project/project.go)）取得，不再自己拼
`story/...`。磁盘写在 `Root.Write` 里统一走"同目录临时文件 + rename"的原子替换，因此中断的
`outline.json`/`story_setup.json` 写入不会留下半截文件。

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
- 只读端点直接读取项目文件并透传原始 JSON；**所有写、派生文件与长任务都通过 `exec` 调用 `novelgen` 二进制**（例如保存大纲后调用 `novelgen compose normalize --force` 重写 `outline.md`）。
- webui **不 import 父 module 的任何包**（连 `internal/models` 也没有）：它自己拼只读路径（约 50 处），因此这部分布局知识与 `internal/store` 是**并行维护**的，改动项目布局时必须两边一起改。章节 id 目前由文件名派生（`chapters/<id>.md` → id 去掉 `.md`），所以对外的 id 是文件名而不是大纲里的章节 ID。
- 本机访问加固在 [`webui/security.go`](../webui/security.go)。

**集成边界（由 `webui/boundary_test.go` 强制）**：webui 不得 import `novelgen/cmd/**` 或 `novelgen/internal/**`；写与派生文件只走 CLI `exec`。二进制位置可用 `NOVELGEN_BIN` 覆盖。

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
2. **依赖方向已强制**：[`internal/architecture`](../internal/architecture/layers_test.go) 的测试现在同时约束三件事——① internal 包只能向下依赖（层表 `layer`）；② internal 包不得依赖 `novelgen/cmd/**`；③ `cmd/<group>/` 之间不得互相 import，且命令子包只能由根 `cmd` 包的 `*_register.go` 接线。新增 internal 包必须在层表登记，否则测试直接失败。
3. **`cmd` 已按组拆完**：`config`/`translate`/`initcmd`/`export`/`recap`/`analyzecmd`/`simulatedsl`/`rpgcmd`/`craftcmd`/`draftcmd`/`setupcmd`/`composecmd`/`writecmd`/`polishcmd`/`projectcmd`/`rpgdsl`/`tool` 都是独立子包；根 `cmd` 包只保留 `root.go`、`registry.go` 与接线文件。
4. **双执行路径的脚手架已部分收敛**：Agent SDK 调用的公共形状（Bash-only 工具面、`dontAsk` 权限、`RequireSDK`）与 allowlist 去重规则由 [`internal/agents/agent_sdk_params.go`](../internal/agents/agent_sdk_params.go) 统一；章节 check/refresh 命令串（原先手拼二十余处）由 [`internal/toolcmd`](../internal/toolcmd/commands.go) 统一，toolplane 的导航输出与 agents 的指令共用同一构造器；非 ASCII patch 规则与"唯一允许写入"子句抽到 [`agent_sdk_instructions.go`](../internal/agents/agent_sdk_instructions.go)；provider 名称与"是否走 Agent SDK"的判定收敛到 [`internal/models/providers.go`](../internal/models/providers.go) 的 `ProviderXxx` 常量与 `IsAgentSDKProvider`（agents / llm / init / setup 均复用它）。仍各自实现的是各阶段的 tool 命令清单与 patch/check 文案主体、review 结果转换（`toModelReviewResult`），是下一步的收敛目标。
5. **并发已统一（基本完成）**：craft / recap / draft / polish / write / compose 的章节与批次循环已全部改用 [`internal/workerpool`](../internal/workerpool/workerpool.go) 的 `Map`（结果按输入顺序返回，去掉了各处手写的 `WaitGroup+Mutex+channel`）。仅 write 的 best-of 生成/评分子流程仍是自定义 `WaitGroup`，因为它不是“按项独立”的扇出。
6. **webui 已收敛为单一集成路径**：`exec` 调用 CLI 是唯一的写入/派生路径，只读端点透传原始 JSON；webui 不再 import 父 module 的任何包（`boundary_test.go` 强制）。为支持这条路径，`compose normalize` 新增 `--force`（无规范化改动时也重写派生文件），二进制可用 `NOVELGEN_BIN` 指定。
7. **write/polish 共享逻辑已下沉**：章节上下文加载、生成/改进、post-save 检查与 recap 门禁从 `cmd/writecmd` 抽到 [`internal/writepipe`](../internal/writepipe/)，`polishcmd` 不再依赖 `writecmd`。包内按 `context.go` / `improve.go` / `postsave.go` / `recap.go` 分文件。
8. **`internal/rpg/dsl` 有尚未接线的 Phase 2 组件**：`hook*.go`、`evaluator.go`、`functions_enhanced.go`、`converter.go`、`errors_enhanced.go`、`logger.go`、`parser_enhanced.go` 目前没有被生产代码调用（仅有 placeholder 测试覆盖），但它们是 [RPG_DSL_SPEC.md](RPG_DSL_SPEC.md) 里 Phase 2 的既定交付物。**接入前不要把它们当作可用能力**；若决定不做，应连同文档一起删除。
9. **`internal/rpg` 引擎仍有一批 CLI 未使用的方法**（如 `GameWorld.RemoveItemFromPlayer`、`MapManager.MoveEntity`、`QuestManager.FailQuest`）：引擎本身是文档化的分层组件，这些方法属于游戏运行时 API 面。这里选择保留而非删除，接入新玩法时再评估。（真正无人使用的重复实现已清理：`SharedModelConverter` 的 models→GameWorld 导入半边和孤立的 `SkillInstance` 类型。）
10. **测试已归位到被测包**：原先堆在 `cmd/` 的 7k+ 行测试已迁到 `internal/toolplane`（151 个用例）、`internal/quality`、`internal/writepipe`、`internal/writersdl`、`internal/project`、`cmd/writecmd`，`cmd/` 只保留命令面（`command_surface_test.go`）与文档一致性（`agent_tools_docs_test.go`）两类测试。跨包共享的测试夹具抽到 `internal/testfixtures`；`internal/continuity` 及其 `logic/continuity/{character,transition}` 子包补上离线 recap、转场/角色启发式与 fix helper 的用例；`internal/writersdl` 补上批 manifest/缓存失效、章节 ID 归一、合并去重、issue 过滤与分阶段文件名推断（`batch_test.go`）；`cmd/export`、`cmd/initcmd`、`internal/rpgllm` 也补了包内用例。
11. **`internal/models` 无法复用 `internal/utils`**（层规则里 models 是第 0 层），因此 `outline_normalizer.go` 保留了自己的 `compactStrings`。这是刻意的分层结果，不是遗漏。
12. **工具与内部流水线已统一**：`tools/check_novel` 原先自带一份章节→DSL 批处理/缓存的旧副本，现在改为调用 [`writersdl.EmitWriteRPGDSL`](../internal/writersdl/emit.go)（新增的共享入口，`EmitChapterRPGDSLForProject` 也复用它），该工具从 1113 行降到 597 行，并自动获得 manifest 章节校验与战斗信号增强。同一轮还把 `dsl.ApplySimulationDefaults`（simulatedsl 与 check_novel 各一份）和 `models.ReviewSuggestionPriorityRank`（quality 与 agents 各一份）收敛为单一实现。
13. **仍存的少量平行实现**（有意保留）：各 `tools/*` 自带的 `printHelp`/`printSummary` 等打印函数；`models.Event` 与 `rpg.StoryEvent` 各自的旧格式事件适配（服务于两套不同的模型，需同步维护）；`toolplane.reviewPriorityRank` 与 models 版本签名不同（前者区分未知优先级）。
14. **静态检查基线**：根模块与 webui 的 `go vet ./...` 与 `staticcheck ./...` 均为干净，唯一剩余的是第 8 条里 DSL Phase-2 脚手架的 4 个 `U1000`。新增告警请当场处理，别让基线退化——这一轮就是靠它翻出 `draft improve` 丢弃生成稿、`fmt.Errorf` 动态格式串、JSON 提取被字符串内 `}` 截断等问题。
15. **小说 storage 已收敛（本轮完成）**：新增 [`internal/store`](../internal/store/)，磁盘布局、原子写、检查点/备份、类型化产物读写都只此一处；`internal/project` 退化为它上面的门面，`cmd/root.go` 负责解析一次根（`--dir` / `NOVELGEN_PROJECT_DIR` / 向上查找）。之前"同一份布局散在 47 个文件里、`compose` 用 cwd 相对路径而 `write` 用向上查找"的不一致已消除——`compose check` 现在在子目录与 `--dir` 两种场景下都可用（原实现要求 cwd 就是项目根）。`internal/models` 现在只保留 `Parse*`/`MarshalForSave` 与**路径入参**的便利读写（不含布局知识），`FindProjectRoot` 移入 store。测试不再靠 `os.Chdir` 定位项目：改为向命令上下文注入 root（`bindProject`）或安装进程默认根（`useProjectRoot`），全仓只剩 `cmd/rpgdsl` 一处 chdir，因为 `rpg-dsl` 的契约本身就是"在含 `books/` 的工作区里运行"，那不是项目根、store 无法表达。
16. **RPG 侧的工作区假设已消除（本轮完成）**：`rpg.LoadNovelgenProject` 以前签名是 `(工作区根, 书名)` 并硬编码 `<工作区>/books/<书名>/story/...`，导致任何不在 `books/<name>` 下的项目（包括 `--dir` 指定的）都加载失败；现在它只接受**项目目录**，路径由 `store` 给出，书名取 `novel.json` 的 `name`（缺失才退回目录名）。调用方（`cmd/rpgcmd`、`cmd/rpgdsl`、`internal/agents/chapter_to_dsl_adapter`、`tools/novelgen-rpg`、`tools/rpg-write`）都只传目录；`internal/rpg/dsl` 的 setup 查找同样改用 store。`cmd/rpgcmd` 仍保留 `<workspace>/output/rpg_data.json` 这份"顺手再存一份"的行为，但只在项目确实位于 `books/<name>` 下时才写，不再往凭空推导的路径写文件。RPG 侧现在也容忍 BOM。
17. **仍是两套寻址约定 + 一个"当前项目"重复来源**：项目命令与 `rpg`/`rpg-dsl`/`tools/*` 都按给定目录工作（`rpg-dsl`/`tools/*` 仍以 `<workspace>/books/<name>` 作为**默认**目录，字面量收敛到 `store.BooksDirName`/`store.BookDir`），webui 则自己拼只读路径（见 §8）。`logger.ProjectDir()` 与 store 根并存：write 流水线用 `logger.SetProjectDir` 钉住日志目录，agents/llm 的日志与工作区根现在按"logger → `store.Default()` → cwd"的顺序解析（`agentLogRoot`/`agentWorkspaceRoot`/`runtimeWorkspaceRoot`），`logger` 处于层表第 1 层因此不能自己读 store。下一步可把 logger 的那份也换成 `*store.Root`。

---

## 11. 相关文档

- [README.md](../README.md) — 命令参考
- [STAGE_CONTRACTS.md](STAGE_CONTRACTS.md) — 各阶段数据契约（改动阶段行为前必读）
- [RPG_DSL_SPEC.md](RPG_DSL_SPEC.md) / [DSL_RPG_INTEGRATION_SPEC.md](DSL_RPG_INTEGRATION_SPEC.md)
- [RPG_WRITE_USAGE.md](RPG_WRITE_USAGE.md) / [DSL_INCREMENTAL_WORKFLOW.md](DSL_INCREMENTAL_WORKFLOW.md)
- [tools/README.md](../tools/README.md) — 独立工具的清单
