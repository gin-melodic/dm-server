package auth

import (
	"reflect"
	"testing"
)

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
	score, level, key, signals := psycheIntegration(nil)
	if score != 20 || level != "low" {
		t.Fatalf("empty profile = %v/%s", score, level)
	}
	if key != "profileIntegrationInsightLow" || len(signals) == 0 {
		t.Fatalf("empty profile missing insight metadata: %q/%#v", key, signals)
	}
	score, level, _, _ = psycheIntegration([]psycheDreamRow{
		{ConfidenceScore: 0.86},
		{ConfidenceScore: 0.86},
	})
	if level != "low" || score <= 35 || score > 55 {
		t.Fatalf("low profile = %v/%s", score, level)
	}
	score, level, _, _ = psycheIntegration([]psycheDreamRow{
		{ConfidenceScore: 0.86, Symbols: "门,光"},
		{ConfidenceScore: 0.86, Symbols: "门"},
		{ConfidenceScore: 0.86, Tags: "guide"},
		{ConfidenceScore: 0.86},
		{ConfidenceScore: 0.86},
	})
	if level != "moderate" || score <= 50 {
		t.Fatalf("moderate profile = %v/%s", score, level)
	}
	highDreams := make([]psycheDreamRow, 30)
	for i := range highDreams {
		highDreams[i] = psycheDreamRow{ConfidenceScore: 0.95, Symbols: "门"}
	}
	score, level, _, _ = psycheIntegration(highDreams)
	if level != "high" || score != 96 {
		t.Fatalf("high capped profile = %v/%s", score, level)
	}
}

func TestPsycheArchetypesDeterministic(t *testing.T) {
	dreams := []psycheDreamRow{
		{Emotion: "fear", Tags: "dark,shadow", Symbols: "阴影"},
		{Emotion: "peaceful", Tags: "guide,teacher", Symbols: "智慧"},
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
		if !reflect.DeepEqual(first[i], second[i]) {
			t.Fatalf("archetype result not deterministic at %d: %#v vs %#v", i, first[i], second[i])
		}
	}
	if first[0].Type != dominant || first[0].InsightKey == "" || len(first[0].Signals) == 0 {
		t.Fatalf("dominant archetype missing insight metadata: %q %#v", dominant, first[0])
	}
}

func TestPsycheArchetypesUseSymbolsBeforeTags(t *testing.T) {
	dreams := []psycheDreamRow{
		{Emotion: "neutral", Tags: "work", Symbols: "智慧,老师"},
		{Emotion: "neutral", Tags: "work", Symbols: "guide"},
	}
	items, dominant := psycheArchetypes(dreams, 50)
	if dominant != "sage" {
		t.Fatalf("expected sage to dominate from symbols, got %q/%#v", dominant, items)
	}
	if len(items) != 5 {
		t.Fatalf("expected 5 archetypes, got %d", len(items))
	}
}
