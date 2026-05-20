package dream

import (
	"context"
	"math/rand"

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

		resultChan, err := s.callAIService(ctx, service, prompt, dreamContent)
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

// callAIService Call the specified AI service
func (s *sDream) callAIService(ctx context.Context, service, prompt, dreamContent string) (<-chan string, error) {
	switch service {
	case "openrouter":
		return shareOpenRouter.analyzeDreamStream(ctx, prompt, dreamContent)
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
