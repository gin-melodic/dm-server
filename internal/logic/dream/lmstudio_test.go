package dream

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

func TestLMStudio_AnalyzeDreamStream_Success(t *testing.T) {
	ctx := context.Background()

	// Spin up a mock HTTP server to simulate LMStudio SSE stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request details
		if r.Method != http.MethodPost {
			t.Errorf("Expected method POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("Expected path /chat/completions, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send mock SSE chunks
		chunks := []string{
			`data: {"choices": [{"delta": {"content": "解析"}}]}`,
			`data: {"choices": [{"delta": {"content": "您的"}}]}`,
			`data: {"choices": [{"delta": {"content": "梦境"}}]}`,
			`data: {"choices": [{"delta": {"content": "。"}, "finish_reason": "stop"}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			_, _ = fmt.Fprint(w, chunk+"\n\n")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	// Configure GoFrame to point to the mock server URL
	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		// Set dynamic configuration parameters
		adapter.Set("lmstudio.base_url", server.URL)
		adapter.Set("lmstudio.model", "qwen3.6-27b-ud-test")
		adapter.Set("lmstudio.timeout", "5")
	}

	// Trigger the analysis stream
	ch, err := shareLMStudio.analyzeDreamStream(ctx, "系统提示词", "梦境内容")
	if err != nil {
		t.Fatalf("Failed to analyze dream stream: %v", err)
	}

	var results []string
	for chunk := range ch {
		results = append(results, chunk)
	}

	fullResponse := strings.Join(results, "")
	expectedResponse := "解析您的梦境。"
	if fullResponse != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, fullResponse)
	}
}

func TestLMStudio_AnalyzeDreamStream_DefaultModel(t *testing.T) {
	ctx := context.Background()

	// Spin up a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, `data: {"choices": [{"delta": {"content": "测试"}}]} `+"\n\n")
		_, _ = fmt.Fprint(w, `data: [DONE]`+"\n\n")
	}))
	defer server.Close()

	// Configure GoFrame with blank model to test default fallback
	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		adapter.Set("lmstudio.base_url", server.URL)
		adapter.Set("lmstudio.model", "") // blank model triggers fallback to qwen3.6-27b-ud
		adapter.Set("lmstudio.timeout", "5")
	}

	ch, err := shareLMStudio.analyzeDreamStream(ctx, "系统提示词", "梦境内容")
	if err != nil {
		t.Fatalf("Failed to analyze dream stream: %v", err)
	}

	var results []string
	for chunk := range ch {
		results = append(results, chunk)
	}

	fullResponse := strings.Join(results, "")
	if fullResponse != "测试" {
		t.Errorf("Expected response %q, got %q", "测试", fullResponse)
	}
}

func TestLMStudio_AnalyzeDreamStream_Timeout(t *testing.T) {
	ctx := context.Background()

	// Spin up a slow mock HTTP server to trigger timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, `data: {"choices": [{"delta": {"content": "不应该收到这一条"}}]} `+"\n\n")
		_, _ = fmt.Fprint(w, `data: [DONE]`+"\n\n")
	}))
	defer server.Close()

	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		adapter.Set("lmstudio.base_url", server.URL)
		adapter.Set("lmstudio.model", "qwen3.6-27b-ud-test")
		adapter.Set("lmstudio.timeout", "1") // 1 second timeout
	}

	// We create a context with a short timeout to enforce quick failure if http client doesn't trigger first
	shortCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	_, err := shareLMStudio.analyzeDreamStream(shortCtx, "系统提示词", "梦境内容")
	if err == nil {
		t.Fatal("Expected error due to context timeout, got nil")
	}
}

func TestLMStudio_AnalyzeDreamStream_Reasoning(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		chunks := []string{
			`data: {"choices": [{"delta": {"reasoning_content": "思考"}, "finish_reason": "none"}]}`,
			`data: {"choices": [{"delta": {"reasoning": "中"}, "finish_reason": "null"}]}`,
			`data: {"choices": [{"delta": {"content": "分析内容"}}]}`,
			`data: {"choices": [{"delta": {}, "finish_reason": "stop"}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			_, _ = fmt.Fprint(w, chunk+"\n\n")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		adapter.Set("lmstudio.base_url", server.URL)
		adapter.Set("lmstudio.model", "qwen3.6-27b-ud-test")
		adapter.Set("lmstudio.timeout", "5")
	}

	ch, err := shareLMStudio.analyzeDreamStream(ctx, "系统提示词", "梦境内容")
	if err != nil {
		t.Fatalf("Failed to analyze dream stream with reasoning: %v", err)
	}

	var results []string
	for chunk := range ch {
		results = append(results, chunk)
	}

	fullResponse := strings.Join(results, "")
	expectedResponse := "<think>思考中</think>分析内容"
	if fullResponse != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, fullResponse)
	}
}
