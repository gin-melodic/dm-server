package history

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestValidateDreamContent(t *testing.T) {
	if err := validateDreamContent(" walking through a bright city "); err != nil {
		t.Fatalf("expected valid content, got %v", err)
	}
	if err := validateDreamContent("   "); err == nil {
		t.Fatal("expected empty content to fail")
	}
	if err := validateDreamContent(strings.Repeat("梦", maxDreamContentRunes+1)); err == nil {
		t.Fatal("expected oversized content to fail")
	}
}

func TestExtractAnalysisTitleAndBody(t *testing.T) {
	title, body := extractAnalysisTitleAndBody("# 「晨光之门」\n\n## 综合小结\n新的开始")
	if title != "晨光之门" {
		t.Fatalf("unexpected title: %q", title)
	}
	if !strings.Contains(body, "新的开始") {
		t.Fatalf("expected body to preserve interpretation, got %q", body)
	}

	longTitle := "# " + strings.Repeat("梦", maxDreamTitleRunes+5)
	title, _ = extractAnalysisTitleAndBody(longTitle)
	if utf8.RuneCountInString(title) != maxDreamTitleRunes {
		t.Fatalf("expected title to be trimmed to %d runes, got %d", maxDreamTitleRunes, utf8.RuneCountInString(title))
	}
}

func TestDeriveDreamKeywords(t *testing.T) {
	keywords := deriveDreamKeywords("飞翔 城市 飞翔", "老师 指引 智慧", "peaceful")
	if len(keywords) == 0 {
		t.Fatal("expected keywords")
	}
	seen := map[string]struct{}{}
	for _, keyword := range keywords {
		if _, ok := seen[keyword]; ok {
			t.Fatalf("duplicate keyword %q in %#v", keyword, keywords)
		}
		seen[keyword] = struct{}{}
	}
	if keywords[len(keywords)-1] != "peaceful" {
		t.Fatalf("expected emotion keyword to be included last, got %#v", keywords)
	}
}

func TestNormalizeLocale(t *testing.T) {
	if got := normalizeLocale(""); got != "zh-CN" {
		t.Fatalf("empty locale fallback = %q", got)
	}
	if got := normalizeLocale("en-US"); got != "en-US" {
		t.Fatalf("valid locale = %q", got)
	}
	if got := normalizeLocale("../bad"); got != "zh-CN" {
		t.Fatalf("invalid locale fallback = %q", got)
	}
}

func TestRecommendationTier(t *testing.T) {
	cases := map[float64]string{
		0.95: "high",
		0.88: "standard",
		0.70: "low",
	}
	for score, want := range cases {
		if got := recommendationTier(score); got != want {
			t.Fatalf("recommendationTier(%v) = %q, want %q", score, got, want)
		}
	}
}
