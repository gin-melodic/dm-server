package history

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	v1 "dm-server/api/history/v1"
	"dm-server/internal/consts"
	"dm-server/internal/model"
	"dm-server/internal/service"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
	"github.com/gogf/gf/v2/util/guid"
)

type fakeDreamStream struct {
	chunks             []string
	err                error
	emotionTags        *[]string
	symbols            []string
	sunkSymbols        *[]string
	sinkCalled         *bool
	sinkSourceDream    *string
	sinkInterpretation *string
}

func (f fakeDreamStream) ExtractDreamSymbols(ctx context.Context, content string, emotionTags []string) ([]string, error) {
	return f.symbols, nil
}

func (f fakeDreamStream) SinkDreamSymbolCache(ctx context.Context, userId string, symbols []string, interpretation string, sourceDreamId string) error {
	if f.sinkCalled != nil {
		*f.sinkCalled = true
	}
	if f.sunkSymbols != nil {
		*f.sunkSymbols = append((*f.sunkSymbols)[:0], symbols...)
	}
	if f.sinkSourceDream != nil {
		*f.sinkSourceDream = sourceDreamId
	}
	if f.sinkInterpretation != nil {
		*f.sinkInterpretation = interpretation
	}
	return nil
}

func (f fakeDreamStream) StreamDream(ctx context.Context, content string) (<-chan model.DreamStreamEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.emotionTags != nil {
		tags, _ := ctx.Value(consts.CtxDreamEmotionTags).([]string)
		*f.emotionTags = append((*f.emotionTags)[:0], tags...)
	}
	ch := make(chan model.DreamStreamEvent, len(f.chunks)+1)
	go func() {
		defer close(ch)
		for _, chunk := range f.chunks {
			ch <- model.DreamStreamEvent{Type: model.DreamStreamEventDelta, Content: chunk}
		}
		ch <- model.DreamStreamEvent{Type: model.DreamStreamEventCompleted, FinishReason: "stop"}
	}()
	return ch, nil
}

func TestHistoryLogicDatabaseFlow(t *testing.T) {
	ctx := context.Background()
	configureTestDB(t)
	if _, err := g.DB().GetValue(ctx, "SELECT 1"); err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	if _, err := g.DB().GetValue(ctx, "SELECT emotion FROM dreams LIMIT 0"); err != nil {
		t.Skipf("dream API schema changes are not applied: %v", err)
	}
	if _, err := g.DB().GetValue(ctx, "SELECT symbols FROM dreams LIMIT 0"); err != nil {
		t.Skipf("dream symbol schema changes are not applied: %v", err)
	}

	userID := createLogicTestUser(t, ctx)
	t.Cleanup(func() { cleanupLogicTestUser(ctx, userID) })
	userCtx := context.WithValue(ctx, consts.CtxUserId, userID)
	svc := New()

	var observedEmotionTags []string
	var sinkCalled bool
	service.RegisterDream(fakeDreamStream{
		chunks:      []string{"# 「清晨之门」\n\n", "## 综合小结\n醒来后感觉轻松。"},
		emotionTags: &observedEmotionTags,
		symbols:     []string{"门", "光"},
		sinkCalled:  &sinkCalled,
	})
	analyzeRes, err := svc.CreateDreamAnalysis(userCtx, &v1.CreateDreamAnalysisReq{
		Content: "我梦见自己穿过一扇发光的门",
		Emotion: "peaceful",
		Locale:  "en-US",
	})
	if err != nil {
		t.Fatalf("CreateDreamAnalysis failed: %v", err)
	}
	if analyzeRes.Dream == nil || analyzeRes.Dream.Id == 0 {
		t.Fatalf("expected persisted dream, got %#v", analyzeRes.Dream)
	}
	if analyzeRes.Analysis.Locale != "en" || analyzeRes.Dream.Emotion != "peaceful" {
		t.Fatalf("unexpected analysis result: %#v", analyzeRes)
	}
	if len(observedEmotionTags) != 1 || observedEmotionTags[0] != "peaceful" {
		t.Fatalf("expected emotion tag to be forwarded, got %#v", observedEmotionTags)
	}
	if len(analyzeRes.Dream.Symbols) != 2 || analyzeRes.Dream.Symbols[0] != "门" || analyzeRes.Analysis.Symbols[1] != "光" {
		t.Fatalf("expected symbols to be returned, got dream=%#v analysis=%#v", analyzeRes.Dream.Symbols, analyzeRes.Analysis.Symbols)
	}
	if sinkCalled {
		t.Fatal("symbol-level L1 cache should not be written after final analysis")
	}

	detail, err := svc.GetDream(userCtx, &v1.GetDreamReq{Id: analyzeRes.Dream.Id})
	if err != nil {
		t.Fatalf("GetDream failed: %v", err)
	}
	if !strings.Contains(detail.Interpretation, "醒来后感觉轻松") {
		t.Fatalf("expected completed analysis interpretation, got %q", detail.Interpretation)
	}
	if len(detail.Symbols) != 2 || detail.Symbols[0] != "门" {
		t.Fatalf("expected persisted symbols, got %#v", detail.Symbols)
	}

	updateRes, err := svc.UpdateDream(userCtx, &v1.UpdateDreamReq{Id: detail.Id, Content: "我重新记录了这个梦", Emotion: "neutral"})
	if err != nil {
		t.Fatalf("UpdateDream failed: %v", err)
	}
	if updateRes.Interpretation != "" {
		t.Fatalf("expected content update to invalidate analysis, got %q", updateRes.Interpretation)
	}
	if len(updateRes.Symbols) != 0 {
		t.Fatalf("expected content update to clear symbols, got %#v", updateRes.Symbols)
	}
	statusValue, err := g.DB().Model("dreams").Fields("status").Where("id", detail.Id).Value()
	if err != nil {
		t.Fatalf("query updated status: %v", err)
	}
	status := statusValue.String()
	if status != dreamStatusPending {
		t.Fatalf("expected pending after content update, got %q", status)
	}

	completeDreamID := insertLogicTestDream(t, ctx, userID, time.Now(), "happy", 0.94)
	if _, err := svc.SetDreamFavorite(userCtx, &v1.SetDreamFavoriteReq{Id: completeDreamID, IsFavorite: true}); err != nil {
		t.Fatalf("SetDreamFavorite failed: %v", err)
	}
	olderDreamID := insertLogicTestDream(t, ctx, userID, time.Now().AddDate(0, 0, -1), "anxious", 0.82)
	pagedList, err := svc.FetchDreamList(userCtx, &v1.FetchDreamListReq{Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("FetchDreamList default pagination failed: %v", err)
	}
	if pagedList.Page != 1 || pagedList.PageSize != 1 || !pagedList.HasMore || len(pagedList.Items) != 1 || pagedList.Total < 2 {
		t.Fatalf("unexpected paged list: %#v", pagedList)
	}
	rangeList, err := svc.FetchDreamList(userCtx, &v1.FetchDreamListReq{
		StartDate: time.Now().AddDate(0, 0, -2).Format("2006-01-02"),
		EndDate:   time.Now().AddDate(0, 0, 1).Format("2006-01-02"),
		Page:      1,
		PageSize:  1,
	})
	if err != nil {
		t.Fatalf("FetchDreamList date range failed: %v", err)
	}
	if rangeList.Page != 0 || rangeList.PageSize != 0 || len(rangeList.Items) < 2 {
		t.Fatalf("expected date range to return all matches without pagination, got %#v", rangeList)
	}
	filteredList, err := svc.FetchDreamList(userCtx, &v1.FetchDreamListReq{Emotion: "happy", FavoriteOnly: true})
	if err != nil {
		t.Fatalf("FetchDreamList emotion favorite filter failed: %v", err)
	}
	if filteredList.Total != 1 || len(filteredList.Items) != 1 || filteredList.Items[0].Id != completeDreamID {
		t.Fatalf("unexpected emotion favorite filter result: %#v", filteredList)
	}
	keywordList, err := svc.FetchDreamList(userCtx, &v1.FetchDreamListReq{Keyword: "completed test analysis"})
	if err != nil {
		t.Fatalf("FetchDreamList keyword filter failed: %v", err)
	}
	if keywordList.Total < 2 || !containsDreamID(keywordList.Items, olderDreamID) {
		t.Fatalf("expected keyword search to match analysis text, got %#v", keywordList)
	}
	home, err := svc.GetDreamHome(userCtx, &v1.GetDreamHomeReq{})
	if err != nil {
		t.Fatalf("GetDreamHome failed: %v", err)
	}
	if home.TotalDreams == 0 || home.Recommendation == nil || len(home.RecentDreams) == 0 {
		t.Fatalf("unexpected home result: %#v", home)
	}
	today, err := svc.GetTodayDreamRecommendation(userCtx, &v1.GetTodayDreamRecommendationReq{})
	if err != nil {
		t.Fatalf("GetTodayDreamRecommendation failed: %v", err)
	}
	if today.Dream == nil || today.Dream.Id != completeDreamID || today.Tier != "high" {
		t.Fatalf("unexpected today recommendation: %#v", today)
	}
}

func configureTestDB(t *testing.T) {
	t.Helper()
	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		if err := adapter.SetPath("../../../manifest/config"); err != nil {
			t.Fatalf("set config path: %v", err)
		}
	}
}

func createLogicTestUser(t *testing.T, ctx context.Context) uint64 {
	t.Helper()
	openID := "logic_test_" + guid.S()
	id, err := g.DB().Model("users").Data(g.Map{
		"openid": openID,
	}).InsertAndGetId()
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	return uint64(id)
}

func cleanupLogicTestUser(ctx context.Context, userID uint64) {
	_, _ = g.DB().Exec(ctx, "DELETE FROM analysis_sessions WHERE dream_id IN (SELECT id FROM dreams WHERE user_id = ?)", userID)
	_, _ = g.DB().Model("dreams").Where("user_id", userID).Delete()
	_, _ = g.DB().Model("user_settings").Where("user_id", userID).Delete()
	_, _ = g.DB().Model("users").Where("id", userID).Delete()
}

func insertLogicTestDream(t *testing.T, ctx context.Context, userID uint64, dreamDate time.Time, emotion string, confidence float64) uint64 {
	t.Helper()
	id, err := g.DB().Model("dreams").Data(g.Map{
		"user_id":          userID,
		"title":            fmt.Sprintf("logic dream %d", time.Now().UnixNano()),
		"content":          "logic test dream",
		"dream_date":       dreamDate.Format("2006-01-02"),
		"tags":             "guide,teacher",
		"symbols":          "老师,门",
		"emotion":          emotion,
		"status":           dreamStatusCompleted,
		"confidence_score": confidence,
	}).InsertAndGetId()
	if err != nil {
		t.Fatalf("insert test dream: %v", err)
	}
	_, err = g.DB().Model("analysis_sessions").Data(g.Map{
		"dream_id":       id,
		"session_uuid":   guid.S(),
		"analysis_type":  "dream",
		"status":         dreamStatusCompleted,
		"progress":       100,
		"result_summary": "completed test analysis",
	}).Insert()
	if err != nil {
		t.Fatalf("insert test analysis: %v", err)
	}
	return uint64(id)
}

func containsDreamID(items []v1.DreamRecord, dreamID uint64) bool {
	for _, item := range items {
		if item.Id == dreamID {
			return true
		}
	}
	return false
}
