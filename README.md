# Novelgen - AI 辅助小说生成工具

Novelgen 是一个命令行工具，用于 AI 辅助小说创作。它提供了一个结构化的工作流程，从最初的创意到完整的小说生成，并创新性地引入了 RPG-DSL 系统实现智能化的内容验证和推演。

## 📚 文档导航

| 文档 | 说明 |
|------|------|
| [ARCHITECTURE.md](ARCHITECTURE.md) | 项目架构总览 |
| [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) | 开发者指南 |
| [docs/RPG_DSL_DOCUMENTATION_INDEX.md](docs/RPG_DSL_DOCUMENTATION_INDEX.md) | RPG-DSL 系统文档索引 |
| [docs/](docs/) | 技术文档目录 |

## ✨ 核心特性

- **结构化创作**: 部→卷→章的三级大纲体系
- **AI 驱动**: 每个阶段都有专门的 AI Agent
- **RPG 验证**: 通过 DSL 系统验证内容一致性
- **约束写作**: RPG 约束指导 AI 写作，避免战力崩坏等问题
- **连续性保证**: 自动检测角色出场、转场桥段、章节回顾

## 🚀 快速开始

```bash
# 1. 构建项目
./build.ps1

# 2. 初始化项目
./bin/novelgen.exe init my_novel --genre "科幻" --chapter 20

# 3. 进入项目目录
cd my_novel

# 4. 生成故事设定
./bin/novelgen.exe setup gen "一个关于太空探险的故事"

# 5. 生成大纲
./bin/novelgen.exe compose gen

# 6. 生成世界元素
./bin/novelgen.exe craft gen
```

## 核心工作流程

```
1. init     → 初始化新项目
2. setup    → 创建故事设定（类型、前提、主题等）
3. compose  → 生成故事大纲（部 → 卷 → 章）
4. craft    → 创建详细的世界元素（角色、地点、物品）
5. write    → 直接生成最终章节、评审/改进、抽取 recap、输出 RPG DSL
6. export   → 导出完成的小说
```

## 整体生成流程说明（从灵感到成书）

- **项目初始化**：`init` 生成项目结构与基础配置。
- **设定生成**：`setup` 产出故事设定（类型、前提、规则、主题、叙事风格）。
- **大纲搭建**：`compose` 构建部→卷→章的层级大纲。
- **世界元素完善**：`craft` 扫描大纲并补齐角色、地点、物品等。
- **最终章节生成**：推荐使用 `write pipeline` 直接从 setup/outline/craft/RPGState/recap 生成最终章节，并自动评审、改进、抽取 recap、更新 `story/rpg/04_chapters.rpg`。
- **草稿工作流（可选/旧版）**：`draft gen/review/improve` 仍保留用于旧项目或试写，但不再是默认主流程。
- **导出成书**：`export` 输出为 markdown/txt。

---

## 命令完整列表

### 1. `novelgen init` - 初始化项目

初始化一个新的 novelgen 项目。

**用法：**
```bash
novelgen init <book_name> [options]
```

**Options：**
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--chapter` | int | 20 | 章节数量 |
| `--genre` | string | "" | 类型（逗号分隔，如"科幻,废土"） |
| `--mode` | string | "" | LLM 模型 |
| `--provider` | string | "claude" | LLM/agent runtime 提供商 |
| `--language` | string | "zh" | 故事语言 |

**示例：**
```bash
novelgen init my_novel --genre "科幻" --chapter 20
```

---

### 2. `novelgen setup` - 创建故事设定

创建或更新小说的故事设定。

**子命令：**
- `gen <prompt>` - 使用 AI 从提示生成故事设定
- `regen [--prompt]` - 重新生成故事设定
- `improve [--max-rounds]` - 改进现有故事设定
- `import [markdown_file]` - 从 Markdown 导入故事设定

**示例：**
```bash
novelgen setup gen "一个关于太空探险的故事"
novelgen setup regen --prompt "增加更多悬疑元素"
novelgen setup improve --max-rounds 2
novelgen setup import story/setup/story_setup.md
```

---

### 3. `novelgen compose` - 生成故事大纲

生成具有严格三级结构（部 → 卷 → 章）的故事大纲。

**子命令：**
- `gen` - 生成新大纲
- `regen [id]` - 重新生成特定部分
  - `--prompt` (string) - 重新生成时的建议
- `improve [--max-rounds]` - 改进现有大纲
- `skeleton-review` - 在生成章节前评审部/卷骨架
- `skeleton-improve` - 只改进部/卷骨架，并保留已有章节数组
- `pipeline` - 按全局卷序号逐卷执行 skeleton/生成/improve/cross patch

**示例：**
```bash
novelgen compose gen                      # 生成完整大纲
novelgen compose pipeline --from-volume 1 --to-volume 1
novelgen compose pipeline --from-volume 2 --to-volume 7 --max-rounds 1
novelgen compose regen 1_1_1              # 重新生成第1部第1卷第1章
novelgen compose regen 1_1_1 --prompt "加强冲突"
novelgen compose improve --max-rounds 3   # 改进大纲3轮
novelgen compose improve --agent-sdk --from-volume 1 --to-volume 3   # Agent SDK 逐卷改进
novelgen compose review --agent-sdk --prompt "重点检查角色动机和伏笔回收节奏"   # Agent SDK 只读审阅
novelgen compose skeleton-review          # 只评审大纲骨架
novelgen compose skeleton-improve --max-rounds 1
```

Agent SDK 模式（`--agent-sdk`）的逐卷改进支持：
- `--repair-budget`（int，默认 20）：每次门禁修复 pass 最多处理的目标问题数。
- 允许只读查询相邻卷的 `payoff_contract/summary`，用于跨卷连续性核对（不可修改其它卷）。
- 运行结束后自动在 `logs/` 下生成 `compose_improve_report_<时间戳>.md` 改进报告（每卷评分、改动摘要、剩余问题、门禁修复后剩余问题）。

建议报告流水线（AI 审阅与确定性检查解耦，统一喂给 Agent SDK 改进）：
```bash
novelgen compose review --agent-sdk --prompt "..." --out story/reviews/outline_review.json
novelgen compose check --suggestions-out story/reviews/outline_check.json
novelgen compose improve --agent-sdk --suggestions story/reviews/outline_review.json,story/reviews/outline_check.json
```

`compose review --agent-sdk` 是只读审阅（不 patch、不写项目文件），输出 `models.ReviewResult` JSON；
`compose check --suggestions-out` 把确定性检查结果转成同样的 `models.ReviewResult` 形状；
`compose improve --agent-sdk --suggestions` 读取这些报告并合并（去重、按 prompt 边界过滤）为改进种子。

---

### 4. `novelgen craft` - 生成世界元素

从大纲中扫描并生成详细的故事元素。

**子命令：**
- `gen` - 生成元素
- `improve` - 改进现有元素

**Options：**
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--chapter` | string | "" | 指定章节（如 "1", "P1-V1-C1"） |
| `--volume` | string | "" | 指定卷 |
| `--part` | string | "" | 指定部 |
| `--prompt` | string | "" | 额外提示 |
| `--batch` | int | 1 | 每批生成数量 |
| `--concurrency` | int | 1 | 并发数 |
| `--type` | string | "all" | 元素类型（all/characters/locations/items） |
| `--name` | string | "" | `craft improve` 精确改进一个元素名 |
| `--max-rounds` | int | 1 | 改进轮数 |

**示例：**
```bash
novelgen craft gen                        # 生成所有元素
novelgen craft gen --chapter 1            # 生成第1章的元素
novelgen craft gen --concurrency 3        # 并发生成
novelgen craft improve --type characters --max-rounds 2
novelgen craft improve --type characters --name "李侑" --agent-sdk --agent-apply
```

---

### Legacy. `novelgen draft` - 可选草稿工作流

`draft` 已从默认主流程中软移除。它仍可用于旧项目兼容、试写或临时草稿，但新项目推荐直接使用 `novelgen write pipeline` 生成最终章节。

**子命令：**

#### `draft gen` - 生成草稿
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--chapter` | string | "" | 章节号（如 "1", "1-5", "P1-V1-C1"） |
| `--volume` | string | "" | 卷号 |
| `--part` | string | "" | 部号 |
| `--words` | int | 500 | 目标字数 |
| `--all` | bool | false | 生成所有章节 |
| `--concurrency` | int | 1 | 并发数 |
| `--context` | int | 1 | 上下文章节数 |

**说明：**
- `draft gen` 会尝试读取上一章 recap 作为生成上下文的一部分（若存在）。
- 生成完成后会自动抽取并保存 recap（best-effort）。

#### `draft review` - 评审草稿
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--chapter` | string | "" | 指定章节 |
| `--volume` | string | "" | 指定卷 |
| `--part` | string | "" | 指定部 |
| `--concurrency` | int | 1 | 并发数 |

#### `draft improve` - 改进草稿
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--chapter` | string | "" | 指定章节 |
| `--volume` | string | "" | 指定卷 |
| `--part` | string | "" | 指定部 |
| `--max-rounds` | int | 1 | 最大改进轮数 |
| `--min-score` | int | 7 | 最低可接受分数 (1-10) |
| `--concurrency` | int | 1 | 并发数 |
| `--enable-teleport-auto-fix` | bool | true | 启用瞬移自动修复 |
| `--enable-character-presence-auto-fix` | bool | true | 启用角色出场自动修复 |
| `--bridge-retries` | int | 1 | 转场桥段重试次数 |
| `--character-patch-retries` | int | 1 | 角色补丁重试次数 |

**连续性/自动修复说明：**
- 仅在 `draft improve` 阶段生效。
- `teleport` 修复会尝试补齐章节转场桥段（依赖上一章 recap）。
- `character presence` 修复会补出场缺失角色的补丁段。

**示例：**
```bash
novelgen draft gen --chapter 1            # 生成第1章草稿
novelgen draft gen --chapter 1-5          # 生成第1-5章草稿
novelgen draft gen --all                  # 生成所有草稿
novelgen draft review --volume 1          # 评审第1卷
novelgen draft improve --volume 1 --max-rounds 3
```

---

### 5. `novelgen write` - 直接生成最终章节

推荐使用 `write pipeline` 直接从大纲、故事设定、世界元素、RPGState、recap 和上下文章节生成最终章节。Pipeline 会自动完成生成、评审、改进、recap 抽取和章节级 RPG DSL 输出。

**子命令：**

#### `write pipeline` - 推荐主流程
```bash
novelgen write pipeline --chapter P1-V1-C1
novelgen write pipeline --volume P1-V1 --max-rounds 2
novelgen write pipeline --all --rpg-batch-size 10
novelgen write pipeline --chapter P1-V1-C1 --agent-sdk
novelgen write pipeline --chapter P1-V1-C1 --agent-sdk --agent-apply
```

| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--agent-sdk` | bool | false | 使用 Agent SDK 进行章节生成、改进和最终 recap 抽取，Go 仍负责校验与保存 |
| `--agent-apply` | bool | false | 配合 `--agent-sdk`，允许 agent 通过 validated chapter patch 写入正文 |
| `--recap-agent-sdk` | bool | false | 仅将自动 recap 抽取切到 Agent SDK |

#### `write gen` - 生成最终章节
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--chapter` | string | "" | 章节号 |
| `--volume` | string | "" | 卷号 |
| `--part` | string | "" | 部号 |
| `--words` | int | 2000 | 目标字数 |
| `--all` | bool | false | 生成所有章节 |
| `--context` | int | 2 | 上下文章节数 |
| `--concurrency` | int | 1 | 并发数 |
| `--agent-sdk` | bool | false | 使用 Agent SDK focused chapter workflow，Go 仍负责校验和保存 |
| `--recap-agent-sdk` | bool | false | 使用 Agent SDK 抽取自动 recap |

#### `write improve` - 改进最终章节
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--chapter` | string | "" | 指定章节 |
| `--volume` | string | "" | 指定卷 |
| `--part` | string | "" | 指定部 |
| `--max-rounds` | int | 1 | 最大改进轮数 |
| `--min-score` | int | 70 | 最低可接受分数 (0-100) |
| `--concurrency` | int | 1 | 并发数 |
| `--enable-teleport-auto-fix` | bool | true | 启用瞬移自动修复 |
| `--enable-character-presence-auto-fix` | bool | true | 启用角色出场自动修复 |
| `--bridge-retries` | int | 1 | 转场桥段重试次数 |
| `--character-patch-retries` | int | 1 | 角色补丁重试次数 |
| `--agent-sdk` | bool | false | 使用 Agent SDK 的逐章 focused repair workflow |
| `--agent-apply` | bool | false | 配合 `--agent-sdk`，允许 agent 通过 validated chapter patch 写入 |

Agent SDK 小修默认保持当前章节篇幅：除非问题明确要求扩写或补字数，否则不会为了项目目标字数重写整章。对 system log / 信息差题材，日志线索、可执行判断和信息优势也计入主角成长。

**连续性/自动修复说明：**
- 仅在 `write improve` 阶段生效。
- `teleport` 修复会尝试补齐章节转场桥段（依赖上一章 recap）。
- `character presence` 修复会补出场缺失角色的补丁段。

**示例：**
```bash
novelgen write pipeline --chapter 1       # 推荐：生成/评审/改进/recap/RPG DSL
novelgen write pipeline --all             # 推荐：处理所有章节
novelgen write improve --volume 1 --min-score 75  # 手动改进第1卷，低于75分则改进
```

---

### 6. `novelgen polish` - 卷级润色

`polish` 会先做卷级整体评审，再逐章应用改进建议并刷新 recap。它适合在一卷章节已经生成后做整体连贯性、节奏和衔接修复。

**常用选项：**
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--volume` | string | "" | 指定卷（如 "1", "P1-V1"） |
| `--part` | string | "" | 指定部 |
| `--max-rounds` | int | 2 | 最大润色轮数 |
| `--min-score` | int | 8 | 最低可接受分数 (1-10) |
| `--prompt` | string | "" | 额外润色要求 |
| `--agent-sdk` | bool | false | 使用 Agent SDK 的逐章 focused repair workflow |
| `--agent-apply` | bool | false | 配合 `--agent-sdk`，允许 agent 通过 validated chapter patch 写入 |
| `--recap-agent-sdk` | bool | false | 使用 Agent SDK 刷新 recap |

**示例：**
```bash
novelgen polish --volume 1 --prompt "加强人物情感描写"
novelgen polish --volume 1 --agent-sdk
novelgen polish --volume 1 --agent-sdk --agent-apply
```

---

### 7. `novelgen export` - 导出小说

将完成的小说导出为各种格式。

**子命令：**
- `novel` - 导出完整小说

**Options：**
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--format` | string | "md" | 格式 (md, txt) |
| `--output` | string | "" | 输出文件路径 |

**示例：**
```bash
novelgen export novel                     # 导出为 markdown
novelgen export novel --format txt        # 导出为文本
novelgen export novel --output my_book.md # 指定输出文件
```

---

### 8. `novelgen recap` - 章节回顾

提取高信号、规范的章节回顾 JSON，用于改善章节间连续性。

**说明：**
- `write pipeline` / `write gen` 会在生成后自动抽取并保存 recap（best-effort）。
- `novelgen recap gen` 主要用于批量重建或指定源文本；默认从最终章节 `chapters/` 读取。

**子命令：**
- `gen` - 生成回顾 JSON

**Options：**
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--chapter` | string | "" | 章节号 |
| `--all` | bool | false | 所有章节 |
| `--source` | string | "chapters" | 源文本（chapters/drafts；drafts 为旧版兼容） |
| `--concurrency` | int | 1 | 并发数 |

**示例：**
```bash
novelgen recap gen --chapter 1            # 生成第1章回顾
novelgen recap gen --chapter 1-10         # 生成第1-10章回顾
novelgen recap gen --all                  # 生成所有章节回顾
novelgen recap gen --source chapters      # 从最终章节生成
```

---

### 9. `novelgen translate` - 翻译内容

使用 AI 将小说内容从一种语言翻译为另一种语言。

**Options：**
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--source-lang` | string | "zh" | 源语言 |
| `--target-lang` | string | "en" | 目标语言 |
| `--output` | string | "" | 输出文件 |

**示例：**
```bash
novelgen translate story/chapters/chapter_001.txt
novelgen translate story/setup/story_setup.md --target-lang en
novelgen translate chapter.txt --source-lang zh --target-lang en --output chapter_en.txt
```

---

### 10. `novelgen config` - 管理 LLM 配置

管理 AI 生成功能的 LLM 提供商设置。

**子命令：**
- `show` - 显示当前配置
- `set` - 交互式配置
- `agent` - configure Claude/Anthropic-compatible agent runtime

**示例：**
```bash
novelgen config show                      # 显示配置
novelgen config set                       # 交互式设置
novelgen config agent                     # configure Claude/Anthropic-compatible agent runtime
```

Agent execution defaults to the Claude Python runner when the project provider
is `claude`. Install Python and configure `~/.novelgen/agent_config.json`, or
provide Claude-compatible environment values through `~/.claude/settings.json`
such as `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN`, and `ANTHROPIC_MODEL`.

Agent runtimes can also reference an LLM provider already configured in
`~/.novelgen/llm_config.json`:

```json
{
  "default_runtime": "claude",
  "runtimes": {
    "claude": {
      "type": "python_process",
      "provider": "opencode",
      "model": "deepseek-v4-flash"
    }
  }
}
```

The base URL, API key, and timeout are resolved from the provider at runtime
(explicit fields win), and a Claude flag-settings file is auto-generated under
`~/.novelgen/agents/settings/` so no manual `settings` path is needed.

---

## 完整工作流程示例

```bash
# 1. 初始化项目
novelgen init my_novel --genre "科幻" --chapter 20

# 2. 创建故事设定
novelgen setup gen "一个关于太空探险的故事"

# 3. 生成大纲
novelgen compose gen

# 4. 生成世界元素
novelgen craft gen

# 5. 直接生成最终章节、评审/改进、抽取 recap、输出 RPG DSL
novelgen write pipeline --all --max-rounds 2

# 6. 如有需要，手动重建章节回顾（默认从 chapters/ 读取）
novelgen recap gen --all

# 7. 导出小说
novelgen export novel --output my_novel.md

# 8. 翻译（可选）
novelgen translate my_novel.md --target-lang en --output my_novel_en.md
```

---

## 项目目录结构

```
project-root/
├── novel.json              # 项目配置
├── llm_config.json         # LLM 配置
├── story/                  # 故事相关配置
│   ├── setup/              # 故事设定
│   │   ├── story_setup.json
│   │   └── story_setup.md
│   ├── compose/            # 大纲
│   │   ├── outline.json
│   │   └── outline.md
│   ├── craft/              # 世界元素
│   │   ├── characters.json
│   │   ├── locations.json
│   │   └── items.json
│   ├── recaps/             # 章节回顾
│   │   └── {chapter_id}.json
│   └── reviews/            # 评审结果
│       └── V{n}_review.json
├── chapters/               # 最终章节
│   └── chapter-{n}.md
├── drafts/                 # 旧版/可选草稿
│   └── C{n}.md
└── logs/                   # 日志
```

---

## 安装

```powershell
# 生成固定路径的可执行文件（bin/novelgen.exe）
./build.ps1

# Windows bat 版本（双击或命令行执行）
# build.bat

# 或直接使用 go build
# go build -o bin/novelgen.exe
```

## 使用帮助

```bash
novelgen --help                           # 显示主帮助
novelgen <command> --help                 # 显示命令帮助
```

---

## 🎮 RPG-DSL 系统

Novelgen 创新性地引入了 RPG-DSL（Role-Playing Game Domain-Specific Language）系统，将小说内容转化为可执行的游戏化数据结构，实现智能化的内容验证和推演。

### 为什么需要 RPG-DSL？

- **内容验证**: 自动检测战力崩坏、角色死亡滥用、时间线混乱等问题
- **约束写作**: RPG 约束指导 AI 写作，确保内容一致性
- **游戏化**: 为小说改编游戏提供数值基础
- **AI 友好**: 类自然语言的声明式语法，降低 AI 生成难度

### RPG-DSL 工作流

```
故事数据 (JSON)
    ↓
AI 分析 → RPG DSL 文件 (.rpg)
    ↓
DSL Parser → AST
    ↓
DSL Validator → 验证报告
    ↓
DSL Converter → RPG World
    ↓
Simulator → 模拟报告（问题检测）
    ↓
Constraint System → 约束规则
    ↓
反馈给 AI 写作 Agent
```

### 快速使用

```bash
# 转换小说为 RPG DSL
novelgen rpg-dsl convert -b mine

# 只转换已生成的某一卷 outline DSL
novelgen rpg-dsl convert -b mine --phase outline --volume 7

# 运行模拟检测问题
novelgen simulate-dsl books/mine/story/rpg/final.rpg

# 查看模拟报告
cat simulation_report.json
```

### DSL 示例

```dsl
metadata {
    title = "矿脉仙途"
    genre = ["修仙", "穿越", "悬疑"]
    dsl_version = "0.2.0"
}

characters {
    player "林砚" {
        id = "char_player"
        stats {
            hp = 100
            mp = 50
            attack = 15
        }
    }
}

storyline {
    chapter "P1-V1-C1" {
        objective "寒矿醒转" {
            step 1 "苏醒" {
                event "status_change" {
                    subject = "休眠状态"
                    change = "resolved"
                }
            }
        }
    }
}
```

### RPG-DSL 文档

| 文档 | 说明 |
|------|------|
| [RPG-DSL 文档索引](docs/RPG_DSL_DOCUMENTATION_INDEX.md) | 文档导航和快速开始 |
| [DSL-RPG 集成规范](docs/DSL_RPG_INTEGRATION_SPEC.md) | 完整的架构设计和数据流 |
| [DSL 规格文档](docs/RPG_DSL_SPEC.md) | 详细的语法定义和函数库 |
| [约束系统集成](docs/RPG_CONSTRAINT_INTEGRATION.md) | RPG 约束指导写作方案 |
| [RPG 写作指南](docs/RPG_WRITE_USAGE.md) | 使用 RPG 约束进行写作 |

---

## 📖 更多文档

- [ARCHITECTURE.md](ARCHITECTURE.md) - 项目架构总览
- [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) - 开发者指南
- [docs/](docs/) - 技术文档目录

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT
