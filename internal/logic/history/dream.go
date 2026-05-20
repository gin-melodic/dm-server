package history

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"

	v1 "dm-server/api/history/v1"
	"dm-server/internal/consts"
	"dm-server/internal/model/entity"
	"dm-server/internal/service"
)

type sHistory struct{}

func New() *sHistory {
	return insDream
}

func init() {
	service.RegisterHistory(New())
}

var (
	insDream = &sHistory{}
)

func (s *sHistory) FetchDreamList(ctx context.Context, req *v1.FetchDreamListReq) (*v1.FetchDreamListRes, error) {
	// 1) Get user ID (injected by auth middleware)
	userIdVal := ctx.Value(consts.CtxUserId)
	if userIdVal == nil {
		return nil, gerror.New("unauthorized: user id not found in context")
	}
	userID, ok := userIdVal.(uint64)
	if !ok || userID == 0 {
		return nil, gerror.New("invalid user id in context")
	}

	// 2) Build query model
	dbModel := g.DB().Model("dreams d").
		Where("d.user_id", userID).
		Where("d.deleted_at IS NULL").
		Where("d.status = 'completed'")

	// 3) Add date range conditions
	if req.StartDate != "" && req.EndDate != "" {
		dbModel = dbModel.Where("d.dream_date BETWEEN ? AND ?", req.StartDate, req.EndDate)
	} else {
		// Date range is required
		// Return 400
		return nil, gerror.New("start_date and end_date is required")
	}

	// 4) Get total count
	total, err := dbModel.Count()
	if err != nil {
		return nil, gerror.Wrap(err, "failed to count dreams")
	}

	// 5) Set sorting
	dbModel = dbModel.Order("d.created_at DESC")

	// 6) Set pagination parameters
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 7) Execute paginated query with field selection
	dbModel = dbModel.Fields("d.id, d.title, SUBSTRING(d.content,1,120) AS summary, DATE_FORMAT(d.created_at, '%Y-%m-%d %H:%i:%s') AS create_time")
	var list []v1.DreamSummary
	err = dbModel.Page(page, pageSize).Scan(&list)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query dreams")
	}

	// 8) Return results
	return &v1.FetchDreamListRes{
		Data:     list,
		Page:     page,
		PageSize: pageSize,
		Total:    int64(total),
	}, nil
}

// DeleteDream Delete a dream record
func (s *sHistory) DeleteDream(ctx context.Context, req *v1.DeleteDreamReq) (*v1.DeleteDreamRes, error) {
	// 1) Get user ID (injected by auth middleware)
	userIdVal := ctx.Value(consts.CtxUserId)
	if userIdVal == nil {
		return nil, gerror.New("unauthorized: user id not found in context")
	}
	userID, ok := userIdVal.(uint64)
	if !ok || userID == 0 {
		return nil, gerror.New("invalid user id in context")
	}

	// Delete data from 2 tables within a single transaction
	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Query the dreams table to confirm this dream exists and belongs to the user
		var dream entity.Dreams
		err := tx.Model("dreams").Where("id = ? AND user_id = ?", req.Id, userID).Scan(&dream)
		if err != nil {
			return gerror.Wrap(err, "failed to query dream")
		}
		if g.IsNil(dream) {
			return gerror.New("dream not found")
		}

		// Soft delete records in analysis_sessions table associated with dream_id
		_, err = tx.Model("analysis_sessions").Where("dream_id = ?", req.Id).Update(map[string]interface{}{
			"deleted_at": time.Now(),
		})
		if err != nil {
			return gerror.Wrap(err, "failed to delete analysis_sessions")
		}

		// Delete records in dreams table associated with dream_id
		_, err = tx.Model("dreams").Where("id = ?", req.Id).Update(map[string]interface{}{
			"deleted_at": time.Now(),
		})
		if err != nil {
			return gerror.Wrap(err, "failed to delete dreams")
		}
		return nil
	})
	if err != nil {
		glog.Errorf(ctx, "failed to delete dream: %v", err)
		return &v1.DeleteDreamRes{
			Success: false,
		}, gerror.Wrap(err, "failed to delete dream")
	}
	return &v1.DeleteDreamRes{
		Success: true,
	}, nil
}

// GetDreamAnalyzeResult Get dream analysis result by dream ID
func (s *sHistory) GetDreamAnalyzeResult(ctx context.Context, req *v1.GetDreamAnalyzeResultReq) (*v1.GetDreamAnalyzeResultRes, error) {
	// 1) Get user ID (injected by auth middleware)
	userIdVal := ctx.Value(consts.CtxUserId)
	if userIdVal == nil {
		return nil, gerror.New("unauthorized: user id not found in context")
	}
	userID, ok := userIdVal.(uint64)
	if !ok || userID == 0 {
		return nil, gerror.New("invalid user id in context")
	}
	// Query the dreams table to confirm this dream exists and belongs to the user
	var dream entity.Dreams
	err := g.DB().Model("dreams").Where("id = ? AND user_id = ?", req.Id, userID).Scan(&dream)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query dream")
	}
	if g.IsNil(dream) {
		return nil, gerror.New("dream not found")
	}

	// Query analysis_sessions table by dream_id and return the result_summary field
	var analysisSession entity.AnalysisSessions
	err = g.DB().Model("analysis_sessions").Where("dream_id = ?", req.Id).Scan(&analysisSession)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query analysis session")
	}
	if analysisSession.ResultSummary == "" {
		return nil, gerror.New("analysis session not found")
	}
	return &v1.GetDreamAnalyzeResultRes{
		Result: analysisSession.ResultSummary,
	}, nil
}
