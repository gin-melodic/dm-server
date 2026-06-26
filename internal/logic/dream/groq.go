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

type sGroq struct{}

var shareGroq = &sGroq{}

// Groq configuration example:
//
// # groq:
// # api_key: "gsk_xxxx"
// # model: "qwen/qwen3-32b" # or "meta-llama/llama-4-scout-17b-16e-instruct"
// # base_url: "https://api.groq.com/openai/v1"

// sGroqRequest represents the request structure for Groq API.
type sGroqRequest struct {
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

// analyzeDreamStream Use Groq to analyze dream.
func (s *sGroq) analyzeDreamStream(ctx context.Context, prompt, dreamContent string) (<-chan model.DreamStreamEvent, error) {
	groqConfig, err := g.Cfg().Get(ctx, "groq")
	if err != nil {
		return nil, gerror.Wrap(err, "获取 Groq 配置失败")
	}

	config := groqConfig.MapStrStr()
	baseURL := config["base_url"]
	modelName := configuredModel(ctx, config["model"], "qwen/qwen3-32b")
	apiKey := config["api_key"]
	timeouts := providerTimeouts(config)
	requestCtx, cancel := context.WithTimeout(ctx, timeouts.Generation)
	streamOwnsCancel := false
	defer func() {
		if !streamOwnsCancel {
			cancel()
		}
	}()

	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	apiURL := fmt.Sprintf("%s/chat/completions", baseURL)

	glog.Infof(ctx, "Base URL: %s", baseURL)
	glog.Infof(ctx, "Model: %s", modelName)
	glog.Debugf(ctx, "API Key is empty: %t", apiKey == "")

	fullPrompt := fmt.Sprintf("%s\n\n用户梦境内容：\n%s", prompt, dreamContent)
	glog.Infof(ctx, "Prompt: %s", previewRunes(fullPrompt, 1000))
	glog.Debugf(ctx, "Full request URL: %s", apiURL)

	request := sGroqRequest{
		Model: modelName,
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
		MaxTokens:        positiveInt(config["max_tokens"], 4096),
		Temperature:      0.7,
		TopP:             0.9,
		FrequencyPenalty: 1.05,
		Stop:             []string{"\n\n\n", "## 结束", "//完成"},
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, gerror.Wrap(err, "序列化 Groq 请求失败")
	}

	glog.Debugf(ctx, "Request body size: %d bytes", len(requestBody))

	maxRetries := 3
	retryDelay := 2 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		glog.Debugf(ctx, "Attempt %d (max retries: %d)", attempt, maxRetries)

		httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodPost, apiURL, bytes.NewReader(requestBody))
		if err != nil {
			glog.Debugf(ctx, "Failed to create HTTP request: %v", err)
			return nil, gerror.Wrap(err, "创建 Groq HTTP 请求失败")
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

		glog.Debugf(ctx, "Request Header Content-Type: %s", httpReq.Header.Get("Content-Type"))
		glog.Debugf(ctx, "Request Header Authorization is set: %t", apiKey != "")

		resp, err := streamingHTTPClient(timeouts).Do(httpReq)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				glog.Debugf(ctx, "Request was cancelled by context: %v", err)
				return nil, context.Canceled
			}
			glog.Debugf(ctx, "Failed to send request: %v", err)
			return nil, gerror.Wrap(err, "发送 Groq 请求失败")
		}

		glog.Debugf(ctx, "Received HTTP response, status code: %d", resp.StatusCode)

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			glog.Debugf(ctx, "Rate limiting encountered (429 Too Many Requests)")
			if attempt < maxRetries {
				glog.Warningf(ctx, "Groq API 触发速率限制，将在 %v 后重试", retryDelay)
				select {
				case <-time.After(retryDelay):
					retryDelay *= 2
					continue
				case <-ctx.Done():
					glog.Debugf(ctx, "Context cancelled during retry")
					return nil, context.Canceled
				}
			}
			glog.Debugf(ctx, "Reached maximum retry attempts, returning rate limiting error")
			return nil, gerror.New("Groq API 调用频率超限，请稍后重试")
		}

		if resp.StatusCode != http.StatusOK {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			glog.Debugf(ctx, "Non-200 status code response content: %s", string(body))
			return nil, gerror.Newf("Groq API 返回状态码 %d：%s", resp.StatusCode, string(body))
		}

		glog.Debugf(ctx, "Groq remaining tokens this minute: %s", resp.Header.Get("x-ratelimit-remaining-tokens"))

		streamOwnsCancel = true
		return streamOpenAIResponse(requestCtx, resp, "groq", modelName, timeouts.Idle, cancel), nil
	}

	return nil, gerror.New("Groq API 调用失败，已达到最大重试次数")
}
