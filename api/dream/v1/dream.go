package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

type ChatWebSocketReq struct {
	g.Meta `path:"/chat/ws" method:"get" tags:"Chat" summary:"WebSocket聊天连接"`
}

type ChatWebSocketRes struct{}

// WebSocket 消息结构
type ChatMessage struct {
	Type         string `json:"type"`         // message, error, done
	DreamContent string `json:"dreamContent"` // dream content
	Content      string `json:"content"`      // response content(stream chunk)
	Result       string `json:"result"`       // complete result(only when type=done)
	Error        string `json:"error"`        // error info
}
