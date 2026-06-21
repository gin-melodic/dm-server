package dream

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	v1 "dm-server/api/dream/v1"
	"dm-server/internal/consts"
	"dm-server/internal/middleware"
	"dm-server/internal/model"
	"dm-server/internal/service"

	"github.com/gorilla/websocket"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
)

// gorilla/websocket Upgrader (strictly validate Origin in production)
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Strictly validate Origin in production
		return true
	},
	// Optional: Error callback
	Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
		g.Log().Error(r.Context(), reason)
	},
}

const (
	wsPongWait   = 60 * time.Second
	wsPingPeriod = 25 * time.Second
	wsWriteWait  = 10 * time.Second
)

func (c *ControllerV1) ChatWebSocket(ctx context.Context, req *v1.ChatWebSocketReq) (res *v1.ChatWebSocketRes, err error) {
	r := g.RequestFromCtx(ctx)
	ctx = middleware.AuthWS(r)
	return c.chatWebSocket(ctx, req)
}

func (c *ControllerV1) chatWebSocket(ctx context.Context, req *v1.ChatWebSocketReq) (res *v1.ChatWebSocketRes, err error) {
	r := g.RequestFromCtx(ctx)

	// Upgrade to WebSocket connection
	glog.Infof(ctx, "Upgrading to WebSocket connection")
	conn, err := wsUpgrader.Upgrade(r.Response.Writer, r.Request, nil)
	if err != nil {
		g.Log().Error(ctx, "websocket upgrade failed:", err)
		return nil, nil
	}
	defer conn.Close()

	// Get user ID
	userId := ctx.Value(consts.CtxUserId)
	if userId == nil || userId == "" {
		g.Log().Error(ctx, "failed to get user id")
		r.Response.WriteStatus(401)
		return nil, nil
	}

	// Derive cancellable ctx
	wsCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type wsMessage struct {
		mt  int
		msg []byte
		err error
	}
	msgChan := make(chan wsMessage, 10)

	// Single read loop: continuously read to receive all client messages, close/ping/pong/errors
	go func() {
		conn.SetReadLimit(1 << 20)
		_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
			return nil
		})
		for {
			mt, msg, err := conn.ReadMessage()
			select {
			case msgChan <- wsMessage{mt: mt, msg: msg, err: err}:
			case <-wsCtx.Done():
				return
			}
			if err != nil {
				cancel()
				return
			}
		}
	}()
	pingTicker := time.NewTicker(wsPingPeriod)
	defer pingTicker.Stop()

	// Message loop: handle incoming client messages and stream responses
	for {
		select {
		case <-wsCtx.Done():
			return nil, nil
		case <-pingTicker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				cancel()
				return nil, nil
			}
		case wsMsg := <-msgChan:
			if wsMsg.err != nil {
				g.Log().Debug(ctx, "ws read closed:", wsMsg.err)
				return nil, nil
			}

			var in v1.ChatMessage
			if err := json.Unmarshal(wsMsg.msg, &in); err != nil {
				writeWsError(conn, wsMsg.mt, "invalid json: "+err.Error())
				continue
			}
			if in.Type != "message" || in.DreamContent == "" {
				writeWsError(conn, wsMsg.mt, "invalid payload: need Type=message, prompt & dreamContent")
				continue
			}

			streamCtx := wsCtx
			if emotion := strings.TrimSpace(in.Emotion); emotion != "" {
				streamCtx = context.WithValue(streamCtx, consts.CtxDreamEmotionTags, []string{emotion})
			}

			// Call Service: begin LLM streaming processing
			stream, err := service.Dream().StreamDream(streamCtx, in.DreamContent)
			if err != nil {
				writeWsError(conn, wsMsg.mt, "service error: "+err.Error())
				continue
			}

			// Write loop: pull chunks from stream and write back; stop if wsCtx cancelled or write fails
			var fullResult strings.Builder
			completed := false
			failed := false
		writeLoop:
			for {
				select {
				case <-wsCtx.Done():
					failed = true
					break writeLoop
				case <-pingTicker.C:
					_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						cancel()
						failed = true
						break writeLoop
					}
				case event, ok := <-stream:
					if !ok {
						break writeLoop
					}
					switch event.Type {
					case model.DreamStreamEventDelta:
						if event.Content == "" {
							continue
						}
						fullResult.WriteString(event.Content)
						out := v1.ChatMessage{Type: "message", Content: event.Content}
						if err := writeWsJSON(conn, wsMsg.mt, out); err != nil {
							cancel()
							failed = true
							break writeLoop
						}
					case model.DreamStreamEventWarning:
						_ = writeWsJSON(conn, wsMsg.mt, v1.ChatMessage{Type: "warning", Error: event.Message})
					case model.DreamStreamEventError:
						failed = true
						writeWsError(conn, wsMsg.mt, event.Message)
						break writeLoop
					case model.DreamStreamEventCompleted:
						completed = true
						break writeLoop
					}
				}
			}
			// If normally completed and connection still alive, send done with full result
			if !completed && !failed {
				writeWsError(conn, wsMsg.mt, "stream closed without completion")
				failed = true
			}
			select {
			case <-wsCtx.Done():
				// Already cancelled, no more writes
			default:
				if completed && !failed {
					_ = writeWsJSON(conn, wsMsg.mt, v1.ChatMessage{Type: "done", Result: fullResult.String()})
				}
			}
		}
	}
}

func writeWsJSON(conn *websocket.Conn, mt int, v any) error {
	b, _ := json.Marshal(v)
	_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	return conn.WriteMessage(mt, b)
}

func writeWsError(conn *websocket.Conn, mt int, msg string) {
	_ = writeWsJSON(conn, mt, map[string]any{
		"type":  "error",
		"error": msg,
	})
}
