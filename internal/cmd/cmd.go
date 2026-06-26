package cmd

import (
	"context"
	"fmt"
	"dm-server/internal/config"
	"dm-server/internal/controller/devtool"
	"dm-server/internal/router"
	"dm-server/internal/utility/limiter"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"
)

var (
	Main = gcmd.Command{
		Name:  "main",
		Usage: "main",
		Brief: "start http server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// Get config from env
			config.RewriteConfigFromEnv()

			fmt.Printf("AI_SERVICE: %s\n", g.Cfg().MustGet(ctx, "ai_service").String())

			s := g.Server()

			// Static resource directory mapping for dev testing tool
			s.AddStaticPath("/static", "resource/public")

			// Rate limiter initialization
			_ = limiter.Init(ctx)

			router := router.NewServerRouter("/api")

			// Register Dev API Test Page & Proxy (Bypass auth middleware)
			s.Group("/dev", func(group *ghttp.RouterGroup) {
				devCtrl := &devtool.Controller{}
				group.GET("/api-test", devCtrl.Index)
				group.POST("/api-test/proxy", devCtrl.Proxy)
			})

			s.Group("/", func(group *ghttp.RouterGroup) {
				router.Register()(group)
			})
			s.Run()
			return nil
		},
	}
)
