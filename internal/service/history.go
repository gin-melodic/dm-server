// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
	v1 "dm-server/api/history/v1"
)

type (
	IHistory interface {
		FetchDreamList(ctx context.Context, req *v1.FetchDreamListReq) (*v1.FetchDreamListRes, error)
		GetDream(ctx context.Context, req *v1.GetDreamReq) (*v1.GetDreamRes, error)
		UpdateDream(ctx context.Context, req *v1.UpdateDreamReq) (*v1.UpdateDreamRes, error)
		// DeleteDream Delete a dream record
		DeleteDream(ctx context.Context, req *v1.DeleteDreamReq) (*v1.DeleteDreamRes, error)
		CreateDreamAnalysis(ctx context.Context, req *v1.CreateDreamAnalysisReq) (*v1.CreateDreamAnalysisRes, error)
		// GetDreamAnalyzeResult Get dream analysis result by dream ID
		GetDreamAnalyzeResult(ctx context.Context, req *v1.GetDreamAnalyzeResultReq) (*v1.GetDreamAnalyzeResultRes, error)
		SetDreamFavorite(ctx context.Context, req *v1.SetDreamFavoriteReq) (*v1.SetDreamFavoriteRes, error)
		GetDreamHome(ctx context.Context, req *v1.GetDreamHomeReq) (*v1.GetDreamHomeRes, error)
		GetTodayDreamRecommendation(ctx context.Context, req *v1.GetTodayDreamRecommendationReq) (*v1.GetTodayDreamRecommendationRes, error)
	}
)

var (
	localHistory IHistory
)

func History() IHistory {
	if localHistory == nil {
		panic("implement not found for interface IHistory, forgot register?")
	}
	return localHistory
}

func RegisterHistory(i IHistory) {
	localHistory = i
}
