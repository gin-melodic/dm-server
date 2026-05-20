package history

import (
	"context"

	v1 "dm-server/api/history/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) FetchDreamList(ctx context.Context, req *v1.FetchDreamListReq) (res *v1.FetchDreamListRes, err error) {
	res, err = service.History().FetchDreamList(ctx, req)
	return
}
