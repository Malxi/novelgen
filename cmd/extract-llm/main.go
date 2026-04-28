package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"novelgen/internal/rpg"
)

// SimpleLLMClient 简单的LLM客户端实现
type SimpleLLMClient struct {
	apiKey string
	model  string
}

// Complete 调用LLM完成请求
func (c *SimpleLLMClient) Complete(prompt string, text string) (string, error) {
	// 这里可以实现实际的LLM调用
	// 目前使用模拟实现
	fmt.Println("🤖 正在调用LLM分析文本...")

	// 模拟LLM响应
	// 实际使用时，这里应该调用OpenAI、Claude或其他LLM API
	return simulateLLMResponse(text), nil
}

// simulateLLMResponse 模拟LLM响应
func simulateLLMResponse(text string) string {
	// 基于文本内容生成简单的分析结果
	characters := extractCharactersFromText(text)
	locations := extractLocationsFromText(text)
	events := extractEventsFromText(text)

	result := map[string]interface{}{
		"characters": characters,
		"items":      []interface{}{},
		"skills":     []interface{}{},
		"locations":  locations,
		"events":     events,
		"timeline":   []interface{}{},
		"analysis": map[string]interface{}{
			"plot_summary":     "从文本中提取的故事情节",
			"power_system":     "基于文本分析的战力体系",
			"potential_issues": []string{},
		},
	}

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonBytes)
}

// extractCharactersFromText 从文本中提取角色
func extractCharactersFromText(text string) []map[string]interface{} {
	// 简单的启发式提取
	var characters []map[string]interface{}

	// 常见角色名模式
	patterns := []string{"林凡", "萧炎", "唐三", "韩立", "叶凡", "石昊", "罗峰", "秦羽",
		"主角", "男主", "女主", "师父", "师傅", "师兄", "师姐", "师弟", "师妹",
		"长老", "宗主", "掌门", "门主", "家主", "族长", "皇帝", "国王"}

	found := make(map[string]bool)
	for _, pattern := range patterns {
		if strings.Contains(text, pattern) && !found[pattern] {
			found[pattern] = true
			charType := "supporting"
			if pattern == "主角" || pattern == "男主" || pattern == "女主" {
				charType = "protagonist"
			}
			characters = append(characters, map[string]interface{}{
				"name":        pattern,
				"type":        charType,
				"personality": "从文本中推断的性格",
				"background":  "从文本中推断的背景",
				"goals":       "从文本中推断的目标",
				"power_level": "未知",
				"confidence":  0.7,
			})
		}
	}

	return characters
}

// extractLocationsFromText 从文本中提取地点
func extractLocationsFromText(text string) []map[string]interface{} {
	var locations []map[string]interface{}

	// 常见地点模式
	patterns := []string{"宗", "门", "城", "镇", "村", "山", "谷", "洞", "府", "阁", "殿", "宫", "岛", "海", "林", "原", "漠", "矿场", "矿洞"}

	found := make(map[string]bool)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		for _, pattern := range patterns {
			if strings.Contains(line, pattern) {
				// 尝试提取地点名
				words := strings.FieldsFunc(line, func(r rune) bool {
					return r == ' ' || r == '，' || r == '。' || r == '、' || r == '；'
				})
				for _, word := range words {
					if strings.Contains(word, pattern) && len(word) >= 2 && len(word) <= 10 && !found[word] {
						found[word] = true
						locations = append(locations, map[string]interface{}{
							"name":        word,
							"type":        "location",
							"description": "从文本中提取的地点",
							"confidence":  0.6,
						})
					}
				}
			}
		}
	}

	return locations
}

// extractEventsFromText 从文本中提取事件
func extractEventsFromText(text string) []map[string]interface{} {
	var events []map[string]interface{}

	// 常见事件模式
	eventPatterns := map[string]string{
		"死亡": "death",
		"复活": "resurrection",
		"战斗": "battle",
		"突破": "breakthrough",
		"修炼": "cultivation",
		"逃跑": "escape",
		"相遇": "meeting",
		"背叛": "betrayal",
		"发现": "discovery",
		"穿越": "transmigration",
		"觉醒": "awakening",
		"升级": "levelup",
		"进阶": "advancement",
		"获得": "acquisition",
		"失去": "loss",
		"胜利": "victory",
		"失败": "defeat",
		"救援": "rescue",
		"被困": "trapped",
		"逃脱": "escape",
		"追杀": "pursuit",
		"复仇": "revenge",
	}

	for chinese, eventType := range eventPatterns {
		if strings.Contains(text, chinese) {
			events = append(events, map[string]interface{}{
				"type":         eventType,
				"participants": []string{},
				"location":     "未知",
				"description":  "文本中提到的" + chinese + "事件",
				"confidence":   0.7,
			})
		}
	}

	return events
}

func main() {
	var (
		inputPath  = flag.String("i", "", "输入小说文件路径 (.md 或 .txt) (必需)")
		outputPath = flag.String("o", "", "输出JSON文件路径 (可选)")
		verbose    = flag.Bool("v", false, "显示详细信息")
		useMock    = flag.Bool("mock", true, "使用模拟LLM (默认true)")
	)
	flag.Parse()

	if *inputPath == "" {
		fmt.Println("错误: 请提供输入文件路径")
		fmt.Println("用法: extract-llm -i <小说文件路径> [选项]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// 读取小说文件
	fmt.Printf("📖 正在读取小说文件: %s\n", *inputPath)
	content, err := os.ReadFile(*inputPath)
	if err != nil {
		fmt.Printf("❌ 错误: 无法读取文件: %v\n", err)
		os.Exit(1)
	}

	// 创建LLM客户端
	var client rpg.LLMClient
	if *useMock {
		fmt.Println("🧪 使用模拟LLM客户端")
		client = &rpg.MockLLMClient{}
	} else {
		fmt.Println("🤖 使用真实LLM客户端")
		client = &SimpleLLMClient{}
	}

	// 创建LLM提取器
	extractor := rpg.NewLLMExtractor(client)

	// 执行提取
	fmt.Println("🤖 LLM正在分析小说内容...")
	data, err := extractor.ExtractFromNovel(string(content))
	if err != nil {
		fmt.Printf("❌ 提取失败: %v\n", err)
		os.Exit(1)
	}

	// 显示结果
	fmt.Println("\n============================================================")
	fmt.Println("📊 LLM提取结果")
	fmt.Println("============================================================")

	fmt.Printf("\n【角色】(%d)\n", len(data.Characters))
	for _, char := range data.Characters {
		fmt.Printf("  - %s [类型:%s]\n", char.Name, char.Type)
	}

	fmt.Printf("\n【物品】(%d)\n", len(data.Items))
	for _, item := range data.Items {
		fmt.Printf("  - %s [%s]\n", item.Name, item.Type)
	}

	fmt.Printf("\n【技能】(%d)\n", len(data.Skills))
	for _, skill := range data.Skills {
		fmt.Printf("  - %s [%s]\n", skill.Name, skill.Type)
	}

	fmt.Printf("\n【地点】(%d)\n", len(data.Locations))
	for _, loc := range data.Locations {
		fmt.Printf("  - %s [%s]\n", loc.Name, loc.Type)
	}

	fmt.Printf("\n【事件统计】\n")
	for _, event := range data.Events {
		fmt.Printf("  - %s\n", event.Name)
	}

	// 保存到文件
	if *outputPath != "" {
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			fmt.Printf("❌ 序列化失败: %v\n", err)
			os.Exit(1)
		}

		if err := os.WriteFile(*outputPath, jsonData, 0644); err != nil {
			fmt.Printf("❌ 保存失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n💾 结果已保存到: %s\n", *outputPath)
	}

	if *verbose {
		fmt.Println("\n============================================================")
		fmt.Println("📋 详细信息")
		fmt.Println("============================================================")
		jsonData, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(jsonData))
	}

	fmt.Println("\n✅ LLM提取完成!")
}
