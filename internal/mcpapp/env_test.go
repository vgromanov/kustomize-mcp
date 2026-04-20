package mcpapp

import (
	"testing"
)

func TestParseBoolEnv(t *testing.T) {
	key := "KUSTOMIZE_MCP_TEST_BOOL_" + t.Name()
	t.Setenv(key, "")
	if !ParseBoolEnv(key, true) {
		t.Fatal("empty should use default true")
	}
	if ParseBoolEnv(key, false) {
		t.Fatal("empty should use default false")
	}
	for _, v := range []string{"0", "false", "no", "off", "FALSE"} {
		t.Setenv(key, v)
		if ParseBoolEnv(key, true) {
			t.Fatalf("%q should be false", v)
		}
	}
	t.Setenv(key, "1")
	if !ParseBoolEnv(key, false) {
		t.Fatal("1 should be true")
	}
}

func TestParseBoolEnv_whitespaceUsesDefault(t *testing.T) {
	key := "KUSTOMIZE_MCP_TEST_BOOL_WS_" + t.Name()
	t.Setenv(key, "   ")
	if !ParseBoolEnv(key, true) {
		t.Fatal("whitespace-only should trim to empty and use default")
	}
}

func TestParseBoolEnv_unknownStringIsTrue(t *testing.T) {
	key := "KUSTOMIZE_MCP_TEST_BOOL_UNK_" + t.Name()
	t.Setenv(key, "maybe")
	if !ParseBoolEnv(key, false) {
		t.Fatal("non-false tokens should be true")
	}
}

func TestOptionsFromEnv(t *testing.T) {
	t.Setenv("KUSTOMIZE_LOAD_RESTRICTIONS", "false")
	t.Setenv("KUSTOMIZE_ENABLE_HELM", "1")
	o := OptionsFromEnv()
	if o.LoadRestrictions {
		t.Fatal("expected load restrictions off")
	}
	if !o.Helm {
		t.Fatal("expected helm on")
	}
	t.Setenv("KUSTOMIZE_LOAD_RESTRICTIONS", "")
	t.Setenv("KUSTOMIZE_ENABLE_HELM", "")
	o2 := OptionsFromEnv()
	if !o2.LoadRestrictions || o2.Helm {
		t.Fatalf("defaults: %+v", o2)
	}
}
