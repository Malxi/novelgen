# RPG 与 Compose 系统集成指南

## 概述

RPG 系统和 Compose 系统现在已经深度集成，可以在大纲生成的各个阶段进行结构验证和质量检查。

## 主要功能

### 1. 独立的验证命令

使用 `validate-outline` 命令可以独立验证已有的大纲文件：

```bash
# 验证当前项目的大纲
novelgen validate-outline

# 验证指定文件
novelgen validate-outline books/mine/story/compose/outline.json

# 保存验证报告
novelgen validate-outline -o validation_report.json

# 严格模式：遇到警告也视为失败
novelgen validate-outline --strict
```

### 2. 验证报告内容

验证报告包含以下信息：

- **总评分**：0-100 分的综合评分和等级（S/A/B/C/D/F）
- **基础属性**：
  - 结构完整性
  - 逻辑一致性
  - 角色平衡性
  - 剧情连贯性
  - 节奏质量
- **负面状态**：如发现的问题类型和严重程度
- **BOSS级问题**：严重结构问题的诊断
- **改进建议**：基于发现问题的具体改进建议

### 3. 编程接口

在 Go 代码中使用验证功能：

```go
import "novelgen/internal/rpg"

// 验证大纲文件
report, err := rpg.ValidateOutlineFile("path/to/outline.json")
if err != nil {
    log.Fatal(err)
}

// 访问验证结果
fmt.Printf("总评分: %d/100 (等级: %s)\n", 
    report.Result.TotalScore, 
    report.Result.Grade)

// 打印验证摘要
rpg.PrintValidationSummary(report)

// 保存报告
err = rpg.SaveValidationReport(report, "validation.json")
```

## 工作流程示例

### 完整的大纲生成和验证流程

```bash
# 1. 初始化项目
novelgen init my-book
cd my-book

# 2. 创建故事设定
novelgen setup gen

# 3. 生成大纲
novelgen compose gen --hierarchical

# 4. 验证大纲质量
novelgen validate-outline -o validation_report.json

# 5. 根据验证结果改进大纲（如果需要）
novelgen compose improve --max-rounds 3

# 6. 再次验证
novelgen validate-outline
```

## 评分标准

- **S级 (90-100分)**：大纲结构完美，可以直接进入写作阶段
- **A级 (80-89分)**：大纲状态良好，少量细节需要优化
- **B级 (70-79分)**：大纲状态一般，存在一些问题需要修复
- **C级 (60-69分)**：大纲状态较差，存在明显缺陷
- **D级 (50-59分)**：大纲状态危险，需要大规模重构
- **F级 (<50分)**：大纲状态崩溃，建议重新设计

## 注意事项

1. 验证系统支持标准的大纲格式 (parts → volumes → chapters)
2. 确保大纲文件包含必要的字段（id, title, summary, chapters等）
3. 验证报告保存为 JSON 格式，便于后续处理和分析
4. 建议在每次大纲修改后重新运行验证，确保质量达标
