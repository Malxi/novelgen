package cmd

import "testing"

func TestValidateSetupAgentApplyOptionRequiresAgentSDK(t *testing.T) {
	if err := validateSetupAgentApplyOption(false, true); err == nil {
		t.Fatalf("expected --agent-apply without --agent-sdk to fail")
	}
	if err := validateSetupAgentApplyOption(true, true); err != nil {
		t.Fatalf("agent apply with agent sdk returned error: %v", err)
	}
	if err := validateSetupAgentApplyOption(false, false); err != nil {
		t.Fatalf("ordinary setup improve returned error: %v", err)
	}
}

func TestSetupRegenCommandExposesAgentSDKFlags(t *testing.T) {
	for _, name := range []string{"prompt", "agent-sdk", "agent-apply"} {
		if setupRegenCmd.Flags().Lookup(name) == nil {
			t.Fatalf("setup regen missing --%s flag", name)
		}
	}
}

func TestHasSetupPatchIgnoresEmptyAndIDOnlyPatch(t *testing.T) {
	if hasSetupPatch(nil) {
		t.Fatalf("nil patch should be empty")
	}
	if hasSetupPatch(map[string]interface{}{"id": "story_setup"}) {
		t.Fatalf("id-only patch should be empty")
	}
	if !hasSetupPatch(map[string]interface{}{"theme": "new theme"}) {
		t.Fatalf("theme patch should be non-empty")
	}
}

func TestApplySetupAgentSDKPatchMergesAndChecks(t *testing.T) {
	setup := validLongFormSetup()
	patch := map[string]interface{}{
		"theme": "Power becomes durable only when it creates visible trust.",
	}

	merged, check, changes, err := applySetupAgentSDKPatch(setup, patch)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Theme != "Power becomes durable only when it creates visible trust." {
		t.Fatalf("theme = %q", merged.Theme)
	}
	if check == nil || check.Target != "setup" || check.Kind != "all" {
		t.Fatalf("unexpected check result: %#v", check)
	}
	if !hasPatchChange(changes, "setup.theme") {
		t.Fatalf("theme change not reported: %#v", changes)
	}
}

func TestApplySetupAgentSDKPatchRejectsSuspiciousText(t *testing.T) {
	setup := validLongFormSetup()
	_, _, _, err := applySetupAgentSDKPatch(setup, map[string]interface{}{
		"theme": "\u93cb\u6945\u5679完成校准",
	})
	if err == nil {
		t.Fatalf("expected suspicious text patch to be rejected")
	}
}
