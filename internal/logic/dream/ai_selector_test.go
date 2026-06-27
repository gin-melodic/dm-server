package dream

import (
	"context"
	"reflect"
	"testing"

	"dm-server/internal/model"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

func TestConfiguredProviderModelsReadsModelList(t *testing.T) {
	adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile)
	if !ok {
		t.Fatal("expected file config adapter")
	}
	const provider = "selector_rotation_test"
	if err := adapter.Set(provider+".models", []string{" first ", "", "second"}); err != nil {
		t.Fatalf("set model list: %v", err)
	}
	t.Cleanup(func() {
		_ = adapter.Set(provider, nil)
	})

	want := []string{"first", "second"}
	if got := configuredProviderModels(context.Background(), provider); !reflect.DeepEqual(got, want) {
		t.Fatalf("models = %v, want %v", got, want)
	}
}

func TestNormalizeAIServiceListSplitsCommaSeparatedServices(t *testing.T) {
	got := normalizeAIServiceList([]string{"groq, openrouter", " qwen ", "", "hunyuan,lmstudio"})
	want := []string{"groq", "openrouter", "qwen", "hunyuan", "lmstudio"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("services = %v, want %v", got, want)
	}
}

func TestAnalyzeDreamStreamRetriesBeforeFirstDelta(t *testing.T) {
	configureSelectorModels(t, []string{"first", "second"})
	var called []string
	svc := &sDream{providerCall: func(ctx context.Context, _, _, _ string) (<-chan model.DreamStreamEvent, error) {
		modelName := configuredModel(ctx, "legacy", "fallback")
		called = append(called, modelName)
		if modelName == "first" {
			return eventStream(model.DreamStreamEvent{Type: model.DreamStreamEventError, Message: "upstream failed", FinishReason: "provider_error"}), nil
		}
		return eventStream(
			model.DreamStreamEvent{Type: model.DreamStreamEventDelta, Content: "second result"},
			model.DreamStreamEvent{Type: model.DreamStreamEventCompleted, FinishReason: "stop"},
		), nil
	}}

	stream, err := svc.analyzeDreamStream(context.Background(), "prompt", "dream")
	if err != nil {
		t.Fatalf("analyzeDreamStream: %v", err)
	}
	events := collectEvents(stream)
	if got := eventContent(events); got != "second result" {
		t.Fatalf("content = %q, want second result", got)
	}
	if !reflect.DeepEqual(called, []string{"first", "second"}) {
		t.Fatalf("models called = %v", called)
	}
	assertTerminal(t, events, model.DreamStreamEventCompleted, "stop")
}

func TestAnalyzeDreamStreamDoesNotMixModelsAfterDelta(t *testing.T) {
	configureSelectorModels(t, []string{"first", "second"})
	var called []string
	svc := &sDream{providerCall: func(ctx context.Context, _, _, _ string) (<-chan model.DreamStreamEvent, error) {
		modelName := configuredModel(ctx, "legacy", "fallback")
		called = append(called, modelName)
		return eventStream(
			model.DreamStreamEvent{Type: model.DreamStreamEventDelta, Content: "partial"},
			model.DreamStreamEvent{Type: model.DreamStreamEventError, Message: "connection lost", FinishReason: "read_error"},
		), nil
	}}

	stream, err := svc.analyzeDreamStream(context.Background(), "prompt", "dream")
	if err != nil {
		t.Fatalf("analyzeDreamStream: %v", err)
	}
	events := collectEvents(stream)
	if got := eventContent(events); got != "partial" {
		t.Fatalf("content = %q, want partial", got)
	}
	if !reflect.DeepEqual(called, []string{"first"}) {
		t.Fatalf("models called = %v, want only first", called)
	}
	assertTerminal(t, events, model.DreamStreamEventError, "read_error")
}

func configureSelectorModels(t *testing.T, models []string) {
	t.Helper()
	adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile)
	if !ok {
		t.Fatal("expected file config adapter")
	}
	_ = adapter.Set("ai_service", "groq")
	_ = adapter.Set("groq.models", models)
	t.Cleanup(func() {
		_ = adapter.Set("groq.models", nil)
	})
}

func eventStream(events ...model.DreamStreamEvent) <-chan model.DreamStreamEvent {
	ch := make(chan model.DreamStreamEvent, len(events))
	for _, event := range events {
		ch <- event
	}
	close(ch)
	return ch
}
