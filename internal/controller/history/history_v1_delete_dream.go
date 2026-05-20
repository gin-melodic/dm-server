package history

import (
	"context"

	v1 "dm-server/api/history/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) DeleteDream(ctx context.Context, req *v1.DeleteDreamReq) (res *v1.DeleteDreamRes, err error) {
	res, err = service.History().DeleteDream(ctx, req)
	return
}
