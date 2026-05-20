package user

import (
	"context"

	v1 "dm-server/api/user/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) UpdateUserInfo(ctx context.Context, req *v1.UpdateUserInfoReq) (res *v1.UpdateUserInfoRes, err error) {
	return service.Auth().UpdateUserInfo(ctx, req)
}
