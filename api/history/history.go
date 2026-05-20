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
	DeleteDream(ctx context.Context, req *v1.DeleteDreamReq) (res *v1.DeleteDreamRes, err error)
	GetDreamAnalyzeResult(ctx context.Context, req *v1.GetDreamAnalyzeResultReq) (res *v1.GetDreamAnalyzeResultRes, err error)
}
