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

type sQwen struct{}

var shareQwen = &sQwen{}

// sQwenRequest represents the request structure for Qwen API
type sQwenRequest struct {
	Model    string      `json:"model"`
	Messages []sMessages `json:"messages"`
	Stream   bool        `json:"stream"`
	// Sampling temperature, controls the diversity of model-generated text. [0, 2)
	Temperature float64 `json:"temperature"`
	// Nucleus sampling probability threshold, controls the diversity of model-generated text. (0,1.0]
	TopP float64 `json:"top_p"`
	// Controls the repetition of content when the model generates text.
	// Range: [-2.0, 2.0]. Positive values reduce repetition, negative values increase it.
	// Applicable scenarios:
	// Higher presence_penalty is suitable for scenarios requiring diversity, fun, or creativity, such as creative writing or brainstorming.
	// Lower presence_penalty is suitable for scenarios requiring consistency or technical terms, such as technical documents or other formal documents.
	PresencePenalty float64 `json:"presence_penalty"`
	MaxTokens       int     `json:"max_tokens"`
	EnableThinking  bool    `json:"enable_thinking"`
	ThinkingBudget  int     `json:"thinking_budget"`
}

type sMessages struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// sQwenResponse represents the response structure from Qwen API
type sChoices struct {
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
	Message      sMessages `json:"message"`
	FinishReason string    `json:"finish_reason"`
	Index        int       `json:"index"`
}

type sUsage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}

type sQwenResponse struct {
	Choices []sChoices `json:"choices"`
	Object  string     `json:"object"`
	Usage   sUsage     `json:"usage"`
	Created int64      `json:"created"`
	Model   string     `json:"model"`
	ID      string     `json:"id"`
}

// analyzeDreamStream Use Alibaba Cloud (Qwen) to analyze dreams
func (s *sQwen) analyzeDreamStream(ctx context.Context, prompt, dreamContent string) (<-chan model.DreamStreamEvent, error) {
	// Get Alibaba Cloud configuration
	qwenConfig, err := g.Cfg().Get(ctx, "qwen")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get Alibaba Cloud configuration")
	}

	config := qwenConfig.MapStrStr()
	baseURL := config["base_url"]
	modelName := configuredModel(ctx, config["model"], "qwen3.6-flash")
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
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	// apiUrl
	apiUrl := fmt.Sprintf("%s/chat/completions", baseURL)

	// Print the config
	glog.Infof(ctx, "Base URL: %s", baseURL)
	glog.Infof(ctx, "Model: %s", modelName)
	glog.Debugf(ctx, "API Key is empty: %t", apiKey == "")

	// Construct the complete prompt
	// Take the first 20 characters for printing
	fullPrompt := fmt.Sprintf("%s\n\n用户梦境内容：\n%s", previewRunes(prompt, 20), previewRunes(dreamContent, 20))
	glog.Infof(ctx, "Prompt: %s", previewRunes(fullPrompt, 1000))
	glog.Debugf(ctx, "Full request URL: %s", apiUrl)

	// Construct the request
	request := sQwenRequest{
		Model:  modelName,
		Stream: true,
		Messages: []sMessages{
			{
				Role:    "system",
				Content: prompt,
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("用户梦境内容：\n%s", dreamContent),
			},
		},
		Temperature:    0.95,
		MaxTokens:      positiveInt(config["max_tokens"], 4096),
		EnableThinking: configuredBool(config["enable_thinking"], true),
		ThinkingBudget: positiveInt(config["thinking_budget"], 4096),
	}

	// Convert request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to serialize request")
	}

	glog.Debugf(ctx, "Request body size: %d bytes", len(requestBody))

	// Use exponential backoff retry mechanism to handle rate limiting
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
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		httpReq.Header.Set("X-DashScope-SSE", "enable")

		// Log request headers
		glog.Debugf(ctx, "Request header Content-Type: %s", httpReq.Header.Get("Content-Type"))
		glog.Debugf(ctx, "Request header Authorization set: %t", apiKey != "")

		// Send the HTTP request
		resp, err := streamingHTTPClient(timeouts).Do(httpReq)
		if err != nil {
			// Check if the error is caused by context cancellation
			if errors.Is(err, context.Canceled) {
				glog.Debugf(ctx, "Request canceled by context: %v", err)
				return nil, context.Canceled
			}
			glog.Debugf(ctx, "Failed to send request: %v", err)
			return nil, gerror.Wrap(err, "Failed to send request")
		}

		glog.Debugf(ctx, "HTTP response status code: %d", resp.StatusCode)

		// Check if rate limiting is encountered
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			glog.Debugf(ctx, "Rate limiting (429 Too Many Requests)")
			if attempt < maxRetries {
				glog.Warningf(ctx, "Tencent Hunyuan API rate limiting, retrying in %v", retryDelay)
				select {
				case <-time.After(retryDelay):
					// Exponentially increase delay time
					retryDelay *= 2
					continue
				case <-ctx.Done():
					glog.Debugf(ctx, "Retry context cancelled")
					return nil, context.Canceled
				}
			} else {
				glog.Debugf(ctx, "Reached maximum retry attempts, returning rate limiting error")
				return nil, gerror.New("Tencent Hunyuan API call frequency exceeded, please try again later")
			}
		}

		// Check HTTP status code
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			glog.Errorf(ctx, "API returned error status code %d: %s", resp.StatusCode, string(body))
			return nil, gerror.Newf("API returned error status code %d: %s", resp.StatusCode, string(body))
		}

		streamOwnsCancel = true
		return streamOpenAIResponse(requestCtx, resp, "qwen", modelName, timeouts.Idle, cancel), nil
	}

	return nil, gerror.New("Tencent Hunyuan API call failed, maximum retry attempts reached")
}
