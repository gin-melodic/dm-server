// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// Users is the golang structure for table users.
type Users struct {
	Id           uint64      `json:"id"           orm:"id"            description:"User ID"`                                      // User ID
	Openid       string      `json:"openid"       orm:"openid"        description:"WeChat openid, nullable for non-wechat users"` // WeChat openid, nullable for non-wechat users
	Unionid      string      `json:"unionid"      orm:"unionid"       description:"WeChat unionid, nullable"`                     // WeChat unionid, nullable
	Nickname     string      `json:"nickname"     orm:"nickname"      description:"User nickname"`                                // User nickname
	AvatarUrl    string      `json:"avatarUrl"    orm:"avatar_url"    description:"Avatar URL"`                                   // Avatar URL
	Mobile       string      `json:"mobile"       orm:"mobile"        description:"Mobile number"`                                // Mobile number
	Email        string      `json:"email"        orm:"email"         description:"Email address"`                                // Email address
	CreatedAt    *gtime.Time `json:"createdAt"    orm:"created_at"    description:"Created at"`                                   // Created at
	UpdatedAt    *gtime.Time `json:"updatedAt"    orm:"updated_at"    description:"Updated at"`                                   // Updated at
	DeletedAt    *gtime.Time `json:"deletedAt"    orm:"deleted_at"    description:"Deleted at, NULL means not deleted"`           // Deleted at, NULL means not deleted
	SupabaseUid  string      `json:"supabaseUid"  orm:"supabase_uid"  description:"Supabase Auth user UUID (JWT sub)"`            // Supabase Auth user UUID (JWT sub)
	AuthProvider string      `json:"authProvider" orm:"auth_provider" description:"Auth provider: wechat | email | ..."`          // Auth provider: wechat | email | ...
}
