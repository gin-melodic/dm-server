package history

import (
	"strings"
	"testing"
	"unicode/utf8"

	v1 "dm-server/api/history/v1"
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

	title, body = extractAnalysisTitleAndBody("<think>draft title and reasoning</think>\n# 「深海书卷」\n\n真正解析")
	if title != "深海书卷" || strings.Contains(body, "<think>") || !strings.Contains(body, "真正解析") {
		t.Fatalf("expected think content to be removed, title=%q body=%q", title, body)
	}
}

func TestRelatedDreamScoringAndInsight(t *testing.T) {
	record := v1.DreamRecord{
		Id:             7,
		Content:        "我在海边看到蓝色的光，远处有一本书。",
		Interpretation: "海、光和书反复出现，提示情绪与记忆的整理。",
		Emotion:        "anxious",
		Symbols:        []string{"海", "光", "书"},
	}
	score := scoreRelatedDream("深海图书馆里有发光书卷", "anxious", []string{"海", "光", "书"}, record)
	if score < relatedDreamMinSimilarity {
		t.Fatalf("expected similar dream to pass threshold, got %.2f", score)
	}
	insight := buildRelatedDreamInsight([]v1.RelatedDream{
		{Symbols: []string{"海", "光"}, EmotionTags: []string{"anxious"}},
		{Symbols: []string{"海", "书"}, EmotionTags: []string{"anxious"}},
	}, []string{"海", "光"}, "anxious")
	if !strings.Contains(insight, "海") || !strings.Contains(insight, "焦虑") {
		t.Fatalf("unexpected insight: %q", insight)
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
