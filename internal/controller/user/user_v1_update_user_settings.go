package user

import (
	"context"

	v1 "dm-server/api/user/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) UpdateUserSettings(ctx context.Context, req *v1.UpdateUserSettingsReq) (res *v1.UpdateUserSettingsRes, err error) {
	return service.Auth().UpdateUserSettings(ctx, req)
}
