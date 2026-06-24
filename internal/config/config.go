package config

import (
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
	"github.com/gogf/gf/v2/os/genv"
)

func RewriteConfigFromEnv() {
	if jwtSecret := strings.TrimSpace(genv.Get("JWT_SECRET", "").String()); jwtSecret != "" {
		g.Cfg().GetAdapter().(*gcfg.AdapterFile).Set("jwt.secret", jwtSecret)
	}
	if jwtTimeout := genv.Get("JWT_TIMEOUT", 0).Int(); jwtTimeout > 0 {
		g.Cfg().GetAdapter().(*gcfg.AdapterFile).Set("jwt.timeout", jwtTimeout)
	}

	if projectURL := genv.Get("SUPABASE_PROJECT_URL", "").String(); projectURL != "" {
		g.Cfg().GetAdapter().(*gcfg.AdapterFile).Set("supabase.project_url", projectURL)
	}
	if publishableKey := genv.Get("SUPABASE_PUBLISHABLE_KEY", "").String(); publishableKey != "" {
		g.Cfg().GetAdapter().(*gcfg.AdapterFile).Set("supabase.publishable_key", publishableKey)
	}
	if secretKey := genv.Get("SUPABASE_SECRET_KEY", "").String(); secretKey != "" {
		g.Cfg().GetAdapter().(*gcfg.AdapterFile).Set("supabase.secret_key", secretKey)
	}

	// if env is empty, skip
	if genv.Get("PORT", "").String() == "" || genv.Get("DB_USER", "").String() == "" {
		return
	}
	// override config from env
	port := genv.Get("PORT", "").Int()
	g.Cfg().GetAdapter().(*gcfg.AdapterFile).Set("server.address", fmt.Sprintf(":%d", port))
	dbUser := genv.Get("DB_USER", "").String()
	dbPassword := genv.Get("DB_PASSWORD", "").String()
	dbHost := genv.Get("DB_HOST", "").String()
	dbPort := genv.Get("DB_PORT", "").String()
	dbName := genv.Get("DB_NAME", "").String()
	dbLink := fmt.Sprintf("pgsql:%s:%s@tcp(%s:%s)/%s",
		dbUser, dbPassword, dbHost, dbPort, dbName)
	g.Cfg().GetAdapter().(*gcfg.AdapterFile).Set("database.default.link", dbLink)

	fmt.Println("config overridden from env")
}
