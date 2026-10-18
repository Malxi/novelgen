package quality

import (
	"testing"

	"novelgen/internal/contract/story"
	"novelgen/internal/fiction/domain/creativity"
)

func TestScoreSetupFourAxisRewardsEngineFidelity(t *testing.T) {
	engine := creativity.Engine{
		Genre:              "科幻",
		Artifact:           "虫洞战舰保险单",
		Issuer:             "深空保险公司",
		Holder:             "舰长",
		StatedPurpose:      "承保战舰损失",
		HiddenFunction:     "用理赔数据预测虫洞闭合",
		Trigger:            "全损认定",
		Loophole:           "只有保险人信标确认的残骸才算全损",
		Cost:               "未来十年的劳动",
		ProtagonistRole:    "机甲理赔员",
		CounterExpectation: "主角让公司继续存在，但把理赔权交给所有船员",
		ConflictTypes:      []string{"资源", "制度", "认知", "道德"},
	}
	good := models.StorySetup{
		ProjectName: "第七天理赔",
		Genres:      []string{"科幻", "太空歌剧"},
		Premise:     "在虫洞边缘，每艘战舰都靠一份虫洞战舰保险单确认自己的残骸价值。深空保险公司表面承保战舰损失，实际用理赔数据预测虫洞闭合。主角是机甲理赔员，必须在全损认定前决定替谁签字。",
		Theme:       "制度救人，也吃人。",
		Rules: []string{
			"只有保险人信标确认的残骸才算全损；没有信标，任何人都不能领取赔付。",
			"绕过信标登记需要支付未来十年的劳动，而且必须留下可审计的记录。",
			"保险公司靠虫洞闭合预测获利，所以它不会主动废除理赔制度。",
		},
		Storylines: []models.Storyline{
			{
				Name: "理赔战争", Type: "resource", Importance: 10,
				Description:  "三支舰队争夺同一批残骸的理赔权。",
				Desire:       "拿到冷却剂和救援配额",
				Opposition:   "深空保险公司延迟确认全损",
				Stakes:       "整艘医疗舰失去补给",
				Turn:         "主角把敌舰登记成自己的残骸",
				OpenQuestion: "谁有权确认一艘船已经死了？",
				AppealEngine: &models.AppealEngine{
					Appeal: "用保险规则反杀舰队", SurfaceLimit: "必须先在事故前备案",
					Exploit: "利用信标漏洞", SignatureWin: "让敌人替自己支付理赔金",
					UpgradePath: "从单船理赔到舰队清算", OpponentMisread: "以为主角要逃理赔",
					RewardType: "航道与补给",
				},
			},
			{
				Name: "虫洞预测", Type: "epistemic", Importance: 9,
				Description:  "主角发现理赔数据本身就是虫洞模型。",
				Desire:       "查明公司隐瞒的闭合时间",
				Opposition:   "公司封存全部事故记录",
				Stakes:       "所有船在虫洞闭合时被困",
				Turn:         "沉船是计划内成本",
				OpenQuestion: "如果事故被设计过，理赔还算理赔吗？",
				AppealEngine: &models.AppealEngine{
					Appeal: "用对方的数据揭穿对方", SignatureWin: "在听证会上公开风险模型",
					OpponentMisread: "以为主角看不懂账本", RewardType: "真相与航道",
				},
			},
			{
				Name: "签字权", Type: "institutional", Importance: 9,
				Description:  "理赔员必须在制度与幸存者之间选择。",
				Desire:       "保住妹妹的医疗舰",
				Opposition:   "签字意味着承认其他船已经死亡",
				Stakes:       "一次签字决定数百人的身份",
				Turn:         "主角把自己登记为事故资产",
				OpenQuestion: "谁有资格替别人宣布死亡？",
				AppealEngine: &models.AppealEngine{
					Appeal: "一纸签字改变整支舰队", SignatureWin: "用死亡登记冻结敌方战舰",
					OpponentMisread: "以为主角不敢签字", RewardType: "身份与救援窗口",
				},
			},
		},
		Premises: []models.Premise{
			{
				Name: "理赔等级", Category: "institution",
				Description: "理赔权限从单船到舰队逐级扩大。",
				Progression: []models.ProgressionStage{{Level: 1, Name: "单船理赔", Description: "确认一艘船的全损"}},
				AppealEngine: &models.AppealEngine{
					Appeal: "签字权升级", Exploit: "利用审计漏洞", SignatureWin: "一次签字冻结舰队",
				},
			},
			{
				Name: "虫洞模型", Category: "physics",
				Description: "公司用沉船数据预测虫洞闭合。",
				Progression: []models.ProgressionStage{{Level: 1, Name: "事故采样", Description: "记录沉船坐标"}},
			},
		},
		WorldTimeline: []models.WorldTimelineEntry{
			{Year: "星元 12 年", Event: "虫洞边缘被划为理赔试验区", Impact: "战舰残骸成为可定价资产"},
			{Year: "星元 19 年", Event: "第一批沉船数据校准了虫洞模型", Impact: "保险公司开始故意延迟救援"},
		},
		WorldResources: []models.WorldResource{
			{Name: "冷却剂", Category: "消耗品", Scarcity: "稀有", Description: "虫洞边缘唯一能维持反应堆的物资"},
		},
		CoreCast: []models.CoreCastSeed{
			{Name: "陈默", Role: "protagonist", Importance: 10, StoryFunction: "唯一能签字的机甲理赔员", Payoff: "决定谁被登记为死亡"},
			{Name: "赵鸣", Role: "antagonist", Importance: 9, StoryFunction: "执行理赔条款的保险人代表", Payoff: "证明制度比个人更有效"},
			{Name: "岚", Role: "ally", Importance: 8, StoryFunction: "能修好信标的打捞船长", Payoff: "提供绕过信标的物理手段"},
		},
		LongFormPlan: &models.LongFormPlan{
			TargetChapters: 300, TargetVolumes: 3,
			VolumePattern: []string{"事故", "理赔", "反杀", "更大事故"},
		},
	}
	bad := models.StorySetup{
		ProjectName: "废柴逆袭",
		Genres:      []string{"科幻"},
		Premise:     "一个废柴少年捡到老爷爷戒指，靠神秘血脉逆袭，最后打败虫族母巢。",
		Theme:       "逆袭",
		Rules:       []string{"主角很强", "主角会赢", "主角很强", "主角会赢"},
		Storylines: []models.Storyline{{
			Name: "宗门大比", Type: "main", Description: "主角参加宗门大比，秘境夺宝，然后逆袭。",
		}},
		CoreCast: []models.CoreCastSeed{{Name: "主角", Role: "protagonist"}},
	}

	goodScore := ScoreSetupFourAxis(&good, &engine, nil)
	badScore := ScoreSetupFourAxis(&bad, nil, []string{"废柴", "逆袭", "老爷爷", "宗门大比", "秘境夺宝"})

	if goodScore.Total <= badScore.Total {
		t.Fatalf("good score %.1f should beat bad score %.1f (%#v vs %#v)", goodScore.Total, badScore.Total, goodScore, badScore)
	}
	if goodScore.Creativity <= 60 || goodScore.Plausibility <= 60 || goodScore.Logic <= 60 || goodScore.Fun <= 60 {
		t.Fatalf("good setup should clear all four axes: %#v", goodScore)
	}
	if badScore.Plausibility >= 60 || badScore.Logic >= 60 {
		t.Fatalf("bad setup should be penalized: %#v", badScore)
	}
	if len(goodScore.Evidence) == 0 || len(badScore.Issues) == 0 {
		t.Fatalf("scores should explain themselves: good=%#v bad=%#v", goodScore, badScore)
	}
}

func TestScoreSetupFourAxisHandlesNilAndClamps(t *testing.T) {
	if got := ScoreSetupFourAxis(nil, nil, nil); got.Total != 0 {
		t.Fatalf("nil setup score = %#v", got)
	}
	setup := models.StorySetup{Premise: "一个废柴逆袭的故事"}
	score := ScoreSetupFourAxis(&setup, nil, []string{"废柴", "逆袭"})
	for _, value := range []float64{score.Creativity, score.Plausibility, score.Logic, score.Fun, score.Total} {
		if value < 0 || value > 100 {
			t.Fatalf("score out of range: %#v", score)
		}
	}
}
