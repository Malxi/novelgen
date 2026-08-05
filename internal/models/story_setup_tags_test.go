package models

import (
	"reflect"
	"strings"
	"testing"
)

func TestStorySetupPromptTagsAreReadable(t *testing.T) {
	visited := map[reflect.Type]bool{}
	checkStorySetupPromptTags(t, reflect.TypeOf(StorySetup{}), visited)
}

func checkStorySetupPromptTags(t *testing.T, typ reflect.Type, visited map[reflect.Type]bool) {
	t.Helper()
	for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct || typ.PkgPath() != "novelgen/internal/models" || visited[typ] {
		return
	}
	visited[typ] = true

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" || jsonTag == "" {
			continue
		}
		rawTag := string(field.Tag)
		for _, key := range []string{"prompt", "desc"} {
			if !strings.Contains(rawTag, key+":") {
				continue
			}
			value, ok := field.Tag.Lookup(key)
			if !ok || strings.TrimSpace(value) == "" {
				t.Fatalf("%s.%s has unreadable %s tag: %q", typ.Name(), field.Name, key, rawTag)
			}
			if looksLikeMojibake(value) {
				t.Fatalf("%s.%s has mojibake in %s tag: %q", typ.Name(), field.Name, key, value)
			}
		}
		checkStorySetupPromptTags(t, field.Type, visited)
	}
}

func looksLikeMojibake(value string) bool {
	for _, marker := range []string{
		"鎴", "鏁", "璁", "鍙", "灏", "绫", "鐩", "瑙", "鐨", "鍦", "鍚", "鑳", "娑", "鏃", "閬", "绋",
		"銆", "€", "�",
	} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}
