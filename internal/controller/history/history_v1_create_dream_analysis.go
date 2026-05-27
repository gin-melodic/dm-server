package history

import (
	"context"

	v1 "dm-server/api/history/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) CreateDreamAnalysis(ctx context.Context, req *v1.CreateDreamAnalysisReq) (res *v1.CreateDreamAnalysisRes, err error) {
	return service.History().CreateDreamAnalysis(ctx, req)
}
