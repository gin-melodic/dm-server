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
	dreams       map[uint64]*v1History.DreamSummary
	dreamResults map[uint64]string
	nextDreamID  uint64
	nextUserID   uint64
	config       TestConfig
	shouldFail   bool // flag to simulate service level errors
}

// NewMockService creates a self-contained MockService
func NewMockService(cfg TestConfig) *MockService {
	m := &MockService{
		users:        make(map[uint64]*v1User.UserInfo),
		dreams:       make(map[uint64]*v1History.DreamSummary),
		dreamResults: make(map[uint64]string),
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

	m.dreams[1] = &v1History.DreamSummary{
		Id:         1,
		Title:      "飞翔之梦",
		Summary:    "在云端自由飞翔的轻松梦境",
		CreateTime: "2026-05-20 10:00:00",
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

	if req.AccessToken == "" {
		return nil, gerror.New("Access token cannot be empty")
	}

	userID := m.nextUserID
	m.nextUserID++

	email := fmt.Sprintf("user_%d@example.com", userID)
	supabaseUID := fmt.Sprintf("supabase_uid_%s", req.AccessToken)

	userInfo := &v1User.UserInfo{
		Id:       userID,
		OpenId:   supabaseUID,
		Nickname: fmt.Sprintf("Email用户_%d", userID),
		Avatar:   "https://example.com/default-avatar.png",
		Email:    email,
	}
	m.users[userID] = userInfo

	token, err := GenerateTestToken(userID, supabaseUID, m.config.JWTSecret)
	if err != nil {
		return nil, gerror.Wrap(err, "Mock JWT generation failed")
	}

	return &v1User.EmailAuthRes{
		Token:    token,
		UserInfo: userInfo,
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

	user.Nickname = req.Nickname
	user.Avatar = req.Avatar

	return &v1User.UpdateUserInfoRes{
		UserInfo: user,
	}, nil
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

	if req.StartDate == "" || req.EndDate == "" {
		return nil, gerror.New("StartDate and EndDate are required")
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}

	var list []v1History.DreamSummary
	for _, dream := range m.dreams {
		// Simply add all to list (date filtering is mocked as matching)
		list = append(list, *dream)
	}

	total := int64(len(list))

	// Implement simple pagination
	startIdx := (page - 1) * pageSize
	if startIdx >= len(list) {
		return &v1History.FetchDreamListRes{
			Data:     []v1History.DreamSummary{},
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		}, nil
	}

	endIdx := startIdx + pageSize
	if endIdx > len(list) {
		endIdx = len(list)
	}

	return &v1History.FetchDreamListRes{
		Data:     list[startIdx:endIdx],
		Page:     page,
		PageSize: pageSize,
		Total:    total,
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

// ==========================================
// service.IDream Implementation
// ==========================================

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

	m.dreams[dreamID] = &v1History.DreamSummary{
		Id:         dreamID,
		Title:      "解析梦境",
		Summary:    strings.Fields(content)[0] + "...",
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
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
