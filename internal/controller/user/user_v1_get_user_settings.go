package user

import (
	"context"

	v1 "dm-server/api/user/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) GetUserSettings(ctx context.Context, req *v1.GetUserSettingsReq) (res *v1.GetUserSettingsRes, err error) {
	return service.Auth().GetUserSettings(ctx, req)
}
