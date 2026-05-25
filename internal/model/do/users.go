// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// Users is the golang structure of table users for DAO operations like Where/Data.
type Users struct {
	g.Meta       `orm:"table:users, do:true"`
	Id           any         // User ID
	Openid       any         // WeChat openid, nullable for non-wechat users
	Unionid      any         // WeChat unionid, nullable
	Nickname     any         // User nickname
	AvatarUrl    any         // Avatar URL
	Mobile       any         // Mobile number
	Email        any         // Email address
	CreatedAt    *gtime.Time // Created at
	UpdatedAt    *gtime.Time // Updated at
	DeletedAt    *gtime.Time // Deleted at, NULL means not deleted
	SupabaseUid  any         // Supabase Auth user UUID (JWT sub)
	AuthProvider any         // Auth provider: wechat | email | ...
}
