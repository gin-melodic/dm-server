package history

import (
	"context"

	v1 "dm-server/api/history/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) GetDreamHome(ctx context.Context, req *v1.GetDreamHomeReq) (res *v1.GetDreamHomeRes, err error) {
	return service.History().GetDreamHome(ctx, req)
}
