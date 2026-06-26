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

	"dm-server/internal/model"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
)

type sOllama struct{}

var shareOllama = &sOllama{}

// sOllamaRequest represents the request structure for Ollama API
type sOllamaRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Stream  bool   `json:"stream"`
	Options struct {
		Temperature      float64  `json:"temperature,omitempty"`
		MaxTokens        int      `json:"num_predict,omitempty"`
		TopP             float64  `json:"top_p,omitempty"`
		FrequencyPenalty float64  `json:"repeat_penalty,omitempty"`
		Stop             []string `json:"stop,omitempty"`
	} `json:"options,omitempty"`
}

// analyzeDreamStream Analyze dream using Ollama
func (s *sOllama) analyzeDreamStream(ctx context.Context, prompt, dreamContent string) (<-chan model.DreamStreamEvent, error) {
	// Get Ollama config
	ollamaConfig, err := g.Cfg().Get(ctx, "ollama")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get Ollama config")
	}

	config := ollamaConfig.MapStrStr()
	baseURL := config["base_url"]
	modelName := configuredModel(ctx, config["model"], "qwq:32b")
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
		baseURL = "http://localhost:11434"
	}
	// apiUrl
	apiUrl := fmt.Sprintf("%s/api/generate", baseURL)

	// Print the config
	glog.Infof(ctx, "Base URL: %s", baseURL)
	glog.Infof(ctx, "Model: %s", modelName)

	// Construct full prompt (sent to LLM, keep in Chinese for the model)
	fullPrompt := fmt.Sprintf("%s\n\n用户的梦境内容如下：\n%s", prompt, dreamContent)
	glog.Infof(ctx, "Prompt: %s", previewRunes(fullPrompt, 1000))

	// Construct request
	request := sOllamaRequest{
		Model:  modelName,
		Prompt: fullPrompt,
		Stream: true,
	}

	// Set model parameters to prevent overthinking
	request.Options.Temperature = 0.7
	request.Options.MaxTokens = 2048 // Limit max output tokens
	request.Options.TopP = 0.9
	request.Options.FrequencyPenalty = 1.05
	request.Options.Stop = []string{"\n\n\n", "## 结束", "//完成"}

	// Convert request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to serialize request")
	}

	// Use http.NewRequest + WithContext(ctx)
	httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodPost, apiUrl, bytes.NewReader(requestBody))
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to build HTTP request")
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Reuse default client or configure Transport to ensure cancel support
	resp, err := streamingHTTPClient(timeouts).Do(httpReq)
	if err != nil {
		// If ctx is canceled, err may be context.Canceled
		if errors.Is(err, context.Canceled) {
			return nil, context.Canceled
		}
		return nil, gerror.Wrap(err, "Failed to send request")
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, gerror.Newf("Ollama API returned status code %d: %s", resp.StatusCode, string(b))
	}

	ch := make(chan model.DreamStreamEvent, 16)
	streamOwnsCancel = true
	go func() {
		defer resp.Body.Close()
		defer close(ch)
		defer cancel()

		reader := bufio.NewScanner(resp.Body)
		// Adjustable buffer to avoid truncation of large lines
		buf := make([]byte, 0, 64*1024)
		reader.Buffer(buf, 1024*1024)

		idle := time.NewTimer(timeouts.Idle)
		defer idle.Stop()
		lines := make(chan sseReadResult, 1)
		go func() {
			for reader.Scan() {
				lines <- sseReadResult{data: string(reader.Bytes())}
			}
			if err := reader.Err(); err != nil {
				lines <- sseReadResult{err: err}
			} else {
				lines <- sseReadResult{eof: true}
			}
			close(lines)
		}()
		emit := func(event model.DreamStreamEvent) bool {
			event.Provider = "ollama"
			event.Model = modelName
			if event.Terminal() {
				select {
				case ch <- event:
					return true
				case <-time.After(time.Second):
					return false
				}
			}
			select {
			case ch <- event:
				return true
			case <-requestCtx.Done():
				return false
			}
		}
		for {
			select {
			case <-requestCtx.Done():
				_ = emit(model.DreamStreamEvent{Type: model.DreamStreamEventError, Message: requestCtx.Err().Error(), FinishReason: "canceled"})
				return
			case <-idle.C:
				_ = resp.Body.Close()
				_ = emit(model.DreamStreamEvent{Type: model.DreamStreamEventError, Message: "model stream idle timeout", FinishReason: "idle_timeout"})
				return
			case line := <-lines:
				if !idle.Stop() {
					select {
					case <-idle.C:
					default:
					}
				}
				idle.Reset(timeouts.Idle)
				if line.err != nil {
					_ = emit(model.DreamStreamEvent{Type: model.DreamStreamEventError, Message: line.err.Error(), FinishReason: "read_error"})
					return
				}
				if line.eof {
					_ = emit(model.DreamStreamEvent{Type: model.DreamStreamEventError, Message: "model stream ended without done=true", FinishReason: "unexpected_eof"})
					return
				}
				var result struct {
					Content string `json:"response"`
					Done    bool   `json:"done"`
				}
				if err := json.Unmarshal([]byte(line.data), &result); err == nil {
					if result.Content != "" {
						if !emit(model.DreamStreamEvent{Type: model.DreamStreamEventDelta, Content: result.Content}) {
							return
						}
					}
					if result.Done {
						_ = emit(model.DreamStreamEvent{Type: model.DreamStreamEventCompleted, FinishReason: "done"})
						return
					}
				} else {
					_ = emit(model.DreamStreamEvent{Type: model.DreamStreamEventError, Message: "invalid Ollama stream JSON: " + err.Error(), FinishReason: "parse_error"})
					return
				}
			}
		}
	}()
	return ch, nil
}
