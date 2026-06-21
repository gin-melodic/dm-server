package dream

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"dm-server/internal/model"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

// TestLiveRemoteModelStreams is intentionally opt-in because it consumes real
// provider quota. It validates every configured remote model through the same
// adapters and stream parser used by production.
func TestLiveRemoteModelStreams(t *testing.T) {
	if os.Getenv("DREAM_LIVE_LLM_TEST") != "1" {
		t.Skip("set DREAM_LIVE_LLM_TEST=1 to test configured remote LLMs")
	}

	adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile)
	if !ok {
		t.Fatal("live LLM test requires GoFrame file configuration")
	}

	// Keep the live probe deliberately small. Defaults used in production are
	// unchanged when these test-only overrides are restored.
	overrides := map[string]any{
		"openrouter.max_tokens": 16,
		"groq.max_tokens":       16,
		"hunyuan.max_tokens":    16,
		"qwen.max_tokens":       16,
		"qwen.enable_thinking":  false,
		"qwen.thinking_budget":  16,
	}
	previous := make(map[string]any, len(overrides))
	for key, value := range overrides {
		current, err := g.Cfg().Get(context.Background(), key)
		if err == nil && !current.IsNil() {
			previous[key] = current.Val()
		}
		if err := adapter.Set(key, value); err != nil {
			t.Fatalf("set test override %s: %v", key, err)
		}
	}
	t.Cleanup(func() {
		for key := range overrides {
			_ = adapter.Set(key, previous[key])
		}
	})

	type target struct {
		provider string
		model    string
	}
	var targets []target
	for _, provider := range []string{"openrouter", "qwen", "hunyuan", "groq"} {
		models := configuredProviderModels(context.Background(), provider)
		if len(models) == 0 {
			configured, err := g.Cfg().Get(context.Background(), provider+".model")
			if err != nil || strings.TrimSpace(configured.String()) == "" {
				t.Fatalf("%s has no configured model", provider)
			}
			models = []string{configured.String()}
		}
		for _, modelName := range models {
			targets = append(targets, target{provider: provider, model: modelName})
		}
	}

	for _, current := range targets {
		current := current
		t.Run(current.provider+"/"+current.model, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			ctx = context.WithValue(ctx, modelOverrideContextKey{}, current.model)

			stream, err := (&sDream{}).callAIService(ctx, current.provider, "只回复 OK，不要解释。", "测试")
			if err != nil {
				t.Fatalf("start stream: %v", err)
			}

			var content strings.Builder
			terminalCount := 0
			var terminal model.DreamStreamEvent
			for event := range stream {
				if event.Type == model.DreamStreamEventDelta {
					content.WriteString(event.Content)
				}
				if event.Terminal() {
					terminalCount++
					terminal = event
				}
			}

			if terminalCount != 1 {
				t.Fatalf("expected exactly one terminal event, got %d", terminalCount)
			}
			if terminal.Type != model.DreamStreamEventCompleted {
				t.Fatalf("stream failed: reason=%s message=%s", terminal.FinishReason, terminal.Message)
			}
			if content.Len() == 0 {
				t.Fatal("stream completed without content")
			}
			t.Logf("completed: provider=%s model=%s reason=%s bytes=%d", current.provider, current.model, terminal.FinishReason, content.Len())
		})
	}
}
