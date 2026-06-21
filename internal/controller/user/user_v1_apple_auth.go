package user

import (
	"context"

	v1 "dm-server/api/user/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) AppleAuth(ctx context.Context, req *v1.AppleAuthReq) (res *v1.AppleAuthRes, err error) {
	return service.Auth().AppleLogin(ctx, req)
}
