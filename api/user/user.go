// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package user

import (
	"context"

	"dm-server/api/user/v1"
)

type IUserV1 interface {
	WechatAuth(ctx context.Context, req *v1.WechatAuthReq) (res *v1.WechatAuthRes, err error)
	EmailAuth(ctx context.Context, req *v1.EmailAuthReq) (res *v1.EmailAuthRes, err error)
	AppleAuth(ctx context.Context, req *v1.AppleAuthReq) (res *v1.AppleAuthRes, err error)
	GetUserInfo(ctx context.Context, req *v1.GetUserInfoReq) (res *v1.GetUserInfoRes, err error)
	UpdateUserInfo(ctx context.Context, req *v1.UpdateUserInfoReq) (res *v1.UpdateUserInfoRes, err error)
	GetUserSettings(ctx context.Context, req *v1.GetUserSettingsReq) (res *v1.GetUserSettingsRes, err error)
	UpdateUserSettings(ctx context.Context, req *v1.UpdateUserSettingsReq) (res *v1.UpdateUserSettingsRes, err error)
	GetPsycheProfile(ctx context.Context, req *v1.GetPsycheProfileReq) (res *v1.GetPsycheProfileRes, err error)
}
