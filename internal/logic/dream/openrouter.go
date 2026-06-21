package dream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"dm-server/internal/model"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
)

type sOpenRouter struct{}

var shareOpenRouter = &sOpenRouter{}

// sOpenRouterRequest represents the request structure for OpenRouter API
type sOpenRouterRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Stream           bool     `json:"stream"`
	MaxTokens        int      `json:"max_tokens,omitempty"`
	Temperature      float64  `json:"temperature,omitempty"`
	TopP             float64  `json:"top_p,omitempty"`
	FrequencyPenalty float64  `json:"frequency_penalty,omitempty"`
	Stop             []string `json:"stop,omitempty"`
}

// analyzeDreamStream Use OpenRouter to analyze dream
func (s *sOpenRouter) analyzeDreamStream(ctx context.Context, prompt, dreamContent string) (<-chan model.DreamStreamEvent, error) {
	// Get OpenRouter configuration
	openRouterConfig, err := g.Cfg().Get(ctx, "openrouter")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get OpenRouter configuration")
	}

	config := openRouterConfig.MapStrStr()
	baseURL := config["base_url"]
	model := configuredModel(ctx, config["model"], "anthropic/claude-3.6-sonnet")
	apiKey := config["api_key"]
	timeouts := providerTimeouts(config)
	requestCtx, cancel := context.WithTimeout(ctx, timeouts.Generation)
	streamOwnsCancel := false
	defer func() {
		if !streamOwnsCancel {
			cancel()
		}
	}()

	// Set default values
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	// apiUrl
	apiUrl := fmt.Sprintf("%s/chat/completions", baseURL)

	// Print the config
	glog.Infof(ctx, "Base URL: %s", baseURL)
	glog.Infof(ctx, "Model: %s", model)
	glog.Debugf(ctx, "API Key is empty: %t", apiKey == "")

	// Construct full prompt (sent to LLM, keep in Chinese for the model)
	fullPrompt := fmt.Sprintf("%s\n\n用户梦境内容：\n%s", prompt, dreamContent)
	glog.Infof(ctx, "Prompt: %s", previewRunes(fullPrompt, 100))
	glog.Debugf(ctx, "Full request URL: %s", apiUrl)

	// Construct request
	request := sOpenRouterRequest{
		Model: model,
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{
				Role:    "user",
				Content: fullPrompt,
			},
		},
		Stream:           true,
		MaxTokens:        positiveInt(config["max_tokens"], 4096), // limit max output tokens
		Temperature:      0.7,                                     // control randomness
		TopP:             0.9,                                     // control text diversity
		FrequencyPenalty: 1.05,                                    // control repetition
		Stop:             []string{"\n\n\n", "## 结束", "//完成"},     // set stop words
	}

	// Convert request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to serialize request")
	}

	glog.Debugf(ctx, "Request body size: %d bytes", len(requestBody))

	// Use exponential backoff retry for rate limiting
	maxRetries := 3
	retryDelay := time.Second * 2

	for attempt := 0; attempt <= maxRetries; attempt++ {
		glog.Debugf(ctx, "Attempt %d (max retries: %d)", attempt, maxRetries)

		// Use http.NewRequest + WithContext(ctx)
		httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodPost, apiUrl, bytes.NewReader(requestBody))
		if err != nil {
			glog.Debugf(ctx, "Failed to create HTTP request: %v", err)
			return nil, gerror.Wrap(err, "Failed to build HTTP request")
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

		// Record request header information
		glog.Debugf(ctx, "Request Header Content-Type: %s", httpReq.Header.Get("Content-Type"))
		glog.Debugf(ctx, "Request Header Authorization is set: %t", apiKey != "")

		// Use default client or configure Transport to ensure cancellation is supported
		resp, err := streamingHTTPClient(timeouts).Do(httpReq)
		if err != nil {
			// If it's ctx cancellation, err might be context.Canceled
			if errors.Is(err, context.Canceled) {
				glog.Debugf(ctx, "Request was cancelled by context: %v", err)
				return nil, context.Canceled
			}
			glog.Debugf(ctx, "Failed to send request: %v", err)
			return nil, gerror.Wrap(err, "Failed to send request")
		}

		glog.Debugf(ctx, "Received HTTP response, status code: %d", resp.StatusCode)

		// Check for rate limiting
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			glog.Debugf(ctx, "Rate limiting encountered (429 Too Many Requests)")
			if attempt < maxRetries {
				glog.Warningf(ctx, "OpenRouter API rate limiting, retrying in %v", retryDelay)
				select {
				case <-time.After(retryDelay):
					// Exponentially increase delay time
					retryDelay *= 2
					continue
				case <-ctx.Done():
					glog.Debugf(ctx, "Context cancelled during retry")
					return nil, context.Canceled
				}
			} else {
				glog.Debugf(ctx, "Reached maximum retry attempts, returning rate limiting error")
				return nil, gerror.New("OpenRouter API call frequency exceeded, please try again later")
			}
		}

		if resp.StatusCode != http.StatusOK {
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			glog.Debugf(ctx, "Non-200 status code response content: %s", string(b))
			return nil, gerror.Newf("OpenRouter API returned status code %d: %s", resp.StatusCode, string(b))
		}

		streamOwnsCancel = true
		return streamOpenAIResponse(requestCtx, resp, "openrouter", model, timeouts.Idle, cancel), nil
	}

	return nil, gerror.New("OpenRouter API call failed, maximum retry attempts reached")
}
