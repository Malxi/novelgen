package dsl

import (
	"regexp"
	"strconv"
	"strings"
)

func buildStructuredProgressionDeltas(cultivation string, keyItems, injuries []string) []StateDelta {
	var deltas []StateDelta
	if level := extractGeneLevel(cultivation); level > 0 {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "gene",
			Field:  "level",
			To:     strconv.Itoa(level),
			Delta:  level,
			Note:   "structured gene level from cultivation: " + cultivation,
		})
	}
	if stability := extractGeneStability(cultivation); stability > 0 {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "gene",
			Field:  "stability",
			To:     strconv.Itoa(stability),
			Delta:  stability,
			Unit:   "%",
			Note:   "structured gene stability from cultivation: " + cultivation,
		})
	}

	mech := extractMechSnapshot(keyItems)
	if mech.Form != "" {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "mech",
			Field:  "form",
			To:     mech.Form,
			Note:   "structured mech form from key_items",
		})
		if mech.Level > 0 {
			deltas = append(deltas, StateDelta{
				Target: "protagonist",
				Kind:   "mech",
				Field:  "level",
				To:     strconv.Itoa(mech.Level),
				Delta:  mech.Level,
				Note:   "structured mech level from form: " + mech.Form,
			})
		}
	}
	if mech.Energy > 0 {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "mech",
			Field:  "energy",
			To:     strconv.Itoa(mech.Energy),
			Delta:  mech.Energy,
			Unit:   "%",
			Note:   "structured mech energy from key_items",
		})
	}
	for _, module := range mech.Modules {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "mech",
			Field:  "module",
			To:     module,
			Note:   "structured mech module from key_items",
		})
	}
	for _, blueprint := range mech.ModuleBlueprints {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "mech",
			Field:  "module_blueprint",
			To:     blueprint,
			Note:   "structured mech module blueprint from key_items",
		})
	}
	for _, damage := range mech.Damage {
		deltas = append(deltas, StateDelta{
			Target: "protagonist",
			Kind:   "mech",
			Field:  "damage",
			To:     damage,
			Note:   "structured mech damage from key_items",
		})
	}

	for _, injury := range injuries {
		if stability := extractGeneStability(injury); stability > 0 {
			deltas = append(deltas, StateDelta{
				Target: "protagonist",
				Kind:   "gene",
				Field:  "stability",
				To:     strconv.Itoa(stability),
				Delta:  stability,
				Unit:   "%",
				Note:   "structured gene stability from injury note: " + injury,
			})
		}
	}
	return deltas
}

func extractGeneLevel(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`([一二三四五六七八九十\d]+)\s*级\s*基因`),
		regexp.MustCompile(`基因(?:适配|进化|强化|等级)?\s*([一二三四五六七八九十\d]+)\s*级`),
	}
	for _, pattern := range patterns {
		if match := pattern.FindStringSubmatch(text); len(match) >= 2 {
			if level := parseNarrativeLevel(match[1]); level > 0 {
				return level
			}
		}
	}
	return 0
}

func extractGeneStability(text string) int {
	text = strings.TrimSpace(text)
	if text == "" || !strings.Contains(text, "基因") {
		return 0
	}
	pattern := regexp.MustCompile(`基因(?:稳定性|稳定度|稳定)?\s*([0-9]{1,3})\s*%`)
	if match := pattern.FindStringSubmatch(text); len(match) >= 2 {
		if value, err := strconv.Atoi(match[1]); err == nil && value >= 0 && value <= 100 {
			return value
		}
	}
	return 0
}

type mechSnapshot struct {
	Form             string
	Level            int
	Energy           int
	Modules          []string
	ModuleBlueprints []string
	Damage           []string
}

func extractMechSnapshot(keyItems []string) mechSnapshot {
	var snapshot mechSnapshot
	for _, raw := range keyItems {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if isMechFormItem(item) {
			form := cleanMechForm(item)
			if form != "" {
				snapshot.Form = form
				snapshot.Level = inferMechLevelFromForm(form)
			}
		}
		if energy := extractMechEnergy(item); energy > 0 {
			snapshot.Energy = energy
		}
		modules, blueprints := extractMechModules(item)
		snapshot.Modules = mergeNames(snapshot.Modules, modules)
		snapshot.ModuleBlueprints = mergeNames(snapshot.ModuleBlueprints, blueprints)
		snapshot.Damage = mergeNames(snapshot.Damage, extractMechDamage(item))
	}
	return snapshot
}

func isMechFormItem(item string) bool {
	if !containsAnyText(item, "火种机甲", "火种甲", "机甲", "外骨骼") {
		return false
	}
	if containsAnyText(item, "蓝图", "密钥", "编码") {
		return false
	}
	if containsAnyText(item, "部件", "零件", "导管", "装甲板") {
		return false
	}
	if strings.Contains(item, "核心") && !containsAnyText(item, "能量", "组装完成", "穿戴", "已解锁") {
		return false
	}
	return true
}

func cleanMechForm(item string) string {
	form := stripParenthetical(item)
	form = strings.TrimSpace(form)
	form = strings.TrimSuffix(form, "×1")
	form = strings.TrimSuffix(form, "x1")
	return strings.TrimSpace(form)
}

func inferMechLevelFromForm(form string) int {
	switch {
	case containsAnyText(form, "星际", "母巢", "全球修复"):
		return 7
	case containsAnyText(form, "指挥链路", "联军"):
		return 5
	case containsAnyText(form, "飞行"):
		return 4
	case containsAnyText(form, "改良", "远程", "重火力"):
		return 3
	case containsAnyText(form, "基础版", "基础火种甲", "基础"):
		return 2
	case containsAnyText(form, "残骸", "外骨骼"):
		return 1
	default:
		if containsAnyText(form, "火种机甲", "火种甲", "机甲") {
			return 2
		}
		return 0
	}
}

func extractMechEnergy(item string) int {
	if !strings.Contains(item, "能量") {
		return 0
	}
	pattern := regexp.MustCompile(`能量(?:恢复至|恢复到|至|剩余|为)?\s*([0-9]{1,3})\s*%`)
	if match := pattern.FindStringSubmatch(item); len(match) >= 2 {
		if value, err := strconv.Atoi(match[1]); err == nil && value >= 0 && value <= 100 {
			return value
		}
	}
	fallback := regexp.MustCompile(`([0-9]{1,3})\s*%`)
	if match := fallback.FindStringSubmatch(item); len(match) >= 2 {
		if value, err := strconv.Atoi(match[1]); err == nil && value >= 0 && value <= 100 {
			return value
		}
	}
	return 0
}

func extractMechModules(item string) ([]string, []string) {
	if !strings.Contains(item, "模块") {
		return nil, nil
	}
	var modules []string
	var blueprints []string
	for _, token := range splitLooseTokens(item) {
		if !strings.Contains(token, "模块") {
			continue
		}
		name := normalizeMechModuleName(token)
		if name == "" {
			continue
		}
		if strings.Contains(token, "蓝图") || strings.Contains(token, "图纸") {
			blueprints = mergeNames(blueprints, []string{name})
		} else {
			modules = mergeNames(modules, []string{name})
		}
	}
	return modules, blueprints
}

func normalizeMechModuleName(token string) string {
	token = strings.TrimSpace(token)
	token = strings.TrimPrefix(token, "已解锁")
	token = strings.TrimPrefix(token, "解锁")
	token = strings.TrimPrefix(token, "旧文明数据芯片")
	token = strings.Trim(token, "（）()[]【】")
	replacements := []string{"可用", "可校准", "已获得", "临时", "前置条件", "尚未安装", "已安装"}
	for _, replacement := range replacements {
		token = strings.ReplaceAll(token, replacement, "")
	}
	token = strings.ReplaceAll(token, "蓝图", "")
	token = strings.ReplaceAll(token, "图纸", "")
	token = regexp.MustCompile(`v\d+(?:\.\d+)?`).ReplaceAllString(token, "")
	if idx := strings.Index(token, "·"); idx >= 0 {
		token = token[:idx]
	}
	token = strings.Trim(token, " ：:，,。；;、-")
	if !strings.Contains(token, "模块") {
		return ""
	}
	return token
}

func extractMechDamage(item string) []string {
	if containsAnyText(item, "已修复", "修复完成") {
		return []string{"none"}
	}
	var damage []string
	for _, token := range splitLooseTokens(item) {
		if containsAnyText(token, "受损", "损坏", "凹痕", "裂纹", "过热", "锁死") {
			damage = mergeNames(damage, []string{strings.TrimSpace(token)})
		}
	}
	return damage
}

func stripParenthetical(text string) string {
	for {
		start, endMark := firstParentheticalStart(text)
		if start < 0 {
			return strings.TrimSpace(text)
		}
		end := strings.Index(text[start:], endMark)
		if end < 0 {
			return strings.TrimSpace(text[:start])
		}
		text = text[:start] + text[start+end+len(endMark):]
	}
}

func firstParentheticalStart(text string) (int, string) {
	full := strings.Index(text, "（")
	ascii := strings.Index(text, "(")
	switch {
	case full < 0 && ascii < 0:
		return -1, ""
	case full >= 0 && (ascii < 0 || full < ascii):
		return full, "）"
	default:
		return ascii, ")"
	}
}

func splitLooseTokens(text string) []string {
	replacer := strings.NewReplacer("（", ",", "）", ",", "(", ",", ")", ",", "、", ",", "；", ",", ";", ",", "，", ",")
	parts := strings.Split(replacer.Replace(text), ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseStateDeltaInt(delta StateDelta) int {
	candidates := []string{delta.To, delta.From}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if value, err := strconv.Atoi(candidate); err == nil {
			return value
		}
		if value := parseNarrativeLevel(candidate); value > 0 {
			return value
		}
	}
	if delta.Delta != 0 {
		return delta.Delta
	}
	return 0
}

func clampPercent(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}
