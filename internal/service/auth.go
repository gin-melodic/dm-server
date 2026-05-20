// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
	v1 "dm-server/api/user/v1"
	"dm-server/internal/model"
)

type (
	IAuth interface {
		// WechatLogin WeChat login
		WechatLogin(ctx context.Context, req *v1.WechatAuthReq) (*v1.WechatAuthRes, error)
		// GetUserInfo Get user information
		// Used after login to get detailed user info
		GetUserInfo(ctx context.Context, req *v1.GetUserInfoReq) (*v1.GetUserInfoRes, error)
		// UpdateUserInfo Update user information
		UpdateUserInfo(ctx context.Context, req *v1.UpdateUserInfoReq) (*v1.UpdateUserInfoRes, error)
		// VerifyJWT Verify JWT token
		VerifyJWT(ctx context.Context, tokenString string) (*model.AuthClaims, error)
	}
)

var (
	localAuth IAuth
)

func Auth() IAuth {
	if localAuth == nil {
		panic("implement not found for interface IAuth, forgot register?")
	}
	return localAuth
}

func RegisterAuth(i IAuth) {
	localAuth = i
}
