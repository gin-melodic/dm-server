package dream

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"dm-server/internal/model"
)

const (
	defaultResponseHeaderTimeout = 20 * time.Second
	defaultStreamIdleTimeout     = 45 * time.Second
	defaultGenerationTimeout     = 300 * time.Second
)

type streamTimeouts struct {
	ResponseHeader time.Duration
	Idle           time.Duration
	Generation     time.Duration
}

var streamingHTTPClients sync.Map

func providerTimeouts(config map[string]string) streamTimeouts {
	return streamTimeouts{
		ResponseHeader: durationSeconds(config["response_header_timeout"], defaultResponseHeaderTimeout),
		Idle:           durationSeconds(config["idle_timeout"], defaultStreamIdleTimeout),
		Generation:     durationSeconds(config["timeout"], defaultGenerationTimeout),
	}
}

func durationSeconds(raw string, fallback time.Duration) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func positiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func configuredBool(raw string, fallback bool) bool {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

func streamingHTTPClient(timeouts streamTimeouts) *http.Client {
	if cached, ok := streamingHTTPClients.Load(timeouts.ResponseHeader); ok {
		return cached.(*http.Client)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = timeouts.ResponseHeader
	client := &http.Client{Transport: transport}
	actual, _ := streamingHTTPClients.LoadOrStore(timeouts.ResponseHeader, client)
	return actual.(*http.Client)
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			Reasoning        string `json:"reasoning"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason any `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"error,omitempty"`
}

type sseReadResult struct {
	data string
	err  error
	eof  bool
}

// streamOpenAIResponse parses the SSE variants used by OpenAI-compatible APIs.
func streamOpenAIResponse(
	ctx context.Context,
	resp *http.Response,
	provider, providerModel string,
	idleTimeout time.Duration,
	cancel context.CancelFunc,
) <-chan model.DreamStreamEvent {
	out := make(chan model.DreamStreamEvent, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		defer cancel()

		reads := make(chan sseReadResult, 1)
		go scanSSE(ctx, resp.Body, reads)

		idle := time.NewTimer(idleTimeout)
		defer idle.Stop()
		thinking := false
		terminal := false

		emit := func(event model.DreamStreamEvent) bool {
			event.Provider = provider
			event.Model = providerModel
			if event.Terminal() {
				select {
				case out <- event:
					return true
				case <-time.After(time.Second):
					return false
				}
			}
			select {
			case out <- event:
				return true
			case <-ctx.Done():
				return false
			}
		}
		finishThinking := func() bool {
			if !thinking {
				return true
			}
			thinking = false
			return emit(model.DreamStreamEvent{Type: model.DreamStreamEventDelta, Content: "</think>"})
		}
		emitError := func(message, reason string) {
			if terminal {
				return
			}
			terminal = true
			_ = finishThinking()
			_ = emit(model.DreamStreamEvent{Type: model.DreamStreamEventError, Message: message, FinishReason: reason})
		}

		for !terminal {
			select {
			case <-ctx.Done():
				emitError(ctx.Err().Error(), "canceled")
			case <-idle.C:
				_ = resp.Body.Close()
				emitError("model stream idle timeout", "idle_timeout")
			case item, ok := <-reads:
				if !ok {
					emitError("model stream ended without a completion marker", "unexpected_eof")
					continue
				}
				if !idle.Stop() {
					select {
					case <-idle.C:
					default:
					}
				}
				idle.Reset(idleTimeout)
				if item.err != nil {
					emitError("failed to read model stream: "+item.err.Error(), "read_error")
					continue
				}
				if item.eof {
					emitError("model stream ended without a completion marker", "unexpected_eof")
					continue
				}
				data := strings.TrimSpace(item.data)
				if data == "" {
					continue
				}
				if data == "[DONE]" {
					if !finishThinking() {
						return
					}
					terminal = true
					_ = emit(model.DreamStreamEvent{Type: model.DreamStreamEventCompleted, FinishReason: "done"})
					continue
				}

				var chunk openAIStreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					emitError("invalid model stream JSON: "+err.Error(), "parse_error")
					continue
				}
				if chunk.Error != nil {
					emitError(chunk.Error.Message, "provider_error")
					continue
				}
				for _, choice := range chunk.Choices {
					reasoning := choice.Delta.ReasoningContent
					if reasoning == "" {
						reasoning = choice.Delta.Reasoning
					}
					if reasoning != "" {
						if !thinking {
							thinking = true
							if !emit(model.DreamStreamEvent{Type: model.DreamStreamEventDelta, Content: "<think>"}) {
								return
							}
						}
						if !emit(model.DreamStreamEvent{Type: model.DreamStreamEventDelta, Content: reasoning}) {
							return
						}
					}
					content := choice.Delta.Content
					if content == "" {
						content = choice.Message.Content
					}
					if content != "" {
						if !finishThinking() || !emit(model.DreamStreamEvent{Type: model.DreamStreamEventDelta, Content: content}) {
							return
						}
					}
					reason := normalizeFinishReason(choice.FinishReason)
					if reason == "" || reason == "none" || reason == "null" {
						continue
					}
					if !finishThinking() {
						return
					}
					if reason == "length" {
						if !emit(model.DreamStreamEvent{Type: model.DreamStreamEventWarning, Message: "由于内容长度限制，生成的内容可能不完整。", FinishReason: reason}) {
							return
						}
					}
					terminal = true
					_ = emit(model.DreamStreamEvent{Type: model.DreamStreamEventCompleted, FinishReason: reason})
					break
				}
			}
		}
	}()
	return out
}

func normalizeFinishReason(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(typed))
	default:
		return strings.ToLower(strings.TrimSpace(fmt.Sprint(typed)))
	}
}

func scanSSE(ctx context.Context, reader io.Reader, out chan<- sseReadResult) {
	defer close(out)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	data := make([]string, 0, 1)
	flush := func() bool {
		if len(data) == 0 {
			return true
		}
		item := sseReadResult{data: strings.Join(data, "\n")}
		data = data[:0]
		select {
		case out <- item:
			return true
		case <-ctx.Done():
			return false
		}
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if !flush() {
				return
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			value := strings.TrimPrefix(line, "data:")
			value = strings.TrimPrefix(value, " ")
			data = append(data, value)
		}
	}
	if !flush() {
		return
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, http.ErrBodyReadAfterClose) {
		select {
		case out <- sseReadResult{err: err}:
		case <-ctx.Done():
		}
		return
	}
	select {
	case out <- sseReadResult{eof: true}:
	case <-ctx.Done():
	}
}
