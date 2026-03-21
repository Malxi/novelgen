# Nolvegen - AI 辅助小说生成工具

Nolvegen 是一个命令行工具，用于 AI 辅助小说创作。它提供了一个结构化的工作流程，从最初的创意到完整的小说生成。

## 核心工作流程

```
1. init     → 初始化新项目
2. setup    → 创建故事设定（类型、前提、主题等）
3. compose  → 生成故事大纲（部 → 卷 → 章）
4. craft    → 创建详细的世界元素（角色、地点、物品）
5. draft    → 生成并改进草稿章节
6. write    → 生成最终润色的章节
7. export   → 导出完成的小说
```

## 整体生成流程说明（从灵感到成书）

- **项目初始化**：`init` 生成项目结构与基础配置。
- **设定生成**：`setup` 产出故事设定（类型、前提、规则、主题、叙事风格）。
- **大纲搭建**：`compose` 构建部→卷→章的层级大纲。
- **世界元素完善**：`craft` 扫描大纲并补齐角色、地点、物品等。
- **草稿生产与修订**：`draft gen/review/improve` 生成草稿并按评审反馈迭代；必要时触发连续性修复（转场桥段/角色出场）。
- **最终章节生成**：`write gen` 基于草稿输出最终文本，并自动抽取 recap；`write improve` 继续按评审修订。
- **导出成书**：`export` 输出为 markdown/txt。

---

## 命令完整列表

### 1. `novel init` - 初始化项目

初始化一个新的 novel 项目。

**用法：**
```bash
novel init <book_name> [options]
```

**Options：**
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--chapter` | int | 20 | 章节数量 |
| `--genre` | string | "" | 类型（逗号分隔，如"科幻,废土"） |
| `--mode` | string | "" | LLM 模型 |
| `--provider` | string | "ollama" | LLM 提供商 |
| `--language` | string | "zh" | 故事语言 |

**示例：**
```bash
novel init my_novel --genre "科幻" --chapter 20
```

---

### 2. `novel setup` - 创建故事设定

创建或更新小说的故事设定。

**子命令：**
- `gen <prompt>` - 使用 AI 从提示生成故事设定
- `regen [--prompt]` - 重新生成故事设定
- `improve [--max-rounds]` - 改进现有故事设定
- `import [markdown_file]` - 从 Markdown 导入故事设定

**示例：**
```bash
novel setup gen "一个关于太空探险的故事"
novel setup regen --prompt "增加更多悬疑元素"
novel setup improve --max-rounds 2
novel setup import story/setup/story_setup.md
```

---

### 3. `novel compose` - 生成故事大纲

生成具有严格三级结构（部 → 卷 → 章）的故事大纲。

**子命令：**
- `gen` - 生成新大纲
- `regen [id]` - 重新生成特定部分
  - `--prompt` (string) - 重新生成时的建议
- `improve [--max-rounds]` - 改进现有大纲

**示例：**
```bash
novel compose gen                      # 生成完整大纲
novel compose regen 1_1_1              # 重新生成第1部第1卷第1章
novel compose regen 1_1_1 --prompt "加强冲突"
novel compose improve --max-rounds 3   # 改进大纲3轮
```

---

### 4. `novel craft` - 生成世界元素

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
| `--max-rounds` | int | 1 | 改进轮数 |

**示例：**
```bash
novel craft gen                        # 生成所有元素
novel craft gen --chapter 1            # 生成第1章的元素
novel craft gen --concurrency 3        # 并发生成
novel craft improve --type characters --max-rounds 2
```

---

### 5. `novel draft` - 生成草稿

基于大纲和故事状态生成、评审和改进草稿章节。

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
novel draft gen --chapter 1            # 生成第1章草稿
novel draft gen --chapter 1-5          # 生成第1-5章草稿
novel draft gen --all                  # 生成所有草稿
novel draft review --volume 1          # 评审第1卷
novel draft improve --volume 1 --max-rounds 3
```

---

### 6. `novel write` - 生成最终章节

基于草稿生成润色的最终章节内容。

**子命令：**

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

#### `write improve` - 改进最终章节
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--chapter` | string | "" | 指定章节 |
| `--volume` | string | "" | 指定卷 |
| `--part` | string | "" | 指定部 |
| `--max-rounds` | int | 1 | 最大改进轮数 |
| `--min-score` | int | 7 | 最低可接受分数 |
| `--concurrency` | int | 1 | 并发数 |
| `--enable-teleport-auto-fix` | bool | true | 启用瞬移自动修复 |
| `--enable-character-presence-auto-fix` | bool | true | 启用角色出场自动修复 |
| `--bridge-retries` | int | 1 | 转场桥段重试次数 |
| `--character-patch-retries` | int | 1 | 角色补丁重试次数 |

**连续性/自动修复说明：**
- 仅在 `write improve` 阶段生效。
- `teleport` 修复会尝试补齐章节转场桥段（依赖上一章 recap）。
- `character presence` 修复会补出场缺失角色的补丁段。

**示例：**
```bash
novel write gen --chapter 1            # 生成第1章最终版
novel write gen --all                  # 生成所有章节
novel write improve --volume 1         # 改进第1卷
```

---

### 7. `novel export` - 导出小说

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
novel export novel                     # 导出为 markdown
novel export novel --format txt        # 导出为文本
novel export novel --output my_book.md # 指定输出文件
```

---

### 8. `novel recap` - 章节回顾

提取高信号、规范的章节回顾 JSON，用于改善章节间连续性。

**说明：**
- `draft gen` / `write gen` 会在生成后自动抽取并保存 recap（best-effort）。
- `novel recap gen` 主要用于批量重建或指定源文本（`drafts`/`chapters`）。

**子命令：**
- `gen` - 生成回顾 JSON

**Options：**
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--chapter` | string | "" | 章节号 |
| `--all` | bool | false | 所有章节 |
| `--source` | string | "drafts" | 源文本（drafts/chapters） |
| `--concurrency` | int | 1 | 并发数 |

**示例：**
```bash
novel recap gen --chapter 1            # 生成第1章回顾
novel recap gen --chapter 1-10         # 生成第1-10章回顾
novel recap gen --all                  # 生成所有章节回顾
novel recap gen --source chapters      # 从最终章节生成
```

---

### 9. `novel translate` - 翻译内容

使用 AI 将小说内容从一种语言翻译为另一种语言。

**Options：**
| Option | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `--source-lang` | string | "zh" | 源语言 |
| `--target-lang` | string | "en" | 目标语言 |
| `--output` | string | "" | 输出文件 |

**示例：**
```bash
novel translate story/chapters/chapter_001.txt
novel translate story/setup/story_setup.md --target-lang en
novel translate chapter.txt --source-lang zh --target-lang en --output chapter_en.txt
```

---

### 10. `novel config` - 管理 LLM 配置

管理 AI 生成功能的 LLM 提供商设置。

**子命令：**
- `show` - 显示当前配置
- `set` - 交互式配置

**示例：**
```bash
novel config show                      # 显示配置
novel config set                       # 交互式设置
```

---

## 完整工作流程示例

```bash
# 1. 初始化项目
novel init my_novel --genre "科幻" --chapter 20

# 2. 创建故事设定
novel setup gen "一个关于太空探险的故事"

# 3. 生成大纲
novel compose gen

# 4. 生成世界元素
novel craft gen

# 5. 生成草稿
novel draft gen --all

# 6. 评审和改进草稿
novel draft review --all
novel draft improve --all --max-rounds 3

# 7. 生成章节回顾（用于连续性）
novel recap gen --all

# 8. 生成最终章节
novel write gen --all

# 9. 改进最终章节
novel write improve --all --max-rounds 2

# 10. 导出小说
novel export novel --output my_novel.md

# 11. 翻译（可选）
novel translate my_novel.md --target-lang en --output my_novel_en.md
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
├── drafts/                 # 草稿
│   └── C{n}.md
└── logs/                   # 日志
```

---

## 安装

```powershell
# 生成固定路径的可执行文件（bin/nolvegen.exe）
./build.ps1

# Windows bat 版本（双击或命令行执行）
# build.bat

# 或直接使用 go build
# go build -o bin/nolvegen.exe
```

## 使用帮助

```bash
novel --help                           # 显示主帮助
novel <command> --help                 # 显示命令帮助
```
