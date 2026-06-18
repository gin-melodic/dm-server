package dream

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

func TestKnowledgeInterpretDreamL1Hit(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected method POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/interpret_dream" {
			t.Fatalf("expected path /api/v1/interpret_dream, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("expected json content type, got %s", r.Header.Get("Content-Type"))
		}

		var req interpretDreamRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Description != "梦到水和蛇" || req.UserId != "14" || !req.UseWeightedSearch {
			t.Fatalf("unexpected request: %+v", req)
		}
		if len(req.EmotionTags) != 1 || req.EmotionTags[0] != "fearful" {
			t.Fatalf("unexpected emotion tags: %+v", req.EmotionTags)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {
				"interpretation": "这个梦的核心意象“水”通常指向情绪流动。",
				"inference_level": "L1",
				"symbols_detected": ["水", "蛇"],
				"symbols_matched": ["水", "蛇"],
				"frameworks_used": []
			},
			"message": "L1缓存命中，无LLM调用"
		}`))
	}))
	defer server.Close()

	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		adapter.Set("knowledge.base_url", server.URL)
		adapter.Set("knowledge.timeout", "5")
	}

	resp, err := shareKnowledge.interpretDream(ctx, "梦到水和蛇", "14", []string{"fearful"})
	if err != nil {
		t.Fatalf("interpretDream failed: %v", err)
	}
	if !resp.isL1Hit() {
		t.Fatalf("expected L1 hit, got %+v", resp)
	}
}

func TestInterpretDreamResponseToSearchResponse(t *testing.T) {
	resp := &interpretDreamResponse{
		Success: true,
		Data: interpretDreamResponseData{
			TotalMatches: 1,
			Results: map[string]frameworkData{
				"freud": {
					Passages: []searchResponseResult{
						{Score: 0.8, Source: "freud", FullText: "片段内容"},
					},
				},
			},
		},
	}

	results := resp.toSearchResponse()
	if results == nil || len(*results) != 1 {
		t.Fatalf("expected one search result, got %+v", results)
	}
	if (*results)[0].FullText != "片段内容" {
		t.Fatalf("unexpected search result: %+v", (*results)[0])
	}
}

func TestKnowledgeExtractSymbols(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/extract_symbols" {
			t.Fatalf("expected path /api/v1/extract_symbols, got %s", r.URL.Path)
		}
		var req extractSymbolsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.Description != "我梦到乌龟吃草" || len(req.EmotionTags) != 1 || req.EmotionTags[0] != "peaceful" {
			t.Fatalf("unexpected request: %+v", req)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {"symbols_detected": ["乌龟", "草", "乌龟"]},
			"message": "符号提取完成"
		}`))
	}))
	defer server.Close()
	configureKnowledgeTest(t, server.URL)

	symbols, err := shareKnowledge.extractSymbols(ctx, "我梦到乌龟吃草", []string{"peaceful"})
	if err != nil {
		t.Fatalf("extractSymbols failed: %v", err)
	}
	if len(symbols) != 2 || symbols[0] != "乌龟" || symbols[1] != "草" {
		t.Fatalf("unexpected symbols: %+v", symbols)
	}
}

func TestKnowledgeSinkSymbolCache(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/symbol_cache/sink" {
			t.Fatalf("expected path /api/v1/symbol_cache/sink, got %s", r.URL.Path)
		}
		var req sinkSymbolCacheRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.UserId != "14" || req.SourceDreamId != "99" || req.Interpretation != "乌龟意味着缓慢而稳定。" {
			t.Fatalf("unexpected request: %+v", req)
		}
		if len(req.Symbols) != 2 || req.Symbols[0] != "乌龟" || req.Symbols[1] != "草" {
			t.Fatalf("unexpected symbols: %+v", req.Symbols)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": {"stored": 1, "skipped": 1},
			"message": "L1符号缓存回写完成"
		}`))
	}))
	defer server.Close()
	configureKnowledgeTest(t, server.URL)

	if err := shareKnowledge.sinkSymbolCache(ctx, "14", []string{"乌龟", "草", "乌龟"}, "乌龟意味着缓慢而稳定。", "99"); err != nil {
		t.Fatalf("sinkSymbolCache failed: %v", err)
	}
}

func configureKnowledgeTest(t *testing.T, baseURL string) {
	t.Helper()
	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		adapter.Set("knowledge.base_url", baseURL)
		adapter.Set("knowledge.timeout", "5")
	}
}
