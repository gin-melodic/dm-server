package history

import (
	"context"

	v1 "dm-server/api/history/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) UpdateDream(ctx context.Context, req *v1.UpdateDreamReq) (res *v1.UpdateDreamRes, err error) {
	return service.History().UpdateDream(ctx, req)
}
