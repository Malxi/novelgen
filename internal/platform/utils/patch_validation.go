package utils

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// ValidatePatchJSONText rejects patch payloads that contain broken encoding
// markers before they can become project state.
func ValidatePatchJSONText(data []byte) error {
	data = StripUTF8BOM(data)
	if !utf8.Valid(data) {
		return fmt.Errorf("patch contains invalid UTF-8")
	}
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	return ValidateNoSuspiciousPatchText(value)
}

func StripUTF8BOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return data[3:]
	}
	return data
}

// ValidateNoSuspiciousPatchText recursively checks decoded patch structs/maps.
func ValidateNoSuspiciousPatchText(value interface{}) error {
	return validateNoSuspiciousPatchText(value, "$")
}

func validateNoSuspiciousPatchText(value interface{}, path string) error {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		if hasSuspiciousPatchText(typed) {
			return fmt.Errorf("patch contains suspicious/garbled text at %s: %q", path, typed)
		}
	case bool, float32, float64,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return nil
	case []string:
		for i, item := range typed {
			if err := validateNoSuspiciousPatchText(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case []interface{}:
		for i, item := range typed {
			if err := validateNoSuspiciousPatchText(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case map[string]interface{}:
		for key, item := range typed {
			if err := validateNoSuspiciousPatchText(item, path+"."+key); err != nil {
				return err
			}
		}
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		var decoded interface{}
		if err := json.Unmarshal(data, &decoded); err != nil {
			return nil
		}
		return validateNoSuspiciousPatchText(decoded, path)
	}
	return nil
}

func hasSuspiciousPatchText(value string) bool {
	return strings.ContainsRune(value, '\uFFFD') ||
		strings.Contains(value, "????") ||
		LooksLikeChineseMojibake(value)
}

// RepairLikelyMojibakeText repairs the common Windows failure mode where
// UTF-8 Chinese text was decoded as GBK before reaching a CLI flag. It is
// intentionally conservative and should be used for lookup keys, not patch
// payloads; patch payloads are rejected when suspicious.
func RepairLikelyMojibakeText(value string) string {
	if !LooksLikeChineseMojibake(value) {
		return value
	}
	encoded, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(value))
	if err != nil || !utf8.Valid(encoded) {
		return value
	}
	repaired := string(encoded)
	if LooksLikeChineseMojibake(repaired) || countCJKRunes(repaired) == 0 {
		return value
	}
	return repaired
}

func LooksLikeChineseMojibake(value string) bool {
	score := 0
	for _, marker := range []string{
		"\u93cb",       // 鏋
		"\u6945",       // 楅
		"\u5679",       // 噹
		"\u94cf",       // 铏
		"\u68cc",       // 棌
		"\u6ad5",       // 櫕
		"\u951f\u65a4", // 锟斤
		"\u7f01",       // 缁
		"\u5bb8",       // 宸
		"\u7487",       // 璇
		"\u7ed4",       // 绔
		"\u9420",       // 鐠
		"\u943e",       // 鐾
		"\u9366",       // 鍦
		"\u934f",       // 鍏
		"\u93ad",       // 鎭
		"\u93c0",       // 鏀
		"\u93c1",       // 鏁
		"\u7039",       // 瀹
		"\u9420",       // 鐠
		"\u947d",       // 鑽
	} {
		score += strings.Count(value, marker)
		if score >= 2 {
			return true
		}
	}
	for _, r := range value {
		if r == '\uFFFD' || (r >= '\uE000' && r <= '\uF8FF') {
			return true
		}
	}
	return false
}

func countCJKRunes(value string) int {
	count := 0
	for _, r := range value {
		if (r >= '\u4E00' && r <= '\u9FFF') ||
			(r >= '\u3400' && r <= '\u4DBF') ||
			(r >= '\uF900' && r <= '\uFAFF') {
			count++
		}
	}
	return count
}
