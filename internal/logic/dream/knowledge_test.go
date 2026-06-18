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
