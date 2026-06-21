package dream

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"dm-server/internal/model"
)

func TestOpenAIStreamAcceptsSSEVariantsAndCompletes(t *testing.T) {
	body := strings.Join([]string{
		": keepalive\r",
		"data:{\"choices\":[{\"delta\":{\"content\":\"解\"}}]}\r",
		"\r",
		"data: {\"choices\":[{\"delta\":{\"content\":\"析\"},\"finish_reason\":\"stop\"}]}\r",
		"\r",
	}, "\n")
	events := testOpenAIEvents(t, body, time.Second)
	if got := eventContent(events); got != "解析" {
		t.Fatalf("content = %q, want 解析", got)
	}
	assertTerminal(t, events, model.DreamStreamEventCompleted, "stop")
}

func TestOpenAIStreamDoneMarkerCompletes(t *testing.T) {
	events := testOpenAIEvents(t, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata:[DONE]\n\n", time.Second)
	assertTerminal(t, events, model.DreamStreamEventCompleted, "done")
}

func TestOpenAIStreamSupportsMultiLineDataAndLengthFinish(t *testing.T) {
	body := "data: {\"choices\":[\n" +
		"data: {\"delta\":{\"content\":\"truncated\"},\"finish_reason\":\"length\"}\n" +
		"data: ]}\n\n"
	events := testOpenAIEvents(t, body, time.Second)
	if got := eventContent(events); got != "truncated" {
		t.Fatalf("content = %q, want truncated", got)
	}
	assertTerminal(t, events, model.DreamStreamEventCompleted, "length")
	warnings := 0
	for _, event := range events {
		if event.Type == model.DreamStreamEventWarning {
			warnings++
		}
	}
	if warnings != 1 {
		t.Fatalf("warning count = %d, want 1", warnings)
	}
}

func TestOpenAIStreamRejectsMalformedJSON(t *testing.T) {
	events := testOpenAIEvents(t, "data: {bad json}\n\n", time.Second)
	assertTerminal(t, events, model.DreamStreamEventError, "parse_error")
}

func TestOpenAIStreamRejectsUnexpectedEOF(t *testing.T) {
	events := testOpenAIEvents(t, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n", time.Second)
	if got := eventContent(events); got != "partial" {
		t.Fatalf("content = %q, want partial", got)
	}
	assertTerminal(t, events, model.DreamStreamEventError, "unexpected_eof")
}

func TestOpenAIStreamIdleTimeout(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	resp := &http.Response{Body: reader}
	events := collectEvents(streamOpenAIResponse(ctx, resp, "test", "test-model", 20*time.Millisecond, cancel))
	assertTerminal(t, events, model.DreamStreamEventError, "idle_timeout")
}

func testOpenAIEvents(t *testing.T, body string, idle time.Duration) []model.DreamStreamEvent {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	return collectEvents(streamOpenAIResponse(ctx, resp, "test", "test-model", idle, cancel))
}

func collectEvents(ch <-chan model.DreamStreamEvent) []model.DreamStreamEvent {
	events := make([]model.DreamStreamEvent, 0, 4)
	for event := range ch {
		events = append(events, event)
	}
	return events
}

func eventContent(events []model.DreamStreamEvent) string {
	var result strings.Builder
	for _, event := range events {
		if event.Type == model.DreamStreamEventDelta {
			result.WriteString(event.Content)
		}
	}
	return result.String()
}

func assertTerminal(t *testing.T, events []model.DreamStreamEvent, eventType model.DreamStreamEventType, reason string) {
	t.Helper()
	terminalCount := 0
	for _, event := range events {
		if !event.Terminal() {
			continue
		}
		terminalCount++
		if event.Type != eventType || event.FinishReason != reason {
			t.Fatalf("terminal event = %+v, want type=%s reason=%s", event, eventType, reason)
		}
	}
	if terminalCount != 1 {
		t.Fatalf("terminal event count = %d, want 1; events=%+v", terminalCount, events)
	}
}
