package v1

import "github.com/gogf/gf/v2/frame/g"

// 微信小程序授权请求
type WechatAuthReq struct {
	g.Meta `path:"/wechat/auth" method:"post" summary:"WeChat MiniProgram Authorization Login" tags:"user_auth"`
	Code   string `json:"code" v:"required#Authorization code cannot be empty" dc:"WeChat Authorization Code"`
}

type WechatAuthRes struct {
	Token    string    `json:"token" dc:"JWT Token"`
	OpenId   string    `json:"openid" dc:"WeChat OpenID"`
	UserInfo *UserInfo `json:"user_info" dc:"user info"`
}

// Email login via Supabase token
type EmailAuthReq struct {
	g.Meta      `path:"/email/auth" method:"post" summary:"Email Login via Supabase Token" tags:"user_auth"`
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
	g.Meta `path:"/user/info" method:"get" summary:"Get User Info" tags:"user_management"`
}

type GetUserInfoRes struct {
	UserInfo *UserInfo `json:"user_info"`
}

// Update User Info Request
type UpdateUserInfoReq struct {
	g.Meta `path:"/user/info" method:"put" summary:"Update User Info" tags:"user_management"`
	UserInfo
}

type UpdateUserInfoRes struct {
	UserInfo *UserInfo `json:"user_info"`
}
