package user

import (
	"context"

	v1 "dm-server/api/user/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) GetPsycheProfile(ctx context.Context, req *v1.GetPsycheProfileReq) (res *v1.GetPsycheProfileRes, err error) {
	return service.Auth().GetPsycheProfile(ctx, req)
}
