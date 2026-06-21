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
	"github.com/gogf/gf/v2/os/gtime"
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
func (s *sLMStudio) analyzeDreamStream(ctx context.Context, prompt, dreamContent string) (<-chan string, error) {
	// Get LMStudio configuration
	var baseURL string
	var model string
	var timeoutVal int64 = 300 // default 300 seconds

	lmStudioConfig, err := g.Cfg().Get(ctx, "lmstudio")
	if err == nil && !lmStudioConfig.IsEmpty() {
		cfgMap := lmStudioConfig.MapStrStr()
		baseURL = cfgMap["base_url"]
		model = cfgMap["model"]
		if timeoutStr, ok := cfgMap["timeout"]; ok && timeoutStr != "" {
			if parsedTimeout, parseErr := gtime.ParseDuration(timeoutStr + "s"); parseErr == nil {
				timeoutVal = int64(parsedTimeout.Seconds())
			}
		}
	}
	model = configuredModel(ctx, model, "qwen3.6-27b-ud")

	// Set default values
	if baseURL == "" {
		baseURL = "http://127.0.0.1:1234/v1"
	}
	// apiUrl
	apiUrl := fmt.Sprintf("%s/chat/completions", baseURL)

	// Print the config
	glog.Infof(ctx, "LMStudio Base URL: %s", baseURL)
	glog.Infof(ctx, "LMStudio Model: %s", model)
	glog.Infof(ctx, "LMStudio Timeout: %d seconds", timeoutVal)

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
	client := &http.Client{
		Timeout: time.Duration(timeoutVal) * time.Second,
	}

	// Use http.NewRequest + WithContext(ctx)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiUrl, bytes.NewReader(requestBody))
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

	ch := make(chan string)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		reader := bufio.NewScanner(resp.Body)
		// Adjust buffer to avoid truncating large lines
		buf := make([]byte, 0, 64*1024)
		reader.Buffer(buf, 1024*1024)

		isThinking := false
		defer func() {
			if isThinking {
				ch <- "</think>"
			}
		}()

		for {
			select {
			case <-ctx.Done():
				glog.Debugf(ctx, "LMStudio context cancelled during streaming read")
				return
			default:
				if !reader.Scan() {
					if err := reader.Err(); err != nil {
						glog.Errorf(ctx, "Error reading LMStudio response stream: %v", err)
					}
					return
				}

				line := reader.Text()
				if line == "" {
					continue
				}

				// Skip non-data lines (must start with "data: ")
				if !bytes.HasPrefix([]byte(line), []byte("data: ")) {
					continue
				}

				// Extract JSON from data line
				jsonData := line[6:]

				// Check for end marker
				if jsonData == "[DONE]" {
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
						Message string      `json:"message"`
						Code    interface{} `json:"code"`
					} `json:"error,omitempty"`
				}

				if err := json.Unmarshal([]byte(jsonData), &result); err == nil {
					if result.Error != nil {
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
							if finishReason == "length" {
								ch <- "[warning]由于内容长度限制，生成的内容可能不完整。如需完整分析，请尝试提供更短的梦境描述。"
							}
							return
						}
					}
				}
			}
		}
	}()

	return ch, nil
}
