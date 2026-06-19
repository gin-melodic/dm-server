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
func (s *sOllama) analyzeDreamStream(ctx context.Context, prompt, dreamContent string) (<-chan string, error) {
	// Get Ollama config
	ollamaConfig, err := g.Cfg().Get(ctx, "ollama")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get Ollama config")
	}

	baseURL := ollamaConfig.MapStrStr()["base_url"]
	model := ollamaConfig.MapStrStr()["model"]

	// Set default values
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "qwq:32b"
	}

	// apiUrl
	apiUrl := fmt.Sprintf("%s/api/generate", baseURL)

	// Print the config
	glog.Infof(ctx, "Base URL: %s", baseURL)
	glog.Infof(ctx, "Model: %s", model)

	// Construct full prompt (sent to LLM, keep in Chinese for the model)
	fullPrompt := fmt.Sprintf("%s\n\n用户的梦境内容如下：\n%s", prompt, dreamContent)
	glog.Infof(ctx, "Prompt: %s", previewRunes(fullPrompt, 100))

	// Construct request
	request := sOllamaRequest{
		Model:  model,
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiUrl, bytes.NewReader(requestBody))
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to build HTTP request")
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Reuse default client or configure Transport to ensure cancel support
	resp, err := http.DefaultClient.Do(httpReq)
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

	ch := make(chan string)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		reader := bufio.NewScanner(resp.Body)
		// Adjustable buffer to avoid truncation of large lines
		buf := make([]byte, 0, 64*1024)
		reader.Buffer(buf, 1024*1024)

		for {
			select {
			case <-ctx.Done():
				// Trigger cancel: exit directly, HTTP client will break underlying connection
				return
			default:
				if !reader.Scan() {
					// EOF or error
					if err := reader.Err(); err != nil {
						// Can pass error upstream when necessary
						// ch <- "[error]" + err.Error()
					}
					return
				}
				var result struct {
					Content string `json:"response"`
					Done    bool   `json:"done"`
				}
				if err := json.Unmarshal(reader.Bytes(), &result); err == nil {
					if result.Content != "" {
						ch <- result.Content
					}
					if result.Done {
						return
					}
				}
			}
		}
	}()
	return ch, nil
}
