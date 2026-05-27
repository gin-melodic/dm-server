package history

import (
	"context"

	v1 "dm-server/api/history/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) GetTodayDreamRecommendation(ctx context.Context, req *v1.GetTodayDreamRecommendationReq) (res *v1.GetTodayDreamRecommendationRes, err error) {
	return service.History().GetTodayDreamRecommendation(ctx, req)
}
