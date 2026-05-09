package models

import (
	"encoding/json"
	"os"
	"strings"
)

// StorySetup represents the story configuration
type StorySetup struct {
	ProjectName    string               `json:"project_name" prompt:"灏忚鍚嶇О" desc:"2-6涓瓧锛屾湁鍚稿紩鍔涳紝涓嶈秴杩?0涓瓧绗?`
	Genres         []string             `json:"genres" prompt:"绫诲瀷" desc:"2-4涓叿浣撳皬璇寸被鍨?`
	Premise        string               `json:"premise" prompt:"鏍稿績璁惧畾" desc:"鏍稿績璁惧畾锛?-4鍙ヨ瘽锛屼笉瑕佸垪琛?`
	Theme          string               `json:"theme" prompt:"涓婚" desc:"涓婚锛屾竻鏅扮殑闄堣堪锛屼笉瑕佸崟涓瘝"`
	Rules          []string             `json:"rules" prompt:"瑙勫垯" desc:"灏忚璁惧畾鐨勮鍒?`
	TargetAudience string               `json:"target_audience" prompt:"鐩爣璇昏€? desc:"鍖呭惈骞撮緞娈靛拰璇昏€呯被鍨?`
	Tone           string               `json:"tone" prompt:"鍩鸿皟" desc:"灏忚鍩鸿皟锛屼竴鍙ヨ瘽锛?-4涓舰瀹硅瘝锛岄€楀彿鍒嗛殧"`
	Tense          string               `json:"tense" prompt:"鏃舵€? desc:"杩囧幓鏃舵垨鐜板湪鏃?`
	POVStyle       string               `json:"pov_style" prompt:"瑙嗚" desc:"绗竴浜虹О銆佺涓変汉绉版湁闄愯瑙掓垨绗笁浜虹О鍏ㄧ煡瑙嗚"`
	WritingStyle   WritingStyle         `json:"writing_style,omitempty" prompt:"鍐欎綔椋庢牸" desc:"鍙€夛紝鍐欎綔椋庢牸瑕佹眰涓庡弬鑰冪墖娈碉紱write 闃舵鍙綔涓洪鏍煎弬鐓э紝涓嶄綔涓哄墽鎯呬簨瀹?`
	LongFormPlan   *LongFormPlan        `json:"long_form_plan,omitempty" prompt:"Long Form Plan" desc:"Optional serial-scale capacity plan: target chapters, volume count, repeatable loop, escalation ladder, reader promises, payoff cadence, and late-story mutation."`
	CoreCast       []CoreCastSeed       `json:"core_cast,omitempty" prompt:"Core Cast" desc:"Setup-level core character seeds: role, importance, story function, relationship arc, entry phase, payoff, and storyline references. Craft expands these into full character cards."`
	Storylines     []Storyline          `json:"storylines,omitempty" prompt:"鏁呬簨绾? desc:"鏁呬簨绾?`
	Premises       []Premise            `json:"premises,omitempty" prompt:"璁惧畾浣撶郴" desc:"璁惧畾鍗囩骇浣撶郴锛堝惈闃佃惀瀹氫箟鍜屼富瑙掕兘鍔涗綋绯伙級"`
	WorldTimeline  []WorldTimelineEntry `json:"world_timeline,omitempty" prompt:"涓栫晫鏃堕棿绾? desc:"鍏抽敭鍘嗗彶浜嬩欢鏃堕棿绾?`
	WorldResources []WorldResource      `json:"world_resources,omitempty" prompt:"鏍稿績璧勬簮" desc:"涓栫晫涓殑鏍稿績璧勬簮瀹氫箟"`
}

// WritingStyle captures optional prose-level style instructions for write agents.
type WritingStyle struct {
	Name             string   `json:"name,omitempty" prompt:"鍐欎綔椋庢牸鍚嶇О" desc:"鍙€夛紝椋庢牸鍚嶇О鎴栧弬鐓у璞★紝渚嬪锛氬喎宄诲厠鍒躲€佽交鍠滃墽缃戞枃銆佸弬鑰冩枃鐨勮妭濂?`
	Description      string   `json:"description,omitempty" prompt:"鍐欎綔椋庢牸鎻忚堪" desc:"鍙€夛紝2-6鍙ヨ瘽锛岃鏄庡彊杩板０闊炽€佸彞寮忚妭濂忋€佹弿鍐欏瘑搴︺€佸鐧介鏍?`
	Principles       []string `json:"principles,omitempty" prompt:"鍐欎綔鍘熷垯" desc:"鍙€夛紝3-8鏉″彲鎵ц鍐欎綔鍘熷垯"`
	Avoid            []string `json:"avoid,omitempty" prompt:"閬垮厤浜嬮」" desc:"鍙€夛紝椋庢牸绂佸繉鎴栦笉瑕佹ā浠跨殑鍐呭"`
	ReferenceExcerpt string   `json:"reference_excerpt,omitempty" prompt:"鍙傝€冩枃绔犵墖娈? desc:"鍙€夛紝浣滀负鏂囬鍙傜収鐨勭煭鏂囩墖娈碉紱鍙ā浠块鏍硷紝涓嶇户鎵挎儏鑺傘€佽瀹氭垨浜虹墿"`
}

// IsZero reports whether the style carries any user-visible instruction.
func (s WritingStyle) IsZero() bool {
	if strings.TrimSpace(s.Name) != "" ||
		strings.TrimSpace(s.Description) != "" ||
		strings.TrimSpace(s.ReferenceExcerpt) != "" {
		return false
	}
	return len(nonEmptyStrings(s.Principles)) == 0 && len(nonEmptyStrings(s.Avoid)) == 0
}

// LongFormPlan is a setup-level serial capacity contract. It guides outline
// scale and escalation without becoming a chapter plan.
type LongFormPlan struct {
	TargetChapters   int      `json:"target_chapters,omitempty" prompt:"Target Chapters" desc:"Intended long-form scale, such as 300, 600, or 1000 chapters"`
	TargetVolumes    int      `json:"target_volumes,omitempty" prompt:"Target Volumes" desc:"Intended number of major volumes or books"`
	MainLoop         string   `json:"main_loop,omitempty" prompt:"Main Loop" desc:"Repeatable reader loop, such as challenge, exploit, win, reward, bigger game"`
	EscalationLadder []string `json:"escalation_ladder,omitempty" prompt:"Escalation Ladder" desc:"High-level stages of scope escalation, not chapter numbers"`
	ReaderPromises   []string `json:"reader_promises,omitempty" prompt:"Reader Promises" desc:"Repeatable attractions readers should expect"`
	PayoffCadence    string   `json:"payoff_cadence,omitempty" prompt:"Payoff Cadence" desc:"How often small, medium, and large payoffs should land"`
	VolumePattern    []string `json:"volume_pattern,omitempty" prompt:"Volume Pattern" desc:"Reusable volume blueprint beats such as hook, pressure, exploit, win, reward, next gate"`
	MidpointMutation string   `json:"midpoint_mutation,omitempty" prompt:"Midpoint Mutation" desc:"How the serial changes after the initial loop would otherwise become stale"`
	EndgamePromise   string   `json:"endgame_promise,omitempty" prompt:"Endgame Promise" desc:"The far-horizon payoff or final larger game"`
}

// IsZero reports whether the plan carries user-visible long-form guidance.
func (p *LongFormPlan) IsZero() bool {
	if p == nil {
		return true
	}
	return p.TargetChapters == 0 &&
		p.TargetVolumes == 0 &&
		strings.TrimSpace(p.MainLoop) == "" &&
		len(nonEmptyStrings(p.EscalationLadder)) == 0 &&
		len(nonEmptyStrings(p.ReaderPromises)) == 0 &&
		strings.TrimSpace(p.PayoffCadence) == "" &&
		len(nonEmptyStrings(p.VolumePattern)) == 0 &&
		strings.TrimSpace(p.MidpointMutation) == "" &&
		strings.TrimSpace(p.EndgamePromise) == ""
}

// CompactReference returns a copy with ReferenceExcerpt trimmed to maxRunes.
func (s WritingStyle) CompactReference(maxRunes int) WritingStyle {
	s.Name = strings.TrimSpace(s.Name)
	s.Description = strings.TrimSpace(s.Description)
	s.Principles = nonEmptyStrings(s.Principles)
	s.Avoid = nonEmptyStrings(s.Avoid)
	if maxRunes <= 0 {
		s.ReferenceExcerpt = ""
		return s
	}
	runes := []rune(strings.TrimSpace(s.ReferenceExcerpt))
	if len(runes) > maxRunes {
		s.ReferenceExcerpt = string(runes[:maxRunes]) + "..."
	} else {
		s.ReferenceExcerpt = string(runes)
	}
	return s
}

func nonEmptyStrings(items []string) []string {
	var result []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// CoreCastSeed is a setup-level character promise. It is intentionally lighter
// than a craft character card: setup owns why the character exists, while craft
// owns concrete biography, voice, appearance, stats, and writing details.
type CoreCastSeed struct {
	ID                 string   `json:"id,omitempty" prompt:"ID" desc:"Optional stable seed ID, such as cast_protagonist"`
	Name               string   `json:"name" prompt:"Name" desc:"Character name or stable placeholder"`
	Role               string   `json:"role" prompt:"Role" desc:"protagonist, female_lead, male_support, rival, villain, mentor, ally, antagonist, etc."`
	Importance         int      `json:"importance" prompt:"Importance" desc:"1-10 importance to the story engine"`
	StoryFunction      string   `json:"story_function" prompt:"Story Function" desc:"Why this character must exist in the story machine"`
	RelationshipToLead string   `json:"relationship_to_lead,omitempty" prompt:"Relationship To Lead" desc:"Initial or central relationship to the protagonist"`
	RelationshipArc    string   `json:"relationship_arc,omitempty" prompt:"Relationship Arc" desc:"High-level relationship movement, not chapter details"`
	EntryPhase         string   `json:"entry_phase" prompt:"Entry Phase" desc:"opening, early, mid, late, or series"`
	Payoff             string   `json:"payoff,omitempty" prompt:"Payoff" desc:"Emotional, plot, or information payoff this character promises"`
	StorylineRefs      []string `json:"storyline_refs,omitempty" prompt:"Storyline Refs" desc:"Names of setup.storylines this character supports"`
}

// Storyline represents a story arc or plot line
type Storyline struct {
	Name               string        `json:"name" prompt:"Name" desc:"鏁呬簨绾垮悕绉?`
	Description        string        `json:"description" prompt:"Description" desc:"鏁呬簨绾挎弿杩帮紝2-4鍙ヨ瘽"`
	Type               string        `json:"type" prompt:"Type" desc:"鏁呬簨绾跨被鍨?`                     // main, subplot, character_arc, etc.
	Importance         int           `json:"importance" prompt:"Importance" desc:"鏁呬簨绾块噸瑕佹€э紝1-10"` // 1-10, 10 being most important
	Scope              string        `json:"scope,omitempty" prompt:"Scope" desc:"鍙€夛紝楂樺眰浣滅敤鍩燂細opening銆乿olume銆乥ook銆乻eries锛涗笉瑕佸啓鍏蜂綋绔犺妭"`
	PayoffStyle        string        `json:"payoff_style,omitempty" prompt:"Payoff Style" desc:"鍙€夛紝鍏戠幇鏂瑰紡锛歩mmediate銆乻taged_reveal銆乻low_burn銆乫inal_turn 绛夐珮灞傛柟寮?`
	SetupRole          string        `json:"setup_role,omitempty" prompt:"Setup Role" desc:"鍙€夛紝杩欐潯绾垮湪鏁呬簨鏈哄櫒涓殑楂樺眰浣滅敤锛氱敓瀛橀挬瀛愩€侀暱鏈熸偓蹇点€佸娍鍔涘帇鍔涖€佹垚闀块┍鍔ㄧ瓑"`
	RepeatablePressure string        `json:"repeatable_pressure,omitempty" prompt:"Repeatable Pressure" desc:"How this storyline can repeatedly create pressure across many volumes"`
	PayoffCadence      string        `json:"payoff_cadence,omitempty" prompt:"Payoff Cadence" desc:"How often this storyline should give partial or major payoffs"`
	Mutation           string        `json:"mutation,omitempty" prompt:"Mutation" desc:"How this storyline changes after the initial pattern would become stale"`
	FailureMode        string        `json:"failure_mode,omitempty" prompt:"Failure Mode" desc:"The most likely way this storyline could become boring, repetitive, or inconsistent"`
	Desire             string        `json:"desire,omitempty" prompt:"Desire" desc:"杩欐潯鏁呬簨绾夸腑瑙掕壊鎴栧娍鍔涙渶鎯冲緱鍒颁粈涔?`
	Opposition         string        `json:"opposition,omitempty" prompt:"Opposition" desc:"闃绘杩欐潯鏁呬簨绾挎帹杩涚殑浜恒€佽鍒欍€佸洶澧冩垨浠ｄ环"`
	Stakes             string        `json:"stakes,omitempty" prompt:"Stakes" desc:"濡傛灉澶辫触浼氬け鍘讳粈涔堬紝鎴栨垚鍔熶細鏀瑰彉浠€涔?`
	Turn               string        `json:"turn,omitempty" prompt:"Turn" desc:"杩欐潯绾挎渶鏈夋垙鍓у紶鍔涚殑鍙嶈浆銆佽鍒ゆ垨鍏崇郴鍙樺寲"`
	Payoff             string        `json:"payoff,omitempty" prompt:"Payoff" desc:"杩欐潯绾挎壙璇虹粰璇昏€呯殑鎯呯华鎴栦俊鎭洖鏀?`
	OpenQuestion       string        `json:"open_question,omitempty" prompt:"Open Question" desc:"椹卞姩璇昏€呯户缁湅鐨勬湭瑙ｉ棶棰?`
	PressurePoints     []string      `json:"pressure_points,omitempty" prompt:"Pressure Points" desc:"鍙€夌殑2-4涓帹杩涘帇鍔涚偣锛屼笉蹇呭啓鎴愬浐瀹氶樁娈?`
	AppealEngine       *AppealEngine `json:"appeal_engine,omitempty" prompt:"Appeal Engine" desc:"鍙€夛紝鐖界偣寮曟搸锛氳兘鍔涚埥鐐广€佽〃闈㈤檺鍒躲€佺牬瑙ｆ柟寮忋€佽耽娉曞睍绀恒€佸崌绾ч挬瀛愩€佹晫浜鸿鍒ゃ€佹敹鐩婄被鍨?`
}

// AppealEngine describes how a setting or arc creates satisfying power-fantasy payoffs.
// It frames limits as exploitable surfaces rather than grim mandatory costs.
type AppealEngine struct {
	Appeal          string `json:"appeal,omitempty" prompt:"Appeal" desc:"鏍稿績鑳藉姏鐖界偣鎴栬鑰呮湡寰?`
	SurfaceLimit    string `json:"surface_limit,omitempty" prompt:"Surface Limit" desc:"琛ㄩ潰闄愬埗銆佸喎鍗淬€佺洸鍖恒€佹潯浠舵垨瑙勫垯杈圭晫锛岄伩鍏嶈兘鍔涙棤闄愯啫鑳€"`
	Exploit         string `json:"exploit,omitempty" prompt:"Exploit" desc:"涓昏濡備綍鍒╃敤瑙勫垯銆佹椂鏈恒€佷俊鎭樊鎴栨晫浜哄亣璁炬紓浜牬灞€"`
	SignatureWin    string `json:"signature_win,omitempty" prompt:"Signature Win" desc:"杩欎釜璁惧畾鑳藉埗閫犵殑鍏蜂綋璧㈡硶鐢婚潰"`
	UpgradePath     string `json:"upgrade_path,omitempty" prompt:"Upgrade Path" desc:"鍚庣画濡備綍鍗囩骇鐖界偣涓斾笉鐮村潖瑙勫垯"`
	OpponentMisread string `json:"opponent_misread,omitempty" prompt:"Opponent Misread" desc:"鏁屼汉閫氬父浼氳鍒よ繖涓瀹氱殑鍦版柟"`
	RewardType      string `json:"reward_type,omitempty" prompt:"Reward Type" desc:"璧㈠悗鏀剁泭绫诲瀷锛氳祫婧愩€佸湴浣嶃€佺瀵嗐€佺洘鍙嬨€佸湴鐩樸€佸悕澹般€佽嚜鐢辩瓑"`
}

// Premise represents a story premise/setting element with progression system
type Premise struct {
	Name         string             `json:"name" prompt:"Name" desc:"璁惧畾浣撶郴鍚嶇О"`
	Description  string             `json:"description" prompt:"Description" desc:"璁惧畾浣撶郴鎻忚堪锛?-4鍙ヨ瘽"`
	Category     string             `json:"category" prompt:"Category" desc:"璁惧畾浣撶郴绫诲瀷"`          // 鏈虹敳, 鍩哄洜, 椋炶埞, 榄旀硶, etc.
	Progression  []ProgressionStage `json:"progression" prompt:"Progression" desc:"璁惧畾浣撶郴鍗囩骇浣撶郴"` // 鍗囩骇浣撶郴
	AppealEngine *AppealEngine      `json:"appeal_engine,omitempty" prompt:"Appeal Engine" desc:"鍙€夛紝璇ヨ瀹氫綋绯荤殑鐖界偣寮曟搸"`
}

// ProgressionStage represents a single stage in the progression system
type ProgressionStage struct {
	Level        int    `json:"level" prompt:"Level" desc:"璁惧畾浣撶郴鍗囩骇浣撶郴绛夌骇"`
	Name         string `json:"name" prompt:"Name" desc:"璁惧畾浣撶郴鍗囩骇浣撶郴鍚嶇О"`
	Description  string `json:"description" prompt:"Description" desc:"璁惧畾浣撶郴鍗囩骇浣撶郴鎻忚堪"`
	Requirements string `json:"requirements,omitempty" prompt:"Requirements" desc:"璁惧畾浣撶郴鍗囩骇浣撶郴瑕佹眰"`
}

// WorldTimelineEntry represents a key historical event.
type WorldTimelineEntry struct {
	Year           string `json:"year" prompt:"鏃堕棿" desc:"鏃堕棿鏍囪瘑锛屽锛氬叕鍏?247骞淬€佹槦鍏?9骞?`
	Event          string `json:"event" prompt:"浜嬩欢" desc:"浜嬩欢绠€杩?`
	Impact         string `json:"impact,omitempty" prompt:"褰卞搷" desc:"瀵瑰綋鍓嶄笘鐣岀殑褰卞搷"`
	RelatedMystery string `json:"related_mystery,omitempty" prompt:"鍏宠仈浼忕瑪" desc:"鍏宠仈鐨勮皽棰業D锛屽 myst_timeline_gap"`
}

// WorldResource defines a core resource in the world.
type WorldResource struct {
	Name        string `json:"name" prompt:"鍚嶇О" desc:"璧勬簮鍚嶇О锛屽锛氭皻鏅朵綋銆佸熀鍥犺繘鍖栬嵂鍓?`
	Category    string `json:"category" prompt:"绫诲瀷" desc:"鑳芥簮/娑堣€楀搧/鏉愭枡/璐у竵"`
	Scarcity    string `json:"scarcity" prompt:"绋€鏈夊害" desc:"甯歌/绋€鏈?鐙竴鏃犱簩"`
	Description string `json:"description" prompt:"鎻忚堪" desc:"鍔熻兘涓庢潵婧愮畝杩?`
}

// Save writes the story setup to a file
func (s *StorySetup) Save(path string) error {
	NormalizeStorySetup(s)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStorySetup reads the story setup from a file
func LoadStorySetup(path string) (*StorySetup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var setup StorySetup
	if err := json.Unmarshal(data, &setup); err != nil {
		return nil, err
	}
	return &setup, nil
}
