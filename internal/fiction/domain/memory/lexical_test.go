package memory

import "testing"

func TestTokenizeSupportsChineseAndLatinTerms(t *testing.T) {
	tokens := Tokenize("Lin推开门，看见蓝色钥匙 Blue-Key")
	for _, want := range []string{"lin", "推开", "开门", "蓝色", "钥匙", "blue", "key"} {
		if !containsString(tokens, want) {
			t.Fatalf("tokens missing %q: %#v", want, tokens)
		}
	}
}

func TestLexicalSearchRanksChineseScene(t *testing.T) {
	index := newLexicalIndex([]SceneRecord{
		{ID: "a", ChapterTitle: "Mine", Text: "林野找到蓝色钥匙，听见门后有人呼吸。"},
		{ID: "b", ChapterTitle: "Market", Text: "林野在集市买了一只红色灯笼。"},
		{ID: "c", ChapterTitle: "Camp", Text: "众人讨论明日的行军路线。"},
	})
	hits := index.Search("蓝色钥匙 门后", 3)
	if len(hits) == 0 || hits[0].ID != "a" {
		t.Fatalf("unexpected lexical hits: %#v", hits)
	}
}
