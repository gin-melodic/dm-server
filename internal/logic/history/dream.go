package history

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/text/gstr"
	"github.com/gogf/gf/v2/util/guid"

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
	Symbols         string      `orm:"symbols"`
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

const (
	dreamStatusPending    = "pending"
	dreamStatusProcessing = "processing"
	dreamStatusCompleted  = "completed"
	dreamStatusError      = "error"

	defaultConfidenceScore = 0.86
	analysisConfidence     = 0.88
	maxDreamTitleRunes     = 128
	maxDreamContentRunes   = 10000
	homeRecentLimit        = 5
	homeWaveDays           = 30
)

var allowedDreamEmotions = map[string]struct{}{
	"neutral":  {},
	"happy":    {},
	"sad":      {},
	"angry":    {},
	"fear":     {},
	"anxious":  {},
	"excited":  {},
	"peaceful": {},
	"confused": {},
}

func (s *sHistory) FetchDreamList(ctx context.Context, req *v1.FetchDreamListReq) (*v1.FetchDreamListRes, error) {
	userID, err := getContextUserID(ctx)
	if err != nil {
		return nil, err
	}

	dbModel := g.DB().Model("dreams d").
		Where("d.user_id", userID).
		Where("d.deleted_at IS NULL").
		Where("d.status = 'completed'")

	hasDateRange := req.StartDate != "" && req.EndDate != ""
	if hasDateRange {
		dbModel = dbModel.Where("d.dream_date BETWEEN ? AND ?", req.StartDate, req.EndDate)
	} else if req.StartDate != "" || req.EndDate != "" {
		return nil, gerror.New("startDate and endDate must be provided together")
	}

	if req.Emotion != "" {
		if !isAllowedDreamEmotion(req.Emotion) {
			return nil, gerror.New("invalid dream emotion")
		}
		dbModel = dbModel.Where("d.emotion", req.Emotion)
	}

	if req.FavoriteOnly {
		dbModel = dbModel.Where("d.is_favorite", true)
	}

	keyword := strings.TrimSpace(req.Keyword)
	if keyword != "" {
		likeKeyword := "%" + keyword + "%"
		dbModel = dbModel.Where(
			`(
				d.title ILIKE ?
				OR d.content ILIKE ?
				OR d.tags ILIKE ?
				OR d.symbols ILIKE ?
				OR EXISTS (
					SELECT 1
					FROM analysis_sessions a
					WHERE a.dream_id = d.id
						AND a.deleted_at IS NULL
						AND a.status = ?
						AND a.result_summary ILIKE ?
				)
			)`,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			likeKeyword,
			dreamStatusCompleted,
			likeKeyword,
		)
	}

	total, err := dbModel.Count()
	if err != nil {
		return nil, gerror.Wrap(err, "failed to count dreams")
	}

	dbModel = dbModel.Order("d.created_at DESC")

	page, pageSize := 0, 0
	if !hasDateRange {
		page = req.Page
		if page <= 0 {
			page = 1
		}
		pageSize = req.PageSize
		if pageSize <= 0 {
			pageSize = 10
		}
		if pageSize > 100 {
			pageSize = 100
		}
		dbModel = dbModel.Page(page, pageSize)
	}

	var dreams []dreamRow
	if err := dbModel.Scan(&dreams); err != nil {
		return nil, gerror.Wrap(err, "failed to query dreams")
	}

	items := make([]v1.DreamRecord, 0, len(dreams))
	for _, dream := range dreams {
		items = append(items, *buildDreamRecordWithLatestAnalysis(ctx, dream))
	}

	res := &v1.FetchDreamListRes{
		Items: items,
		Total: int64(total),
	}
	if !hasDateRange {
		res.Page = page
		res.PageSize = pageSize
		res.HasMore = page*pageSize < total
	}
	return res, nil
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
	if _, err := s.getDreamRecord(ctx, userID, req.Id); err != nil {
		return nil, err
	}

	data := g.Map{}
	if req.Title != "" {
		if utf8.RuneCountInString(req.Title) > maxDreamTitleRunes {
			return nil, gerror.New("title is too long")
		}
		data["title"] = req.Title
	}
	if req.Content != "" {
		if err := validateDreamContent(req.Content); err != nil {
			return nil, err
		}
		data["content"] = req.Content
		data["status"] = dreamStatusPending
		data["symbols"] = ""
	}
	if req.Emotion != "" {
		if !isAllowedDreamEmotion(req.Emotion) {
			return nil, gerror.New("invalid dream emotion")
		}
		data["emotion"] = req.Emotion
	}
	if req.IsFavorite != nil {
		data["is_favorite"] = *req.IsFavorite
	}
	if len(data) > 0 {
		data["updated_at"] = time.Now()
		err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			if _, err := tx.Model("dreams").
				Where("id = ? AND user_id = ? AND deleted_at IS NULL", req.Id, userID).
				Data(data).
				Update(); err != nil {
				return gerror.Wrap(err, "failed to update dream")
			}
			if req.Content != "" {
				if _, err := tx.Model("analysis_sessions").
					Where("dream_id = ? AND deleted_at IS NULL", req.Id).
					Data(g.Map{"deleted_at": time.Now(), "updated_at": time.Now()}).
					Update(); err != nil {
					return gerror.Wrap(err, "failed to invalidate analysis sessions")
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
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
	if err := validateDreamContent(req.Content); err != nil {
		return nil, err
	}
	locale := normalizeLocale(req.Locale)
	emotion := req.Emotion
	if emotion == "" {
		emotion = "neutral"
	}
	if !isAllowedDreamEmotion(emotion) {
		return nil, gerror.New("invalid dream emotion")
	}

	now := time.Now()
	var dreamID uint64
	var sessionID uint64
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		id, err := tx.Model("dreams").Data(g.Map{
			"user_id":          userID,
			"content":          req.Content,
			"dream_date":       now.Format("2006-01-02"),
			"emotion":          emotion,
			"status":           dreamStatusPending,
			"confidence_score": analysisConfidence,
			"created_at":       now,
			"updated_at":       now,
		}).InsertAndGetId()
		if err != nil {
			return gerror.Wrap(err, "failed to insert dream")
		}
		dreamID = uint64(id)
		sid, err := tx.Model("analysis_sessions").Data(g.Map{
			"dream_id":      dreamID,
			"session_uuid":  guid.S(),
			"analysis_type": "dream",
			"status":        dreamStatusPending,
			"progress":      0,
			"created_at":    now,
			"updated_at":    now,
		}).InsertAndGetId()
		if err != nil {
			return gerror.Wrap(err, "failed to insert analysis session")
		}
		sessionID = uint64(sid)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := updateAnalysisLifecycle(ctx, dreamID, sessionID, dreamStatusProcessing, 10, ""); err != nil {
		return nil, err
	}

	emotionTags := filterHistoryEmotionTags(emotion)
	symbols, err := service.Dream().ExtractDreamSymbols(ctx, req.Content, emotionTags)
	if err != nil {
		glog.Warningf(ctx, "知识库符号提取失败: %v", err)
	}
	streamMetadata := &consts.DreamStreamMetadata{
		SymbolsDetected: normalizeDreamSymbols(symbols),
	}
	streamCtx := context.WithValue(ctx, consts.CtxDreamEmotionTags, emotionTags)
	streamCtx = context.WithValue(streamCtx, consts.CtxDreamStreamMetadata, streamMetadata)
	ch, err := service.Dream().StreamDream(streamCtx, req.Content)
	if err != nil {
		_ = updateAnalysisLifecycle(ctx, dreamID, sessionID, dreamStatusError, 100, err.Error())
		return nil, err
	}
	var builder strings.Builder
	for piece := range ch {
		if strings.HasPrefix(piece, "[error]") {
			msg := strings.TrimSpace(strings.TrimPrefix(piece, "[error]"))
			_ = updateAnalysisLifecycle(ctx, dreamID, sessionID, dreamStatusError, 100, msg)
			return nil, gerror.New(msg)
		}
		builder.WriteString(piece)
	}
	analysisText := builder.String()
	title, interpretation := extractAnalysisTitleAndBody(analysisText)
	keywords := deriveDreamKeywords(req.Content, interpretation, emotion)
	finalSymbols := normalizeDreamSymbols(streamMetadata.SymbolsDetected)
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Model("dreams").
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", dreamID, userID).
			Data(g.Map{
				"title":            title,
				"tags":             strings.Join(keywords, ","),
				"symbols":          strings.Join(finalSymbols, ","),
				"emotion":          emotion,
				"status":           dreamStatusCompleted,
				"confidence_score": analysisConfidence,
				"updated_at":       time.Now(),
			}).
			Update(); err != nil {
			return gerror.Wrap(err, "failed to update dream analysis metadata")
		}
		if _, err := tx.Model("analysis_sessions").
			Where("id = ? AND dream_id = ? AND deleted_at IS NULL", sessionID, dreamID).
			Data(g.Map{
				"status":         dreamStatusCompleted,
				"progress":       100,
				"result_summary": interpretation,
				"updated_at":     time.Now(),
			}).
			Update(); err != nil {
			return gerror.Wrap(err, "failed to complete analysis session")
		}
		return nil
	})
	if err != nil {
		_ = updateAnalysisLifecycle(ctx, dreamID, sessionID, dreamStatusError, 100, err.Error())
		return nil, err
	}
	if streamMetadata.InferenceLevel != "L1" && len(finalSymbols) > 0 {
		if err := service.Dream().SinkDreamSymbolCache(ctx, fmt.Sprintf("%d", userID), finalSymbols, interpretation, fmt.Sprintf("%d", dreamID)); err != nil {
			glog.Warningf(ctx, "知识库L1符号缓存回写失败: %v", err)
		}
	}

	record, err := s.getDreamRecord(ctx, userID, dreamID)
	if err != nil {
		return nil, err
	}
	analysis := buildAnalysisResult(interpretation, record, locale)
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
	var dream dreamRow
	err := g.DB().Model("dreams").Where("id = ? AND user_id = ? AND deleted_at IS NULL", req.Id, userID).Scan(&dream)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query dream")
	}
	if dream.Id == 0 {
		return nil, gerror.New("dream not found")
	}

	// Query analysis_sessions table by dream_id and return the result_summary field
	var analysisSession entity.AnalysisSessions
	err = g.DB().Model("analysis_sessions").
		Where("dream_id = ? AND deleted_at IS NULL AND status = ?", req.Id, dreamStatusCompleted).
		OrderDesc("id").
		Limit(1).
		Scan(&analysisSession)
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
		Where("user_id = ? AND deleted_at IS NULL AND status = ?", userID, dreamStatusCompleted).
		Count()
	if err != nil {
		return nil, gerror.Wrap(err, "failed to count dreams")
	}
	recent, err := s.listDreamRecords(ctx, userID, homeRecentLimit)
	if err != nil {
		return nil, err
	}
	waves, err := s.listEmotionWaves(ctx, userID, homeWaveDays)
	if err != nil {
		return nil, err
	}
	streak, err := s.currentDreamDateStreak(ctx, userID)
	if err != nil {
		return nil, err
	}
	home := v1.GetDreamHomeRes{
		TotalDreams:       total,
		CurrentStreakDays: streak,
		EmotionWaves:      waves,
		RecentDreams:      recent,
	}
	if len(recent) > 0 {
		score := recent[0].ConfidenceScore
		home.Recommendation = &v1.DreamRecommendation{
			Dream: &recent[0],
			Score: score,
			Tier:  recommendationTier(score),
		}
	}
	return &home, nil
}

func (s *sHistory) GetTodayDreamRecommendation(ctx context.Context, req *v1.GetTodayDreamRecommendationReq) (*v1.GetTodayDreamRecommendationRes, error) {
	userID, err := getContextUserID(ctx)
	if err != nil {
		return nil, err
	}
	today := time.Now().Format("2006-01-02")
	var todayDreams []dreamRow
	err = g.DB().Model("dreams").
		Where("user_id = ? AND deleted_at IS NULL AND status = ? AND dream_date = ?", userID, dreamStatusCompleted, today).
		OrderDesc("confidence_score").
		OrderDesc("created_at").
		Limit(1).
		Scan(&todayDreams)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query today dream recommendation")
	}
	if len(todayDreams) == 0 {
		return nil, gerror.New("dream not found")
	}
	record := buildDreamRecordWithLatestAnalysis(ctx, todayDreams[0])
	res := v1.GetTodayDreamRecommendationRes{
		Dream: record,
		Score: record.ConfidenceScore,
		Tier:  recommendationTier(record.ConfidenceScore),
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
		Where("dream_id = ? AND deleted_at IS NULL AND status = ?", dream.Id, dreamStatusCompleted).
		OrderDesc("id").
		Limit(1).
		Scan(&analysis)
	return buildDreamRecord(dream, analysis.ResultSummary), nil
}

func (s *sHistory) listDreamRecords(ctx context.Context, userID uint64, limit int) ([]v1.DreamRecord, error) {
	var dreams []dreamRow
	err := g.DB().Model("dreams").
		Where("user_id = ? AND deleted_at IS NULL AND status = ?", userID, dreamStatusCompleted).
		OrderDesc("created_at").
		Limit(limit).
		Scan(&dreams)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query recent dreams")
	}
	records := make([]v1.DreamRecord, 0, len(dreams))
	for _, dream := range dreams {
		records = append(records, *buildDreamRecordWithLatestAnalysis(ctx, dream))
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
		dream.ConfidenceScore = defaultConfidenceScore
	}
	return &v1.DreamRecord{
		Id:              dream.Id,
		Title:           dream.Title,
		Content:         dream.Content,
		Interpretation:  interpretation,
		Emotion:         dream.Emotion,
		Keywords:        splitKeywords(dream.Tags),
		Symbols:         splitSymbols(dream.Symbols),
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
		Symbols:         record.Symbols,
		ConfidenceScore: record.ConfidenceScore,
		Locale:          locale,
	}
}

func buildDreamRecordWithLatestAnalysis(ctx context.Context, dream dreamRow) *v1.DreamRecord {
	var analysis entity.AnalysisSessions
	_ = g.DB().Model("analysis_sessions").
		Where("dream_id = ? AND deleted_at IS NULL AND status = ?", dream.Id, dreamStatusCompleted).
		OrderDesc("id").
		Limit(1).
		Scan(&analysis)
	return buildDreamRecord(dream, analysis.ResultSummary)
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

func splitSymbols(symbols string) []string {
	if symbols == "" {
		return []string{}
	}
	return normalizeDreamSymbols(strings.Split(symbols, ","))
}

func normalizeDreamSymbols(symbols []string) []string {
	seen := make(map[string]struct{}, len(symbols))
	normalized := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		normalized = append(normalized, symbol)
	}
	return normalized
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

func validateDreamContent(content string) error {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return gerror.New("dream content cannot be empty")
	}
	if utf8.RuneCountInString(trimmed) > maxDreamContentRunes {
		return gerror.New("dream content is too long")
	}
	return nil
}

func isAllowedDreamEmotion(emotion string) bool {
	_, ok := allowedDreamEmotions[strings.ToLower(strings.TrimSpace(emotion))]
	return ok
}

func filterHistoryEmotionTags(emotion string) []string {
	emotion = strings.TrimSpace(emotion)
	if emotion == "" {
		return nil
	}
	return []string{emotion}
}

func normalizeLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return "zh-CN"
	}
	if matched, _ := regexp.MatchString(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`, locale); !matched {
		return "zh-CN"
	}
	return locale
}

func updateAnalysisLifecycle(ctx context.Context, dreamID, sessionID uint64, status string, progress int, result string) error {
	now := time.Now()
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Model("dreams").
			Where("id = ? AND deleted_at IS NULL", dreamID).
			Data(g.Map{"status": status, "updated_at": now}).
			Update(); err != nil {
			return gerror.Wrap(err, "failed to update dream lifecycle")
		}
		data := g.Map{"status": status, "progress": progress, "updated_at": now}
		if result != "" {
			data["result_summary"] = result
		}
		if _, err := tx.Model("analysis_sessions").
			Where("id = ? AND dream_id = ? AND deleted_at IS NULL", sessionID, dreamID).
			Data(data).
			Update(); err != nil {
			return gerror.Wrap(err, "failed to update analysis lifecycle")
		}
		return nil
	})
}

func extractAnalysisTitleAndBody(text string) (string, string) {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return "Dream analysis", ""
	}
	title := ""
	lines := strings.Split(clean, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		line = strings.Trim(line, " 「」[]")
		if line != "" {
			title = line
			break
		}
	}
	if title == "" {
		title = "Dream analysis"
	}
	if utf8.RuneCountInString(title) > maxDreamTitleRunes {
		title = string([]rune(title)[:maxDreamTitleRunes])
	}
	return title, clean
}

func deriveDreamKeywords(content, interpretation, emotion string) []string {
	seen := map[string]struct{}{}
	keywords := make([]string, 0, 5)
	add := func(value string) {
		value = strings.Trim(strings.TrimSpace(value), "，。,.!！?？:：;；、|[]()（）「」\"'")
		if value == "" || utf8.RuneCountInString(value) > 24 {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		keywords = append(keywords, value)
	}
	for _, token := range regexp.MustCompile(`[\s,，。.!！?？、]+`).Split(content, -1) {
		add(token)
		if len(keywords) >= 4 {
			break
		}
	}
	if len(keywords) < 4 {
		for _, token := range regexp.MustCompile(`[\s,，。.!！?？、]+`).Split(interpretation, -1) {
			add(gstr.Trim(token))
			if len(keywords) >= 4 {
				break
			}
		}
	}
	add(emotion)
	if len(keywords) > 5 {
		return keywords[:5]
	}
	return keywords
}

func recommendationTier(score float64) string {
	switch {
	case score >= 0.9:
		return "high"
	case score >= 0.75:
		return "standard"
	default:
		return "low"
	}
}

func (s *sHistory) listEmotionWaves(ctx context.Context, userID uint64, days int) ([]v1.EmotionWavePoint, error) {
	start := time.Now().AddDate(0, 0, -days+1).Format("2006-01-02")
	var rows []struct {
		Date    string `orm:"date"`
		Emotion string `orm:"emotion"`
		Count   int    `orm:"count"`
	}
	err := g.DB().Model("dreams").
		Fields("TO_CHAR(dream_date, 'YYYY-MM-DD') AS date, emotion, COUNT(*) AS count").
		Where("user_id = ? AND deleted_at IS NULL AND status = ? AND dream_date >= ?", userID, dreamStatusCompleted, start).
		Group("TO_CHAR(dream_date, 'YYYY-MM-DD'), emotion").
		Order("date ASC").
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query emotion waves")
	}
	waves := make([]v1.EmotionWavePoint, 0, len(rows))
	for _, row := range rows {
		waves = append(waves, v1.EmotionWavePoint{Date: row.Date, Emotion: row.Emotion, Count: row.Count})
	}
	return waves, nil
}

func (s *sHistory) currentDreamDateStreak(ctx context.Context, userID uint64) (int, error) {
	var rows []struct {
		Date string `orm:"date"`
	}
	err := g.DB().Model("dreams").
		Fields("DISTINCT TO_CHAR(dream_date, 'YYYY-MM-DD') AS date").
		Where("user_id = ? AND deleted_at IS NULL AND status = ? AND dream_date <= ?", userID, dreamStatusCompleted, time.Now().Format("2006-01-02")).
		Order("date DESC").
		Scan(&rows)
	if err != nil {
		return 0, gerror.Wrap(err, "failed to query dream streak")
	}
	if len(rows) == 0 {
		return 0, nil
	}
	expected := time.Now()
	streak := 0
	for _, row := range rows {
		date, err := time.ParseInLocation("2006-01-02", row.Date, time.Local)
		if err != nil {
			return 0, fmt.Errorf("invalid dream date %q: %w", row.Date, err)
		}
		expectedDay := expected.Format("2006-01-02")
		if row.Date == expectedDay {
			streak++
			expected = expected.AddDate(0, 0, -1)
			continue
		}
		if streak == 0 && row.Date == expected.AddDate(0, 0, -1).Format("2006-01-02") {
			streak++
			expected = date.AddDate(0, 0, -1)
			continue
		}
		break
	}
	return streak, nil
}
