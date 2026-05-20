package router

import (
	"dm-server/internal/controller/history"
	"dm-server/internal/controller/user"
	"dm-server/internal/middleware"

	"github.com/gogf/gf/v2/net/ghttp"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
)

type ServerRouter struct {
	Prefix string
}

func NewServerRouter(prefix string) *ServerRouter {
	return &ServerRouter{Prefix: prefix}
}

func (s *ServerRouter) Register() func(r *ghttp.RouterGroup) {
	return func(r *ghttp.RouterGroup) {
		r.Middleware(middlewareCORS)
		r.Middleware(ghttp.MiddlewareHandlerResponse)

		// Set prefix using Group method
		r.Group(s.Prefix, func(prefixGroup *ghttp.RouterGroup) {
			// Auth globally for /api routes (skips wechat/auth automatically)
			prefixGroup.Middleware(middleware.Auth)

			// Bind public router
			prefixGroup.Bind(
				user.NewV1(),
			)

			// Need auth (middleware.Auth runs above, so no need to repeat it here, but keeping it is harmless)
			prefixGroup.Group("/v1", func(apiGroup *ghttp.RouterGroup) {
				// Bind controllers that require authentication
				apiGroup.Bind(
					history.NewV1(),
				)
			})
		})
	}
}

func middlewareCORS(r *ghttp.Request) {
	r.Response.CORSDefault()
	r.Middleware.Next()
}
