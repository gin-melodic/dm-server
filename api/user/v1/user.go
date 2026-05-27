package v1

import "github.com/gogf/gf/v2/frame/g"

// 微信小程序授权请求
type WechatAuthReq struct {
	g.Meta `path:"/v1/wechat/auth" method:"post" summary:"WeChat MiniProgram Authorization Login" tags:"user_auth"`
	Code   string `json:"code" v:"required#Authorization code cannot be empty" dc:"WeChat Authorization Code"`
}

type WechatAuthRes struct {
	Token    string    `json:"token" dc:"JWT Token"`
	OpenId   string    `json:"openid" dc:"WeChat OpenID"`
	UserInfo *UserInfo `json:"user_info" dc:"user info"`
}

// Email login via Supabase token
type EmailAuthReq struct {
	g.Meta      `path:"/v1/email/auth" method:"post" summary:"Email Login via Supabase Token" tags:"user_auth"`
	AccessToken string `json:"access_token" v:"required#access_token cannot be empty" dc:"Supabase access_token"`
}

type EmailAuthRes struct {
	Token    string    `json:"token"`
	UserInfo *UserInfo `json:"user_info"`
}

type UserInfo struct {
	Id       uint64 `json:"id"`
	OpenId   string `json:"openid"`
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar_url"`
}

// Get User Info Request
type GetUserInfoReq struct {
	g.Meta `path:"/v1/user/info" method:"get" summary:"Get User Info" tags:"user_management"`
}

type GetUserInfoRes struct {
	UserInfo *UserInfo `json:"user_info"`
}

// Update User Info Request
type UpdateUserInfoReq struct {
	g.Meta `path:"/v1/user/info" method:"put" summary:"Update User Info" tags:"user_management"`
	UserInfo
}

type UpdateUserInfoRes struct {
	UserInfo *UserInfo `json:"user_info"`
}

// Get user settings request
type GetUserSettingsReq struct {
	g.Meta `path:"/v1/user/settings" method:"get" summary:"Get User Settings" tags:"user_management"`
	// TODO: Return current user settings.
}

type UserSettings struct {
	Language             string `json:"language" dc:"Preferred language"`
	PrivacyMode          string `json:"privacy_mode" dc:"Privacy mode"`
	DreamReminderEnabled bool   `json:"dream_reminder_enabled" dc:"Whether dream reminders are enabled"`
	DreamReminderTime    string `json:"dream_reminder_time" dc:"Daily dream reminder time"`
	StorageMode          string `json:"storage_mode" dc:"Storage mode"`
}

type GetUserSettingsRes struct {
	UserSettings *UserSettings `json:"user_settings"`
}

// Update user settings request
type UpdateUserSettingsReq struct {
	g.Meta `path:"/v1/user/settings" method:"put" summary:"Update User Settings" tags:"user_management"`
	UserSettings
	// TODO: Persist current user settings.
}

type UpdateUserSettingsRes struct {
	UserSettings *UserSettings `json:"user_settings"`
}

// Get user psyche profile request
type GetPsycheProfileReq struct {
	g.Meta `path:"/v1/user/psyche-profile" method:"get" summary:"Get Psyche Profile" tags:"user_management"`
	// TODO: Return the current user's psyche profile.
}

type PsycheProfile struct {
	IntegrationScore       float64 `json:"integration_score" dc:"Profile integration score"`
	IntegrationLevel       string  `json:"integration_level" dc:"Profile integration level"`
	IntegrationDescription string  `json:"integration_description" dc:"Profile integration description"`
	DominantArchetype      string  `json:"dominant_archetype" dc:"Dominant archetype"`
	UpdatedAt              string  `json:"updated_at" dc:"Last update time"`
}

type GetPsycheProfileRes struct {
	PsycheProfile *PsycheProfile `json:"psyche_profile"`
}
