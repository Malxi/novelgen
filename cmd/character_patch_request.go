package cmd

import "strings"

func buildCharacterPresencePatchRequest(missing []string, unexpected []string) string {
	var sb strings.Builder
	sb.WriteString("【PATCH 请求：CHARACTER_PRESENCE_PATCH】\n")
	sb.WriteString("你需要仅输出一段可直接插入正文开头的‘角色出场补丁段’（120–220字），并用下面格式包裹：\n")
	sb.WriteString("<CHARACTER_PRESENCE_PATCH>\n")
	sb.WriteString("...补丁正文...\n")
	sb.WriteString("</CHARACTER_PRESENCE_PATCH>\n")
	sb.WriteString("要求：\n")
	sb.WriteString("1) 目标：让大纲 characters 列表中的角色在正文中真实出场（至少一句动作/台词/被提及的明确描写）。\n")
	sb.WriteString("2) 补丁不得引入新剧情大事件，只做补出场/补承接。\n")
	sb.WriteString("3) 语气与本章一致，尽量自然地嵌入，不要写元评论。\n")
	sb.WriteString("4) 仅输出补丁块，不要重写完整章节。\n")
	if len(missing) > 0 {
		sb.WriteString("必须补出场的角色：" + strings.Join(missing, ", ") + "\n")
	}
	if len(unexpected) > 0 {
		sb.WriteString("若开头出现不该出现的角色，请用一句切镜/并线说明或降低其出镜：" + strings.Join(unexpected, ", ") + "\n")
	}
	return sb.String()
}
