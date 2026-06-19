package dream

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

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
func (s *sQwen) analyzeDreamStream(ctx context.Context, prompt, dreamContent string) (<-chan string, error) {
	// Get Alibaba Cloud configuration
	qwenConfig, err := g.Cfg().Get(ctx, "qwen")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get Alibaba Cloud configuration")
	}

	baseURL := qwenConfig.MapStrStr()["base_url"]
	model := qwenConfig.MapStrStr()["model"]
	apiKey := qwenConfig.MapStrStr()["api_key"]

	// Set default values
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	if model == "" {
		model = "qwen3.6-flash"
	}

	// apiUrl
	apiUrl := fmt.Sprintf("%s/chat/completions", baseURL)

	// Print the config
	glog.Infof(ctx, "Base URL: %s", baseURL)
	glog.Infof(ctx, "Model: %s", model)
	glog.Debugf(ctx, "API Key is empty: %t", apiKey == "")

	// Construct the complete prompt
	// Take the first 20 characters for printing
	fullPrompt := fmt.Sprintf("%s\n\n用户梦境内容：\n%s", previewRunes(prompt, 20), previewRunes(dreamContent, 20))
	glog.Infof(ctx, "Prompt: %s", previewRunes(fullPrompt, 20))
	glog.Debugf(ctx, "Full request URL: %s", apiUrl)

	// Construct the request
	request := sQwenRequest{
		Model:  model,
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
		EnableThinking: true,
		ThinkingBudget: 4096,
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
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiUrl, bytes.NewReader(requestBody))
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
		resp, err := http.DefaultClient.Do(httpReq)
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

		// Create a channel for returning
		resultChan := make(chan string, 10)

		// Start a goroutine to read the streaming response
		go func() {
			defer close(resultChan)
			defer resp.Body.Close()

			reader := bufio.NewScanner(resp.Body)
			// Adjust buffer to avoid large lines being truncated
			buf := make([]byte, 0, 64*1024)
			reader.Buffer(buf, 1024*1024)

			glog.Debugf(ctx, "Started reading response stream")

			for {
				select {
				case <-ctx.Done():
					// Cancellation triggered: exit directly, HTTP client will break the underlying connection
					glog.Debugf(ctx, "Context cancelled during streaming read")
					return
				default:
					if !reader.Scan() {
						// EOF or error
						if err := reader.Err(); err != nil {
							glog.Debugf(ctx, "Error reading response stream: %v", err)
							resultChan <- "[error]读取响应出错"
						} else {
							glog.Debugf(ctx, "Response stream read complete (EOF)")
						}
						return
					}

					line := reader.Text()
					// Skip empty lines
					if line == "" {
						continue
					}

					// Skip non-data lines (like "event: completion")
					if !bytes.HasPrefix([]byte(line), []byte("data: ")) {
						glog.Debugf(ctx, "Skipping non-data line: %s", line)
						continue
					}

					// Extract JSON from data line
					jsonData := line[6:] // Skip "data: " prefix
					// glog.Debugf(ctx, "Received data line: %s", jsonData)

					// Check for end marker
					if jsonData == "[DONE]" {
						glog.Debugf(ctx, "Received end marker [DONE]")
						return
					}

					var response sQwenResponse
					if err := json.Unmarshal([]byte(jsonData), &response); err != nil {
						glog.Debugf(ctx, "JSON parse failed: %v, data: %s", err, jsonData)
						continue
					}

					// Send response content to the result channel
					if len(response.Choices) > 0 {
						choice := response.Choices[0]
						content := choice.Message.Content
						if content == "" {
							content = choice.Delta.Content
						}
						if content != "" {
							// glog.Debugf(ctx, "Send content to channel: %s", content)
							resultChan <- content
						}

						// Handle finish reason
						if choice.FinishReason != "" && choice.FinishReason != "null" {
							glog.Debugf(ctx, "Received finish reason: %s", choice.FinishReason)

							// Handle different finish reasons
							switch choice.FinishReason {
							case "length":
								// Content length limit
								glog.Debugf(ctx, "Model reached maximum length limit, sending truncation warning")
								resultChan <- "[warning]由于内容长度限制，生成的内容可能不完整。如需完整分析，请尝试提供更短的梦境描述。"
							case "stop":
								// Normal completion
								glog.Debugf(ctx, "Model completed generation normally")
							case "content_filter":
								// Content filtering
								glog.Warningf(ctx, "Generated content contains filtered content")
								resultChan <- "[warning]生成内容包含不合适的内容，已被过滤。"
							default:
								// Other unknown reasons
								glog.Debugf(ctx, "Unknown finish reason: %s", choice.FinishReason)
								resultChan <- fmt.Sprintf("[info]模型完成生成，原因: %s", choice.FinishReason)
							}

							glog.Info(ctx, "Model completed generation")
							return
						}
					}
				}
			}
		}()

		return resultChan, nil
	}

	return nil, gerror.New("Tencent Hunyuan API call failed, maximum retry attempts reached")
}
