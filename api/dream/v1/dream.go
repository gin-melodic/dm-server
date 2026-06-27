package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

type ChatWebSocketReq struct {
	g.Meta `path:"/v1/chat/ws" method:"get" tags:"Chat" summary:"WebSocket聊天连接"`
}

type ChatWebSocketRes struct{}

// WebSocket 消息结构
type ChatMessage struct {
	Type         string `json:"type"`              // message, warning, error, done
	DreamContent string `json:"dreamContent"`      // dream content
	Emotion      string `json:"emotion,omitempty"` // dream emotion tag
	Locale       string `json:"locale,omitempty"`  // response locale: en, zh-Hans, zh-Hant
	Lang         string `json:"lang,omitempty"`    // response language alias
	Content      string `json:"content"`           // response content(stream chunk)
	Result       string `json:"result"`            // complete result(only when type=done)
	Error        string `json:"error"`             // error info
}
