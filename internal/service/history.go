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
		// DeleteDream Delete a dream record
		DeleteDream(ctx context.Context, req *v1.DeleteDreamReq) (*v1.DeleteDreamRes, error)
		// GetDreamAnalyzeResult Get dream analysis result by dream ID
		GetDreamAnalyzeResult(ctx context.Context, req *v1.GetDreamAnalyzeResultReq) (*v1.GetDreamAnalyzeResultRes, error)
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
