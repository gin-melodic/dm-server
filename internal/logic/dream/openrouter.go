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
func (s *sOpenRouter) analyzeDreamStream(ctx context.Context, prompt, dreamContent string) (<-chan string, error) {
	// Get OpenRouter configuration
	openRouterConfig, err := g.Cfg().Get(ctx, "openrouter")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get OpenRouter configuration")
	}

	baseURL := openRouterConfig.MapStrStr()["base_url"]
	model := openRouterConfig.MapStrStr()["model"]
	apiKey := openRouterConfig.MapStrStr()["api_key"]

	// Set default values
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	if model == "" {
		model = "anthropic/claude-3.6-sonnet"
	}

	// apiUrl
	apiUrl := fmt.Sprintf("%s/chat/completions", baseURL)

	// Print the config
	glog.Infof(ctx, "Base URL: %s", baseURL)
	glog.Infof(ctx, "Model: %s", model)
	glog.Debugf(ctx, "API Key is empty: %t", apiKey == "")

	// Construct full prompt (sent to LLM, keep in Chinese for the model)
	fullPrompt := fmt.Sprintf("%s\n\n用户梦境内容：\n%s", prompt, dreamContent)
	glog.Infof(ctx, "Prompt: %s", fullPrompt)
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
		MaxTokens:        4096,                                // limit max output tokens
		Temperature:      0.7,                                 // control randomness
		TopP:             0.9,                                 // control text diversity
		FrequencyPenalty: 1.05,                                // control repetition
		Stop:             []string{"\n\n\n", "## 结束", "//完成"}, // set stop words
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
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiUrl, bytes.NewReader(requestBody))
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
		resp, err := http.DefaultClient.Do(httpReq)
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

		ch := make(chan string)
		go func() {
			defer resp.Body.Close()
			defer close(ch)

			reader := bufio.NewScanner(resp.Body)
			// Adjust buffer to avoid truncating large lines
			buf := make([]byte, 0, 64*1024)
			reader.Buffer(buf, 1024*1024)

			glog.Debugf(ctx, "Start reading response stream")

			isThinking := false
			defer func() {
				if isThinking {
					ch <- "</think>"
				}
			}()

			for {
				select {
				case <-ctx.Done():
					// Trigger cancellation: exit directly, the http client will interrupt the underlying connection
					glog.Debugf(ctx, "Context cancelled during streaming read")
					return
				default:
					if !reader.Scan() {
						// EOF or error
						if err := reader.Err(); err != nil {
							glog.Debugf(ctx, "Error reading response stream: %v", err)
						} else {
							glog.Debugf(ctx, "Response stream reading complete (EOF)")
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

					// Check for end marker
					if jsonData == "[DONE]" {
						glog.Debugf(ctx, "Received stream end marker [DONE]")
						return
					}

					var result struct {
						Choices []struct {
							Delta struct {
								Content          string `json:"content"`
								ReasoningContent string `json:"reasoning_content"`
								Reasoning        string `json:"reasoning"`
							} `json:"delta"`
							FinishReason string `json:"finish_reason"`
						} `json:"choices"`
						Error *struct {
							Message string `json:"message"`
							Code    string `json:"code"`
						} `json:"error,omitempty"`
					}

					if err := json.Unmarshal([]byte(jsonData), &result); err == nil {
						// Check for errors
						if result.Error != nil {
							glog.Debugf(ctx, "API returned error: Code=%s, Message=%s", result.Error.Code, result.Error.Message)
							// If it's a rate limiting error, log it
							if result.Error.Code == "rate_limit_exceeded" {
								glog.Warning(ctx, "OpenRouter returned rate limiting error:", result.Error.Message)
							}
							ch <- "[error]" + result.Error.Message
							return
						}

						if len(result.Choices) > 0 {
							choice := result.Choices[0]

							// Get reasoning content if any
							reasoning := choice.Delta.ReasoningContent
							if reasoning == "" {
								reasoning = choice.Delta.Reasoning
							}

							if reasoning != "" {
								if !isThinking {
									isThinking = true
									ch <- "<think>"
								}
								ch <- reasoning
							}

							content := choice.Delta.Content
							if content != "" {
								if isThinking {
									isThinking = false
									ch <- "</think>"
								}
								ch <- content
							}

							finishReason := choice.FinishReason
							if finishReason != "" && finishReason != "none" && finishReason != "null" {
								if isThinking {
									isThinking = false
									ch <- "</think>"
								}
								glog.Debugf(ctx, "Received finish reason: %s", finishReason)
								if finishReason == "length" {
									glog.Debugf(ctx, "Model reached maximum length limit, sending truncation warning")
									ch <- "[warning]由于内容长度限制，生成的内容可能不完整。如需完整分析，请尝试提供更短的梦境描述。"
								}
								return
							}
						}
					} else {
						glog.Debugf(ctx, "JSON parsing failed: %v, data: %s", err, jsonData)
					}
				}
			}
		}()
		return ch, nil
	}

	return nil, gerror.New("OpenRouter API call failed, maximum retry attempts reached")
}
