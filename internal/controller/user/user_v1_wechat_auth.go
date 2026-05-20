package user

import (
	"context"

	v1 "dm-server/api/user/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) WechatAuth(ctx context.Context, req *v1.WechatAuthReq) (res *v1.WechatAuthRes, err error) {
	return service.Auth().WechatLogin(ctx, req)
}
