package model

import "github.com/golang-jwt/jwt/v4"

type WechatUserInfo struct {
	Openid    string `json:"openid"`
	Unionid   string `json:"unionid"`
	Nickname  string `json:"nickname"`
	AvatarUrl string `json:"headimgurl"`
}

// AuthClaims JWT user info
type AuthClaims struct {
	ID     uint64 `json:"id"`
	OpenID string `json:"openid"`
	jwt.RegisteredClaims
}
