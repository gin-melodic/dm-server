package dream

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dm-server/internal/consts"
	"dm-server/internal/model"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

func TestStreamDreamIgnoresL1InterpretationForFinalAnswer(t *testing.T) {
	metadata := &consts.DreamStreamMetadata{}
	ctx := context.WithValue(dreamStreamTestContext(), consts.CtxDreamStreamMetadata, metadata)
	var ollamaCalled bool

	knowledgeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/interpret_dream" && r.URL.Path != "/api/v1/search" {
			t.Fatalf("unexpected knowledge path: %s", r.URL.Path)
		}
		if r.URL.Path == "/api/v1/search" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
			return
		}
		var req interpretDreamRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.UserId != "14" || len(req.EmotionTags) != 1 || req.EmotionTags[0] != "fearful" {
			t.Fatalf("unexpected L1 request: %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"interpretation": "L1 assembled interpretation",
				"inference_level": "L1",
				"symbols_detected": ["水", "蛇"],
				"symbols_matched": ["水", "蛇"],
				"frameworks_used": []
			},
			"message": "L1缓存命中，无LLM调用"
		}`))
	}))
	defer knowledgeServer.Close()

	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ollamaCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"LLM output","done":true}` + "\n"))
	}))
	defer ollamaServer.Close()
	configureDreamStreamTest(t, knowledgeServer.URL, ollamaServer.URL)

	ch, err := New().StreamDream(ctx, "梦到水和蛇")
	if err != nil {
		t.Fatalf("StreamDream failed: %v", err)
	}
	if got := collectStream(ch); got != "LLM output" {
		t.Fatalf("unexpected stream output: %q", got)
	}
	if !ollamaCalled {
		t.Fatal("expected L1 hit to still call LLM")
	}
	if metadata.InferenceLevel != "L1" || len(metadata.SymbolsDetected) != 2 || metadata.SymbolsDetected[0] != "水" {
		t.Fatalf("expected L1 metadata symbols, got %+v", metadata)
	}
}

func TestStreamDreamUsesInterpretDreamPassagesWhenL1Misses(t *testing.T) {
	metadata := &consts.DreamStreamMetadata{}
	ctx := context.WithValue(dreamStreamTestContext(), consts.CtxDreamStreamMetadata, metadata)
	var prompt string

	knowledgeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/interpret_dream" {
			t.Fatalf("unexpected knowledge path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"results": {
					"jungian": {
						"passages": [
							{"score": 0.9, "source": "jung", "full_text": "知识片段内容"}
						]
					}
				},
				"total_matches": 1,
				"symbols_detected": ["门"],
				"inference_level": "L2",
				"frameworks_used": ["jungian"]
			},
			"message": "梦境解析完成"
		}`))
	}))
	defer knowledgeServer.Close()

	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sOllamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode ollama request: %v", err)
		}
		prompt = req.Prompt
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"LLM output","done":true}` + "\n"))
	}))
	defer ollamaServer.Close()
	configureDreamStreamTest(t, knowledgeServer.URL, ollamaServer.URL)

	ch, err := New().StreamDream(ctx, "梦到门")
	if err != nil {
		t.Fatalf("StreamDream failed: %v", err)
	}
	if got := collectStream(ch); got != "LLM output" {
		t.Fatalf("unexpected stream output: %q", got)
	}
	if !strings.Contains(prompt, "知识片段内容") {
		t.Fatalf("expected prompt to include interpretation passages, got %q", prompt)
	}
	if !strings.Contains(prompt, "Target output locale: zh-Hans") {
		t.Fatalf("expected prompt to include simplified Chinese locale contract, got %q", prompt)
	}
	if metadata.InferenceLevel != "L2" || len(metadata.SymbolsDetected) != 1 || metadata.SymbolsDetected[0] != "门" {
		t.Fatalf("expected miss metadata symbols, got %+v", metadata)
	}
}

func TestDreamResponseLocaleNormalizationAndFallback(t *testing.T) {
	ctx := context.WithValue(context.Background(), consts.CtxDreamResponseLocale, "zh-TW")
	if got := resolveDreamResponseLocale(ctx, "I walked through a quiet city"); got != "zh-Hant" {
		t.Fatalf("explicit locale = %q", got)
	}
	if got := resolveDreamResponseLocale(context.Background(), "I walked through a quiet city"); got != "en" {
		t.Fatalf("english input fallback = %q", got)
	}
	if got := resolveDreamResponseLocale(context.Background(), ""); got != "en" {
		t.Fatalf("empty input fallback = %q", got)
	}
	if got := normalizeDreamLocale("fr-FR"); got != "" {
		t.Fatalf("unsupported locale = %q", got)
	}
	contract := buildDreamLanguageContract("en", "zh-Hans")
	if !strings.Contains(contract, "The app supports exactly three output languages") || !strings.Contains(contract, "Target output locale: en") {
		t.Fatalf("unexpected language contract: %q", contract)
	}
}

func TestStreamDreamFallsBackToLegacySearchWhenInterpretDreamFails(t *testing.T) {
	ctx := dreamStreamTestContext()
	var prompt string

	knowledgeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/interpret_dream":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
		case "/api/v1/search":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"score":0.8,"source":"legacy","full_text":"legacy search passage"}]`))
		default:
			t.Fatalf("unexpected knowledge path: %s", r.URL.Path)
		}
	}))
	defer knowledgeServer.Close()

	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sOllamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode ollama request: %v", err)
		}
		prompt = req.Prompt
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"fallback output","done":true}` + "\n"))
	}))
	defer ollamaServer.Close()
	configureDreamStreamTest(t, knowledgeServer.URL, ollamaServer.URL)

	ch, err := New().StreamDream(ctx, "梦到桥")
	if err != nil {
		t.Fatalf("StreamDream failed: %v", err)
	}
	if got := collectStream(ch); got != "fallback output" {
		t.Fatalf("unexpected stream output: %q", got)
	}
	if !strings.Contains(prompt, "legacy search passage") {
		t.Fatalf("expected prompt to include legacy search passage, got %q", prompt)
	}
}

func dreamStreamTestContext() context.Context {
	ctx := context.WithValue(context.Background(), consts.CtxUserId, uint64(14))
	ctx = context.WithValue(ctx, consts.CtxOpenId, "openid-14")
	ctx = context.WithValue(ctx, consts.CtxDreamEmotionTags, []string{"fearful"})
	return ctx
}

func configureDreamStreamTest(t *testing.T, knowledgeURL, ollamaURL string) {
	t.Helper()
	adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile)
	if !ok {
		t.Fatal("expected file config adapter")
	}
	adapter.Set("knowledge.base_url", knowledgeURL)
	adapter.Set("knowledge.timeout", "5")
	adapter.Set("ai_service", "ollama")
	adapter.Set("ollama.base_url", ollamaURL)
	adapter.Set("ollama.model", "test-model")
	adapter.Set("prompts.dream_analysis", "测试提示词")
}

func collectStream(ch <-chan model.DreamStreamEvent) string {
	var builder strings.Builder
	for event := range ch {
		if event.Type == model.DreamStreamEventDelta {
			builder.WriteString(event.Content)
		}
	}
	return builder.String()
}
