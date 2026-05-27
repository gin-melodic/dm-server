package history

import (
	"context"

	v1 "dm-server/api/history/v1"
	"dm-server/internal/service"
)

func (c *ControllerV1) SetDreamFavorite(ctx context.Context, req *v1.SetDreamFavoriteReq) (res *v1.SetDreamFavoriteRes, err error) {
	return service.History().SetDreamFavorite(ctx, req)
}
