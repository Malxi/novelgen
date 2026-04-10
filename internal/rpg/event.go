package rpg

import (
	"encoding/json"
	"math/rand"
)

// 事件触发类型
type EventTriggerType string

const (
	EventTriggerAuto        EventTriggerType = "auto"        // 自动触发
	EventTriggerTouch       EventTriggerType = "touch"       // 接触触发
	EventTriggerAction      EventTriggerType = "action"      // 交互触发（按键）
	EventTriggerParallel    EventTriggerType = "parallel"    // 并行处理（每帧执行）
	EventTriggerAutorun     EventTriggerType = "autorun"     // 自动执行（阻塞）
	EventTriggerCollision   EventTriggerType = "collision"   // 碰撞触发
)

// 事件命令类型
type EventCommandType string

const (
	// 消息相关
	CmdShowText        EventCommandType = "show_text"        // 显示文本
	CmdShowChoices     EventCommandType = "show_choices"     // 显示选项
	CmdInputNumber     EventCommandType = "input_number"     // 输入数字
	CmdShowTextBubble  EventCommandType = "show_text_bubble" // 显示气泡文本
	
	// 角色相关
	CmdChangeHP        EventCommandType = "change_hp"        // 改变HP
	CmdChangeMP        EventCommandType = "change_mp"        // 改变MP
	CmdChangeState     EventCommandType = "change_state"     // 改变状态
	CmdRecoverAll      EventCommandType = "recover_all"      // 完全恢复
	CmdChangeLevel     EventCommandType = "change_level"     // 改变等级
	CmdChangeExp       EventCommandType = "change_exp"       // 改变经验
	CmdChangeParam     EventCommandType = "change_param"     // 改变属性
	CmdLearnSkill      EventCommandType = "learn_skill"      // 学习技能
	CmdForgetSkill     EventCommandType = "forget_skill"     // 遗忘技能
	CmdChangeEquip     EventCommandType = "change_equip"     // 改变装备
	CmdChangeName      EventCommandType = "change_name"      // 改变名称
	CmdChangeClass     EventCommandType = "change_class"     // 改变职业
	
	// 移动相关
	CmdTransferPlayer  EventCommandType = "transfer_player"  // 传送玩家
	CmdSetMoveRoute    EventCommandType = "set_move_route"   // 设置移动路线
	CmdScrollMap       EventCommandType = "scroll_map"       // 滚动地图
	
	// 角色移动
	CmdMoveCharacter   EventCommandType = "move_character"   // 移动角色
	CmdJumpCharacter   EventCommandType = "jump_character"   // 跳跃
	CmdRotateCharacter EventCommandType = "rotate_character" // 旋转
	CmdChangeOpacity   EventCommandType = "change_opacity"   // 改变透明度
	CmdChangeGraphic   EventCommandType = "change_graphic"   // 改变图像
	
	// 地图相关
	CmdChangeTileset   EventCommandType = "change_tileset"   // 改变图块集
	CmdChangeBattleBG  EventCommandType = "change_battle_bg" // 改变战斗背景
	CmdChangeParallax  EventCommandType = "change_parallax"  // 改变远景
	CmdChangeWeather   EventCommandType = "change_weather"   // 改变天气
	CmdChangeFog       EventCommandType = "change_fog"       // 改变雾
	CmdChangeTint      EventCommandType = "change_tint"      // 改变色调
	CmdFlashScreen     EventCommandType = "flash_screen"     // 闪烁屏幕
	CmdShakeScreen     EventCommandType = "shake_screen"     // 震动屏幕
	CmdWait            EventCommandType = "wait"             // 等待
	
	// 图片/动画
	CmdShowPicture     EventCommandType = "show_picture"     // 显示图片
	CmdMovePicture     EventCommandType = "move_picture"     // 移动图片
	CmdRotatePicture   EventCommandType = "rotate_picture"   // 旋转图片
	CmdTintPicture     EventCommandType = "tint_picture"     // 色调图片
	CmdErasePicture    EventCommandType = "erase_picture"    // 擦除图片
	CmdShowAnimation   EventCommandType = "show_animation"   // 显示动画
	
	// 音频
	CmdPlayBGM         EventCommandType = "play_bgm"         // 播放BGM
	CmdFadeOutBGM      EventCommandType = "fade_out_bgm"     // 淡出BGM
	CmdSaveBGM         EventCommandType = "save_bgm"         // 保存BGM
	CmdReplayBGM       EventCommandType = "replay_bgm"       // 重播BGM
	CmdPlayBGS         EventCommandType = "play_bgs"         // 播放BGS
	CmdFadeOutBGS      EventCommandType = "fade_out_bgs"     // 淡出BGS
	CmdPlayME          EventCommandType = "play_me"          // 播放ME
	CmdPlaySE          EventCommandType = "play_se"          // 播放SE
	CmdStopSE          EventCommandType = "stop_se"          // 停止SE
	
	// 场景控制
	CmdBattle          EventCommandType = "battle"           // 战斗处理
	CmdShop            EventCommandType = "shop"             // 商店处理
	CmdNameInput       EventCommandType = "name_input"       // 名称输入
	CmdMenu            EventCommandType = "menu"             // 打开菜单
	CmdSave            EventCommandType = "save"             // 打开存档
	CmdGameOver        EventCommandType = "game_over"        // 游戏结束
	CmdTitle           EventCommandType = "title"            // 返回标题
	
	// 系统设置
	CmdChangeItem      EventCommandType = "change_item"      // 增减物品
	CmdChangeWeapon    EventCommandType = "change_weapon"    // 增减武器
	CmdChangeArmor     EventCommandType = "change_armor"     // 增减护甲
	CmdChangeMoney     EventCommandType = "change_money"     // 增减金钱
	
	// 变量/开关
	CmdControlSwitch   EventCommandType = "control_switch"   // 操作开关
	CmdControlVariable EventCommandType = "control_variable" // 操作变量
	CmdControlSelfSwitch EventCommandType = "control_self_switch" // 操作独立开关
	
	// 流程控制
	CmdConditionalBranch EventCommandType = "conditional_branch" // 条件分支
	CmdLoop            EventCommandType = "loop"             // 循环
	CmdBreakLoop       EventCommandType = "break_loop"       // 跳出循环
	CmdExitEvent       EventCommandType = "exit_event"       // 退出事件
	CmdCallEvent       EventCommandType = "call_event"       // 调用事件
	CmdLabel           EventCommandType = "label"            // 标签
	CmdJumpToLabel     EventCommandType = "jump_to_label"    // 跳转到标签
	
	// 脚本
	CmdScript          EventCommandType = "script"           // 执行脚本
)

// 事件命令
type EventCommand struct {
	Code       EventCommandType `json:"code"`
	Indent     int              `json:"indent"`     // 缩进级别（用于分支）
	Parameters []interface{}    `json:"parameters"` // 参数
}

// 事件页
type EventPage struct {
	ID         int              `json:"id"`
	Conditions EventConditions  `json:"conditions"` // 触发条件
	Graphic    EventGraphic     `json:"graphic"`    // 图像
	MoveType   int              `json:"move_type"`  // 移动类型
	MoveSpeed  int              `json:"move_speed"`
	MoveFreq   int              `json:"move_freq"`
	WalkAnime  bool             `json:"walk_anime"`
	StepAnime  bool             `json:"step_anime"`
	Direction  bool             `json:"direction"`
	Through    bool             `json:"through"`    // 穿透
	Priority   int              `json:"priority"`   // 优先级
	Trigger    EventTriggerType `json:"trigger"`
	List       []EventCommand   `json:"list"`       // 命令列表
}

// 事件触发条件
type EventConditions struct {
	Switch1Valid    bool   `json:"switch1_valid"`
	Switch1ID       string `json:"switch1_id,omitempty"`
	Switch2Valid    bool   `json:"switch2_valid"`
	Switch2ID       string `json:"switch2_id,omitempty"`
	VariableValid   bool   `json:"variable_valid"`
	VariableID      string `json:"variable_id,omitempty"`
	VariableValue   int    `json:"variable_value,omitempty"`
	SelfSwitchValid bool   `json:"self_switch_valid"`
	SelfSwitchCh    string `json:"self_switch_ch,omitempty"` // A, B, C, D
	ItemValid       bool   `json:"item_valid"`
	ItemID          string `json:"item_id,omitempty"`
	ActorValid      bool   `json:"actor_valid"`
	ActorID         string `json:"actor_id,omitempty"`
	TimerValid      bool   `json:"timer_valid"`
	TimerSec        int    `json:"timer_sec,omitempty"`
}

// 事件图像
type EventGraphic struct {
	CharacterName string `json:"character_name"`
	CharacterIndex int   `json:"character_index"`
	Direction     int    `json:"direction"`
	Pattern       int    `json:"pattern"`
	TileID        int    `json:"tile_id,omitempty"`
}

// 事件定义
type Event struct {
	ID       string      `json:"id"`
	Name     string      `json:"name,omitempty"`
	X        float64     `json:"x"`
	Y        float64     `json:"y"`
	Pages    []EventPage `json:"pages"`
	
	// 运行时状态
	SelfSwitches map[string]bool `json:"self_switches,omitempty"` // 独立开关状态
	ActivePage   int             `json:"active_page,omitempty"`   // 当前活动页
}

// 公共事件
type CommonEvent struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Trigger     int            `json:"trigger"` // 0: 无, 1: 自动执行, 2: 并行处理
	SwitchID    string         `json:"switch_id,omitempty"`
	List        []EventCommand `json:"list"`
}

// 事件执行上下文
type EventContext struct {
	EventID     string                 `json:"event_id"`
	PageID      int                    `json:"page_id"`
	CommandIndex int                   `json:"command_index"`
	Variables   map[string]interface{} `json:"variables"`
	Switches    map[string]bool        `json:"switches"`
	Waiting     bool                   `json:"waiting"`
	WaitTime    int                    `json:"wait_time"`
	BranchStack []bool                 `json:"branch_stack"` // 分支结果栈
}

// 事件执行器
type EventExecutor struct {
	world       *World
	contexts    map[string]*EventContext
	globalVars  map[string]interface{}
	globalSwitches map[string]bool
}

// 世界上下文（用于事件执行）
type World struct {
	CharacterMgr *CharacterManager
	ItemMgr      *ItemManager
	SkillMgr     *SkillManager
	ClassMgr     *ClassManager
	EquipmentMgr *EquipmentManager
	MapMgr       *MapManager
	QuestMgr     *QuestManager
	Player       *Character
	CurrentMap   string
}

// 创建事件执行器
func NewEventExecutor(world *World) *EventExecutor {
	return &EventExecutor{
		world:          world,
		contexts:       make(map[string]*EventContext),
		globalVars:     make(map[string]interface{}),
		globalSwitches: make(map[string]bool),
	}
}

// 执行事件
func (ee *EventExecutor) ExecuteEvent(event *Event, pageIndex int) *EventResult {
	if pageIndex >= len(event.Pages) {
		return nil
	}
	
	page := event.Pages[pageIndex]
	
	// 检查条件
	if !ee.checkConditions(page.Conditions) {
		return nil
	}
	
	context := &EventContext{
		EventID:      event.ID,
		PageID:       pageIndex,
		CommandIndex: 0,
		Variables:    make(map[string]interface{}),
		Switches:     make(map[string]bool),
		BranchStack:  make([]bool, 0),
	}
	
	ee.contexts[event.ID] = context
	
	result := &EventResult{
		EventID:  event.ID,
		Commands: make([]ExecutedCommand, 0),
	}
	
	// 执行命令列表
	for context.CommandIndex < len(page.List) {
		cmd := page.List[context.CommandIndex]
		executed := ee.executeCommand(cmd, context)
		result.Commands = append(result.Commands, executed)
		
		if executed.Break {
			break
		}
		
		context.CommandIndex++
	}
	
	return result
}

// 检查条件
func (ee *EventExecutor) checkConditions(conds EventConditions) bool {
	if conds.Switch1Valid && !ee.globalSwitches[conds.Switch1ID] {
		return false
	}
	if conds.Switch2Valid && !ee.globalSwitches[conds.Switch2ID] {
		return false
	}
	if conds.VariableValid {
		val, ok := ee.globalVars[conds.VariableID].(int)
		if !ok || val < conds.VariableValue {
			return false
		}
	}
	// 其他条件检查...
	return true
}

// 执行单个命令
func (ee *EventExecutor) executeCommand(cmd EventCommand, ctx *EventContext) ExecutedCommand {
	result := ExecutedCommand{
		Command: cmd.Code,
		Success: true,
	}
	
	switch cmd.Code {
	case CmdShowText:
		if len(cmd.Parameters) > 0 {
			result.Message = cmd.Parameters[0].(string)
		}
		
	case CmdShowChoices:
		result.Choices = make([]string, 0)
		for _, p := range cmd.Parameters {
			result.Choices = append(result.Choices, p.(string))
		}
		
	case CmdChangeHP:
		charID := cmd.Parameters[0].(string)
		amount := cmd.Parameters[1].(int)
		char := ee.world.CharacterMgr.GetCharacter(charID)
		if char != nil {
			char.CurrentStats.HP += amount
			if char.CurrentStats.HP > char.BaseStats.HP {
				char.CurrentStats.HP = char.BaseStats.HP
			}
			if char.CurrentStats.HP <= 0 {
				char.CurrentStats.HP = 0
				char.State = CharacterStateDead
			}
		}
		
	case CmdChangeItem:
		itemID := cmd.Parameters[0].(string)
		amount := cmd.Parameters[1].(int)
		if ee.world.Player != nil {
			if amount > 0 {
				ee.world.Player.AddItem(itemID, amount)
			} else {
				ee.world.Player.RemoveItem(itemID, -amount)
			}
		}
		
	case CmdChangeMoney:
		amount := cmd.Parameters[0].(int)
		// 实现金钱系统...
		result.Value = amount
		
	case CmdControlSwitch:
		switchID := cmd.Parameters[0].(string)
		value := cmd.Parameters[1].(bool)
		ee.globalSwitches[switchID] = value
		
	case CmdControlVariable:
		varID := cmd.Parameters[0].(string)
		operation := cmd.Parameters[1].(string) // set, add, sub, mul, div, mod
		value := cmd.Parameters[2].(int)
		
		current, _ := ee.globalVars[varID].(int)
		switch operation {
		case "set":
			ee.globalVars[varID] = value
		case "add":
			ee.globalVars[varID] = current + value
		case "sub":
			ee.globalVars[varID] = current - value
		case "mul":
			ee.globalVars[varID] = current * value
		case "div":
			if value != 0 {
				ee.globalVars[varID] = current / value
			}
		}
		
	case CmdTransferPlayer:
		mapID := cmd.Parameters[0].(string)
		x := cmd.Parameters[1].(float64)
		y := cmd.Parameters[2].(float64)
		ee.world.CurrentMap = mapID
		if ee.world.Player != nil {
			ee.world.Player.Position.MapID = mapID
			ee.world.Player.Position.X = x
			ee.world.Player.Position.Y = y
		}
		
	case CmdConditionalBranch:
		// 简化版条件分支
		condition := cmd.Parameters[0].(Condition)
		passed := ee.evaluateCondition(condition)
		ctx.BranchStack = append(ctx.BranchStack, passed)
		result.BranchResult = passed
		
	case CmdWait:
		frames := cmd.Parameters[0].(int)
		ctx.Waiting = true
		ctx.WaitTime = frames
		
	case CmdScript:
		script := cmd.Parameters[0].(string)
		result.Script = script
		// 这里可以执行脚本...
		
	case CmdExitEvent:
		result.Break = true
	}
	
	return result
}

// 评估条件
func (ee *EventExecutor) evaluateCondition(cond Condition) bool {
	switch cond.Type {
	case ConditionFlag:
		return ee.globalSwitches[cond.TargetID]
	case ConditionHasItem:
		if ee.world.Player != nil {
			for _, item := range ee.world.Player.Items {
				if item.ItemID == cond.TargetID {
					return true
				}
			}
		}
		return false
	case ConditionLevel:
		if ee.world.Player != nil {
			level := ee.world.Player.Level
			targetLevel := cond.Value.(int)
			switch cond.Operator {
			case ">=":
				return level >= targetLevel
			case ">":
				return level > targetLevel
			case "<=":
				return level <= targetLevel
			case "<":
				return level < targetLevel
			case "==":
				return level == targetLevel
			}
		}
	case ConditionRandom:
		chance := cond.Value.(float64)
		return rand.Float64() <= chance
	}
	return false
}

// 获取开关值
func (ee *EventExecutor) GetSwitch(id string) bool {
	return ee.globalSwitches[id]
}

// 设置开关
func (ee *EventExecutor) SetSwitch(id string, value bool) {
	ee.globalSwitches[id] = value
}

// 获取变量
func (ee *EventExecutor) GetVariable(id string) interface{} {
	return ee.globalVars[id]
}

// 设置变量
func (ee *EventExecutor) SetVariable(id string, value interface{}) {
	ee.globalVars[id] = value
}

// 执行结果
type EventResult struct {
	EventID  string            `json:"event_id"`
	Commands []ExecutedCommand `json:"commands"`
}

// 已执行命令
type ExecutedCommand struct {
	Command      EventCommandType `json:"command"`
	Success      bool             `json:"success"`
	Message      string           `json:"message,omitempty"`
	Choices      []string         `json:"choices,omitempty"`
	Value        interface{}      `json:"value,omitempty"`
	BranchResult bool             `json:"branch_result,omitempty"`
	Script       string           `json:"script,omitempty"`
	Break        bool             `json:"break,omitempty"`
}

// 事件管理器
type EventManager struct {
	events       map[string]*Event
	commonEvents map[string]*CommonEvent
	executor     *EventExecutor
}

func NewEventManager(executor *EventExecutor) *EventManager {
	return &EventManager{
		events:       make(map[string]*Event),
		commonEvents: make(map[string]*CommonEvent),
		executor:     executor,
	}
}

func (em *EventManager) AddEvent(event *Event) {
	em.events[event.ID] = event
}

func (em *EventManager) AddCommonEvent(event *CommonEvent) {
	em.commonEvents[event.ID] = event
}

func (em *EventManager) GetEvent(id string) *Event {
	return em.events[id]
}

func (em *EventManager) GetCommonEvent(id string) *CommonEvent {
	return em.commonEvents[id]
}

func (em *EventManager) TriggerEvent(eventID string, pageIndex int) *EventResult {
	event := em.events[eventID]
	if event == nil {
		return nil
	}
	return em.executor.ExecuteEvent(event, pageIndex)
}

// 序列化
func (e *Event) ToJSON() string {
	data, _ := json.MarshalIndent(e, "", "  ")
	return string(data)
}

func (ce *CommonEvent) ToJSON() string {
	data, _ := json.MarshalIndent(ce, "", "  ")
	return string(data)
}
