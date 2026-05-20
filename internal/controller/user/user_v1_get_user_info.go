package user

import (
	"context"

	v1 "dm-server/api/user/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) GetUserInfo(ctx context.Context, req *v1.GetUserInfoReq) (res *v1.GetUserInfoRes, err error) {
	return service.Auth().GetUserInfo(ctx, req)
}
