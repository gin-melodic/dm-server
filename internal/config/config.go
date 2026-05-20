package config

import (
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
	"github.com/gogf/gf/v2/os/genv"
)

func RewriteConfigFromEnv() {
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
	dbLink := fmt.Sprintf("mysql:%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		dbUser, dbPassword, dbHost, dbPort, dbName)
	g.Cfg().GetAdapter().(*gcfg.AdapterFile).Set("database.default.link", dbLink)

	fmt.Println("config overridden from env")
}
