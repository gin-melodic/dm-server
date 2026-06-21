package dream

import (
	"context"
	"math/rand"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
)

// analyzeDreamStream Select AI service by config and analyze dream
func (s *sDream) analyzeDreamStream(ctx context.Context, prompt, dreamContent string) (<-chan string, error) {
	// Get AI service config
	aiServices := []string{"ollama"} // Default to ollama
	aiConfig, err := g.Cfg().Get(ctx, "ai_service")
	if err == nil {
		if aiConfig.IsSlice() {
			// If array config, use all services
			services := aiConfig.Strings()
			if len(services) > 0 {
				aiServices = services
			}
		} else if aiConfig.String() != "" {
			// If string config, use the service
			aiServices = []string{aiConfig.String()}
		}
	}

	// Randomly shuffle service list for load balancing
	rand.Shuffle(len(aiServices), func(i, j int) {
		aiServices[i], aiServices[j] = aiServices[j], aiServices[i]
	})

	glog.Infof(ctx, "Available AI services: %v", aiServices)

	// Try each service until success or all fail
	var lastError error
	for _, service := range aiServices {
		glog.Infof(ctx, "Trying AI service: %s", service)

		models := configuredProviderModels(ctx, service)
		resultChan, err := tryAIProvider(ctx, service, models, func(callCtx context.Context) (<-chan string, error) {
			return s.callAIService(callCtx, service, prompt, dreamContent)
		})
		if err == nil {
			// Successfully called the service
			return resultChan, nil
		}

		// Record error and try next service
		glog.Warningf(ctx, "AI service %s failed: %v", service, err)
		lastError = err
	}

	// All services failed
	return nil, gerror.Wrap(lastError, "All AI services are unavailable")
}

type modelOverrideContextKey struct{}

// configuredProviderModels returns the provider's rotation list. An absent list
// preserves the legacy behavior where the adapter reads its single model value.
func configuredProviderModels(ctx context.Context, service string) []string {
	modelsConfig, err := g.Cfg().Get(ctx, service+".models")
	if err != nil || !modelsConfig.IsSlice() {
		return nil
	}

	configured := modelsConfig.Strings()
	models := make([]string, 0, len(configured))
	for _, model := range configured {
		if model = strings.TrimSpace(model); model != "" {
			models = append(models, model)
		}
	}
	return models
}

func tryAIProvider(
	ctx context.Context,
	service string,
	models []string,
	call func(context.Context) (<-chan string, error),
) (<-chan string, error) {
	if len(models) == 0 {
		return call(ctx)
	}

	var lastError error
	for index, model := range models {
		glog.Infof(ctx, "Trying AI service %s with model: %s", service, model)
		callCtx := context.WithValue(ctx, modelOverrideContextKey{}, model)
		resultChan, err := call(callCtx)
		if err == nil {
			return resultChan, nil
		}

		lastError = err
		if !isRateLimitError(err) || index == len(models)-1 {
			return nil, err
		}
		glog.Warningf(ctx, "AI service %s model %s was rate limited; trying next model", service, model)
	}

	return nil, lastError
}

func configuredModel(ctx context.Context, configured, fallback string) string {
	if model, ok := ctx.Value(modelOverrideContextKey{}).(string); ok && model != "" {
		return model
	}
	if configured != "" {
		return configured
	}
	return fallback
}

// Adapters already report rate limits through their ordinary errors (HTTP 429,
// "rate limit", or their existing localized frequency-limit messages).
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "429") ||
		strings.Contains(message, "rate limit") ||
		strings.Contains(message, "frequency exceeded") ||
		strings.Contains(message, "requestlimitexceeded") ||
		strings.Contains(message, "频率超限")
}

// callAIService Call the specified AI service
func (s *sDream) callAIService(ctx context.Context, service, prompt, dreamContent string) (<-chan string, error) {
	switch service {
	case "openrouter":
		return shareOpenRouter.analyzeDreamStream(ctx, prompt, dreamContent)
	case "groq":
		return shareGroq.analyzeDreamStream(ctx, prompt, dreamContent)
	case "qwen":
		return shareQwen.analyzeDreamStream(ctx, prompt, dreamContent)
	case "hunyuan":
		return shareHunyuan.analyzeDreamStream(ctx, prompt, dreamContent)
	case "lmstudio":
		return shareLMStudio.analyzeDreamStream(ctx, prompt, dreamContent)
	default:
		return shareOllama.analyzeDreamStream(ctx, prompt, dreamContent)
	}
}
