package utils

import "testing"

func TestValidatePatchJSONTextRejectsReplacementGarbledText(t *testing.T) {
	err := ValidatePatchJSONText([]byte(`{"state_anchor":{"cultivation":"` + "\u951f\u65a4\u62f7\u951f\u7aed????" + `"}}`))
	if err == nil {
		t.Fatalf("expected garbled text rejection")
	}
}

func TestValidatePatchJSONTextRejectsChineseMojibakeText(t *testing.T) {
	err := ValidatePatchJSONText([]byte(`{"summary":"` + "\u93cb\u6945\u5679\u5b8c\u6210\u6821\u51c6" + `"}`))
	if err == nil {
		t.Fatalf("expected mojibake text rejection")
	}
}

func TestValidatePatchJSONTextRejectsCommonWindowsMojibake(t *testing.T) {
	err := ValidatePatchJSONText([]byte(`{"summary":"` + "\u5bb8\u67e5\u7379\u7487\u01b3\u588d\u93c8\u5938\u672f" + `"}`))
	if err == nil {
		t.Fatalf("expected common Windows mojibake rejection")
	}
}

func TestValidatePatchJSONTextAcceptsNormalChinesePatch(t *testing.T) {
	err := ValidatePatchJSONText([]byte(`{"summary":"` + "\u6797\u91ce\u5b8c\u6210\u6821\u51c6\uff0c\u706b\u79cd\u6838\u5fc3\u7a33\u5b9a\u4e0b\u6765" + `"}`))
	if err != nil {
		t.Fatalf("normal patch rejected: %v", err)
	}
}

func TestRepairLikelyMojibakeTextRepairsLookupKey(t *testing.T) {
	got := RepairLikelyMojibakeText("\u93cb\u6945\u5679")
	if got != "\u6797\u91ce" {
		t.Fatalf("repaired key = %q, want 林野", got)
	}
}

func TestRepairLikelyMojibakeTextKeepsNormalChinese(t *testing.T) {
	input := "\u94f6\u6cb3\u706b\u79cd"
	if got := RepairLikelyMojibakeText(input); got != input {
		t.Fatalf("normal Chinese changed to %q", got)
	}
}

func TestStripUTF8BOM(t *testing.T) {
	got := StripUTF8BOM([]byte{0xEF, 0xBB, 0xBF, '{', '}'})
	if string(got) != "{}" {
		t.Fatalf("stripped = %q, want {}", string(got))
	}
}
