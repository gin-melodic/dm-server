package auth

import "testing"

func TestUserSettingsValidationHelpers(t *testing.T) {
	if !isValidLanguageTag("zh-CN") || !isValidLanguageTag("en") {
		t.Fatal("expected common language tags to be valid")
	}
	if isValidLanguageTag("../bad") {
		t.Fatal("expected invalid language tag to fail")
	}
	if !isValidReminderTime("08:30") || !isValidReminderTime("23:59") {
		t.Fatal("expected valid reminder times")
	}
	if isValidReminderTime("24:00") || isValidReminderTime("8:30") {
		t.Fatal("expected invalid reminder times to fail")
	}
	if !isAllowedUserSetting("private", allowedPrivacyModes) || isAllowedUserSetting("public", allowedPrivacyModes) {
		t.Fatal("privacy mode allowlist mismatch")
	}
	if !isAllowedUserSetting("local_cache", allowedStorageModes) || isAllowedUserSetting("remote", allowedStorageModes) {
		t.Fatal("storage mode allowlist mismatch")
	}
}

func TestPsycheIntegration(t *testing.T) {
	score, level, _ := psycheIntegration(0)
	if score != 20 || level != "low" {
		t.Fatalf("empty profile = %v/%s", score, level)
	}
	score, level, _ = psycheIntegration(5)
	if level != "moderate" || score <= 50 {
		t.Fatalf("moderate profile = %v/%s", score, level)
	}
	score, level, _ = psycheIntegration(30)
	if level != "high" || score != 96 {
		t.Fatalf("high capped profile = %v/%s", score, level)
	}
}

func TestPsycheArchetypesDeterministic(t *testing.T) {
	dreams := []psycheDreamRow{
		{Emotion: "fear", Tags: "dark,shadow"},
		{Emotion: "peaceful", Tags: "guide,teacher"},
	}
	first, dominant := psycheArchetypes(dreams, 62)
	second, secondDominant := psycheArchetypes(dreams, 62)
	if dominant != secondDominant {
		t.Fatalf("dominant archetype changed: %q vs %q", dominant, secondDominant)
	}
	if len(first) != len(second) || len(first) == 0 {
		t.Fatalf("unexpected archetype lengths: %d/%d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("archetype result not deterministic at %d: %#v vs %#v", i, first[i], second[i])
		}
	}
}
