package history

import (
	"context"

	v1 "dm-server/api/history/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) GetDream(ctx context.Context, req *v1.GetDreamReq) (res *v1.GetDreamRes, err error) {
	return service.History().GetDream(ctx, req)
}
