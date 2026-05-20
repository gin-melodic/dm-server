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
	GetUserInfo(ctx context.Context, req *v1.GetUserInfoReq) (res *v1.GetUserInfoRes, err error)
	UpdateUserInfo(ctx context.Context, req *v1.UpdateUserInfoReq) (res *v1.UpdateUserInfoRes, err error)
}
