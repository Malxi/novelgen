package recap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"novelgen/internal/models"
)

// Store persists per-chapter recaps for continuity
type Store struct {
	projectRoot string
}

func NewStore(projectRoot string) *Store {
	return &Store{projectRoot: projectRoot}
}

func (s *Store) recapPath(chapterID string) string {
	return filepath.Join(s.projectRoot, "story", "recaps", chapterID+".json")
}

func (s *Store) Save(recap *models.ChapterRecap) error {
	if s.projectRoot == "" || recap == nil {
		return nil
	}
	dir := filepath.Join(s.projectRoot, "story", "recaps")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(recap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.recapPath(recap.ChapterID), b, 0644)
}

func (s *Store) Load(chapterID string) (*models.ChapterRecap, error) {
	if s.projectRoot == "" {
		return nil, os.ErrNotExist
	}
	b, err := os.ReadFile(s.recapPath(chapterID))
	if err != nil {
		return nil, err
	}
	var recap models.ChapterRecap
	if err := json.Unmarshal(b, &recap); err != nil {
		return nil, err
	}
	return &recap, nil
}

// ValidateMinimal checks whether a recap is minimally useful as a continuity anchor.
func ValidateMinimal(r *models.ChapterRecap) (ok bool, reasons []string) {
	if r == nil {
		return false, []string{"recap 为空"}
	}

	if strings.TrimSpace(r.ChapterID) == "" {
		reasons = append(reasons, "chapter_id 为空")
	}
	if strings.TrimSpace(r.Title) == "" {
		reasons = append(reasons, "title 为空")
	}

	// Scene anchor essentials
	if strings.TrimSpace(r.Location) == "" {
		reasons = append(reasons, "location 为空（缺少场景锚点）")
	}
	if len(r.Present) == 0 {
		reasons = append(reasons, "present 为空（缺少在场角色）")
	}
	if strings.TrimSpace(r.LastLine) == "" {
		reasons = append(reasons, "last_line 为空（缺少上一章最后一幕/最后一句）")
	}
	if strings.TrimSpace(r.NextOpeningHint) == "" {
		reasons = append(reasons, "next_opening_hint 为空（缺少下一章开头承接）")
	}

	return len(reasons) == 0, reasons
}

// ValidateConsistency performs a lightweight consistency check.
func ValidateConsistency(r *models.ChapterRecap) (ok bool, reasons []string) {
	if r == nil {
		return false, []string{"recap 为空"}
	}
	ll := strings.TrimSpace(r.LastLine)
	hint := strings.TrimSpace(r.NextOpeningHint)
	if ll == "" || hint == "" {
		return false, []string{"last_line 或 next_opening_hint 为空"}
	}

	key := firstMeaningfulToken(ll)
	if key != "" && !strings.Contains(hint, key) {
		reasons = append(reasons, "next_opening_hint 与 last_line 缺少明显承接词（可能跑题）")
	}

	return len(reasons) == 0, reasons
}

// CheckQuality is a deterministic quality gate for recap availability.
func CheckQuality(projectRoot string, chapterID string) (ok bool, issues []string, suggestions []string) {
	chapterID = strings.TrimSpace(chapterID)
	if projectRoot == "" || chapterID == "" {
		return false, []string{"projectRoot/chapterID 为空"}, []string{"确保运行环境能定位项目根目录并生成 story/recaps"}
	}

	p := filepath.Join(projectRoot, "story", "recaps", chapterID+".json")
	b, err := os.ReadFile(p)
	if err != nil {
		return false, []string{fmt.Sprintf("未找到 recap 文件: %s", p)}, []string{"先生成章节或运行 recap 抽取；确保 recap 保存成功"}
	}
	var r models.ChapterRecap
	if err := json.Unmarshal(b, &r); err != nil {
		return false, []string{"recap JSON 解析失败"}, []string{"检查 recap 抽取提示与输出是否为有效 JSON"}
	}
	if ok, reasons := ValidateMinimal(&r); !ok {
		return false, append([]string{"recap 未通过最小门禁"}, reasons...), []string{"重新抽取 recap，补齐 location/present/last_line/next_opening_hint"}
	}
	return true, nil, nil
}

func firstMeaningfulToken(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	r := []rune(s)
	buf := make([]rune, 0, 6)
	for _, c := range r {
		if len(buf) >= 6 {
			break
		}
		switch c {
		case '，', '。', '！', '？', '“', '”', '（', '）', '：', ':', '—', '-', ' ', '\n', '\t':
			continue
		default:
			buf = append(buf, c)
		}
	}
	return string(buf)
}
