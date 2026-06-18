package integration

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	v1History "dm-server/api/history/v1"
	v1User "dm-server/api/user/v1"
	"dm-server/internal/consts"
	"dm-server/internal/model"
	"dm-server/internal/service"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/golang-jwt/jwt/v5"
)

// MockService implements service.IAuth, service.IDream, and service.IHistory
type MockService struct {
	mu           sync.RWMutex
	users        map[uint64]*v1User.UserInfo
	dreams       map[uint64]*v1History.DreamRecord
	dreamResults map[uint64]string
	settings     map[uint64]*v1User.UserSettings
	nextDreamID  uint64
	nextUserID   uint64
	config       TestConfig
	shouldFail   bool // flag to simulate service level errors
}

// NewMockService creates a self-contained MockService
func NewMockService(cfg TestConfig) *MockService {
	m := &MockService{
		users:        make(map[uint64]*v1User.UserInfo),
		dreams:       make(map[uint64]*v1History.DreamRecord),
		dreamResults: make(map[uint64]string),
		settings:     make(map[uint64]*v1User.UserSettings),
		nextDreamID:  100,
		nextUserID:   1,
		config:       cfg,
	}

	// Seed some mock data
	m.users[1] = &v1User.UserInfo{
		Id:       1,
		OpenId:   "test_openid_1",
		Nickname: "SleepyDreamer",
		Avatar:   "https://example.com/avatar.png",
	}

	m.dreams[1] = &v1History.DreamRecord{
		Id:              1,
		Title:           "飞翔之梦",
		Content:         "在云端自由飞翔的轻松梦境",
		Interpretation:  "梦见飞翔体现了潜意识中摆脱日常束缚、追求绝对自由的愿望满足。",
		Emotion:         "happy",
		Keywords:        []string{"飞翔", "自由"},
		Symbols:         []string{"天空"},
		ConfidenceScore: 0.88,
		IsFavorite:      false,
		CreatedAt:       "2026-05-20 10:00:00",
		UpdatedAt:       "2026-05-20 10:00:00",
	}
	m.dreamResults[1] = "# 飞翔之梦\n\n## 梦境元素映射\n| 意象 | 潜在象征 | 弗洛伊德对应 | 深层原型层 |\n|---|---|---|---|\n| 飞翔 | 摆脱束缚，追求掌控 | 愿望满足 | 自我超越原型 |\n\n## 多元解析视角\n### 弗洛伊德视角 (55%)\n梦见飞翔体现了潜意识中摆脱日常束缚、追求绝对自由的愿望满足。\n### 民俗视角 (45%)\n民俗中飞翔预示着事业上升或生活中的一次重大飞跃。\n\n## 现实结合\n- 发展性建议：勇敢尝试手头正在犹豫的创新项目。\n- 心理预警：防范从高度兴奋突然跌落的心理落差。\n- 机遇提示：3个月内或有重大职位晋升机会。"

	return m
}

// Register registers mock implementations to their respective packages
func (m *MockService) Register() {
	service.RegisterAuth(m)
	service.RegisterDream(m)
	service.RegisterHistory(m)
}

// SetShouldFail configures the mock to return errors
func (m *MockService) SetShouldFail(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = fail
}

// ==========================================
// service.IAuth Implementation
// ==========================================

func (m *MockService) WechatLogin(ctx context.Context, req *v1User.WechatAuthReq) (*v1User.WechatAuthRes, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return nil, gerror.New("Mock WeChat Authentication Server Unavailable")
	}

	if req.Code == "" {
		return nil, gerror.New("WeChat Code cannot be empty")
	}

	// Generate user
	userID := m.nextUserID
	m.nextUserID++
	openid := fmt.Sprintf("openid_%s", req.Code)

	userInfo := &v1User.UserInfo{
		Id:       userID,
		OpenId:   openid,
		Nickname: fmt.Sprintf("微信用户_%d", userID),
		Avatar:   "https://example.com/default-avatar.png",
	}
	m.users[userID] = userInfo

	token, err := GenerateTestToken(userID, openid, m.config.JWTSecret)
	if err != nil {
		return nil, gerror.Wrap(err, "Mock JWT generation failed")
	}

	return &v1User.WechatAuthRes{
		Token:    token,
		OpenId:   openid,
		UserInfo: userInfo,
	}, nil
}

func (m *MockService) EmailLogin(ctx context.Context, req *v1User.EmailAuthReq) (*v1User.EmailAuthRes, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return nil, gerror.New("Mock Supabase Authentication Unavailable")
	}

	if req.Email == "" || req.Password == "" {
		return nil, gerror.New("email/password cannot be empty")
	}

	userID := m.nextUserID
	m.nextUserID++

	email := req.Email
	supabaseUID := fmt.Sprintf("supabase_uid_%s", email)

	userInfo := &v1User.UserInfo{
		Id:       userID,
		OpenId:   supabaseUID,
		Nickname: fmt.Sprintf("Email用户_%d", userID),
		Avatar:   "https://example.com/default-avatar.png",
		Email:    email,
		Lang:     nil,
	}
	m.users[userID] = userInfo

	token, err := GenerateTestToken(userID, supabaseUID, m.config.JWTSecret)
	if err != nil {
		return nil, gerror.Wrap(err, "Mock JWT generation failed")
	}

	return &v1User.EmailAuthRes{
		Token:    token,
		UserInfo: userInfo,
		AuthFlow: "signedIn",
	}, nil
}

func (m *MockService) GetUserInfo(ctx context.Context, req *v1User.GetUserInfoReq) (*v1User.GetUserInfoRes, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldFail {
		return nil, gerror.New("Mock Database Error")
	}

	// Extract userId from Context as the real server does
	userIDVal := ctx.Value(consts.CtxUserId)
	if userIDVal == nil {
		userIDVal = ctx.Value("userId")
	}
	if userIDVal == nil {
		return nil, gerror.New("User not logged in (context user ID is empty)")
	}

	var userID uint64
	switch v := userIDVal.(type) {
	case uint64:
		userID = v
	case float64:
		userID = uint64(v)
	case int:
		userID = uint64(v)
	default:
		return nil, gerror.New("Invalid Context User ID type")
	}

	user, exists := m.users[userID]
	if !exists {
		// Provide a default user to prevent test failures on dynamic JWTs
		user = &v1User.UserInfo{
			Id:       userID,
			OpenId:   fmt.Sprintf("openid_%d", userID),
			Nickname: fmt.Sprintf("测试用户_%d", userID),
			Avatar:   "https://example.com/default.png",
		}
		m.users[userID] = user
	}

	return &v1User.GetUserInfoRes{
		UserInfo: user,
	}, nil
}

func (m *MockService) UpdateUserInfo(ctx context.Context, req *v1User.UpdateUserInfoReq) (*v1User.UpdateUserInfoRes, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return nil, gerror.New("Mock Database Error")
	}

	userIDVal := ctx.Value(consts.CtxUserId)
	if userIDVal == nil {
		userIDVal = ctx.Value("userId")
	}
	if userIDVal == nil {
		return nil, gerror.New("User not logged in (context user ID is empty)")
	}

	var userID uint64
	switch v := userIDVal.(type) {
	case uint64:
		userID = v
	case float64:
		userID = uint64(v)
	case int:
		userID = uint64(v)
	default:
		return nil, gerror.New("Invalid Context User ID type")
	}

	user, exists := m.users[userID]
	if !exists {
		user = &v1User.UserInfo{
			Id:     userID,
			OpenId: fmt.Sprintf("openid_%d", userID),
		}
		m.users[userID] = user
	}

	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}
	return &v1User.UpdateUserInfoRes{
		UserInfo: user,
	}, nil
}

func (m *MockService) GetUserSettings(ctx context.Context, req *v1User.GetUserSettingsReq) (*v1User.GetUserSettingsRes, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	userID, err := mockUserID(ctx)
	if err != nil {
		return nil, err
	}
	settings := m.settings[userID]
	if settings == nil {
		enabled := false
		settings = &v1User.UserSettings{
			Language:             "zh-CN",
			PrivacyMode:          "private",
			DreamReminderEnabled: &enabled,
			StorageMode:          "local_cache",
		}
		m.settings[userID] = settings
	}
	res := v1User.GetUserSettingsRes(*settings)
	return &res, nil
}

func (m *MockService) UpdateUserSettings(ctx context.Context, req *v1User.UpdateUserSettingsReq) (*v1User.UpdateUserSettingsRes, error) {
	current, err := m.GetUserSettings(ctx, &v1User.GetUserSettingsReq{})
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	userID, err := mockUserID(ctx)
	if err != nil {
		return nil, err
	}
	settings := v1User.UserSettings(*current)
	if req.Language != "" {
		settings.Language = req.Language
	}
	if req.PrivacyMode != "" {
		settings.PrivacyMode = req.PrivacyMode
	}
	if req.DreamReminderEnabled != nil {
		settings.DreamReminderEnabled = req.DreamReminderEnabled
	}
	if req.DreamReminderTime != "" {
		settings.DreamReminderTime = req.DreamReminderTime
	}
	if req.StorageMode != "" {
		settings.StorageMode = req.StorageMode
	}
	m.settings[userID] = &settings
	res := v1User.UpdateUserSettingsRes(settings)
	return &res, nil
}

func (m *MockService) GetPsycheProfile(ctx context.Context, req *v1User.GetPsycheProfileReq) (*v1User.GetPsycheProfileRes, error) {
	profile := v1User.GetPsycheProfileRes{
		IntegrationScore:       62,
		IntegrationLevel:       "moderate",
		IntegrationDescription: "Your dream journal shows emerging symbolic continuity.",
		Archetypes: []v1User.ArchetypeProfileItem{
			{Type: "self", Name: "Self", Score: 62, Description: "Wholeness and inner alignment"},
			{Type: "persona", Name: "Persona", Score: 48, Description: "Social identity and roles"},
			{Type: "shadow", Name: "Shadow", Score: 42, Description: "Unintegrated emotions"},
			{Type: "anima", Name: "Anima", Score: 38, Description: "Inner symbolic imagination"},
			{Type: "sage", Name: "Sage", Score: 44, Description: "Insight and meaning-making"},
		},
		DominantArchetype: "self",
		UpdatedAt:         time.Now().Format("2006-01-02 15:04:05"),
	}
	return &profile, nil
}

func (m *MockService) VerifyJWT(ctx context.Context, tokenString string) (*model.AuthClaims, error) {
	// Directly verify token string locally
	claims, err := service.Auth().(*MockService).verifyJWTLocal(tokenString)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func (m *MockService) verifyJWTLocal(tokenString string) (*model.AuthClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return []byte(m.config.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, gerror.New("Invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, gerror.New("Invalid token claims")
	}

	var userID uint64
	if id, exists := claims["userId"]; exists {
		switch v := id.(type) {
		case float64:
			userID = uint64(v)
		case uint64:
			userID = v
		}
	}

	var openID string
	if oid, exists := claims["openid"]; exists {
		openID = oid.(string)
	}

	return &model.AuthClaims{
		ID:     userID,
		OpenID: openID,
	}, nil
}

// ==========================================
// service.IHistory Implementation
// ==========================================

func (m *MockService) FetchDreamList(ctx context.Context, req *v1History.FetchDreamListReq) (*v1History.FetchDreamListRes, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldFail {
		return nil, gerror.New("Database fetch failed")
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}

	var list []v1History.DreamRecord
	for _, dream := range m.dreams {
		if req.Keyword != "" && !strings.Contains(dream.Title, req.Keyword) && !strings.Contains(dream.Content, req.Keyword) && !strings.Contains(dream.Interpretation, req.Keyword) {
			continue
		}
		if req.Emotion != "" && dream.Emotion != req.Emotion {
			continue
		}
		if req.FavoriteOnly && !dream.IsFavorite {
			continue
		}
		list = append(list, *dream)
	}

	total := int64(len(list))
	hasDateRange := req.StartDate != "" && req.EndDate != ""
	if req.StartDate != "" && req.EndDate == "" || req.StartDate == "" && req.EndDate != "" {
		return nil, gerror.New("StartDate and EndDate must be provided together")
	}

	if hasDateRange {
		return &v1History.FetchDreamListRes{
			Items: list,
			Total: total,
		}, nil
	}

	// Implement simple pagination
	startIdx := (page - 1) * pageSize
	if startIdx >= len(list) {
		return &v1History.FetchDreamListRes{
			Items:    []v1History.DreamRecord{},
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		}, nil
	}

	endIdx := startIdx + pageSize
	if endIdx > len(list) {
		endIdx = len(list)
	}

	return &v1History.FetchDreamListRes{
		Items:    list[startIdx:endIdx],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  endIdx < len(list),
	}, nil
}

func (m *MockService) DeleteDream(ctx context.Context, req *v1History.DeleteDreamReq) (*v1History.DeleteDreamRes, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return nil, gerror.New("Delete dream database transaction failed")
	}

	if _, exists := m.dreams[req.Id]; !exists {
		return &v1History.DeleteDreamRes{Success: false}, nil
	}

	delete(m.dreams, req.Id)
	delete(m.dreamResults, req.Id)
	return &v1History.DeleteDreamRes{Success: true}, nil
}

func (m *MockService) GetDreamAnalyzeResult(ctx context.Context, req *v1History.GetDreamAnalyzeResultReq) (*v1History.GetDreamAnalyzeResultRes, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldFail {
		return nil, gerror.New("Database query error")
	}

	result, exists := m.dreamResults[req.Id]
	if !exists {
		return nil, gerror.New("Dream analysis result not found")
	}

	return &v1History.GetDreamAnalyzeResultRes{
		Result: result,
	}, nil
}

func (m *MockService) GetDream(ctx context.Context, req *v1History.GetDreamReq) (*v1History.GetDreamRes, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, err := m.mockDreamRecord(req.Id)
	if err != nil {
		return nil, err
	}
	res := v1History.GetDreamRes(*record)
	return &res, nil
}

func (m *MockService) UpdateDream(ctx context.Context, req *v1History.UpdateDreamReq) (*v1History.UpdateDreamRes, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	dream, exists := m.dreams[req.Id]
	if !exists {
		return nil, gerror.New("Dream not found")
	}
	if req.Title != "" {
		dream.Title = req.Title
	}
	if req.Content != "" {
		dream.Content = req.Content
	}
	record, err := m.mockDreamRecord(req.Id)
	if err != nil {
		return nil, err
	}
	res := v1History.UpdateDreamRes(*record)
	return &res, nil
}

func (m *MockService) CreateDreamAnalysis(ctx context.Context, req *v1History.CreateDreamAnalysisReq) (*v1History.CreateDreamAnalysisRes, error) {
	ch, err := m.StreamDream(ctx, req.Content)
	if err != nil {
		return nil, err
	}
	var builder strings.Builder
	for piece := range ch {
		builder.WriteString(piece)
	}
	id := m.nextDreamID - 1
	record, err := m.mockDreamRecord(id)
	if err != nil {
		return nil, err
	}
	record.Content = req.Content
	record.Interpretation = builder.String()
	record.Emotion = req.Emotion
	return &v1History.CreateDreamAnalysisRes{
		Dream: record,
		Analysis: &v1History.DreamAnalysisResult{
			Summary:         record.Title,
			Interpretation:  builder.String(),
			Keywords:        []string{},
			ConfidenceScore: record.ConfidenceScore,
			Locale:          req.Locale,
		},
		Steps: []v1History.DreamAnalysisStep{
			{Key: "record", Title: "Record dream", Description: "Dream content saved", Status: "completed"},
			{Key: "analyze", Title: "Analyze symbols", Description: "Dream interpretation generated", Status: "completed"},
		},
	}, nil
}

func (m *MockService) SetDreamFavorite(ctx context.Context, req *v1History.SetDreamFavoriteReq) (*v1History.SetDreamFavoriteRes, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	dream, exists := m.dreams[req.Id]
	if !exists {
		return nil, gerror.New("Dream not found")
	}
	dream.IsFavorite = req.IsFavorite
	record, err := m.mockDreamRecord(req.Id)
	if err != nil {
		return nil, err
	}
	res := v1History.SetDreamFavoriteRes(*record)
	return &res, nil
}

func (m *MockService) GetDreamHome(ctx context.Context, req *v1History.GetDreamHomeReq) (*v1History.GetDreamHomeRes, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	recent := make([]v1History.DreamRecord, 0, len(m.dreams))
	for id := range m.dreams {
		record, _ := m.mockDreamRecord(id)
		recent = append(recent, *record)
	}
	home := v1History.GetDreamHomeRes{
		TotalDreams:       len(recent),
		CurrentStreakDays: len(recent),
		RecentDreams:      recent,
		EmotionWaves:      []v1History.EmotionWavePoint{},
	}
	if len(recent) > 0 {
		home.Recommendation = &v1History.DreamRecommendation{Dream: &recent[0], Score: 0.86, Tier: "standard"}
	}
	return &home, nil
}

func (m *MockService) GetTodayDreamRecommendation(ctx context.Context, req *v1History.GetTodayDreamRecommendationReq) (*v1History.GetTodayDreamRecommendationRes, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for id := range m.dreams {
		record, _ := m.mockDreamRecord(id)
		res := v1History.GetTodayDreamRecommendationRes{Dream: record, Score: 0.86, Tier: "standard"}
		return &res, nil
	}
	return nil, gerror.New("Dream not found")
}

// ==========================================
// service.IDream Implementation
// ==========================================

func (m *MockService) ExtractDreamSymbols(ctx context.Context, content string, emotionTags []string) ([]string, error) {
	return []string{}, nil
}

func (m *MockService) SinkDreamSymbolCache(ctx context.Context, userId string, symbols []string, interpretation string, sourceDreamId string) error {
	return nil
}

func (m *MockService) StreamDream(ctx context.Context, content string) (<-chan string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return nil, gerror.New("AI analysis engine overloaded")
	}

	ch := make(chan string, 5)

	// Save dream summary and result mock
	dreamID := m.nextDreamID
	m.nextDreamID++

	m.dreams[dreamID] = &v1History.DreamRecord{
		Id:              dreamID,
		Title:           "解析梦境",
		Content:         content,
		Interpretation:  "",
		Emotion:         "neutral",
		Keywords:        []string{},
		Symbols:         []string{},
		ConfidenceScore: 0.86,
		IsFavorite:      false,
		CreatedAt:       time.Now().Format("2006-01-02 15:04:05"),
		UpdatedAt:       time.Now().Format("2006-01-02 15:04:05"),
	}

	markdownOutput := []string{
		"# 梦境解析专家分析报告\n\n",
		"## 梦境元素映射\n",
		"| 意象 | 潜在象征 | 弗洛伊德对应 | 深层原型层 |\n",
		"|------|----------|--------------|------------|\n",
		fmt.Sprintf("| 场景 | %s | 潜意识意愿 | 自我映射 |\n\n", content),
		"## 多元解析视角\n",
		"### 弗洛伊德视角 (55%)\n",
		"此梦反映了做梦者近期的心理张力和愿望补偿。\n\n",
		"### 民俗视角 (45%)\n",
		"民俗中预示着局势的缓和与人际关系的改善。\n\n",
		"## 综合小结\n",
		"综合而言，这个梦境代表了自我调节和心理治愈的过程。\n\n",
		"## 现实结合\n",
		"- 发展性建议：保持稳定的作息，记下更多梦境。\n",
		"- 心理预警：注意适当放松，避免过度焦虑。",
	}

	m.dreamResults[dreamID] = strings.Join(markdownOutput, "")

	go func() {
		defer close(ch)

		for _, chunk := range markdownOutput {
			// Simulate latency and network jitter
			sleepTime := m.config.Latency
			if m.config.JitterMax > 0 {
				sleepTime += time.Duration(rand.Int63n(int64(m.config.JitterMax)))
			}

			select {
			case <-ctx.Done():
				// Instantly cancel if client aborts or closes connection
				return
			case <-time.After(sleepTime):
				ch <- chunk
			}
		}
	}()

	return ch, nil
}

func (m *MockService) mockDreamRecord(id uint64) (*v1History.DreamRecord, error) {
	dream, exists := m.dreams[id]
	if !exists {
		return nil, gerror.New("Dream not found")
	}
	record := *dream
	if record.Interpretation == "" {
		record.Interpretation = m.dreamResults[id]
	}
	if record.Emotion == "" {
		record.Emotion = "neutral"
	}
	if record.Keywords == nil {
		record.Keywords = []string{}
	}
	if record.Symbols == nil {
		record.Symbols = []string{}
	}
	if record.ConfidenceScore == 0 {
		record.ConfidenceScore = 0.86
	}
	return &record, nil
}

func mockUserID(ctx context.Context) (uint64, error) {
	userIDVal := ctx.Value(consts.CtxUserId)
	if userIDVal == nil {
		userIDVal = ctx.Value("userId")
	}
	if userIDVal == nil {
		return 0, gerror.New("User not logged in (context user ID is empty)")
	}
	switch v := userIDVal.(type) {
	case uint64:
		return v, nil
	case float64:
		return uint64(v), nil
	case int:
		return uint64(v), nil
	default:
		return 0, gerror.New("Invalid Context User ID type")
	}
}
