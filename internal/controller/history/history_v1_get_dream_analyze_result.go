package history

import (
	"context"

	v1 "dm-server/api/history/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) GetDreamAnalyzeResult(ctx context.Context, req *v1.GetDreamAnalyzeResultReq) (res *v1.GetDreamAnalyzeResultRes, err error) {
	res, err = service.History().GetDreamAnalyzeResult(ctx, req)
	return
}
