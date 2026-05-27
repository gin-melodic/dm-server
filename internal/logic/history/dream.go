package history

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "dm-server/api/history/v1"
	"dm-server/internal/consts"
	"dm-server/internal/model/entity"
	"dm-server/internal/service"
)

type sHistory struct{}

type dreamRow struct {
	Id              uint64      `orm:"id"`
	UserId          uint64      `orm:"user_id"`
	Title           string      `orm:"title"`
	Content         string      `orm:"content"`
	DreamDate       *gtime.Time `orm:"dream_date"`
	Tags            string      `orm:"tags"`
	CreatedAt       *gtime.Time `orm:"created_at"`
	UpdatedAt       *gtime.Time `orm:"updated_at"`
	DeletedAt       *gtime.Time `orm:"deleted_at"`
	Status          string      `orm:"status"`
	Emotion         string      `orm:"emotion"`
	IsFavorite      bool        `orm:"is_favorite"`
	ConfidenceScore float64     `orm:"confidence_score"`
}

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

func (s *sHistory) GetDream(ctx context.Context, req *v1.GetDreamReq) (*v1.GetDreamRes, error) {
	userID, err := getContextUserID(ctx)
	if err != nil {
		return nil, err
	}
	record, err := s.getDreamRecord(ctx, userID, req.Id)
	if err != nil {
		return nil, err
	}
	res := v1.GetDreamRes(*record)
	return &res, nil
}

func (s *sHistory) UpdateDream(ctx context.Context, req *v1.UpdateDreamReq) (*v1.UpdateDreamRes, error) {
	userID, err := getContextUserID(ctx)
	if err != nil {
		return nil, err
	}

	data := g.Map{}
	if req.Title != "" {
		data["title"] = req.Title
	}
	if req.Content != "" {
		data["content"] = req.Content
	}
	if req.Emotion != "" {
		data["emotion"] = req.Emotion
	}
	if req.IsFavorite != nil {
		data["is_favorite"] = *req.IsFavorite
	}
	if len(data) > 0 {
		data["updated_at"] = time.Now()
		_, err = g.DB().Model("dreams").
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", req.Id, userID).
			Data(data).
			Update()
		if err != nil {
			return nil, gerror.Wrap(err, "failed to update dream")
		}
	}

	record, err := s.getDreamRecord(ctx, userID, req.Id)
	if err != nil {
		return nil, err
	}
	res := v1.UpdateDreamRes(*record)
	return &res, nil
}

func (s *sHistory) CreateDreamAnalysis(ctx context.Context, req *v1.CreateDreamAnalysisReq) (*v1.CreateDreamAnalysisRes, error) {
	userID, err := getContextUserID(ctx)
	if err != nil {
		return nil, err
	}

	ch, err := service.Dream().StreamDream(ctx, req.Content)
	if err != nil {
		return nil, err
	}
	var builder strings.Builder
	for piece := range ch {
		if strings.HasPrefix(piece, "[error]") {
			return nil, gerror.New(strings.TrimPrefix(piece, "[error]"))
		}
		builder.WriteString(piece)
	}

	var dream entity.Dreams
	err = g.DB().Model("dreams").
		Where("user_id = ? AND content = ? AND deleted_at IS NULL", userID, req.Content).
		OrderDesc("id").
		Limit(1).
		Scan(&dream)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query analyzed dream")
	}
	if dream.Id == 0 {
		return nil, gerror.New("analyzed dream not found")
	}

	updateData := g.Map{"confidence_score": 0.88, "updated_at": time.Now()}
	if req.Emotion != "" {
		updateData["emotion"] = req.Emotion
	}
	if _, err := g.DB().Model("dreams").
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", dream.Id, userID).
		Data(updateData).
		Update(); err != nil {
		return nil, gerror.Wrap(err, "failed to update dream analysis metadata")
	}

	record, err := s.getDreamRecord(ctx, userID, dream.Id)
	if err != nil {
		return nil, err
	}
	analysis := buildAnalysisResult(builder.String(), record, req.Locale)
	return &v1.CreateDreamAnalysisRes{
		Dream:    record,
		Analysis: analysis,
		Steps: []v1.DreamAnalysisStep{
			{Key: "record", Title: "Record dream", Description: "Dream content saved", Status: "completed"},
			{Key: "analyze", Title: "Analyze symbols", Description: "Dream interpretation generated", Status: "completed"},
			{Key: "persist", Title: "Persist result", Description: "Analysis result stored", Status: "completed"},
		},
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

func (s *sHistory) SetDreamFavorite(ctx context.Context, req *v1.SetDreamFavoriteReq) (*v1.SetDreamFavoriteRes, error) {
	userID, err := getContextUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.getDreamRecord(ctx, userID, req.Id); err != nil {
		return nil, err
	}
	if _, err := g.DB().Model("dreams").
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", req.Id, userID).
		Data(g.Map{"is_favorite": req.IsFavorite, "updated_at": time.Now()}).
		Update(); err != nil {
		return nil, gerror.Wrap(err, "failed to update dream favorite")
	}
	record, err := s.getDreamRecord(ctx, userID, req.Id)
	if err != nil {
		return nil, err
	}
	res := v1.SetDreamFavoriteRes(*record)
	return &res, nil
}

func (s *sHistory) GetDreamHome(ctx context.Context, req *v1.GetDreamHomeReq) (*v1.GetDreamHomeRes, error) {
	userID, err := getContextUserID(ctx)
	if err != nil {
		return nil, err
	}
	total, err := g.DB().Model("dreams").
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Count()
	if err != nil {
		return nil, gerror.Wrap(err, "failed to count dreams")
	}
	recent, err := s.listDreamRecords(ctx, userID, 5)
	if err != nil {
		return nil, err
	}
	waves := make([]v1.EmotionWavePoint, 0, len(recent))
	for _, item := range recent {
		date := item.CreatedAt
		if len(date) >= 10 {
			date = date[:10]
		}
		waves = append(waves, v1.EmotionWavePoint{
			Date:    date,
			Emotion: item.Emotion,
			Count:   1,
		})
	}
	home := v1.GetDreamHomeRes{
		TotalDreams:       total,
		CurrentStreakDays: currentStreakDays(recent),
		EmotionWaves:      waves,
		RecentDreams:      recent,
	}
	if len(recent) > 0 {
		home.Recommendation = &v1.DreamRecommendation{
			Dream: &recent[0],
			Score: recent[0].ConfidenceScore,
			Tier:  "standard",
		}
	}
	return &home, nil
}

func (s *sHistory) GetTodayDreamRecommendation(ctx context.Context, req *v1.GetTodayDreamRecommendationReq) (*v1.GetTodayDreamRecommendationRes, error) {
	userID, err := getContextUserID(ctx)
	if err != nil {
		return nil, err
	}
	recent, err := s.listDreamRecords(ctx, userID, 1)
	if err != nil {
		return nil, err
	}
	if len(recent) == 0 {
		return nil, gerror.New("dream not found")
	}
	res := v1.GetTodayDreamRecommendationRes{
		Dream: &recent[0],
		Score: recent[0].ConfidenceScore,
		Tier:  "standard",
	}
	return &res, nil
}

func (s *sHistory) getDreamRecord(ctx context.Context, userID, dreamID uint64) (*v1.DreamRecord, error) {
	var dream dreamRow
	err := g.DB().Model("dreams").
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", dreamID, userID).
		Scan(&dream)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query dream")
	}
	if dream.Id == 0 {
		return nil, gerror.New("dream not found")
	}
	var analysis entity.AnalysisSessions
	_ = g.DB().Model("analysis_sessions").
		Where("dream_id = ? AND deleted_at IS NULL", dream.Id).
		OrderDesc("id").
		Limit(1).
		Scan(&analysis)
	return buildDreamRecord(dream, analysis.ResultSummary), nil
}

func (s *sHistory) listDreamRecords(ctx context.Context, userID uint64, limit int) ([]v1.DreamRecord, error) {
	var dreams []dreamRow
	err := g.DB().Model("dreams").
		Where("user_id = ? AND deleted_at IS NULL", userID).
		OrderDesc("created_at").
		Limit(limit).
		Scan(&dreams)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query recent dreams")
	}
	records := make([]v1.DreamRecord, 0, len(dreams))
	for _, dream := range dreams {
		var analysis entity.AnalysisSessions
		_ = g.DB().Model("analysis_sessions").
			Where("dream_id = ? AND deleted_at IS NULL", dream.Id).
			OrderDesc("id").
			Limit(1).
			Scan(&analysis)
		records = append(records, *buildDreamRecord(dream, analysis.ResultSummary))
	}
	return records, nil
}

func getContextUserID(ctx context.Context) (uint64, error) {
	userIdVal := ctx.Value(consts.CtxUserId)
	if userIdVal == nil {
		return 0, gerror.New("unauthorized: user id not found in context")
	}
	userID, ok := userIdVal.(uint64)
	if !ok || userID == 0 {
		return 0, gerror.New("invalid user id in context")
	}
	return userID, nil
}

func buildDreamRecord(dream dreamRow, interpretation string) *v1.DreamRecord {
	if dream.Emotion == "" {
		dream.Emotion = "neutral"
	}
	if dream.ConfidenceScore == 0 {
		dream.ConfidenceScore = 0.86
	}
	return &v1.DreamRecord{
		Id:              dream.Id,
		Title:           dream.Title,
		Content:         dream.Content,
		Interpretation:  interpretation,
		Emotion:         dream.Emotion,
		Keywords:        splitKeywords(dream.Tags),
		ConfidenceScore: dream.ConfidenceScore,
		IsFavorite:      dream.IsFavorite,
		CreatedAt:       formatGTime(dream.CreatedAt),
		UpdatedAt:       formatGTime(dream.UpdatedAt),
	}
}

func buildAnalysisResult(text string, record *v1.DreamRecord, locale string) *v1.DreamAnalysisResult {
	summary := record.Title
	if summary == "" {
		summary = "Dream analysis"
	}
	return &v1.DreamAnalysisResult{
		Summary:         summary,
		Interpretation:  text,
		Keywords:        record.Keywords,
		ConfidenceScore: record.ConfidenceScore,
		Locale:          locale,
	}
}

func splitKeywords(tags string) []string {
	if tags == "" {
		return []string{}
	}
	parts := strings.Split(tags, ",")
	keywords := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			keywords = append(keywords, trimmed)
		}
	}
	return keywords
}

func formatGTime(t *gtime.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("Y-m-d H:i:s")
}

func currentStreakDays(records []v1.DreamRecord) int {
	if len(records) == 0 {
		return 0
	}
	days := make(map[string]struct{})
	for _, record := range records {
		if len(record.CreatedAt) >= 10 {
			days[record.CreatedAt[:10]] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(days))
	for day := range days {
		ordered = append(ordered, day)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ordered)))
	return len(ordered)
}
