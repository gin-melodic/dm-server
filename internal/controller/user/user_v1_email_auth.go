package user

import (
	"context"

	v1 "dm-server/api/user/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) EmailAuth(ctx context.Context, req *v1.EmailAuthReq) (res *v1.EmailAuthRes, err error) {
	return service.Auth().EmailLogin(ctx, req)
}
