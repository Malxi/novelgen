import json

outline = {
    "parts": [
        {
            "id": "P1",
            "title": "第一部：外门风云",
            "summary": "外门弟子觉醒「日志查看系统」，看到大师兄模拟器的30天毁灭预言。凭借信息差截胡苏瑶的机缘、带纪迟签到万剑冢、联手陆青禾全自动拆解。30天大比击败大师兄，夺冠进入内门。",
            "volumes": []
        }
    ]
}

vol1_chapters = [
    {
        "id": "ch1",
        "title": "第1章：觉醒金手指",
        "summary": "主角在宗门后山意外觉醒「日志查看系统」，赫然看到大师兄的模拟器日志——【30天后，你被主角击败】。主角看看自己练气三层的废柴修为，陷入沉思。",
        "characters": ["主角", "大师兄"],
        "location": "玄云宗·外门后山",
        "events": [
            {"actor": "主角", "action": "awaken", "target": "日志查看系统", "target_type": "premise", "context": "后山悬崖", "result": "主角觉醒独一无二的日志查看系统，能看到其他系统宿主的任务面板"},
            {"actor": "大师兄", "action": "use", "target": "模拟器系统", "target_type": "premise", "context": "大师兄密室", "result": "大师兄模拟未来，惊恐发现30天后会被主角击败"},
            {"actor": "主角", "action": "discover", "target": "大师兄的末日预言", "target_type": "knowledge", "context": "后山", "result": "主角通过日志看到大师兄模拟器的完整预言"}
        ],
        "conflict": "主角从废柴到觉醒的震撼，以及对30天预言的恐惧与好奇",
        "pacing": "fast",
        "timeline": {"anchor": "第1天", "start_time": "午后", "end_time": "傍晚", "duration": "当天"},
        "state_anchor": {"cultivation": "练气三层", "spirit_stones": 10, "allies": [], "injuries": [], "location": "玄云宗外门后山", "key_items": [], "notes": "刚觉醒日志查看系统，处于震惊状态"},
        "scenes": [
            {"order": 1, "pov": "主角", "goal": "去后山采药完成杂役任务", "location": "外门后山", "characters": ["主角"], "tone": "轻松", "beats": ["主角抱怨外门杂役的枯燥", "失足滑落悬崖缝隙"]},
            {"order": 2, "pov": "主角", "goal": "在濒死中觉醒系统", "location": "悬崖缝隙", "characters": ["主角", "大师兄(日志中)"], "tone": "紧张", "beats": ["主角摔伤后头部撞击，视野中浮现半透明面板", "系统绑定完成，第一条日志弹出——大师兄的模拟器预言"]}
        ],
        "chapter_payoff": {
            "desire": "活下来，搞清楚发生了什么",
            "pressure": "受伤被困悬崖缝隙，更可怕的是看到了不可思议的预言",
            "clever_move": "在绝境中意外绑定系统，获得信息优势的第一步",
            "payoff_moment": "主角盯着脑海中的半透明面板，大师兄的模拟器日志——【30天后，你被主角击败】——像一道闪电劈开了他的认知",
            "reward": "日志查看系统激活",
            "social_proof": "无人知晓",
            "hook": "30天后会发生什么？大师兄会怎么做？"
        }
    },
    {
        "id": "ch2",
        "title": "第2章：大师兄的第一发暗箭",
        "summary": "大师兄破防后连夜模拟，推演出「明天派人毒杀主角」。主角后台秒收日志，提前得知毒杀时间、地点、毒药种类，决定将计就计。",
        "characters": ["主角", "大师兄", "狗腿子"],
        "location": "玄云宗·外门食堂/后山",
        "events": [
            {"actor": "大师兄", "action": "use", "target": "模拟器系统", "target_type": "premise", "context": "大师兄密室深夜", "result": "大师兄模拟毒杀主角的剧本，付出模拟代价"},
            {"actor": "主角", "action": "discover", "target": "毒杀计划", "target_type": "knowledge", "context": "主角住处", "result": "主角从日志中看到下一次的暗杀细节：时间、地点、毒药"},
            {"actor": "狗腿子", "action": "acquire", "target": "断魂散", "target_type": "item", "context": "黑市", "result": "大师兄派出的狗腿子购买毒药准备下手"}
        ],
        "conflict": "主角提前知道有人要暗杀他，但他的实力远不如对手",
        "pacing": "fast",
        "timeline": {"anchor": "第2天", "time_jump": True, "previous_gap": "次日", "duration": "当天"},
        "state_anchor": {"cultivation": "练气三层", "spirit_stones": 10, "allies": [], "injuries": [], "location": "玄云宗外门", "key_items": [], "notes": "已觉醒日志系统，暗中观察"},
        "scenes": [
            {"order": 1, "pov": "大师兄", "goal": "深夜模拟如何逆转未来", "location": "大师兄密室", "characters": ["大师兄"], "tone": "紧张", "beats": ["大师兄看着【你被主角击败】的预言，手指发抖", "模拟器给出方案：派人毒杀，成功率93%"]},
            {"order": 2, "pov": "主角", "goal": "确认威胁真实性", "location": "外门食堂", "characters": ["主角", "狗腿子"], "tone": "悬疑", "beats": ["主角在食堂看到狗腿子悄悄往一个茶壶里倒粉末", "日志同步刷新：[断魂散投放完毕，目标将在半个时辰后毒发]"]}
        ],
        "chapter_payoff": {
            "desire": "确认预言的真实性，提前应对",
            "pressure": "自己只有练气三层，正面冲突必死",
            "clever_move": "利用日志的信息差，提前两小时知道毒杀的全部细节",
            "payoff_moment": "主角看着食堂里狗腿子鬼鬼祟祟的背影，脑海中的日志清清楚楚写着毒药种类、剂量和发作时间",
            "reward": "掌握了反向操作的全部信息",
            "social_proof": "狗腿子以为计划天衣无缝",
            "hook": "主角会如何化解这场毒杀？"
        }
    },
    {
        "id": "ch3",
        "title": "第3章：反向投毒",
        "summary": "主角将计就计，把毒药反喂给下毒的狗腿子。大师兄第二天看到狗腿子口吐白沫，赶紧再次模拟——【由于下毒失败，30天后你依然被主角击败】。大师兄道心开始动摇。",
        "characters": ["主角", "大师兄", "狗腿子"],
        "location": "玄云宗·外门食堂/执法堂外围",
        "events": [
            {"actor": "主角", "action": "use", "target": "调包毒药", "target_type": "item", "context": "食堂", "result": "主角用日志提示的手法将毒药调换，狗腿子喝下自己下的毒"},
            {"actor": "大师兄", "action": "use", "target": "模拟器系统", "target_type": "premise", "context": "大师兄住处", "result": "模拟器刷新：【由于下毒失败，30天后你依然被主角击败】"},
            {"actor": "狗腿子", "action": "lose", "target": "战斗力", "target_type": "status", "context": "食堂角落", "result": "狗腿子口吐白沫倒地，被执法堂带走"}
        ],
        "conflict": "主角的反向操作能否成功，大师兄道心开始动摇",
        "pacing": "fast",
        "timeline": {"anchor": "第3天", "time_jump": True, "previous_gap": "次日", "duration": "当天"},
        "state_anchor": {"cultivation": "练气三层", "spirit_stones": 10, "allies": [], "injuries": [], "location": "玄云宗外门", "key_items": ["断魂散残渣"], "notes": "成功反制第一次暗杀"},
        "scenes": [
            {"order": 1, "pov": "主角", "goal": "在食堂调包毒茶", "location": "外门食堂", "characters": ["主角", "狗腿子"], "tone": "紧张", "beats": ["主角假装无意撞到狗腿子，趁乱调换茶杯", "狗腿子毫无察觉，喝下自己准备的毒茶"]},
            {"order": 2, "pov": "大师兄", "goal": "检查毒杀结果", "location": "执法堂外围", "characters": ["大师兄", "狗腿子"], "tone": "震惊", "beats": ["大师兄看到狗腿子被抬出食堂口吐白沫", "慌忙打开模拟器：预言未变！30天后的失败不可逆转"]}
        ],
        "chapter_payoff": {
            "desire": "化解毒杀危机，同时不暴露自己",
            "pressure": "必须精确操作，一丝差错就会丧命",
            "clever_move": "利用日志精确知道毒药的发作时间和剂量，完美反向操作",
            "payoff_moment": "大师兄看着模拟器上【下毒失败，预言不变】的冰冷文字，第一次感到后背发凉",
            "reward": "安全度过第一次危机，验证了日志的信息价值",
            "social_proof": "大师兄开始怀疑人生，狗腿子被抬走",
            "hook": "毒杀事件惊动了执法堂——陆青禾即将登场"
        }
    },
    {
        "id": "ch4",
        "title": "第4章：陆青禾的硬核登场",
        "summary": "毒杀事件惊动执法堂，陆青禾带队彻查。她用严密的逻辑推理把所有人问得哑口无言，差点端了大师兄的秘密据点。主角第一次见识到「没挂的女主有多恐怖」。",
        "characters": ["主角", "陆青禾", "大师兄", "执法堂弟子"],
        "location": "玄云宗·外门/执法堂",
        "events": [
            {"actor": "陆青禾", "action": "discover", "target": "毒杀事件线索", "target_type": "knowledge", "context": "食堂现场", "result": "陆青禾从食堂的茶杯、毒药残留、狗腿子的口供中锁定嫌疑方向"},
            {"actor": "陆青禾", "action": "meet", "target": "主角", "target_type": "character", "context": "询问现场", "result": "陆青禾注意到主角的证词过于完美，对他产生初步兴趣"},
            {"actor": "大师兄", "action": "lose", "target": "秘密据点", "target_type": "location", "context": "宗门西区", "result": "陆青禾排查过程中差点找到大师兄的密室，大师兄紧急销毁证据"}
        ],
        "conflict": "陆青禾的铁面排查 vs 大师兄的惊慌失措，主角在夹缝中观察",
        "pacing": "normal",
        "timeline": {"anchor": "第4天", "time_jump": True, "previous_gap": "次日清晨", "duration": "当天"},
        "state_anchor": {"cultivation": "练气三层", "spirit_stones": 10, "allies": [], "injuries": [], "location": "玄云宗外门", "key_items": [], "notes": "刚反杀毒杀事件，正在观察执法堂调查"},
        "scenes": [
            {"order": 1, "pov": "陆青禾", "goal": "调查毒杀事件真相", "location": "外门食堂", "characters": ["陆青禾", "主角", "狗腿子", "执法堂弟子"], "tone": "严肃", "beats": ["陆青禾逐一盘问在场弟子，逻辑链环环相扣", "主角回答问题滴水不漏，陆青禾多看了他一眼"]},
            {"order": 2, "pov": "大师兄", "goal": "销毁证据防止被查", "location": "大师兄秘密据点", "characters": ["大师兄"], "tone": "惊慌", "beats": ["大师兄听到执法堂排查方向，疯狂销毁密室中的证据", "自言自语：「这个女人太可怕了，她根本没有系统，纯靠脑子！」"]}
        ],
        "chapter_payoff": {
            "desire": "不被陆青禾发现自己的秘密",
            "pressure": "陆青禾的推理能力远超预期，主角感觉自己像在被X光扫描",
            "clever_move": "用真话回答真话，只是刻意遗漏了日志系统的存在",
            "payoff_moment": "陆青禾转身离开前突然回头：「你的证词逻辑上很完整——太完整了。我会再找你。」",
            "reward": "对陆青禾的能力有了深刻认识",
            "social_proof": "在大师兄眼中从「废物」变成「需要被暗杀的对象」",
            "hook": "主角需要尽快变强——他开始寻找其他系统宿主"
        }
    },
    {
        "id": "ch5",
        "title": "第5章：寻找新韭菜",
        "summary": "为了在30天内真正变强，主角开始在宗门里闲逛寻找其他「系统宿主」。成功锁定对着垃圾堆流口水的苏瑶，和在长老闭关室门口鬼鬼祟祟的纪迟。",
        "characters": ["主角", "苏瑶", "纪迟"],
        "location": "玄云宗·外门各处",
        "events": [
            {"actor": "主角", "action": "discover", "target": "苏瑶的机缘系统", "target_type": "premise", "context": "外门杂物区", "result": "主角通过日志确认苏瑶拥有「机缘系统」，即将触发朱果机缘"},
            {"actor": "主角", "action": "discover", "target": "纪迟的签到系统", "target_type": "premise", "context": "长老闭关室外", "result": "主角发现纪迟拥有「签到系统」，下次签到地点：万剑冢"},
            {"actor": "主角", "action": "set", "target": "截胡计划", "target_type": "goal", "context": "主角内心", "result": "主角决定截胡苏瑶的朱果，拉拢纪迟为盟友"}
        ],
        "conflict": "主角锁定两个新系统宿主，但如何利用他们而不暴露自己",
        "pacing": "normal",
        "timeline": {"anchor": "第5天", "time_jump": True, "previous_gap": "次日", "duration": "当天"},
        "state_anchor": {"cultivation": "练气三层", "spirit_stones": 10, "allies": [], "injuries": [], "location": "玄云宗外门", "key_items": [], "notes": "锁定两个系统宿主目标"},
        "scenes": [
            {"order": 1, "pov": "主角", "goal": "在宗门闲逛扫描系统宿主", "location": "外门杂物区", "characters": ["主角", "苏瑶"], "tone": "轻松", "beats": ["主角看到苏瑶正蹲在垃圾堆前流口水，日志弹出：[机缘系统-宿主苏瑶：3小时后，后山枯树洞获得百年朱果]", "主角默默记下坐标和时间"]},
            {"order": 2, "pov": "主角", "goal": "寻找更多系统宿主", "location": "长老闭关室附近", "characters": ["主角", "纪迟"], "tone": "好奇", "beats": ["主角发现纪迟在闭关室门口鬼鬼祟祟", "日志弹出：[签到系统-宿主纪迟：目标签到地点——万剑冢（未到签到窗口期）]"]}
        ],
        "chapter_payoff": {
            "desire": "找到能帮助自己变强的系统宿主",
            "pressure": "30天倒计时已经过了5天，时间紧迫",
            "clever_move": "用日志扫描宗门弟子，15分钟就锁定了两个系统宿主",
            "payoff_moment": "主角看着苏瑶流口水的样子和纪迟鬼祟的背影，嘴角上扬：「两个工具人，一个送机缘，一个送装备。」",
            "reward": "掌握了苏瑶和纪迟的系统信息和近期机缘",
            "social_proof": "苏瑶和纪迟完全不知道有人在暗中观察他们",
            "hook": "3小时后朱果成熟，主角必须抢先一步截胡苏瑶"
        }
    },
    {
        "id": "ch6",
        "title": "第6章：光速截胡朱果",
        "summary": "主角看日志得知苏瑶3小时后在后山获取朱果。主角提前2小时45分钟赶到，连根拔走。苏瑶赶到后面对空空如也的土坑怀疑人生，喃喃自语「难道我被天道抛弃了？」",
        "characters": ["主角", "苏瑶"],
        "location": "玄云宗·后山枯树洞",
        "events": [
            {"actor": "主角", "action": "acquire", "target": "百年朱果", "target_type": "item", "context": "后山枯树洞", "result": "主角提前截胡苏瑶的机缘，获得百年朱果"},
            {"actor": "苏瑶", "action": "move", "target": "枯树洞", "target_type": "location", "context": "后山", "result": "苏瑶按机缘系统提示准时到达，只看到一个空坑"},
            {"actor": "苏瑶", "action": "lose", "target": "机缘", "target_type": "item", "context": "枯树洞", "result": "苏瑶的机缘系统首次「失败」，系统无任何报警"}
        ],
        "conflict": "主角精确截胡，苏瑶遭遇人生第一次机缘失败",
        "pacing": "fast",
        "timeline": {"anchor": "第5天傍晚", "duration": "3小时内"},
        "state_anchor": {"cultivation": "练气三层", "spirit_stones": 10, "allies": [], "injuries": [], "location": "玄云宗后山", "key_items": [], "notes": "正在执行第一次截胡行动"},
        "scenes": [
            {"order": 1, "pov": "主角", "goal": "抢在苏瑶之前拿走朱果", "location": "后山枯树洞", "characters": ["主角"], "tone": "紧张", "beats": ["主角抄近路15分钟赶到，找到一个藏在枯树洞深处的半透明朱红色果实", "连根带土挖走，还细心地用树叶遮盖痕迹"]},
            {"order": 2, "pov": "苏瑶", "goal": "获取机缘系统提示的朱果", "location": "后山枯树洞", "characters": ["苏瑶"], "tone": "喜剧", "beats": ["苏瑶准时赶到，对着枯树洞翻了三遍", "最终跪在空坑前：「难道天道抛弃我了？难道我的系统是假的？」"]}
        ],
        "chapter_payoff": {
            "desire": "截胡苏瑶的第一个机缘，验证日志系统的价值",
            "pressure": "必须在苏瑶到达前完成，时间精确到分钟",
            "clever_move": "利用日志精确知道机缘的时间、地点、获取方式",
            "payoff_moment": "主角躲在远处灌木丛中，看着苏瑶跪在空坑前怀疑人生的表情，努力不笑出声来",
            "reward": "百年朱果（可巩固根基，价值约200灵石）",
            "social_proof": "苏瑶完全不知道有人截胡，开始怀疑自己",
            "hook": "苏瑶的机缘系统会不会因为「失败」而报错？"
        }
    },
    {
        "id": "ch7",
        "title": "第7章：系统的报错与苏瑶的脑补",
        "summary": "苏瑶的机缘系统因频繁被截胡开始报错。苏瑶没有怀疑有人截胡，反而脑补出「太上长老在考验我」，于是更加拼命地去触发奇葩机缘，给主角提供更多日志。",
        "characters": ["主角", "苏瑶", "其他外门弟子"],
        "location": "玄云宗·外门各处/后山多个地点",
        "events": [
            {"actor": "苏瑶", "action": "use", "target": "机缘系统", "target_type": "premise", "context": "外门各处", "result": "苏瑶的机缘系统连续报错：[机缘获取失败][触发异常]"},
            {"actor": "主角", "action": "acquire", "target": "灵草三株", "target_type": "item", "context": "后山溪边", "result": "主角截胡苏瑶的第二个机缘——三株百年灵草"},
            {"actor": "苏瑶", "action": "set", "target": "接受太上长老考验", "target_type": "goal", "context": "苏瑶内心", "result": "苏瑶脑补出完整世界观：太上长老在暗中考验她，越失败说明考验越大"}
        ],
        "conflict": "系统报错让苏瑶陷入自我怀疑与脑补，主角吃得越来越肥",
        "pacing": "normal",
        "timeline": {"anchor": "第6-8天", "time_jump": True, "previous_gap": "次日开始连续数天", "duration": "三天", "transition": "主角在这三天里连续截胡了苏瑶的多个机缘"},
        "state_anchor": {"cultivation": "练气四层（朱果初步吸收）", "spirit_stones": 15, "allies": [], "injuries": [], "location": "玄云宗外门", "key_items": ["百年朱果（已服用）", "灵草三株"], "notes": "通过截胡苏瑶的机缘稳步提升修为"},
        "scenes": [
            {"order": 1, "pov": "苏瑶", "goal": "触发新的机缘来证明系统没坏", "location": "后山溪边", "characters": ["苏瑶"], "tone": "喜剧", "beats": ["苏瑶按系统提示找到溪边灵草，却只看到新挖的土坑", "系统弹出：[机缘获取失败——目标已被提取]"]},
            {"order": 2, "pov": "苏瑶", "goal": "脑补解释一切", "location": "外门宿舍", "characters": ["苏瑶", "其他外门弟子"], "tone": "喜剧", "beats": ["苏瑶对室友说：「宗门里有太上长老在考验我！我的机缘被人抢先是因为长老在教我戒骄戒躁！」", "室友面面相觑，苏瑶的眼神却越来越亮：「我一定要触发更多机缘来通过考验！」"]}
        ],
        "chapter_payoff": {
            "desire": "理解为什么会失败",
            "pressure": "系统连续报错，信心受挫",
            "clever_move": "苏瑶用脑补构建了完美的解释框架（虽然完全错误）",
            "payoff_moment": "主角通过日志看到苏瑶的脑补内容，差点被自己呛到：「太上长老……这也行？行吧，你继续。」",
            "reward": "苏瑶为「通过考验」触发更多机缘，主角获得稳定机缘来源",
            "social_proof": "苏瑶在外门赢得「修炼狂人」的名声",
            "hook": "主角决定去锁定纪迟的签到系统"
        }
    },
    {
        "id": "ch8",
        "title": "第8章：纪迟的签到流与大预言家",
        "summary": "主角锁定纪迟的签到系统，得知下一次签到地点是万剑冢。主角冒充「预言家」接近纪迟：「纪师弟，我昨晚梦到你的机缘在万剑冢！」纪迟半信半疑。",
        "characters": ["主角", "纪迟"],
        "location": "玄云宗·外门/万剑冢入口",
        "events": [
            {"actor": "主角", "action": "discover", "target": "万剑冢签到窗口期", "target_type": "knowledge", "context": "主角住处", "result": "主角通过日志看到纪迟系统的详细信息：签到地点万剑冢，窗口期明天辰时"},
            {"actor": "主角", "action": "meet", "target": "纪迟", "target_type": "character", "context": "外门演武场", "result": "主角故意在纪迟吃饭时坐到对面，抛出「预言」"},
            {"actor": "纪迟", "action": "set", "target": "测试预言", "target_type": "goal", "context": "纪迟内心", "result": "纪迟半信半疑，决定第二天去万剑冢验证主角的预言"}
        ],
        "conflict": "主角需要赢得纪迟的信任，但不能暴露日志系统",
        "pacing": "normal",
        "timeline": {"anchor": "第9天", "time_jump": True, "previous_gap": "第二天", "duration": "当天"},
        "state_anchor": {"cultivation": "练气四层", "spirit_stones": 15, "allies": [], "injuries": [], "location": "玄云宗外门", "key_items": ["灵草三株"], "notes": "已截胡苏瑶，准备拉拢纪迟"},
        "scenes": [
            {"order": 1, "pov": "主角", "goal": "以自然的方式接近纪迟", "location": "外门食堂", "characters": ["主角", "纪迟"], "tone": "轻松", "beats": ["主角端着饭盘坐到纪迟对面：「纪师弟，我看你印堂发亮，最近怕是有机缘」", "纪迟一脸警惕：「你谁啊？」"]},
            {"order": 2, "pov": "纪迟", "goal": "判断主角是真的预言家还是骗子", "location": "外门演武场", "characters": ["主角", "纪迟"], "tone": "好奇", "beats": ["主角：「我昨晚梦到你在万剑冢找到了好东西——而且是明天辰时。去不去，由你。」", "纪迟心中一震：他的签到系统确实提示明天万剑冢！这个人怎么可能知道？"]}
        ],
        "chapter_payoff": {
            "desire": "让纪迟相信自己是预言家",
            "pressure": "纪迟是老实人但不傻，需要给出不可反驳的证据",
            "clever_move": "直接说出只有纪迟自己知道的系统信息（签到地点+时间），让对方无从反驳",
            "payoff_moment": "纪迟心中翻江倒海：「这个人说的两个信息——万剑冢、辰时——跟我的系统提示一字不差！他真的是预言家！」",
            "reward": "纪迟的初步信任",
            "social_proof": "纪迟用看神仙的眼神看着主角",
            "hook": "明天万剑冢之行，主角需要避开陷阱帮纪迟拿到奖励"
        }
    },
    {
        "id": "ch9",
        "title": "第9章：组队万剑冢",
        "summary": "纪迟按捺不住，第二天拉着主角前往万剑冢。主角靠日志预警避开三道陷阱，两人顺利抵达核心区域。纪迟在中心位置完成签到，获得上古剑意传承。",
        "characters": ["主角", "纪迟"],
        "location": "玄云宗·万剑冢",
        "events": [
            {"actor": "纪迟", "action": "use", "target": "签到系统", "target_type": "premise", "context": "万剑冢核心区", "result": "纪迟在核心位置签到成功，获得上古剑意传承"},
            {"actor": "主角", "action": "acquire", "target": "剑形灵草六株", "target_type": "item", "context": "万剑冢路径上", "result": "主角利用日志避开陷阱的同时，顺手采走万剑冢路径上的附带宝物"},
            {"actor": "主角", "action": "use", "target": "日志系统预警", "target_type": "premise", "context": "万剑冢", "result": "主角提前识别三道致命陷阱并带纪迟绕开"}
        ],
        "conflict": "万剑冢的致命陷阱 vs 主角的日志预警",
        "pacing": "fast",
        "timeline": {"anchor": "第10天辰时", "time_jump": True, "previous_gap": "次日清晨", "duration": "当天"},
        "state_anchor": {"cultivation": "练气四层", "spirit_stones": 15, "allies": ["纪迟(初步信任)"], "injuries": [], "location": "万剑冢入口", "key_items": ["灵草三株"], "notes": "即将带纪迟进入万剑冢"},
        "scenes": [
            {"order": 1, "pov": "纪迟", "goal": "验证主角的预言是否准确", "location": "万剑冢入口", "characters": ["主角", "纪迟"], "tone": "紧张", "beats": ["纪迟站在万剑冢入口犹豫：「这里好像是禁地级别的地方……」", "主角：「放心，我梦里把路线都走通了。跟着我，不要乱踩。」"]},
            {"order": 2, "pov": "主角", "goal": "安全抵达核心区并让纪迟签到", "location": "万剑冢深处", "characters": ["主角", "纪迟"], "tone": "燃", "beats": ["主角三次拦住纪迟：「这里有陷阱」「那里有剑意反噬」「这块石板不能踩」", "纪迟在核心石台签到，金光灌顶——上古剑意传承！"]}
        ],
        "chapter_payoff": {
            "desire": "帮纪迟拿到签到奖励，同时自己捞点好处",
            "pressure": "万剑冢有三道致命陷阱，走错一步就死",
            "clever_move": "日志不仅能看到系统信息，还能看到环境中的「剑意浓度分布」，变相等于透视地图",
            "payoff_moment": "纪迟被上古剑意灌顶的瞬间，主角淡定地把旁边石台上六株剑形灵草放进储物袋：「不客气。」",
            "reward": "六株剑形灵草（有价值炼器材料），纪迟的深度信任",
            "social_proof": "纪迟：「大哥！你就是气运之子！以后你去哪我就去哪！」",
            "hook": "苏瑶的机缘被截胡，纪迟签到成功——两个系统宿主的线索都指向主角，大师兄开始察觉到不对劲"
        }
    },
    {
        "id": "ch10",
        "title": "第10章：大师兄的二阶段怀疑",
        "summary": "大师兄在这十几天里疯狂模拟了十几次，但每次他的「变强方案」都会被主角通过苏瑶或纪迟的线索截胡。大师兄开始怀疑自己的模拟器是不是中了病毒。主角的日志系统弹出新提示：检测到内门方向有更多系统宿主信号。",
        "characters": ["主角", "大师兄", "纪迟", "苏瑶"],
        "location": "玄云宗·外门/大师兄密室",
        "events": [
            {"actor": "大师兄", "action": "use", "target": "模拟器系统", "target_type": "premise", "context": "大师兄密室", "result": "大师兄第15次模拟，每次模拟后实际的资源获取都被主角截胡"},
            {"actor": "大师兄", "action": "discover", "target": "系统异常", "target_type": "knowledge", "context": "大师兄内心", "result": "大师兄开始怀疑自己的系统出了问题——要么中毒，要么有更高权限的系统在干扰"},
            {"actor": "主角", "action": "discover", "target": "内门系统宿主信号", "target_type": "knowledge", "context": "主角住处", "result": "日志系统提示：检测到内门方向存在多个系统宿主信号"}
        ],
        "conflict": "大师兄的自我怀疑 vs 主角的持续暗中截胡",
        "pacing": "normal",
        "timeline": {"anchor": "第10-15天", "time_jump": True, "previous_gap": "数日后", "duration": "五天跨度", "transition": "主角在这几天继续截胡苏瑶的机缘和大师兄的药引，同时与纪迟建立了稳固的同盟关系"},
        "state_anchor": {"cultivation": "练气六层", "spirit_stones": 30, "allies": ["纪迟(死忠粉)", "苏瑶(不知情的工具人)"], "injuries": [], "location": "玄云宗外门", "key_items": ["灵草三株", "剑形灵草六株", "多种截胡来的丹药"], "notes": "10天内从练气三层升到六层"},
        "scenes": [
            {"order": 1, "pov": "大师兄", "goal": "第十几次模拟后破口大骂", "location": "大师兄密室", "characters": ["大师兄"], "tone": "愤怒", "beats": ["大师兄双眼通红：「我换个功法你也截？我买个药你也截？你是不是系统？！」", "模拟器弹出提示：【检测到未知外部变量干扰】——大师兄愣住：真有毒？"]},
            {"order": 2, "pov": "主角", "goal": "清点战果，规划下一步", "location": "主角住处", "characters": ["主角", "纪迟"], "tone": "轻松", "beats": ["主角和纪迟盘点这十天的收获：灵石、丹药、灵草堆了小半桌", "日志系统弹出新提示：检测到内门方向有更多系统宿主信号"]}
        ],
        "chapter_payoff": {
            "desire": "持续积累实力，缩小30天大比的差距",
            "pressure": "大师兄虽然被截胡但修为根基远胜主角，时间只剩15天",
            "clever_move": "利用两个系统宿主的线索网络同时截胡三线资源",
            "payoff_moment": "主角看着日志新提示——内门方向有更多系统宿主——眼中闪过一道光：「外门只是新手村。内门，才是真正的狩猎场。」",
            "reward": "灵石积累到30，修为练气六层，建立初步工具人网络",
            "social_proof": "大师兄怀疑系统中毒但查不出原因，苏瑶越挫越勇，纪迟死心塌地",
            "hook": "陆青禾开始串联宗门异动，即将深夜堵门"
        }
    }
]

vol1 = {
    "id": "P1-V1",
    "title": "第一卷：外门修仙，但后台日志漏了",
    "summary": "主角觉醒日志系统，看到大师兄的模拟器预言。大师兄模拟下毒被主角反制，陆青禾查案登场。主角锁定苏瑶和纪迟，截胡朱果、组队万剑冢。大师兄开始怀疑系统中毒。",
    "payoff_contract": {
        "volume_question": "一个练气三层的废柴，凭什么活过大师兄的30天猎杀？",
        "power_promise": "别人拼死拼活，主角看眼日志就知道敌人的计划、他人的机缘。",
        "main_opponent_misread": "大师兄觉得杀个废柴动动手指就行。",
        "big_win": "主角反向投毒挫败暗杀，截胡苏瑶首个朱果，带纪迟拿下万剑冢签到。",
        "visible_reward": "修为升到练气六层，灵石30，六株剑形灵草，朱果，纪迟成为死忠粉。",
        "reputation_shift": "从透明废柴变成「撞大运的神秘弟子」。",
        "next_bigger_game": "大师兄开始疯狂模拟，内门方向检测到更多系统宿主信号。"
    },
    "chapters": vol1_chapters
}

outline["parts"][0]["volumes"].append(vol1)

# Placeholder for remaining 29 volumes (will be filled by AI pipeline)
for part_idx in range(6):
    if part_idx >= len(outline["parts"]):
        outline["parts"].append({
            "id": f"P{part_idx+1}",
            "title": f"第{part_idx+1}部：(待AI生成)",
            "summary": "待通过 compose pipeline 生成",
            "volumes": []
        })
    # Add placeholder volumes for parts that need them
    existing_vols = len(outline["parts"][part_idx]["volumes"])
    for vol_local_idx in range(existing_vols, 5):
        global_vol = part_idx * 5 + vol_local_idx + 1
        outline["parts"][part_idx]["volumes"].append({
            "id": f"P{part_idx+1}-V{vol_local_idx+1}",
            "title": f"第{global_vol}卷：(待AI生成)",
            "summary": "待通过 compose pipeline 生成",
            "chapters": []
        })

with open('d:/Code/nolvegen/story/compose/outline.json', 'w', encoding='utf-8') as f:
    json.dump(outline, f, ensure_ascii=False, indent=2)

print(f"Written outline.json")
print(f"Part 1, Volume 1: {len(vol1_chapters)} detailed chapters based on user draft")
print(f"Remaining 29 volumes: placeholder, to be filled by compose pipeline")
print(f"Total: {sum(1 for p in outline['parts'] for v in p['volumes'])} volumes, {sum(len(v.get('chapters',[])) for p in outline['parts'] for v in p['volumes'])} chapters")
