// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// SystemConfigs is the golang structure of table system_configs for DAO operations like Where/Data.
type SystemConfigs struct {
	g.Meta    `orm:"table:system_configs, do:true"`
	Key       any         // Config key
	Value     any         // Config value (JSON or plain text)
	UpdatedAt *gtime.Time // Updated at
}
