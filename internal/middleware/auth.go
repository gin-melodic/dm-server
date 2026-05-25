package middleware

import (
	"context"
	"dm-server/internal/consts"
	"dm-server/internal/service"
	"net/url"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/glog"
)

// JWT Authentication Middleware
func Auth(r *ghttp.Request) {
	// Skip auth for public login endpoints
	path := r.URL.Path
	if strings.Contains(path, "/wechat/auth") || strings.Contains(path, "/email/auth") {
		r.Middleware.Next()
		return
	}

	glog.Infof(context.Background(), "Auth: header: %s", r.Header.Get("X-Dev-Token"))

	// Passby from dev tool
	// Only in `development` env
	if g.Cfg().MustGet(context.Background(), "env", "production").String() == "development" {
		if r.Header.Get("X-Dev-Token") == "nowled2_token" {
			r.SetCtxVar(consts.CtxUserId, uint64(1))
			r.SetCtxVar(consts.CtxOpenId, "oXXXXXXXXXXXXXXXXXXXXXXX")
			r.Middleware.Next()
			return
		}
	}

	// Get Authorization header
	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		r.Response.Status = 401
		r.Response.WriteJsonExit(ghttp.DefaultHandlerResponse{
			Code:    401,
			Message: "Please log in first",
		})
		return
	}

	// Extract token
	tokenString := strings.TrimPrefix(authorization, "Bearer ")
	if tokenString == authorization {
		r.Response.Status = 401
		r.Response.WriteJsonExit(ghttp.DefaultHandlerResponse{
			Code:    401,
			Message: "Invalid token format",
		})
		return
	}

	// Verify token
	claims, err := service.Auth().VerifyJWT(r.Context(), tokenString)
	if err != nil {
		r.Response.Status = 401
		r.Response.WriteJsonExit(ghttp.DefaultHandlerResponse{
			Code:    401,
			Message: "Token verification failed: " + err.Error(),
		})
		return
	}

	// Store user info in context
	r.SetCtxVar(consts.CtxUserId, claims.ID)
	r.SetCtxVar(consts.CtxOpenId, claims.OpenID)
	r.SetCtxVar(consts.CtxSupabaseUid, claims.SupabaseUID)

	r.Middleware.Next()
}

// JWT Authentication Middleware for WebSocket connections
func AuthWS(r *ghttp.Request) context.Context {
	glog.Infof(context.Background(), "AuthWS: %s", r.GetUrl())

	// Dev tool bypass: query param dev_token (browser WS cannot set custom headers)
	if r.GetQuery("dev_token").String() == "nowled2_token" && g.Cfg().MustGet(context.Background(), "env", "production").String() == "development" {
		r.SetCtxVar(consts.CtxUserId, uint64(1))
		r.SetCtxVar(consts.CtxOpenId, "oXXXXXXXXXXXXXXXXXXXXXXX")
		ctx := context.WithValue(r.Context(), consts.CtxUserId, uint64(1))
		ctx = context.WithValue(ctx, consts.CtxOpenId, "oXXXXXXXXXXXXXXXXXXXXXXX")
		return ctx
	}

	// Extract token from query params for WebSocket
	tokenString := r.GetQuery("token").String()
	// Need unescape
	tokenString, err := url.QueryUnescape(tokenString)
	if err != nil {
		glog.Warning(r.Context(), "WebSocket connection unauthorized request, token decode failed, "+err.Error())
		r.Response.WriteStatus(401)
		r.Exit()
		return nil
	}
	if g.IsEmpty(tokenString) {
		glog.Warning(r.Context(), "WebSocket connection unauthorized request, token is empty")
		r.Response.WriteStatus(401)
		r.Exit()
		return nil
	}

	// Verify token
	claims, err := service.Auth().VerifyJWT(r.Context(), tokenString)
	if err != nil {
		glog.Warning(r.Context(), "WebSocket connection unauthorized request, JWT verification failed, "+err.Error())
		r.Response.WriteStatus(401)
		r.Exit()
		return nil
	}

	ctx := context.WithValue(r.Context(), consts.CtxUserId, claims.ID)

	return ctx
}
