package envutil

import (
	"testing"
	"time"
)

func TestStringAndDefaultTrim(t *testing.T) {
	t.Setenv("NOVELGEN_TEST_STR", "  value  ")
	if got := String("NOVELGEN_TEST_STR"); got != "value" {
		t.Fatalf("String = %q, want %q", got, "value")
	}

	t.Setenv("NOVELGEN_TEST_STR", "   ")
	if got := StringDefault("NOVELGEN_TEST_STR", "fallback"); got != "fallback" {
		t.Fatalf("StringDefault on blank = %q, want fallback", got)
	}

	t.Setenv("NOVELGEN_TEST_STR", "set")
	if got := StringDefault("NOVELGEN_TEST_STR", "fallback"); got != "set" {
		t.Fatalf("StringDefault = %q, want set", got)
	}
}

func TestBool(t *testing.T) {
	truthy := []string{"1", "true", "TRUE", "yes", "on", " On "}
	for _, value := range truthy {
		t.Setenv("NOVELGEN_TEST_BOOL", value)
		if !Bool("NOVELGEN_TEST_BOOL", false) {
			t.Errorf("Bool(%q) = false, want true", value)
		}
	}
	falsy := []string{"0", "false", "no", "off"}
	for _, value := range falsy {
		t.Setenv("NOVELGEN_TEST_BOOL", value)
		if Bool("NOVELGEN_TEST_BOOL", true) {
			t.Errorf("Bool(%q) = true, want false", value)
		}
	}
	t.Setenv("NOVELGEN_TEST_BOOL", "garbage")
	if Bool("NOVELGEN_TEST_BOOL", true) != true {
		t.Errorf("unrecognized value must return fallback")
	}
	t.Setenv("NOVELGEN_TEST_BOOL", "")
	if Bool("NOVELGEN_TEST_BOOL", true) != true {
		t.Errorf("unset value must return fallback")
	}
}

func TestPositiveInt(t *testing.T) {
	t.Setenv("NOVELGEN_TEST_INT", "1500")
	if got := PositiveInt("NOVELGEN_TEST_INT", 60); got != 1500 {
		t.Fatalf("PositiveInt = %d, want 1500", got)
	}
	// Invalid / non-positive values fall back, mirroring the pre-refactor
	// behavior the agent stages relied on.
	for _, value := range []string{"", "0", "-5", "bogus"} {
		t.Setenv("NOVELGEN_TEST_INT", value)
		if got := PositiveInt("NOVELGEN_TEST_INT", 60); got != 60 {
			t.Errorf("PositiveInt(%q) = %d, want fallback 60", value, got)
		}
	}
}

func TestPositiveIntMilliseconds(t *testing.T) {
	t.Setenv("NOVELGEN_TEST_MS", "2500")
	if got := PositiveIntMilliseconds("NOVELGEN_TEST_MS", time.Second); got != 2500*time.Millisecond {
		t.Fatalf("PositiveIntMilliseconds = %v, want 2.5s", got)
	}
	for _, value := range []string{"", "0", "-1", "nope"} {
		t.Setenv("NOVELGEN_TEST_MS", value)
		if got := PositiveIntMilliseconds("NOVELGEN_TEST_MS", time.Second); got != time.Second {
			t.Errorf("PositiveIntMilliseconds(%q) = %v, want fallback 1s", value, got)
		}
	}
}

func TestTimestampRFC3339(t *testing.T) {
	t.Setenv("NOVELGEN_TEST_TS", "2026-09-26T12:02:32Z")
	got, ok := TimestampRFC3339("NOVELGEN_TEST_TS")
	if !ok {
		t.Fatal("TimestampRFC3339 ok = false, want true")
	}
	if got.UTC().Format(time.RFC3339) != "2026-09-26T12:02:32Z" {
		t.Fatalf("TimestampRFC3339 = %s", got.UTC().Format(time.RFC3339))
	}

	t.Setenv("NOVELGEN_TEST_TS", "")
	if _, ok := TimestampRFC3339("NOVELGEN_TEST_TS"); ok {
		t.Error("empty value should return ok=false")
	}
	t.Setenv("NOVELGEN_TEST_TS", "not-a-time")
	if _, ok := TimestampRFC3339("NOVELGEN_TEST_TS"); ok {
		t.Error("invalid value should return ok=false")
	}
}
