package dsl

import "testing"

func TestSplitOutlineLocation(t *testing.T) {
	name, description := splitOutlineLocation("地下机甲库：空气里有机油味，灯光断续闪烁")

	if name != "地下机甲库" {
		t.Fatalf("unexpected name: %q", name)
	}
	if description != "空气里有机油味，灯光断续闪烁" {
		t.Fatalf("unexpected description: %q", description)
	}
}
