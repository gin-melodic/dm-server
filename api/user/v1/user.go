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

// Email login or signup via password.
type EmailAuthReq struct {
	g.Meta   `path:"/v1/email/auth" method:"post" summary:"Email Login or Auto Signup" tags:"user_auth"`
	Email    string `json:"email" v:"required|email#email cannot be empty|invalid email" dc:"Email address"`
	Password string `json:"password" v:"required|length:6,128#password cannot be empty|password must be 6-128 characters" dc:"Password"`
}

type EmailAuthRes struct {
	Token                     string    `json:"token"`
	UserInfo                  *UserInfo `json:"user_info"`
	AuthFlow                  string    `json:"auth_flow,omitempty"`
	EmailVerificationRequired bool      `json:"email_verification_required,omitempty"`
}

type UserInfo struct {
	Id       uint64  `json:"id"`
	OpenId   string  `json:"openid"`
	Email    string  `json:"email"`
	Nickname string  `json:"nickname"`
	Avatar   string  `json:"avatar_url"`
	Lang     *string `json:"lang"`
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
	Language *string `json:"language" dc:"Deprecated. Use lang."`
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
	PrivacyMode          string `json:"privacyMode" dc:"Privacy mode"`
	DreamReminderEnabled *bool  `json:"dreamReminderEnabled" dc:"Whether dream reminders are enabled"`
	DreamReminderTime    string `json:"dreamReminderTime" dc:"Daily dream reminder time"`
	StorageMode          string `json:"storageMode" dc:"Storage mode"`
}

type GetUserSettingsRes = UserSettings

// Update user settings request
type UpdateUserSettingsReq struct {
	g.Meta `path:"/v1/user/settings" method:"put" summary:"Update User Settings" tags:"user_management"`
	UserSettings
	// TODO: Persist current user settings.
}

type UpdateUserSettingsRes = UserSettings

// Get user psyche profile request
type GetPsycheProfileReq struct {
	g.Meta `path:"/v1/user/psyche-profile" method:"get" summary:"Get Psyche Profile" tags:"user_management"`
	// TODO: Return the current user's psyche profile.
}

type PsycheProfile struct {
	IntegrationScore       float64                `json:"integrationScore" dc:"Profile integration score"`
	IntegrationLevel       string                 `json:"integrationLevel" dc:"Profile integration level"`
	IntegrationDescription string                 `json:"integrationDescription" dc:"Profile integration description"`
	Archetypes             []ArchetypeProfileItem `json:"archetypes" dc:"Archetype profile"`
	DominantArchetype      string                 `json:"dominantArchetype" dc:"Dominant archetype"`
	UpdatedAt              string                 `json:"updatedAt" dc:"Last update time"`
}

type ArchetypeProfileItem struct {
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
}

type GetPsycheProfileRes = PsycheProfile
