package main

import (
	_ "dm-server/internal/logic"

	"github.com/gogf/gf/v2/os/gctx"

	"dm-server/internal/cmd"
)

func main() {
	cmd.Main.Run(gctx.GetInitCtx())
}
