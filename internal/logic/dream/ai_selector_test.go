package dream

import (
	"context"
	"errors"
	"reflect"
	"testing"

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

func TestTryAIProviderRotatesModelsOnRateLimit(t *testing.T) {
	models := []string{"first", "second"}
	var called []string

	result, err := tryAIProvider(context.Background(), "groq", models, func(ctx context.Context) (<-chan string, error) {
		model := configuredModel(ctx, "legacy", "fallback")
		called = append(called, model)
		if model == "first" {
			return nil, errors.New("Groq API 调用频率超限，请稍后重试")
		}
		ch := make(chan string)
		close(ch)
		return ch, nil
	})

	if err != nil {
		t.Fatalf("tryAIProvider returned error: %v", err)
	}
	if result == nil {
		t.Fatal("tryAIProvider returned a nil channel")
	}
	if !reflect.DeepEqual(called, models) {
		t.Fatalf("models called = %v, want %v", called, models)
	}
}

func TestTryAIProviderDoesNotRotateOnOtherErrors(t *testing.T) {
	var calls int
	wantErr := errors.New("authentication failed")

	_, err := tryAIProvider(context.Background(), "groq", []string{"first", "second"}, func(context.Context) (<-chan string, error) {
		calls++
		return nil, wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestTryAIProviderPreservesSingleModelConfig(t *testing.T) {
	var gotModel string

	_, err := tryAIProvider(context.Background(), "groq", nil, func(ctx context.Context) (<-chan string, error) {
		gotModel = configuredModel(ctx, "legacy-model", "fallback")
		ch := make(chan string)
		close(ch)
		return ch, nil
	})

	if err != nil {
		t.Fatalf("tryAIProvider returned error: %v", err)
	}
	if gotModel != "legacy-model" {
		t.Fatalf("model = %q, want legacy-model", gotModel)
	}
}
