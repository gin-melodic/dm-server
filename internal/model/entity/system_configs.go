// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// SystemConfigs is the golang structure for table system_configs.
type SystemConfigs struct {
	Key       string      `json:"key"       orm:"key"        description:"Config key"`                        // Config key
	Value     string      `json:"value"     orm:"value"      description:"Config value (JSON or plain text)"` // Config value (JSON or plain text)
	UpdatedAt *gtime.Time `json:"updatedAt" orm:"updated_at" description:"Updated at"`                        // Updated at
}
