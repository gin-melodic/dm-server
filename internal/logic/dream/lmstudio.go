package dream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"dm-server/internal/model"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
)

type sLMStudio struct{}

var shareLMStudio = &sLMStudio{}

// sLMStudioRequest represents the request structure for LMStudio API (OpenAI compatible)
type sLMStudioRequest struct {
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

// analyzeDreamStream Use LMStudio to analyze dream
func (s *sLMStudio) analyzeDreamStream(ctx context.Context, prompt, dreamContent string) (<-chan model.DreamStreamEvent, error) {
	// Get LMStudio configuration
	var baseURL string
	var model string
	lmStudioConfig, err := g.Cfg().Get(ctx, "lmstudio")
	config := map[string]string{}
	if err == nil && !lmStudioConfig.IsEmpty() {
		config = lmStudioConfig.MapStrStr()
		baseURL = config["base_url"]
		model = config["model"]
	}
	model = configuredModel(ctx, model, "qwen3.6-27b-ud")
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
		baseURL = "http://127.0.0.1:1234/v1"
	}
	// apiUrl
	apiUrl := fmt.Sprintf("%s/chat/completions", baseURL)

	// Print the config
	glog.Infof(ctx, "LMStudio Base URL: %s", baseURL)
	glog.Infof(ctx, "LMStudio Model: %s", model)
	glog.Infof(ctx, "LMStudio Timeout: %s", timeouts.Generation)

	// Construct full prompt (sent to LLM, keep in Chinese for the model)
	fullPrompt := fmt.Sprintf("%s\n\n用户梦境内容：\n%s", prompt, dreamContent)
	glog.Debugf(ctx, "LMStudio Full request URL: %s", apiUrl)

	// Construct request
	request := sLMStudioRequest{
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
		MaxTokens:        131072,                              // limit max output tokens
		Temperature:      0.7,                                 // control randomness
		TopP:             0.9,                                 // control text diversity
		FrequencyPenalty: 1.05,                                // control repetition
		Stop:             []string{"\n\n\n", "## 结束", "//完成"}, // set stop words
	}

	// Convert request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to serialize LMStudio request")
	}

	// Set up the client with timeout
	client := streamingHTTPClient(timeouts)

	// Use http.NewRequest + WithContext(ctx)
	httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodPost, apiUrl, bytes.NewReader(requestBody))
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to build LMStudio HTTP request")
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send HTTP request
	resp, err := client.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, context.Canceled
		}
		return nil, gerror.Wrap(err, "Failed to send request to LMStudio")
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, gerror.Newf("LMStudio API returned status code %d: %s", resp.StatusCode, string(b))
	}

	streamOwnsCancel = true
	return streamOpenAIResponse(requestCtx, resp, "lmstudio", model, timeouts.Idle, cancel), nil
}
