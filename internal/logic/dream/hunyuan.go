package dream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"dm-server/internal/model"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
)

const defaultTokenHubBaseURL = "https://tokenhub.tencentmaas.com/v1"

type sHunyuan struct{}

var shareHunyuan = &sHunyuan{}

type tokenHubChatRequest struct {
	Model       string                `json:"model"`
	Messages    []tokenHubChatMessage `json:"messages"`
	Stream      bool                  `json:"stream"`
	MaxTokens   int                   `json:"max_tokens,omitempty"`
	Temperature float64               `json:"temperature,omitempty"`
	TopP        float64               `json:"top_p,omitempty"`
}

type tokenHubChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// analyzeDreamStream uses Tencent TokenHub's OpenAI-compatible streaming API.
// The legacy secret_key setting is retained as the TokenHub bearer API key.
func (s *sHunyuan) analyzeDreamStream(ctx context.Context, prompt, dreamContent string) (<-chan model.DreamStreamEvent, error) {
	hunyuanConfig, err := g.Cfg().Get(ctx, "hunyuan")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get Tencent TokenHub configuration")
	}

	config := hunyuanConfig.MapStrStr()
	baseURL := strings.TrimRight(strings.TrimSpace(config["base_url"]), "/")
	if baseURL == "" {
		baseURL = defaultTokenHubBaseURL
	}
	modelName := configuredModel(ctx, config["model"], "deepseek-v4-flash")
	apiKey := strings.TrimSpace(config["secret_key"])
	if apiKey == "" {
		apiKey = strings.TrimSpace(config["api_key"])
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("TENCENT_TOKENHUB_API_KEY"))
	}
	if apiKey == "" {
		// Backward-compatible fallback for deployments that injected this variable.
		apiKey = strings.TrimSpace(os.Getenv("TENCENTCLOUD_SECRET_KEY"))
	}
	if apiKey == "" {
		return nil, gerror.New("Tencent TokenHub API key is not configured")
	}

	timeouts := providerTimeouts(config)
	requestCtx, cancel := context.WithTimeout(ctx, timeouts.Generation)
	streamOwnsCancel := false
	defer func() {
		if !streamOwnsCancel {
			cancel()
		}
	}()

	requestBody, err := json.Marshal(tokenHubChatRequest{
		Model: modelName,
		Messages: []tokenHubChatMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: fmt.Sprintf("用户的梦境内容如下：\n%s", dreamContent)},
		},
		Stream:      true,
		MaxTokens:   positiveInt(config["max_tokens"], 4096),
		Temperature: 0.7,
		TopP:        0.9,
	})
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to serialize Tencent TokenHub request")
	}

	apiURL := baseURL + "/chat/completions"
	glog.Infof(ctx, "Tencent TokenHub base URL: %s", baseURL)
	glog.Infof(ctx, "Tencent TokenHub model: %s", modelName)
	glog.Infof(ctx, "Prompt: %s", previewRunes(prompt+"\n\n"+dreamContent, 100))

	maxRetries := 3
	retryDelay := 2 * time.Second
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, apiURL, bytes.NewReader(requestBody))
		if err != nil {
			return nil, gerror.Wrap(err, "Failed to build Tencent TokenHub request")
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")

		resp, err := streamingHTTPClient(timeouts).Do(req)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, requestCtx.Err()
			}
			return nil, gerror.Wrap(err, "Failed to call Tencent TokenHub API")
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries {
			resp.Body.Close()
			glog.Warningf(ctx, "Tencent TokenHub rate limited the request, retrying in %v", retryDelay)
			select {
			case <-time.After(retryDelay):
				retryDelay *= 2
				continue
			case <-requestCtx.Done():
				return nil, requestCtx.Err()
			}
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
			resp.Body.Close()
			return nil, gerror.Newf("Tencent TokenHub API returned status code %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		streamOwnsCancel = true
		return streamOpenAIResponse(requestCtx, resp, "hunyuan", modelName, timeouts.Idle, cancel), nil
	}

	return nil, gerror.New("Tencent TokenHub API call failed, maximum retry attempts reached")
}
