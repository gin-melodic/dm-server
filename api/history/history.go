// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package history

import (
	"context"

	"dm-server/api/history/v1"
)

type IHistoryV1 interface {
	FetchDreamList(ctx context.Context, req *v1.FetchDreamListReq) (res *v1.FetchDreamListRes, err error)
	GetDream(ctx context.Context, req *v1.GetDreamReq) (res *v1.GetDreamRes, err error)
	UpdateDream(ctx context.Context, req *v1.UpdateDreamReq) (res *v1.UpdateDreamRes, err error)
	DeleteDream(ctx context.Context, req *v1.DeleteDreamReq) (res *v1.DeleteDreamRes, err error)
	CreateDreamAnalysis(ctx context.Context, req *v1.CreateDreamAnalysisReq) (res *v1.CreateDreamAnalysisRes, err error)
	GetDreamAnalyzeResult(ctx context.Context, req *v1.GetDreamAnalyzeResultReq) (res *v1.GetDreamAnalyzeResultRes, err error)
	SetDreamFavorite(ctx context.Context, req *v1.SetDreamFavoriteReq) (res *v1.SetDreamFavoriteRes, err error)
	GetDreamHome(ctx context.Context, req *v1.GetDreamHomeReq) (res *v1.GetDreamHomeRes, err error)
	GetTodayDreamRecommendation(ctx context.Context, req *v1.GetTodayDreamRecommendationReq) (res *v1.GetTodayDreamRecommendationRes, err error)
}
