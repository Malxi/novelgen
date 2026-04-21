package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"novelgen/internal/rpg"
	"novelgen/internal/rpg/dsl"
)

func main() {
	separator := strings.Repeat("=", 50)
	fmt.Println("🚀 Fire-Galaxy DSL 测试开始")
	fmt.Println(separator)

	// 1. 创建游戏世界
	fmt.Println("\n🌍 创建游戏世界...")
	world := createFireGalaxyWorld()
	fmt.Printf("✅ 世界创建完成\n")
	fmt.Printf("   - 玩家: %s (Level %d)\n", world.Player.Name, world.Player.Level)
	fmt.Printf("   - 当前位置: %s\n", world.Player.Position.MapID)
	fmt.Printf("   - NPC数量: %d\n", len(world.Characters.GetAllCharacters()))

	// 2. 测试增强的 Hook 系统
	fmt.Println("\n🪝 测试增强 Hook 系统...")
	testEnhancedHooks(world)

	// 3. 测试增强的表达式求值
	fmt.Println("\n🧮 测试增强表达式求值...")
	testEnhancedExpressions(world)

	// 4. 测试 DSL 执行日志
	fmt.Println("\n📝 测试 DSL 执行日志...")
	testDSLLogging(world)

	// 5. 测试错误报告
	fmt.Println("\n❌ 测试错误报告系统...")
	testErrorReporting()

	fmt.Println("\n" + separator)
	fmt.Println("✨ 所有测试完成!")
}

func createFireGalaxyWorld() *rpg.GameWorld {
	world := rpg.NewGameWorld()

	// 创建主角陆星眠
	protagonistTemplate := &rpg.CharacterTemplate{
		ID:   "template_陆星眠",
		Name: "陆星眠",
		Type: rpg.CharacterTypePlayer,
		BaseStats: rpg.BaseStats{
			HP: 100, MP: 50, Attack: 15, Defense: 10,
			Magic: 10, Resistance: 10, Speed: 15, Luck: 10,
		},
		GrowthStats: rpg.GrowthStats{
			HP: 12, MP: 8, Attack: 3,
			Defense: 2, Magic: 3, Resistance: 2,
			Speed: 2, Luck: 1,
		},
	}
	protagonist := rpg.NewCharacter(protagonistTemplate, "陆星眠")
	protagonist.Description = "来自21世纪的普通人，因意外进入休眠仓穿越到3024年"
	protagonist.Skills = []string{"快速学习", "适应环境", "腐蚀免疫"}
	world.SetPlayer(protagonist)

	// 创建NPC - 陈野
	chenYeTemplate := &rpg.CharacterTemplate{
		ID:   "template_陈野",
		Name: "陈野",
		Type: rpg.CharacterTypeNPC,
		BaseStats: rpg.BaseStats{
			HP: 80, MP: 30, Attack: 12, Defense: 8,
			Magic: 5, Resistance: 6, Speed: 10, Luck: 8,
		},
	}
	chenYe := rpg.NewCharacter(chenYeTemplate, "陈野")
	chenYe.Description = "拾荒队长，性格豪爽，带领小队在废墟中生存"
	world.Characters.AddCharacterInstance(chenYe)

	// 创建NPC - 苏晓
	suXiaoTemplate := &rpg.CharacterTemplate{
		ID:   "template_苏晓",
		Name: "苏晓",
		Type: rpg.CharacterTypeNPC,
		BaseStats: rpg.BaseStats{
			HP: 60, MP: 40, Attack: 8, Defense: 6,
			Magic: 12, Resistance: 8, Speed: 9, Luck: 10,
		},
	}
	suXiao := rpg.NewCharacter(suXiaoTemplate, "苏晓")
	suXiao.Description = "队医，温柔细心，负责小队的医疗工作"
	world.Characters.AddCharacterInstance(suXiao)

	// 创建敌人 - 低阶镰虫
	insectTemplate := &rpg.CharacterTemplate{
		ID:   "template_低阶镰虫",
		Name: "低阶镰虫",
		Type: rpg.CharacterTypeEnemy,
		BaseStats: rpg.BaseStats{
			HP: 60, MP: 10, Attack: 15, Defense: 5,
			Magic: 2, Resistance: 3, Speed: 12, Luck: 3,
		},
	}
	insect := rpg.NewCharacter(insectTemplate, "低阶镰虫")
	insect.Description = "虫族基础战斗单位，拥有镰刀状前肢"
	insect.Skills = []string{"skill_insect_claw", "skill_acid_spray"}
	world.Characters.AddCharacterInstance(insect)

	// 创建地图 - 锈墙据点
	rustWallBase := &rpg.Map{
		ID:          "map_锈墙据点",
		Name:        "锈墙幸存者据点",
		Description: "人类在废墟中建立的临时避难所，有高墙和防御设施",
		Type:        "city",
		Width:       50,
		Height:      40,
		IsIndoor:    false,
	}
	world.Maps.AddMap(rustWallBase)

	// 创建地图 - 休眠基地
	cryoBase := &rpg.Map{
		ID:          "map_休眠基地",
		Name:        "华国西南废弃地下休眠基地A3区",
		Description: "主角醒来的地方，三百年前的冬眠设施",
		Type:        "dungeon",
		Width:       30,
		Height:      25,
		IsIndoor:    true,
	}
	world.Maps.AddMap(cryoBase)

	// 设置玩家初始位置
	world.Player.Position.MapID = "map_锈墙据点"
	world.Context.CurrentMap = "map_锈墙据点"

	return world
}

// 辅助函数：获取特定类型的角色
func getCharactersByType(cm *rpg.CharacterManager, charType rpg.CharacterType) []*rpg.Character {
	var result []*rpg.Character
	for _, char := range cm.GetAllCharacters() {
		if char.Type == charType {
			result = append(result, char)
		}
	}
	return result
}

func testEnhancedHooks(world *rpg.GameWorld) {
	hm := dsl.NewEnhancedHookManager()

	// 注册一个击杀虫族的 Hook
	hm.RegisterHook("on_kill", "insect_slayer", dsl.HookConfig{
		Filter: "enemy_id starts_with 'template_'",
		Action: "log_kill",
	})

	// 初始化击杀计数器
	hm.InitializeCounter("insect_kills", dsl.CounterConfig{
		Track:      "虫族击杀",
		Milestones: []int{5, 10, 25, 50},
	})

	// 模拟击杀事件
	if world.Player != nil {
		// 获取一个敌人
		enemies := getCharactersByType(world.Characters, rpg.CharacterTypeEnemy)
		if len(enemies) > 0 {
			enemy := enemies[0]

			// 测试击杀
			results := hm.OnKillEnhanced(world.Player, enemy, world)
			fmt.Printf("   ✓ 击杀 [%s] 触发 %d 个 Hook\n", enemy.Name, len(results))

			// 增加计数器
			hm.IncrementCounter("insect_kills")
			hm.IncrementCounter("insect_kills")
			hm.IncrementCounter("insect_kills")

			// 检查里程碑
			count := hm.GetKillCount("insect_kills")
			if hm.IsMilestoneReached("insect_kills", 5) {
				fmt.Printf("   🏆 已达到里程碑: 5 击杀 (当前: %d)\n", count)
			} else {
				fmt.Printf("   ⏳ 当前击杀数: %d/5\n", count)
			}

			// 保存状态
			state := hm.SaveState()
			fmt.Printf("   💾 状态已保存，包含 %d 个数据项\n", len(state))
		}
	}

	fmt.Println("   ✅ Hook 系统测试完成")
}

func testEnhancedExpressions(world *rpg.GameWorld) {
	eval := dsl.NewEnhancedExpressionEvaluator()

	// 测试字符串函数
	tests := []struct {
		name string
		expr string
	}{
		{"字符串长度", `len("陆星眠")`},
		{"字符串连接", `concat("Fire", "-", "Galaxy")`},
		{"大写转换", `upper("rust wall base")`},
		{"数学计算", `abs(-42)`},
		{"平方根", `sqrt(16)`},
		{"条件判断", `if(true, "是", "否")`},
		{"类型检查", `type_of("虫族")`},
		{"数值转换", `to_number("123")`},
	}

	for _, test := range tests {
		result, err := eval.Evaluate(test.expr, nil)
		if err != nil {
			fmt.Printf("   ❌ %s: %v\n", test.name, err)
		} else {
			fmt.Printf("   ✓ %s: %v\n", test.name, result)
		}
	}

	fmt.Println("   ✅ 表达式求值测试完成")
}

func testDSLLogging(world *rpg.GameWorld) {
	// 创建日志记录器
	logger := dsl.NewLogger(os.Stdout, dsl.WithMinLevel(dsl.LogLevelInfo))
	execLogger := dsl.NewDSLExecutionLogger(logger)

	// 记录解析开始
	execLogger.LogParseStart(`metadata { title = "Fire Galaxy" }`)
	time.Sleep(10 * time.Millisecond)
	execLogger.LogParseEnd(true, nil)

	// 记录验证
	execLogger.LogValidationStart()
	execLogger.LogValidationEnd(nil, nil)

	// 记录世界统计
	worldStats := map[string]interface{}{
		"characters": len(world.Characters.GetAllCharacters()),
		"maps":       len(world.Maps.GetAllMaps()),
		"player_lvl": world.Player.Level,
	}
	execLogger.LogSystem("Fire-Galaxy 世界加载完成", worldStats)

	// 记录 Hook 执行
	execLogger.LogHookExecution("on_kill", "陆星眠", 1)

	// 记录表达式求值
	execLogger.LogExpression("level >= 10", false, nil)

	// 生成报告
	report := execLogger.GenerateReport()
	fmt.Printf("   ⏱️  总执行时间: %v ms\n", report["total_duration_ms"])

	fmt.Println("   ✅ 日志系统测试完成")
}

func testErrorReporting() {
	// 测试错误报告
	source := `metadata {
    title = "Fire Galaxy"
    version = "1.0"
}`

	reporter := dsl.NewErrorReporter(source)
	err := &dsl.DSLParseError{
		Pos:     dsl.Position{Line: 2, Column: 5},
		Message: "expected identifier",
		Context: "...metadata { title...",
	}

	report := reporter.Report(err)
	fmt.Printf("   错误报告示例:\n%s\n", report)

	fmt.Println("   ✅ 错误报告测试完成")
}
