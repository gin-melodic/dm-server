package dream

import (
	"context"
	"math/rand"
	"strings"

	"dm-server/internal/model"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
)

// analyzeDreamStream Select AI service by config and analyze dream
func (s *sDream) analyzeDreamStream(ctx context.Context, prompt, dreamContent string) (<-chan model.DreamStreamEvent, error) {
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

	out := make(chan model.DreamStreamEvent, 16)
	go func() {
		defer close(out)
		lastFailure := model.DreamStreamEvent{Type: model.DreamStreamEventError, Message: "All AI services are unavailable", FinishReason: "all_providers_failed"}
		send := func(event model.DreamStreamEvent) bool {
			select {
			case out <- event:
				return true
			case <-ctx.Done():
				return false
			}
		}

		for _, serviceName := range aiServices {
			models := configuredProviderModels(ctx, serviceName)
			if len(models) == 0 {
				models = []string{""}
			}
			for _, modelName := range models {
				callCtx := ctx
				if modelName != "" {
					callCtx = context.WithValue(ctx, modelOverrideContextKey{}, modelName)
				}
				glog.Infof(ctx, "Trying AI service %s with model: %s", serviceName, configuredModel(callCtx, "configured", "default"))
				stream, err := s.startAIService(callCtx, serviceName, prompt, dreamContent)
				if err != nil {
					lastFailure = model.DreamStreamEvent{Type: model.DreamStreamEventError, Message: err.Error(), FinishReason: "provider_start_error", Provider: serviceName, Model: modelName}
					glog.Warningf(ctx, "AI service %s model %s failed before streaming: %v", serviceName, modelName, err)
					continue
				}

				seenDelta := false
				terminal := false
				pending := make([]model.DreamStreamEvent, 0, 1)
				for event := range stream {
					switch event.Type {
					case model.DreamStreamEventDelta:
						if !seenDelta {
							seenDelta = true
							for _, buffered := range pending {
								if !send(buffered) {
									return
								}
							}
							pending = nil
						}
						if !send(event) {
							return
						}
					case model.DreamStreamEventWarning:
						if seenDelta {
							if !send(event) {
								return
							}
						} else {
							pending = append(pending, event)
						}
					case model.DreamStreamEventCompleted:
						terminal = true
						if !seenDelta {
							lastFailure = model.DreamStreamEvent{Type: model.DreamStreamEventError, Message: "AI stream completed without content", FinishReason: "empty_stream", Provider: event.Provider, Model: event.Model}
							break
						}
						for _, buffered := range pending {
							if !send(buffered) {
								return
							}
						}
						_ = send(event)
						return
					case model.DreamStreamEventError:
						terminal = true
						lastFailure = event
						if seenDelta {
							_ = send(event)
							return
						}
					}
					if terminal {
						break
					}
				}
				if !terminal {
					lastFailure = model.DreamStreamEvent{Type: model.DreamStreamEventError, Message: "AI stream closed without terminal event", FinishReason: "protocol_error", Provider: serviceName, Model: modelName}
					if seenDelta {
						_ = send(lastFailure)
						return
					}
				}
				glog.Warningf(ctx, "AI service %s model %s failed before first delta: %s", serviceName, modelName, lastFailure.Message)
			}
		}
		_ = send(lastFailure)
	}()
	return out, nil
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

func configuredModel(ctx context.Context, configured, fallback string) string {
	if model, ok := ctx.Value(modelOverrideContextKey{}).(string); ok && model != "" {
		return model
	}
	if configured != "" {
		return configured
	}
	return fallback
}

// callAIService Call the specified AI service
func (s *sDream) callAIService(ctx context.Context, service, prompt, dreamContent string) (<-chan model.DreamStreamEvent, error) {
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

func (s *sDream) startAIService(ctx context.Context, service, prompt, dreamContent string) (<-chan model.DreamStreamEvent, error) {
	if s.providerCall != nil {
		return s.providerCall(ctx, service, prompt, dreamContent)
	}
	return s.callAIService(ctx, service, prompt, dreamContent)
}
