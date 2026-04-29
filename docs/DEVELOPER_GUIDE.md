# Novelgen 开发者指南

> 版本: 0.3.0  
> 最后更新: 2026-04-24

---

## 1. 快速入门

### 1.1 环境要求

- **Go**: 1.21+
- **操作系统**: Windows/Linux/macOS
- **LLM 配置**: OpenAI 兼容 API 或 Ollama

### 1.2 构建项目

```bash
# Windows PowerShell
./build.ps1

# 或直接构建
go build -o bin/novelgen.exe

# 验证安装
./bin/novelgen.exe --help
```

### 1.3 创建第一个项目

```bash
# 初始化项目
./bin/novelgen.exe init my_novel --genre "科幻" --chapter 20

# 进入项目目录
cd my_novel

# 生成故事设定
./bin/novelgen.exe setup gen "一个关于太空探险的故事"

# 生成大纲
./bin/novelgen.exe compose gen

# 生成世界元素
./bin/novelgen.exe craft gen
```

---

## 2. 项目结构

### 2.1 核心目录

```
novelgen/
├── cmd/                    # CLI 命令
├── internal/               # 内部包
│   ├── agents/             # AI Agent
│   ├── llm/                # LLM 客户端
│   ├── models/             # 数据模型
│   ├── rpg/                # RPG 系统
│   └── logic/              # 业务逻辑
├── docs/                   # 技术文档
└── books/                  # 示例项目
```

### 2.2 项目目录结构

```
my_novel/
├── novel.json              # 项目配置
├── llm_config.json         # LLM 配置
├── story/
│   ├── setup/              # 故事设定
│   ├── compose/            # 大纲
│   ├── craft/              # 世界元素
│   ├── recaps/             # 章节回顾
│   └── rpg/                # RPG DSL
├── drafts/                 # 草稿
└── chapters/               # 最终章节
```

---

## 3. 开发工作流

### 3.1 添加新命令

#### 步骤 1: 创建命令文件

```go
// cmd/mycommand.go
package cmd

import (
    "github.com/spf13/cobra"
)

func newMyCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "mycommand",
        Short: "My new command",
        RunE:  runMyCommand,
    }
    
    cmd.Flags().String("option", "", "An option")
    return cmd
}

func runMyCommand(cmd *cobra.Command, args []string) error {
    // 实现命令逻辑
    return nil
}
```

#### 步骤 2: 注册命令

```go
// cmd/registry.go
func RegisterAllCommands(rootCmd *cobra.Command) {
    // ... 现有命令
    
    // 添加新命令
    rootCmd.AddCommand(newMyCommand())
}
```

### 3.2 添加新 Agent

#### 步骤 1: 创建 Agent

```go
// internal/agents/my_agent.go
package agents

type MyAgent struct {
    *BaseAgent
}

func NewMyAgent(client *llm.Client, config *llm.Config, model string) *MyAgent {
    return &MyAgent{
        BaseAgent: NewBaseAgent(client, config, model),
    }
}

func (a *MyAgent) Execute(ctx context.Context, input MyInput) (*MyOutput, error) {
    // 构建提示词
    prompt := a.buildPrompt(input)
    
    // 调用 LLM
    response, err := a.client.Chat(ctx, prompt)
    if err != nil {
        return nil, err
    }
    
    // 解析响应
    output := a.parseResponse(response)
    return &output, nil
}
```

#### 步骤 2: 在命令中使用

```go
// cmd/mycommand.go
func runMyCommand(cmd *cobra.Command, args []string) error {
    agent := agents.NewMyAgent(client, config, model)
    output, err := agent.Execute(ctx, input)
    // ...
}
```

### 3.3 添加 DSL 功能

#### 步骤 1: 定义 AST

```go
// internal/rpg/dsl/ast.go
type MyBlock struct {
    Name     string
    ID       string
    Properties map[string]interface{}
}
```

#### 步骤 2: 实现 Parser

```go
// internal/rpg/dsl/parser.go
func (p *Parser) parseMyBlock() (*MyBlock, error) {
    // 解析逻辑
    block := &MyBlock{
        Name: p.currentToken.Value,
    }
    // ...
    return block, nil
}
```

#### 步骤 3: 实现 Validator

```go
// internal/rpg/dsl/validator.go
func (v *Validator) validateMyBlock(block *MyBlock) error {
    if block.ID == "" {
        return fmt.Errorf("my block must have an ID")
    }
    return nil
}
```

#### 步骤 4: 实现 Converter

```go
// internal/rpg/dsl/converter.go
func (c *Converter) convertMyBlock(block *MyBlock) error {
    // 转换为 RPG World 对象
    return nil
}
```

---

## 4. 代码规范

### 4.1 命名规范

- **包名**: 小写，简短，如 `llm`, `rpg`, `dsl`
- **类型名**: PascalCase，如 `BaseAgent`, `ConstraintSystem`
- **函数名**: PascalCase（公开）/ camelCase（私有）
- **变量名**: camelCase
- **常量**: UPPER_SNAKE_CASE

### 4.2 错误处理

```go
// 使用 fmt.Errorf 包装错误
if err != nil {
    return fmt.Errorf("failed to parse DSL: %w", err)
}

// 定义哨兵错误
var ErrInvalidDSL = errors.New("invalid DSL syntax")

// 检查特定错误
if errors.Is(err, ErrInvalidDSL) {
    // 处理
}
```

### 4.3 日志记录

```go
import "novelgen/internal/logger"

// 使用结构化日志
logger.Info("Parsing DSL file", "file", filename, "lines", lineCount)
logger.Warn("Deprecated feature used", "feature", "old_syntax")
logger.Error("Failed to parse", "error", err, "line", lineNum)
```

---

## 5. 测试指南

### 5.1 单元测试

```go
// internal/rpg/dsl/parser_test.go
package dsl

import (
    "testing"
)

func TestParser_ParseMetadata(t *testing.T) {
    input := `
metadata {
    title = "Test"
    genre = ["sci-fi"]
}
`
    parser := NewParser(input)
    result, err := parser.Parse()
    
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    if result.Metadata.Title != "Test" {
        t.Errorf("expected title 'Test', got '%s'", result.Metadata.Title)
    }
}
```

### 5.2 基准测试

```go
// internal/rpg/benchmark/benchmark_test.go
package benchmark

import (
    "testing"
)

func BenchmarkSimulate(b *testing.B) {
    testCase := LoadTestCase("medium")
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        simulator := NewSimulator(testCase.RPGData)
        simulator.Simulate()
    }
}
```

### 5.3 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包测试
go test ./internal/rpg/dsl/...

# 运行基准测试
go test -bench=. ./internal/rpg/benchmark/...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## 6. 调试技巧

### 6.1 启用调试日志

```bash
# 设置日志级别
export NOVELGEN_LOG_LEVEL=debug

# 运行命令
./bin/novelgen.exe compose gen
```

### 6.2 DSL 调试

```bash
# 验证 DSL 文件
./bin/novelgen.exe rpg dsl validate books/mine/story/rpg/01_outline.rpg

# 查看解析结果
./bin/novelgen.exe rpg dsl parse books/mine/story/rpg/01_outline.rpg --output ast.json
```

### 6.3 模拟调试

```bash
# 运行模拟并查看详细日志
./bin/novelgen.exe simulate-dsl books/mine/story/rpg/final.rpg --verbose

# 生成模拟报告
./bin/novelgen.exe simulate-dsl books/mine/story/rpg/final.rpg --output report.json
```

---

## 7. 性能优化

### 7.1 并发处理

```go
// 使用 errgroup 并发处理
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(3) // 限制并发数

for _, chapter := range chapters {
    ch := chapter
    g.Go(func() error {
        return processChapter(ctx, ch)
    })
}

if err := g.Wait(); err != nil {
    return err
}
```

### 7.2 缓存机制

```go
// 简单的内存缓存
type Cache struct {
    mu    sync.RWMutex
    items map[string]*CacheItem
}

func (c *Cache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    item, ok := c.items[key]
    if !ok || item.Expired() {
        return nil, false
    }
    return item.Value, true
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.items[key] = &CacheItem{
        Value:     value,
        ExpiresAt: time.Now().Add(ttl),
    }
}
```

### 7.3 批量处理

```go
// 批量处理章节
func processBatch(chapters []Chapter, batchSize int) error {
    for i := 0; i < len(chapters); i += batchSize {
        end := i + batchSize
        if end > len(chapters) {
            end = len(chapters)
        }
        
        batch := chapters[i:end]
        if err := processBatchParallel(batch); err != nil {
            return err
        }
    }
    return nil
}
```

---

## 8. 扩展开发

### 8.1 添加新的 LLM 提供商

```go
// internal/llm/provider/my_provider.go
package provider

type MyProvider struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

func (p *MyProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // 实现 API 调用
}

// 在 config.go 中注册
func init() {
    RegisterProvider("my_provider", func(config ProviderConfig) Provider {
        return &MyProvider{
            baseURL: config.BaseURL,
            apiKey:  config.APIKey,
        }
    })
}
```

### 8.2 添加新的约束类型

```go
// internal/rpg/constraint_system.go
type EmotionalConstraint struct {
    MinIntensity float64
    MaxIntensity float64
    RequiredEmotions []string
}

func (cs *ConstraintSystem) buildEmotionalConstraints() *EmotionalConstraint {
    // 从 RPG 数据提取情感约束
    return &EmotionalConstraint{
        MinIntensity: 0.3,
        MaxIntensity: 0.8,
        RequiredEmotions: []string{"紧张", "期待"},
    }
}

func (cs *ConstraintSystem) validateEmotionalConstraint(chapterID, content string) []ConstraintViolation {
    // 检查情感强度
    return nil
}
```

### 8.3 添加新的模拟事件

```go
// internal/rpg/simulation.go
func (se *SimulationEngine) simulatePuzzleObjective(step SimulationStep, objective QuestObjective) SimulationStep {
    step.Type = "puzzle"
    step.Description = "解谜完成"
    step.Results = append(step.Results, SimulationResult{
        Type:    "success",
        Message: "成功解开谜题",
    })
    return step
}

// 在 dispatchObjective 中注册
func (se *SimulationEngine) dispatchObjective(step SimulationStep, objective QuestObjective) SimulationStep {
    switch objective.Type {
    case "kill":
        return se.simulateKillObjective(step, objective)
    case "puzzle":
        return se.simulatePuzzleObjective(step, objective)
    // ...
    }
}
```

---

## 9. 常见问题

### Q: 如何调试 AI 生成的内容？

A: 检查 `logs/` 目录下的日志文件，包含完整的 AI 请求和响应。

### Q: DSL 解析失败怎么办？

A: 使用 `novelgen rpg dsl parse --output ast.json` 查看解析结果，定位语法错误。

### Q: 如何自定义提示词？

A: 在 `internal/prompts/` 下创建新的提示词文件，并在 `base.go` 中注册。

### Q: 并发处理时出现限流怎么办？

A: 使用 `--concurrency` 参数降低并发数，或在 LLM 配置中调整限流设置。

---

## 10. 相关资源

- [ARCHITECTURE.md](ARCHITECTURE.md) - 项目架构
- [docs/RPG_DSL_DOCUMENTATION_INDEX.md](../docs/RPG_DSL_DOCUMENTATION_INDEX.md) - RPG-DSL 文档索引
- [README.md](../README.md) - 主文档
- `internal/` - 核心源码
- `docs/` - 技术文档

---

## 11. 贡献指南

### 11.1 提交代码

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

### 11.2 代码审查要点

- 代码风格一致性
- 完善的错误处理
- 必要的单元测试
- 文档更新
- 性能影响评估

### 11.3 版本发布

遵循语义化版本：
- **MAJOR**: 不兼容的 API 变更
- **MINOR**: 向后兼容的功能新增
- **PATCH**: 向后兼容的 bug 修复
